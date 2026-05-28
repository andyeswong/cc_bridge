package repository

import (
	"path/filepath"
	"testing"
	"time"

	"claude-bridge/internal/storage"
	"claude-bridge/internal/storage/models"
)

func TestUsageRepository_Create(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "usage.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := NewUsageRepository(db)

	record := &models.Usage{
		Model:            "claude-code",
		Provider:         "claude",
		PromptTokens:     1,
		CompletionTokens: 2,
		TotalTokens:      3,
		DurationMs:       100,
		IsError:          false,
	}
	if err := repo.Create(record); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if record.ID == 0 {
		t.Fatalf("expected ID to be set")
	}
}

func TestUsageRepository_Summary(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "usage.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := NewUsageRepository(db)

	records := []*models.Usage{
		{
			Model:            "m1",
			Provider:         "claude",
			PromptTokens:     1,
			CompletionTokens: 2,
			TotalTokens:      3,
			DurationMs:       100,
			IsError:          false,
			CreatedAt:        time.Now().Add(-3 * time.Minute),
		},
		{
			Model:            "m1",
			Provider:         "claude",
			PromptTokens:     2,
			CompletionTokens: 3,
			TotalTokens:      5,
			DurationMs:       300,
			IsError:          true,
			CreatedAt:        time.Now().Add(-2 * time.Minute),
		},
		{
			Model:            "m2",
			Provider:         "ollama",
			PromptTokens:     4,
			CompletionTokens: 5,
			TotalTokens:      9,
			DurationMs:       50,
			IsError:          false,
			CreatedAt:        time.Now().Add(-1 * time.Minute),
		},
	}

	for _, rec := range records {
		if err := repo.Create(rec); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	rows, err := repo.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d want %d", len(rows), 2)
	}

	if rows[0].Model != "m1" || rows[0].Provider != "claude" {
		t.Fatalf("first row group: got %s/%s", rows[0].Model, rows[0].Provider)
	}
	if rows[0].TotalRequests != 2 {
		t.Fatalf("total_requests: got %d want %d", rows[0].TotalRequests, 2)
	}
	if rows[0].Errors != 1 {
		t.Fatalf("errors: got %d want %d", rows[0].Errors, 1)
	}
	if rows[0].PromptTokens != 3 || rows[0].CompletionTokens != 5 || rows[0].TotalTokens != 8 {
		t.Fatalf("tokens: got prompt=%d completion=%d total=%d", rows[0].PromptTokens, rows[0].CompletionTokens, rows[0].TotalTokens)
	}
	if rows[0].AvgDurationMs != 200 {
		t.Fatalf("avg_duration_ms: got %v want %v", rows[0].AvgDurationMs, 200.0)
	}
}

func TestUsageRepository_Summary_EmptyTableReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "usage.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := NewUsageRepository(db)

	rows, err := repo.Summary()
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(rows))
	}
}

func TestUsageRepository_FindRecent(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "usage.db")
	db, err := storage.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := NewUsageRepository(db)

	base := time.Now().Add(-10 * time.Minute)

	records := []*models.Usage{
		{Model: "m", Provider: "claude", DurationMs: 1, CreatedAt: base.Add(1 * time.Second)},
		{Model: "m", Provider: "claude", DurationMs: 2, CreatedAt: base.Add(2 * time.Second)},
		{Model: "m", Provider: "claude", DurationMs: 3, CreatedAt: base.Add(3 * time.Second)},
	}

	for _, rec := range records {
		if err := repo.Create(rec); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	recent, err := repo.FindRecent(2)
	if err != nil {
		t.Fatalf("FindRecent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("recent: got %d want %d", len(recent), 2)
	}
	if recent[0].DurationMs != 3 || recent[1].DurationMs != 2 {
		t.Fatalf("order: got %d then %d", recent[0].DurationMs, recent[1].DurationMs)
	}
}
