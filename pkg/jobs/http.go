package jobs

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/milan604/core-lab/pkg/apperr"
	"github.com/milan604/core-lab/pkg/response"
	"github.com/milan604/core-lab/pkg/server"
)

// RegisterAdminRoutes mounts job administration routes onto the provided router.
func RegisterAdminRoutes(router gin.IRoutes, manager *Manager) {
	router.GET("/healthz", func(c *gin.Context) {
		response.Success(c, manager.Health())
	})

	router.GET("/handlers", func(c *gin.Context) {
		response.Success(c, manager.Handlers())
	})

	router.GET("/stats", func(c *gin.Context) {
		stats, err := manager.Stats(c.Request.Context())
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.Success(c, stats)
	})

	router.GET("/jobs", func(c *gin.Context) {
		filter, err := parseJobFilter(c)
		if err != nil {
			response.HandleError(c, err)
			return
		}

		jobs, err := manager.ListJobs(c.Request.Context(), filter)
		if err != nil {
			response.HandleError(c, err)
			return
		}

		response.JSONSuccess(c, http.StatusOK, jobs, map[string]any{
			"limit":  filter.Limit,
			"offset": filter.Offset,
			"count":  len(jobs),
		})
	})

	router.GET("/jobs/:id", func(c *gin.Context) {
		job, err := manager.GetJob(c.Request.Context(), c.Param("id"))
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.Success(c, job)
	})

	router.POST("/jobs", func(c *gin.Context) {
		var req EnqueueRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.HandleError(c, apperr.New(apperr.ErrorCodeInvalidRequest).
				WithMessage("invalid enqueue request").
				AddSuggestion("body", err.Error()))
			return
		}

		job, err := manager.Enqueue(c.Request.Context(), req)
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.JSONSuccess(c, http.StatusAccepted, job, nil)
	})

	router.POST("/jobs/:id/retry", func(c *gin.Context) {
		job, err := manager.RetryJob(c.Request.Context(), c.Param("id"))
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.Success(c, job)
	})

	router.POST("/jobs/:id/cancel", func(c *gin.Context) {
		job, err := manager.CancelJob(c.Request.Context(), c.Param("id"))
		if err != nil {
			response.HandleError(c, err)
			return
		}
		response.Success(c, job)
	})
}

// NewAdminEngine builds a standalone HTTP server for the job runtime.
func NewAdminEngine(manager *Manager, opts ...server.EngineOption) *gin.Engine {
	engineOpts := []server.EngineOption{
		server.WithLogger(manager.log),
		server.WithRecovery(true),
		server.WithPrometheus(true),
	}
	engineOpts = append(engineOpts, opts...)

	engine := server.NewEngine(engineOpts...)
	RegisterAdminRoutes(engine.Group("/"), manager)
	return engine
}

func parseJobFilter(c *gin.Context) (JobFilter, error) {
	filter := JobFilter{
		Queue:  c.Query("queue"),
		Type:   c.Query("type"),
		Limit:  50,
		Offset: 0,
	}

	if limit := c.Query("limit"); limit != "" {
		value, err := strconv.Atoi(limit)
		if err != nil || value < 0 {
			return JobFilter{}, apperr.New(apperr.ErrorCodeInvalidInput).
				WithMessage("invalid limit").
				AddSuggestion("limit", "provide a non-negative integer")
		}
		filter.Limit = value
	}

	if offset := c.Query("offset"); offset != "" {
		value, err := strconv.Atoi(offset)
		if err != nil || value < 0 {
			return JobFilter{}, apperr.New(apperr.ErrorCodeInvalidInput).
				WithMessage("invalid offset").
				AddSuggestion("offset", "provide a non-negative integer")
		}
		filter.Offset = value
	}

	if statuses := c.Query("status"); statuses != "" {
		parts := strings.Split(statuses, ",")
		filter.Statuses = make([]Status, 0, len(parts))
		for _, part := range parts {
			status := Status(strings.TrimSpace(part))
			switch status {
			case StatusScheduled, StatusQueued, StatusRunning, StatusSucceeded, StatusFailed, StatusCanceled:
				filter.Statuses = append(filter.Statuses, status)
			default:
				return JobFilter{}, apperr.New(apperr.ErrorCodeInvalidInput).
					WithMessage("invalid status filter").
					AddSuggestion("status", "use scheduled,queued,running,succeeded,failed,canceled")
			}
		}
	}

	return filter, nil
}
