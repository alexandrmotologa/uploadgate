package processor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/alexandrmotologa/uploadgate/internal/config"
)

// GeneratedThumbnail contains the binary thumbnail payload and metadata.
type GeneratedThumbnail struct {
	VariantName string
	Format      string
	MIMEType    string
	Width       int
	Height      int
	Data        []byte
}

// GenerateThumbnails derives responsive image variants from an input image stream.
func GenerateThumbnails(src io.Reader, mimeType string, variants []config.ThumbnailVariant) ([]GeneratedThumbnail, error) {
	if !strings.HasPrefix(mimeType, "image/") || mimeType == "image/svg+xml" {
		return nil, nil // Not a raster image
	}

	srcImg, _, err := image.Decode(src)
	if err != nil {
		return nil, fmt.Errorf("failed to decode source image for thumbnailing: %w", err)
	}

	srcBounds := srcImg.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	if srcW == 0 || srcH == 0 {
		return nil, fmt.Errorf("invalid image dimensions %dx%d", srcW, srcH)
	}

	var results []GeneratedThumbnail

	for _, v := range variants {
		targetW := v.Width
		targetH := v.Height
		if targetW <= 0 {
			targetW = 150
		}
		if targetH <= 0 {
			targetH = 150
		}

		var dst *image.RGBA

		if v.Fit == "crop" {
			// Center crop to exact aspect ratio then scale
			srcAspect := float64(srcW) / float64(srcH)
			dstAspect := float64(targetW) / float64(targetH)

			var cropRect image.Rectangle
			if srcAspect > dstAspect {
				// Source is wider than target: crop sides
				newW := int(math.Round(float64(srcH) * dstAspect))
				offsetX := (srcW - newW) / 2
				cropRect = image.Rect(srcBounds.Min.X+offsetX, srcBounds.Min.Y, srcBounds.Min.X+offsetX+newW, srcBounds.Max.Y)
			} else {
				// Source is taller than target: crop top/bottom
				newH := int(math.Round(float64(srcW) / dstAspect))
				offsetY := (srcH - newH) / 2
				cropRect = image.Rect(srcBounds.Min.X, srcBounds.Min.Y+offsetY, srcBounds.Max.X, srcBounds.Min.Y+offsetY+newH)
			}

			dst = image.NewRGBA(image.Rect(0, 0, targetW, targetH))
			draw.BiLinear.Scale(dst, dst.Bounds(), srcImg, cropRect, draw.Over, nil)
		} else {
			// Fit: preserve aspect ratio within bounding box
			scaleW := float64(targetW) / float64(srcW)
			scaleH := float64(targetH) / float64(srcH)
			scale := math.Min(scaleW, scaleH)
			if scale > 1.0 {
				scale = 1.0 // Do not upscale small images
			}

			actualW := int(math.Round(float64(srcW) * scale))
			actualH := int(math.Round(float64(srcH) * scale))
			if actualW <= 0 {
				actualW = 1
			}
			if actualH <= 0 {
				actualH = 1
			}

			dst = image.NewRGBA(image.Rect(0, 0, actualW, actualH))
			draw.BiLinear.Scale(dst, dst.Bounds(), srcImg, srcBounds, draw.Over, nil)
			targetW = actualW
			targetH = actualH
		}

		// Encode to JPEG variant
		var jpegBuf bytes.Buffer
		if err := jpeg.Encode(&jpegBuf, dst, &jpeg.Options{Quality: 85}); err == nil {
			results = append(results, GeneratedThumbnail{
				VariantName: v.Name,
				Format:      "jpeg",
				MIMEType:    "image/jpeg",
				Width:       targetW,
				Height:      targetH,
				Data:        jpegBuf.Bytes(),
			})
		}

		// Encode to PNG variant if source is PNG/transparent
		if mimeType == "image/png" || mimeType == "image/gif" {
			var pngBuf bytes.Buffer
			if err := png.Encode(&pngBuf, dst); err == nil {
				results = append(results, GeneratedThumbnail{
					VariantName: v.Name,
					Format:      "png",
					MIMEType:    "image/png",
					Width:       targetW,
					Height:      targetH,
					Data:        pngBuf.Bytes(),
				})
			}
		}
	}

	return results, nil
}

// CreateTestImage generates a simple solid color PNG image for testing.
func CreateTestImage(width, height int, col color.Color) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, col)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
