# G HUB / Options+ alternatives and other HID++ lighting code

A scan of GitHub projects that replace G HUB or Logitech Options+, or that
contain HID++ lighting code (found by code search for 0x8070, 0x8071, 0x8081,
`SetSWControl`, `manageSwControl`, `setRgbClusterEffect`, `ProfileManagement
8101` and `PER_KEY_LIGHTING_V2`). Scanned 2026-10-14.

Method: every repo was shallow cloned and read as text only (grep, sed, diff).
Nothing was built, installed or run. Snippets are quoted with their file and a
`blob/HEAD` URL. Claims marked "hardware verified" are what the project itself
says; we did not repeat those tests.

Markers used below: **NEW** (not in our notes or code), **CONFIRMS** (agrees
with our notes or hardware log), **CONTRADICTS** (disagrees).

## Summary

| Project | Stars | Language | License | Updated | Relevance | Key takeaway |
|---|---|---|---|---|---|---|
| gabriellaines/rigdeck | 0 | Python | GPL-3.0 | 2026-10-09 | High | usbmon capture of G HUB on a 0x8101 keyboard (PRO X TKL RAPID). Explains the 0x8101 directory entry and fn9 field layout, fn1 slot listing, and the G HUB per-key claim sequence |
| Xteom/logitech_linux_controllers | 0 | Python | none | 2026-10-14 | High | G815 per-key daemon with dated hardware logs: 1 ms write spacing, BUSY only on frameEnd, release needs a profile re-select, per-key colors stay with only 0x8100 host mode |
| EmilB04/LumaBridge | 0 | C++ | none | 2026-10-09 | High | G HUB capture on the G502 X PLUS: G HUB software ID 0x0B, wave and per-key payloads, persistence byte 0x00 is a no-op |
| Slyvan25/OpenGHub | 22 | Rust | MIT | 2026-09-28 | High | Tested on G502 X PLUS and G915: events byte 0x05 for mice and 0x07 for G915 over LIGHTSPEED, persistence 0 not shown, G915 stuck in power mode 2 |
| loxsmoke/openlogi-net | 53 | C# | MIT | 2026-10-09 | Medium-high | G915: 0x8071 mode 1 shows the onboard profile over the host effect (same as our G515 result); 0x8081 works with only 0x8100 host mode; G915 0x8100 lighting descriptors |
| AprilNEA/OpenLogi | 23564 | Rust | Apache-2.0 | 2026-10-14 | Medium | Complete and correctly named 0x8071 fn0-fn9 wrapper, but lighting only "verified against scripted devices". Uses mode 1 with no host mode switch, so likely invisible on a G515. HID++ on usage page 0xFF43 usage 0x0602 for wired G keyboards |
| Nicodemo-Campos/OpenHub | 0 | C++ | PolyForm NC (NOASSERTION) | 2026-10-01 | Medium | G915 X (C356): device index 0x01 wired, 0x8081 v0, 126 address map, a fixed color via 0x8071 went dark, 0x8081 SetRange works |
| TimP4w/HaloDaemon-plugins | 0 | Lua | GPL-3.0-or-later (REUSE.toml) | 2026-09-13 | Medium | Ships mode 1 `[01 01]` on G502 X PLUS and PRO X TKL; G HUB wave and ripple bytes; uses software ID 1 |
| DavidLBruno/contrix | 0 | Rust | none | 2026-08-19 | Medium | PRO X TKL LIGHTSPEED: 0x8100 host mode only, no fn5; 0x8040 set needs 16-bit value; zone 111 is Fn |
| ugogo/light-controls | 0 | C# | MIT | 2026-05-30 | Low-medium | Mouse power LED via 0x8071 fn5 `[01 03 05]` and 0x8090; fn7 written one byte off |
| OpenMouse-Project/mouse-protocol | 21 | TypeScript | AGPL-3.0 | 2026-10-13 | Low-medium | WebHID; Solaar style claim `[01 03 04]`; G502 X per-LED prep matches ours |
| fredaime/plasmaar | 0 | Python | GPL-2.0-or-later | 2026-10-03 | Low | Solaar master fork for KDE; RGB code identical to Solaar |
| MrSchnirschuh/Ghub4Linux | 2 | Python | GPL-3.0-or-later | 2026-10-01 | Low | 0x8070 fn7/fn8 software control, 0x1300 LEDControl details on G502 LIGHTSPEED |
| cj-vana/macos-logitech-rgb | 1 | Swift | MIT | 2026-06-09 | Low | G Pro keyboard (c339) g810-led style, macOS SET_REPORT to the 0xFF43 interface |
| xatuke/logitech-superstrike-pro-x2-controller | 0 | C / Python | GPL-2.0 | 2026-10-07 | Low | PRO X2 SUPERSTRIKE has no RGB feature; avoid software ID 0 |
| saurasanchezpablo/oma-logitech | 0 | QML / Python | MIT | 2026-10-09 | Low | Headset tool, leaves lighting to OpenRGB; software ID 0x0B |
| Abrek34/AbrekMouse | 0 | C++ | none | 2026-10-07 | None | AI agent built acceleration daemon; 0x8040 set sent with the wrong function |
| JakDoh/solaar_win | 1 | C++ | GPL-2.0 | 2026-01-02 | None | Solaar CLI port; only feature constants; commits prebuilt `.exe`/`.dll` (not inspected) |
| vbob/OpenLogi | 0 | Rust | Apache-2.0 | 2026-09-22 | None | Copy of OpenLogi 0.8.5 |
| kurtlieber/omalogi | 0 | Rust | Apache-2.0 | 2026-10-04 | None | Hard fork of OpenLogi 0.8.9, same lighting code |
| xzy-nine/logi-battery | 0 | Rust | Apache-2.0 | 2026-09-10 | None | Battery library cut from OpenLogi, no 0x8071 path |
| PixlOne/logiops | 4386 | C++ | GPL-3.0 | 2025-12-29 | None | No lighting; misnames 0x8070/0x8071 as RGB_EFFECTS / RGB_EFFECTS_V2 |
| JacKeTUs/hid-logitech-hidpp | 13 | C | none (GPL kernel code) | 2025-05-10 | None | Kernel fork for racing wheel force feedback, no lighting |

