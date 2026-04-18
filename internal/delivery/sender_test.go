package delivery

import (
	"context"
	"testing"

	"notification-service/internal/storage"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestMockSender_Send(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	t.Run("Successful delivery", func(t *testing.T) {
		// Set failure rate to 0 for deterministic success
		sender := NewMockSender(storage.ChannelEmail, 0.0, &logger)

		notification := &storage.Notification{
			ID:        "test-1",
			Channel:   storage.ChannelEmail,
			Recipient: "test@example.com",
			Message:   "Test message",
		}

		err := sender.Send(context.Background(), notification)
		assert.NoError(t, err)
	})

	t.Run("Failed delivery", func(t *testing.T) {
		// Set failure rate to 1.0 for deterministic failure
		sender := NewMockSender(storage.ChannelSMS, 1.0, &logger)

		notification := &storage.Notification{
			ID:        "test-2",
			Channel:   storage.ChannelSMS,
			Recipient: "+1234567890",
			Message:   "Test SMS",
		}

		err := sender.Send(context.Background(), notification)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mock sms delivery failed")
	})

	t.Run("Context cancellation", func(t *testing.T) {
		sender := NewMockSender(storage.ChannelPush, 0.0, &logger)

		notification := &storage.Notification{
			ID:        "test-3",
			Channel:   storage.ChannelPush,
			Recipient: "device-token",
			Message:   "Test push",
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := sender.Send(ctx, notification)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("Random failure rate", func(t *testing.T) {
		// Test with 50% failure rate
		sender := NewMockSender(storage.ChannelEmail, 0.5, &logger)

		notification := &storage.Notification{
			ID:        "test-4",
			Channel:   storage.ChannelEmail,
			Recipient: "test@example.com",
			Message:   "Test message",
		}

		// Run multiple times to test randomness
		successes := 0
		failures := 0
		const iterations = 100

		for i := 0; i < iterations; i++ {
			err := sender.Send(context.Background(), notification)
			if err != nil {
				failures++
			} else {
				successes++
			}
		}

		// With 50% failure rate, we should have roughly 50/50 split
		// Allow some variance for randomness
		assert.InDelta(t, 50, successes, 20)
		assert.InDelta(t, 50, failures, 20)
		assert.Equal(t, iterations, successes+failures)
	})
}
