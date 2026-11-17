package lighting

import (
	"bytes"
	"encoding/hex"
	"hash/crc32"
	"testing"
)

func h(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func TestDirLayout(t *testing.T) {
	// Factory directory of the G515 TKL: one entry.
	tkl := h("5e38092501018101406399" + "6dec0101ff")
	if n := pmDirLen(tkl); n != 16 {
		t.Fatalf("pmDirLen(tkl) = %d, want 16", n)
	}
	e := pmDirEntries(tkl)
	if len(e) != 1 || e[0][4] != 0x63 || e[0][9] != 0x01 {
		t.Fatalf("pmDirEntries(tkl) = % x", e)
	}
	if !validDir(tkl, true) || pmProfileEntry(tkl, 1) != 5 {
		t.Fatal("G515 TKL directory is not valid")
	}

	// Factory directory of the G515 Rapid TKL (issue #2): three 109 byte
	// profiles and a 0x1B08 analog key file. Its checksum does not match.
	rapid := h("1bc0c0db04018101406d3bd406de01028101406d9c0cec2502038101406d08a4aaf003041b0840be808221ee04ff")
	if n := pmDirLen(rapid[:16]); n != 46 {
		t.Fatalf("pmDirLen(rapid) = %d, want 46", n)
	}
	if len(pmDirEntries(rapid)) != 4 || !validDir(rapid, false) || validDir(rapid, true) {
		t.Fatal("G515 Rapid TKL factory directory not recognized")
	}
	for id, want := range map[byte]int{1: 5, 2: 15, 3: 25, 4: -1, 5: -1} {
		if got := pmProfileEntry(rapid, id); got != want {
			t.Errorf("pmProfileEntry(rapid, %d) = %d, want %d", id, got, want)
		}
	}
	if validDir(rapid[:16], false) {
		t.Fatal("truncated directory must not be valid")
	}
}

func TestRapidUserDir(t *testing.T) {
	// User directory and user profile 1 of a G515 Rapid TKL written by G
	// HUB (issue #2).
	dir := h("2256be0a05018101406d7f7cfaab08028101406db45b723b09038101406d483140d20a041b0840be89c47aa00b051b0800ce7aa0caa10dff")
	prof := h("0001ffffffff00500052004f00460049004c0045005f004e0041004d0000000000000000000000000001ffffff0200000000000000000000000000000000000000000000000000000000000000000000000000080100000000000014000005ffffffffffffffffffffffffffff")
	if !validDir(dir, true) {
		t.Fatal("user directory checksum does not validate")
	}
	if !validProfile(prof, 109) || validProfile(prof, 99) {
		t.Fatal("user profile not recognized")
	}
	off := pmProfileEntry(dir, 1)
	if off != 5 || dir[off+9] != 0x08 || crc32.ChecksumIEEE(prof) != be32(dir[off+5:]) {
		t.Fatalf("profile 1 entry at %d does not describe the profile", off)
	}
	if got := pmDirWithFile(dir, off, prof); !bytes.Equal(got, dir) {
		t.Fatalf("rebuilding the unchanged directory gives % x", got)
	}

	p := bytes.Clone(prof)
	p[42] = 0x00 // white to cyan
	got := pmDirWithFile(dir, off, p)
	if !validDir(got, true) || be32(got[off+5:]) != crc32.ChecksumIEEE(p) {
		t.Fatal("updated directory does not validate")
	}
	// Only the checksums change; the other entries are kept.
	if !bytes.Equal(got[4:off+5], dir[4:off+5]) || !bytes.Equal(got[off+9:], dir[off+9:]) {
		t.Fatalf("updated directory changed other bytes: % x", got)
	}
}
