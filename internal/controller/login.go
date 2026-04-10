package controller

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/utils"
	"golang.org/x/oauth2"
)

type LoginController struct {
	environment.Application
	auth         environment.Authenticator
	errorHandler *utils.ErrorHandler
	validator    *utils.Validator
}

func NewLoginController(app environment.Application, auth environment.Authenticator) LoginController {
	return LoginController{
		Application:  app,
		auth:         auth,
		errorHandler: utils.NewErrorHandler(app.Logger),
		validator:    utils.NewValidator(),
	}
}

func (c *LoginController) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// for now, we only support login by azure ad.
	// so, simply redirect to auth endpoint
	state, err := generateRandomState()
	if err != nil {
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Unable to initiate login")
		return
	}

	c.SessionManager.Put(ctx, "state", state)
	redirectURL := c.auth.AuthCodeURL(state, oauth2.SetAuthURLParam("state", state))

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (c *LoginController) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	oauthError := r.URL.Query().Get("error")
	oauthErrorDescription := r.URL.Query().Get("error_description")

	if oauthError != "" {
		c.Logger.Error("Azure AD callback returned OAuth error",
			"oauth_error", oauthError,
			"oauth_error_description", oauthErrorDescription,
			"has_state", state != "",
			"state_len", len(state),
			"path", r.URL.Path,
		)
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Authentication failed")
		return
	}

	// Validate inputs
	stateValidation := c.validator.ValidateState(state)
	codeValidation := c.validator.ValidateAuthCode(code)

	if !stateValidation.Valid || !codeValidation.Valid {
		c.Logger.Error("Login callback validation failed",
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

	if state != c.SessionManager.PopString(ctx, "state") {
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Invalid login state")
		return
	}

	oauth2Token, err := c.auth.Exchange(ctx, code)
	if err != nil {
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Authentication failed")
		return
	}

	idToken, err := c.auth.VerifyIDToken(ctx, oauth2Token)
	if err != nil {
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Token verification failed")
		return
	}

	c.SessionManager.Put(ctx, "user", idToken)
	c.SessionManager.Put(ctx, "token", oauth2Token)
	claims := new(environment.AzureADClaims)
	environment.ParseClaims(idToken, claims)
	c.SessionManager.Put(ctx, "claims", claims)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (c *LoginController) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	c.SessionManager.Destroy(ctx)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	// URL-safe state avoids '+' handling issues in query strings.
	state := base64.RawURLEncoding.EncodeToString(b)

	return state, nil
}

func (c *LoginController) RegisterRoutes(router *chi.Mux) {
	router.Get("/login", c.Login)
	router.Get("/oauth2/callback", c.Callback)
	router.Get("/logout", c.Logout)
}
