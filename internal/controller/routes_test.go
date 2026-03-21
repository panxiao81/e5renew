package controller

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/environment"
)

func TestControllerRegisterRoutes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := environment.Application{Logger: logger, SessionManager: scs.New()}
	r := chi.NewRouter()

	home := NewHomeController(app, nil, nil)
	home.RegisteRoutes(r)

	login := NewLoginController(app, environment.Authenticator{Config: oauth2.Config{}})
	login.RegisterRoutes(r)

	userToken := NewUserTokenController(app, environment.Authenticator{Config: oauth2.Config{}}, nil)
	userToken.RegisterRoutes(r)

	logs := NewLogsController(app, nil)
	logs.RegisterRoutes(r)

	health := NewHealthController(app)
	health.RegisterRoutes(r)

	paths := map[string]bool{}
	err := chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		paths[method+" "+route] = true
		return nil
	})
	require.NoError(t, err)

	require.True(t, paths["GET /"])
	require.True(t, paths["GET /about"])
	require.True(t, paths["GET /login"])
	require.True(t, paths["GET /logout"])
	require.True(t, paths["GET /user/authorize-token"])
	require.True(t, paths["POST /user/revoke-token"])
	require.True(t, paths["GET /logs"])
	require.True(t, paths["GET /health"])
}
