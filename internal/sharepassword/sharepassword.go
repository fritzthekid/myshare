// Package sharepassword derives a Nextcloud share password from a
// self-chosen name using a simple two-factor scheme (see cmd/backend,
// scripts/share-to-web.sh): only the publicly displayed short form (crypt)
// appears on the printout/cloudweb display - anyone who only sees that
// (e.g. a photographed QR code) can't get to the file with it alone. The
// recipient additionally has to know the name outside this system (e.g.
// agreed verbally) and type "<name><crypt>" as a combined password when
// opening the link.
package sharepassword

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"net"
	"strings"
)

// DefaultLength is the default code length (NC_LEN_CODE, see
// internal/config) unless configured otherwise - divisible by 5, so
// FormatGroups yields three evenly sized groups of five with no remainder.
const DefaultLength = 15

// Derive computes:
//
//	crypt    = FormatGroups(base32(sha256(hardenedSeed))[:length]) - displayed/printed
//	password = name + crypt                                        - the actual share password
//
// name and hardenedSeed are deliberately kept separate: password consists
// only of name+crypt, because those are the only two parts the recipient
// actually knows/sees (name agreed verbally, crypt displayed publicly).
// hardenedSeed, on the other hand, flows ONLY into the computation of
// crypt and should contain more than just name (e.g. + this machine's MAC
// address + a value unique per share, see LocalMAC, cmd/backend) -
// otherwise crypt could be computed from just the name communicated to the
// recipient (no real 2FA) and would be identical for every share with the
// same name.
//
// crypt (and thus also password) contains hyphens as a readability/typing
// aid (see FormatGroups) - the password actually to be typed into
// Nextcloud is exactly what's displayed, no mental transformation needed
// by the recipient.
func Derive(name, hardenedSeed string, length int) (crypt string, password string) {
	if length <= 0 {
		length = DefaultLength
	}
	sum := sha256.Sum256([]byte(hardenedSeed))
	raw := base32.StdEncoding.EncodeToString(sum[:])[:length]
	crypt = FormatGroups(raw)
	return crypt, name + crypt
}

// FormatGroups splits code into hyphen-separated groups of five (e.g.
// "SJBB3ZMD7PR4HIQ" -> "SJBB3-ZMD7P-R4HIQ") - makes it easier to read/type
// a long random code without errors. If the length isn't divisible by 5,
// the last group is correspondingly shorter.
func FormatGroups(code string) string {
	const groupSize = 5
	if len(code) <= groupSize {
		return code
	}
	var groups []string
	for i := 0; i < len(code); i += groupSize {
		end := i + groupSize
		if end > len(code) {
			end = len(code)
		}
		groups = append(groups, code[i:end])
	}
	return strings.Join(groups, "-")
}

// LocalMAC returns the MAC address of the first network interface found
// with a real hardware address (loopback and interfaces without a MAC are
// skipped). Serves as a silent second factor when deriving a share
// password (see cmd/backend): an attacker who only knows the name intended
// for the recipient can't compute crypt themselves without knowing this
// MAC address.
func LocalMAC() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		return iface.HardwareAddr.String(), nil
	}
	return "", errors.New("no network interface with a MAC address found")
}
