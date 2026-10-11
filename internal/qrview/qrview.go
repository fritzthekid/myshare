// Package qrview generates a QR code PNG from the share link.
//
// The public link is meant to be displayed later as a QR code. For now,
// the PNG is just written to a file (analogous to the text output of the
// link).
package qrview

import (
	"os"

	qrcode "github.com/skip2/go-qrcode"
)

// PNGBytes renders link as a QR code and returns the PNG bytes (e.g. for a
// direct HTTP response, without going through a file).
func PNGBytes(link string) ([]byte, error) {
	return qrcode.Encode(link, qrcode.Medium, 256)
}

// WritePNG renders link as a QR code and saves it as a PNG at path.
func WritePNG(link string, path string) error {
	data, err := PNGBytes(link)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
