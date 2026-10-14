# HID++ 2.0 feature 0x8071 RGBEffects

Research notes for logirgb. 0x8071 is the successor of 0x8070
(`8070-color-led-effects.md`). It is used by current G keyboards (G515,
G915/G815, G Pro), newer mice (G502 X PLUS, G703 HERO, G Pro Wireless) and,
with the same function layout under ID 0x0600, by Centurion headsets. Besides
firmware effects it arbitrates host versus firmware control, handles RGB
power modes (dim and off on idle), user activity events, non-volatile boot
and shutdown effects, and a shutdown trigger.

## Provenance markers

| Marker | Meaning |
|---|---|
| **[spec]** | Logitech's published `x8071_rgbeffects_v4.pdf` (Version 4) from the public "hidpp20 public" Google Drive folder. |
| **[hw]** | Verified on hardware (G515 TKL wired 046d:c358 / G502 HERO 046d:c08b). For this feature only the G515 TKL applies; the G502 HERO has 0x8070. |
| **[Solaar]**, **[OpenRGB]**, **[libratbag]**, **[openlogi]**, **[g810-led]**, **[OpenRGB G915]** | Read from that tool's source code or documentation (OpenRGB G915 = the older per-model `LogitechG915Controller`). |
| **[unverified]** | Inferred, contradictory between sources, or not tested by us. |

## Wire basics

Long report `11 <devIdx> <featIdx> <fn<<4 | swid> <16 bytes>`. All multi
byte fields are big endian **[spec]**. The function byte on the wire is
`fn<<4 | swid`; events from the device carry software ID 0 **[hw]**. Error
replies are `11 <dev> ff <featIdx> <fnByte> <code>`. Relevant codes:
`0x02` INVALID_ARGUMENT (unsupported parameter, for example a cluster or
effect index that does not exist **[hw]**), `0x05` NOT_ALLOWED (the spec
name; Solaar calls 0x05 LOGITECH_ERROR), `0x08` BUSY.

Several functions are "manage" functions whose first byte is `getOrSet`:
`0x00` get, `0x01` set (fn4 also has `0x02` get backup) **[spec]**. Replies
echo the getOrSet byte at offset 0, so the data of a get reply starts at
byte 1.

## Function table

| fn | Wire byte (swid 0xA) | Name **[spec]** | Purpose |
|---|---|---|---|
| 0 | `0x0A` | getInfo(clusterIdx, effectIdx, typeOfInfo, param1..13) | device, cluster, effect and onboard-effect info |
| 1 | `0x1A` | setRgbClusterEffect(cluster, effectIdx, param1..10, persistence, powerMode) | set an effect |
| 2 | `0x2A` | setMultiLedRgbClusterPattern(cluster, pattern) | on/off pattern for multi LED clusters (DPI LEDs) |
| 3 | `0x3A` | manageNvConfig(getOrSet, nvCapability, state, param1..6) | boot/shutdown effects, ramp to off, etc |
| 4 | `0x4A` | manageRgbLedBinInfo(getOrSet, cluster, binIdx, param1..8) | factory LED binning |
| 5 | `0x5A` | manageSwControl(getOrSet, swControlFlags, eventsNotificationFlags) | host versus firmware ownership and event subscription |
| 6 | `0x6A` | setEffectSyncCorrection(cluster, driftValue) | effect phase sync |
| 7 | `0x7A` | manageRgbPowerModeConfig(getOrSet, flags, toPowerSave, toOff) | idle and off timers |
| 8 | `0x8A` | manageRgbPowerMode(getOrSet, rgbPowerMode) | force full / power save / off |
| 9 | `0x9A` | shutdown() | reset SW control, play shutdown effect, deep sleep (v4) |

Events **[spec]**: `[0] effectSyncEvent`, `[1] userActivityEvent`,
`[2] rgbClusterChangedEvent`.

Names used by tools for the same functions:

| fn | libratbag `CMD_RGB_EFFECTS_*` | OpenRGB `FN_8071_*` | Solaar usage |
|---|---|---|---|
| 0 | GET_INFO | GET_INFO | `RGBEffectsInfo`, `_probe_tmpl_bytes` |
| 1 | SET_RGB_CLUSTER_EFFECT | SET_EFFECT | `RGBEffectSetting` write (0x10) |
| 2 | SET_MULTI_LED_RGB_CLUSTER_PATTERN | SET_PATTERN | not used |
| 3 | MANAGE_NV_CONFIG | NV_CONFIG | `_RgbBootEffectSetting` (0x30) |
| 4 | MANAGE_RGB_LED_BIN_INFO | BIN_INFO | not used |
| 5 | MANAGE_SW_CONTROL | SW_CONTROL | `RGBControl` (0x50) |
| 6 | SET_EFFECT_SYNC_CORRECTION | SYNC | not used |
| 7 | MANAGE_RGB_POWER_MODE_CONFIG | PWR_CONFIG | `_read_firmware_timers` (0x70) |
| 8 | MANAGE_RGB_POWER_MODE | PWR_MODE | `_start_sleep`, `_set_power_mode_with_retry` (0x80) |
| 9 | (not defined) | `LOGITECH_FP8071_SHUTDOWN 0x90` in LogitechProtocolCommon.h | `cleanup()` sends 0x90 with `01 00` |

## [0] getInfo

Request: `[rgbClusterIndex, rgbClusterEffectIndex, typeOfInfo, param1..13]`
**[spec]**. The first two bytes select what is described:

| clusterIdx | effectIdx | typeOfInfo | Describes |
|---|---|---|---|
| 0xFF | 0xFF | 0x00 | the device |
| n | 0xFF | 0x00 | cluster n |
| n (or 0xFF for multi cluster effects) | e | 0x00 | effect e of cluster n |
| 0xFF | onboard stored effect index | 0x01 | an onboard stored (custom) effect slot, param1 = slot, param2 = SlotInfoType |

