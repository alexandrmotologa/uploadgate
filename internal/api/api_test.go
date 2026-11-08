package api

import (
	"bytes"
	"encoding/json"
	"image/color"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandrmotologa/uploadgate/internal/config"
	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/alexandrmotologa/uploadgate/internal/processor"
	"github.com/alexandrmotologa/uploadgate/internal/storage"
	"github.com/alexandrmotologa/uploadgate/internal/webhook"
)

func setupTestServer(t *testing.T) (*Server, http.Handler, *config.Config) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "api_test.db")
	storageDir := filepath.Join(tmpDir, "local_storage")

	cfg := config.DefaultConfig()
	cfg.Server.APIKey = "test_api_key_123"
	cfg.Server.DBPath = dbPath
	cfg.Storage.Providers["local"] = config.StorageProviderConfig{
		Type:          "local",
		Directory:     storageDir,
		PublicBaseURL: "http://localhost:8080/v1/storage/local",
	}

	database, err := db.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}

	repo := db.NewRepository(database)
	storageMgr, err := storage.NewManager(cfg)
	if err != nil {
		database.Close()
		t.Fatalf("NewManager failed: %v", err)
	}

	webhookDispatcher := webhook.NewDispatcher(repo, &cfg.Webhooks)

	server := NewServer(ServerOptions{
		Config:            cfg,
		Repo:              repo,
		StorageMgr:        storageMgr,
		WebhookDispatcher: webhookDispatcher,
	})

	t.Cleanup(func() {
		webhookDispatcher.Stop()
		database.Close()
	})

	return server, server.Router(), cfg
}

func TestEndToEndUploadLifecycle(t *testing.T) {
	_, handler, _ := setupTestServer(t)

	// Step 1: Request presigned ticket for valid PNG
	pngData, err := processor.CreateTestImage(640, 480, color.RGBA{R: 0, G: 128, B: 255, A: 255})
	if err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	presignReqBody := map[string]interface{}{
		"filename":   "banner.png",
		"size_bytes": len(pngData),
		"mime_type":  "image/png",
		"bucket":     "local",
		"metadata": map[string]string{
			"project": "uploadgate",
		},
	}
	reqBytes, _ := json.Marshal(presignReqBody)

	req := httptest.NewRequest("POST", "/v1/uploads/presign", bytes.NewReader(reqBytes))
	req.Header.Set("Authorization", "Bearer test_api_key_123")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("presign failed: code %d, body: %s", rec.Code, rec.Body.String())
	}

	var presignResp storage.PresignResult
	if err := json.NewDecoder(rec.Body).Decode(&presignResp); err != nil {
		t.Fatalf("failed to parse presign response: %v", err)
	}

	if presignResp.UploadID == "" || presignResp.URL == "" {
		t.Fatalf("invalid presign response: %+v", presignResp)
	}

	// Extract local token
	parts := strings.Split(presignResp.URL, "/upload/")
	if len(parts) != 2 {
		t.Fatalf("invalid local presigned URL: %s", presignResp.URL)
	}
	token := parts[1]

	// Step 2: Direct upload to local storage endpoint
	uploadReq := httptest.NewRequest("PUT", "/v1/storage/local/upload/"+token, bytes.NewReader(pngData))
	uploadReq.Header.Set("Content-Type", "image/png")
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusOK {
		t.Fatalf("direct upload failed: code %d, body: %s", uploadRec.Code, uploadRec.Body.String())
	}

	// Step 3: Trigger completion and verification pipeline
	completeReq := httptest.NewRequest("POST", "/v1/uploads/"+presignResp.UploadID+"/complete", strings.NewReader("{}"))
	completeReq.Header.Set("Authorization", "Bearer test_api_key_123")
	completeReq.Header.Set("Content-Type", "application/json")
	completeRec := httptest.NewRecorder()
	handler.ServeHTTP(completeRec, completeReq)

	if completeRec.Code != http.StatusOK {
		t.Fatalf("completion failed: code %d, body: %s", completeRec.Code, completeRec.Body.String())
	}

	var verifiedRecord db.Upload
	if err := json.NewDecoder(completeRec.Body).Decode(&verifiedRecord); err != nil {
		t.Fatalf("failed to decode complete response: %v", err)
	}

	if verifiedRecord.Status != db.StatusVerified {
		t.Errorf("expected verified status, got %s", verifiedRecord.Status)
	}
	if verifiedRecord.Width != 640 || verifiedRecord.Height != 480 {
		t.Errorf("expected 640x480 dimensions, got %dx%d", verifiedRecord.Width, verifiedRecord.Height)
	}
	if verifiedRecord.DetectedMIME != "image/png" {
		t.Errorf("expected detected mime image/png, got %s", verifiedRecord.DetectedMIME)
	}
	if len(verifiedRecord.Thumbnails) == 0 {
		t.Errorf("expected thumbnails to be generated")
	}

	// Step 4: Get upload by ID
	getReq := httptest.NewRequest("GET", "/v1/uploads/"+presignResp.UploadID, nil)
	getReq.Header.Set("Authorization", "Bearer test_api_key_123")
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Errorf("get upload failed: code %d", getRec.Code)
	}

	// Step 5: List uploads
	listReq := httptest.NewRequest("GET", "/v1/uploads?status=verified", nil)
	listReq.Header.Set("Authorization", "Bearer test_api_key_123")
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Errorf("list uploads failed: code %d", listRec.Code)
	}
}

