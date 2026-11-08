"use client";

import React, { useState } from "react";
// In your Next.js application, import from "@uploadgate/widget" or copy FileUpload component
import { FileUpload } from "./FileUpload";

interface UploadedAsset {
  id: string;
  filename: string;
  public_url: string;
  width: number;
  height: number;
  sha256: string;
  thumbnails?: Array<{
    variant_name: string;
    width: number;
    height: number;
    public_url: string;
  }>;
}

export default function UploadPage() {
  const [uploadedAsset, setUploadedAsset] = useState<UploadedAsset | null>(null);

  return (
    <div className="max-w-xl mx-auto p-8 space-y-6">
      <h1 className="text-2xl font-bold text-slate-100">Uploadgate Next.js Client Example</h1>
      <p className="text-sm text-slate-400">
        Direct-to-storage presigned upload with client-side magic-byte verification and WebP thumbnails.
      </p>

      <FileUpload
        endpoint="http://localhost:8080"
        apiKey="ug_secret_key_change_in_production"
        bucket="local"
        accept="image/png,image/jpeg,image/webp"
        maxSizeBytes={10 * 1024 * 1024} // 10MB
        metadata={{ user_id: "usr_42" }}
        onUploadStart={(file) => {
          console.log("Starting upload:", file.name);
        }}
        onProgress={(pct) => {
          console.log(`Upload progress: ${pct}%`);
        }}
        onUploadComplete={(asset) => {
          console.log("Upload completed and verified:", asset);
          setUploadedAsset(asset as UploadedAsset);
        }}
        onError={(err) => {
          console.error("Upload error:", err.message);
        }}
      />

      {uploadedAsset && (
        <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
          <h2 className="text-sm font-semibold text-emerald-400">Upload Verified!</h2>
          <div className="text-xs space-y-1 text-slate-300 font-mono">
            <div>Filename: {uploadedAsset.filename}</div>
            <div>ID: {uploadedAsset.id}</div>
            <div>Dimensions: {uploadedAsset.width} × {uploadedAsset.height} px</div>
            <div className="truncate">URL: {uploadedAsset.public_url}</div>
          </div>

          {uploadedAsset.thumbnails && uploadedAsset.thumbnails.length > 0 && (
            <div className="pt-2">
              <span className="text-xs text-slate-400 block mb-2 font-medium">Generated Responsive Thumbnails:</span>
              <div className="flex gap-3">
                {uploadedAsset.thumbnails.map((th, i) => (
                  <div key={i} className="text-center">
                    <img
                      src={th.public_url}
                      alt={th.variant_name}
                      className="w-20 h-20 object-cover rounded-lg border border-slate-700 mb-1"
                    />
                    <span className="text-[10px] text-slate-400 font-mono">
                      {th.variant_name} ({th.width}×{th.height})
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
