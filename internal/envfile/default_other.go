//go:build !windows

package envfile

// LoadDefault is a no-op on Linux/macOS: there, systemd's
// "EnvironmentFile=" (see deploy/*.service) takes care of reading the
// configuration file, no Go code is needed for that.
func LoadDefault() error {
	return nil
}