### Device info `[FF FF 00]`

| Reply byte | Field **[spec]** |
|---|---|
| 0 | rgbClusterIndex (0xFF) |
| 1 | rgbClusterEffectIndex |
| 2 | rgbClusterCount |
| 3-4 | nvCapabilities |
| 5-6 | extCapabilities |
| 7 | NumberOfMulticlusterEffects (effects that can run on cluster 0xFF) |

extCapabilities **[spec]**:

| Bit | Meaning |
|---|---|
| 0x0001 | supports "getZoneEffect" (a 0x8070 leftover; there is no such function in 0x8071) |
| 0x0002 | reserved, ignore, always set to 1 in new implementations |
| 0x0004 | supports set of LED bin info |
| 0x0008 | monochrome only |
| 0x0010 | does NOT support setEffectSyncCorrection nor effectSyncEvent |
| 0x0020 | supports shutdown() (fn9) |
| 0x0040 | sends rgbClusterChangedEvent |

nvCapabilities: the nvCapability IDs of manageNvConfig (table below).

**[hw] G515 TKL:** `8071 0 ff ff 00 -> ff 00 01 00 63 00 26 06`:

- 1 cluster.
- nvCapabilities `0x0063` = 0x0001 bootUpEffect, 0x0002 demo, 0x0020
  rampDownToOff, 0x0040 shutdownEffect.
- extCapabilities `0x0026` = 0x0002 reserved, 0x0004 setLedBinInfo, 0x0020
  shutdown(). Bit 0x0010 is clear, so effect sync is supported; bit 0x0040 is
  clear, so no rgbClusterChangedEvent.
- 6 multi cluster effects.

Tool interpretations of the same reply differ:

- **[libratbag]** `hidpp20_rgb_effects_get_device_info` reads
  `nv_caps = be16(&parameters[3])` and `ext_caps = be16(&parameters[4])`.
  The second offset is a bug (overlaps nv_caps); it should be 5.
- **[OpenRGB]** logs bytes 3-4 as "extCaps", 5-6 as "effectBlocks" and 7 as
  "supportedClusterIdx", which does not match the spec. Its
  `LOGITECH_FP8071_FLAGS` enum (0x01 get status, 0x04 set bin info, 0x08
  monochrome, 0x10 no sync, 0x20 supports shutdown, 0x40 cluster changed)
  matches the spec extCapabilities.
- **[Solaar]** `RGBEffectsInfo` unpacks `"!BBBHH"` and uses
  `capabilities & 0x1` of the third field (extCaps) as "readable".

### Cluster info `[n FF 00]`

| Reply byte | Field **[spec]** |
|---|---|
| 0 | rgbClusterIndex |
| 1 | rgbClusterEffectIndex |
| 2-3 | locationEffect (u16) |
| 4 | effectsNumber |
| 5 | displayPersistencyCapabilities: 0x00 none, 0x01 event display always on, 0x02 always off, 0x04 on then off |
| 6 | effectPersistencyCapabilities: 0x00 none, 0x01 effect persistence in EEPROM supported |
| 7 | multiledPatternCapabilities: 0x00 no, 0x01 setMultiLedRgbClusterPattern supported |

Location codes **[spec]**:

| Value | Location |
|---|---|
| 0x01 | primary (for example the keys of a keyboard) |
| 0x02 | logo |
| 0x03 | left side |
| 0x04 | right side |
| 0x05 | combined |
| 0x06-0x0B | zonal one .. zonal six |
| 0x81 | idle state (state IDs for devices that multiplex one LED, for example microphones) |
| 0x82 | in-use state |
| 0x83 | muted state |
| 0x84 | soft muted state (Zoom/Teams mute) |
| 0xFF | all |

**[hw] G515 TKL:** `8071 0 00 ff 00 -> 00 00 00 01 06 ...`: cluster 0,
location 0x0001 primary, 6 effects. `8071 0 01 ff 00` returns error 0x02
(no cluster 1).

**[libratbag]** reads `num_effects = parameters[4]` and
`persistency_caps = parameters[5]` (its struct comment declares u16 fields,
but the parser reads single bytes, matching the spec). **[OpenRGB]** reads
location at 2-3 and count at 4.

### Effect info `[n e 00]`

| Reply byte | Field **[spec]** |
|---|---|
| 0 | rgbClusterIndex |
| 1 | rgbClusterEffectIndex |
| 2-3 | effectId |
| 4-5 | effectCapabilities |
| 6-7 | effectPeriod (ms, 0 if not available) |

**[hw] G515 TKL** (cluster 0):

| Index | Raw reply | effectId | caps | period |
|---|---|---|---|---|
| 0 | `00 00 00 00 ...` | 0x0000 Disabled | 0x0000 | 0 |
| 1 | `00 01 00 01 00 00 00 00` | 0x0001 Fixed | 0x0000 | 0 |
| 2 | `00 02 00 0a c0 01 1e 0b` | 0x000A Breathing | 0xC001 | 0x1E0B = 7691 ms |
| 3 | `00 03 00 15 c0 05 03 e0` | 0x0015 Cycling (saturation) | 0xC005 | 0x03E0 = 992 ms |
| 4 | `00 04 00 16 dc e1 00 1e` | 0x0016 Wave (saturation) | 0xDCE1 | 0x001E = 30 ms |
| 5 | `00 05 00 17 00 00 00 32` | 0x0017 Ripple (saturation) | 0x0000 | 0x0032 = 50 ms |
| 6 | error 0x02 | | | |

