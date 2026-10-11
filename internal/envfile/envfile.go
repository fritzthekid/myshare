// Package envfile loads a simple KEY=VALUE configuration file into the
// process environment - the Windows equivalent of systemd's
// "EnvironmentFile=" (see deploy/backend.env.example, deploy/*.service),
// which doesn't exist for Windows services (New-Service).
package envfile

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Load reads path line by line as KEY=VALUE (blank lines and lines starting
// with "#" are skipped) and sets each variable via os.Setenv. An already-set
// environment variable is overwritten, so a configuration file reliably
// takes effect as written.
func Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: not a KEY=VALUE line: %q", path, lineNo, line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("%s:%d: empty key", path, lineNo)
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
