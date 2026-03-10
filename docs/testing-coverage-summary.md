# Testing Coverage Updates (2026-03-10)

## Goals
- Boost automated test coverage for critical areas (`cmd/root`, `internal/services`, `internal/middleware`, `internal/view`).
- Make coverage run repeatable via Dockerized Go + Postgres and document the commands.

## Changes
1. Added new/expanded unit tests:
   - `cmd/root_init_execute_test.go`: exercises `requiredConfigForCommand`, `initConfig` (defaults, config file, env prefix), and `Execute` (success and `os.Exit(1)` branches via subprocesses).
   - `internal/services/mail_test.go`: covers Graph helper conversions, empty inputs, and async `logGraphAPICall` branch with `sqlmock`.
   - `internal/services/apilog_test.go`: tests `LogAPICall`, stats conversions, and `DeleteOldAPILogs` error handling.
   - `internal/middleware/apilog_test.go`: ensures Graph vs non-Graph flows, `extractEndpoint`, response size handling, and client creation.
   - `internal/view/template_test.go`: checks `Render`/`RenderWithContext`, including i18n localizer integration and missing template errors.

2. Coverage workflow: run within Docker (+ Postgres container) to match CI expectations, capture `coverage.out`, and summarize per-package coverage.

3. Documented persistent blind spots (controller, db engines, environment, services, telemetry, view, etc.) and blockers such as Azure tenant skips and Graph SDK access.

## Commands (repeatable in CI)
```bash
docker run --rm -d --name e5renew-test-postgres \
  -e POSTGRES_PASSWORD=secret -e POSTGRES_USER=e5renew -e POSTGRES_DB=e5renew_test \
  -p 15432:5432 postgres:15

docker run --rm --network host \
  -v $PWD:/src -w /src \
  -e E5RENEW_TEST_POSTGRES_DSN=postgres://e5renew:secret@127.0.0.1:15432/e5renew_test?sslmode=disable \
  golang:1.24.5 bash -c 'make test-coverage'

# optional: inspect coverage
docker run --rm --network host \
  -v $PWD:/src -w /src \
  -e E5RENEW_TEST_POSTGRES_DSN=postgres://e5renew:secret@127.0.0.1:15432/e5renew_test?sslmode=disable \
  golang:1.24.5 bash -c 'go tool cover -func=coverage.out'

docker stop e5renew-test-postgres
```
