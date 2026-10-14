// Package config persists the desired lighting state per device model.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/mevdschee/logirgb/lighting"
)

// Device holds the settings for one device model.
type Device struct {
	Name  string                     `json:"name,omitempty"`
	Zones map[string]lighting.Effect `json:"zones,omitempty"`
	// Profiles overrides the zone effects per onboard profile (1-based),
	// for devices that store lighting in each profile (G502).
	Profiles map[string]map[string]lighting.Effect `json:"profiles,omitempty"`
	// Brightness in device units (G515: 0-100); nil leaves it alone.
	Brightness *int `json:"brightness,omitempty"`
	// IdleTimeout in seconds before the lighting dims; 0 disables dimming.
	IdleTimeout *int `json:"idle_timeout,omitempty"`
	// SleepTimeout in seconds of inactivity before the lighting turns off;
	// 0 disables it.
	SleepTimeout *int `json:"sleep_timeout,omitempty"`
	// DimPercent is the brightness of the dimmed lighting relative to the
	// normal lighting (keyboards with profile storage).
	DimPercent *int `json:"dim_percent,omitempty"`
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

// Config is the whole configuration file.
type Config struct {
	Devices map[string]*Device `json:"devices"`
}

// Key identifies a device model in the config, e.g. "046d:c358".
func Key(vendor, product uint16) string {
	return fmt.Sprintf("%04x:%04x", vendor, product)
}

// Device returns (creating if needed) the entry for a device.
func (c *Config) Device(key string) *Device {
	if c.Devices == nil {
		c.Devices = map[string]*Device{}
	}
	d, ok := c.Devices[key]
	if !ok {
		d = &Device{}
		c.Devices[key] = d
	}
	return d
}

// Path returns the config file location.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logirgb", "config.json"), nil
}

// Load reads the config file; a missing file yields an empty config.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	return &c, nil
}

// Save writes the config file atomically.
func (c *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// IntPtr is a small helper for the optional fields.
func IntPtr(v int) *int { return &v }
