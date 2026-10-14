package lighting

import (
	"errors"
	"fmt"
	"slices"
)

const (
	featureAdjustableDPI = 0x2201
	featureReportRate    = 0x8060
	dpiSlots             = 5
)

// ProfileSettings are the pointer settings of an onboard profile (0x8100
// profile format 2, see research/hardware-log.md): the report rate, up to
// five DPI levels and the level the profile starts on and the one the DPI
// shift (sniper) button switches to.
type ProfileSettings struct {
	ReportRate int   `json:"report_rate"` // Hz
	DPI        []int `json:"dpi"`         // dpiSlots levels, 0 for unused
	DefaultDPI int   `json:"default_dpi"` // index into DPI
	ShiftDPI   int   `json:"shift_dpi"`   // index into DPI
}

// Levels returns the number of DPI levels in use.
func (s ProfileSettings) Levels() int {
	n := 0
	for _, v := range s.DPI {
		if v > 0 {
			n++
		}
	}
	return n
}

// compact moves the unused DPI slots to the end, as the firmware expects,
// and keeps the default and shift indexes on the same levels.
func (s ProfileSettings) compact() ProfileSettings {
	out := s
	out.DPI = make([]int, 0, dpiSlots)
	for i, v := range s.DPI {
		if v == 0 {
			continue
		}
		if i == s.DefaultDPI {
			out.DefaultDPI = len(out.DPI)
		}
		if i == s.ShiftDPI {
			out.ShiftDPI = len(out.DPI)
		}
		out.DPI = append(out.DPI, v)
	}
	for len(out.DPI) < dpiSlots {
		out.DPI = append(out.DPI, 0)
	}
	return out
}

// DPIRange describes the DPI values the sensor accepts.
type DPIRange struct {
	Min, Max, Step int
}

// HasProfileSettings reports whether the onboard profiles hold DPI and
// report rate settings this package can edit.
func (c *Controller) HasProfileSettings() bool {
	return c.HasOnboardProfiles()
}

// DPIRange returns the sensor's DPI range from 0x2201 AdjustableDPI, or the
// G502 HERO range when the device does not report one.
func (c *Controller) DPIRange() DPIRange {
	def := DPIRange{Min: 100, Max: 25600, Step: 50}
	r, err := c.dev.Call(featureAdjustableDPI, 1, 0)
	if err != nil {
		return def
	}
	// The list is big endian values after the sensor index; a value with
	// the top three bits set is the step between the values around it.
	var vals []int
	step := 0
	for i := 1; i+1 < len(r); i += 2 {
		v := int(r[i])<<8 | int(r[i+1])
		if v == 0 {
			break
		}
		if v >= 0xE000 {
			step = v & 0x1FFF
			continue
		}
		vals = append(vals, v)
	}
	if len(vals) < 2 || step == 0 {
		return def
	}
	return DPIRange{Min: vals[0], Max: vals[len(vals)-1], Step: step}
}

// ReportRates returns the supported report rates in Hz, fastest first, from
// 0x8060 ReportRate, or 1000, 500, 250 and 125 Hz when it is not available.
func (c *Controller) ReportRates() []int {
	r, err := c.dev.Call(featureReportRate, 0)
	if err != nil || r[0] == 0 {
		return []int{1000, 500, 250, 125}
	}
	// Bit n set means a report every n+1 ms is supported.
	var rates []int
	for n := 0; n < 8; n++ {
		if r[0]&(1<<n) != 0 {
			rates = append(rates, 1000/(n+1))
		}
	}
	return rates
}

func decodeSettings(p []byte) ProfileSettings {
	s := ProfileSettings{DefaultDPI: int(p[1]), ShiftDPI: int(p[2])}
	if p[0] > 0 {
		s.ReportRate = 1000 / int(p[0])
	}
	for i := 0; i < dpiSlots; i++ {
		v := int(p[3+2*i]) | int(p[4+2*i])<<8
		if v == 0xFFFF {
			v = 0
		}
		s.DPI = append(s.DPI, v)
	}
	return s
}

// validate checks the settings against the device before they are written.
func (c *Controller) validate(s ProfileSettings) error {
	if !slices.Contains(c.ReportRates(), s.ReportRate) {
		return fmt.Errorf("report rate %d Hz is not supported (supported: %v)", s.ReportRate, c.ReportRates())
	}
	if len(s.DPI) != dpiSlots {
		return fmt.Errorf("expected %d DPI levels, got %d", dpiSlots, len(s.DPI))
	}
	rng := c.DPIRange()
	for _, v := range s.DPI {
		if v != 0 && (v < rng.Min || v > rng.Max) {
			return fmt.Errorf("DPI %d is outside %d-%d", v, rng.Min, rng.Max)
		}
	}
	n := s.Levels()
	if n == 0 {
		return errors.New("at least one DPI level is needed")
	}
	if s.DefaultDPI < 0 || s.DefaultDPI >= n || s.ShiftDPI < 0 || s.ShiftDPI >= n {
		return fmt.Errorf("default and shift DPI must be one of the %d levels", n)
	}
	return nil
}

func encodeSettings(p []byte, s ProfileSettings) {
	p[0] = byte(1000 / s.ReportRate)
	p[1], p[2] = byte(s.DefaultDPI), byte(s.ShiftDPI)
	for i, v := range s.DPI {
		p[3+2*i], p[4+2*i] = byte(v), byte(v>>8)
	}
}
