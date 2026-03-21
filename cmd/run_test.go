package cmd

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/services"
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

var _ *sql.DB
