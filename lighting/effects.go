// Package lighting drives the RGB features of HID++ 2.0 devices:
// 0x8070 ColorLEDEffects (e.g. G502 HERO), 0x8071 RGBEffects (e.g. G515),
// 0x8040 BrightnessControl and the 0x8071 idle/sleep timers.
package lighting

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Param identifies an effect parameter.
type Param string

const (
	ParamColor      Param = "color"
	ParamColor2     Param = "color2"
	ParamSpeed      Param = "speed"
	ParamPeriod     Param = "period"
	ParamIntensity  Param = "intensity"
	ParamSaturation Param = "saturation"
	ParamDirection  Param = "direction"
	ParamForm       Param = "form"
)

// ParamSpec describes how to edit a parameter.
type ParamSpec struct {
	Name    Param
	Label   string
	Min     int
	Max     int
	Default int
	Step    int
	Unit    string
	Choices []string // for enumerations; value is the index
}

var paramSpecs = map[Param]ParamSpec{
	ParamColor:      {Name: ParamColor, Label: "Color", Min: 0, Max: 0xFFFFFF, Default: 0xFFFFFF},
	ParamColor2:     {Name: ParamColor2, Label: "Star color", Min: 0, Max: 0xFFFFFF, Default: 0xFFFFFF},
	ParamSpeed:      {Name: ParamSpeed, Label: "Speed", Min: 0, Max: 255, Default: 100, Step: 5},
	ParamPeriod:     {Name: ParamPeriod, Label: "Period", Min: 1000, Max: 20000, Default: 5000, Step: 500, Unit: "ms"},
	ParamIntensity:  {Name: ParamIntensity, Label: "Intensity", Min: 1, Max: 100, Default: 100, Step: 5, Unit: "%"},
	ParamSaturation: {Name: ParamSaturation, Label: "Saturation", Min: 0, Max: 255, Default: 255, Step: 5},
	ParamDirection: {Name: ParamDirection, Label: "Direction", Min: 0, Max: 8, Default: 1,
		Choices: []string{"cycle", "right", "down", "center-out", "in", "out", "left", "up", "center-in"}},
	ParamForm: {Name: ParamForm, Label: "Waveform", Min: 0, Max: 6, Default: 0,
		Choices: []string{"default", "sine", "square", "triangle", "sawtooth", "shark-fin", "exponential"}},
}

type paramSlot struct {
	param  Param
	offset int
	size   int
}

// EffectType is a known effect ID with its parameter layout inside the 10
// byte parameter block of SetZoneEffect.
type EffectType struct {
	ID    uint16
	Name  string
	slots []paramSlot
	// fixed holds bytes the firmware expects at a position regardless of
	// user input, keyed by offset.
	fixed map[int]byte
	// overrides adjusts parameter ranges for this effect.
	overrides map[Param]ParamSpec
}

// Layouts follow Solaar's LEDEffects table, which was derived from G HUB
// captures.
var effectTypes = map[uint16]EffectType{
	0x00: {ID: 0x00, Name: "off"},
	0x01: {ID: 0x01, Name: "static", slots: []paramSlot{{ParamColor, 0, 3}}, fixed: map[int]byte{3: 0x02}},
	0x02: {ID: 0x02, Name: "pulse", slots: []paramSlot{{ParamColor, 0, 3}, {ParamSpeed, 3, 1}}},
	0x03: {ID: 0x03, Name: "cycle", slots: []paramSlot{{ParamPeriod, 5, 2}, {ParamIntensity, 7, 1}}},
	0x04: {ID: 0x04, Name: "wave", slots: []paramSlot{{ParamPeriod, 6, 2}, {ParamDirection, 9, 1}}},
	0x05: {ID: 0x05, Name: "starlight", slots: []paramSlot{{ParamColor, 0, 3}, {ParamColor2, 3, 3}}},
	0x0A: {ID: 0x0A, Name: "breathe", slots: []paramSlot{{ParamColor, 0, 3}, {ParamPeriod, 3, 2}, {ParamForm, 5, 1}, {ParamIntensity, 6, 1}}},
	0x0B: {ID: 0x0B, Name: "ripple", slots: []paramSlot{{ParamColor, 0, 3}, {ParamPeriod, 4, 2}},
		overrides: map[Param]ParamSpec{ParamPeriod: {Min: 2, Max: 200, Default: 20, Step: 2}}},
	0x15: {ID: 0x15, Name: "cycle", slots: []paramSlot{{ParamSaturation, 1, 1}, {ParamPeriod, 6, 2}, {ParamIntensity, 8, 1}}},
	0x16: {ID: 0x16, Name: "wave", slots: []paramSlot{{ParamSaturation, 1, 1}, {ParamPeriod, 6, 2}, {ParamIntensity, 8, 1}, {ParamDirection, 9, 1}}},
	0x17: {ID: 0x17, Name: "ripple", slots: []paramSlot{{ParamColor, 0, 3}, {ParamSaturation, 3, 1}, {ParamPeriod, 6, 2}},
		overrides: map[Param]ParamSpec{ParamPeriod: {Min: 2, Max: 200, Default: 20, Step: 2}}},
}

// IsColor reports whether the parameter holds an RGB color.
func (p Param) IsColor() bool { return p == ParamColor || p == ParamColor2 }

