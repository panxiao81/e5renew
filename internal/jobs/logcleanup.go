package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

// LogCleanupJob cleans up old API logs to maintain database size
type LogCleanupJob struct {
	apiLogService *services.APILogService
	logger        *slog.Logger
	retentionDays int
}

// NewLogCleanupJob creates a new LogCleanupJob
func NewLogCleanupJob(apiLogService *services.APILogService, logger *slog.Logger, retentionDays int) *LogCleanupJob {
	if retentionDays <= 0 {
		retentionDays = 30 // Default to 30 days retention
	}

	return &LogCleanupJob{
		apiLogService: apiLogService,
		logger:        logger,
		retentionDays: retentionDays,
	}
}

// Execute runs the log cleanup job
func (j *LogCleanupJob) Execute(ctx context.Context) error {
	ctx, span := telemetry.StartSpan(ctx, "github.com/panxiao81/e5renew/jobs", "LogCleanupJob.Execute")
	defer span.End()

	startTime := time.Now()
	cutoffTime := startTime.Add(-time.Duration(j.retentionDays) * 24 * time.Hour)

	j.logger.Info("🧹 Starting log cleanup job",
		"job", "LogCleanupJob",
		"retentionDays", j.retentionDays,
		"cutoffTime", cutoffTime.Format(time.RFC3339),
		"type", "maintenance_job",
		"description", "Cleaning up old API logs to maintain database size")

	// Delete old API logs
	err := j.apiLogService.DeleteOldAPILogs(ctx, cutoffTime)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		j.logger.Error("❌ Log cleanup job failed",
			"job", "LogCleanupJob",
			"error", err,
			"retentionDays", j.retentionDays,
			"type", "maintenance_job",
			"status", "failed")
		return err
	}

	executionDuration := time.Since(startTime)
	span.SetAttributes(attribute.Int64("job.duration_ms", executionDuration.Milliseconds()))

	j.logger.Info("🎉 Log cleanup job completed successfully",
		"job", "LogCleanupJob",
		"executionDuration", executionDuration,
		"cutoffTime", cutoffTime.Format(time.RFC3339),
		"retentionDays", j.retentionDays,
		"type", "maintenance_job",
		"status", "completed")

	return nil
}

// RegisterLogCleanupJob registers the log cleanup job with the scheduler
func (js *JobScheduler) RegisterLogCleanupJob(apiLogService *services.APILogService, logger *slog.Logger, retentionDays int) error {
	_, span := telemetry.StartSpan(context.Background(), "github.com/panxiao81/e5renew/jobs", "RegisterLogCleanupJob")
	defer span.End()

	job := NewLogCleanupJob(apiLogService, logger, retentionDays)

	// Schedule the job to run daily at 2:00 AM
	_, err := js.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(2, 0, 0))),
		gocron.NewTask(job.Execute, context.Background()),
		gocron.WithName("log_cleanup"),
		gocron.WithTags("maintenance", "cleanup", "database"),
	)

	if err != nil {
		telemetry.RecordSpanError(span, err)
		logger.Error("Failed to register LogCleanupJob", "error", err)
		return err
	}

	logger.Info("✅ Successfully registered log cleanup job",
		"job", "LogCleanupJob",
		"schedule", "daily at 2:00 AM",
		"retentionDays", retentionDays,
		"type", "maintenance_job",
		"tags", "maintenance,cleanup,database")

	return nil
}
