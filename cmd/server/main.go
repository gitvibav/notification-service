package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"notification-service/internal/api"
	"notification-service/internal/config"
	"notification-service/internal/delivery"
	"notification-service/internal/service"
	"notification-service/internal/storage"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	cfg.Logger.Info().Msg("Starting notification service")

	// Initialize storage
	strg, err := storage.NewStorage(cfg)
	if err != nil {
		cfg.Logger.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer strg.Close()

	// Initialize service
	svc := service.NewService(cfg, strg)

	// Initialize delivery manager
	deliveryManager := delivery.NewManager(cfg, strg)

	// Initialize API handlers
	handler := api.NewHandler(svc, &cfg.Logger)

	// Setup Gin router
	if cfg.Logger.GetLevel() == zerolog.DebugLevel {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Add structured logging middleware
	router.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		cfg.Logger.Info().
			Str("method", method).
			Str("path", path).
			Int("status", statusCode).
			Str("ip", clientIP).
			Dur("latency", latency).
			Msg("HTTP request")
	})

	handler.RegisterRoutes(router)

	// Create HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	// Start delivery manager
	go deliveryManager.Start(svc.GetQueue())

	// Start HTTP server in goroutine
	go func() {
		cfg.Logger.Info().Str("port", cfg.Server.Port).Msg("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			cfg.Logger.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cfg.Logger.Info().Msg("Shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownWait)
	defer cancel()

	// Shutdown delivery manager
	if err := deliveryManager.Shutdown(shutdownCtx); err != nil {
		cfg.Logger.Error().Err(err).Msg("Error during delivery manager shutdown")
	}

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		cfg.Logger.Error().Err(err).Msg("Error during server shutdown")
	}

	// Close service
	if err := svc.Close(); err != nil {
		cfg.Logger.Error().Err(err).Msg("Error during service shutdown")
	}

	cfg.Logger.Info().Msg("Server exited")
}
