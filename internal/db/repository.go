package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository provides thread-safe access to persistent data.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new repository instance.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateUpload inserts a new pending upload record.
func (r *Repository) CreateUpload(ctx context.Context, u *Upload) error {
	now := time.Now().UTC()
	u.CreatedAt = now
	u.UpdatedAt = now
	if u.MetadataJSON == "" {
		u.MetadataJSON = "{}"
	}

	query := `
	INSERT INTO uploads (
		id, filename, original_size, declared_mime, detected_mime,
		storage_provider, storage_bucket, storage_key, public_url,
		status, sha256, width, height, duration_seconds,
		error_reason, metadata_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		u.ID, u.Filename, u.OriginalSize, u.DeclaredMIME, u.DetectedMIME,
		u.StorageProvider, u.StorageBucket, u.StorageKey, u.PublicURL,
		u.Status, u.SHA256, u.Width, u.Height, u.DurationSeconds,
		u.ErrorReason, u.MetadataJSON, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

// GetUpload retrieves an upload by ID, including its thumbnails.
func (r *Repository) GetUpload(ctx context.Context, id string) (*Upload, error) {
	query := `
	SELECT id, filename, original_size, declared_mime, detected_mime,
	       storage_provider, storage_bucket, storage_key, public_url,
	       status, sha256, width, height, duration_seconds,
	       error_reason, metadata_json, created_at, updated_at
	FROM uploads WHERE id = ?
	`
	row := r.db.QueryRowContext(ctx, query, id)

	u := &Upload{}
	err := row.Scan(
		&u.ID, &u.Filename, &u.OriginalSize, &u.DeclaredMIME, &u.DetectedMIME,
		&u.StorageProvider, &u.StorageBucket, &u.StorageKey, &u.PublicURL,
		&u.Status, &u.SHA256, &u.Width, &u.Height, &u.DurationSeconds,
		&u.ErrorReason, &u.MetadataJSON, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	thumbnails, err := r.ListThumbnailsByUploadID(ctx, id)
	if err != nil {
		return nil, err
	}
	u.Thumbnails = thumbnails

	return u, nil
}

// UpdateUploadVerification updates the upload once security and thumbnail processing complete.
func (r *Repository) UpdateUploadVerification(ctx context.Context, u *Upload) error {
	u.UpdatedAt = time.Now().UTC()
	query := `
	UPDATE uploads SET
		detected_mime = ?,
		public_url = ?,
		status = ?,
		sha256 = ?,
		width = ?,
		height = ?,
		duration_seconds = ?,
		error_reason = ?,
		metadata_json = ?,
		updated_at = ?
	WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		u.DetectedMIME, u.PublicURL, u.Status, u.SHA256,
		u.Width, u.Height, u.DurationSeconds, u.ErrorReason,
		u.MetadataJSON, u.UpdatedAt, u.ID,
	)
	return err
}

// ListUploads retrieves recent uploads with optional status filter.
func (r *Repository) ListUploads(ctx context.Context, status string, limit, offset int) ([]*Upload, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	whereClause := ""
	var args []interface{}
	if status != "" {
		whereClause = "WHERE status = ?"
		args = append(args, status)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM uploads %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	selectQuery := fmt.Sprintf(`
	SELECT id, filename, original_size, declared_mime, detected_mime,
	       storage_provider, storage_bucket, storage_key, public_url,
	       status, sha256, width, height, duration_seconds,
	       error_reason, metadata_json, created_at, updated_at
	FROM uploads %s
	ORDER BY created_at DESC
	LIMIT ? OFFSET ?
	`, whereClause)

	argsWithLimit := append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, selectQuery, argsWithLimit...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var uploads []*Upload
	for rows.Next() {
		u := &Upload{}
		if err := rows.Scan(
			&u.ID, &u.Filename, &u.OriginalSize, &u.DeclaredMIME, &u.DetectedMIME,
			&u.StorageProvider, &u.StorageBucket, &u.StorageKey, &u.PublicURL,
			&u.Status, &u.SHA256, &u.Width, &u.Height, &u.DurationSeconds,
			&u.ErrorReason, &u.MetadataJSON, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		uploads = append(uploads, u)
	}

	return uploads, total, nil
}

// CreateThumbnail records a generated thumbnail variant.
func (r *Repository) CreateThumbnail(ctx context.Context, t *Thumbnail) error {
	t.CreatedAt = time.Now().UTC()
	query := `
	INSERT INTO thumbnails (
		upload_id, variant_name, format, width, height,
		storage_key, public_url, size_bytes, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, query,
		t.UploadID, t.VariantName, t.Format, t.Width, t.Height,
		t.StorageKey, t.PublicURL, t.SizeBytes, t.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		t.ID = id
	}
	return nil
}

// ListThumbnailsByUploadID returns all thumbnails for an upload.
func (r *Repository) ListThumbnailsByUploadID(ctx context.Context, uploadID string) ([]Thumbnail, error) {
	query := `
	SELECT id, upload_id, variant_name, format, width, height,
	       storage_key, public_url, size_bytes, created_at
	FROM thumbnails
	WHERE upload_id = ?
	ORDER BY width ASC
	`
	rows, err := r.db.QueryContext(ctx, query, uploadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Thumbnail
	for rows.Next() {
		var t Thumbnail
		if err := rows.Scan(
			&t.ID, &t.UploadID, &t.VariantName, &t.Format, &t.Width, &t.Height,
			&t.StorageKey, &t.PublicURL, &t.SizeBytes, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

// RecordWebhookDelivery stores the outcome of an outbound webhook request.
func (r *Repository) RecordWebhookDelivery(ctx context.Context, w *WebhookDelivery) error {
	w.CreatedAt = time.Now().UTC()
	query := `
	INSERT INTO webhook_deliveries (
		upload_id, event, endpoint, payload, status_code,
		attempt, response_body, error_message, delivered_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := r.db.ExecContext(ctx, query,
		w.UploadID, w.Event, w.Endpoint, w.Payload, w.StatusCode,
		w.Attempt, w.ResponseBody, w.ErrorMessage, w.DeliveredAt, w.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		w.ID = id
	}
	return nil
}

// ListWebhookDeliveries returns recent webhook delivery logs.
func (r *Repository) ListWebhookDeliveries(ctx context.Context, limit int) ([]*WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
	SELECT id, upload_id, event, endpoint, payload, status_code,
	       attempt, response_body, error_message, delivered_at, created_at
	FROM webhook_deliveries
	ORDER BY created_at DESC
	LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*WebhookDelivery
	for rows.Next() {
		w := &WebhookDelivery{}
		if err := rows.Scan(
			&w.ID, &w.UploadID, &w.Event, &w.Endpoint, &w.Payload, &w.StatusCode,
			&w.Attempt, &w.ResponseBody, &w.ErrorMessage, &w.DeliveredAt, &w.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, w)
	}
	return list, nil
}
