<p align="center">
  <img src="docs/images/logo.png" alt="UploadGate Logo" width="130" style="border-radius: 24px;" />
</p>

<h1 align="center">UploadGate</h1>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/react-19-61DAFB?style=flat-square&logo=react" alt="React 19" />
  <img src="https://img.shields.io/badge/memory-%3C40MB%20RAM-emerald?style=flat-square" alt="Memory Footprint" />
  <img src="https://img.shields.io/badge/storage-S3%20%7C%20R2%20%7C%20GCS%20%7C%20MinIO%20%7C%20Local-blue?style=flat-square" alt="Storage Providers" />
  <img src="https://img.shields.io/badge/security-magic--bytes%20%2B%20anti--SSRF-purple?style=flat-square" alt="Security" />
  <img src="https://img.shields.io/badge/license-MIT-slate?style=flat-square" alt="License" />
</p>

<p align="center">
  <strong>Direct-to-storage presigned upload gateway and asset ingestion daemon with binary magic byte validation, responsive WebP generation, and HMAC-signed webhooks under 40MB resident RAM.</strong>
</p>

<p align="center">
  <a href="#key-capabilities">Key Capabilities</a> &bull;
  <a href="#architecture">Architecture</a> &bull;
  <a href="#quick-start">Quick Start</a> &bull;
  <a href="#live-cockpit--interface">Live Cockpit</a> &bull;
  <a href="#api-reference">API Reference</a> &bull;
  <a href="#client-widget">Client Widget</a> &bull;
  <a href="#configuration">Configuration</a>
</p>

---

## Live Demonstration

<p align="center">
  <img src="docs/images/uploadgate_demo.gif" alt="UploadGate Live Execution and Ingestion Demo" width="96%" style="border-radius: 12px; border: 1px solid #1e293b;" />
</p>

UploadGate coordinates client browser transfers directly into cloud object storage (AWS S3, Cloudflare R2, Google Cloud Storage, MinIO, or local disk). Frontends receive an authenticated presigned ticket and upload payloads directly to target storage, removing file transfer load from application servers. Upon completion, UploadGate checks magic bytes to detect spoofed binaries, sanitizes SVGs against stored XSS, extracts dimensions, builds responsive WebP thumbnails, and delivers HMAC-SHA256 signed webhooks to your backend services.

---

## Live Cockpit & Interface

UploadGate includes a built-in single-binary management cockpit built with React 19 and Tailwind CSS. The interface provides real-time SSE telemetry, interactive upload sandboxes, and outbound webhook delivery inspection.

<p align="center">
  <img src="docs/images/uploadgate_dashboard.png" alt="UploadGate Desktop Cockpit" width="98%" style="border-radius: 8px; border: 1px solid #1e293b;" />
</p>

### Responsive Multi-Device Support

The web cockpit renders responsively across desktop, tablet, and mobile displays with compact telemetry ribbons and touch-friendly controls:

<p align="center">
  <img src="docs/images/uploadgate_tablet.png" alt="UploadGate Tablet Layout" width="58%" style="border-radius: 8px; border: 1px solid #1e293b; margin-right: 1%;" />
  <img src="docs/images/uploadgate_mobile.png" alt="UploadGate Mobile Layout" width="37%" style="border-radius: 8px; border: 1px solid #1e293b;" />
</p>

### Deep Security Inspection & Disguised Payload Neutralization

When a malicious executable disguised as an image is uploaded, UploadGate's magic-byte analyzer detects the discrepancy, terminates processing with HTTP 422, marks the record as `REJECTED`, and logs the security violation:

<p align="center">
  <img src="docs/images/uploadgate_asset_modal.png" alt="UploadGate Security Inspection Modal" width="90%" style="border-radius: 8px; border: 1px solid #1e293b;" />
</p>

---

## Key Capabilities

