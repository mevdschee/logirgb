// Command logirgb sets the RGB lighting of Logitech HID++ devices on Linux.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mevdschee/logirgb/hidpp"
	"github.com/mevdschee/logirgb/internal/app"
	"github.com/mevdschee/logirgb/internal/state"
	"github.com/mevdschee/logirgb/internal/tui"
	"github.com/mevdschee/logirgb/internal/udev"
	"github.com/mevdschee/logirgb/lighting"
)

const usage = `logirgb - control the RGB lighting of Logitech devices

Usage:
  logirgb                       open the interactive TUI
  logirgb list                  list devices, zones, effects and settings
  logirgb set [flags] <effect>  set a lighting effect and store it on the device
  logirgb brightness [-d dev] <value>
  logirgb timeouts [-d dev] [--idle s] [--sleep s] [--dim percent]
  logirgb release [-d dev]      drop a preview and show the stored lighting
  logirgb profiles [-d dev] [--enable n,..] [--disable n,..]
                                show or choose the profiles the profile button cycles through
  logirgb reset-profiles [-d dev]  go back to the factory onboard profiles
  logirgb udev-rule             print the udev rule granting device access

Devices (-d) are selected by list number, hidraw path or name fragment
such as "g515". Without -d every usable device is changed.

Settings are read from the devices and written to their onboard memory;
set and timeouts take --preview to only show a change until the next
profile switch or power cycle.

Run "logirgb <command> -h" for the flags of a command.
`

