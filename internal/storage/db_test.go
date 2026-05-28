package storage

import (
	"path/filepath"
	"testing"

	"claude-bridge/internal/storage/models"
)

func TestOpenDatabase_TemporarySQLiteFile_AutoMigrates(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "usage.db")

	db, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if !db.Migrator().HasTable(&models.Usage{}) {
		t.Fatalf("expected usage table to exist after automigrate")
	}
}
