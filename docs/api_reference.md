# API Reference

All requests to the UploadGate API require an `Authorization: Bearer <API_KEY>` header unless authentication is disabled for specific public routes.

---

## Endpoints Summary

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/v1/uploads/presign` | Generate a single presigned PUT upload URL |
| `POST` | `/v1/uploads/presign-batch` | Generate batch presigned URLs for up to 100 files |
| `POST` | `/v1/uploads/presign-multipart` | Initiate S3 multipart upload and get part URLs |
| `POST` | `/v1/uploads/from-url` | Ingest asset directly from remote URL with SSRF protection |
| `POST` | `/v1/uploads/:id/complete` | Complete upload and run security and thumbnail pipeline |
| `GET` | `/v1/uploads/:id` | Fetch upload record status, metadata, and thumbnails |
| `GET` | `/v1/uploads` | List recent uploads with pagination and status filters |
| `GET` | `/v1/events` | Server-Sent Events (SSE) real-time stream |
| `GET` | `/v1/buckets` | List configured storage buckets and connection health |
| `GET` | `/v1/webhooks` | List outbound webhook delivery logs |
| `POST` | `/v1/webhooks/:id/retry` | Manually re-dispatch a failed webhook event |
| `GET` | `/healthz` | Liveness and readiness health probe |
| `GET` | `/metrics` | Prometheus metrics endpoint |

---

## 1. Presign Upload

### Request
`POST /v1/uploads/presign`

```json
{
  "filename": "document.pdf",
  "size_bytes": 1048576,
  "mime_type": "application/pdf",
  "bucket": "default",
  "metadata": {
    "account_id": "acc_102"
  }
}
```

### Response (`200 OK`)
```json
{
  "upload_id": "upl_01JBCDEF89012345",
  "presigned_url": "https://bucket.s3.amazonaws.com/uploads/2026/11/08/document.pdf?X-Amz-Signature=...",
  "method": "PUT",
  "headers": {
    "Content-Type": "application/pdf"
  },
  "storage_key": "uploads/2026/11/08/document.pdf",
  "expires_at": "2026-11-08T22:00:00Z"
}
```

---

## 2. Complete Upload

### Request
`POST /v1/uploads/:id/complete`

```json
{}
```

### Response (`200 OK`)
```json
{
  "upload_id": "upl_01JBCDEF89012345",
  "status": "verified",
  "sha256": "4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a",
  "detected_mime": "application/pdf",
  "size_bytes": 1048576,
  "storage_key": "uploads/2026/11/08/document.pdf",
  "public_url": "https://cdn.example.com/uploads/2026/11/08/document.pdf"
}
```

---

## 3. Real-Time Stream (SSE)

`GET /v1/events`

Clients receive Server-Sent Events formatted as JSON:

```
event: upload.verified
data: {"upload_id":"upl_01JBCDEF89012345","filename":"document.pdf","status":"verified","detected_mime":"application/pdf","size_bytes":1048576}
```
