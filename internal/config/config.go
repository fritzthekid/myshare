// Package config reads the driver configuration exclusively from
// environment variables (R6).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fritzthekid/myshare/internal/sharepassword"
)

type Config struct {
	BaseURL        string
	Username       string
	Password       string
	TargetDir      string
	LinkExpireDays int
	LenCode        int
}

// FromEnv builds the Config from env. If env is nil, os.Environ is used.
func FromEnv(env map[string]string) (*Config, error) {
	lookup := func(key string) (string, bool) {
		if env != nil {
			v, ok := env[key]
			return v, ok
		}
		return os.LookupEnv(key)
	}

	values := map[string]string{}
	var missing []string
	for _, key := range []string{"NC_BASE_URL", "NC_USERNAME", "NC_PASSWORD"} {
		v, ok := lookup(key)
		if !ok {
			missing = append(missing, key)
			continue
		}
		values[key] = v
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("Missing environment variable(s): %s", strings.Join(missing, ", "))
	}

	targetDir := "/PrinterUploads"
	if v, ok := lookup("NC_TARGET_DIR"); ok {
		targetDir = v
	}

	expireDays := 1
	if v, ok := lookup("NC_LINK_EXPIRE_DAYS"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("Invalid value for NC_LINK_EXPIRE_DAYS: %q", v)
		}
		expireDays = n
	}

	lenCode := sharepassword.DefaultLength
	if v, ok := lookup("NC_LEN_CODE"); ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("Invalid value for NC_LEN_CODE: %q", v)
		}
		lenCode = n
	}

	return &Config{
		BaseURL:        strings.TrimRight(values["NC_BASE_URL"], "/"),
		Username:       values["NC_USERNAME"],
		Password:       values["NC_PASSWORD"],
		TargetDir:      targetDir,
		LinkExpireDays: expireDays,
		LenCode:        lenCode,
	}, nil
}
