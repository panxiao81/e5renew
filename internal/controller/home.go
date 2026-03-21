package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/utils"
)

type HomeController struct {
	environment.Application
	userTokenService *services.UserTokenService
	mailService      *services.MailService
	errorHandler     *utils.ErrorHandler
}

var hasUserTokenForHome = func(s *services.UserTokenService, ctx context.Context, userID string) (bool, error) {
	return s.HasUserToken(ctx, userID)
}

var getUserTokenForHome = func(s *services.UserTokenService, ctx context.Context, userID string) (*oauth2.Token, error) {
	return s.GetUserToken(ctx, userID)
}

var processUserMailActivityForHome = func(s *services.MailService, ctx context.Context, userID string) error {
	return s.ProcessUserMailActivity(ctx, userID)
}

// NewHomeController creates a new instance of HomeController.
func NewHomeController(app environment.Application, userTokenService *services.UserTokenService, mailService *services.MailService) *HomeController {
	return &HomeController{
		Application:      app,
		userTokenService: userTokenService,
		mailService:      mailService,
		errorHandler:     utils.NewErrorHandler(app.Logger),
	}
}

// Index handles the home page request.
func (hc *HomeController) Index(w http.ResponseWriter, r *http.Request) {
	hc.Logger.Debug("Hit Index Handler")

	idRaw, hasUser := hc.safeSessionGet(r.Context(), "user")
	id, ok := asIDToken(idRaw)

	// This method would typically render a template or return a response.
	// For simplicity, we return a string here.
	var logged *oidc.IDToken
	if hasUser && ok {
		logged = &id
	} else {
		logged = nil
	}
	err := hc.Template.RenderWithContext(r.Context(), w, "index.html", map[string]interface{}{
		"Title": "E5Renew",
		"User":  logged,
	})
	if err != nil {
		hc.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Unable to render page")
		return
	}
}

// About handles the about page request.
func (hc *HomeController) About() string {
	// This method would typically render a template or return a response.
	// For simplicity, we return a string here.
	return "About E5Renew: A tool for renewing Microsoft E5 licenses."
}

