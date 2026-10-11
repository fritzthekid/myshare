// Command sendfile shares an arbitrary file via Nextcloud + QR code.
//
// Uses the same building blocks as the CUPS backend (config, upload, share
// link, log/QR output), but without the CUPS invocation signature
// (job-id user title copies options [file]). Instead, a simple call:
//
//	sendfile path/to/test.zip
//	sendfile path/to/test.zip "Other Title.zip"
//	cat test.zip | sendfile - test.zip
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fritzthekid/myshare/internal/config"
	"github.com/fritzthekid/myshare/internal/filenames"
	"github.com/fritzthekid/myshare/internal/nextcloud"
	"github.com/fritzthekid/myshare/internal/output"
)

// As function variables so tests can replace them with fakes.
var (
	uploadFile       = nextcloud.UploadFile
	createPublicLink = nextcloud.CreatePublicLink
	defaultOutput    = output.Default
)

var errNoData = errors.New("no data received")

func readData(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		if stdin == nil {
			stdin = os.Stdin
		}
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

func run(argv []string, cfg *config.Config, out func(string) error, stdin io.Reader) int {
	if len(argv) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: sendfile <path|-> [label]")
		return 1
	}
	path := argv[1]

	var label string
	switch {
	case len(argv) >= 3:
		label = argv[2]
	case path == "-":
		label = "file"
	default:
		label = filepath.Base(path)
	}

	ext := filepath.Ext(label)
	stem := strings.TrimSuffix(label, ext)
	if stem == "" {
		stem = label
	}
	if ext == "" {
		ext = ".bin"
	}

	link, err := doRun(path, stem, ext, cfg, stdin)
	if err != nil {
		return reportError(err)
	}

	if out == nil {
		out = defaultOutput
	}
	if err := out(link); err != nil {
		return reportError(err)
	}
	return 0
}

func doRun(path, stem, ext string, cfg *config.Config, stdin io.Reader) (string, error) {
	var err error
	if cfg == nil {
		cfg, err = config.FromEnv(nil)
		if err != nil {
			return "", err
		}
	}

	data, err := readData(path, stdin)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errNoData
	}

	filename := filenames.Generate(stem, ext, time.Now())
	remotePath, err := uploadFile(data, filename, cfg)
	if err != nil {
		return "", err
	}
	return createPublicLink(remotePath, cfg)
}

// reportError reports err to stderr and returns exit code 1: the "no data"
// message and NextcloudError are kept as-is, anything else counts as an
// unexpected error.
func reportError(err error) int {
	switch {
	case errors.Is(err, errNoData):
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	default:
		var nce *nextcloud.Error
		if errors.As(err, &nce) {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: unexpected error: %v\n", err)
		}
	}
	return 1
}

func main() {
	if dir := os.Getenv("MYSHARE_OUTPUT_DIR"); dir != "" {
		output.Dir = dir
	}
	if hook := os.Getenv("MYSHARE_DISPLAY_HOOK"); hook != "" {
		output.Hook = hook
	}
	os.Exit(run(os.Args, nil, nil, nil))
}
