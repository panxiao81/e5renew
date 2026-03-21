package requestctx

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
)

type contextKey string

const userKey contextKey = "request-user"

func WithUser(ctx context.Context, user *oidc.IDToken) context.Context {
	if user == nil {
		return ctx
	}
	return context.WithValue(ctx, userKey, user)
}

func UserFromContext(ctx context.Context) (*oidc.IDToken, bool) {
	user, ok := ctx.Value(userKey).(*oidc.IDToken)
	if !ok || user == nil {
		return nil, false
	}
	return user, true
}
