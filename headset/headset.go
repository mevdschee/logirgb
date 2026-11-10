// Package headset reads and sets the sidetone (0x8300) and the auto-off
// time (0x1F20) of Logitech G headsets.
package headset

import (
	"errors"

	"github.com/mevdschee/logirgb/hidpp"
)

// ErrNoSettings is returned for devices without headset settings.
var ErrNoSettings = errors.New("device has no headset settings")

// AutoOffMinutes are the auto-off times the headset offers, 0 is never.
var AutoOffMinutes = []int{0, 5, 10, 15, 30, 60, 90}

// autoOffModels are the PRO X Wireless dongles, whose 0x1F20 fn 1 and fn 2
// are known to read and set the auto-off time in minutes. On other headsets
// these functions are unknown, so they are left alone.
var autoOffModels = map[uint16]bool{0x0AAA: true, 0x0ABA: true, 0x0AFB: true, 0x0AFC: true}

// Controller exposes the settings of one headset.
type Controller struct {
	dev      *hidpp.Device
	sidetone bool
	autoOff  bool
}

// New inspects the device for headset settings.
func New(dev *hidpp.Device) (*Controller, error) {
	c := &Controller{
		dev:      dev,
		sidetone: dev.HasFeature(hidpp.FeatureSidetone),
		autoOff:  autoOffModels[dev.Info.ProductID] && dev.HasFeature(hidpp.FeatureHeadsetPower),
	}
	if !c.sidetone && !c.autoOff {
		return nil, ErrNoSettings
	}
	return c, nil
}

// HasSidetone reports whether the headset can play the microphone back.
func (c *Controller) HasSidetone() bool { return c.sidetone }

// Sidetone returns the sidetone level, 0-100.
func (c *Controller) Sidetone() (int, error) {
	r, err := c.dev.Call(hidpp.FeatureSidetone, 0)
	if err != nil {
		return 0, err
	}
	return int(r[0]), nil
}

// SetSidetone sets the sidetone level, 0-100. Some headsets only play it
// while an application uses the microphone.
func (c *Controller) SetSidetone(v int) error {
	_, err := c.dev.Call(hidpp.FeatureSidetone, 1, byte(max(0, min(100, v))))
	return err
}

// HasAutoOff reports whether the headset's auto-off time can be set.
func (c *Controller) HasAutoOff() bool { return c.autoOff }

// AutoOff returns the minutes without sound before the headset turns
// off, 0 is never.
func (c *Controller) AutoOff() (int, error) {
	r, err := c.dev.Call(hidpp.FeatureHeadsetPower, 1)
	if err != nil {
		return 0, err
	}
	return int(r[0]), nil
}

// SetAutoOff sets the auto-off time in minutes (0-90), 0 is never.
func (c *Controller) SetAutoOff(minutes int) error {
	_, err := c.dev.Call(hidpp.FeatureHeadsetPower, 2, byte(max(0, min(90, minutes))))
	return err
}