func (t EffectType) without(p Param) EffectType {
	var slots []paramSlot
	for _, s := range t.slots {
		if s.param != p {
			slots = append(slots, s)
		}
	}
	t.slots = slots
	return t
}

// Params returns the editable parameters of the effect.
func (t EffectType) Params() []ParamSpec {
	out := make([]ParamSpec, 0, len(t.slots))
	for _, s := range t.slots {
		spec := paramSpecs[s.param]
		if o, ok := t.overrides[s.param]; ok {
			spec.Min, spec.Max, spec.Default, spec.Step = o.Min, o.Max, o.Default, o.Step
		}
		out = append(out, spec)
	}
	return out
}

// Effect is an effect choice plus parameter values, as stored in the config.
type Effect struct {
	Name   string        `json:"effect"`
	Values map[Param]int `json:"params,omitempty"`
}

// Get returns the value of p, or the default of the given type.
func (e Effect) Get(t EffectType, p Param) int {
	if v, ok := e.Values[p]; ok {
		return v
	}
	for _, spec := range t.Params() {
		if spec.Name == p {
			return spec.Default
		}
	}
	return 0
}

// Set stores a parameter value.
func (e *Effect) Set(p Param, v int) {
	if e.Values == nil {
		e.Values = map[Param]int{}
	}
	e.Values[p] = v
}

// encode builds the 10 byte parameter block for SetZoneEffect.
func (e Effect) encode(t EffectType) [10]byte {
	var b [10]byte
	for off, v := range t.fixed {
		b[off] = v
	}
	for _, s := range t.slots {
		v := e.Get(t, s.param)
		for i := 0; i < s.size; i++ {
			b[s.offset+i] = byte(v >> (8 * (s.size - 1 - i)))
		}
	}
	return b
}

// ParseColor accepts "#rrggbb", "rrggbb", "r,g,b" or a few color names.
func ParseColor(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if v, ok := namedColors[s]; ok {
		return v, nil
	}
	if parts := strings.Split(s, ","); len(parts) == 3 {
		var r, g, b int
		if _, err := fmt.Sscanf(s, "%d,%d,%d", &r, &g, &b); err != nil {
			return 0, fmt.Errorf("invalid color %q", s)
		}
		for _, c := range []int{r, g, b} {
			if c < 0 || c > 255 {
				return 0, fmt.Errorf("invalid color %q: components must be 0-255", s)
			}
		}
		return r<<16 | g<<8 | b, nil
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "#"), "0x")
	var v int
	if len(s) != 6 {
		return 0, fmt.Errorf("invalid color %q", s)
	}
	if _, err := fmt.Sscanf(s, "%06x", &v); err != nil {
		return 0, fmt.Errorf("invalid color %q", s)
	}
	return v, nil
}

// FormatColor renders a color as #rrggbb.
func FormatColor(v int) string {
	return fmt.Sprintf("#%06x", v&0xFFFFFF)
}

var namedColors = map[string]int{
	"white":   0xFFFFFF,
	"black":   0x000000,
	"red":     0xFF0000,
	"green":   0x00FF00,
	"blue":    0x0000FF,
	"yellow":  0xFFFF00,
	"cyan":    0x00FFFF,
	"magenta": 0xFF00FF,
	"orange":  0xFF8000,
	"purple":  0x8000FF,
	"pink":    0xFF4080,
}

// ParseChoice maps a choice name (or number) of an enumerated parameter to
// its value.
func ParseChoice(p Param, s string) (int, error) {
	spec := paramSpecs[p]
	s = strings.ToLower(strings.TrimSpace(s))
	for i, c := range spec.Choices {
		if c == s {
			return i, nil
		}
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err == nil && v >= spec.Min && v <= spec.Max {
		return v, nil
	}
	return 0, fmt.Errorf("invalid %s %q (choose from %s)", p, s, strings.Join(spec.Choices, ", "))
}

// Format renders a parameter value for display.
func (p ParamSpec) Format(v int) string {
	switch {
	case p.Name.IsColor():
		return FormatColor(v)
	case p.Choices != nil && v >= 0 && v < len(p.Choices):
		return p.Choices[v]
	}
	return fmt.Sprintf("%d%s", v, p.Unit)
}

// MarshalJSON writes colors as "#rrggbb" for a readable config file.
func (e Effect) MarshalJSON() ([]byte, error) {
	out := struct {
		Name   string         `json:"effect"`
		Params map[string]any `json:"params,omitempty"`
	}{Name: e.Name}
	for p, v := range e.Values {
		if out.Params == nil {
			out.Params = map[string]any{}
		}
		if p.IsColor() {
			out.Params[string(p)] = FormatColor(v)
		} else {
			out.Params[string(p)] = v
		}
	}
	return json.Marshal(out)
}

// UnmarshalJSON accepts colors as "#rrggbb" strings or plain numbers.
func (e *Effect) UnmarshalJSON(data []byte) error {
	var in struct {
		Name   string                     `json:"effect"`
		Params map[string]json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	*e = Effect{Name: in.Name}
	for k, raw := range in.Params {
		var n int
		if err := json.Unmarshal(raw, &n); err == nil {
			e.Set(Param(k), n)
			continue
		}
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return fmt.Errorf("param %s: %w", k, err)
		}
		v, err := ParseColor(str)
		if err != nil {
			return fmt.Errorf("param %s: %w", k, err)
		}
		e.Set(Param(k), v)
	}
	return nil
}
