# Architecture Specification

## Overview

UploadGate is an asset ingestion gateway and direct-to-storage presigning daemon written in Go with an embedded React 19 cockpit. It is engineered to replace third-party SaaS services (such as Uploadcare, Filestack, and Transloadit) by operating entirely inside self-hosted or private cloud infrastructure while consuming less than 40MB resident RAM.

Traditional upload architectures require routing heavy multipart file streams through application servers. This causes high memory consumption, socket starvation, and request timeouts. UploadGate avoids this bottleneck by issuing secure presigned URLs that allow frontend clients to stream bytes directly into object storage buckets.

---

## Data Flow Pipeline

```
1. Client Handshake
   Client ──(POST /v1/uploads/presign)──► UploadGate Daemon
   UploadGate checks MIME allowlist, size limits, and bucket policies.
   UploadGate returns presigned URL + upload ticket ID.

2. Direct Byte Transfer
   Client ──(HTTP PUT / Multipart)──────► Object Storage (S3 / R2 / MinIO / Local)
   Bytes flow directly between the client device and the storage provider.
   UploadGate and backend servers receive zero payload traffic.

3. Completion & Ingestion Verification
   Client ──(POST /v1/uploads/:id/complete)──► UploadGate Daemon
   UploadGate fetches object headers and binary chunks:
   - Evaluates magic bytes against declared MIME type.
   - Parses SVG files to detect and block stored XSS vectors.
   - Computes SHA-256 cryptographic checksum and verifies ETag.
   - Extracts image dimensions and strips sensitive EXIF metadata.
   - Generates responsive WebP and JPEG thumbnail variants.

4. Event Notification
   UploadGate dispatches HMAC-SHA256 signed webhook to backend API.
   UploadGate publishes real-time ingestion event to Server-Sent Events (SSE) hub.
```

---

## Core Security Model

### 1. Magic-Byte Verification
File extensions can be easily spoofed. An attacker could rename a Windows executable (`payload.exe`) or shell script (`script.sh`) to `image.png`.
UploadGate reads the binary file header directly from the storage target and matches it against canonical binary signatures (magic bytes):
- JPEG: `FF D8 FF`
- PNG: `89 50 4E 47 0D 0A 1A 0A`
- GIF: `47 49 46 38 37 61` or `47 49 46 38 39 61`
- WebP: `52 49 46 46 .... 57 45 42 50`
- PDF: `25 50 44 46`
- MP4 / QuickTime: `ftyp` box indicator at byte offset 4

If a mismatch is detected, the upload status is set to `rejected` with reason `mime_mismatch`, thumbnails are aborted, and a security alert webhook is dispatched.

### 2. SVG Sanitization
Scalable Vector Graphics (SVG) files are XML documents that can embed `<script>` tags, base64 data URIs, or malicious event attributes such as `onload` or `onerror`.
UploadGate runs an XML tokenizer over SVG files to reject or sanitize active code vectors before assets are marked verified.

### 3. Outbound Webhook Signatures
Webhooks are signed using HMAC-SHA256:
```
Header: X-UploadGate-Signature: t=1731089200,v1=9f83...
Payload: JSON string
```
Backend services verify the signature using the shared webhook secret to guarantee authenticity and prevent replay attacks.
