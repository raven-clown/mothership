# Mothership: shared contract

Four Go projects that each work on their own and fit together when used side by side. This file is the agreement between them. Each project's own spec must follow it, and a change here is agreed before any project depends on it.

The four projects and the `mothership` repository that ties them together share a spaceflight theme.

| Project | Name | What it is |
|---|---|---|
| 1 | `airlock` | MCP gateway: one front door for many MCP servers, with login, permissions, audit log and rate limits |
| 2 | `rosetta` | Protocol bridge: write an agent once, serve it over MCP, A2A, ACP and AG-UI |
| 3a | `transponder` | LLM gateway: one API in front of Anthropic, OpenAI, Gemini and Ollama, with fallback and cost tracking |
| 3b | `logbook` | Agent memory: a shared memory any agent can read and write, served as an MCP server |

## How they connect

```
 editor / web UI / other agents
        |  MCP, A2A, ACP, AG-UI
    [rosetta] ------------------------+
        |  MCP                        |  OpenAI-compatible HTTP
    [airlock]                  [transponder] --> Anthropic, OpenAI, Gemini, Ollama
        |  MCP                        ^
   +----+---------+                   |  embeddings, extraction
 [logbook]  other MCP servers         |
   +----------------------------------+
```

- Clients reach tools through `airlock`. `logbook` is one of the MCP servers behind it.
- Agents hosted by `rosetta` call tools through `airlock` and models through `transponder`.
- `logbook` calls `transponder` for embeddings and for pulling facts out of text.
- None of these links is required. Each project runs alone, and each link is a URL in its config.

## Interfaces

**airlock**
- Upstream: an MCP server over Streamable HTTP, plus stdio for local use.
- Downstream: MCP servers over stdio or Streamable HTTP, listed in config.
- Tool names are prefixed with the server name: `github__create_issue`.

**logbook**
- An MCP server over Streamable HTTP and stdio. Tools: `remember`, `recall`, `verify`, `forget`, `list_memories`.
- The same operations over a small REST API under `/v1/`.
- Memories belong to a scope: `user`, `project` or `team`, taken from the caller's identity.

**transponder**
- OpenAI-compatible `/v1/chat/completions` and `/v1/embeddings`.
- Anthropic-compatible `/v1/messages`, so Claude clients work unchanged.
- Usage and cost per call are reported in the response headers listed below. A streamed response sends the same names as HTTP trailers, and the stream's own usage event carries the counts for clients that can't read trailers. Callers that stream read the trailers.
- Besides `Authorization: Bearer`, it accepts `x-api-key`, because Anthropic clients send that. Only transponder accepts it.
- Route names are stable aliases for models, so another project can ask for `memory-extract` without knowing which provider serves it.
- An `echo` provider answers without calling anyone, so tests and the mothership end-to-end run need no keys.

**rosetta**
- Agents are written once against one Go interface, or declared in config.
- Each agent is served as an MCP server, an A2A agent, an ACP agent over stdio, and an AG-UI endpoint over SSE.
- It is a client of `airlock` and `transponder` when those are configured.

## Shared conventions

**Identity**
- Callers send `Authorization: Bearer <token>`.
- Tokens are OIDC JWTs checked against a configured issuer, or static API keys for service-to-service calls. Any OIDC provider works, including IdpForge.
- Inside the stack, a component that calls another forwards the caller's token unchanged, so every component sees the same user. The stack components accept the same audience.
- airlock doesn't forward the caller's token to third-party MCP servers by default, since the MCP authorization spec warns against token passthrough. Forwarding is opt-in per backend, and token exchange (RFC 8693) can come later.
- `X-Project` names the project a call belongs to. airlock can set it per caller, and logbook uses it to scope memories.
- Every table that holds runtime or usage data has an `org_id` column, `default` until organisations ship, and every query filters by it. Multi-tenancy then needs no data migration.

**Headers**
- `traceparent` and `tracestate` (W3C Trace Context) are forwarded on every hop.
- `X-Request-ID` is kept if present, and generated if not.
- `transponder` adds `X-Usage-Input-Tokens`, `X-Usage-Output-Tokens` and `X-Usage-Cost-USD`.

**Operations**
- One static binary per project, configured by a YAML file passed with `--config`. An environment variable can override any key: `AIRLOCK_LISTEN_ADDR` overrides `listen_addr`, and `__` separates nested keys, as in `AIRLOCK_STORAGE__DSN` for `storage.dsn`.
- With no config at all, a project starts on SQLite in the working directory and prints a one-time admin API key to stderr. That key is how a new user opens the console without writing YAML.
- In stdio mode, logs go to stderr, so stdout stays clean for the protocol.
- MCP code in every project uses `github.com/mark3labs/mcp-go`, so the four share one MCP library.
- `/healthz` reports that the process is up. `/readyz` reports that its dependencies are reachable.
- `/metrics` serves Prometheus metrics. Traces are exported over OTLP when configured.
- Logs are JSON on stdout.

**Everything is free**
- There is no paid edition and no license key. SSO, roles and permissions, audit logs, multi-tenancy, clustering, the admin API and the future console all ship in the Apache-2.0 build.
- No feature is held back for a hosted or enterprise version. This is a selling point, and each README says so.