func (hc *HomeController) User(w http.ResponseWriter, r *http.Request) {
	hc.Logger.Debug("Hit User Handler")
	if !hc.safeSessionExists(r.Context(), "user") {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	userRaw, ok := hc.safeSessionGet(r.Context(), "user")
	if !ok {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	user, ok := asIDToken(userRaw)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	claimsRaw, ok := hc.safeSessionGet(r.Context(), "claims")
	if !ok {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	claims, ok := asAzureADClaims(claimsRaw)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	tokenRaw, ok := hc.safeSessionGet(r.Context(), "token")
	if !ok {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}
	token, ok := asOAuth2Token(tokenRaw)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Note: For now, we'll skip the Graph API call in the user page since it requires apiLogService
	// This call was mainly for demo purposes and the real functionality is in the background jobs
	messages := make(map[string]int)

	hc.Logger.Info("User page accessed")

	// Check user token status
	userID := claims.Email
	hasUserToken := false
	if hc.userTokenService != nil {
		hasUserToken, _ = hasUserTokenForHome(hc.userTokenService, r.Context(), userID)
	}
	var userTokenExpiry *time.Time
	if hasUserToken && hc.userTokenService != nil {
		userToken, err := getUserTokenForHome(hc.userTokenService, r.Context(), userID)
		if err == nil {
			userTokenExpiry = &userToken.Expiry
		}
	}

	err := hc.Template.RenderWithContext(r.Context(), w, "user.html", map[string]interface{}{
		"Title":           "User Details",
		"User":            user,
		"Claims":          claims,
		"Token":           token,
		"Messages":        messages,
		"Debug":           viper.GetBool("debug"),
		"HasUserToken":    hasUserToken,
		"UserTokenExpiry": userTokenExpiry,
	})
	if err != nil {
		hc.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Unable to render user page")
		return
	}
}

// TriggerMailAPI handles manual trigger of mail API call
func (hc *HomeController) TriggerMailAPI(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "TriggerMailAPI")
	defer span.End()

	// Check if user is authenticated
	if !hc.safeSessionExists(ctx, "user") {
		span.SetAttributes(attribute.String("error", "user_not_authenticated"))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user claims
	claimsRaw, ok := hc.safeSessionGet(ctx, "claims")
	if !ok {
		span.SetAttributes(attribute.String("error", "missing_claims"))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	claims, ok := asAzureADClaims(claimsRaw)
	if !ok {
		span.SetAttributes(attribute.String("error", "invalid_claims"))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := claims.Email

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.String("trigger_type", "manual"),
	)

	if hc.userTokenService == nil || hc.mailService == nil {
		span.SetAttributes(attribute.String("error", "service_not_configured"))
		hc.errorHandler.HandleJSONError(w, r, nil, http.StatusInternalServerError, "Required service is not configured", "service_not_configured")
		return
	}

	// Check if user has a token
	hasUserToken, err := hasUserTokenForHome(hc.userTokenService, ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", "failed_to_check_user_token"))
		hc.errorHandler.HandleJSONError(w, r, err, http.StatusInternalServerError, "Unable to check user token", "token_check_failed")
		return
	}

	if !hasUserToken {
		span.SetAttributes(attribute.String("error", "no_user_token"))
		response := map[string]interface{}{
			"success": false,
			"error":   "No personal mail access token found. Please authorize personal mail access first.",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Trigger mail API call
	startTime := time.Now()
	err = processUserMailActivityForHome(hc.mailService, ctx, userID)
	duration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int64("processing_duration_ms", duration.Milliseconds()),
		attribute.Bool("success", err == nil),
	)

	if err != nil {
		span.RecordError(err)
		hc.errorHandler.HandleJSONError(w, r, err, http.StatusInternalServerError, "Failed to process mail activity", "mail_processing_failed")
		return
	}

	// Success response
	response := map[string]interface{}{
		"success":            true,
		"message":            "Mail API call completed successfully",
		"processing_time":    duration.String(),
		"processing_time_ms": duration.Milliseconds(),
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	hc.Logger.Info("Manual mail API trigger completed",
		"userID", userID,
		"duration", duration,
		"success", true)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (hc *HomeController) RegisteRoutes(router *chi.Mux) {
	// Register the routes for the home controller.
	router.Get("/", hc.Index)
	router.Get("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(hc.About()))
	})
	router.Get("/user", hc.User)
	router.Post("/user/trigger-mail", hc.TriggerMailAPI)
}

func (hc *HomeController) safeSessionExists(ctx context.Context, key string) (exists bool) {
	defer func() {
		if recover() != nil {
			exists = false
		}
	}()
	return hc.SessionManager.Exists(ctx, key)
}

func (hc *HomeController) safeSessionGet(ctx context.Context, key string) (value any, ok bool) {
	defer func() {
		if recover() != nil {
			value = nil
			ok = false
		}
	}()
	return hc.SessionManager.Get(ctx, key), true
}

func asIDToken(value any) (oidc.IDToken, bool) {
	switch v := value.(type) {
	case oidc.IDToken:
		return v, true
	case *oidc.IDToken:
		if v == nil {
			return oidc.IDToken{}, false
		}
		return *v, true
	default:
		return oidc.IDToken{}, false
	}
}

func asAzureADClaims(value any) (environment.AzureADClaims, bool) {
	switch v := value.(type) {
	case environment.AzureADClaims:
		return v, true
	case *environment.AzureADClaims:
		if v == nil {
			return environment.AzureADClaims{}, false
		}
		return *v, true
	default:
		return environment.AzureADClaims{}, false
	}
}

func asOAuth2Token(value any) (oauth2.Token, bool) {
	switch v := value.(type) {
	case oauth2.Token:
		return v, true
	case *oauth2.Token:
		if v == nil {
			return oauth2.Token{}, false
		}
		return *v, true
	default:
		return oauth2.Token{}, false
	}
}
