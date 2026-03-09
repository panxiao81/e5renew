package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/panxiao81/e5renew/internal/middleware"
)

// MailMessage represents a simplified Microsoft Graph mail message
type MailMessage struct {
	ID         string `json:"id"`
	Subject    string `json:"subject"`
	ReceivedAt string `json:"receivedDateTime"`
	IsRead     bool   `json:"isRead"`
	From       struct {
		EmailAddress struct {
			Address string `json:"address"`
			Name    string `json:"name"`
		} `json:"emailAddress"`
	} `json:"from"`
}

// MailResponse represents the response from Microsoft Graph API
type MailResponse struct {
	Value []MailMessage `json:"value"`
}

// MailService handles Microsoft Graph API mail operations
type MailService struct {
	userTokenService *UserTokenService
	httpClient       *http.Client
	logger           *slog.Logger
	apiLogService    *APILogService
}

// NewMailService creates a new MailService instance
func NewMailService(userTokenService *UserTokenService, apiLogService *APILogService, logger *slog.Logger) *MailService {
	return &MailService{
		userTokenService: userTokenService,
		httpClient:       &http.Client{Timeout: 30 * time.Second},
		logger:           logger,
		apiLogService:    apiLogService,
	}
}

// GetUserMail retrieves mail messages for a specific user using Microsoft Graph SDK
func (s *MailService) GetUserMail(ctx context.Context, userID string) (*MailResponse, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetUserMail")
	defer span.End()

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.String("api_endpoint", "graph.microsoft.com/v1.0/me/messages"),
	)

	// Create custom token credential
	credential := NewDatabaseTokenCredential(userID, s.userTokenService, s.logger)

	// Create Graph client with custom credential
	graphServiceClient, err := msgraphsdkgo.NewGraphServiceClientWithCredentials(
		credential,
		[]string{"https://graph.microsoft.com/Mail.Read"},
	)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", "failed_to_create_graph_client"))
		return nil, fmt.Errorf("failed to create Graph client: %w", err)
	}

	// Manual API logging for Graph SDK calls (SDK doesn't support custom HTTP clients)
	apiStartTime := time.Now()

	span.SetAttributes(attribute.String("graph_operation", "me.messages.get"))

	// Make request to Microsoft Graph API
	messages, err := graphServiceClient.Me().Messages().Get(ctx, nil)

	// Log the API call manually for now
	apiEndTime := time.Now()
	apiDuration := apiEndTime.Sub(apiStartTime)
	s.logGraphAPICall(ctx, userID, "me/messages", "GET", apiStartTime, apiEndTime, apiDuration, err == nil, err)

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", "graph_api_request_failed"))
		return nil, fmt.Errorf("failed to get messages from Graph API: %w", err)
	}

	// Convert Graph API response to our response format
	mailResponse := s.convertGraphMessagesToMailResponse(messages)

	span.SetAttributes(
		attribute.Int("messages_count", len(mailResponse.Value)),
		attribute.Bool("success", true),
	)

	s.logger.Debug("Successfully retrieved user mail via Graph SDK",
		"userID", userID,
		"messageCount", len(mailResponse.Value))

	return mailResponse, nil
}

// logGraphAPICall logs Graph API calls to database (manual logging for SDK calls)
func (s *MailService) logGraphAPICall(_ context.Context, userID, endpoint, method string, startTime, endTime time.Time, duration time.Duration, success bool, err error) {
	apiLogEntry := middleware.APILogEntry{
		UserID:         &userID,
		APIEndpoint:    endpoint,
		HTTPMethod:     method,
		RequestTime:    startTime,
		ResponseTime:   endTime,
		DurationMs:     int(duration.Milliseconds()),
		RequestSize:    0, // Graph SDK doesn't expose request size
		ResponseSize:   0, // Graph SDK doesn't expose response size
		JobType:        "user_mail",
		HTTPStatusCode: 200, // Default to success
		Success:        success,
	}

	if err != nil {
		apiLogEntry.HTTPStatusCode = 500
		errorMsg := err.Error()
		apiLogEntry.ErrorMessage = &errorMsg
	}

	// Log API call asynchronously
	go func() {
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if logErr := s.apiLogService.LogAPICall(logCtx, apiLogEntry); logErr != nil {
			s.logger.Error("Failed to log Graph API call", "error", logErr)
		}
	}()
}

