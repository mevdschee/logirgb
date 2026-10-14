package hidpp

import (
	"encoding/hex"
	"testing"
)

func TestHasLongReport(t *testing.T) {
	cases := []struct {
		name string
		desc string
		want bool
	}{
		// G515 TKL (046d:c358) HID++ interface, usage page 0xFF00.
		{"g515", "0600ff0901a101851095067508150026ff000901810009019100c00600ff0902a101851195137508150026ff000902810009029100c0", true},
		// G815 style collection on usage page 0xFF43 with reports 0x11 and 0x12.
		{"ff43", "0643ff0a0206a10185117508951315002600ff09028100090291 00c00643ff0a0406a1018512750895 3f15002600ff09048100090491 00c0", true},
		// Vendor page with only the short report.
		{"short only", "0600ff0901a101851095067508150026ff00090181000901910 0c0", false},
		// Keyboard boot interface.
		{"keyboard", "05010906a101050719e029e7150025017501950881 02c0", false},
	}
	for _, c := range cases {
		desc, err := hex.DecodeString(stripSpaces(c.desc))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := hasLongReport(desc); got != c.want {
			t.Errorf("%s: hasLongReport = %v, want %v", c.name, got, c.want)
		}
	}
}

func stripSpaces(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

func TestIsWebcam(t *testing.T) {
	if !isWebcam(0x0825) || isWebcam(0xC358) || isWebcam(0xC08B) {
		t.Error("webcam range check is wrong")
	}
}
