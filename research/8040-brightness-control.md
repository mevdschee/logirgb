# HID++ 2.0 feature 0x8040 BrightnessControl

Research notes for logirgb. 0x8040 sets the overall brightness of a device's
lighting, independent of the effect chosen with 0x8070/0x8071. On keyboards
like the G515 and G915 it is the feature behind the FN brightness key. It is
"not intended for controlling white illumination lighting" (that is 0x1990
Illumination Light, used for example by the Litra lights) **[spec]**.

## Provenance markers

| Marker | Meaning |
|---|---|
| **[spec]** | Logitech's published `x8040_brightnesscontrol_v1.pdf` (Version 1) from the public "hidpp20 public" Google Drive folder. |
| **[hw]** | Verified on hardware (G515 TKL wired 046d:c358 / G502 HERO 046d:c08b). Only the G515 has 0x8040; the G502 HERO does not. |
| **[Solaar]**, **[OpenRGB]**, **[openlogi]** | Read from that tool's source code or documentation. |
| **[unverified]** | Inferred or untested. |

The G515 reports 0x8040 as version 0 in Solaar's feature dump
(`BRIGHTNESS CONTROL {8040} V0`) **[Solaar]**, so the v1 additions below
(illumination functions, 16 bit steps, min brightness semantics, hw_onoff and
transient flags) may not apply to it.

## Function and event table **[spec]**

| fn | Wire byte (swid 0xA) | Name | Request | Reply |
|---|---|---|---|---|
| 0 | `0x0A` | getInfo | none | maxBrightness, steps, capabilities, minBrightness |
| 1 | `0x1A` | getBrightness | none | brightness |
| 2 | `0x2A` | setBrightness | brightness | none |
| 3 | `0x3A` | getIllumination (v1) | none | state |
| 4 | `0x4A` | setIllumination (v1) | state | none |

| Event | fn byte | Payload |
|---|---|---|
| [0] brightnessChangeEvent | `0x00` | brightness u16 |
| [1] illuminationChangeEvent (v1) | `0x10` | state |

All requests fit in a short report (3 parameter bytes). Multi byte values are
big endian. The function byte on the wire is `fn<<4 | swid`.

## [0] getInfo

| Reply byte | Field **[spec]** |
|---|---|
| 0-1 | maxBrightness (u16) |
| 2 | steps (LSB) |
| 3 | capabilities |
| 4-5 | minBrightness (u16) |
| 6 | steps (MSB), added in v1 (reserved, so 0, in v0) |
| 7-15 | reserved |

capabilities bits **[spec]**:

| Bit | Mask | Name | Meaning |
|---|---|---|---|
| 0 | 0x01 | hw_brightness (v0: "hw") | a dedicated hardware brightness control (button or knob) exists |
| 1 | 0x02 | events | the device sends brightnessChangeEvent / illuminationChangeEvent |
| 2 | 0x04 | illumination | getIllumination/setIllumination are supported |
| 3 | 0x08 | hw_onoff | a dedicated hardware on/off control exists |
| 4 | 0x10 | transient | 1 = settings are NOT persisted across power cycles |
| 5-7 | | reserved | |

Field notes **[spec]**:

- maxBrightness: device defined unit, for example PWM or percent.
- minBrightness: 0 is valid and means the control can dial down to off. With
  hw_onoff it is recommended to be the lowest still visible level.
- steps: "the number of brightness steps in any default, factory behavior
  including the 'off' and 'max' states". 0 means two steps (on/off). Steps do
  not have to be linear: "some devices advertise five steps while the hardware
  controls allows the brightness to be set to the values 0, 12, 25, 50, 100".
- A brightness of 0 "is generally considered to be equivalent to the
  illumination being switched off".

**[hw] G515 TKL:** `8040 0 -> 00 64 05 03 00 00`: max 100, 5 steps, caps
0x03 (hw_brightness + events), min 0. No illumination functions, no hw_onoff,
settings persistent (transient bit clear). Byte 6 was not recorded; as a v0
device it should be 0.