Decoding the capability words with the spec tables (the saturation variants
are assumed to reuse the bits of their base effect **[unverified]**):

- Breathing 0xC001: supported, sync capable, 16 bit period. No speed,
  intensity or waveform bits, so the firmware may ignore waveform and
  intensity.
- Cycling 0xC005: supported, intensity, sync, period.
- Wave 0xDCE1: supported, horizontal (1), vertical (2), center-out (3),
  reverse horizontal (6), reverse vertical (7), center-in (8), sync, period.
  Inward (0x0100) and outward (0x0200) are absent. This matches Solaar's
  `LedDirectionBlocklist = {"40B4": {4, 5}}` ("G515 LS TKL, no edge-radiating
  wave geometry") and OpenRGB's comment that the G515 uses 6 of 8 directions,
  so the capability bits can drive the direction list instead of a per model
  blocklist.
- Ripple 0x0000: "Raptor/FlyingV compatibility assumed".

Note the G515 lists Disabled (0x0000) although it has a 0x8040 brightness
key; the spec recommends that devices with x8040 hw_onoff do not implement
Disabled, but the G515 does not advertise hw_onoff (see
`8040-brightness-control.md`).

### effectCapabilities per effect **[spec]**

Bit 0x0001 must be set for any other bit to count; 0 means legacy (Raptor)
defaults.

| Effect | Bits |
|---|---|
| 0x01 Fixed | 0x0002 ramp up/down, 0x0004 no effect |
| 0x03 Cycling | 0x0002 speed, 0x0004 intensity, 0x4000 sync, 0x8000 period |
| 0x04 Wave | 0x0002 startColor, 0x0004 stopColor, 0x0008 speed, 0x0010 intensity, 0x0020 horizontal, 0x0040 vertical, 0x0080 center-out, 0x0100 inward, 0x0200 outward, 0x0400 reverse horizontal, 0x0800 reverse vertical, 0x1000 center-in, 0x4000 sync, 0x8000 period |
| 0x0A Breathing | 0x0002 speed, 0x0004 intensity, 0x0008 sine, 0x0010 square, 0x0020 triangle, 0x0040 sawtooth, 0x0080 shark fin, 0x0100 exponential, 0x4000 sync, 0x8000 period |
| 0x0C Custom onboard | 0x0002 several slots, 0x0004 intensity, 0x0008 location in slot, 0x0010 frame period, 0x0020 start at any frame, 0x0040 stop at any frame, 0x4000 sync |
| 0x18 "Smooth Star Breathing" | 0x0002 speed, 0x0004 intensity, 0x0008 sync, 0x0010 frame period |
| 0x19 "Smooth Wave" | same as 0x18 |

(The spec labels 0x18/0x19 "Smooth Star Breathing" and "Smooth Wave ... for
SheafHER product" in the capability section, and "Formula-based Signature
effect (Active/Passive)" in the effect table. Both names refer to the same
IDs.)

### Onboard stored effect info `[FF idx 01 slot SlotInfoType ...]` (long request)

Reply bytes 3-15 depend on SlotInfoType **[spec]**:

| SlotInfoType | Reply params |
|---|---|
| 0x00 SlotState | slot, 0x00, dataValidity, lengthFrames hi, lo |
| 0x01 Defaults | slot, 0x01, initFrameToPlay hi, lo, lengthFrames hi, lo, framePeriod_ms hi, lo, intensity |
| 0x02 / 0x03 | UUID bytes 0-10 / 11-16 |
| 0x04 / 0x05 / 0x06 | effect name characters 0-10 / 11-21 / 22-31 |

An absent slot returns INVALID_ARGUMENT **[spec]**.

**[OpenRGB]** `DiscoverEffectCards` sends `[FF 00 01 00 01]` ("GetEffectSpecificInfo,
page 1") on the G502 X PLUS and copies reply bytes 10-11 ("template bytes",
`20 64` on that mouse) into a per-key prep write. In spec terms this is
SlotInfoType 0x01 Defaults of onboard effect 0, slot 0, and the copied bytes
are framePeriod_ms (0x0020 = 32 ms) low byte and intensity 0x64 = 100
**[unverified interpretation]**. Solaar has the same probe
(`_probe_tmpl_bytes`, request `FF 00 01 00 01`) but leaves the call that
uses it commented out **[Solaar]**.

## [1] setRgbClusterEffect

Long request **[spec]**:

| Byte | Field |
|---|---|
| 0 | rgbClusterIndex (0xFF = apply a multi cluster effect to all clusters) |
| 1 | rgbClusterEffectIndex (index from getInfo, NOT the effect ID) |
| 2-11 | param1..param10 |
| 12 | bits 0-1 persistence, bits 2-3 powerMode, bits 4-7 reserved |
| 13-15 | reserved |

Persistence (bit field) **[spec]**: bit 0 = volatile (RAM), bit 1 =
non-volatile (EEPROM). `0x03` updates both. EEPROM support is advertised by
effectPersistencyCapabilities.

powerMode **[spec]**: 0 = full power mode, 1 = power save mode. "In case the
current RGB powermode matches the powermode in the request, the effect shall
be launched immediately. Otherwise it will be launched at next RGB power mode
change." So byte 12 = `0x05` would stage a RAM effect for the dimmed (power
save) state **[unverified on hw]**.

Errors **[spec]**: INVALID_ARGUMENT for unsupported parameters, NOT_ALLOWED
(0x05) "if the enableSWControlOfAllRgbClusters is not enabled".

**[hw] G515 TKL:**

- Without software control the write returns error `0x05`, exactly the spec's
  NOT_ALLOWED.