func main() {
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()
	args := flag.Args()
	cmd := ""
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}

	var err error
	switch cmd {
	case "":
		err = tui.Run()
	case "list", "ls":
		err = cmdList(args)
	case "set":
		err = cmdSet(args)
	case "brightness":
		err = cmdBrightness(args)
	case "timeouts":
		err = cmdTimeouts(args)
	case "release":
		err = cmdRelease(args)
	case "profiles":
		err = cmdProfiles(args)
	case "reset-profiles":
		err = cmdResetProfiles(args)
	case "udev-rule":
		fmt.Print(udev.Rule)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// scan discovers devices and warns about the ones we cannot access.
func scan() ([]*app.Entry, error) {
	entries, err := app.Scan(hidpp.DefaultSoftwareID)
	if err != nil {
		return nil, err
	}
	if denied := app.PermissionProblems(entries); len(denied) > 0 {
		fmt.Fprintln(os.Stderr, app.PermissionHelp(denied))
	}
	if len(entries) == 0 {
		return nil, errors.New("no Logitech HID++ devices found")
	}
	return entries, nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	fs.Parse(args)
	entries, err := scan()
	if err != nil {
		return err
	}
	defer app.Close(entries)

	for i, e := range entries {
		fmt.Printf("%d. %s  [%s, %s]\n", i+1, e.Name, e.Info.Path, e.Key)
		if !e.Usable() {
			fmt.Printf("   unavailable: %v\n", e.Err)
			continue
		}
		ctl := e.Ctl
		saved, err := app.Load(e)
		if err != nil {
			fmt.Printf("   reading the onboard memory failed: %v\n", err)
		}
		for zi, z := range ctl.Zones() {
			fmt.Printf("   zone %d (%s)", zi, z.Location)
			if z.Description != "" {
				fmt.Printf(": %s", z.Description)
			}
			if eff, ok := saved.Zone(zi); ok {
				fmt.Printf("  stored: %s", describe(z, eff))
			}
			fmt.Println()
			for _, p := range saved.ProfileNumbers() {
				if eff, ok := saved.Profiles[strconv.Itoa(p)][strconv.Itoa(zi)]; ok {
					fmt.Printf("     profile %d: %s\n", p, describe(z, eff))
				}
			}
			for _, ze := range z.Effects {
				var ps []string
				for _, p := range ze.Type.Params() {
					ps = append(ps, string(p.Name))
				}
				fmt.Printf("     - %-10s %s\n", ze.Type.Name, strings.Join(ps, ", "))
			}
		}
		if ctl.HasBrightness() {
			lo, hi, _ := ctl.BrightnessRange()
			v, err := ctl.Brightness()
			if err == nil {
				fmt.Printf("   brightness: %d (range %d-%d)\n", v, lo, hi)
			}
		}
		if ctl.HasTimers() {
			idle, sleep, err := ctl.Timers()
			if err == nil {
				fmt.Printf("   firmware timers: idle %ds, sleep %ds\n", idle, sleep)
			}
		}
	}
	return nil
}

func describe(z lighting.Zone, eff lighting.Effect) string {
	ze, ok := z.Find(eff.Name)
	if !ok {
		return eff.Name
	}
	parts := []string{eff.Name}
	for _, p := range ze.Type.Params() {
		v := eff.Get(ze.Type, p.Name)
		parts = append(parts, fmt.Sprintf("%s=%s", p.Name, p.Format(v)))
	}
	return strings.Join(parts, " ")
}

func optSeconds(v *int) string {
	if v == nil {
		return "unset"
	}
	if *v == 0 {
		return "off"
	}
	return fmt.Sprintf("%ds", *v)
}

func cmdSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	dev := fs.String("d", "", "device")
	zone := fs.Int("z", -1, "zone number (default: all zones)")
	profile := fs.Int("p", 0, "onboard profile number, for devices that store lighting per profile (default: all profiles)")
	color := fs.String("color", "", "color: #rrggbb, r,g,b or a name like red")
	color2 := fs.String("color2", "", "second color (starlight)")
	period := fs.Int("period", -1, "effect period in ms")
	speed := fs.Int("speed", -1, "pulse speed 0-255")
	intensity := fs.Int("intensity", -1, "intensity 1-100")
	saturation := fs.Int("saturation", -1, "saturation 0-255")
	direction := fs.String("direction", "", "wave direction: right, left, up, down, center-out, center-in, ...")
	form := fs.String("form", "", "breathe waveform: sine, square, triangle, sawtooth, shark-fin, exponential")
	preview := fs.Bool("preview", false, "only show the lighting, do not store it in onboard memory")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: logirgb set [flags] <off|static|pulse|breathe|cycle|wave|ripple|starlight>")
		fs.PrintDefaults()
	}
	pos := parseArgs(fs, args)
	if len(pos) != 1 {
		fs.Usage()
		os.Exit(2)
	}
	eff := lighting.Effect{Name: strings.ToLower(pos[0])}
	if eff.Name == "color" || eff.Name == "solid" {
		eff.Name = "static"
	}
	for _, c := range []struct {
		s string
		p lighting.Param
	}{{*color, lighting.ParamColor}, {*color2, lighting.ParamColor2}} {
		if c.s != "" {
			v, err := lighting.ParseColor(c.s)
			if err != nil {
				return err
			}
			eff.Set(c.p, v)
		}
	}
	for _, n := range []struct {
		v int
		p lighting.Param
	}{{*period, lighting.ParamPeriod}, {*speed, lighting.ParamSpeed}, {*intensity, lighting.ParamIntensity}, {*saturation, lighting.ParamSaturation}} {
		if n.v >= 0 {
			eff.Set(n.p, n.v)
		}
	}
	for _, c := range []struct {
		s string
		p lighting.Param
	}{{*direction, lighting.ParamDirection}, {*form, lighting.ParamForm}} {
		if c.s != "" {
			v, err := lighting.ParseChoice(c.p, c.s)
			if err != nil {
				return err
			}
			eff.Set(c.p, v)
		}
	}

	entries, err := scan()
	if err != nil {
		return err
	}
	defer app.Close(entries)
	targets, err := app.Select(entries, *dev)
	if err != nil {
		return err
	}
	for _, e := range targets {
		zones := []int{*zone}
		if *zone < 0 {
			zones = zones[:0]
			for i := range e.Ctl.Zones() {
				zones = append(zones, i)
			}
		}
		if err := e.Ctl.Claim(); err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
		if *profile > 0 && !e.Ctl.HasOnboardProfiles() {
			return fmt.Errorf("%s: no per profile lighting", e.Name)
		}
		d, err := app.Load(e)
		if err != nil {
			return fmt.Errorf("%s: read onboard memory: %w", e.Name, err)
		}
		for _, z := range zones {
			if err := e.Ctl.SetEffect(z, eff); err != nil {
				return fmt.Errorf("%s: %w", e.Name, err)
			}
			where := fmt.Sprintf("zone %d", z)
			if *profile > 0 {
				d.SetProfileZone(*profile, z, eff)
				where = fmt.Sprintf("profile %d %s", *profile, where)
			} else if e.Ctl.HasOnboardProfiles() {
				d.SetAllProfilesZone(z, eff)
			} else {
				d.SetZone(z, eff)
			}
			fmt.Printf("%s %s: %s\n", e.Name, where, describe(e.Ctl.Zones()[z], eff))
		}
		if err := store(e, d, *preview); err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
	}
	return nil
}

