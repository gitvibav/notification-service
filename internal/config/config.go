package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Channels ChannelsConfig
	Logger   zerolog.Logger
}

type ServerConfig struct {
	Port         string
	ShutdownWait time.Duration
}

type DatabaseConfig struct {
	Type string // "sqlite" or "memory"
	Path string // for SQLite
}

type ChannelsConfig struct {
	Email ChannelConfig
	SMS   ChannelConfig
	Push  ChannelConfig
}

type ChannelConfig struct {
	RateLimit      int           // requests per second
	FailureRate    float64       // 0.0 to 1.0
	MaxRetries     int
	InitialBackoff time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			ShutdownWait: getEnvDuration("SHUTDOWN_WAIT", 10*time.Second),
		},
		Database: DatabaseConfig{
			Type: getEnv("DB_TYPE", "memory"),
			Path: getEnv("DB_PATH", "./notifications.db"),
		},
		Channels: ChannelsConfig{
			Email: ChannelConfig{
				RateLimit:      getEnvInt("EMAIL_RATE_LIMIT", 100),
				FailureRate:    getEnvFloat("EMAIL_FAILURE_RATE", 0.2),
				MaxRetries:     getEnvInt("MAX_RETRIES", 3),
				InitialBackoff: getEnvDuration("INITIAL_BACKOFF", 100*time.Millisecond),
			},
			SMS: ChannelConfig{
				RateLimit:      getEnvInt("SMS_RATE_LIMIT", 20),
				FailureRate:    getEnvFloat("SMS_FAILURE_RATE", 0.2),
				MaxRetries:     getEnvInt("MAX_RETRIES", 3),
				InitialBackoff: getEnvDuration("INITIAL_BACKOFF", 100*time.Millisecond),
			},
			Push: ChannelConfig{
				RateLimit:      getEnvInt("PUSH_RATE_LIMIT", 500),
				FailureRate:    getEnvFloat("PUSH_FAILURE_RATE", 0.2),
				MaxRetries:     getEnvInt("MAX_RETRIES", 3),
				InitialBackoff: getEnvDuration("INITIAL_BACKOFF", 100*time.Millisecond),
			},
		},
	}

	// Setup structured logging
	logLevel := getEnv("LOG_LEVEL", "info")
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(level)

	if getEnv("LOG_PRETTY", "false") == "true" {
		cfg.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		cfg.Logger = log.Logger
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}

	if c.Database.Type != "sqlite" && c.Database.Type != "memory" {
		return fmt.Errorf("database type must be 'sqlite' or 'memory'")
	}

	if c.Database.Type == "sqlite" && c.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty for SQLite")
	}

	// Validate channel configs
	channels := []ChannelConfig{c.Channels.Email, c.Channels.SMS, c.Channels.Push}
	for _, ch := range channels {
		if ch.RateLimit <= 0 {
			return fmt.Errorf("rate limit must be positive")
		}
		if ch.FailureRate < 0 || ch.FailureRate > 1 {
			return fmt.Errorf("failure rate must be between 0 and 1")
		}
		if ch.MaxRetries < 0 {
			return fmt.Errorf("max retries cannot be negative")
		}
		if ch.InitialBackoff <= 0 {
			return fmt.Errorf("initial backoff must be positive")
		}
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
