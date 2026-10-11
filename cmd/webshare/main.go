// Command webshare offers sendfile as a small HTTP service, so any device
// on the network (e.g. a phone via PWA/browser instead of a native app) can
// upload files and have the QR code displayed directly, without needing
// its own Nextcloud credentials.
//
// All routes live under /s/{token}/ - {token} must exactly match
// WEBSHARE_TOKEN, anything else returns 404 (secured via a secret URL
// component instead of a login).
package main

import (
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fritzthekid/myshare/internal/config"
	"github.com/fritzthekid/myshare/internal/filenames"
	"github.com/fritzthekid/myshare/internal/nextcloud"
	"github.com/fritzthekid/myshare/internal/output"
	"github.com/fritzthekid/myshare/internal/qrview"
)

//go:embed static/index.html
var indexHTML []byte

// 200 MiB - generous enough for photos/small videos/zip archives, but
// prevents a client from exhausting the process with a huge body.
const maxUploadBytes = 200 << 20

// As function variables so tests can replace them with fakes.
var (
	uploadFile       = nextcloud.UploadFile
	createPublicLink = nextcloud.CreatePublicLink
)

type uploadResponse struct {
	Link        string `json:"link"`
	QRPNGBase64 string `json:"qr_png_base64"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleUpload(cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "File too large or invalid request"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "No file received"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "File could not be read"})
		return
	}
	if len(data) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "File is empty"})
		return
	}

	label := strings.TrimSpace(r.FormValue("label"))
	if label == "" {
		label = header.Filename
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
		log.Printf("ERROR: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}

	link, err := createPublicLink(remotePath, cfg)
	if err != nil {
		log.Printf("ERROR: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}

	// Log + optional display hook like backend/sendfile - best effort, the
	// link has already been successfully created at this point.
	if err := output.Default(link); err != nil {
		log.Printf("WARNING: output (log/QR file/hook) failed: %v", err)
	}

	png, err := qrview.PNGBytes(link)
	if err != nil {
		log.Printf("ERROR: QR code generation failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "QR code generation failed"})
		return
	}

	writeJSON(w, http.StatusOK, uploadResponse{
		Link:        link,
		QRPNGBase64: base64.StdEncoding.EncodeToString(png),
	})
}

func tokenMatches(r *http.Request, token string) bool {
	got := r.PathValue("token")
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

func newMux(cfg *config.Config, token string) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /s/{token}/", func(w http.ResponseWriter, r *http.Request) {
		if !tokenMatches(r, token) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})

	mux.HandleFunc("POST /s/{token}/upload", func(w http.ResponseWriter, r *http.Request) {
		if !tokenMatches(r, token) {
			http.NotFound(w, r)
			return
		}
		handleUpload(cfg, w, r)
	})

	return mux
}

func main() {
	cfg, err := config.FromEnv(nil)
	if err != nil {
		log.Fatalf("ERROR: %v", err)
	}

	token := os.Getenv("WEBSHARE_TOKEN")
	if token == "" {
		log.Fatal("ERROR: WEBSHARE_TOKEN must be set (a long random token, e.g. `openssl rand -hex 24`)")
	}

	if dir := os.Getenv("MYSHARE_OUTPUT_DIR"); dir != "" {
		output.Dir = dir
	}
	if hook := os.Getenv("MYSHARE_DISPLAY_HOOK"); hook != "" {
		output.Hook = hook
	}

	addr := os.Getenv("WEBSHARE_LISTEN")
	if addr == "" {
		addr = ":8642"
	}

	mux := newMux(cfg, token)
	log.Printf("myshare-webshare listening on %s (path /s/<token>/)", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
