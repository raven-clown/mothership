# Scale, limits and usage events

This document says how the four projects grow from one binary on a laptop to a large deployment across regions, without a different product at each size. `CONTRACT.md` lists the rules that every project must follow. This file explains them and gives the details.

Status: approved design, not built yet. Each project adds these pieces in its own milestones, and its README only claims what it has shipped and measured.

## Principles

1. **One binary, many roles.** A project is never split into microservices. The same binary runs every part, and a flag picks which parts a process runs. A small setup runs everything in one process. A large one runs each role as its own deployment and scales them apart.
2. **Small by default, big by configuration.** Every dependency added for scale is optional. With no config, a project still starts on SQLite with nothing else installed.
3. **The request path does not read SQL.** Auth, config, permissions and limits are served from memory or from a fast counter store. SQL holds the source of truth and is written in batches.
4. **Each kind of data lives where it fits.** Config in Postgres, live counters in Valkey, usage and audit as events in a log store. One database does not do all three jobs.
5. **Fail small.** A slow tenant, a dead provider or a lost cache slows one part, never the whole system.

## Sizes

| Size | For | What runs |
|---|---|---|
| 0 | one person | the binary and SQLite |
| 1 | a team | Postgres, two or three replicas behind a load balancer |
| 2 | a company | adds Valkey for counters, NATS for events and change notices, roles split into their own deployments, Postgres read replicas |
| 3 | a large service, several regions | adds cells, an event store such as ClickHouse, and a global router |

A project moves up a size by adding config, never by a rewrite. Every option in this document has a default that keeps size 0 working.

## 1. Roles

```yaml
roles: [gateway, admin, worker]   # default: all
```

`--roles=gateway` or `AIRLOCK_ROLES=gateway` work the same way.

| Role | Serves |
|---|---|
| `gateway` | the API that clients call: MCP, `/v1/...` |
| `admin` | `/admin/v1/` |
| `worker` | background jobs: sending events, retention, embeddings, exports |

- Any combination of roles works, including all three in one process.
- Roles never call each other. They meet only through the database and the event bus.
- A project may add roles of its own, such as a `runner` in rosetta.
- Every role serves `/healthz`, `/readyz` and `/metrics`. `ops_listen_addr` can move them to their own port.

## 2. Profiles

A profile is a named set of limits. The four projects read the same format, so one user gets the same plan everywhere.

```yaml
profiles:
  - name: free
    priority: 1
    limits:
      requests:        {rate: 60/m, concurrency: 4}
      llm.tokens:      {per_day: 100000, per_month: 2000000}
      llm.cost_usd:    {per_month: 5}
      llm.max_output_tokens: 2048
      tools.calls:     {rate: 30/m}
      agent.runs:      {rate: 10/m, concurrency: 1, max_steps: 20, timeout: 5m}
      memory.items:    1000
      memory.bytes:    10MB
      memory.writes:   {rate: 20/m}
  - name: pro
    priority: 5
    limits:
      requests:        {rate: 600/m, concurrency: 16}
      llm.tokens:      {per_day: 2000000}
```

**Keys.** `requests` applies in every project. The rest are owned by one project each:

| Prefix | Project |
|---|---|
| `llm.` | transponder |
| `tools.` | airlock |
| `agent.` | rosetta |
| `memory.` | logbook |

A project ignores keys owned by another project, so the same file works everywhere. An unknown key under its own prefix is a config error at startup.

**Values.** Rates are `N/s`, `N/m`, `N/h` or `N/d`, parsed by one shared function. Sizes are `KB`, `MB` or `GB`. Durations use Go syntax, such as `5m`.

**Who gets which profile.** The most specific match wins, and its profile applies whole:

1. a binding on the API key
2. a binding on the user
3. a binding on a team the user is in
4. the OIDC claim named by `auth.profile_claim`, `tier` by default
5. `default_profile`

A binding may carry `overrides`, such as a `pro` user with a larger token allowance. Profiles are never merged.

**Two levels.** A request passes only when both the caller's profile and the organisation's `org_profile` allow it.

**One file for all four.** `profiles_file: /etc/mothership/profiles.yaml` points every project at the same file. A console can instead write profiles to each project's admin API.

**Admin API.** `/admin/v1/profiles`, `/admin/v1/profile-bindings`, and `GET /admin/v1/limits/{subject}`, which shows what a subject has left.

## 3. When a limit is hit

Every project answers the same way:

| Status | `error.code` | Retry |
|---|---|---|
| 429 | `rate_limited` | yes, after `Retry-After` |
| 429 | `concurrency_limited` | yes, when a running request ends |
| 402 | `quota_exceeded` | no, the daily or monthly allowance is gone |
| 503 | `overloaded` | yes, after `Retry-After` |

Every response carries `RateLimit-Policy` and `RateLimit` headers, from the IETF `draft-ietf-httpapi-ratelimit-headers`, so a client can slow down before it is refused.

## 4. Counting

```yaml
limits:
  store: memory        # memory, sql or valkey
  on_store_error: local
```

| Store | Size | How it counts |
|---|---|---|
| `memory` | 0 | in the process |
| `sql` | 1 | in memory, flushed to Postgres in batches every second |
| `valkey` | 2 and up | in Valkey, shared by every replica; Redis works too |

