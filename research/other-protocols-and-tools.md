# Other protocols, SDKs and Linux tools for Logitech G5xx RGB

Companion to `hidpp-transport.md`. This file covers everything around Logitech
G-series lighting that is not the HID++ 2.0 lighting features themselves
(0x8070, 0x8071, 0x8080, 0x8081, 0x8100, 0x8101): the "old" G-keyboard
command set used by g810-led, the Windows SDKs, G HUB behaviour, other Linux
tools, and bugs reported for the G502 HERO and the G515.

## Legend

| Tag | Meaning |
|---|---|
| **[HW]** | Verified on hardware in this project (G515 TKL wired 046d:c358, G502 HERO 046d:c08b) |
| **[SRC: tool]** | Read in the named tool's source code |
| **[DOC]** | Official Logitech documentation or support pages |
| **[ISSUE]** | Issue, PR or MR text in a project tracker |
| **[FORUM]** | Forum post, retailer page, Q&A |
| **[INF]** | Our own inference from the above |
| **[Unverified]** | Could not be confirmed |

Dates are as of 2026-10-14.

## 1. The "G810 protocol" is HID++ 2.0 with hardcoded indexes

g810-led (MatMoul/g810-led, v0.4.3, last commit 2022-11-30) drives the
2015-2019 generation of G keyboards with fixed byte strings such as
`11 ff 0c 3a ...` and `11 ff 0c 5a`. These are ordinary HID++ 2.0 long (`0x11`,
20 bytes) or very long (`0x12`, 64 bytes) reports:

| Byte | Example `11 ff 0c 3a` | Meaning |
|---|---|---|
| 0 | `11` / `12` | long / very long report |
| 1 | `ff` | device index, corded device |
| 2 | `0c` | feature index, **hardcoded per model**, never looked up through IRoot |
| 3 | `3a` | function 3, software ID `0xA` |

g810-led uses software ID `0xA` for writes and `0xC`/`0xD`/`0xE`/`0xF` in
some FX and G910 commands [SRC: g810-led `Keyboard.cpp`]. That collides with
the software ID this project uses (`0xA`), but g810-led opens, writes and
closes for every packet and does not read replies in its hidapi build, so in
practice it is harmless [INF].

### 1.1 Supported keyboards

From `src/classes/Keyboard.h` (`SupportedKeyboards`) and `udev/g810-led.rules`
[SRC: g810-led]:

| Model | PID (VID 046d) | Per-key | Protocol family (`-tuk`) |
|---|---|---|---|
| G213 Prodigy | c336 | no, 5 zones | 3 (g213) |
| G410 Atlas Spectrum | c330 | yes | 1 (g810) |
| G413 | c33a | no (single colour) | 3 |
| G512 Carbon | c342 | yes | 1 |
| G513 Carbon | c33c | yes | 1 |
| G610 Orion | c333, c338 | yes (monochrome per key) | 1 |
| G810 Orion Spectrum | c331, c337 | yes | 1 |
| G815 LIGHTSYNC | c33f | yes | 4 (g815) |
| G910 Orion Spark/Spectrum | c32b, c335 | yes | 2 (g910) |
| G Pro | c339 | yes | 1 |

**G515: not supported** by g810-led (not in the table, no issues) and it would
not work with these byte strings anyway, because the G515 has 0x8071/0x8081 at
different indexes (see 1.4). **G502: not applicable**, g810-led is keyboard
only. Of the "G5xx" family, only the **G512** and **G513** keyboards use the
g810-led protocol.

The README describes testing an unknown keyboard with
`g810-led -dv 046d -dp c331 -tuk N -a 000000` where N selects the protocol
family; "for 2019 keyboard start with -tuk 4" [SRC: g810-led README].

### 1.2 Command bytes

All from `src/classes/Keyboard.cpp` [SRC: g810-led]. Short commands are padded
to 20 bytes, per-key frames to 64.

| Purpose | G410/G512/G513/G610/G810/G Pro | G213/G413 | G815 | G910 |
|---|---|---|---|---|
| Commit | `11 ff 0c 5a` | none ("non-transactional") | `11 ff 10 7f` | `11 ff 0f 5d` |
| Per-key colours (header, then `keyId R G B` × 14) | keys `12 ff 0c 3a 00 01 00 0e`, logo `11 ff 0c 3a 00 10 00 01`, indicators `12 ff 0c 3a 00 40 00 05`, media (G610/G810) `12 ff 0c 3a 00 02 00 05` | not supported | `11 ff 10 6c R G B k1..k13` (one colour, up to 13 keys) | keys `12 ff 0f 3d 00 01 00 0e`, logo `11 ff 0f 3a 00 10 00 02`, G-keys `12 ff 0f 3e 00 04 00 09` |
| Region colour | | `11 ff 0c 3a <region 1..5> 01 R G B` | | |
| Native effect (FX) | `11 ff 0d 3c <part> <effect> R G B period... ` | `11 ff 0c 3c ...` | `11 ff 0f 1c ...` after `11 ff 0f 5c 01 03 03` | `11 ff 10 3c ...` |
| Startup mode | `11 ff 0d 5a 00 01 <01 wave \| 02 colour>` | `11 ff 0d 5a ...` (G213) | | `11 ff 10 5e 00 01 <mode>` |
| Onboard / software mode | | | `11 ff 11 1a <01 board \| 02 software>` | |
| G-key / M-key / MR | | | `11 ff 0a 2b`, `11 ff 0b 1c`, `11 ff 0c 0c` | `11 ff 08 2e`, `11 ff 09 1e`, `11 ff 0a 0e` |

