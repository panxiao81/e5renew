package requestctx

import (
	"context"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/require"
)

func TestUserContextHelpers(t *testing.T) {
	t.Run("with nil user keeps context empty", func(t *testing.T) {
		ctx := WithUser(context.Background(), nil)
		user, ok := UserFromContext(ctx)
		require.False(t, ok)
		require.Nil(t, user)
	})

	t.Run("with user stores and returns user", func(t *testing.T) {
		expected := &oidc.IDToken{Subject: "u1"}
		ctx := WithUser(context.Background(), expected)
		user, ok := UserFromContext(ctx)
		require.True(t, ok)
		require.Same(t, expected, user)
	})

	t.Run("wrong type in context is ignored", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), userKey, "bad")
		user, ok := UserFromContext(ctx)
		require.False(t, ok)
		require.Nil(t, user)
	})
}
