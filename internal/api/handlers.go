package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"notification-service/internal/service"
)

type Handler struct {
	service *service.Service
	logger  *zerolog.Logger
}

func NewHandler(service *service.Service, logger *zerolog.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1")
	{
		v1.POST("/notify", h.CreateNotification)
		v1.GET("/notifications/:id", h.GetNotification)
		v1.GET("/health", h.HealthCheck)
	}
}

// CreateNotification handles POST /api/v1/notify
func (h *Handler) CreateNotification(c *gin.Context) {
	var req service.CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error().
			Err(err).
			Msg("Invalid request body")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	resp, err := h.service.CreateNotification(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("channel", string(req.Channel)).
			Str("recipient", req.Recipient).
			Msg("Failed to create notification")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create notification",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info().
		Str("notification_id", resp.ID).
		Str("channel", string(req.Channel)).
		Str("recipient", req.Recipient).
		Msg("Notification created successfully")

	c.JSON(http.StatusAccepted, gin.H{
		"notification_id": resp.ID,
		"message": "Notification queued for processing",
	})
}

// GetNotification handles GET /api/v1/notifications/:id
func (h *Handler) GetNotification(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Notification ID is required",
		})
		return
	}

	resp, err := h.service.GetNotification(c.Request.Context(), id)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("notification_id", id).
			Msg("Failed to get notification")
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Notification not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// HealthCheck handles GET /api/v1/health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": "notification-service",
	})
}