FX effect groups: off 0, colour 1, breathing 2, cycle 3, waves 4, ripple 5.
Parts: keys 0x00, logo 0x01. The storage byte selects none (0) or user slot
(1) which "can be recalled with backlight+7" [SRC: g810-led].

Short snippet (GPL-3.0, g810-led `Keyboard.cpp`, commit):

```cpp
case KeyboardModel::g410:
case KeyboardModel::g512:
case KeyboardModel::g513:
...
	data = { 0x11, 0xff, 0x0c, 0x5a };
```

### 1.3 What the indexes mean

Using the function tables of keyleds (`KEYLEDS_FEATURE_LEDS 0x8080`, functions
`GET_KEYBLOCKS=0 ... SET_LEDS=3, SET_BLOCK_LEDS=4, COMMIT=5`) and OpenRGB's
unified controller (0x8080 `SET_KEY_COLORS 0x30`, `FLUSH_LEDS 0x50`, 14 keys
per 0x12 frame; 0x8081 `SET_INDIVIDUAL 0x10 ... SET_SINGLE_VALUE 0x60,
FRAME_END 0x70`; 0x8071 `SET_EFFECT 0x10 ... SW_CONTROL 0x50`; 0x8100
`SET_ONBOARD_MODE 0x10`), the g810-led strings decode as [INF from SRC:
keyleds, OpenRGB, Solaar]:

| Model | Index | Feature | g810-led bytes |
|---|---|---|---|
| G410/G512/G513/G610/G810/G Pro | `0x0c` | 0x8080 PerKeyLighting | `0c 3a` SetKeyColors (fn 3), `0c 5a` commit (fn 5) |
| same | `0x0d` | 0x8070 ColorLEDEffects | `0d 3c` SetZoneEffect (fn 3), `0d 5a` fn 5 used for startup mode (OpenRGB names 0x8070 fn 5 a getter, so this one is uncertain) |
| G213/G413 | `0x0c` | 0x8070 ColorLEDEffects | both `0c 3a` "region" and `0c 3c` FX are fn 3 SetZoneEffect, region writes use effect 1 (static) |
| G815 | `0x0f` | 0x8071 RGBEffects | `0f 1c` SetRgbClusterEffect, `0f 5c 01 03 03` SetSWControl |
| G815 | `0x10` | 0x8081 PerKeyLighting v2 | `10 1c` SetIndividualRgbZones, `10 6c` SetRgbZonesSingleValue, `10 7f` FrameEnd |
| G815 | `0x11` | 0x8100 OnboardProfiles | `11 1a` SetOnboardMode |
| G815 | `0x0a`/`0x0b`/`0x0c` | 0x8010 G-keys, 0x8020 M-keys, 0x8030 MR | |

Key "types" in the 0x8080 header bytes (`00 01` keys, `00 02` media, `00 04`
G-keys, `00 10` logo, `00 40` indicators) match OpenRGB's 0x8080 key type
flags (KEYBOARD 0x01, CONSUMER 0x02, GKEYS 0x04, BUTTONS 0x08, LOGO 0x10,
OTHER 0x40) [SRC: OpenRGB `LogitechHIDPP20Controller.h`].

OpenRGB's G213 controller uses exactly the same `11 ff 0c 3a` and
`11 ff 0c 3c` strings [SRC: OpenRGB `LogitechG213Controller.cpp`].

### 1.4 Why hardcoded indexes are fragile

