package storage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStorage_Create(t *testing.T) {
	storage := NewMemoryStorage()

	notification := &Notification{
		ID:         "test-1",
		Channel:    ChannelEmail,
		Recipient:  "test@example.com",
		Message:    "Test message",
		Status:     StatusPending,
		RetryCount: 0,
	}

	err := storage.Create(notification)
	require.NoError(t, err)

	// Test duplicate
	err = storage.Create(notification)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestMemoryStorage_Get(t *testing.T) {
	storage := NewMemoryStorage()

	// Test non-existent
	_, err := storage.Get("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test existing
	notification := &Notification{
		ID:         "test-1",
		Channel:    ChannelEmail,
		Recipient:  "test@example.com",
		Message:    "Test message",
		Status:     StatusPending,
		RetryCount: 0,
	}

	err = storage.Create(notification)
	require.NoError(t, err)

	retrieved, err := storage.Get("test-1")
	require.NoError(t, err)
	assert.Equal(t, notification.ID, retrieved.ID)
	assert.Equal(t, notification.Channel, retrieved.Channel)
	assert.Equal(t, notification.Recipient, retrieved.Recipient)
	assert.Equal(t, notification.Message, retrieved.Message)
	assert.Equal(t, notification.Status, retrieved.Status)
	assert.NotZero(t, retrieved.CreatedAt)
	assert.NotZero(t, retrieved.UpdatedAt)
}

func TestMemoryStorage_Update(t *testing.T) {
	storage := NewMemoryStorage()

	notification := &Notification{
		ID:         "test-1",
		Channel:    ChannelEmail,
		Recipient:  "test@example.com",
		Message:    "Test message",
		Status:     StatusPending,
		RetryCount: 0,
	}

	err := storage.Create(notification)
	require.NoError(t, err)

	// Update status
	notification.Status = StatusSent
	notification.RetryCount = 1
	notification.LastError = ""

	err = storage.Update(notification)
	require.NoError(t, err)

	retrieved, err := storage.Get("test-1")
	require.NoError(t, err)
	assert.Equal(t, StatusSent, retrieved.Status)
	assert.Equal(t, 1, retrieved.RetryCount)
	assert.True(t, retrieved.UpdatedAt.After(retrieved.CreatedAt))

	// Test update non-existent
	nonExistent := &Notification{
		ID: "non-existent",
	}
	err = storage.Update(nonExistent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMemoryStorage_ListByStatus(t *testing.T) {
	storage := NewMemoryStorage()

	// Create notifications with different statuses
	notifications := []*Notification{
		{ID: "test-1", Channel: ChannelEmail, Recipient: "test1@example.com", Message: "Test 1", Status: StatusPending, RetryCount: 0},
		{ID: "test-2", Channel: ChannelSMS, Recipient: "test2@example.com", Message: "Test 2", Status: StatusSent, RetryCount: 0},
		{ID: "test-3", Channel: ChannelPush, Recipient: "test3@example.com", Message: "Test 3", Status: StatusPending, RetryCount: 0},
		{ID: "test-4", Channel: ChannelEmail, Recipient: "test4@example.com", Message: "Test 4", Status: StatusFailed, RetryCount: 3},
	}

	for _, notif := range notifications {
		err := storage.Create(notif)
		require.NoError(t, err)
	}

	// Test pending notifications
	pending, err := storage.ListByStatus(StatusPending)
	require.NoError(t, err)
	assert.Len(t, pending, 2)

	pendingIDs := make(map[string]bool)
	for _, notif := range pending {
		pendingIDs[notif.ID] = true
	}
	assert.True(t, pendingIDs["test-1"])
	assert.True(t, pendingIDs["test-3"])
	assert.False(t, pendingIDs["test-2"])
	assert.False(t, pendingIDs["test-4"])

	// Test sent notifications
	sent, err := storage.ListByStatus(StatusSent)
	require.NoError(t, err)
	assert.Len(t, sent, 1)
	assert.Equal(t, "test-2", sent[0].ID)

	// Test failed notifications
	failed, err := storage.ListByStatus(StatusFailed)
	require.NoError(t, err)
	assert.Len(t, failed, 1)
	assert.Equal(t, "test-4", failed[0].ID)
}

func TestMemoryStorage_ListPending(t *testing.T) {
	storage := NewMemoryStorage()

	// Create notifications with different statuses
	notifications := []*Notification{
		{ID: "test-1", Channel: ChannelEmail, Recipient: "test1@example.com", Message: "Test 1", Status: StatusPending, RetryCount: 0},
		{ID: "test-2", Channel: ChannelSMS, Recipient: "test2@example.com", Message: "Test 2", Status: StatusSent, RetryCount: 0},
		{ID: "test-3", Channel: ChannelPush, Recipient: "test3@example.com", Message: "Test 3", Status: StatusPending, RetryCount: 0},
	}

	for _, notif := range notifications {
		err := storage.Create(notif)
		require.NoError(t, err)
	}

	pending, err := storage.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, 2)

	pendingIDs := make(map[string]bool)
	for _, notif := range pending {
		pendingIDs[notif.ID] = true
	}
	assert.True(t, pendingIDs["test-1"])
	assert.True(t, pendingIDs["test-3"])
	assert.False(t, pendingIDs["test-2"])
}

func TestMemoryStorage_ConcurrentAccess(t *testing.T) {
	storage := NewMemoryStorage()

	// Test concurrent writes
	const numGoroutines = 10
	const numNotifications = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(startID int) {
			defer func() { done <- true }()
			for j := 0; j < numNotifications; j++ {
				id := startID*numNotifications + j
				notification := &Notification{
					ID:         fmt.Sprintf("test-%d", id),
					Channel:    ChannelEmail,
					Recipient:  fmt.Sprintf("test%d@example.com", id),
					Message:    fmt.Sprintf("Test message %d", id),
					Status:     StatusPending,
					RetryCount: 0,
				}
				storage.Create(notification)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all notifications were created
	pending, err := storage.ListPending()
	require.NoError(t, err)
	assert.Len(t, pending, numGoroutines*numNotifications)
}
