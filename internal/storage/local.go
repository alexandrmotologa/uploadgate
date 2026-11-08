package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LocalProvider implements Provider for local filesystem storage.
type LocalProvider struct {
	name          string
	directory     string
	publicBaseURL string
	secretKey     []byte
}

// LocalUploadToken holds signed claims for local presigned uploads.
type LocalUploadToken struct {
	UploadID  string `json:"upload_id"`
	Key       string `json:"key"`
	MIMEType  string `json:"mime"`
	MaxBytes  int64  `json:"max_bytes"`
	ExpiresAt int64  `json:"exp"`
}

// NewLocalProvider creates a new local filesystem storage provider.
func NewLocalProvider(name, directory, publicBaseURL string, secretKey string) (*LocalProvider, error) {
	if directory == "" {
		directory = "./local_storage"
	}
	absDir, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("invalid local storage directory: %w", err)
	}
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create local storage directory %q: %w", absDir, err)
	}

	if secretKey == "" {
		secretKey = "ug_local_storage_secret_key"
	}

	baseURL := strings.TrimRight(publicBaseURL, "/")
	if baseURL == "" {
		baseURL = "/v1/storage/local"
	}

	return &LocalProvider{
		name:          name,
		directory:     absDir,
		publicBaseURL: baseURL,
		secretKey:     []byte(secretKey),
	}, nil
}

func (l *LocalProvider) Name() string {
	return l.name
}

func (l *LocalProvider) Type() string {
	return "local"
}

// PresignPut issues a signed upload token and direct PUT URL for the local gateway.
func (l *LocalProvider) PresignPut(ctx context.Context, key string, mimeType string, sizeBytes int64, expiry time.Duration) (*PresignResult, error) {
	uploadID := "upl_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	exp := time.Now().Add(expiry)

	tokenClaims := LocalUploadToken{
		UploadID:  uploadID,
		Key:       key,
		MIMEType:  mimeType,
		MaxBytes:  sizeBytes,
		ExpiresAt: exp.Unix(),
	}

	tokenBytes, err := json.Marshal(tokenClaims)
	if err != nil {
		return nil, err
	}

	tokenB64 := base64.RawURLEncoding.EncodeToString(tokenBytes)
	mac := hmac.New(sha256.New, l.secretKey)
	mac.Write([]byte(tokenB64))
	sigB64 := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	signedToken := fmt.Sprintf("%s.%s", tokenB64, sigB64)

	presignedURL := fmt.Sprintf("%s/upload/%s", l.publicBaseURL, signedToken)

	return &PresignResult{
		UploadID:   uploadID,
		URL:        presignedURL,
		Method:     "PUT",
		Headers:    map[string]string{"Content-Type": mimeType},
		StorageKey: key,
		ExpiresAt:  exp,
	}, nil
}

// ValidateToken parses and verifies the signature and expiration of an upload token.
func (l *LocalProvider) ValidateToken(tokenStr string) (*LocalUploadToken, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}

	tokenB64 := parts[0]
	sigB64 := parts[1]

	expectedSig := hmac.New(sha256.New, l.secretKey)
	expectedSig.Write([]byte(tokenB64))
	calcSig := expectedSig.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil || !hmac.Equal(calcSig, actualSig) {
		return nil, errors.New("invalid token signature")
	}

	data, err := base64.RawURLEncoding.DecodeString(tokenB64)
	if err != nil {
		return nil, errors.New("invalid token payload")
	}

	var claims LocalUploadToken
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, errors.New("corrupt token claims")
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, errors.New("token expired")
	}

	return &claims, nil
}

// InitiateMultipart is a stub for local storage (local storage handles large files directly).
func (l *LocalProvider) InitiateMultipart(ctx context.Context, key string, mimeType string) (string, error) {
	return "local_multipart_" + uuid.New().String(), nil
}

