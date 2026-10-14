package lighting

import "testing"

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