### Halving steps

The G515's 5 steps from 0 to 100 are not linear. Solaar computes "a halving
series from max_value down by powers of two, plus 0" and snaps written values
to it **[Solaar]**:

```python
# Solaar, lib/logitech_receiver/settings_templates.py, halving_marks()
# https://github.com/pwr-Solaar/Solaar (GPL-2.0-or-later)
levels = [-(-max_value // (1 << i)) for i in range(step_count - 1)]
return sorted({0, *levels})
```

For max 100 and 5 steps this gives `[0, 13, 25, 50, 100]`, documented in
Solaar as "matching the G515 FN-F8 cycle". The spec example uses 12 instead
of 13; the exact firmware levels below 25 are not confirmed **[unverified]**.
Solaar also uses these marks for its idle "Dim" intensity choices.

Solaar parses `steps = reply[2] & 0x0F` and ignores byte 6 **[Solaar]**; the
spec defines byte 2 as the full low byte of a 16 bit count.

## [1] getBrightness

Reply bytes 0-1: current brightness, minBrightness..maxBrightness **[spec]**.

**[hw] G515 TKL:** `8040 1 -> 00 32` = 50. Later the same call returned 55
(`0x37`) without logirgb writing it. 55 is not one of the halving levels, so
it was probably set by another host tool or by a firmware path we do not
understand **[unverified]**; the FN brightness key would be expected to land
on a preset.

## [2] setBrightness(brightness)

Request bytes 0-1: brightness (u16) **[spec]**. Values between the presets are
allowed; the device either uses them as is or rounds to a preset, and if it
rounds it must send a brightnessChangeEvent with the rounded value. Out of
range values are rejected with INVALID_ARGUMENT (0x02), not clamped.

**[hw] G515 TKL:** `8040 2 00 64` (100) took effect immediately and is
visible on top of whatever 0x8071 effect is running, also while the host owns
the 0x8071 clusters. Brightness and the 0x8071 effect are separate layers.

## [3] getIllumination / [4] setIllumination (v1)

- getIllumination reply byte 0 bit 0: 0 off, 1 on **[spec]**.
- setIllumination request byte 0 bit 0: 0 off, 1 on **[spec]**.
- Both return NOT_ALLOWED (0x05) when the illumination capability bit is not
  set **[spec]**.
- The illumination state is a "master switch" orthogonal to brightness:
  turning it off does not change the stored brightness, getBrightness keeps
  reporting the old value, and setBrightness while off does not turn the
  light on **[spec]**.

The G515 does not set the illumination bit **[hw]**, so these were not
tested.

Solaar calls the illumination bit "on_off" (`on_off = bool(caps & 0x04)`).
When it is set, Solaar reads fn3 first and reports brightness 0 if the light
is off, and implements a write below the minimum as fn4 `[00]` (off), any
other write as fn4 `[01]` followed by fn2 **[Solaar]**.

## Events **[spec]**

| Event | Payload | When |
|---|---|---|
| [0] brightnessChangeEvent | bytes 0-1 brightness | user changed brightness on the hardware, at power on / wake, or after setBrightness rounding. Not sent for an exact setBrightness. |
| [1] illuminationChangeEvent | byte 0 bit 0 state | user toggled the light on the hardware, at power on / wake. Not sent for setIllumination. |

Only sent when the events capability bit is set (the G515 sets it **[hw]**).
Events arrive with software ID 0, so the function byte is `0x00` or `0x10`.

**[Solaar]** `notifications.py`:

```python
# Solaar, lib/logitech_receiver/notifications.py
# https://github.com/pwr-Solaar/Solaar (GPL-2.0-or-later)
if notification.address == 0x00:
    brightness = struct.unpack("!H", notification.data[:2])[0]
    device.setting_callback(device, settings_templates.BrightnessControl, [brightness])
elif notification.address == 0x10:
    brightness = notification.data[0] & 0x01
```

