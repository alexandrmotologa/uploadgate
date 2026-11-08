# `uploadgate` — Architecture & Engineering Master Plan

> **Direct-to-Storage Presigned Upload Gateway & Asset Ingestion Daemon (< 40MB RAM)**  
> *Self-Hosted Uploadcare, Filestack, and Transloadit Alternative in a Single Binary (Go 1.24+ & React 19)*

---

## 1. Executive Summary & Problem Space

### The Core Problem
Handling file uploads (user avatars, documents, CSV datasets, video clips) is one of the most tedious and fragile components of web and mobile software:
1. **Application Server Bottlenecks**: Streaming uploads through application servers (Node.js, Python, Ruby, PHP) consumes massive heap memory, ties up worker threads during slow mobile uploads, and easily triggers request body size limits and gateway timeouts.
2. **Commercial SaaS Lock-In (Uploadcare, Filestack, Transloadit)**: Metered cloud upload platforms charge $49 to $299+ per month, enforce bandwidth quotas, and store customer files inside proprietary third-party cloud accounts.
3. **Complex DIY Direct-to-Storage Pipelines**: Implementing presigned S3/R2 direct uploads in-house requires complex S3 CORS rules, IAM policy configurations, multipart chunk assembly, client-side signature generation, magic-byte security checks, and thumbnail pipelines.

### The Solution: `uploadgate`
`uploadgate` is a lightweight, single-binary asset ingestion gateway written in Go with an embedded React 19 asset cockpit and a 3KB client-side `<FileUpload />` widget. It acts as an orchestrator between your frontend, your backend, and your object storage (AWS S3, Cloudflare R2, Google Cloud Storage, MinIO, or local disk).

Frontends request a presigned upload ticket (`POST /v1/uploads/presign`), upload bytes directly to the storage bucket, and notify `uploadgate` on completion. `uploadgate` validates file magic bytes (preventing malicious `.exe` files disguised as `.png`), checks SVG files for stored XSS payloads, extracts image/video metadata, generates responsive thumbnails, and fires an HMAC-signed webhook to your application backend.

---

## 2. System Architecture & Upload Lifecycle

```
[ Client Browser / Mobile App ]
          │  1. POST /v1/uploads/presign { filename, size, mime_type }
          ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        `uploadgate` Gateway Daemon                     │
│                                                                        │
│  ┌───────────────────────┐         ┌────────────────────────────────┐  │
│  │ API Key Authorization │ ──────► │ Policy & MIME Validator        │  │
│  └───────────────────────┘         └────────────────┬───────────────┘  │
│                                                     │                  │
│                                                     ▼                  │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Multi-Cloud Presigner (AWS S3, Cloudflare R2, GCS, MinIO, Local) │  │
│  │ - Generates AWS SigV4 Presigned URL (or S3 Multipart Upload ID)  │  │
│  │ - Local signed token stream for air-gapped / on-premise storage  │  │
│  └──────────────────────────────────┬───────────────────────────────┘  │
│                                     │ 2. Returns Presigned URL Ticket  │
│                                     ▼                                  │
│  [ Client Browser ] ◄───────────────────────────────────────────────┘  │
│          │                                                             │
│          │  3. Direct PUT/POST to Storage Bucket (Bypasses Backend)    │
│          ▼                                                             │
│  [ Object Storage Bucket: S3 / Cloudflare R2 / MinIO / Local ]         │
│          │                                                             │
│          │  4. Client notifies: POST /v1/uploads/:id/complete          │
│          ▼                                                             │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Ingestion & Verification Engine                                  │  │
│  │ - Magic-Byte Sniffer: inspects binary signature for MIME parity  │  │
│  │ - SVG Security Sanitizer: blocks stored XSS and active scripts   │  │
│  │ - Checksum Verifier: SHA-256 / ETag integrity check              │  │
│  │ - Metadata Extractor: image dimensions (W x H), EXIF orientation │  │
│  │ - Thumbnail Synthesizer: pure-Go WebP/JPEG responsive variants   │  │
│  └──────────────────────────────────┬───────────────────────────────┘  │
│                                     │ 5. HMAC-SHA256 Webhook Dispatch  │
│                                     ▼                                  │
│  [ Application Backend API (Node, Python, Go, Ruby) ]                  │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐  │
│  │ Real-Time SSE Hub & Embedded React 19 Asset Cockpit              │  │
│  │ - Live Ingestion Stream, Storage Bucket Browser, Presign Tester  │  │
│  │ - Webhook Replay Console & Prometheus Metrics (/metrics)         │  │
│  └──────────────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Technology Stack

| Layer | Component | Specification |
| :--- | :--- | :--- |
| **Language & Runtime** | Go 1.24+ | Native execution, low memory overhead (< 40MB RAM), zero CGo dependencies |
| **HTTP Routing** | `go-chi/chi/v5` | Lightweight, RFC-compliant REST API router with middleware chain |
| **Storage Connectors** | Multi-Cloud Adapter | AWS S3, Cloudflare R2, MinIO, GCS (S3-compatible API), and Local Filesystem |
| **Multipart Engine** | S3 Multipart Presigner | Chunked presigning for large files (50MB to 5GB+) |
| **Security Validation** | Magic-Byte Sniffer | Binary file signature validation against declared MIME types |
| **XSS Shield** | SVG Sanitizer | XML token scanner detecting `<script>`, `javascript:`, and malicious event handlers |
| **Thumbnail Engine** | Pure-Go Image Resizer | High-quality Lanczos/Bilinear resampler creating responsive WebP/JPEG variants |
| **Database & Cache** | Embedded SQLite (WAL) | Fast, single-file catalog for upload records, audit trails, and webhook logs |
| **Real-Time Stream** | Server-Sent Events (SSE) | Zero-overhead native SSE pub/sub broadcasting upload events |
| **Webhook Engine** | Reliable Outbound Webhooks | Exponential backoff retry engine with HMAC-SHA256 signatures |
| **Client UI Component** | 3KB Drop-In Widget | Vanilla Web Component & React 19 `<FileUpload />` component |
| **Embedded Admin UI** | React 19 + TypeScript + Vite | Built as a single bundle and served via Go `//go:embed` |
| **UI Styling** | Tailwind CSS + Lucide Icons | Clean modern dark-mode asset management dashboard |
| **Observability** | Prometheus Metrics & Health | `/metrics` and `/healthz` endpoints for monitoring |

