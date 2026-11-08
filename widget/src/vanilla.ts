// UploadGate Vanilla Web Component (<uploadgate-picker>)
// Size: < 3KB gzipped, zero dependencies.

class UploadGatePicker extends HTMLElement {
  private shadow: ShadowRoot;
  private input!: HTMLInputElement;
  private dropZone!: HTMLDivElement;
  private progressBar!: HTMLDivElement;
  private progressFill!: HTMLDivElement;
  private statusLabel!: HTMLDivElement;
  private previewImg!: HTMLImageElement;

  constructor() {
    super();
    this.shadow = this.attachShadow({ mode: "open" });
  }

  static get observedAttributes() {
    return ["endpoint", "api-key", "bucket", "accept", "max-bytes"];
  }

  connectedCallback() {
    this.render();
  }

  private render() {
    const style = document.createElement("style");
    style.textContent = `
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

    const container = document.createElement("div");
    container.className = "ug-zone";
    this.dropZone = container;

    const fileInput = document.createElement("input");
    fileInput.type = "file";
    fileInput.style.display = "none";
    fileInput.accept = this.getAttribute("accept") || "";
    this.input = fileInput;

    const preview = document.createElement("img");
    preview.className = "ug-preview";
    this.previewImg = preview;

    const title = document.createElement("div");
    title.className = "ug-title";
    title.textContent = "Upload direct to storage";

    const sub = document.createElement("div");
    sub.className = "ug-sub";
    sub.textContent = this.getAttribute("accept") ? `Accepted: ${this.getAttribute("accept")}` : "Click or drop file here";

    const progWrap = document.createElement("div");
    progWrap.className = "ug-progress-wrap";
    this.progressBar = progWrap;

    const progBar = document.createElement("div");
    progBar.className = "ug-progress-bar";
    this.progressFill = progBar;
    progWrap.appendChild(progBar);

    const status = document.createElement("div");
    status.className = "ug-status";
    this.statusLabel = status;

    container.appendChild(preview);
    container.appendChild(title);
    container.appendChild(sub);
    container.appendChild(progWrap);
    container.appendChild(status);
    container.appendChild(fileInput);

    this.shadow.appendChild(style);
    this.shadow.appendChild(container);

    this.attachEvents();
  }

  private attachEvents() {
    this.dropZone.addEventListener("click", () => this.input.click());
    this.dropZone.addEventListener("dragover", (e) => {
      e.preventDefault();
      this.dropZone.classList.add("dragover");
    });
    this.dropZone.addEventListener("dragleave", () => this.dropZone.classList.remove("dragover"));
    this.dropZone.addEventListener("drop", (e) => {
      e.preventDefault();
      this.dropZone.classList.remove("dragover");
      if (e.dataTransfer && e.dataTransfer.files.length > 0) {
        this.startUpload(e.dataTransfer.files[0]);
      }
    });

    this.input.addEventListener("change", () => {
      if (this.input.files && this.input.files.length > 0) {
        this.startUpload(this.input.files[0]);
      }
    });
  }

  private async startUpload(file: File) {
    const endpoint = (this.getAttribute("endpoint") || "").replace(/\/+$/, "");
    const apiKey = this.getAttribute("api-key") || "";
    const bucket = this.getAttribute("bucket") || "";

    if (!endpoint) {
      this.showStatus("Error: endpoint attribute is required", true);
      return;
    }

    if (file.type.startsWith("image/")) {
      this.previewImg.src = URL.createObjectURL(file);
      this.previewImg.style.display = "block";
    }

    this.progressBar.style.display = "block";
    this.progressFill.style.width = "0%";
    this.showStatus("Requesting presigned upload ticket...");

    this.dispatchEvent(new CustomEvent("upload-start", { detail: { file } }));

    try {
      const headers: Record<string, string> = { "Content-Type": "application/json" };
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

      const presignRes = await fetch(`${endpoint}/v1/uploads/presign`, {
        method: "POST",
        headers,
        body: JSON.stringify({
          filename: file.name,
          size_bytes: file.size,
          mime_type: file.type || "application/octet-stream",
          bucket,
        }),
      });

      if (!presignRes.ok) {
        const err = await presignRes.json().catch(() => ({}));
        throw new Error(err.error || `Presign failed (${presignRes.status})`);
      }

      const ticket = await presignRes.json();
      this.showStatus("Directly transferring bytes to storage...");

      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open(ticket.method || "PUT", ticket.presigned_url, true);
        if (ticket.headers) {
          for (const [k, v] of Object.entries(ticket.headers)) {
            xhr.setRequestHeader(k, String(v));
          }
        }
        xhr.upload.onprogress = (e) => {
          if (e.lengthComputable) {
            const pct = Math.round((e.loaded / e.total) * 100);
            this.progressFill.style.width = `${pct}%`;
            this.dispatchEvent(new CustomEvent("progress", { detail: { percent: pct, loaded: e.loaded, total: e.total } }));
          }
        };
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) resolve();
          else reject(new Error(`Storage transfer failed (${xhr.status})`));
        };
        xhr.onerror = () => reject(new Error("Network transfer failed"));
        xhr.send(file);
      });

      this.showStatus("Verifying magic bytes...");
      const completeRes = await fetch(`${endpoint}/v1/uploads/${ticket.upload_id}/complete`, {
        method: "POST",
        headers,
        body: JSON.stringify({}),
      });

      if (!completeRes.ok) {
        const err = await completeRes.json().catch(() => ({}));
        throw new Error(err.error || `Verification failed (${completeRes.status})`);
      }

      const verified = await completeRes.json();
      this.progressFill.style.width = "100%";
      this.showStatus("Verified successfully!", false);
      this.dispatchEvent(new CustomEvent("complete", { detail: verified }));
    } catch (err: unknown) {
      const e = err instanceof Error ? err : new Error(String(err));
      this.showStatus(e.message, true);
      this.dispatchEvent(new CustomEvent("error", { detail: e }));
    }
  }

  private showStatus(msg: string, isError = false) {
    this.statusLabel.textContent = msg;
    this.statusLabel.style.color = isError ? "#ef4444" : "#60a5fa";
  }
}

if (typeof customElements !== "undefined" && !customElements.get("uploadgate-picker")) {
  customElements.define("uploadgate-picker", UploadGatePicker);
}

export { UploadGatePicker };
