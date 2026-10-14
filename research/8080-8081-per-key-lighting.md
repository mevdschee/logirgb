# HID++ 2.0 features 0x8080 PerKeyLighting and 0x8081 PerKeyLighting v2

Research notes for logirgb. Both features paint individual keys (or other
individually driven LEDs) with host supplied colors, on top of the effect
engine of 0x8070/0x8071. 0x8080 is the older wired-keyboard design (G810,
G910, G410, G Pro, G512/G513 class) using 64 byte very long reports; 0x8081 is
the low bandwidth redesign for wireless keyboards (G915, G815, G515, and some
mice such as the G502 X PLUS) that fits every request in a 16 byte long
report.

## Provenance markers

| Marker | Meaning |
|---|---|
| **[spec]** | Logitech's published `x8081_perkeylighting_v0.pdf` (Version 0) from the public "hidpp20 public" Google Drive folder. There is no public 0x8080 document in that folder. |
| **[hw]** | Verified on hardware (G515 TKL wired 046d:c358 / G502 HERO 046d:c08b). Only the G515 has 0x8081, and only getInfo was exercised; per-key writes are not yet verified by us. The G502 HERO has neither feature. |
| **[Solaar]**, **[OpenRGB]**, **[OpenRGB G915]**, **[g810-led]**, **[openlogi]** | Read from that tool's source code or documentation. |
| **[unverified]** | Inferred, contradictory, or untested. |

## 0x8081 PerKeyLighting (v2)

### Concepts **[spec]**

- An RGB zone is the smallest independently driven light: a key, a button, a
  LED or a group of LEDs driven together.
- Zone IDs are 8 bit. `0x00` and `0xFF` are invalid and mark "end of list"
  in every set function; valid IDs are 1-254. The device has a fixed array of
  zones "known by both FW and SW for every product and every keyboard
  layout".
- Writes are staged and become visible only on `frameEnd` (fn7).
- Guidelines: never read colors back (there is no read function), keep state
  on both sides, validate every request and resend if the ack is missing,
  prefer range and single-value writes, and use delta compression for
  animations.

### Function table **[spec]**

| fn | Wire byte (swid 0xA) | Name | Zones per request |
|---|---|---|---|
| 0 | `0x0A` | getInfo(typeOfInfo, param1) | n/a |
| 1 | `0x1A` | setIndividualRgbZones | up to 4 arbitrary zones |
| 2 | `0x2A` | setConsecutiveRgbZones | 5 consecutive zones |
| 3 | `0x3A` | setConsecutiveRgbZonesDeltaCompression5bit | 8 consecutive zones |
| 4 | `0x4A` | setConsecutiveRgbZonesDeltaCompression4bit | 10 consecutive zones |
| 5 | `0x5A` | setRangeRgbZones | up to 3 ranges |
| 6 | `0x6A` | setRgbZonesSingleValue | up to 13 zones, one color |
| 7 | `0x7A` | frameEnd(persistence, currentFrame, nFramesTillNextChange) | commit |

No events are defined. All set functions take a 16 byte long request.

The older per-model OpenRGB G915 driver uses the same numbers with software
ID 0xF: `0x1F` "little frame" (fn1), `0x6F` "big frame" (fn6) and `0x7F`
commit (fn7) **[OpenRGB G915]**. g810-led sends `0x1c` (fn1, swid 0xC) on the
G815 **[g810-led]**.

### [0] getInfo(typeOfInfo, param1): RGB zone presence

Request: `[typeOfInfo, param1]`. Only typeOfInfo `0x00` (RGBZonePresence) is
defined **[spec]**:

| param1 | Zone IDs covered |
|---|---|
| 0x00 | 0-111 |
| 0x01 | 112-223 |
| 0x02 | 224-255 (bits for 256-335 must be 0) |

Reply **[spec]**: byte 0 typeOfInfo, byte 1 param1, bytes 2-15 a 112 bit
bitmap, LSB first within each byte: `zone = page*112 + (byte-2)*8 + bit`.
"Both 0x00 (invalid ZoneID) and 0xFF (all ZoneIDs) are special Zone ID values
that shall be reported as SUPPORTED by all devices." Error INVALID_ARGUMENT
for param1 out of range.

