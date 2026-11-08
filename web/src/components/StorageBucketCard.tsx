import { Database, HardDrive, CheckCircle2, AlertCircle, RefreshCw } from "lucide-react";

export interface BucketInfo {
  name: string;
  type: string;
  public_url: string;
  healthy: boolean;
  error?: string;
}

interface Props {
  buckets: BucketInfo[];
  defaultProvider: string;
  onRefresh: () => void;
  isLoading: boolean;
}

export function StorageBucketCard({ buckets, defaultProvider, onRefresh, isLoading }: Props) {
  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Database className="w-5 h-5 text-blue-400" />
          <h2 className="text-base font-semibold text-slate-100">Storage Connectors</h2>
        </div>
        <button
          onClick={onRefresh}
          disabled={isLoading}
          className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 rounded-lg transition-colors border border-slate-700"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${isLoading ? "animate-spin" : ""}`} />
          Refresh
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        {buckets.map((b) => {
          const isDefault = b.name === defaultProvider;
          return (
            <div
              key={b.name}
              className={`p-3.5 rounded-lg border transition-all ${
                b.healthy
                  ? "bg-slate-950/60 border-slate-800 hover:border-slate-700"
                  : "bg-red-950/20 border-red-900/50"
              }`}
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-2">
                  <HardDrive className="w-4 h-4 text-slate-400" />
                  <span className="font-medium text-sm text-slate-200">{b.name}</span>
                </div>
                <div className="flex items-center gap-1.5">
                  {isDefault && (
                    <span className="text-[10px] font-semibold uppercase tracking-wider bg-blue-500/10 text-blue-400 px-1.5 py-0.5 rounded border border-blue-500/20">
                      Default
                    </span>
                  )}
                  <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">
                    {b.type}
                  </span>
                </div>
              </div>

              <div className="mt-3 flex items-center justify-between text-xs">
                <div className="flex items-center gap-1.5">
                  {b.healthy ? (
                    <>
                      <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                      <span className="text-emerald-400 font-medium">Ready</span>
                    </>
                  ) : (
                    <>
                      <AlertCircle className="w-3.5 h-3.5 text-rose-400" />
                      <span className="text-rose-400 font-medium truncate max-w-[150px]" title={b.error}>
                        {b.error || "Offline"}
                      </span>
                    </>
                  )}
                </div>
                <span className="text-slate-500 text-[11px] truncate max-w-[140px]" title={b.public_url}>
                  {b.public_url.replace(/^https?:\/\//, "")}
                </span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