No repo looked like a malware lure. JakDoh/solaar_win commits binaries
(`build/solaar.exe`, `hidapi.dll`, `hidapi.lib`); those were not opened.

## Answers to the key questions

**Does anyone get firmware-driven dimming while the host sets colors?** No.
Every project that shows host colors either uses mode 3 (`[01 03 xx]`) and
dims on the host (Solaar and plasmaar, LumaBridge) or never dims (OpenGHub,
Xteom, OpenMouse, rigdeck, light-controls). Three projects use mode 1
(`[01 01 ..]`): AprilNEA/OpenLogi (untested on hardware), loxsmoke/openlogi-net
(reports that on a G915 the onboard profile keeps showing, the same as our
G515 result) and HaloDaemon (does not say whether colors show). Two leads
remain untested by anyone:

- 0x8081 per-key frames with only host mode and no 0x8071 claim
  (openlogi-net on G915, Xteom on G815, contrix on PRO X TKL). If the
  firmware still owns power modes in that state, it might dim by itself.
- setRgbClusterEffect byte 12 bits 2-3 (powerMode = 1, "power save") to stage
  a dimmed RAM effect. OpenLogi models the field but nobody uses it.

**fn5 byte semantics.** Control byte: everyone sends 0x03 (or 0x01 in the
mode 1 projects), release is `[01 00 00]` everywhere. Events byte values seen:
0x04 (Solaar, Xteom, OpenMouse, OpenHub first try, us), 0x05 (OpenGHub mice,
light-controls, G HUB step 3), 0x07 (OpenGHub G915, OpenHub G915 X, G HUB step
1), 0x02 (Solaar idle), 0x03 (light-controls "idle"). Only OpenLogi names the
bits as the spec does (control 0x01 clusters, 0x02 power modes; events 0x01
sync, 0x02 activity, 0x04 no-activity). OpenMouse copies Solaar's misnaming.

**Non-static effects in the G515 0x8101 profile?** Nobody writes 0x8101
profiles. rigdeck notes that G HUB's onboard save stores effects only, not
per-key colors, and that it reset all three profiles to the factory copies.
Our record 1 effect storage is still the only implementation.

**Software IDs.**

