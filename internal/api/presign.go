package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/alexandrmotologa/uploadgate/internal/storage"
	"github.com/google/uuid"
)

// PresignRequest defines the input payload for single upload presigning.
type PresignRequest struct {
	Filename  string                 `json:"filename"`
	SizeBytes int64                  `json:"size_bytes"`
	MIMEType  string                 `json:"mime_type"`
	Bucket    string                 `json:"bucket,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MultipartPresignRequest defines the input payload for multipart chunk presigning.
type MultipartPresignRequest struct {
	Filename  string                 `json:"filename"`
	SizeBytes int64                  `json:"size_bytes"`
	MIMEType  string                 `json:"mime_type"`
	PartCount int                    `json:"part_count"`
	Bucket    string                 `json:"bucket,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HandlePresign processes POST /v1/uploads/presign
func (s *Server) HandlePresign(w http.ResponseWriter, r *http.Request) {
	var req PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload: " + err.Error()})
		return
	}

	req.Filename = strings.TrimSpace(req.Filename)
	if req.Filename == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "filename is required"})
		return
	}

	if req.SizeBytes <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "size_bytes must be greater than zero"})
		return
	}

	if req.SizeBytes > s.cfg.Limits.MaxFileSizeBytes {
		respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error": fmt.Sprintf("file size %d bytes exceeds maximum limit of %d bytes", req.SizeBytes, s.cfg.Limits.MaxFileSizeBytes),
		})
		return
	}

	if req.MIMEType == "" {
		req.MIMEType = "application/octet-stream"
	}

	if !s.cfg.Limits.IsMIMEAllowed(req.MIMEType) {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": fmt.Sprintf("mime type %q is not permitted by policy", req.MIMEType),
		})
		return
	}

	prov, err := s.storageMgr.Get(req.Bucket)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	ext := filepath.Ext(req.Filename)
	safeBase := strings.TrimSuffix(filepath.Base(req.Filename), ext)
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

	presignResult, err := prov.PresignPut(r.Context(), storageKey, req.MIMEType, req.SizeBytes, s.cfg.Limits.PresignExpiry())
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate presigned url: " + err.Error()})
		return
	}

	metaJSON, _ := json.Marshal(req.Metadata)
	uploadRecord := &db.Upload{
		ID:              presignResult.UploadID,
		Filename:        req.Filename,
		OriginalSize:    req.SizeBytes,
		DeclaredMIME:    req.MIMEType,
		StorageProvider: prov.Name(),
		StorageBucket:   req.Bucket,
		StorageKey:      storageKey,
		PublicURL:       prov.PublicURL(storageKey),
		Status:          db.StatusPending,
		MetadataJSON:    string(metaJSON),
	}

	if err := s.repo.CreateUpload(r.Context(), uploadRecord); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error: " + err.Error()})
		return
	}

	// Update metrics & SSE stream
	s.metrics.PresignRequestsTotal.WithLabelValues(prov.Name()).Inc()
	s.sseHub.Broadcast("upload.created", uploadRecord)

	respondJSON(w, http.StatusOK, presignResult)
}

// HandlePresignMultipart processes POST /v1/uploads/presign-multipart
func (s *Server) HandlePresignMultipart(w http.ResponseWriter, r *http.Request) {
	var req MultipartPresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload: " + err.Error()})
		return
	}

	req.Filename = strings.TrimSpace(req.Filename)
	if req.Filename == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "filename is required"})
		return
	}

	if req.PartCount <= 0 || req.PartCount > 10000 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "part_count must be between 1 and 10000"})
		return
	}

	if req.SizeBytes <= 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "size_bytes must be greater than zero"})
		return
	}

	if !s.cfg.Limits.IsMIMEAllowed(req.MIMEType) {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": fmt.Sprintf("mime type %q is not permitted by policy", req.MIMEType),
		})
		return
	}

	prov, err := s.storageMgr.Get(req.Bucket)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	now := time.Now().UTC()
	ext := filepath.Ext(req.Filename)
	uploadUUID := uuid.New().String()
	storageKey := fmt.Sprintf("uploads/%04d/%02d/%02d/%s_%s%s",
		now.Year(), now.Month(), now.Day(),
		uploadUUID[:8], "multipart", ext,
	)

	mpID, err := prov.InitiateMultipart(r.Context(), storageKey, req.MIMEType)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to initiate multipart upload: " + err.Error()})
		return
	}

	partSize := req.SizeBytes / int64(req.PartCount)
	var parts []storage.PartPresign

	for i := 1; i <= req.PartCount; i++ {
		url, err := prov.PresignPart(r.Context(), storageKey, mpID, int32(i), s.cfg.Limits.PresignExpiry())
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to presign part: " + err.Error()})
			return
		}
		parts = append(parts, storage.PartPresign{
			PartNumber: int32(i),
			URL:        url,
		})
	}

	uploadID := "upl_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	metaJSON, _ := json.Marshal(req.Metadata)
	uploadRecord := &db.Upload{
		ID:              uploadID,
		Filename:        req.Filename,
		OriginalSize:    req.SizeBytes,
		DeclaredMIME:    req.MIMEType,
		StorageProvider: prov.Name(),
		StorageBucket:   req.Bucket,
		StorageKey:      storageKey,
		PublicURL:       prov.PublicURL(storageKey),
		Status:          db.StatusPending,
		MetadataJSON:    string(metaJSON),
	}

	if err := s.repo.CreateUpload(r.Context(), uploadRecord); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error: " + err.Error()})
		return
	}

	s.metrics.PresignRequestsTotal.WithLabelValues(prov.Name()).Inc()
	s.sseHub.Broadcast("upload.created", uploadRecord)

	respondJSON(w, http.StatusOK, storage.MultipartPresignResult{
		UploadID:          uploadID,
		MultipartUploadID: mpID,
		PartSize:          partSize,
		StorageKey:        storageKey,
		Parts:             parts,
		ExpiresAt:         now.Add(s.cfg.Limits.PresignExpiry()),
	})
}
