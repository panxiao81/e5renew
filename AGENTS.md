# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

E5renew is a Go web application that helps maintain Microsoft Office 365 E5 subscriptions by automatically calling Microsoft Graph API endpoints. The application uses Azure AD OAuth2 for authentication and schedules periodic API calls to keep subscriptions active.

## Development Commands

### Build and Run
- `make dev` - Run the application in development mode with config.dev.yaml
- `make build` - Build the application binary to `bin/e5renew`
- `go run main.go --config config.dev.yaml` - Alternative way to run in development mode

### Testing
- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report (generates coverage.html)
- `make test-race` - Run tests with race condition detection
- `make bench` - Run benchmarks
- Current repo-wide unit test coverage is around 87% (`coverage.out`, `coverage.html`)
- Some Postgres integration tests are opt-in and require `E5RENEW_TEST_POSTGRES_DSN`
- Frontend browser smoke/accessibility tests run with `npm run test:frontend`

### Code Quality
- `make fmt` - Format code using go fmt
- `make vet` - Run go vet for static analysis
- `make lint` - Run golangci-lint for comprehensive linting
- `make lint-fix` - Run golangci-lint with automatic fixes
- `make clean` - Clean build artifacts and coverage reports

### Database
- `make sqlc` - Generate Go code from SQL using sqlc
- SQL schema: `sql/schema.sql`
- SQL queries: `sql/query.sql`
- Generated code outputs to: `internal/db/`

### Database Migrations
- `make migrate-up` - Apply all pending migrations
- `make migrate-down` - Rollback the last migration
- `make migrate-status` - Show migration status
- `make migrate-version` - Show current migration version
- `make migrate-force` - Force migration to a specific version (use with caution)
- Migration files: `migrations/` directory
- Uses golang-migrate/migrate library

### Configuration
- Configuration files use YAML format
- Development config: `config.dev.yaml` (uses environment variables)
- Production config template: `config.prod.yaml.template`
- Environment variables: Use `E5RENEW_` prefix (e.g., `E5RENEW_AZUREAD_TENANT`)
- Default config location: `$HOME/.e5renew.yaml`
- `.env` file support for local development
- Configuration validation with required field checks

### OpenTelemetry Configuration
- Service name: `E5RENEW_OTEL_SERVICE_NAME` (default: "e5renew")
- Service version: `E5RENEW_OTEL_SERVICE_VERSION` (default: "v0.1.0")
- Environment: `E5RENEW_OTEL_ENVIRONMENT` (default: "development")
- OTLP endpoint: `E5RENEW_OTEL_OTLP_ENDPOINT` (for production)
- Enable tracing: `E5RENEW_OTEL_ENABLE_TRACING` (default: true)
- Enable metrics: `E5RENEW_OTEL_ENABLE_METRICS` (default: true)
- Enable stdout export: `E5RENEW_OTEL_ENABLE_STDOUT` (default: true for dev, false for prod)

## Architecture

### Core Components

1. **CLI Framework**: Uses Cobra for command-line interface (`cmd/`)
   - `cmd/root.go`: Root command and configuration initialization
   - `cmd/run.go`: Main application runner with HTTP server and job scheduler

2. **Web Server**: Chi router with session management
   - HTTP server runs on configurable port (default :8080)
   - Session management using SCS with MySQL store
   - JSON logging with OTEL schema

3. **Authentication**: Azure AD OAuth2 integration
   - Uses `github.com/coreos/go-oidc/v3` for OIDC (application login)
   - Client credentials flow for Graph API access
   - Standard OAuth2 authorization code flow for personal mail access
   - Personal mail auth requires delegated Microsoft Graph permissions for `Mail.Read` and `User.ReadBasic.All`
   - Session lifetime: 1 hour (matches Azure AD ID token)

4. **Database Layer**: SQL-first approach with sqlc
   - MySQL and PostgreSQL database support with connection pooling
   - Generated queries in `internal/db/mysql/` and `internal/db/postgres/`
   - Runtime persistence goes through dependency-injected repositories in `internal/repository/`
   - `internal/db/` now mainly handles connection/bootstrap and sqlc-facing compatibility helpers

