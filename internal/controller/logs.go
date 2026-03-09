package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/services"
)

type LogsController struct {
	environment.Application
	apiLogService *services.APILogService
}

// NewLogsController creates a new LogsController
func NewLogsController(app environment.Application, apiLogService *services.APILogService) *LogsController {
	return &LogsController{
		Application:   app,
		apiLogService: apiLogService,
	}
}

// Index displays the main logs page
func (c *LogsController) Index(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "logs_index")
	defer span.End()

	// Check if user is logged in
	if !c.SessionManager.Exists(ctx, "user") {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Parse query parameters
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := int32(50)
	offset := int32((page - 1) * int(limit))

	// Get filter parameters
	jobType := r.URL.Query().Get("job_type")
	userID := r.URL.Query().Get("user_id")

	// Time range filter (last 24 hours by default)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if timeRangeStr := r.URL.Query().Get("time_range"); timeRangeStr != "" {
		switch timeRangeStr {
		case "1h":
			startTime = endTime.Add(-time.Hour)
		case "6h":
			startTime = endTime.Add(-6 * time.Hour)
		case "24h":
			startTime = endTime.Add(-24 * time.Hour)
		case "7d":
			startTime = endTime.Add(-7 * 24 * time.Hour)
		case "30d":
			startTime = endTime.Add(-30 * 24 * time.Hour)
		}
	}

	span.SetAttributes(
		attribute.Int("page", page),
		attribute.Int("limit", int(limit)),
		attribute.String("job_type", jobType),
		attribute.String("user_id", userID),
		attribute.String("start_time", startTime.Format(time.RFC3339)),
		attribute.String("end_time", endTime.Format(time.RFC3339)),
	)

	// Get logs based on filters
	var logs []db.ApiLog
	var err error

	switch {
	case jobType != "":
		logs, err = c.apiLogService.GetAPILogsByJobType(ctx, jobType, limit, offset)
	case userID != "":
		logs, err = c.apiLogService.GetAPILogsByUser(ctx, userID, limit, offset)
	default:
		logs, err = c.apiLogService.GetAPILogsByTimeRange(ctx, startTime, endTime, limit, offset)
	}

	if err != nil {
		span.RecordError(err)
		c.Logger.Error("Failed to get API logs", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get statistics
	stats, err := c.apiLogService.GetAPILogStats(ctx, startTime, endTime)
	if err != nil {
		span.RecordError(err)
		c.Logger.Error("Failed to get API log stats", "error", err)
		// Continue without stats
		stats = nil
	}

	// Get endpoint statistics
	endpointStats, err := c.apiLogService.GetAPILogStatsByEndpoint(ctx, startTime, endTime)
	if err != nil {
		span.RecordError(err)
		c.Logger.Error("Failed to get API log endpoint stats", "error", err)
		// Continue without endpoint stats
		endpointStats = nil
	}

	// Calculate success rates for endpoint stats
	type EndpointStatsWithRate struct {
		services.APILogEndpointStats
		SuccessRate float64
	}
	var endpointStatsWithRates []EndpointStatsWithRate
	if endpointStats != nil {
		for _, stat := range endpointStats {
			rate := 0.0
			if stat.TotalRequests > 0 {
				rate = float64(stat.SuccessfulRequests) / float64(stat.TotalRequests) * 100
			}
			endpointStatsWithRates = append(endpointStatsWithRates, EndpointStatsWithRate{
				APILogEndpointStats: stat,
				SuccessRate:         rate,
			})
		}
	}

	span.SetAttributes(
		attribute.Int("logs_count", len(logs)),
		attribute.Bool("has_stats", stats != nil),
		attribute.Bool("has_endpoint_stats", endpointStats != nil),
	)

	// Render template
	data := map[string]interface{}{
		"Title":         "API Logs",
		"Logs":          logs,
		"Stats":         stats,
		"EndpointStats": endpointStatsWithRates,
		"CurrentPage":   page,
		"PrevPage":      page - 1,
		"NextPage":      page + 1,
		"Limit":         limit,
		"JobType":       jobType,
		"UserID":        userID,
		"TimeRange":     r.URL.Query().Get("time_range"),
		"StartTime":     startTime.Format("2006-01-02T15:04:05"),
		"EndTime":       endTime.Format("2006-01-02T15:04:05"),
	}

	err = c.Template.RenderWithContext(r.Context(), w, "logs.html", data)
	if err != nil {
		span.RecordError(err)
		c.Logger.Error("Failed to render logs template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// Stats displays API statistics
func (c *LogsController) Stats(w http.ResponseWriter, r *http.Request) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/controller")
	ctx, span := tracer.Start(r.Context(), "logs_stats")
	defer span.End()

	// Check if user is logged in
	if !c.SessionManager.Exists(ctx, "user") {
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
		return
	}

	// Time range (last 24 hours by default)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if timeRangeStr := r.URL.Query().Get("time_range"); timeRangeStr != "" {
		switch timeRangeStr {
		case "1h":
			startTime = endTime.Add(-time.Hour)
		case "6h":
			startTime = endTime.Add(-6 * time.Hour)
		case "24h":
			startTime = endTime.Add(-24 * time.Hour)
		case "7d":
			startTime = endTime.Add(-7 * 24 * time.Hour)
		case "30d":
			startTime = endTime.Add(-30 * 24 * time.Hour)
		}
	}

	// Get overall statistics
	stats, err := c.apiLogService.GetAPILogStats(ctx, startTime, endTime)
	if err != nil {
		span.RecordError(err)
		c.Logger.Error("Failed to get API log stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get endpoint statistics
	endpointStats, err := c.apiLogService.GetAPILogStatsByEndpoint(ctx, startTime, endTime)
	if err != nil {
		span.RecordError(err)
		c.Logger.Error("Failed to get API log endpoint stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Calculate success rates for endpoint stats
	type EndpointStatsWithRate struct {
		services.APILogEndpointStats
		SuccessRate float64
	}
	var endpointStatsWithRates []EndpointStatsWithRate
	for _, stat := range endpointStats {
		rate := 0.0
		if stat.TotalRequests > 0 {
			rate = float64(stat.SuccessfulRequests) / float64(stat.TotalRequests) * 100
		}
		endpointStatsWithRates = append(endpointStatsWithRates, EndpointStatsWithRate{
			APILogEndpointStats: stat,
			SuccessRate:         rate,
		})
	}

	span.SetAttributes(
		attribute.Int64("total_requests", stats.TotalRequests),
		attribute.Int64("successful_requests", stats.SuccessfulRequests),
		attribute.Int64("failed_requests", stats.FailedRequests),
		attribute.Int("endpoint_count", len(endpointStats)),
	)

	// Calculate success rate
	successRate := float64(0)
	if stats.TotalRequests > 0 {
		successRate = float64(stats.SuccessfulRequests) / float64(stats.TotalRequests) * 100
	}

	data := map[string]interface{}{
		"Title":         "API Statistics",
		"Stats":         stats,
		"EndpointStats": endpointStatsWithRates,
		"SuccessRate":   successRate,
		"TimeRange":     r.URL.Query().Get("time_range"),
		"StartTime":     startTime.Format("2006-01-02T15:04:05"),
		"EndTime":       endTime.Format("2006-01-02T15:04:05"),
	}

	err = c.Template.RenderWithContext(r.Context(), w, "stats.html", data)
	if err != nil {
		span.RecordError(err)
		c.Logger.Error("Failed to render stats template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// RegisterRoutes registers the logs controller routes
func (c *LogsController) RegisterRoutes(router *chi.Mux) {
	router.Get("/logs", c.Index)
	router.Get("/logs/stats", c.Stats)
}
