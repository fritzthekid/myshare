// Command cloudweb is a small web display for the most recently generated
// QR code + link (+ optional password) - a replacement for a physical
// screen (e.g. instead of the Raspberry Pi framebuffer, see raspi/). The
// print driver (or sendfile/webshare) "pushes" the result via HTTP POST to
// /push (see internal/output.Hook + scripts/push-to-cloudweb.sh); a display
// page open in the browser polls automatically and shows whatever's
// newest.
package main

import (
	_ "embed"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/fritzthekid/myshare/internal/envfile"
)

//go:embed static/index.html
var indexHTML []byte

// state holds the most recently pushed QR code + link (+ password). A
// process shows only the latest state at any time, no history.
type state struct {
	mu        sync.RWMutex
	png       []byte
	link      string
	password  string
	updatedAt time.Time
}

func (s *state) set(png []byte, link, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.png = png
	s.link = link
	s.password = password
	s.updatedAt = time.Now()
}

func (s *state) get() (png []byte, link, password string, updatedAt time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.png, s.link, s.password, s.updatedAt
}

type statusResponse struct {
	Link      string `json:"link"`
	Password  string `json:"password,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	HasImage  bool   `json:"has_image"`
}

// isLocalRequest allows /push only from this host itself - the hook runs
// in the same trust context as the print driver, the display page remains
// reachable on the network regardless (e.g. for a monitor/tablet instead
// of the Pi display).
func isLocalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func handlePush(st *state, w http.ResponseWriter, r *http.Request) {
	if !isLocalRequest(r) {
		http.Error(w, "only allowed from localhost", http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "field 'file' is missing", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "file could not be read", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "file is empty", http.StatusBadRequest)
		return
	}

	st.set(data, r.FormValue("link"), r.FormValue("password"))
	w.WriteHeader(http.StatusNoContent)
}

func handleCurrentPNG(st *state, w http.ResponseWriter, r *http.Request) {
	png, _, _, _ := st.get()
	if png == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

func handleCurrentJSON(st *state, w http.ResponseWriter, r *http.Request) {
	png, link, password, updatedAt := st.get()
	resp := statusResponse{Link: link, Password: password, HasImage: png != nil}
	if !updatedAt.IsZero() {
		resp.UpdatedAt = updatedAt.Format(time.RFC3339Nano)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

func newMux(st *state) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Without this header, the browser keeps a once-loaded page cached
		// indefinitely - deploying a new cloudweb version then only shows
		// up after a manual hard reload (observed live).
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("POST /push", func(w http.ResponseWriter, r *http.Request) {
		handlePush(st, w, r)
	})
	mux.HandleFunc("GET /current.png", func(w http.ResponseWriter, r *http.Request) {
		handleCurrentPNG(st, w, r)
	})
	mux.HandleFunc("GET /current.json", func(w http.ResponseWriter, r *http.Request) {
		handleCurrentJSON(st, w, r)
	})
	return mux
}

func main() {
	if err := envfile.LoadDefault(); err != nil {
		// If the file is missing, LoadDefault() is already a no-op (nil); an
		// error here means the file exists but is broken - that shouldn't
		// prevent the plain display function though, hence just a warning
		// instead of aborting.
		println("WARNING: configuration file could not be loaded:", err.Error())
	}

	addr := os.Getenv("CLOUDWEB_LISTEN")
	if addr == "" {
		addr = ":40080"
	}

	st := &state{}
	runService(st, newMux(st), addr)
}
