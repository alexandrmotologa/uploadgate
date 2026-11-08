import React, { useState, useRef, DragEvent, ChangeEvent } from "react";

export interface FileUploadProps {
  endpoint: string;
  apiKey?: string;
  bucket?: string;
  accept?: string;
  maxSizeBytes?: number;
  metadata?: Record<string, unknown>;
  onUploadStart?: (file: File) => void;
  onProgress?: (percent: number, loaded: number, total: number) => void;
  onUploadComplete?: (asset: unknown) => void;
  onError?: (error: Error) => void;
  className?: string;
}

export function FileUpload({
  endpoint,
  apiKey,
  bucket,
  accept,
  maxSizeBytes,
  metadata,
  onUploadStart,
  onProgress,
  onUploadComplete,
  onError,
  className = "",
}: FileUploadProps) {
  const [isDragging, setIsDragging] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [previewUrl, setPreviewUrl] = useState<string | null>(null);
  const [statusText, setStatusText] = useState<string>("");
  const inputRef = useRef<HTMLInputElement>(null);

  const cleanEndpoint = endpoint.replace(/\/+$/, "");

  const handleFiles = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    const file = files[0];

    if (maxSizeBytes && file.size > maxSizeBytes) {
      const err = new Error(`File size exceeds ${(maxSizeBytes / 1024 / 1024).toFixed(1)}MB limit`);
      onError?.(err);
      setStatusText(err.message);
      return;
    }

    if (file.type.startsWith("image/")) {
      setPreviewUrl(URL.createObjectURL(file));
    } else {
      setPreviewUrl(null);
    }

    setIsUploading(true);
    setProgress(0);
    setStatusText("Preparing direct upload...");
    onUploadStart?.(file);

    try {
      // 1. Request presigned ticket
      const presignHeaders: Record<string, string> = {
        "Content-Type": "application/json",
      };
      if (apiKey) {
        presignHeaders["Authorization"] = `Bearer ${apiKey}`;
      }

      const presignRes = await fetch(`${cleanEndpoint}/v1/uploads/presign`, {
        method: "POST",
        headers: presignHeaders,
        body: JSON.stringify({
          filename: file.name,
          size_bytes: file.size,
          mime_type: file.type || "application/octet-stream",
          bucket: bucket || "",
          metadata: metadata || {},
        }),
      });

      if (!presignRes.ok) {
        const errorData = await presignRes.json().catch(() => ({}));
        throw new Error(errorData.error || `Presign failed with status ${presignRes.status}`);
      }

      const ticket = await presignRes.json();
      setStatusText("Streaming bytes directly to storage...");

      // 2. Direct binary transfer via XMLHttpRequest for accurate progress events
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open(ticket.method || "PUT", ticket.presigned_url, true);

        if (ticket.headers) {
          for (const [key, val] of Object.entries(ticket.headers)) {
            xhr.setRequestHeader(key, String(val));
          }
        }

        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const pct = Math.round((e.loaded / e.total) * 100);
            setProgress(pct);
            onProgress?.(pct, e.loaded, e.total);
          }
        };

        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            resolve();
          } else {
            reject(new Error(`Direct storage upload failed with HTTP ${xhr.status}`));
          }
        };

        xhr.onerror = () => reject(new Error("Network error during direct storage upload"));
        xhr.send(file);
      });

      // 3. Complete and trigger ingestion verification
      setStatusText("Verifying file integrity and generating variants...");
      const completeRes = await fetch(`${cleanEndpoint}/v1/uploads/${ticket.upload_id}/complete`, {
        method: "POST",
        headers: presignHeaders,
        body: JSON.stringify({}),
      });

      if (!completeRes.ok) {
        const errorData = await completeRes.json().catch(() => ({}));
        throw new Error(errorData.error || `Verification failed with status ${completeRes.status}`);
      }

      const verifiedAsset = await completeRes.json();
      setIsUploading(false);
      setProgress(100);
      setStatusText("Upload verified successfully!");
      onUploadComplete?.(verifiedAsset);
    } catch (err: unknown) {
      setIsUploading(false);
      const e = err instanceof Error ? err : new Error(String(err));
      setStatusText(e.message);
      onError?.(e);
    }
  };

  const onDragOver = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const onDragLeave = () => {
    setIsDragging(false);
  };

  const onDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);
    handleFiles(e.dataTransfer.files);
  };

  const onInputChange = (e: ChangeEvent<HTMLInputElement>) => {
    handleFiles(e.target.files);
  };

  return (
    <div
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
      onClick={() => !isUploading && inputRef.current?.click()}
      style={{
        border: `2px dashed ${isDragging ? "#3b82f6" : "#4b5563"}`,
        borderRadius: "12px",
        padding: "24px",
        textAlign: "center",
        cursor: isUploading ? "not-allowed" : "pointer",
        background: isDragging ? "rgba(59, 130, 246, 0.05)" : "transparent",
        transition: "all 0.2s ease",
      }}
      className={className}
    >
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        style={{ display: "none" }}
        onChange={onInputChange}
        disabled={isUploading}
      />

      {previewUrl && (
        <div style={{ marginBottom: "12px" }}>
          <img
            src={previewUrl}
            alt="Preview"
            style={{
              maxHeight: "120px",
              maxWidth: "100%",
              borderRadius: "8px",
              objectFit: "contain",
              margin: "0 auto",
            }}
          />
        </div>
      )}

      <div style={{ fontSize: "14px", fontWeight: 500, color: "#e5e7eb", marginBottom: "4px" }}>
        {isUploading ? "Uploading direct to storage..." : "Click or drag & drop file to upload"}
      </div>

      <div style={{ fontSize: "12px", color: "#9ca3af" }}>
        {accept ? `Accepted formats: ${accept}` : "Direct-to-storage presigned upload"}
      </div>

      {isUploading && (
        <div style={{ marginTop: "16px" }}>
          <div
            style={{
              width: "100%",
              height: "6px",
              background: "#374151",
              borderRadius: "3px",
              overflow: "hidden",
            }}
          >
            <div
              style={{
                width: `${progress}%`,
                height: "100%",
                background: "#3b82f6",
                transition: "width 0.2s ease",
              }}
            />
          </div>
          <div style={{ fontSize: "12px", color: "#60a5fa", marginTop: "6px" }}>
            {progress}% {statusText}
          </div>
        </div>
      )}

      {!isUploading && statusText && (
        <div
          style={{
            marginTop: "12px",
            fontSize: "12px",
            color: statusText.includes("failed") || statusText.includes("exceeds") ? "#ef4444" : "#10b981",
          }}
        >
          {statusText}
        </div>
      )}
    </div>
  );
}