---

## 4. Repository & Directory Structure

```
B:\workgit\uploadgate\
├── cmd\
│   └── uploadgate\
│       └── main.go                 # Gateway daemon entrypoint, CLI commands (serve, verify, presign, health)
├── internal\
│   ├── config\
│   │   └── config.go               # Configuration (storage providers, bucket credentials, limits, tokens)
│   ├── api\
│   │   ├── router.go               # Chi router, CORS, auth middleware, SSE hub
│   │   ├── presign.go              # POST /v1/uploads/presign (single & multipart)
│   │   ├── complete.go             # POST /v1/uploads/:id/complete handler & pipeline
│   │   ├── assets.go               # GET /v1/assets/:id metadata inspection & download
│   │   ├── events.go               # GET /v1/events SSE real-time stream
│   │   ├── webhooks_api.go         # GET /v1/webhooks history & POST /v1/webhooks/:id/retry
│   │   └── metrics.go              # GET /metrics Prometheus collector
│   ├── storage\
│   │   ├── provider.go             # Storage backend interface
│   │   ├── s3.go                   # AWS S3, Cloudflare R2, MinIO SigV4 presigner & object reader
│   │   ├── local.go                # Local disk direct-upload handler & streaming token validator
│   │   └── multipart.go            # S3 multipart upload lifecycle
│   ├── validator\
│   │   ├── magic_bytes.go          # Binary signature detector (JPEG, PNG, WebP, AVIF, PDF, MP4, etc.)
│   │   ├── svg_sanitizer.go        # Stored XSS protection for vector SVG files
│   │   └── mime_allowlist.go       # Allowed extension and MIME validation rules
│   ├── processor\
│   │   ├── metadata.go             # Image dimensions, EXIF stripper, video/document metadata
│   │   └── thumbnail.go            # Generates WebP & JPEG thumbnails (150x150, 600x600)
│   ├── db\
│   │   ├── sqlite.go               # Embedded SQLite schema, migrations, connection pool
│   │   └── repository.go           # CRUD for uploads, assets, and webhook events
│   ├── webhook\
│   │   └── dispatcher.go           # Outbound webhook sender with exponential backoff & HMAC-SHA256
│   └── widget\
│       └── embed.go                # Serves pre-built client widget uploadgate-widget.min.js
├── web\
│   ├── src\
│   │   ├── components\
│   │   │   ├── IngestionFeed.tsx   # Real-time streaming log of uploaded files and status
│   │   │   ├── AssetPreviewModal.tsx# Deep inspector with EXIF, dimensions, and URL builder
│   │   │   ├── StorageBucketCard.tsx# Active bucket health, capacity, and CORS status
│   │   │   ├── PresignTester.tsx   # Interactive GUI to test presigned upload policies
│   │   │   └── WebhooksMonitor.tsx # Outbound webhook delivery logs and manual replay
│   │   ├── App.tsx                 # Root administration cockpit
│   │   ├── main.tsx                # React 19 entrypoint
│   │   └── index.css               # Tailwind CSS declarations
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── widget\                         # 3KB Drop-In Client-Side File Uploader
│   ├── src\
│   │   ├── uploader.tsx            # React 19 <FileUpload /> component
│   │   └── vanilla.ts              # Web Component <uploadgate-picker>
│   └── vite.config.ts
├── docs\
│   ├── architecture.md             # Detailed data flow and security architecture
│   ├── api_reference.md            # Complete REST and SSE API contract
│   ├── storage_providers.md        # Configuration guides for AWS S3, Cloudflare R2, MinIO, Local
│   └── client_integration.md       # Integration guide for React, Vue, Next.js, and Vanilla JS
├── examples\
│   ├── nextjs-client\              # Example Next.js 15 client integration
│   └── backend-webhook\            # Example Node.js/Go webhook receiver
├── Dockerfile                      # Multi-stage scratch container image (< 25MB)
├── docker-compose.yml              # Local development stack with MinIO & UploadGate
├── Makefile                        # Multi-platform build and release targets
├── README.md                       # Comprehensive guide and integration examples
└── MASTER_PLAN.md                  # This master specification
```

