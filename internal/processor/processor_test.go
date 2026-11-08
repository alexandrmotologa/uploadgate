package processor

import (
	"bytes"
	"image/color"
	"testing"

	"github.com/alexandrmotologa/uploadgate/internal/config"
)

func TestProcessorMetadataAndThumbnails(t *testing.T) {
	// 1. Generate test 800x600 PNG
	imgBytes, err := CreateTestImage(800, 600, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	if err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	// 2. Extract metadata
	meta, err := ExtractMetadata(bytes.NewReader(imgBytes), "image/png")
	if err != nil {
		t.Fatalf("ExtractMetadata failed: %v", err)
	}
	if meta.Width != 800 {
		t.Errorf("expected width 800, got %d", meta.Width)
	}
	if meta.Height != 600 {
		t.Errorf("expected height 600, got %d", meta.Height)
	}
	if meta.SHA256 == "" {
		t.Errorf("expected non-empty SHA256")
	}

	// 3. Generate thumbnails
	variants := []config.ThumbnailVariant{
		{Name: "thumb", Width: 150, Height: 150, Fit: "crop"},
		{Name: "medium", Width: 400, Height: 400, Fit: "contain"},
	}

	thumbs, err := GenerateThumbnails(bytes.NewReader(imgBytes), "image/png", variants)
	if err != nil {
		t.Fatalf("GenerateThumbnails failed: %v", err)
	}

	if len(thumbs) < 2 {
		t.Fatalf("expected at least 2 thumbnails, got %d", len(thumbs))
	}

	for _, th := range thumbs {
		if th.VariantName == "thumb" {
			if th.Width != 150 || th.Height != 150 {
				t.Errorf("expected 150x150 for crop variant, got %dx%d", th.Width, th.Height)
			}
		}
		if len(th.Data) == 0 {
			t.Errorf("thumbnail %s has empty binary payload", th.VariantName)
		}
	}
}
