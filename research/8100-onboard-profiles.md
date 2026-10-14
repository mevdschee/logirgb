# 0x8100 OnboardProfiles

Research notes on HID++ 2.0 feature `0x8100` (OnboardProfiles), which most
Logitech G mice (G502 HERO, G502 Proteus Spectrum, G303, G403, G305, G903,
G502 X / X PLUS, PRO X 2 family) and some keyboards (G815, G915 TKL) use to
keep settings in flash. Also short notes on `0x8110` MouseButtonSpy and
`0x8060` / `0x8061` report rate, which interact with it.

Logitech has never published the `0x8100` specification. Everything here
comes from open source tools, issue threads and our own hardware tests.

## Legend

Each claim is tagged with where it comes from:

- **[HW]** verified on hardware by this project (G502 SE HERO, `046d:c08b`,
  wired). The raw log is in [hardware-log.md](hardware-log.md).
- **[libratbag]**, **[Solaar]**, **[libhidpp]**, **[OpenRGB]**,
  **[superstrike]**: read from that tool's source code or docs (see Sources).
- **[issue]**: a log or statement in a public issue or PR, from someone
  else's hardware.
- **[unverified]**: our inference, or something only one source claims.

Byte offsets are decimal unless written as hex. Multi-byte protocol fields
are big endian unless noted. DPI values inside the profile are little endian.

## Functions

Function IDs are the high nibble of the HID++ function byte (`fn3` is sent
as `0x3?` with the software ID in the low nibble).

| fn | Name | Request params | Reply | Source |
|---|---|---|---|---|
| 0 | GetInfo (GetDescription, GetProfilesDescr) | none | 16 bytes, see below | [libratbag] [Solaar] [libhidpp] [HW] |
| 1 | SetOnboardMode | `[mode]`: 0 no change, 1 onboard, 2 host | none | [libratbag] [libhidpp] [Solaar] |
| 2 | GetOnboardMode | none | `[mode]` | [libratbag] [libhidpp] [HW: `01`] |
| 3 | SetCurrentProfile | `[sector hi, sector lo]` | none | [libhidpp] [Solaar] [HW] |
| 4 | GetCurrentProfile | none | `[sector hi, sector lo]` | [libhidpp] [HW] |
| 5 | MemoryRead | `[sector hi, sector lo, offset hi, offset lo]` | 16 bytes | all tools, [HW] |
| 6 | MemoryAddrWrite (write start) | `[sector hi, sector lo, offset hi, offset lo, len hi, len lo]` | none | all tools, [HW] |
| 7 | MemoryWrite | 16 data bytes | none | all tools, [HW] |
| 8 | MemoryWriteEnd | none | none (may return an error) | all tools, [HW] |
| 9, 10 (A) | unknown | | | no tool uses them |
| 11 (B) | GetCurrentDpiIndex | none | `[index]` | [libratbag] [libhidpp] |
| 12 (C) | SetCurrentDpiIndex | `[index]` (0 to 4) | none | [libratbag] [libhidpp] |

Events (notifications from the device) **[libhidpp] [Solaar]**:

| Event | Name | Payload |
|---|---|---|
| 0 | CurrentProfileChanged | `[sector hi, sector lo]` |
| 1 | CurrentDpiIndexChanged | `[index]` |

Solaar listens for both (`notifications.py`, address `0x00` and `0x10`) and
updates its UI. libhidpp documents that events are sent in onboard mode and
not in host mode.

**[HW]** On the G502 HERO, pressing the profile switch button behind the
scroll wheel cycled profiles (`fn4` went from `01 03` to `01 01`), but a
listener on the hidraw node saw **no** HID++ notification at all over 60
seconds of button presses. So on this firmware the CurrentProfileChanged
event either does not exist or needs to be enabled somehow. Software that
wants to know the active profile has to poll `fn4`.

### Sector addressing in the requests

The 16-bit "sector" is really `[memory type, page]` **[libhidpp]**:

- high byte `0x00`: writable flash ("user" sectors)
- high byte `0x01`: ROM (factory or "out of box", OOB, profiles)

libhidpp's `memoryRead(mem_type, page, offset)` puts `mem_type` in param 0
and `page` in param 1; libratbag and Solaar treat the pair as one big endian
`uint16`. These are the same bytes on the wire. `MemoryAddrWrite` only ever
targets memory type 0.

### Reading

`fn5` always returns 16 bytes. **[libratbag] [HW]** A read whose 16-byte
window would run past the end of the sector is rejected with
`ERR_INVALID_ARGUMENT`, so for a 255-byte sector the last read must start at
offset `size - 16` (239) and only its tail is new. libratbag, Solaar and
linux-superstrike all do this. A short libratbag snippet (GPL-2.0, from
`hidpp20_onboard_profiles_read_sector`):

```c
/* the firmware replies with an ERR_INVALID_ARGUMENT error
 * if we try to read past sector_size - 16 ... */
offset = (sector_size - offset < 16) ? sector_size - 16 : offset;
```

### Writing

