package lighting

import (
	"bytes"
	"reflect"
	"testing"
)

func TestCRC16(t *testing.T) {
	// Standard check value for CRC-16/CCITT-FALSE.
	if got := crc16([]byte("123456789")); got != 0x29B1 {
		t.Fatalf("crc16 = %04x, want 29b1", got)
	}
	data := make([]byte, 255)
	for i := range data {
		data[i] = byte(i)
	}
	sealSector(data)
	if !validSector(data) {
		t.Fatal("sealed sector is not valid")
	}
	data[10]++
	if validSector(data) {
		t.Fatal("corrupted sector is valid")
	}
}

func TestProfileSettings(t *testing.T) {
	// ROM profile 0x0101 of the G502 HERO.
	rom := []byte{0x01, 0x01, 0x00, 0xb0, 0x04, 0x60, 0x09, 0x80, 0x0c, 0x00, 0x19, 0x00, 0x00}
	s := decodeSettings(rom)
	want := ProfileSettings{ReportRate: 1000, DPI: []int{1200, 2400, 3200, 6400, 0}, DefaultDPI: 1, ShiftDPI: 0}
	if !reflect.DeepEqual(s, want) {
		t.Fatalf("decodeSettings = %+v, want %+v", s, want)
	}
	out := make([]byte, len(rom))
	encodeSettings(out, s)
	if !bytes.Equal(out, rom) {
		t.Fatalf("encodeSettings = % x, want % x", out, rom)
	}

	gap := ProfileSettings{ReportRate: 500, DPI: []int{800, 0, 1600, 0, 3200}, DefaultDPI: 2, ShiftDPI: 0}
	got := gap.compact()
	want = ProfileSettings{ReportRate: 500, DPI: []int{800, 1600, 3200, 0, 0}, DefaultDPI: 1, ShiftDPI: 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("compact = %+v, want %+v", got, want)
	}
}
