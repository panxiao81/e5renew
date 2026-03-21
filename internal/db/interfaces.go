package db

import (
	"context"
	"database/sql"
	"time"
)

type apiLogStore interface {
	CreateAPILog(context.Context, CreateAPILogParams) (sql.Result, error)
	DeleteOldAPILogs(context.Context, time.Time) error
	GetAPILogStats(context.Context, GetAPILogStatsParams) (GetAPILogStatsRow, error)
	GetAPILogStatsByEndpoint(context.Context, GetAPILogStatsByEndpointParams) ([]GetAPILogStatsByEndpointRow, error)
	GetAPILogs(context.Context, GetAPILogsParams) ([]ApiLog, error)
	GetAPILogsByJobType(context.Context, GetAPILogsByJobTypeParams) ([]ApiLog, error)
	GetAPILogsByTimeRange(context.Context, GetAPILogsByTimeRangeParams) ([]ApiLog, error)
	GetAPILogsByUser(context.Context, GetAPILogsByUserParams) ([]ApiLog, error)
}

type userTokenStore interface {
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
