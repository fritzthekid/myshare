// Command fileshare is the Windows equivalent of
// scripts/share-to-web.sh + scripts/push-to-cloudweb.sh combined in one
// program - meant for integration as an Explorer "Send to" entry (see
// doc/windows_requirement.md, deploy/windows/install.ps1).
//
// Invocation (triggered automatically by Explorer "Send to"):
//
//	fileshare.exe C:\path\to\file.pdf
//
// Without a second argument, the optional "Name" (the basis for the share
// password, see internal/sharepassword) is prompted interactively on the
// console - empty means: no password-protected share, same as a share
// without a name on Linux (scripts/share-to-web.sh without a second
// argument).
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fritzthekid/myshare/internal/config"
	"github.com/fritzthekid/myshare/internal/envfile"
	"github.com/fritzthekid/myshare/internal/filenames"
	"github.com/fritzthekid/myshare/internal/nextcloud"
	"github.com/fritzthekid/myshare/internal/qrview"
	"github.com/fritzthekid/myshare/internal/sharepassword"
)

// As function variables so tests can replace them with fakes (analogous
// to cmd/backend, cmd/sendfile).
var (
	uploadFile       = nextcloud.UploadFile
	createPublicLink = nextcloud.CreatePublicLinkWithPassword
	localMAC         = sharepassword.LocalMAC
	pushToCloudweb   = defaultPush
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

// promptName asks for the optional name interactively - only used if it
// wasn't already passed as a second argument (Explorer "Send to" can't
// pass additional arguments).
func promptName(stdin io.Reader, stdout io.Writer) (string, error) {
	fmt.Fprint(stdout, "Name (optional, for password protection; empty = no password): ")
	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		return "", scanner.Err()
	}
	return strings.TrimSpace(scanner.Text()), nil
}

func run(argv []string, cfg *config.Config, stdin io.Reader, stdout io.Writer) int {
	if len(argv) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: fileshare <path|-> [name]")
		return 1
	}
	path := argv[1]

	var name string
	var err error
	if len(argv) >= 3 {
		name = argv[2]
	} else {
		name, err = promptName(stdin, stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not read input: %v\n", err)
			return 1
		}
	}

	link, crypt, err := doRun(path, name, cfg, stdin)
	if err != nil {
		return reportError(err)
	}

	fmt.Fprintf(stdout, "Link: %s\n", link)
	if crypt != "" {
		fmt.Fprintf(stdout, "Password short form (type in together with %q): %s\n", name, crypt)
	}

	// An unreachable cloudweb service shouldn't report the already
	// successful share as a failure - the link/password are already shown
	// above.
	if err := pushToCloudweb(link, crypt); err != nil {
		fmt.Fprintf(stdout, "Note: push to cloudweb failed (%v) - the link/QR code above is still valid.\n", err)
	}

	return 0
}

func doRun(path, name string, cfg *config.Config, stdin io.Reader) (link, crypt string, err error) {
	if cfg == nil {
		cfg, err = config.FromEnv(nil)
		if err != nil {
			return "", "", err
		}
	}

	data, err := readData(path, stdin)
	if err != nil {
		return "", "", err
	}
	if len(data) == 0 {
		return "", "", errNoData
	}

	label := filepath.Base(path)
	if path == "-" {
		label = "file"
	}
	ext := filepath.Ext(label)
	stem := strings.TrimSuffix(label, ext)
	if stem == "" {
		stem = label
	}
	if ext == "" {
		ext = ".bin"
	}

	filename := filenames.Generate(stem, ext, time.Now())
	remotePath, err := uploadFile(data, filename, cfg)
	if err != nil {
		return "", "", err
	}

	var password string
	if name != "" {
		// See cmd/backend: same derivation (name + this machine's MAC
		// address + unique remote path), so the behavior for the recipient
		// is identical to the Linux CloudWeb share.
		mac, macErr := localMAC()
		if macErr != nil {
			return "", "", fmt.Errorf("password derivation failed (MAC address): %w", macErr)
		}
		crypt, password = sharepassword.Derive(name, name+mac+remotePath, cfg.LenCode)
	}

	link, err = createPublicLink(remotePath, cfg, password)
	return link, crypt, err
}

// defaultPush corresponds to scripts/push-to-cloudweb.sh: renders the QR
// code itself (no external hook needed) and pushes it via
// multipart/form-data to a locally running cloudweb service. crypt is -
// like with the CUPS backend - the short display form, not the full share
// password.
func defaultPush(link, crypt string) error {
	png, err := qrview.PNGBytes(link)
	if err != nil {
		return err
	}

	base := os.Getenv("CLOUDWEB_URL")
	if base == "" {
		base = "http://127.0.0.1:40080"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "qr.png")
	if err != nil {
		return err
	}
	if _, err := fw.Write(png); err != nil {
		return err
	}
	if err := w.WriteField("link", link); err != nil {
		return err
	}
	if crypt != "" {
		if err := w.WriteField("password", crypt); err != nil {
			return err
		}
	}
	if err := w.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/push", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("cloudweb responded with status %d", resp.StatusCode)
	}
	return nil
}

// reportError reports err to stderr and returns exit code 1 (analogous to
// cmd/backend, cmd/sendfile).
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
	_ = envfile.LoadDefault()
	code := run(os.Args, nil, os.Stdin, os.Stdout)

	// Explorer "Send to" opens a console window that closes immediately
	// when the process ends - without this pause, the link/password short
	// form would never be readable.
	fmt.Fprintln(os.Stdout, "\nPress Enter to exit...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')

	os.Exit(code)
}
