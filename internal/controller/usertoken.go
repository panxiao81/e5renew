package controller

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"
)

type UserTokenController struct {
	environment.Application
	auth             environment.Authenticator
	userTokenService *services.UserTokenService
	errorHandler     *utils.ErrorHandler
	validator        *utils.Validator
}

func NewUserTokenController(app environment.Application, auth environment.Authenticator, userTokenService *services.UserTokenService) *UserTokenController {
	return &UserTokenController{
		Application:      app,
		auth:             auth,
		userTokenService: userTokenService,
		errorHandler:     utils.NewErrorHandler(app.Logger),
		validator:        utils.NewValidator(),
	}
}

// AuthorizeUserToken initiates the OAuth2 flow for user token authorization
func (c *UserTokenController) AuthorizeUserToken(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "authorize_user_token")
	defer span.End()

	// Check if user is logged in
	if !c.SessionManager.Exists(ctx, "user") {
		span.SetAttributes(attribute.Bool("user_logged_in", false))
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Get user claims for user ID
	claims := c.SessionManager.Get(ctx, "claims").(environment.AzureADClaims)
	userID := claims.Email // Use email as user ID

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.Bool("user_logged_in", true),
	)

	// Generate random state for security
	state, err := generateUserTokenState()
	if err != nil {
		span.RecordError(err)
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Unable to initiate authorization")
		return
	}

	// Store state in session
	c.SessionManager.Put(ctx, "user_token_state", state)

	// Create authorization URL
	authURL := c.auth.AuthCodeURL(state, oauth2.SetAuthURLParam("state", state))

	span.SetAttributes(
		attribute.String("auth_url", authURL),
		attribute.String("auth_provider", "azure_ad"),
		attribute.String("auth_action", "user_token_authorize"),
	)

	c.Logger.Info("Starting user token authorization", "userID", userID)

	// Redirect to Azure AD for authorization
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// UserTokenCallback handles the OAuth2 callback for user token authorization
func (c *UserTokenController) UserTokenCallback(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "user_token_callback")
	defer span.End()

	// Verify state parameter
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	sessionState := c.SessionManager.PopString(ctx, "user_token_state")

	// Validate inputs
	stateValidation := c.validator.ValidateState(state)
	codeValidation := c.validator.ValidateAuthCode(code)
	
	if !stateValidation.Valid || !codeValidation.Valid {
		span.SetAttributes(
			attribute.String("error", "invalid_input"),
			attribute.Bool("success", false),
		)
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Invalid request parameters")
		return
	}

	if state != sessionState {
		span.SetAttributes(
			attribute.String("error", "state_mismatch"),
			attribute.Bool("success", false),
		)
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Invalid authorization state")
		return
	}

	// Check if user is still logged in
	if !c.SessionManager.Exists(ctx, "user") {
		span.SetAttributes(attribute.Bool("user_logged_in", false))
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Get user claims for user ID
	claims := c.SessionManager.Get(ctx, "claims").(environment.AzureADClaims)
	userID := claims.Email // Use email as user ID

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.String("auth_provider", "azure_ad"),
		attribute.String("auth_action", "user_token_callback"),
	)

	// Exchange code for token (code already validated above)
	token, err := c.auth.Exchange(ctx, code)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.String("error", "token_exchange_failed"),
			attribute.Bool("success", false),
		)
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Authorization failed")
		return
	}

	// Save token to database
	err = c.userTokenService.SaveUserToken(ctx, userID, token)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.String("error", "token_save_failed"),
			attribute.Bool("success", false),
		)
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Failed to save authorization")
		return
	}

	span.SetAttributes(
		attribute.Bool("success", true),
		attribute.String("token_type", token.TokenType),
		attribute.Bool("has_refresh_token", token.RefreshToken != ""),
		attribute.String("token_expiry", token.Expiry.Format("2006-01-02T15:04:05Z")),
	)

	c.Logger.Info("Successfully saved user token", "userID", userID)

	// Redirect back to user page
	http.Redirect(w, r, "/user", http.StatusTemporaryRedirect)
}

// RevokeUserToken revokes and deletes a user's token
func (c *UserTokenController) RevokeUserToken(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "revoke_user_token")
	defer span.End()

	// Check if user is logged in
	if !c.SessionManager.Exists(ctx, "user") {
		span.SetAttributes(attribute.Bool("user_logged_in", false))
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Get user claims for user ID
	claims := c.SessionManager.Get(ctx, "claims").(environment.AzureADClaims)
	userID := claims.Email // Use email as user ID

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.String("auth_action", "revoke_token"),
	)

	// Delete token from database
	err := c.userTokenService.DeleteUserToken(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("success", false))
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Failed to revoke authorization")
		return
	}

	span.SetAttributes(attribute.Bool("success", true))
	c.Logger.Info("Successfully revoked user token", "userID", userID)

	// Redirect back to user page
	http.Redirect(w, r, "/user", http.StatusSeeOther)
}

// generateUserTokenState generates a random state string for OAuth2 security
func generateUserTokenState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// RegisterRoutes registers the user token controller routes
func (c *UserTokenController) RegisterRoutes(router *chi.Mux) {
	router.Get("/user/authorize-token", c.AuthorizeUserToken)
	router.Get("/oauth2/callback-user-token", c.UserTokenCallback)
	router.Post("/user/revoke-token", c.RevokeUserToken)
}
