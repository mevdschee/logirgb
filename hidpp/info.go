package hidpp

import (
	"fmt"
	"strings"
)

// Kind returns the device type from 0x0005 GetDeviceType, such as
// "keyboard", "mouse" or "headset", or "" when it is not known.
func (d *Device) Kind() string {
	r, err := d.Call(FeatureDeviceName, 2)
	if err != nil {
		return ""
	}
	kinds := []string{"keyboard", "remote control", "numpad", "mouse", "touchpad", "trackball", "presenter", "receiver", "headset"}
	if int(r[0]) < len(kinds) {
		return kinds[r[0]]
	}
	return ""
}

// UnitID returns the device's unit ID from 0x0003 GetDeviceInfo, which tells
// the same device apart when it shows up on more than one node. It is not
// meant to be shown.
func (d *Device) UnitID() (uint32, error) {
	r, err := d.Call(FeatureFirmware, 0)
	if err != nil {
		return 0, err
	}
	return uint32(r[1])<<24 | uint32(r[2])<<16 | uint32(r[3])<<8 | uint32(r[4]), nil
}

// Models returns the device's product IDs per connection, read from
// 0x0003 GetDeviceInfo, like "USB C343 · LIGHTSPEED 408E".
func (d *Device) Models() string {
	r, err := d.Call(FeatureFirmware, 0)
	if err != nil {
		return ""
	}
	return ModelText(r)
}

// ModelText decodes the transports (bytes 5-6) and model IDs (bytes 7-12,
// one per set transport bit, in bit order) of a 0x0003 GetDeviceInfo reply.
func ModelText(info []byte) string {
	if len(info) < 13 {
		return ""
	}
	transports := int(info[5])<<8 | int(info[6])
	var ids []int
	for i := 0; i < 3; i++ {
		ids = append(ids, int(info[7+2*i])<<8|int(info[8+2*i]))
	}
	names := []string{"Bluetooth", "Bluetooth", "LIGHTSPEED", "USB"}
	var labels []string
	for bit := 0; bit < 4; bit++ {
		if transports>>bit&1 != 0 {
			labels = append(labels, names[bit])
		}
	}
	matches := len(labels) > 0 && len(labels) <= 3
	for i, id := range ids {
		if (i < len(labels)) != (id != 0) {
			matches = false
		}
	}
	var parts []string
	if !matches {
		for _, id := range ids {
			if id != 0 {
				parts = append(parts, fmt.Sprintf("%04X", id))
			}
		}
		return strings.Join(parts, " · ")
	}
	for _, want := range []string{"USB", "LIGHTSPEED", "Bluetooth"} {
		for i, l := range labels {
			if l == want {
				parts = append(parts, fmt.Sprintf("%s %04X", l, ids[i]))
			}
		}
	}
	return strings.Join(parts, " · ")
}

// Firmware returns the version of the main firmware from 0x0003, like
// "U1 25.01 build 0017".
func (d *Device) Firmware() string {
	r, err := d.Call(FeatureFirmware, 0)
	if err != nil {
		return ""
	}
	for e := 0; e < int(r[0]) && e < 8; e++ {
		f, err := d.Call(FeatureFirmware, 1, byte(e))
		if err != nil || f[0] != 0 {
			continue
		}
		return fmt.Sprintf("%s %02X.%02X build %02X%02X", strings.Trim(string(f[1:4]), "\x00 "), f[4], f[5], f[6], f[7])
	}
	return ""
}

// Battery is the charge of a wireless device.
type Battery struct {
	Percent int
	// Approx is set when the percentage comes from the voltage.
	Approx     bool
	Millivolts int
	Charging   bool
	Full       bool
}

func (b Battery) String() string {
	s := fmt.Sprintf("%d%%", b.Percent)
	if b.Approx {
		s = "~" + s
	}
	switch {
	case b.Full:
		s += " (full)"
	case b.Charging:
		s += " (charging)"
	}
	return s
}

// percentFromMillivolts estimates the charge of a lithium cell.
func percentFromMillivolts(mv int) int {
	return max(0, min(100, (mv-3500)*100/600))
}

// Battery reads the battery with 0x1004 UnifiedBattery, 0x1000
// BatteryLevelStatus, 0x1001 BatteryVoltage or the headsets' 0x1F20. It
// returns nil without error for a device without a battery.
func (d *Device) Battery() (*Battery, error) {
	switch {
	case d.HasFeature(FeatureUnifiedBattery):
		r, err := d.Call(FeatureUnifiedBattery, 1)
		if err != nil {
			return nil, err
		}
		return &Battery{Percent: int(r[0]), Charging: r[2] == 1 || r[2] == 2, Full: r[2] == 3}, nil
	case d.HasFeature(FeatureBattery):
		r, err := d.Call(FeatureBattery, 0)
		if err != nil {
			return nil, err
		}
		return &Battery{Percent: int(r[0]), Charging: r[2] == 1 || r[2] == 2 || r[2] == 4, Full: r[2] == 3}, nil
	case d.HasFeature(FeatureBatteryVoltage):
		r, err := d.Call(FeatureBatteryVoltage, 0)
		if err != nil {
			return nil, err
		}
		mv := int(r[0])<<8 | int(r[1])
		plugged := r[2]&0x80 != 0
		return &Battery{Percent: percentFromMillivolts(mv), Approx: true, Millivolts: mv,
			Charging: plugged && r[2]&0x07 == 0, Full: plugged && r[2]&0x07 == 1}, nil
	case d.HasFeature(FeatureHeadsetPower):
		r, err := d.Call(FeatureHeadsetPower, 0)
		if err != nil {
			return nil, err
		}
		mv := int(r[0])<<8 | int(r[1])
		return &Battery{Percent: percentFromMillivolts(mv), Approx: true, Millivolts: mv, Charging: r[2] == 3}, nil
	}
	return nil, nil
}