Note: OpenRGB's comment describes the request as "uint16 BE typeOfInfo + 1
pad byte" and sends `{0x00, page, 0x00}`; that is the same bytes as the spec's
`[typeOfInfo=0, param1=page]` **[OpenRGB]**. Solaar sends
`feature_request(0x00, 0x00, page)` for pages 0-2, concatenates
`reply[2:]` and tests `key_bitmap[i // 8] >> (i % 8) & 1` for i in 1..254
**[Solaar]**. An earlier OpenRGB version only read page 0 and lost G-keys,
media and logo zones that live in pages 1 and 2 **[OpenRGB]**.

**[hw] G515 TKL:**

```
8081 0 00 00 -> 00 00 ff ff ff ff ff ff ff ff ff ff 00 00 fe ff
8081 0 00 01 -> 00 01 00 00 00 00 00 00 00 00 00 00 00 00 00 00
8081 0 00 02 -> 00 02 00 00 00 40 00 ...
```

Decoded:

| Bitmap bytes | Bits | Zones present |
|---|---|---|
| 0-9 = `ff` | 0-79 | 0 (reserved, reported as the spec requires) and 1-79 |
| 10-11 = `00` | 80-95 | none (numpad absent, TKL) |
| 12 = `fe` | 97-103 | 97-103 (96 absent) |
| 13 = `ff` | 104-111 | 104-111 (modifiers) |
| page 1 | 112-223 | none |
| page 2 byte 3 = `40` | bit 30 | 254 |

So the G515 reports zones 1-79, 97-111 and 254. Zone 255 (bit 31 of page 2)
is not set, which contradicts the spec's "0xFF shall be reported as
SUPPORTED"; possibly the firmware is off by one there, or 254 is something
physical **[unverified]**. Solaar's G515 dump lists "KEY 254" as a paintable
key; OpenRGB treats every id `>= 0xFE` as "a reserved protocol id" and hides
it **[Solaar] [OpenRGB]**.

Zones 100-103 are reported but the G515 TKL has no such keys. OpenRGB maps
them as Keypad =, F13, F14, F15 (`zone = usage - 3`) and classifies them as
"known-but-unstuffed": "Zone ids index the firmware's fixed enumeration
(superset reported, subset stuffed)" **[OpenRGB]**. So the presence bitmap
is a superset, and an ANSI board still reports the ISO keys (47, 97).

### [1] setIndividualRgbZones

Request **[spec]**: up to 4 entries of `[zoneId, R, G, B]` at bytes 0-3, 4-7,
8-11, 12-15. Data is ignored from the first invalid zone ID (`0x00` or
`0xFF`), so a short list is terminated with `0x00` or `0xFF`.

Reply **[spec]**: bytes 0-3 = the four zone IDs echoed, rest zero.
**[OpenRGB]** matches acks by this echo and treats an echoed `0xFF` as
"rejected by firmware" for that zone. The OpenRGB G915 driver terminates
partial frames with `0xFF` **[OpenRGB G915]**; Solaar sends a shorter payload
and lets the transport zero pad **[Solaar]**.

### [2] setConsecutiveRgbZones

Request **[spec]**: byte 0 first zone ID, then 5 x `[R, G, B]` (bytes 1-15).
Reply: byte 0 first zone ID. The spec recommends it over fn1 when 5 changed
zones are consecutive (one more zone per request). No tool we looked at uses
it.

### [3] / [4] delta compression

Delta = current minus previous component value per frame, with 8 bit
rollover; use whichever of the two results has the smaller absolute value
(254 to 2 is +4) **[spec]**.

- fn3, 5 bit: byte 0 first zone, then 8 zones x 3 components x 5 bit signed
  (range -16..+15) packed MSB first, zone then channel, in bytes 1-15.
  Error INVALID_ARGUMENT if `first + 8` exceeds the zone array.
- fn4, 4 bit: byte 0 first zone, then 10 zones x 3 nibbles (range -8..+7),
  high nibble first: byte 1 = R0|G0, byte 2 = B0|R1, byte 3 = G1|B1, and so
  on. Error if `first + 10` exceeds the array.

Not used by Solaar or OpenRGB; logirgb has no animation use for it.

### [5] setRangeRgbZones

Request **[spec]**: up to 3 entries of `[first, last, R, G, B]` at bytes 0-4,
5-9, 10-14; byte 15 unused. Data is ignored from the first invalid first-zone
ID. `first` must be less than `last`, otherwise error. Reply: bytes 0-2 =
the three first-zone IDs. Error INVALID_ARGUMENT if the first zone ID is
invalid.

