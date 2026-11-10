// Package report builds a text report of a device, to add support for it.
// It only calls functions that are known to only read, and leaves out the
// unit ID, the name the user gave the device and the hidraw path.
package report

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/mevdschee/logirgb/hidpp"
	"github.com/mevdschee/logirgb/internal/app"
)

// IssueURL is where a report can be sent.
const IssueURL = "https://github.com/mevdschee/logirgb/issues/new"

// Reads lists the functions per feature that only read. Add a function
// here only when it is known to only read.
var Reads = map[uint16][]byte{
	0x0001: {0, 1},
	0x0003: {0, 1},
	0x0005: {2},
	0x1000: {0},
	0x1001: {0},
	0x1004: {0, 1},
	0x1B04: {0, 1},
	0x1F20: {0, 1},
	0x2201: {0, 1, 2},
	0x4540: {0},
	0x8010: {0},
	0x8020: {0},
	0x8040: {0, 1},
	0x8060: {0, 1},
	0x8070: {0, 1, 2},
	0x8071: {0},
	0x8081: {0},
	0x8100: {0, 2, 4, 5, 0xB},
	0x8101: {8, 0xC},
	0x8300: {0},
	0x8310: {2},
}

var names = map[uint16]string{
	0x0001: "feature set", 0x0003: "firmware", 0x0005: "name", 0x0007: "friendly name",
	0x0020: "config change", 0x1000: "battery", 0x1001: "battery voltage", 0x1004: "battery",
	0x1300: "LED control", 0x1814: "change host", 0x1815: "hosts info", 0x1B04: "controls",
	0x1D4B: "wireless status", 0x1F20: "battery and auto-off", 0x2201: "DPI", 0x4540: "keyboard layout",
	0x8010: "G keys", 0x8020: "M keys", 0x8030: "MR key", 0x8040: "brightness", 0x8060: "report rate",
	0x8070: "LED effects", 0x8071: "RGB effects", 0x8080: "per-key lighting", 0x8081: "per-key lighting",
	0x8100: "onboard profiles", 0x8101: "onboard files", 0x8110: "button spy", 0x8300: "sidetone", 0x8310: "equalizer",
}

type reader struct {
	dev    *hidpp.Device
	b      strings.Builder
	err    string
	misses int
}

// ask calls a known read. After two timeouts in a row it stops asking.
func (r *reader) ask(feature uint16, fn byte, params ...byte) []byte {
	if !slices.Contains(Reads[feature], fn) {
		panic(fmt.Sprintf("not a known read: feature %04X fn %d", feature, fn))
	}
	r.err = ""
	if r.misses >= 2 {
		r.err = "no answer"
		return nil
	}
	out, err := r.dev.Call(feature, fn, params...)
	var e *hidpp.Error
	switch {
	case err == nil:
		r.misses = 0
		return out
	case errors.As(err, &e):
		r.misses = 0
		r.err = fmt.Sprintf("error %02X", e.Code)
	case errors.Is(err, hidpp.ErrNotSupported):
		r.err = "missing"
	case strings.Contains(err.Error(), "timeout"):
		r.misses++
		r.err = "no answer"
	default:
		r.err = err.Error()
	}
	return nil
}

func (r *reader) show(label string, feature uint16, fn byte, params ...byte) []byte {
	out := r.ask(feature, fn, params...)
	if out == nil {
		r.line(label + "=" + r.err)
	} else {
		r.line(label + "=" + hex(out, " "))
	}
	return out
}

func (r *reader) line(s string) { r.b.WriteString(s + "\n") }

func (r *reader) row(name, value string) {
	if value == "" {
		value = "-"
	}
	r.line(fmt.Sprintf("%-10s%s", name, value))
}

