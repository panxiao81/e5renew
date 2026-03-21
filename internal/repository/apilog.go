package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/panxiao81/e5renew/internal/db"
)

type APILog struct {
	ID             int64
	UserID         sql.NullString
	ApiEndpoint    string
	HttpMethod     string
	HttpStatusCode int32
	RequestTime    time.Time
	ResponseTime   time.Time
	DurationMs     int32
	RequestSize    sql.NullInt32
	ResponseSize   sql.NullInt32
	ErrorMessage   sql.NullString
	JobType        string
	Success        bool
}

type APILogEntry struct {
	UserID         sql.NullString
	ApiEndpoint    string
	HttpMethod     string
	HttpStatusCode int32
	RequestTime    time.Time
	ResponseTime   time.Time
	DurationMs     int32
	RequestSize    sql.NullInt32
	ResponseSize   sql.NullInt32
	ErrorMessage   sql.NullString
	JobType        string
	Success        bool
}

type APILogStats struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AvgDurationMs      float64
	MinDurationMs      any
	MaxDurationMs      any
}

type APILogEndpointStats struct {
	APIEndpoint        string
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AvgDurationMs      float64
}

type APILogRepository interface {
	CreateAPILog(context.Context, APILogEntry) error
	DeleteOldAPILogs(context.Context, time.Time) error
	GetAPILogStats(context.Context, time.Time, time.Time) (*APILogStats, error)
	GetAPILogStatsByEndpoint(context.Context, time.Time, time.Time) ([]APILogEndpointStats, error)
	GetAPILogs(context.Context, int32, int32) ([]APILog, error)
	GetAPILogsByJobType(context.Context, string, int32, int32) ([]APILog, error)
	GetAPILogsByTimeRange(context.Context, time.Time, time.Time, int32, int32) ([]APILog, error)
	GetAPILogsByUser(context.Context, string, int32, int32) ([]APILog, error)
}

type apilogRepository struct {
	store apilogStore
}

func NewAPILogRepositoryWithEngine(engine db.Engine, conn db.DBTX) APILogRepository {
	return NewAPILogRepository(newAPILogStore(engine, conn))
}

func NewAPILogRepository(store apilogStore) APILogRepository {
	return &apilogRepository{store: store}
}

func (r *apilogRepository) CreateAPILog(ctx context.Context, entry APILogEntry) error {
	_, err := r.store.CreateAPILog(ctx, db.CreateAPILogParams{
		UserID:         entry.UserID,
		ApiEndpoint:    entry.ApiEndpoint,
		HttpMethod:     entry.HttpMethod,
		HttpStatusCode: entry.HttpStatusCode,
		RequestTime:    entry.RequestTime,
		ResponseTime:   entry.ResponseTime,
		DurationMs:     entry.DurationMs,
		RequestSize:    entry.RequestSize,
		ResponseSize:   entry.ResponseSize,
		ErrorMessage:   entry.ErrorMessage,
		JobType:        entry.JobType,
		Success:        entry.Success,
	})
	return err
}

func (r *apilogRepository) DeleteOldAPILogs(ctx context.Context, before time.Time) error {
	return r.store.DeleteOldAPILogs(ctx, before)
}

func (r *apilogRepository) GetAPILogStats(ctx context.Context, start, end time.Time) (*APILogStats, error) {
	stats, err := r.store.GetAPILogStats(ctx, db.GetAPILogStatsParams{RequestTime: start, RequestTime_2: end})
	if err != nil {
		return nil, err
	}
	return &APILogStats{
		TotalRequests:      stats.TotalRequests,
		SuccessfulRequests: stats.SuccessfulRequests,
		FailedRequests:     stats.FailedRequests,
		AvgDurationMs:      stats.AvgDurationMs,
		MinDurationMs:      stats.MinDurationMs,
		MaxDurationMs:      stats.MaxDurationMs,
	}, nil
}

func (r *apilogRepository) GetAPILogStatsByEndpoint(ctx context.Context, start, end time.Time) ([]APILogEndpointStats, error) {
	stats, err := r.store.GetAPILogStatsByEndpoint(ctx, db.GetAPILogStatsByEndpointParams{RequestTime: start, RequestTime_2: end})
	if err != nil {
		return nil, err
	}
	result := make([]APILogEndpointStats, len(stats))
	for i, stat := range stats {
		result[i] = APILogEndpointStats{
			APIEndpoint:        stat.ApiEndpoint,
			TotalRequests:      stat.TotalRequests,
			SuccessfulRequests: stat.SuccessfulRequests,
			FailedRequests:     stat.FailedRequests,
			AvgDurationMs:      stat.AvgDurationMs,
		}
	}
	return result, nil
}

func (r *apilogRepository) GetAPILogs(ctx context.Context, limit, offset int32) ([]APILog, error) {
	logs, err := r.store.GetAPILogs(ctx, db.GetAPILogsParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	return toRepoLogs(logs), nil
}

func (r *apilogRepository) GetAPILogsByJobType(ctx context.Context, jobType string, limit, offset int32) ([]APILog, error) {
	logs, err := r.store.GetAPILogsByJobType(ctx, db.GetAPILogsByJobTypeParams{JobType: jobType, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	return toRepoLogs(logs), nil
}

func (r *apilogRepository) GetAPILogsByTimeRange(ctx context.Context, start, end time.Time, limit, offset int32) ([]APILog, error) {
	logs, err := r.store.GetAPILogsByTimeRange(ctx, db.GetAPILogsByTimeRangeParams{RequestTime: start, RequestTime_2: end, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	return toRepoLogs(logs), nil
}

func (r *apilogRepository) GetAPILogsByUser(ctx context.Context, userID string, limit, offset int32) ([]APILog, error) {
	logs, err := r.store.GetAPILogsByUser(ctx, db.GetAPILogsByUserParams{UserID: sql.NullString{String: userID, Valid: true}, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	return toRepoLogs(logs), nil
}

func toRepoLogs(logs []db.ApiLog) []APILog {
	result := make([]APILog, len(logs))
	for i, log := range logs {
		result[i] = APILog(log)
	}
	return result
}
