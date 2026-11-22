var H = Object.defineProperty;
var M = (n, d, t) => d in n ? H(n, d, { enumerable: !0, configurable: !0, writable: !0, value: t }) : n[d] = t;
var v = (n, d, t) => M(n, typeof d != "symbol" ? d + "" : d, t);
import { jsxs as D, jsx as x } from "react/jsx-runtime";
import { useState as $, useRef as W } from "react";
async function Z(n, d) {
  return !n.type.startsWith("image/") || n.type === "image/svg+xml" ? n : new Promise((t) => {
    const e = new Image(), p = URL.createObjectURL(n);
    e.onload = () => {
      URL.revokeObjectURL(p);
      const m = d.maxWidth || 2048, c = d.maxHeight || 2048, o = d.quality ?? 0.85, r = d.format || "image/webp";
      let s = e.width, i = e.height;
      if (s > m || i > c) {
        const a = Math.min(m / s, c / i);
        s = Math.round(s * a), i = Math.round(i * a);
      }
      const u = document.createElement("canvas");
      u.width = s, u.height = i;
      const y = u.getContext("2d");
      if (!y) {
        t(n);
        return;
      }
      y.drawImage(e, 0, 0, s, i), u.toBlob(
        (a) => {
          if (!a) {
            t(n);
            return;
          }
          const l = r === "image/webp" ? ".webp" : ".jpg", f = n.name.replace(/\.[^/.]+$/, "") + l, z = new File([a], f, { type: r });
          t(z);
        },
        r,
        o
      );
    }, e.onerror = () => t(n), e.src = p;
  });
}
function Y({
  endpoint: n,
  apiKey: d,
  bucket: t,
  accept: e,
  maxSizeBytes: p,
  compress: m,
  metadata: c,
  onUploadStart: o,
  onProgress: r,
  onUploadComplete: s,
  onError: i,
  className: u = ""
}) {
  const [y, a] = $(!1), [l, f] = $(!1), [z, O] = $(0), [U, N] = $(null), [C, k] = $(""), A = W(null), F = n.replace(/\/+$/, ""), I = async (h) => {
    if (!h || h.length === 0) return;
    let g = h[0];
    if (f(!0), O(0), m && g.type.startsWith("image/")) {
      k("Optimizing image client-side...");
      try {
        g = await Z(g, m);
      } catch {
      }
    }
    if (p && g.size > p) {
      f(!1);
      const w = new Error(`File size (${(g.size / 1024 / 1024).toFixed(1)}MB) exceeds limit`);
      i == null || i(w), k(w.message);
      return;
    }
    g.type.startsWith("image/") ? N(URL.createObjectURL(g)) : N(null), k("Preparing direct upload ticket..."), o == null || o(g);
    try {
      const w = {
        "Content-Type": "application/json"
      };
      d && (w.Authorization = `Bearer ${d}`);
      const E = await fetch(`${F}/v1/uploads/presign`, {
        method: "POST",
        headers: w,
        body: JSON.stringify({
          filename: g.name,
          size_bytes: g.size,
          mime_type: g.type || "application/octet-stream",
          bucket: t || "",
          metadata: c || {}
        })
      });
      if (!E.ok) {
        const R = await E.json().catch(() => ({}));
        throw new Error(R.error || `Presign failed with status ${E.status}`);
      }
      const L = await E.json();
      k("Streaming directly to storage..."), await new Promise((R, P) => {
        const b = new XMLHttpRequest();
        if (b.open(L.method || "PUT", L.presigned_url, !0), L.headers)
          for (const [S, j] of Object.entries(L.headers))
            b.setRequestHeader(S, String(j));
        b.upload.onprogress = (S) => {
          if (S.lengthComputable) {
            const j = Math.round(S.loaded / S.total * 100);
            O(j), r == null || r(j, S.loaded, S.total);
          }
        }, b.onload = () => {
          b.status >= 200 && b.status < 300 ? R() : P(new Error(`Storage upload failed with HTTP ${b.status}`));
        }, b.onerror = () => P(new Error("Network error during direct storage upload")), b.send(g);
      }), k("Verifying integrity & generating variants...");
      const T = await fetch(`${F}/v1/uploads/${L.upload_id}/complete`, {
        method: "POST",
        headers: w,
        body: JSON.stringify({})
      });
      if (!T.ok) {
        const R = await T.json().catch(() => ({}));
        throw new Error(R.error || `Verification failed with status ${T.status}`);
      }
      const B = await T.json();
      f(!1), O(100), k("Upload verified successfully!"), s == null || s(B);
    } catch (w) {
      f(!1);
      const E = w instanceof Error ? w : new Error(String(w));
      k(E.message), i == null || i(E);
    }
  };
  return /* @__PURE__ */ D(
    "div",
    {
      onDragOver: (h) => {
        h.preventDefault(), a(!0);
      },
      onDragLeave: () => {
        a(!1);
      },
      onDrop: (h) => {
        h.preventDefault(), a(!1), I(h.dataTransfer.files);
      },
      onClick: () => {
        var h;
        return !l && ((h = A.current) == null ? void 0 : h.click());
      },
      style: {
        border: `2px dashed ${y ? "#3b82f6" : "#4b5563"}`,
        borderRadius: "12px",
        padding: "24px",
        textAlign: "center",
        cursor: l ? "not-allowed" : "pointer",
        background: y ? "rgba(59, 130, 246, 0.05)" : "transparent",
        transition: "all 0.2s ease"
      },
      className: u,
      children: [
        /* @__PURE__ */ x(
          "input",
          {
            ref: A,
            type: "file",
            accept: e,
            style: { display: "none" },
            onChange: (h) => {
              I(h.target.files);
            },
            disabled: l
          }
        ),
        U && /* @__PURE__ */ x("div", { style: { marginBottom: "12px" }, children: /* @__PURE__ */ x(
          "img",
          {
            src: U,
            alt: "Preview",
            style: {
              maxHeight: "120px",
              maxWidth: "100%",
              borderRadius: "8px",
              objectFit: "contain",
              margin: "0 auto"
            }
          }
        ) }),
        /* @__PURE__ */ x("div", { style: { fontSize: "14px", fontWeight: 500, color: "#e5e7eb", marginBottom: "4px" }, children: l ? "Uploading direct to storage..." : "Click or drag & drop file to upload" }),
        /* @__PURE__ */ x("div", { style: { fontSize: "12px", color: "#9ca3af" }, children: e ? `Accepted formats: ${e}` : "Direct-to-storage presigned upload" }),
        l && /* @__PURE__ */ D("div", { style: { marginTop: "16px" }, children: [
          /* @__PURE__ */ x(
            "div",
            {
              style: {
                width: "100%",
                height: "6px",
                background: "#374151",
                borderRadius: "3px",
                overflow: "hidden"
              },
              children: /* @__PURE__ */ x(
                "div",
                {
                  style: {
                    width: `${z}%`,
                    height: "100%",
                    background: "#3b82f6",
                    transition: "width 0.2s ease"
                  }
                }
              )
            }
          ),
          /* @__PURE__ */ D("div", { style: { fontSize: "12px", color: "#60a5fa", marginTop: "6px" }, children: [
            z,
            "% ",
            C
          ] })
        ] }),
        !l && C && /* @__PURE__ */ x(
          "div",
          {
            style: {
              marginTop: "12px",
              fontSize: "12px",
              color: C.includes("failed") || C.includes("exceeds") ? "#ef4444" : "#10b981"
            },
            children: C
          }
        )
      ]
    }
  );
}
class q extends HTMLElement {
  constructor() {
    super();
    v(this, "shadow");
    v(this, "input");
    v(this, "dropZone");
    v(this, "progressBar");
    v(this, "progressFill");
    v(this, "statusLabel");
    v(this, "previewImg");
    this.shadow = this.attachShadow({ mode: "open" });
  }
  static get observedAttributes() {
    return ["endpoint", "api-key", "bucket", "accept", "max-bytes"];
  }
  connectedCallback() {
    this.render();
  }
  render() {
    const t = document.createElement("style");
    t.textContent = `
      :host {
        display: block;
        font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      }
      .ug-zone {
        border: 2px dashed #4b5563;
        border-radius: 12px;
        padding: 24px;
        text-align: center;
        cursor: pointer;
        background: #111827;
        color: #f3f4f6;
        transition: border-color 0.2s, background 0.2s;
      }
      .ug-zone:hover, .ug-zone.dragover {
        border-color: #3b82f6;
        background: #1f2937;
      }
      .ug-title {
        font-size: 14px;
        font-weight: 600;
        margin-bottom: 4px;
      }
      .ug-sub {
        font-size: 12px;
        color: #9ca3af;
      }
      .ug-progress-wrap {
        margin-top: 14px;
        background: #374151;
        border-radius: 9999px;
        height: 6px;
        overflow: hidden;
        display: none;
      }
      .ug-progress-bar {
        background: #3b82f6;
        height: 100%;
        width: 0%;
        transition: width 0.15s ease;
      }
      .ug-status {
        margin-top: 8px;
        font-size: 12px;
        color: #60a5fa;
      }
      .ug-preview {
        max-height: 100px;
        max-width: 100%;
        border-radius: 8px;
        margin: 0 auto 12px;
        display: none;
      }
    `;
    const e = document.createElement("div");
    e.className = "ug-zone", this.dropZone = e;
    const p = document.createElement("input");
    p.type = "file", p.style.display = "none", p.accept = this.getAttribute("accept") || "", this.input = p;
    const m = document.createElement("img");
    m.className = "ug-preview", this.previewImg = m;
    const c = document.createElement("div");
    c.className = "ug-title", c.textContent = "Upload direct to storage";
    const o = document.createElement("div");
    o.className = "ug-sub", o.textContent = this.getAttribute("accept") ? `Accepted: ${this.getAttribute("accept")}` : "Click or drop file here";
    const r = document.createElement("div");
    r.className = "ug-progress-wrap", this.progressBar = r;
    const s = document.createElement("div");
    s.className = "ug-progress-bar", this.progressFill = s, r.appendChild(s);
    const i = document.createElement("div");
    i.className = "ug-status", this.statusLabel = i, e.appendChild(m), e.appendChild(c), e.appendChild(o), e.appendChild(r), e.appendChild(i), e.appendChild(p), this.shadow.appendChild(t), this.shadow.appendChild(e), this.attachEvents();
  }
  attachEvents() {
    this.dropZone.addEventListener("click", () => this.input.click()), this.dropZone.addEventListener("dragover", (t) => {
      t.preventDefault(), this.dropZone.classList.add("dragover");
    }), this.dropZone.addEventListener("dragleave", () => this.dropZone.classList.remove("dragover")), this.dropZone.addEventListener("drop", (t) => {
      t.preventDefault(), this.dropZone.classList.remove("dragover"), t.dataTransfer && t.dataTransfer.files.length > 0 && this.startUpload(t.dataTransfer.files[0]);
    }), this.input.addEventListener("change", () => {
      this.input.files && this.input.files.length > 0 && this.startUpload(this.input.files[0]);
    });
  }
  async startUpload(t) {
    const e = (this.getAttribute("endpoint") || "").replace(/\/+$/, ""), p = this.getAttribute("api-key") || "", m = this.getAttribute("bucket") || "";
    if (!e) {
      this.showStatus("Error: endpoint attribute is required", !0);
      return;
    }
    t.type.startsWith("image/") && (this.previewImg.src = URL.createObjectURL(t), this.previewImg.style.display = "block"), this.progressBar.style.display = "block", this.progressFill.style.width = "0%", this.showStatus("Requesting presigned upload ticket..."), this.dispatchEvent(new CustomEvent("upload-start", { detail: { file: t } }));
    try {
      const c = { "Content-Type": "application/json" };
      p && (c.Authorization = `Bearer ${p}`);
      const o = await fetch(`${e}/v1/uploads/presign`, {
        method: "POST",
        headers: c,
        body: JSON.stringify({
          filename: t.name,
          size_bytes: t.size,
          mime_type: t.type || "application/octet-stream",
          bucket: m
        })
      });
      if (!o.ok) {
        const u = await o.json().catch(() => ({}));
        throw new Error(u.error || `Presign failed (${o.status})`);
      }
      const r = await o.json();
      this.showStatus("Directly transferring bytes to storage..."), await new Promise((u, y) => {
        const a = new XMLHttpRequest();
        if (a.open(r.method || "PUT", r.presigned_url, !0), r.headers)
          for (const [l, f] of Object.entries(r.headers))
            a.setRequestHeader(l, String(f));
        a.upload.onprogress = (l) => {
          if (l.lengthComputable) {
            const f = Math.round(l.loaded / l.total * 100);
            this.progressFill.style.width = `${f}%`, this.dispatchEvent(new CustomEvent("progress", { detail: { percent: f, loaded: l.loaded, total: l.total } }));
          }
        }, a.onload = () => {
          a.status >= 200 && a.status < 300 ? u() : y(new Error(`Storage transfer failed (${a.status})`));
        }, a.onerror = () => y(new Error("Network transfer failed")), a.send(t);
      }), this.showStatus("Verifying magic bytes...");
      const s = await fetch(`${e}/v1/uploads/${r.upload_id}/complete`, {
        method: "POST",
        headers: c,
        body: JSON.stringify({})
      });
      if (!s.ok) {
        const u = await s.json().catch(() => ({}));
        throw new Error(u.error || `Verification failed (${s.status})`);
      }
      const i = await s.json();
      this.progressFill.style.width = "100%", this.showStatus("Verified successfully!", !1), this.dispatchEvent(new CustomEvent("complete", { detail: i }));
    } catch (c) {
      const o = c instanceof Error ? c : new Error(String(c));
      this.showStatus(o.message, !0), this.dispatchEvent(new CustomEvent("error", { detail: o }));
    }
  }
  showStatus(t, e = !1) {
    this.statusLabel.textContent = t, this.statusLabel.style.color = e ? "#ef4444" : "#60a5fa";
  }
}
typeof customElements < "u" && !customElements.get("uploadgate-picker") && customElements.define("uploadgate-picker", q);
export {
  Y as FileUpload,
  q as UploadGatePicker
};
