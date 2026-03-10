package db

import (
	"context"
	"database/sql"
	"time"
)

func (q *Queries) CreateAPILog(ctx context.Context, arg CreateAPILogParams) (sql.Result, error) {
	return q.apilog.CreateAPILog(ctx, arg)
}

func (q *Queries) CreateUserTokens(ctx context.Context, arg CreateUserTokensParams) (sql.Result, error) {
	return q.tokens.CreateUserTokens(ctx, arg)
}

func (q *Queries) DeleteOldAPILogs(ctx context.Context, requestTime time.Time) error {
	return q.apilog.DeleteOldAPILogs(ctx, requestTime)
}

func (q *Queries) DeleteUserTokens(ctx context.Context, userID string) error {
	return q.tokens.DeleteUserTokens(ctx, userID)
}

func (q *Queries) GetAPILogStats(ctx context.Context, arg GetAPILogStatsParams) (GetAPILogStatsRow, error) {
	return q.apilog.GetAPILogStats(ctx, arg)
}

func (q *Queries) GetAPILogStatsByEndpoint(ctx context.Context, arg GetAPILogStatsByEndpointParams) ([]GetAPILogStatsByEndpointRow, error) {
	return q.apilog.GetAPILogStatsByEndpoint(ctx, arg)
}

func (q *Queries) GetAPILogs(ctx context.Context, arg GetAPILogsParams) ([]ApiLog, error) {
	return q.apilog.GetAPILogs(ctx, arg)
}

func (q *Queries) GetAPILogsByJobType(ctx context.Context, arg GetAPILogsByJobTypeParams) ([]ApiLog, error) {
	return q.apilog.GetAPILogsByJobType(ctx, arg)
}

func (q *Queries) GetAPILogsByTimeRange(ctx context.Context, arg GetAPILogsByTimeRangeParams) ([]ApiLog, error) {
	return q.apilog.GetAPILogsByTimeRange(ctx, arg)
}

func (q *Queries) GetAPILogsByUser(ctx context.Context, arg GetAPILogsByUserParams) ([]ApiLog, error) {
	return q.apilog.GetAPILogsByUser(ctx, arg)
}

func (q *Queries) GetUserToken(ctx context.Context, userID string) (UserToken, error) {
	return q.tokens.GetUserToken(ctx, userID)
}

func (q *Queries) ListUserTokens(ctx context.Context) ([]UserToken, error) {
	return q.tokens.ListUserTokens(ctx)
}

func (q *Queries) UpdateUserTokens(ctx context.Context, arg UpdateUserTokensParams) (sql.Result, error) {
	return q.tokens.UpdateUserTokens(ctx, arg)
}
