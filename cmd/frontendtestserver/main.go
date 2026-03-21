package main

import (
	"context"
	"database/sql"
	"encoding/gob"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/controller"
	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/i18n"
	appmiddleware "github.com/panxiao81/e5renew/internal/middleware"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/view"
)

type frontendAPILogStore struct{}

func (frontendAPILogStore) CreateAPILog(context.Context, db.CreateAPILogParams) (sql.Result, error) {
	return nil, nil
}

func (frontendAPILogStore) DeleteOldAPILogs(context.Context, time.Time) error { return nil }

func (frontendAPILogStore) GetAPILogStats(context.Context, db.GetAPILogStatsParams) (db.GetAPILogStatsRow, error) {
	return db.GetAPILogStatsRow{
		TotalRequests:      3,
		SuccessfulRequests: 2,
		FailedRequests:     1,
		AvgDurationMs:      42.5,
		MinDurationMs:      int32(10),
		MaxDurationMs:      int32(80),
	}, nil
}

func (frontendAPILogStore) GetAPILogStatsByEndpoint(context.Context, db.GetAPILogStatsByEndpointParams) ([]db.GetAPILogStatsByEndpointRow, error) {
	return []db.GetAPILogStatsByEndpointRow{
		{
			ApiEndpoint:        "me/messages",
			TotalRequests:      2,
			SuccessfulRequests: 2,
			FailedRequests:     0,
			AvgDurationMs:      35,
		},
		{
			ApiEndpoint:        "users",
			TotalRequests:      1,
			SuccessfulRequests: 0,
			FailedRequests:     1,
			AvgDurationMs:      80,
		},
	}, nil
}

func (frontendAPILogStore) GetAPILogs(context.Context, db.GetAPILogsParams) ([]db.ApiLog, error) {
	return frontendLogs(), nil
}

func (frontendAPILogStore) GetAPILogsByJobType(ctx context.Context, params db.GetAPILogsByJobTypeParams) ([]db.ApiLog, error) {
	var filtered []db.ApiLog
	for _, log := range frontendLogs() {
		if log.JobType == params.JobType {
			filtered = append(filtered, log)
		}
	}
	return filtered, nil
}

func (frontendAPILogStore) GetAPILogsByTimeRange(context.Context, db.GetAPILogsByTimeRangeParams) ([]db.ApiLog, error) {
	return frontendLogs(), nil
}

func (frontendAPILogStore) GetAPILogsByUser(ctx context.Context, params db.GetAPILogsByUserParams) ([]db.ApiLog, error) {
	var filtered []db.ApiLog
	for _, log := range frontendLogs() {
		if log.UserID.Valid && params.UserID.Valid && log.UserID.String == params.UserID.String {
			filtered = append(filtered, log)
		}
	}
	return filtered, nil
}

func frontendLogs() []db.ApiLog {
	now := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	return []db.ApiLog{
		{
			ID:             1,
			UserID:         sql.NullString{String: "frontend@example.com", Valid: true},
			ApiEndpoint:    "me/messages",
			HttpMethod:     "GET",
			HttpStatusCode: 200,
			RequestTime:    now,
			ResponseTime:   now.Add(20 * time.Millisecond),
			DurationMs:     20,
			JobType:        "user_mail",
			Success:        true,
		},
		{
			ID:             2,
			ApiEndpoint:    "users",
			HttpMethod:     "GET",
			HttpStatusCode: 500,
			RequestTime:    now.Add(-time.Minute),
			ResponseTime:   now.Add(-time.Minute + 80*time.Millisecond),
			DurationMs:     80,
			ErrorMessage:   sql.NullString{String: "graph request failed", Valid: true},
			JobType:        "client_credentials",
			Success:        false,
		},
	}
}

func main() {
	port := os.Getenv("FRONTEND_TEST_PORT")
	if port == "" {
		port = "4173"
	}

	if err := i18n.Init(); err != nil {
		panic(err)
	}

	gob.Register(oidc.IDToken{})
	gob.Register(oauth2.Token{})
	gob.Register(environment.AzureADClaims{})

	tmpl, err := view.New()
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionManager := scs.New()
	app := environment.Application{
		Logger:         logger,
		Template:       tmpl,
		SessionManager: sessionManager,
	}

	home := controller.NewHomeController(app, nil, nil)
	health := controller.NewHealthController(app)
	apiLogService := services.NewAPILogService(frontendAPILogStore{}, logger)
	logs := controller.NewLogsController(app, apiLogService)

	r := chi.NewRouter()
	r.Use(sessionManager.LoadAndSave)
	r.Use(appmiddleware.I18nMiddleware)
	r.Use(appmiddleware.SessionUserMiddleware(sessionManager))
	r.Handle("/statics/*", view.StaticFileHandler())
	r.Get("/", home.Index)
	r.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(home.About()))
	})
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Login placeholder"))
	})
	r.Get("/user", home.User)
	r.Get("/logs", logs.Index)
	r.Get("/logs/stats", logs.Stats)
	r.Get("/health/live", health.Live)
	r.Get("/test/login", func(w http.ResponseWriter, r *http.Request) {
		sessionManager.Put(r.Context(), "user", oidc.IDToken{Subject: "frontend-test-user"})
		sessionManager.Put(r.Context(), "claims", environment.AzureADClaims{
			Name:              "Frontend Test User",
			PreferredUsername: "frontend@example.com",
			Email:             "frontend@example.com",
		})
		sessionManager.Put(r.Context(), "token", oauth2.Token{AccessToken: "frontend-token"})
		http.Redirect(w, r, "/user", http.StatusSeeOther)
	})

	if err := http.ListenAndServe(":"+port, r); err != nil {
		panic(fmt.Errorf("frontend test server failed: %w", err))
	}
}