1. `fn6 [sector u16, offset u16 = 0, length u16]`
2. `fn7 [16 bytes]` repeated; the last chunk is padded, extra bytes past
   `length` are ignored **[libhidpp]**
3. `fn8`

**[HW]** This project wrote whole 255-byte sectors this way and verified each
with `fn5` read back. libhidpp notes that `fn8` "may throw errors if the data
was not valid, but the data is written anyway", so always read back.
**[superstrike]** reports occasional transient CRC mismatches on read back
("busy" reads) and retries.

Writes to ROM sectors (`0x01xx`) are not possible. Solaar refuses to write
any sector `>= sector_count`.

## GetInfo (fn0) layout

| Byte | Field | Notes |
|---|---|---|
| 0 | memory model | only `1` is known **[libratbag] [libhidpp]** |
| 1 | profile format | 1 to 7 known, see table below |
| 2 | macro format | only `1` is known |
| 3 | profile count | number of user profile slots |
| 4 | profile count OOB | number of ROM (factory) profiles |
| 5 | button count | buttons per profile (max 16 in the layout) |
| 6 | sector count | number of writable sectors (pages) |
| 7-8 | sector size | big endian, bytes per sector |
| 9 | mechanical layout | bits 0-1 == 2: G-shift supported; bits 2-3 == 2: DPI shift supported |
| 10 | various info | bits 0-2: 1 corded, 2 wireless, 4 corded and wireless |
| 11-15 | reserved | |

Solaar derives the G-shift button count as `buttons if (shift & 0x3 == 0x2) else 0`
and refuses profile formats above 5 (`if memory != 0x01 or profile > 0x05`).
libratbag refuses anything not in its list (1 to 5, plus 6 and 7 in open PRs).

Known replies:

| Device | Reply | Decoded | Source |
|---|---|---|---|
| G502 SE HERO `c08b` (our unit) | `01 02 01 05 05 0b 10 00 ff 0a 01` | model 1, format 2, macro 1, 5 profiles, 5 OOB, 11 buttons, 16 sectors, **sector size 255**, G-shift and DPI shift, corded | **[HW]** |
| G502 HERO `c08b` (2020 log) | `01 02 01 05 05 0b 10 01 00 0a 01` | same, but **sector size 256** | **[issue]** libratbag #1181 |
| G305 | `01 03 01 01 01 06 06 00 ff 0a 02` | format 3, 1 profile, 1 OOB, 6 buttons, 6 sectors, 255 bytes, wireless | **[issue]** libratbag #873 |
| PRO X 2 DEX `c0a0` | `01 07 01 05 01 05 10 00 ff 0a 04` | format 7, 5 profiles, 1 OOB, 5 buttons, 16 sectors, 255 bytes, corded and wireless | **[issue]** libratbag #1870 |

Note that two G502 HERO units report different sector sizes (255 versus
256). Never hard-code the sector size.

### Profile format IDs

| ID | Name in libratbag | Devices (examples) | Notes |
|---|---|---|---|
| 1 | G402 | G402, G502 Proteus Core | libhidpp: profile ends at 208 (actually 224), no RGB blocks |
| 2 | G303 | G303, G502 Proteus Spectrum, G502 HERO | RGB LED blocks at 208 and 219 |
| 3 | G900 | G900, G903, G305, G403 | same layout as 2 for our purposes |
| 4 | G915 | G915 / G815 keyboards | libratbag treats it like format 2 |
| 5 | G502X | G502 X, G502 X PLUS, G705 | adds a custom animation index at 252 |
| 6 | GPX2 | PRO X Superlight 2, PRO 2 Lightspeed | different layout, see below (open libratbag PR #1896) |
| 7 | GPX2_BHOP | same mice, firmware from 2025-04 with "BHOP" | format 6 plus a bhop timeout byte |

## Sectors and directory

| Sector | Contents |
|---|---|
| `0x0000` | user profile directory (also called dictionary or control sector) |
| `0x0001` .. `0x00NN` | user profiles (and macro pages), NN < sector count |
| `0x0100` | ROM directory **[Solaar] [superstrike]**, not read by us |
| `0x0101` .. `0x01NN` | ROM profiles, NN = OOB count |

**[HW]** On our G502 HERO the user directory (`0x0000`) and user sectors
`0x0001`, `0x0002` were all `0xFF` (never written: the mouse had never been
configured with G HUB). In that state the firmware runs the ROM profiles and
`fn4` returns `01 0N`.

### Directory format

The directory is a list of 4-byte entries, terminated by `ff ff`, padded with
`ff`, with a CRC in the last two bytes:

| Bytes | Field |
|---|---|
| 0-1 | profile sector, big endian (`00 01`, `00 02`, ...) |
| 2 | enabled (1) / disabled (0) |
| 3 | unknown, written as 0 by every tool (libhidpp calls it `dir_unknown`) |

After the last entry: `ff ff 00 00` (libratbag and Solaar write the `00 00`;
libhidpp writes only `ff ff`). The rest is `ff`, then CRC16 big endian.

Example from libratbag issue #1181 (256-byte sector, written by libratbag or
G HUB): `00 01 01 00 00 02 00 00 00 03 00 00 00 04 00 00 00 05 00 00 ff ff 00 00 ff ... d2 aa`,
which is profile 1 enabled, profiles 2 to 5 disabled.

