// Package hidpp implements the subset of the Logitech HID++ 2.0 protocol
// needed to discover devices and talk to their features over Linux hidraw.
package hidpp

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	reportShort = 0x10
	reportLong  = 0x11

	shortLen = 7
	longLen  = 20

	// DeviceIndexDirect addresses a device that is connected directly over
	// USB or Bluetooth rather than through a receiver.
	DeviceIndexDirect = 0xFF

	// DefaultSoftwareID tags our requests. It avoids the IDs known to be
	// used by the kernel (0x01), OpenRGB (0x07), libratbag (0x08), several
	// G HUB replacements (0x0A), Solaar (0x0B) and G HUB (0x0C, 0x0D).
	DefaultSoftwareID = 0x0E

	defaultTimeout = 2 * time.Second
)

// Feature IDs used by this tool.
const (
	FeatureRoot              uint16 = 0x0000
	FeatureFeatureSet        uint16 = 0x0001
	FeatureFirmware          uint16 = 0x0003
	FeatureDeviceName        uint16 = 0x0005
	FeatureBattery           uint16 = 0x1000
	FeatureBatteryVoltage    uint16 = 0x1001
	FeatureUnifiedBattery    uint16 = 0x1004
	FeatureHeadsetPower      uint16 = 0x1F20
	FeatureBrightnessControl uint16 = 0x8040
	FeatureColorLEDEffects   uint16 = 0x8070
	FeatureRGBEffects        uint16 = 0x8071
	FeaturePerKeyLighting    uint16 = 0x8080
	FeaturePerKeyLighting2   uint16 = 0x8081
	FeatureOnboardProfiles   uint16 = 0x8100
	FeatureProfileManagement uint16 = 0x8101
	FeatureSidetone          uint16 = 0x8300
)

// ErrNotSupported is returned when a device does not implement a feature.
var ErrNotSupported = errors.New("feature not supported")

// Error is a HID++ 2.0 error reply.
type Error struct {
	Code byte
}

var errorNames = map[byte]string{
	0x01: "unknown",
	0x02: "invalid argument",
	0x03: "out of range",
	0x04: "hardware error",
	0x05: "logitech internal",
	0x06: "invalid feature index",
	0x07: "invalid function",
	0x08: "busy",
	0x09: "unsupported",
}

func (e *Error) Error() string {
	if n, ok := errorNames[e.Code]; ok {
		return "hid++ error: " + n
	}
	return fmt.Sprintf("hid++ error 0x%02x", e.Code)
}

// Error10 is a HID++ 1.0 error reply, as sent by receivers.
type Error10 struct {
	Code byte
}

func (e *Error10) Error() string {
	return fmt.Sprintf("hid++ 1.0 error 0x%02x", e.Code)
}

// HID++ 1.0 error codes a receiver sends for a slot.
const (
	err10ConnectFail   = 0x04
	err10ResourceError = 0x09
)

// Device is an open HID++ 2.0 device.
type Device struct {
	Info  Info
	Index byte
	// SoftwareID tags requests so concurrent processes on the same hidraw
	// node can tell their replies apart (1-15).
	SoftwareID byte

	fd       int
	mu       sync.Mutex
	features map[uint16]byte
	versions map[uint16]byte
	pending  []Notification
	name     string
}

// Notification is an unsolicited event sent by the device.
type Notification struct {
	FeatureIndex byte
	Function     byte
	Data         []byte
}

// Open opens the hidraw node described by info and locates a HID++ 2.0
// device on it. Directly attached devices answer on index 0xFF; for
// receivers the first paired device that responds is used.
func Open(info Info) (*Device, error) {
	slots, err := OpenAll(info)
	if err != nil {
		return nil, err
	}
	var dev *Device
	for _, s := range slots {
		if s.Dev != nil && dev == nil {
			dev = s.Dev
		} else if s.Dev != nil {
			s.Dev.Close()
		}
	}
	if dev == nil {
		return nil, fmt.Errorf("%s: no HID++ 2.0 device responded", info.Path)
	}
	return dev, nil
}

// Slot is a device found on a hidraw node. Dev is nil for a device that is
// paired with a receiver but asleep or out of range.
type Slot struct {
	Index byte
	Dev   *Device
}