// Build reads the device and returns the report.
func Build(e *app.Entry) string {
	r := &reader{dev: e.Dev}
	r.line("logirgb device report")
	r.line("logirgb " + version() + " · " + runtime.GOOS + "/" + runtime.GOARCH + " · " + kernel())
	r.line("")
	r.row("Device", e.Name)
	r.row("Kind", e.Kind)
	r.row("Link", link(e))
	r.row("Driver", e.Info.Driver)
	if e.Dev == nil {
		r.row("Status", fmt.Sprint(e.Err))
		return r.b.String()
	}
	r.row("Model", e.Dev.Models())
	r.row("Firmware", e.Dev.Firmware())
	battery := ""
	if e.Battery != nil {
		battery = e.Battery.String()
		if e.Battery.Millivolts > 0 {
			battery += fmt.Sprintf(" · %d mV", e.Battery.Millivolts)
		}
	}
	r.row("Battery", battery)

	for _, id := range r.features() {
		if _, ok := Reads[id]; !ok || id == 0x0001 {
			continue
		}
		r.line("")
		r.line(fmt.Sprintf("%04X %s", id, names[id]))
		r.section(e, id)
	}
	return r.b.String()
}

func link(e *app.Entry) string {
	var parts []string
	switch e.Info.Bus {
	case hidpp.BusBluetooth:
		parts = append(parts, "Bluetooth")
	case hidpp.BusUSB:
		parts = append(parts, "USB")
	default:
		parts = append(parts, fmt.Sprintf("bus %02X", e.Info.Bus))
	}
	parts = append(parts, fmt.Sprintf("%04X:%04X", e.Info.VendorID, e.Info.ProductID))
	if s := e.Slot(); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, " · ")
}

func (r *reader) features() []uint16 {
	r.line("")
	count := r.ask(0x0001, 0)
	if count == nil {
		r.line("Features: " + r.err)
		return nil
	}
	r.line("Features")
	var ids []uint16
	for i := 1; i <= int(count[0]); i++ {
		x := r.ask(0x0001, 1, byte(i))
		if x == nil {
			r.line(fmt.Sprintf("%02X %s", i, r.err))
			continue
		}
		id := uint16(x[0])<<8 | uint16(x[1])
		ids = append(ids, id)
		var flags []string
		for bit, f := range map[byte]string{0x80: "obsolete", 0x40: "hidden", 0x20: "internal"} {
			if x[2]&bit != 0 {
				flags = append(flags, f)
			}
		}
		slices.Sort(flags)
		desc := strings.TrimSpace(names[id] + " " + strings.Join(flags, " "))
		r.line(strings.TrimRight(fmt.Sprintf("%02X %04X v%-2d %s", i, id, x[3], desc), " "))
	}
	return ids
}

func (r *reader) section(e *app.Entry, id uint16) {
	switch id {
	case 0x0003:
		c := r.ask(id, 0)
		for n := 0; c != nil && n < min(int(c[0]), 8); n++ {
			x := r.ask(id, 1, byte(n))
			if x == nil {
				r.line(fmt.Sprintf("entity %d=%s", n, r.err))
				continue
			}
			for i := 11; i < len(x); i++ {
				x[i] = 0
			}
			r.line(fmt.Sprintf("entity %d=%s", n, hex(x, " ")))
		}
	case 0x0005:
		r.show("type", id, 2)
	case 0x1000:
		r.show("level", id, 0)
	case 0x1001:
		r.show("voltage", id, 0)
	case 0x1004:
		r.show("capabilities", id, 0)
		r.show("status", id, 1)
	case 0x1B04:
		c := r.show("count", id, 0)
		for i := 0; c != nil && i < min(int(c[0]), 48); i++ {
			r.show(fmt.Sprintf("control %d", i), id, 1, byte(i))
		}
	case 0x1F20:
		r.show("battery", id, 0)
		r.show("auto-off", id, 1)
	case 0x2201:
		r.show("sensors", id, 0)
		r.show("dpi list", id, 1, 0)
		r.show("dpi", id, 2, 0)
	case 0x4540:
		r.show("layout", id, 0)
	case 0x8010, 0x8020:
		r.show("count", id, 0)
	case 0x8040:
		r.show("info", id, 0)
		r.show("now", id, 1)
	case 0x8060:
		r.show("rates", id, 0)
		r.show("now", id, 1)
	case 0x8070:
		info := r.show("info", id, 0)
		for z := 0; info != nil && z < min(int(info[0]), 8); z++ {
			zone := r.show(fmt.Sprintf("zone %d", z), id, 1, byte(z), 0xFF, 0)
			for n := 0; zone != nil && n < min(int(zone[3]), 32); n++ {
				r.show(fmt.Sprintf("zone %d effect %d", z, n), id, 2, byte(z), byte(n), 0xFF, 0)
			}
		}
	case 0x8071:
		r.show("info", id, 0, 0xFF, 0xFF, 0)
		for c := 0; c < 8; c++ {
			n := 0
			for ; n < 32; n++ {
				x := r.ask(id, 0, byte(c), byte(n), 0)
				if x == nil {
					break
				}
				r.line(fmt.Sprintf("cluster %d effect %d=%s", c, n, hex(x, " ")))
			}
			if n == 0 {
				break
			}
		}
	case 0x8081:
		var lit []string
		for page := 0; page < 3; page++ {
			x := r.show(fmt.Sprintf("page %d", page), id, 0, 0, byte(page), 0)
			for i := 0; x != nil && i < 14; i++ {
				for bit := 0; bit < 8; bit++ {
					if x[2+i]>>bit&1 != 0 {
						lit = append(lit, fmt.Sprintf("%02X", page*112+i*8+bit))
					}
				}
			}
		}
		r.line("lit ids=" + strings.Join(lit, " "))
	case 0x8100:
		r.profiles()
	case 0x8101:
		files := []int{0x0000, 16, 0x0100, 16}
		if strings.Contains(strings.ToUpper(e.Name), "G515") {
			files = []int{0x0000, 16, 0x0008, 99, 0x0100, 16, 0x0101, 99}
		}
		for i := 0; i < len(files); i += 2 {
			r.file(files[i], files[i+1])
		}
	case 0x8300:
		r.show("sidetone", id, 0)
	case 0x8310:
		r.show("bands", id, 2)
	}
}

