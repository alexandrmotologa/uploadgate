package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLocalProviderPresignAndUpload(t *testing.T) {
	tmpDir := t.TempDir()
	p, err := NewLocalProvider("test_local", tmpDir, "http://localhost:8080/v1/storage/local", "test_secret_123")
	if err != nil {
		t.Fatalf("failed to create local provider: %v", err)
	}

	ctx := context.Background()
	key := "uploads/2026/test_file.txt"
	mime := "text/plain"
	content := []byte("Hello UploadGate local storage!")

	// 1. Presign
	presignRes, err := p.PresignPut(ctx, key, mime, int64(len(content)), 10*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut failed: %v", err)
	}

	if presignRes.Method != "PUT" {
		t.Errorf("expected PUT method, got %s", presignRes.Method)
	}
	if !strings.Contains(presignRes.URL, "/upload/") {
		t.Errorf("expected URL to contain /upload/, got %s", presignRes.URL)
	}

	// Extract token from URL
	parts := strings.Split(presignRes.URL, "/upload/")
	if len(parts) != 2 {
		t.Fatalf("failed to extract token from URL %s", presignRes.URL)
	}
	token := parts[1]

	// 2. Simulate HTTP PUT direct upload
	req := httptest.NewRequest("PUT", "/v1/storage/local/upload/"+token, bytes.NewReader(content))
	req.Header.Set("Content-Type", mime)
	w := httptest.NewRecorder()

	p.HandleUpload(w, req, token)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("HandleUpload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 3. Read back with GetObject
	rc, size, err := p.GetObject(ctx, key)
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}

	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}

	readBytes, err := io.ReadAll(rc)
	if err != nil {
		rc.Close()
		t.Fatalf("failed to read bytes: %v", err)
	}
	rc.Close() // Close file handle before deleting (critical on Windows)

	if string(readBytes) != string(content) {
		t.Errorf("content mismatch: got %q, expected %q", string(readBytes), string(content))
	}

	// 4. Test Health Check
	if err := p.CheckHealth(ctx); err != nil {
		t.Errorf("health check failed: %v", err)
	}

	// 5. Delete Object
	if err := p.DeleteObject(ctx, key); err != nil {
		t.Errorf("DeleteObject failed: %v", err)
	}

	_, _, err = p.GetObject(ctx, key)
	if !os.IsNotExist(err) && err == nil {
		t.Errorf("expected file not to exist after deletion")
	}
}
