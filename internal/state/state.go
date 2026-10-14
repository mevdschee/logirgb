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
	// Kept marks the zones per onboard profile (1-based) whose stored
	// effect could not be read; they keep it when written until set.
	Kept map[string]map[string]bool
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

// Effects are cloned on the way in and out, so an effect shared by several
// profiles cannot be changed for all of them through one.

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

// ProfileZone returns the effect for a zone in an onboard profile: the
// profile's own effect if it has one, else the effect for all profiles. A
// kept zone has no effect.
func (d *Device) ProfileZone(profile, i int) (lighting.Effect, bool) {
	p, z := strconv.Itoa(profile), strconv.Itoa(i)
	if e, ok := d.Profiles[p][z]; ok {
		return e.Clone(), true
	}
	if d.Kept[p][z] {
		return lighting.Effect{}, false
	}
	return d.Zone(i)
}

// KeepProfileZone marks a zone in an onboard profile as keeping its stored
// effect, for an effect that could not be read.
func (d *Device) KeepProfileZone(profile, i int) {
	if d.Kept == nil {
		d.Kept = map[string]map[string]bool{}
	}
	p := strconv.Itoa(profile)
	if d.Kept[p] == nil {
		d.Kept[p] = map[string]bool{}
	}
	d.Kept[p][strconv.Itoa(i)] = true
}

func (d *Device) unkeep(p, z string) {
	delete(d.Kept[p], z)
	if len(d.Kept[p]) == 0 {
		delete(d.Kept, p)
	}
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
	d.unkeep(p, strconv.Itoa(i))
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
// dropping the per profile and kept effects for that zone.
func (d *Device) SetAllProfilesZone(i int, e lighting.Effect) {
	d.SetZone(i, e)
	z := strconv.Itoa(i)
	for p, zones := range d.Profiles {
		delete(zones, z)
		if len(zones) == 0 {
			delete(d.Profiles, p)
		}
	}
	for p := range d.Kept {
		d.unkeep(p, z)
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
