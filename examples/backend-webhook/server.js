// Example Node.js / Express Webhook Receiver for UploadGate
// Demonstrates HMAC-SHA256 signature verification

const express = require("express");
const crypto = require("crypto");

const app = express();
const WEBHOOK_SECRET = process.env.UPLOADGATE_WEBHOOK_SECRET || "whsec_change_me_to_random_secret";

// Important: Parse raw body for signature verification
app.post("/webhooks/uploads", express.raw({ type: "application/json" }), (req, res) => {
  const signatureHeader = req.headers["x-uploadgate-signature"];
  if (!signatureHeader) {
    return res.status(401).json({ error: "Missing X-UploadGate-Signature header" });
  }

  // Header format: t=1731089200,v1=9f83abc...
  const parts = signatureHeader.split(",");
  let timestamp = null;
  let signature = null;

  for (const part of parts) {
    const [k, v] = part.trim().split("=");
    if (k === "t") timestamp = v;
    if (k === "v1") signature = v;
  }

  if (!timestamp || !signature) {
    return res.status(401).json({ error: "Malformed signature header" });
  }

  // Prevent replay attacks (reject payloads older than 5 minutes)
  const now = Math.floor(Date.now() / 1000);
  if (Math.abs(now - parseInt(timestamp, 10)) > 300) {
    return res.status(401).json({ error: "Webhook timestamp expired" });
  }

  // Compute expected HMAC-SHA256 signature
  const signedPayload = `${timestamp}.${req.body.toString("utf8")}`;
  const hmac = crypto.createHmac("sha256", WEBHOOK_SECRET);
  hmac.update(signedPayload);
  const expectedSignature = hmac.digest("hex");

  if (!crypto.timingSafeEqual(Buffer.from(signature), Buffer.from(expectedSignature))) {
    return res.status(403).json({ error: "Invalid HMAC signature" });
  }

  // Signature valid: process event
  const event = JSON.parse(req.body.toString("utf8"));
  console.log(`[Webhook Verified] Event: ${event.event}, Upload ID: ${event.upload_id}`);
  console.log(`Storage URL: ${event.storage.public_url}`);
  console.log(`Dimensions: ${event.file.width}x${event.file.height}, SHA256: ${event.file.sha256}`);

  return res.status(200).json({ received: true });
});

app.listen(4000, () => {
  console.log("UploadGate webhook receiver listening on http://localhost:4000/webhooks/uploads");
});
