package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v3"
	"github.com/go-co-op/gocron/v2"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/panxiao81/e5renew/internal/controller"
	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/i18n"
	"github.com/panxiao81/e5renew/internal/jobs"
	"github.com/panxiao81/e5renew/internal/middleware"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/telemetry"
	"github.com/panxiao81/e5renew/internal/view"
)

func Run() error {
	ctx := context.Background()

	// This function is intended to be the entry point for the application.
	// It can be used to initialize configurations, set up logging, or perform any
	// other necessary setup before executing the main logic of the application.

	// logger := log.New(os.Stdout, "e5renew: ", log.LstdFlags)
	logger := newHttpLogger()

	// Initialize OpenTelemetry
	otelConfig := telemetry.NewConfig()
	otelProvider, err := telemetry.NewProvider(ctx, otelConfig, logger)
	if err != nil {
		logger.Error("Failed to initialize OpenTelemetry: " + err.Error())
		return fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}

	// Initialize metrics
	meter := otel.Meter("github.com/panxiao81/e5renew")
	metrics, err := telemetry.NewMetrics(meter, logger)
	if err != nil {
		logger.Error("Failed to initialize metrics: " + err.Error())
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	// Initialize tracer
	tracer := otel.Tracer("github.com/panxiao81/e5renew")

	// Create main span for application startup
	ctx, span := tracer.Start(ctx, "application_startup")
	defer span.End()

	r := chi.NewRouter()

	// Add OpenTelemetry middleware
	r.Use(telemetry.HTTPMiddleware("e5renew", metrics))

	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		Level:         slog.LevelInfo,
		Schema:        httplog.SchemaOTEL,
		RecoverPanics: true,
	}))

	// Initialize database with tracing
	ctx, dbSpan := tracer.Start(ctx, "database_initialization")
	engine := db.ParseEngine(viper.GetString("database.engine"))
	dbConn, err := db.NewDBWithEngine(engine, viper.GetString("database.dsn"))
	if err != nil {
		logger.Error("Failed to initialize Database Connection " + err.Error())
		telemetry.RecordError(ctx, metrics, err, "database_connection")
		dbSpan.RecordError(err)
		dbSpan.End()
		return fmt.Errorf("failed to initialize database connection: %w", err)
	}
	defer dbConn.Close()
	queries := db.New(dbConn)
	metrics.RecordDBConnection(ctx, 1)
	dbSpan.SetAttributes(
		attribute.String("database.dsn", viper.GetString("database.dsn")),
		attribute.String("database.engine", string(engine)),
	)
	dbSpan.End()

	sessionManager := scs.New()
	// newSessionStoreForEngine picks mysqlstore/postgresstore based on database.engine
	store, cleanup, err := newSessionStoreForEngine(engine, dbConn, 30*time.Minute)
	if err != nil {
		logger.Error("Failed to initialize session store: " + err.Error())
		telemetry.RecordError(ctx, metrics, err, "session_store")
		return fmt.Errorf("failed to initialize session store: %w", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	sessionManager.Store = store
	// ID Token from Microsoft Azure AD is 1 hour.
	sessionManager.Lifetime = time.Hour
	r.Use(sessionManager.LoadAndSave)

	// Add i18n middleware
	r.Use(middleware.I18nMiddleware)

	// Template initialization with tracing
	ctx, tmplSpan := tracer.Start(ctx, "template_initialization")
	tmpl, err := view.New()
	if err != nil {
		logger.Error("Failed to initialize templates: %v " + err.Error())
		telemetry.RecordError(ctx, metrics, err, "template_initialization")
		tmplSpan.RecordError(err)
		tmplSpan.End()
		return fmt.Errorf("failed to initialize templates: %w", err)
	}
	tmplSpan.End()

	// I18n initialization with tracing
	ctx, i18nSpan := tracer.Start(ctx, "i18n_initialization")
	err = i18n.Init()
	if err != nil {
		logger.Error("Failed to initialize i18n: " + err.Error())
		telemetry.RecordError(ctx, metrics, err, "i18n_initialization")
		i18nSpan.RecordError(err)
		i18nSpan.End()
		return fmt.Errorf("failed to initialize i18n: %w", err)
	}
	i18nSpan.End()

	// Authenticator initialization with tracing
	ctx, authSpan := tracer.Start(ctx, "authenticator_initialization")
	auth, err := environment.NewAuthenticator()
	if err != nil {
		logger.Error("Failed to initialize Authenticator Provider " + err.Error())
		telemetry.RecordError(ctx, metrics, err, "authenticator_initialization")
		authSpan.RecordError(err)
		authSpan.End()
		return fmt.Errorf("failed to initialize authenticator: %w", err)
	}
	authSpan.End()

	// User token authenticator initialization with tracing
	ctx, userTokenAuthSpan := tracer.Start(ctx, "user_token_authenticator_initialization")
	userTokenAuth, err := environment.NewUserTokenAuthenticator()
	if err != nil {
		logger.Error("Failed to initialize User Token Authenticator Provider " + err.Error())
		telemetry.RecordError(ctx, metrics, err, "user_token_authenticator_initialization")
		userTokenAuthSpan.RecordError(err)
		userTokenAuthSpan.End()
		return fmt.Errorf("failed to initialize user token authenticator: %w", err)
	}
	userTokenAuthSpan.End()

	// Services initialization with tracing
	ctx, servicesSpan := tracer.Start(ctx, "services_initialization")

	// Initialize encryption service
	encryptionService, err := services.NewEncryptionService()
	if err != nil {
		servicesSpan.RecordError(err)
		servicesSpan.End()
		return fmt.Errorf("failed to initialize encryption service: %w", err)
	}

	userTokenService := services.NewUserTokenService(queries, &userTokenAuth.Config, logger, encryptionService)
	apiLogService := services.NewAPILogService(queries, logger)
	mailService := services.NewMailService(userTokenService, apiLogService, logger)
	servicesSpan.End()

	app := environment.NewApplication(logger, tmpl, sessionManager, queries)

	// Register routes
	homeController := controller.NewHomeController(*app, userTokenService, mailService)
	homeController.RegisteRoutes(r)
	loginController := controller.NewLoginController(*app, *auth)
	loginController.RegisterRoutes(r)
	userTokenController := controller.NewUserTokenController(*app, *userTokenAuth, userTokenService)
	userTokenController.RegisterRoutes(r)
	logsController := controller.NewLogsController(*app, apiLogService)
	logsController.RegisterRoutes(r)
	healthController := controller.NewHealthController(*app)
	healthController.RegisterRoutes(r)

	r.Handle("/statics/*", view.StaticFileHandler())

	// Job scheduler initialization with tracing
	ctx, jobSpan := tracer.Start(ctx, "job_scheduler_initialization")
	s, err := newJobScheduler(queries, mailService, apiLogService, logger)
	if err != nil {
		logger.Error(err.Error())
		telemetry.RecordError(ctx, metrics, err, "job_scheduler_initialization")
		jobSpan.RecordError(err)
		jobSpan.End()
		return fmt.Errorf("failed to initialize job scheduler: %w", err)
	}
	jobSpan.End()

	logger.Info("🚀 Starting job scheduler",
		"totalJobs", "2",
		"jobs", "e5_renewal_client_scope,process_user_mail_tokens",
		"status", "starting")

	s.Start()

	logger.Info("✅ Job scheduler started successfully",
		"status", "running",
		"description", "All background jobs are now active")

	server := &http.Server{
		Addr:    viper.GetString("listen"),
		Handler: r,
	}
	serverCtx, serverStopCtx := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig
		shutdownCtx, shutdownCancel := context.WithTimeout(serverCtx, 30*time.Second)
		defer shutdownCancel()

		go func() {
			<-shutdownCtx.Done()
			if shutdownCtx.Err() == context.DeadlineExceeded {
				logger.Error("graceful shotdown timed out.. forcing exit.")
			}
		}()

		err := server.Shutdown(shutdownCtx)

		if err != nil {
			logger.Error(err.Error())
		}
		err = s.Shutdown()
		if err != nil {
			logger.Error(err.Error())
		}

		// Shutdown OpenTelemetry
		shutdownOtelCtx, shutdownOtelCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownOtelCancel()
		err = otelProvider.Shutdown(shutdownOtelCtx)
		if err != nil {
			logger.Error("Failed to shutdown OpenTelemetry: " + err.Error())
		}

		serverStopCtx()
	}()

	// Mark application as fully started
	span.SetAttributes(attribute.String("server.listen", viper.GetString("listen")))
	span.AddEvent("server_starting")

	logger.Info("Starting server on " + viper.GetString("listen"))
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		logger.Error(err.Error())
		return fmt.Errorf("server failed to start: %w", err)
	}
	<-serverCtx.Done()
	return nil
}