5. **Job Scheduling**: Automated API calls
   - Uses `github.com/go-co-op/gocron/v2`
   - Random interval between 1-2 hours for client scope calls
   - Every 30 minutes for user mail token processing
   - Calls `jobs.GetUsersAndMessagesClientScope` function
   - Processes user mail tokens via `jobs.ProcessUserMailTokensJob`

6. **Controllers and Views**:
   - `internal/controller/`: HTTP request handlers
   - `internal/controller/usertoken.go`: User token OAuth2 flow management
   - `internal/view/`: HTML templates and static files
   - `internal/view/user.html`: User dashboard with token authorization UI
   - Template rendering with layout system and i18n support
   - Session user context is injected via middleware so shared layouts can render auth-aware nav automatically

7. **Observability**: OpenTelemetry integration
   - `internal/telemetry/`: OpenTelemetry setup and metrics
   - Distributed tracing for all HTTP requests and operations
   - Comprehensive metrics collection for HTTP, auth, database, and jobs
   - Configurable exporters (stdout for development, OTLP for production)

### Key Dependencies
- **Azure SDK**: `github.com/Azure/azure-sdk-for-go/sdk/azidentity`
- **Microsoft Graph**: `github.com/microsoftgraph/msgraph-sdk-go`
- **Web Framework**: `github.com/go-chi/chi/v5`
- **Configuration**: `github.com/spf13/viper`
- **Database**: `github.com/go-sql-driver/mysql`
- **Sessions**: `github.com/alexedwards/scs/v2`
- **Internationalization**: `github.com/nicksnyder/go-i18n/v2`
- **OpenTelemetry**: `go.opentelemetry.io/otel` (tracing, metrics, instrumentation)
- **OTEL HTTP**: `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`

### Application Flow
1. Application starts with OpenTelemetry initialization
2. HTTP server starts with distributed tracing middleware
3. Job scheduler starts with instrumented operations
4. Users authenticate via Azure AD OAuth2 (traced)
5. Users can authorize personal mail access with offline_access scope
6. Scheduled jobs make Graph API calls using client credentials (traced)
7. Additional scheduled jobs process user mail tokens (traced)
8. API calls target user messages to maintain subscription activity
9. All operations are traced and metrics are collected
10. Graceful shutdown with OpenTelemetry cleanup

### Important Files
- `internal/jobs/call.go`: Contains the core Graph API calling logic (with tracing)
- `internal/jobs/usermail.go`: User mail token processing job
- `internal/repository/apilog.go`: API log persistence boundary used by services
- `internal/repository/usertoken.go`: User token persistence boundary used by services
- `internal/repository/health.go`: Health/ping repository used by app startup and health checks
- `internal/services/usertoken.go`: User token database operations
- `internal/services/tokensource.go`: Token refresh with database persistence
- `internal/services/mail.go`: Microsoft Graph API mail operations
- `internal/controller/usertoken.go`: User token OAuth2 flow controller
- `internal/environment/auth.go`: Authentication setup (includes NewUserTokenAuthenticator)
- `internal/telemetry/`: OpenTelemetry configuration, metrics, and middleware
- `internal/i18n/`: Internationalization setup and translation management
- `internal/i18n/locales/`: Translation files (en.json, zh.json)
- `internal/middleware/i18n.go`: Language detection and context middleware
- `sqlc.yaml`: Database code generation configuration
- `config.dev.yaml`: Development configuration with Azure AD and OpenTelemetry settings

## User Token Management

### Personal Mail Access Feature
The application supports personal mail access authorization for enhanced E5 renewal activity:

#### User Flow
1. **Authorization**: Users can click "Authorize Personal Mail Access" on the user dashboard
2. **OAuth2 Flow**: Standard OAuth2 authorization code flow with `offline_access` and `Mail.Read` scopes
3. **Token Storage**: Stores OAuth2 tokens in database with automatic refresh capability
4. **Background Processing**: Scheduled job processes all user tokens every 30 minutes
5. **Revocation**: Users can revoke access at any time

#### Technical Implementation
- **Database**: `user_tokens` table stores encrypted tokens with expiry tracking
- **Token Refresh**: Automatic refresh with database persistence using `DatabaseUpdatingTokenSource`
- **API Calls**: Uses Microsoft Graph API `/me/messages` endpoint
- **Scopes**: `offline_access`, `https://graph.microsoft.com/Mail.Read`, `https://graph.microsoft.com/User.ReadBasic.All`

