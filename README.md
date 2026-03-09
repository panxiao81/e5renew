# e5renew

The `e5renew` service is now shipping first-class Postgres support alongside the existing MySQL defaults. The Postgres workflow is documented below so you can run the application, migrations, and session store with a Postgres backend.

## Configuration

- Set `database.engine` to `mysql` (default) or `postgres`. The Go config loader will normalize common variations (`postgresql`, `pg`, etc.).
- `database.port` can be used to document or override the client-side port (e.g. `15432` when you run Postgres in Docker Compose). The DSN still needs to include the correct port/host.
- Use the DSN format `postgres://user:password@host:port/database?sslmode=disable` when `database.engine` is `postgres`.
- The Helm chart now exposes `config.database.engine` and `config.database.port`, and the config map injects them via `E5RENEW_DATABASE_ENGINE` / `E5RENEW_DATABASE_PORT`.

## SQLC and Migrations

- `sqlc` now includes a `postgresql` engine block that references `sql/schema-postgres.sql`. Run `sqlc generate` after editing queries and schema to regenerate Postgres-friendly structs.
- The migration commands automatically read from `migrations/mysql` or `migrations/postgres` depending on `database.engine`. Run `go run ./cmd migrate up` with `DATABASE_ENGINE=postgres` to pop the Postgres migrations.

## Docker Compose (Postgres)

```bash
docker compose -f compose.yaml up -d postgres
```

The Postgres service listens on `${POSTGRES_PORT:-15432}` so that tests and local tooling do not conflict with a system Postgres instance. Set `E5RENEW_DATABASE_DSN=postgres://e5renew:e5renew@localhost:${POSTGRES_PORT:-15432}/e5renew?sslmode=disable` and `E5RENEW_DATABASE_ENGINE=postgres` before running the server.

## Helm notes

- The chart now depends on the Bitnami `postgresql` sub-chart guarded by `postgres.enabled`. Set the following to bring up an embedded Postgres:

```bash
helm install e5renew ./helm/e5renew \
  --set postgres.enabled=true \
  --set postgresql.enabled=true \
  --set config.database.engine=postgres \
  --set config.database.dsn="postgres://e5renew:e5renew@{{ include \"e5renew-postgresql.fullname\" . }}:5432/e5renew?sslmode=disable"
```

You can customize the credentials under `postgresql.auth`. The generated Secret still stores `database-dsn`, so you only need to switch the host/port and engine values so the application points at the Postgres service.

## Testing

- The Postgres-focused tests expect a Postgres container from `docker run -p 15432:5432 postgres:15` and honor the `DATABASE_ENGINE=postgres` flag.
- `golang-migrate` now understands both engines; run the CLI with `DATABASE_ENGINE=postgres go run ./cmd migrate up` to verify the Postgres migrations.
- Session store integration tests hit the Postgres service on port 15432 and ensure the SCS Postgres store works alongside the MySQL store.
