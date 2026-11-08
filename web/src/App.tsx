import { useState, useEffect, useCallback } from "react";
import { UploadCloud, Activity, Key, BarChart3, ShieldCheck, CheckCircle2, XCircle } from "lucide-react";
import { StorageBucketCard, BucketInfo } from "./components/StorageBucketCard";
import { PresignTester } from "./components/PresignTester";
import { IngestionFeed } from "./components/IngestionFeed";
import { AssetPreviewModal, UploadAsset } from "./components/AssetPreviewModal";
import { WebhooksMonitor, WebhookDeliveryItem } from "./components/WebhooksMonitor";
import { WebhookPlayground } from "./components/WebhookPlayground";

export function App() {
  const [apiKey, setApiKey] = useState<string>(() => localStorage.getItem("ug_api_key") || "");
  const [showKeyModal, setShowKeyModal] = useState(false);
  const [tempKey, setTempKey] = useState("");

  const [buckets, setBuckets] = useState<BucketInfo[]>([]);
  const [defaultProvider, setDefaultProvider] = useState("local");
  const [assets, setAssets] = useState<UploadAsset[]>([]);
  const [statusFilter, setStatusFilter] = useState("");
  const [selectedAsset, setSelectedAsset] = useState<UploadAsset | null>(null);
  const [webhooks, setWebhooks] = useState<WebhookDeliveryItem[]>([]);
  const [sseConnected, setSseConnected] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  const fetchBuckets = useCallback(async () => {
    try {
      const headers: Record<string, string> = {};
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      const res = await fetch("/v1/buckets", { headers });
      if (res.ok) {
        const data = await res.json();
        setBuckets(data.buckets || []);
        setDefaultProvider(data.default_provider || "local");
      }
    } catch {
      // Ignore network errors in dev
    }
  }, [apiKey]);

  const fetchAssets = useCallback(async () => {
    try {
      setIsLoading(true);
      const headers: Record<string, string> = {};
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      const url = statusFilter ? `/v1/uploads?status=${statusFilter}&limit=100` : "/v1/uploads?limit=100";
      const res = await fetch(url, { headers });
      if (res.ok) {
        const data = await res.json();
        setAssets(data.items || []);
      }
    } finally {
      setIsLoading(false);
    }
  }, [apiKey, statusFilter]);

  const fetchWebhooks = useCallback(async () => {
    try {
      const headers: Record<string, string> = {};
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      const res = await fetch("/v1/webhooks?limit=25", { headers });
      if (res.ok) {
        const data = await res.json();
        setWebhooks(data.items || []);
      }
    } catch {
      // Ignore in dev
    }
  }, [apiKey]);

  // Initial load
  useEffect(() => {
    fetchBuckets();
    fetchAssets();
    fetchWebhooks();
  }, [fetchBuckets, fetchAssets, fetchWebhooks]);

  // Server-Sent Events (SSE) listener
  useEffect(() => {
    const sse = new EventSource("/v1/events");

    sse.onopen = () => {
      setSseConnected(true);
    };

    sse.onerror = () => {
      setSseConnected(false);
    };

    sse.addEventListener("upload.created", (e) => {
      try {
        const created: UploadAsset = JSON.parse(e.data);
        setAssets((prev) => [created, ...prev.filter((a) => a.id !== created.id)]);
      } catch {
        // Ignore parse error
      }
    });

    sse.addEventListener("upload.verified", (e) => {
      try {
        const verified: UploadAsset = JSON.parse(e.data);
        setAssets((prev) => [verified, ...prev.filter((a) => a.id !== verified.id)]);
        fetchWebhooks();
      } catch {
        // Ignore parse error
      }
    });

    sse.addEventListener("upload.rejected", (e) => {
      try {
        const rejected: UploadAsset = JSON.parse(e.data);
        setAssets((prev) => [rejected, ...prev.filter((a) => a.id !== rejected.id)]);
        fetchWebhooks();
      } catch {
        // Ignore parse error
      }
    });

    return () => {
      sse.close();
    };
  }, [fetchWebhooks]);

  const handleSaveKey = () => {
    setApiKey(tempKey);
    localStorage.setItem("ug_api_key", tempKey);
    setShowKeyModal(false);
  };

  const handleRetryWebhook = async (id: number) => {
    try {
      const headers: Record<string, string> = {};
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;
      await fetch(`/v1/webhooks/${id}/retry`, { method: "POST", headers });
      setTimeout(fetchWebhooks, 600);
    } catch {
      // Ignore
    }
  };

  // Quick stats calculation
  const verifiedCount = assets.filter((a) => a.status === "verified").length;
  const rejectedCount = assets.filter((a) => a.status === "rejected").length;
  const totalBytes = assets.reduce((acc, curr) => acc + (curr.original_size || 0), 0);

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans">
      {/* Top Navbar */}
      <header className="border-b border-slate-800/80 bg-slate-900/60 backdrop-blur sticky top-0 z-40">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-14 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-1.5 rounded-lg bg-blue-600 text-white shadow-sm shadow-blue-500/20">
              <UploadCloud className="w-5 h-5" />
            </div>
            <div>
              <span className="font-bold text-sm tracking-tight text-white">UploadGate</span>
              <span className="ml-2 text-[10px] font-mono uppercase bg-slate-800 text-slate-400 px-1.5 py-0.5 rounded border border-slate-700">
                Daemon
              </span>
            </div>
          </div>

          <div className="flex items-center gap-1.5 sm:gap-3">
            {/* SSE Live Status */}
            <div className="flex items-center gap-1.5 px-2 sm:px-2.5 py-1 rounded-full bg-slate-800/60 border border-slate-700/60 text-xs">
              <span
                className={`w-2 h-2 rounded-full shrink-0 ${
                  sseConnected ? "bg-emerald-400 animate-pulse" : "bg-amber-400"
                }`}
              />
              <span className="text-slate-400 text-[11px] font-medium hidden sm:inline">
                {sseConnected ? "Live SSE Stream" : "Reconnecting..."}
              </span>
              <span className="text-slate-400 text-[11px] font-medium sm:hidden">
                {sseConnected ? "Live" : "Off"}
              </span>
            </div>

            <a
              href="/metrics"
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-1 text-xs text-slate-400 hover:text-white px-2 sm:px-2.5 py-1 rounded-lg hover:bg-slate-800 transition-colors"
            >
              <BarChart3 className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Metrics</span>
            </a>

            <button
              onClick={() => {
                setTempKey(apiKey);
                setShowKeyModal(true);
              }}
              className="flex items-center gap-1.5 px-2.5 sm:px-3 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg border border-slate-700 transition-colors"
            >
              <Key className="w-3.5 h-3.5 text-blue-400 shrink-0" />
              <span className="hidden sm:inline">{apiKey ? "API Key Configured" : "Set API Key"}</span>
              <span className="sm:hidden">{apiKey ? "Key" : "Auth"}</span>
            </button>
          </div>
        </div>
      </header>

      {/* Main Container */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6 flex-1 w-full">
        {/* KPI Ribbon */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
            <div className="flex items-center justify-between text-slate-400 text-xs mb-1">
              <span>Total Uploads</span>
              <Activity className="w-4 h-4 text-blue-400" />
            </div>
            <div className="text-xl font-bold text-white font-mono">{assets.length}</div>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
            <div className="flex items-center justify-between text-slate-400 text-xs mb-1">
              <span>Verified & Processed</span>
              <CheckCircle2 className="w-4 h-4 text-emerald-400" />
            </div>
            <div className="text-xl font-bold text-emerald-400 font-mono">{verifiedCount}</div>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
            <div className="flex items-center justify-between text-slate-400 text-xs mb-1">
              <span>Security Blocked</span>
              <XCircle className="w-4 h-4 text-rose-400" />
            </div>
            <div className="text-xl font-bold text-rose-400 font-mono">{rejectedCount}</div>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-4 shadow-sm">
            <div className="flex items-center justify-between text-slate-400 text-xs mb-1">
              <span>Processed Volume</span>
              <ShieldCheck className="w-4 h-4 text-purple-400" />
            </div>
            <div className="text-xl font-bold text-slate-200 font-mono">
              {(totalBytes / 1024 / 1024).toFixed(1)} MB
            </div>
          </div>
        </div>

        {/* Storage Connectors & Presign Sandbox */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <StorageBucketCard
            buckets={buckets}
            defaultProvider={defaultProvider}
            onRefresh={fetchBuckets}
            isLoading={isLoading}
          />
          <PresignTester
            apiKey={apiKey}
            defaultBucket={defaultProvider}
            onUploadSuccess={() => {
              fetchAssets();
              fetchWebhooks();
            }}
          />
        </div>

        {/* Live Ingestion Feed */}
        <IngestionFeed
          assets={assets}
          onSelectAsset={setSelectedAsset}
          statusFilter={statusFilter}
          onFilterChange={setStatusFilter}
          isLoading={isLoading}
        />

        {/* Webhook Signature Playground & Developer Console */}
        <WebhookPlayground />

        {/* Outbound Webhooks Log */}
        <WebhooksMonitor
          deliveries={webhooks}
          onRetry={handleRetryWebhook}
          isLoading={isLoading}
        />
      </main>

      {/* Asset Preview Modal */}
      <AssetPreviewModal
        asset={selectedAsset}
        onClose={() => setSelectedAsset(null)}
      />

      {/* API Key Modal */}
      {showKeyModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 w-full max-w-md shadow-2xl">
            <h3 className="text-base font-semibold text-slate-100 mb-2">Set Cockpit API Key</h3>
            <p className="text-xs text-slate-400 mb-4">
              Enter the API key configured in <code className="bg-slate-950 px-1 py-0.5 rounded">config.yaml</code> to authorize presigning and management calls from this cockpit.
            </p>
            <input
              type="password"
              placeholder="ug_secret_key_..."
              value={tempKey}
              onChange={(e) => setTempKey(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500 mb-4"
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setShowKeyModal(false)}
                className="px-3 py-1.5 text-xs text-slate-400 hover:text-slate-200 rounded-lg hover:bg-slate-800"
              >
                Cancel
              </button>
              <button
                onClick={handleSaveKey}
                className="px-4 py-1.5 text-xs bg-blue-600 hover:bg-blue-500 text-white font-medium rounded-lg shadow"
              >
                Save Key
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Footer */}
      <footer className="border-t border-slate-800/80 py-4 text-center text-xs text-slate-500">
        UploadGate &bull; Direct-to-Storage Presigned Upload Gateway & Asset Ingestion Daemon &bull; MIT License
      </footer>
    </div>
  );
}
