// Package app ties discovery, permissions and lighting control together for
// the CLI and TUI front ends.
package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/mevdschee/logirgb/headset"
	"github.com/mevdschee/logirgb/hidpp"
	"github.com/mevdschee/logirgb/internal/state"
	"github.com/mevdschee/logirgb/internal/udev"
	"github.com/mevdschee/logirgb/lighting"
)

// ErrPermission marks a device the current user cannot open.
var ErrPermission = errors.New("no permission")

// ErrAsleep marks a device paired with a receiver that does not answer.
var ErrAsleep = errors.New("asleep or out of range")

// Entry is one discovered device.
type Entry struct {
	Info hidpp.Info
	// Index is the device's slot on a receiver, or 0xFF for a device
	// attached directly.
	Index   byte
	Name    string
	Key     string
	Kind    string
	Dev     *hidpp.Device
	Ctl     *lighting.Controller
	Headset *headset.Controller
	Battery *hidpp.Battery
	Err     error
}

// Usable reports whether the device has settings logirgb can change.
func (e *Entry) Usable() bool { return e.Ctl != nil || e.Headset != nil }

// Slot describes where a device behind a receiver sits, or "" for a device
// attached directly.
func (e *Entry) Slot() string {
	if e.Index == hidpp.DeviceIndexDirect {
		return ""
	}
	return fmt.Sprintf("slot %d", e.Index)
}

// Scan discovers Logitech HID++ devices, every paired device of a receiver
// included, and opens them. Devices that cannot be opened or have nothing
// to change are returned with Err set.
func Scan(softwareID byte) ([]*Entry, error) {
	infos, err := hidpp.Discover()
	if err != nil {
		return nil, err
	}
	var out []*Entry
	seen := map[uint32]bool{}
	// A receiver run by the kernel's hid-logitech-dj driver also gives each
	// paired device a node of its own; those come first, so the receiver
	// only adds what they lack.
	slices.SortStableFunc(infos, func(a, b hidpp.Info) int {
		return cmpBool(a.DJReceiver(), b.DJReceiver())
	})
	for _, info := range infos {
		key := state.Key(info.VendorID, info.ProductID)
		if unix.Access(info.Path, unix.R_OK|unix.W_OK) != nil {
			out = append(out, &Entry{Info: info, Index: hidpp.DeviceIndexDirect, Name: info.Name, Key: key, Err: ErrPermission})
			continue
		}
		slots, err := hidpp.OpenAll(info)
		if err != nil {
			if !info.DJReceiver() {
				out = append(out, &Entry{Info: info, Index: hidpp.DeviceIndexDirect, Name: info.Name, Key: key, Err: err})
			}
			continue
		}
		for _, slot := range slots {
			e := &Entry{Info: info, Index: slot.Index, Name: info.Name, Key: key}
			if slot.Dev == nil {
				if !info.DJReceiver() {
					e.Name = fmt.Sprintf("%s, %s", info.Name, e.Slot())
					e.Err = ErrAsleep
					out = append(out, e)
				}
				continue
			}
			dev := slot.Dev
			if id, err := dev.UnitID(); err == nil && id != 0 {
				if seen[id] {
					dev.Close()
					continue
				}
				seen[id] = true
			}
			out = append(out, e)
			open(e, dev, softwareID)
		}
	}
	return out, nil
}

func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

// open fills in what an opened device has.
func open(e *Entry, dev *hidpp.Device, softwareID byte) {
	dev.SoftwareID = softwareID
	e.Dev = dev
	e.Name = dev.Name()
	e.Kind = dev.Kind()
	e.Battery, _ = dev.Battery()
	ctl, err := lighting.New(dev)
	if err == nil {
		e.Ctl = ctl
	}
	if hs, herr := headset.New(dev); herr == nil {
		e.Headset = hs
	} else if e.Ctl == nil {
		if errors.Is(err, lighting.ErrNoLighting) {
			err = errors.New("no lighting or headset settings logirgb can change")
		}
		e.Err = err
	}
}

