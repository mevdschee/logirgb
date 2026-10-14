// Package app ties discovery, permissions and lighting control together for
// the CLI and TUI front ends.
package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/mevdschee/logirgb/hidpp"
	"github.com/mevdschee/logirgb/internal/config"
	"github.com/mevdschee/logirgb/internal/udev"
	"github.com/mevdschee/logirgb/lighting"
)

// ErrPermission marks a device the current user cannot open.
var ErrPermission = errors.New("no permission")

// Entry is one discovered device.
type Entry struct {
	Info hidpp.Info
	Name string
	Key  string
	Dev  *hidpp.Device
	Ctl  *lighting.Controller
	Err  error
}

// Usable reports whether the device can be controlled.
func (e *Entry) Usable() bool { return e.Ctl != nil }

// Scan discovers Logitech HID++ devices and opens those with lighting.
// Devices that cannot be opened are returned with Err set.
func Scan(softwareID byte) ([]*Entry, error) {
	infos, err := hidpp.Discover()
	if err != nil {
		return nil, err
	}
	var out []*Entry
	for _, info := range infos {
		e := &Entry{Info: info, Name: info.Name, Key: config.Key(info.VendorID, info.ProductID)}
		out = append(out, e)
		if unix.Access(info.Path, unix.R_OK|unix.W_OK) != nil {
			e.Err = ErrPermission
			continue
		}
		dev, err := hidpp.Open(info)
		if err != nil {
			e.Err = err
			continue
		}
		dev.SoftwareID = softwareID
		e.Dev = dev
		e.Name = dev.Name()
		ctl, err := lighting.New(dev)
		if err != nil {
			e.Err = err
			continue
		}
		e.Ctl = ctl
	}
	return out, nil
}

// Close releases all opened devices.
func Close(entries []*Entry) {
	for _, e := range entries {
		if e.Dev != nil {
			e.Dev.Close()
		}
	}
}

// Select picks devices by list number (1-based), hidraw path or a case
// insensitive name fragment such as "g515". An empty selector picks all
// usable devices.
func Select(entries []*Entry, sel string) ([]*Entry, error) {
	var out []*Entry
	if sel == "" {
		for _, e := range entries {
			if e.Usable() {
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
		if !e.Usable() {
			return nil, fmt.Errorf("%s (%s): %v", e.Name, e.Info.Path, e.Err)
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