| Software ID | Users |
|---|---|
| 0x00 | ugogo/light-controls (avoid: superstrike saw the kernel treat a swid 0 reply as a fake event) |
| 0x01 | openlogi-net, HaloDaemon (both collide with the kernel driver's 0x01) |
| 0x02 / 0x03 | logiops (0x03 for no-ack requests) |
| 0x05 | contrix, OpenMouse |
| 0x0A | OpenGHub, OpenHub, LumaBridge, OpenLogi raw 0x8080 frames, us |
| 0x0B | G HUB per LumaBridge capture, oma-logitech |
| 0x0D | G HUB per superstrike docs, cj-vana lgkb |
| rotating | OpenLogi (one ID 1..15 leased per hidraw node with a file lock), Ghub4Linux 1..15, Xteom 1..15 with quarantine, AbrekMouse 1..15, superstrike 2..15 |

Our fixed 0x0A collides with OpenGHub, OpenHub and LumaBridge if they run at
the same time, but not with G HUB or the kernel.

## Projects

### gabriellaines/rigdeck

Python and Qt6 control app for the Logitech PRO X TKL RAPID (046d:c35b, Hall
effect keyboard). GPL-3.0, last commit 2026-10-09. Built from usbmon
captures of G HUB in a Windows VM, then checked on Linux. The device has
0x8071 v4, 0x8081, 0x8040 and 0x8101 but no 0x8100, which is the G515 family.
The doc is careful about what was verified.

G HUB per-key claim (`docs/protocols/logitech-pro-x-tkl-rapid.md`,
https://github.com/gabriellaines/rigdeck/blob/HEAD/docs/protocols/logitech-pro-x-tkl-rapid.md):

```
1. Take the LEDs: 0x8071 fn 5 01 03 07, fn 8 01 01 00, fn 5 01 03 05, then fn 1
   ff 01 00x10 01 (per-key canvas). fn 5 00 00 00 reads the state.
3. Give them back: 0x8071 fn 5 01 00 00 (the profile's own effect returns).
```

There is no 0x8101 fn6 `[05]` in this capture. G HUB static color:
`00 01 f8 2f 25 02 00 00 00 00 00 00 01` (same `02` and trailing `01` as ours).

0x8101 table from the same doc:

```
| 1 list file slots | [0, offset, 0], offsets 0x00/0x10/0x20 | ... 3-byte slots e0 1b 08, e1 1b 08 ... ff ends |
| 9 commit   | [feature hi, lo][file][02][len 3 B][crc32 4 B]            | stores the written buffer as that feature's file |
| 9 activate | [feature hi, lo][file][00][dir entry][len 2 B][crc32 4 B] | points the file back at an existing stored copy |
```

Directory (bank 0 sector 0): "4-byte header, entry count, then 10-byte entries
`[entry id][feature 2 B][flags|len hi][len lo][crc32 4 B][sector]`; `ff` ends."
Profiles are 0x6d (109) bytes at sectors 8/9/0a with a UTF-16 name. Bank 1
holds factory copies. Flash write is fn7 `[bank 0][sector][len 2 B][crc32 4 B]`.
fn6 `0f` returns `03 00 <active profile 1-3>`. getInfo is
`01 02 00 80 05 80 7f 7f 07 14 ff ff ff 20 03 07`.

Per-key: 0x8081 fn5 takes up to 3 ranges `[first, last, R, G, B]`, fn1 up to 4
`[zone, R, G, B]`, fn7 (16 zero bytes) shows the frame. Zones are usage-3,
modifiers 0x68-0x6f, media keys Light 0x96, Play 0x98, Mute 0x99, Next 0x9a,
Prev 0x9b. Fn has no LED. A profile switch (Fn+F1..F3) wipes live lighting;
their service polls fn6 and re-applies.

Power: none. Looks human driven with tool help; dated hardware checks.

### Xteom/logitech_linux_controllers

Python. Started as a G560 speaker CLI and grew a G815 (046d:c33f) per-key
"ripple wave" daemon. No license, last commit 2026-10-14. Has CLAUDE.md,
AGENTS.md and `docs/superpowers/`, so AI assisted, but `experiments/README.md`
has dated hardware gates and counts.

Transport: hidraw, interface 1, usage page 0xFF43 usage 0x0602, report 0x11,
device 0xFF. Software ID rotates 1..15; an ID whose request got no reply is
quarantined, and a root ping (`fn1 [00 00 ping]`) is used as a fence to drain
late replies. Feature indexes on this G815: 0x8081 = 0x10, 0x8071 = 0x0f,
0x8100 = 0x11.

Claim and release (`g560ctl/src/logitech_lighting/devices/g815/keyboard.py`,
https://github.com/Xteom/logitech_linux_controllers/blob/HEAD/g560ctl/src/logitech_lighting/devices/g815/keyboard.py):

```
claim    0x8071 fn5 [01 03 04], 0x8100 fn1 [02], 0x8081 fn0 [00 page 00] for pages 0-2
release  0x8071 fn5 [01 00 00], 0x8100 fn1 [01], 0x8100 fn4 -> profile, 0x8100 fn3 [profile]
```

Hardware findings: release with fn5 `[01 00 00]` plus 0x8100 `[01]` returns
ok but the keys keep the last host frame; only re-selecting the current
profile restores the onboard effect. "Held base #140028 for 60 s with
SetSWControl; without SetSWControl (host mode only) colors also stay."
Back-to-back writes are silently dropped; 0.5 ms spacing still drops, 1 ms
works; about 333 acknowledged reports per second; BUSY (0x08) only on
frameEnd (1,342 frames, 0 timeouts). fn1 and fn5 echo only parameter byte 0,
fn6 echoes bytes 0-3. One invalid zone fails the whole fn6 report. Zone 0x00
is in the bitmap, accepts writes, lights nothing.

Zone map matches ours: usage-3, modifiers 0x68-0x6f, backlight key 0x99, media
0x9b-0x9e, G1-G5 0xb4-0xb8, logo 0xd2. Power: subscribes to idle events (0x04)
but ignores them; every 10 s reads 0x8100 fn2 to detect a takeover and
repaints. No dimming tests.

### EmilB04/LumaBridge

C++20 Windows game-lighting bridge. No license, last commit 2026-10-09. Has a
CLAUDE.md that tells the agent to hide AI attribution. Its claims are backed
by a USB capture of G HUB on a G502 X PLUS (receiver C547, device index 1,
0x8071 at index 09) and a probe tool.

`src/app/peripherals/logitech_hidpp.h`
(https://github.com/EmilB04/LumaBridge/blob/HEAD/src/app/peripherals/logitech_hidpp.h):

```cpp
kSwId = 0x0A;  // LumaBridge's software ID (G HUB uses 0x0B)
```

Effect payloads from `docs/peripherals.md`
(https://github.com/EmilB04/LumaBridge/blob/HEAD/docs/peripherals.md), all fn1:

| Effect | Payload |
|---|---|
| Fixed | `00 idx R G B 02 00.. 01` |
| Breathing | `00 idx R G B <period BE> 00 <intensity> 00 00 00 01` |
| Cycle | `00 idx 00x5 <period BE> <intensity> 00 00 01` |
| Color wave | `FF 00 00x6 <period lo> 01 <intensity> <period hi> 01` (5000 ms gives `88 .. 13`) |
| Per-key canvas | `FF <idx> 00x10 01`, effect ID 0x0013 at cluster FF index 4 |

"Sent with the last byte 00 instead of 01, the commands are answered but the
mouse doesn't change." getInfo `[FF e 00]` does return effect ID 0x0013, so
the per-key effect is advertised. 0x8081: fn1 with up to 4 `<zone R G B>`,
fn7 `00 00 00 00`. Strip order is 3 4 8 7 6 5 2 1; zone 0 lights nothing.
Neither 0x8071 nor 0x8081 can read back what is shown.

Power: host only. After 1 minute without input it fades over 2.5 s and stops
sending so the mouse can sleep. It reads G HUB's
`/lighting/turn_off_for_inactivity` over G HUB's local WebSocket
(127.0.0.1:9010). It sends no fn5 because G HUB holds the claim.

### Slyvan25/OpenGHub

Linux G HUB reimplementation in Rust with Tauri and Svelte (DPI, buttons,
macros, lighting, 0x8100 profiles, wheels, Lua, firmware). MIT, 22 stars, last
commit 2026-09-28 with an AI co-author trailer. Lighting notes cite tests on a
G502 LIGHTSPEED, G502 X PLUS, G915 and G923.

Transport (`src-tauri/src/hidpp/mod.rs`): hidapi, usage page 0xFF00, falls
back to interface 1 or 2 when no usage page is reported; reports 0x10/0x11;
software ID 0x0a; wired device 0xff.

0x8071 (`src-tauri/src/hidpp/features.rs`,
https://github.com/Slyvan25/OpenGHub/blob/HEAD/src-tauri/src/hidpp/features.rs,
and `docs/lighting.md`):

```rust
SW_CONTROL_SET_MOUSE    = [0x01, 0x03, 0x05]
SW_CONTROL_SET_KEYBOARD = [0x01, 0x03, 0x07]
```

Keyboards are recognised by having 0x8040. Their note: over LIGHTSPEED "a
G502 X keeps its own effect with 0x07 and obeys with 0x05, while a G915 only
obeys with 0x07. Wired, both accept either." Byte 12 is always 1: "a trailing
0 is acknowledged and not shown (verified on a G502 X PLUS and a G915)". Fixed
colors set `params[5] = 0x02`. After the claim it reads fn8 and sends
`[01 01]` if the power mode is not 1, because "a wireless G915 sat in
rgbPowerMode 2 and lit only its logo". If 0x8040 reads 0 it sets max. Before
any write it sets 0x8100 host mode (`02`), because "a G915 in onboard mode
acknowledges every write and shows none".

Effect params: breathing color, period BE at 3-4, intensity at 6; cycle
period at 5-6, intensity at 7; wave 0x04 (OpenRGB G915 layout) `p[6]` period
low, `p[7]` direction, `p[8]` intensity, `p[9]` period high; ripple 0x0b color
then `p[5]` rate 2-200 ms. 0x8081: fn1 up to 4 `[led r g b]`, fn6
`[r g b led x13]`, fn7 commit; G915 LED IDs usage-3, modifiers usage-0x78,
G-keys 0xb3+n, logo 0xd2. 0x8100: zone N at 208+N*11, copied to 230+N*11,
which overwrites the power save records so onboard dimming is lost. No idle
events, no fn7 timers, no 0x8101, no G515.

### loxsmoke/openlogi-net

C# Windows port of OpenLogi with its own HID++ code. MIT, 53 stars, last
commit 2026-10-09. Has hardware-verified markers for the G915.

`src/OpenLogi.Hid/DeviceSession.Lighting.cs`
(https://github.com/loxsmoke/openlogi-net/blob/HEAD/src/OpenLogi.Hid/DeviceSession.Lighting.cs):

```csharp
/// PerKeyLighting v2 (0x8081, newer G-series like the G915 — HARDWARE-VERIFIED);
// 0x8081 (PerKeyLighting v2) — works on the G915 via host mode; prefer it
// over 0x8071 (which the onboard profile replays over).
```

The 0x8071 path claims `SwControlAllClusters` only (mode 1, `[01 01 00]`),
assumes effect index 1 is static, writes clusters 0..3 blindly and never
releases. The per-key path calls `EnsureHostModeAsync` (0x8100 fn1 `[02]`,
`DeviceSession.Profiles.cs`) then 0x8081 SetRange `00..fe` and FrameEnd
volatile, with no fn5 of its own. (Caveat: if the 0x8071 path ran earlier in
the session the mode 1 claim is still held.) G915 0x8100 format 4: 255 byte
sector, CRC16-CCITT over 0..252 at 253-254, three 11-byte lighting descriptors
at 0xD0, 0xDB, 0xE6; fixed `[01 R G B]`, breathing
`[0a R G B periodHi periodLo brightness]` (brightness at byte 6, where our
0x8100 notes put waveform; either G915 differs or this is a bug), cycle
`[03 ... 08 34 brightness]` (2100 ms). G915 LIGHTSPEED dongle interface takes
only 7-byte reports; the keyboard parks its RF link 1-2 s after the last key.
Software ID fixed 0x01.

### AprilNEA/OpenLogi (and copies vbob/OpenLogi, kurtlieber/omalogi, xzy-nine/logi-battery)

Rust, local-first Options+ replacement (buttons, DPI, SmartShift, lighting).
Apache-2.0 (dual with MIT in Cargo metadata), 23.5k stars, updated daily. Has
CLAUDE.md and AGENTS.md and very long comments; `docs/DECISIONS.md` says
lighting was "Verified against scripted devices only".

`crates/openlogi-hidpp/src/feature/rgb_effects.rs`
(https://github.com/AprilNEA/OpenLogi/blob/HEAD/crates/openlogi-hidpp/src/feature/rgb_effects.rs)
wraps fn0..fn9 with spec names (`RgbSwControl`, `SwControlFlags`,
`RgbPowerModeConfig`, `RgbPersistence`, `RgbPowerMode`) and reads device info
capability offsets correctly (libratbag does not). Byte 12 is persistence in
bits 0-1 plus power mode in bits 2-3.

`crates/openlogi-device/src/write/lighting/rgb.rs`
(https://github.com/AprilNEA/OpenLogi/blob/HEAD/crates/openlogi-device/src/write/lighting/rgb.rs):

```rust
.set_sw_control(
    previous.control | SwControlFlags::ALL_CLUSTERS,
...
    lease.control.difference(SwControlFlags::ALL_CLUSTERS),
```

So it adds only the cluster bit to whatever was there (mode 1 from a released
device), writes RAM only, and on exit clears only that bit. No 0x8100/0x8101
host mode switch, no fn7/fn8, events ignored. On a G515 this is the mode 1
case we found invisible. Per-key: claim, set static as base, paint 13 zones
per fn6, frameEnd, replay the first frame once (as Solaar). No zone map.
0x8070 writes are volatile with no software control.

Transport (`crates/openlogi-hid/src/transport.rs`,
https://github.com/AprilNEA/OpenLogi/blob/HEAD/crates/openlogi-hid/src/transport.rs):

```rust
/// - `0xFF43 / 0x0602` — wired G-series gaming keyboards (e.g. the G513): a
///   distinct vendor collection on the same `0xFF43` page. Carries both report
    (0xff43, 0x0202, true),
    (0xff43, 0x0602, false),
```

Software ID: one ID from 1..15 leased per hidraw node with a file lock; raw
0x8080 frames use 0x0a.

vbob/OpenLogi (0.8.5) and kurtlieber/omalogi (0.8.9) are copies of older
OpenLogi, not GitHub forks; the lighting code is identical and older.
xzy-nine/logi-battery keeps an old 0x8070/0x8081 file but has no 0x8071 path.

### Nicodemo-Campos/OpenHub

Qt/C++ Linux control center: read-only probing, 0x8100 writes for the G502
LIGHTSPEED, an LED address explorer for the G915 X. PolyForm Noncommercial
1.0.0 plus commercial license, last commit 2026-10-01. Very likely AI written
(milestone docs, a 5285 line `src/hidpp/HidppProbe.cpp`), but reports real
hardware results including failures.

Raw hidraw, software ID 0x0A, tries device index 0x01 first for "g915" names.
Per `docs/CONTINUITY.md` the G915 X LIGHTSPEED (046d:C356) answers wired at
device index 0x01, HID++ 4.2, with 0x8071 v4, 0x8081 at version 0, 0x8040 and
0x8101.

First attempt on G915 X: 0x8101 fn6 `[05]`, fn5 `[01 03 04]`, fixed color
with ramp `02` and byte 12 = `00`: acknowledged, keyboard went fully dark.
Replacement sequence (`src/hidpp/HidppProbe.cpp` around line 4516,
https://github.com/Nicodemo-Campos/OpenHub/blob/HEAD/src/hidpp/HidppProbe.cpp):

```
fn3 [00 00 20]
fn5 [00 00 00]
fn5 [01 03 07]
fn7 [01 00 00 00 3c 01 2c 00]   (60 s / 300 s)
fn7 [01 00 00 00 00 00 5a 00]   (power save 0 / off 90 s)
fn5 [01 03 05]
fn3 [00 00 01]
```

then 0x8081 fn5 SetRange per run and fn7 commit, which displayed. Address
bitmap via fn0 `[00 00]`, `[00 01]`, `[00 02]`: 126 addresses, 0x01-0x6F,
0x99, 0x9B-0x9E, 0xB4-0xBC, 0xD2. No dimming. G502 LIGHTSPEED 0x8100: writing
the record at 208 changed something visible, 219 did not; they label 208
"Primary".

### TimP4w/HaloDaemon-plugins

Lua plugins for a lighting daemon. GPL-3.0-or-later per REUSE.toml, last
commit 2026-09-13, has AGENTS.md, credits Solaar. Devices G502 X PLUS
(c095), G502 HERO (c08b), G PRO X TKL (c352), usage page 0xFF00 usage 1.
Software ID fixed 0x01.

`logitech/lib/hidpp2.lua` line 646
(https://github.com/TimP4w/HaloDaemon-plugins/blob/HEAD/logitech/lib/hidpp2.lua):

```lua
feature(dev, RGB_EFFECTS, 0x50, bytes(1, 1))
```

That is fn5 `[01 01]`, mode 1, no events. 0x8100 host mode is a separate user
toggle; 0x8070 devices get fn8 `[01]`. Static "from G HUB":
`zone slot R G B 64 0B B8 64 00 00 00 01 00 00` (differs from the `02` form in
rigdeck and LumaBridge). Wave `ff 00 0 0 0 0 0 0 88 01 64 13 01` (matches
LumaBridge). Ripple `ff 03 5e 5e 5e ff 0 0 0 14 0 0 01`. 0x8081 frames are
diffed: fn5 `[00 FF r g b]`, fn2 for runs, fn7 `[00]`; the G502 strip uses fn1
because "optimized pixmap frames can leave one strip LED at its previous
colour". No power handling.

### DavidLBruno/contrix

Rust, Spanish, for the PRO X TKL LIGHTSPEED. No license, last commit
2026-08-19, has CLAUDE.md; findings dated 2026-08-11, guesses marked
INFERIDO. Software ID 5. One cluster with effect IDs 0x0000, 0x0001, 0x000A,
0x0015, 0x0016, 0x0017. Only 0x8100 fn1 `[02]`, no fn5; without host mode "the
keyboard runs its stored effects and ignores all writes" (acked without
error). 0x8040 fn2 `[00 64]`: a one-byte value fails. 0x8081 fn1 echoes zone
IDs, colors show only after fn7. Zone 111 lights Fn on this TKL (our table
says R GUI).

### ugogo/light-controls

Windows .NET tray app: OpenRGB plus direct HID++ for the power LED of the PRO
2 and Superlight 2. MIT, last commit 2026-05-30. Software ID 0, device index
0x01, receivers C54D, C547, C543, C53F.
`src/LightControls.Core/Logitech/Hidpp20/Hidpp20Session.cs`
(https://github.com/ugogo/light-controls/blob/HEAD/src/LightControls.Core/Logitech/Hidpp20/Hidpp20Session.cs):
claim fn5 `[01 03 05]`, "idle" `[01 03 03]`, release `[01 00 00]`; effect
`[00 idx R G B ...]` persistence 1, no ramp byte. fn7 is sent as
`[01 00 00 00 FF FF FF FF]`, which by the spec sets power save 255 s and off
0xFFFF (likely meant both infinite, one byte off). Re-applies the color every
5 s and treats any 0x8071 event as lost lighting. Also 0x8090 ModeStatus fn3
`[00 zone R G B]` for the power LED.

### OpenMouse-Project/mouse-protocol

TypeScript WebHID library. AGPL-3.0, 21 stars, last commit 2026-10-13.
Software ID 0x05. `src/drivers/logitech/hidpp.ts`
(https://github.com/OpenMouse-Project/mouse-protocol/blob/HEAD/src/drivers/logitech/hidpp.ts):
0x8100 host mode if present, then fn5 `[01 03 04]` with Solaar's flag names;
G502 X per-LED prep fn1 `[FF, <cluster 0 effect count>, 0 x10, 01]`; release
fn5 `[01 00 00]` and back to onboard if it was before. 0x8070 static ramp 0,
0x8071 fixed ramp 2. Pro X Superlight advertises 0x8070 but rejects getInfo
with 0x05. No idle handling.

### fredaime/plasmaar

Python, KDE Plasma port of Solaar (NOTICE: Solaar master e7304c4c,
2026-08-17). GPL-2.0-or-later. `rgb_power.py`, `rgb_effects_probe.py` and
`device_quirks.py` are unmodified Solaar. Claim in
`lib/logitech_receiver/settings_templates.py` `_claim_sw_control`
(https://github.com/fredaime/plasmaar/blob/HEAD/lib/logitech_receiver/settings_templates.py):
0x8101 fn6 `[05]` (or 0x8100 `[02]`), then fn5 `[01 03 04]` (the comment
still says "mode=3, flags=5"). Host dimming: 25 step GLib ramp over about 5 s
with fn1 static writes and 0x8081 fn6 for per-key; sleep fn8 `[01 03 00]`;
wake power mode 1, fn5 `[01 03 04]`, repaint; idle fn5 `[01 03 02]`; release
fn5 `[01 00 00]`, 0x8101 fn6 `[03]`, fn9 `[01 00]`. `rgb_effects_probe.py` is
about headset features 0x0621/0x0622, not 0x8071. Nothing new versus our
Solaar notes.

### MrSchnirschuh/Ghub4Linux

GTK4 Python app for G502 LIGHTSPEED, POWERPLAY and PRO X 2 DEX.
GPL-3.0-or-later in pyproject.toml (no LICENSE file), last commit
2026-10-01, likely written by an AI agent, with a large mocked test suite.
hidraw on usage page 0xFF00, software ID rotates 1..15. `core/rgb.py`
(0x8070, cites x8070 v7): fn7 reads software control, fn8 sets `[01 events]`
before every write, fn9 reads effect settings, fnE reads the zone effect;
fixed color byte 4 ramp (0 default, 1 ramp, 2 instant). `core/led.py`
(0x1300 LEDControl on G502 LIGHTSPEED 407f): three LEDs (battery, DPI,
profile), fn5 SetState `[led, mode BE16, displayIndex BE16, 4 bytes]`,
rejected with 0x02 until fn3 claims software control; mode reads back little
endian but must be written big endian. No 0x8071/0x8081/0x8101.

### cj-vana/macos-logitech-rgb

Swift CLI and menu bar app for the G Pro keyboard (046d:c339) on macOS. MIT,
last commit 2026-06-09. g810-led style with hardcoded indexes (0x0C = 0x8080,
0x0D = 0x8070) and software ID 0xD: frames `12 FF 0C 3D 00 ZZ 00 NN [key R G B]x14`,
commit `11 FF 0C 5D`, effects `11 FF 0D 3D zone mode ...`. Sends HID
SET_REPORT to interface 1 (usage page 0xFF43) via IOUSBHost, avoiding the
Input Monitoring permission. Quirks: the logo only latches through the 20
byte frame; Fn key LED at 0xE7; no brightness command. Sources
`docs/PROTOCOL.md`, `Sources/LgkbCore/GProtocol.swift`.

### xatuke/logitech-superstrike-pro-x2-controller

C11 and Python, GPL-2.0, last commit 2026-10-07. The PRO X2 SUPERSTRIKE
(40bd/c0a8) has no 0x807x RGB feature. Its 0x8100 profile still has lighting
slots at 0xd0..0xfb (4 x 11 bytes, placeholders), flag at 0xfc, and power
save/off timeouts `003c`/`012c` at 0x2c/0x2e. `src/ss2k.c` rotates software ID
2..15, avoiding 0 and 1, because a reply with software ID 0 was parsed by the
kernel as a fake battery event. Docs say G HUB uses 0xD. Docs look AI assisted
with real dumps.

### saurasanchezpablo/oma-logitech

QML and Python headset panel for Omarchy (G733 tested). MIT, last commit
2026-10-09. Leaves lighting to OpenRGB; notes the G733 has 0x8070 with 2
zones. Software ID 0x0B; matches replies on feature, function and software ID
so it can share the device.

### Abrek34/AbrekMouse

C++ Raw Accel port with GTK4 GUI. No license, last commit 2026-10-07. Built
by several AI agents (AGENTS.md, Turkish agent logs in `.aihaberlesme/`);
README says no CI run completed. No 0x8071 write path; quirk table copied from
Solaar. `set_led_brightness` sends 0x8040 function 0x10 with one byte, but fn1
is getBrightness and setBrightness is fn2 with a 16-bit value, so it likely
does nothing. Software ID rotates 1..15.

### JakDoh/solaar_win

C++ CLI port of Solaar for Windows. GPL-2.0, last commit 2026-01-02. Lighting
is only the 0x8070/0x8071/0x8081 constants plus backlight 0x1982. The repo
contains prebuilt `build/solaar.exe`, `hidapi.dll` and `hidapi.lib`; those were
skipped.

### PixlOne/logiops

C++ userspace driver for HID++ mice. GPL-3.0. No lighting code, only enum
constants that name 0x8070 `RGB_EFFECTS` and 0x8071 `RGB_EFFECTS_V2`.
Software ID 2, and 3 for requests that do not wait for a reply.

### JacKeTUs/hid-logitech-hidpp

DKMS fork of the kernel driver (6.5 base) for G PRO / G923 wheels and force
feedback. No lighting additions; the kernel still uses software ID 0x01.

## Findings for logirgb

New:

- **0x8101 directory entry layout (rigdeck).** Our 16-byte G515 entry fits
  rigdeck's directory format exactly: CRC32 (bytes 0-3), entry count (byte 4 =
  `01`), entry id (byte 5 = `01`), feature `81 01` (6-7), flags plus length
  high (byte 8 = `40`), length low (byte 9 = `63`), file CRC32 (10-13), sector
  (byte 14 = `01` factory, `08` user), `ff` end (15). This resolves the
  "unknown, constant" bytes in `8101-profile-management.md`. A G515 entry
  count above 1 would mean more files to keep when we rewrite the directory.
- **0x8101 fn9 has two forms.** Activate is
  `[feature hi lo][file][00][dir entry][len 2][crc32]`, which matches our
  `81 01 01 00 01 00 63 crc` (feature 0x8101, file 1, op 00, entry 1, length
  99). Commit is `[feature hi lo][file][02][len 3][crc32]` and makes a staged
  buffer live without touching flash. That could let us apply a profile live
  without the fn7 flash write.
- **0x8101 fn1 lists file slots**: request `[0, offset, 0]` for offsets 0x00,
  0x10, 0x20, reply 3-byte `[file][feature hi lo]` entries ending with `ff`.
  Our notes list fn1 as unknown.
- **fn7 address is bank then sector** (bank 0 writable, bank 1 factory), which
  matches our `0x0008` / `0x0101` addressing. G HUB's flash write order is an
  empty directory first, then the profile sectors, then the new directory.
- **The 24-byte profile name is probably UTF-16** (rigdeck finds a UTF-16 name
  in the PRO X RAPID 109-byte profile).
- **G HUB's own per-key claim on a 0x8101 keyboard** is fn5 `[01 03 07]`, fn8
  `[01 01 00]` (force full power), fn5 `[01 03 05]`, fn1 `[FF 01 00x10 01]`, with
  no 0x8101 fn6 `[05]`. Release is fn5 `[01 00 00]` alone. Worth trying on the
  G515 to see whether fn6 `[05]` is needed at all.
- **0x8081 without a 0x8071 claim.** openlogi-net (G915), Xteom (G815) and
  contrix (PRO X TKL) show per-key frames with only 0x8100 host mode. On the
  G515 the equivalent test is `8101 fn6 05` with fn5 left at 0, then 0x8081
  frames. If they show and the firmware still owns power modes, that is the
  first candidate for host colors plus firmware dimming.
- **setRgbClusterEffect powerMode bits** (byte 12 bits 2-3) are modelled by
  OpenLogi but used by nobody. Staging a dimmed RAM effect with byte 12 =
  `0x05` in mode 1 is another untested route to firmware dimming.
- **Events byte matters over LIGHTSPEED** (OpenGHub): a wireless G502 X needs
  `0x05`, a wireless G915 needs `0x07`; wired both accept either. Our `0x04`
  is fine wired on the G515 but may fail on wireless G515 LIGHTSPEED.
- **Wireless G915 can be left in rgbPowerMode 2** (logo only) and with 0x8040
  brightness 0 after a claim; OpenGHub reads fn8 and 0x8040 and fixes both.
  Reading fn8 after a claim would be cheap insurance.
- **Wired G keyboards expose HID++ on usage page 0xFF43 usage 0x0602**
  (OpenLogi, Xteom, cj-vana). Our discovery should accept 0xFF43 as well as
  0xFF00.
- **G815 timing** (Xteom): at least 1 ms between writes or they are silently
  dropped; BUSY only on frameEnd; one bad zone fails a whole fn6 report.
- **G815 release needs a profile re-select** (0x8100 fn4 then fn3) for the
  onboard effect to return. Our G515 release path (fn6 `[03]`) might need the
  0x8101 equivalent (fn9 activate) if the last host frame sticks.
- **G502 X PLUS details** (LumaBridge): per-key effect 0x0013 is advertised at
  cluster FF index 4; LED strip order is 3 4 8 7 6 5 2 1; zone 0 lights nothing.
- **Zone map differences** on other boards: PRO X TKL RAPID media keys are
  0x96 (light), 0x98-0x9b (play, mute, next, prev); PRO X TKL zone 111 is Fn.
  Our G815-derived table (brightness 0x99, media 0x9b-0x9e) is not universal.
- **G915 X (C356)**: wired device index 0x01, 0x8081 at feature version 0,
  126 address map, and a 0x8071 fixed color with byte 12 = 0 went dark.
- **G HUB effect bytes**: color wave
  `FF 00 00x6 <period lo> 01 <intensity> <period hi> 01`; ripple
  `ff 03 R G B ff 00 00 00 <rate> 00 00 01`.
- **Software IDs in the wild**: G HUB 0x0B (LumaBridge capture) or 0x0D
  (superstrike docs); OpenGHub, OpenHub and LumaBridge also use 0x0A.

Confirms:

- **Mode 1 shows the onboard profile, not the host effect**, now also seen on
  a G915 by openlogi-net. Nobody has made mode 1 visible.
- **Mode 3 means host owns power**: nobody gets firmware dimming in mode 3;
  Solaar, plasmaar and LumaBridge dim on the host.
- **Persistence byte 0x01 is required for live effects**; byte 12 = `0x00` is
  acknowledged but not shown (OpenGHub on G502 X PLUS and G915, LumaBridge on
  G502 X PLUS, OpenHub on G915 X). This contradicts OpenRGB's live painting
  with 0x00 and matches what `lighting/controller.go` already sends.
- Fixed color layout `R G B 02` (rigdeck G HUB capture, LumaBridge, OpenGHub,
  OpenHub).
- Claim `[01 03 xx]` wrapped in 0x8101 fn6 `05` / `03` (Solaar, plasmaar);
  release `[01 00 00]` everywhere.
- fn7 layout and G515 style defaults `003c 012c` (OpenHub, superstrike).
- Effect index must be looked up per cluster (OpenGHub, OpenLogi).
- Zone rules usage-3, modifiers 0x68-0x6f, G-keys 0xb4+, logo 0xd2 (Xteom,
  OpenGHub, OpenHub, rigdeck).
- Live effects are lost on an onboard profile switch (rigdeck, Xteom).
- 0x8040 setBrightness is fn2 with a 16-bit value (contrix, rigdeck getInfo
  `00 64 05 03 00 00`).
- G502 X PLUS per-key prep `fn1 [FF <n> 00x10 01]` (OpenMouse, plasmaar).
- Our fn5 bit naming follows the spec; OpenLogi agrees, Solaar, OpenRGB and
  OpenMouse use the misnamed flags.

Contradicts or conflicts:

- OpenRGB's comment that the G502 X PLUS does not advertise the per-key
  effect (quoted in `devices.md`): LumaBridge sees 0x0013 in getInfo.
- HaloDaemon's G HUB static preset `R G B 64 0B B8 64 ...` versus the `02` form
  in the rigdeck and LumaBridge captures (maybe another G HUB version or
  device).
- G502 0x8100 LED blocks: OpenHub labels 208 "Primary" and saw no change at
  219; OpenGHub maps zone N to 208+N*11. Our hardware log (208 logo, 219
  primary) is better tested; neither project checked which LED changed.
- openlogi-net puts breathing brightness at byte 6 of a G915 0x8100
  descriptor where our notes put waveform.
- rigdeck treats 0x8101 fn6 `0f` as a read; by our bit table it writes
  onboard mode plus the power bit (it happened to match the current state).
- OpenGHub overwrites the 0x8100 power save records (230+) with the full
  color, which disables onboard dimming. Our onboard writer keeps a dimmed
  copy; keep doing that.

## Sources

- https://github.com/gabriellaines/rigdeck/blob/HEAD/docs/protocols/logitech-pro-x-tkl-rapid.md
- https://github.com/gabriellaines/rigdeck/blob/HEAD/rigdeck/modules/keyboard/lighting.py
- https://github.com/Xteom/logitech_linux_controllers/blob/HEAD/g560ctl/src/logitech_lighting/devices/g815/keyboard.py
- https://github.com/Xteom/logitech_linux_controllers/blob/HEAD/experiments/README.md
- https://github.com/EmilB04/LumaBridge/blob/HEAD/docs/peripherals.md
- https://github.com/EmilB04/LumaBridge/blob/HEAD/src/app/peripherals/logitech_hidpp.h
- https://github.com/Slyvan25/OpenGHub/blob/HEAD/src-tauri/src/hidpp/features.rs
- https://github.com/Slyvan25/OpenGHub/blob/HEAD/src-tauri/src/hidpp/mod.rs
- https://github.com/Slyvan25/OpenGHub/blob/HEAD/docs/lighting.md
- https://github.com/loxsmoke/openlogi-net/blob/HEAD/src/OpenLogi.Hid/DeviceSession.Lighting.cs
- https://github.com/loxsmoke/openlogi-net/blob/HEAD/src/OpenLogi.Hid/DeviceSession.Profiles.cs
- https://github.com/AprilNEA/OpenLogi/blob/HEAD/crates/openlogi-hidpp/src/feature/rgb_effects.rs
- https://github.com/AprilNEA/OpenLogi/blob/HEAD/crates/openlogi-device/src/write/lighting/rgb.rs
- https://github.com/AprilNEA/OpenLogi/blob/HEAD/crates/openlogi-hid/src/transport.rs
- https://github.com/AprilNEA/OpenLogi/blob/HEAD/docs/DECISIONS.md
- https://github.com/vbob/OpenLogi
- https://github.com/kurtlieber/omalogi
- https://github.com/xzy-nine/logi-battery
- https://github.com/Nicodemo-Campos/OpenHub/blob/HEAD/src/hidpp/HidppProbe.cpp
- https://github.com/Nicodemo-Campos/OpenHub/blob/HEAD/docs/HIDPP.md
- https://github.com/Nicodemo-Campos/OpenHub/blob/HEAD/docs/CONTINUITY.md
- https://github.com/TimP4w/HaloDaemon-plugins/blob/HEAD/logitech/lib/hidpp2.lua
- https://github.com/TimP4w/HaloDaemon-plugins/blob/HEAD/logitech/lib/lighting.lua
- https://github.com/DavidLBruno/contrix
- https://github.com/ugogo/light-controls/blob/HEAD/src/LightControls.Core/Logitech/Hidpp20/Hidpp20Session.cs
- https://github.com/OpenMouse-Project/mouse-protocol/blob/HEAD/src/drivers/logitech/hidpp.ts
- https://github.com/fredaime/plasmaar/blob/HEAD/lib/logitech_receiver/settings_templates.py
- https://github.com/fredaime/plasmaar/blob/HEAD/lib/logitech_receiver/rgb_power.py
- https://github.com/MrSchnirschuh/Ghub4Linux/blob/HEAD/src/ghub4linux/core/rgb.py
- https://github.com/MrSchnirschuh/Ghub4Linux/blob/HEAD/src/ghub4linux/core/led.py
- https://github.com/cj-vana/macos-logitech-rgb/blob/HEAD/docs/PROTOCOL.md
- https://github.com/xatuke/logitech-superstrike-pro-x2-controller/blob/HEAD/src/ss2k.c
- https://github.com/saurasanchezpablo/oma-logitech
- https://github.com/Abrek34/AbrekMouse
- https://github.com/JakDoh/solaar_win
- https://github.com/PixlOne/logiops
- https://github.com/JacKeTUs/hid-logitech-hidpp
