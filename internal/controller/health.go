package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/panxiao81/e5renew/internal/environment"
)

// HealthStatus represents the health status of a component
type HealthStatus string

const (
	HealthStatusUp      HealthStatus = "up"
	HealthStatusDown    HealthStatus = "down"
	HealthStatusUnknown HealthStatus = "unknown"
)

// HealthCheck represents a single health check result
type HealthCheck struct {
	Name      string            `json:"name"`
	Status    HealthStatus      `json:"status"`
	Message   string            `json:"message,omitempty"`
	Duration  string            `json:"duration,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// HealthResponse represents the overall health response
type HealthResponse struct {
	Status      HealthStatus  `json:"status"`
	Timestamp   time.Time     `json:"timestamp"`
	Duration    string        `json:"duration"`
	Checks      []HealthCheck `json:"checks"`
	Version     string        `json:"version"`
	Environment string        `json:"environment"`
}

// HealthController provides health check endpoints
type HealthController struct {
	environment.Application
}

// NewHealthController creates a new HealthController instance
func NewHealthController(app environment.Application) *HealthController {
	return &HealthController{
		Application: app,
	}
}

// Health performs comprehensive health checks
func (h *HealthController) Health(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "health_check")
	defer span.End()

	startTime := time.Now()
	h.Logger.Info("Health check started",
		"path", r.URL.Path,
		"method", r.Method,
		"remote_addr", r.RemoteAddr,
	)

	checks := []HealthCheck{
		h.checkDatabase(ctx),
		h.checkAzureADIssuer(ctx),
		h.checkApplication(ctx),
	}

	overallStatus := HealthStatusUp
	for _, check := range checks {
		if check.Status == HealthStatusDown {
			overallStatus = HealthStatusDown
			break
		}
		if check.Status == HealthStatusUnknown && overallStatus == HealthStatusUp {
			overallStatus = HealthStatusUnknown
		}
	}

	duration := time.Since(startTime)

	response := HealthResponse{
		Status:      overallStatus,
		Timestamp:   time.Now(),
		Duration:    duration.String(),
		Checks:      checks,
		Version:     "v0.1.0",      // Could be made configurable
		Environment: "development", // Could be made configurable
	}

	statusCode := http.StatusOK
	if overallStatus == HealthStatusDown {
		statusCode = http.StatusServiceUnavailable
	}

	span.SetAttributes(
		attribute.String("health.status", string(overallStatus)),
		attribute.Int("health.checks.count", len(checks)),
		attribute.Int64("health.duration.ms", duration.Milliseconds()),
		attribute.Int("http.status_code", statusCode),
	)

	if overallStatus != HealthStatusUp {
		h.Logger.Error("Health check not healthy",
			"overall_status", overallStatus,
			"http_status", statusCode,
			"duration", duration.String(),
			"checks", checks,
		)
	} else {
		h.Logger.Info("Health check completed",
			"overall_status", overallStatus,
			"http_status", statusCode,
			"duration", duration.String(),
		)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		span.RecordError(err)
		h.Logger.Error("Failed to encode health response", "error", err)
	}
}

// Ready provides a readiness check (simpler than health)
func (h *HealthController) Ready(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "readiness_check")
	defer span.End()

	dbCheck := h.checkDatabase(ctx)

	if dbCheck.Status == HealthStatusUp {
		span.SetAttributes(attribute.Bool("ready", true))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	} else {
		span.SetAttributes(attribute.Bool("ready", false))
		span.RecordError(fmt.Errorf("database not ready: %s", dbCheck.Message))
		h.Logger.Error("Readiness check failed", "database_check", dbCheck)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
	}
}

// Live provides a liveness check
func (h *HealthController) Live(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	_, span := tracer.Start(r.Context(), "liveness_check")
	defer span.End()

	span.SetAttributes(attribute.Bool("alive", true))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("alive"))
}

// checkDatabase performs a database health check
func (h *HealthController) checkDatabase(ctx context.Context) HealthCheck {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(ctx, "health_check_database")
	defer span.End()

	startTime := time.Now()
	check := HealthCheck{
		Name:      "database",
		Timestamp: startTime,
		Metadata:  make(map[string]string),
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	check.Metadata["engine"] = viper.GetString("database.engine")
	check.Metadata["dsn_set"] = fmt.Sprintf("%t", viper.GetString("database.dsn") != "")
	check.Metadata["dsn_hint"] = sanitizeDSN(viper.GetString("database.dsn"))

	if stats, err := h.DB.Stats(); err == nil {
		check.Metadata["pool_open_connections"] = fmt.Sprintf("%d", stats.OpenConnections)
		check.Metadata["pool_in_use"] = fmt.Sprintf("%d", stats.InUse)
		check.Metadata["pool_idle"] = fmt.Sprintf("%d", stats.Idle)
	} else {
		check.Metadata["pool_stats_error"] = err.Error()
	}

	h.Logger.Info("Health database ping start",
		"engine", check.Metadata["engine"],
		"dsn_hint", check.Metadata["dsn_hint"],
		"pool_open_connections", check.Metadata["pool_open_connections"],
		"pool_in_use", check.Metadata["pool_in_use"],
		"pool_idle", check.Metadata["pool_idle"],
	)

	if err := h.DB.PingContext(ctx); err != nil {
		check.Status = HealthStatusDown
		check.Message = fmt.Sprintf("Database ping failed: %v", err)
		span.RecordError(err)
		h.Logger.Error("Health database ping failed",
			"error", err,
			"engine", check.Metadata["engine"],
			"dsn_hint", check.Metadata["dsn_hint"],
		)
	} else {
		check.Status = HealthStatusUp
		check.Message = "Database ping healthy"
		h.Logger.Info("Health database ping success",
			"engine", check.Metadata["engine"],
			"dsn_hint", check.Metadata["dsn_hint"],
		)
	}

	check.Duration = time.Since(startTime).String()
	check.Metadata["connection_type"] = "database/sql_ping"

	span.SetAttributes(
		attribute.Bool("database.healthy", check.Status == HealthStatusUp),
		attribute.String("database.connection_type", "database/sql_ping"),
	)

	return check
}

func (h *HealthController) checkAzureADIssuer(ctx context.Context) HealthCheck {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(ctx, "health_check_azure_ad_issuer")
	defer span.End()

	startTime := time.Now()
	check := HealthCheck{
		Name:      "azure_ad_issuer",
		Timestamp: startTime,
		Metadata:  make(map[string]string),
	}

	tenant := viper.GetString("azureAD.tenant")
	if tenant == "" {
		tenant = "common"
	}
	issuerEndpoint := environment.AzureADIssuerFromConfig()
	clientID := viper.GetString("azureAD.clientID")
	redirectURL := viper.GetString("azureAD.redirectURL")
	userTokenRedirectURL := environment.DeriveUserTokenRedirectURL(redirectURL)

	check.Metadata["tenant"] = tenant
	check.Metadata["issuer_endpoint"] = issuerEndpoint
	check.Metadata["issuer_from_config"] = fmt.Sprintf("%t", viper.GetString("azureAD.issuer") != "")
	check.Metadata["client_id_set"] = fmt.Sprintf("%t", clientID != "")
	check.Metadata["redirect_url"] = redirectURL
	check.Metadata["user_token_redirect_url"] = userTokenRedirectURL

	redirectIssue := validateRedirectURL(redirectURL)
	if redirectIssue != "" {
		check.Metadata["redirect_warning"] = redirectIssue
		h.Logger.Error("Azure AD redirect URL validation warning", "issue", redirectIssue, "redirect_url", redirectURL)
	} else {
		h.Logger.Info("Azure AD redirect URL looks valid for login callback", "redirect_url", redirectURL)
	}

	userTokenRedirectIssue := validateUserTokenRedirectURL(redirectURL, userTokenRedirectURL)
	if userTokenRedirectIssue != "" {
		check.Metadata["user_token_redirect_warning"] = userTokenRedirectIssue
		h.Logger.Error("Azure AD user-token redirect URL validation warning", "issue", userTokenRedirectIssue, "user_token_redirect_url", userTokenRedirectURL)
	} else {
		h.Logger.Info("Azure AD user-token redirect URL looks valid", "user_token_redirect_url", userTokenRedirectURL)
	}

	h.Logger.Info("Azure AD health config snapshot",
		"tenant", tenant,
		"issuer_endpoint", issuerEndpoint,
		"issuer_from_config", check.Metadata["issuer_from_config"],
		"client_id_set", check.Metadata["client_id_set"],
		"redirect_url", redirectURL,
		"user_token_redirect_url", userTokenRedirectURL,
	)

	providerCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	provider, err := oidc.NewProvider(providerCtx, issuerEndpoint)
	if err != nil {
		check.Status = HealthStatusDown
		check.Message = fmt.Sprintf("Azure AD issuer discovery failed: %v", err)
		check.Metadata["redirect_mismatch_causes_issuer_fetch_failure"] = "false"
		span.RecordError(err)
		h.Logger.Error("Azure AD issuer discovery failed",
			"tenant", tenant,
			"issuer_endpoint", issuerEndpoint,
			"redirect_url", redirectURL,
			"error", err,
		)
	} else {
		check.Status = HealthStatusUp
		check.Message = "Azure AD issuer discovery healthy"
		check.Metadata["resolved_auth_url"] = provider.Endpoint().AuthURL
		check.Metadata["redirect_mismatch_causes_issuer_fetch_failure"] = "false"
		h.Logger.Info("Azure AD issuer discovery success",
			"tenant", tenant,
			"issuer_endpoint", issuerEndpoint,
			"auth_url", provider.Endpoint().AuthURL,
		)
	}

	if check.Status == HealthStatusUp && (redirectIssue != "" || userTokenRedirectIssue != "") {
		check.Status = HealthStatusUnknown
		check.Message = "Azure AD issuer is healthy but redirectURL config may cause OAuth callback mismatch"
	}

	check.Duration = time.Since(startTime).String()
	span.SetAttributes(
		attribute.Bool("azuread.healthy", check.Status == HealthStatusUp),
		attribute.String("azuread.tenant", tenant),
		attribute.String("azuread.issuer_endpoint", issuerEndpoint),
	)

	return check
}

func validateRedirectURL(redirectURL string) string {
	if redirectURL == "" {
		return "azureAD.redirectURL is empty"
	}

	u, err := url.Parse(redirectURL)
	if err != nil {
		return fmt.Sprintf("azureAD.redirectURL parse failed: %v", err)
	}
	if u.Scheme != "https" && !strings.HasPrefix(u.Host, "localhost") {
		return "azureAD.redirectURL should use https in non-localhost environment"
	}
	if u.Path != "/oauth2/callback" {
		return fmt.Sprintf("azureAD.redirectURL path is %q; expected %q (user-token callback is derived by appending -user-token)", u.Path, "/oauth2/callback")
	}
	return ""
}

func validateUserTokenRedirectURL(baseRedirectURL, userTokenRedirectURL string) string {
	base, err := url.Parse(baseRedirectURL)
	if err != nil {
		return fmt.Sprintf("base redirect URL parse failed: %v", err)
	}
	userToken, err := url.Parse(userTokenRedirectURL)
	if err != nil {
		return fmt.Sprintf("user token redirect URL parse failed: %v", err)
	}
	if base.Scheme != userToken.Scheme || base.Host != userToken.Host {
		return "user token redirect URL has scheme/host mismatch with azureAD.redirectURL"
	}
	if userToken.Path != "/oauth2/callback-user-token" {
		return fmt.Sprintf("user token redirect path is %q; expected %q", userToken.Path, "/oauth2/callback-user-token")
	}
	return ""
}

func sanitizeDSN(dsn string) string {
	if dsn == "" {
		return "(empty)"
	}
	if u, err := url.Parse(dsn); err == nil && u.Scheme != "" {
		host := u.Host
		if host == "" {
			host = "(none)"
		}
		return fmt.Sprintf("%s://%s%s", u.Scheme, host, u.Path)
	}
	if at := strings.LastIndex(dsn, "@"); at >= 0 && at+1 < len(dsn) {
		return "***@" + dsn[at+1:]
	}
	if len(dsn) > 24 {
		return dsn[:24] + "..."
	}
	return dsn
}

// checkApplication performs application-specific health checks
func (h *HealthController) checkApplication(ctx context.Context) HealthCheck {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	_, span := tracer.Start(ctx, "health_check_application")
	defer span.End()

	startTime := time.Now()
	check := HealthCheck{
		Name:      "application",
		Timestamp: startTime,
		Metadata:  make(map[string]string),
	}

	check.Status = HealthStatusUp
	check.Message = "Application healthy"
	check.Duration = time.Since(startTime).String()
	check.Metadata["session_manager"] = "active"
	check.Metadata["template_engine"] = "active"

	span.SetAttributes(
		attribute.Bool("application.healthy", true),
		attribute.String("application.session_manager", "active"),
		attribute.String("application.template_engine", "active"),
	)

	return check
}

// RegisterRoutes registers the health check routes
func (h *HealthController) RegisterRoutes(router *chi.Mux) {
	router.Get("/health", h.Health)
	router.Get("/health/ready", h.Ready)
	router.Get("/health/live", h.Live)
}