#### Routes
- `/user/authorize-token`: Initiates OAuth2 flow
- `/oauth2/user-token-callback`: Handles OAuth2 callback
- `/user/revoke-token`: Revokes user token

#### Services
- `UserTokenService`: Database operations for token management
- `MailService`: Microsoft Graph API mail operations
- `DatabaseUpdatingTokenSource`: Automatic token refresh with database updates

## Security Features
- Environment variable-based configuration (no hardcoded secrets)
- Debug mode toggle for sensitive token display
- Input validation and configuration validation
- Secure session management with configurable lifetime
- Proper error handling without exposing sensitive information
- OAuth2 state parameter validation for user token flow
- Automatic token refresh with secure database storage

## Testing
- Unit tests for controllers, authentication, and database layers
- Broad coverage for startup, jobs, services, middleware, templates, telemetry, and migration helpers
- Frontend Playwright coverage for smoke, mobile, localization, and accessibility checks
- Configuration validation tests
- Test setup with proper mocking for external dependencies
- Comprehensive test coverage for core functionality

## CI/CD
- GitHub Actions workflow at `.github/workflows/go-test.yml`
- GitHub Actions workflow at `.github/workflows/docker-image.yml`
- GitHub Actions workflow at `.github/workflows/frontend-e2e.yml`
- Pushes and pull requests run `make test` via the Go test workflow
- Version tags (`v*`) build and publish images to `ghcr.io/panxiao81/e5renew`
- Published Docker tags include `latest`, version-tag refs, and short SHA tags
- Frontend E2E workflow runs Playwright smoke tests on PRs, pushes to `master`, and manual runs

## Code Quality
- Configured golangci-lint for comprehensive code analysis
- Proper error handling with structured errors
- Graceful shutdown implementation
- Clean architecture with separated concerns

## OpenTelemetry Features
- **Distributed Tracing**: Full request tracing across all components
- **Metrics Collection**: HTTP, authentication, database, and job metrics
- **Structured Logging**: Trace correlation for debugging
- **Flexible Exporters**: stdout for development, OTLP for production
- **Comprehensive Instrumentation**: All major operations are traced

## Observability Data
- **HTTP Metrics**: Request count, duration, active requests, status codes
- **Authentication Metrics**: Login attempts, success/failure rates, session lifecycle
- **Database Metrics**: Connection pool, query performance, error rates
- **Job Metrics**: Execution count, duration, success rates, Graph API calls
- **Application Metrics**: Startup time, errors, configuration reloads

## Internationalization (i18n)

### Supported Languages
- **English (en)**: Default language
- **Chinese (zh)**: Simplified Chinese translation

### Language Detection
- **URL Parameter**: `?lang=en` or `?lang=zh`
- **Cookie**: Persistent language preference stored in `lang` cookie
- **Accept-Language Header**: Browser language preference fallback
- **Default**: Falls back to English if no preference detected

### Language Switching
- **UI Dropdown**: Available in the main navigation bar
- **Persistent**: Language preference saved in browser cookie (1 year)
- **Immediate**: Language change takes effect immediately without page reload

### Translation System
- **Library**: Uses `nicksnyder/go-i18n/v2` for translation management
- **Format**: JSON translation files in `internal/i18n/locales/`
- **Template Functions**: 
  - `{{t "message.id"}}` - Basic translation
  - `{{t "message.id" (dict "key" "value")}}` - Translation with parameters
  - `{{tDefault "message.id" "fallback"}}` - Translation with fallback text
- **Context-Aware**: Integrates with HTTP request context for proper language detection

### Implementation Details
- **Middleware**: `internal/middleware/i18n.go` handles language detection
- **Template Integration**: Enhanced template rendering with i18n functions
- **Embedded Files**: Translation files are embedded in the binary
- **Performance**: Efficient caching and context-based localization

## Notes
- Uses MySQL or PostgreSQL for application data and session storage
- Designed for Office 365 E5 subscription renewal automation
- Supports graceful shutdown with signal handling
- Production-ready configuration management
- Comprehensive testing and linting setup
- Full OpenTelemetry integration for production observability
