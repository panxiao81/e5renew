package db

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	mydb "github.com/panxiao81/e5renew/internal/db/mysql"
	pgdb "github.com/panxiao81/e5renew/internal/db/postgres"
)

type healthStore struct {
	db DBTX
}

type mysqlAdapter struct {
	q *mydb.Queries
}

func (h *healthStore) PingContext(ctx context.Context) error {
	pinger, ok := h.db.(interface {
		PingContext(context.Context) error
	})
	if !ok {
		return fmt.Errorf("underlying DB does not implement PingContext")
	}
	return pinger.PingContext(ctx)
}

func (h *healthStore) Stats() (sql.DBStats, error) {
	statser, ok := h.db.(interface {
		Stats() sql.DBStats
	})
	if !ok {
		return sql.DBStats{}, fmt.Errorf("underlying DB does not expose Stats")
	}
	return statser.Stats(), nil
}

func newStores(engine Engine, db DBTX) (apiLogStore, userTokenStore, HealthStore) {
	var apilog apiLogStore
	var tokens userTokenStore

	switch engine {
	case EngineMySQL:
		mysqlQueries := mydb.New(db)
		adapter := &mysqlAdapter{q: mysqlQueries}
		apilog = adapter
		tokens = adapter
	default:
		pgQueries := pgdb.New(db)
		apilog = pgQueries
		tokens = pgQueries
	}

	return apilog, tokens, &healthStore{db: db}
}

func (m *mysqlAdapter) CreateAPILog(ctx context.Context, arg CreateAPILogParams) (sql.Result, error) {
	return m.q.CreateAPILog(ctx, mydb.CreateAPILogParams(arg))
}

func (m *mysqlAdapter) CreateUserTokens(ctx context.Context, arg CreateUserTokensParams) (sql.Result, error) {
	return m.q.CreateUserTokens(ctx, mydb.CreateUserTokensParams(arg))
}

func (m *mysqlAdapter) DeleteOldAPILogs(ctx context.Context, requestTime time.Time) error {
	return m.q.DeleteOldAPILogs(ctx, requestTime)
}

func (m *mysqlAdapter) DeleteUserTokens(ctx context.Context, userID string) error {
	return m.q.DeleteUserTokens(ctx, userID)
}

func (m *mysqlAdapter) GetAPILogStats(ctx context.Context, arg GetAPILogStatsParams) (GetAPILogStatsRow, error) {
	row, err := m.q.GetAPILogStats(ctx, mydb.GetAPILogStatsParams(arg))
	if err != nil {
		return GetAPILogStatsRow{}, err
	}
	return GetAPILogStatsRow{
		TotalRequests:      row.TotalRequests,
		SuccessfulRequests: row.SuccessfulRequests,
		FailedRequests:     row.FailedRequests,
		AvgDurationMs:      asFloat64(row.AvgDurationMs),
		MinDurationMs:      row.MinDurationMs,
		MaxDurationMs:      row.MaxDurationMs,
	}, nil
}

func (m *mysqlAdapter) GetAPILogStatsByEndpoint(ctx context.Context, arg GetAPILogStatsByEndpointParams) ([]GetAPILogStatsByEndpointRow, error) {
	rows, err := m.q.GetAPILogStatsByEndpoint(ctx, mydb.GetAPILogStatsByEndpointParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]GetAPILogStatsByEndpointRow, len(rows))
	for i := range rows {
		result[i] = GetAPILogStatsByEndpointRow{
			ApiEndpoint:        rows[i].ApiEndpoint,
			TotalRequests:      rows[i].TotalRequests,
			SuccessfulRequests: rows[i].SuccessfulRequests,
			FailedRequests:     rows[i].FailedRequests,
			AvgDurationMs:      asFloat64(rows[i].AvgDurationMs),
		}
	}
	return result, nil
}

func (m *mysqlAdapter) GetAPILogs(ctx context.Context, arg GetAPILogsParams) ([]ApiLog, error) {
	rows, err := m.q.GetAPILogs(ctx, mydb.GetAPILogsParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]ApiLog, len(rows))
	for i := range rows {
		result[i] = ApiLog(rows[i])
	}
	return result, nil
}

func (m *mysqlAdapter) GetAPILogsByJobType(ctx context.Context, arg GetAPILogsByJobTypeParams) ([]ApiLog, error) {
	rows, err := m.q.GetAPILogsByJobType(ctx, mydb.GetAPILogsByJobTypeParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]ApiLog, len(rows))
	for i := range rows {
		result[i] = ApiLog(rows[i])
	}
	return result, nil
}

func (m *mysqlAdapter) GetAPILogsByTimeRange(ctx context.Context, arg GetAPILogsByTimeRangeParams) ([]ApiLog, error) {
	rows, err := m.q.GetAPILogsByTimeRange(ctx, mydb.GetAPILogsByTimeRangeParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]ApiLog, len(rows))
	for i := range rows {
		result[i] = ApiLog(rows[i])
	}
	return result, nil
}

func (m *mysqlAdapter) GetAPILogsByUser(ctx context.Context, arg GetAPILogsByUserParams) ([]ApiLog, error) {
	rows, err := m.q.GetAPILogsByUser(ctx, mydb.GetAPILogsByUserParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]ApiLog, len(rows))
	for i := range rows {
		result[i] = ApiLog(rows[i])
	}
	return result, nil
}

func (m *mysqlAdapter) GetUserToken(ctx context.Context, userID string) (UserToken, error) {
	token, err := m.q.GetUserToken(ctx, userID)
	return UserToken(token), err
}

func (m *mysqlAdapter) ListUserTokens(ctx context.Context) ([]UserToken, error) {
	tokens, err := m.q.ListUserTokens(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]UserToken, len(tokens))
	for i := range tokens {
		result[i] = UserToken(tokens[i])
	}
	return result, nil
}

func (m *mysqlAdapter) UpdateUserTokens(ctx context.Context, arg UpdateUserTokensParams) (sql.Result, error) {
	return m.q.UpdateUserTokens(ctx, mydb.UpdateUserTokensParams(arg))
}

func asFloat64(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case int:
		return float64(v)
	case []byte:
		f, _ := strconv.ParseFloat(string(v), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		f, _ := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		return f
	}
}
