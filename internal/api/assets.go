package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/go-chi/chi/v5"
)

// HandleGetUpload processes GET /v1/uploads/:id
func (s *Server) HandleGetUpload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "upload id is required"})
		return
	}

	upload, err := s.repo.GetUpload(r.Context(), id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if upload == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "upload not found"})
		return
	}

	respondJSON(w, http.StatusOK, upload)
}

// HandleListUploads processes GET /v1/uploads
func (s *Server) HandleListUploads(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	uploads, total, err := s.repo.ListUploads(r.Context(), status, limit, offset)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items":  uploads,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// BucketInfo represents health and provider information.
type BucketInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	PublicURL string `json:"public_url"`
	Healthy   bool   `json:"healthy"`
	Error     string `json:"error,omitempty"`
}

// HandleListBuckets processes GET /v1/buckets
func (s *Server) HandleListBuckets(w http.ResponseWriter, r *http.Request) {
	providers := s.storageMgr.ListProviders()
	var list []BucketInfo

	for name, prov := range providers {
		err := prov.CheckHealth(r.Context())
		info := BucketInfo{
			Name:      name,
			Type:      prov.Type(),
			PublicURL: prov.PublicURL(""),
			Healthy:   err == nil,
		}
		if err != nil {
			info.Error = err.Error()
		}
		list = append(list, info)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"default_provider": s.cfg.Storage.DefaultProvider,
		"buckets":          list,
	})
}

// HandleListWebhooks processes GET /v1/webhooks
func (s *Server) HandleListWebhooks(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	deliveries, err := s.repo.ListWebhookDeliveries(r.Context(), limit)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"items": deliveries,
		"total": len(deliveries),
	})
}

// HandleRetryWebhook processes POST /v1/webhooks/:id/retry
func (s *Server) HandleRetryWebhook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook id"})
		return
	}

	deliveries, err := s.repo.ListWebhookDeliveries(r.Context(), 100)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var target *db.WebhookDelivery
	for _, d := range deliveries {
		if d.ID == id {
			target = d
			break
		}
	}

	if target == nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "webhook delivery record not found"})
		return
	}

	var payload map[string]interface{}
	_ = json.Unmarshal([]byte(target.Payload), &payload)

	s.webhookDispatcher.Dispatch(target.UploadID, target.Event, payload)
	respondJSON(w, http.StatusOK, map[string]string{"status": "queued for retry"})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
