package storage

import (
	"fmt"

	"notification-service/internal/config"
)

func NewStorage(cfg *config.Config) (Storage, error) {
	switch cfg.Database.Type {
	case "memory":
		return NewMemoryStorage(), nil
	case "sqlite":
		return NewSQLiteStorage(cfg.Database.Path)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}
}
