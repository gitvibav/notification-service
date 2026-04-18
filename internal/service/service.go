package service

import (
	"context"
	"fmt"
	"time"

	"notification-service/internal/config"
	"notification-service/internal/storage"
)

type Service struct {
	cfg     *config.Config
	storage storage.Storage
	queue   chan *storage.Notification
}

type CreateNotificationRequest struct {
	Channel   storage.Channel `json:"channel" binding:"required"`
	Recipient string          `json:"recipient" binding:"required"`
	Message   string          `json:"message" binding:"required"`
}

type CreateNotificationResponse struct {
	ID string `json:"id"`
}

type NotificationResponse struct {
	ID         string          `json:"id"`
	Channel    storage.Channel `json:"channel"`
	Recipient  string          `json:"recipient"`
	Message    string          `json:"message"`
	Status     storage.Status  `json:"status"`
	RetryCount int             `json:"retry_count"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	LastError  string          `json:"last_error,omitempty"`
}

func NewService(cfg *config.Config, strg storage.Storage) *Service {
	return &Service{
		cfg:     cfg,
		storage: strg,
		queue:   make(chan *storage.Notification, 1000), // Buffered channel for queue
	}
}

func (s *Service) CreateNotification(ctx context.Context, req *CreateNotificationRequest) (*CreateNotificationResponse, error) {
	// Validate channel
	switch req.Channel {
	case storage.ChannelEmail, storage.ChannelSMS, storage.ChannelPush:
		// Valid channels
	default:
		return nil, fmt.Errorf("invalid channel: %s", req.Channel)
	}

	// Generate unique ID
	id := generateID()

	notification := &storage.Notification{
		ID:         id,
		Channel:    req.Channel,
		Recipient:  req.Recipient,
		Message:    req.Message,
		Status:     storage.StatusPending,
		RetryCount: 0,
	}

	// Store notification
	if err := s.storage.Create(notification); err != nil {
		return nil, fmt.Errorf("failed to create notification: %w", err)
	}

	// Queue for processing
	select {
	case s.queue <- notification:
		s.cfg.Logger.Info().
			Str("notification_id", id).
			Str("channel", string(req.Channel)).
			Str("recipient", req.Recipient).
			Msg("Notification queued for processing")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return &CreateNotificationResponse{ID: id}, nil
}

func (s *Service) GetNotification(ctx context.Context, id string) (*NotificationResponse, error) {
	notification, err := s.storage.Get(id)
	if err != nil {
		return nil, err
	}

	return &NotificationResponse{
		ID:         notification.ID,
		Channel:    notification.Channel,
		Recipient:  notification.Recipient,
		Message:    notification.Message,
		Status:     notification.Status,
		RetryCount: notification.RetryCount,
		CreatedAt:  notification.CreatedAt,
		UpdatedAt:  notification.UpdatedAt,
		LastError:  notification.LastError,
	}, nil
}

func (s *Service) GetQueue() <-chan *storage.Notification {
	return s.queue
}

func (s *Service) Close() error {
	close(s.queue)
	return s.storage.Close()
}

// Simple ID generator - in production, use UUID or similar
func generateID() string {
	return fmt.Sprintf("notif_%d", time.Now().UnixNano())
}
