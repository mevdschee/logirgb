# logirgb

`logirgb` sets the RGB lighting of Logitech G devices on Linux. It talks
HID++ 2.0 directly over `/dev/hidraw`, so it needs no G HUB, Solaar, OpenRGB or
libusb. It has a TUI and a scriptable CLI, and can store your lighting in the
device itself so it survives power cycles without anything running.

Tested with:

| Device | USB ID | Lighting feature | Zones | Brightness | Dim/off on idle |
|---|---|---|---|---|---|
| G515 TKL (wired) | 046d:c358 | 0x8071 RGBEffects + 0x8101 profile | 1 | yes (0x8040) | yes, in firmware |
| G502 HERO / G502 SE HERO | 046d:c08b | 0x8070 ColorLEDEffects + 0x8100 onboard profiles | 2 (primary, logo) | no | no |

On the G502 HERO zone 1 is the logo and zone 0 ("primary") is the color of
the DPI LEDs. Those only light up briefly when you press a DPI or profile
button, to show the DPI level. After a DPI button press they flash in the
zone 0 color, after a profile button press they show blue regardless of
zone 0.

Other devices that expose 0x8070 or 0x8071 should work too. Devices are found
by scanning `/sys/class/hidraw` for Logitech nodes that declare the HID++ long
report.

## Install

```bash
go install github.com/mevdschee/logirgb/cmd/logirgb@latest
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
| w | store the lighting (and G515 timers) in the device |
| o | show the onboard lighting again |
| ctrl+r | rescan devices |
| ? | help screen |
| q | quit |

The CLI does the same from scripts:

```bash
logirgb list                                     # devices, zones, effects, current settings
logirgb set static --color orange                # all zones of all devices
logirgb set -d g515 breathe --color blue --period 3000
logirgb set -d g502 -z 1 cycle --period 8000
logirgb set -d g502 -p 2 -z 1 static --color green  # logo of onboard profile 2 only
logirgb set -d g515 wave --direction left --period 4000
logirgb brightness -d g515 50
logirgb timeouts -d g515 --idle 60 --sleep 300 --dim 50
logirgb apply                                    # re-apply saved settings
logirgb release                                  # back to the onboard profile
logirgb set -d g515 static --color orange --save # set and store in the keyboard
logirgb save                                     # store lighting (and timers) in the devices
logirgb reset-profiles -d g502                   # back to the factory profiles
```

Effects are `off`, `static`, `pulse`, `breathe`, `cycle`, `wave`, `ripple` and
`starlight`; `logirgb list` shows which ones each zone supports and which
parameters they take (`--color`, `--color2`, `--period`, `--speed`,
`--intensity`, `--saturation`, `--direction`, `--form`).

Settings are stored per device model in `~/.config/logirgb/config.json`.

### Storing settings in the device

Changes made with `set`, `brightness` or the TUI are a live preview: they go
to the device's RAM and are lost on a power cycle. `logirgb save` (or `w` in
the TUI, or `set --save`) stores them in the device's onboard memory. The
previous contents are backed up to `~/.local/share/logirgb/backups/` first,
and every write is verified by reading it back. The TUI starts from what
the onboard memory holds (the G515 profile's lighting, dim level and
timeouts, and the lighting of each G502 profile), so `w` writes back what
you see.

**G515 TKL.** The keyboard's onboard profile holds the lighting, a dimmed
copy of it and the idle and sleep timeouts. After `save` the firmware runs
that profile by itself: it dims to the configured level (`--dim`, percent of
the normal lighting) after the idle timeout, switches off after the sleep
timeout and wakes up on the next key press, also after a power cycle. While
you are previewing changes the host controls the lighting and the firmware
does not dim; `save` hands control back, and `o` in the TUI (or
`logirgb release`) drops a preview without storing it.

**G502 HERO.** The mouse re-applies the lighting of its onboard profile
whenever you switch profiles (the button behind the scroll wheel) or replug
it, so `save` writes the lighting into all onboard profiles. Each profile can
have its own lighting: pick it with the Profile row in the TUI (it starts on
the active profile, which is marked) or with `set -p <n>`. Without `-p`,
`set` changes every profile and replaces the per profile lighting of that
zone. The TUI lists all zones of the device under each other.

The TUI also shows the selected profile's report rate, its DPI levels
(up to five, `off` for unused ones), the level the profile starts on and the
level the DPI shift button switches to. The DPI range and report rates come
from the mouse (100-25600 DPI in steps of 50 and 125-1000 Hz on the G502
HERO). These are not previewed: `w` writes them, and the mouse stays on its
current DPI level.


- A mouse that still runs its factory (ROM) profiles gets them copied to the
  user profile sectors with only the LED blocks changed. DPI, buttons and
  report rate stay at the factory values.
- Existing user profiles (for example from G HUB or Piper) keep everything
  except their LED blocks. Profiles with a bad checksum are not touched.
- The profile directory is written last.
- The active profile's zone effects are also stored with the 0x8070
  persistence flag, which is what the mouse shows at power-up, before any
  profile switch.

`logirgb reset-profiles -d g502` clears the profile directory so the mouse
goes back to its factory profiles.

## Library

The protocol code is usable on its own:

```go
import (
	"github.com/mevdschee/logirgb/hidpp"
	"github.com/mevdschee/logirgb/lighting"
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
  storing lighting in onboard memory (0x8100 profiles, 0x8101 profile management).

## References

The protocol details come from reading
[Solaar](https://github.com/pwr-Solaar/Solaar),
[OpenRGB](https://gitlab.com/CalcProgrammer1/OpenRGB) and
[libratbag](https://github.com/libratbag/libratbag) and the G515 profile notes
in [logitech-g515-onboard](https://github.com/federico-duppa/logitech-g515-onboard),
and were checked against real hardware.
The [research](research/) folder collects the protocol notes, device
details and hardware test results behind this tool, together with
a copy of Logitech's published HID++ 2.0 feature documents in
[research/drive-download.zip](research/drive-download.zip).

## License

MIT
