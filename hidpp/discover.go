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
					vid, _ := strconv.ParseUint(parts[1], 16, 32)
					pid, _ := strconv.ParseUint(parts[2], 16, 32)
					info.VendorID, info.ProductID = uint16(vid), uint16(pid)
				}
			}
		}
		if info.VendorID != VendorLogitech || isWebcam(info.ProductID) {
			continue
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
// report (ID 0x11) in a vendor collection. Most devices use usage page
// 0xFF00; older G keyboards (G213, G512/G513, G815, G910, G Pro) use 0xFF43.
func hasLongReport(desc []byte) bool {
	for _, page := range [][]byte{{0x06, 0x00, 0xFF}, {0x06, 0x43, 0xFF}} {
		if i := bytes.Index(desc, page); i >= 0 && bytes.Contains(desc[i:], []byte{0x85, reportLong}) {
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
