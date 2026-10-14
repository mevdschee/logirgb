package lighting

import (
	"errors"
	"fmt"

	"github.com/mevdschee/logirgb/hidpp"
)

// Zone is an independently controllable lighting area.
type Zone struct {
	Index    byte
	Location string
	Effects  []ZoneEffect
}

// ZoneEffect is an effect the zone supports, with its device specific index.
type ZoneEffect struct {
	Index         byte
	Type          EffectType
	DefaultPeriod int
}

// Find returns the zone's effect with the given name.
func (z Zone) Find(name string) (ZoneEffect, bool) {
	for _, e := range z.Effects {
		if e.Type.Name == name {
			return e, true
		}
	}
	return ZoneEffect{}, false
}

var zoneLocations = map[uint16]string{
	0x00: "unknown",
	0x01: "primary",
	0x02: "logo",
	0x03: "left side",
	0x04: "right side",
	0x05: "combined",
	0x06: "primary 1",
	0x07: "primary 2",
	0x08: "primary 3",
	0x09: "primary 4",
	0x0A: "primary 5",
	0x0B: "primary 6",
}

// Controller exposes the lighting capabilities of one device.
type Controller struct {
	dev     *hidpp.Device
	feature uint16
	zones   []Zone
}

// ErrNoLighting is returned for devices without a supported RGB feature.
var ErrNoLighting = errors.New("device has no supported RGB lighting feature")

// New inspects the device and enumerates its zones and effects.
func New(dev *hidpp.Device) (*Controller, error) {
	c := &Controller{dev: dev}
	switch {
	case dev.HasFeature(hidpp.FeatureRGBEffects):
		c.feature = hidpp.FeatureRGBEffects
	case dev.HasFeature(hidpp.FeatureColorLEDEffects):
		c.feature = hidpp.FeatureColorLEDEffects
	default:
		return nil, ErrNoLighting
	}
	if err := c.enumerate(); err != nil {
		return nil, fmt.Errorf("enumerate zones: %w", err)
	}
	return c, nil
}

// Device returns the underlying HID++ device.
func (c *Controller) Device() *hidpp.Device { return c.dev }

// Zones lists the lighting zones.
func (c *Controller) Zones() []Zone { return c.zones }

// Feature returns the RGB feature in use (0x8070 or 0x8071).
func (c *Controller) Feature() uint16 { return c.feature }

func (c *Controller) enumerate() error {
	rgb := c.feature == hidpp.FeatureRGBEffects

	var count int
	if rgb {
		r, err := c.dev.Call(c.feature, 0, 0xFF, 0xFF, 0x00)
		if err != nil {
			return err
		}
		count = int(r[2])
	} else {
		r, err := c.dev.Call(c.feature, 0)
		if err != nil {
			return err
		}
		count = int(r[0])
	}

	for z := 0; z < count; z++ {
		var info []byte
		var err error
		if rgb {
			info, err = c.dev.Call(c.feature, 0, byte(z), 0xFF, 0x00)
			if err == nil {
				info = info[1:]
			}
		} else {
			info, err = c.dev.Call(c.feature, 1, byte(z), 0xFF, 0x00)
		}
		if err != nil {
			return err
		}
		loc := uint16(info[1])<<8 | uint16(info[2])
		zone := Zone{Index: byte(z), Location: zoneLocations[loc]}
		if zone.Location == "" {
			zone.Location = fmt.Sprintf("zone %d", z)
		}
		for e := 0; e < int(info[3]); e++ {
			var r []byte
			if rgb {
				r, err = c.dev.Call(c.feature, 0, byte(z), byte(e), 0x00)
			} else {
				r, err = c.dev.Call(c.feature, 2, byte(z), byte(e), 0x00)
			}
			if err != nil {
				return err
			}
			id := uint16(r[2])<<8 | uint16(r[3])
			t, ok := effectTypes[id]
			if !ok {
				continue
			}
			if rgb {
				// 0x8071 firmware fixes the breathe waveform itself.
				t = t.without(ParamForm)
			}
			zone.Effects = append(zone.Effects, ZoneEffect{
				Index:         byte(e),
				Type:          t,
				DefaultPeriod: int(r[6])<<8 | int(r[7]),
			})
		}
		c.zones = append(c.zones, zone)
	}
	return nil
}

// Claim takes lighting control away from the onboard firmware so host
// written effects become visible. Only 0x8071 devices need this.
func (c *Controller) Claim() error {
	if c.feature != hidpp.FeatureRGBEffects {
		return nil
	}
	if c.dev.HasFeature(hidpp.FeatureProfileManagement) {
		if _, err := c.dev.Call(hidpp.FeatureProfileManagement, 6, 0x05); err != nil {
			return fmt.Errorf("profile management: %w", err)
		}
	} else if c.dev.HasFeature(hidpp.FeatureOnboardProfiles) {
		if _, err := c.dev.Call(hidpp.FeatureOnboardProfiles, 1, 0x02); err != nil {
			return fmt.Errorf("onboard profiles: %w", err)
		}
	}
	return c.setSWControl(swActive)
}

// Release hands lighting back to the onboard profile.
func (c *Controller) Release() error {
	if c.feature != hidpp.FeatureRGBEffects {
		return nil
	}
	if err := c.setSWControl(swRelease); err != nil {
		return err
	}
	if c.dev.HasFeature(hidpp.FeatureProfileManagement) {
		_, err := c.dev.Call(hidpp.FeatureProfileManagement, 6, 0x03)
		return err
	} else if c.dev.HasFeature(hidpp.FeatureOnboardProfiles) {
		_, err := c.dev.Call(hidpp.FeatureOnboardProfiles, 1, 0x01)
		return err
	}
	return nil
}