// OpenAll opens every HID++ 2.0 device on the hidraw node. A directly
// attached device answers on index 0xFF. A receiver answers there with a
// HID++ 1.0 error, and then each of its slots is tried: every paired device
// gets its own Device, and paired devices that do not answer are returned
// without one. A node the kernel made for one device behind a receiver
// answers on that device's index.
func OpenAll(info Info) ([]Slot, error) {
	d, err := openIndex(info, DeviceIndexDirect)
	if err != nil {
		return nil, err
	}
	// A busy device occasionally misses the first ping, so the direct index
	// gets a few attempts before receiver slots are tried.
	receiver := false
	for i := 0; i < 3; i++ {
		err := d.ping(byte(i))
		if err == nil {
			return []Slot{{Index: DeviceIndexDirect, Dev: d}}, nil
		}
		var e10 *Error10
		if errors.As(err, &e10) {
			receiver = true
			break
		}
	}
	var slots []Slot
	for idx := byte(1); idx <= 7; idx++ {
		d.Index = idx
		err := d.ping(3 + idx)
		if err == nil {
			slots = append(slots, Slot{Index: idx, Dev: d})
			if !receiver {
				return slots, nil
			}
			if d, err = openIndex(info, DeviceIndexDirect); err != nil {
				closeSlots(slots)
				return nil, err
			}
			continue
		}
		var e10 *Error10
		if receiver && errors.As(err, &e10) && (e10.Code == err10ConnectFail || e10.Code == err10ResourceError) {
			slots = append(slots, Slot{Index: idx})
		}
	}
	d.Close()
	if len(slots) == 0 {
		return nil, fmt.Errorf("%s: no HID++ 2.0 device responded", info.Path)
	}
	return slots, nil
}

func closeSlots(slots []Slot) {
	for _, s := range slots {
		if s.Dev != nil {
			s.Dev.Close()
		}
	}
}

// openFD opens a hidraw node; tests replace it with a fake device.
var openFD = func(path string) (int, error) {
	return unix.Open(path, unix.O_RDWR|unix.O_CLOEXEC, 0)
}

func openIndex(info Info, idx byte) (*Device, error) {
	fd, err := openFD(info.Path)
	if err != nil {
		if errors.Is(err, unix.EACCES) {
			return nil, fmt.Errorf("open %s: permission denied (install the udev rule, see README)", info.Path)
		}
		return nil, fmt.Errorf("open %s: %w", info.Path, err)
	}
	return &Device{
		Info: info, Index: idx, SoftwareID: DefaultSoftwareID, fd: fd,
		features: map[uint16]byte{FeatureRoot: 0},
		versions: map[uint16]byte{},
	}, nil
}

// ping checks that a HID++ 2.0 device answers on d.Index. The ping echoes
// its third byte; a changing value makes sure the reply belongs to this
// ping and not to an earlier one.
func (d *Device) ping(n byte) error {
	payload := 0x5A + n
	r, err := d.call(0, 1, 500*time.Millisecond, 0, 0, payload)
	if err != nil {
		return err
	}
	if r[2] != payload {
		return errors.New("ping reply does not match")
	}
	return nil
}

// Close releases the hidraw node.
func (d *Device) Close() error {
	return unix.Close(d.fd)
}

// FeatureIndex resolves a feature ID to the device specific index.
func (d *Device) FeatureIndex(id uint16) (byte, error) {
	d.mu.Lock()
	idx, ok := d.features[id]
	d.mu.Unlock()
	if ok {
		if idx == 0 && id != FeatureRoot {
			return 0, ErrNotSupported
		}
		return idx, nil
	}
	resp, err := d.call(0, 0, defaultTimeout, byte(id>>8), byte(id))
	if err != nil {
		return 0, err
	}
	d.mu.Lock()
	d.features[id] = resp[0]
	d.versions[id] = resp[2]
	d.mu.Unlock()
	if resp[0] == 0 {
		return 0, ErrNotSupported
	}
	return resp[0], nil
}

