# Keyboard RGB projects on GitHub: what they tell us about HID++ keyboards

This note covers third party keyboard lighting tools on GitHub (g810-led and
its ports, G213 scripts, macOS and Windows tools, and newer G815, G915 and
G915 X tools). For each one it records the transport, the exact bytes and
the quirks, and then lists what logitech-rgb would need to drive these
keyboards. Scanned on 2026-10-14. Every repository was shallow-cloned and
read as text only. Nothing was built, installed or run. Two published packet
capture sets (G213Colors `pcaps/`, MatMoul/g810-led-resources `dumps/`) were
decoded with a small pcap parser written for this note, which ran outside
the cloned trees.

Provenance markers, as in the other notes:

- **[SRC]** read in that project's source.
- **[CAP]** decoded from a USB capture published in that project.
- **[ISSUE]** from a GitHub issue thread.
- **[INF]** our inference.
- **[HW]** verified on our hardware (G515 TKL `c358`, G502 SE HERO `c08b`).

Earlier notes already cover g810-led at a high level
(`other-protocols-and-tools.md` sections 1.1 to 1.5), the 0x8080/0x8081
function tables (`8080-8081-per-key-lighting.md`), the FF43 usage page list
(`hidpp-transport.md` 6.2) and logitech-g515-onboard (`8101-profile-management.md`).
This file does not repeat those tables. It adds what is new and marks what
confirms or contradicts them.

## Summary

Stars and dates are from `gh repo view` / `git log` on 2026-10-14. "Updated"
is the date of the last commit on the default branch.

| Project | Stars | Lang | License | Updated | Devices (PID) | Key takeaway |
|---|---|---|---|---|---|---|
| MatMoul/g810-led | 1455 | C++ | GPL-3.0 | 2022-11-30 | G213 c336, G410 c330, G413 c33a, G512 c342, G513 c33c, G610 c333/c338, G810 c331/c337, G815 c33f, G910 c32b/c335, G Pro c339 | Hardcoded indexes. hidapi build opens the first hidraw node of the PID (interface 0), libusb build claims interface 1. Its "startup mode" is 0x8070 setNvConfig(bootUpEffect) and is sent to the wrong index on the G213 |
| MatMoul/g810-led-resources | 23 | C++ | GPL-3.0 | 2020-04-26 | G213, G413, G502 Spectrum, G810, G815, G910 captures | The G815 capture contains the vendor report descriptor: page 0xFF43, reports 0x11 and 0x12 only, no 0x10. Also G HUB's full G815 startup sequence |
| peppapig450/logitech-led-control | 3 | Rust | GPL-3.0 | 2026-04-15 | same as g810-led | Port of g810-led. Porting bug: G410/G512/G513/G Pro/G610/G810 key frames use the G910 header `12 ff 0f 3d` instead of `12 ff 0c 3a` |
| klyse/LogitechKeyboardLED | 5 | Go | MIT | 2020-08-29 | G815, G915 (any LED SDK device) | Windows only. Calls the LED SDK DLL (`LogitechLedEnginesWrapper.dll`, shipped as a binary in the repo). No HID++ |
| SebiTimeWaster/G213Colors | 155 | Python | MIT | 2021-02-05 | G213 c336 | pyusb SET_REPORT on interface 1. Its pcaps show LGS polling 0x8070 getCurrentColor (fn12) during wave, which contradicts the README claim that wave is a software effect |
| Agundur-KDE/G213Tray | 1 | Python | GPL-3.0 | 2026-07-20 | G213 c336 | pyusb, detaches the kernel driver from interface 1, one static colour packet to zone 0 |
| KaganCubukcu/g213-lightcontrol | 0 | Python | MIT | 2026-02-17 | G213 | GUI that shells out to `g213-led` (g810-led). No protocol code |
| ie04/openrgb-presets | 0 | Rust | all rights reserved | 2026-08-16 | G513 c33c, G502 HERO | OpenRGB SDK client. OpenRGB names the G513 "Logitech G512 RGB". Concurrent clients interleave the G513's multi-packet updates |
| cj-vana/macos-logitech-rgb | 1 | Swift | MIT | 2026-06-09 | G Pro c339 (tested), G410/G512/G513/G610/G810 | macOS, SET_REPORT on endpoint 0 to interface 1. Logo LED only latches from a 20 byte `0x11` frame. No brightness command |
| minimec/g810-profile | 5 | Shell | none | 2018-06-06 | g810-led keyboards | Per-application profiles by piping files into `g810-led -pp` |
| EricMountain/keyboard-animation | 0 | Python | MIT | 2022-05-15 | g810-led keyboards | Animations by spawning `g810-led -pp` every 0.3 s |
| levid0s/Logi-SetRGB | 0 | C# | GPL-3.0 | 2023-04-09 | any LED SDK keyboard | Windows LED SDK behind a named pipe. Ships the SDK DLL as a binary |
| testudor/Logitech-WLED-Sync | 23 | C# | MIT | 2022-09-17 | any LED SDK device | Uses the LED SDK (`LogiLedSetLighting`), not raw HID. Listens for WLED UDP sync packets |
| keyleds/keyleds | 195 | C/C++ | GPL-3.0 | 2021-04-08 | G410, G513, G610, G810, G910, G Pro | Picks the hidraw node by USB interface protocol and the report size from the report descriptor. Uses 0x8080 fn4 to fill a whole block with one 20 byte report |
| acehighdan/openrgb-logitech-g915tkl-fix | 1 | C++ | GPL-2.0 | 2026-04-12 | OpenRGB fork, G915/G915 TKL | Snapshot of OpenRGB's legacy detectors: per-PID interface, usage page and usage, plus hardcoded G915 indexes for wired and receiver |
| TheMorpheus407/g915x-heatmap | 5 | Python | MIT | 2026-06-19 | G915 X c356, c359 | 0x8081 per-key on hidraw with runtime index lookup. Needs a 0x8071 SW-control handshake first, or edge LEDs return error 5 |
| joshman1019/g915x-rgb | 1 | Python | MIT | 2026-03-28 | G915 X c359 | hidraw, interface 2, device index 0x01. Never takes SW control and so needs one commit per "special" key |
| Electronic-Supersonic/logitech-g915x-driver | 0 | Rust | GPL-2.0 | 2026-03-24 | G915 X c356 | hidapi filter: interface 2, page 0xFF00, usage 2. Hardcoded indexes 0x08/0x09 |
| csutcliff/gkeys-rs | 0 | Rust | GPL-3.0 | 2026-09-25 | G815 c33f | G-key daemon on hidraw interface 1 that leaves the kernel driver attached. Disables onboard mode with 0x8100 |
| lavers/g815-driver | 1 | Rust | none | 2021-08-13 | G815 c33f | hidapi interface 1. Protocol notes show G HUB answering 0x8071 user activity events with SetSWControl |
| developersu/LogiLedus | 2 | Java | GPL-3.0 | 2026-07-14 | G513 c33c | usb4java SET_REPORT on interface 1. Extra wave direction bytes 6, 7, 8 and ripple effect 6 |
| italktocomputers/carbon | 0 | C | GPL-2.0 | 2017-10-11 | G413 c33a | hidapi `hid_open`, 0x8070 static red with brightness as the red channel |
| grumpybollocks/g910-control | 1 | Python | MIT | 2026-09-20 | G910 | GUI on top of keyleds. Six G910 keys have no LED |
| others (short) | | | | | | federico-duppa/logitech-g515-onboard (already in our notes), Nabsflix/g515-rgb-controller (G HUB legacy LED SDK, Windows), x00FB/g515-tweaks (hwdb Fn swap, lists the three G515 LS IDs), xpire/logilight (snap around g810-led), df8819/G915ColorC (calls `ratbagctl`), hakangit/g512-keyboard-controller (OpenRGB SDK, Windows), Tom1243/Logitech-Pro-g815-m1---mr-off (0x8020 M-key LEDs off) |

