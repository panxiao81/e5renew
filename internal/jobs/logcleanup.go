package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/panxiao81/e5renew/internal/services"
	"go.opentelemetry.io/otel"
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
	tracer := otel.Tracer("github.com/panxiao81/e5renew/jobs")
	ctx, span := tracer.Start(ctx, "LogCleanupJob.Execute")
	defer span.End()

	startTime := time.Now()
	cutoffTime := startTime.Add(-time.Duration(j.retentionDays) * 24 * time.Hour)

	span.SetAttributes(
		attribute.String("job.name", "log_cleanup"),
		attribute.String("job.type", "maintenance"),
		attribute.Int("retention_days", j.retentionDays),
		attribute.String("cutoff_time", cutoffTime.Format(time.RFC3339)),
		attribute.String("job.execution_time", startTime.Format(time.RFC3339)),
	)

	j.logger.Info("🧹 Starting log cleanup job",
		"job", "LogCleanupJob",
		"retentionDays", j.retentionDays,
		"cutoffTime", cutoffTime.Format(time.RFC3339),
		"type", "maintenance_job",
		"description", "Cleaning up old API logs to maintain database size")

	// Delete old API logs
	err := j.apiLogService.DeleteOldAPILogs(ctx, cutoffTime)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("job.success", false),
			attribute.String("job.error", err.Error()),
		)
		j.logger.Error("❌ Log cleanup job failed",
			"job", "LogCleanupJob",
			"error", err,
			"retentionDays", j.retentionDays,
			"type", "maintenance_job",
			"status", "failed")
		return err
	}

	executionDuration := time.Since(startTime)

	span.SetAttributes(
		attribute.Bool("job.success", true),
		attribute.Int64("job.duration_ms", executionDuration.Milliseconds()),
	)

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
	tracer := otel.Tracer("github.com/panxiao81/e5renew/jobs")
	_, span := tracer.Start(context.Background(), "RegisterLogCleanupJob")
	defer span.End()

	job := NewLogCleanupJob(apiLogService, logger, retentionDays)

	span.SetAttributes(
		attribute.String("job.name", "log_cleanup"),
		attribute.String("job.schedule", "daily at 2:00 AM"),
		attribute.String("job.type", "maintenance"),
		attribute.Int("retention_days", retentionDays),
	)

	// Schedule the job to run daily at 2:00 AM
	_, err := js.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(2, 0, 0))),
		gocron.NewTask(job.Execute, context.Background()),
		gocron.WithName("log_cleanup"),
		gocron.WithTags("maintenance", "cleanup", "database"),
	)

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("job.registration_success", false))
		logger.Error("Failed to register LogCleanupJob", "error", err)
		return err
	}

	span.SetAttributes(attribute.Bool("job.registration_success", true))
	logger.Info("✅ Successfully registered log cleanup job",
		"job", "LogCleanupJob",
		"schedule", "daily at 2:00 AM",
		"retentionDays", retentionDays,
		"type", "maintenance_job",
		"tags", "maintenance,cleanup,database")

	return nil
}
