package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fritzthekid/myshare/internal/config"
	"github.com/fritzthekid/myshare/internal/nextcloud"
)

var testCfg = &config.Config{
	BaseURL:        "https://cloud.example.com",
	Username:       "change-cloud-user",
	Password:       "secret",
	TargetDir:      "/PrinterUploads",
	LinkExpireDays: 1,
}

func argv(filename string) []string {
	a := []string{"backend", "42", "alice", "Testdocument", "1", ""}
	if filename != "" {
		a = append(a, filename)
	}
	return a
}

func withFakes(upload func([]byte, string, *config.Config) (string, error), share func(string, *config.Config, string) (string, error)) func() {
	origUpload, origShare := uploadFile, createPublicLink
	if upload != nil {
		uploadFile = upload
	}
	if share != nil {
		createPublicLink = share
	}
	return func() { uploadFile, createPublicLink = origUpload, origShare }
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestNoArgsPrintsDiscoveryLine(t *testing.T) {
	var rc int
	stdout := captureStdout(t, func() {
		rc = run([]string{"backend"}, testCfg, nil, nil)
	})

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.HasPrefix(stdout, "network nextcloud:/ ") {
		t.Errorf("stdout = %q, want CUPS discovery line", stdout)
	}
}

func TestReadsPDFFromFile(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "job.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 file-content"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var gotData []byte
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			gotData = data
			return "/PrinterUploads/x.pdf", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			return "https://cloud.example.com/s/xyz", nil
		},
	)()

	var outputs []string
	rc := run(argv(pdfPath), testCfg, func(link, password string) error {
		outputs = append(outputs, link)
		return nil
	}, nil)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if string(gotData) != "%PDF-1.4 file-content" {
		t.Errorf("gotData = %q", gotData)
	}
	if len(outputs) != 1 || outputs[0] != "https://cloud.example.com/s/xyz" {
		t.Errorf("outputs = %v", outputs)
	}
}

func TestNonPDFPayloadGetsMatchingExtension(t *testing.T) {
	// "lp -d <queue> arbitrary.zip" passes non-PDF content through
	// unchanged (the backend doesn't check this) - the extension must then
	// depend on the actual content, not blindly be ".pdf", otherwise the
	// ZIP would end up as "....zip.pdf".
	var gotFilename string
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			gotFilename = filename
			return "/PrinterUploads/x.zip", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			return "https://cloud.example.com/s/xyz", nil
		},
	)()

	argvZip := []string{"backend", "42", "alice", "testarchive.zip", "1", ""}
	rc := run(argvZip, testCfg, func(link, password string) error { return nil }, strings.NewReader("PK\x03\x04 zip-content"))

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.HasSuffix(gotFilename, ".zip") {
		t.Errorf("gotFilename = %q, want .zip suffix (not .pdf)", gotFilename)
	}
	if strings.HasSuffix(gotFilename, ".zip.zip") {
		t.Errorf("gotFilename = %q, title extension .zip should have been removed (duplication)", gotFilename)
	}
}

func TestTitleExtensionKeptWhenItDiffersFromContent(t *testing.T) {
	// A job title like "report.docx" with actual PDF content (normal print
	// case) keeps its extension in the title - only a title extension that
	// exactly matches the content is removed (see above).
	var gotFilename string
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			gotFilename = filename
			return "/PrinterUploads/x.pdf", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			return "https://cloud.example.com/s/xyz", nil
		},
	)()

	argvDocx := []string{"backend", "42", "alice", "report.docx", "1", ""}
	rc := run(argvDocx, testCfg, func(link, password string) error { return nil }, strings.NewReader("%PDF-1.4"))

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !strings.HasSuffix(gotFilename, "_report.docx.pdf") {
		t.Errorf("gotFilename = %q, want suffix _report.docx.pdf", gotFilename)
	}
}

