package storage

import (
	"fmt"
	"sync"
	"time"
)

type MemoryStorage struct {
	mu            sync.RWMutex
	notifications map[string]*Notification
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		notifications: make(map[string]*Notification),
	}
}

func (m *MemoryStorage) Create(notification *Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.notifications[notification.ID]; exists {
		return fmt.Errorf("notification with ID %s already exists", notification.ID)
	}

	now := time.Now()
	notification.CreatedAt = now
	notification.UpdatedAt = now

	m.notifications[notification.ID] = notification
	return nil
}

func (m *MemoryStorage) Get(id string) (*Notification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	notification, exists := m.notifications[id]
	if !exists {
		return nil, fmt.Errorf("notification with ID %s not found", id)
	}

	// Return a copy to avoid race conditions
	copy := *notification
	return &copy, nil
}

func (m *MemoryStorage) Update(notification *Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.notifications[notification.ID]
	if !exists {
		return fmt.Errorf("notification with ID %s not found", notification.ID)
	}

	// Update timestamp
	notification.UpdatedAt = time.Now()

	m.notifications[notification.ID] = notification
	_ = existing // avoid unused variable warning
	return nil
}

func (m *MemoryStorage) ListByStatus(status Status) ([]*Notification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Notification
	for _, notification := range m.notifications {
		if notification.Status == status {
			// Return copies to avoid race conditions
			copy := *notification
			result = append(result, &copy)
		}
	}

	return result, nil
}

func (m *MemoryStorage) ListPending() ([]*Notification, error) {
	return m.ListByStatus(StatusPending)
}

func (m *MemoryStorage) Close() error {
	// No cleanup needed for in-memory storage
	return nil
}
