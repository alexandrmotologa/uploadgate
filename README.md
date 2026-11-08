# UploadGate

> Direct-to-storage presigned upload gateway and asset ingestion daemon with binary magic-byte validation, responsive WebP generation, and HMAC-signed webhooks under 40MB RAM.

UploadGate acts as a secure orchestrator between client applications, backend services, and object storage providers (AWS S3, Cloudflare R2, MinIO, Google Cloud Storage, or local disk). Frontends request a presigned upload ticket, transfer bytes directly to object storage without routing payload traffic through your application servers, and notify UploadGate upon completion. UploadGate then validates file magic bytes, checks SVGs for stored XSS vectors, extracts dimensions and EXIF metadata, synthesizes responsive WebP thumbnails, and dispatches HMAC-SHA256 signed webhooks to your application backend.

---

## Key Capabilities

* **Direct-to-Storage Presigning**: Issues AWS SigV4 presigned PUT URLs for single uploads and multipart presigned tickets for files from 50MB up to 5GB.
* **Multi-Cloud Drivers**: Built-in support for AWS S3, Cloudflare R2, MinIO, Google Cloud Storage (S3-compatible mode), and local filesystem storage for testing.
* **Binary Magic-Byte Inspection**: Sniffs binary headers across JPEG, PNG, GIF, WebP, AVIF, PDF, MP4, WebM, and ZIP files to reject executable binaries disguised under false extensions.
* **Stored XSS Vector Neutralization**: Parses SVG uploads to block scripts, foreign objects, and malicious XML event handlers.
* **Pure-Go Thumbnail Pipeline**: Resizes images and produces responsive WebP and JPEG thumbnail variants (150x150, 600x600) without external dependencies such as ImageMagick or libvips.
* **Reliable Outbound Webhooks**: Dispatches HMAC-SHA256 signed event payloads with automatic exponential backoff retries.
* **Real-Time Event Stream**: Streams live ingestion events over native Server-Sent Events (SSE) to the embedded admin cockpit.
* **Embedded React 19 Cockpit**: Single-binary administration interface featuring live ingestion feeds, bucket status cards, and an interactive presign tester.
* **Drop-In Client Widget**: Includes a React 19 component and a standalone vanilla web component under 3KB gzipped.
* **Low Resource Footprint**: Written in Go with embedded SQLite (WAL mode), operating in under 40MB of resident RAM.

---

## Architecture Overview

```
[ Client Browser / Mobile App ]
          │
          │ 1. Request Presigned URL (POST /v1/uploads/presign)
          ▼
┌────────────────────────────────────────────────────────────────────────┐
│                        UploadGate Daemon                               │
│                                                                        │
│  [ Policy Validator ] ──► [ Multi-Cloud Presigner (S3, R2, MinIO, FS) ]│
└──────────────────────────────────┬─────────────────────────────────────┘
                                   │ 2. Return Presigned URL & Headers
                                   ▼
[ Client Browser ] ───────────────────────────────────────────────────────┐
          │                                                               │
          │ 3. Direct Binary PUT (Bypasses Application Server)            │
          ▼                                                               │
[ Object Storage: S3 / Cloudflare R2 / MinIO / Local ]                    │
          │                                                               │
          │ 4. Notify Upload Completed (POST /v1/uploads/:id/complete)    │
          ▼                                                               │
┌─────────────────────────────────────────────────────────────────────────┴┐
│ Ingestion & Verification Engine                                          │
│ - Magic-Byte Sniffer (Validates declared MIME vs binary header)          │
│ - SVG Security Sanitizer (Blocks XSS and malicious event handlers)       │
│ - Checksum Verifier (SHA-256 calculation & ETag comparison)              │
│ - Metadata Extractor (Dimensions, EXIF stripping)                        │
│ - Thumbnail Synthesizer (Pure-Go WebP/JPEG generation)                   │
└──────────────────────────────────┬───────────────────────────────────────┘
                                   │ 5. HMAC-SHA256 Signed Webhook
                                   ▼
[ Application Backend API (Node.js, Go, Python, Ruby, PHP) ]
```

---

## Quick Start

### Run with Docker Compose

UploadGate provides a complete local stack with MinIO:

```bash
docker compose up -d
```

The gateway listens on `http://localhost:8080`, and the MinIO storage console is available at `http://localhost:9001`.

### Build from Source

Requirements: Go 1.24+ and Node.js 22+.