**[HW]** We wrote `00 01 01 00 | 00 02 01 00 | 00 03 01 00 | 00 04 01 00 | 00 05 01 00 | ff ff 00 00`,
padded `ff`, CRC at 253-254. The firmware accepted it and switched to the
user profiles.

How the firmware chooses (from libratbag, Solaar and our test):

- Directory CRC invalid (for example all `ff`): use ROM profiles. libratbag
  logs "Profile directory has an invalid CRC... Reading ROM profiles."
- Directory valid: use the listed sectors. A listed profile with a bad CRC
  makes libratbag fall back to the ROM profile at the same index; what the
  firmware does in that case is **[unverified]**.
- Solaar's `get_profile_headers` treats a user directory starting with
  `00 00 00 00` or `ff ff ff ff` as "look in ROM" and then reads the ROM
  directory at `0x0100`.

### CRC

CRC-16/CCITT-FALSE: polynomial `0x1021`, init `0xFFFF`, no reflection, no
final XOR, computed over `sector[0 .. size-2)`, stored **big endian** in the
last two bytes. Same for directory and profile sectors. **[HW] [libratbag]
[Solaar] [superstrike]**

```go
// this project, lighting/onboard.go
crc := uint16(0xFFFF)
for _, b := range data {
    crc ^= uint16(b) << 8
    for i := 0; i < 8; i++ {
        if crc&0x8000 != 0 { crc = crc<<1 ^ 0x1021 } else { crc <<= 1 }
    }
}
```

Useful reference values: the CRC of 253 bytes of `0xFF` is `0xB92A`, of 254
bytes of `0x00` is `0x592D`. libratbag logs such as "Invalid CRC (ffff !=
b92a)" (G903, issue #1149) mean "erased 255-byte sector", and "Invalid CRC
(0000 != 592d)" (G502 Proteus Spectrum, issue #1798) means "zeroed 256-byte
sector".

**[HW]** ROM sectors do not carry a CRC: all five G502 HERO ROM profiles end in
`ff ff`. Do not validate ROM sectors.

## Profile layout, formats 1 to 5

Offsets for a 255 or 256-byte sector. The CRC is always the last two bytes,
so with 255-byte sectors it sits at 253-254 and with 256-byte sectors at
254-255. libratbag's struct is 256 bytes with `crc` at 254; libratbag PR
#1789 (rejected) pointed out this is wrong for 255-byte sectors, but the
struct field is unused because libratbag computes the CRC position from the
sector size.

| Offset | Size | Field | Notes |
|---|---|---|---|
| 0 | 1 | report rate | in ms (1 = 1000 Hz, 2 = 500 Hz, 4 = 250 Hz, 8 = 125 Hz). libratbag: `hz = 1000 / max(1, byte)` |
| 1 | 1 | default DPI index | 0 to 4 |
| 2 | 1 | shifted DPI index | used while a "DPI shift" (sniper) button is held |
| 3 | 10 | DPI list | 5 x `uint16` **little endian**; `0x0000` or `0xFFFF` ends the list (libhidpp). LGS writes 0 for unused slots |
| 13 | 3 | profile color RGB | Solaar "red, green, blue", libhidpp "unused on G502 (format 2)". Purpose unclear |
| 16 | 1 | power mode | libhidpp only exposes it for non-corded devices; `0xff` = not applicable |
| 17 | 1 | angle snapping | 0 off, 1 on |
| 18 | 2 | write counter | Solaar `write_count` (LE), libhidpp `revision` ("metadata used by LGS?"). libratbag treats 18-27 as reserved |
| 20 | 8 | reserved | |
| 28 | 2 | power save timeout | wireless mice; units unknown (Solaar docs say ms, **[unverified]**) |
| 30 | 2 | power off timeout | as above |
| 32 | 64 | buttons | 16 x 4-byte bindings, only `button count` are used |
| 96 | 64 | G-shift buttons | 16 x 4-byte bindings, used while a G-shift button is held |
| 160 | 48 | profile name | 24 UTF-16LE code units, zero padded; all `ff` means "default name" |
| 208 | 11 | LED block 0 | see LED blocks |
| 219 | 11 | LED block 1 | |
| 230 | 11 | alt LED block 0 | power save lighting (Solaar: "for power saving mode") |
| 241 | 11 | alt LED block 1 | |
| 252 | 1 | custom animation index | format 5 (G502 X PLUS, G705); libratbag sets it to 0 with the G502X_PLUS quirk |
| 253 | 1 | free / unused | overlaps the CRC on 255-byte sectors |
| size-2 | 2 | CRC16, big endian | |

Notes on the name: Solaar and libhidpp decode UTF-16LE. libratbag (master)
copies it as a C `char[48]` and only gets proper UTF-16 in open PR #1896.

