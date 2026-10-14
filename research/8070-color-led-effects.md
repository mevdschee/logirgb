# HID++ 2.0 feature 0x8070 ColorLEDEffects

Research notes for logirgb. 0x8070 is the older firmware effect engine used
by many G mice (G502 HERO, G403, G703 older firmware), the G810/G910/G410
keyboard family (together with 0x8080), G560 speakers and some headsets. Its
successor is 0x8071 RGBEffects (see `8071-rgb-effects.md`).

## Provenance markers

Every statement in this file carries one of these markers:

| Marker | Meaning |
|---|---|
| **[spec]** | Logitech's published feature document `x8070_colorledeffect_v7.pdf` (Version 7) from the public "hidpp20 public" Google Drive folder. |
| **[hw]** | Verified on hardware (G515 TKL wired 046d:c358 / G502 HERO 046d:c08b). For this feature only the G502 HERO applies. |
| **[Solaar]**, **[OpenRGB]**, **[libratbag]**, **[g810-led]**, **[openlogi]**, **[kernel]**, **[kernel patch]** | Read from that tool's source code or documentation. |
| **[unverified]** | Inferred, contradictory between sources, or not tested by us. |

## Wire basics

Requests are HID++ 2.0 long reports (`0x11`, 20 bytes):
`11 <devIdx> <featIdx> <fn<<4 | swid> <16 parameter bytes>`. Short
reports (`0x10`, 7 bytes, 3 parameter bytes) work for the getters, whose
requests fit in 3 bytes **[openlogi] [OpenRGB]**. In this file "fn" is the function
number; the byte on the wire is `fn<<4 | swid`. Multi byte fields are big
endian **[spec]**. Error replies use feature index `0xFF`:
`11 <dev> ff <featIdx> <fn<<4|swid> <errorCode>` **[kernel]**.

