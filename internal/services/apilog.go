package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/middleware"
	"github.com/panxiao81/e5renew/internal/repository"
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
	repo   repository.APILogRepository
	logger *slog.Logger
}

// NewAPILogService creates a new APILogService instance
func NewAPILogService(repo repository.APILogRepository, logger *slog.Logger) *APILogService {
	return &APILogService{
		repo:   repo,
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
	// Convert to database parameters
	var userID sql.NullString
	if entry.UserID != nil {
		userID = sql.NullString{String: *entry.UserID, Valid: true}
	}

	var errorMessage sql.NullString
	if entry.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *entry.ErrorMessage, Valid: true}
	}

	err := s.repo.CreateAPILog(ctx, repository.APILogEntry{
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
	logs, err := s.repo.GetAPILogs(ctx, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to get API logs: %w", err)
	}
	return toDBLogs(logs), nil
}

// GetAPILogsByUser retrieves API logs for a specific user
func (s *APILogService) GetAPILogsByUser(ctx context.Context, userID string, limit, offset int32) ([]db.ApiLog, error) {
	logs, err := s.repo.GetAPILogsByUser(ctx, userID, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to get API logs by user: %w", err)
	}
	return toDBLogs(logs), nil
}

// GetAPILogsByTimeRange retrieves API logs within a time range
func (s *APILogService) GetAPILogsByTimeRange(ctx context.Context, start, end time.Time, limit, offset int32) ([]db.ApiLog, error) {
	logs, err := s.repo.GetAPILogsByTimeRange(ctx, start, end, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to get API logs by time range: %w", err)
	}
	return toDBLogs(logs), nil
}

// GetAPILogsByJobType retrieves API logs by job type
func (s *APILogService) GetAPILogsByJobType(ctx context.Context, jobType string, limit, offset int32) ([]db.ApiLog, error) {
	logs, err := s.repo.GetAPILogsByJobType(ctx, jobType, limit, offset)

	if err != nil {
		return nil, fmt.Errorf("failed to get API logs by job type: %w", err)
	}
	return toDBLogs(logs), nil
}

// GetAPILogStats retrieves aggregated statistics
func (s *APILogService) GetAPILogStats(ctx context.Context, start, end time.Time) (*APILogStats, error) {
	stats, err := s.repo.GetAPILogStats(ctx, start, end)

	if err != nil {
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

	return result, nil
}

// GetAPILogStatsByEndpoint retrieves statistics grouped by endpoint
func (s *APILogService) GetAPILogStatsByEndpoint(ctx context.Context, start, end time.Time) ([]APILogEndpointStats, error) {
	stats, err := s.repo.GetAPILogStatsByEndpoint(ctx, start, end)

	if err != nil {
		return nil, fmt.Errorf("failed to get API log stats by endpoint: %w", err)
	}

	result := make([]APILogEndpointStats, len(stats))
	for i, stat := range stats {
		result[i] = APILogEndpointStats{
			APIEndpoint:        stat.APIEndpoint,
			TotalRequests:      stat.TotalRequests,
			SuccessfulRequests: stat.SuccessfulRequests,
			FailedRequests:     stat.FailedRequests,
			AvgDurationMs:      stat.AvgDurationMs,
		}
	}

	return result, nil
}

// DeleteOldAPILogs removes API logs older than the specified time
func (s *APILogService) DeleteOldAPILogs(ctx context.Context, before time.Time) error {
	err := s.repo.DeleteOldAPILogs(ctx, before)
	if err != nil {
		return fmt.Errorf("failed to delete old API logs: %w", err)
	}

	s.logger.Info("Deleted old API logs", "before", before)
	return nil
}

func toDBLogs(logs []repository.APILog) []db.ApiLog {
	result := make([]db.ApiLog, len(logs))
	for i, log := range logs {
		result[i] = db.ApiLog(log)
	}
	return result
}
