package hidpp

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// VendorLogitech is Logitech's USB vendor ID.
const VendorLogitech = 0x046D

// Info describes a hidraw node that carries HID++ traffic.
type Info struct {
	Path      string // /dev/hidrawN
	Name      string // kernel HID name
	VendorID  uint16
	ProductID uint16
	// Bus is the HID bus type: 0x03 USB, 0x05 Bluetooth.
	Bus uint16
	// Driver is the kernel HID driver bound to the node, such as
	// "logitech-djreceiver" for a receiver whose paired devices the kernel
	// gives nodes of their own.
	Driver string
}

// Bus types from HID_ID.
const (
	BusUSB       = 0x03
	BusBluetooth = 0x05
)

// DJReceiver reports whether the kernel's hid-logitech-dj driver handles
// the node, so the devices paired with it also get nodes of their own.
func (i Info) DJReceiver() bool {
	return i.Driver == "logitech-djreceiver"
}

// Discover lists Logitech hidraw nodes whose report descriptor declares the
// vendor specific HID++ long report (usage page 0xFF00, report ID 0x11).
func Discover() ([]Info, error) {
	nodes, err := filepath.Glob("/sys/class/hidraw/hidraw*")
	if err != nil {
		return nil, err
	}
	var out []Info
	for _, node := range nodes {
		uevent, err := os.ReadFile(filepath.Join(node, "device", "uevent"))
		if err != nil {
			continue
		}
		info := Info{Path: "/dev/" + filepath.Base(node)}
		for _, line := range strings.Split(string(uevent), "\n") {
			k, v, _ := strings.Cut(line, "=")
			switch k {
			case "HID_NAME":
				info.Name = v
			case "HID_ID":
				// bus:vendor:product, all hex.
				parts := strings.Split(v, ":")
				if len(parts) == 3 {
					bus, _ := strconv.ParseUint(parts[0], 16, 32)
					vid, _ := strconv.ParseUint(parts[1], 16, 32)
					pid, _ := strconv.ParseUint(parts[2], 16, 32)
					info.Bus, info.VendorID, info.ProductID = uint16(bus), uint16(vid), uint16(pid)
				}
			}
		}
		if info.VendorID != VendorLogitech || isWebcam(info.ProductID) {
			continue
		}
		if drv, err := os.Readlink(filepath.Join(node, "device", "driver")); err == nil {
			info.Driver = filepath.Base(drv)
		}
		desc, err := os.ReadFile(filepath.Join(node, "device", "report_descriptor"))
		if err != nil || !hasLongReport(desc) {
			continue
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// hasLongReport reports whether the descriptor declares the HID++ long
// report (ID 0x11) in a vendor collection (usage page 0xFFxx). Most devices
// use usage page 0xFF00; older G keyboards (G213, G512/G513, G815, G910,
// G Pro) use 0xFF43.
func hasLongReport(desc []byte) bool {
	for i := 0; i+2 < len(desc); i++ {
		if desc[i] == 0x06 && desc[i+2] == 0xFF && bytes.Contains(desc[i+3:], []byte{0x85, reportLong}) {
			return true
		}
	}
	return false
}

// isWebcam excludes Logitech webcams, which can lock up when probed with
// HID++ (product IDs 0x0800-0x09FF).
func isWebcam(pid uint16) bool {
	return pid >= 0x0800 && pid <= 0x09FF
}
