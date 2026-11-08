package storage

import (
	"context"
	"io"
	"time"
)

// PresignResult contains the presigned URL and client request headers.
type PresignResult struct {
	UploadID   string            `json:"upload_id"`
	URL        string            `json:"presigned_url"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers"`
	StorageKey string            `json:"storage_key"`
	ExpiresAt  time.Time         `json:"expires_at"`
}

// CompletedPart represents an uploaded chunk in a multipart upload.
type CompletedPart struct {
	PartNumber int32  `json:"part_number"`
	ETag       string `json:"etag"`
}

// PartPresign contains the presigned URL for a single chunk.
type PartPresign struct {
	PartNumber int32  `json:"part_number"`
	URL        string `json:"presigned_url"`
}

// MultipartPresignResult contains the multipart session details.
type MultipartPresignResult struct {
	UploadID          string        `json:"upload_id"`
	MultipartUploadID string        `json:"multipart_upload_id"`
	PartSize          int64         `json:"part_size"`
	StorageKey        string        `json:"storage_key"`
	Parts             []PartPresign `json:"parts"`
	ExpiresAt         time.Time     `json:"expires_at"`
}

// Provider defines the interface for all cloud and local storage backends.
type Provider interface {
	Name() string
	Type() string
	PresignPut(ctx context.Context, key string, mimeType string, sizeBytes int64, expiry time.Duration) (*PresignResult, error)
	InitiateMultipart(ctx context.Context, key string, mimeType string) (string, error)
	PresignPart(ctx context.Context, key string, multipartUploadID string, partNumber int32, expiry time.Duration) (string, error)
	CompleteMultipart(ctx context.Context, key string, multipartUploadID string, parts []CompletedPart) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, int64, error)
	PutObject(ctx context.Context, key string, body io.Reader, sizeBytes int64, contentType string) error
	DeleteObject(ctx context.Context, key string) error
	PublicURL(key string) string
	CheckHealth(ctx context.Context) error
}