---

## 5. API Specification & Webhook Contract

### 1. Requesting a Presigned Single Ticket (`POST /v1/uploads/presign`)

```json
{
  "filename": "team_avatar.png",
  "size_bytes": 1048576,
  "mime_type": "image/png",
  "bucket": "production-assets",
  "metadata": {
    "user_id": "usr_9921",
    "organization_id": "org_45"
  }
}
```

**Response (`200 OK`)**:
```json
{
  "upload_id": "upl_01JBCDEF89012345",
  "presigned_url": "https://production-assets.s3.us-east-1.amazonaws.com/uploads/2026/11/08/team_avatar.png?X-Amz-Signature=...",
  "method": "PUT",
  "headers": {
    "Content-Type": "image/png",
    "x-amz-meta-upload-id": "upl_01JBCDEF89012345"
  },
  "storage_key": "uploads/2026/11/08/team_avatar.png",
  "expires_at": "2026-11-08T23:00:00Z"
}
```

### 2. Requesting a Multipart Upload (`POST /v1/uploads/presign-multipart`)

```json
{
  "filename": "raw_footage.mp4",
  "size_bytes": 524288000,
  "mime_type": "video/mp4",
  "part_count": 5
}
```

**Response (`200 OK`)**:
```json
{
  "upload_id": "upl_01JBCDEF99999999",
  "multipart_upload_id": "mp_abc123xyz",
  "part_size": 104857600,
  "parts": [
    { "part_number": 1, "presigned_url": "https://bucket.s3.amazonaws.com/raw_footage.mp4?partNumber=1&uploadId=mp_abc123xyz&..." },
    { "part_number": 2, "presigned_url": "https://bucket.s3.amazonaws.com/raw_footage.mp4?partNumber=2&uploadId=mp_abc123xyz&..." }
  ]
}
```

### 3. Client Completion Notification (`POST /v1/uploads/:id/complete`)

```json
{
  "parts": [
    { "part_number": 1, "etag": "\"etag1\"" },
    { "part_number": 2, "etag": "\"etag2\"" }
  ]
}
```

**Response (`200 OK`)**:
```json
{
  "upload_id": "upl_01JBCDEF89012345",
  "status": "verified",
  "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "detected_mime": "image/png",
  "width": 1024,
  "height": 1024,
  "thumbnails": [
    { "width": 150, "height": 150, "url": "https://cdn.example.com/thumbnails/team_avatar_150x150.webp" },
    { "width": 600, "height": 600, "url": "https://cdn.example.com/thumbnails/team_avatar_600x600.webp" }
  ]
}
```