* **Direct Presigned Transfers**: Generates AWS SigV4 presigned PUT URLs for single files and chunked multipart presign plans for objects up to 5GB.
* **Multi-Cloud Drivers**: Native storage adapters for AWS S3, Cloudflare R2, MinIO, Google Cloud Storage, and local filesystem paths.
* **Magic Byte Verification**: Inspects file headers to verify genuine formats (PNG, JPEG, WebP, AVIF, GIF, PDF, MP4, WebM, ZIP) and blocks disguised executables (ELF, Windows PE, shell scripts).
* **Anti-SSRF Safe Fetcher**: Dial-time IP validation stops SSRF exploits during URL ingestion by blocking private, loopback, link-local, and cloud metadata addresses (169.254.169.254).
* **SVG XSS Sanitizer**: Strips script tags, external entities, foreign objects, and malicious event handlers from uploaded SVG graphics.
* **Pure-Go Image Pipeline**: Built-in resizing and dual WebP/JPEG generation with bilinear interpolation, avoiding ImageMagick and libvips system dependencies.
* **Signed HMAC Webhooks**: Emits `upload.completed` and `upload.failed` events with SHA-256 signatures (`X-UploadGate-Signature: t=...,v1=...`) and exponential backoff retry.
* **Real-Time Telemetry**: Broadcasts Server-Sent Events (SSE) to connected management clients with zero polling.
* **Lightweight Embedded Footprint**: Built with Go and embedded SQLite (WAL mode), running in under 40MB resident RAM.

---

## Architecture

```
[ Client Browser / Mobile App ]
          │
          │ 1. Request Presigned Ticket (POST /v1/uploads/presign)
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
          │ 3. Direct Binary PUT (Payload Bypasses App Server)            │
          ▼                                                               │
[ Object Storage: S3 / Cloudflare R2 / MinIO / Local ]                    │
          │                                                               │
          │ 4. Notify Upload Completed (POST /v1/uploads/:id/complete)    │
          ▼                                                               │
┌─────────────────────────────────────────────────────────────────────────┴┐
│ Ingestion & Verification Engine                                          │
│ - Magic Byte Sniffer (Validates declared MIME vs binary header)          │
│ - SVG Security Sanitizer (Blocks stored XSS and malicious XML handlers)  │
│ - Anti-SSRF URL Ingestion Guard (Blocks RFC 1918 and metadata endpoints) │
│ - Checksum Verifier (SHA-256 calculation & ETag comparison)              │
│ - Metadata Extractor (Image dimensions & MP4 atom duration)              │
│ - Thumbnail Synthesizer (Pure-Go WebP/JPEG generation)                   │
└──────────────────────────────────┬───────────────────────────────────────┘
                                   │ 5. HMAC-SHA256 Signed Webhook
                                   ▼
[ Application Backend API (Node.js, Go, Python, Ruby, PHP) ]
```

---

## Quick Start

### Using Docker Compose

Run UploadGate alongside a preconfigured local MinIO instance:

```bash
docker compose up -d
```

The gateway listens at `http://localhost:8080`, and the MinIO storage console runs at `http://localhost:9001` (user: `minioadmin`, pass: `minioadmin`).

### Building from Source

Prerequisites: Go 1.24+ and Node.js 22+.

```bash
# Clone the repository
git clone https://github.com/alexandrmotologa/uploadgate.git
cd uploadgate

# Compile frontend cockpit and Go binary
make build

# Launch the daemon
./bin/uploadgate serve --config config.example.yaml
```

Verify service status:

```bash
./bin/uploadgate health --url http://localhost:8080
```

---

## API Reference

All administrative calls require the configured `Authorization: Bearer <API_KEY>` header.

### 1. Request Presigned URL

```http
POST /v1/uploads/presign
Authorization: Bearer ug_secret_key_change_in_production
Content-Type: application/json

{
  "filename": "team-photo.png",
  "size_bytes": 1048576,
  "mime_type": "image/png",
  "bucket": "local"
}
```

Response (`200 OK`):

```json
{
  "upload_id": "upl_01JBCDEF89012345",
  "presigned_url": "http://localhost:8080/v1/storage/local/upload/eyJ1cGxvYW...sig",
  "method": "PUT",
  "headers": {
    "Content-Type": "image/png"
  },
  "storage_key": "uploads/2026/11/08/9a8b7c_team-photo.png",
  "expires_at": "2026-11-08T22:00:00Z"
}
```

### 2. Complete Upload & Run Ingestion

```http
POST /v1/uploads/upl_01JBCDEF89012345/complete
Authorization: Bearer ug_secret_key_change_in_production
Content-Type: application/json

{}
```

Response (`200 OK`):

