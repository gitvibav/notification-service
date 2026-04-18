package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &SQLiteStorage{db: db}
	if err := storage.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return storage, nil
}

func (s *SQLiteStorage) init() error {
	query := `
	CREATE TABLE IF NOT EXISTS notifications (
		id TEXT PRIMARY KEY,
		channel TEXT NOT NULL,
		recipient TEXT NOT NULL,
		message TEXT NOT NULL,
		status TEXT NOT NULL,
		retry_count INTEGER DEFAULT 0,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		last_error TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
	CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at);
	`

	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStorage) Create(notification *Notification) error {
	query := `
	INSERT INTO notifications (id, channel, recipient, message, status, retry_count, created_at, updated_at, last_error)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	notification.CreatedAt = now
	notification.UpdatedAt = now

	_, err := s.db.Exec(query,
		notification.ID,
		string(notification.Channel),
		notification.Recipient,
		notification.Message,
		string(notification.Status),
		notification.RetryCount,
		notification.CreatedAt,
		notification.UpdatedAt,
		notification.LastError,
	)

	return err
}

func (s *SQLiteStorage) Get(id string) (*Notification, error) {
	query := `
	SELECT id, channel, recipient, message, status, retry_count, created_at, updated_at, last_error
	FROM notifications
	WHERE id = ?
	`

	var notification Notification
	var channelStr, statusStr string

	err := s.db.QueryRow(query, id).Scan(
		&notification.ID,
		&channelStr,
		&notification.Recipient,
		&notification.Message,
		&statusStr,
		&notification.RetryCount,
		&notification.CreatedAt,
		&notification.UpdatedAt,
		&notification.LastError,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("notification with ID %s not found", id)
		}
		return nil, err
	}

	notification.Channel = Channel(channelStr)
	notification.Status = Status(statusStr)

	return &notification, nil
}

func (s *SQLiteStorage) Update(notification *Notification) error {
	query := `
	UPDATE notifications
	SET channel = ?, recipient = ?, message = ?, status = ?, retry_count = ?, updated_at = ?, last_error = ?
	WHERE id = ?
	`

	notification.UpdatedAt = time.Now()

	_, err := s.db.Exec(query,
		string(notification.Channel),
		notification.Recipient,
		notification.Message,
		string(notification.Status),
		notification.RetryCount,
		notification.UpdatedAt,
		notification.LastError,
		notification.ID,
	)

	return err
}

func (s *SQLiteStorage) ListByStatus(status Status) ([]*Notification, error) {
	query := `
	SELECT id, channel, recipient, message, status, retry_count, created_at, updated_at, last_error
	FROM notifications
	WHERE status = ?
	ORDER BY created_at ASC
	`

	rows, err := s.db.Query(query, string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*Notification
	for rows.Next() {
		var notification Notification
		var channelStr, statusStr string

		err := rows.Scan(
			&notification.ID,
			&channelStr,
			&notification.Recipient,
			&notification.Message,
			&statusStr,
			&notification.RetryCount,
			&notification.CreatedAt,
			&notification.UpdatedAt,
			&notification.LastError,
		)

		if err != nil {
			return nil, err
		}

		notification.Channel = Channel(channelStr)
		notification.Status = Status(statusStr)
		notifications = append(notifications, &notification)
	}

	return notifications, rows.Err()
}

func (s *SQLiteStorage) ListPending() ([]*Notification, error) {
	return s.ListByStatus(StatusPending)
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}
