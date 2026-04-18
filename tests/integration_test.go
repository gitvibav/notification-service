package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"notification-service/internal/api"
	"notification-service/internal/config"
	"notification-service/internal/delivery"
	"notification-service/internal/service"
	"notification-service/internal/storage"
)

func setupTestServer(t *testing.T) (*gin.Engine, *service.Service, *delivery.Manager) {
	// Create test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:         "8080",
			ShutdownWait: 10 * time.Second,
		},
		Database: config.DatabaseConfig{
			Type: "memory",
		},
		Channels: config.ChannelsConfig{
			Email: config.ChannelConfig{
				RateLimit:      100,
				FailureRate:    0.0, // No failures for integration test
				MaxRetries:     3,
				InitialBackoff: 100 * time.Millisecond,
			},
			SMS: config.ChannelConfig{
				RateLimit:      20,
				FailureRate:    0.0, // No failures for integration test
				MaxRetries:     3,
				InitialBackoff: 100 * time.Millisecond,
			},
			Push: config.ChannelConfig{
				RateLimit:      500,
				FailureRate:    0.0, // No failures for integration test
				MaxRetries:     3,
				InitialBackoff: 100 * time.Millisecond,
			},
		},
		Logger: zerolog.New(zerolog.NewTestWriter(t)),
	}

	// Initialize storage
	strg, err := storage.NewStorage(cfg)
	require.NoError(t, err)

	// Initialize service
	svc := service.NewService(cfg, strg)

	// Initialize delivery manager
	deliveryManager := delivery.NewManager(cfg, strg)

	// Initialize API handlers
	handler := api.NewHandler(svc, &cfg.Logger)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler.RegisterRoutes(router)

	// Start delivery manager in background
	go deliveryManager.Start(svc.GetQueue())

	return router, svc, deliveryManager
}

func TestIntegration_NotificationFlow(t *testing.T) {
	router, _, deliveryManager := setupTestServer(t)
	defer deliveryManager.Shutdown(context.Background())

	t.Run("Create and process email notification", func(t *testing.T) {
		// Create notification request
		reqBody := map[string]interface{}{
			"channel":   "email",
			"recipient": "test@example.com",
			"message":   "Hello, this is a test email!",
		}

		reqJSON, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notify", bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Send request
		router.ServeHTTP(w, req)

		// Check response
		assert.Equal(t, http.StatusAccepted, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		notificationID, ok := response["notification_id"].(string)
		require.True(t, ok)
		assert.NotEmpty(t, notificationID)

		// Wait a bit for processing
		time.Sleep(200 * time.Millisecond)

		// Check notification status
		req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/notifications/%s", notificationID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var statusResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &statusResponse)
		require.NoError(t, err)

		assert.Equal(t, "sent", statusResponse["status"])
		assert.Equal(t, "email", statusResponse["channel"])
		assert.Equal(t, "test@example.com", statusResponse["recipient"])
		assert.Equal(t, "Hello, this is a test email!", statusResponse["message"])
	})

	t.Run("Create and process SMS notification", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"channel":   "sms",
			"recipient": "+1234567890",
			"message":   "Hello, this is a test SMS!",
		}

		reqJSON, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notify", bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		notificationID, ok := response["notification_id"].(string)
		require.True(t, ok)

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		// Check status
		req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/notifications/%s", notificationID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var statusResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &statusResponse)
		require.NoError(t, err)

		assert.Equal(t, "sent", statusResponse["status"])
		assert.Equal(t, "sms", statusResponse["channel"])
	})

	t.Run("Create and process push notification", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"channel":   "push",
			"recipient": "device-token-123",
			"message":   "Hello, this is a test push notification!",
		}

		reqJSON, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notify", bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusAccepted, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		notificationID, ok := response["notification_id"].(string)
		require.True(t, ok)

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		// Check status
		req = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/notifications/%s", notificationID), nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var statusResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &statusResponse)
		require.NoError(t, err)

		assert.Equal(t, "sent", statusResponse["status"])
		assert.Equal(t, "push", statusResponse["channel"])
	})

	t.Run("Health check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "healthy", response["status"])
		assert.Equal(t, "notification-service", response["service"])
	})

	t.Run("Invalid notification request", func(t *testing.T) {
		// Missing required fields
		reqBody := map[string]interface{}{
			"channel": "invalid",
		}

		reqJSON, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notify", bytes.NewBuffer(reqJSON))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Non-existent notification", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/non-existent", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