// Close releases all opened devices.
func Close(entries []*Entry) {
	for _, e := range entries {
		if e.Dev != nil {
			e.Dev.Close()
		}
	}
}

// Need is what a command needs a device to have.
type Need int

const (
	// NeedLighting asks for a device with lighting.
	NeedLighting Need = iota
	// NeedHeadset asks for a headset with sidetone or auto-off.
	NeedHeadset
)

// check returns why the entry does not have what is needed.
func (e *Entry) check(n Need) error {
	if e.Dev == nil {
		return e.Err
	}
	switch {
	case n == NeedLighting && e.Ctl == nil:
		return errors.New("no lighting logirgb can change")
	case n == NeedHeadset && e.Headset == nil:
		return errors.New("no headset settings")
	}
	return nil
}

// Select picks devices by list number (1-based), hidraw path or a case
// insensitive name fragment such as "g515". An empty selector picks all
// devices that have what is needed.
func Select(entries []*Entry, sel string, need Need) ([]*Entry, error) {
	var out []*Entry
	if sel == "" {
		for _, e := range entries {
			if e.check(need) == nil {
				out = append(out, e)
			}
		}
		if len(out) == 0 {
			return nil, errors.New("no usable devices found")
		}
		return out, nil
	}
	if n, err := strconv.Atoi(sel); err == nil {
		if n < 1 || n > len(entries) {
			return nil, fmt.Errorf("no device #%d", n)
		}
		out = append(out, entries[n-1])
	} else {
		for _, e := range entries {
			if e.Info.Path == sel || strings.Contains(strings.ToLower(e.Name), strings.ToLower(sel)) ||
				strings.Contains(strings.ToLower(e.Info.Name), strings.ToLower(sel)) {
				out = append(out, e)
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no device matches %q", sel)
	}
	for _, e := range out {
		if err := e.check(need); err != nil {
			return nil, fmt.Errorf("%s (%s): %v", e.Name, e.Info.Path, err)
		}
	}
	return out, nil
}

// PermissionProblems returns the entries that failed on permissions.
func PermissionProblems(entries []*Entry) []*Entry {
	var out []*Entry
	for _, e := range entries {
		if errors.Is(e.Err, ErrPermission) {
			out = append(out, e)
		}
	}
	return out
}

// PermissionHelp explains how to grant access to the given devices.
func PermissionHelp(denied []*Entry) string {
	if len(denied) == 0 {
		return ""
	}
	self := "logirgb"
	if exe, err := os.Executable(); err == nil && !strings.HasPrefix(exe, os.TempDir()) {
		self = exe
	}
	var b strings.Builder
	b.WriteString("No read/write access to:\n")
	var paths []string
	for _, e := range denied {
		fmt.Fprintf(&b, "  %s  %s\n", e.Info.Path, e.Name)
		paths = append(paths, e.Info.Path)
	}
	b.WriteString("\nPermanent fix (udev rule for the logged-in user):\n")
	fmt.Fprintf(&b, "  %s udev-rule | sudo tee /etc/udev/rules.d/%s >/dev/null\n", self, udev.RuleName)
	b.WriteString("  sudo udevadm control --reload-rules && sudo udevadm trigger --subsystem-match=hidraw --action=add\n")
	b.WriteString("\nTemporary fix (until the device is replugged):\n")
	fmt.Fprintf(&b, "  sudo setfacl -m u:%s:rw %s\n", currentUser(), strings.Join(paths, " "))
	return b.String()
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return strconv.Itoa(os.Getuid())
}

// UdevRuleInstalled reports whether the rule exists in /etc/udev/rules.d.
func UdevRuleInstalled() bool {
	_, err := os.Stat(filepath.Join("/etc/udev/rules.d", udev.RuleName))
	return err == nil
}