// convertGraphMessagesToMailResponse converts Graph SDK messages to our MailResponse format
func (s *MailService) convertGraphMessagesToMailResponse(messages models.MessageCollectionResponseable) *MailResponse {
	if messages == nil {
		return &MailResponse{Value: []MailMessage{}}
	}

	graphMessages := messages.GetValue()
	if graphMessages == nil {
		return &MailResponse{Value: []MailMessage{}}
	}

	mailMessages := make([]MailMessage, 0, len(graphMessages))
	for _, graphMsg := range graphMessages {
		if graphMsg == nil {
			continue
		}

		mailMsg := MailMessage{
			ID:      getStringValue(graphMsg.GetId()),
			Subject: getStringValue(graphMsg.GetSubject()),
			IsRead:  getBoolValue(graphMsg.GetIsRead()),
		}

		// Convert received date time
		if receivedDateTime := graphMsg.GetReceivedDateTime(); receivedDateTime != nil {
			mailMsg.ReceivedAt = receivedDateTime.Format(time.RFC3339)
		}

		// Convert sender information
		if sender := graphMsg.GetSender(); sender != nil {
			if emailAddress := sender.GetEmailAddress(); emailAddress != nil {
				mailMsg.From.EmailAddress.Address = getStringValue(emailAddress.GetAddress())
				mailMsg.From.EmailAddress.Name = getStringValue(emailAddress.GetName())
			}
		}

		mailMessages = append(mailMessages, mailMsg)
	}

	return &MailResponse{Value: mailMessages}
}

// Helper functions for safely extracting values from Graph SDK models
func getStringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func getBoolValue(ptr *bool) bool {
	if ptr == nil {
		return false
	}
	return *ptr
}

// ProcessUserMailActivity processes mail activity for a user to maintain E5 activity
func (s *MailService) ProcessUserMailActivity(ctx context.Context, userID string) error {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "ProcessUserMailActivity")
	defer span.End()

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.String("activity_type", "mail_read"),
	)

	startTime := time.Now()

	// Get user mail to simulate activity
	mailResponse, err := s.GetUserMail(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("success", false))
		s.logger.Error("Failed to process mail activity",
			"userID", userID,
			"error", err)
		return fmt.Errorf("failed to process mail activity for user %s: %w", userID, err)
	}

	processingDuration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int("messages_processed", len(mailResponse.Value)),
		attribute.Int64("processing_duration_ms", processingDuration.Milliseconds()),
		attribute.Bool("success", true),
	)

	s.logger.Info("✅ Successfully processed mail activity for user",
		"userID", userID,
		"messagesProcessed", len(mailResponse.Value),
		"processingDuration", processingDuration,
		"activity", "mail_read",
		"purpose", "e5_renewal")

	return nil
}

// ProcessAllUserMailActivity processes mail activity for all users with stored tokens
func (s *MailService) ProcessAllUserMailActivity(ctx context.Context) error {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "ProcessAllUserMailActivity")
	defer span.End()

	startTime := time.Now()

	s.logger.Info("🔄 Starting bulk mail activity processing",
		"activity", "bulk_mail_processing",
		"purpose", "e5_renewal",
		"startTime", startTime.Format(time.RFC3339))

	// Get all user IDs with stored tokens
	userIDs, err := s.userTokenService.GetAllUserIDs(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.Bool("success", false))
		return fmt.Errorf("failed to get user IDs: %w", err)
	}

	span.SetAttributes(
		attribute.Int("total_users", len(userIDs)),
		attribute.String("job_type", "bulk_mail_processing"),
	)

	successCount := 0
	errorCount := 0

	// Process each user
	for _, userID := range userIDs {
		_, userSpan := tracer.Start(ctx, "ProcessUserMailActivity")
		userSpan.SetAttributes(attribute.String("user_id", userID))

		if err := s.ProcessUserMailActivity(ctx, userID); err != nil {
			errorCount++
			userSpan.RecordError(err)
			userSpan.SetAttributes(attribute.Bool("success", false))
			s.logger.Error("Failed to process mail activity for user",
				"userID", userID,
				"error", err)
		} else {
			successCount++
			userSpan.SetAttributes(attribute.Bool("success", true))
		}

		userSpan.End()
	}

	processingDuration := time.Since(startTime)

	span.SetAttributes(
		attribute.Int("success_count", successCount),
		attribute.Int("error_count", errorCount),
		attribute.Int64("total_duration_ms", processingDuration.Milliseconds()),
		attribute.Bool("success", errorCount == 0),
	)

	s.logger.Info("🎯 Completed bulk mail activity processing",
		"totalUsers", len(userIDs),
		"successCount", successCount,
		"errorCount", errorCount,
		"processingDuration", processingDuration,
		"activity", "bulk_mail_processing",
		"purpose", "e5_renewal")

	if errorCount > 0 {
		return fmt.Errorf("failed to process mail activity for %d out of %d users", errorCount, len(userIDs))
	}

	return nil
}
