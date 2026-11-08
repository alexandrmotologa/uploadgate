package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/alexandrmotologa/uploadgate/internal/processor"
	"github.com/alexandrmotologa/uploadgate/internal/validator"
	"github.com/google/uuid"
)

// FromURLRequest defines the payload for URL ingestion.
type FromURLRequest struct {
	URL      string                 `json:"url"`
	Filename string                 `json:"filename,omitempty"`
	Bucket   string                 `json:"bucket,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HandleFromURL processes POST /v1/uploads/from-url
func (s *Server) HandleFromURL(w http.ResponseWriter, r *http.Request) {
	var req FromURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload: " + err.Error()})
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	// 1. Validate against SSRF vulnerabilities
	parsedURL, err := validator.ValidateURL(req.URL)
	if err != nil {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "ssrf blocked: " + err.Error()})
		return
	}

	// 2. Derive filename if omitted
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = filepath.Base(parsedURL.Path)
		if filename == "" || filename == "." || filename == "/" {
			filename = "remote_asset"
		}
	}

	prov, err := s.storageMgr.Get(req.Bucket)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// 3. Fetch remote asset using safe HTTP client
	client := validator.NewSafeHTTPClient(30 * time.Second)
	httpReq, err := http.NewRequestWithContext(r.Context(), "GET", req.URL, nil)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	httpReq.Header.Set("User-Agent", "UploadGate-Ingestion/1.0")

	resp, err := client.Do(httpReq)
	if err != nil {
		respondJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to fetch remote asset: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respondJSON(w, http.StatusBadGateway, map[string]string{
			"error": fmt.Sprintf("remote server returned HTTP %d", resp.StatusCode),
		})
		return
	}

	// Read first chunk into buffer for magic-byte sniffing
	headerBuf := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, headerBuf)
	initialChunk := headerBuf[:n]

	// Detect MIME type from binary signature
	detectedMIME, _, _ := validator.SniffHeader(bytes.NewReader(initialChunk))
	if detectedMIME == "" || detectedMIME == "application/octet-stream" {
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			parts := strings.Split(ct, ";")
			detectedMIME = strings.TrimSpace(parts[0])
		}
	}

	// Validate against allowed MIME whitelist
	if !s.cfg.Limits.IsMIMEAllowed(detectedMIME) {
		respondJSON(w, http.StatusForbidden, map[string]string{
			"error": fmt.Sprintf("mime type %q is not permitted by policy", detectedMIME),
		})
		return
	}

	// Buffer body respecting max limit
	limitReader := io.LimitReader(resp.Body, s.cfg.Limits.MaxFileSizeBytes+1)
	var fullPayload bytes.Buffer
	fullPayload.Write(initialChunk)
	bytesCopied, err := io.Copy(&fullPayload, limitReader)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "error reading remote body: " + err.Error()})
		return
	}

	totalSize := int64(len(initialChunk)) + bytesCopied
	if totalSize > s.cfg.Limits.MaxFileSizeBytes {
		respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error": fmt.Sprintf("remote asset exceeds max file size of %d bytes", s.cfg.Limits.MaxFileSizeBytes),
		})
		return
	}

	dataBytes := fullPayload.Bytes()

	// If SVG, sanitize against XSS
	if detectedMIME == "image/svg+xml" {
		ok, reason := validator.SanitizeSVG(bytes.NewReader(dataBytes))
		if !ok {
			respondJSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "svg rejected: " + reason,
			})
			return
		}
	}

	// 4. Generate storage key
	now := time.Now().UTC()
	ext := filepath.Ext(filename)
	if ext == "" {
		if detectedMIME == "image/png" {
			ext = ".png"
		} else if detectedMIME == "image/jpeg" {
			ext = ".jpg"
		} else if detectedMIME == "image/webp" {
			ext = ".webp"
		} else if detectedMIME == "application/pdf" {
			ext = ".pdf"
		}
	}

	uploadUUID := uuid.New().String()
	uploadID := "upl_" + strings.ReplaceAll(uploadUUID, "-", "")
	storageKey := fmt.Sprintf("uploads/%04d/%02d/%02d/%s_from_url%s",
		now.Year(), now.Month(), now.Day(),
		uploadUUID[:8], ext,
	)

	// 5. Store asset in target storage provider
	if err := prov.PutObject(r.Context(), storageKey, bytes.NewReader(dataBytes), totalSize, detectedMIME); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store asset: " + err.Error()})
		return
	}

	// 6. Extract metadata & generate thumbnails
	meta, err := processor.ExtractMetadata(bytes.NewReader(dataBytes), detectedMIME)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "metadata extraction failed: " + err.Error()})
		return
	}

	metaJSON, _ := json.Marshal(req.Metadata)
	uploadRecord := &db.Upload{
		ID:              uploadID,
		Filename:        filename,
		OriginalSize:    totalSize,
		DeclaredMIME:    detectedMIME,
		DetectedMIME:    detectedMIME,
		StorageProvider: prov.Name(),
		StorageBucket:   req.Bucket,
		StorageKey:      storageKey,
		PublicURL:       prov.PublicURL(storageKey),
		Status:          db.StatusVerified,
		SHA256:          meta.SHA256,
		Width:           meta.Width,
		Height:          meta.Height,
		DurationSeconds: meta.DurationSeconds,
		MetadataJSON:    string(metaJSON),
	}

	if err := s.repo.CreateUpload(r.Context(), uploadRecord); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error: " + err.Error()})
		return
	}

	// Generate thumbnails for raster images
	var thumbs []db.Thumbnail
	if s.cfg.Thumbnails.Enabled && strings.HasPrefix(detectedMIME, "image/") && detectedMIME != "image/svg+xml" {
		generated, err := processor.GenerateThumbnails(bytes.NewReader(dataBytes), detectedMIME, s.cfg.Thumbnails.Variants)
		if err == nil {
			for _, th := range generated {
				thKey := fmt.Sprintf("thumbnails/%s_%s_%dx%d.%s", uploadID, th.VariantName, th.Width, th.Height, th.Format)
				if err := prov.PutObject(r.Context(), thKey, bytes.NewReader(th.Data), int64(len(th.Data)), th.MIMEType); err == nil {
					rec := db.Thumbnail{
						UploadID:    uploadID,
						VariantName: th.VariantName,
						Format:      th.Format,
						Width:       th.Width,
						Height:      th.Height,
						StorageKey:  thKey,
						PublicURL:   prov.PublicURL(thKey),
						SizeBytes:   int64(len(th.Data)),
					}
					_ = s.repo.CreateThumbnail(r.Context(), &rec)
					thumbs = append(thumbs, rec)
				}
			}
		}
	}
	uploadRecord.Thumbnails = thumbs

	// 7. Metrics, SSE, and Webhook dispatch
	s.metrics.UploadsTotal.WithLabelValues("verified").Inc()
	s.metrics.UploadBytesTotal.Add(float64(totalSize))
	s.sseHub.Broadcast("upload.verified", uploadRecord)

	s.webhookDispatcher.Dispatch(uploadID, "upload.completed", map[string]interface{}{
		"event":     "upload.completed",
		"upload_id": uploadID,
		"source":    "url_ingestion",
		"url":       req.URL,
		"status":    "verified",
		"storage": map[string]interface{}{
			"provider":   uploadRecord.StorageProvider,
			"key":        uploadRecord.StorageKey,
			"public_url": uploadRecord.PublicURL,
		},
		"file": map[string]interface{}{
			"name":          filename,
			"size_bytes":    totalSize,
			"detected_mime": detectedMIME,
			"sha256":        meta.SHA256,
		},
	})

	respondJSON(w, http.StatusOK, uploadRecord)
}
