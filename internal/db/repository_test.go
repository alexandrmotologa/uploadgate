package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRepositoryCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_uploadgate.db")

	database, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer database.Close()

	repo := NewRepository(database)
	ctx := context.Background()

	// 1. Create upload
	upload := &Upload{
		ID:              "upl_test_123",
		Filename:        "photo.jpg",
		OriginalSize:    10240,
		DeclaredMIME:    "image/jpeg",
		StorageProvider: "local",
		StorageBucket:   "default",
		StorageKey:      "uploads/photo.jpg",
		Status:          StatusPending,
	}

	if err := repo.CreateUpload(ctx, upload); err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// 2. Fetch upload
	fetched, err := repo.GetUpload(ctx, "upl_test_123")
	if err != nil {
		t.Fatalf("GetUpload failed: %v", err)
	}
	if fetched == nil {
		t.Fatalf("expected upload to be found")
	}
	if fetched.Filename != "photo.jpg" {
		t.Errorf("expected photo.jpg, got %s", fetched.Filename)
	}
	if fetched.Status != StatusPending {
		t.Errorf("expected pending status, got %s", fetched.Status)
	}

	// 3. Update verification
	fetched.Status = StatusVerified
	fetched.DetectedMIME = "image/jpeg"
	fetched.SHA256 = "abc123sha256"
	fetched.Width = 1920
	fetched.Height = 1080
	fetched.PublicURL = "http://localhost:8080/v1/storage/local/uploads/photo.jpg"

	if err := repo.UpdateUploadVerification(ctx, fetched); err != nil {
		t.Fatalf("UpdateUploadVerification failed: %v", err)
	}

	// 4. Create thumbnail
	thumb := &Thumbnail{
		UploadID:    "upl_test_123",
		VariantName: "thumb",
		Format:      "webp",
		Width:       150,
		Height:      150,
		StorageKey:  "thumbnails/photo_150x150.webp",
		PublicURL:   "http://localhost:8080/v1/storage/local/thumbnails/photo_150x150.webp",
		SizeBytes:   2048,
	}
	if err := repo.CreateThumbnail(ctx, thumb); err != nil {
		t.Fatalf("CreateThumbnail failed: %v", err)
	}

	// 5. Re-fetch and check thumbnails
	updated, err := repo.GetUpload(ctx, "upl_test_123")
	if err != nil {
		t.Fatalf("GetUpload failed: %v", err)
	}
	if updated.Status != StatusVerified {
		t.Errorf("expected verified status, got %s", updated.Status)
	}
	if len(updated.Thumbnails) != 1 {
		t.Fatalf("expected 1 thumbnail, got %d", len(updated.Thumbnails))
	}
	if updated.Thumbnails[0].VariantName != "thumb" {
		t.Errorf("expected variant thumb, got %s", updated.Thumbnails[0].VariantName)
	}

	// 6. Record Webhook Delivery
	wh := &WebhookDelivery{
		UploadID:     "upl_test_123",
		Event:        "upload.completed",
		Endpoint:     "https://api.example.com/webhook",
		Payload:      `{"event":"upload.completed"}`,
		StatusCode:   200,
		Attempt:      1,
		ResponseBody: `{"received":true}`,
	}
	if err := repo.RecordWebhookDelivery(ctx, wh); err != nil {
		t.Fatalf("RecordWebhookDelivery failed: %v", err)
	}

	deliveries, err := repo.ListWebhookDeliveries(ctx, 10)
	if err != nil {
		t.Fatalf("ListWebhookDeliveries failed: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 webhook delivery, got %d", len(deliveries))
	}
}