**Reserve, then settle.** Tokens and money are reserved before the call, using `max_tokens` for the output, and settled to the real numbers when the call ends. A call that fails gives the reservation back.

**Rates** use GCRA, a token bucket that needs one key per subject.

**When the store is down**, `on_store_error: local` counts in each replica with the limit divided by the number of replicas, so the service keeps working with a stricter limit. `deny` refuses instead.

**Overshoot is allowed and documented.** A limit can be passed by at most rate × flush interval × replicas. Each project states its number in its docs, and the tests check it.

## 5. Usage events

Every operation that a bill or a report might count produces an event in CloudEvents 1.0 JSON.

```json
{
  "specversion": "1.0",
  "id": "01JC8Z6Q2V4B7N3K5M9P0R1S2T",
  "source": "transponder/cell-a/pod-3",
  "type": "dev.mothership.transponder.llm.completed.v1",
  "time": "2026-11-05T10:00:00Z",
  "data": {
    "org_id": "acme",
    "subject": "user:42",
    "key_id": "k_9",
    "profile": "pro",
    "project": "web",
    "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
    "request_id": "req_7Hk2",
    "units": {"input_tokens": 812, "output_tokens": 240, "reasoning_tokens": 96, "cost_nusd": 4100000}
  }
}
```

| Type | `units` |
|---|---|
| `transponder.llm.completed` | `input_tokens`, `output_tokens`, `reasoning_tokens`, `cost_nusd` |
| `airlock.tool.called` | `calls`, `duration_ms` |
| `rosetta.run.finished` | `runs`, `steps`, `duration_ms` |
| `logbook.memory.written` | `items`, `bytes` |
| `<project>.audit` | the audit record |

- Events never carry content: no prompts, completions, tool arguments or memory text. A project may add content only behind its own opt-in setting.
- Delivery is at least once. `id` is unique, so a consumer can drop duplicates.
- On Postgres, events are written to an outbox table in the same transaction as the change, and a `worker` sends them. An event is not lost when a pod dies.
- Sinks, any number at once: `sql`, `stdout`, `webhook`, `nats`.
- OpenMeter reads CloudEvents directly, and Lago reads them through a webhook, so a deployment can bill from these events without changing any project.

## 6. The request path

- **No SQL reads while serving a request.** API key hashes are cached, JWTs are checked locally against a cached JWKS, config and permissions come from an in-memory snapshot, and limits come from the counter store.
- **SQL writes from a request go through a batch writer or the outbox.**
- **Config changes reach every replica within 2 seconds.** Size 1 polls a revision number. Size 2 and up uses Postgres `LISTEN/NOTIFY` or NATS.
- **No state lives only in one process from size 2 up.** Sessions are kept in a shared store, or the load balancer routes them by a header, such as airlock's `Mcp-Session-Id`.
- **Overload:**
  - `max_inflight` per replica; past it, 503 `overloaded`.
  - A cap on in-flight requests per organisation, so a heavy tenant cannot starve the others.
  - transponder and rosetta may queue instead of refusing: `queue.max_wait`, ordered by weighted fair queuing on the profile's `priority`.
- **Timeouts at every hop.** Retries only for idempotent calls, with jitter and a retry budget.
- **Graceful shutdown.** `/readyz` fails first, then open streams get `shutdown_grace`, 30 seconds by default, to finish.

## 7. Tenancy, cells and operations

**Organisations.** `org_id` comes from the claim named by `auth.org_claim`, or from the API key's binding.

**Cells.**
- A cell is a full copy of the stack with its own databases.
- `cell_id` names the cell. Every metric, event and log line carries it.
- With `cell.orgs` set, a request for an organisation outside the list gets 421 `wrong_cell`.
- `/admin/v1/orgs/{id}/export` and `import` move an organisation between cells.
- A global router sends each organisation to its cell. It lives in mothership and comes last.

**Optional dependencies** are Valkey, NATS JetStream and S3. Every client is pure Go, each one has a fallback that needs nothing installed, and `/readyz` reports each one.

**Shared metric names** in every project:

| Metric | Labels |
|---|---|
| `<project>_requests_total` | `route`, `code` |
| `<project>_request_duration_seconds` | `route` |
| `<project>_inflight_requests` | |
| `<project>_limited_total` | `reason` |
| `<project>_events_dropped_total` | `sink` |

**Migrations** follow expand and contract, so version N and N+1 run against the same schema during a rolling update.

**In this repository:**
- `deploy/compose/`: the whole stack in one command.
- `deploy/helm/<project>`: one chart per project, each role its own Deployment, scaled on `<project>_inflight_requests`, with workers scaled on queue depth.
- `loadtest/`: k6 scripts that run every night, with the results published.

**Targets to measure**, with the echo provider on 2 vCPU:

| Project | Added latency, p99 | Throughput per replica |
|---|---|---|
| transponder | 10 ms or less | 2,000 req/s |
| airlock | 15 ms or less | 2,000 req/s |

No README states a number until the load test has measured it.

## Order of work

1. This contract change.
2. Size 1 everywhere: Postgres and replicas, already in each project's plan.
3. Size 2 in transponder and airlock first, since they carry the most traffic: roles, Valkey counters, NATS events.
4. Helm charts and the nightly load test.
5. Size 3: cells, the event store and the global router.
6. An optional project for public sign-up, credits and payments, built only on the admin APIs and events above.
