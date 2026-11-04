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
- An MCP server over Streamable HTTP and stdio. Tools: `remember`, `logbook`, `forget`, `list_memories`.
- The same operations over a small REST API under `/v1/`.
- Memories belong to a scope: `user`, `project` or `team`, taken from the caller's identity.

**transponder**
- OpenAI-compatible `/v1/chat/completions` and `/v1/embeddings`.
- Anthropic-compatible `/v1/messages`, so Claude clients work unchanged.
- Usage and cost per call are reported in the response headers listed below.

**rosetta**
- Agents are written once against one Go interface, or declared in config.
- Each agent is served as an MCP server, an A2A agent, an ACP agent over stdio, and an AG-UI endpoint over SSE.
- It is a client of `airlock` and `transponder` when those are configured.

## Shared conventions

**Identity**
- Callers send `Authorization: Bearer <token>`.
- Tokens are OIDC JWTs checked against a configured issuer, or static API keys for service-to-service calls. Any OIDC provider works, including IdpForge.
- A component that calls another forwards the caller's token unchanged, so every component sees the same user.

**Headers**
- `traceparent` and `tracestate` (W3C Trace Context) are forwarded on every hop.
- `X-Request-ID` is kept if present, and generated if not.
- `transponder` adds `X-Usage-Input-Tokens`, `X-Usage-Output-Tokens` and `X-Usage-Cost-USD`.

**Operations**
- One static binary per project, configured by a YAML file passed with `--config`. An environment variable can override any key: `AIRLOCK_LISTEN_ADDR` overrides `listen_addr`.
- `/healthz` reports that the process is up. `/readyz` reports that its dependencies are reachable.
- `/metrics` serves Prometheus metrics. Traces are exported over OTLP when configured.
- Logs are JSON on stdout.

**Repositories**
- `github.com/raven-clown/<name>`, Apache-2.0, Go 1.26 or newer.
- An `mothership` repository holds this contract, a Docker Compose file that runs all four together, and an end-to-end test of the path in the diagram.

## Build order

1. `airlock`, `transponder` and `logbook` are built in parallel against this contract.
2. `rosetta` starts at the same time on its protocol layer and adds the `airlock` and `transponder` clients once their APIs are in place.
3. The `mothership` end-to-end test lands once the first three answer on their interfaces.