Skipped: `Logitech-G-HUB-2026/.github` turned up in the "lightsync" search.
It is an organisation named after G HUB, created 2026-09-07, whose repo holds
only a README and a profile describing G HUB. It looks like a lure that
impersonates Logitech, so it was not opened beyond the metadata.

## Which keyboards use which interface, page and reports

This is the evidence asked for. It explains why `hidpp/discover.go` misses
these keyboards: it only accepts a descriptor that contains `06 00 FF`
followed by `85 11`.

| Keyboard (PID) | USB interface | Usage page / usage | Reports | Evidence |
|---|---|---|---|---|
| G213 (c336) | 1 | 0xFF43 / 0x0602 | 0x11 (0x12 not used) | OpenRGB detector `1, 0xFF43, 0x0602` [SRC acehighdan fork]; SET_REPORT `wIndex 1`, IN endpoint 0x82 [CAP G213Colors] |
| G413 (c33a) | 1 | 0xFF43 [INF] | 0x11 | g810-led, carbon [SRC] |
| G410, G512 (c342), G512/G513 (c33c), G610 (c333/c338), G810 (c331/c337), G Pro (c339), G910 (c32b/c335), G813 (c232), G815 (c33f) | 1 | 0xFF43 / 0x0602 for 0x11, 0x0604 for 0x12 | 0x11 (20 bytes), 0x12 (64 bytes) | OpenRGB detector `1, 0xFF43` and its comment "Usage 0x0602 for 20 byte, usage 0x0604 for 64 byte, both are on usage page 0xFF43" [SRC]; lgkb PROTOCOL.md [SRC]; G815 descriptor below [CAP] |
| G915 wired (c33e), G915 TKL wired (c343) | 2 | 0xFF00 / 2 | 0x10, 0x11 [INF] | OpenRGB detector `2, 0xFF00, 2` [SRC] |
| G915 receivers (c541, c545, c547) | 2 | 0xFF00 / 2 | 0x10, 0x11 | OpenRGB detector [SRC]; g810-led issue #198 (c541: interface 0 keyboard EP 0x81, 1 mouse EP 0x82, 2 HID++ EP 0x83) [ISSUE] |
| G915 X LIGHTSPEED, cable (c356) | 2 | 0xFF00 / 2 | 0x10, 0x11 | logitech-g915x-driver hidapi filter [SRC]; g915x-heatmap ("the keyboard accepts the long form for everything") [SRC] |
| G915 X variant (c359) | 2 ("INTERFACE=3/0/0") | not recorded | 0x10, 0x11 | g915x-rgb [SRC] |
| G515 TKL (c358), G515 LS TKL cable (c355) | 2 | 0xFF00 | 0x10, 0x11 | [HW] for c358; logitech-g515-onboard requires `HID_PHYS` ending in `/input2` for c355 [SRC] |

The G815 descriptor is in G HUB's GET_DESCRIPTOR traffic in
`dumps/g815/G815-Hotplug-to-start.pcapng` [CAP g810-led-resources]:

```
06 43 ff  0a 02 06  a1 01  85 11  75 08  95 13  15 00  26 ff 00  09 02 81 00  09 02 91 00  c0
06 43 ff  0a 04 06  a1 01  85 12  75 08  95 3f  15 00  26 ff 00  09 04 81 00  09 04 91 00  c0
```

So usage page 0xFF43, usage 0x0602 with report 0x11 (19 data bytes, input
and output), and usage 0x0604 with report 0x12 (63 data bytes). There is no
report 0x10. G HUB sends every request on this keyboard as a long `0x11`
report, IRoot lookups included. On Linux both collections belong to the same
interface and therefore to the same `/dev/hidrawN` node [INF from the
descriptor and the OpenRGB detector, which opens one path for both handles
when built against hidraw]. This answers the "[unverified]" note in
`8080-8081-per-key-lighting.md`: on these keyboards report 0x12 is sent on
the same hidraw node as 0x11.

