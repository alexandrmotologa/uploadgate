package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/alexandrmotologa/uploadgate/internal/processor"
	"github.com/alexandrmotologa/uploadgate/internal/storage"
	"github.com/alexandrmotologa/uploadgate/internal/validator"
	"github.com/go-chi/chi/v5"
)

// CompleteRequest defines the payload sent by clients when direct transfer finishes.
type CompleteRequest struct {
	MultipartUploadID string                  `json:"multipart_upload_id,omitempty"`
	Parts             []storage.CompletedPart `json:"parts,omitempty"`
}

// HandleComplete processes POST /v1/uploads/:id/complete
func (s *Server) HandleComplete(w http.ResponseWriter, r *http.Request) {
	uploadID := chi.URLParam(r, "id")
	if uploadID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "upload id is required"})
		return
	}

	upload, err := s.repo.GetUpload(r.Context(), uploadID)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error: " + err.Error()})
		return
	}
	if upload == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "upload record not found"})
		return
	}

	if upload.Status == db.StatusVerified {
		// Idempotent: already verified
		respondJSON(w, http.StatusOK, upload)
		return
	}

	var req CompleteRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	prov, err := s.storageMgr.Get(upload.StorageProvider)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "storage provider error: " + err.Error()})
		return
	}

	startTime := time.Now()

	// 1. Complete multipart if applicable
	if req.MultipartUploadID != "" && len(req.Parts) > 0 {
		if err := prov.CompleteMultipart(r.Context(), upload.StorageKey, req.MultipartUploadID, req.Parts); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to complete multipart upload: " + err.Error()})
			return
		}
	}

	// 2. Fetch object from storage to verify bytes
	reader, size, err := prov.GetObject(r.Context(), upload.StorageKey)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "object not found in storage bucket. Did the client complete the direct upload?",
		})
		return
	}
	defer reader.Close()

	if size > 0 && upload.OriginalSize == 0 {
		upload.OriginalSize = size
	}

	// Buffer initial chunk for sniffing
	headerBuf := make([]byte, 512)
	n, _ := io.ReadFull(reader, headerBuf)
	initialChunk := headerBuf[:n]

	// 3. Binary Magic-Byte Sniffer
	detectedMIME, _, err := validator.SniffHeader(bytes.NewReader(initialChunk))
	if err != nil {
		detectedMIME = "application/octet-stream"
	}

	// 4. Parity check between declared MIME and binary magic bytes
	isParityValid, parityReason := validator.ValidateMIMEParity(upload.DeclaredMIME, detectedMIME)
	if !isParityValid {
		upload.Status = db.StatusRejected
		upload.ErrorReason = parityReason
		upload.DetectedMIME = detectedMIME
		_ = s.repo.UpdateUploadVerification(r.Context(), upload)

		s.metrics.UploadsTotal.WithLabelValues("rejected").Inc()
		s.sseHub.Broadcast("upload.rejected", upload)
		s.webhookDispatcher.Dispatch(upload.ID, "upload.rejected", map[string]interface{}{
			"event":        "upload.rejected",
			"upload_id":    upload.ID,
			"error_reason": parityReason,
			"status":       "rejected",
			"declared":     upload.DeclaredMIME,
			"detected":     detectedMIME,
		})

		respondJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
			"upload_id": upload.ID,
			"status":    "rejected",
			"error":     parityReason,
		})
		return
	}

	// Combine stream for remaining operations
	fullStream := io.MultiReader(bytes.NewReader(initialChunk), reader)

	// If file is SVG, sanitize against stored XSS
	if detectedMIME == "image/svg+xml" {
		var svgBuf bytes.Buffer
		tee := io.TeeReader(fullStream, &svgBuf)
		isSVGValid, svgReason := validator.SanitizeSVG(tee)
		if !isSVGValid {
			upload.Status = db.StatusRejected
			upload.ErrorReason = svgReason
			upload.DetectedMIME = detectedMIME
			_ = s.repo.UpdateUploadVerification(r.Context(), upload)

			s.metrics.UploadsTotal.WithLabelValues("rejected").Inc()
			s.sseHub.Broadcast("upload.rejected", upload)
			s.webhookDispatcher.Dispatch(upload.ID, "upload.rejected", map[string]interface{}{
				"event":        "upload.rejected",
				"upload_id":    upload.ID,
				"error_reason": svgReason,
				"status":       "rejected",
			})

			respondJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
				"upload_id": upload.ID,
				"status":    "rejected",
				"error":     svgReason,
			})
			return
		}
		fullStream = &svgBuf
	}

	// 5. Extract metadata & calculate SHA-256
	var processedBuffer bytes.Buffer
	teeReader := io.TeeReader(fullStream, &processedBuffer)
	meta, err := processor.ExtractMetadata(teeReader, detectedMIME)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "metadata extraction failed: " + err.Error()})
		return
	}

	upload.Status = db.StatusVerified
	upload.DetectedMIME = detectedMIME
	upload.SHA256 = meta.SHA256
	upload.Width = meta.Width
	upload.Height = meta.Height
	upload.DurationSeconds = meta.DurationSeconds
	upload.PublicURL = prov.PublicURL(upload.StorageKey)

	// 6. Generate thumbnails for raster images if enabled
	var generatedThumbnails []db.Thumbnail
	if s.cfg.Thumbnails.Enabled && strings.HasPrefix(detectedMIME, "image/") && detectedMIME != "image/svg+xml" {
		thumbs, err := processor.GenerateThumbnails(bytes.NewReader(processedBuffer.Bytes()), detectedMIME, s.cfg.Thumbnails.Variants)
		if err == nil {
			for _, th := range thumbs {
				thumbKey := fmt.Sprintf("thumbnails/%s_%s_%dx%d.%s", upload.ID, th.VariantName, th.Width, th.Height, th.Format)
				// Write thumbnail to storage
				if err := prov.PutObject(r.Context(), thumbKey, bytes.NewReader(th.Data), int64(len(th.Data)), th.MIMEType); err == nil {
					thumbRecord := db.Thumbnail{
						UploadID:    upload.ID,
						VariantName: th.VariantName,
						Format:      th.Format,
						Width:       th.Width,
						Height:      th.Height,
						StorageKey:  thumbKey,
						PublicURL:   prov.PublicURL(thumbKey),
						SizeBytes:   int64(len(th.Data)),
					}
					_ = s.repo.CreateThumbnail(r.Context(), &thumbRecord)
					generatedThumbnails = append(generatedThumbnails, thumbRecord)
				}
			}
		}
	}
	upload.Thumbnails = generatedThumbnails

	// 7. Save verified state to SQLite
	if err := s.repo.UpdateUploadVerification(r.Context(), upload); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update upload record: " + err.Error()})
		return
	}

	// 8. Prometheus metrics & SSE broadcast
	s.metrics.UploadsTotal.WithLabelValues("verified").Inc()
	s.metrics.UploadBytesTotal.Add(float64(upload.OriginalSize))
	s.metrics.VerificationDurationSec.Observe(time.Since(startTime).Seconds())
	s.sseHub.Broadcast("upload.verified", upload)

	// 9. Dispatch HMAC-signed webhook
	var parsedMetadata map[string]interface{}
	_ = json.Unmarshal([]byte(upload.MetadataJSON), &parsedMetadata)

	webhookPayload := map[string]interface{}{
		"event":     "upload.completed",
		"upload_id": upload.ID,
		"status":    "verified",
		"storage": map[string]interface{}{
			"provider":   upload.StorageProvider,
			"bucket":     upload.StorageBucket,
			"key":        upload.StorageKey,
			"public_url": upload.PublicURL,
		},
		"file": map[string]interface{}{
			"name":          upload.Filename,
			"size_bytes":    upload.OriginalSize,
			"declared_mime": upload.DeclaredMIME,
			"detected_mime": upload.DetectedMIME,
			"width":         upload.Width,
			"height":        upload.Height,
			"sha256":        upload.SHA256,
		},
		"thumbnails": upload.Thumbnails,
		"metadata":   parsedMetadata,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	s.webhookDispatcher.Dispatch(upload.ID, "upload.completed", webhookPayload)

	respondJSON(w, http.StatusOK, upload)
}
