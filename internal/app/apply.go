package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mevdschee/logirgb/internal/config"
	"github.com/mevdschee/logirgb/lighting"
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

// ApplyTimers writes the idle (dim) and sleep (off) timeouts to firmware.
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
	if err := e.Ctl.SetTimers(idle, sleep); err != nil {
		return fmt.Errorf("write timers: %w", err)
	}
	return nil
}

// DefaultDimPercent is used when no dim level was configured.
const DefaultDimPercent = 50

// SaveToProfiles stores the configured lighting in the device's onboard
// memory, after backing that memory up. For keyboards with profile storage
// it also stores the idle/sleep timeouts and hands the lighting back to the
// firmware, which then dims and switches off by itself. It returns the
// number of profiles written and the backup directory.
func SaveToProfiles(e *Entry, d *config.Device) (int, string, error) {
	effects := profileEffects(e, d, 0)
	if len(effects) == 0 && len(d.Profiles) == 0 {
		return 0, "", fmt.Errorf("no effects configured yet, set one first")
	}
	dir, err := backup(e)
	if err != nil {
		return 0, "", fmt.Errorf("backup: %w", err)
	}
	if !e.Ctl.HasProfileStorage() {
		count, err := e.Ctl.ProfileCount()
		if err != nil {
			return 0, dir, err
		}
		profiles := make([]lighting.Profile, count)
		for p := range profiles {
			profiles[p] = lighting.Profile{Effects: profileEffects(e, d, p+1), Settings: d.Settings(p + 1)}
		}
		n, err := e.Ctl.SaveToProfiles(profiles)
		if err != nil {
			return n, dir, err
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
				return n, dir, fmt.Errorf("zone %d: %w", zone, err)
			}
		}
		return n, dir, nil
	}
	eff, ok := effects[0]
	if !ok {
		return 0, dir, fmt.Errorf("no effect configured for zone 0")
	}
	idle, sleep, err := e.Ctl.Timers()
	if err != nil {
		return 0, dir, err
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
		return 0, dir, err
	}
	// The stored timers only load on power-up; set the running ones too.
	return 1, dir, e.Ctl.SetTimers(idle, sleep)
}

// LoadProfiles takes the lighting stored in the device's onboard memory
// into d, so it shows and is written back as stored. On a keyboard with
// profile storage that is the effect, dim level and timeouts. On a mouse
// with onboard profiles a profile gets its own effect for a zone where it
// differs from the effect for all profiles, a zone without an effect for all
// profiles takes the active profile's, and each profile's report rate and
// DPI levels are taken too. d is only changed in memory.
func LoadProfiles(e *Entry, d *config.Device) error {
	if e.Ctl.HasProfileStorage() {
		sp, ok, err := e.Ctl.ReadProfile()
		if err != nil || !ok {
			return err
		}
		d.SetZone(0, sp.Effect)
		if sp.DimPercent > 0 {
			d.DimPercent = config.IntPtr(sp.DimPercent)
		}
		d.IdleTimeout = config.IntPtr(sp.Idle)
		d.SleepTimeout = config.IntPtr(sp.Sleep)
		return nil
	}
	if !e.Ctl.HasOnboardProfiles() {
		return nil
	}
	profiles, err := e.Ctl.ReadProfiles()
	if err != nil {
		return err
	}
	active, err := e.Ctl.ActiveProfile()
	if err != nil || active < 1 || active > len(profiles) {
		active = 1
	}
	for i, z := range e.Ctl.Zones() {
		all, ok := d.Zone(i)
		if !ok {
			if all, ok = profiles[active-1].Effects[i]; ok {
				d.SetZone(i, all)
			}
		}
		for p, prof := range profiles {
			eff, stored := prof.Effects[i]
			switch {
			case !stored:
			case ok && z.Same(eff, all):
				d.ClearProfileZone(p+1, i)
			default:
				d.SetProfileZone(p+1, i, eff)
			}
		}
	}
	for p, prof := range profiles {
		d.SetSettings(p+1, *prof.Settings)
	}
	return nil
}

// profileEffects returns the configured zone effects of an onboard profile
// (1-based), or of all profiles for profile 0.
func profileEffects(e *Entry, d *config.Device, profile int) map[int]lighting.Effect {
	effects := map[int]lighting.Effect{}
	for i := range e.Ctl.Zones() {
		if eff, ok := d.ProfileZone(profile, i); ok {
			effects[i] = eff
		}
	}
	return effects
}

// backup saves the device's onboard memory under
// ~/.local/share/logirgb/backups/<vid-pid>-<time>/.
func backup(e *Entry) (string, error) {
	files, err := e.Ctl.BackupProfiles()
	if err != nil {
		return "", err
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(base, "logirgb", "backups",
		strings.ReplaceAll(e.Key, ":", "-")+"-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}
