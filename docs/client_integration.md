# Client Integration Guide

UploadGate provides multiple ways to integrate direct-to-storage uploads into your web and mobile applications:
1. Native React 19 component (`<FileUpload />`)
2. Vanilla Web Component (`<uploadgate-picker>`)
3. Plain JavaScript Fetch / XHR directly against the REST API

---

## 1. React 19 Component

Install the client package or import the component directly:

```tsx
import React, { useState } from "react";
import { FileUpload } from "./FileUpload";

export function ProfileForm() {
  const [avatarUrl, setAvatarUrl] = useState("");

  return (
    <div className="space-y-4">
      <label className="block text-sm font-medium">Profile Photo</label>
      <FileUpload
        endpoint="https://upload.example.com"
        apiKey="ug_public_client_key"
        bucket="avatars"
        accept="image/png,image/jpeg,image/webp"
        maxSizeBytes={5 * 1024 * 1024}
        onUploadComplete={(asset) => {
          setAvatarUrl(asset.storage.public_url);
        }}
        onError={(err) => {
          alert(`Upload failed: ${err.message}`);
        }}
      />
      {avatarUrl && <img src={avatarUrl} alt="Avatar" className="w-24 h-24 rounded-full" />}
    </div>
  );
}
```

---

## 2. Vanilla Web Component

Include the standalone 3KB widget in your HTML:

```html
<!DOCTYPE html>
<html>
<head>
  <script src="https://upload.example.com/widget/uploadgate-picker.min.js" async></script>
</head>
<body>
  <h3>Upload Document</h3>
  <uploadgate-picker
    id="doc-picker"
    endpoint="https://upload.example.com"
    api-key="ug_public_key"
    accept=".pdf,.png,.jpg"
  ></uploadgate-picker>

  <script>
    const picker = document.getElementById("doc-picker");
    picker.addEventListener("complete", (event) => {
      console.log("File uploaded successfully:", event.detail);
    });
    picker.addEventListener("error", (event) => {
      console.error("Upload error:", event.detail);
    });
  </script>
</body>
</html>
```

---

## 3. Direct JavaScript / TypeScript Implementation

If you prefer not using UI widgets, implement the direct-to-storage protocol in plain JavaScript:

```ts
async function uploadFileDirectly(file: File) {
  // Step 1: Request presigned ticket
  const presignRes = await fetch("https://upload.example.com/v1/uploads/presign", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer YOUR_CLIENT_KEY",
    },
    body: JSON.stringify({
      filename: file.name,
      size_bytes: file.size,
      mime_type: file.type || "application/octet-stream",
    }),
  });

  const { upload_id, presigned_url, headers } = await presignRes.json();

  // Step 2: Upload bytes directly to object storage
  await fetch(presigned_url, {
    method: "PUT",
    headers: {
      "Content-Type": file.type || "application/octet-stream",
      ...headers,
    },
    body: file,
  });

  // Step 3: Trigger completion and verification
  const completeRes = await fetch(`https://upload.example.com/v1/uploads/${upload_id}/complete`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": "Bearer YOUR_CLIENT_KEY",
    },
    body: JSON.stringify({}),
  });

  const result = await completeRes.json();
  return result;
}
```
