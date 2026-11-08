package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host               string   `yaml:"host"`
	Port               int      `yaml:"port"`
	APIKey             string   `yaml:"api_key"`
	CORSAllowedOrigins []string `yaml:"cors_allowed_origins"`
	DBPath             string   `yaml:"db_path"`
	ReadTimeout        int      `yaml:"read_timeout_seconds"`
	WriteTimeout       int      `yaml:"write_timeout_seconds"`
}

// StorageProviderConfig holds settings for a specific storage target.
type StorageProviderConfig struct {
	Type            string `yaml:"type"` // "s3" or "local"
	Endpoint        string `yaml:"endpoint"`
	Bucket          string `yaml:"bucket"`
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	PublicBaseURL   string `yaml:"public_base_url"`
	UsePathStyle    bool   `yaml:"use_path_style"`
	Directory       string `yaml:"directory"` // For local filesystem storage
}

// StorageConfig holds all configured storage providers.
type StorageConfig struct {
	DefaultProvider string                           `yaml:"default_provider"`
	Providers       map[string]StorageProviderConfig `yaml:"providers"`
}

// LimitsConfig defines upload size and MIME boundaries.
type LimitsConfig struct {
	MaxFileSizeBytes     int64    `yaml:"max_file_size_bytes"`
	PresignExpirySeconds int      `yaml:"presign_expiry_seconds"`
	AllowedMIMETypes     []string `yaml:"allowed_mime_types"`
}

// ThumbnailVariant defines width, height, and fit strategy.
type ThumbnailVariant struct {
	Name   string `yaml:"name"`
	Width  int    `yaml:"width"`
	Height int    `yaml:"height"`
	Fit    string `yaml:"fit"` // "crop" or "contain"
}

// ThumbnailsConfig controls responsive image derivation.
type ThumbnailsConfig struct {
	Enabled  bool               `yaml:"enabled"`
	Formats  []string           `yaml:"formats"` // "webp", "jpeg"
	Variants []ThumbnailVariant `yaml:"variants"`
}

// WebhooksConfig controls outgoing notification dispatch.
type WebhooksConfig struct {
	Endpoint       string `yaml:"endpoint"`
	Secret         string `yaml:"secret"`
	MaxRetries     int    `yaml:"max_retries"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

// Config represents the complete UploadGate configuration.
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Storage    StorageConfig    `yaml:"storage"`
	Limits     LimitsConfig     `yaml:"limits"`
	Thumbnails ThumbnailsConfig `yaml:"thumbnails"`
	Webhooks   WebhooksConfig   `yaml:"webhooks"`
}

// DefaultConfig returns sane production defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:               "0.0.0.0",
			Port:               8080,
			APIKey:             "ug_default_secret_key",
			CORSAllowedOrigins: []string{"*"},
			DBPath:             "./data/uploadgate.db",
			ReadTimeout:        30,
			WriteTimeout:       30,
		},
		Storage: StorageConfig{
			DefaultProvider: "local",
			Providers: map[string]StorageProviderConfig{
				"local": {
					Type:          "local",
					Directory:     "./local_storage",
					PublicBaseURL: "http://localhost:8080/v1/storage/local",
				},
			},
		},
		Limits: LimitsConfig{
			MaxFileSizeBytes:     100 * 1024 * 1024, // 100MB
			PresignExpirySeconds: 3600,              // 1 hour
			AllowedMIMETypes: []string{
				"image/jpeg",
				"image/png",
				"image/webp",
				"image/gif",
				"image/svg+xml",
				"application/pdf",
				"video/mp4",
				"video/webm",
				"application/zip",
				"application/json",
				"text/csv",
			},
		},
		Thumbnails: ThumbnailsConfig{
			Enabled: true,
			Formats: []string{"webp", "jpeg"},
			Variants: []ThumbnailVariant{
				{Name: "thumb", Width: 150, Height: 150, Fit: "crop"},
				{Name: "medium", Width: 600, Height: 600, Fit: "contain"},
			},
		},
		Webhooks: WebhooksConfig{
			Endpoint:       "",
			Secret:         "whsec_default_signing_secret",
			MaxRetries:     5,
			TimeoutSeconds: 10,
		},
	}
}

// LoadConfig reads configuration from file and applies environment variable overrides.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
			}
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file %q: %w", path, err)
			}
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if val := os.Getenv("UPLOADGATE_SERVER_PORT"); val != "" {
		if p, err := strconv.Atoi(val); err == nil {
			cfg.Server.Port = p
		}
	}
	if val := os.Getenv("UPLOADGATE_SERVER_HOST"); val != "" {
		cfg.Server.Host = val
	}
	if val := os.Getenv("UPLOADGATE_SERVER_API_KEY"); val != "" {
		cfg.Server.APIKey = val
	}
	if val := os.Getenv("UPLOADGATE_DB_PATH"); val != "" {
		cfg.Server.DBPath = val
	}
	if val := os.Getenv("UPLOADGATE_STORAGE_DEFAULT_PROVIDER"); val != "" {
		cfg.Storage.DefaultProvider = val
	}
	if val := os.Getenv("UPLOADGATE_WEBHOOK_ENDPOINT"); val != "" {
		cfg.Webhooks.Endpoint = val
	}
	if val := os.Getenv("UPLOADGATE_WEBHOOK_SECRET"); val != "" {
		cfg.Webhooks.Secret = val
	}

	// Environment variable provider configuration e.g. AWS S3
	if s3Bucket := os.Getenv("AWS_S3_BUCKET"); s3Bucket != "" {
		p := cfg.Storage.Providers["s3"]
		p.Type = "s3"
		p.Bucket = s3Bucket
		if ep := os.Getenv("AWS_S3_ENDPOINT"); ep != "" {
			p.Endpoint = ep
		} else if p.Endpoint == "" {
			p.Endpoint = "https://s3.amazonaws.com"
		}
		if reg := os.Getenv("AWS_REGION"); reg != "" {
			p.Region = reg
		}
		if key := os.Getenv("AWS_ACCESS_KEY_ID"); key != "" {
			p.AccessKeyID = key
		}
		if sec := os.Getenv("AWS_SECRET_ACCESS_KEY"); sec != "" {
			p.SecretAccessKey = sec
		}
		cfg.Storage.Providers["s3"] = p
	}
}

// GetAddress returns the formatted host:port string.
func (c *ServerConfig) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// PresignExpiry returns the time.Duration for presigned URLs.
func (l *LimitsConfig) PresignExpiry() time.Duration {
	if l.PresignExpirySeconds <= 0 {
		return 1 * time.Hour
	}
	return time.Duration(l.PresignExpirySeconds) * time.Second
}

// IsMIMEAllowed checks if the given MIME type is within the whitelist.
func (l *LimitsConfig) IsMIMEAllowed(mimeType string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(mimeType))
	if len(l.AllowedMIMETypes) == 0 {
		return true
	}
	for _, allowed := range l.AllowedMIMETypes {
		if strings.EqualFold(allowed, cleaned) {
			return true
		}
		// Prefix matching for e.g. "image/*"
		if strings.HasSuffix(allowed, "/*") {
			prefix := strings.TrimSuffix(allowed, "/*")
			if strings.HasPrefix(cleaned, prefix+"/") {
				return true
			}
		}
	}
	return false
}
