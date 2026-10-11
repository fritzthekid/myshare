// Package output provides the preliminary test output (R7): instead of
// stdout (which isn't visible under the CUPS daemon), the link is written
// to a directory so it can be looked up after a test print - as a text
// line in links.log and as a QR code PNG. Will later be replaced by the
// actual QR view.
package output

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fritzthekid/myshare/internal/qrview"
)

// Dir is the output directory; tests can redirect it.
var Dir = "tmp"

// Hook is an optional external script that's called as
// `Hook <png-path> <link>` after writing the PNG + log line - e.g. to
// display the QR code on an attached display (see raspi/). Empty (default)
// means no hook.
var Hook = ""

// Default writes link as a line to Dir/links.log, additionally renders it
// as a QR code PNG into Dir, and then - if set - calls Hook.
func Default(link string) error {
	return DefaultWithPassword(link, "")
}

// DefaultWithPassword is like Default, but - if password is not empty -
// appends it as a third argument to the hook call (see cmd/backend:
// Nextcloud shares with an additional share password,
// scripts/share-to-web.sh, internal/sharepassword).
func DefaultWithPassword(link, password string) error {
	if err := os.MkdirAll(Dir, 0o755); err != nil {
		return err
	}

	now := time.Now()
	f, err := os.OpenFile(filepath.Join(Dir, "links.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s\t%s\n", now.Format("2006-01-02T15:04:05"), link); err != nil {
		return err
	}

	pngPath := filepath.Join(Dir, now.Format("20060102-150405")+".png")
	if err := qrview.WritePNG(link, pngPath); err != nil {
		return err
	}

	if Hook != "" {
		runHook(pngPath, link, password)
	}
	return nil
}

// runHook calls Hook, but only reports a failure as a warning: link + QR
// PNG have already been successfully written at this point, an unreachable
// display shouldn't make the print job fail.
func runHook(pngPath, link, password string) {
	args := []string{pngPath, link}
	if password != "" {
		args = append(args, password)
	}
	cmd := exec.Command(Hook, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// CUPS routes backend stderr lines by their (English) prefix into
		// the matching log level category; "WARNING:" thus becomes visible
		// in /var/log/cups/error_log even at the default log level "warn".
		fmt.Fprintf(os.Stderr, "WARNING: output hook (%s) failed: %v\n", Hook, err)
	}
}