### 4. Outbound Completion Webhook (`upload.completed`)
Delivered to your application backend with signature header `X-UploadGate-Signature: t=1731089200,v1=...`:
```json
{
  "event": "upload.completed",
  "upload_id": "upl_01JBCDEF89012345",
  "status": "verified",
  "storage": {
    "provider": "s3",
    "bucket": "production-assets",
    "key": "uploads/2026/11/08/team_avatar.png",
    "public_url": "https://cdn.example.com/uploads/2026/11/08/team_avatar.png"
  },
  "file": {
    "name": "team_avatar.png",
    "size_bytes": 1048576,
    "declared_mime": "image/png",
    "detected_mime": "image/png",
    "width": 1024,
    "height": 1024,
    "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  },
  "thumbnails": [
    { "width": 150, "height": 150, "url": "https://cdn.example.com/thumbnails/team_avatar_150x150.webp" },
    { "width": 600, "height": 600, "url": "https://cdn.example.com/thumbnails/team_avatar_600x600.webp" }
  ],
  "metadata": {
    "user_id": "usr_9921"
  },
  "timestamp": "2026-11-08T20:26:00Z"
}
```

---

## 6. Implementation Roadmap

### Phase 1: Gateway Core, Storage Connectors & SQLite Catalog
- [x] Initialize Go 1.24 module `github.com/alexandrmotologa/uploadgate`.
- [x] Implement SQLite schema and repository in `internal/db` with WAL mode.
- [x] Implement storage provider interface and multi-cloud drivers:
  - AWS S3, Cloudflare R2, MinIO presigned URL generator (SigV4).
  - S3 Multipart Upload lifecycle (initiate, presign parts, complete).
  - Local filesystem storage adapter with secure streaming tokens.
- [x] Implement `POST /v1/uploads/presign` (single and multipart) with policy rules and size bounds.
- [x] Build configuration loader supporting YAML and environment variables.

### Phase 2: Ingestion Verification, Security Sniffer & Thumbnail Engine
- [x] Implement binary magic-byte sniffer supporting JPEG, PNG, GIF, WebP, AVIF, PDF, MP4, WebM, ZIP, JSON.
- [x] Implement SVG security sanitizer blocking stored XSS, script execution, and malicious attributes.
- [x] Implement checksum calculation (SHA-256) and ETag integrity matching.
- [x] Implement pure-Go image metadata extractor (width, height, orientation) and EXIF sanitizer.
- [x] Implement pure-Go responsive thumbnail synthesizer generating WebP and JPEG variants (150x150, 600x600).
- [x] Implement `POST /v1/uploads/:id/complete` endpoint executing the full verification and ingestion pipeline.

### Phase 3: Real-Time SSE Hub, Webhook Dispatcher & Client Widgets
- [x] Implement Server-Sent Events (SSE) hub at `GET /v1/events` for real-time live ingestion streams.
- [x] Implement outbound webhook dispatcher with exponential backoff retries and HMAC-SHA256 signatures.
- [x] Implement webhook history inspection and manual retry endpoint (`POST /v1/webhooks/:id/retry`).
- [x] Implement 3KB drop-in client widget (`widget/`):
  - React 19 `<FileUpload />` component with drag-and-drop, progress bar, and instant preview.
  - Vanilla Web Component `<uploadgate-picker>` for plain HTML applications.

### Phase 4: Embedded React 19 Asset Cockpit
- [x] Scaffold Vite + React 19 + TypeScript + Tailwind CSS application in `web/`.
- [x] Implement Live Ingestion Feed with real-time SSE stream, status badges, and search filter.
- [x] Implement Storage Buckets Health Card displaying provider, region, and connectivity status.
- [x] Implement Interactive Presign Tester allowing direct browser uploads with progress bars.
- [x] Implement Asset Deep Inspector Modal with metadata, thumbnails, and preview.
- [x] Implement Webhooks Monitor showing delivery status, payload view, and retry button.
- [x] Embed static assets directly into Go binary using `//go:embed`.

### Phase 5: CLI Subcommands, Prometheus Metrics & Production Hardening
- [x] Build CLI subcommands in `cmd/uploadgate`:
  - `serve`: Starts API server, embedded cockpit, and background workers.
  - `verify <path|url>`: Standalone CLI validator for file signatures and metadata.
  - `presign`: CLI generator for test presigned URLs.
  - `health`: Liveness and readiness health checks.
- [x] Implement Prometheus metrics endpoint (`GET /metrics`).
- [x] Write integration test suite covering presigning, magic byte detection, SVG sanitization, and webhooks.
- [x] Prepare Dockerfile (< 25MB scratch image), `docker-compose.yml` with MinIO, and comprehensive documentation in `docs/`.
- [x] Finalize humanized `README.md` with configuration guides and drop-in integration snippets.
