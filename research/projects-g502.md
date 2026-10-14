# G502 projects on GitHub: what they do and what we can learn

A survey of small third-party projects that control the Logitech G502 family
(mostly the G502 HERO / SE HERO, `046d:c08b`) over HID++ 2.0, read on
2026-10-14. All repos were shallow-cloned and only their source text was read.
Nothing was built, installed or executed.

Tags used below: **[HW]** means our own hardware observation (see
[hardware-log.md](hardware-log.md)), "claim" means the author says so but we
did not verify it.

## Summary

| Project | Stars | Language | License | Updated | Relevance | Key takeaway |
|---|---|---|---|---|---|---|
| [vaisajuh/logiled](https://github.com/vaisajuh/logiled) | 0 | Python | MIT | 2026-10-12 | High | Clean 0x8070 tool for the G502 HERO; sends SetZoneEffect with persistence byte `01` by default and claims that survives replugging |
| [8081Works/g502-bass-led](https://github.com/8081Works/g502-bass-led) | 0 | Python | MIT | 2026-08-30 | High | Uses zone 0 color plus 0x8100 `fnC` SetCurrentDpiIndex to drive the 3 DPI LEDs as a bar meter: zone 0 colors the DPI LEDs, the DPI index sets how many light |
| [lexr1/omm.py](https://github.com/lexr1/omm.py) | 10 | Python | GPL-3.0 | 2026-06-02 | High | Full 0x8100 profile editor; treats directory byte 2 as "visible in cycle"; forces sector size 255 to 256 (breaks on 255-byte units) |
| [DalindaLimon/Fivo-o2-Hub](https://github.com/DalindaLimon/Fivo-o2-Hub) | 0 | Python | none | 2026-10-01 | High | GTK profile editor; writes LED block N for zone N (libratbag order) and mirrors into the alt blocks; self-described as vibe coded |
| [UZ9/g502-lightcontroller](https://github.com/UZ9/g502-lightcontroller) | 0 | C | none | 2025-02-06 | Medium | Replays a G HUB capture: `11 ff 02 3c [zone] 01 r g b 02 ...`, zone 0 = DPI, 1 = logo, G HUB software ID `0xC` |
| [Roritharr/logitech-g502-led-off](https://github.com/Roritharr/logitech-g502-led-off) | 0 | Python | MIT | 2026-06-04 | Medium | 0x8070 "off" with persistence byte `01`, claims it survives reboot; macOS hidapi shared-open trick |
| [meiyoutou/Mouser-G502](https://github.com/meiyoutou/Mouser-G502) | 2 | Python | MIT | 2026-09-24 | Medium | Rewrites DPI and profile buttons to F13 to F16 so the host can see them; rejects sector sizes that are not a multiple of 16 |
| [geromet/g502-linux-control](https://github.com/geromet/g502-linux-control) | 0 | Rust | none | 2026-09-19 | Low-medium | All via ratbagd D-Bus; documents that ratbagd misses hardware DPI and profile changes on the G502 HERO |
| [marco-vrinssen/logitech-onboard-memory](https://github.com/marco-vrinssen/logitech-onboard-memory) | 0 | Swift | MIT | 2026-10-01 | Low (not G502) | PRO X 2 DEX format 7; confirms 255-byte sectors are real; format 7 report rate byte order differs from libratbag PR |
| [RalphJS/borochid-driver-logitech-hidpp](https://github.com/RalphJS/borochid-driver-logitech-hidpp) | 0 | Python | Apache-2.0 | 2026-09-28 | Low | G502 X LIGHTSPEED, host mode only, no lighting feature; good notes on host/onboard mode behavior |
| [timunrau/g502-hero-macos-onboard-memory](https://github.com/timunrau/g502-hero-macos-onboard-memory) | 0 | Python | GPL-3.0 | 2026-09-09 | Low | An AI agent "skill" that vendors omm.py with a macOS patch; useful G502 HERO button index map |
| [TboyGingy/G502-Hero-Manager](https://github.com/TboyGingy/G502-Hero-Manager) | 0 | JavaScript | MIT (package.json) | 2026-08-07 | Low | Electron app; copies omm.py's 255 to 256 bug; its "RGB" code is dead code at a wrong offset |
| [qodroi/g502h-drv](https://github.com/qodroi/g502h-drv) | 5 | C | GPL-3.0+ (header) | 2025-12-11 | Low | Unfinished kernel module; hard-coded feature indices; LED code never written |
| [ch4rlesexe/LMD](https://github.com/ch4rlesexe/LMD) | 0 | none | MIT | 2025-10-28 | None | Empty: only a LICENSE file, no branches or releases |
| Extra: [pamirmundt/Logitech-G502-LED-Controller](https://github.com/pamirmundt/Logitech-G502-LED-Controller) | 1 | Python | MIT | 2019-04-20 | Low-medium | G502 Proteus Spectrum (`c332`) capture: `11 ff 02 3b [zone] 01 r g b 02 ...`, same zone order as the HERO |
| Extra: [ProfessorMoose74/Logitech-Gaming-Keyboard-Mouse-Controls](https://github.com/ProfessorMoose74/Logitech-Gaming-Keyboard-Mouse-Controls) | 2 | Python | MIT | 2026-03-03 | None | Wrapper around `ratbagctl led 0 set ...`, no protocol code |

Stars and dates come from `gh repo view` (`pushedAt`). "none" means no
LICENSE file and no license header.

## Projects

### vaisajuh/logiled

Linux CLI and GTK 3 GUI for G502 (SE) HERO lighting, DPI, report rate and
onboard mode. Pure Python, no dependencies. It also ships as a component of the
author's `clcr` lighting app. It has unit tests that run the transport against
a simulated mouse over a socket pair, so the packet encoding is tested, but the
hardware behavior is only claimed. The code is tidy and consistent and was
probably written with AI help.

- **Device discovery**: scans `/sys/class/hidraw/*`, keeps `046d` nodes whose
  report descriptor has usage page `0xFF00` and report ID `0x11`. Tries device
  index `0xFF` first, then 1 to 6. This is the same approach as ours.
- **Features**: 0x0005 name, 0x2201 DPI (decodes the "range with step"
  list), 0x8060 rate, 0x8070 lighting, 0x8100 fn1/fn2 only.
- **0x8070 SetZoneEffect** with a persistence byte, on by default
  (`logiled/device.py`):

  ```python
  self.hidpp.feature_request(
      COLOR_LED_EFFECTS, 3, zone.index, zone.effects[effect], *payload, 1 if persist else 0
  )
  ```

  Parameter layouts, from `effect_params()`: static `[r, g, b, 0]`, cycle
  `[0,0,0,0,0, period_hi, period_lo, brightness]`, breathe
  `[r, g, b, period_hi, period_lo, 0, brightness]`, ripple
  `[r, g, b, 0, period_hi, period_lo]`. These match ours except static, where we
  put `0x02` in byte 3.
- **Onboard and host mode**: it never switches modes itself, but the README
  says that "By default lighting is saved to the mouse's memory, so it stays
  after replugging". It says DPI and rate changes may be overridden by the
  active profile in onboard mode, and suggests `logiled mode host`. It does not
  mention profile switches.
- **Profiles**: no 0x8100 memory access.

### 8081Works/g502-bass-led

A GTK app that drives the G502 HERO lights from audio: the logo follows bass,
and the three DPI LEDs work as a 1 to 3 step bar meter. One 1056-line Python
script, with a Romanian and English UI. Tested on `046d:c08b`.

- **Device discovery**: the hidraw `uevent` must contain `046D:0000C08B`. The
  node whose uevent mentions `input1` is preferred ("the vendor interface
  (input1) is the HID++ one"). Feature indices are looked up with root fn0.
  If that lookup fails it falls back to `0x02` for 0x8070 and `0x0C` for
  0x8100.
- **0x8070 SetZoneEffect, RAM only**, zone 1 = logo, zone 0 = "DPI band"
  (`g502-bass`):

  ```python
  pkt[3] = (CMD_SET_ZONE_EFFECT & 0xF0) | (SW_ID & 0x0F)   # 0x30 | 0x0A
  pkt[4] = zone
  pkt[5] = mode          # effect index: 0 off, 1 static
  pkt[6] = r; pkt[7] = g; pkt[8] = b
  ```

  The logo is limited to one write every 16 ms and the DPI zone to one every
  50 ms, "so we don't flood the mouse's USB".
- **DPI indicator**: to light 1, 2 or 3 DPI LEDs it sends 0x8100 `fnC`
  (SetCurrentDpiIndex) with index 0, 1 or 2, and sets the zone 0 color:

  ```python
  def set_vu_bars(self, bars, rgb):
      ...
      self.set_rgb(SIDE_ZONE, r, g, b)
      self.set_dpi_index(bars - 1)
  ```

  So it does not keep the mouse from changing DPI, all five DPI slots are first
  set to the same value through ratbagd (`ratbagctl`), and the user's own values
  are backed up to JSON and restored when the effect stops.
- **Onboard and host mode**: stays in onboard mode. `fnC` works there.
- **Hardware notes**: the README says the DPI LEDs show 1, 2 or 3 lit LEDs in
  the zone 0 color as the DPI index changes. That is the only project that
  says anything about what zone 0 actually shows.
- Probably AI-assisted, but it shows signs of real use (the rate limits and
  the DPI backup recovery).

### lexr1/omm.py

"Unofficial Logitech Onboard Memory Profile Tool": a Python (hidapi) CLI to
dump, export and import 0x8100 profiles as JSON, including buttons, G-shift,
macros and RGB. The author tested it on Windows 10 with a G502. It looks
hand-written and is the base for timunrau's skill and for the JavaScript
G502-Hero-Manager.

- **Device discovery**: hidapi enumerate, usage page `>= 0xFF00`, then checks
  the report descriptor. The PID and device index come from `devices.ini`
  (`pid=0xc08b`, `index=0xff`). Requests are short reports (`0x10`) with
  software ID `0xF`.
- **GetInfo sample from the author's G502** (comment in
  `libs/FeatureOnboardProfile.py`):

  ```python
  #sample output on G502
  #0x11 0xff 0x0c 0x0f 0x01 0x02 0x01 0x05 0x05 0x0b 0x10 0x01 0x00 0x0a 0x01 ...
  ```

  This shows 0x8100 at feature index `0x0c` and a sector size of `0x0100`
  (256), the same as the 2020 libratbag #1181 log and unlike our 255.
- **Sector size forced to 256**:

  ```python
  if self.page_size == 255:
      self.page_size = 256
  assert self.page_size in [256, 1024], ...
  ```

  On a 255-byte unit like ours, this reads 16 aligned chunks. The last one
  (offset 240) runs past the end of the sector, which our firmware rejects. The
  CRC also lands on the wrong position.
- **Directory**: entries are read as `[mem type, page, visible]`. `rom == 0x01`
  means "profile is on ROM" and `rom == 0xFF` means disabled. Enabling writes
  `00 NN 01 00`. Disabling writes the whole entry as `ff ff ff ff`, not
  `00 NN 00 00` as libratbag does. Byte 2 is called **visibility** (whether
  the profile button cycles to it), and "enabled" means the entry exists. The
  docstring says `profile_visibility` hides a profile from the cycle, and
  `current_profile` refuses to hide the active one.
- **SetCurrentProfile**: `fn3 [00, N, 00]`. GetCurrentProfile reads byte 1
  only, so it does not tell user and ROM sectors apart.
- **Profile layout**: the same as our table. LED handling reads all **four**
  11-byte blocks (208, 219, 230, 241) as `rgb[0..3]` and writes them back from
  JSON, so the alt blocks can be edited and are never automatically copied. The
  docs say "The profile assigns 4 RGB configs for mouse, each for a different
  LED", and "Unsupported model/zone will show all `FF`". Block encodings match
  ours (static `01 r g b` plus 7 zeros, cycle period at 6-7 and brightness at
  8, breathe period at 4-5 and brightness at 7). It does not map blocks to
  zones.
- **Macro pages**: by default, user pages after the profiles are shared out
  at `(sectors - profiles - 1) / profiles` pages per profile, so pages 6 to 15
  on a G502 with 16 sectors. Long macros use a next-page opcode.
- **Mode**: refuses to show info unless onboard mode is on ("run `omm.py
  --onboard on` first").

### DalindaLimon/Fivo-o2-Hub

A GTK 4 / libadwaita app for DPI, buttons, lighting, profiles and macros. The
repo description says "Tested extensively for 2 minutes. Unfortunately vibe
coded (sorry)". Its only hardware claim is the **G502 LIGHTSPEED**. The G502
HERO is listed as "should work". There is no LICENSE file.

- **Device discovery and transport**: hidraw, long reports, software ID
  `0x0A`, feature indices from root fn0.
- **Profile read** (`onboard.py`): forces onboard mode, reads GetInfo, then the
  directory if its CRC is valid, otherwise sector `n` (falling back to ROM
  `0x0100 + n` if the CRC is bad). The 16-byte reads clamp to `size - 16`, as
  ours do.
- **LED blocks**: zone index N goes to block N, the libratbag order, and the
  same data is written into the alt block:

  ```python
  def set_led(self, zone, effect):
      """Store an LedEffect for `zone` (also mirrored into the alternate-LED slot, as libratbag does)."""
      enc = effect.encode()
      for base in (OFF_LEDS, OFF_ALT_LEDS):
          off = base + LED_LEN * zone
  ```

  `get_led` documents zone 0 at offset 208 as "primary/DPI" and zone 1 as
  "logo on the G502". On our G502 SE HERO that is reversed **[HW]**. The UI
  defaults to "Same effect on all zones", which hides the problem.
- **Directory write**: `[sector BE, enabled, 00]` per profile, then
  `ff ff 00 00`, `ff` padding and CRC. This is identical to ours.
- **After a save**: if the saved profile is the active one, it reads
  `fnB` (current DPI index), sends `fn3 [00 NN]` to reload the profile, then
  restores the DPI index with `fnC`. That avoids the DPI jumping back to the
  profile default after a reload. It refuses to disable the last enabled
  profile, and "reset to factory" copies ROM profile 1 over the slot.
- Effect encodings for the profile blocks: static `[01, r, g, b]`, cycle
  `[03, .., period at 6-7, intensity at 8]`, breathe
  `[0a, r, g, b, period at 4-5, 0, intensity at 7]`. Brightness 100 is written
  as 0, meaning "default 100%".

### UZ9/g502-lightcontroller

A 200-line libusb C program with `logo-light <hex>` and `dpi-light <hex>`
commands. It detaches the kernel driver from USB interface 1 and sends a raw
`SET_REPORT` (`wValue 0x0211`, `wIndex 1`). The author says the bytes were
"pulled via wireshark" from G HUB (`main.c`):

```c
#define G502_MAGIC_NUMBER_1 0x11   // long report
#define G502_MAGIC_NUMBER_2 0xff   // device index
#define G502_MAGIC_NUMBER_3 0x02   // feature index of 0x8070
#define G502_MAGIC_NUMBER_4 0x3c   // fn3, software id 0xC
...
enum G502_LED_ID { LOGO_LED = 0x01, DPI_LED = 0x00 };
...
  interface, 0x01, red, blue, green, 0x02, 0x00, 0x00, ...
```

- That decodes to SetZoneEffect `[zone, effect index 1 (static), r, g, b, 02,
  00 ...]`, with persistence `00`. G HUB puts `0x02` in parameter byte 3, as
  we do.
- G HUB's software ID on the G502 HERO is `0xC`.
- The `red, blue, green` naming is a parameter-order slip that cancels out at
  the call site. The output is correct RGB.
- No onboard handling and no profile writes. Small, hand-written, and works
  for its narrow purpose. It needs root or a broad `usb` udev rule and steals
  interface 1 from the kernel while it runs.

### Roritharr/logitech-g502-led-off

A Python (hidapi) tool that turns both G502 HERO zones off "persistently" with
0x8070 and no daemon. Tested on macOS. Probably AI-assisted (one commit,
polished README).

- **Device discovery**: hidapi, usage page `0xFF00` and usage `0x02`. Software
  ID `0x09`.
- **Payload** (`g502_led_off/core.py`):

  ```python
  params = bytes([zone, (EFFECT_OFF >> 8) & 0xFF, EFFECT_OFF & 0xFF] + [0x00] * 9 + [persist_byte])
  st, r = call(dev, di, led, 0x03, params)  # func 0x03 = set_zone_effect
  ```

  It treats bytes 1-2 as a 16-bit effect *ID*, but byte 1 is the zone effect
  *index*. It works only because index 0 happens to be "off" on the G502. The
  persistence byte ends up at byte 12, which is correct. The README says
  "byte 12 of the request is a persist flag that stores the change to flash"
  and that the result "survives reboots and unplugging". It does not say what
  happens on a profile switch.
- **macOS quirk** (new to us, useful if we ever port): hidapi opens devices
  exclusively on macOS by default, and G HUB's DriverKit HID filter refuses
  that, so the open fails with a generic "open failed" that looks like a
  permissions error. The fix is `hid_darwin_set_open_exclusive(0)`.

### meiyoutou/Mouser-G502

A fork of TomBadash/Mouser (a Python/Qt mouse remapper) with G502 HERO
support. `core/g502_onboard.py` (957 lines) implements a "conservative subset"
of 0x8100. It is Windows-focused and probably AI-assisted.

- **Device discovery**: hidapi, PID `c08b` only, usage page `>= 0xFF00`,
  preferring usage `0x0002`. It pings device indices `0xFF` and 1 to 6.
- **Profiles**: it backs up the directory and every listed sector to JSON
  first. It refuses to continue if the directory CRC is invalid, which means it
  will not work on a mouse that still runs its ROM profiles. Its
  `validate_for_g502_hero` also rejects any sector size that is not a multiple
  of 16:

  ```python
  or self.sector_size % 16 != 0
  ```

  So it would reject our 255-byte unit. Its writes need the length to be a
  multiple of 16.
- **Button unlock**: rewrites the active profile's DPI shift, DPI down, DPI
  up and profile cycle buttons into keyboard bindings for F13, F14, F15 and
  F16. The firmware does not report those special actions to the host, so the
  app intercepts the F keys instead. After writing, it switches to onboard mode
  if needed and re-selects the active sector with `fn3`.
- No lighting.

### geromet/g502-linux-control

A Rust daemon, CLI and iced GUI on top of ratbagd (D-Bus), with no direct
HID++. It keeps one DPI value shared across profiles by mapping the DPI buttons
to F13/F14 macros and rewriting resolution slot 1 in both profiles through
ratbagd. It can also edit LED mode, color and brightness per profile through
ratbagd. Tested on a G502 HERO `c08b` under Fedora 44. The README is very
detailed and was probably written with AI help.

Hardware notes the author documents:

- Pressing the hardware DPI buttons changes the pointer speed, but ratbagd's
  active resolution "keeps returning 1". The host cannot see onboard DPI
  changes.
- ratbagd's `Profile.IsActive` "is not trustworthy on the tested mouse": it
  reported a disabled profile as active. `Profile.SetActive` does switch the
  mouse.
- `Device.Commit()` takes about 270 ms on the G502 HERO.
- Profile names cannot be set on this mouse through libratbag.

### marco-vrinssen/logitech-onboard-memory

A Swift app and CLI (`lomm`) for macOS, built and tested only on the
**PRO X 2 DEX** (profile format 7). Editing is blocked on formats 5 and older,
so it does not write to the G502. One commit.

- It confirms our 255-byte finding: "255-byte sectors are real. The device
  reports 255 and means it. A 16-byte read crossing that boundary is rejected
  ... Other tools assume the reported 255 means 256 and fail on this hardware."
- Its format 7 table puts the **wired** report rate index at byte 0 and the
  **wireless** one at byte 1 ("verified via feature 0x8061"), and sleep and
  power-off timeouts "in seconds, u16le" at 0x2C. Our 8100 notes (from
  libratbag PR #1896) say byte 0 is wireless and byte 1 is wired.
- On macOS, `IOHIDDeviceSetReport` needs the report ID kept in the buffer.
- Enforces onboard mode on sight and edits profile 1 only.

### RalphJS/borochid-driver-logitech-hidpp

A plugin for the author's "Borochid" input service. It targets the
**G502 X LIGHTSPEED** (`046d:409f` through receiver `c547`, `c098` wired),
keeps the mouse in host mode and never writes flash. It has a test suite with
a fake mouse. No lighting: the author reports the G502 X LIGHTSPEED has **no**
0x8070, 0x8071 or 0x1300.

Measured notes worth keeping (G502 X LIGHTSPEED, profile format 3):

- "Onboard mode refuses report-rate changes (`INVALID_ARGUMENT`) but accepts
  DPI. Host mode accepts both."
- Host mode does not survive a power cycle. The mouse comes back in onboard
  mode.
- "Leaving host mode keeps the host's last DPI; re-selecting the onboard DPI
  level (ONBOARD_PROFILES fn 0x0C) restores the profile's value."
- In host mode each button is reported twice: as a plain HID button and as a
  0x8110 spy event. An entry of 0 in the spy remap table silences the HID
  button while the spy still reports it.
- On the cable, HID++ is on USB interface 2. Through the receiver, the kernel
  driver also talks on the node with software ID 1.

### timunrau/g502-hero-macos-onboard-memory

An instructions file ("Codex skill") for an AI coding agent, bundling a copy
of omm.py (pinned to upstream commit `8e6c3f0`) with a macOS patch for
hidapi's exclusive open (the same trick as Roritharr) and a
`profile_edit.py` JSON helper. It is written for an agent to drive. There is no
new protocol work.

Useful reference: the physical map of the G502 HERO's 11 profile button slots
(`references/button-map.md`): 0 G1 left, 1 G2 right, 2 G3 middle, 3 G4 back,
4 G5 forward, 5 G6 sniper (DPI shift), 6 G7 DPI down, 7 G8 DPI up, 8 G9 profile
cycle, 9 wheel tilt right, 10 wheel tilt left. This matches our ROM table
(`9007ffff` at 5, `9004ffff` at 6, `9003ffff` at 7, `900affff` at 8). It also
warns that on macOS the reply to an immediate `--switch` can time out even
after the save worked.

### TboyGingy/G502-Hero-Manager

A Windows Electron and React app (node-hid) for DPI, buttons and profiles. The
README points to a `dist/` installer that is not in the repo. We did not look
for or run any binaries. The code credits omm.py and libhidpp and looks
largely AI-generated.

- **Windows interface split**: it writes short reports to the `0xFF00` /
  usage `0x0001` collection and reads replies from the usage `0x0002`
  collection, because Windows exposes them as separate HID devices.
- Copies omm.py's `if (pageSize === 255) pageSize = 256;`, so it has the same
  problem on 255-byte units.
- Before a write it reads the slot back and refuses if the CRC is bad or the
  page is mostly `00` or `ff` ("Reset the mouse in G HUB first"). It then
  patches only bytes 0-12 and reseals the CRC.
- The README advertises "RGB LED Control". The live write path does not touch
  LEDs. The only RGB code is in unused `_parseProfile` / `_serializeProfile`
  functions with a made-up layout (5-byte buttons at 24, "RGB (bytes
  136-138)"). Bytes 136-138 are really inside the G-shift button table. Not a
  trustworthy reference.

### qodroi/g502h-drv

An out-of-tree HID kernel module for `c08b` (GPL-3.0-or-later header, no
LICENSE file). It exposes DPI and report rate in sysfs and does software
profile switching. It is unfinished: the README admits sysfs writes "might need
to be submitted 2-3 times". It is hand-written.

- Hard-coded feature indices for the G502 HERO (`include/g502.h`): `0x02` =
  0x8070, `0x03` = 0x0003 firmware, `0x0a` = 0x2201, `0x0b` = 0x8060, `0x0c` =
  0x8100. This agrees with UZ9, bass-led and the omm.py sample.
- On probe it sends 0x8100 `fn1 [02]` (host mode) and then runs its own
  in-driver profiles.
- Its report-rate table maps `0x1` to 125 Hz and `0x8` to 1000 Hz, which is
  backwards (the byte is the interval in ms).
- The LED enum names zone 0 `G_LED_PRIMARY /* That is the "DPI" Level LED */`
  and zone 1 `G_LED_LOGO`, but no LED command is ever sent.
- A report descriptor fixup changes Usage Maximum from 16 to 11 buttons for the
  67-byte descriptor. HID++ traffic is only sent on USB interface 1.

### ch4rlesexe/LMD

"Linux Mouse Driver Suite" for the G502 Hero. The repo only has a LICENSE file
(one "Initial commit" on `main`, no other branches, no releases). Nothing to
review.

### Extra: pamirmundt/Logitech-G502-LED-Controller

A 2019 project for the **G502 Proteus Spectrum** (`046d:c332`, firmware
`U103.02_B0012`) on a Raspberry Pi and Arduino. It includes USBlyzer captures
and `lsusb` output. The Python script uses pyusb `SET_REPORT` on interface 1:

```python
start = [0x11, 0xFF, 0x02, 0x3B]   # 0x8070 at index 2, fn3, software id 0xB
data = start + [0] + [1] + color + [2] + end   # dpiColor: zone 0
data = start + [1] + [1] + color + [2] + end   # logoColor: zone 1
```

This is the same command, zone order and `0x02` byte as G HUB uses on the
HERO. The older Logitech Gaming Software apparently used software ID `0xB`.

### Extra: ProfessorMoose74/Logitech-Gaming-Keyboard-Mouse-Controls

"LogiLight". For mice it shells out to `ratbagctl <dev> led 0 set ...`. No
protocol content. The repo has SEO and push-instruction docs and looks
AI-generated.

### Searches

`gh search repos` for "g502 led", "g502 rgb" and "g502 hidpp" found the two
extras above, plus `SkullboyNL/OmniLED-G502-and-G935-plugin` (a Lua plugin
that shows DPI on a SteelSeries OLED) and `Benonsen/Logitech-LED-Changer`
(C#, 2020, LGS SDK based). Neither was relevant. More repo and code searches
("0x8070 onboard", "g502 onboard memory", "0x8070 g502" code search) hit the
GitHub API rate limit and were not run.

## Findings for logirgb

### LED block order in profiles

- **Nobody else independently confirms our reversed order.** Fivo-o2-Hub
  writes zone N to block N (libratbag's order), so on a G502 HERO its
  "primary" setting would land on the logo. Its default "Same effect on all
  zones" hides this. omm.py exposes the four blocks as a plain list without
  mapping them to zones. libhidpp (`logo_effect` at 208) is still the only
  outside source that agrees with our **[HW]** result. Fivo's only tested
  device is a G502 LIGHTSPEED, so it is possible, but **[unverified]**, that
  the order differs between models. Our `slot = len(zones) - 1 - zone` rule
  remains verified on `c08b` only.
- **The 0x8070 zone order is confirmed** by three captures and labels: zone 0
  is the DPI LEDs and zone 1 is the logo (UZ9's G HUB capture, pamirmundt's
  Proteus Spectrum capture, bass-led, qodroi's enum).

### Sector size and reads

- Another G502 GetInfo with sector size **256** (omm.py's comment, feature
  index `0x0c`). So far: two units at 256 (libratbag #1181, omm.py author) and
  ours at 255. This confirms "never hard-code the sector size".
- **Three tools break on 255-byte sectors**: omm.py and G502-Hero-Manager turn
  255 into 256, and Mouser-G502 requires a multiple of 16.
  marco-vrinssen confirms on a PRO X 2 DEX that 255 is real and that reads
  crossing the end are rejected. This backs up our `size - 16` clamp. Worth a
  line in the pitfalls list in [8100-onboard-profiles.md](8100-onboard-profiles.md).

### Directory format

- Fivo-o2-Hub writes exactly what we write (`[sector BE, enabled, 00]`, then
  `ff ff 00 00`, `ff` padding, CRC). Confirms.
- **New interpretation from omm.py**: byte 0 is the memory type (`0x01` means
  the entry points at a ROM page), byte 2 is "visible in the profile cycle", and
  a disabled slot is written as a full `ff ff ff ff` entry. Under this reading,
  `00 0N 00 00` (libratbag) means "exists but is skipped by the profile button".
  That is consistent with Solaar's "Read-Only Profile N" (`fn3 [01 0N]`). We
  have not tested what the cycle button does with byte 2 = 0, or whether the
  firmware accepts directory entries that point at `01 0N`. Both are cheap
  hardware tests.

### Alt LED blocks (230-251)

- Fivo-o2-Hub copies the normal blocks into the alt blocks (like libratbag).
  omm.py lets the user edit all four blocks. **Nobody documents what the alt
  blocks do** on a corded G502. We leave them as copied from ROM (`ff`). No
  project gives a reason to change that. Copying the normal blocks there would
  match libratbag and Fivo and would cost nothing if we want to be safe.

### Profile button switching and reloads

- Several projects confirm that the host cannot see onboard DPI and profile
  button presses. geromet sees ratbagd's active resolution and active profile
  go stale. geromet and Mouser-G502 work around it by rebinding the buttons to
  F13 to F16 keys (geromet via libratbag macros, Mouser as plain `80 02` key
  bindings). This matches our **[HW]** "no notification" finding. Rebinding the
  profile button to a key would be one way to detect switches, but it costs
  the firmware's own profile cycling.
- Fivo-o2-Hub and Mouser-G502 both re-select the active profile with `fn3`
  after writing it, as we do. Fivo also reads `fnB` before the reload and
  restores the DPI index with `fnC` after it. Otherwise the reload resets DPI to
  the profile's default index. **New**: `SaveToProfiles` should do the same
  (read `fnB`, reload with `fn3`, write `fnC`), so saving lighting does not
  change the user's DPI.

### DPI indicator LEDs (zone 0)

- **New and possibly contradicting our notes.** g502-bass-led sets the zone 0
  color with 0x8070 and then sends 0x8100 `fnC [0|1|2]` in onboard mode. The
  README says this lights 1, 2 or 3 DPI LEDs in that color. Our
  hardware log says zone 0 "made no visible difference" because the DPI LEDs are
  normally off and only flash briefly on a DPI or profile button press. The
  likely explanation is that the zone 0 color is the color of that flash, and
  `fnC` triggers the flash. Test on our unit: set zone 0 to red, then press DPI
  up, or send `8100 fnC [01]`, and see whether the flash is red instead of the
  default blue.
- If that holds, a profile's LED block 1 (zone 0) sets the DPI flash color
  per profile. The UI could then show zone 0 as "DPI indicator color" instead
  of a normal light.
- `fnC` SetCurrentDpiIndex is accepted in onboard mode (bass-led on the HERO,
  borochid on the G502 X). This confirms libhidpp.

### 0x8070 SetZoneEffect persistence byte

- logiled (default) and Roritharr send persistence `0x01` and both claim the
  setting **survives replugging or reboot**. Neither mentions profile
  switches. G HUB (UZ9's capture) and LGS (pamirmundt) send `0x00`, as we do.
- Our **[HW]** note says profile loads (switch, `fn3`, power-up) overwrite the
  zone effect, but we only tested persistence `0`. Two cases to test:
  (a) persistence `1` with the ROM profiles active, then a power cycle, and
  (b) persistence `1` followed by a profile button press. The likely result is
  that power-up still loads the profile LED blocks, so the claims would only
  hold for some states, for example when the active profile's blocks are `ff`.
  But this is **[unverified]**, and if it does persist, a simpler "set and
  forget" path would exist without writing profiles.
- Persistence `1` writes flash every time, so live preview should stay at `0`
  (bass-led makes the same point: "RAM only, so it doesn't wear the flash").

### Parameter details

- G HUB and LGS both send static as `[r, g, b, 0x02]` (parameter byte 3 =
  `0x02`, "no ramp"). This confirms our `fixed: {3: 0x02}`. logiled and
  bass-led send `0x00` there, which also works.
- G HUB uses software ID `0xC` on the G502 HERO, and LGS used `0xB` on the
  Proteus Spectrum. We should avoid both so that if G HUB is running its
  replies are not mixed up with ours.
- Feature indices on the G502 HERO (`c08b`): 0x8070 = `0x02`, 0x8100 =
  `0x0c`, 0x2201 = `0x0a`, 0x8060 = `0x0b`. Several projects hard-code these,
  and we should not. They are useful for reading captures.

### Host and onboard mode

- borochid (G502 X): onboard mode rejects 0x8060 SetReportRate with
  `INVALID_ARGUMENT`, and host mode is lost on power cycle. This confirms our
  notes. New: leaving host mode keeps the host's DPI until `fnC` re-selects a
  profile DPI level.
- qodroi forces host mode on probe, and Fivo and marco-vrinssen force onboard
  mode. None of the lighting tools needs host mode for 0x8070 on the HERO,
  which confirms our choice to stay in onboard mode.

### Format 7 (not G502, for 8100-onboard-profiles.md)

- marco-vrinssen (PRO X 2 DEX) has byte 0 = wired rate and byte 1 = wireless
  rate. libratbag PR #1896 has the opposite. Flag this as a conflict in our
  format 6/7 table.

### Other platform notes

- macOS: hidapi's default exclusive open fails when G HUB's HID filter is
  installed. Use `hid_darwin_set_open_exclusive(0)`, and keep the report ID in
  the `IOHIDDeviceSetReport` buffer (Roritharr, timunrau, marco-vrinssen).
- Windows: the short (`0x10`, usage `0x0001`) and long (`0x11`, usage
  `0x0002`) HID++ collections are separate devices, and replies come back on
  the long one (G502-Hero-Manager).

### Quality warnings

Several of these projects carry bugs that would corrupt or brick profiles on a
unit like ours. Do not copy code from them without checking it:

- omm.py and G502-Hero-Manager: the 255 to 256 sector size bug.
- Fivo-o2-Hub: the LED block order is likely wrong on the HERO.
- Roritharr: effect index and effect ID are mixed up.
- G502-Hero-Manager: dead RGB code at a wrong offset.
- qodroi: the report rate table is reversed.

## Sources

Repositories (read 2026-10-14):

- https://github.com/qodroi/g502h-drv (`g502.c`, `include/g502.h`, `README`)
- https://github.com/vaisajuh/logiled (`logiled/device.py`, `logiled/hidpp.py`, `README.md`)
- https://github.com/UZ9/g502-lightcontroller (`main.c`, `README.md`)
- https://github.com/Roritharr/logitech-g502-led-off (`g502_led_off/core.py`, `g502_led_off/hidpp.py`, `README.md`)
- https://github.com/geromet/g502-linux-control (`README.md`, `baseline/README.md`, `src/`)
- https://github.com/8081Works/g502-bass-led (`g502-bass`, `README.md`)
- https://github.com/ch4rlesexe/LMD (empty)
- https://github.com/timunrau/g502-hero-macos-onboard-memory (`SKILL.md`, `references/button-map.md`, `NOTICE`, `scripts/omm/`)
- https://github.com/lexr1/omm.py (`libs/FeatureOnboardProfile.py`, `libs/HidppProfile.py`, `libs/LogiHPP20.py`, `docs/RGB.MD`)
- https://github.com/marco-vrinssen/logitech-onboard-memory (`README.md`, `Sources/LogiHIDPP/OnboardProfiles.swift`)
- https://github.com/meiyoutou/Mouser-G502 (`core/g502_onboard.py`, `README.md`)
- https://github.com/DalindaLimon/Fivo-o2-Hub (`onboard.py`, `hidpp.py`, `README.md`)
- https://github.com/RalphJS/borochid-driver-logitech-hidpp (`README.md`, `src/borochid_logitech_hidpp/`)
- https://github.com/TboyGingy/G502-Hero-Manager (`src/main/hid-device.js`, `README.md`, `package.json`)
- https://github.com/pamirmundt/Logitech-G502-LED-Controller (`Python/LogitechG502_setLedColor.py`, `deviceInfo.txt`)
- https://github.com/ProfessorMoose74/Logitech-Gaming-Keyboard-Mouse-Controls (`logilight/devices/mouse.py`)

Related notes in this repo: [8100-onboard-profiles.md](8100-onboard-profiles.md),
[8070-color-led-effects.md](8070-color-led-effects.md),
[hardware-log.md](hardware-log.md).