On an illumination "on" event Solaar re-reads fn1 to get the brightness.
Brightness events from the G515 FN key have not been captured by us yet
**[unverified on hw]**.

## Interaction with 0x8071 **[spec]**

Both the x8040 and the x8071 documents carry this note: devices with a
hardware on/off button (hw_onoff) that implement getIllumination and
setIllumination "are recommended not to implement the Disabled effect
(effect ID 0) from x8071 ... This prevents the device from getting into a
state where the hardware button fails to turn on the illumination because
the host has set the x8071 effect to Disabled." The G515 has no hw_onoff bit
and does list Disabled (0x0000) as 0x8071 effect index 0 **[hw]**, so on the
G515 a host that sets Disabled leaves the FN brightness key without visible
effect until an effect is set again **[unverified]**.

Host side dimming tools combine both features differently: Solaar and OpenRGB
dim during idle by rescaling colors on the 0x8071/0x8081 layer, not by
writing 0x8040 **[Solaar] [OpenRGB]**. The G515 firmware's own idle dim (with
host control released) uses the dimmed copy stored in the onboard profile,
not 0x8040 **[hw]**. Writing 0x8040 for a dim effect would persist (the
transient bit is clear), so it would survive a crash of the host tool
**[unverified]**.

**[OpenRGB]** The older G915 controller looks up the 0x8040 feature index
(`feature_8040_idx`, hard coded 0x13 wired / 0x14 wireless) but does not use
it; the unified HIDPP20 controller does not implement 0x8040.

## Version history **[spec]**

- v0: getInfo, getBrightness, setBrightness, brightnessChangeEvent.
- v1: getIllumination/setIllumination and illuminationChangeEvent; non-zero
  minBrightness allowed; steps widened to 16 bits (MSB at byte 6); hw_onoff
  and transient capability flags; v0's "hw" flag renamed hw_brightness.

## Recommended use in logirgb

1. getInfo once; read max, min, steps and capability bits (handle v0 by
   treating byte 6 as 0).
2. Offer the halving levels when `steps` is small and max is 100, but accept
   any value in range (the spec allows in-between values).
3. setBrightness with a big endian u16; expect no reply data.
4. Listen for event `0x00` to track FN key changes, if the events bit is set.
5. Only use fn3/fn4 when the illumination bit (0x04) is set; otherwise expect
   INVALID_FUNCTION or NOT_ALLOWED.

## Sources

- Logitech, `x8040_brightnesscontrol_v1.pdf`, HID++ 2.0 feature document,
  Version 1. Folder: https://drive.google.com/drive/folders/0BxbRzx7vEV7eWmgwazJ3NUFfQ28 ,
  file: https://drive.google.com/file/d/1EIiHKilvLXxFdZJMh2UrvfqYffVyRfH7/view
- Logitech, `x8071_rgbeffects_v4.pdf` (the same Disabled effect note):
  https://drive.google.com/file/d/1lecrJQgAC7wnXlo8PEnwVSb3L0GioALG/view
- Logitech cpg-docs feature list (0x8040 "Brightness control"):
  https://github.com/Logitech/cpg-docs/tree/master/hidpp20
- Solaar, `lib/logitech_receiver/settings_templates.py` (BrightnessControl,
  halving_marks, auto_step_count), `notifications.py`, `hidpp20_constants.py`,
  `docs/devices/G515 LS TKL 40B4.text`: https://github.com/pwr-Solaar/Solaar
- OpenLogi HID++ reference, 0x8040:
  https://openlogi.org/hidpp/features/x8040-brightness-control
- OpenRGB, `Controllers/LogitechController/LogitechG915Controller/LogitechG915Controller.cpp`:
  https://gitlab.com/CalcProgrammer1/OpenRGB
- Linux kernel patch "hid: Support for Litra Glow" (uses 0x1990, the white
  light counterpart): https://lkml.iu.edu/hypermail/linux/kernel/2212.1/07246.html
- logirgb hardware log: `research/hardware-log.md` in this repository.
