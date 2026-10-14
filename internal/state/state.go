// Package state holds the settings of a device as read from it and edited
// before they are written back. Nothing is kept on disk: the device is the
// source of truth.
package state

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/mevdschee/logirgb/lighting"
)

// Device holds the settings of one device.
type Device struct {
	Name string
	// Zones holds the zone effects of a device without onboard profiles,
	// or with a single one (G515).
	Zones map[string]lighting.Effect
	// Profiles holds the zone effects per onboard profile (1-based), for
	// devices that store lighting in each profile (G502). A zone without
	// an effect keeps what the profile stores.
	Profiles map[string]map[string]lighting.Effect
	// ProfileSettings holds the report rate and DPI levels per onboard
	// profile (1-based); profiles without an entry keep what they store.
	ProfileSettings map[string]*lighting.ProfileSettings
	// Disabled marks the onboard profiles (1-based) that the profile
	// button skips.
	Disabled map[string]bool
	// Brightness in device units (G515: 0-100); nil leaves it alone.
	Brightness *int
	// IdleTimeout in seconds before the lighting dims; 0 disables dimming.
	IdleTimeout *int
	// SleepTimeout in seconds of inactivity before the lighting turns off;
	// 0 disables it.
	SleepTimeout *int
	// DimPercent is the brightness of the dimmed lighting relative to the
	// normal lighting (keyboards with profile storage).
	DimPercent *int
}

// Effects are cloned on the way in and out, so changing an effect that was
// handed out does not change the stored one.

// Zone returns the stored effect for a zone.
func (d *Device) Zone(i int) (lighting.Effect, bool) {
	e, ok := d.Zones[strconv.Itoa(i)]
	return e.Clone(), ok
}

// SetZone stores the effect for a zone.
func (d *Device) SetZone(i int, e lighting.Effect) {
	if d.Zones == nil {
		d.Zones = map[string]lighting.Effect{}
	}
	d.Zones[strconv.Itoa(i)] = e.Clone()
}

// ProfileZone returns the effect for a zone in an onboard profile, or the
// zone effect for profile 0.
func (d *Device) ProfileZone(profile, i int) (lighting.Effect, bool) {
	if profile == 0 {
		return d.Zone(i)
	}
	e, ok := d.Profiles[strconv.Itoa(profile)][strconv.Itoa(i)]
	return e.Clone(), ok
}

// SetProfileZone stores the effect for a zone in one onboard profile.
func (d *Device) SetProfileZone(profile, i int, e lighting.Effect) {
	if d.Profiles == nil {
		d.Profiles = map[string]map[string]lighting.Effect{}
	}
	p := strconv.Itoa(profile)
	if d.Profiles[p] == nil {
		d.Profiles[p] = map[string]lighting.Effect{}
	}
	d.Profiles[p][strconv.Itoa(i)] = e.Clone()
}

// Settings returns the report rate and DPI settings of an onboard profile,
// or nil.
func (d *Device) Settings(profile int) *lighting.ProfileSettings {
	return d.ProfileSettings[strconv.Itoa(profile)]
}

// SetSettings stores the report rate and DPI settings of an onboard
// profile.
func (d *Device) SetSettings(profile int, s lighting.ProfileSettings) {
	if d.ProfileSettings == nil {
		d.ProfileSettings = map[string]*lighting.ProfileSettings{}
	}
	s.DPI = slices.Clone(s.DPI)
	d.ProfileSettings[strconv.Itoa(profile)] = &s
}

// Enabled reports whether the profile button cycles to an onboard profile.
func (d *Device) Enabled(profile int) bool {
	return !d.Disabled[strconv.Itoa(profile)]
}

// SetEnabled sets whether the profile button cycles to an onboard profile.
func (d *Device) SetEnabled(profile int, enabled bool) {
	p := strconv.Itoa(profile)
	if enabled {
		delete(d.Disabled, p)
		return
	}
	if d.Disabled == nil {
		d.Disabled = map[string]bool{}
	}
	d.Disabled[p] = true
}

// ProfileNumbers returns the onboard profiles with effects, in order.
func (d *Device) ProfileNumbers() []int {
	var out []int
	for p := range d.Profiles {
		if n, err := strconv.Atoi(p); err == nil {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}

// Key identifies a device model, e.g. "046d:c358".
func Key(vendor, product uint16) string {
	return fmt.Sprintf("%04x:%04x", vendor, product)
}

// IntPtr is a small helper for the optional fields.
func IntPtr(v int) *int { return &v }
