package middleware

import (
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/panxiao81/e5renew/internal/requestctx"
)

func SessionUserMiddleware(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sessionManager == nil || !sessionManager.Exists(r.Context(), "user") {
				next.ServeHTTP(w, r)
				return
			}

			var user *oidc.IDToken
			switch v := sessionManager.Get(r.Context(), "user").(type) {
			case oidc.IDToken:
				user = &v
			case *oidc.IDToken:
				user = v
			}

			if user == nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := requestctx.WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
