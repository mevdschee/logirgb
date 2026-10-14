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
	Name  string
	Zones map[string]lighting.Effect
	// Profiles overrides the zone effects per onboard profile (1-based),
	// for devices that store lighting in each profile (G502).
	Profiles map[string]map[string]lighting.Effect
	// ProfileSettings holds the report rate and DPI levels per onboard
	// profile (1-based); profiles without an entry keep what they store.
	ProfileSettings map[string]*lighting.ProfileSettings
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

// Zone returns the stored effect for a zone.
func (d *Device) Zone(i int) (lighting.Effect, bool) {
	e, ok := d.Zones[strconv.Itoa(i)]
	return e, ok
}

// SetZone stores the effect for a zone.
func (d *Device) SetZone(i int, e lighting.Effect) {
	if d.Zones == nil {
		d.Zones = map[string]lighting.Effect{}
	}
	d.Zones[strconv.Itoa(i)] = e
}

// ProfileZone returns the effect for a zone in an onboard profile: the
// profile's own effect if it has one, else the effect for all profiles.
func (d *Device) ProfileZone(profile, i int) (lighting.Effect, bool) {
	if e, ok := d.Profiles[strconv.Itoa(profile)][strconv.Itoa(i)]; ok {
		return e, true
	}
	return d.Zone(i)
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
	d.Profiles[p][strconv.Itoa(i)] = e
}

// ClearProfileZone drops a profile's own effect for a zone, so it uses the
// effect for all profiles again.
func (d *Device) ClearProfileZone(profile, i int) {
	p := strconv.Itoa(profile)
	delete(d.Profiles[p], strconv.Itoa(i))
	if len(d.Profiles[p]) == 0 {
		delete(d.Profiles, p)
	}
}

// SetAllProfilesZone stores the effect for a zone in every onboard profile,
// dropping the per profile effects for that zone.
func (d *Device) SetAllProfilesZone(i int, e lighting.Effect) {
	d.SetZone(i, e)
	z := strconv.Itoa(i)
	for p, zones := range d.Profiles {
		delete(zones, z)
		if len(zones) == 0 {
			delete(d.Profiles, p)
		}
	}
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

// ProfileNumbers returns the onboard profiles with their own effects, in
// order.
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
