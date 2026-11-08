package validator

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// MagicSignature defines a binary byte pattern and offset.
type MagicSignature struct {
	MIMEType    string
	Offset      int
	Magic       []byte
	Mask        []byte
	Description string
}

var knownSignatures = []MagicSignature{
	// Images
	{
		MIMEType:    "image/jpeg",
		Offset:      0,
		Magic:       []byte{0xFF, 0xD8, 0xFF},
		Description: "JPEG image",
	},
	{
		MIMEType:    "image/png",
		Offset:      0,
		Magic:       []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
		Description: "PNG image",
	},
	{
		MIMEType:    "image/gif",
		Offset:      0,
		Magic:       []byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61}, // GIF87a
		Description: "GIF87a image",
	},
	{
		MIMEType:    "image/gif",
		Offset:      0,
		Magic:       []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, // GIF89a
		Description: "GIF89a image",
	},
	{
		MIMEType:    "image/webp",
		Offset:      8,
		Magic:       []byte{0x57, 0x45, 0x42, 0x50}, // WEBP at offset 8 (RIFF....WEBP)
		Description: "WebP image",
	},
	// Documents
	{
		MIMEType:    "application/pdf",
		Offset:      0,
		Magic:       []byte{0x25, 0x50, 0x44, 0x46}, // %PDF
		Description: "PDF document",
	},
	// Archives
	{
		MIMEType:    "application/zip",
		Offset:      0,
		Magic:       []byte{0x50, 0x4B, 0x03, 0x04}, // PK..
		Description: "ZIP archive",
	},
	{
		MIMEType:    "application/gzip",
		Offset:      0,
		Magic:       []byte{0x1F, 0x8B},
		Description: "GZIP archive",
	},
	// Video & Media
	{
		MIMEType:    "video/webm",
		Offset:      0,
		Magic:       []byte{0x1A, 0x45, 0xDF, 0xA3}, // Matroska / WebM EBML
		Description: "WebM video",
	},
}

// SniffHeader reads up to 512 bytes from a stream and detects the MIME type.
func SniffHeader(r io.Reader) (string, []byte, error) {
	buf := make([]byte, 512)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", nil, fmt.Errorf("failed to read file header: %w", err)
	}
	header := buf[:n]

	// 1. Check known binary signatures
	for _, sig := range knownSignatures {
		if len(header) >= sig.Offset+len(sig.Magic) {
			if bytes.Equal(header[sig.Offset:sig.Offset+len(sig.Magic)], sig.Magic) {
				// Additional validation for RIFF WebP
				if sig.MIMEType == "image/webp" {
					if len(header) >= 4 && bytes.Equal(header[0:4], []byte("RIFF")) {
						return "image/webp", header, nil
					}
					continue
				}
				return sig.MIMEType, header, nil
			}
		}
	}

	// 2. Check MP4 / QuickTime ftyp signatures
	if len(header) >= 12 && bytes.Equal(header[4:8], []byte("ftyp")) {
		brand := string(header[8:12])
		if brand == "avif" || brand == "avis" {
			return "image/avif", header, nil
		}
		return "video/mp4", header, nil
	}

	// 3. Text-based detection: SVG, JSON, CSV
	trimmed := bytes.TrimSpace(header)
	if len(trimmed) > 0 {
		lower := bytes.ToLower(trimmed)
		if bytes.HasPrefix(lower, []byte("<?xml")) || bytes.HasPrefix(lower, []byte("<svg")) {
			if bytes.Contains(lower, []byte("<svg")) {
				return "image/svg+xml", header, nil
			}
		}
		if (trimmed[0] == '{' && bytes.Contains(trimmed, []byte("}"))) ||
			(trimmed[0] == '[' && bytes.Contains(trimmed, []byte("]"))) {
			return "application/json", header, nil
		}
	}

	// 4. Executable / dangerous binary check
	if len(header) >= 2 && bytes.Equal(header[0:2], []byte{0x4D, 0x5A}) { // MZ (DOS / Windows executable)
		return "application/x-dosexec", header, nil
	}
	if len(header) >= 4 && bytes.Equal(header[0:4], []byte{0x7F, 0x45, 0x4C, 0x46}) { // ELF (Linux binary)
		return "application/x-executable", header, nil
	}
	if len(header) >= 2 && bytes.Equal(header[0:2], []byte("#!")) { // Shell script
		return "application/x-shellscript", header, nil
	}

	// Fallback to text/plain if all bytes are valid UTF-8
	if len(header) > 0 && utf8.Valid(header) {
		return "text/plain", header, nil
	}

	return "application/octet-stream", header, nil
}

// ValidateMIMEParity checks if the declared MIME matches the detected binary MIME.
func ValidateMIMEParity(declaredMIME, detectedMIME string) (bool, string) {
	declared := strings.ToLower(strings.TrimSpace(declaredMIME))
	detected := strings.ToLower(strings.TrimSpace(detectedMIME))

	// If detected is dangerous executable, reject immediately regardless of declared
	if detected == "application/x-dosexec" || detected == "application/x-executable" || detected == "application/x-shellscript" {
		return false, fmt.Sprintf("security violation: binary executable detected (%s)", detected)
	}

	if declared == detected {
		return true, ""
	}

	// Permissive match for octet-stream when declared is specific but detected is valid
	if declared == "application/octet-stream" {
		return true, ""
	}

	// Match jpeg / jpg synonyms
	if (declared == "image/jpeg" || declared == "image/jpg") && (detected == "image/jpeg" || detected == "image/jpg") {
		return true, ""
	}

	// Plain text synonyms
	if (declared == "text/csv" || declared == "text/plain") && detected == "text/plain" {
		return true, ""
	}

	return false, fmt.Sprintf("declared MIME %q does not match detected binary signature %q", declared, detected)
}
