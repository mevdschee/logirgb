package lighting

import (
	"bytes"
	"testing"
)

func TestDirLayout(t *testing.T) {
	// Factory directory of the G515 TKL: one entry.
	tkl := []byte{0x5e, 0x38, 0x09, 0x25, 0x01, 0x01, 0x81, 0x01, 0x40, 0x63, 0x99, 0x6d, 0xec, 0x01, 0x01, 0xff}
	if n := pmDirLen(tkl); n != 16 {
		t.Fatalf("pmDirLen(tkl) = %d, want 16", n)
	}
	e := pmDirEntries(tkl)
	if len(e) != 1 || e[0][4] != 0x63 || e[0][9] != 0x01 {
		t.Fatalf("pmDirEntries(tkl) = % x", e)
	}
	if !validDir(tkl) {
		t.Fatal("G515 TKL directory is not valid")
	}

	// First 16 bytes of the factory directory of the G515 Rapid TKL (issue
	// #2): four entries, the first a 109 byte profile at slot 1.
	rapid := []byte{0x1b, 0xc0, 0xc0, 0xdb, 0x04, 0x01, 0x81, 0x01, 0x40, 0x6d, 0x3b, 0xd4, 0x06, 0xde, 0x01, 0x02}
	if n := pmDirLen(rapid); n != 46 {
		t.Fatalf("pmDirLen(rapid) = %d, want 46", n)
	}
	e = pmDirEntries(rapid)
	if len(e) != 1 || !bytes.Equal(e[0][1:5], []byte{0x81, 0x01, 0x40, 0x6d}) || e[0][9] != 0x01 {
		t.Fatalf("pmDirEntries(rapid) = % x", e)
	}
	if validDir(rapid) {
		t.Fatal("G515 Rapid TKL directory must not pass the G515 TKL check")
	}
}