**Storage**
- State that must survive a restart lives in SQL: embedded SQLite by default, so one binary still runs with no setup, and Postgres for teams and for several replicas.
- Use pure-Go drivers (`modernc.org/sqlite`, `pgx`), so builds don't need cgo.
- The schema is versioned with migrations that run at startup.

**Admin API for a future console**
- A separate web console is planned later. No project ships its own UI.
- The console has to look good and be easy to use. A new user goes from opening it to a working setup without touching a YAML file, and every common task, like adding a server, giving someone access or reading the audit log, takes a few clicks. It has light and dark themes and works on a phone.
- So the admin API must cover every common task, not only reads.
- Each project serves a JSON admin API under `/admin/v1/`, described by an OpenAPI file in the repository, with the same identity rules as the rest of the API.
- The admin API covers what a console would show or change: configuration, status, usage and audit records, and the project's own data, such as airlock's servers and permissions or logbook's memories.
- Lists take `cursor` and `limit` (default 50, at most 500) and return `{"items": [...], "next_cursor": "..."}`. `next_cursor` is absent on the last page.
- Errors are `{"error": {"code": "...", "message": "...", "field": "..."}}` with a matching status. A validation error names the field, so the console can show the message next to it.
- Every resource has an integer `version`. `PATCH` and `DELETE` take it in `If-Match`, and a mismatch is 409 `version_conflict`.
- Roles are `admin`, `member` and `viewer` everywhere. A project may add its own, such as logbook's `verifier`.
- Every project serves `GET /admin/v1/status` and `GET /admin/v1/audit`, and every write adds an audit record.
- Items defined in the YAML file are read-only through the API and marked `managed_by_file`.
- Cost is shown from transponder's records. Other projects count their own operations but don't track money.

**Scale and limits**

The details and examples are in [docs/scale.md](docs/scale.md). These are the rules:

- One binary per project, never split into services. `roles` (`gateway`, `admin`, `worker`, default all) picks what a process runs, and any combination works. Roles meet only through the database and the event bus.
- Every dependency added for scale (Valkey, NATS, S3) is optional, uses a pure-Go client, has a fallback that needs nothing installed, and shows in `/readyz`.
- Serving a request reads no SQL. Keys, JWKS, config and permissions are cached, limits come from the counter store, and SQL writes go through a batch writer or an outbox.
- Config changes reach every replica within 2 seconds. From size 2 up, no state lives only in one process.
- Limits come from named profiles in one shared format. Limit keys are prefixed by the project that owns them (`llm.`, `tools.`, `agent.`, `memory.`), with `requests` shared. The most specific binding wins: key, user, team, the `auth.profile_claim` claim, then `default_profile`. A request must pass both the caller's profile and its organisation's.
- A limit answers 429 `rate_limited` or `concurrency_limited`, 402 `quota_exceeded`, or 503 `overloaded`, with `Retry-After` when a retry can work, and every response sends `RateLimit-Policy` and `RateLimit`.
- Tokens and money are reserved before a call and settled after it. Each project documents how far a limit can be overshot.
- Usage and audit events are CloudEvents 1.0 of type `dev.mothership.<project>.<event>.v1`, delivered at least once with a unique `id`, and never carry prompts, completions, tool arguments or memory text unless an opt-in setting says so.
- Every metric, event and log line carries `cell_id` when one is set. Shared metrics are `<project>_requests_total`, `_request_duration_seconds`, `_inflight_requests`, `_limited_total` and `_events_dropped_total`.
- Migrations follow expand and contract, so two versions run against one schema during a rolling update.
- No README states a performance number that the load test has not measured.

**Repositories**
- `github.com/raven-clown/<name>`, Apache-2.0, Go 1.26 or newer.
- An `mothership` repository holds this contract, a Docker Compose file that runs all four together, and an end-to-end test of the path in the diagram.

## Contributing

- Every repository copies the contents of `contributing/` to its root: CONTRIBUTING.md, AGENTS.md (with CLAUDE.md, GEMINI.md and `.github/copilot-instructions.md` pointing to it), SECURITY.md, CODE_OF_CONDUCT.md, CHANGELOG.md, LICENSE, the issue and pull request templates, CODEOWNERS, the CI workflows, Dependabot, `.golangci.yml`, `.testcoverage.yml` and the Makefile. A project can add to them, not loosen them.
- `scripts/setup-repo.sh owner/name` applies the merge rules in `ruleset.json`, the labels and the security settings to a new repository.
- `main` only changes through a squash-merged pull request with a code owner's approval and every check green: tests on Linux, macOS and Windows, coverage of 80% or more, lint, CodeQL, dependency review and a Conventional Commits title.
- Every change is committed and pushed right away, and AGENTS.md is updated in the same commit whenever commands, layout or rules change.
- Once a project's spec is approved, its first milestone is written as issues in the format of `contributing/labels.md`, with a few `good first issue` items, drafted in `docs/issues.md` for review before they are posted.

## Build order

1. `airlock`, `transponder` and `logbook` are built in parallel against this contract.
2. `rosetta` starts at the same time on its protocol layer and adds the `airlock` and `transponder` clients once their APIs are in place.
3. The `mothership` end-to-end test lands once the first three answer on their interfaces.