Device index: corded keyboards ignore byte 1. A G915 owner reports that
`0xFF` works on the cable, and that only the receiver uses it to pick the
target (`0x01` keyboard, `0xFF` receiver) [ISSUE g810-led #198, yawor].
g915x-rgb talks to c359 with index `0x01`, g915x-heatmap probes `0xFF` and
then `0x01` on c356 [SRC].

## MatMoul/g810-led

C++, GPL-3.0, v0.4.3, last commit 2022-11-30 (pushed 2023-08). The table of
models, the `-tuk` families, the per-model byte strings and the libusb build
are in `other-protocols-and-tools.md` section 1. New details from a full read
of `src/classes/Keyboard.cpp`, `Keyboard.h` and the issue tracker:

**Interface selection.** There is none in the hidapi build. `open()` matches
VID/PID in `hid_enumerate()` and then calls `hid_open(vid, pid, serial)`
(`Keyboard.cpp` L210), which opens the first enumerated node of that PID.
With hidapi-hidraw that is normally interface 0, the boot keyboard. A G915
owner traced this exactly: "In hid mode, the code opens first hidraw
interface it finds with given VID/PID ... This fails because it tries to
open interface 0" [ISSUE #198]. On the older keyboards the HID++ writes
therefore go to interface 0 as SET_REPORT and are apparently accepted by the
firmware [INF: the hidapi build is the default and is widely used]. Replies
then arrive on interface 1, which g810-led never reads. The libusb build
hardcodes interface 1 and IN endpoint 0x82 (L1043-L1051), so it cannot work
on keyboards whose HID++ interface is 2 (G915 family).

**Write path.** hidapi build: open, `hid_write`, close for every packet,
with the reply read commented out. libusb build: `libusb_control_transfer(h,
0x21, 0x09, 0x0212 or 0x0211, 1, ...)`, `usleep(1000)`, then a 1 ms
interrupt read from 0x82 to drain the ack.

```cpp
// g810-led src/classes/Keyboard.cpp L1043 (GPL-3.0)
if (data.size() > 20) {
	if(libusb_control_transfer(m_hidHandle, 0x21, 0x09, 0x0212, 1,
```

**Very long frames (0x8080 SetKeyColors, fn3).** Header
`12 ff 0c 3a 00 <keyType> 00 <count>` then up to 14 `keyId R G B` quads,
padded to 64 bytes. g810-led always writes count `0x0e` for main keys and
`0x05` for indicators and media, even when fewer quads follow, and pads with
zero quads (keyId 0 is "padding" per OpenRGB). The logo is the exception:
`11 ff 0c 3a 00 10 00 01 <id R G B>` in a 20 byte report. lgkb found on
hardware that the G Pro logo ignores the 64 byte form (see below). Key IDs
in the keyboard block are HID usages (`Key::a` is 0x04); media IDs are
consumer usages (0xb5 next, 0xb6 prev, 0xb7 stop, 0xcd play, 0xe2 mute);
indicators are 1 backlight, 2 game, 3 caps, 4 scroll, 5 num.

**Commit.** `11 ff 0c 5a` (0x8080 fn5) on G410/G512/G513/G610/G810/G Pro,
`11 ff 0f 5d` on G910, `11 ff 10 7f` on G815 (0x8081 fn7 frameEnd). G213 and
G413 return without sending anything ("Keyboard is non-transactional").

**G815 per-key.** g810-led groups keys by colour and sends 0x8081 fn6
(setRgbZonesSingleValue) `11 ff 10 6c R G B id1..id13`, terminating with
`0xff` when fewer than 13 IDs fit (L433). The ID mapping matches our 0x8081
table: keys usage - 3, modifiers usage - 0x78, logo 0xd2, backlight
indicator 0x99, media 0x9b play, 0x9c mute, 0x9d next, 0x9e prev, G1-G5
0xb4-0xb8. Caps/scroll/num/game indicators and `logo2` are dropped on the
G815.

**G815 native effects.** Before every 0x8071 effect it sends
`11 ff 0f 5c 01 03 03` (0x8071 fn5 manageSwControl: set, flags 3, events 3),
then `11 ff 0f 1c <cluster> <effect> ...` with byte 16 (payload byte 12, the
persistence byte) set to 1. Cluster 1 is keys and 0 is the logo; logo effect
indexes differ (breathing 3, cycle 2), which g810-led remaps. These match
G HUB's own G815 frames in the g810-led-resources capture:

```
11ff0f1c 01 01 8d40ff 02 000000000000 01 000000   static keys
11ff0f1c 00 01 8d40ff 02 000000000000 01 000000   static logo
11ff0f1a 01 04 000000 000000 88 01 64 13 01 ...   wave keys: periodLSB, dir, 0x64, periodMSB
```

**Startup mode, decoded.** `-s color|wave` sends `11 ff 0d 5a 00 01 <mode>`
(L852) on G213/G410/G610/G810/G Pro and `11 ff 10 5e 00 01 <mode>` on G910.
With the 0x8070 function table this is fn5 setNvConfig(capability 0x0001
bootUpEffect, state): state 1 = enabled (plays the stored effect, which
g810-led calls "wave"), state 2 = disabled (keeps the last colours, "color").
The captures confirm it: LGS on a G810 sends `11ff0d5a 0001 02` for "boot
colour" and `0001 01` for "boot rainbow"; LGS on a G213 sends
`11ff0c5d 0001 02` / `0001 01` [CAP g810-led-resources]. On the G213 0x8070
is at index 0x0c, so g810-led's G213 startup command to index 0x0d goes to
some other feature. This is a g810-led bug on the G213, inherited by the
Rust port [INF from CAP]. This also settles the uncertainty in
`other-protocols-and-tools.md` 1.3 about `0d 5a`.

**On-board mode.** `11 ff 11 1a <01 board | 02 software>` on the G815 only
(0x8100 fn1 setOnboardMode). G HUB sends `11 ff 11 1c 02` at hotplug.

**Region (G213).** `11 ff 0c 3a <region 1..5> 01 R G B` (L833), 0x8070 fn3
setZoneEffect with the static effect. LGS adds a `02` after the colour (the
static "no effect / instant" mode byte from our 0x8070 notes); g810-led
leaves it 0.

**G413.** `setAllKeys` uses the native static effect on zone 0, because the
G413 has no per-key control.

**Brightness.** None. No 0x8040 call anywhere.

**Quirks from issues.** G915 is unsupported (#198, #223, #290, all open or
closed without code). #198 also documents the G915 receiver interface layout
and that on the receiver the 0x8081 index is 0x0b (`11 01 0b 1d ...`,
`11 01 0b 6d ...`, commit `11 01 0b 7d`), against 0x10 on the wired G815.
The G512 SE (c342) has "only effects work, not stationary colors" (#226,
closed) and an open support request (#230). The newest open issue is a
udev add/remove loop through a KVM (#323, 2026-10).

## MatMoul/g810-led-resources

GPL-3.0, 2020-04-26. USB captures (USBPcap and usbmon) taken with LGS or
G HUB. Decoded for this note [CAP]:

- **G815 hotplug, G HUB.** Every request uses report 0x11 and software ID
  0xC. Order: IRoot ping, IFeatureSet enumeration, then lookups. Index map
  on this unit: 0x8040 at 0x0d, 0x8071 at 0x0f, 0x8081 at 0x10, 0x8100 at
  0x11, 0x4540 at 0x09, 0x8010/0x8020/0x8030 at 0x0a/0x0b/0x0c (G-keys,
  M-keys, MR). 0x8070 and 0x8080 are looked up and absent. G HUB reads the
  0x8100 sectors with fn5, sets host mode (`11 1c 02`), enables G-key
  diversion (`0a 2c 01`), lights M1 (`0b 1c 01`), then for 0x8071 sends
  fn5 get, `5c 01 03 07`, fn7 `7c 01`, fn8 `8c 01 01`, `5c 01 03 05`,
  fn3 `3c 00 00 01`, enumerates effects with fn0 `0c ff 00 01 ..`, sets
  both clusters and finishes with `5c 01 03 03`. It also looks up 0x8310 and
  then calls index 0x08 fn3 and fn1 around every lighting change
  (`g815-raw-data.txt` calls them "Unknown, often called after the commit").
  Solaar names 0x8310 EQUALIZER; what it does on a keyboard is unknown
  [INF].
- **G815 all keys, G HUB.** Nine `11 ff 10 6c FD 76 00 <13 ids>` frames then
  `11 ff 10 7c` commit. The ID set is 0x01-0x6f minus 0x2f, 0x61 and
  0x63-0x67, then 0x9d 0x9e 0x9b 0x9c, 0xb4-0xb8, 0xd2, 0x99. G HUB pads
  short frames with 0x00, not 0xff.
- **G815 effects.** G HUB follows breathing and cycle with 0x8071 fn6
  setEffectSyncCorrection per cluster (`0f 6a 00 00 f7 17`).
- **G810 per-key, LGS (usbmon).** LGS first clears each block with 0x8080
  fn4 `11 ff 0c 4a 00 <block> ...`, then streams `12 ff 0c 3a 00 01 00 0e`
  frames, commits with `11 ff 0c 5a`, then calls fn0 and fn1. The ack to a
  0x12 request comes back as a 0x11 report (`11 ff 0c 3a 00 01 00 0e 00..`).
  Key IDs include 0x64 (ISO), 0x87-0x8b (international keys).
- **G213, LGS.** Each zone write is preceded by 0x8070 fn0 getInfo, whose
  reply is `06 00 01`: six zones (0 = whole board, 1-5 regions) and
  nvCapabilities 0x0001 bootUpEffect.
- **G413, LGS.** 0x8070 at 0x0c, getInfo reply `01 00 01 00 08`, static
  effect `0c 3b 00 01 82 00 00 02` (single colour; first colour byte is the
  level).

## peppapig450/logitech-led-control

Rust 2024, GPL-3.0, crate `logi-led` 0.1.0, last commit 2026-04-15. A
rewrite of g810-led with the same PID table, a data-driven `ModelSpec` per
model, the same hidapi and optional libusb (rusb) backends (libusb build
claims interface 1, `0x0211`/`0x0212`), and g810-led profile file
compatibility. The hidapi backend filters `device_list()` by VID/PID only and
opens the first match (`device/hid.rs`), so it has the same interface 0
behaviour. Writes of 21 to 63 bytes are rejected; only 20 or 64 bytes pass.

Divergence from g810-led that looks like a porting bug [INF]: the key block
header for G213, G410, G413, G512, G513, G Pro, G610 and G810 is

```rust
// peppapig450/logitech-led-control src/keyboard/spec.rs L126 (GPL-3.0)
(4, &[0x12, 0xff, 0x0f, 0x3d, 0x00, 0x01, 0x00, 0x0e]),
```

which is g810-led's G910 header (index 0x0f). g810-led uses
`12 ff 0c 3a 00 01 00 0e` for those models. On a G513 index 0x0f is not
0x8080 (it is at 0x0c), so per-key colours would fail. The G213 startup
header is also `11 ff 0d 5a 00 01` (same bug as g810-led). G815 effects
reproduce the `0f 5c 01 03 03` setup and byte 16 = 1.

## klyse/LogitechKeyboardLED

Go, MIT, last commit 2020-08-29 (pushed 2023). A Windows-only shortcut helper
for G815/G915: a keyboard hook lights the keys of IDE shortcuts while
modifiers are held. All lighting goes through the Logitech LED SDK via
`windows.NewLazyDLL("LogitechLedEnginesWrapper.dll")`
(`LogiLedInit`, `LogiLedSetLightingForKeyWithKeyName`, `LogiLedFlashSingleKey`,
`LogiLedPulseSingleKey`, `LogiLedStopEffects`). The repository contains the
20 KB SDK DLL as a binary and two large GIFs. Nothing about the wire
protocol. Not useful for logitech-rgb except as a reminder that on Windows
the G815/G915 per-key path goes through G HUB.

## SebiTimeWaster/G213Colors

Python (pyusb), MIT, last commit 2021-02-05, 155 stars. Single script plus
11 Windows USBPcap captures from 2016.

Transport: `usb.core.find(0x046d, 0xc336)`, detach the kernel driver from
interface 1, `ctrl_transfer(0x21, 0x09, 0x0211, 0x0001, data)`, then
`device.read(0x82, 20)` because "otherwise commands may not be completely
executed". The README warns that detaching interface 1 kills the media keys
while the script runs.

Commands (all 0x8070 at index 0x0c, fn3 setZoneEffect):

```python
# SebiTimeWaster/G213Colors G213Colors.py (MIT)
colorCommand   = '11ff0c3a{}01{}0200000000000000000000'
breatheCommand = '11ff0c3a0002{}{}006400000000000000'
cycleCommand   = '11ff0c3a0003ffffff0000{}64000000000000'
```

Zone 0 = whole keyboard, 1-5 = regions. The `02` after the colour is the
static "no effect" mode byte; breathing is `RGB periodBE 00 0x64`; cycle is
`RGB 00 00 periodBE 0x64`. Periods are 32-65535 ms.

What the captures add [CAP]: setup packet `21 09 11 02 01 00 14 00`
(SET_REPORT, output report 0x11, interface 1, 20 bytes); LGS precedes each
write with fn0 getInfo; the "game key" capture uses index 0x03 with
`11 ff 03 1a e3 65 e7 39` (disable left GUI, menu, right GUI, caps lock),
which is 0x4522 disable keys by usage; LGS varies the software ID (0xA and
0xD). The wave capture shows LGS setting the firmware wave effect once
(`11ff0c3a 0004 ffb4aa 000000 88 01 64 13`) and then polling 0x8070 fn12
getCurrentColor (`11ff0cca <zone>`, reply `<zone> R G B`) for all five zones
about 250 times. So wave is a firmware effect on the G213 and LGS only reads
the colours back, most likely for its on-screen preview. This contradicts
the README ("completely generated in the software") [INF from CAP]. The
device also sends unsolicited `11 ff 0c 00 00 <u16>` reports after breathing
or cycle is set: 0x8070 event 0 syncEffectEvent with the effect counter.

## Agundur-KDE/G213Tray

Python (PyQt5 + pyusb), GPL-3.0, 2026-07-20. KDE tray applet. Detaches
interface 1, sends one packet, releases and reattaches:

```python
# Agundur-KDE/G213Tray g213tray.py (GPL-3.0)
pkt = [0x11, 0xff, 0x0c, 0x3a, 0x00, 0x01, r, g, b,
       0x00, 0x00, ...]
dev.ctrl_transfer(0x21, 0x09, 0x0211, iface, pkt)
```

Zone 0, static, no mode byte, no reply read. The udev rule is
`SUBSYSTEM=="usb" ... MODE="0666", TAG+="uaccess"` on the whole USB device.
Toggle "off" is just colour 000000. No brightness, no onboard handling.

## KaganCubukcu/g213-lightcontrol

Python Tkinter GUI, MIT, 2026-02-17. Runs `g213-led -a <hex>` through
`subprocess`; also calls `g213-led -nm`, which is not a g810-led option. Its
`setup_permissions.sh` writes a world-writable (`MODE="0666"`) udev rule for
the whole USB device. No protocol content.

## ie04/openrgb-presets

Rust, "All rights reserved" (no open source license), 2026-08-16. A client
of the OpenRGB SDK server (`openrgb2` crate) that sets static cyan on a G513
and a G502 HERO and runs a keypress ripple at about 30 FPS on the keyboard,
reading evdev without grabbing. Useful details:

- OpenRGB reports the G513 (`046d:c33c`) as `Logitech G512 RGB` and drives
  it with the G810-family controller, a 7x23 matrix.
- The evdev node it uses is `usb-Logitech_G513_Carbon_*-event-kbd`; it
  "deliberately ignores the G513 `if01` keyboard endpoint", i.e. interface 1
  also carries a keyboard-class input device next to the vendor collection.
- "Concurrent clients can interleave the G513's multi-packet updates and
  produce mixed colors." 0x8080 frames are staged in a device buffer and
  only shown on commit, so two writers interleaving frames before one commit
  mix their colours [INF].

## cj-vana/macos-logitech-rgb (lgkb)

Swift, MIT, 2026-06-09. CLI and menu bar app for the G Pro keyboard on
macOS, tested on c339, written from g810-led and OpenRGB. `docs/PROTOCOL.md`
is a clean description of the 0x8080/0x8070 family.

Transport: `IOUSBHostDevice` control transfers (bmRequestType 0x21,
SET_REPORT, wValue `0x0211`/`0x0212`, wIndex 1). This avoids macOS Input
Monitoring approval. Acks are never read ("macOS's HID driver drains
those"); a ~1 ms gap keeps streaming stable; sends are retried 4 times.

Bytes (index 0x0c = 0x8080, 0x0d = 0x8070, software ID 0xD):

```
12 FF 0C 3D 00 ZZ 00 NN  [key R G B] x NN          per-key, ZZ 01 keys, 10 logo, 40 indicators
11 FF 0C 3D 00 10 00 01  01 R G B                   logo, must be the 20 byte form
11 FF 0C 5D                                         commit
11 FF 0D 3D ZZ MM ...                               0x8070 effect, ZZ 00 keys, 01 logo
11 FF 0D 5A 00 01 MM                                startup: 01 stored effect, 02 last colours
```

New or confirming details: the G Pro logo LED "ignores 64-byte frames. It
only latches via the 20-byte report form" (verified on hardware); the G Pro
Fn key LED is at usage 0xE7 (the Right GUI slot the board lacks); wave
directions 0 cycling, 1 horizontal, 2 vertical, 3 centre-out; byte 16 = 1
persists the effect to the user slot; "a commit after effect packets makes
the logo zone latch reliably"; "There is no brightness command; brightness
is RGB scaling in software"; committed colours survive until power off.
G Pro X, G Pro X TKL, G815 and G915 are listed as unsupported.

## minimec/g810-profile

Bash, no license file, 2018-06-06. Polls `ps` for running programs and
loads matching profile files with `g810-led -pp < file`; also listens to
MPRIS on D-Bus to colour media keys. Profile files are g810-led's text
format (`k <key> <colour>`, `c` to commit). No protocol content.

## EricMountain/keyboard-animation

Python, MIT, 2022-05-15. Builds a g810-led profile string per frame and
pipes it into `g810-led -pp` through `subprocess.call(..., shell=True)` with
a 0.3 s sleep. Shows that one process launch per frame (each packet is its
own open/write/close in g810-led) limits animations to a few frames per
second. No protocol content.

## levid0s/Logi-SetRGB

C#, GPL-3.0, 2023-04-09. Windows console app that opens a named pipe and
maps `setkey,<id>,r,g,b` / `setall,r,g,b` to `LogiLedSetLightingForKeyWithKeyName`
and `LogiLedSetLighting` from the LED SDK (`LogitechGSDK.cs` plus the SDK DLL
committed as a binary under `lib/`). Key IDs are the SDK's scan-code style
`keyboardNames` (0x0E Backspace, 0x3B F1). No HID++.

## testudor/Logitech-WLED-Sync

C#, MIT, last commit 2022-09-17, 23 stars. Uses the **LED SDK**, not raw
HID: `LogitechGSDK.LogiLedInitWithName`, `LogiLedSetLighting(R%, G%, B%)`,
`LogiLedShutdown`. The README tells the user to download the SDK and drop
`LogitechLedEnginesWrapper.dll` next to the exe. The data flow is WLED to
Logitech: it listens for WLED UDP sync packets and reads brightness from
byte 2 and RGB from bytes 3-5, then cross-fades with a 1 ms loop. SDK colour
values are percentages (0-100), scaled here by WLED brightness. No device
specific code at all.

## keyleds/keyleds

C (libkeyleds) and C++ (keyledsd), GPL-3.0, last commit 2021-04-08, 195
stars. Supports G410, G513, G610, G810, G910 and G Pro over hidraw. Its
0x8080 function table is already in our notes. New for us:

- **Node selection.** keyledsd accepts a Logitech hidraw node when the
  parent USB interface has `bInterfaceProtocol == 0` (not a boot keyboard or
  mouse); the shipped udev rule does the same:
  `ATTRS{bInterfaceProtocol}=="00", TAG+="uaccess"`.
- **Report choice from the descriptor.** `keyleds_open()` reads the report
  descriptor (`HIDIOCGRDESC`), keeps every output report whose usage is on a
  vendor page (`(usage & 0xff000000) == 0xff000000`, so 0xFF00 and 0xFF43
  alike), 8 bit items, 0-255, and sends each call in the smallest report
  that fits. That is how it gets 0x12 for long payloads and 0x11 for the
  rest, with no PID table.
- **0x8080 fn4.** `keyleds_set_led_block(block, R, G, B)` fills one key
  block with one colour in a single short call:
  `[blockId u16, R, G, B]`, then fn5 commit. LGS uses the same call with
  black to clear blocks before streaming. Our notes say fn4 is "not used".
- A G410 commit takes 10-15 ms.

## acehighdan/openrgb-logitech-g915tkl-fix (OpenRGB legacy controllers)

GPL-2.0 fork of OpenRGB, 2026-04-12. Its own fix is to map receiver c547 to
the G915 TKL layout and to always write 20 byte packets in the G915
controller ("The keyboard silently ignored these truncated packets"). Its
real value here is that it still contains the per-model legacy detectors,
which upstream has been replacing with the unified HID++ 2.0 controller:

```cpp
// Controllers/LogitechController/LogitechControllerDetect.cpp (GPL-2.0)
REGISTER_HID_DETECTOR_IPU("Logitech G213", ..., LOGITECH_G213_PID, 1, 0xFF43, 0x0602);
REGISTER_HID_DETECTOR_IP ("Logitech G815 RGB Mechanical Gaming Keyboard", ..., 1, 0xFF43);
REGISTER_HID_DETECTOR_IPU("Logitech G915TKL Wireless RGB Mechanical Gaming Keyboard (Wired)", ..., 2, 0xFF00, 2);
```

PIDs: G512 c342, "G512 RGB" c33c, G813 c232, G815 c33f, G915 wired c33e,
G915 receivers c541 and c547, G915 TKL wired c343, G915 TKL receiver c545,
G Pro c339, G910 c32b/c335, G610 c333/c338, G810 c331/c337. The G915
controller hardcodes indexes: wired 0x4522=0x0e, 0x8040=0x13, 0x8071=0x09,
0x8081=0x0a with device index 0xFF; receiver everything one higher with
device index 0x01. The G810 controller writes 0x12 frames on the 0x0604
handle and reads the ack from the 0x0602 handle (a 0x11 report).

## TheMorpheus407/g915x-heatmap

Python stdlib, MIT, 2026-06-19. A typing heatmap daemon for the G915 X on
Linux hidraw, with a good `PROTOCOL.md` derived from G HUB captures in
OpenRGB issue #4461 and checked on hardware.

- **Discovery.** Matches `HID_ID` 046d:c356 or c359, opens each node and
  sends an IRoot ping; "The other c356 nodes reject writes with EPIPE". So
  writing a ping is a usable way to find the HID++ node when the descriptor
  test is ambiguous.
- **Indexes** resolved at runtime: 0x8071 at 0x08 and 0x8081 at 0x09 on the
  tested unit; 0x4522 absent.
- **0x8081.** fn1 `11 FF 09 1d [id R G B]x4`, fn5 range
  `11 FF 09 5d [first last R G B]x3`, fn6 `[R G B][id x13]`, fn7 commit.
  G1-G9 are 0xB4-0xBC, logo 0xD2, indicator 0x99, media 0x9B-0x9E; the
  wireless/Bluetooth/mode row LEDs have no ID.
- **SW control handshake.** Without it "the edge/media LEDs reject writes
  with HID++ error 5". It replays G HUB:

```
10 FF <8071> 3d 00 00 20         fn3 manageNvConfig get
10 FF <8071> 5d 00 00 00         fn5 get SW control
10 FF <8071> 5d 01 03 07         fn5 set flags 3, events 7
11 FF <8071> 7d 01 00 00 00 3c 01 2c   fn7 power config 60 s / 300 s
11 FF <8071> 7d 01 00 00 00 00 00 5a
10 FF <8071> 5d 01 03 05
10 FF <8071> 3d 00 00 01
```

- "Entering software/host mode blanks every LED not in the buffer", and
  "Never send an 0x8071 effect frame while streaming per-key, it flips the
  board back to the onboard firmware profile and blanks the 0x8081 buffer."
- Suspend drops software lighting; a resume unit repaints it.
- The README says c356 is the receiver and c359 the cable. OpenLogi's test
  data for the G915 X LS lists model IDs `[0xb38a, 0x40b5, 0xc356]`
  (BTLE, eQuad, USB) [SRC AprilNEA/OpenLogi `device.rs`], and our 8101
  notes have c356 / 40B5. So c356 is the USB cable PID and the README is
  wrong on that point; the code works either way because it probes `0xFF`
  and `0x01`.

## joshman1019/g915x-rgb

Python (GTK4), MIT, 2026-03-28. Per-key GUI and systemd user service for
the "G915 X" with PID **c359**. Finds the hidraw node whose USB interface
uevent has `INTERFACE=3/0/0` (HID, no subclass, no protocol), which is
interface 2. Uses device index 0x01 ("Keyboard behind Lightspeed receiver")
and short `0x10` getFeature with software ID 0. Writes 0x8081 fn1/fn5/fn6/fn7
the same way as the heatmap. It never takes SW control, and its notes say
"Special keys (modifiers, G-keys, media, logo) need individual commits to
work reliably". Given the heatmap's error 5 finding, that workaround is
probably a symptom of the missing 0x8071 handshake [INF]. Which G915 X
variant c359 is remains unresolved.

## Electronic-Supersonic/logitech-g915x-driver

Rust (hidapi, evdev), GPL-2.0, 2026-03-24. Small, no README. Its device
filter is the cleanest evidence for the G915 X interface:

```rust
// src/main.rs (GPL-2.0)
&& d.usage_page() == 0xff00
&& d.interface_number() == 2
&& d.usage() == 2
```

PID c356, device index 0xFF, hardcoded indexes: 0x8071 at 0x08
(`11 ff 08 54 01 01`, fn5 set SW control), 0x8081 at 0x09 (`11 ff 09 1a`
per key, `11 ff 09 7a` commit). It commits before every key, which is
slow but works.

## csutcliff/gkeys-rs

Rust, GPL-3.0, last commit 2026-09-25. G-key macro daemon for
the G815 that "uses hidraw instead of libusb, so the kernel HID driver
remains attached and OpenRGB can control keyboard lighting". Picks the c33f
hidraw node whose sysfs path contains `:1.1/` (interface 1). Init: IRoot
ping until the firmware answers (KVM re-enumeration), drain stale reports,
match replies by feature and function, 0x8100 setOnboardMode(2), 0x8010
enable diversion. LEDs: `11 ff 0b 1c <mask>` (M1-M3), `11 ff 0c 0c 01` (MR),
`11 ff 10 6c R G B b4..b8 ff` then `11 ff 10 7c` (G-key colours). Lists G915
c33e and G915 TKL c343 as probably compatible. Notes that after an MR LED
write the keyboard echoes a phantom MR event, debounced for 30 ms.

## lavers/g815-driver

Rust (hidapi), no license file, 2021-08-13. Opens c33f interface 1.
`protocol-notes.txt` is a set of G HUB traces with useful behaviour:

- On every key press the G815 sends a 0x8071 userActivity event
  (`11 ff 0f 10 01`) and G HUB answers `11 ff 0f 5a 01 03 05`; on release
  `11 ff 0f 10 00` and G HUB answers `01 03 03`.
- The light key sends a 0x8040 event `11 ff 0d 00 00 <level>` with levels
  0x64, 0x32, 0x19, 0x0c, 0x00 (100, 50, 25, 12, 0 %).
- On an M-key press G HUB "fully redraws keyboard", including setting IDs
  0x2f, 0x61, 0x63-0x67 to black ("unknown invisible keys").

## developersu/LogiLedus

Java (usb4java, OpenJFX), GPL-3.0, 2026-07-14 (GitHub mirror). G513 c33c GUI.
libusb: claim interface 1, `controlTransfer(0x21, 0x09, 0x211|0x212, 1)`,
read the ack from 0x82. Per-key `12 ff 0c 3a 00 01 00 0e`, indicators
`12 ff 0c 3a 00 40 00 05`, commit `11 ff 0c 5a`. Effects on 0x8070 at 0x0d:
static `00 01 R G B 02`, breathing `00 02 RGB periodBE 00 64`, ripple
("circles on press") effect **6** with period at byte 11, cycle `00 03 ...
periodBE 64`, wave `00 04 ... periodLSB <dir> 64 periodMSB` with direction
bytes 1, 2, 3, 6, 7, 8 (horizontal, vertical, centre-out and the three
reverses). g810-led only knows 1-3. Game mode uses 0x4522 at index 0x03.

## italktocomputers/carbon

C (hidapi), GPL-2.0, 2017. G413 c33a. `hid_open(vid, pid)` (first node),
`11 ff 0c 3c 00 01 <level> 00 00 02` static and `00 02 <level> 00 00 <speed> 00 64`
breathing on 0x8070 at 0x0c. The G413 has white or red LEDs only, so the
first colour byte works as brightness.

## grumpybollocks/g910-control

Python GUI on top of `keyledsctl`/libkeyleds, MIT, 2026-09-20. Hardware
notes: on the G910 Orion Spectrum "Win, Alt, AltGr, Menu, right-Ctrl,
right-Shift have no individual LED at all", they only follow a whole-board
fill; reading key colours (0x8080 fn2) while another writer is active
returns corrupt data.

## Other hits

- **federico-duppa/logitech-g515-onboard** (Python, GPL-3.0, 2026-09-10):
  already summarised in `8101-profile-management.md`. It confirms c355 HID++
  on `input2`, device 0xFF, long reports, 0x8101 index 0x0e vs 0x0f, and
  locks the hidraw node with `flock`.
- **x00FB/g515-tweaks** (MIT, 2026-10-12): hwdb only. Confirms the three
  G515 LS TKL IDs: Bluetooth `b389` (hid-generic), cable `c355`
  (hid-generic), receiver `c547`/`40b4` (logitech-hidpp-device).
- **Nabsflix/g515-rgb-controller** (Python, no license, 2026-05-30):
  Windows; loads `C:\Program Files\LGHUB\sdks\sdk_legacy_led_x64.dll`
  (the LED SDK as shipped inside G HUB) and the Razer Chroma REST API.
- **xpire/logilight** (Python, MIT, 2026-09-13): snap packaging around
  g810-led binaries, needs the `raw-usb` interface. Its PID table is copied
  from g810-led's udev rules.
- **df8819/G915ColorC** (Python, no license, 2025-03-27): runs `ratbagctl`
  to rewrite LED 0 (logo) and LED 1 (keys) of onboard profile 0 on a G915.
  So libratbag's 0x8100 support reaches G915 keyboards.
- **hakangit/g512-keyboard-controller** (C#, MIT, 2026-09-23): Windows
  OpenRGB SDK client; assumes a "108-LED G512 layout".
- **Tom1243/Logitech-Pro-g815-m1---mr-off** (Python hidapi, 2026-09-01):
  resolves 0x8020 at runtime and sends fn1 `00` to turn the M-key LEDs off.

Searches for G713, G715 and G512 Linux lighting found nothing relevant
(results were ASUS ROG laptops and unrelated projects). No public tool
targets the G515 TKL wired (c358) except Solaar, OpenRGB and logitech-rgb.

## Findings for logitech-rgb

### New

- **Discovery must accept usage page 0xFF43.** Concrete descriptor bytes for
  the G815 (`06 43 ff 0a 02 06 ... 85 11 ...` and `06 43 ff 0a 04 06 ... 85 12`)
  and the per-PID detector table above show that the G213, G413, G410,
  G512/G513, G610, G810, G813, G815, G910 and G Pro all put HID++ on
  interface 1, page 0xFF43. The minimal change to `hasLongReport` is to
  accept `06 43 FF` as well as `06 00 FF` before `85 11`. A more robust
  version is keyleds' rule: any Logitech node whose descriptor has an output
  report with a vendor usage page (0xFFxx), 8 bit items and report ID 0x11.
  An alternative is the heatmap's probe: write an IRoot ping and keep the
  node that answers (non-HID++ nodes fail the write with EPIPE).
- **No short report on these keyboards.** The G815 HID++ collection has no
  report 0x10, and G HUB sends every request as 0x11. `hidpp/device.go`
  already sends only long reports, which is right; keep it that way and do
  not add a short-report fast path without checking the descriptor.
- **0x12 lives on the same node.** On Linux the 0x12 collection is on the
  same interface, so the same `/dev/hidrawN` accepts 64 byte writes. The ack
  for a 0x12 request comes back as a 0x11 report (G810 capture, OpenRGB
  reads it from the 0x11 handle). `device.go` would need a `CallVeryLong`
  that writes 64 bytes and keeps the current 0x11 reply matching. Replies
  to 0x8080 fn2 GetKeyColors are 0x12, which the reader currently drops
  (`msg[0]` must be 0x10 or 0x11).
- **Solid colour on 0x8080 without 0x12.** 0x8080 fn4 `[blockId u16, R, G, B]`
  fills a whole key block in one 20 byte report (keyleds, LGS), followed by
  fn5 commit. For "set the whole keyboard to one colour" logitech-rgb could
  support the G410/G512/G513/G610/G810/G Pro family with only long reports:
  fn0 to read the block flags, fn4 per block (0x0001 keys, 0x0002 media,
  0x0004 G-keys, 0x0010 logo, 0x0040 indicators), fn5 commit. Per-key
  colours still need 0x12 or one key per fn3 call in a 0x11 report (the
  logo form shows fn3 accepts a single quad in 20 bytes).
- **Logo latching.** On the G Pro the logo only accepts the 20 byte fn3
  form. If logitech-rgb adds 0x12 per-key frames, send keyType 0x0010 in a
  0x11 report.
- **g810-led "startup mode" is 0x8070 setNvConfig(bootUpEffect).** State 1
  plays the stored effect at power on, 2 keeps the last colours. This is a
  cheap, documented persistence knob for 0x8070 keyboards (the G213 reports
  nvCapabilities 0x0001). g810-led sends it to index 0x0d on the G213,
  where 0x8070 is at 0x0c; resolve the index through IRoot.
- **G213 specifics.** 0x8070 with six zones (0 whole board, 1-5 regions),
  index 0x0c, 0x4522 at 0x03. No commit; each fn3 call shows at once.
  The firmware sends 0x8070 syncEffectEvent reports (software ID 0) after
  breathing and cycle, which our reader already treats as notifications.
- **0x8070 effect IDs on keyboards.** G513: 0 off, 1 static, 2 breathing,
  3 cycle, 4 wave, 6 ripple (LogiLedus). Wave direction byte: 1 horizontal,
  2 vertical, 3 centre-out, 6/7/8 the reverses. Parameter positions match
  our 0x8070 notes (wave: periodLSB at 8, direction 9, intensity 10,
  periodMSB 11).
- **G915/G815 family is mostly the 0x8071 + 0x8081 path we already have,**
  but on interface 1 / page 0xFF43 for the G815 and interface 2 / page 0xFF00
  for the G915 family. Behind a receiver (c541, c545, c547) the keyboard is
  device index 0x01 and every feature index shifts by one compared with the
  cable (OpenRGB, g810-led #198). logitech-rgb already resolves indexes at
  runtime; it needs receiver support (device index 1 on the receiver node)
  for the wireless case.
- **SW control before per-key on 0x8081 keyboards.** On the G915 X, edge and
  media LEDs return HID++ error 5 until 0x8071 manageSwControl has been set
  (G HUB uses `01 03 07`, then `01 03 05`). Taking host mode blanks every
  LED not in the 0x8081 buffer, and any 0x8071 effect write while streaming
  per-key returns the board to its onboard profile and clears the buffer.
- **G HUB's activity handshake.** G815 traces show the keyboard sending a
  0x8071 userActivity event on every key press and G HUB toggling
  manageSwControl between `01 03 05` (active) and `01 03 03` (idle). This is
  the host side of the idle handling we only knew from the spec.
- **Interleaving.** 0x8080/0x8081 updates are multi-packet and only shown on
  commit. Two writers (for example logitech-rgb and OpenRGB) produce mixed
  colours (openrgb-presets). Our hidraw `flock` style locking, as
  logitech-g515-onboard does, would at least stop two logitech-rgb processes
  from interleaving.

### Confirms

- Hardcoded indexes differ per model and per link: G815 0x8071 0x0f and
  0x8081 0x10 (wired) vs 0x0b for 0x8081 behind the G915 receiver; G915
  wired 0x09/0x0a vs receiver 0x0a/0x0b; G915 X 0x08/0x09; G213 0x8070 at
  0x0c vs G513 0x8070 at 0x0d. Runtime IRoot lookup, as logitech-rgb does,
  is the only safe approach.
- The 0x8081 zone ID map in `8080-8081-per-key-lighting.md` (usage - 3,
  modifiers usage - 0x78, logo 0xd2, indicator 0x99, media 0x9b-0x9e, G-keys
  from 0xb4) matches g810-led, G HUB's G815 frames and both G915 X tools.
  G915 X G-keys extend to G9 = 0xbc.
- 0x8071 setRgbClusterEffect payload byte 12 is the persistence flag; G HUB
  and g810-led set it to 1 on the G815.
- The G512 and G513 share PID c33c and OpenRGB calls it "G512 RGB"; c342 is
  the G512 (SE).
- Corded keyboards ignore the device index byte; `0xFF` is the convention.

### Contradicts or corrects

- `other-protocols-and-tools.md` 1.3 marks `0d 5a` as uncertain. It is
  0x8070 setNvConfig(bootUpEffect, state), see above.
- `8080-8081-per-key-lighting.md` says 0x8080 fn4 is "not used". keyleds
  and LGS use it as SetBlockLeds(block, R, G, B).
- The same file says "we have not seen a feature dump for the G513";
  `devices.md` has one from Solaar #2653 (0x8080 at 0x0c, 0x8070 at 0x0d),
  and the G810 captures agree with that layout.
- `8080-8081-per-key-lighting.md` marks 0x12 on the same hidraw node as
  unverified. The G815 descriptor shows both 0x11 and 0x12 collections on
  one interface, so on Linux they share a node.
- The G213Colors README says wave is a software effect. The captures show a
  firmware wave (effect 4) plus LGS polling getCurrentColor.
- g915x-heatmap says c356 is the G915 X receiver. Device model IDs put c356
  in the USB slot; it is the cable PID.
- peppapig450/logitech-led-control uses the G910 key header for the G513
  family; g810-led does not.

### What logitech-rgb needs, in order

1. Discovery: accept 0xFF43 as well as 0xFF00 (or any vendor page with
   report 0x11). This alone makes the G213, G413, G512/G513, G610, G810,
   G Pro and G815 visible, and their 0x8070 or 0x8071 zone effects should
   then work with the existing code.
2. For the G213, expose zones 1-5 and zone 0 ("all"); for 0x8070 keyboards
   add the bootUpEffect setNvConfig as the persistence option.
3. 0x8080 support: fn0/fn1 to list blocks, fn4 + fn5 for solid colours on
   long reports, then a very long (0x12) write path for per-key frames, with
   the logo in a 0x11 frame.
4. For 0x8081 keyboards beyond the G515 (G815, G915, G915 X): take 0x8071
   SW control before per-key writes, never mix 0x8071 effect writes into a
   per-key session, and support receiver device index 0x01.
5. Optional: lock the hidraw node, and document that only one lighting tool
   may write at a time.

## Sources

Projects (all read at the commit cloned on 2026-10-14):

- https://github.com/MatMoul/g810-led (commit efa3c35e), files
  https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.cpp,
  https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.h,
  https://github.com/MatMoul/g810-led/blob/master/udev/g810-led.rules,
  https://github.com/MatMoul/g810-led/blob/master/makefile
- g810-led issues: https://github.com/MatMoul/g810-led/issues/198,
  https://github.com/MatMoul/g810-led/issues/223,
  https://github.com/MatMoul/g810-led/issues/226,
  https://github.com/MatMoul/g810-led/issues/230,
  https://github.com/MatMoul/g810-led/issues/290,
  https://github.com/MatMoul/g810-led/issues/323
- https://github.com/MatMoul/g810-led-resources (dumps/g815, dumps/g810-spectrum,
  dumps/g213-prodigy, dumps/g413-carbon)
- https://github.com/peppapig450/logitech-led-control (src/keyboard/spec.rs,
  packet.rs, device/hid.rs, device/libusb.rs)
- https://github.com/klyse/LogitechKeyboardLED (LogiKeyboard/Keyboard.go)
- https://github.com/SebiTimeWaster/G213Colors (G213Colors.py, pcaps/)
- https://github.com/Agundur-KDE/G213Tray (g213tray.py, 99-g213.rules)
- https://github.com/KaganCubukcu/g213-lightcontrol (src/led_ctrl.py)
- https://github.com/ie04/openrgb-presets (README.md, src/ripple.rs)
- https://github.com/cj-vana/macos-logitech-rgb (docs/PROTOCOL.md,
  Sources/LgkbCore/USBTransport.swift, KeyboardModel.swift)
- https://github.com/minimec/g810-profile (src/g810-profile.sh)
- https://github.com/EricMountain/keyboard-animation (anim.py, paint.py)
- https://github.com/levid0s/Logi-SetRGB (Program.cs, lib/LogitechGSDK.cs)
- https://github.com/testudor/Logitech-WLED-Sync (WledSyncApplicationContext.cs)
- https://github.com/keyleds/keyleds (libkeyleds/src/device.c, hid_parser.c,
  feature_leds.c, keyledsd/src/device/Logitech.cxx, logitech.rules)
- https://github.com/acehighdan/openrgb-logitech-g915tkl-fix
  (Controllers/LogitechController/LogitechControllerDetect.cpp,
  LogitechG915Controller/LogitechG915Controller.cpp,
  LogitechG810Controller/LogitechG810Controller.cpp, OpenRGB.pro)
- https://github.com/TheMorpheus407/g915x-heatmap (PROTOCOL.md, g915x-heatmap.py,
  udev/99-g915x.rules)
- https://github.com/joshman1019/g915x-rgb (src/g915x_rgb/backend.py)
- https://github.com/Electronic-Supersonic/logitech-g915x-driver (src/main.rs,
  src/keyboard/keyboard.rs)
