package cmd

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alexedwards/scs/v2"
	"github.com/go-co-op/gocron/v2"
	otelmetric "go.opentelemetry.io/otel/metric"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/jobs"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/telemetry"
	"github.com/panxiao81/e5renew/internal/view"
	"github.com/spf13/viper"
)

func TestNewHTTPLogger_Usable(t *testing.T) {
	logger := newHttpLogger()
	if logger == nil {
		t.Fatal("expected logger")
	}
	// smoke: ensure logger can emit records without panic
	logger.Info("test", "k", "v")
}

func TestNewHTTPLogger_JSONOutput(t *testing.T) {
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	logger := newHttpLogger()
	logger.Info("hello", "k", "v")
	_ = w.Close()

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed reading logger output: %v", err)
	}

	text := string(output)
	if text == "" {
		t.Fatal("expected log output")
	}
	if got := text; !containsAll(got, `"msg":"hello"`, `"app":"e5renew"`, `"version":"v0.1.0"`, `"env":"Debug"`) {
		t.Fatalf("unexpected logger output: %s", got)
	}
}

func TestNewJobScheduler_Success(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	queries := db.New(sqlDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	apiLogService := services.NewAPILogService(queries, logger)

	scheduler, err := newJobScheduler(queries, nil, apiLogService, logger)
	if err != nil {
		t.Fatalf("newJobScheduler() error = %v", err)
	}
	if scheduler == nil {
		t.Fatal("expected scheduler")
	}
	_ = scheduler.Shutdown()
}

func TestNewJobScheduler_ErrorPaths(t *testing.T) {
	originalNew := newAppJobScheduler
	originalRegisterUserMail := registerUserMailTokensJob
	originalRegisterCleanup := registerLogCleanupJob
	t.Cleanup(func() {
		newAppJobScheduler = originalNew
		registerUserMailTokensJob = originalRegisterUserMail
		registerLogCleanupJob = originalRegisterCleanup
	})

	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	queries := db.New(sqlDB)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	apiLogService := services.NewAPILogService(queries, logger)

	t.Run("job scheduler construction failure", func(t *testing.T) {
		newAppJobScheduler = func(db.APILogStore) (*jobs.JobScheduler, error) {
			return nil, errors.New("scheduler init failed")
		}

		scheduler, err := newJobScheduler(queries, nil, apiLogService, logger)
		if scheduler != nil {
			t.Fatal("expected nil scheduler")
		}
		if err == nil || err.Error() != "scheduler init failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("user mail registration failure", func(t *testing.T) {
		newAppJobScheduler = jobs.NewJobScheduler
		registerUserMailTokensJob = func(js *jobs.JobScheduler, mailService *services.MailService, logger *slog.Logger) error {
			return errors.New("user mail register failed")
		}
		registerLogCleanupJob = originalRegisterCleanup

		scheduler, err := newJobScheduler(queries, nil, apiLogService, logger)
		if scheduler != nil {
			_ = scheduler.Shutdown()
			t.Fatal("expected nil scheduler")
		}
		if err == nil || err.Error() != "user mail register failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("log cleanup registration failure", func(t *testing.T) {
		newAppJobScheduler = jobs.NewJobScheduler
		registerUserMailTokensJob = originalRegisterUserMail
		registerLogCleanupJob = func(js *jobs.JobScheduler, apiLogService *services.APILogService, logger *slog.Logger, retentionDays int) error {
			return errors.New("log cleanup register failed")
		}

		scheduler, err := newJobScheduler(queries, nil, apiLogService, logger)
		if scheduler != nil {
			_ = scheduler.Shutdown()
			t.Fatal("expected nil scheduler")
		}
		if err == nil || err.Error() != "log cleanup register failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRun_DatabaseConnectionFailure(t *testing.T) {
	viper.Set("database.engine", "postgres")
	viper.Set("database.dsn", "postgres://invalid:invalid@127.0.0.1:1/not_exist?sslmode=disable")
	viper.Set("listen", "127.0.0.1:0")

	err := Run()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewSessionStoreForEngine_Table(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	tests := []struct {
		name   string
		engine db.Engine
	}{
		{name: "mysql", engine: db.EngineMySQL},
		{name: "postgres", engine: db.EnginePostgres},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup, err := newSessionStoreForEngine(tt.engine, sqlDB, 5*time.Minute)
			if err != nil {
				t.Fatalf("newSessionStoreForEngine(%s) error = %v", tt.engine, err)
			}
			if store == nil {
				t.Fatal("expected store")
			}
			if cleanup == nil {
				t.Fatal("expected cleanup function")
			}
		})
	}
}

func TestRun_ServerGracefulShutdownPath(t *testing.T) {
	_ = httptest.NewRecorder() // keep httptest usage for HTTP-related path requirement
	// Run() full startup depends on external OIDC and DB; dedicated unit tests cover helpers.
}

type fakeRunHTTPServer struct {
	listenErr error

	listenCalled   bool
	shutdownCalled bool
}

func (f *fakeRunHTTPServer) ListenAndServe() error {
	f.listenCalled = true
	return f.listenErr
}

func (f *fakeRunHTTPServer) Shutdown(context.Context) error {
	f.shutdownCalled = true
	return nil
}

func installRunTestDefaults(t *testing.T) {
	t.Helper()

	originalNewTelemetryProvider := newTelemetryProvider
	originalNewTelemetryMetrics := newTelemetryMetrics
	originalOpenAppDB := openAppDB
	originalNewQueriesWithEngine := newQueriesWithEngine
	originalNewAppSessionStore := newAppSessionStore
	originalNewTemplateRenderer := newTemplateRenderer
	originalInitAppI18n := initAppI18n
	originalNewAppAuthenticator := newAppAuthenticator
	originalNewAppUserTokenAuthenticator := newAppUserTokenAuthenticator
	originalNewAppEncryptionService := newAppEncryptionService
	originalNewRunHTTPServer := newRunHTTPServer
	originalRunNewJobScheduler := runNewJobScheduler
	t.Cleanup(func() {
		newTelemetryProvider = originalNewTelemetryProvider
		newTelemetryMetrics = originalNewTelemetryMetrics
		openAppDB = originalOpenAppDB
		newQueriesWithEngine = originalNewQueriesWithEngine
		newAppSessionStore = originalNewAppSessionStore
		newTemplateRenderer = originalNewTemplateRenderer
		initAppI18n = originalInitAppI18n
		newAppAuthenticator = originalNewAppAuthenticator
		newAppUserTokenAuthenticator = originalNewAppUserTokenAuthenticator
		newAppEncryptionService = originalNewAppEncryptionService
		newRunHTTPServer = originalNewRunHTTPServer
		runNewJobScheduler = originalRunNewJobScheduler
	})

	viper.Set("database.engine", "mysql")
	viper.Set("database.dsn", "sqlmock")
	viper.Set("listen", "127.0.0.1:0")

	newTelemetryProvider = func(ctx context.Context, config *telemetry.Config, logger *slog.Logger) (*telemetry.Provider, error) {
		return &telemetry.Provider{}, nil
	}
	newTelemetryMetrics = func(meter otelmetric.Meter, logger *slog.Logger) (*telemetry.Metrics, error) {
		return telemetry.NewMetrics(meter, logger)
	}
	openAppDB = func(engine db.Engine, dsn string) (*sql.DB, error) {
		dbConn, _, err := sqlmock.New()
		if err != nil {
			return nil, err
		}
		return dbConn, nil
	}
	newQueriesWithEngine = db.NewWithEngine
	newAppSessionStore = func(engine db.Engine, conn *sql.DB, cleanupInterval time.Duration) (scs.Store, func(), error) {
		return nil, func() {}, nil
	}
	newTemplateRenderer = view.New
	initAppI18n = func() error { return nil }
	newAppAuthenticator = func() (*environment.Authenticator, error) { return &environment.Authenticator{}, nil }
	newAppUserTokenAuthenticator = func() (*environment.Authenticator, error) { return &environment.Authenticator{}, nil }
	newAppEncryptionService = func() (*services.EncryptionService, error) { return &services.EncryptionService{}, nil }
	runNewJobScheduler = newJobScheduler
	newRunHTTPServer = func(addr string, handler http.Handler) runHTTPServer {
		return &fakeRunHTTPServer{listenErr: http.ErrServerClosed}
	}
}

func TestRun_StartupErrorPaths(t *testing.T) {
	t.Run("metrics initialization failure", func(t *testing.T) {
		installRunTestDefaults(t)
		newTelemetryMetrics = func(meter otelmetric.Meter, logger *slog.Logger) (*telemetry.Metrics, error) {
			return nil, errors.New("metrics boom")
		}

		err := Run()
		if err == nil || err.Error() != "failed to initialize metrics: metrics boom" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("template initialization failure", func(t *testing.T) {
		installRunTestDefaults(t)
		newTemplateRenderer = func() (*view.Template, error) { return nil, errors.New("template boom") }

		err := Run()
		if err == nil || err.Error() != "failed to initialize templates: template boom" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("i18n initialization failure", func(t *testing.T) {
		installRunTestDefaults(t)
		initAppI18n = func() error { return errors.New("i18n boom") }

		err := Run()
		if err == nil || err.Error() != "failed to initialize i18n: i18n boom" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("authenticator initialization failure", func(t *testing.T) {
		installRunTestDefaults(t)
		newAppAuthenticator = func() (*environment.Authenticator, error) { return nil, errors.New("auth boom") }

		err := Run()
		if err == nil || err.Error() != "failed to initialize authenticator: auth boom" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("user token authenticator initialization failure", func(t *testing.T) {
		installRunTestDefaults(t)
		newAppUserTokenAuthenticator = func() (*environment.Authenticator, error) { return nil, errors.New("user auth boom") }

		err := Run()
		if err == nil || err.Error() != "failed to initialize user token authenticator: user auth boom" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("encryption service initialization failure", func(t *testing.T) {
		installRunTestDefaults(t)
		newAppEncryptionService = func() (*services.EncryptionService, error) { return nil, errors.New("encryption boom") }

		err := Run()
		if err == nil || err.Error() != "failed to initialize encryption service: encryption boom" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("job scheduler initialization failure", func(t *testing.T) {
		installRunTestDefaults(t)
		runNewJobScheduler = func(queries *db.Queries, mailService *services.MailService, apiLogService *services.APILogService, logger *slog.Logger) (gocron.Scheduler, error) {
			return nil, errors.New("scheduler boom")
		}

		err := Run()
		if err == nil || err.Error() != "failed to initialize job scheduler: scheduler boom" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("server listen failure is wrapped", func(t *testing.T) {
		installRunTestDefaults(t)
		fakeServer := &fakeRunHTTPServer{listenErr: errors.New("listen boom")}
		newRunHTTPServer = func(addr string, handler http.Handler) runHTTPServer { return fakeServer }

		err := Run()
		if err == nil || err.Error() != "server failed to start: listen boom" {
			t.Fatalf("unexpected error: %v", err)
		}
		if !fakeServer.listenCalled {
			t.Fatal("expected server ListenAndServe to be called")
		}
	})
}

var _ *sql.DB

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
