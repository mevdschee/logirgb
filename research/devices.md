# Logitech G5xx devices: RGB lighting research notes

Notes on the Logitech G5xx family (the G502 mouse line, plus the G512/G513/G515 keyboards and the G535/G560 audio products), focused on what matters for driving RGB over HID++ 2.0 on Linux hidraw.

Every fact below carries a provenance tag:

- **[HW]** verified on hardware in this project.
- **[SRC]** read from the source code or data files of an open-source tool (Solaar, libratbag, OpenRGB, g810-led).
- **[DUMP]** taken from a `solaar show` feature dump, either in Solaar's `docs/devices/` or posted in an issue tracker (real device output, but not our hardware).
- **[WEB]** taken from vendor pages, retailer listings, usb.ids, or linux-hardware.org.
- **[GUESS]** inference, forum hearsay, or unconfirmed.

All USB IDs use vendor `046d` (Logitech).

## Summary table

| Device | Wired USB PID | Wireless PID (WPID) / receiver | HID++ | Lighting feature(s) | RGB zones | Onboard profiles | Linux tools |
|---|---|---|---|---|---|---|---|
| G502 Proteus Core (original, 2014) | C07D | none | 4.2 | 0x1300 only, no 0x8070 | logo + DPI LEDs, single colour (not RGB) | 0x8100 (libratbag shows 3) | libratbag (LED control reported broken), Solaar (basic) |
| G502 Proteus Spectrum | C332 | none | 4.2 | 0x8070 v3, 0x1300 | 2 (primary, logo) | 0x8100 | Solaar, libratbag, OpenRGB |
| G502 HERO / G502 SE HERO (and K/DA, GUESS) | C08B | none | 4.2 | 0x8070, 0x1300 | 2: zone0 primary = DPI LEDs, zone1 logo [HW] | 0x8100: 5 profiles [HW] | Solaar, libratbag, OpenRGB |
| G502 LIGHTSPEED (HERO wireless) | C08D | 407F via receiver C539 | 4.2 | 0x8070 v4, 0x1300 | 2 (primary, logo) | 0x8100 | Solaar, libratbag, OpenRGB |
| G502 X (wired) | C099 | none | 4.2 | none | 0 (no RGB) | 0x8100 | Solaar, libratbag (DPI, buttons only) |
| G502 X LIGHTSPEED | C098 | 409F via receiver C547 | 4.2 | none | 0 (no RGB) | 0x8100 | Solaar, libratbag |
| G502 X PLUS | C095 | 4099 via receiver C547 | 4.2 | 0x8071 v2, 0x8081 v2 | 1 effect zone (primary) + 8 per-LED zones (light strip) | 0x8100 | Solaar, libratbag (quirk G502X_PLUS), OpenRGB |
| G512 Carbon / G513 Carbon | C33C (shared) | none | 4.2 | 0x8070 v7, 0x8080 v2 | 1 effect zone (primary) + per-key | none listed in the dump | g810-led, Solaar, libratbag (G513), OpenRGB |
| G512 SE | C342 | none | (not dumped) | presumed same as G512 | per-key | ? | g810-led |
| G515 TKL (wired-only model) | C358 [HW] | none | 4.2 | 0x8071, 0x8081, 0x8040 [HW] | 1 effect zone (primary) + per-key [HW] | 0x8101 (not 0x8100) [HW] | OpenRGB (generic HID++ 2.0), our tool |
| G515 LIGHTSPEED TKL | C355 (cable) | 40B4 via receiver C547; BLE id B389 | 4.2 | 0x8071 v4, 0x8081 v0, 0x8040 | 1 effect zone + per-key | 0x8101 | Solaar, OpenRGB |
| G535 headset | 0AC4 | built-in dongle | 4.2 | none | 0 (no RGB) | n/a | Solaar (sidetone, battery) |
| G560 speaker | 0A78 | n/a | HID++ 2.0 | 0x8070 (zones only, no 0x8080) | 4 (Logitech spec) | n/a | OpenRGB |

