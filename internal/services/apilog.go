package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/middleware"
)

// APILogEntry represents a logged API call
type APILogEntry struct {
	UserID         *string   `json:"user_id,omitempty"`
	APIEndpoint    string    `json:"api_endpoint"`
	HTTPMethod     string    `json:"http_method"`
	HTTPStatusCode int       `json:"http_status_code"`
	RequestTime    time.Time `json:"request_time"`
	ResponseTime   time.Time `json:"response_time"`
	DurationMs     int       `json:"duration_ms"`
	RequestSize    int       `json:"request_size"`
	ResponseSize   int       `json:"response_size"`
	ErrorMessage   *string   `json:"error_message,omitempty"`
	JobType        string    `json:"job_type"`
	Success        bool      `json:"success"`
}

// APILogStats represents aggregated statistics
type APILogStats struct {
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`
	AvgDurationMs      float64 `json:"avg_duration_ms"`
	MinDurationMs      int32   `json:"min_duration_ms"`
	MaxDurationMs      int32   `json:"max_duration_ms"`
}

// APILogEndpointStats represents statistics per endpoint
type APILogEndpointStats struct {
	APIEndpoint        string  `json:"api_endpoint"`
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`
	AvgDurationMs      float64 `json:"avg_duration_ms"`
}

// APILogService handles API log operations
type APILogService struct {
	db     *db.Queries
	logger *slog.Logger
}

// NewAPILogService creates a new APILogService instance
func NewAPILogService(database *db.Queries, logger *slog.Logger) *APILogService {
	return &APILogService{
		db:     database,
		logger: logger,
	}
}

// LogAPICall logs an API call to the database (middleware.APILogger interface)
func (s *APILogService) LogAPICall(ctx context.Context, entry middleware.APILogEntry) error {
	// Convert middleware.APILogEntry to services.APILogEntry
	serviceEntry := APILogEntry{
		UserID:         entry.UserID,
		APIEndpoint:    entry.APIEndpoint,
		HTTPMethod:     entry.HTTPMethod,
		HTTPStatusCode: entry.HTTPStatusCode,
		RequestTime:    entry.RequestTime,
		ResponseTime:   entry.ResponseTime,
		DurationMs:     entry.DurationMs,
		RequestSize:    entry.RequestSize,
		ResponseSize:   entry.ResponseSize,
		ErrorMessage:   entry.ErrorMessage,
		JobType:        entry.JobType,
		Success:        entry.Success,
	}

	return s.logAPICallInternal(ctx, serviceEntry)
}

// logAPICallInternal logs an API call to the database (internal method)
func (s *APILogService) logAPICallInternal(ctx context.Context, entry APILogEntry) error {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "LogAPICall")
	defer span.End()

	span.SetAttributes(
		attribute.String("api_endpoint", entry.APIEndpoint),
		attribute.String("http_method", entry.HTTPMethod),
		attribute.Int("http_status_code", entry.HTTPStatusCode),
		attribute.String("job_type", entry.JobType),
		attribute.Bool("success", entry.Success),
		attribute.Int("duration_ms", entry.DurationMs),
	)

	if entry.UserID != nil {
		span.SetAttributes(attribute.String("user_id", *entry.UserID))
	}

	// Convert to database parameters
	var userID sql.NullString
	if entry.UserID != nil {
		userID = sql.NullString{String: *entry.UserID, Valid: true}
	}

	var errorMessage sql.NullString
	if entry.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *entry.ErrorMessage, Valid: true}
	}

	_, err := s.db.CreateAPILog(ctx, db.CreateAPILogParams{
		UserID:         userID,
		ApiEndpoint:    entry.APIEndpoint,
		HttpMethod:     entry.HTTPMethod,
		HttpStatusCode: int32(entry.HTTPStatusCode),
		RequestTime:    entry.RequestTime,
		ResponseTime:   entry.ResponseTime,
		DurationMs:     int32(entry.DurationMs),
		RequestSize:    sql.NullInt32{Int32: int32(entry.RequestSize), Valid: true},
		ResponseSize:   sql.NullInt32{Int32: int32(entry.ResponseSize), Valid: true},
		ErrorMessage:   errorMessage,
		JobType:        entry.JobType,
		Success:        entry.Success,
	})

	if err != nil {
		span.RecordError(err)
		s.logger.Error("Failed to log API call", "error", err, "endpoint", entry.APIEndpoint)
		return fmt.Errorf("failed to log API call: %w", err)
	}

	s.logger.Debug("API call logged successfully",
		"endpoint", entry.APIEndpoint,
		"method", entry.HTTPMethod,
		"status", entry.HTTPStatusCode,
		"duration_ms", entry.DurationMs,
		"success", entry.Success)

	return nil
}

