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

// ProcessUserMailTokensJob processes mail activity for all users with stored tokens
type ProcessUserMailTokensJob struct {
	mailService *services.MailService
	logger      *slog.Logger
}

// NewProcessUserMailTokensJob creates a new ProcessUserMailTokensJob
func NewProcessUserMailTokensJob(mailService *services.MailService, logger *slog.Logger) *ProcessUserMailTokensJob {
	return &ProcessUserMailTokensJob{
		mailService: mailService,
		logger:      logger,
	}
}

// Execute runs the mail token processing job
func (j *ProcessUserMailTokensJob) Execute(ctx context.Context) error {
	ctx, span := telemetry.StartSpan(ctx, "github.com/panxiao81/e5renew/jobs", "ProcessUserMailTokensJob.Execute")
	defer span.End()

	startTime := time.Now()

	j.logger.Info("🔄 Starting user mail tokens processing job",
		"job", "ProcessUserMailTokensJob",
		"startTime", startTime.Format(time.RFC3339),
		"type", "background_job",
		"description", "Processing mail activity for all users with stored tokens")

	// Process all user mail activity
	err := j.mailService.ProcessAllUserMailActivity(ctx)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		j.logger.Error("❌ User mail tokens processing job failed",
			"job", "ProcessUserMailTokensJob",
			"error", err,
			"duration", time.Since(startTime),
			"type", "background_job",
			"status", "failed")
		return err
	}

	executionDuration := time.Since(startTime)
	span.SetAttributes(attribute.Int64("job.duration_ms", executionDuration.Milliseconds()))

	j.logger.Info("🎉 User mail tokens processing job completed successfully",
		"job", "ProcessUserMailTokensJob",
		"executionDuration", executionDuration,
		"type", "background_job",
		"status", "completed",
		"description", "All user mail tokens processed successfully")

	return nil
}

// RegisterUserMailTokensJob registers the user mail tokens processing job with the scheduler
func (js *JobScheduler) RegisterUserMailTokensJob(mailService *services.MailService, logger *slog.Logger) error {
	_, span := telemetry.StartSpan(context.Background(), "github.com/panxiao81/e5renew/jobs", "RegisterUserMailTokensJob")
	defer span.End()

	job := NewProcessUserMailTokensJob(mailService, logger)

	// Schedule the job to run every 30 minutes
	_, err := js.NewJob(
		gocron.DurationJob(30*time.Minute),
		gocron.NewTask(job.Execute, context.Background()),
		gocron.WithName("process_user_mail_tokens"),
		gocron.WithTags("mail", "user_tokens", "e5_activity"),
	)

	if err != nil {
		telemetry.RecordSpanError(span, err)
		logger.Error("Failed to register ProcessUserMailTokensJob", "error", err)
		return err
	}

	logger.Info("✅ Successfully registered user mail tokens processing job",
		"job", "ProcessUserMailTokensJob",
		"schedule", "every 30 minutes",
		"type", "recurring_job",
		"tags", "mail,user_tokens,e5_activity")

	return nil
}
