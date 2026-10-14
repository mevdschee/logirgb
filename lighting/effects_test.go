package lighting

import (
	"encoding/json"
	"testing"
)

func TestParseColor(t *testing.T) {
	for in, want := range map[string]int{
		"#ff8000":   0xFF8000,
		"00ff00":    0x00FF00,
		"0x0000FF":  0x0000FF,
		"255,0,128": 0xFF0080,
		"Red":       0xFF0000,
	} {
		got, err := ParseColor(in)
		if err != nil || got != want {
			t.Errorf("ParseColor(%q) = %06x, %v; want %06x", in, got, err, want)
		}
	}
	for _, in := range []string{"", "#fff", "256,0,0", "nope"} {
		if _, err := ParseColor(in); err == nil {
			t.Errorf("ParseColor(%q) succeeded, want error", in)
		}
	}
}

func TestEncode(t *testing.T) {
	cases := []struct {
		effect Effect
		id     uint16
		want   [10]byte
	}{
		{Effect{Name: "static", Values: map[Param]int{ParamColor: 0x112233}}, 0x01,
			[10]byte{0x11, 0x22, 0x33, 0x02}},
		{Effect{Name: "breathe", Values: map[Param]int{ParamColor: 0x0000FF, ParamPeriod: 3000}}, 0x0A,
			[10]byte{0x00, 0x00, 0xFF, 0x0B, 0xB8, 0x00, 100}},
		{Effect{Name: "wave", Values: map[Param]int{ParamDirection: 6}}, 0x16,
			[10]byte{0, 255, 0, 0, 0, 0, 0x13, 0x88, 100, 6}},
	}
	for _, c := range cases {
		if got := c.effect.encode(effectTypes[c.id]); got != c.want {
			t.Errorf("%s: encode = % x, want % x", c.effect.Name, got, c.want)
		}
	}
}

func TestParseChoice(t *testing.T) {
	if v, err := ParseChoice(ParamDirection, "left"); err != nil || v != 6 {
		t.Errorf("left = %d, %v", v, err)
	}
	if _, err := ParseChoice(ParamDirection, "sideways"); err == nil {
		t.Error("sideways accepted")
	}
}

func TestEffectJSON(t *testing.T) {
	in := Effect{Name: "breathe", Values: map[Param]int{ParamColor: 0xFF8000, ParamPeriod: 2500}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Effect
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != in.Name || out.Values[ParamColor] != 0xFF8000 || out.Values[ParamPeriod] != 2500 {
		t.Errorf("round trip: %s -> %+v", data, out)
	}
}
