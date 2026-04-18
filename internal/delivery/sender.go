package delivery

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"notification-service/internal/storage"

	"github.com/rs/zerolog"
)

type Sender interface {
	Send(ctx context.Context, notification *storage.Notification) error
}

type MockSender struct {
	channel     storage.Channel
	failureRate float64
	logger      *zerolog.Logger
}

func NewMockSender(channel storage.Channel, failureRate float64, logger *zerolog.Logger) *MockSender {
	return &MockSender{
		channel:     channel,
		failureRate: failureRate,
		logger:      logger,
	}
}

func (m *MockSender) Send(ctx context.Context, notification *storage.Notification) error {
	// Simulate network delay
	select {
	case <-time.After(time.Duration(10+rand.Intn(50)) * time.Millisecond):
		// Continue
	case <-ctx.Done():
		return ctx.Err()
	}

	// Simulate random failure
	if rand.Float64() < m.failureRate {
		err := fmt.Errorf("mock %s delivery failed", m.channel)
		m.logger.Error().
			Str("notification_id", notification.ID).
			Str("channel", string(m.channel)).
			Err(err).
			Msg("Delivery failed")
		return err
	}

	m.logger.Info().
		Str("notification_id", notification.ID).
		Str("channel", string(m.channel)).
		Str("recipient", notification.Recipient).
		Msg("Delivery successful")

	return nil
}