### Verified example: G502 HERO ROM profiles [HW]

Full decode of the five ROM profiles of our unit (sector size 255):

| Sector | Rate (ms) | Default idx | DPI list | Buttons 0-10 | LED 0 = LED 1 |
|---|---|---|---|---|---|
| `0x0101` | 1 | 1 | 1200, 2400, 3200, 6400 | standard | `03 00 00 00 00 00 1f 40 00 00 00` cycle, 8000 ms |
| `0x0102` | 1 | 2 | 400, 800, 1600, 2400 | standard | `0a 00 dc ff 1f 40 00 00 00 00 00` breathe `#00dcff`, 8000 ms |
| `0x0103` | 2 | 0 | 2400 | productivity (see below) | `01 ff ff ff 00 ...` static white |
| `0x0104` | 1 | 0 | 3200 | standard | `01 ff dc 00 00 ...` static `#ffdc00` |
| `0x0105` | 1 | 0 | 6400 | standard | `01 80 00 ff 00 ...` static `#8000ff` |

In all ROM profiles: shift DPI index 0, profile color `ff ff ff`, power mode
`ff`, angle snap `00`, bytes 18-31 all `ff`, name all `ff`, alt LED blocks
(230-251) all `ff`, bytes 252-254 `ff ff ff` (no CRC).

"Standard" button table (bytes 32-75):

```
80010001 80010002 80010004 80010008 80010010   left right middle back forward
9007ffff                                       DPI shift (sniper button)
9004ffff 9003ffff                              DPI down, DPI up
900affff                                       cycle profile (button behind the wheel)
9002ffff 9001ffff                              wheel tilt right, tilt left
```

ROM profile 3 is a "productivity" profile: button 5 is `900bffff` (G-shift),
buttons 6 and 7 are `80020106` (Ctrl+C) and `80020119` (Ctrl+V), and the
G-shift table at 96 is populated with Ctrl+T (`80020117`), Ctrl+F4
(`8002013d`), Ctrl+Shift+T (`80020317`), Ctrl+X, Ctrl+Z, Ctrl+0, Ctrl+Tab and
Ctrl+Shift+Tab. That makes a good test vector for button and G-shift parsing.

### Example: user profile written by libratbag (G502 HERO, 256-byte sectors) [issue]

From the raw log in libratbag #1181 (sector `0x0001`):

```
00: 01 01 00 b0 04 64 00 64 00 64 00 00 00 ff ff ff   rate 1, idx 1, dpi 1200,100,100,100,0, color ffffff
10: ff ff ff ff ff ff ff ff ff ff ff ff ff ff ff ff   power mode, snap, write counter, timeouts all ff
20: 80 01 00 01 80 01 00 02 80 01 00 04 80 02 02 09   buttons ...
...
a0: 00 ... 00                                          name zeroed
d0: 0a 72 9f cf 13 88 00 2e 00 00 00 01 ef 29 29 00   led0 breathe 729fcf 5000 ms 46%, led1 static ef2929
e0: 00 00 00 00 00 00 0a 72 9f cf 13 88 00 2e 00 00   alt led0 = led0
f0: 00 01 ef 29 29 00 00 00 00 00 00 00 ff ff 49 dd   alt led1 = led1, 252-253 ff ff, CRC 49dd
```

This matches libratbag's writer, which copies the normal LED blocks into the
alt blocks ("we write the current led instead of the stored value").

## Profile layout, formats 6 and 7 (PRO X 2 family)

From libratbag PR #1896 (`union hidpp20_internal_profile_ext`, not merged)
and the linux-superstrike reverse engineering notes. 255-byte sectors.
**[unverified]** by us.

| Offset | Size | Field |
|---|---|---|
| 0 | 1 | wireless report rate (0x8061 encoding) |
| 1 | 1 | wired report rate (0x8061 encoding) |
| 2 | 1 | default DPI index |
| 3 | 1 | shifted DPI index |
| 4 | 25 | 5 x `[x u16, y u16, lod u8]` (lift-off distance 1 low, 2 medium, 3 high) |
| 29 | 8 | reserved |
| 37 | 1 | bhop timeout (format 7 only), 100 ms units, 0 = off |
| 38 | 6 | reserved |
| 44 | 2 | power save timeout |
| 46 | 2 | power off timeout |
| 48 | 64 | buttons (16 x 4) |
| 112 | 48 | G-shift buttons (12 x 4) |
| 160 | 48 | name, UTF-16LE |
| 208 | 44 | LED blocks and alt LED blocks as in format 2 |
| 252 | 1 | reserved (default `0x03`) |
| 253 | 2 | CRC16 |

linux-superstrike (PRO X 2 Superstrike) describes byte 0 as the report rate
with `Hz = 125 << byte` (0 = 125 Hz up to 6 = 8000 Hz), verified by measuring
the input report rate, and the button table at 48 "not Solaar's generic 32".
Its DPI word layout differs from libratbag's; treat both as tentative.
Solaar's `OnboardProfile` class assumes the format 1 to 5 layout, so it
decodes these devices wrongly.

