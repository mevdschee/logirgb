package app

import (
	"fmt"

	"github.com/mevdschee/logirgb/internal/state"
	"github.com/mevdschee/logirgb/lighting"
)

// Load reads the settings of a device: what its onboard memory stores and,
// for devices without onboard memory, the current zone effects where the
// device reports them.
func Load(e *Entry) (*state.Device, error) {
	d := &state.Device{Name: e.Name}
	if e.Ctl.HasBrightness() {
		if v, err := e.Ctl.Brightness(); err == nil {
			d.Brightness = state.IntPtr(v)
		}
	}
	if e.Ctl.HasTimers() {
		if idle, sleep, err := e.Ctl.Timers(); err == nil {
			d.IdleTimeout, d.SleepTimeout = state.IntPtr(idle), state.IntPtr(sleep)
		}
	}
	if !e.Ctl.HasOnboardProfiles() && !e.Ctl.HasProfileStorage() {
		for i := range e.Ctl.Zones() {
			if eff, ok := e.Ctl.Effect(i); ok {
				d.SetZone(i, eff)
			}
		}
		return d, nil
	}
	return d, loadOnboard(e, d)
}

// ApplyTimers writes the idle (dim) and sleep (off) timeouts to firmware.
func ApplyTimers(e *Entry, d *state.Device) error {
	if !e.Ctl.HasTimers() || (d.IdleTimeout == nil && d.SleepTimeout == nil) {
		return nil
	}
	idle, sleep, err := e.Ctl.Timers()
	if err != nil {
		return fmt.Errorf("read timers: %w", err)
	}
	if d.IdleTimeout != nil {
		idle = *d.IdleTimeout
	}
	if d.SleepTimeout != nil {
		sleep = *d.SleepTimeout
	}
	if err := e.Ctl.SetTimers(idle, sleep); err != nil {
		return fmt.Errorf("write timers: %w", err)
	}
	return nil
}

// DefaultDimPercent is used when no dim level was configured.
const DefaultDimPercent = 50

// SaveToProfiles stores the configured lighting in the device's onboard
// memory. For keyboards with profile storage it also stores the idle/sleep
// timeouts and hands the lighting back to the firmware, which then dims and
// switches off by itself. It returns the number of profiles written.
func SaveToProfiles(e *Entry, d *state.Device) (int, error) {
	effects := profileEffects(e, d, 0)
	if len(effects) == 0 && len(d.Profiles) == 0 {
		return 0, fmt.Errorf("no effects configured yet, set one first")
	}
	if !e.Ctl.HasProfileStorage() {
		count, err := e.Ctl.ProfileCount()
		if err != nil {
			return 0, err
		}
		profiles := make([]lighting.Profile, count)
		for p := range profiles {
			enabled := d.Enabled(p + 1)
			profiles[p] = lighting.Profile{Effects: profileEffects(e, d, p+1), Settings: d.Settings(p + 1), Enabled: &enabled}
		}
		n, err := e.Ctl.SaveToProfiles(profiles)
		if err != nil {
			return n, err
		}
		// The profiles cover profile switches; the persistent zone effect
		// covers power-up, when the mouse shows its stored zone effect.
		// Use the active profile's lighting, which the mouse runs again
		// after power-up.
		if active, err := e.Ctl.ActiveProfile(); err == nil && active >= 1 && active <= count {
			effects = profiles[active-1].Effects
		}
		for zone, eff := range effects {
			if err := e.Ctl.SetEffectPersistent(zone, eff); err != nil {
				return n, fmt.Errorf("zone %d: %w", zone, err)
			}
		}
		return n, nil
	}
	eff, ok := effects[0]
	if !ok {
		return 0, fmt.Errorf("no effect configured for zone 0")
	}
	idle, sleep, err := e.Ctl.Timers()
	if err != nil {
		return 0, err
	}
	if d.IdleTimeout != nil {
		idle = *d.IdleTimeout
	}
	if d.SleepTimeout != nil {
		sleep = *d.SleepTimeout
	}
	dim := DefaultDimPercent
	if d.DimPercent != nil {
		dim = *d.DimPercent
	}
	if err := e.Ctl.StoreProfile(eff, dim, idle, sleep); err != nil {
		return 0, err
	}
	// The stored timers only load on power-up; set the running ones too.
	return 1, e.Ctl.SetTimers(idle, sleep)
}

// loadOnboard takes the settings stored in the device's onboard memory
// into d, so they show and are written back as stored. On a keyboard with
// profile storage that is the effect, dim level and timeouts. On a mouse
// with onboard profiles it is each profile's zone effects, report rate, DPI
// levels and enabled flag; a zone whose effect cannot be read is left out,
// so the profile keeps it.
func loadOnboard(e *Entry, d *state.Device) error {
	if e.Ctl.HasProfileStorage() {
		sp, ok, err := e.Ctl.ReadProfile()
		if err != nil || !ok {
			return err
		}
		d.SetZone(0, sp.Effect)
		if sp.DimPercent > 0 {
			d.DimPercent = state.IntPtr(sp.DimPercent)
		}
		d.IdleTimeout = state.IntPtr(sp.Idle)
		d.SleepTimeout = state.IntPtr(sp.Sleep)
		return nil
	}
	if !e.Ctl.HasOnboardProfiles() {
		return nil
	}
	profiles, err := e.Ctl.ReadProfiles()
	if err != nil {
		return err
	}
	for p, prof := range profiles {
		for i, eff := range prof.Effects {
			d.SetProfileZone(p+1, i, eff)
		}
	}
	for p, prof := range profiles {
		d.SetSettings(p+1, *prof.Settings)
		d.SetEnabled(p+1, *prof.Enabled)
	}
	return nil
}

// profileEffects returns the configured zone effects of an onboard profile
// (1-based), or of all profiles for profile 0.
func profileEffects(e *Entry, d *state.Device, profile int) map[int]lighting.Effect {
	effects := map[int]lighting.Effect{}
	for i := range e.Ctl.Zones() {
		if eff, ok := d.ProfileZone(profile, i); ok {
			effects[i] = eff
		}
	}
	return effects
}
