// Package filenames generates unique, filesystem-safe filenames from job
// title + timestamp (R3).
package filenames

import (
	"bytes"
	"regexp"
	"strings"
	"time"
)

var unsafe = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// Generate builds "<timestamp>_<safe-title><ext>". ext without a leading dot
// is normalized automatically; an empty safeTitle falls back to "print".
func Generate(jobTitle string, ext string, timestamp time.Time) string {
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	ts := timestamp.Format("20060102-150405")
	safeTitle := strings.Trim(unsafe.ReplaceAllString(strings.TrimSpace(jobTitle), "_"), "_")
	if safeTitle == "" {
		safeTitle = "print"
	}
	return ts + "_" + safeTitle + ext
}

// DetectExtension guesses the file extension from the first bytes (magic
// numbers). For callers without a reliable file extension - e.g. the CUPS
// backend, whose job title is arbitrary text rather than a filename, and
// whose payload, despite "PDF backend", isn't necessarily a PDF (see R1:
// "lp -d <queue> arbitrary.zip" passes the file through unchanged).
// Unknown content yields ".bin".
func DetectExtension(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("%PDF-")):
		return ".pdf"
	case bytes.HasPrefix(data, []byte("PK\x03\x04")),
		bytes.HasPrefix(data, []byte("PK\x05\x06")),
		bytes.HasPrefix(data, []byte("PK\x07\x08")):
		return ".zip"
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return ".png"
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return ".jpg"
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return ".gif"
	default:
		return ".bin"
	}
}
