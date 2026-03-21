package main

import (
	"context"
	"database/sql"
	"testing"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/stretchr/testify/require"
)

func TestFrontendAPILogStoreFilters(t *testing.T) {
	store := frontendAPILogStore{}

	logs, err := store.GetAPILogsByJobType(context.Background(), db.GetAPILogsByJobTypeParams{JobType: "client_credentials"})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "client_credentials", logs[0].JobType)

	logs, err = store.GetAPILogsByUser(context.Background(), db.GetAPILogsByUserParams{UserID: sql.NullString{String: "frontend@example.com", Valid: true}})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "frontend@example.com", logs[0].UserID.String)
}

func TestFrontendLogsSeedData(t *testing.T) {
	logs := frontendLogs()
	require.Len(t, logs, 2)
	require.Equal(t, int64(1), logs[0].ID)
	require.Equal(t, "me/messages", logs[0].ApiEndpoint)
	require.Equal(t, int64(2), logs[1].ID)
	require.Equal(t, "graph request failed", logs[1].ErrorMessage.String)
}
