package app

import (
	"fmt"

	"github.com/mevdschee/logitech-rgb/internal/config"
	"github.com/mevdschee/logitech-rgb/lighting"
)

// Apply pushes the stored settings of a device to the hardware.
func Apply(e *Entry, d *config.Device) error {
	if err := e.Ctl.Claim(); err != nil {
		return fmt.Errorf("claim lighting: %w", err)
	}
	if d.Brightness != nil && e.Ctl.HasBrightness() {
		if err := e.Ctl.SetBrightness(*d.Brightness); err != nil {
			return fmt.Errorf("brightness: %w", err)
		}
	}
	for i := range e.Ctl.Zones() {
		eff, ok := d.Zone(i)
		if !ok {
			continue
		}
		if err := e.Ctl.SetEffect(i, eff); err != nil {
			return fmt.Errorf("zone %d: %w", i, err)
		}
	}
	return ApplyTimers(e, d)
}

// ApplyTimers writes the idle/sleep timeouts to firmware, which uses the
// idle timeout to decide when to report the user as idle.
func ApplyTimers(e *Entry, d *config.Device) error {
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
	// The firmware only reports idleness once, at its idle timeout. When
	// dimming is disabled that report has to come at the sleep timeout.
	if idle == 0 {
		idle = sleep
	}
	if err := e.Ctl.SetTimers(idle, sleep); err != nil {
		return fmt.Errorf("write timers: %w", err)
	}
	return nil
}

// SaveToProfiles stores the configured zone effects in the onboard profiles.
func SaveToProfiles(e *Entry, d *config.Device) (int, error) {
	effects := map[int]lighting.Effect{}
	for i := range e.Ctl.Zones() {
		if eff, ok := d.Zone(i); ok {
			effects[i] = eff
		}
	}
	if len(effects) == 0 {
		return 0, fmt.Errorf("no effects configured yet, set one first")
	}
	return e.Ctl.SaveToProfiles(effects)
}
