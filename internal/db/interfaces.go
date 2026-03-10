package db

import (
	"context"
	"database/sql"
	"time"
)

// APILogStore exposes API log persistence operations used by services.
type APILogStore interface {
	CreateAPILog(context.Context, CreateAPILogParams) (sql.Result, error)
	DeleteOldAPILogs(context.Context, time.Time) error
	GetAPILogStats(context.Context, GetAPILogStatsParams) (GetAPILogStatsRow, error)
	GetAPILogStatsByEndpoint(context.Context, GetAPILogStatsByEndpointParams) ([]GetAPILogStatsByEndpointRow, error)
	GetAPILogs(context.Context, GetAPILogsParams) ([]ApiLog, error)
	GetAPILogsByJobType(context.Context, GetAPILogsByJobTypeParams) ([]ApiLog, error)
	GetAPILogsByTimeRange(context.Context, GetAPILogsByTimeRangeParams) ([]ApiLog, error)
	GetAPILogsByUser(context.Context, GetAPILogsByUserParams) ([]ApiLog, error)
}

// UserTokenStore exposes OAuth token persistence operations used by services.
type UserTokenStore interface {
	CreateUserTokens(context.Context, CreateUserTokensParams) (sql.Result, error)
	DeleteUserTokens(context.Context, string) error
	GetUserToken(context.Context, string) (UserToken, error)
	ListUserTokens(context.Context) ([]UserToken, error)
	UpdateUserTokens(context.Context, UpdateUserTokensParams) (sql.Result, error)
}

// HealthStore exposes DB health and pool stats operations.
type HealthStore interface {
	PingContext(context.Context) error
	Stats() (sql.DBStats, error)
}

// Database is the root DB abstraction used by app/services/controllers.
type Database interface {
	APILogStore
	UserTokenStore
	HealthStore
	WithTx(*sql.Tx) Database
}
