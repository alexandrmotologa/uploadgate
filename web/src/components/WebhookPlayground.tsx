import { useState, useMemo } from "react";
import { Shield, Copy, Check } from "lucide-react";

export function WebhookPlayground() {
  const [secret, setSecret] = useState("whsec_sample_secret_key_1234567890");
  const [timestamp, setTimestamp] = useState<number>(() => Math.floor(Date.now() / 1000));
  const [payloadText, setPayloadText] = useState(
    JSON.stringify(
      {
        event: "upload.completed",
        upload_id: "upl_01JBCDEF89012345",
        status: "verified",
        storage: {
          provider: "s3",
          key: "uploads/2026/11/08/avatar.png",
          public_url: "https://cdn.example.com/uploads/2026/11/08/avatar.png",
        },
      },
      null,
      2
    )
  );

  const [activeLang, setActiveLang] = useState<"node" | "go" | "python" | "php">("node");
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // Client-side HMAC calculation using Web Crypto API
  const [calculatedSig, setCalculatedSig] = useState<string>("");

  useMemo(() => {
    async function computeHMAC() {
      try {
        const enc = new TextEncoder();
        const keyData = enc.encode(secret);
        const cryptoKey = await window.crypto.subtle.importKey(
          "raw",
          keyData,
          { name: "HMAC", hash: "SHA-256" },
          false,
          ["sign"]
        );

        const dataToSign = enc.encode(`${timestamp}.${payloadText}`);
        const signatureBuf = await window.crypto.subtle.sign("HMAC", cryptoKey, dataToSign);
        const hashArray = Array.from(new Uint8Array(signatureBuf));
        const hashHex = hashArray.map((b) => b.toString(16).padStart(2, "0")).join("");
        setCalculatedSig(hashHex);
      } catch {
        setCalculatedSig("error");
      }
    }
    computeHMAC();
  }, [secret, timestamp, payloadText]);

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(label);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const codeSnippets = {
    node: `// Node.js (Express) Signature Verification
const crypto = require("crypto");

function verifyUploadGateSignature(rawBody, signatureHeader, secret) {
  const parts = signatureHeader.split(",");
  let t = null, v1 = null;
  for (const part of parts) {
    const [k, v] = part.trim().split("=");
    if (k === "t") t = v;
    if (k === "v1") v1 = v;
  }
  if (!t || !v1) return false;

  // Prevent replay attacks (5 minute window)
  if (Math.abs(Date.now() / 1000 - parseInt(t, 10)) > 300) return false;

  const expected = crypto
    .createHmac("sha256", secret)
    .update(\`\${t}.\${rawBody}\`)
    .digest("hex");

  return crypto.timingSafeEqual(Buffer.from(v1), Buffer.from(expected));
}`,
    go: `// Go HMAC-SHA256 Verification
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "strings"
)

func VerifySignature(secret, header string, payload []byte) bool {
    var t, v1 string
    for _, part := range strings.Split(header, ",") {
        kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
        if len(kv) == 2 {
            if kv[0] == "t" { t = kv[1] }
            if kv[0] == "v1" { v1 = kv[1] }
        }
    }
    if t == "" || v1 == "" { return false }

    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(fmt.Sprintf("%s.%s", t, string(payload))))
    expected := hex.EncodeToString(mac.Sum(nil))

    return hmac.Equal([]byte(expected), []byte(v1))
}`,
    python: `# Python (FastAPI / Flask) Verification
import hmac, hashlib, time

def verify_uploadgate_signature(secret: str, header: str, payload_bytes: bytes) -> bool:
    parts = dict(p.strip().split("=", 1) for p in header.split(",") if "=" in p)
    t, v1 = parts.get("t"), parts.get("v1")
    if not t or not v1:
        return False

    if abs(time.time() - int(t)) > 300:
        return False

    signed_data = f"{t}.".encode("utf-8") + payload_bytes
    expected = hmac.new(secret.encode("utf-8"), signed_data, hashlib.sha256).hexdigest()
    return hmac.compare_digest(expected, v1)`,
    php: `<?php
// PHP Webhook Signature Verification
function verifyUploadGateSignature(string $rawBody, string $header, string $secret): bool {
    $parts = explode(",", $header);
    $parsed = [];
    foreach ($parts as $p) {
        $kv = explode("=", trim($p), 2);
        if (count($kv) === 2) $parsed[$kv[0]] = $kv[1];
    }
    if (!isset($parsed['t']) || !isset($parsed['v1'])) return false;

    if (abs(time() - (int)$parsed['t']) > 300) return false;

    $signed = $parsed['t'] . "." . $rawBody;
    $expected = hash_hmac('sha256', $signed, $secret);
    return hash_equals($expected, $parsed['v1']);
}`,
  };

  return (
    <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-sm space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Shield className="w-5 h-5 text-indigo-400" />
          <h3 className="text-base font-semibold text-slate-100">Webhook Signature & Developer Console</h3>
        </div>
        <span className="text-xs text-slate-400">HMAC-SHA256 Signature Verification</span>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Left: Interactive Generator */}
        <div className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1">Webhook Secret</label>
            <input
              type="text"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs font-mono text-slate-200 focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="text-xs font-medium text-slate-400">Timestamp (Unix)</label>
              <button
                onClick={() => setTimestamp(Math.floor(Date.now() / 1000))}
                className="text-[11px] text-blue-400 hover:text-blue-300"
              >
                Set to Now
              </button>
            </div>
            <input
              type="number"
              value={timestamp}
              onChange={(e) => setTimestamp(parseInt(e.target.value, 10) || 0)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-1.5 text-xs font-mono text-slate-200 focus:outline-none focus:border-indigo-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1">Payload JSON</label>
            <textarea
              rows={6}
              value={payloadText}
              onChange={(e) => setPayloadText(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2.5 text-xs font-mono text-slate-300 focus:outline-none focus:border-indigo-500 resize-none"
            />
          </div>

          {/* Computed Header Output */}
          <div className="bg-slate-950 border border-slate-800 rounded-lg p-3">
            <div className="flex items-center justify-between text-xs text-slate-400 mb-1">
              <span className="font-semibold text-slate-300">Generated Header:</span>
              <button
                onClick={() =>
                  copyToClipboard(`t=${timestamp},v1=${calculatedSig}`, "header")
                }
                className="flex items-center gap-1 text-[11px] text-indigo-400 hover:text-indigo-300"
              >
                {copiedKey === "header" ? <Check className="w-3 h-3 text-emerald-400" /> : <Copy className="w-3 h-3" />}
                Copy Header
              </button>
            </div>
            <code className="block text-[11px] text-indigo-300 break-all font-mono">
              X-UploadGate-Signature: t={timestamp},v1={calculatedSig}
            </code>
          </div>
        </div>

        {/* Right: Code Generator */}
        <div className="flex flex-col space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-slate-400">Backend Verification Snippets:</span>
            <div className="flex gap-1">
              {(["node", "go", "python", "php"] as const).map((lang) => (
                <button
                  key={lang}
                  onClick={() => setActiveLang(lang)}
                  className={`px-2 py-1 rounded text-[11px] font-mono font-medium transition-colors ${
                    activeLang === lang
                      ? "bg-indigo-600 text-white"
                      : "bg-slate-800 text-slate-400 hover:text-slate-200"
                  }`}
                >
                  {lang.toUpperCase()}
                </button>
              ))}
            </div>
          </div>

          <div className="relative flex-1 bg-slate-950 border border-slate-800 rounded-lg p-3 overflow-hidden">
            <button
              onClick={() => copyToClipboard(codeSnippets[activeLang], "code")}
              className="absolute right-3 top-3 p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded text-xs flex items-center gap-1"
            >
              {copiedKey === "code" ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              <span>Copy</span>
            </button>
            <pre className="text-xs font-mono text-slate-300 overflow-x-auto h-[260px]">
              {codeSnippets[activeLang]}
            </pre>
          </div>
        </div>
      </div>
    </div>
  );
}