// GetAPILogs retrieves API logs with pagination
func (s *APILogService) GetAPILogs(ctx context.Context, limit, offset int32) ([]db.ApiLog, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetAPILogs")
	defer span.End()

	span.SetAttributes(
		attribute.Int("limit", int(limit)),
		attribute.Int("offset", int(offset)),
	)

	logs, err := s.db.GetAPILogs(ctx, db.GetAPILogsParams{
		Limit:  limit,
		Offset: offset,
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get API logs: %w", err)
	}

	span.SetAttributes(attribute.Int("logs_count", len(logs)))
	return logs, nil
}

// GetAPILogsByUser retrieves API logs for a specific user
func (s *APILogService) GetAPILogsByUser(ctx context.Context, userID string, limit, offset int32) ([]db.ApiLog, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetAPILogsByUser")
	defer span.End()

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.Int("limit", int(limit)),
		attribute.Int("offset", int(offset)),
	)

	logs, err := s.db.GetAPILogsByUser(ctx, db.GetAPILogsByUserParams{
		UserID: sql.NullString{String: userID, Valid: true},
		Limit:  limit,
		Offset: offset,
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get API logs by user: %w", err)
	}

	span.SetAttributes(attribute.Int("logs_count", len(logs)))
	return logs, nil
}

// GetAPILogsByTimeRange retrieves API logs within a time range
func (s *APILogService) GetAPILogsByTimeRange(ctx context.Context, start, end time.Time, limit, offset int32) ([]db.ApiLog, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetAPILogsByTimeRange")
	defer span.End()

	span.SetAttributes(
		attribute.String("start_time", start.Format(time.RFC3339)),
		attribute.String("end_time", end.Format(time.RFC3339)),
		attribute.Int("limit", int(limit)),
		attribute.Int("offset", int(offset)),
	)

	logs, err := s.db.GetAPILogsByTimeRange(ctx, db.GetAPILogsByTimeRangeParams{
		RequestTime:   start,
		RequestTime_2: end,
		Limit:         limit,
		Offset:        offset,
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get API logs by time range: %w", err)
	}

	span.SetAttributes(attribute.Int("logs_count", len(logs)))
	return logs, nil
}

// GetAPILogsByJobType retrieves API logs by job type
func (s *APILogService) GetAPILogsByJobType(ctx context.Context, jobType string, limit, offset int32) ([]db.ApiLog, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetAPILogsByJobType")
	defer span.End()

	span.SetAttributes(
		attribute.String("job_type", jobType),
		attribute.Int("limit", int(limit)),
		attribute.Int("offset", int(offset)),
	)

	logs, err := s.db.GetAPILogsByJobType(ctx, db.GetAPILogsByJobTypeParams{
		JobType: jobType,
		Limit:   limit,
		Offset:  offset,
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get API logs by job type: %w", err)
	}

	span.SetAttributes(attribute.Int("logs_count", len(logs)))
	return logs, nil
}

// GetAPILogStats retrieves aggregated statistics
func (s *APILogService) GetAPILogStats(ctx context.Context, start, end time.Time) (*APILogStats, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetAPILogStats")
	defer span.End()

	span.SetAttributes(
		attribute.String("start_time", start.Format(time.RFC3339)),
		attribute.String("end_time", end.Format(time.RFC3339)),
	)

	stats, err := s.db.GetAPILogStats(ctx, db.GetAPILogStatsParams{
		RequestTime:   start,
		RequestTime_2: end,
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get API log stats: %w", err)
	}

	avgDuration := stats.AvgDurationMs

	var minDuration, maxDuration int32
	if stats.MinDurationMs != nil {
		if val, ok := stats.MinDurationMs.(int32); ok {
			minDuration = val
		}
	}
	if stats.MaxDurationMs != nil {
		if val, ok := stats.MaxDurationMs.(int32); ok {
			maxDuration = val
		}
	}

	result := &APILogStats{
		TotalRequests:      stats.TotalRequests,
		SuccessfulRequests: stats.SuccessfulRequests,
		FailedRequests:     stats.FailedRequests,
		AvgDurationMs:      avgDuration,
		MinDurationMs:      minDuration,
		MaxDurationMs:      maxDuration,
	}

	span.SetAttributes(
		attribute.Int64("total_requests", result.TotalRequests),
		attribute.Int64("successful_requests", result.SuccessfulRequests),
		attribute.Int64("failed_requests", result.FailedRequests),
		attribute.Float64("avg_duration_ms", result.AvgDurationMs),
	)

	return result, nil
}

// GetAPILogStatsByEndpoint retrieves statistics grouped by endpoint
func (s *APILogService) GetAPILogStatsByEndpoint(ctx context.Context, start, end time.Time) ([]APILogEndpointStats, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetAPILogStatsByEndpoint")
	defer span.End()

	span.SetAttributes(
		attribute.String("start_time", start.Format(time.RFC3339)),
		attribute.String("end_time", end.Format(time.RFC3339)),
	)

	stats, err := s.db.GetAPILogStatsByEndpoint(ctx, db.GetAPILogStatsByEndpointParams{
		RequestTime:   start,
		RequestTime_2: end,
	})

	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get API log stats by endpoint: %w", err)
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

	span.SetAttributes(attribute.Int("endpoints_count", len(result)))
	return result, nil
}

// DeleteOldAPILogs removes API logs older than the specified time
func (s *APILogService) DeleteOldAPILogs(ctx context.Context, before time.Time) error {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "DeleteOldAPILogs")
	defer span.End()

	span.SetAttributes(
		attribute.String("before_time", before.Format(time.RFC3339)),
	)

	err := s.db.DeleteOldAPILogs(ctx, before)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete old API logs: %w", err)
	}

	s.logger.Info("Deleted old API logs", "before", before)
	return nil
}
