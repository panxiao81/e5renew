package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/panxiao81/e5renew/internal/db"
	mydb "github.com/panxiao81/e5renew/internal/db/mysql"
	pgdb "github.com/panxiao81/e5renew/internal/db/postgres"
)

type apilogStore interface {
	CreateAPILog(context.Context, db.CreateAPILogParams) (sql.Result, error)
	DeleteOldAPILogs(context.Context, time.Time) error
	GetAPILogStats(context.Context, db.GetAPILogStatsParams) (db.GetAPILogStatsRow, error)
	GetAPILogStatsByEndpoint(context.Context, db.GetAPILogStatsByEndpointParams) ([]db.GetAPILogStatsByEndpointRow, error)
	GetAPILogs(context.Context, db.GetAPILogsParams) ([]db.ApiLog, error)
	GetAPILogsByJobType(context.Context, db.GetAPILogsByJobTypeParams) ([]db.ApiLog, error)
	GetAPILogsByTimeRange(context.Context, db.GetAPILogsByTimeRangeParams) ([]db.ApiLog, error)
	GetAPILogsByUser(context.Context, db.GetAPILogsByUserParams) ([]db.ApiLog, error)
}

func newAPILogStore(engine db.Engine, conn db.DBTX) apilogStore {
	if engine == db.EngineMySQL {
		return &mysqlAPILogStore{q: mydb.New(conn)}
	}
	return &postgresAPILogStore{q: pgdb.New(conn)}
}

type postgresAPILogStore struct{ q *pgdb.Queries }

func (p *postgresAPILogStore) CreateAPILog(ctx context.Context, arg db.CreateAPILogParams) (sql.Result, error) {
	return p.q.CreateAPILog(ctx, pgdb.CreateAPILogParams(arg))
}
func (p *postgresAPILogStore) DeleteOldAPILogs(ctx context.Context, t time.Time) error {
	return p.q.DeleteOldAPILogs(ctx, t)
}
func (p *postgresAPILogStore) GetAPILogStats(ctx context.Context, arg db.GetAPILogStatsParams) (db.GetAPILogStatsRow, error) {
	row, err := p.q.GetAPILogStats(ctx, pgdb.GetAPILogStatsParams(arg))
	return db.GetAPILogStatsRow(row), err
}
func (p *postgresAPILogStore) GetAPILogStatsByEndpoint(ctx context.Context, arg db.GetAPILogStatsByEndpointParams) ([]db.GetAPILogStatsByEndpointRow, error) {
	rows, err := p.q.GetAPILogStatsByEndpoint(ctx, pgdb.GetAPILogStatsByEndpointParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.GetAPILogStatsByEndpointRow, len(rows))
	for i := range rows {
		result[i] = db.GetAPILogStatsByEndpointRow(rows[i])
	}
	return result, nil
}
func (p *postgresAPILogStore) GetAPILogs(ctx context.Context, arg db.GetAPILogsParams) ([]db.ApiLog, error) {
	rows, err := p.q.GetAPILogs(ctx, pgdb.GetAPILogsParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.ApiLog, len(rows))
	for i := range rows {
		result[i] = db.ApiLog(rows[i])
	}
	return result, nil
}
func (p *postgresAPILogStore) GetAPILogsByJobType(ctx context.Context, arg db.GetAPILogsByJobTypeParams) ([]db.ApiLog, error) {
	rows, err := p.q.GetAPILogsByJobType(ctx, pgdb.GetAPILogsByJobTypeParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.ApiLog, len(rows))
	for i := range rows {
		result[i] = db.ApiLog(rows[i])
	}
	return result, nil
}
func (p *postgresAPILogStore) GetAPILogsByTimeRange(ctx context.Context, arg db.GetAPILogsByTimeRangeParams) ([]db.ApiLog, error) {
	rows, err := p.q.GetAPILogsByTimeRange(ctx, pgdb.GetAPILogsByTimeRangeParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.ApiLog, len(rows))
	for i := range rows {
		result[i] = db.ApiLog(rows[i])
	}
	return result, nil
}
func (p *postgresAPILogStore) GetAPILogsByUser(ctx context.Context, arg db.GetAPILogsByUserParams) ([]db.ApiLog, error) {
	rows, err := p.q.GetAPILogsByUser(ctx, pgdb.GetAPILogsByUserParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.ApiLog, len(rows))
	for i := range rows {
		result[i] = db.ApiLog(rows[i])
	}
	return result, nil
}

