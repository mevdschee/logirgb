// Package power implements dim-on-idle and off-on-sleep for devices whose
// firmware reports user activity but leaves the lighting to the host while
// it is under software control (0x8071 RGBEffects, e.g. the G515).
package power

import (
	"context"
	"log"
	"time"

	"github.com/mevdschee/logitech-rgb/hidpp"
	"github.com/mevdschee/logitech-rgb/internal/app"
	"github.com/mevdschee/logitech-rgb/internal/config"
	"github.com/mevdschee/logitech-rgb/lighting"
)

type state int

const (
	active state = iota
	dimmed
	asleep
)

// Manager runs the idle state machine for one device.
type Manager struct {
	entry *app.Entry
	cfg   func() *config.Device
	state state
	// idleSince is when the firmware reported the user as idle.
	idleSince time.Time
	// settleUntil masks the burst of stale activity events the firmware
	// sends right after software control is (re)claimed.
	settleUntil time.Time
}

const settle = 2 * time.Second

// New returns a manager; cfg is called whenever the current settings are
// needed so config reloads take effect without restarting.
func New(e *app.Entry, cfg func() *config.Device) *Manager {
	return &Manager{entry: e, cfg: cfg}
}

// Run processes device events until ctx is done or the device goes away.
func (m *Manager) Run(ctx context.Context) error {
	dev := m.entry.Dev
	featIdx, err := dev.FeatureIndex(hidpp.FeatureRGBEffects)
	if err != nil {
		return err
	}
	m.settleUntil = time.Now().Add(settle)
	for ctx.Err() == nil {
		n, err := dev.NextNotification(500 * time.Millisecond)
		if err != nil {
			return err
		}
		if n != nil && n.FeatureIndex == featIdx && n.Function == 1 && time.Now().After(m.settleUntil) {
			// onUserActivity: 0 is idle, anything else is activity. The
			// firmware repeats each event a few times; transitions are
			// idempotent so repeats are harmless.
			if n.Data[0] == 0 {
				m.onIdle()
			} else {
				m.onActive()
			}
		}
		m.tick()
	}
	return nil
}

func (m *Manager) settings() (idle, sleep int, d *config.Device) {
	d = m.cfg()
	if d.IdleTimeout != nil {
		idle = *d.IdleTimeout
	}
	if d.SleepTimeout != nil {
		sleep = *d.SleepTimeout
	}
	return idle, sleep, d
}

func (m *Manager) onIdle() {
	if m.state != active {
		return
	}
	idle, _, d := m.settings()
	m.idleSince = time.Now()
	ctl := m.entry.Ctl
	// Ask the firmware to report activity from now on.
	if err := ctl.SetActivityWatch(); err != nil {
		log.Printf("%s: %v", m.entry.Name, err)
	}
	m.state = dimmed
	if idle > 0 && d.DimBrightness != nil && ctl.HasBrightness() {
		log.Printf("%s: idle, dimming", m.entry.Name)
		if err := ctl.SetBrightness(*d.DimBrightness); err != nil {
			log.Printf("%s: dim: %v", m.entry.Name, err)
		}
	}
	m.tick()
}

func (m *Manager) tick() {
	if m.state != dimmed {
		return
	}
	idle, sleep, _ := m.settings()
	if sleep <= 0 {
		return
	}
	// The idle event arrived at the firmware idle timeout, see ApplyTimers.
	fired := idle
	if fired == 0 {
		fired = sleep
	}
	if time.Since(m.idleSince) < time.Duration(sleep-fired)*time.Second {
		return
	}
	log.Printf("%s: sleeping, lighting off", m.entry.Name)
	ctl := m.entry.Ctl
	for i, z := range ctl.Zones() {
		if _, ok := z.Find("off"); ok {
			if err := ctl.SetEffect(i, lighting.Effect{Name: "off"}); err != nil {
				log.Printf("%s: off: %v", m.entry.Name, err)
			}
		}
	}
	m.state = asleep
}

func (m *Manager) onActive() {
	if m.state == active {
		return
	}
	log.Printf("%s: active, restoring lighting", m.entry.Name)
	m.state = active
	if err := app.Apply(m.entry, m.cfg()); err != nil {
		log.Printf("%s: restore: %v", m.entry.Name, err)
	}
	m.settleUntil = time.Now().Add(settle)
}
