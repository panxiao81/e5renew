package controller

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/utils"
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
	ctx := r.Context()

	// Check if user is logged in
	if !c.SessionManager.Exists(ctx, "user") {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Get user claims for user ID
	claims := c.SessionManager.Get(ctx, "claims").(environment.AzureADClaims)
	userID := claims.Email // Use email as user ID

	// Generate random state for security
	state, err := generateUserTokenState()
	if err != nil {
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Unable to initiate authorization")
		return
	}

	// Store state in session
	c.SessionManager.Put(ctx, "user_token_state", state)

	// Create authorization URL
	authURL := c.auth.AuthCodeURL(state, oauth2.SetAuthURLParam("state", state))

	c.Logger.Info("Starting user token authorization", "userID", userID)

	// Redirect to Azure AD for authorization
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// UserTokenCallback handles the OAuth2 callback for user token authorization
func (c *UserTokenController) UserTokenCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Verify state parameter
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	oauthError := r.URL.Query().Get("error")
	oauthErrorDescription := r.URL.Query().Get("error_description")
	sessionState := c.SessionManager.PopString(ctx, "user_token_state")

	if oauthError != "" {
		c.Logger.Error("User-token callback returned OAuth error",
			"oauth_error", oauthError,
			"oauth_error_description", oauthErrorDescription,
			"has_state", state != "",
			"state_len", len(state),
			"path", r.URL.Path,
		)
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Authorization failed")
		return
	}

	// Validate inputs
	stateValidation := c.validator.ValidateState(state)
	codeValidation := c.validator.ValidateAuthCode(code)

	if !stateValidation.Valid || !codeValidation.Valid {
		c.Logger.Error("User-token callback validation failed",
			"state_valid", stateValidation.Valid,
			"state_errors", strings.Join(stateValidation.Errors, "; "),
			"state_len", len(state),
			"code_valid", codeValidation.Valid,
			"code_errors", strings.Join(codeValidation.Errors, "; "),
			"code_len", len(code),
			"path", r.URL.Path,
		)
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Invalid request parameters")
		return
	}

	if state != sessionState {
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Invalid authorization state")
		return
	}

	// Check if user is still logged in
	if !c.SessionManager.Exists(ctx, "user") {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Get user claims for user ID
	claims := c.SessionManager.Get(ctx, "claims").(environment.AzureADClaims)
	userID := claims.Email // Use email as user ID

	// Exchange code for token (code already validated above)
	token, err := c.auth.Exchange(ctx, code)
	if err != nil {
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Authorization failed")
		return
	}

	// Save token to database
	err = c.userTokenService.SaveUserToken(ctx, userID, token)
	if err != nil {
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Failed to save authorization")
		return
	}

	c.Logger.Info("Successfully saved user token", "userID", userID)

	// Redirect back to user page
	http.Redirect(w, r, "/user", http.StatusTemporaryRedirect)
}

// RevokeUserToken revokes and deletes a user's token
func (c *UserTokenController) RevokeUserToken(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if user is logged in
	if !c.SessionManager.Exists(ctx, "user") {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Get user claims for user ID
	claims := c.SessionManager.Get(ctx, "claims").(environment.AzureADClaims)
	userID := claims.Email // Use email as user ID

	// Delete token from database
	err := c.userTokenService.DeleteUserToken(ctx, userID)
	if err != nil {
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Failed to revoke authorization")
		return
	}
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
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RegisterRoutes registers the user token controller routes
func (c *UserTokenController) RegisterRoutes(router *chi.Mux) {
	router.Get("/user/authorize-token", c.AuthorizeUserToken)
	router.Get("/oauth2/callback-user-token", c.UserTokenCallback)
	router.Post("/user/revoke-token", c.RevokeUserToken)
}