## Button bindings (4 bytes)

| Bytes | Meaning | Source |
|---|---|---|
| `80 01 hh ll` | mouse buttons, `uint16` bitmask big endian: `0001` left, `0002` right, `0004` middle, `0008` back, `0010` forward, ... | all |
| `80 02 mm kk` | keyboard: `mm` modifier bits, `kk` HID keyboard usage | all |
| `80 03 hh ll` | consumer control usage, big endian | all |
| `80 00 ff ff` | no action (HID type, subtype NOOP) | [Solaar], libratbag #1898 fix |
| `90 ss ff dd` | special function `ss`, `dd` = data (profile number for "profile select") | [libratbag] `[90, special, reserved, profile]`, [Solaar] `[9x, func, ff, data]`, [HW] ROM uses `90 ss ff ff` |
| `0b pp oo oo` with b = 0 | macro: page `pp`, offset `oooo` | see macros |
| `ff ff ff ff` | unused / not mapped | [libhidpp] (type `0xff` = disabled), [HW] ROM |

Solaar reads byte 0 as a "behavior" nibble: `0x0` execute macro, `0x1` stop
macro, `0x2` stop all macros, `0x8` send, `0x9` function. libratbag and
libhidpp read byte 0 as a type: `0x80` HID, `0x90` special, `0x00` macro,
`0xff` disabled. Both views agree on everything the tools write.

Modifier byte (`80 02`): libratbag defines `0x01` Ctrl and `0x02` Shift; the
ROM data uses `0x01` (Ctrl) and `0x03` (Ctrl+Shift). It is most likely the
standard HID modifier byte (`0x04` Alt, `0x08` GUI, upper nibble for right
hand modifiers), **[unverified]**.

Special functions (`90 ss`), from libratbag `hidpp20.h` and Solaar
`ButtonFunctions`:

| ss | Function |
|---|---|
| 00 | no action |
| 01 | wheel tilt left |
| 02 | wheel tilt right |
| 03 | next DPI |
| 04 | previous DPI |
| 05 | cycle DPI |
| 06 | default DPI |
| 07 | DPI shift (hold) |
| 08 | next profile |
| 09 | previous profile |
| 0a | cycle profile |
| 0b | G-shift (hold) |
| 0c | battery indicator (memory model >= 3 per libratbag) |
| 0d | profile select (`dd` = profile) |
| 0e | performance / mode switch |
| 0f | host button (switch host, Solaar marks unverified) |
| 10 | scroll down |
| 11 | scroll up |

libratbag only maps `01`-`0b` to ratbag actions; the rest read as invalid.