func (l *LocalProvider) PresignPart(ctx context.Context, key string, multipartUploadID string, partNumber int32, expiry time.Duration) (string, error) {
	token := fmt.Sprintf("%s_part_%d", multipartUploadID, partNumber)
	return fmt.Sprintf("%s/upload/part/%s", l.publicBaseURL, token), nil
}

func (l *LocalProvider) CompleteMultipart(ctx context.Context, key string, multipartUploadID string, parts []CompletedPart) error {
	return nil
}

// GetObject opens a stored file for reading.
func (l *LocalProvider) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	cleanPath := filepath.Clean(key)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return nil, 0, errors.New("illegal storage key path")
	}

	targetPath := filepath.Join(l.directory, cleanPath)
	info, err := os.Stat(targetPath)
	if err != nil {
		return nil, 0, err
	}

	f, err := os.Open(targetPath)
	if err != nil {
		return nil, 0, err
	}

	return f, info.Size(), nil
}

// PutObject writes content directly to local storage.
func (l *LocalProvider) PutObject(ctx context.Context, key string, body io.Reader, sizeBytes int64, contentType string) error {
	cleanPath := filepath.Clean(key)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return errors.New("illegal storage key path")
	}

	targetPath := filepath.Join(l.directory, cleanPath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if sizeBytes > 0 {
		_, err = io.CopyN(f, body, sizeBytes)
	} else {
		_, err = io.Copy(f, body)
	}
	return err
}

// DeleteObject deletes a file from local storage.
func (l *LocalProvider) DeleteObject(ctx context.Context, key string) error {
	cleanPath := filepath.Clean(key)
	targetPath := filepath.Join(l.directory, cleanPath)
	return os.Remove(targetPath)
}

// PublicURL returns the accessible HTTP URL for an asset.
func (l *LocalProvider) PublicURL(key string) string {
	cleanKey := strings.TrimPrefix(filepath.ToSlash(key), "/")
	return fmt.Sprintf("%s/files/%s", l.publicBaseURL, cleanKey)
}

// CheckHealth verifies write and read access to the local storage directory.
func (l *LocalProvider) CheckHealth(ctx context.Context) error {
	testFile := filepath.Join(l.directory, ".health_check")
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("local storage directory not writable: %w", err)
	}
	_ = os.Remove(testFile)
	return nil
}

// HandleUpload streams an incoming client direct PUT request to disk.
func (l *LocalProvider) HandleUpload(w http.ResponseWriter, r *http.Request, tokenStr string) {
	claims, err := l.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid upload token: %v", err), http.StatusForbidden)
		return
	}

	cleanPath := filepath.Clean(claims.Key)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		http.Error(w, "invalid destination key", http.StatusBadRequest)
		return
	}

	targetPath := filepath.Join(l.directory, cleanPath)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}

	f, err := os.Create(targetPath)
	if err != nil {
		http.Error(w, "failed to create destination file", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	var reader io.Reader = r.Body
	if claims.MaxBytes > 0 {
		// Enforce max byte ceiling with 1 byte margin to detect overflow
		reader = io.LimitReader(r.Body, claims.MaxBytes+1)
	}

	written, err := io.Copy(f, reader)
	if err != nil {
		_ = os.Remove(targetPath)
		http.Error(w, "upload interrupted", http.StatusInternalServerError)
		return
	}

	if claims.MaxBytes > 0 && written > claims.MaxBytes {
		_ = os.Remove(targetPath)
		http.Error(w, "payload exceeds declared file size", http.StatusRequestEntityTooLarge)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"success","bytes_written":` + fmt.Sprintf("%d", written) + `}`))
}

// HandleServe serves an asset file from local storage to clients.
func (l *LocalProvider) HandleServe(w http.ResponseWriter, r *http.Request, key string) {
	cleanPath := filepath.Clean(key)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	targetPath := filepath.Join(l.directory, cleanPath)
	http.ServeFile(w, r, targetPath)
}
