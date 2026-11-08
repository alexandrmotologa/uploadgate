package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexandrmotologa/uploadgate"
	"github.com/alexandrmotologa/uploadgate/internal/api"
	"github.com/alexandrmotologa/uploadgate/internal/config"
	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/alexandrmotologa/uploadgate/internal/gc"
	"github.com/alexandrmotologa/uploadgate/internal/processor"
	"github.com/alexandrmotologa/uploadgate/internal/storage"
	"github.com/alexandrmotologa/uploadgate/internal/validator"
	"github.com/alexandrmotologa/uploadgate/internal/webhook"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "serve":
		cmdServe(os.Args[2:])
	case "verify":
		cmdVerify(os.Args[2:])
	case "presign":
		cmdPresign(os.Args[2:])
	case "health":
		cmdHealth(os.Args[2:])
	case "version":
		fmt.Printf("UploadGate v%s (Go direct-to-storage presigned gateway)\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`UploadGate — Direct-to-Storage Presigned Upload Gateway & Asset Ingestion Daemon (v%s)

Usage:
  uploadgate <command> [arguments]

Commands:
  serve      Start the API gateway daemon, embedded cockpit, and webhook workers
  verify     Inspect file magic bytes, extract metadata, and test security parity
  presign    Generate a presigned upload URL directly from the terminal
  health     Ping the health probe of a running UploadGate instance
  version    Show daemon version

Examples:
  uploadgate serve --config config.yaml
  uploadgate verify ./sample.png
  uploadgate presign --bucket local --file avatar.jpg --size 102400
  uploadgate health --url http://localhost:8080/healthz
`, version)
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "Path to YAML configuration file")
	portOverride := fs.Int("port", 0, "Override server port")
	_ = fs.Parse(args)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("[UploadGate] Configuration error: %v", err)
	}

	if *portOverride > 0 {
		cfg.Server.Port = *portOverride
	}

	database, err := db.OpenDatabase(cfg.Server.DBPath)
	if err != nil {
		log.Fatalf("[UploadGate] Database error: %v", err)
	}
	defer database.Close()

	repo := db.NewRepository(database)

	storageMgr, err := storage.NewManager(cfg)
	if err != nil {
		log.Fatalf("[UploadGate] Storage manager error: %v", err)
	}

	webhookDispatcher := webhook.NewDispatcher(repo, &cfg.Webhooks)
	defer webhookDispatcher.Stop()

	cleaner := gc.NewCleaner(repo, storageMgr, 15*time.Minute, 2*time.Hour)
	cleaner.Start()
	defer cleaner.Stop()

	server := api.NewServer(api.ServerOptions{
		Config:            cfg,
		Repo:              repo,
		StorageMgr:        storageMgr,
		WebhookDispatcher: webhookDispatcher,
		WebFS:             uploadgate.GetWebFS(),
		WidgetFS:          uploadgate.GetWidgetFS(),
	})

	httpServer := &http.Server{
		Addr:         cfg.Server.GetAddress(),
		Handler:      server.Router(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	go func() {
		log.Printf("[UploadGate] Daemon listening on http://%s (Cockpit at /)", cfg.Server.GetAddress())
		log.Printf("[UploadGate] Storage default provider: %s", cfg.Storage.DefaultProvider)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[UploadGate] Server failed: %v", err)
		}
	}()

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[UploadGate] Shutting down daemon gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("[UploadGate] Server shutdown error: %v", err)
	}
	log.Println("[UploadGate] Daemon stopped.")
}

func cmdVerify(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: uploadgate verify <file-path>\n")
		os.Exit(1)
	}

	filePath := args[0]
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to read file %q: %v", filePath, err)
	}

	detectedMIME, _, err := validator.SniffHeader(bytes.NewReader(fileBytes))
	if err != nil {
		detectedMIME = "application/octet-stream"
	}

	meta, err := processor.ExtractMetadata(bytes.NewReader(fileBytes), detectedMIME)
	if err != nil {
		log.Fatalf("Failed to extract metadata: %v", err)
	}

	fmt.Printf("\n--- UploadGate Asset Verification Report ---\n")
	fmt.Printf("File:          %s\n", filePath)
	fmt.Printf("Size:          %d bytes (%.2f KB)\n", len(fileBytes), float64(len(fileBytes))/1024)
	fmt.Printf("Detected MIME: %s\n", detectedMIME)
	fmt.Printf("SHA-256:       %s\n", meta.SHA256)

	if meta.Width > 0 {
		fmt.Printf("Dimensions:    %d x %d px\n", meta.Width, meta.Height)
	}
	if meta.DurationSeconds > 0 {
		fmt.Printf("Duration:      %.2f seconds\n", meta.DurationSeconds)
	}

	if detectedMIME == "image/svg+xml" {
		ok, reason := validator.SanitizeSVG(bytes.NewReader(fileBytes))
		if !ok {
			fmt.Printf("Security:      FAILED (Stored XSS detected: %s)\n", reason)
			os.Exit(1)
		}
		fmt.Printf("Security:      PASSED (Clean SVG, zero active scripts)\n")
	}

	if detectedMIME == "application/x-dosexec" || detectedMIME == "application/x-executable" {
		fmt.Printf("Security:      CRITICAL VIOLATION (Binary executable detected)\n")
		os.Exit(1)
	}

	fmt.Printf("Status:        VERIFIED VALID\n\n")
}

func cmdPresign(args []string) {
	fs := flag.NewFlagSet("presign", flag.ExitOnError)
	configPath := fs.String("config", "config.yaml", "Path to config file")
	bucketName := fs.String("bucket", "", "Target bucket/provider name")
	fileName := fs.String("file", "test.png", "Filename")
	sizeBytes := fs.Int64("size", 1024, "Size in bytes")
	mimeType := fs.String("mime", "image/png", "MIME type")
	_ = fs.Parse(args)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	storageMgr, err := storage.NewManager(cfg)
	if err != nil {
		log.Fatalf("Storage manager error: %v", err)
	}

	prov, err := storageMgr.Get(*bucketName)
	if err != nil {
		log.Fatalf("Provider error: %v", err)
	}

	key := fmt.Sprintf("uploads/cli_%s", *fileName)
	res, err := prov.PresignPut(context.Background(), key, *mimeType, *sizeBytes, cfg.Limits.PresignExpiry())
	if err != nil {
		log.Fatalf("Presign error: %v", err)
	}

	fmt.Printf("\n--- Presigned Upload Ticket ---\n")
	fmt.Printf("Upload ID:     %s\n", res.UploadID)
	fmt.Printf("Method:        %s\n", res.Method)
	fmt.Printf("URL:           %s\n", res.URL)
	fmt.Printf("Storage Key:   %s\n", res.StorageKey)
	fmt.Printf("Expires:       %s\n\n", res.ExpiresAt.Format(time.RFC3339))
}

func cmdHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	url := fs.String("url", "http://localhost:8080/healthz", "Health endpoint URL")
	_ = fs.Parse(args)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(*url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check unhealthy: HTTP %d, body: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Printf("Health OK: %s\n", string(body))
}