- https://github.com/csutcliff/gkeys-rs (src/device.rs, src/events.rs, src/led.rs)
- https://github.com/lavers/g815-driver (protocol-notes.txt, src/device/g815.rs)
- https://github.com/developersu/LogiLedus (src/main/java/logiledus/USB/*.java,
  Controllers/Helpers/LoEffects.java)
- https://github.com/italktocomputers/carbon (effects.c, main.c)
- https://github.com/grumpybollocks/g910-control (README.md)
- https://github.com/federico-duppa/logitech-g515-onboard (g515.py, PROTOCOL.md)
- https://github.com/x00FB/g515-tweaks (90-logitech-g515.hwdb, README.md)
- https://github.com/Nabsflix/g515-rgb-controller (bridge.py)
- https://github.com/xpire/logilight (logilight/core.py)
- https://github.com/df8819/G915ColorC (change_color.py, README.md)
- https://github.com/hakangit/g512-keyboard-controller (README.md)
- https://github.com/Tom1243/Logitech-Pro-g815-m1---mr-off (g815_mkeys_off.py)
- https://github.com/AprilNEA/OpenLogi (crates/openlogi-core/src/device.rs, G915 X
  model IDs)

Referenced but not opened: OpenRGB issue #4461 (G915 X captures),
https://gitlab.com/CalcProgrammer1/OpenRGB/-/issues/4461.
Skipped as a likely lure: https://github.com/Logitech-G-HUB-2026/.github.
