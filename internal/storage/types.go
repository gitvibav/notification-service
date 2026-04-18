package storage

import (
	"time"
)

type Channel string

const (
	ChannelEmail Channel = "email"
	ChannelSMS   Channel = "sms"
	ChannelPush  Channel = "push"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusSent    Status = "sent"
	StatusFailed  Status = "failed"
)

type Notification struct {
	ID        string    `json:"id"`
	Channel   Channel   `json:"channel"`
	Recipient string    `json:"recipient"`
	Message   string    `json:"message"`
	Status    Status    `json:"status"`
	RetryCount int      `json:"retry_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	LastError string    `json:"last_error,omitempty"`
}

type Storage interface {
	Create(notification *Notification) error
	Get(id string) (*Notification, error)
	Update(notification *Notification) error
	ListByStatus(status Status) ([]*Notification, error)
	ListPending() ([]*Notification, error)
	Close() error
}
