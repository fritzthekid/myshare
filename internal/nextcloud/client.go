// Package nextcloud handles the upload via WebDAV (R4) and the creation of
// a public, expiring link via the OCS API (R5).
package nextcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fritzthekid/myshare/internal/config"
)

const httpTimeout = 30 * time.Second

// Error is returned on a failed upload or a failed share creation.
type Error struct {
	Msg string
}

func (e *Error) Error() string { return e.Msg }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func davURL(cfg *config.Config, remotePath string) string {
	return fmt.Sprintf("%s/remote.php/dav/files/%s%s", cfg.BaseURL, cfg.Username, remotePath)
}

// ensureRemoteDir creates the target directory (including intermediate
// levels) via WebDAV MKCOL. Nextcloud doesn't create missing parent folders
// on a PUT, so the target directory must exist beforehand. An already
// existing folder reports 405 (Method Not Allowed), which is deliberately
// ignored.
func ensureRemoteDir(cfg *config.Config) error {
	client := &http.Client{Timeout: httpTimeout}
	path := ""
	for _, segment := range strings.Split(strings.Trim(cfg.TargetDir, "/"), "/") {
		if segment == "" {
			continue
		}
		path += "/" + segment
		req, err := http.NewRequest("MKCOL", davURL(cfg, path), nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(cfg.Username, cfg.Password)

		resp, err := client.Do(req)
		if err != nil {
			return &Error{Msg: fmt.Sprintf("Failed to create target folder: %v", err)}
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusMethodNotAllowed {
			return &Error{Msg: fmt.Sprintf("Failed to create target folder (%d): %s", resp.StatusCode, truncate(string(body), 200))}
		}
	}
	return nil
}

// UploadFile uploads data via WebDAV PUT as filename in cfg.TargetDir and
// returns the resulting remote path.
func UploadFile(data []byte, filename string, cfg *config.Config) (string, error) {
	if err := ensureRemoteDir(cfg); err != nil {
		return "", err
	}

	remotePath := strings.TrimRight(cfg.TargetDir, "/") + "/" + filename

	req, err := http.NewRequest(http.MethodPut, davURL(cfg, remotePath), bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(cfg.Username, cfg.Password)

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", &Error{Msg: fmt.Sprintf("Upload failed: %v", err)}
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return remotePath, nil
	default:
		return "", &Error{Msg: fmt.Sprintf("Upload failed (%d): %s", resp.StatusCode, truncate(string(body), 200))}
	}
}

type shareResponse struct {
	OCS struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	} `json:"ocs"`
}

// CreatePublicLink creates a share link for remotePath via the OCS share API
// (shareType=3, public link) that expires after cfg.LinkExpireDays days.
//
// The plain share link (".../s/TOKEN") returns Nextcloud's web UI preview
// page. With "/download" appended, the server instead responds with
// "Content-Disposition: attachment", so browsers download the file directly
// or open it in the phone's PDF viewer, instead of showing it in the web
// cloud viewer first.
func CreatePublicLink(remotePath string, cfg *config.Config) (string, error) {
	return CreatePublicLinkWithPassword(remotePath, cfg, "")
}

// CreatePublicLinkWithPassword is like CreatePublicLink, but additionally
// sets a share password (Nextcloud's OCS share API supports this natively)
// if password is not empty - see cmd/backend, internal/sharepassword,
// scripts/share-to-web.sh.
func CreatePublicLinkWithPassword(remotePath string, cfg *config.Config, password string) (string, error) {
	shareURL := fmt.Sprintf("%s/ocs/v2.php/apps/files_sharing/api/v1/shares", cfg.BaseURL)
	expireDate := time.Now().AddDate(0, 0, cfg.LinkExpireDays).Format("2006-01-02")

	form := url.Values{}
	form.Set("path", remotePath)
	form.Set("shareType", "3")
	form.Set("expireDate", expireDate)
	if password != "" {
		form.Set("password", password)
	}

	req, err := http.NewRequest(http.MethodPost, shareURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cfg.Username, cfg.Password)

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", &Error{Msg: fmt.Sprintf("Failed to create share: %v", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", &Error{Msg: fmt.Sprintf("Failed to create share (%d): %s", resp.StatusCode, truncate(string(body), 200))}
	}

	var parsed shareResponse
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.OCS.Data.URL == "" {
		return "", &Error{Msg: fmt.Sprintf("Unexpected response from share API: %s", truncate(string(body), 200))}
	}

	return strings.TrimRight(parsed.OCS.Data.URL, "/") + "/download", nil
}
