package storage

import (
	"claude-bridge/internal/storage/models"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func OpenDatabase(path string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}
	if err := db.AutoMigrate(&models.Usage{}); err != nil {
		return nil, fmt.Errorf("failed to auto migrate usage model: %w", err)
	}
	return db, nil
}