```json
{
  "id": "upl_01JBCDEF89012345",
  "filename": "team-photo.png",
  "original_size": 1048576,
  "declared_mime": "image/png",
  "detected_mime": "image/png",
  "status": "verified",
  "sha256": "4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a",
  "width": 1920,
  "height": 1080,
  "public_url": "http://localhost:8080/v1/storage/local/files/uploads/2026/11/08/9a8b7c_team-photo.png",
  "thumbnails": [
    {
      "variant_name": "thumb",
      "format": "webp",
      "width": 150,
      "height": 150,
      "public_url": "http://localhost:8080/v1/storage/local/files/thumbnails/2026/11/08/9a8b7c_thumb.webp"
    }
  ]
}
```

### 3. Remote URL Ingestion (Anti-SSRF Protected)

Ingest assets directly from public HTTP endpoints:

```http
POST /v1/uploads/from-url
Authorization: Bearer ug_secret_key_change_in_production
Content-Type: application/json

{
  "url": "https://images.unsplash.com/photo-1579783900882-c0d3dad7b119",
  "filename": "landscape.jpg",
  "bucket": "local"
}
```

---

## Client Widget Integration

UploadGate includes a compact client widget package under 3KB gzipped.

### React 19 Component

```tsx
import { FileUpload } from "@uploadgate/react";

export function ProfilePhotoUploader() {
  return (
    <FileUpload
      endpoint="http://localhost:8080"
      apiKey="ug_client_public_token"
      bucket="s3"
      accept="image/png,image/jpeg,image/webp"
      maxSizeBytes={10 * 1024 * 1024}
      autoCompress={true}
      maxWidth={2048}
      onUploadComplete={(asset) => {
        console.log("Uploaded successfully:", asset.public_url);
      }}
      onError={(err) => {
        console.error("Upload failed:", err.message);
      }}
    />
  );
}
```

### Vanilla Web Component

```html
<script src="http://localhost:8080/widget/uploadgate-picker.min.js"></script>

<uploadgate-picker
  endpoint="http://localhost:8080"
  api-key="ug_client_public_token"
  bucket="local"
  accept="image/*,application/pdf"
></uploadgate-picker>

<script>
  const picker = document.querySelector('uploadgate-picker');
  picker.addEventListener('uploadgate:complete', (event) => {
    console.log('Ingestion verified:', event.detail);
  });
</script>
```

---

## Webhook Signature Verification

Outbound webhooks include a cryptographic signature header:

```
X-UploadGate-Signature: t=1762635600,v1=9f83...6a12
```

Validate signatures in your application:

```javascript
const crypto = require('crypto');

function verifyWebhook(payloadRaw, signatureHeader, secret) {
  const parts = Object.fromEntries(
    signatureHeader.split(',').map((pair) => pair.split('='))
  );

  const timestamp = parts.t;
  const signature = parts.v1;
  const signedPayload = `${timestamp}.${payloadRaw}`;

  const expected = crypto
    .createHmac('sha256', secret)
    .update(signedPayload)
    .digest('hex');

  return crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(expected));
}
```

---

## Configuration Reference

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  api_key: "ug_secret_key_change_in_production"
  cors_allowed_origins:
    - "http://localhost:3000"
    - "https://app.example.com"
  db_path: "./data/uploadgate.db"

storage:
  default_provider: "s3"
  providers:
    s3:
      type: "s3"
      endpoint: "https://s3.us-east-1.amazonaws.com"
      bucket: "production-assets"
      region: "us-east-1"
      access_key_id: "${AWS_ACCESS_KEY_ID}"
      secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
      public_base_url: "https://cdn.example.com"
    r2:
      type: "s3"
      endpoint: "https://<account-id>.r2.cloudflarestorage.com"
      bucket: "media-assets"
      region: "auto"
      access_key_id: "${R2_ACCESS_KEY_ID}"
      secret_access_key: "${R2_SECRET_ACCESS_KEY}"
      public_base_url: "https://media.example.com"
    local:
      type: "local"
      directory: "./local_storage"
      public_base_url: "http://localhost:8080/v1/storage/local"

limits:
  max_file_size_bytes: 104857600
  presign_expiry_seconds: 3600
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
      fit: "fit"

webhooks:
  secret: "whsec_live_secret_key"
  retry_count: 3
  timeout_seconds: 5
  endpoints:
    - url: "https://api.example.com/webhooks/uploads"
      events:
        - "upload.completed"
        - "upload.failed"
```

---

## License

MIT License. Designed and developed by [Alexandr Motologa](https://mtlg.site).
