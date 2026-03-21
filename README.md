# e5renew

`e5renew` is a Go web application that helps keep Microsoft 365 E5 subscriptions active by automating Microsoft Graph API activity. It supports Azure AD login, scheduled renewal jobs, personal mail authorization, OpenTelemetry, and both MySQL and PostgreSQL backends.

Runtime persistence now goes through dependency-injected repositories in `internal/repository`, while `internal/db` primarily handles connection/bootstrap concerns plus sqlc-generated query packages.

## Highlights

- Azure AD OAuth2 login plus optional personal mail authorization
- Scheduled Graph API activity for E5 renewal flows
- MySQL and PostgreSQL support for app data, migrations, and session storage
- OpenTelemetry tracing and metrics
- HTML UI with English and Simplified Chinese localization
- Docker image publishing to `ghcr.io/panxiao81/e5renew`

## Development

### Build and run

- `make dev` - run with `config.dev.yaml`
- `make build` - build `bin/e5renew`
- `go run main.go --config config.dev.yaml` - alternate local run command

### Testing

- `make test` - run all tests
- `make test-coverage` - generate `coverage.out` and `coverage.html`
- `make test-race` - run race-enabled tests
- `make bench` - run benchmarks
- `npm run test:frontend` - run Playwright smoke, mobile, localization, and accessibility tests

The repo currently sits around 87% unit-test coverage.

Some PostgreSQL-focused tests are opt-in and require `E5RENEW_TEST_POSTGRES_DSN`.

## Configuration

- Set `database.engine` to `mysql` (default) or `postgres`
- Use `E5RENEW_`-prefixed environment variables for config overrides
- Production config template: `config.prod.yaml.template`
- Default config file path: `$HOME/.e5renew.yaml`
- Postgres DSN format: `postgres://user:password@host:port/database?sslmode=disable`

## Database and migrations

- `make migrate-up`
- `make migrate-down`
- `make migrate-status`
- `make migrate-version`
- `make migrate-force`

Migrations are selected automatically from `migrations/mysql` or `migrations/postgres` based on `database.engine`.

## Persistence architecture

- `internal/db/mysql/` and `internal/db/postgres/` contain sqlc-generated query code
- `internal/repository/` is the app-facing persistence boundary used by services
- `internal/db/` now mainly provides connection/bootstrap helpers plus DB-internal compatibility/query helpers

## Docker and GHCR

Build locally:

```bash
docker build -t ghcr.io/panxiao81/e5renew:latest .
```

The repository includes these GitHub Actions workflows:

- `.github/workflows/go-test.yml`
- `.github/workflows/docker-image.yml`
- `.github/workflows/frontend-e2e.yml`

Workflow behavior:

- Docker images are only built and pushed when a version tag matching `v*` is created
- published images are pushed to `ghcr.io/panxiao81/e5renew`

Frontend workflow behavior:

- pull requests to `master` run Playwright smoke tests
- pushes to `master` run Playwright smoke tests
- manual runs are supported with `workflow_dispatch`

Go test workflow behavior:

- every push runs `make test`
- every pull request runs `make test`
- manual runs are supported with `workflow_dispatch`

Published image tags include:

- `latest` on the default branch
- version tag refs
- short commit SHA tags

## Postgres local workflow

Start Postgres with Docker Compose:

```bash
docker compose -f compose.yaml up -d postgres
```

Example local environment:

```bash
export E5RENEW_DATABASE_ENGINE=postgres
export E5RENEW_DATABASE_DSN=postgres://e5renew:e5renew@localhost:${POSTGRES_PORT:-15432}/e5renew?sslmode=disable
```

## Helm notes

The Helm chart supports embedded Postgres via the Bitnami `postgresql` sub-chart.

Example:

```bash
helm install e5renew ./helm/e5renew \
  --set postgres.enabled=true \
  --set postgresql.enabled=true \
  --set config.database.engine=postgres \
  --set config.database.dsn="postgres://e5renew:e5renew@{{ include \"e5renew-postgresql.fullname\" . }}:5432/e5renew?sslmode=disable"
```

## Notes

- SQL code generation is configured through `sqlc.yaml`
- i18n locale files live in `internal/i18n/locales/`
- OpenTelemetry config is controlled with `E5RENEW_OTEL_*` variables
- shared user-aware page rendering is handled through request-context middleware plus template rendering defaults
