package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/config"
	"github.com/alexandrmotologa/uploadgate/internal/db"
)

func TestSignatureGenerationAndVerification(t *testing.T) {
	secret := "whsec_test_secret_key"
	payload := []byte(`{"event":"upload.completed","upload_id":"upl_123"}`)

	testTimestamp := int64(1731089200)
	testSig := GenerateSignature(secret, testTimestamp, payload)
	headerVal := "t=1731089200,v1=" + testSig

	if !VerifySignature(secret, headerVal, payload) {
		t.Errorf("expected signature to verify successfully")
	}

	if VerifySignature(secret, "t=1731089200,v1=tampered", payload) {
		t.Errorf("expected tampered signature to be rejected")
	}

	if VerifySignature(secret, "invalid_header", payload) {
		t.Errorf("expected invalid header to be rejected")
	}
}

func TestWebhookDispatcherDelivery(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		sigHeader := r.Header.Get("X-UploadGate-Signature")
		if sigHeader == "" {
			t.Errorf("expected X-UploadGate-Signature header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	database, err := db.OpenDatabase(filepath.Join(tmpDir, "test_wh.db"))
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer database.Close()
	repo := db.NewRepository(database)

	cfg := &config.WebhooksConfig{
		Endpoint:       server.URL,
		Secret:         "whsec_test_secret",
		MaxRetries:     2,
		TimeoutSeconds: 5,
	}

	dispatcher := NewDispatcher(repo, cfg)
	defer dispatcher.Stop()

	dispatcher.Dispatch("upl_test_999", "upload.completed", map[string]interface{}{
		"status": "verified",
	})

	// Wait up to 1 second for worker to process queue
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&callCount) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if atomic.LoadInt32(&callCount) == 0 {
		t.Errorf("expected webhook endpoint to be called at least once")
	}

	// Verify delivery was recorded in DB
	deliveries, err := repo.ListWebhookDeliveries(context.Background(), 10)
	if err != nil {
		t.Fatalf("failed to list webhook deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 recorded delivery, got %d", len(deliveries))
	}
	if deliveries[0].StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", deliveries[0].StatusCode)
	}
}