func TestPasswordSeedFromOptionsDerivesPassword(t *testing.T) {
	// "lp -o nc-password-seed=username" ends up as "nc-password-seed=username"
	// in argv[5] (options) - see scripts/share-to-web.sh. The MAC address is
	// mocked for a deterministic test (flows in as a silent second factor,
	// see internal/sharepassword).
	origMAC := localMAC
	localMAC = func() (string, error) { return "mac", nil }
	defer func() { localMAC = origMAC }()

	var gotPassword string
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			return "/PrinterUploads/x.zip", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			gotPassword = password
			return "https://cloud.example.com/s/xyz", nil
		},
	)()

	var gotCrypt string
	argvWithSeed := []string{"backend", "42", "alice", "testdata.zip", "1", "nc-password-seed=username"}
	rc := run(argvWithSeed, testCfg, func(link, password string) error {
		gotCrypt = password
		return nil
	}, strings.NewReader("PK\x03\x04 zip-content"))

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	// crypt is now in groups of five (see internal/sharepassword.FormatGroups)
	// and uses the new default length NC_LEN_CODE=15 (testCfg doesn't set
	// LenCode, so Derive falls back to sharepassword.DefaultLength).
	if gotCrypt != "XZTT7-HL7VO-AWOEW" {
		t.Errorf("password passed to out()/crypt = %q, want %q", gotCrypt, "XZTT7-HL7VO-AWOEW")
	}
	// password must consist ONLY of name+crypt (not the extended
	// hardenedSeed), otherwise the recipient could never type it correctly -
	// exactly this bug was observed live and fixed here.
	if gotPassword != "usernameXZTT7-HL7VO-AWOEW" {
		t.Errorf("password passed to createPublicLink = %q, want %q", gotPassword, "usernameXZTT7-HL7VO-AWOEW")
	}
}

func TestPasswordSeedRespectsConfiguredLenCode(t *testing.T) {
	origMAC := localMAC
	localMAC = func() (string, error) { return "mac", nil }
	defer func() { localMAC = origMAC }()

	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			return "/PrinterUploads/x.zip", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			return "https://cloud.example.com/s/xyz", nil
		},
	)()

	cfgWithLenCode := *testCfg
	cfgWithLenCode.LenCode = 10

	var gotCrypt string
	argvWithSeed := []string{"backend", "42", "alice", "testdata.zip", "1", "nc-password-seed=username"}
	rc := run(argvWithSeed, &cfgWithLenCode, func(link, password string) error {
		gotCrypt = password
		return nil
	}, strings.NewReader("PK\x03\x04 zip-content"))

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	rawLen := len(strings.ReplaceAll(gotCrypt, "-", ""))
	if rawLen != 10 {
		t.Errorf("crypt length (without hyphens) = %d, want NC_LEN_CODE = 10 (crypt = %q)", rawLen, gotCrypt)
	}
}

func TestSameNameProducesDifferentCryptOnRepeatedRuns(t *testing.T) {
	// remotePath (unique per run thanks to the timestamp) flows into the
	// password derivation so that the same "name" doesn't yield the same
	// crypt every time (otherwise a crypt seen once could be reused for all
	// future shares with the same name).
	origMAC := localMAC
	localMAC = func() (string, error) { return "mac", nil }
	defer func() { localMAC = origMAC }()

	callCount := 0
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			callCount++
			return fmt.Sprintf("/PrinterUploads/run%d.zip", callCount), nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			return "https://cloud.example.com/s/xyz", nil
		},
	)()

	argvWithSeed := []string{"backend", "42", "alice", "testdata.zip", "1", "nc-password-seed=username"}

	var crypt1, crypt2 string
	rc1 := run(argvWithSeed, testCfg, func(link, password string) error { crypt1 = password; return nil }, strings.NewReader("PK\x03\x04 zip-content"))
	rc2 := run(argvWithSeed, testCfg, func(link, password string) error { crypt2 = password; return nil }, strings.NewReader("PK\x03\x04 zip-content"))

	if rc1 != 0 || rc2 != 0 {
		t.Fatalf("rc1=%d rc2=%d, want both 0", rc1, rc2)
	}
	if crypt1 == crypt2 {
		t.Errorf("same name produced the same crypt twice: %q", crypt1)
	}
}

func TestPasswordSeedRequestedButMACLookupFailsAborts(t *testing.T) {
	// Without the MAC address, crypt could be computed from just the name
	// communicated to the recipient (no real 2FA) - so it's better to let
	// the whole job fail than to silently fall back to a weaker scheme.
	origMAC := localMAC
	localMAC = func() (string, error) { return "", errors.New("no network interface") }
	defer func() { localMAC = origMAC }()

	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			return "/PrinterUploads/x.zip", nil
		},
		nil,
	)()

	argvWithSeed := []string{"backend", "42", "alice", "testdata.zip", "1", "nc-password-seed=username"}
	rc := run(argvWithSeed, testCfg, func(link, password string) error { return nil }, strings.NewReader("PK\x03\x04 zip-content"))

	if rc != 1 {
		t.Fatalf("rc = %d, want 1 (job should fail on MAC lookup error)", rc)
	}
}