func (r *reader) profiles() {
	desc := r.show("info", 0x8100, 0)
	r.show("mode", 0x8100, 2)
	r.show("current", 0x8100, 4)
	r.show("dpi index", 0x8100, 0xB)
	if desc == nil {
		return
	}
	size := int(desc[7])<<8 | int(desc[8])
	if size < 16 || size > 0x1000 {
		r.line(fmt.Sprintf("sector size %d, not read", size))
		return
	}
	dir := r.sector(0x0000, size)
	var sectors []int
	for i := 0; dir != nil && i+1 < len(dir) && len(sectors) < 8; i += 4 {
		at := int(dir[i])<<8 | int(dir[i+1])
		if at == 0xFFFF {
			break
		}
		if at > 0 && at < 0x100 && !slices.Contains(sectors, at) {
			sectors = append(sectors, at)
		}
	}
	for _, at := range sectors {
		r.sector(at, size)
	}
	r.sector(0x0100, size)
	for k := 1; k <= min(int(desc[4]), 8); k++ {
		r.sector(0x0100+k, size)
	}
}

func (r *reader) sector(sector, size int) []byte {
	p := make([]byte, size)
	for off := 0; off < size; off += 16 {
		at := min(off, size-16)
		x := r.ask(0x8100, 5, byte(sector>>8), byte(sector), byte(at>>8), byte(at))
		if x == nil {
			r.line(fmt.Sprintf("%04X=%s", sector, r.err))
			return nil
		}
		copy(p[off:], x[off-at:])
	}
	r.line(fmt.Sprintf("%04X=%s", sector, hex(p, "")))
	return p
}

func (r *reader) file(addr, size int) {
	if r.ask(0x8101, 8, byte(addr>>8), byte(addr), byte(size>>8), byte(size)) == nil {
		r.line(fmt.Sprintf("%04X=%s", addr, r.err))
		return
	}
	var out []byte
	for off := 0; off < size; off += 16 {
		x := r.ask(0x8101, 0xC, byte(off>>8), byte(off))
		if x == nil {
			r.line(fmt.Sprintf("%04X=%s", addr, r.err))
			return
		}
		out = append(out, x...)
	}
	r.line(fmt.Sprintf("%04X=%s", addr, hex(out[:size], "")))
}

func hex(b []byte, gap string) string {
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = fmt.Sprintf("%02X", c)
	}
	return strings.Join(parts, gap)
}

func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "?"
}

func kernel() string {
	b, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	if s := strings.TrimSpace(string(b)); s != "" {
		return "Linux " + s
	}
	return "Linux"
}