Known writer bug: libratbag master writes a "disabled" button as `00 xx xx xx`
(type 0, which is a macro) instead of `80 00 ff ff`, corrupting the button
(libratbag #1898, reported on a G502 HERO, with the fix in the issue).

## Macros

Macros live in user sectors, addressed by `[page, offset]` from the button
binding. All three implementations agree on the byte layout of the binding
but disagree on the macro stream details, and none of the projects claims
full support. libratbag issues #1722 and #1862 report half-working macros.

Binding: byte 0 `0x00` (memory type writable, or Solaar behavior "execute"),
byte 1 page (sector), bytes 2-3 offset big endian (libratbag only uses byte 3).

Macro opcodes, macro format 1 (from libhidpp `MacroFormat.cpp` and
libratbag `hidpp20.h`). The opcode's top 3 bits give the length: `0x00`-`0x1f`
1 byte, `0x20`-`0x3f` 2 bytes, `0x40`-`0x5f` 3 bytes, `0x60`-`0x7f` 5 bytes.

| Op | Length | Meaning |
|---|---|---|
| 00 | 1 | no-op (wait one macro step) |
| 01 | 1 | wait for release |
| 02 | 1 | repeat while pressed |
| 03 | 1 | repeat until cancelled |
| 20 | 2 | wheel (signed 8-bit) |
| 21 | 2 | horizontal wheel / AC pan |
| 40 | 3 | delay, ms, big endian |
| 41 / 42 | 3 | mouse buttons down / up, bitmask big endian |
| 43 / 44 | 3 | key press / release `[modifiers, key]` |
| 45 / 46 | 3 | consumer control down / up, big endian |
| 60 | 5 | jump `[memory type, page, offset u16]` (libhidpp; libratbag parses it as 3 bytes `[offset, page]`) |
| 61 | 5 | pointer move `[x s16, y s16]` |
| ff | 1 | end (mandatory, otherwise the firmware runs whatever follows) |

Macros that cross a page boundary continue on the next page, or use a jump.
libhidpp says format 1 has no conditional jumps (JumpIfPressed,
JumpIfReleased), so only simple loops are possible.

## LED blocks (11 bytes)

Format: `[effect ID, 10 parameter bytes]`, the same effect IDs and parameter
layout as `0x8070` ColorLEDEffects SetZoneEffect, but with the **effect ID**
in byte 0, not the zone's effect index. **[HW]** This project builds the
block as `[zoneEffect.Type.ID, params...]` and the firmware rendered it.

| ID | Effect | Parameters (byte offsets in the block) | Source |
|---|---|---|---|
| 00 | off | none | all |
| 01 | static | 1-3 RGB, 4 ramp/effect flag (0 default, 1 ramp, 2 no ramp per Solaar) | [libratbag] [Solaar] [HW] |
| 03 | cycle | 6-7 period ms BE, 8 intensity (0 = 100%) | [libratbag] [libhidpp] [HW ROM1] |
| 04 | color wave | | [libratbag] enum only |
| 05 | starlight | 1-3 sky RGB, 4-6 star RGB | [libratbag] |
| 0a | breathe | 1-3 RGB, 4-5 period ms BE, 6 waveform, 7 intensity (0 = 100%) | [libratbag] [libhidpp] [HW ROM2] |
| 0b | ripple | 1-3 RGB, 4 reserved, 5-6 period BE | [libratbag] |
| 0c | custom (onboard animation) | 1 slot, 2-3 first frame, 4-5 length, 6-7 frame period, 8 intensity | [libratbag], G502 X PLUS / G705 |

libratbag waveform values for breathe: 0 default, 1 sine, 2 square, 3
triangle, 4 sawtooth, 5 sharkfin, 6 exponential.

### Which block drives which LED

**[HW] Important.** On the G502 HERO, `0x8070` reports zone 0 at location 1
("primary", the DPI indicator LEDs) and zone 1 at location 2 ("logo").
Writing green to profile block 0 and red to block 1 gave `8070 fn14 [00]`
(GetZoneEffect) `00 01 ff 00 00` (zone 0 red) and `[01]` -> `01 01 00 ff 00`
(zone 1 green), and the logo was green. So:

- **LED block 0 (offset 208) drives zone 1 (logo)**
- **LED block 1 (offset 219) drives zone 0 (primary / DPI indicator)**

That is the reverse of the 0x8070 zone order. libhidpp agrees by naming
208 `logo_effect` and 219 `side_effect`. libratbag does **not**: it uses
`profile->leds[led->index]` with the 0x8070 zone index, and its log labels
location 1 as "Logo LED". This is a likely cause of libratbag #1181
("[G502 HERO] Can't control DPI leds"). For the G502 X PLUS libratbag has a
quirk to always use slot 1 for its single reported LED, which fits the same
pattern (single zone, stored in the last block).

This project's code maps zones with `slot = len(zones) - 1 - zone`; that is
verified for the 2-zone G502 HERO only.

### When the firmware applies the blocks

**[HW]** In onboard mode, a zone effect set with `0x8070` SetZoneEffect
(fn3) is shown immediately, but is **overwritten whenever the firmware loads
a profile**: on a profile switch (button behind the wheel), on
SetCurrentProfile, and on power up. To make a color stick on a mouse with
onboard profiles, write it into every profile's LED blocks. After we did
that, the logo kept its color across all profile switches and power cycles.

Alt LED blocks: Solaar describes 230-251 as lighting "for power saving
mode", libratbag writes a copy of the normal blocks there. On our ROM
profiles they are all `ff` (presumably "use the normal block"). What the
firmware shows in power save on corded mice is **[unverified]**.

## Host mode versus onboard mode

`fn1 [02]` (host mode) and `fn1 [01]` (onboard mode):

- **Onboard (1)**: the mouse runs the active profile: report rate, DPI list,
  button mapping, macros, LED blocks. `fn3`/`fn4`/`fnB`/`fnC` work, events
  are sent (per libhidpp).
- **Host (2)**: the profile is not applied. Buttons send plain HID button
  reports, mapped through `0x8110` MouseButtonSpy; G-shift, DPI and profile
  buttons do nothing in firmware (so DPI and report rate must be set with
  `0x2201` and `0x8060`). libhidpp: "Current profile or DPI index cannot be
  set (invalid argument error) and events will not be sent."
- The mode is runtime state. After a power cycle the device starts in
  onboard mode (Solaar and OpenRGB both re-assert host mode on reconnect).
  Our G502 HERO reported `fn2 -> 01` at startup **[HW]**.

How tools use it:

- **libratbag** forces onboard mode in `hidpp20_onboard_profiles_allocate`
  if the device is not already in it. On the PRO X 2 (format 7) this left the
  mouse with empty button maps when the user directory was unreadable
  (libratbag PR #1806, #1896 fall back to host mode instead).
- **Solaar** exposes a setting "Onboard Profiles" with choices Disabled
  (`fn1 [02]`), "Profile N" (`fn1 [01]` + `fn3 [00 0N]`) and "Read-Only Profile
  N" (`fn3 [01 0N]`). It warns that report rate and other settings may need
  onboard profiles disabled. For RGB devices (0x8071) Solaar's LED control
  claim sends `fn1 [02]` on devices without 0x8101 and `fn1 [01]` on
  release.
- **OpenRGB** (LogitechHIDPP20Controller) sends `fn1 [02]` when claiming
  software control and `fn1 [01]` on release, except for devices with the
  `KEEP_ONBOARD_MODE` quirk (G915 TKL: "host mode breaks the
  onboard-profile-mapped F-row"). It only implements fn1 and fn2 of 0x8100.
- **This project** leaves 0x8070 mice (G502 HERO) in onboard mode, writes
  zone effects live with 0x8070, and offers `SaveToProfiles` to make them
  persistent. Host mode is only used for 0x8071 devices that lack 0x8101.

## GetCurrentProfile and SetCurrentProfile semantics

**[HW]** `fn4` returns the sector of the running profile, not an index:
`01 03` while the mouse ran ROM profile 3, and `00 01` after we wrote user
profiles and selected the first one with `fn3 [00 01]`. libhidpp models it
as `[memory type, page]`, Solaar as a 16-bit sector.

libratbag reads byte 1 and treats it as a 0-based index, which is wrong for
user sectors (they start at 1). Instead of fixing the interpretation it grew
an `INDEX_OFFSET` quirk ("Device returns 1-indexed profile, decrement by
1") that is now enabled for the G502 HERO, G502 X, G502 X PLUS and requested
for the G Pro Wireless (libratbag #1799, #1872, #1875). The real rule is:
the reply is `[mem type, page]`, and page N is user profile N.

libratbag's `set_current_profile` sends `[00, index + 1]`, which is correct
for user profiles. Selecting a ROM profile needs `[01, N]` (Solaar does this;
this project's `ResetProfiles` sends `fn3 [01 NN]` after clearing the
directory).

## How the tools read and write profiles

| Tool | Read | Write | Notes |
|---|---|---|---|
| libratbag / Piper | directory, then each listed sector (CRC checked), ROM fallback per profile | full sector rebuilt from parsed fields (rest `ff`), then directory with all `num_profiles` entries | rewrites every enabled profile on commit; LED alt blocks copied from normal blocks; buttons only re-encoded when dirty |
| Solaar (`solaar profiles`) | directory (ROM directory if user one empty), each listed sector | YAML round trip; rebuilds sector from fields, skips sectors whose content (minus CRC) is unchanged | refuses sectors `>= sector_count`; does not handle macros; profile version and device name checked before loading |
| libhidpp (`hidpp20-onboard-profiles-*`) | page based memory mapping | XML profiles, writes whole pages | only format 2 tested (G502 Spectrum) |
| linux-superstrike | read sector, patch bytes, recompute CRC, write, read back | same | formats 6/7 |
| This project (`SaveToProfiles`) | directory; if CRC invalid, ROM profiles `0x0101..` | copies each ROM profile (or existing user profile) byte for byte, only replaces LED blocks, reseals CRC, writes, reads back; writes the directory **last** | refuses to touch user profiles with a bad CRC; reselects the active profile with `fn3` so the firmware reloads |

Writing the directory last matters: until it is written the firmware keeps
running the untouched ROM profiles, so an interrupted write leaves a working
mouse.

## Known pitfalls

- **Sector size differs per unit and firmware.** 255 on our G502 HERO, 256
  on another G502 HERO, 255 on the G305, G304, G903, PRO X 2. Read GetInfo.
- **Reads cannot run past the sector end**, so the last read of a 255-byte
  sector starts at 239.
- **CRC position follows the sector size** (last two bytes), not a fixed
  struct offset.
- **ROM sectors have no CRC** (`ff ff`). Don't reject them.
- **Unwritten user sectors**: the G502 HERO returns `ff` bytes; the G305 and
  PRO X 2 return `ERR_INVALID_ARGUMENT` when reading the never-written user
  directory (libratbag `HIDPP20_QUIRK_G305`, PR #1806). Treat both as
  "use ROM".
- **libratbag's LED index mapping** does not match the firmware's block
  order on the G502 HERO (block 0 = logo = zone 1).
- **GetCurrentProfile returns a sector**, not an index (see above).
- **The profile switch button sends no notification** on our G502 HERO.
- **Profile switches and power cycles overwrite live 0x8070 effects** with
  the profile's LED blocks.
- **Disabled-button encoding bug** in libratbag (#1898).
- **Format 6 and 7** use a different layout; tools that assume the classic
  layout corrupt them. Solaar caps the format at 5, libratbag master rejects
  6 and 7.
- **Name encoding**: UTF-16LE (Solaar, libhidpp, G HUB) versus bytes
  (libratbag master).
- **fn8 can report an error and still have written the data** (libhidpp).
  Always read back.
- **Report rate**: set in the profile in onboard mode; `0x8060` SetReportRate
  is only reliable in host mode (Solaar note; libratbag writes the rate into
  the profile when 0x8100 is present).

## 0x8110 MouseButtonSpy (brief)

From libhidpp `IMouseButtonSpy.h`; libratbag only detects the feature.
Present on the G502 HERO (index `0x0d` in libratbag #1181) and G502 X PLUS.

| fn | Name | Params / reply |
|---|---|---|
| 0 | GetMouseButtonCount | -> `[count]` |
| 1 | StartMouseButtonSpy | start sending button events |
| 2 | StopMouseButtonSpy | |
| 3 | GetMouseButtonMapping | -> one byte per button, 0 = disabled, 1-16 = HID button |
| 4 | SetMouseButtonMapping | same format |
| event 0 | MouseButtonEvent | 16-bit bitmask of raw button states |

Spy events report raw buttons regardless of remapping. The remapping from
this feature only applies in **host mode**; in onboard mode the profile's
bindings win. The mapping resets when the mouse resets.

## 0x8060 AdjustableReportRate and 0x8061 (brief)

`0x8060` **[libratbag] [Solaar]**:

| fn | Name | Params / reply |
|---|---|---|
| 0 | GetReportRateList | -> `[bitmask]`, bit n set means (n+1) ms is supported |
| 1 | GetReportRate | -> `[ms]` |
| 2 | SetReportRate | `[ms]` |

G502 HERO: list `0x8b` = 1, 2, 4, 8 ms; current `01` = 1000 Hz (libratbag
#1181). In onboard mode the profile's byte 0 controls the rate.

`0x8061` ExtendedAdjustableReportRate (PRO X 2 family, Superstrike): Solaar
reads with fn2 and writes with fn3, index 0..6 = 8 ms, 4 ms, 2 ms, 1 ms,
500 us, 250 us, 125 us. libratbag PR #1896 adds it with separate wired and
wireless lists. linux-superstrike found that the live setter changes the rate
in host mode but is not persistent, and that the getter keeps reporting the
old value.

## Sources

Code and docs:

- libratbag `src/hidpp20.c`, `src/hidpp20.h`, `src/driver-hidpp20.c`:
  https://github.com/libratbag/libratbag (local copy in the scratchpad)
- libratbag device files: https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-hero.device,
  https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g305.device
- Solaar `lib/logitech_receiver/hidpp20.py` (`OnboardProfile`,
  `OnboardProfiles`, `Button`, `LEDEffectSetting`), `settings_templates.py`
  (`OnboardProfiles`, `ReportRate`), `notifications.py`, `rgb_power.py`:
  https://github.com/pwr-Solaar/Solaar
- Solaar profile documentation: https://pwr-solaar.github.io/Solaar/capabilities/
  (source `docs/capabilities.md`)
- libhidpp (cvuchener/hidpp) `src/libhidpp/hidpp20/IOnboardProfiles.h`,
  `ProfileFormat.cpp`, `MacroFormat.cpp`, `ProfileDirectoryFormat.cpp`,
  `MemoryMapping.cpp`, `IMouseButtonSpy.h`, `profile_format.md`:
  https://github.com/cvuchener/hidpp
- OpenRGB `LogitechHIDPP20Controller.cpp/.h`:
  https://gitlab.com/CalcProgrammer1/OpenRGB (merge requests !3270, !3352)
- linux-superstrike `REVERSE_ENGINEERING.md` (MIT):
  https://github.com/oddvalue/linux-superstrike
- This project: `lighting/onboard.go`, `research/hardware-log.md`

Issues and PRs:

- libratbag #1181 G502 HERO DPI LEDs, with full raw sector dump:
  https://github.com/libratbag/libratbag/issues/1181
- libratbag #873 G305 ERR_INVALID_ARGUMENT on sector 0:
  https://github.com/libratbag/libratbag/issues/873
- libratbag #1788 / #1789 profile struct size 0xff:
  https://github.com/libratbag/libratbag/issues/1788,
  https://github.com/libratbag/libratbag/pull/1789
- libratbag #1898 disabled button written as macro:
  https://github.com/libratbag/libratbag/issues/1898
- libratbag #1870 PRO X 2 DEX format 0x07:
  https://github.com/libratbag/libratbag/issues/1870
- libratbag #1806 and #1896 format 6/7 support:
  https://github.com/libratbag/libratbag/pull/1806,
  https://github.com/libratbag/libratbag/pull/1896
- libratbag #1863, #1872, #1875 INDEX_OFFSET and G502X_PLUS quirks:
  https://github.com/libratbag/libratbag/pull/1863,
  https://github.com/libratbag/libratbag/pull/1872,
  https://github.com/libratbag/libratbag/issues/1875
- libratbag #1903 G502 X PLUS RGB enumeration:
  https://github.com/libratbag/libratbag/pull/1903
- libratbag #1149 G903 LED resets (blank-sector CRC):
  https://github.com/libratbag/libratbag/issues/1149
- libratbag #1798 G502 Proteus Spectrum settings written incorrectly:
  https://github.com/libratbag/libratbag/issues/1798
- Solaar #3205 feature lists for G815, G502 X PLUS, G903, G915 X:
  https://github.com/pwr-Solaar/Solaar/issues/3205
- Solaar PR #3149 discussion (0x8100 versus 0x8101 mode switching):
  https://github.com/pwr-Solaar/Solaar/pull/3149
