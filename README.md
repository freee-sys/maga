# netcluster

SNMP discovery + rule-based virtual cluster assignment for network
devices (Phase 1). Devices are found via SNMP, matched against
priority-ordered regex rules on their hostname, and grouped into
dynamically created "clusters" — for monitoring/topology grouping today,
with room left for config-management use later.

LLDP/CDP-based topology discovery is Phase 2 and not part of this build.

## Prerequisites

- Go 1.25+
- Node.js 20+ (for the web UI)
- Docker (for Postgres locally, and for integration tests via
  [testcontainers-go](https://golang.testcontainers.org/))

## Running locally

The web UI is embedded into the Go binary via `go:embed`, so it must be
built once before `go build`/`go run` (the embed directive requires
`web/dist` to exist):

```bash
docker compose up -d postgres
cd web && npm install && npm run build && cd ..
go run ./cmd/apiserver
```

The server runs its own migrations on startup, then listens on `:8080`
(configurable, see below) serving both the API and the UI from the same
origin. Health check: `curl localhost:8080/healthz`.

- Web UI: http://localhost:8080/
- API docs (Swagger UI): http://localhost:8080/swagger/index.html

### Frontend development

For UI iteration with hot reload, run the Vite dev server instead of
rebuilding on every change — it proxies `/api` to the Go server on
`:8080` (see `web/vite.config.ts`), so run both at once:

```bash
docker compose up -d postgres
go run ./cmd/apiserver &     # API on :8080
cd web && npm run dev        # UI on :5173, proxying /api to :8080
```

`web/src/api/schema.d.ts` is generated from the live Swagger spec — after
changing any API handler or DTO, regenerate the Go docs first
(see below) then run `npm run gen-api` inside `web/`.

## Configuration

All settings are environment variables, namespaced `NETCLUSTER_*` to
avoid colliding with anything else already set in your shell:

| Variable | Default | Purpose |
|---|---|---|
| `NETCLUSTER_LISTEN_ADDR` | `:8080` | HTTP listen address |
| `NETCLUSTER_DATABASE_URL` | `postgres://postgres:postgres@localhost:55432/netcluster?sslmode=disable` | Postgres connection string (port 55432 matches `docker-compose.yml`, chosen to avoid colliding with a locally installed Postgres on the default 5432) |
| `NETCLUSTER_DEFAULT_SNMP_COMMUNITY` | `public` | Not currently used by the API (community is required per discovery request); reserved for a future UI default |
| `NETCLUSTER_DISCOVERY_WORKER_POOL_SIZE` | `20` | Max concurrent SNMP requests during a discovery scan |

## Testing

```bash
go test ./...                    # unit tests only, no external deps
go test -tags=integration ./...  # + Postgres-backed tests via testcontainers (needs Docker)
```

The DSL parser also has a fuzz test:

```bash
go test ./internal/dsl/... -fuzz=FuzzParse -fuzztime=30s
```

## Regenerating Swagger docs

After changing any `// @...` handler annotation or a `dto` type:

```bash
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/apiserver/main.go -o docs
```

## Project layout

```
cmd/apiserver/        main() — wiring only
internal/
  domain/              shared structs, no external deps
  dsl/                 the capture-group transform pipeline (parser + interpreter)
  ruleengine/          pure hostname -> cluster-key evaluation
  clusters/            cluster GetOrCreate/rename business logic
  assignment/          orchestrates ruleengine + clusters + element/history writes
  snmp/                SNMP client interface + gosnmp implementation
    snmptest/          fake in-process UDP SNMP agent, for wire-protocol tests without Docker
  discovery/            target expansion + bounded-concurrency scan orchestrator
  storage/postgres/    concrete repo implementations (pgx)
  api/http/            chi router, handlers, dto/ request-response shapes
  config/              env var loading
migrations/            golang-migrate SQL, embedded into the binary
docs/                  generated Swagger/OpenAPI (do not hand-edit)
web/                   React + Vite + TypeScript SPA
  src/api/schema.d.ts  generated types from the Swagger spec (do not hand-edit)
  src/api/dsl.ts        hand-written DSL op registry — the generated schema for
                        capture_transforms is wrong (swaggo can't see the Go
                        side's custom JSON marshaling), so this file is the
                        real source of truth for the UI; keep it in sync with
                        internal/dsl/ops.go by hand
  embed.go             go:embed of web/dist into the Go binary
```

## Adding a new DSL operation

Register it in `internal/dsl/ops.go`'s `registry` map (name, arity,
`ArgNames` for JSON serialization, and the `OpFunc` implementation), then
add table-driven cases to `internal/dsl/ops_test.go`. No changes are
needed elsewhere — the API layer, storage, and rule engine all go through
`dsl.Pipeline`/`dsl.FromJSON` generically.
