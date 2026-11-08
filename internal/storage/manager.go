package storage

import (
	"fmt"
	"sync"

	"github.com/alexandrmotologa/uploadgate/internal/config"
)

// Manager coordinates storage providers.
type Manager struct {
	mu              sync.RWMutex
	defaultProvider string
	providers       map[string]Provider
}

// NewManager initializes all storage providers from configuration.
func NewManager(cfg *config.Config) (*Manager, error) {
	m := &Manager{
		defaultProvider: cfg.Storage.DefaultProvider,
		providers:       make(map[string]Provider),
	}

	for name, provCfg := range cfg.Storage.Providers {
		switch provCfg.Type {
		case "local":
			p, err := NewLocalProvider(name, provCfg.Directory, provCfg.PublicBaseURL, cfg.Server.APIKey)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize local provider %q: %w", name, err)
			}
			m.providers[name] = p
		case "s3":
			p, err := NewS3Provider(S3Options{
				Name:            name,
				Endpoint:        provCfg.Endpoint,
				Bucket:          provCfg.Bucket,
				Region:          provCfg.Region,
				AccessKeyID:     provCfg.AccessKeyID,
				SecretAccessKey: provCfg.SecretAccessKey,
				PublicBaseURL:   provCfg.PublicBaseURL,
				UsePathStyle:    provCfg.UsePathStyle,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to initialize s3 provider %q: %w", name, err)
			}
			m.providers[name] = p
		default:
			return nil, fmt.Errorf("unsupported storage provider type %q for provider %q", provCfg.Type, name)
		}
	}

	if _, ok := m.providers[m.defaultProvider]; !ok {
		// Fallback to first available provider if default is invalid
		for name := range m.providers {
			m.defaultProvider = name
			break
		}
	}

	if len(m.providers) == 0 {
		return nil, fmt.Errorf("no valid storage providers configured")
	}

	return m, nil
}

// Get returns the named provider, or default if name is empty.
func (m *Manager) Get(name string) (Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if name == "" {
		name = m.defaultProvider
	}

	p, ok := m.providers[name]
	if !ok {
		return nil, fmt.Errorf("storage provider %q not found", name)
	}
	return p, nil
}

// Default returns the default storage provider.
func (m *Manager) Default() Provider {
	p, _ := m.Get(m.defaultProvider)
	return p
}

// ListProviders returns all registered providers.
func (m *Manager) ListProviders() map[string]Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	copyMap := make(map[string]Provider)
	for k, v := range m.providers {
		copyMap[k] = v
	}
	return copyMap
}