func cmdBrightness(args []string) error {
	fs := flag.NewFlagSet("brightness", flag.ExitOnError)
	dev := fs.String("d", "", "device")
	pos := parseArgs(fs, args)
	if len(pos) != 1 {
		return errors.New("usage: logirgb brightness [-d dev] <value>")
	}
	v, err := strconv.Atoi(strings.TrimSuffix(pos[0], "%"))
	if err != nil {
		return fmt.Errorf("invalid brightness %q", pos[0])
	}
	return eachDevice(*dev, func(e *app.Entry) error {
		if !e.Ctl.HasBrightness() {
			fmt.Printf("%s: no brightness control, skipped\n", e.Name)
			return nil
		}
		lo, hi, err := e.Ctl.BrightnessRange()
		if err != nil {
			return err
		}
		if v < lo || v > hi {
			return fmt.Errorf("brightness must be %d-%d", lo, hi)
		}
		if err := e.Ctl.SetBrightness(v); err != nil {
			return err
		}
		fmt.Printf("%s: brightness %d\n", e.Name, v)
		return nil
	})
}

func cmdTimeouts(args []string) error {
	fs := flag.NewFlagSet("timeouts", flag.ExitOnError)
	dev := fs.String("d", "", "device")
	idle := fs.Int("idle", -1, "seconds of inactivity before dimming, 0 disables")
	sleep := fs.Int("sleep", -1, "seconds of inactivity before the lighting turns off, 0 disables")
	dim := fs.Int("dim", -1, "brightness while dimmed, in percent of the normal lighting")
	preview := fs.Bool("preview", false, "only set the running timers, do not store them in onboard memory")
	fs.Parse(args)
	return eachDevice(*dev, func(e *app.Entry) error {
		if !e.Ctl.HasTimers() {
			fmt.Printf("%s: no idle timers, skipped\n", e.Name)
			return nil
		}
		d, err := app.Load(e)
		if err != nil {
			return fmt.Errorf("read onboard memory: %w", err)
		}
		if *idle >= 0 {
			d.IdleTimeout = state.IntPtr(*idle)
		}
		if *sleep >= 0 {
			d.SleepTimeout = state.IntPtr(*sleep)
		}
		if *dim >= 0 {
			if *dim > 100 {
				return fmt.Errorf("dim must be 0-100")
			}
			d.DimPercent = state.IntPtr(*dim)
		}
		if err := app.ApplyTimers(e, d); err != nil {
			return err
		}
		fmt.Printf("%s: dim to %s after %s, off after %s\n", e.Name, dimPercent(d), optSeconds(d.IdleTimeout), optSeconds(d.SleepTimeout))
		return store(e, d, *preview)
	})
}

func cmdRelease(args []string) error {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	dev := fs.String("d", "", "device")
	fs.Parse(args)
	return eachDevice(*dev, func(e *app.Entry) error {
		if err := e.Ctl.Release(); err != nil {
			return err
		}
		fmt.Printf("%s: lighting returned to the onboard profile\n", e.Name)
		return nil
	})
}

