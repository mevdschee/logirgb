# logitech-rgb

`logirgb` sets the RGB lighting of Logitech G devices on Linux. It talks
HID++ 2.0 directly over `/dev/hidraw`, so it needs no G HUB, Solaar, OpenRGB or
libusb. It has a TUI, a scriptable CLI and a small daemon that dims and
switches off the lighting when you are idle.

Tested with:

| Device | USB ID | Lighting feature | Zones | Brightness | Dim/off on idle |
|---|---|---|---|---|---|
| G515 TKL (wired) | 046d:c358 | 0x8071 RGBEffects | 1 | yes (0x8040) | yes |
| G502 HERO / G502 SE HERO | 046d:c08b | 0x8070 ColorLEDEffects + 0x8100 onboard profiles | 2 (primary, logo) | no | no |

On the G502 HERO the logo (zone 1) is the light you see. The firmware also
accepts settings for zone 0 ("primary"), but on the tested mouse that has no
visible effect: the DPI LEDs are a blue status light that the firmware only
shows when you press the profile/DPI buttons.

Other devices that expose 0x8070 or 0x8071 should work too. Devices are found
by scanning `/sys/class/hidraw` for Logitech nodes that declare the HID++ long
report.

## Install

```bash
go install github.com/mevdschee/logitech-rgb/cmd/logirgb@latest
```

### Permissions

hidraw nodes are usually only accessible to root. `logirgb` checks this on
startup and prints the commands to fix it. The permanent fix is a udev rule
that gives the logged-in user access to Logitech hidraw devices:

```bash
logirgb udev-rule | sudo tee /etc/udev/rules.d/70-logirgb.rules >/dev/null
sudo udevadm control --reload-rules && sudo udevadm trigger --subsystem-match=hidraw --action=add
```

## Usage

Run `logirgb` without arguments for the TUI: pick a device on the left, change
effect, colors, brightness and timeouts on the right. Changes are applied
immediately and saved.

| Key | Action |
|---|---|
| tab | switch between device list and settings |
| ↑ ↓ | select device or setting |
| ← → | change value (shift or H/L for bigger steps); cycles colors |
| enter | type a color (`#ff8000`, `255,128,0` or `orange`) |
| a | re-apply the saved settings |
| w | store the lighting in the onboard profiles (G502) |
| o | hand lighting back to the onboard profile |
| ctrl+r | rescan devices |
| q | quit |

The CLI does the same from scripts:

```bash
logirgb list                                     # devices, zones, effects, current settings
logirgb set static --color orange                # all zones of all devices
logirgb set -d g515 breathe --color blue --period 3000
logirgb set -d g502 -z 1 cycle --period 8000
logirgb set -d g515 wave --direction left --period 4000
logirgb brightness -d g515 50
logirgb timeouts -d g515 --idle 60 --sleep 300 --dim 13
logirgb apply                                    # re-apply saved settings
logirgb release                                  # back to the onboard profile
logirgb save -d g502                             # store lighting in the onboard profiles
logirgb reset-profiles -d g502                   # back to the factory profiles
```

Effects are `off`, `static`, `pulse`, `breathe`, `cycle`, `wave`, `ripple` and
`starlight`; `logirgb list` shows which ones each zone supports and which
parameters they take (`--color`, `--color2`, `--period`, `--speed`,
`--intensity`, `--saturation`, `--direction`, `--form`).

Settings are stored per device model in `~/.config/logirgb/config.json`.

### Idle dimming and sleep

The G515 firmware detects inactivity, but once software controls the lighting
it only reports idle and active events and leaves the dimming to the host.
`logirgb daemon` handles those events: after the idle timeout it lowers the
brightness to the dim level, after the sleep timeout it switches the lighting
off, and on the next key press it restores everything. It also re-applies
your settings when a device is plugged in, since the lighting resets on power
loss.

Run it as a systemd user service:

```bash
cp contrib/logirgb.service ~/.config/systemd/user/
systemctl --user enable --now logirgb
```

The G502 HERO has no idle reporting, so it keeps the same lighting all the time.

### What is stored where

`set`, `brightness` and `timeouts` write to the device's RAM. On the G515,
`logirgb release` (or `o` in the TUI) gives control back to the onboard
profile, and after a power cycle the keyboard shows its onboard lighting
until `logirgb apply` or the daemon runs.

The G502 HERO re-applies the lighting of its onboard profile whenever you
switch profiles (the button behind the scroll wheel) or replug it.
`logirgb save` (or `w` in the TUI) therefore writes the configured lighting
into all onboard profiles in flash:

- A mouse that still runs its factory (ROM) profiles gets them copied to the
  user profile sectors with only the LED blocks changed. DPI, buttons and
  report rate stay at the factory values.
- Existing user profiles (for example from G HUB or Piper) keep everything
  except their LED blocks. Profiles with a bad checksum are not touched.
- Every sector is checksummed and read back after writing; the profile
  directory is written last.

`logirgb reset-profiles` clears the profile directory so the mouse goes back
to its factory profiles.

## Library

The protocol code is usable on its own:

```go
import (
	"github.com/mevdschee/logitech-rgb/hidpp"
	"github.com/mevdschee/logitech-rgb/lighting"
)

infos, _ := hidpp.Discover()
dev, _ := hidpp.Open(infos[0])
defer dev.Close()

ctl, _ := lighting.New(dev)
ctl.Claim()
ctl.SetEffect(0, lighting.Effect{Name: "static", Values: map[lighting.Param]int{lighting.ParamColor: 0xff8000}})
```

- `hidpp`: discovery, request/response matching, feature lookup, device name, notifications.
- `lighting`: zones and effects (0x8070/0x8071), brightness (0x8040), idle/sleep timers,
  storing lighting in onboard profiles (0x8100).

## References

The protocol details come from reading
[Solaar](https://github.com/pwr-Solaar/Solaar),
[OpenRGB](https://gitlab.com/CalcProgrammer1/OpenRGB) and
[libratbag](https://github.com/libratbag/libratbag), and were checked against
real hardware.

## License

MIT