HID++ 2.0 error codes (from the kernel `HIDPP20_ERROR_*` defines and
Solaar's `ErrorCode`):

| Code | Kernel name | Solaar name | Notes |
|---|---|---|---|
| 0x01 | UNKNOWN | UNKNOWN | |
| 0x02 | INVALID_ARGS | INVALID_ARGUMENT | bad zone or effect index **[hw]** |
| 0x03 | OUT_OF_RANGE | OUT_OF_RANGE | |
| 0x04 | HW_ERROR | HARDWARE_ERROR | |
| 0x05 | NOT_ALLOWED | LOGITECH_ERROR | the spec calls this NOT_ALLOWED |
| 0x06 | INVALID_FEATURE_INDEX | INVALID_FEATURE_INDEX | |
| 0x07 | INVALID_FUNCTION_ID | INVALID_FUNCTION | unimplemented function |
| 0x08 | BUSY | BUSY | retry |
| 0x09 | UNSUPPORTED | UNSUPPORTED | |

## Function table

| fn | Wire byte (swid 0xA) | Name **[spec]** | Short request | Notes |
|---|---|---|---|---|
| 0 | `0x0A` | getInfo | yes | zone count, nvCaps, extCaps |
| 1 | `0x1A` | getZoneInfo | yes | location, effect count, persistency caps |
| 2 | `0x2A` | getZoneEffectInfo | yes | effect ID, caps, default period |
| 3 | `0x3A` | setZoneEffect | no (13 bytes) | the main "set effect" call |
| 4 | `0x4A` | getNvConfig | yes | |
| 5 | `0x5A` | setNvConfig | yes | |
| 6 | `0x6A` | getLedBinInfo | yes | factory LED binning data |
| 7 | `0x7A` | getSwControl | yes | |
| 8 | `0x8A` | setSwControl | yes | |
| 9 | `0x9A` | getEffectSettings | yes | not supported if extCaps bit 0x0002 |
| 10 | `0xAA` | clearEffectSettings | yes | |
| 11 | `0xBA` | setCyclingDirection | yes | |
| 12 | `0xCA` | getCurrentColor | yes | |
| 13 | `0xDA` | synchronizeEffect | yes | |
| 14 | `0xEA` | getZoneEffect | yes | only if extCaps bit 0x0001 |
| 15 | `0xFA` | setLedBinInfo | no | only if extCaps bit 0x0004 |

Event: `[0] syncEffectEvent` **[spec]**.

The same table appears with different names in OpenRGB's
`LOGITECH_FP8070` enum (`INFO, ZONE_INFO, EFFECT_INFO, SET_EFFECT, SET_CFG,
GET_CFG, GET_BIN_INFO, GET_SW_CTL, SET_SW_CTL, GET_STATUS, CLEAR_EFFECT,
SET_DIR, GET_COLOUR, SYNC_CFG, GET_EFFECT, SET_BIN_INFO`) **[OpenRGB]**.
Note that OpenRGB names fn4/fn5 SET_CFG/GET_CFG, the reverse of the spec
(fn4 = get, fn5 = set) **[spec]**.

## [0] getInfo

Request: no parameters.

| Reply byte | Field **[spec]** |
|---|---|
| 0 | zoneCount |
| 1-2 | nvCapabilities (u16) |
| 3-4 | extCapabilities (u16) |
| 5-15 | reserved |

extCapabilities bits **[spec]**:

| Bit | Meaning |
|---|---|
| 0x0001 | supports getZoneEffect (fn14) |
| 0x0002 | does NOT support getEffectSettings (fn9) |
| 0x0004 | supports setLedBinInfo (fn15) |
| 0x0008 | monochrome effects only |
| 0x0010 | does NOT support synchronizeEffect nor send syncEffectEvent |

nvCapabilities bits are the nvCapability IDs of getNvConfig (0x0001
bootUpEffect, 0x0002 demo, 0x0004 userDemoMode) **[spec]**.

**[hw] G502 HERO:** `8070 0` replied `02 00 03 ...`: two zones. The hardware
log reads this as "extCaps 0x03", but by the spec layout bytes 1-2 are
nvCapabilities, so `00 03` is nvCaps = bootUpEffect + demo, and extCaps is in
bytes 3-4 (not recorded in the log). Since fn14 getZoneEffect worked on this
mouse, extCaps bit 0x0001 is presumably set in bytes 3-4. **[unverified]**:
re-read the full 16 byte reply to confirm.

**[libratbag]** reads the reply into a packed struct
`{u8 zone_count; u16 nv_caps; u16 ext_caps}` which matches the spec. In
`hidpp20.c` it only uses `ext_caps & HAS_ZONE_EFFECT` to decide whether to
call fn14, and `MONOCHROME_ONLY` to decide whether colors apply.

**[Solaar]** `LEDEffectsInfo` (hidpp20.py) unpacks `"!BHH"` and takes
`readable = capabilities & 0x1` from the third field, that is extCaps bit
0x0001, matching the spec.

## [1] getZoneInfo(zoneIndex)

Request: `[zoneIndex]`.

| Reply byte | Field **[spec]** |
|---|---|
| 0 | zoneIndex |
| 1-2 | locationEffect (u16; the text says 8-bit, the table shows MSB/LSB) |
| 3 | effectsNumber |
| 4 | persistencyCapabilities |

Location codes **[spec]**:

| Value | Location |
|---|---|
| 0 | undefined (not in spec; libratbag/Solaar "Unknown") |
| 1 | primary |
| 2 | logo |
| 3 | left side |
| 4 | right side |
| 5 | combined |
| 6-11 | primary_1 .. primary_6 (separate sub zones of the primary region, v5 "Finch") |

persistencyCapabilities **[spec]**: `0x00` not supported, `0x01` always on,
`0x02` always off, `0x04` on then off. These describe how event displays (for
example DPI LEDs) persist, not how effects are stored.

**[hw] G502 HERO:**

```
8070 1 00 -> 00 00 01 04     zone 0, location 0x0001 primary, 4 effects
8070 1 01 -> 01 00 02 04     zone 1, location 0x0002 logo, 4 effects
```

On this mouse zone 1 (logo) is the visible G logo. Zone 0 ("primary") is
described as the DPI indicator, but setting it had no visible effect: the DPI
LEDs only flash blue on DPI/profile button presses **[hw]**.

Note: Solaar's `LEDZoneInfo` and libratbag both read location as a u16 at
bytes 1-2 and effectsNumber at byte 3 **[Solaar] [libratbag]**.

## [2] getZoneEffectInfo(zoneIndex, zoneEffectIndex)

Request: `[zoneIndex, zoneEffectIndex]`.

| Reply byte | Field **[spec]** |
|---|---|
| 0 | zoneIndex |
| 1 | zoneEffectIndex |
| 2-3 | effectID (u16) |
| 4-5 | effectCapabilities (u16) |
| 6-7 | effectPeriod (u16, ms, 0 if not available) |

**[hw] G502 HERO:** both zones report 4 effects with IDs 0x00 off, 0x01
static, 0x0A breathe, 0x03 cycle. The order of indices on the mouse is
idx0 = 0x00, idx1 = 0x01, idx2 = 0x0A, idx3 = 0x03 (idx3 = cycle is confirmed
by getZoneEffect below).

**[OpenRGB]** notes that an earlier version of its 0x8070 code read bytes 0-1
as the effect ID, which only worked on zone 0. The correct ID is at bytes 2-3.

### Effect IDs **[spec]**

| ID | Name | Notes |
|---|---|---|
| 0x00 | Disabled | |
| 0x01 | Fixed (static) | |
| 0x02 | Pulsing/Breathing (legacy, "speed") | |
| 0x03 | Color Cycling | |
| 0x04 | Color Wave | |
| 0x05 | Starlight | |
| 0x06 | Light on Press | |
| 0x07 | Audio Visualizer | params TBD in spec |
| 0x08 | Boot Up | no params |
| 0x09 | Demo Mode | no params |
| 0x0A | Pulsing/Breathing (Waveform) | |
| 0x0B | Ripple | |

libratbag also defines 0x0C "custom" for 0x8070 and maps DEMO_MODE to 8 (a
typo, the spec says 9) **[libratbag]**. 0x0C is only defined in 0x8071.

### effectCapabilities per effect **[spec]**

"If zero, Raptor compatibility is assumed." Bit 0x0001 must be set for any
other bit to be meaningful.

| Effect | Bits |
|---|---|
| 0x01 Fixed | 0x0002 ramp up/down supported, 0x0004 "no effect" (instant) supported |
| 0x03 Cycling | 0x0002 speed (8 bit), 0x0004 intensity, 0x4000 sync capable, 0x8000 period (16 bit) |
| 0x04 Wave | 0x0002 startColor, 0x0004 stopColor, 0x0008 speed, 0x0010 intensity, 0x0020 horizontal, 0x0040 vertical, 0x0080 center-out, 0x0100 inward, 0x0200 outward, 0x0400 reverse horizontal, 0x0800 reverse vertical, 0x1000 center-in, 0x4000 sync, 0x8000 period |
| 0x0A Breathing | 0x0002 speed, 0x0004 intensity, 0x0008 sine, 0x0010 square, 0x0020 triangle, 0x0040 sawtooth, 0x0080 shark fin, 0x0100 exponential, 0x4000 sync, 0x8000 period |
| 0x0B Ripple | 0x0002 speed ("if zero, FlyingV capabilities assumed") |

## [3] setZoneEffect(zoneIndex, zoneEffectIndex, param1..10, persistence)

Long request **[spec]**:

| Byte | Field |
|---|---|
| 0 | zoneIndex |
| 1 | zoneEffectIndex (index from fn2, NOT the effect ID) |
| 2-11 | param1..param10 |
| 12 | persistence |
| 13-15 | reserved |

Persistence values **[spec]**:

| Value | Meaning |
|---|---|
| 0 | volatile (RAM) |
| 1 | volatile and non-volatile (RAM and EEPROM) |
| 2 | non-volatile only (EEPROM) |

This differs from 0x8071, where byte 12 is a bit field (bit0 RAM, bit1
EEPROM, bits 2-3 power mode). On 0x8070 a value of `0x01` writes EEPROM.
libratbag deliberately sends `parameters[12] = 1` ("write to RAM and flash")
**[libratbag]**; OpenRGB's G-Lightsync mouse controller and logirgb's live
preview use `0x00`. OpenRGB's unified controller comment says "Persistence
bits 1-0, Power bits 2-3" for both features, which is the 0x8071 layout
**[OpenRGB] [unverified for 0x8070]**.

Reply: empty (an ack with zeroed parameters).

**[hw] G502 HERO:** `8070 3 [zone, effectIndex, params10]` (persistence byte
0) works directly in onboard mode, without switching 0x8100 to host mode and
without fn8 setSwControl. The zone effect is overwritten whenever the
firmware loads a profile (profile button), with or without fn8 `[01]`. The
profile button sends no HID++ notification, so the host cannot detect it and
re-apply. Storing the effect in the onboard profile LED blocks is the only
durable fix (and the profile LED block order is reversed relative to the
zones: block 0 drives zone 1). See `research/hardware-log.md`.

**[kernel patch]** An unmerged 2022 patch for `hid-logitech-hidpp.c`
("support Color LED feature (8071)", which actually uses 0x8070) sent
`CMD_COLOR_LED_EFFECTS_SET_ZONE_STATE 0x31` (fn3, swid 1) with
`[zone, 1, R, G, B]`, so it hard coded effect index 1 = static. It was never
merged; mainline has no RGB support.

### 10 byte parameter layouts **[spec]**

Offsets are 0-based into the 10 parameter bytes (spec param1 = offset 0).
On the wire, add 2 for the offset within the request (add 6 for the offset
within the 20 byte report).

| ID | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 |
|---|---|---|---|---|---|---|---|---|---|---|
| 0x00 Disabled | | | | | | | | | | |
| 0x01 Fixed | R | G | B | mode: 0 default, 1 ramp up/down, 2 no effect | | | | | | |
| 0x02 Pulsing (legacy) | R | G | B | speed (ms, 8 bit) | | | | | | |
| 0x03 Cycling | | | | | | period MSB | period LSB | intensity (0 = 100, 1-100) | | |
| 0x04 Wave | R start | G start | B start | R stop | G stop | B stop | period LSB (or speed) | direction | intensity | period MSB |
| 0x05 Starlight | R sky | G sky | B sky | R star | G star | B star | | | | |
| 0x06 Light on Press | R press | G press | B press | R release | G release | B release | delay MSB (ms) | delay LSB | | |
| 0x0A Breathing | R | G | B | period MSB | period LSB | waveform | intensity | | | |
| 0x0B Ripple | R | G | B | unused | period MSB | period LSB | | | | |

Notes:

- Cycling and Wave: "period MSB 0: unused; > 0: param7 contains the LSB
  portion of the period. The 16-bit value must be a multiple of the effect
  period from getEffectInfo" **[spec]**. With the MSB at 0 the LSB byte is
  read as an 8 bit device dependent speed.
- Wave splits the period: LSB at offset 6, MSB at offset 9. OpenRGB's
  unified controller encodes 0x04 exactly like this and warns that mixing it
  up with the 0x16 layout gives INVALID_ARGUMENT **[OpenRGB]**. Solaar's 0x04
  entry instead assumes the 0x16 layout (period at 6-7, direction at 9) with a
  comment that no probed device enumerates base Wave **[Solaar]**, so
  Solaar's 0x04 layout is likely wrong.
- Wave direction **[spec]**: 0 default (all play), 1 horizontal, 2 vertical,
  3 center-out, 4 inward, 5 outward, 6 reverse horizontal, 7 reverse vertical,
  8 center-in. Solaar names 1 Right, 2 Down, 6 Left, 7 Up **[Solaar]**.
- Breathing waveform **[spec]**: 0 default, 1 sine, 2 square, 3 triangle,
  4 sawtooth, 5 shark fin, 6 exponential. Intensity: 0 = default (100),
  valid 1-100.
- Fixed byte 3: Solaar and OpenRGB write `0x02` ("no effect", instant) for
  static colors; OpenRGB writes `0x00` when the color is black **[OpenRGB]**.
  libratbag writes `0x00` **[libratbag]**.
- libratbag's `hidpp20_internal_led` union (11 bytes: mode + 10 params) uses
  the same layouts: fixed `{rgb, effect}`, cycle `{5 unused, u16 period,
  intensity}`, breathing `{rgb, u16 period, waveform, intensity}`, ripple
  `{rgb, reserved, u16 period}`, starlight `{rgb sky, rgb star}`. This is the
  format used both by 0x8070 setZoneEffect and by the 0x8100 onboard profile
  LED blocks **[libratbag]**.
- **[g810-led]** builds one combined frame for all effects:
  `[part, effectIdx, R, G, B, period hi, period lo, period hi, period lo,
  waveVariation, 0x64, period hi, storage, 0, 0, 0]`, which is the union of
  the Fixed/Breathing (offsets 0-4), Cycling (5-7) and Wave (6-9) layouts.

**[hw] G502 HERO** readback of the factory cycle effect:
`8070 e 00 -> 00 03 00 00 00 00 00 1f 40 ...`: zone 0, effect index 3,
params `00 00 00 00 00 1f 40 00 00 00`, so period 0x1F40 = 8000 ms at offsets
5-6, matching the Cycling row. The ROM onboard profile LED blocks hold the
same bytes with the effect ID instead of the index: `03 00 00 00 00 00 1f 40
00 00 00` (cycle 8000 ms), `0a 00 dc ff 1f 40` (breathe #00dcff 8000 ms),
`01 ff ff ff` (static white).

Code snippet (Cycling build, 0-based data[] includes the 2 index bytes):

```cpp
// OpenRGB, Controllers/LogitechController/LogitechHIDPP20Controller/LogitechHIDPP20Controller.cpp
// https://gitlab.com/CalcProgrammer1/OpenRGB (GPL-2.0-or-later), SetZoneEffect()
case 0x0003:    // Color Cycle / Spectrum
    data[7] = (period >> 8) & 0xFF;   // param offset 5
    data[8] = period & 0xFF;          // param offset 6
    data[9] = brightness;             // param offset 7
```

## [4] getNvConfig(nvCapability) / [5] setNvConfig

getNvConfig request `[capMSB, capLSB]`, reply **[spec]**:

| Byte | Field |
|---|---|
| 0-1 | nvCapability echo |
| 2 | capabilityState |
| 3 | param1 |
| 4 | param2 |

setNvConfig request: `[capMSB, capLSB, state, param1, param2]` **[spec]**.

| nvCapability | Name | State values **[spec]** | Comment |
|---|---|---|---|
| 0x0001 | bootUpEffect | 0 no change (enabled assumed), 1 enabled, 2 disabled | when off, no effect on power reset |
| 0x0002 | demo | same | point-of-purchase demo firmware, no HID/HID++ |
| 0x0004 | userDemoMode | same, "on start-up" | demo stops as soon as an effect is set |

Only one capability bit per request **[spec]**. 0x8071 adds more capabilities
(see that file).

## [6] getLedBinInfo / [15] setLedBinInfo

Request `[zoneIndex, ledBinIndex]`. Reply `[zone, binIdx, red u16, green
u16, blue u16, white u16]` **[spec]**. ledBinIndex: 0 bin brightness, 1 bin
color, 2 calibration factors, 3 brightness, 4 colorimetric X, 5 colorimetric
Y. White is only meaningful for colorimetric requests. setLedBinInfo uses the
same 10 bytes as request and echoes them in the reply. These are factory
calibration values; logirgb should never write them.

## [7] getSwControl / [8] setSwControl

| fn | Request | Reply **[spec]** |
|---|---|---|
| 7 | none | `[swControl, syncEvents]` |
| 8 | `[swControl, syncEvents]` | none |

swControl: `0x00` firmware owns all LEDs, `0x01` software owns all LEDs.
"SW takes control of the color and the effect. No memorization is done in
EEPROM." syncEvents: `0x01` the device sends syncEffectEvent once per effect
period **[spec]**.

Tool behavior:

- **[OpenRGB]** unified controller: zone-only 0x8070 devices are claimed with
  `SetSWControl(1,1)` = `[01 01]` and released with `[00 00]`, "the official
  app and Solaar both do exactly this". For per-key keyboards with 0x8080
  (G810 family) it skips the claim, because "a partial SetSWControl claim
  breaks per-key on the G810; the full sequence is host-mode ->
  SetSWControl(1,1) -> SetZoneEffect off per zone -> per-key".
- **[OpenRGB]** `LogitechGLightsyncController::SetDirectMode` sends a short
  report with function byte `0x8A` (fn8) and `[01 01]` for direct mode,
  `[00 00]` otherwise.
- **[Solaar]** `LEDControl` setting reads fn7 (`0x70`) and writes fn8
  (`0x80`) with a boolean (1 = Solaar, 0 = device).

**[hw] G502 HERO:** `8070 7 -> 00` (firmware). `8070 8 01` is accepted, but
the firmware still overwrites the zones when a profile is loaded. fn8 is not
needed for setZoneEffect to take effect on this mouse.

## [9] getEffectSettings(zoneIndex, persistence)

Request `[zoneIndex, source]`, source 0 = RAM, 1 = EEPROM. Reply **[spec]**:
`[zone, R, G, B, period u16, brightness (0-100), effectParam]`. Not
supported when extCaps bit 0x0002 is set.

## [10] clearEffectSettings(zoneIndex)

Request `[zoneIndex]`. Clears the zone's settings, returns to default mode
**[spec]**.

## [11] setCyclingDirection(direction)

Request `[direction]`, `0x00` clockwise, `0x01` anticlockwise. Sending it at
random moments gives a random looking cycle **[spec]**.

## [12] getCurrentColor(zoneIndex)

Request `[zoneIndex]`, reply `[zone, R, G, B]`, the instantaneous color
**[spec]**. **[OpenRGB]** reads fn14 and fn12 at trace log level to dump the
running onboard effect.

## [13] synchronizeEffect(zoneIndex, driftValue)

Request **[spec]**: byte 0 zoneIndex (`0xFF` = all zones), byte 1 reserved
(the spec table skips it), bytes 2-3 driftValue (signed i16, ms). Only valid
when syncEvents are on. The device speeds up or slows down to cancel the
drift. Equivalent to 0x8071 fn6 setEffectSyncCorrection.

## [14] getZoneEffect(zoneIndex, persistence)

Request `[zoneIndex, source]`, source 0 = active RAM effect (default), 1 =
saved EEPROM effect. Reply: `[zone, zoneEffectIndex, param1..10]` **[spec]**.
Requires extCaps bit 0x0001 (libratbag checks this before calling)
**[libratbag]**. **[hw]** works on the G502 HERO, see the readback above.

## Events

| Event | Payload **[spec]** |
|---|---|
| [0] syncEffectEvent | byte 0 zoneIndex (`0xFF` all), bytes 1-2 effectCounter (u16, ms into the current period) |

Sent once per period when syncEvents is enabled. Events arrive with software
ID 0 in the low nibble of the function byte.

## Version history **[spec]**

- v0 initial.
- v1 period effects and synchronization.
- v2 more GetInfo capabilities, persistence byte on setZoneEffect, new
  getZoneEffect.
- v3 persistence byte on getZoneEffect request.
- v5 more zones (primary_1..6) "for Finch".
- v6 global sync flag in extCapabilities.

Observed feature versions: OpenRGB has seen 0x8070 v0, v3, v5 **[OpenRGB]**;
Solaar device dumps show v3-v5 on headsets and v4 on a G502 **[Solaar device dumps]**.

## Practical recipe (G502 HERO class mouse)

1. Find the feature index with root fn0 `[0x80, 0x70]`.
2. fn0 for zone count; fn1 per zone for location; fn2 per (zone, index) to map
   effect IDs to indices.
3. Write with fn3 `[zone, effectIndex, params10, 0x00]` (RAM only). No
   software control claim is required **[hw]**.
4. To survive profile switches and power cycles, write the effect into the
   onboard profile LED blocks via 0x8100 instead (logirgb does this).

## Sources

- Logitech, `x8070_colorledeffect_v7.pdf`, HID++ 2.0 feature document,
  Version 7. Folder: https://drive.google.com/drive/folders/0BxbRzx7vEV7eWmgwazJ3NUFfQ28 ,
  file: https://drive.google.com/file/d/1vdRwDOmaDpHwusJoDsGxe2Q4XBmNr1bb/view
- Logitech cpg-docs HID++ 2.0 feature list:
  https://github.com/Logitech/cpg-docs/tree/master/hidpp20
- Solaar, `lib/logitech_receiver/hidpp20.py` (LEDEffects, LEDZoneInfo,
  LEDEffectsInfo), `settings_templates.py` (LEDControl, LEDZoneSetting),
  `hidpp20_constants.py`: https://github.com/pwr-Solaar/Solaar
- OpenRGB, `Controllers/LogitechController/LogitechHIDPP20Controller/LogitechHIDPP20Controller.cpp`,
  `LogitechProtocolCommon.h`, `LogitechGLightsyncController/LogitechGLightsyncController.cpp`:
  https://gitlab.com/CalcProgrammer1/OpenRGB
- libratbag, `src/hidpp20.c`, `src/hidpp20.h`, `src/driver-hidpp20.c`:
  https://github.com/libratbag/libratbag
- g810-led, `src/classes/Keyboard.cpp`:
  https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.cpp
- OpenLogi HID++ reference, 0x8070:
  https://openlogi.org/hidpp/features/x8070-color-led-effects
- Linux kernel patch "HID: logitech-hidpp: support Color LED feature (8071)",
  M. Schönlaub, 2022 (not merged): https://lkml.org/lkml/2022/3/8/1453
- Linux kernel `drivers/hid/hid-logitech-hidpp.c` (HID++ 2.0 error codes):
  https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/drivers/hid/hid-logitech-hidpp.c
- libratbag issue 735 (G703 uses 0x8071, not 0x8070):
  https://github.com/libratbag/libratbag/issues/735
- logirgb hardware log: `research/hardware-log.md` in this repository.