Feature-ID cheat sheet, using Solaar's names from `hidpp20_constants.py` [SRC]: 0x1300 LED_CONTROL, 0x1990 ILLUMINATION, 0x8040 BRIGHTNESS_CONTROL, 0x8060 REPORT_RATE, 0x8070 COLOR_LED_EFFECTS, 0x8071 RGB_EFFECTS, 0x8080 PER_KEY_LIGHTING, 0x8081 PER_KEY_LIGHTING_V2, 0x8100 ONBOARD_PROFILES, 0x8101 PROFILE_MANAGEMENT, 0x8110 MOUSE_BUTTON_SPY, 0x8051 LOGI_MODIFIERS, 0x4522 KEYBOARD_DISABLE_BY_USAGE, 0x4523 KEYBOARD_DISABLE_CONTROLS, 0x4540 KEYBOARD_LAYOUT_2, 0x1B05 FULL_KEY_CUSTOMIZATION, 0x1B10 CONTROL_LIST, 0x0011 PROPERTY_ACCESS, 0x00C3 DFUCONTROL (secure). None of the G5xx devices covered here exposes 0x1990, 0x8061 or 0x8090.

### General observations

- Every G5xx device with RGB uses one of two generations. Older devices use **0x8070** (COLOR_LED_EFFECTS), plus 0x8080 on per-key keyboards: the G502 Proteus Spectrum, the G502 HERO, the G502 LIGHTSPEED, the G512/G513 and the G560. Newer devices use **0x8071** (RGB_EFFECTS), plus 0x8081 for per-LED control: the G502 X PLUS and the G515 family. [DUMP][HW]
- The G502 X and G502 X LIGHTSPEED (non-PLUS) expose no lighting feature at all. [DUMP]
- The 0x8070 mice (C332, C08B, 407F) all also expose the legacy 0x1300 LED_CONTROL. Solaar, libratbag and OpenRGB ignore it for RGB. [DUMP][SRC]
- On 0x8070 mice the LED state that survives power cycles lives in the **0x8100 onboard profile** (one LED slot per zone in each profile). libratbag writes LEDs only through the profile. Solaar and OpenRGB use 0x8070 directly, with "SW control" claimed. [SRC]
- The 0x8070/0x8071 zone location codes are the same in Solaar and libratbag [SRC]: 0 undefined, 1 primary, 2 logo, 3 left side, 4 right side, 5 combined, 6 to 11 primary 1 to 6.
- Effect IDs as Solaar knows them (`LEDEffects` in `hidpp20.py`) [SRC]: 0x00 disabled, 0x01 static, 0x02 pulse, 0x03 cycle, 0x04 wave, 0x08 boot, 0x09 demo, 0x0A breathe, 0x0B ripple, 0x0E decomposition, 0x0F signature1, 0x10 signature2, 0x15 cycle (with saturation), 0x16 wave (with saturation), 0x17 ripple (with saturation). In practice 0x8070 mice use 0x03 for cycle and 0x8071 keyboards use 0x15/0x16/0x17. [HW]

## Mice

### G502 Proteus Core (original G502), 046d:C07D

- Connection: wired USB only. HID++ 4.2. Device name string "Tunable FPS Gaming Mouse G502". Firmware U 88.02.B0017, bootloader BOT 14.00.B0007. [DUMP: Solaar `G502 Gaming Mouse C07D.text`]
- Feature list (19) [DUMP]: 0000, 0001, 0002, 0003, 0005, **1300**, 18A1, 1E00, 1E20, 1EB0, 2201, 2230, 2240, **8060**, **8100**, **8110**, 1850, 00C1, 1801.
- Lighting: there is **no 0x8070 and no 0x8071**. The LEDs (blue G logo and the DPI indicator bar) are single-colour, not RGB. The only lighting-related features are 0x1300 and the LED slots inside the 0x8100 profiles. [DUMP][GUESS: the LEDs being single-colour comes from the product, not the dump]
- libratbag file `logitech-g502-proteus-core.device` matches `usb:046d:c07d`, driver hidpp20, with no quirk. [SRC] In libratbag issue #975 `ratbagctl info` shows 11 buttons, 3 profiles, and 3 LEDs of depth "monochrome". Changing the LEDs had no effect, and the logo kept breathing (issue still open). [WEB]
- Solaar descriptor: `_D("G502 Gaming Mouse", codename="G502", usbid=0xC07D, interface=1)`. [SRC]
- usb.ids name: "G502 Mouse". [WEB]
- OpenRGB: no specific detector. It may be picked up by the generic HID++ 2.0 detector, but with no 0x8070/0x8071 there is nothing to drive. [SRC][GUESS]