// store writes the settings to the device's onboard memory, unless only a
// preview was asked for or the device has no onboard memory.
func store(e *app.Entry, d *state.Device, preview bool) error {
	if preview {
		fmt.Printf("%s: preview only, not stored\n", e.Name)
		return nil
	}
	if !e.Ctl.HasOnboardProfiles() && !e.Ctl.HasProfileStorage() {
		fmt.Printf("%s: no onboard memory, the change lasts until the next power cycle\n", e.Name)
		return nil
	}
	n, err := app.SaveToProfiles(e, d)
	if err != nil {
		return err
	}
	if e.Ctl.HasProfileStorage() {
		fmt.Printf("%s: stored; the keyboard now dims and turns off by itself\n", e.Name)
	} else {
		fmt.Printf("%s: stored in %d onboard profiles\n", e.Name, n)
	}
	return nil
}

func dimPercent(d *state.Device) string {
	v := app.DefaultDimPercent
	if d.DimPercent != nil {
		v = *d.DimPercent
	}
	return fmt.Sprintf("%d%%", v)
}

func cmdProfiles(args []string) error {
	fs := flag.NewFlagSet("profiles", flag.ExitOnError)
	dev := fs.String("d", "", "device")
	enable := fs.String("enable", "", "profiles the profile button cycles to, e.g. 1,2")
	disable := fs.String("disable", "", "profiles the profile button skips, e.g. 3,4,5")
	fs.Parse(args)
	on, err := profileList(*enable)
	if err != nil {
		return err
	}
	off, err := profileList(*disable)
	if err != nil {
		return err
	}
	return eachDevice(*dev, func(e *app.Entry) error {
		if !e.Ctl.HasOnboardProfiles() {
			fmt.Printf("%s: no onboard profiles, skipped\n", e.Name)
			return nil
		}
		d, err := app.Load(e)
		if err != nil {
			return fmt.Errorf("read onboard memory: %w", err)
		}
		count, err := e.Ctl.ProfileCount()
		if err != nil {
			return err
		}
		for _, p := range append(on, off...) {
			if p > count {
				return fmt.Errorf("no profile %d (device has %d)", p, count)
			}
		}
		active, _ := e.Ctl.ActiveProfile()
		for _, p := range on {
			d.SetEnabled(p, true)
		}
		for _, p := range off {
			if p == active {
				return fmt.Errorf("profile %d is active and cannot be disabled", p)
			}
			d.SetEnabled(p, false)
		}
		var ps []string
		for p := 1; p <= count; p++ {
			v := fmt.Sprintf("%d on", p)
			if !d.Enabled(p) {
				v = fmt.Sprintf("%d off", p)
			}
			if p == active {
				v += " (active)"
			}
			ps = append(ps, v)
		}
		fmt.Printf("%s profiles: %s\n", e.Name, strings.Join(ps, ", "))
		if len(on) == 0 && len(off) == 0 {
			return nil
		}
		return store(e, d, false)
	})
}

// profileList parses a comma separated list of profile numbers.
func profileList(s string) ([]int, error) {
	var out []int
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		n, err := strconv.Atoi(f)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid profile %q", f)
		}
		out = append(out, n)
	}
	return out, nil
}

func cmdResetProfiles(args []string) error {
	fs := flag.NewFlagSet("reset-profiles", flag.ExitOnError)
	dev := fs.String("d", "", "device")
	fs.Parse(args)
	return eachDevice(*dev, func(e *app.Entry) error {
		if !e.Ctl.HasOnboardProfiles() {
			fmt.Printf("%s: not supported, skipped\n", e.Name)
			return nil
		}
		if err := e.Ctl.ResetProfiles(); err != nil {
			return err
		}
		fmt.Printf("%s: back on the factory profiles\n", e.Name)
		return nil
	})
}

// eachDevice runs fn for the selected devices.
func eachDevice(sel string, fn func(*app.Entry) error) error {
	entries, err := scan()
	if err != nil {
		return err
	}
	defer app.Close(entries)
	targets, err := app.Select(entries, sel)
	if err != nil {
		return err
	}
	for _, e := range targets {
		if err := fn(e); err != nil {
			return fmt.Errorf("%s: %w", e.Name, err)
		}
	}
	return nil
}

// parseArgs parses flags that may appear before or after positional
// arguments and returns the positional ones.
func parseArgs(fs *flag.FlagSet, args []string) []string {
	var pos []string
	for {
		fs.Parse(args)
		if fs.NArg() == 0 {
			return pos
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}