func TestDisguisedExecutableRejection(t *testing.T) {
	_, handler, _ := setupTestServer(t)

	// Disguised payload: MZ header (.exe) claiming to be profile.png
	exePayload := []byte{0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00, 0x04, 0x00}

	presignReqBody := map[string]interface{}{
		"filename":   "profile.png",
		"size_bytes": len(exePayload),
		"mime_type":  "image/png",
		"bucket":     "local",
	}
	reqBytes, _ := json.Marshal(presignReqBody)

	req := httptest.NewRequest("POST", "/v1/uploads/presign", bytes.NewReader(reqBytes))
	req.Header.Set("Authorization", "Bearer test_api_key_123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var presignResp storage.PresignResult
	_ = json.NewDecoder(rec.Body).Decode(&presignResp)
	token := strings.Split(presignResp.URL, "/upload/")[1]

	// Direct upload fake payload
	uploadReq := httptest.NewRequest("PUT", "/v1/storage/local/upload/"+token, bytes.NewReader(exePayload))
	uploadRec := httptest.NewRecorder()
	handler.ServeHTTP(uploadRec, uploadReq)

	// Trigger complete
	completeReq := httptest.NewRequest("POST", "/v1/uploads/"+presignResp.UploadID+"/complete", strings.NewReader("{}"))
	completeReq.Header.Set("Authorization", "Bearer test_api_key_123")
	completeRec := httptest.NewRecorder()
	handler.ServeHTTP(completeRec, completeReq)

	if completeRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity for disguised executable, got %d", completeRec.Code)
	}

	body, _ := io.ReadAll(completeRec.Body)
	if !strings.Contains(string(body), "binary executable detected") {
		t.Errorf("expected security violation error in response, got: %s", string(body))
	}
}

func TestFromURLSSRFProtection(t *testing.T) {
	_, handler, _ := setupTestServer(t)

	// Attempting to ingest internal metadata
	body := map[string]string{
		"url": "http://169.254.169.254/latest/meta-data/",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/uploads/from-url", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test_api_key_123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for SSRF attempt, got %d", rec.Code)
	}

	respBody, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(respBody), "ssrf blocked") {
		t.Errorf("expected ssrf blocked message, got: %s", string(respBody))
	}
}

func TestPresignBatch(t *testing.T) {
	_, handler, _ := setupTestServer(t)

	batchBody := map[string]interface{}{
		"bucket": "local",
		"files": []map[string]interface{}{
			{
				"filename":   "avatar.png",
				"size_bytes": 1024,
				"mime_type":  "image/png",
			},
			{
				"filename":   "document.pdf",
				"size_bytes": 4096,
				"mime_type":  "application/pdf",
			},
		},
	}
	bodyBytes, _ := json.Marshal(batchBody)

	req := httptest.NewRequest("POST", "/v1/uploads/presign-batch", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer test_api_key_123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("presign-batch failed with status %d: %s", rec.Code, rec.Body.String())
	}

	var batchResp BatchPresignResponse
	if err := json.NewDecoder(rec.Body).Decode(&batchResp); err != nil {
		t.Fatalf("failed to decode batch response: %v", err)
	}

	if len(batchResp.Items) != 2 {
		t.Fatalf("expected 2 batch items, got %d", len(batchResp.Items))
	}

	for _, item := range batchResp.Items {
		if item.Error != "" {
			t.Errorf("unexpected error for %s: %s", item.Filename, item.Error)
		}
		if item.UploadID == "" || item.PresignedURL == "" {
			t.Errorf("missing upload ID or presigned URL for %s", item.Filename)
		}
	}
}

func TestHealthAndMetricsEndpoints(t *testing.T) {
	_, handler, _ := setupTestServer(t)

	// Health check
	healthReq := httptest.NewRequest("GET", "/healthz", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, healthReq)

	if healthRec.Code != http.StatusOK {
		t.Errorf("expected 200 for /healthz, got %d", healthRec.Code)
	}

	// Prometheus metrics
	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Errorf("expected 200 for /metrics, got %d", metricsRec.Code)
	}
	if !strings.Contains(metricsRec.Body.String(), "uploadgate_uploads_total") {
		t.Errorf("expected uploadgate_uploads_total in metrics output")
	}
}