- After `8101 6 05` (0x8101 ProfileManagement fn6 mode 5, host) and
  `8071 5 01 03 04` the write `8071 1 [cluster, effectIndex, params10, 01]`
  is accepted and shown. Confirmed visible: static green
  `8071 1 00 01 00 ff 00 02 00 00 00 00 00 00 01`, and breathe blue with a
  3000 ms period (color at params 0-2, period at 3-4, intensity at 6).
- Byte 12 = 0x01 means RAM only, full power mode.

Tool usage:

- **[Solaar]** `RGBEffectSetting.build` uses `setup(device, None, 0x10, b"\x01")`:
  fn1 with suffix `01`, no read function.
- **[OpenRGB]** `SetZoneEffect` writes byte 12 = `0x01` only on Save and
  `0x00` for live painting (comment: "Only a Save sets it, so live painting
  stays volatile"). By the spec, `0x00` means neither RAM nor EEPROM; the
  hardware accepted `0x01` for live use. **[unverified]** whether `0x00` is
  treated as RAM by firmware.
- **[OpenRGB G915]** `SendMode` writes `0x1E` (fn1, swid 0xE) with byte 12 =
  `0x01`. On the G915 the keyboard is cluster 1 and the logo cluster 0; the
  effect indices are 0 off, 1 static, 2 breathing, 3 cycle, 4 wave, 5 ripple
  for the keys and 0 off, 1 static, 2 cycle, 3 breathing for the logo (these
  are indices, which is why breathing is 2 on one cluster and 3 on the other).

### Effect IDs **[spec]**

| ID | Name | In G515 list **[hw]** |
|---|---|---|
| 0x00 | Disabled | idx 0 |
| 0x01 | Fixed | idx 1 |
| 0x02 | Pulsing/Breathing (legacy) | |
| 0x03 | Color Cycling | |
| 0x04 | Color Wave | |
| 0x05 | Starlight | |
| 0x06 | Light on Press | |
| 0x07 | Audio Visualizer (TBD) | |
| 0x08 | Boot Up | |
| 0x09 | Demo Mode | |
| 0x0A | Pulsing/Breathing (Waveform) | idx 2 |
| 0x0B | Ripple | |
| 0x0C | Custom Onboard Stored | |
| 0x0D | Kitt lighting | |
| 0x0E | Color decomposition | |
| 0x0F | Frame-based Signature effect (Active) | |
| 0x10 | Frame-based Signature effect (Passive) | |
| 0x11, 0x12 | not used | |
| 0x13 | Host streaming | |
| 0x14 | HSV Pulsing/Breathing | |
| 0x15 | Color Cycling Configurable S(aturation) | idx 3 |
| 0x16 | Color Wave Configurable S | idx 4 |
| 0x17 | Ripple Configurable S | idx 5 |
| 0x18 | Formula-based Signature effect (Active) | in onboard profile |
| 0x19 | Formula-based Signature effect (Passive) | in onboard profile |

### 10 byte parameter layouts **[spec]**

Offsets are 0-based (spec param1 = offset 0). Add 2 for the request offset,
6 for the offset in the 20 byte report. "intensity" is 0 = default (100),
valid 1-100, above 100 = default.

| ID | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 |
|---|---|---|---|---|---|---|---|---|---|---|
| 0x01 Fixed | R | G | B | 0 default, 1 ramp, 2 no effect | | | | | | |
| 0x02 Pulsing | R | G | B | period (ms) | | | | | | |
| 0x03 Cycling | | | | | | period MSB | period LSB | intensity | | |
| 0x04 Wave | R start | G start | B start | R stop | G stop | B stop | period LSB | direction | intensity | period MSB |
| 0x05 Starlight | R sky | G sky | B sky | R star | G star | B star | | | | |
| 0x06 Light on press | R press | G press | B press | R rel | G rel | B rel | delay MSB | delay LSB | | |
| 0x0A Breathing | R | G | B | period MSB | period LSB | waveform | intensity | | | |
| 0x0B Ripple | R | G | B | 0 default, 1 skip startup animation | period MSB | period LSB | | | | |
| 0x0C Custom onboard | slot | init frame MSB | init frame LSB | length MSB (frames) | length LSB | frame period MSB (ms/frame) | frame period LSB | intensity | | |
| 0x0D Kitt | fg R | fg G | fg B | LED overlap % | period MSB | period LSB | bg R | bg G | bg B | brightness |
| 0x0E Color decomposition | | | | | | | period LSB | period MSB | brightness | |
| 0x0F / 0x10 Frame signature | | | | | | frame period MSB | frame period LSB | intensity | | |
| 0x14 HSV pulsing | hue | saturation | value | | | | period MSB | period LSB | | |
| 0x15 Cycling S | | saturation | | | | | period MSB | period LSB | intensity | |
| 0x16 Wave S | | saturation | | | | | period MSB | period LSB | intensity | direction |
| 0x17 Ripple S | R | G | B | saturation | 0 default, 1 skip startup | | period MSB | period LSB | | |
| 0x18 / 0x19 Formula signature | R1 | G1 | B1 | R2 | G2 | B2 | period MSB | period LSB | intensity | |

Direction (0x04, 0x16) **[spec]**: 0 default (all play), 1 horizontal,
2 vertical, 3 center-out, 4 inward, 5 outward, 6 reverse horizontal,
7 reverse vertical, 8 center-in. Solaar's labels: 0 Cycle, 1 Right, 2 Down,
3 Center Out, 4 In, 5 Out, 6 Left, 7 Up, 8 Center In **[Solaar]**.

Waveform (0x0A) **[spec]**: 0 default, 1 sine, 2 square, 3 triangle,
4 sawtooth, 5 shark fin, 6 exponential.

Cross-check against tools:

| ID | Solaar `LEDEffects` offsets | OpenRGB `SetZoneEffect` (param offsets) | Agrees with spec |
|---|---|---|---|
| 0x01 | color 0, "ramp" 3 | RGB 0-2, `0x02` at 3 unless black | yes |
| 0x02 | color 0, speed 3 | not handled | yes |
| 0x03 | period 5, intensity 7 | period 5-6, intensity 7 | yes |
| 0x04 | period 6-7, direction 9 (comment: "assume the 0x16 layout") | period LSB 6, direction 7, intensity 8, period MSB 9 | OpenRGB yes, Solaar no |
| 0x0A | color 0, period 3, form 5, intensity 6 | RGB 0-2, period 3-4, intensity 6 | yes |
| 0x0B | color 0, period 4 | RGB 0-2, period 4-5 | yes |
| 0x0E | period 6, intensity 8 (big endian) | period BE 6-7, intensity 8 | byte order differs: the spec table puts the LSB at offset 6 **[unverified]** |
| 0x0F, 0x10 | period 5, intensity 7 | period 5-6, intensity 7 | yes |
| 0x15 | saturation 1, period 6, intensity 8 | saturation 1 = 0xFF, period 6-7, intensity 8 | yes |
| 0x16 | saturation 1, period 6, intensity 8, direction 9 | same | yes |
| 0x17 | color 0, saturation 3, period 6 | RGB, saturation 3 = 0xFF, period 6-7 | yes (byte 4 skip-startup left 0) |

Solaar note: 0x8071 "firmware-fixes ramp/form bytes; drop those widgets
here" (`RGBEffectSetting.possible_fields`), and for static it forces offset 3
to `0x02` **[Solaar]**. **[hw]** static with offset 3 = `0x02` works on the
G515.

Ripple period range: Solaar and the old OpenRGB G915 driver use 2-200 ms
with default 20 for ripple (`LOGITECH_G915_SPEED_RIPPLE_*`), while the G515
reports a 50 ms default period for 0x17 **[hw]**.

Code snippet, saturation wave:

```python
# Solaar, lib/logitech_receiver/hidpp20.py, LEDEffects
# https://github.com/pwr-Solaar/Solaar (GPL-2.0-or-later)
0x16: [
    NamedInt(0x16, _("Wave")),
    {LEDParam.saturation: 1, LEDParam.period: 6, LEDParam.intensity: 8, LEDParam.direction: 9},
    {LEDParam.saturation: 255, LEDParam.period: 5000, LEDParam.intensity: 100},
],
```

### Effects 0x18 and 0x19 in the G515 onboard profile **[hw]**

The G515 factory profile (0x8101 file `0x0101`, 99 bytes) holds four 11 byte
lighting records, each `[effectId, params10]`:

```
rec0 18 ff 00 81 80 aa ff 00 00 64 00   0x18, color1 #ff0081, color2 #80aaff, period 0, intensity 100
rec1 00 00 00 00 00 00 00 00 00 00 00
rec2 19 ff 00 81 80 aa ff 00 00 64 00   0x19, same colors
rec3 00 00 00 00 00 00 00 00 00 00 00
flags 03
```

Decoded with the spec layout for 0x18/0x19 (two colors, period at 6-7,
intensity at 8). The colors equal the documented default of the NvConfig
boot and shutdown effects (Solaar: "Device default: Primary #FF0081,
Secondary #80AAFF") **[Solaar]**, so this is the G515's "signature" look.
0x18 is the Active variant and 0x19 the Passive variant. A user profile
written by logirgb uses rec1 = active static color, rec3 = dimmed copy, rec0
and rec2 zero, flags `00`; the firmware then dims to rec3 after the idle
timeout **[hw]**. A plausible reading is that rec0/rec1 belong to full power
mode and rec2/rec3 to power save mode, with the flags byte choosing between
the signature records (0/2) and the parameter records (1/3)
**[unverified]**. 0x18 and 0x19 are not in the G515's getInfo effect list,
so they cannot be selected through setRgbClusterEffect by index.

## [2] setMultiLedRgbClusterPattern(cluster, pattern)

Request `[cluster, pattern]` **[spec]**. Pattern is a device specific code
for which LEDs of a multi LED cluster (DPI LEDs, battery LEDs) are on:
`0xFF` all on, `0x00` all off. NOT_ALLOWED if the cluster does not advertise
multiledPatternCapabilities.

## [3] manageNvConfig

Request **[spec]**:

| Byte | Field |
|---|---|
| 0 | getOrSet (0 get, 1 set) |
| 1-2 | nvCapability (one bit only) |
| 3 | capabilityState (set only) |
| 4-9 | param1..param6 |

The spec request table only lists param1-2, but the capability table uses
param1-6 for the color capabilities; Solaar writes 7 bytes after the
capability (`[state, R1, G1, B1, R2, G2, B2]`) and reads reply bytes 3-9
**[Solaar]**.

Reply: `[getOrSet, capHi, capLo, state, param1..6]`.

Capabilities **[spec]** (state: 0 no change (enabled assumed), 1 enabled, 2
disabled, unless noted):

| nvCapability | Name | Params |
|---|---|---|
| 0x0001 | bootUpEffect | param1-3 first color, param4-6 second color. "When off, the effect does not play on power reset" |
| 0x0002 | demo | unused; point-of-purchase demo firmware |
| 0x0004 | userDemoMode | unused; demo on start-up, stops when an effect is set |
| 0x0008 | eventsDisplay | states: 1 always enabled, 2 disabled, 4 enabled for timeout_s after an event (param1 = timeout_s, typically 7 s); for example DPI status display |
| 0x0010 | ActiveDimming | param1 intensity 0-100 (0 = default); dims LEDs hidden under the hand while in use |
| 0x0020 | RampDownToOFF | param1 fading time in seconds (default 30); LEDs fade out during the transition to power off |
| 0x0040 | ShutdownEffect | param1-3 first color, param4-6 second color; "when off, the effect does not play on power off" |

**[hw] G515 TKL** advertises 0x0001, 0x0002, 0x0020 and 0x0040
(nvCaps 0x0063).

Tool usage:

- **[Solaar]** `_RgbBootEffectSetting`: read `0x30` with `00 capHi capLo`,
  write `0x30` with `01 capHi capLo [enabled, R1, G1, B1, R2, G2, B2]`,
  enabled 0x01 on, 0x02 off. Settings `rgb_startup_animation` (0x0001) and
  `rgb_shutdown_animation` (0x0040) are gated by a per model allowlist in
  `device_quirks.py` because the write is persistent: G515 LIGHTSPEED TKL
  honors both colors for both caps; the G502 X PLUS honors only the enabled
  flag of 0x0001 and does not support 0x0040.
- **[OpenRGB]** `ReadNvSleepRampConfig` reads cap 0x0020 with `00 00 20`,
  checks the echo, takes `enabled = data[3] != 0`, `seconds = data[4]`, and
  notes "G515 default observed from vendor app: enabled=0x01, seconds=0x1E
  (= 30 seconds dim ramp before sleep)". It then starts its host sleep 30 s
  earlier so the firmware fade ends at the configured sleep time.

## [4] manageRgbLedBinInfo

Request **[spec]**: `[getOrSet, cluster, ledBinIndex, param1..8 (set only)]`,
getOrSet 0 get, 1 set, 2 get backup (stored by feature x1806). Reply
`[getOrSet, cluster, binIdx, param1..8]`. ledBinIndex: 0 bin brightness
(5 combination bytes), 1 bin color (2 bin codes), 2 calibration factors R G
B, 3 brightness (R, G, B intensity as u16), 4 colorimetric X (R, G, B, W u16),
5 colorimetric Y. Factory data; do not write.

## [5] manageSwControl

Request **[spec]**: `[getOrSet, swControlFlags, eventsNotificationFlags]`.
Reply: same three bytes.

swControlFlags **[spec]**:

| Bit | Name | Effect |
|---|---|---|
| 0x01 | enableSWControlOfAllRgbClusters | disables all firmware cluster handling; required for setRgbClusterEffect |
| 0x02 | enableSWControlOfPowerModes | disables firmware power mode handling; required for manageRgbPowerMode set |

eventsNotificationFlags **[spec]**:

| Bit | Name | Event |
|---|---|---|
| 0x01 | enableEffectsSyncNotif | effectSyncEvent once per effect period |
| 0x02 | enableUserActivityNotif | userActivityEvent type 1 on user activity |
| 0x04 | enableNoUserActivityTimeoutNotif | userActivityEvent type 0 when the no activity timeout is reached |

"User Activity/NoActivity notifications need to be enabled/disabled
independently, to avoid losing them during RF transmission issues. They are
only disabled by SW when the power mode has changed (i.e. the notification
has implicitly been acknowledged)." **[spec]**

So "mode" and "flags" in the hardware notes are swControlFlags and
eventsNotificationFlags:

| Wire `8071 5 01 MM FF` | Meaning |
|---|---|
| MM = 1 | host owns clusters, firmware owns power modes |
| MM = 2 | firmware owns clusters, host owns power modes |
| MM = 3 | host owns both |
| FF = 0x04 | report "idle timeout reached" |
| FF = 0x02 | report "user active" |
| FF = 0x05 | sync events + idle timeout |

**[hw] G515 TKL mode/flag matrix** (after `8101 6 05`; flags 0-7 were all
accepted by fn5 in every mode):

| swControlFlags | setRgbClusterEffect | What the keyboard shows |
|---|---|---|
| 0 (released) | error 0x05 | onboard lighting; firmware dims and sleeps |
| 1 (clusters) | accepted | onboard lighting stays visible, the host effect is **not** shown |
| 2 (power modes) | error 0x05 | onboard lighting |
| 3 (both) | accepted | host effect shown; firmware never dims or sleeps, with any events flags |

Modes 2 and 0 match the spec (NOT_ALLOWED without the cluster bit). Mode 3
never dimming also matches: with the power mode bit set the firmware only
reports timeouts and the host must call manageRgbPowerMode. Mode 1 being
accepted but invisible is not explained by the spec. A guess is that with
firmware owned power modes the displayed effect comes from the profile's
power mode records rather than the RAM cluster effect **[unverified]**.

**[hw] events in mode 3, flags 0x04**: the firmware idle timer was set to
15 s (fn7). Log:

```
14:24:53 connected
14:24:53 idle         spurious idle/active pair right after claiming
14:24:53 active
14:25:08 idle         15 s after the last key press
14:25:51 active       key press
```

Event frame: feature index of 0x8071, function 1, software ID 0,
`data[0] == 0` for idle (NoActivityTimeoutReached), non-zero for active
(UserActivityDetected). While idle, setting flags `0x02` makes the firmware
report the next activity. The spurious pair right after claiming must be
ignored or debounced.

Tool usage and naming confusion:

- **[Solaar]** `rgb_power.py` names the third byte's bits `FLAG_EFFECT 0x01,
  FLAG_POWER 0x02, FLAG_NVCONFIG 0x04`, which are really the event bits. Its
  constants are `SW_ACTIVE = [01 03 04]` ("firmware monitors idle"),
  `SW_IDLE = [01 03 02]` ("firmware monitors activity") and
  `SW_RELEASE = [01 00 00]`. The behavior is right even if the names are not.
  The claim also switches 0x8101 to mode 5 (or 0x8100 to host mode `02`);
  release restores 0x8101 mode 3 (or 0x8100 `01`). Solaar's docstring says
  "The firmware sends a burst of ~8 events with exponential backoff. Only the
  first event matters."
- **[OpenRGB]** claims with `SetSWControl(3, 6)` (both event kinds), then
  after the first complete per-key frame "upgrades" to `(3, 5)`; on idle it
  sends `(3, 3)`, on wake `(3, 5)`. Its comment says "SW Control flags bits
  (from a wire capture, the doc's overview mislabels them): bit 0 = effect
  control, bit 1 = power management, bit 2 = NV config", which is the same
  misreading as Solaar's; per the spec, bit 0 of the events byte is effect
  sync.
- **[OpenRGB G915]** `InitializeModeSet` sends `0x5E [01 03 07]` (host owns
  both, all events).
- **[g810-led]** sends `0x0f 0x5c [01 03 03]` on the G815 before setting
  effects.

Code snippet:

```python
# Solaar, lib/logitech_receiver/rgb_power.py
# https://github.com/pwr-Solaar/Solaar (GPL-2.0-or-later)
SW_ACTIVE = bytes([0x01, 0x03, FLAG_NVCONFIG])  # firmware monitors idle
SW_IDLE = bytes([0x01, 0x03, FLAG_POWER])  # firmware monitors activity
SW_RELEASE = bytes([0x01, 0x00, 0x00])
```

## [6] setEffectSyncCorrection(cluster, driftValue)

Long request **[spec]**: byte 0 cluster (`0xFF` all), byte 1 reserved, bytes
2-3 driftValue (signed i16, ms). "Device will speed-up or slow down the
current effect play to minimise the driftValue reported by SW." Only active
while effect sync events are enabled. Not used by any tool we looked at.

## [7] manageRgbPowerModeConfig

Request **[spec]**:

| Byte | Field |
|---|---|
| 0 | getOrSet |
| 1-2 | rgbPowerModeFlags (no flags defined yet) |
| 3-4 | rgbNoActTimeoutToPSave (s) |
| 5-6 | rgbNoActTimeoutToOff (s) |

Reply: same layout. Semantics **[spec]**:

- ToPSave: seconds since the last activity until power save. With firmware
  owned power modes the device switches to power save; with host owned power
  modes it sends a notification instead.
- ToOff: seconds "without any user activity since entering Power Save mode
  until Power Off". `0xFFFF` = infinite for both.

**[hw] G515 TKL:**

```
8071 7 00                    -> 00 00 00 00 3c 01 2c    idle 60 s, sleep 300 s
8071 7 01 00 00 00 0a 00 19  -> 01 ...                  idle 10 s, sleep 25 s
```

With software control released and the onboard profile driving the
lighting, the firmware dimmed after 10 s, switched off after 25 s (counted
the way the firmware does it) and woke on a key press. In mode 3 it did
neither, but it still sent the idle event at the ToPSave timeout. Whether the
"off" timer counts from power save entry (spec wording) or from the last
activity was not measured precisely **[unverified]**; Solaar and OpenRGB
both schedule host sleep at `sleep - idle` seconds after the idle event,
which assumes the second value is an absolute time since activity.

The G515 onboard profile also stores the two timers at profile bytes 2-5
(`003c 012c`), so a saved profile restores them after a power cycle **[hw]**.
OpenRGB notes the fn7 values "reset on power cycle but persist across SW
control release/reclaim" **[OpenRGB]**.

Tool usage: **[Solaar]** reads with `[00]` and parses bytes 3-4 and 5-6;
writes `[01 00 00 idleHi idleLo sleepHi sleepLo]`. **[OpenRGB]** same, as a
16 byte long request.

## [8] manageRgbPowerMode

Request `[getOrSet, rgbPowerMode]`, reply `[getOrSet, rgbPowerMode]`
**[spec]**. Modes: `0x01` full RGB, `0x02` power save, `0x03` power off.
NOT_ALLOWED if enableSWControlOfPowerModes is not set.

Tool usage:

- **[Solaar]** sleep: `0x80 [01 03 00]`; wake from sleep:
  `[01 01 00]`, retried 3 times because "first command after wake may fail".
  Solaar never uses mode 2; it dims with a host side ramp of 25 steps of
  200 ms (about 5 s) by rewriting static colors (or per-key colors).
- **[OpenRGB]** `StartSleep` sends `[01 03 00]` ("firmware-managed fade to
  off"), sets its state before sending so that no more frames go out, because
  "a write arriving after SetRgbPowerMode (3) can otherwise wake the device
  and cancel the sleep". `Wake` sends `[01 01]` then `SetSWControl(3,5)` and
  repaints. During claim it sends `SetRGBPowerMode(1)` and writes the timers.
- With RampDownToOff (nvCap 0x0020) enabled the firmware fades over that many
  seconds when mode 3 is set **[OpenRGB]**.

## [9] shutdown()

No parameters, no reply data, no errors **[spec]**. "Instructs the device to
reset the SW control flags, playback the shutdown effect (if enabled), and
then switch to deep sleep mode. Note however that playback of the shutdown
effect may not be feasible for wired devices or for wireless devices
connected via USB." Advertised by extCaps 0x0020 (set on the G515).

**[Solaar]** `cleanup()` sends function byte `0x90` with `01 00`, commented as
"SetRgbPowerMode(set=1, mode=0)". By the spec that is fn9 shutdown(), whose
parameters are ignored. Solaar only does this when the device has NvConfig
cap 0x0040 and documents it as matching G HUB on exit. Not tested by us; on a
wired G515 it would put the keyboard into deep sleep **[unverified]**.

## Events

| Event | fn byte | Payload **[spec]** |
|---|---|---|
| [0] effectSyncEvent | `0x00` | byte 0 cluster (`0xFF` all), bytes 1-2 effectCounter (ms into the period) |
| [1] userActivityEvent | `0x10` | byte 0 activityEventType: `0x00` no activity timeout reached, `0x01` user activity detected |
| [2] rgbClusterChangedEvent | `0x20` | same 13 bytes as setRgbClusterEffect (cluster, effectIdx, params10, persistence/powerMode); only if extCaps 0x0040 |

**[Solaar]** `notifications.py` dispatches `fn == 1` to
`rgb_power.on_user_activity(device, data[0])` and treats any non-zero value
as active. **[OpenRGB]** `DispatchEvent` matches `(func & 0xF0) == 0x10` and
a software ID different from its own.

## Multi LED patterns and multi cluster effects

- `rgbClusterIndex = 0xFF` in setRgbClusterEffect applies a multi cluster
  effect to all clusters; the count of such effects is the last byte of
  device getInfo (6 on the G515) **[spec] [hw]**. Their info is read with
  `getInfo [FF e 00]` **[spec]**; we have not enumerated them **[unverified]**.
- **[Solaar] [OpenRGB]** both send a "per-key prep" on mice:
  `setRgbClusterEffect(0xFF, numEffects, zeros, 0x01)`, an effect index one
  past the enumerated list. OpenRGB calls it "an unadvertised custom/direct
  mode slot ... without writing it the per-key pipeline does not engage" (G502
  X PLUS capture). Given the spec, index `numEffects` on cluster 0xFF is most
  likely the first multi cluster effect, not a hidden slot **[unverified]**.
- **[OpenRGB]** for G815/G915/G Pro it first sends effect index 0 (Disabled)
  with persist 1 to every cluster, then a per-key primer and FrameEnd, cloned
  from the older G915 controller. That controller's `InitializeDirect` sends
  `0x1E` (fn1) with all zero params and byte 12 = 1 for cluster 0 and cluster
  1 **[OpenRGB G915]**.

## Recommended sequence for logirgb (G515 class keyboard)

1. 0x8101 fn6 `[05]` (or 0x8100 fn1 `[02]` on devices with onboard profiles)
   so that the profile engine does not overwrite host lighting.
2. 0x8071 fn5 `[01 03 04]`: own clusters and power modes, get idle events.
3. fn1 `[cluster, effectIndex, params10, 0x01]` for each cluster.
4. On `userActivityEvent 0x00`: dim on the host side (or write the power save
   effect), switch events to `0x02`, and after `sleep - idle` seconds send
   fn8 `[01 03]`.
5. On `userActivityEvent != 0`: fn8 `[01 01]` if sleeping, fn5 `[01 03 04]`,
   repaint.
6. To hand back control: fn5 `[01 00 00]`, then 0x8101 fn6 `[03]`. The
   firmware then runs the onboard profile with its own idle/off handling
   **[hw]**.

## Sources

- Logitech, `x8071_rgbeffects_v4.pdf`, HID++ 2.0 feature document, Version 4.
  Folder: https://drive.google.com/drive/folders/0BxbRzx7vEV7eWmgwazJ3NUFfQ28 ,
  file: https://drive.google.com/file/d/1lecrJQgAC7wnXlo8PEnwVSb3L0GioALG/view
- Logitech, `x8040_brightnesscontrol_v1.pdf` (note on Disabled effect):
  https://drive.google.com/file/d/1EIiHKilvLXxFdZJMh2UrvfqYffVyRfH7/view
- Solaar, `lib/logitech_receiver/rgb_power.py`, `hidpp20.py`,
  `settings_templates.py` (RGBControl, RGBEffectSetting, RGBIdleTimeout,
  RGBSleepTimeout, _RgbBootEffectSetting), `notifications.py`,
  `device_quirks.py`: https://github.com/pwr-Solaar/Solaar
- Solaar PR 3149 "Add RGB lighting persistence and software LED power
  management for G515": https://github.com/pwr-Solaar/Solaar/pull/3149
- Solaar issue 3266 (G915 TKL F-keys lost in host mode):
  https://github.com/pwr-Solaar/Solaar/issues/3266
- OpenRGB, `Controllers/LogitechController/LogitechHIDPP20Controller/LogitechHIDPP20Controller.cpp`
  and `.h`, `RGBController_LogitechHIDPP20.cpp`, `LogitechHIDPP20IdleSettings.h`,
  `Controllers/LogitechController/LogitechProtocolCommon.h`,
  `Controllers/LogitechController/LogitechG915Controller/LogitechG915Controller.cpp`:
  https://gitlab.com/CalcProgrammer1/OpenRGB
- libratbag, `src/hidpp20.c`, `src/hidpp20.h`, `src/driver-hidpp20.c`:
  https://github.com/libratbag/libratbag
- libratbag issue 735 (G703 HERO uses 0x8071):
  https://github.com/libratbag/libratbag/issues/735
- g810-led, `src/classes/Keyboard.cpp`:
  https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.cpp
- OpenLogi HID++ reference, 0x8071:
  https://openlogi.org/hidpp/features/x8071-rgb-effects
- Logitech cpg-docs feature list: https://github.com/Logitech/cpg-docs/tree/master/hidpp20
- Rust `hidpp` crate 0.2.0 (no RGB features implemented, checked):
  https://crates.io/crates/hidpp , https://github.com/lus/logy
- Linux kernel `drivers/hid/hid-logitech-hidpp.c`: no 0x8071 support.
- logirgb hardware log: `research/hardware-log.md` in this repository.
