# Storage Providers Configuration

UploadGate provides native multi-cloud adapters designed to work with any S3-compatible object store or local disk storage.

---

## AWS S3

Configure standard Amazon Web Services S3 buckets:

```yaml
storage:
  default_provider: "aws"
  providers:
    aws:
      type: "s3"
      endpoint: "https://s3.us-east-1.amazonaws.com"
      bucket: "my-production-bucket"
      region: "us-east-1"
      access_key_id: "AKIA..."
      secret_access_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
      public_base_url: "https://cdn.example.com"
      use_path_style: false
```

### Bucket CORS Policy
To allow web browsers to upload directly to your S3 bucket, configure CORS on the bucket:

```json
[
  {
    "AllowedHeaders": ["*"],
    "AllowedMethods": ["PUT", "POST", "GET", "HEAD"],
    "AllowedOrigins": ["*"],
    "ExposeHeaders": ["ETag", "x-amz-meta-upload-id"]
  }
]
```

---

## Cloudflare R2

Cloudflare R2 provides zero-egress-fee object storage:

```yaml
storage:
  default_provider: "r2"
  providers:
    r2:
      type: "s3"
      endpoint: "https://<account_id>.r2.cloudflarestorage.com"
      bucket: "assets"
      region: "auto"
      access_key_id: "r2_access_key_id"
      secret_access_key: "r2_secret_access_key"
      public_base_url: "https://pub-<id>.r2.dev"
      use_path_style: true
```

---

## MinIO (Self-Hosted Object Storage)

MinIO is ideal for local development, on-premise deployments, or air-gapped systems:

```yaml
storage:
  default_provider: "minio"
  providers:
    minio:
      type: "s3"
      endpoint: "http://localhost:9000"
      bucket: "uploadgate"
      region: "us-east-1"
      access_key_id: "minioadmin"
      secret_access_key: "minioadmin"
      public_base_url: "http://localhost:9000/uploadgate"
      use_path_style: true
```

---

## Local Filesystem Storage

For environments where cloud object storage is not required, UploadGate provides a local storage provider:

```yaml
storage:
  default_provider: "local"
  providers:
    local:
      type: "local"
      directory: "./local_storage"
      public_base_url: "http://localhost:8080/v1/storage/local"
```
Uploads in local mode use signed, one-time upload tokens (`PUT /v1/storage/local/:token`), writing bytes directly to the host filesystem.