func newHttpLogger() *slog.Logger {
	logFormat := httplog.SchemaOTEL.Concise(true)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: logFormat.ReplaceAttr,
	})).With(
		slog.String("app", "e5renew"),
		slog.String("version", "v0.1.0"),
		slog.String("env", "Debug"),
	)
	return logger
}

func newJobScheduler(queries *db.Queries, mailService *services.MailService, apiLogService *services.APILogService, logger *slog.Logger) (gocron.Scheduler, error) {
	// Create JobScheduler wrapper
	jobScheduler, err := jobs.NewJobScheduler(queries)
	if err != nil {
		return nil, err
	}

	// Register existing job with API logging
	logger.Info("🔧 Registering E5 renewal job - Graph API client scope",
		"job", "GetUsersAndMessagesClientScope",
		"schedule", "random interval 1-2 hours",
		"type", "recurring_job",
		"description", "Microsoft Graph API calls to maintain E5 subscription activity")

	_, err = jobScheduler.NewJob(
		gocron.DurationRandomJob(
			time.Hour,
			2*time.Hour,
		), gocron.NewTask(
			jobs.GetUsersAndMessagesClientScope,
			context.Background(),
			apiLogService,
			logger,
		),
		gocron.WithName("e5_renewal_client_scope"),
		gocron.WithTags("e5_renewal", "graph_api", "client_scope"),
	)
	if err != nil {
		logger.Error("❌ Failed to register E5 renewal job", "error", err)
		return nil, err
	}

	logger.Info("✅ Successfully registered E5 renewal job",
		"job", "GetUsersAndMessagesClientScope",
		"schedule", "random interval 1-2 hours")

	// Register user mail tokens job
	err = jobScheduler.RegisterUserMailTokensJob(mailService, logger)
	if err != nil {
		return nil, err
	}

	// Register log cleanup job (30 days retention)
	err = jobScheduler.RegisterLogCleanupJob(apiLogService, logger, 30)
	if err != nil {
		return nil, err
	}

	return jobScheduler.Scheduler, nil
}