### G502 Proteus Spectrum, 046d:C332

- Connection: wired USB only. HID++ 4.2. Name string "Tunable RGB Gaming Mouse G502". Firmware U1 03.02.B0012, bootloader BOT 14.00.B0007. [DUMP: Solaar `G502 Proteus Spectrum Optical Mouse C332.txt`]
- Feature list (20) [DUMP]: 0000, 0001, **8070 v3** (at index 2), 0003, 0005, **1300**, 18A1, 1E00, 1E20, 1EB0, 2201 v1, 2230, 2240, **8060**, **8100**, **8110**, 1850, 00C1, 1801, 1802.
- Zones: 2, shown by Solaar as "LEDs Primary" and "LEDs Logo" (Solaar issue #2997, Polish locale: "Diody LED Podstawowy", "Diody LED Logo"). [DUMP] Primary is the DPI indicator area, logo is the G logo. [GUESS by analogy with the G502 HERO]
- Effects: not dumped. Presumably the same 0x8070 set as the G502 HERO (off, static, cycle, breathe). [GUESS]
- libratbag: `logitech-g502-proteus-spectrum.device`, `DeviceMatch=usb:046d:c332`, `Quirk=INDEX_OFFSET`. [SRC]
- OpenRGB: `REGISTER_HID_DETECTOR_IPU("Logitech G502 Proteus Spectrum Gaming Mouse", DetectLogitechWired, LOGITECH_VID, LOGITECH_G502_PROTEUS_SPECTRUM_PID, 1, 0xFF00, 2);`, which means interface 1, usage page 0xFF00, usage 2. [SRC: OpenRGB LogitechControllerDetect.cpp]
- Solaar descriptor: `usbid=0xC332, interface=1`. Solaar 1.1.15 crashed on it (issue #2997), fixed in 1.1.16. [SRC][WEB]

### G502 HERO / G502 SE HERO, 046d:C08B

The plain G502 HERO and the black/white "SE" edition share PID C08B. usb.ids calls it "G502 SE HERO Gaming Mouse". linux-hardware.org and the device itself report "G502 HERO Gaming Mouse". [WEB][DUMP] The G502 HERO K/DA edition (SKU 910-006096/7/8) is a cosmetic variant and almost certainly also enumerates as C08B, but we found no lsusb output to confirm it. [GUESS]

- Connection: wired USB only. HID++ 4.2. Firmware U1 27.03.B0010, bootloader BOT 81.00.B0002. Name "G502 HERO Gaming Mouse". [DUMP: Solaar `G502 SE Hero Gaming Mouse C08B.txt`]
- Feature list (19) [DUMP]: 0000, 0001, **8070** (index 2), 0003, 0005, **1300**, 18A1, 1E00, 1E22, 1EB0, 2201, **8060**, **8100**, **8110**, 00C2, 1801, 1802, 1806, 18B1.
- **Verified on hardware [HW]:**
  - 0x8070 reports **2 zones**:
    - zone 0, location 1 "primary": the DPI indicator LEDs. On the test unit these only light up briefly in blue as a status indicator (for example on a DPI change). They do not show a persistent effect colour the way the logo does.
    - zone 1, location 2 "logo": the G logo.
  - Effects per zone: off (0x00), static (0x01), breathe (0x0A), cycle (0x03).
  - 0x8100 onboard profiles: **5 profiles**, sector size **255** bytes, **16 sectors**, **11 buttons**. The factory ROM profiles are at sectors **0x0101 to 0x0105**.
- libratbag: `logitech-g502-hero.device`, `DeviceMatch=usb:046d:c08b`, `Quirk=INDEX_OFFSET` (profile index is 1-based). [SRC] In libratbag issue #1181, `ratbagctl info` shows "Number of Buttons: 11, Number of Leds: 2, Number of Profiles: 5" and 5 DPI slots. The reporter writes that Piper accepted DPI-LED changes but "the leds stay off", until the DPI LEDs had been enabled once from G HUB/LGS on another OS. [WEB] This matches our observation that zone 0 behaves as a status indicator.
- OpenRGB: `REGISTER_HID_DETECTOR_IPU("Logitech G502 HERO Gaming Mouse", DetectLogitechWired, LOGITECH_VID, LOGITECH_G502_HERO_PID, 1, 0xFF00, 2);` [SRC]
- Solaar: `_D("G502 SE Hero Gaming Mouse", codename="G502 Hero", usbid=0xC08B, interface=1)`. [SRC] Shows LED control and per-zone settings. Issue #2914 notes that no reprogrammable buttons are exposed (there is no 0x1B04 on this mouse). [WEB]

### G502 LIGHTSPEED (G502 HERO wireless), 046d:C08D wired / WPID 407F

- Connection: LIGHTSPEED receiver **046d:C539**, where the device appears as WPID **407F**. A USB cable gives PID **C08D**. Solaar shows `Transport IDs: {'wpid': '407F', 'usbid': 'C08D'}` and Model ID `407FC08D0000`. Firmware MPM 17.00.B0008, BOT 92.00.B0008. [DUMP: Solaar `G502 Lightspeed Wireless Gaming Mouse 407F.txt`, issues #3335, #3247]
- Feature list (30) [DUMP]: 0000, 0001, 0003 v2, 0005, 1D4B, 0020, 1001 v2 (battery voltage), **8070 v4**, **1300**, **8100**, **8110**, **8060**, 2201 v1, 1802, 1803, 1805, 1806 v4, 1811, 1830, 1890 v4, 1891 v4, 18A1, 1801, 18B1, 1DF3, 1E00, 1EB0, 1863, 1E22, 2121.
- Zones: 2, "LEDs Primary" and "LEDs Logo". A saved Solaar setting: `LEDs Primary (saved): !LEDEffectSetting {ID: 1, color: 16711680, ...}` (static red). [DUMP] The layout is the same as the wired G502 HERO (DPI LEDs + logo). [GUESS]
- Solaar issue #3247: LED settings disappeared after a Solaar update, fixed by removing `led_zone__` from `_absent` in config.yaml. Worth knowing because it shows Solaar caches which settings a device lacks. [WEB]
- libratbag: `logitech-g502-hero-wireless.device`, `DeviceMatch=usb:046d:407f;usb:046d:c08d`, `Quirk=INDEX_OFFSET`. [SRC] Issues #1150 and #1089 report wireless-only errors (ERR_UNKNOWN, G-Shift). [WEB]
- OpenRGB: receiver C539 is handled by `DetectLogitechLightspeedReceiver`. Virtual PID 407F has `REGISTER_HID_DETECTOR_IPU("Logitech G502 Wireless Gaming Mouse", DetectLogitechWireless, ..., 0x407F, 2, 0xFF00, 2)`. Wired C08D is registered with `_PU` (no interface number) "to accomodate old and new firmware" (OpenRGB #4627). [SRC]

### G502 X (wired), 046d:C099

- Connection: wired USB only. HID++ 4.2. Firmware U1 60.00.B0009, BL1 59.00.B0002. [DUMP: Solaar `G502 X C099.txt`]
- Feature list (22) [DUMP]: 0000, 0001, 0003 v4, 0005, 0020, 00C2, 1801, 1802, 1805, 1806 v8, 1875, 18A1, 18B1, 18C0, 1E00, 1E22, 1EB0, 2121 v1, 2201 v2, **8060**, **8100**, **8110**.
- **No lighting feature.** The non-PLUS G502 X has no RGB. [DUMP]
- libratbag: `logitech-g502-x.device`, `DeviceMatch=usb:046d:c099`, `Quirk=INDEX_OFFSET`. [SRC]

### G502 X LIGHTSPEED, 046d:C098 wired / WPID 409F

- Connection: LIGHTSPEED receiver **046d:C547** (a newer receiver that takes 2 devices). WPID **409F**, USB **C098**. Model ID `409FC0980000`, firmware MPM 30.00.B0014. [DUMP: Solaar issue #2560]
- Feature list (29, partially printed) [DUMP]: 0000, 0001, 0003 v4, 0005, 1D4B, 0020, 1004 v3 (unified battery), 2201 v2, 2121, **8100**, **8110**, **8060**, 1500 (force pairing), plus hidden internal features.
- **No lighting feature.** [DUMP]
- libratbag: `logitech-g502-x-wireless.device`, `DeviceMatch=usb:046d:c098;usb:046d:409f`, `Quirk=INDEX_OFFSET`. [SRC]

### G502 X PLUS, 046d:C095 wired / WPID 4099

- Connection: LIGHTSPEED receiver **046d:C547**, WPID **4099**. USB cable gives PID **C095**. Model ID `4099C0950000`, firmware MPM 27.00.B0016, BL1 42.00.B0016. [DUMP: Solaar `G502 X PLUS 4099.txt`; OpenRGB work item #2988 lsusb: "ID 046d:c095 Logitech, Inc. G502 X PLUS" and "ID 046d:c547 Logitech, Inc. USB Receiver"]
- Feature list (32) [DUMP]: 0000, 0001, 0003 v4, 0005, 1D4B, 0020, 1004 v3, 2201 v2, 2121, **8071 v2**, **8081 v2**, **8100**, **8110**, **8060**, 1500, 00D0 v3, 1802, 1803, 1806 v8, 1811, 1805, 1830, 1875, 1861, 1890, 18A1, 1801, 1E00, 1E22, 1EB0, 18B1, 18C0. (That Solaar dev build printed some IDs byte-swapped, for example `{0318}` for 1803. This is a display bug, not device data.)
- Lighting:
  - 0x8071 shows a single "LEDs Primary" zone (a Solaar config in issue #3092 stores it as `rgb_zone_1`). [DUMP]
  - 0x8081 per-key V2 exposes **8 individually addressable LEDs**. Solaar labels them with keyboard names A to H because it reuses the keyboard key table. These are the 8 LEDs of the light strip; Logitech specs say "RGB lighting: 8-zone". [DUMP][WEB]
  - Effects: OpenRGB notes the device "only enumerates effects 0..3 via GetEffectInfo", with effect index 2 being breathing. The vendor app also writes an unadvertised index 4 as part of a per-LED takeover. [SRC: OpenRGB LogitechHIDPP20Controller.cpp, "DoObservedPerKeyPrep"]
  - OpenRGB's per-key takeover sequence was captured from G HUB on this mouse. It sends `SetEffectByIndex(cluster=0xFF, idx=2, params[6..7]=0x20 0x64, persist=1)`, then `SetEffectByIndex(0xFF, idx=4, zeros, persist=1)`, and only then 0x8081 writes. The bytes 0x20 0x64 are read back from 0x8071 fn0 GetEffectSpecificInfo "effect cards". [SRC] Short quote from the OpenRGB comment (GPL-2.0-or-later): "effectIdx=0x04: OUT OF RANGE on G502 X PLUS ... Likely a 'custom / direct mode' slot the firmware accepts but does not advertise".
  - Active-play detection turns the lighting off while the hand covers it. [WEB, Logitech marketing]
- libratbag: `logitech-g502-x-plus.device`, `DeviceMatch=usb:046d:4099;usb:046d:c095`, `Quirk=G502X_PLUS;INDEX_OFFSET`. The driver comment explains the quirk [SRC: libratbag src/driver-hidpp20.c]:
  ```
  // On G502X+ the second slot controls the user configurable LED.
  // (Note: There is only 1 reported LED, it just happens to use 2nd index though)
  h_led = &profile->leds[1];
  ```
  libratbag also drops 0x8071 on any device without 0x8100 ("we only support 0x8071 via 0x8100"). Wireless use via C547 is not matched out of the box (issues #1655, #1780, #1805). Users fix it with a local device file adding `usb:046d:c547` and `DeviceIndex=1`. [WEB]
- OpenRGB: `LOGITECH_G502_X_PLUS_PID 0xC095`, `LOGITECH_G502_X_PLUS_LIGHTSPEED_VIRTUAL_PID 0x4099`. It is handled by the unified HID++ 2.0 controller. [SRC]
- Onboard profiles: 0x8100 present, Solaar shows "Profile 1". The count is not dumped. [DUMP]

## Keyboards

### G512 Carbon / G513 Carbon, 046d:C33C

- The G512 and G513 share one PID and one PCB. The G513 adds a palm rest. lsusb shows `iProduct "G512 RGB MECHANICAL GAMING KEYBOARD"` even on G513 units, while HID++ 0x0005 returns the true name (for example "G513 Carbon GX Blue"). [WEB: g810-led #257; linux-hardware.org lists only "G512 RGB MECHANICAL GAMING KEYBOARD" for c33c; DUMP: Solaar #2653]
- Connection: wired USB, HID++ on interface 1, usage page 0xFF43 (OpenRGB's comment lists the G512 under "0xFF43 any usage"). Firmware U1 19.00.B0012. HID++ 4.2. [DUMP][SRC]
- Feature list (20, internal ones hidden) [DUMP: Solaar issue #2653]: 0000, 0001, 0003 v2, **4522** (keyboard disable by usage), 0005, 1E00, 4540, (7 hidden), **8060**, 00C2, (10, 11 hidden), **8080 v2** (index 12 = 0x0C), **8070 v7** (index 13 = 0x0D), 1821, 1BC0, (16 hidden), 40A2, 1B04 v3, 0020.
- Lighting: 0x8070 has one zone ("LEDs Primary"), and per-key colour goes through 0x8080. The Fn-layer keys "Next Color Effect", "Increase/Decrease Color Effect Speed" and "Load Lighting Custom Profile" are divertable through 0x1B04. [DUMP]
- **Cross-check with g810-led [SRC]:** g810-led hard-codes feature index 0x0C for per-key writes and 0x0D for native effects on G410/G512/G513/G Pro. That matches the dump exactly:
  ```
  case LedKeyboard::KeyAddressGroup::keys:
      return { 0x12, 0xff, 0x0c, 0x3a, 0x00, 0x01, 0x00, 0x0e };
  ...
  protocolBytes[0] = 0x0d;  // native effect feature index on g512/g513
  ```
  (from g810-led `src/classes/Keyboard.cpp`, GPL-3.0). The commit frame is `{ 0x11, 0xff, 0x0c, 0x5a }` (0x8080 index, function 5).
- g810-led maps `0xc33c -> g513` and `0xc342 -> g512`. Its native effects are color, breathing, cycle, hwave, vwave, cwave, with parts keys/logo. [SRC]
- Solaar: `_D("G512 RGB Mechanical Gaming Keyboard", codename="G512", usbid=0xC33C, interface=1)` (added in PR #1403 after issue #1400). [SRC]
- libratbag: `logitech-g513.device`, `DeviceMatch=usb:046d:c33c`, DeviceType=keyboard. [SRC]
- OpenRGB: no dedicated PID define. It falls through to the unified HID++ 2.0 detector on usage page 0xFF43. [SRC]
- Onboard profiles: no 0x8100 in the dump. [DUMP]

### G512 SE, 046d:C342

- Kernel log: `idProduct=c342 ... Product: G512 SE`. The reporter describes it as "a 'newer version' of G413 but with RGB lights" (g810-led #200, #230). Model Y-U0034, P/N 820-009154 (g810-led PR #197). [WEB]
- Works with `g512-led -dv 046d -dp c342 -tuk 1 ...`, so the lighting protocol is the same as the G512/G513. No HID++ feature dump is available. [WEB][GUESS for feature list]

### G515 TKL (wired-only model), 046d:C358

- linux-hardware.org name: "G515 TKL". Retail parts 920-012868 (black) and 920-012869 (white) are the wired "G515 TKL" (not LIGHTSPEED). [WEB]
- **Verified on hardware [HW]:**
  - Features: 0000 0001 0003 0005 0020 0011 **8071** **8081** 1B10 4523 4540 **8040** **8101** 1B05 8051 00C3 00D0 1802 1803 1807 1817 1805 1890 1E00 1E02 1602 1EB0 1801 18B0.
  - 0x8071: **one zone**, location 1 (primary). Effects: off (0x00), static (0x01), breathe (0x0A), cycle (0x15), wave (0x16), ripple (0x17).
  - 0x8040 brightness: range 0 to 100.
  - No 0x8100. Profiles use **0x8101** (PROFILE_MANAGEMENT), the newer profile feature.
- Compared with the LIGHTSPEED model (40B4 below), the wired unit lacks 1D4B, 0007, 1004, 1830, 1861 and 1891 (wireless/battery/BLE related) and adds 0x0011 PROPERTY_ACCESS and 0x00C3. [HW vs DUMP]
- Tool support: no tool lists C358 explicitly. OpenRGB's unified HID++ 2.0 detector matches any Logitech node on 0xFF00/usage 2 or 0xFF43, so it should pick it up. [SRC][GUESS] Solaar has no static descriptor but enumerates HID++ 2.0 devices dynamically. [GUESS]

### G515 LIGHTSPEED TKL, 046d:C355 wired / WPID 40B4

- Connection: LIGHTSPEED receiver **046d:C547**, WPID **40B4**. USB-C cable gives PID **C355**. Bluetooth LE id **B389**. Model ID `B38940B4C355`, firmware MPK 25.01.B0011, BL2 19.01.B0011. [DUMP: Solaar `G515 LS TKL 40B4.text`, issue #3310]
- Feature list (34) [DUMP]: 0000, 0001, 0003 v6, 0005 v3, 1D4B, 0020, 0007, 0011, 1004 v5, **8071 v4**, **8081 v0**, 1B10, 4523 v1, 4540 v1, **8040**, **8101**, 1B05, 8051, 00D0 v3, 1802, 1803 v1, 1807 v3, 1817, 1805, 1830, 1890 v9, 1891 v9, 1E00, 1E02, 1602, 1EB0, 1861 v1, 18B0 v1, 1801.
- Lighting: one 0x8071 zone (primary) plus per-key 0x8081 (Solaar lists about 100 key ids). [DUMP]
- Solaar specifics [SRC: Solaar `hidpp20.py`]:
  ```
  LedDirectionBlocklist = {
      "40B4": {4, 5},  # G515 LS TKL - no edge-radiating wave geometry
  }
  ```
  So wave directions 4 and 5 are hidden for this model.
- Solaar issue #3241: static mode "always shows yellow" was caused by a leftover per-key (0x8081) layer. Fixed by setting per-key lighting to "ignore". **The per-key layer overrides the zone effect.** [WEB]
- Solaar issue #3310: software sleep via `SetRgbPowerMode(3)` (0x8071 function 8) parks key scanning, so the first wake key is lost. A maintainer notes that 0x8081 fn6, which switches between onboard and host control mode, resets the keyboard's internal modifier state, "the same behavior on windows". [WEB] Relevant if our tool takes host control.
- OpenRGB [SRC]: `LOGITECH_G515_LS_TKL_PID 0xC355`, virtual PID `0x40B4`. The quirk table has `{ 0x40B4, 0xC355, HIDPP20_QUIRK_FADE_ACCEPTS_WRITES }`. Another comment notes the G515 over USB "silently drops responses (~20 writes then ...)" when writes are pipelined too fast. On receiver PID C547, OpenRGB's legacy G915 detector name-probes and skips devices whose name starts with "G515".
- No libratbag support (keyboard, and it lacks 0x8100). [SRC]

## Audio (brief)

### G535 wireless headset, 046d:0AC4

- No RGB. Features: 0000, 0001, 0003, 0005, 8300 (sidetone), 1F20 (ADC measurement). Solaar supports sidetone and battery. [DUMP: Solaar `G535 Wireless Gaming Headset 0AC4.txt`]

### G560 LIGHTSYNC speaker, 046d:0A78

- 4 lighting zones (Logitech spec, described as left/right, front/rear). [WEB]
- OpenRGB: a udev rule for 0a78, handled by the unified HID++ 2.0 controller. HID++ is on usage page 0xFF43. The device is "0x8070 ZONE-ONLY (no 0x8080: G560/G933 ...)". The claim is just `SetSWControl(1,1)`. Its collection "has no short report" and "rejects the 0x10 write", so all requests must use long reports (0x11). [SRC]
- Not in Solaar or libratbag. [SRC]

## Implementation notes for logitech-rgb

1. Pick the protocol by feature presence, not by PID. 0x8070 means zone effects (function 0x30 SetZoneEffect, 0x80 SetSWControl). 0x8071 means effects (function 0x10 SetEffectByIndex, 0x50 control). Add 0x8081 for per-LED colour. Function numbers come from OpenRGB `LogitechProtocolCommon.h`. [SRC]
2. On G502 HERO-class mice, expect the "primary" zone to be a DPI status indicator. Writing a static colour to it may not show persistently. [HW][WEB]
3. On 0x8071 + 0x8081 devices, the per-key layer wins over the zone effect. Clear or release the per-key layer before applying a zone effect (Solaar #3241). [WEB]
4. The G502 X and G502 X LIGHTSPEED have nothing to control. Detect that and report "no lighting" instead of failing.
5. Wireless devices sit behind receivers C539 (G502 LIGHTSPEED) or C547 (G502 X family, G515 LS TKL, G915 family). Address them through device index 1 or 2 on the receiver's hidraw node. [DUMP]

## Sources

Local copies (read for this note):
- Solaar `docs/devices/`: `G502 Gaming Mouse C07D.text`, `G502 Proteus Spectrum Optical Mouse C332.txt`, `G502 SE Hero Gaming Mouse C08B.txt`, `G502 Lightspeed Wireless Gaming Mouse 407F.txt`, `G502 X C099.txt`, `G502 X PLUS 4099.txt`, `G515 LS TKL 40B4.text`, `G535 Wireless Gaming Headset 0AC4.txt`. Upstream: https://github.com/pwr-Solaar/Solaar/tree/master/docs/devices
- Solaar `lib/logitech_receiver/descriptors.py`, `base_usb.py`, `hidpp20.py`, `hidpp20_constants.py`: https://github.com/pwr-Solaar/Solaar/tree/master/lib/logitech_receiver
- OpenRGB `LogitechControllerDetect.cpp`, `LogitechHIDPP20Controller.cpp/.h`, `LogitechProtocolCommon.h`: https://gitlab.com/CalcProgrammer1/OpenRGB/-/tree/master/Controllers/LogitechController
- libratbag `src/hidpp20.c`, `src/hidpp20.h`, `src/driver-hidpp20.c`: https://github.com/libratbag/libratbag/tree/master/src

libratbag device files:
- https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-proteus-core.device
- https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-proteus-spectrum.device
- https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-hero.device
- https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-hero-wireless.device
- https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-x.device
- https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-x-wireless.device
- https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-x-plus.device
- https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g513.device

g810-led:
- https://github.com/MatMoul/g810-led (README, supported keyboards)
- https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.h (PID table)
- https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.cpp (feature indexes 0x0C/0x0D)
- Issues: https://github.com/MatMoul/g810-led/issues/257, https://github.com/MatMoul/g810-led/issues/230, https://github.com/MatMoul/g810-led/issues/200, https://github.com/MatMoul/g810-led/issues/138, https://github.com/MatMoul/g810-led/pull/197

Issue trackers:
- Solaar: https://github.com/pwr-Solaar/Solaar/issues/3335, https://github.com/pwr-Solaar/Solaar/issues/3247, https://github.com/pwr-Solaar/Solaar/issues/3092, https://github.com/pwr-Solaar/Solaar/issues/2560, https://github.com/pwr-Solaar/Solaar/issues/2997, https://github.com/pwr-Solaar/Solaar/issues/2653, https://github.com/pwr-Solaar/Solaar/issues/1400, https://github.com/pwr-Solaar/Solaar/issues/3241, https://github.com/pwr-Solaar/Solaar/issues/3310, https://github.com/pwr-Solaar/Solaar/issues/2914
- libratbag: https://github.com/libratbag/libratbag/issues/1181, https://github.com/libratbag/libratbag/issues/975, https://github.com/libratbag/libratbag/issues/1655, https://github.com/libratbag/libratbag/issues/1780, https://github.com/libratbag/libratbag/issues/1805
- OpenRGB: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/2988 (G502 X PLUS), https://gitlab.com/CalcProgrammer1/OpenRGB/-/issues/4238 (G515 LS TKL request, no data)

ID databases and vendor pages:
- http://www.linux-usb.org/usb.ids
- https://linux-hardware.org/?id=usb:046d-c33c, https://linux-hardware.org/?id=usb:046d-c358, https://linux-hardware.org/?id=usb:046d-c08b
- https://jp.ifixit.com/Device/Logitech_G513_CARBON (PID C33C for G513)
- https://www.logitechg.com/en-my/products/gaming-mice/g502-x-plus-wireless-lightforce.html (8-zone RGB)
- https://logitechg.com/en-za/products/gaming-keyboards/g515-tkl-wireless-keyboard.html
- https://www.microcenter.com/quickview/685306/logitech-g515-tkl-wired-rgb-mechanical-keyboard-black (920-012868)
- https://www.logitechg.com/de-ch/shop/p/g560-rgb-gaming-speakers (4 lighting zones)
