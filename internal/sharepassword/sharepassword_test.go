package sharepassword

import (
	"strings"
	"testing"
)

func TestDeriveMatchesKnownVector(t *testing.T) {
	// Test vector confirmed via Python:
	//   hashlib.sha256(b"password") -> base32(...)[:16] == "L2EERGG2FACHCUOQ"
	// In groups of five: "L2EER-GG2FA-CHCUO-Q" (see FormatGroups).
	crypt, password := Derive("password", "password", 16)
	if crypt != "L2EER-GG2FA-CHCUO-Q" {
		t.Errorf("crypt = %q, want %q", crypt, "L2EER-GG2FA-CHCUO-Q")
	}
	if password != "passwordL2EER-GG2FA-CHCUO-Q" {
		t.Errorf("password = %q, want %q", password, "passwordL2EER-GG2FA-CHCUO-Q")
	}
}

func TestDeriveUsesDefaultLengthWhenZeroOrNegative(t *testing.T) {
	cryptZero, _ := Derive("password", "password", 0)
	cryptDefault, _ := Derive("password", "password", DefaultLength)
	if cryptZero != cryptDefault {
		t.Errorf("Derive(..., 0) = %q, want same as Derive(..., DefaultLength) = %q", cryptZero, cryptDefault)
	}
	cryptNegative, _ := Derive("password", "password", -1)
	if cryptNegative != cryptDefault {
		t.Errorf("Derive(..., -1) = %q, want same as Derive(..., DefaultLength) = %q", cryptNegative, cryptDefault)
	}
}

func TestDeriveIsDeterministic(t *testing.T) {
	crypt1, password1 := Derive("username", "username+hardening", 16)
	crypt2, password2 := Derive("username", "username+hardening", 16)
	if crypt1 != crypt2 || password1 != password2 {
		t.Errorf("Derive() not deterministic: (%q,%q) != (%q,%q)", crypt1, password1, crypt2, password2)
	}
}

func TestDifferentHardenedSeedsProduceDifferentCrypts(t *testing.T) {
	crypt1, _ := Derive("alice", "alice+seed1", 16)
	crypt2, _ := Derive("alice", "alice+seed2", 16)
	if crypt1 == crypt2 {
		t.Errorf("different hardenedSeed values produced the same crypt: %q", crypt1)
	}
}

func TestPasswordIsNamePlusCryptNotHardenedSeedPlusCrypt(t *testing.T) {
	// The recipient only knows name (verbally) and crypt (displayed) - the
	// actual password must therefore NOT contain the hardenedSeed (unknown
	// to the recipient), otherwise they could never type it correctly
	// (exactly this bug was observed live and fixed here).
	crypt, password := Derive("alice", "alice+mac-address+remote-path", 16)
	if password != "alice"+crypt {
		t.Errorf("password = %q, want name+crypt = %q", password, "alice"+crypt)
	}
}

func TestFormatGroupsSplitsIntoFives(t *testing.T) {
	got := FormatGroups("SJBB3ZMD7PR4HIQ")
	want := "SJBB3-ZMD7P-R4HIQ"
	if got != want {
		t.Errorf("FormatGroups() = %q, want %q", got, want)
	}
}

func TestFormatGroupsHandlesRemainder(t *testing.T) {
	got := FormatGroups("L2EERGG2FACHCUOQ") // 16 chars, not divisible by 5
	want := "L2EER-GG2FA-CHCUO-Q"
	if got != want {
		t.Errorf("FormatGroups() = %q, want %q", got, want)
	}
}

func TestFormatGroupsLeavesShortCodeUnchanged(t *testing.T) {
	got := FormatGroups("ABCDE")
	if got != "ABCDE" {
		t.Errorf("FormatGroups() = %q, want unchanged %q", got, "ABCDE")
	}
}

func TestLocalMACReturnsPlausibleAddressAndIsStable(t *testing.T) {
	mac1, err := LocalMAC()
	if err != nil {
		t.Fatalf("LocalMAC() error = %v (test environment without a network interface with a MAC?)", err)
	}
	if !strings.Contains(mac1, ":") {
		t.Errorf("LocalMAC() = %q, doesn't look like a MAC address", mac1)
	}

	mac2, err := LocalMAC()
	if err != nil {
		t.Fatalf("LocalMAC() (second call) error = %v", err)
	}
	if mac1 != mac2 {
		t.Errorf("LocalMAC() not stable: %q != %q", mac1, mac2)
	}
}
