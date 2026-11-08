package gc

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/config"
	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/alexandrmotologa/uploadgate/internal/storage"
)

func TestCleanerSweep(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gc_test.db")
	storageDir := filepath.Join(tmpDir, "local_storage")

	database, err := db.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer database.Close()

	repo := db.NewRepository(database)

	cfg := config.DefaultConfig()
	cfg.Storage.Providers["local"] = config.StorageProviderConfig{
		Type:          "local",
		Directory:     storageDir,
		PublicBaseURL: "http://localhost:8080/v1/storage/local",
	}

	storageMgr, err := storage.NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	ctx := context.Background()
	prov, _ := storageMgr.Get("local")

	// 1. Create expired upload (TTL in the past)
	past := time.Now().UTC().Add(-10 * time.Minute)
	err = prov.PutObject(ctx, "uploads/expired.jpg", strings.NewReader("dummy content"), 13, "image/jpeg")
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	expiredUpload := &db.Upload{
		ID:              "upl_expired_1",
		Filename:        "expired.jpg",
		OriginalSize:    13,
		DeclaredMIME:    "image/jpeg",
		StorageProvider: "local",
		StorageKey:      "uploads/expired.jpg",
		Status:          db.StatusVerified,
		ExpiresAt:       &past,
	}
	_ = repo.CreateUpload(ctx, expiredUpload)

	// 2. Create valid active upload (TTL in the future)
	future := time.Now().UTC().Add(2 * time.Hour)
	_ = prov.PutObject(ctx, "uploads/active.jpg", strings.NewReader("active content"), 14, "image/jpeg")
	activeUpload := &db.Upload{
		ID:              "upl_active_1",
		Filename:        "active.jpg",
		OriginalSize:    14,
		DeclaredMIME:    "image/jpeg",
		StorageProvider: "local",
		StorageKey:      "uploads/active.jpg",
		Status:          db.StatusVerified,
		ExpiresAt:       &future,
	}
	_ = repo.CreateUpload(ctx, activeUpload)

	// 3. Run sweep
	cleaner := NewCleaner(repo, storageMgr, 1*time.Minute, 2*time.Hour)
	cleaner.sweep()

	// 4. Assertions
	fetchedExpired, _ := repo.GetUpload(ctx, "upl_expired_1")
	if fetchedExpired != nil {
		t.Errorf("expected expired upload to be deleted from db")
	}

	fetchedActive, _ := repo.GetUpload(ctx, "upl_active_1")
	if fetchedActive == nil {
		t.Errorf("expected active upload to remain in db")
	}
}