// FeatureVersion returns the version of a feature the device implements.
func (d *Device) FeatureVersion(id uint16) (int, error) {
	if _, err := d.FeatureIndex(id); err != nil {
		return 0, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return int(d.versions[id]), nil
}

// HasFeature reports whether the device implements the given feature.
func (d *Device) HasFeature(id uint16) bool {
	_, err := d.FeatureIndex(id)
	return err == nil
}

// Call invokes function fn of the feature with the given ID and returns the
// 16 parameter bytes of the reply.
func (d *Device) Call(feature uint16, fn byte, params ...byte) ([]byte, error) {
	idx, err := d.FeatureIndex(feature)
	if err != nil {
		return nil, err
	}
	return d.call(idx, fn, defaultTimeout, params...)
}

// Features lists all feature IDs the device advertises.
func (d *Device) Features() ([]uint16, error) {
	resp, err := d.Call(FeatureFeatureSet, 0)
	if err != nil {
		return nil, err
	}
	ids := []uint16{FeatureRoot}
	for i := 1; i <= int(resp[0]); i++ {
		r, err := d.Call(FeatureFeatureSet, 1, byte(i))
		if err != nil {
			return nil, err
		}
		ids = append(ids, uint16(r[0])<<8|uint16(r[1]))
	}
	return ids, nil
}

// Name returns the marketing name reported by the device, falling back to
// the name the kernel knows it by.
func (d *Device) Name() string {
	if d.name == "" {
		d.name = d.readName()
	}
	return d.name
}

func (d *Device) readName() string {
	resp, err := d.Call(FeatureDeviceName, 0)
	if err != nil {
		return d.Info.Name
	}
	n := int(resp[0])
	name := make([]byte, 0, n)
	for len(name) < n {
		r, err := d.Call(FeatureDeviceName, 1, byte(len(name)))
		if err != nil {
			return d.Info.Name
		}
		for _, c := range r {
			if c == 0 || len(name) == n {
				break
			}
			name = append(name, c)
		}
	}
	return string(name)
}

func (d *Device) call(featIdx, fn byte, timeout time.Duration, params ...byte) ([]byte, error) {
	if len(params) > longLen-4 {
		return nil, fmt.Errorf("too many parameters: %d", len(params))
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	fnsw := fn<<4 | d.SoftwareID&0x0F
	req := make([]byte, longLen)
	req[0] = reportLong
	req[1] = d.Index
	req[2] = featIdx
	req[3] = fnsw
	copy(req[4:], params)
	d.drain()
	if _, err := unix.Write(d.fd, req); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	deadline := time.Now().Add(timeout)
	buf := make([]byte, 64)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, errors.New("timeout waiting for device")
		}
		fds := []unix.PollFd{{Fd: int32(d.fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int(remaining.Milliseconds())+1)
		if err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return nil, fmt.Errorf("poll: %w", err)
		}
		if n == 0 {
			continue
		}
		n, err = unix.Read(d.fd, buf)
		if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		msg := buf[:n]
		if len(msg) < 4 || msg[1] != d.Index {
			continue
		}
		if msg[0] != reportShort && msg[0] != reportLong {
			continue
		}
		switch {
		case msg[2] == 0xFF && msg[3] == featIdx && len(msg) > 5 && msg[4] == fnsw:
			// HID++ 2.0 error.
			return nil, &Error{Code: msg[5]}
		case msg[2] == 0x8F && msg[3] == featIdx && len(msg) > 5 && msg[4] == fnsw:
			// HID++ 1.0 error, e.g. a receiver rejecting a 2.0 request.
			return nil, &Error10{Code: msg[5]}
		case msg[2] == featIdx && msg[3] == fnsw:
			out := make([]byte, longLen-4)
			copy(out, msg[4:])
			return out, nil
		}
		// Anything else is unrelated; queue events and keep waiting.
		d.queue(msg)
	}
}

func (d *Device) queue(msg []byte) {
	// Events carry software ID 0; anything else is a stray reply.
	if msg[3]&0x0F != 0 || msg[2] == 0xFF || msg[2] == 0x8F {
		return
	}
	data := make([]byte, longLen-4)
	copy(data, msg[4:])
	if len(d.pending) < 64 {
		d.pending = append(d.pending, Notification{FeatureIndex: msg[2], Function: msg[3] >> 4, Data: data})
	}
}

// NextNotification waits up to timeout for a device event. It returns nil
// without error when nothing arrived.
func (d *Device) NextNotification(timeout time.Duration) (*Notification, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.pending) == 0 {
		fds := []unix.PollFd{{Fd: int32(d.fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int(timeout.Milliseconds()))
		if err != nil && !errors.Is(err, unix.EINTR) {
			return nil, fmt.Errorf("poll: %w", err)
		}
		if n > 0 {
			if fds[0].Revents&(unix.POLLERR|unix.POLLHUP) != 0 {
				return nil, errors.New("device disconnected")
			}
			buf := make([]byte, 64)
			n, err := unix.Read(d.fd, buf)
			if err != nil {
				return nil, fmt.Errorf("read: %w", err)
			}
			if msg := buf[:n]; len(msg) >= 4 && msg[1] == d.Index && (msg[0] == reportShort || msg[0] == reportLong) {
				d.queue(msg)
			}
		}
	}
	if len(d.pending) == 0 {
		return nil, nil
	}
	n := d.pending[0]
	d.pending = d.pending[1:]
	return &n, nil
}
