package api

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/config"
	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/alexandrmotologa/uploadgate/internal/storage"
	"github.com/alexandrmotologa/uploadgate/internal/webhook"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server coordinates the HTTP routes, storage, database, and background services.
type Server struct {
	cfg               *config.Config
	repo              *db.Repository
	storageMgr        *storage.Manager
	webhookDispatcher *webhook.Dispatcher
	sseHub            *SSEHub
	metrics           *Metrics
	router            chi.Router
	webFS             fs.FS
	widgetFS          fs.FS
}

// ServerOptions configures dependencies when instantiating the API Server.
type ServerOptions struct {
	Config            *config.Config
	Repo              *db.Repository
	StorageMgr        *storage.Manager
	WebhookDispatcher *webhook.Dispatcher
	WebFS             fs.FS
	WidgetFS          fs.FS
}

// NewServer builds and mounts the complete HTTP router.
func NewServer(opts ServerOptions) *Server {
	s := &Server{
		cfg:               opts.Config,
		repo:              opts.Repo,
		storageMgr:        opts.StorageMgr,
		webhookDispatcher: opts.WebhookDispatcher,
		sseHub:            NewSSEHub(),
		metrics:           NewMetrics(),
		router:            chi.NewRouter(),
		webFS:             opts.WebFS,
		widgetFS:          opts.WidgetFS,
	}

	s.setupRoutes()
	return s
}

// Router returns the initialized Chi router.
func (s *Server) Router() chi.Router {
	return s.router
}

func (s *Server) setupRoutes() {
	r := s.router

	// Standard middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	allowedOrigins := s.cfg.Server.CORSAllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-UploadGate-Signature", "x-amz-meta-upload-id"},
		ExposedHeaders:   []string{"Link", "ETag"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health and observability probes
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"version":   "1.0.0",
		})
	})
	r.Handle("/metrics", s.metrics.Handler())

	// Local storage direct upload and asset streaming endpoints
	r.Route("/v1/storage/local", func(r chi.Router) {
		r.Put("/upload/{token}", func(w http.ResponseWriter, req *http.Request) {
			token := chi.URLParam(req, "token")
			p, err := s.storageMgr.Get("local")
			if err != nil {
				http.Error(w, "local storage not configured", http.StatusNotFound)
				return
			}
			localProv, ok := p.(*storage.LocalProvider)
			if !ok {
				http.Error(w, "invalid local provider", http.StatusInternalServerError)
				return
			}
			localProv.HandleUpload(w, req, token)
		})

		r.Get("/files/*", func(w http.ResponseWriter, req *http.Request) {
			key := chi.URLParam(req, "*")
			p, err := s.storageMgr.Get("local")
			if err != nil {
				http.Error(w, "local storage not configured", http.StatusNotFound)
				return
			}
			localProv, ok := p.(*storage.LocalProvider)
			if !ok {
				http.Error(w, "invalid local provider", http.StatusInternalServerError)
				return
			}
			localProv.HandleServe(w, req, key)
		})
	})

	// Server-Sent Events stream for real-time dashboard updates
	r.Get("/v1/events", s.sseHub.ServeHTTP)

	// API endpoints protected by APIKey / Bearer auth
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)

		r.Post("/v1/uploads/presign", s.HandlePresign)
		r.Post("/v1/uploads/presign-multipart", s.HandlePresignMultipart)
		r.Post("/v1/uploads/{id}/complete", s.HandleComplete)
		r.Get("/v1/uploads/{id}", s.HandleGetUpload)
		r.Get("/v1/uploads", s.HandleListUploads)

		r.Get("/v1/buckets", s.HandleListBuckets)
		r.Get("/v1/webhooks", s.HandleListWebhooks)
		r.Post("/v1/webhooks/{id}/retry", s.HandleRetryWebhook)
	})

	// Embedded client widget scripts (/widget/*)
	if s.widgetFS != nil {
		widgetHandler := http.StripPrefix("/widget", http.FileServer(http.FS(s.widgetFS)))
		r.Get("/widget/*", func(w http.ResponseWriter, req *http.Request) {
			widgetHandler.ServeHTTP(w, req)
		})
	}

	// Embedded React 19 web cockpit
	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
			path := strings.TrimPrefix(req.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			// If file exists in embedded FS, serve it
			if f, err := s.webFS.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, req)
				return
			}

			// SPA fallback: serve index.html for client-side routing
			req.URL.Path = "/"
			fileServer.ServeHTTP(w, req)
		})
	}
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If API key is not configured or left empty, allow requests in dev mode
		if s.cfg.Server.APIKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		apiKey := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			apiKey = r.Header.Get("X-API-Key")
		}

		// Also check query param ?api_key= for convenience in widget/tester
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if apiKey != s.cfg.Server.APIKey {
			respondJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "unauthorized: invalid or missing API key",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