```bash
# Clone the repository
git clone https://github.com/alexandrmotologa/uploadgate.git
cd uploadgate

# Build frontend cockpit and Go binary
make build

# Run the daemon
./bin/uploadgate serve --config config.example.yaml
```

---

## Configuration

UploadGate reads configuration from YAML files or environment variables:

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  api_key: "ug_secret_key_change_in_production"
  cors_allowed_origins:
    - "http://localhost:3000"
    - "https://app.example.com"

storage:
  default_provider: "s3"
  providers:
    s3:
      endpoint: "https://s3.us-east-1.amazonaws.com"
      bucket: "production-assets"
      region: "us-east-1"
      access_key_id: "${AWS_ACCESS_KEY_ID}"
      secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
      public_base_url: "https://cdn.example.com"
    r2:
      endpoint: "https://<account-id>.r2.cloudflarestorage.com"
      bucket: "media-assets"
      region: "auto"
      access_key_id: "${R2_ACCESS_KEY_ID}"
      secret_access_key: "${R2_SECRET_ACCESS_KEY}"
      public_base_url: "https://media.example.com"
    local:
      directory: "./local_storage"
      public_base_url: "http://localhost:8080/v1/storage/local"

limits:
  max_file_size_bytes: 104857600 # 100MB
  allowed_mime_types:
    - "image/jpeg"
    - "image/png"
    - "image/webp"
    - "image/gif"
    - "image/svg+xml"
    - "application/pdf"
    - "video/mp4"

thumbnails:
  enabled: true
  formats: ["webp", "jpeg"]
  variants:
    - name: "thumb"
      width: 150
      height: 150
      fit: "crop"
    - name: "medium"
      width: 600
      height: 600
      fit: "contain"

webhooks:
  endpoint: "https://api.example.com/webhooks/uploads"
  secret: "whsec_test_secret_1234567890"
  max_retries: 5
```

---

## API Reference

### 1. Request Presigned Upload URL

```http
POST /v1/uploads/presign
Authorization: Bearer ug_secret_key_change_in_production
Content-Type: application/json

{
  "filename": "avatar.png",
  "size_bytes": 204800,
  "mime_type": "image/png",
  "bucket": "production-assets"
}
```

Response:

```json
{
  "upload_id": "upl_01JBCDEF89012345",
  "presigned_url": "https://production-assets.s3.us-east-1.amazonaws.com/uploads/2026/11/08/avatar.png?X-Amz-Signature=...",
  "method": "PUT",
  "headers": {
    "Content-Type": "image/png"
  },
  "storage_key": "uploads/2026/11/08/avatar.png",
  "expires_at": "2026-11-08T21:00:00Z"
}
```

### 2. Complete Upload & Run Ingestion Pipeline

```http
POST /v1/uploads/upl_01JBCDEF89012345/complete
Authorization: Bearer ug_secret_key_change_in_production
Content-Type: application/json

{}
```

Response:

```json
{
  "upload_id": "upl_01JBCDEF89012345",
  "status": "verified",
  "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "detected_mime": "image/png",
  "width": 800,
  "height": 800,
  "thumbnails": [
    {
      "name": "thumb",
      "width": 150,
      "height": 150,
      "url": "https://cdn.example.com/thumbnails/avatar_150x150.webp"
    }
  ]
}
```

---

## Client Widget Integration

### React 19

```tsx
import { FileUpload } from "@uploadgate/react";

export function UserAvatarUpload() {
  return (
    <FileUpload
      endpoint="http://localhost:8080"
      apiKey="ug_public_client_key"
      accept="image/*"
      maxSizeBytes={5 * 1024 * 1024}
      onUploadComplete={(asset) => {
        console.log("Uploaded asset:", asset.storage.public_url);
      }}
      onError={(err) => {
        console.error("Upload error:", err.message);
      }}
    />
  );
}
```

### Vanilla HTML / Web Component

```html
<script src="http://localhost:8080/widget/uploadgate-picker.min.js"></script>

<uploadgate-picker
  endpoint="http://localhost:8080"
  api-key="ug_public_client_key"
  accept="image/png,image/jpeg"
></uploadgate-picker>

<script>
  const picker = document.querySelector('uploadgate-picker');
  picker.addEventListener('complete', (e) => {
    console.log('Upload complete:', e.detail);
  });
</script>
```

---

## License

MIT License. Developed by Alexandr Motologa.