**[OpenRGB]** fills all keys with one color using a single range from the
lowest to the highest present zone: "Gaps in zone IDs are silently ignored by
firmware". **[Solaar]** uses fn5 `[keys[0], keys[-1], R, G, B]` when the whole
map has one color.

### [6] setRgbZonesSingleValue

Request **[spec]**: bytes 0-2 = R, G, B; bytes 3-15 = up to 13 zone IDs,
terminated by `0x00`/`0xFF`. Reply: R, G, B and the first zone ID only (so
it "does not fit into a short HIDPP report").

**[Solaar]** uses it for any color shared by more than 3 keys (batches of
13), for filling unset keys with the zone base color, and for its idle dim
ramp. **[OpenRGB G915]** "big frame" `0x6F` is the same.

### [7] frameEnd(persistence, currentFrame, nFramesTillNextChange)

Request **[spec]**:

| Byte | Field |
|---|---|
| 0 | persistence: 0 volatile (RAM), 1 RAM and EEPROM ("currently there is no difference") |
| 1-2 | currentFrame (u16) |
| 3-4 | nFramesTillNextChange (u16): 0 while streaming from the host, at least 1 when playing from onboard memory |
| 5-15 | reserved |

No reply data. Tools send all zeros: Solaar `0x70 [00]` (and `00 00 00 00
00` from its dim ramp), OpenRGB a 16 byte zero long report. OpenRGB found that
"short-format hits intermittent BUSY", and that BUSY on frameEnd is
"frame-rate backpressure, not failure: the not-ready window scales with keys
written"; it waits a learned per-zone time before sending frameEnd and polls
on BUSY **[OpenRGB]**. Solaar retries BUSY with 30/60/90 ms backoff and
suppresses frameEnd if any sub-packet failed, "to avoid a visibly wrong
commit" **[Solaar]**.

### Interaction with 0x8071 and the effect layer

- Per-key colors are only visible while the 0x8071 cluster effect is Static.
  With an animated effect "the firmware engine owns the visible layer"
  **[Solaar]**. Solaar treats per-key as "a multi-color sub-mode of Static".
- Keys never written show white (`0xFFFFFF`) once the per-key layer takes
  over; Solaar and OpenRGB both paint unset keys with the zone base color
  first to avoid a white flash **[Solaar] [OpenRGB]**.
- Software control must be claimed first (0x8071 fn5 with the cluster bit,
  see `8071-rgb-effects.md`). Solaar skips all per-key writes while its
  `rgb_control` setting is off **[Solaar]**.
- "First frame after a claim sometimes lands before the firmware has fully
  transitioned out of onboard mode", so Solaar sends the first frame twice
  **[Solaar]**.
- G815/G915/G Pro: OpenRGB (and its older G915 driver) sends a 0x4522
  DisableKeysByUsage fn3 + fn1 empty "handshake", sets every cluster to effect
  index 0 with persist 1, writes one primer key (G915: Escape, zone 0x29 - 3
  = 38) and frameEnd before streaming **[OpenRGB] [OpenRGB G915]**.
- G915 TKL: OpenRGB keeps the keyboard in onboard mode
  (`HIDPP20_QUIRK_KEEP_ONBOARD_MODE`) because host mode breaks the F-row
  G-key mapping; Solaar issue 3266 reports F keys lost on the G915 TKL
  **[OpenRGB] [Solaar issues]**.