OpenRGB notes that "each link assigns different indices to the same features
(G515: RGBEffects at 0x09 wireless, elsewhere on USB)" [SRC: OpenRGB
`RediscoverFeatures`]. The Solaar feature dumps confirm it: the G515 LIGHTSPEED
TKL over its receiver has RGB Effects at index 9 and Per Key Lighting V2 at
10, the wired-only G515 TKL (c358) has them at 6 and 7 [ISSUE: Solaar #2791].
A robust tool resolves every index through IRoot `GetFeature`, as this project
does.

### 1.5 hidapi versus libusb builds of g810-led

[SRC: g810-led `makefile`, `Keyboard.cpp`]

* Default `LIB?=hidapi`, links `-lhidapi-hidraw`. Opens the hidraw node,
  writes, closes, for every packet. No driver detach. On failure it prints
  "Error: Can not write to hidraw, try with the libusb version".
* `make LIB=libusb`: detaches the kernel driver from interface 1, claims it,
  sends HID SET_REPORT control transfers (`wValue 0x0211`, or `0x0212` for
  frames longer than 20 bytes) and drains the interrupt endpoint `0x82`.
  The driver is reattached on close.

```cpp
// g810-led Keyboard.cpp (GPL-3.0), libusb build
if(libusb_kernel_driver_active(m_hidHandle, 1) == 1) {
	if(libusb_detach_kernel_driver(m_hidHandle, 1) != 0) {
...
if(libusb_claim_interface(m_hidHandle, 1) < 0) {
```

While a libusb client holds the interface, the hidraw node for it is gone, so
other tools lose the device for that time [INF].

* udev: v0.4.3 switched its rules to `TAG+="uaccess"` ("Rely on uaccess to
  control device access", PR #297), after an earlier world writable rule
  [SRC: g810-led `udev/g810-led.rules`; FORUM: CVE aggregator].

## 2. LIGHTSYNC

LIGHTSYNC is a marketing name, not a protocol. Logitech describes it as
"dynamic and customizable illumination that synchronizes to any content":
game integrations, a screen sampler, an audio visualiser, "approx. 16.8
million colors", and says "Advanced features require installation of G HUB
and LIGHTSYNC capability varies by products" [DOC: logitechg.com LIGHTSYNC
page]. It launched in March 2018 with the G560 speaker and G513 keyboard
[FORUM: PC Perspective].

| Product | LIGHTSYNC branding | Lighting on the wire |
|---|---|---|
| G213 Prodigy | "five individual lighting zones", LIGHTSYNC [DOC] | 0x8070, g810-led family 3 |
| G512 / G513 Carbon | "G512 Carbon LIGHTSYNC RGB", per key [DOC] | 0x8080 + 0x8070, g810-led family 1 |
| G815 LIGHTSYNC | in product name [SRC: g810-led README] | 0x8071 + 0x8081 + 0x8100 |
| G502 HERO | "LIGHTSYNC RGB" on the store page [Unverified, not fetched] | 0x8070 + 0x8100 **[HW]** |
| G515 TKL / G515 LIGHTSPEED TKL | "Lightsync RGB ... sync other Logitech G gear in G HUB" [FORUM: retailer] | 0x8071 + 0x8081 + 0x8040 + 0x8101 **[HW]** |

So "LIGHTSYNC" spans at least three generations of HID++ lighting features.
On Linux the name tells you nothing about which feature to use; feature
discovery does.

## 3. Windows SDKs

### 3.1 Logitech Gaming LED SDK ("LED Illumination SDK")

Download: `LED_SDK_9.00.zip` from the Logitech G developer lab, files dated
2018-09-20. The page says "G HUB Gaming Software is needed to work with the
SDKs". There is no separate "LIGHTSYNC SDK"; this is it [DOC]. The zip
contains `LogitechGamingLEDSDK.pdf`, `LogitechLEDLib.h`, a C# wrapper and
samples.

The SDK does not talk to devices. It loads `LogitechLed.dll` shipped with LGS
or G HUB and asks the running Logitech application to change the lighting
[DOC: SDK PDF "loads the dll LogitechLed.dll that ships with Logitech Gaming
Software 8.55+"]. So it says nothing about the USB protocol, but a lot about
the device capability model.

API surface (from `LogitechLEDLib.h`, "Copyright (C) 2011-2014 Logitech")
[SRC: header mirror]:

| Group | Functions |
|---|---|
| Init | `LogiLedInit`, `LogiLedInitWithName`, `LogiLedGetSdkVersion`, `LogiLedShutdown` |
| Config options (shown in LGS "Applet Manager") | `LogiLedGetConfigOptionNumber/Bool/Color/Rect/String/KeyInput/Select/Range`, `LogiLedSetConfigOptionLabel` |
| Whole device | `LogiLedSetTargetDevice`, `LogiLedSaveCurrentLighting`, `LogiLedSetLighting(r%, g%, b%)`, `LogiLedRestoreLighting`, `LogiLedFlashLighting`, `LogiLedPulseLighting`, `LogiLedStopEffects` |
| Per key | `LogiLedSetLightingFromBitmap` (21×6 keys, 4 bytes per key), `LogiLedSetLightingForKeyWithScanCode/HidCode/QuartzCode/KeyName`, `LogiLedSaveLightingForKey`, `LogiLedRestoreLightingForKey`, `LogiLedExcludeKeysFromBitmap`, `LogiLedFlashSingleKey`, `LogiLedPulseSingleKey`, `LogiLedStopEffectsOnKey` |
| Zones | `LogiLedSetLightingForTargetZone(DeviceType, zone, r%, g%, b%)` |

Target device classes:

```c
// LogitechLEDLib.h (Logitech)
#define LOGI_DEVICETYPE_MONOCHROME_ORD  0
#define LOGI_DEVICETYPE_RGB_ORD         1
#define LOGI_DEVICETYPE_PERKEY_RGB_ORD  2
#define LOGI_DEVICETYPE_MONOCHROME      (1 << LOGI_DEVICETYPE_MONOCHROME_ORD)
#define LOGI_DEVICETYPE_RGB             (1 << LOGI_DEVICETYPE_RGB_ORD)
#define LOGI_DEVICETYPE_PERKEY_RGB      (1 << LOGI_DEVICETYPE_PERKEY_RGB_ORD)
```

and `LogiLed::DeviceType { Keyboard=0x0, Mouse=0x3, Mousemat=0x4, Headset=0x8,
Speaker=0xe }` for the zone API.

What the SDK PDF says about capabilities [DOC: `LogitechGamingLEDSDK.pdf`]:

* Colours are percentages, 0 to 100 per channel, not 0..255.
* "Different devices have different capabilities. They range from full
  single-key RGB support to single color only." Single colour devices use
  "whichever of the R, G, and B values is the highest"; the G300 thresholds
  each channel at 50 %; the G11 has 3 levels at 33/66 %; the G610 is "single
  key monochrome" but accepts per-key RGB calls.
* "The G600 will work with the SDK only if set to Host mode through Logitech
  Gaming Software." This is the same onboard/host split that HID++ 0x8100
  exposes.
* "The SDK allows only one client to control backlighting at any given
  time ... the latest one will take over control."
* `LogiLedInit` "saves the current lighting for all connected and supported
  devices" and stops running effects; flash and pulse save, play, and restore.
* Zone tables: G213 0 = whole keyboard, 1..5 = left, centre, right, arrows,
  numpad; G900/G903 0 = DPI indicator, 1 = logo; G303 0 = side, 1 = logo;
  G403/G703 0 = scroll wheel, 1 = logo; PRO mouse 0 = logo and side;
  G633/G933 0 = logo, 1 = back stripe; G560 0..3 = four corners; POWERPLAY
  0 = logo. The PDF predates the G502 HERO and has no entry for it.
* Zone writes "will only affect devices with Zonal Lighting. This excludes
  keyboards with single key RGB support" and hit all connected devices of
  that type.

The mouse zone numbering (0 = DPI/primary, 1 = logo) is the same as the
0x8070 zone indexes on the G502 HERO in this project **[HW]**, which suggests
the SDK zone index maps straight onto the HID++ zone index [INF].

### 3.2 G HUB and Windows Dynamic Lighting

* On current Windows, Dynamic Lighting (HID LampArray) competes with G HUB:
  "When Dynamic Lighting is enabled on your devices, you cannot configure the
  lighting in G HUB", and it "is enabled by default the first time the feature
  is installed" [DOC: Logitech support 20684693000471, 20684629132823].
  OpenRGB mentions that the official app publishes HID LampArray as a virtual
  device for older boards [SRC: OpenRGB `LogitechControllerDetect.cpp`].
* SignalRGB users disable `LGHUBUpdaterService` and `logi_lamparray_service`,
  or "the keyboard lighting will turn off after a few minutes" [FORUM:
  SignalRGB forum, G915 X].
* G HUB uses HID++ software ID `0xD` [SRC: Solaar `base.py` comment].

## 4. G HUB / LGS behaviour that a Linux tool has to replace

### 4.1 Onboard memory mode versus software mode

* G HUB's device settings have an "ON-BOARD MEMORY MODE" switch; "You won't be
  able to make any changes to your device through G HUB while ON-BOARD MEMORY
  MODE is enabled" [DOC: Logitech support 360057433714].
* G813/G815 quick start guide: onboard profiles M1..M3 are used "when the
  keyboard is in Onboard mode (selected from G HUB software, or when G HUB
  software is off or not installed)", and "User-stored effects are saved to
  the keyboard by G HUB software" [DOC: G815 QSG PDF].
* Onboard effects can be cycled with Brightness + number keys; "effects and
  settings saved in onboard memory will be overwritten by G HUB settings"
  [DOC: Logitech support 7175302203415]. The firmware "needs to have at least
  one lighting effect stored" [DOC: support 36580165753367].
* An OpenRGB developer: "By default GHub switches keyboard to managed mode
  which disables all settings saved into the keyboard's flash memory ... The
  mode resets on sleep, disconnect or power cycle back to built-in memory
  mode" [ISSUE: OpenRGB MR !713].

On the wire, "software mode" is 0x8100 `SetOnboardMode(2)` (g810-led G815:
`11 ff 11 1a 02`) or, on devices with 0x8101 instead of 0x8100 such as the
G515, 0x8101 function 6 with mode `0x05` (host) and `0x03` (onboard) [SRC:
Solaar `settings_templates.py`, OpenRGB `SetHostMode`]. Lighting set in host
mode is volatile; lighting written into the onboard profile survives power
cycles without software [SRC: libratbag `driver-hidpp20.c`, this project's
`lighting/onboard.go`].

Practical consequence: a tool that wants a colour to "stick" without a daemon
must write it into the onboard profile. A tool that drives lighting live must
re-claim the device after every replug, sleep and resume.

### 4.2 Idle, dim and sleep

* LGS had a "Lighting Sleep Timer": "specify the number of minutes of
  inactivity after which your mouse's lighting will automatically turn off"
  [DOC: Logitech support 360023262014, G402].
* G HUB lets you change lighting after the mouse "has been inactive for x
  amount of minutes" [FORUM: Best Buy Q&A, G703]. We found no official page
  naming the exact G HUB controls ("dim after inactivity", "lighting effects
  on idle") [Unverified].
* On 0x8071 devices the firmware does dim and sleep on its own only while it
  owns the lighting. Once the host holds software control, OpenRGB's comment
  says the "firmware will NOT dim or sleep autonomously; it only emits idle
  events and expects the host to act" [SRC: OpenRGB
  `LogitechHIDPP20Controller.cpp`]. OpenRGB and Solaar (≥ 1.1.20,
  `rgb_power.py`) both implement ACTIVE → DIMMING → IDLE → SLEEPING state
  machines driven by 0x8071 user activity events, with power settings in
  0x8071 functions 7 and 8 [SRC: OpenRGB, Solaar].
* OpenRGB's defaults: `dim_when_idle=false, dim_brightness=50,
  idle_timeout_s=60, allow_sleep=false, sleep_timeout_s=300`, and "G515
  default observed from vendor app: ... seconds=0x1E (= 30 seconds dim ramp
  before sleep)" [SRC: OpenRGB `LogitechHIDPP20IdleSettings.h`,
  `LogitechHIDPP20Controller.cpp`].
* Solaar maintainer: "The G515 has three power modes: on, save, and off. By
  default it switches to save after 60 seconds without any input and then off
  after another 300 seconds. When the power mode changes to on or save and
  onboard profiles are active, the power mode lighting for the active profile
  is applied" [ISSUE: Solaar #2791, pfps 2026-04-24].
* **[HW]** In this project the G515 TKL wired dims and turns off on idle in
  firmware, with the timers configurable and storable in the device; the G502
  HERO has no idle dimming (README "Dim/off on idle: yes, in firmware" / "no").

## 5. Linux tools

### 5.1 Comparison

| Tool | Status (2026-10) | G502 HERO (c08b) | G515 (c358 wired, c355 LS wired, 40b4 via receiver c547, b389 BT) | Notes |
|---|---|---|---|---|
| **Solaar** | 1.1.20 (2026-06-28), master active | Yes. Descriptor "G502 SE Hero Gaming Mouse" [SRC: Solaar `descriptors.py`]. 0x8070 settings `led_control` and `led_zone_N`; Onboard Profiles setting can pick a profile or disable (host mode); `solaar profiles` dumps and loads profiles. No button remapping (no 0x1b04) [ISSUE #2914] | Yes, the most complete on Linux. `rgb_control` (0x8071), `rgb_zone_*`, Per-key Lighting (0x8081), Brightness (0x8040), and since 1.1.20 idle/sleep timeouts, idle effect and software power management (PR #3149). The wired-only c358 shows up as "G515 TKL" [ISSUE #2791] | LED control "on" claims SW control and puts 0x8101 into host mode; "off" releases it, and since PR #3221 Solaar then writes nothing so OpenRGB etc. can drive the LEDs |
| **OpenRGB** | 1.0 (2026-09-11), master active | Yes, legacy Lightspeed controller: `REGISTER_HID_DETECTOR_IPU("Logitech G502 HERO Gaming Mouse", DetectLogitechWired, ..., 0xC08B, 1, 0xFF00, 2)` [SRC]. Modes Off, Direct, Static, Spectrum Cycle, Breathing; Direct uses 0x8070 SetSwControl only; "Saving not supported" [ISSUE #3710] | Yes (c355, 40b4, b389) through the unified HID++ 2.0 controller (MRs !3270, !3352, merged 2026-06; BT in !3496). c358 is not listed by PID but the VID-wide HID++ 2.0 detector should probe it [SRC; INF] | Unified controller sets host mode via 0x8101 (`0x05`) or 0x8100 `SetOnboardMode(2)`. Quirk `{ 0x40B4, 0xC355, HIDPP20_QUIRK_FADE_ACCEPTS_WRITES }` [SRC] |
| **libratbag / Piper** | libratbag 0.18 (2024-09), master active; Piper 0.8 | Yes. `data/devices/logitech-g502-hero.device`: `DeviceMatch=usb:046d:c08b`, `Driver=hidpp20`, `Quirk=INDEX_OFFSET` [SRC]. DPI, buttons, macros, LEDs written into the onboard profile (persistent) | No (mice only, no device file) | Forces onboard mode on load; many "changes don't apply" reports caused by an active profile index bug |
| **g810-led** | v0.4.3, dormant since 2022 | No (keyboards only) | No | G512/G513 yes |
| **keyleds** | v1.1.1 (2020), last commit 2021 | No | Probably not: only 0x8070 and 0x8080, the G515 has 0x8081 [SRC: keyleds `features.h`; INF] | Abandoned; G410/G513/G610/G810/G910/G Pro era |
| **logiops (logid)** | v0.3.5 (2024-09) | No lighting (DPI, SmartShift, HiresScroll, RemapButton, ThumbWheel only) [SRC] | No | MX oriented |
| **ckb-next** | | Not applicable | Not applicable | Corsair only |
| **"lgogo"** | not found | | | The only match is an empty, unrelated GitHub repo; probably a misremembered name [Unverified] |
| **federico-duppa/logitech-g515-onboard** | created 2026-09-10 | No | c355 over USB only. Python, writes a solid colour into the onboard user lighting profile so it survives power cycles. README warns against the wired-only G515, G515 RAPID and G915. Tested on firmware MPK 25.00.B0010 | Experimental flash writer |
| x00FB/g515-tweaks | 2026-10 | No | No lighting, hwdb Fn-lock remap | Lists the IDs: BT b389 and wired c355 on `hid-generic`, receiver c547 → 40b4 on `logitech-hidpp-device` |
| G HUB under Wine | | Not workable per reports | same | Users use a Windows VM with USB passthrough to save lighting to onboard memory [ISSUE: Solaar #1766, #2791] |
| **this project (logirgb)** | | 0x8070 + 0x8100, 2 zones **[HW]** | c358: 0x8071 + 0x8101 + 0x8040, firmware idle dim/off **[HW]** | hidraw only, no daemon |

### 5.2 Feature lists from `solaar show`

G515 TKL wired-only, 046d:c358, HID++ 4.2, firmware U1 67.00.B0006, model ID
`40B4C3580000` (posted by everslick in Solaar #2791, 29 features; the dev
build printed byte-swapped IDs in braces, names are right) [ISSUE]:

```
 0: ROOT {0000}            1: FEATURE SET {0001}      2: DEVICE FW VERSION {0003} V6
 3: DEVICE NAME {0005} V3  4: CONFIG CHANGE {0020}    5: unknown:0011
 6: RGB EFFECTS {8071} V4  7: PER KEY LIGHTING V2 {8081} V0
 8: unknown:1B10           9: unknown:4523           10: KEYBOARD LAYOUT 2 {4540} V1
11: BRIGHTNESS CONTROL {8040} V0                     12: unknown:8101 (PROFILE MANAGEMENT)
13: unknown:1B05          14: unknown:8051           15: DFUCONTROL {00C3}   16: DFU {00D0} V3
17..28: DEVICE RESET 1802, 1803, 1807, 1817, OOBSTATE 1805, 1890, 1E00, 1E02, 1602, 1EB0, 1801, 18B0
```

G515 LIGHTSPEED TKL through the receiver (WPID 40B4) has the same lighting
features at different indexes (0x8071 at 9, 0x8081 at 10, 0x8040 at 14, 0x8101
at 15) plus wireless status 0x1D4B and unified battery 0x1004 [ISSUE: Solaar
#2791; also `docs/devices/` in Solaar].

Neither G515 variant has 0x8100; host/onboard switching is through 0x8101
[ISSUE, SRC: Solaar].

### 5.3 G515 variants and IDs

| Variant | USB / BT ID | Notes |
|---|---|---|
| G515 TKL (wired only) | 046d:c358 | **[HW]** 3 hidraw nodes, HID++ on interface 2, usage page 0xFF00 |
| G515 LIGHTSPEED TKL, cable | 046d:c355 | OpenRGB quirk entry |
| G515 LIGHTSPEED TKL, receiver | receiver c547, device 046d:40b4 | DJ child node via `hid-logitech-dj` |
| G515 LIGHTSPEED TKL, Bluetooth | 046d:b389 | usage page 0xFF43 usage 0x0202, long reports only [ISSUE: OpenRGB MR !3496] |
| G515 RAPID TKL | 046d:c363 | [ISSUE: OpenRGB #5521] |

## 6. Known bugs and quirks

### 6.1 G502 HERO

| Report | Date / status | Summary |
|---|---|---|
| [HW] this project | | The 0x8070 zone 0 ("primary") accepts settings but has no visible effect; zone 1 is the logo. The DPI LEDs are a blue status light the firmware only shows when the DPI/profile buttons are pressed |
| Solaar #1766 "G502 Lightspeed Mouse LED-Control" | 2022-09, closed 2024-02 | LED control added (PR #2245). Maintainer: "led_zone_1 is the lights on the side. led_zone_2 is the logo", the side LEDs "are off unless some action on the mouse turns them on", onboard profiles contain LED data. (Solaar numbers zones by location, not by HID++ index) |
| Solaar #2313 | 2024-02, closed | "Disable" missing from Onboard Profiles on the G502 SE Hero because its profiles are in ROM; fixed by PR #2314 |
| Solaar #2389, #2843 | 2024-2025, closed | With onboard profiles enabled Solaar re-reads the profile and overrides DPI; "The solution is to turn off onboard profiles". Solaar docs: LED and DPI changes while a profile is active "are likely to only be temporary" |
| Solaar #2914 | 2025-07, closed | No button remapping on the G502 SE Hero; only by editing onboard profiles |
| Solaar #3247 / #3335 | 2026, open | LED controls disappeared after the LED code rework / on Debian's 1.1.14 (#3335 is a G502 Lightspeed); fix by cleaning `led_zone_*` from `config.yaml` |
| Solaar #3065 / #3314 | closed | `solaar config` could not set LED zone settings from the CLI (still a TypeError in 1.1.20 per #3314) |
| libratbag #1181 "Cant control DPI leds" | 2021-06, open | Piper accepts DPI LED changes but they stay off; became controllable after configuring once in Logitech software |
| Piper #405 "G502 Hero dpi LED always on" | 2019-10, open | No "always on" option for the DPI LEDs; fixed via LGS "DPI Always ON"; changes revert to cycle mode |
| libratbag #1606 "incorrect active profile" | 2024-04, closed | Active profile index advances on ratbagd restart; root cause of many "changes don't apply" reports (#1522, #1628) |
| libratbag #1717 | 2025-06, duplicate of #1606 | `ratbagctl ... led 1 set mode off` only restarts breathing unless profile 0 is given |
| Piper #1094 "g502 ... ignores any changes from piper" | 2026-01, open | Workaround `ratbagctl "<dev>" profile active set 0`; one report of wrong colours (dark red shows as pink) |
| Piper #1117 "Can't change lighting for G502" | 2026-07, open | c08b, ratbagd 0.18; works with ratbagctl but not Piper |
| libratbag #1898 | 2026-09, open | Disabling a button on 0x8100 profiles writes a MACRO binding instead of NOOP |
| OpenRGB #1776 | 2021, closed | Spectrum Cycle and Breathing broken on the wired G502 Hero; fixed by MR !748 |
| OpenRGB #3710 | 2023, open | "Saving Not Supported" for the G502 |
| OpenRGB #5920 | 2026-09, open | OpenRGB 1.0 shows two G502 Hero devices; only the first works and selecting the duplicate lags. Probably the legacy detector and the new VID-wide HID++ 2.0 detector both match interface 1 [INF] |
| OpenRGB MR !3661 "Start wired G502 HERO in Direct mode" | 2026-09, open | The controller "sets OFF by default", so hot-plug does not restore the user's lighting |

On the claim "OpenRGB switches the G502 to host mode and breaks the DPI
buttons": we could not confirm it. The legacy controller that handles c08b
only sends 0x8070 SetSwControl and has no 0x8100 call [SRC: OpenRGB
`LogitechProtocolCommon.cpp`]. The unified HID++ 2.0 controller does call
0x8100 `SetOnboardMode(2)`, and OpenRGB added `KEEP_ONBOARD_MODE` for the G915
TKL because "host mode breaks the onboard-profile-mapped F-row" [SRC]. The
same mechanism plausibly applies to a mouse whose DPI and profile buttons are
mapped in the onboard profile: in host mode (0x8100 mode 2) the onboard
profile, including its DPI table and button bindings, is not active [INF].
We did not test host mode effects on the G502 HERO buttons on hardware.
See `hidpp-transport.md` section 10.4 for how ratbagd forces onboard mode
back on.

### 6.2 G515

| Report | Date / status | Summary |
|---|---|---|
| Solaar #2791 "Leds config is reset after some time on logitech G515 lightspeed tkl" | 2025-01, closed 2026-05 as fixed | After about 60 s idle the keyboard falls back to its onboard lighting, without any notification. Cause: power mode change re-applies the active onboard profile's lighting. Fix (PR #3149, in 1.1.20): 0x8101 host mode `0x05`, 0x8071 SetSWControl, software power management. The c358 `solaar show` above comes from this issue |
| Solaar #3241 "Static LED mode ignores color selection, always shows yellow" | 2026-05, closed | The per-key map (0xffa500 on every key) overrides the zone colour; set Per-key Lighting to "ignore" when using zone effects |
| Solaar #3310 "first wake key after LED sleep is dropped" | 2026-08, open | After Solaar's software sleep the first key press is swallowed. Maintainer: the 0x8081 host/onboard switch resets keyboard state and G HUB on Windows shows the same, so it is firmware; workaround is no software sleep |
| Solaar PR #3221 | merged, 1.1.20 | With `rgb_control` off Solaar sends no claim and no writes, so other tools can drive the LEDs |
| Solaar PR #3257 | open | Per-key state loss and a CLI TypeError in `solaar config <dev> per-key-lighting`, found on a G915 TKL, same 0x8081 path |
| OpenRGB #5893 "G515 LS TKL - lighting turns off and cannot be re-enabled" | 2026-09, open | Through the receiver the backlight goes dark when OpenRGB starts; only a power cycle restores it |
| OpenRGB #4238, #5457, #5553 (c358), #5521 (RAPID c363) | 2024-2026 | Device requests; #5553 and #5521 closed |
| OpenRGB MRs !3270, !3352, !3496, !3547 | merged 2026 | Unified HID++ 2.0 controller (0x8071, 0x8081, 0x8100/0x8101, power state machine), Bluetooth, per-key effect fixes; "284 per-key zones" on the G515 |
| OpenRGB `FADE_ACCEPTS_WRITES` quirk | source | By default OpenRGB stops sending frames while the device sleeps; the G515 (c355/40b4) opts out because its "firmware accepts host frames during sleep fade without waking". Keyed on c355/40b4 only, so the wired-only c358 does not get it [SRC; INF] |
| OpenRGB comment | source | G515 USB flow control: "~20 writes then the tail drops" [SRC] |

Per-key specifics: 0x8081 cannot be read back, so Solaar keeps its own copy
[ISSUE #2791, PR #3257]; the device reports phantom keys 47, 97, 99..103 and
254 [SRC: Solaar `ui/perkey/binding.py`]; Solaar disables 0x8071 wave effects
4 and 5 for 40b4 because the board has "no edge-radiating wave geometry"
[SRC: Solaar `hidpp20.py`].

## 7. Takeaways for this project

* Feature indexes differ between transports of the same G515 (6/7 wired vs
  9/10 via receiver), so never hardcode them the way g810-led does.
* G512/G513 users would be served by 0x8080 + 0x8070 at indexes found through
  IRoot; those keyboards put HID++ on usage page 0xFF43, which
  `hidpp/discover.go` does not match today (see `hidpp-transport.md` 6.2).
* For persistent lighting without a daemon, writing into the onboard profile
  (0x8100 on the G502 HERO, 0x8101 on the G515) is the only route; Solaar,
  OpenRGB and this project's `--save` all end up there or in host mode.
* In host/software mode the host owns idle handling on 0x8071 devices.
  Expect the same "first key after sleep" artefact Solaar #3310 reports if a
  software sleep toggles host/onboard mode.
* Expect Solaar (if LED control is on), ratbagd (forces onboard mode on load)
  and OpenRGB (claims software mode) to fight over the same device. Document
  that users should turn off LED control in the other tools.

## Sources

g810-led:

* Repository and README: https://github.com/MatMoul/g810-led
* Keyboard.h: https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.h
* Keyboard.cpp: https://github.com/MatMoul/g810-led/blob/master/src/classes/Keyboard.cpp
* udev rules: https://github.com/MatMoul/g810-led/blob/master/udev/g810-led.rules
* CVE note on old udev rule: https://releasealert.dev/github/MatMoul/g810-led/cves

keyleds and logiops:

* keyleds features.h: https://github.com/keyleds/keyleds/blob/master/libkeyleds/include/keyleds/features.h
* logiops: https://github.com/PixlOne/logiops

Logitech:

* Developer lab and LED SDK: https://www.logitechg.com/en-us/innovation/developer-lab.html
* LED SDK zip: https://www.logitechg.com/sdk/LED_SDK_9.00.zip
* LogitechLEDLib.h mirror: https://github.com/ThirteenAG/WidescreenFixesPack/blob/master/includes/LED/LogitechLEDLib.h
* SDK docs mirror: https://dcspy.readthedocs.io/en/stable/reference/sdk/led_sdk/
* LIGHTSYNC: https://www.logitechg.com/en-us/discover/technology/lightsync-rgb
* G512 product page: https://www.logitechg.com/en-ae/shop/p/g512-mechanical-gaming-keyboard
* G213 product page: https://www.logitechg.com/en-ie/shop/p/g213-rgb-gaming-keyboard
* G502 HERO product page: https://logitech.com/en-kw/shop/p/g502-hero-gaming-mouse
* G815/G813 quick start guide: https://logitech.com/assets/65841/3/g815-lightsync-rgb-mechanical-gaming-keyboard.pdf
* On-board memory mode: https://support.logi.com/hc/en-ca/articles/360057433714-How-to-enable-On-Board-Memory-Mode-to-use-Onboard-Memory-Manager
* Onboard lighting effects: https://support.logi.com/hc/en-au/articles/7175302203415-How-do-I-switch-to-another-factory-programmed-lighting-effect
* Managing LIGHTSYNC effects: https://support.logi.com/hc/en-001/articles/36580165753367
* Dynamic Lighting and G HUB: https://support.logi.com/hc/articles/20684693000471 and https://support.logi.com/hc/articles/20684629132823
* LGS lighting sleep timer (G402): https://support.logi.com/hc/en-au/articles/360023262014-Enable-and-disable-illumination-on-the-G402-gaming-mouse

Press, forums, retailers:

* LIGHTSYNC launch: https://pcper.com/2018/03/logitech-announces-g560-speakers-and-g513-keyboard-with-lightsync/
* G515 TKL retailer page: https://jbhifi.co.nz/products/logitech-g515-tkl-wired-gaming-keyboard
* G HUB inactivity lighting Q&A: https://www.bestbuy.com/site/questions/logitech-g703-lightspeed-wireless-optical-gaming-mouse-black/6333840/question/f8109fb5-355d-38fd-a4b4-201193f3a84b
* SignalRGB forum, G915 X and Logitech services: https://forum.signalrgb.com/t/logitech-g915-x-rgb-issues-in-wireless-lightspeed-mode-and-after-battery-idle/3970
* Aurora issue on LogitechLed.dll: https://github.com/antonpup/Aurora/issues/1953

Solaar:

* Repository: https://github.com/pwr-Solaar/Solaar
* Capabilities docs (onboard profiles note): https://pwr-solaar.github.io/Solaar/capabilities/
* #1766: https://github.com/pwr-Solaar/Solaar/issues/1766
* #2313: https://github.com/pwr-Solaar/Solaar/issues/2313
* #2843: https://github.com/pwr-Solaar/Solaar/issues/2843
* #2914: https://github.com/pwr-Solaar/Solaar/issues/2914
* #3247: https://github.com/pwr-Solaar/Solaar/issues/3247
* #3335: https://github.com/pwr-Solaar/Solaar/issues/3335
* #3314: https://github.com/pwr-Solaar/Solaar/issues/3314
* #2791: https://github.com/pwr-Solaar/Solaar/issues/2791
* PR #3149: https://github.com/pwr-Solaar/Solaar/pull/3149
* #3241: https://github.com/pwr-Solaar/Solaar/issues/3241
* #3310: https://github.com/pwr-Solaar/Solaar/issues/3310
* PR #3221: https://github.com/pwr-Solaar/Solaar/pull/3221
* PR #3257: https://github.com/pwr-Solaar/Solaar/pull/3257

OpenRGB:

* Repository: https://gitlab.com/CalcProgrammer1/OpenRGB
* LogitechControllerDetect.cpp: https://gitlab.com/CalcProgrammer1/OpenRGB/-/blob/master/Controllers/LogitechController/LogitechControllerDetect.cpp
* LogitechHIDPP20Controller.h: https://gitlab.com/CalcProgrammer1/OpenRGB/-/blob/master/Controllers/LogitechController/LogitechHIDPP20Controller/LogitechHIDPP20Controller.h
* LogitechG213Controller.cpp: https://gitlab.com/CalcProgrammer1/OpenRGB/-/blob/master/Controllers/LogitechController/LogitechG213Controller/LogitechG213Controller.cpp
* MR !713 (G HUB managed mode note): https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/713
* MR !3270: https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/3270
* MR !3352: https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/3352
* MR !3496: https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/3496
* MR !3547: https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/3547
* MR !3661: https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/3661
* #1776: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/1776
* #3710: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/3710
* #5920: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/5920
* #5893: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/5893
* #4238: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/4238
* #5457: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/5457
* #5553: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/5553
* #5521: https://gitlab.com/CalcProgrammer1/OpenRGB/-/work_items/5521

libratbag / Piper:

* libratbag: https://github.com/libratbag/libratbag
* G502 HERO device file: https://github.com/libratbag/libratbag/blob/master/data/devices/logitech-g502-hero.device
* #1181: https://github.com/libratbag/libratbag/issues/1181
* #1606: https://github.com/libratbag/libratbag/issues/1606
* #1717: https://github.com/libratbag/libratbag/issues/1717
* #1898: https://github.com/libratbag/libratbag/issues/1898
* Piper #405: https://github.com/libratbag/piper/issues/405
* Piper #1094: https://github.com/libratbag/piper/issues/1094
* Piper #1117: https://github.com/libratbag/piper/issues/1117

Other G515 projects:

* logitech-g515-onboard: https://github.com/federico-duppa/logitech-g515-onboard
* g515-tweaks: https://github.com/x00FB/g515-tweaks
* g515-rgb-controller (Windows): https://github.com/Nabsflix/g515-rgb-controller
