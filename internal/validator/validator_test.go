package validator

import (
	"bytes"
	"strings"
	"testing"
)

func TestMagicBytesDetection(t *testing.T) {
	// 1. Valid PNG
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	mime, _, err := SniffHeader(bytes.NewReader(pngHeader))
	if err != nil {
		t.Fatalf("SniffHeader failed: %v", err)
	}
	if mime != "image/png" {
		t.Errorf("expected image/png, got %s", mime)
	}

	valid, _ := ValidateMIMEParity("image/png", mime)
	if !valid {
		t.Errorf("expected parity to be valid for png")
	}

	// 2. Disguised executable (.exe claiming to be .png)
	exeHeader := []byte{0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00}
	exeMIME, _, err := SniffHeader(bytes.NewReader(exeHeader))
	if err != nil {
		t.Fatalf("SniffHeader failed: %v", err)
	}
	if exeMIME != "application/x-dosexec" {
		t.Errorf("expected application/x-dosexec, got %s", exeMIME)
	}

	valid, reason := ValidateMIMEParity("image/png", exeMIME)
	if valid {
		t.Errorf("expected disguised exe to be rejected")
	}
	if !strings.Contains(reason, "executable detected") {
		t.Errorf("expected security violation message, got: %s", reason)
	}

	// 3. WebP detection
	webpHeader := []byte("RIFFxxxxWEBPVP8 ")
	webpMIME, _, err := SniffHeader(bytes.NewReader(webpHeader))
	if err != nil {
		t.Fatalf("SniffHeader failed: %v", err)
	}
	if webpMIME != "image/webp" {
		t.Errorf("expected image/webp, got %s", webpMIME)
	}

	// 4. PDF detection
	pdfHeader := []byte("%PDF-1.7\n")
	pdfMIME, _, err := SniffHeader(bytes.NewReader(pdfHeader))
	if err != nil {
		t.Fatalf("SniffHeader failed: %v", err)
	}
	if pdfMIME != "application/pdf" {
		t.Errorf("expected application/pdf, got %s", pdfMIME)
	}
}

func TestSVGSanitizer(t *testing.T) {
	cleanSVG := `<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg"><circle cx="50" cy="50" r="40" fill="green" /></svg>`
	ok, reason := SanitizeSVG(strings.NewReader(cleanSVG))
	if !ok {
		t.Errorf("expected clean SVG to be accepted, failed: %s", reason)
	}

	maliciousSVG1 := `<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg"><script>alert('xss')</script></svg>`
	ok, _ = SanitizeSVG(strings.NewReader(maliciousSVG1))
	if ok {
		t.Errorf("expected SVG with <script> to be rejected")
	}

	maliciousSVG2 := `<svg width="100" height="100" onload="fetch('//evil.com')"><circle cx="50" cy="50" r="40" /></svg>`
	ok, _ = SanitizeSVG(strings.NewReader(maliciousSVG2))
	if ok {
		t.Errorf("expected SVG with onload= to be rejected")
	}
}
