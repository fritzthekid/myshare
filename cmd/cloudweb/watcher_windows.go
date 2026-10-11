//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fritzthekid/myshare/internal/config"
	"github.com/fritzthekid/myshare/internal/envfile"
	"github.com/fritzthekid/myshare/internal/filenames"
	"github.com/fritzthekid/myshare/internal/nextcloud"
	"github.com/fritzthekid/myshare/internal/qrview"
)

// printPollInterval determines how quickly a print job via the file port
// "CloudWeb" (see doc/windows_requirement.md) is detected - a fixed second
// is enough for the use case (someone prints and then looks at the
// display page) and keeps system load minimal.
const printPollInterval = 1 * time.Second

// incomingFileName is the name the printer port
// "C:\ProgramData\myshare\incoming.pdf" produces (see
// deploy/windows/install.ps1: Add-PrinterPort).
const incomingFileName = "incoming.pdf"

// As function variables so tests can replace them with fakes (analogous
// to cmd/backend, cmd/sendfile).
var (
	uploadFile       = nextcloud.UploadFile
	createPublicLink = nextcloud.CreatePublicLinkWithPassword
)

// startPrintWatcher watches the fixed print target in the background: as
// soon as a file appears there, it's immediately renamed atomically
// (prevents a second, quickly following print job from overwriting it
// before it's processed) and then uploaded and shared just like the Linux
// backend. Runs until stop is closed.
func startPrintWatcher(st *state, stop <-chan struct{}) {
	dir := envfile.DefaultDir()
	incoming := filepath.Join(dir, incomingFileName)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("print watcher: could not create directory %s: %v", dir, err)
		return
	}

	ticker := time.NewTicker(printPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			checkIncoming(st, dir, incoming)
		}
	}
}

func checkIncoming(st *state, dir, incoming string) {
	if _, err := os.Stat(incoming); err != nil {
		return // no new print job yet
	}
	processing := filepath.Join(dir, fmt.Sprintf("job-%d.pdf", time.Now().UnixNano()))
	if err := os.Rename(incoming, processing); err != nil {
		// The spooler is probably still writing - try again on the next
		// tick instead of processing a half-finished file.
		log.Printf("print watcher: rename failed (will retry on next tick): %v", err)
		return
	}
	go handlePrintedJob(st, processing)
}

// handlePrintedJob uploads an already-renamed, finished print file,
// creates the share link + QR code, and updates the display (st) - without
// password protection, that's reserved for the SendTo path (cmd/fileshare)
// (see doc/windows_requirement.md).
func handlePrintedJob(st *state, path string) {
	defer func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("print watcher: could not delete temporary file %s: %v", path, err)
		}
	}()

	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("print watcher: could not read %s: %v", path, err)
		return
	}
	if len(data) == 0 {
		log.Printf("print watcher: %s was empty, discarded", path)
		return
	}

	cfg, err := config.FromEnv(nil)
	if err != nil {
		log.Printf("print watcher: configuration missing: %v", err)
		return
	}

	ext := filenames.DetectExtension(data)
	filename := filenames.Generate("print-job", ext, time.Now())
	remotePath, err := uploadFile(data, filename, cfg)
	if err != nil {
		log.Printf("print watcher: upload failed: %v", err)
		return
	}

	link, err := createPublicLink(remotePath, cfg, "")
	if err != nil {
		log.Printf("print watcher: share creation failed: %v", err)
		return
	}

	png, err := qrview.PNGBytes(link)
	if err != nil {
		log.Printf("print watcher: QR code could not be generated: %v", err)
		return
	}

	st.set(png, link, "")
	log.Printf("print watcher: new print job processed -> %s", link)
}
