package processor

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	_ "golang.org/x/image/webp"
)

// AssetMetadata contains extracted physical properties of a file.
type AssetMetadata struct {
	Width           int
	Height          int
	DurationSeconds float64
	SHA256          string
}

// ExtractMetadata calculates the SHA-256 hash and extracts image/video dimensions.
func ExtractMetadata(r io.Reader, mimeType string) (*AssetMetadata, error) {
	hasher := sha256.New()
	var buf bytes.Buffer
	tee := io.TeeReader(r, io.MultiWriter(hasher, &buf))

	// Read up to first 2MB for config inspection, rest just through hasher
	headerBytes := make([]byte, 2*1024*1024)
	n, _ := io.ReadFull(tee, headerBytes)
	initialData := headerBytes[:n]

	// Finish draining the remaining stream through the hasher
	_, err := io.Copy(hasher, tee)
	if err != nil && err != io.EOF {
		return nil, err
	}

	meta := &AssetMetadata{
		SHA256: hex.EncodeToString(hasher.Sum(nil)),
	}

	// If it is an image, inspect dimensions
	if strings.HasPrefix(mimeType, "image/") && mimeType != "image/svg+xml" {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(initialData))
		if err == nil {
			meta.Width = cfg.Width
			meta.Height = cfg.Height
		}
	} else if mimeType == "video/mp4" || mimeType == "video/quicktime" {
		// Attempt parsing MP4 mvhd atom for duration
		dur := parseMP4Duration(initialData)
		meta.DurationSeconds = dur
	}

	return meta, nil
}

// parseMP4Duration extracts duration in seconds from the MP4 mvhd atom if present in initial chunk.
func parseMP4Duration(data []byte) float64 {
	mvhdIdx := bytes.Index(data, []byte("mvhd"))
	if mvhdIdx == -1 || len(data) < mvhdIdx+28 {
		return 0.0
	}

	payload := data[mvhdIdx+4:]
	version := payload[0]

	if version == 0 && len(payload) >= 20 {
		timeScale := binary.BigEndian.Uint32(payload[12:16])
		duration := binary.BigEndian.Uint32(payload[16:20])
		if timeScale > 0 {
			return float64(duration) / float64(timeScale)
		}
	} else if version == 1 && len(payload) >= 32 {
		timeScale := binary.BigEndian.Uint32(payload[20:24])
		duration := binary.BigEndian.Uint64(payload[24:32])
		if timeScale > 0 {
			return float64(duration) / float64(timeScale)
		}
	}

	return 0.0
}