type mysqlAPILogStore struct{ q *mydb.Queries }

func (m *mysqlAPILogStore) CreateAPILog(ctx context.Context, arg db.CreateAPILogParams) (sql.Result, error) {
	return m.q.CreateAPILog(ctx, mydb.CreateAPILogParams(arg))
}
func (m *mysqlAPILogStore) DeleteOldAPILogs(ctx context.Context, t time.Time) error {
	return m.q.DeleteOldAPILogs(ctx, t)
}
func (m *mysqlAPILogStore) GetAPILogStats(ctx context.Context, arg db.GetAPILogStatsParams) (db.GetAPILogStatsRow, error) {
	row, err := m.q.GetAPILogStats(ctx, mydb.GetAPILogStatsParams(arg))
	if err != nil {
		return db.GetAPILogStatsRow{}, err
	}
	return db.GetAPILogStatsRow{
		TotalRequests:      row.TotalRequests,
		SuccessfulRequests: row.SuccessfulRequests,
		FailedRequests:     row.FailedRequests,
		AvgDurationMs:      asFloat64(row.AvgDurationMs),
		MinDurationMs:      row.MinDurationMs,
		MaxDurationMs:      row.MaxDurationMs,
	}, nil
}
func (m *mysqlAPILogStore) GetAPILogStatsByEndpoint(ctx context.Context, arg db.GetAPILogStatsByEndpointParams) ([]db.GetAPILogStatsByEndpointRow, error) {
	rows, err := m.q.GetAPILogStatsByEndpoint(ctx, mydb.GetAPILogStatsByEndpointParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.GetAPILogStatsByEndpointRow, len(rows))
	for i := range rows {
		result[i] = db.GetAPILogStatsByEndpointRow{
			ApiEndpoint:        rows[i].ApiEndpoint,
			TotalRequests:      rows[i].TotalRequests,
			SuccessfulRequests: rows[i].SuccessfulRequests,
			FailedRequests:     rows[i].FailedRequests,
			AvgDurationMs:      asFloat64(rows[i].AvgDurationMs),
		}
	}
	return result, nil
}
func (m *mysqlAPILogStore) GetAPILogs(ctx context.Context, arg db.GetAPILogsParams) ([]db.ApiLog, error) {
	rows, err := m.q.GetAPILogs(ctx, mydb.GetAPILogsParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.ApiLog, len(rows))
	for i := range rows {
		result[i] = db.ApiLog(rows[i])
	}
	return result, nil
}
func (m *mysqlAPILogStore) GetAPILogsByJobType(ctx context.Context, arg db.GetAPILogsByJobTypeParams) ([]db.ApiLog, error) {
	rows, err := m.q.GetAPILogsByJobType(ctx, mydb.GetAPILogsByJobTypeParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.ApiLog, len(rows))
	for i := range rows {
		result[i] = db.ApiLog(rows[i])
	}
	return result, nil
}
func (m *mysqlAPILogStore) GetAPILogsByTimeRange(ctx context.Context, arg db.GetAPILogsByTimeRangeParams) ([]db.ApiLog, error) {
	rows, err := m.q.GetAPILogsByTimeRange(ctx, mydb.GetAPILogsByTimeRangeParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.ApiLog, len(rows))
	for i := range rows {
		result[i] = db.ApiLog(rows[i])
	}
	return result, nil
}
func (m *mysqlAPILogStore) GetAPILogsByUser(ctx context.Context, arg db.GetAPILogsByUserParams) ([]db.ApiLog, error) {
	rows, err := m.q.GetAPILogsByUser(ctx, mydb.GetAPILogsByUserParams(arg))
	if err != nil {
		return nil, err
	}
	result := make([]db.ApiLog, len(rows))
	for i := range rows {
		result[i] = db.ApiLog(rows[i])
	}
	return result, nil
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
