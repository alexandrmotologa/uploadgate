import { useState } from "react";
import { X, Copy, Check, ExternalLink, Image as ImageIcon, ShieldCheck, FileText } from "lucide-react";

export interface ThumbnailItem {
  id: number;
  variant_name: string;
  format: string;
  width: number;
  height: number;
  storage_key: string;
  public_url: string;
  size_bytes: number;
}

export interface UploadAsset {
  id: string;
  filename: string;
  original_size: number;
  declared_mime: string;
  detected_mime: string;
  storage_provider: string;
  storage_bucket: string;
  storage_key: string;
  public_url: string;
  status: "pending" | "uploaded" | "verified" | "rejected";
  sha256: string;
  width: number;
  height: number;
  duration_seconds: number;
  error_reason?: string;
  metadata_json?: string;
  created_at: string;
  thumbnails?: ThumbnailItem[];
}

interface Props {
  asset: UploadAsset | null;
  onClose: () => void;
}

export function AssetPreviewModal({ asset, onClose }: Props) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  if (!asset) return null;

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(label);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const isImage = asset.detected_mime.startsWith("image/") && asset.detected_mime !== "image/svg+xml";

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-2xl max-h-[90vh] overflow-hidden flex flex-col shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800">
          <div className="flex items-center gap-2.5">
            <div className="p-2 rounded-lg bg-blue-500/10 text-blue-400">
              <FileText className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-semibold text-slate-100 text-sm truncate max-w-md">{asset.filename}</h3>
              <p className="text-xs text-slate-400 font-mono">{asset.id}</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto space-y-6">
          {/* Visual Preview */}
          {isImage && asset.public_url && (
            <div className="bg-slate-950 rounded-xl p-4 border border-slate-800 flex items-center justify-center">
              <img
                src={asset.public_url}
                alt={asset.filename}
                className="max-h-60 max-w-full rounded-lg object-contain shadow"
              />
            </div>
          )}

          {/* Core Properties Grid */}
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3 text-xs">
            <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
              <span className="text-slate-500 block mb-1">Status</span>
              <span
                className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-semibold uppercase ${
                  asset.status === "verified"
                    ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/20"
                    : asset.status === "rejected"
                    ? "bg-rose-500/10 text-rose-400 border border-rose-500/20"
                    : "bg-amber-500/10 text-amber-400 border border-amber-500/20"
                }`}
              >
                {asset.status}
              </span>
            </div>

            <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
              <span className="text-slate-500 block mb-1">Size</span>
              <span className="text-slate-200 font-mono">{(asset.original_size / 1024).toFixed(1)} KB</span>
            </div>

            <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
              <span className="text-slate-500 block mb-1">Dimensions</span>
              <span className="text-slate-200 font-mono">
                {asset.width > 0 ? `${asset.width} × ${asset.height} px` : "—"}
              </span>
            </div>

            <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
              <span className="text-slate-500 block mb-1">Detected MIME</span>
              <span className="text-slate-200 font-mono truncate block" title={asset.detected_mime}>
                {asset.detected_mime || "—"}
              </span>
            </div>

            <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
              <span className="text-slate-500 block mb-1">Declared MIME</span>
              <span className="text-slate-200 font-mono truncate block" title={asset.declared_mime}>
                {asset.declared_mime}
              </span>
            </div>

            <div className="bg-slate-950 p-3 rounded-lg border border-slate-800">
              <span className="text-slate-500 block mb-1">Storage Provider</span>
              <span className="text-slate-200 font-mono uppercase">{asset.storage_provider}</span>
            </div>
          </div>

          {/* SHA-256 Checksum */}
          {asset.sha256 && (
            <div className="bg-slate-950 p-3 rounded-lg border border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-2 overflow-hidden">
                <ShieldCheck className="w-4 h-4 text-emerald-400 shrink-0" />
                <span className="text-slate-400 text-xs shrink-0 font-medium">SHA-256:</span>
                <span className="text-slate-300 font-mono text-xs truncate" title={asset.sha256}>
                  {asset.sha256}
                </span>
              </div>
              <button
                onClick={() => copyToClipboard(asset.sha256, "sha256")}
                className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-slate-800 shrink-0 ml-2"
                title="Copy SHA-256"
              >
                {copiedKey === "sha256" ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              </button>
            </div>
          )}

          {/* Rejection alert */}
          {asset.status === "rejected" && asset.error_reason && (
            <div className="bg-rose-950/30 border border-rose-900/50 rounded-lg p-3 text-xs text-rose-300">
              <span className="font-semibold block mb-0.5">Rejection Reason:</span>
              {asset.error_reason}
            </div>
          )}

          {/* Thumbnails Section */}
          {asset.thumbnails && asset.thumbnails.length > 0 && (
            <div>
              <div className="flex items-center gap-1.5 text-xs font-semibold text-slate-300 mb-2">
                <ImageIcon className="w-3.5 h-3.5 text-blue-400" />
                <span>Responsive Thumbnails ({asset.thumbnails.length})</span>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                {asset.thumbnails.map((th) => (
                  <div
                    key={th.id}
                    className="bg-slate-950 p-2.5 rounded-lg border border-slate-800 flex items-center justify-between text-xs"
                  >
                    <div>
                      <span className="font-medium text-slate-200">
                        {th.variant_name} ({th.width}×{th.height})
                      </span>
                      <span className="text-slate-500 font-mono text-[11px] block">
                        {(th.size_bytes / 1024).toFixed(1)} KB &bull; {th.format.toUpperCase()}
                      </span>
                    </div>
                    <div className="flex items-center gap-1">
                      <button
                        onClick={() => copyToClipboard(th.public_url, `th-${th.id}`)}
                        className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-slate-800"
                        title="Copy URL"
                      >
                        {copiedKey === `th-${th.id}` ? (
                          <Check className="w-3.5 h-3.5 text-emerald-400" />
                        ) : (
                          <Copy className="w-3.5 h-3.5" />
                        )}
                      </button>
                      <a
                        href={th.public_url}
                        target="_blank"
                        rel="noreferrer"
                        className="p-1.5 text-slate-400 hover:text-white rounded hover:bg-slate-800"
                        title="Open image"
                      >
                        <ExternalLink className="w-3.5 h-3.5" />
                      </a>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-slate-800 bg-slate-950/50 flex items-center justify-between text-xs text-slate-500">
          <span>Created at {new Date(asset.created_at).toLocaleString()}</span>
          {asset.public_url && (
            <a
              href={asset.public_url}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-1 text-blue-400 hover:text-blue-300 font-medium"
            >
              Open Direct Storage URL
              <ExternalLink className="w-3 h-3" />
            </a>
          )}
        </div>
      </div>
    </div>
  );
}
