package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	msgraphsdkgo "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/attribute"

	"github.com/panxiao81/e5renew/internal/middleware"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/telemetry"
)

var newClientSecretCredential = func(tenantID, clientID, clientSecret string) (azcore.TokenCredential, error) {
	return azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
}

var newGraphServiceClientWithCredentials = func(credential azcore.TokenCredential, scopes []string) (*msgraphsdkgo.GraphServiceClient, error) {
	return msgraphsdkgo.NewGraphServiceClientWithCredentials(credential, scopes)
}

var getGraphUsers = func(ctx context.Context, client *msgraphsdkgo.GraphServiceClient) (models.UserCollectionResponseable, error) {
	return client.Users().Get(ctx, nil)
}

func GetUsersAndMessagesClientScope(ctx context.Context, apiLogService *services.APILogService, logger *slog.Logger) (map[string]int, error) {
	ctx, span := telemetry.StartSpan(ctx, "github.com/panxiao81/e5renew/jobs", "GetUsersAndMessagesClientScope")
	defer span.End()

	start := time.Now()

	logger.Info("🔄 Starting E5 renewal job - Graph API client scope call",
		"job", "GetUsersAndMessagesClientScope",
		"startTime", start.Format(time.RFC3339),
		"type", "background_job")

	cred, err := newClientSecretCredential(
		viper.GetString("azureAD.tenant"),
		viper.GetString("azureAD.clientID"),
		viper.GetString("azureAD.clientSecret"),
	)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		logger.Error("❌ Failed to create Azure AD credentials",
			"job", "GetUsersAndMessagesClientScope",
			"error", err,
			"step", "credential_creation")
		return nil, fmt.Errorf("failed to create Azure AD client secret credentials: %w", err)
	}

	logger.Debug("✅ Azure AD credentials created successfully",
		"job", "GetUsersAndMessagesClientScope",
		"step", "credential_creation")

	// Create Graph client - API logging is handled manually below due to SDK limitations
	client, err := newGraphServiceClientWithCredentials(cred, []string{"https://graph.microsoft.com/.default"})
	if err != nil {
		telemetry.RecordSpanError(span, err)
		logger.Error("❌ Failed to create Microsoft Graph client",
			"job", "GetUsersAndMessagesClientScope",
			"error", err,
			"step", "graph_client_creation")
		return nil, fmt.Errorf("failed to create Microsoft Graph client: %w", err)
	}

	logger.Debug("✅ Microsoft Graph client created successfully",
		"job", "GetUsersAndMessagesClientScope",
		"step", "graph_client_creation")

	// Record start time for API logging
	apiStartTime := time.Now()

	logger.Info("📞 Calling Microsoft Graph API - Users endpoint",
		"job", "GetUsersAndMessagesClientScope",
		"endpoint", "users",
		"method", "GET",
		"step", "api_call")

	user, err := getGraphUsers(ctx, client)

	// Log the API call
	apiEndTime := time.Now()
	apiDuration := apiEndTime.Sub(apiStartTime)

	apiLogEntry := middleware.APILogEntry{
		UserID:         nil, // Client credentials call
		APIEndpoint:    "users",
		HTTPMethod:     "GET",
		RequestTime:    apiStartTime,
		ResponseTime:   apiEndTime,
		DurationMs:     int(apiDuration.Milliseconds()),
		RequestSize:    0,
		ResponseSize:   0, // Graph SDK doesn't expose response size easily
		JobType:        "client_credentials",
		HTTPStatusCode: 200, // Assume success if no error
		Success:        err == nil,
	}

	if err != nil {
		telemetry.RecordSpanError(span, err)
		apiLogEntry.HTTPStatusCode = 500 // Generic error code
		errorMsg := err.Error()
		apiLogEntry.ErrorMessage = &errorMsg

		logger.Error("❌ Microsoft Graph API call failed",
			"job", "GetUsersAndMessagesClientScope",
			"endpoint", "users",
			"error", err,
			"duration", apiDuration,
			"step", "api_call")

		// Log the failed API call
		go func() {
			logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if logErr := apiLogService.LogAPICall(logCtx, apiLogEntry); logErr != nil {
				logger.Error("Failed to log API call", "error", logErr)
			}
		}()

		return nil, err
	}

	userCount := len(user.GetValue())

	logger.Info("✅ Microsoft Graph API call successful",
		"job", "GetUsersAndMessagesClientScope",
		"endpoint", "users",
		"userCount", userCount,
		"duration", apiDuration,
		"step", "api_call")

	// Log the successful API call
	go func() {
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if logErr := apiLogService.LogAPICall(logCtx, apiLogEntry); logErr != nil {
			logger.Error("Failed to log API call", "error", logErr)
		}
	}()

	count := make(map[string]int)
	for _, u := range user.GetValue() {
		count[*u.GetDisplayName()] = len(u.GetMessages())
	}
	duration := time.Since(start)
	span.SetAttributes(
		attribute.Int("users.total", userCount),
		attribute.Int("messages.total", len(count)),
		attribute.Int64("duration.ms", duration.Milliseconds()),
	)

	logger.Info("🎉 E5 renewal job completed successfully",
		"job", "GetUsersAndMessagesClientScope",
		"totalUsers", userCount,
		"totalMessages", len(count),
		"totalDuration", duration,
		"type", "background_job",
		"status", "completed")

	return count, nil
}
