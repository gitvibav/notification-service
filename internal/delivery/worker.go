package delivery

import (
	"context"
	"fmt"
	"time"

	"notification-service/internal/config"
	"notification-service/internal/storage"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

type Worker struct {
	channel storage.Channel
	sender  Sender
	limiter *rate.Limiter
	storage storage.Storage
	config  *config.ChannelConfig
	logger  *zerolog.Logger
}

func NewWorker(
	channel storage.Channel,
	sender Sender,
	storage storage.Storage,
	config *config.ChannelConfig,
	logger *zerolog.Logger,
) *Worker {
	return &Worker{
		channel: channel,
		sender:  sender,
		limiter: rate.NewLimiter(rate.Limit(config.RateLimit), config.RateLimit),
		storage: storage,
		config:  config,
		logger:  logger,
	}
}

func (w *Worker) Start(ctx context.Context, notifications <-chan *storage.Notification) {
	w.logger.Info().
		Str("channel", string(w.channel)).
		Int("rate_limit", w.config.RateLimit).
		Msg("Starting worker")

	for {
		select {
		case <-ctx.Done():
			w.logger.Info().
				Str("channel", string(w.channel)).
				Msg("Worker shutting down")
			return

		case notification, ok := <-notifications:
			if !ok {
				w.logger.Info().
					Str("channel", string(w.channel)).
					Msg("Notification channel closed, worker shutting down")
				return
			}

			// Only process notifications for this channel
			if notification.Channel != w.channel {
				// Skip notifications for other channels
				// In a real implementation, we'd have proper routing
				continue
			}

			w.processNotification(ctx, notification)
		}
	}
}

func (w *Worker) processNotification(ctx context.Context, notification *storage.Notification) {
	// Rate limit
	if err := w.limiter.Wait(ctx); err != nil {
		w.logger.Error().
			Str("notification_id", notification.ID).
			Str("channel", string(w.channel)).
			Err(err).
			Msg("Rate limit wait failed")
		return
	}

	// Check if we should retry
	if notification.RetryCount >= w.config.MaxRetries {
		w.markFailed(notification, fmt.Sprintf("max retries (%d) exceeded", w.config.MaxRetries))
		return
	}

	// Attempt delivery
	err := w.sender.Send(ctx, notification)
	if err != nil {
		w.handleDeliveryFailure(ctx, notification, err)
		return
	}

	// Success
	w.markSent(notification)
}

func (w *Worker) handleDeliveryFailure(ctx context.Context, notification *storage.Notification, err error) {
	notification.RetryCount++
	notification.LastError = err.Error()

	if notification.RetryCount >= w.config.MaxRetries {
		w.markFailed(notification, fmt.Sprintf("max retries (%d) exceeded. Last error: %s", w.config.MaxRetries, err.Error()))
		return
	}

	// Calculate backoff delay
	backoff := time.Duration(notification.RetryCount) * w.config.InitialBackoff
	w.logger.Info().
		Str("notification_id", notification.ID).
		Str("channel", string(w.channel)).
		Int("retry_count", notification.RetryCount).
		Dur("backoff", backoff).
		Err(err).
		Msg("Delivery failed, scheduling retry")

	// Schedule retry
	go func() {
		select {
		case <-time.After(backoff):
			// Re-queue for retry
			w.processNotification(ctx, notification)
		case <-ctx.Done():
		}
	}()
}

func (w *Worker) markSent(notification *storage.Notification) {
	notification.Status = storage.StatusSent
	notification.LastError = ""

	if err := w.storage.Update(notification); err != nil {
		w.logger.Error().
			Str("notification_id", notification.ID).
			Str("channel", string(w.channel)).
			Err(err).
			Msg("Failed to update notification status to sent")
		return
	}

	w.logger.Info().
		Str("notification_id", notification.ID).
		Str("channel", string(w.channel)).
		Msg("Notification marked as sent")
}

func (w *Worker) markFailed(notification *storage.Notification, reason string) {
	notification.Status = storage.StatusFailed
	notification.LastError = reason

	if err := w.storage.Update(notification); err != nil {
		w.logger.Error().
			Str("notification_id", notification.ID).
			Str("channel", string(w.channel)).
			Err(err).
			Msg("Failed to update notification status to failed")
		return
	}

	w.logger.Error().
		Str("notification_id", notification.ID).
		Str("channel", string(w.channel)).
		Str("reason", reason).
		Msg("Notification marked as failed")
}