// Claimed reports whether the host currently controls the lighting.
func (c *Controller) Claimed() (bool, error) {
	if c.feature != hidpp.FeatureRGBEffects {
		return true, nil
	}
	r, err := c.dev.Call(c.feature, 5, 0x00)
	if err != nil {
		return false, err
	}
	return r[1] != 0, nil
}

// SetSWControl payloads for 0x8071 function 5: [set, mode, flags]. Mode 3
// gives the host the lighting; the firmware then stops dimming on idle.
var (
	swActive  = []byte{0x01, 0x03, 0x04}
	swRelease = []byte{0x01, 0x00, 0x00}
)

func (c *Controller) setSWControl(mode []byte) error {
	_, err := c.dev.Call(c.feature, 5, mode...)
	return err
}

// SetEffect applies an effect to a zone.
func (c *Controller) SetEffect(zone int, e Effect) error {
	return c.setEffect(zone, e, false)
}

// SetEffectPersistent applies an effect and also stores it in the device's
// non-volatile memory (0x8070 persistence byte 1: RAM and EEPROM), so it is
// shown again after a power cycle. On the G502 HERO a profile switch still
// loads the LED blocks of the onboard profile; see SaveToProfiles.
func (c *Controller) SetEffectPersistent(zone int, e Effect) error {
	if c.feature != hidpp.FeatureColorLEDEffects {
		return errors.New("persistent zone effects are only supported on 0x8070 devices")
	}
	return c.setEffect(zone, e, true)
}

func (c *Controller) setEffect(zone int, e Effect, persist bool) error {
	if zone < 0 || zone >= len(c.zones) {
		return fmt.Errorf("no zone %d (device has %d)", zone, len(c.zones))
	}
	z := c.zones[zone]
	ze, ok := z.Find(e.Name)
	if !ok {
		return fmt.Errorf("zone %q does not support effect %q (supported: %s)", z.Location, e.Name, effectNames(z))
	}
	params := e.encode(ze.Type)
	payload := append([]byte{z.Index, ze.Index}, params[:]...)
	if c.feature == hidpp.FeatureRGBEffects {
		// Trailing byte: apply to RAM only, onboard profiles stay untouched.
		_, err := c.dev.Call(c.feature, 1, append(payload, 0x01)...)
		return err
	}
	// Trailing byte: 0 = RAM only, 1 = RAM and EEPROM.
	var p byte
	if persist {
		p = 0x01
	}
	_, err := c.dev.Call(c.feature, 3, append(payload, p)...)
	return err
}

func effectNames(z Zone) string {
	s := ""
	for i, e := range z.Effects {
		if i > 0 {
			s += ", "
		}
		s += e.Type.Name
	}
	return s
}

// HasBrightness reports whether the device has global brightness control.
func (c *Controller) HasBrightness() bool {
	return c.dev.HasFeature(hidpp.FeatureBrightnessControl)
}

// BrightnessRange returns the minimum and maximum brightness values.
func (c *Controller) BrightnessRange() (int, int, error) {
	r, err := c.dev.Call(hidpp.FeatureBrightnessControl, 0)
	if err != nil {
		return 0, 0, err
	}
	return int(r[4])<<8 | int(r[5]), int(r[0])<<8 | int(r[1]), nil
}

// Brightness returns the current global brightness.
func (c *Controller) Brightness() (int, error) {
	r, err := c.dev.Call(hidpp.FeatureBrightnessControl, 1)
	if err != nil {
		return 0, err
	}
	return int(r[0])<<8 | int(r[1]), nil
}

// SetBrightness sets the global brightness.
func (c *Controller) SetBrightness(v int) error {
	_, err := c.dev.Call(hidpp.FeatureBrightnessControl, 2, byte(v>>8), byte(v))
	return err
}

// HasTimers reports whether the device keeps idle/sleep timers in firmware.
func (c *Controller) HasTimers() bool {
	return c.feature == hidpp.FeatureRGBEffects
}

// timerNever is the wire value for a timeout that never expires. A literal 0
// is not "disabled" to the firmware.
const timerNever = 0xFFFF

// Timers returns the firmware idle (dim) and sleep (off) timeouts in
// seconds, with 0 meaning never.
func (c *Controller) Timers() (idle, sleep int, err error) {
	r, err := c.dev.Call(c.feature, 7, 0x00)
	if err != nil {
		return 0, 0, err
	}
	return decodeTimer(r[3:5]), decodeTimer(r[5:7]), nil
}

// SetTimers stores the idle and sleep timeouts in firmware; 0 means never.
func (c *Controller) SetTimers(idle, sleep int) error {
	params := make([]byte, 7)
	params[0] = 0x01
	encodeTimer(params[3:5], idle)
	encodeTimer(params[5:7], sleep)
	_, err := c.dev.Call(c.feature, 7, params...)
	return err
}

func encodeTimer(b []byte, s int) {
	if s <= 0 || s >= timerNever {
		s = timerNever
	}
	b[0], b[1] = byte(s>>8), byte(s)
}

func decodeTimer(b []byte) int {
	s := int(b[0])<<8 | int(b[1])
	if s == timerNever {
		return 0
	}
	return s
}