func TestNoPasswordSeedMeansNoPassword(t *testing.T) {
	var gotPassword string
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			return "/PrinterUploads/x.pdf", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			gotPassword = password
			return "https://cloud.example.com/s/xyz", nil
		},
	)()

	var gotCrypt string
	rc := run(argv(""), testCfg, func(link, password string) error {
		gotCrypt = password
		return nil
	}, strings.NewReader("%PDF-1.4"))

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if gotPassword != "" {
		t.Errorf("gotPassword = %q, want empty (no seed passed)", gotPassword)
	}
	if gotCrypt != "" {
		t.Errorf("gotCrypt = %q, want empty (no seed passed)", gotCrypt)
	}
}

func TestReadsPDFFromStdin(t *testing.T) {
	var gotData []byte
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			gotData = data
			return "/PrinterUploads/x.pdf", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			return "https://cloud.example.com/s/xyz", nil
		},
	)()

	var outputs []string
	rc := run(argv(""), testCfg, func(link, password string) error {
		outputs = append(outputs, link)
		return nil
	}, strings.NewReader("%PDF-1.4 stdin-content"))

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if string(gotData) != "%PDF-1.4 stdin-content" {
		t.Errorf("gotData = %q", gotData)
	}
}

func TestEmptyPDFIsRejected(t *testing.T) {
	var outputs []string
	var rc int
	stderr := captureStderr(t, func() {
		rc = run(argv(""), testCfg, func(link, password string) error {
			outputs = append(outputs, link)
			return nil
		}, strings.NewReader(""))
	})

	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if len(outputs) != 0 {
		t.Errorf("outputs = %v, want empty", outputs)
	}
	if !strings.Contains(stderr, "no PDF data") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestFullFlowOutputsLink(t *testing.T) {
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			return "/PrinterUploads/x.pdf", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			return "https://cloud.example.com/s/final-link", nil
		},
	)()

	var outputs []string
	rc := run(argv(""), testCfg, func(link, password string) error {
		outputs = append(outputs, link)
		return nil
	}, strings.NewReader("%PDF-1.4"))

	if rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if len(outputs) != 1 || outputs[0] != "https://cloud.example.com/s/final-link" {
		t.Errorf("outputs = %v", outputs)
	}
}

func TestUploadErrorIsReported(t *testing.T) {
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			return "", &nextcloud.Error{Msg: "Upload failed (507)"}
		},
		nil,
	)()

	var outputs []string
	var rc int
	stderr := captureStderr(t, func() {
		rc = run(argv(""), testCfg, func(link, password string) error {
			outputs = append(outputs, link)
			return nil
		}, strings.NewReader("%PDF-1.4"))
	})

	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if len(outputs) != 0 {
		t.Errorf("outputs = %v, want empty", outputs)
	}
	if !strings.Contains(stderr, "Upload failed") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestShareErrorIsReported(t *testing.T) {
	defer withFakes(
		func(data []byte, filename string, cfg *config.Config) (string, error) {
			return "/PrinterUploads/x.pdf", nil
		},
		func(remotePath string, cfg *config.Config, password string) (string, error) {
			return "", &nextcloud.Error{Msg: "Failed to create share (404)"}
		},
	)()

	var outputs []string
	var rc int
	stderr := captureStderr(t, func() {
		rc = run(argv(""), testCfg, func(link, password string) error {
			outputs = append(outputs, link)
			return nil
		}, strings.NewReader("%PDF-1.4"))
	})

	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if len(outputs) != 0 {
		t.Errorf("outputs = %v, want empty", outputs)
	}
	if !strings.Contains(stderr, "Failed to create share") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestMissingArgumentsReturnsError(t *testing.T) {
	var rc int
	stderr := captureStderr(t, func() {
		rc = run([]string{"backend", "42"}, testCfg, nil, nil)
	})

	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr = %q", stderr)
	}
}
