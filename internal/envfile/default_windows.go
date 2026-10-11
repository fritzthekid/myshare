//go:build windows

package envfile

import (
	"os"
	"path/filepath"
)

// DefaultDir is the Windows config/working directory, analogous to
// /etc/myshare on Linux (see deploy/install.sh) - this is where backend.env
// lives, as well as (see cmd/cloudweb) the fixed print output file
// incoming.pdf.
func DefaultDir() string {
	dir := os.Getenv("ProgramData")
	if dir == "" {
		dir = `C:\ProgramData`
	}
	return filepath.Join(dir, "myshare")
}

// DefaultPath is the Windows configuration path, analogous to
// /etc/myshare/backend.env on Linux (see deploy/install.sh).
func DefaultPath() string {
	return filepath.Join(DefaultDir(), "backend.env")
}

// LoadDefault loads DefaultPath(), if the file exists. If it's missing
// (e.g. before installation), that's not an error - the subsequent
// config.FromEnv check reports missing required variables clearly anyway.
func LoadDefault() error {
	path := DefaultPath()
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return Load(path)
}
