package validator

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

var dangerousElements = map[string]bool{
	"script":        true,
	"foreignobject": true,
	"applet":        true,
	"object":        true,
	"embed":         true,
	"iframe":        true,
	"frame":         true,
	"frameset":      true,
	"meta":          true,
	"link":          true,
}

// SanitizeSVG inspects an SVG XML stream for stored XSS and malicious scripts.
func SanitizeSVG(r io.Reader) (bool, string) {
	data, err := io.ReadAll(io.LimitReader(r, 10*1024*1024)) // 10MB limit for SVG
	if err != nil {
		return false, fmt.Sprintf("failed to read SVG: %v", err)
	}

	// Quick byte scan for obvious script payloads
	lowerData := bytes.ToLower(data)
	if bytes.Contains(lowerData, []byte("<script")) ||
		bytes.Contains(lowerData, []byte("javascript:")) ||
		bytes.Contains(lowerData, []byte("vbscript:")) ||
		bytes.Contains(lowerData, []byte("onload=")) ||
		bytes.Contains(lowerData, []byte("onerror=")) ||
		bytes.Contains(lowerData, []byte("onclick=")) {
		return false, "svg rejected: contains forbidden script tags or event handlers"
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose
	decoder.Entity = xml.HTMLEntity

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Malformed XML may be an obfuscation technique
			return false, fmt.Sprintf("invalid SVG XML structure: %v", err)
		}

		switch elem := token.(type) {
		case xml.StartElement:
			elemName := strings.ToLower(elem.Name.Local)
			if dangerousElements[elemName] {
				return false, fmt.Sprintf("svg rejected: forbidden element <%s>", elemName)
			}

			for _, attr := range elem.Attr {
				attrName := strings.ToLower(attr.Name.Local)
				attrVal := strings.ToLower(strings.TrimSpace(attr.Value))

				// Block inline event handlers (on*)
				if strings.HasPrefix(attrName, "on") {
					return false, fmt.Sprintf("svg rejected: forbidden event handler attribute %s", attrName)
				}

				// Block dangerous URI schemes
				if attrName == "href" || strings.HasSuffix(attrName, ":href") || attrName == "src" {
					if strings.HasPrefix(attrVal, "javascript:") ||
						strings.HasPrefix(attrVal, "vbscript:") ||
						strings.HasPrefix(attrVal, "data:text/html") {
						return false, fmt.Sprintf("svg rejected: malicious URI scheme in %s", attrName)
					}
				}
			}
		}
	}

	return true, ""
}
