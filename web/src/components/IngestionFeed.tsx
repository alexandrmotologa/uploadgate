import { useState } from "react";
import { UploadAsset } from "./AssetPreviewModal";
import { Search, CheckCircle2, XCircle, Clock, FileText, Image as ImageIcon, Video, Archive, Eye } from "lucide-react";

interface Props {
  assets: UploadAsset[];
  onSelectAsset: (asset: UploadAsset) => void;
  statusFilter: string;
  onFilterChange: (status: string) => void;
  isLoading: boolean;
}

export function IngestionFeed({ assets, onSelectAsset, statusFilter, onFilterChange, isLoading }: Props) {
  const [searchQuery, setSearchQuery] = useState("");

  const filtered = assets.filter((item) => {
    if (!item.filename) return false;
    if (searchQuery.trim() === "") return true;
    const q = searchQuery.toLowerCase();
    return (
      item.filename.toLowerCase().includes(q) ||
      item.id.toLowerCase().includes(q) ||
      item.detected_mime.toLowerCase().includes(q)
    );
  });

  const getMIMEIcon = (mime: string) => {
    if (mime.startsWith("image/")) return <ImageIcon className="w-4 h-4 text-blue-400" />;
    if (mime.startsWith("video/")) return <Video className="w-4 h-4 text-purple-400" />;
    if (mime.includes("zip") || mime.includes("tar") || mime.includes("compressed"))
      return <Archive className="w-4 h-4 text-amber-400" />;
    return <FileText className="w-4 h-4 text-slate-400" />;
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden shadow-sm">
      {/* Feed Controls */}
      <div className="p-4 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          {["", "verified", "rejected", "pending"].map((s) => (
            <button
              key={s}
              onClick={() => onFilterChange(s)}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
                statusFilter === s
                  ? "bg-blue-600 text-white shadow-sm"
                  : "bg-slate-800/80 text-slate-400 hover:text-slate-200 hover:bg-slate-800"
              }`}
            >
              {s === "" ? "All Uploads" : s.charAt(0).toUpperCase() + s.slice(1)}
            </button>
          ))}
        </div>

        <div className="relative">
          <Search className="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500" />
          <input
            type="text"
            placeholder="Search by name, ID, MIME..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full sm:w-60 bg-slate-950 border border-slate-800 rounded-lg pl-8 pr-3 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>

      {/* Asset Table / List */}
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs text-slate-300">
          <thead className="bg-slate-950/70 border-b border-slate-800 text-[11px] uppercase tracking-wider text-slate-500 font-semibold">
            <tr>
              <th className="py-3 px-4">Asset</th>
              <th className="py-3 px-4">MIME Type</th>
              <th className="py-3 px-4">Size</th>
              <th className="py-3 px-4">Status</th>
              <th className="py-3 px-4">Time</th>
              <th className="py-3 px-4 text-right">Action</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60">
            {isLoading && assets.length === 0 ? (
              <tr>
                <td colSpan={6} className="py-12 text-center text-slate-500">
                  Loading ingestion feed...
                </td>
              </tr>
            ) : filtered.length === 0 ? (
              <tr>
                <td colSpan={6} className="py-12 text-center text-slate-500">
                  No upload records found. Use the Presign Sandbox to execute a test upload.
                </td>
              </tr>
            ) : (
              filtered.map((item) => (
                <tr
                  key={item.id}
                  onClick={() => onSelectAsset(item)}
                  className="hover:bg-slate-800/40 cursor-pointer transition-colors group"
                >
                  <td className="py-3 px-4">
                    <div className="flex items-center gap-2.5">
                      <div className="p-1.5 rounded-lg bg-slate-800 border border-slate-700/60 shrink-0">
                        {getMIMEIcon(item.detected_mime || item.declared_mime)}
                      </div>
                      <div className="truncate max-w-[200px] sm:max-w-[280px]">
                        <span className="font-medium text-slate-200 block truncate">{item.filename}</span>
                        <span className="text-[11px] font-mono text-slate-500">{item.id}</span>
                      </div>
                    </div>
                  </td>

                  <td className="py-3 px-4 font-mono text-slate-400">
                    {item.detected_mime || item.declared_mime}
                  </td>

                  <td className="py-3 px-4 font-mono text-slate-400">
                    {(item.original_size / 1024).toFixed(1)} KB
                  </td>

                  <td className="py-3 px-4">
                    {item.status === "verified" ? (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold uppercase bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                        <CheckCircle2 className="w-3 h-3" />
                        Verified
                      </span>
                    ) : item.status === "rejected" ? (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold uppercase bg-rose-500/10 text-rose-400 border border-rose-500/20">
                        <XCircle className="w-3 h-3" />
                        Rejected
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] font-semibold uppercase bg-amber-500/10 text-amber-400 border border-amber-500/20">
                        <Clock className="w-3 h-3" />
                        Pending
                      </span>
                    )}
                  </td>

                  <td className="py-3 px-4 text-slate-500 text-[11px]">
                    {new Date(item.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
                  </td>

                  <td className="py-3 px-4 text-right">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onSelectAsset(item);
                      }}
                      className="p-1 text-slate-400 hover:text-white rounded hover:bg-slate-700 transition-colors"
                      title="Inspect Asset"
                    >
                      <Eye className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
