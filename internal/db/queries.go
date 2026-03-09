package db

import (
	"context"
	"database/sql"
	"time"
)

func (q *Queries) CreateAPILog(ctx context.Context, arg CreateAPILogParams) (sql.Result, error) {
	return q.pg.CreateAPILog(ctx, arg)
}

func (q *Queries) CreateUserTokens(ctx context.Context, arg CreateUserTokensParams) (sql.Result, error) {
	return q.pg.CreateUserTokens(ctx, arg)
}

func (q *Queries) DeleteOldAPILogs(ctx context.Context, requestTime time.Time) error {
	return q.pg.DeleteOldAPILogs(ctx, requestTime)
}

func (q *Queries) DeleteUserTokens(ctx context.Context, userID string) error {
	return q.pg.DeleteUserTokens(ctx, userID)
}

func (q *Queries) GetAPILogStats(ctx context.Context, arg GetAPILogStatsParams) (GetAPILogStatsRow, error) {
	return q.pg.GetAPILogStats(ctx, arg)
}

func (q *Queries) GetAPILogStatsByEndpoint(ctx context.Context, arg GetAPILogStatsByEndpointParams) ([]GetAPILogStatsByEndpointRow, error) {
	return q.pg.GetAPILogStatsByEndpoint(ctx, arg)
}

func (q *Queries) GetAPILogs(ctx context.Context, arg GetAPILogsParams) ([]ApiLog, error) {
	return q.pg.GetAPILogs(ctx, arg)
}

func (q *Queries) GetAPILogsByJobType(ctx context.Context, arg GetAPILogsByJobTypeParams) ([]ApiLog, error) {
	return q.pg.GetAPILogsByJobType(ctx, arg)
}

func (q *Queries) GetAPILogsByTimeRange(ctx context.Context, arg GetAPILogsByTimeRangeParams) ([]ApiLog, error) {
	return q.pg.GetAPILogsByTimeRange(ctx, arg)
}

func (q *Queries) GetAPILogsByUser(ctx context.Context, arg GetAPILogsByUserParams) ([]ApiLog, error) {
	return q.pg.GetAPILogsByUser(ctx, arg)
}

func (q *Queries) GetUserToken(ctx context.Context, userID string) (UserToken, error) {
	return q.pg.GetUserToken(ctx, userID)
}

func (q *Queries) ListUserTokens(ctx context.Context) ([]UserToken, error) {
	return q.pg.ListUserTokens(ctx)
}

func (q *Queries) UpdateUserTokens(ctx context.Context, arg UpdateUserTokensParams) (sql.Result, error) {
	return q.pg.UpdateUserTokens(ctx, arg)
}