- Mice with 0x8081 (G502 X PLUS): Solaar and OpenRGB first send
  `setRgbClusterEffect(0xFF, numEffects, zeros, 0x01)` ("artanis per-key
  prep") **[Solaar] [OpenRGB]**.

Code snippet, Solaar's color bucketing:

```python
# Solaar, lib/logitech_receiver/settings_templates.py, PerKeyLighting._send_perkey_frame
# https://github.com/pwr-Solaar/Solaar (GPL-2.0-or-later)
while len(keys) > 3:  # use an optimized update command that can update up to 13 keys
    data = wire.to_bytes(3, "big") + b"".join([key.to_bytes(1, "big") for key in keys[0:13]])
    if not self._send_with_retry(0x60, data):  # single-value multiple-keys update
        ok = False
    keys = keys[13:]
```

### Zone ID map (0x8081)

The zone IDs are not in the spec, which only says the array is fixed per
product and layout. All tools agree on this encoding, derived from USB HID
keyboard usages:

| Zone ID | Rule | Keys |
|---|---|---|
| 1-29 | usage - 3 (usages 0x04-0x20) | A=1 .. Z=26, 1=27, 2=28, 3=29 |
| 30-36 | usage - 3 | 4=30 .. 9=35, 0=36 |
| 37-53 | usage - 3 | Enter 37, Esc 38, Backspace 39, Tab 40, Space 41, - 42, = 43, [ 44, ] 45, \ (ANSI) 46, ISO #~ 47, ; 48, ' 49, ` 50, , 51, . 52, / 53 |
| 54-66 | usage - 3 | Caps Lock 54, F1 55 .. F12 66 |
| 67-79 | usage - 3 | Print Screen 67, Scroll Lock 68, Pause 69, Insert 70, Home 71, Page Up 72, Delete 73, End 74, Page Down 75, Right 76, Left 77, Down 78, Up 79 |
| 80-96 | usage - 3 | Num Lock 80, KP / 81, KP * 82, KP - 83, KP + 84, KP Enter 85, KP 1-9 86-94, KP 0 95, KP . 96 |
| 97-103 | usage - 3 | ISO \| 97, Menu/Compose 98, Power 99, KP = 100, F13 101, F14 102, F15 103 |
| 104-111 | usage - 0x78 (0xE0-0xE7) | L Ctrl 104, L Shift 105, L Alt 106, L GUI 107, R Ctrl 108, R Shift 109, R Alt 110, R GUI 111 |
| 153 | | brightness key indicator |
| 155-158 | | media: Play/Pause 155, Mute 156, Next 157, Previous 158 |
| 180-184 | G-key number + 0xB3 | G1-G5 |
| 210 | logo number + 0xD1 | logo |

Sources: Solaar `special_keys.KEYCODES` **[Solaar]**, OpenRGB unified
`RGBController_LogitechHIDPP20.cpp` zone table **[OpenRGB]**, and the OpenRGB
G915 driver, which stores raw HID usages and converts with exactly these
offsets (keyboard `- 0x03`, modifiers `- 0x78`, G-keys `+ 0xB3`, logo
`+ 0xD1`, media and indicators unchanged) **[OpenRGB G915]**.

Caveats:

- Solaar's KEYCODES has `"\\": 45`, the same as `"]"`. It should be 46
  (usage 0x31 - 3); the G515 dump shows "KEY 46" unnamed. Solaar also names
  69 "PASTE" where usage 0x48 is Pause, and names 155 "PAUSE" for the media
  Play/Pause key **[Solaar]**.
- On ISO boards the "#~" key is 47 and the extra key next to left Shift is 97;
  on ANSI boards backslash is 46 **[OpenRGB]**.
- The G515 TKL reports 1-79, 97-111 and 254 **[hw]**: no numpad, no media
  keys, no G-keys, no separate logo zone. 100-103 are phantoms. What 254 is
  on the G515 is unknown **[unverified]**; candidates are an indicator or the
  "all zones" alias.
- G915/G815 full size: add 80-96 (numpad), 153 (brightness indicator), 155-158
  (media), 180-184 (G1-G5) and 210 (logo) **[OpenRGB G915]**.
- G513/G512 (wired, older family): these use 0x8080 rather than 0x8081
  according to the OpenRGB unified driver's split between "0x8070 per-key
  keyboards (those exposing 0x8080: G810 family)" and 0x8081 devices
  **[OpenRGB]**; we have not seen a feature dump for the G513 **[unverified]**.

## 0x8080 PerKeyLighting (v1)

There is no public Logitech document for 0x8080 (the Drive folder has no
x8080 file, and OpenLogi does not implement it) **[openlogi]**. The layout
below comes from the OpenRGB unified controller and g810-led.

### Function table **[OpenRGB]**

| fn | OpenRGB name | Report | Request | Reply |
|---|---|---|---|---|
| 0 | GET_INFO | 0x11 | none | typeFlags u16 @0, reserved @2, keyTypeCount u16 @3, maxKeyCount u16 @5 |
| 1 | GET_KEY_TYPE_INFO | 0x11 | keyType u16 | keyCount u16 @0, 3 parameter bytes @2-4 |
| 2 | GET_KEY_COLORS | 0x11 request, 0x12 reply | keyType u16, startIndex u16, persistence (0 = volatile) | 4 byte header, then 14 x (keyId, R, G, B); keyId 0 = padding |
| 3 | SET_KEY_COLORS | **0x12** (64 byte very long report) | keyType u16, count u16, count x (keyId, R, G, B), up to 14 | ack |
| 4 | SET_ALL_KEYS | | not used | |
| 5 | FLUSH_LEDS | 0x11 | empty ("persist = default") | ack; colors appear only after this |

keyType values (single bits, OR'ed in typeFlags) **[OpenRGB]**:

| keyType | Group | keyId meaning |
|---|---|---|
| 0x0001 | keyboard | USB HID keyboard usage (A = 0x04) |
| 0x0002 | consumer / media | vendor local |
| 0x0004 | G-keys | vendor local |
| 0x0008 | buttons | vendor local |
| 0x0010 | logo | vendor local |
| 0x0040 | other / indicators | vendor local |

Quirks seen by OpenRGB: the G910 over-reports typeFlags bit 0x0002; the
G410 keyType 0x40 accepts only keyId 1 and 2; maxKeyCount can be 0 (G810);
fn1 can fail, in which case OpenRGB probes fn2 pages directly; numpad
presence is detected from usages 0x53-0x63 **[OpenRGB]**.

**[g810-led]** builds the same frames with hard coded feature index `0x0c`
(G810): logo `11 ff 0c 3a 00 10 00 01 <keyId R G B>`, indicators
`12 ff 0c 3a 00 40 00 05 ...`, multimedia `12 ff 0c 3a 00 02 00 05 ...`, keys
`12 ff 0c 3a 00 01 00 0e ...` (14 keys per frame), and commits with
`11 ff 0c 5a`. Its effect frames go to the 0x8070 feature index with fn3.

On Windows the 0x12 report lives on a separate top level collection (usage
0x0604), so OpenRGB opens a second handle for it; ACKs come back on the
normal handle **[OpenRGB]**. On Linux hidraw the 0x12 report is sent on the
same node if its report descriptor declares report ID 0x12 **[unverified]**.
The G515 HID++ node only declares reports 0x10 and 0x11 **[hw]**, so 0x8080
style very long writes would not work there anyway.

Differences from 0x8081: 0x8080 addresses keys by (keyType, USB usage)
instead of a flat zone ID, can read colors back (fn2), sends up to 14 keys
per 64 byte report and commits with fn5. 0x8081 has no read function at all
**[spec] [Solaar]**.

## Sources

- Logitech, `x8081_perkeylighting_v0.pdf`, HID++ 2.0 feature document,
  Version 0. Folder: https://drive.google.com/drive/folders/0BxbRzx7vEV7eWmgwazJ3NUFfQ28 ,
  file: https://drive.google.com/file/d/1tKUqcL1dXKj69nuejgOrzoQ8Xq69RO5v/view
- Logitech cpg-docs feature list (lists 0x8080 "Per Key Lighting"):
  https://github.com/Logitech/cpg-docs/tree/master/hidpp20
- Solaar, `lib/logitech_receiver/settings_templates.py` (PerKeyLighting),
  `special_keys.py` (KEYCODES), `rgb_power.py`, `docs/devices/G515 LS TKL 40B4.text`:
  https://github.com/pwr-Solaar/Solaar
- Solaar issue 3266: https://github.com/pwr-Solaar/Solaar/issues/3266
- OpenRGB, `Controllers/LogitechController/LogitechHIDPP20Controller/LogitechHIDPP20Controller.cpp`,
  `.h`, `RGBController_LogitechHIDPP20.cpp`,
  `Controllers/LogitechController/LogitechG915Controller/LogitechG915Controller.cpp`,
  `.h`, `RGBController_LogitechG915.cpp`:
  https://gitlab.com/CalcProgrammer1/OpenRGB
- g810-led, `src/classes/Keyboard.cpp`:
  https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.cpp
- OpenLogi HID++ reference, 0x8081 and 0x8080:
  https://openlogi.org/hidpp/features/x8081-per-key-lighting ,
  https://openlogi.org/hidpp/features/x8080-per-key-lighting
- USB HID Usage Tables, Keyboard/Keypad page (0x07): https://usb.org/document-library/hid-usage-tables-15
- logirgb hardware log: `research/hardware-log.md` in this repository.
