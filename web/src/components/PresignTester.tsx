import { useState, useRef, ChangeEvent, FormEvent } from "react";
import { UploadCloud, CheckCircle, AlertTriangle, ArrowRight, FileCheck, Terminal } from "lucide-react";

interface Props {
  apiKey: string;
  defaultBucket: string;
  onUploadSuccess: () => void;
}

export function PresignTester({ apiKey, defaultBucket, onUploadSuccess }: Props) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [bucket, setBucket] = useState(defaultBucket || "local");
  const [isProcessing, setIsProcessing] = useState(false);
  const [step, setStep] = useState<"idle" | "presigning" | "uploading" | "verifying" | "done" | "error">("idle");
  const [progressPct, setProgressPct] = useState(0);
  const [logs, setLogs] = useState<string[]>([]);
  const [resultJSON, setResultJSON] = useState<string>("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  const addLog = (msg: string) => {
    setLogs((prev) => [...prev, `[${new Date().toLocaleTimeString()}] ${msg}`]);
  };

  const onFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      setSelectedFile(e.target.files[0]);
      setStep("idle");
      setLogs([]);
      setResultJSON("");
    }
  };

  const handleTestUpload = async (e: FormEvent) => {
    e.preventDefault();
    if (!selectedFile) return;

    setIsProcessing(true);
    setLogs([]);
    setResultJSON("");
    setProgressPct(0);

    try {
      // Step 1: Presign
      setStep("presigning");
      addLog(`Requesting presigned ticket for ${selectedFile.name} (${selectedFile.size} bytes)...`);

      const headers: Record<string, string> = { "Content-Type": "application/json" };
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

      const presignRes = await fetch("/v1/uploads/presign", {
        method: "POST",
        headers,
        body: JSON.stringify({
          filename: selectedFile.name,
          size_bytes: selectedFile.size,
          mime_type: selectedFile.type || "application/octet-stream",
          bucket,
          metadata: { tester: "web_cockpit", user_agent: navigator.userAgent },
        }),
      });

      if (!presignRes.ok) {
        const err = await presignRes.json().catch(() => ({}));
        throw new Error(err.error || `Presign failed with HTTP ${presignRes.status}`);
      }

      const ticket = await presignRes.json();
      addLog(`Presigned URL generated: ${ticket.presigned_url.slice(0, 70)}...`);
      addLog(`Method: ${ticket.method}, Upload ID: ${ticket.upload_id}`);

      // Step 2: Direct storage transfer
      setStep("uploading");
      addLog("Streaming bytes directly to storage bucket...");

      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open(ticket.method || "PUT", ticket.presigned_url, true);

        if (ticket.headers) {
          for (const [k, v] of Object.entries(ticket.headers)) {
            xhr.setRequestHeader(k, String(v));
          }
        }

        xhr.upload.onprogress = (evt) => {
          if (evt.lengthComputable) {
            const pct = Math.round((evt.loaded / evt.total) * 100);
            setProgressPct(pct);
          }
        };

        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            resolve();
          } else {
            reject(new Error(`Storage transfer failed with status ${xhr.status}`));
          }
        };
        xhr.onerror = () => reject(new Error("Direct storage transfer network error"));
        xhr.send(selectedFile);
      });

      addLog("Direct bytes transfer complete. Notifying gateway to verify...");

      // Step 3: Complete verification
      setStep("verifying");
      const completeRes = await fetch(`/v1/uploads/${ticket.upload_id}/complete`, {
        method: "POST",
        headers,
        body: JSON.stringify({}),
      });

      const completeData = await completeRes.json();
      if (!completeRes.ok) {
        throw new Error(completeData.error || `Verification rejected with HTTP ${completeRes.status}`);
      }

      setStep("done");
      addLog(`Verification successful! Status: ${completeData.status}`);
      addLog(`SHA-256: ${completeData.sha256}`);
      if (completeData.thumbnails && completeData.thumbnails.length > 0) {
        addLog(`Generated ${completeData.thumbnails.length} responsive thumbnails`);
      }
      setResultJSON(JSON.stringify(completeData, null, 2));
      onUploadSuccess();
    } catch (err: unknown) {
      setStep("error");
      const errorMsg = err instanceof Error ? err.message : String(err);
      addLog(`ERROR: ${errorMsg}`);
    } finally {
      setIsProcessing(false);
    }
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm">
      <div className="flex items-center gap-2 mb-4">
        <UploadCloud className="w-5 h-5 text-blue-400" />
        <h2 className="text-base font-semibold text-slate-100">Interactive Presign Sandbox</h2>
      </div>

      <form onSubmit={handleTestUpload} className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          <div className="md:col-span-2">
            <label className="block text-xs font-medium text-slate-400 mb-1.5">Select File</label>
            <div
              onClick={() => fileInputRef.current?.click()}
              className="border border-dashed border-slate-700 hover:border-slate-600 rounded-lg p-3 text-center cursor-pointer bg-slate-950/40 hover:bg-slate-950 transition-colors"
            >
              <input
                ref={fileInputRef}
                type="file"
                className="hidden"
                onChange={onFileChange}
                disabled={isProcessing}
              />
              <span className="text-sm font-medium text-slate-300">
                {selectedFile ? selectedFile.name : "Choose an image, PDF, SVG, or document to test"}
              </span>
              {selectedFile && (
                <div className="text-xs text-slate-500 mt-0.5">
                  {(selectedFile.size / 1024).toFixed(1)} KB &bull; {selectedFile.type || "unknown mime"}
                </div>
              )}
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1.5">Target Bucket</label>
            <input
              type="text"
              value={bucket}
              onChange={(e) => setBucket(e.target.value)}
              disabled={isProcessing}
              placeholder="local, minio, s3..."
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-slate-200 focus:outline-none focus:border-blue-500"
            />
            <span className="text-[11px] text-slate-500 mt-1 block">Leave default for local testing</span>
          </div>
        </div>

        <div className="flex items-center justify-between pt-1">
          <div className="flex items-center gap-2">
            {step === "presigning" && (
              <span className="text-xs text-blue-400 flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-blue-400 animate-ping" />
                Generating presigned ticket...
              </span>
            )}
            {step === "uploading" && (
              <span className="text-xs text-indigo-400 flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-indigo-400 animate-pulse" />
                Transferring direct to storage ({progressPct}%)...
              </span>
            )}
            {step === "verifying" && (
              <span className="text-xs text-amber-400 flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-amber-400 animate-spin" />
                Verifying magic bytes & thumbnails...
              </span>
            )}
            {step === "done" && (
              <span className="text-xs text-emerald-400 flex items-center gap-1.5 font-medium">
                <CheckCircle className="w-4 h-4" />
                Direct upload verified!
              </span>
            )}
            {step === "error" && (
              <span className="text-xs text-rose-400 flex items-center gap-1.5 font-medium">
                <AlertTriangle className="w-4 h-4" />
                Upload rejected or failed
              </span>
            )}
          </div>

          <button
            type="submit"
            disabled={!selectedFile || isProcessing}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg shadow transition-colors"
          >
            Execute Direct Upload
            <ArrowRight className="w-4 h-4" />
          </button>
        </div>

        {isProcessing && (
          <div className="w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
            <div
              className="bg-blue-500 h-full transition-all duration-150"
              style={{ width: `${progressPct}%` }}
            />
          </div>
        )}
      </form>

      {logs.length > 0 && (
        <div className="mt-4 pt-3 border-t border-slate-800">
          <div className="flex items-center gap-1.5 text-xs text-slate-400 mb-2">
            <Terminal className="w-3.5 h-3.5" />
            <span>Execution Trace</span>
          </div>
          <div className="bg-slate-950 font-mono text-xs rounded-lg p-3 text-slate-300 space-y-1 max-h-40 overflow-y-auto">
            {logs.map((log, idx) => (
              <div key={idx} className={log.includes("ERROR") ? "text-rose-400" : ""}>
                {log}
              </div>
            ))}
          </div>
        </div>
      )}

      {resultJSON && (
        <div className="mt-3">
          <div className="flex items-center gap-1.5 text-xs text-emerald-400 mb-1.5">
            <FileCheck className="w-3.5 h-3.5" />
            <span>Ingestion Response</span>
          </div>
          <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs font-mono text-emerald-300 overflow-x-auto max-h-48">
            {resultJSON}
          </pre>
        </div>
      )}
    </div>
  );
}
