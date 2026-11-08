package db

import (
	"time"
)

// UploadStatus indicates the lifecycle phase of an upload.
type UploadStatus string

const (
	StatusPending  UploadStatus = "pending"
	StatusUploaded UploadStatus = "uploaded"
	StatusVerified UploadStatus = "verified"
	StatusRejected UploadStatus = "rejected"
)

// Upload represents an uploaded asset record.
type Upload struct {
	ID              string       `json:"id"`
	Filename        string       `json:"filename"`
	OriginalSize    int64        `json:"original_size"`
	DeclaredMIME    string       `json:"declared_mime"`
	DetectedMIME    string       `json:"detected_mime"`
	StorageProvider string       `json:"storage_provider"`
	StorageBucket   string       `json:"storage_bucket"`
	StorageKey      string       `json:"storage_key"`
	PublicURL       string       `json:"public_url"`
	Status          UploadStatus `json:"status"`
	SHA256          string       `json:"sha256"`
	Width           int          `json:"width"`
	Height          int          `json:"height"`
	DurationSeconds float64      `json:"duration_seconds"`
	ErrorReason     string       `json:"error_reason,omitempty"`
	MetadataJSON    string       `json:"metadata_json,omitempty"`
	ExpiresAt       *time.Time   `json:"expires_at,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
	Thumbnails      []Thumbnail  `json:"thumbnails,omitempty"`
}

// Thumbnail represents a generated responsive variant of an image.
type Thumbnail struct {
	ID          int64     `json:"id"`
	UploadID    string    `json:"upload_id"`
	VariantName string    `json:"variant_name"`
	Format      string    `json:"format"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	StorageKey  string    `json:"storage_key"`
	PublicURL   string    `json:"public_url"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
}

// WebhookDelivery records outbound webhook dispatches.
type WebhookDelivery struct {
	ID           int64     `json:"id"`
	UploadID     string    `json:"upload_id"`
	Event        string    `json:"event"`
	Endpoint     string    `json:"endpoint"`
	Payload      string    `json:"payload"`
	StatusCode   int       `json:"status_code"`
	Attempt      int       `json:"attempt"`
	ResponseBody string    `json:"response_body"`
	ErrorMessage string    `json:"error_message,omitempty"`
	DeliveredAt  time.Time `json:"delivered_at"`
	CreatedAt    time.Time `json:"created_at"`
}
