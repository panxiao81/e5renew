package controller

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "login_start")
	defer span.End()

	// for now, we only support login by azure ad.
	// so, simply redirect to auth endpoint
	state, err := generateRandomState()
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("success", false))
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Unable to initiate login")
		return
	}

	span.SetAttributes(
		attribute.String("auth.provider", "azure_ad"),
		attribute.String("auth.action", "login_start"),
		attribute.Bool("success", true),
	)

	c.SessionManager.Put(ctx, "state", state)
	redirectURL := c.auth.AuthCodeURL(state, oauth2.SetAuthURLParam("state", state))
	span.SetAttributes(attribute.String("redirect_url", redirectURL))

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (c *LoginController) Callback(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "login_callback")
	defer span.End()

	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	
	// Validate inputs
	stateValidation := c.validator.ValidateState(state)
	codeValidation := c.validator.ValidateAuthCode(code)
	
	if !stateValidation.Valid || !codeValidation.Valid {
		span.SetAttributes(
			attribute.String("auth.provider", "azure_ad"),
			attribute.String("auth.action", "callback"),
			attribute.Bool("success", false),
			attribute.String("error", "invalid_input"),
		)
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Invalid request parameters")
		return
	}
	
	if state != c.SessionManager.PopString(ctx, "state") {
		span.SetAttributes(
			attribute.String("auth.provider", "azure_ad"),
			attribute.String("auth.action", "callback"),
			attribute.Bool("success", false),
			attribute.String("error", "state_mismatch"),
		)
		c.errorHandler.HandleError(w, r, nil, http.StatusBadRequest, "Invalid login state")
		return
	}

	oauth2Token, err := c.auth.Exchange(ctx, code)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.String("auth.provider", "azure_ad"),
			attribute.String("auth.action", "token_exchange"),
			attribute.Bool("success", false),
		)
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Authentication failed")
		return
	}

	idToken, err := c.auth.VerifyIDToken(ctx, oauth2Token)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.String("auth.provider", "azure_ad"),
			attribute.String("auth.action", "token_verify"),
			attribute.Bool("success", false),
		)
		c.errorHandler.HandleError(w, r, err, http.StatusInternalServerError, "Token verification failed")
		return
	}

	c.SessionManager.Put(ctx, "user", idToken)
	c.SessionManager.Put(ctx, "token", oauth2Token)
	claims := new(environment.AzureADClaims)
	environment.ParseClaims(idToken, claims)
	c.SessionManager.Put(ctx, "claims", claims)

	span.SetAttributes(
		attribute.String("auth.provider", "azure_ad"),
		attribute.String("auth.action", "callback"),
		attribute.Bool("success", true),
		attribute.String("user.email", claims.Email),
		attribute.String("user.name", claims.Name),
	)

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (c *LoginController) Logout(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "logout")
	defer span.End()

	span.SetAttributes(
		attribute.String("auth.provider", "azure_ad"),
		attribute.String("auth.action", "logout"),
		attribute.Bool("success", true),
	)

	c.SessionManager.Destroy(ctx)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	state := base64.StdEncoding.EncodeToString(b)

	return state, nil
}

func (c *LoginController) RegisterRoutes(router *chi.Mux) {
	router.Get("/login", c.Login)
	router.Get("/oauth2/callback", c.Callback)
	router.Get("/logout", c.Logout)
}
