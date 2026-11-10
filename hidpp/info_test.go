package hidpp

import "testing"

func TestModelText(t *testing.T) {
	cases := []struct {
		info []byte
		want string
	}{
		// LIGHTSPEED (bit 2) and USB (bit 3): IDs in bit order, shown USB first.
		{[]byte{1, 0, 0, 0, 0, 0x00, 0x0C, 0x40, 0x8E, 0xC3, 0x43, 0, 0}, "USB C343 · LIGHTSPEED 408E"},
		// USB only.
		{[]byte{1, 0, 0, 0, 0, 0x00, 0x08, 0xC0, 0x8B, 0, 0, 0, 0}, "USB C08B"},
		// Transport bits that do not match the IDs.
		{[]byte{1, 0, 0, 0, 0, 0x00, 0x08, 0xC0, 0x8B, 0x40, 0x79, 0, 0}, "C08B · 4079"},
	}
	for _, c := range cases {
		if got := ModelText(c.info); got != c.want {
			t.Errorf("ModelText(% x) = %q, want %q", c.info, got, c.want)
		}
	}
}

func TestPercentFromMillivolts(t *testing.T) {
	for mv, want := range map[int]int{3400: 0, 3500: 0, 3800: 50, 4100: 100, 4200: 100} {
		if got := percentFromMillivolts(mv); got != want {
			t.Errorf("percentFromMillivolts(%d) = %d, want %d", mv, got, want)
		}
	}
}
