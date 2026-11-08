import { useState } from "react";
import { Send, CheckCircle, AlertOctagon, RotateCw, Code2 } from "lucide-react";

export interface WebhookDeliveryItem {
  id: number;
  upload_id: string;
  event: string;
  endpoint: string;
  payload: string;
  status_code: number;
  attempt: number;
  response_body: string;
  error_message?: string;
  delivered_at: string;
  created_at: string;
}

interface Props {
  deliveries: WebhookDeliveryItem[];
  onRetry: (id: number) => void;
  isLoading: boolean;
}

export function WebhooksMonitor({ deliveries, onRetry, isLoading }: Props) {
  const [expandedId, setExpandedId] = useState<number | null>(null);

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
      <div className="p-4 border-b border-slate-800 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Send className="w-4 h-4 text-blue-400" />
          <h3 className="text-sm font-semibold text-slate-100">Outbound Webhooks Log</h3>
        </div>
        <span className="text-xs text-slate-500 font-mono">HMAC-SHA256 Signed</span>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs text-slate-300">
          <thead className="bg-slate-950/70 border-b border-slate-800 text-[11px] uppercase tracking-wider text-slate-500 font-semibold">
            <tr>
              <th className="py-2.5 px-4">Event</th>
              <th className="py-2.5 px-4">Upload ID</th>
              <th className="py-2.5 px-4">Endpoint</th>
              <th className="py-2.5 px-4">Status</th>
              <th className="py-2.5 px-4">Attempt</th>
              <th className="py-2.5 px-4">Timestamp</th>
              <th className="py-2.5 px-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60">
            {isLoading && deliveries.length === 0 ? (
              <tr>
                <td colSpan={7} className="py-8 text-center text-slate-500">
                  Loading webhook logs...
                </td>
              </tr>
            ) : deliveries.length === 0 ? (
              <tr>
                <td colSpan={7} className="py-8 text-center text-slate-500">
                  No webhook dispatches recorded yet. Configure webhooks in config.yaml.
                </td>
              </tr>
            ) : (
              deliveries.map((item) => {
                const isSuccess = item.status_code >= 200 && item.status_code < 300;
                return (
                  <tr key={item.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="py-2.5 px-4 font-mono font-medium text-slate-200">{item.event}</td>
                    <td className="py-2.5 px-4 font-mono text-slate-400">{item.upload_id}</td>
                    <td className="py-2.5 px-4 text-slate-400 truncate max-w-[200px]" title={item.endpoint}>
                      {item.endpoint}
                    </td>
                    <td className="py-2.5 px-4">
                      {isSuccess ? (
                        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                          <CheckCircle className="w-3 h-3" />
                          HTTP {item.status_code}
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">
                          <AlertOctagon className="w-3 h-3" />
                          {item.status_code > 0 ? `HTTP ${item.status_code}` : "Failed"}
                        </span>
                      )}
                    </td>
                    <td className="py-2.5 px-4 text-slate-400">#{item.attempt}</td>
                    <td className="py-2.5 px-4 text-slate-500 text-[11px]">
                      {new Date(item.created_at).toLocaleTimeString()}
                    </td>
                    <td className="py-2.5 px-4 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <button
                          onClick={() => setExpandedId(expandedId === item.id ? null : item.id)}
                          className="p-1 text-slate-400 hover:text-white rounded hover:bg-slate-700"
                          title="View Payload"
                        >
                          <Code2 className="w-3.5 h-3.5" />
                        </button>
                        {!isSuccess && (
                          <button
                            onClick={() => onRetry(item.id)}
                            className="p-1 text-blue-400 hover:text-blue-300 rounded hover:bg-slate-700"
                            title="Retry Webhook"
                          >
                            <RotateCw className="w-3.5 h-3.5" />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {expandedId && (
        <div className="p-4 border-t border-slate-800 bg-slate-950 font-mono text-xs">
          <span className="text-slate-400 font-semibold block mb-1">Payload:</span>
          <pre className="text-slate-300 overflow-x-auto max-h-40">
            {deliveries.find((d) => d.id === expandedId)?.payload}
          </pre>
        </div>
      )}
    </div>
  );
}
