package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/google/uuid"
)

// BatchPresignRequest defines the batch presigning payload.
type BatchPresignRequest struct {
	Bucket string           `json:"bucket,omitempty"`
	Files  []PresignRequest `json:"files"`
}

// BatchPresignItemResult contains either the presigned ticket or an error for each file.
type BatchPresignItemResult struct {
	Filename     string            `json:"filename"`
	UploadID     string            `json:"upload_id,omitempty"`
	PresignedURL string            `json:"presigned_url,omitempty"`
	Method       string            `json:"method,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	StorageKey   string            `json:"storage_key,omitempty"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// BatchPresignResponse wraps the list of generated tickets.
type BatchPresignResponse struct {
	Items []BatchPresignItemResult `json:"items"`
}

// HandlePresignBatch processes POST /v1/uploads/presign-batch
func (s *Server) HandlePresignBatch(w http.ResponseWriter, r *http.Request) {
	var req BatchPresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload: " + err.Error()})
		return
	}

	if len(req.Files) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "files array must not be empty"})
		return
	}

	if len(req.Files) > 100 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "batch size exceeds maximum limit of 100 files"})
		return
	}

	prov, err := s.storageMgr.Get(req.Bucket)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	var results []BatchPresignItemResult

	for _, f := range req.Files {
		filename := strings.TrimSpace(f.Filename)
		if filename == "" {
			results = append(results, BatchPresignItemResult{
				Filename: f.Filename,
				Error:    "filename is required",
			})
			continue
		}

		if f.SizeBytes <= 0 {
			results = append(results, BatchPresignItemResult{
				Filename: filename,
				Error:    "size_bytes must be greater than zero",
			})
			continue
		}

		if f.SizeBytes > s.cfg.Limits.MaxFileSizeBytes {
			results = append(results, BatchPresignItemResult{
				Filename: filename,
				Error:    fmt.Sprintf("file size %d bytes exceeds maximum limit of %d bytes", f.SizeBytes, s.cfg.Limits.MaxFileSizeBytes),
			})
			continue
		}

		mime := f.MIMEType
		if mime == "" {
			mime = "application/octet-stream"
		}

		if !s.cfg.Limits.IsMIMEAllowed(mime) {
			results = append(results, BatchPresignItemResult{
				Filename: filename,
				Error:    fmt.Sprintf("mime type %q is not permitted by policy", mime),
			})
			continue
		}

		ext := filepath.Ext(filename)
		safeBase := strings.TrimSuffix(filepath.Base(filename), ext)
		cleanSafeBase := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				return r
			}
			return '_'
		}, safeBase)

		uploadUUID := uuid.New().String()
		storageKey := fmt.Sprintf("uploads/%04d/%02d/%02d/%s_%s%s",
			now.Year(), now.Month(), now.Day(),
			uploadUUID[:8], cleanSafeBase, ext,
		)

		presignResult, err := prov.PresignPut(r.Context(), storageKey, mime, f.SizeBytes, s.cfg.Limits.PresignExpiry())
		if err != nil {
			results = append(results, BatchPresignItemResult{
				Filename: filename,
				Error:    "presign failed: " + err.Error(),
			})
			continue
		}

		metaJSON, _ := json.Marshal(f.Metadata)
		uploadRecord := &db.Upload{
			ID:              presignResult.UploadID,
			Filename:        filename,
			OriginalSize:    f.SizeBytes,
			DeclaredMIME:    mime,
			StorageProvider: prov.Name(),
			StorageBucket:   req.Bucket,
			StorageKey:      storageKey,
			PublicURL:       prov.PublicURL(storageKey),
			Status:          db.StatusPending,
			MetadataJSON:    string(metaJSON),
		}

		if err := s.repo.CreateUpload(r.Context(), uploadRecord); err != nil {
			results = append(results, BatchPresignItemResult{
				Filename: filename,
				Error:    "database error: " + err.Error(),
			})
			continue
		}

		s.metrics.PresignRequestsTotal.WithLabelValues(prov.Name()).Inc()
		s.sseHub.Broadcast("upload.created", uploadRecord)

		exp := presignResult.ExpiresAt
		results = append(results, BatchPresignItemResult{
			Filename:     filename,
			UploadID:     presignResult.UploadID,
			PresignedURL: presignResult.URL,
			Method:       presignResult.Method,
			Headers:      presignResult.Headers,
			StorageKey:   storageKey,
			ExpiresAt:    &exp,
		})
	}

	respondJSON(w, http.StatusOK, BatchPresignResponse{Items: results})
}
