// Command backend is the CUPS backend: receives the print job already
// converted to PDF by CUPS, uploads it to a Nextcloud instance, creates a
// public share link, and outputs it (for now as plain text).
//
// CUPS invokes backends like this:
//
//	backend job-id user title copies options [filename]
//
// The print data is in filename if present, otherwise on stdin.
//
// If "options" contains "nc-password-seed=<seed>" (e.g. via
// "lp -o nc-password-seed=<seed>", see scripts/share-to-web.sh), the share
// link additionally gets a password derived from seed (see
// internal/sharepassword) - only the publicly displayed short form (crypt)
// is output, not seed itself.
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
	"github.com/fritzthekid/myshare/internal/sharepassword"
)

// As function variables so tests can replace them with fakes.
var (
	uploadFile       = nextcloud.UploadFile
	createPublicLink = nextcloud.CreatePublicLinkWithPassword
	defaultOutput    = output.DefaultWithPassword
	localMAC         = sharepassword.LocalMAC
)

// passwordSeedFromOptions reads "nc-password-seed=<seed>" from the options
// string passed by CUPS (argv[5], space-separated "key=value" pairs) - see
// scripts/share-to-web.sh.
func passwordSeedFromOptions(options string) string {
	for _, tok := range strings.Fields(options) {
		if seed, ok := strings.CutPrefix(tok, "nc-password-seed="); ok {
			return seed
		}
	}
	return ""
}

var errEmptyPDF = errors.New("no PDF data received")

func readPDFData(argv []string, stdin io.Reader) ([]byte, error) {
	if len(argv) >= 7 && argv[6] != "" {
		return os.ReadFile(argv[6])
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	return io.ReadAll(stdin)
}

// discoveryLine is the response to the CUPS device discovery call (0
// arguments), as evaluated by e.g. "lpinfo -v":
// https://www.cups.org/doc/man-backend.html
const discoveryLine = `network nextcloud:/ "Unknown" "Nextcloud QR printer" "Uploads a PDF to Nextcloud and creates a QR code share link"`

func run(argv []string, cfg *config.Config, out func(link, password string) error, stdin io.Reader) int {
	if len(argv) == 1 {
		fmt.Println(discoveryLine)
		return 0
	}
	if len(argv) < 6 {
		fmt.Fprintln(os.Stderr, "Usage: backend job-id user title copies options [file]")
		return 1
	}
	jobTitle := argv[3]

	link, crypt, err := doRun(argv, jobTitle, cfg, stdin)
	if err != nil {
		return reportError(err)
	}

	if out == nil {
		out = defaultOutput
	}
	if err := out(link, crypt); err != nil {
		return reportError(err)
	}
	return 0
}

func doRun(argv []string, jobTitle string, cfg *config.Config, stdin io.Reader) (link, crypt string, err error) {
	if cfg == nil {
		cfg, err = config.FromEnv(nil)
		if err != nil {
			return "", "", err
		}
	}

	pdfBytes, err := readPDFData(argv, stdin)
	if err != nil {
		return "", "", err
	}
	if len(pdfBytes) == 0 {
		return "", "", errEmptyPDF
	}

	// CUPS usually delivers real PDF here, but "lp -d <queue> arbitrary.zip"
	// passes non-PDF content through unchanged too (the backend doesn't
	// check this) - so the extension is based on content rather than
	// blindly assumed to be ".pdf", otherwise e.g. a ZIP would end up as
	// "....zip.pdf".
	ext := filenames.DetectExtension(pdfBytes)
	title := jobTitle
	// "lp <file>" without its own -t title uses the filename as the job
	// title (e.g. "testdata.zip") - if that already carries exactly the
	// detected extension, it would otherwise be duplicated
	// ("...testdata.zip.zip"). Deliberately stripped only on an exact
	// match: a title like "report.docx" with actual PDF content is left
	// untouched (".docx.pdf" still carries information there, not a
	// duplication).
	if titleExt := filepath.Ext(title); titleExt != "" && strings.EqualFold(titleExt, ext) {
		title = strings.TrimSuffix(title, titleExt)
	}
	filename := filenames.Generate(title, ext, time.Now())
	remotePath, err := uploadFile(pdfBytes, filename, cfg)
	if err != nil {
		return "", "", err
	}

	var password string
	if seed := passwordSeedFromOptions(argv[5]); seed != "" {
		// This machine's MAC address flows in as a silent second factor:
		// without it, crypt could be computed from just the name
		// communicated to the recipient (seed) - no real 2FA. remotePath
		// (unique per run thanks to the timestamp) additionally ensures
		// that the same name doesn't yield the same crypt every time - the
		// actual share link isn't known yet at this point (it's only
		// created by the call we already have to pass the password to).
		mac, macErr := localMAC()
		if macErr != nil {
			return "", "", fmt.Errorf("password derivation failed (MAC address): %w", macErr)
		}
		crypt, password = sharepassword.Derive(seed, seed+mac+remotePath, cfg.LenCode)
	}

	link, err = createPublicLink(remotePath, cfg, password)
	return link, crypt, err
}

// reportError reports err to stderr (CUPS convention for failed jobs) and
// returns exit code 1: a NextcloudError and the "no data" message are kept
// as-is, anything else counts as an unexpected error.
func reportError(err error) int {
	switch {
	case errors.Is(err, errEmptyPDF):
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
	// "tmp" is relative to the process's working directory; under CUPS
	// that's not the project directory, hence overridable via an env var
	// (see deploy/backend.env.example).
	if dir := os.Getenv("MYSHARE_OUTPUT_DIR"); dir != "" {
		output.Dir = dir
	}
	if hook := os.Getenv("MYSHARE_DISPLAY_HOOK"); hook != "" {
		output.Hook = hook
	}
	os.Exit(run(os.Args, nil, nil, nil))
}
