# Yongbo Workflow Backend

Go backend for the Yongbo workflow system. The service exposes V1 workflow,
task, asset, ERP, notification, search, report, identity, and admin APIs.

## Current Authority

Use these files as the current source of truth:

1. `transport/http.go` decides which routes are mounted at runtime.
2. `docs/api/openapi.yaml` defines request and response contracts.
3. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` indexes the current V1 authority files.

Generated frontend API notes live under `docs/frontend/` and are downstream of
OpenAPI. Historical material under `docs/archive/`, `docs/iterations/`, and
`prompts/` is evidence only unless restated in the V1 authority files.

## Tech Stack

- Go `1.24`
- Gin HTTP router
- MySQL
- Redis
- OpenAPI 3 validation with `kin-openapi`
- Zap structured logging

## Repository Layout

```text
cmd/server/            Canonical production MAIN entrypoint
cmd/api/               Deprecated compatibility entrypoint
cmd/tools/             Maintenance and validation tools
config/                Runtime configuration loaders and JSON config files
db/                    Database migration files
domain/                Domain models, errors, and shared contract types
repo/mysql/            MySQL repository layer
service/               Business services and workflow orchestration
transport/             HTTP router, handlers, middleware, and WebSocket code
workers/               Background workers
docs/api/openapi.yaml  API contract
docs/frontend/         Generated frontend-facing API docs
tools/contract_audit/  Route/OpenAPI drift audit tool
scripts/agent-check.sh Consolidated verification gate
deploy/                Deployment environment templates
```

## Configuration

The server reads configuration from environment variables. Start from the
templates in `deploy/` and keep real secrets out of git:

- `deploy/main.env.example`
- `deploy/bridge.env.example`
- `deploy/deploy.env.example`

Core variables include:

- `SERVER_PORT`
- `MYSQL_DSN`
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `REDIS_DB`
- `AUTH_SETTINGS_FILE`
- `FRONTEND_ACCESS_SETTINGS_FILE`
- `ERP_BRIDGE_BASE_URL`
- `UPLOAD_SERVICE_*`

## Run Locally

Install Go `1.24`, MySQL, and Redis. Then export the required environment
variables and run the canonical server entrypoint:

```bash
go run ./cmd/server
```

The compatibility entrypoint remains available but should not be used for new
production work:

```bash
go run ./cmd/api
```

## Validation

Run the full repository gate before claiming a contract, handler, service, or
domain change is complete:

```bash
./scripts/agent-check.sh
```

The gate runs:

1. `go vet ./...`
2. `go build ./...`
3. `go test ./... -count=1`
4. `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml`
5. `go run ./tools/contract_audit ... --fail-on-drift true`

For a quick contract-only validation:

```bash
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml
go run ./tools/contract_audit \
  --transport transport/http.go \
  --handlers transport/handler \
  --domain domain \
  --openapi docs/api/openapi.yaml \
  --output tmp/agent_check_audit.json \
  --fail-on-drift true
```

## Frontend API Docs

If `docs/api/openapi.yaml` changes, regenerate frontend docs in the same
logical change:

```bash
python scripts/docs/generate_frontend_docs.py
```

## Development Rules

- Keep route changes and OpenAPI contract changes in the same logical change.
- Do not edit `db/migrations/**` without an explicit migration decision.
- Do not commit real credentials, production tokens, or local `.env` files.
- Prefer one logical change per commit.
- Use `AGENTS.md` for the full agent and governance workflow.

