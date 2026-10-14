# 0x8101 ProfileManagement

Research notes on HID++ 2.0 feature `0x8101` (ProfileManagement), the
file-based successor of `0x8100` OnboardProfiles found on newer Logitech G
keyboards. It stores the onboard profile as checksummed "files" that are
staged in a RAM buffer, committed to flash, and then activated.

Logitech has not published a specification. Almost everything public comes
from one G515 LIGHTSPEED TKL owner capturing G HUB traffic (Ken Sanislo,
`ksanislo`, in Solaar PR #3149 and in his OpenRGB controller), from the
`logitech-g515-onboard` tool built on that work, and from this project's own
hardware tests.

## Legend

- **[HW]** verified on hardware by this project: G515 TKL wired,
  `046d:c358`. Raw log in [hardware-log.md](hardware-log.md).
- **[g515-onboard]** observed on hardware by the logitech-g515-onboard
  project (G515 LIGHTSPEED TKL `046d:c355`, firmware MPK 25.00.B0010), from
  its `PROTOCOL.md` and `g515.py`.
- **[ksanislo]** G HUB captures and hardware tests by Ken Sanislo, posted in
  Solaar PR #3149 (G515 LIGHTSPEED TKL).
- **[Solaar]**, **[OpenRGB]**: from that tool's source code.
- **[unverified]** our inference, or a single unconfirmed claim.

Function numbers are the high nibble of the HID++ function byte (`fn6` is
sent as `0x6?`). All multi-byte fields are big endian.

## Which devices have it

| Device | USB PID / WPID | 0x8101 index | Source |
|---|---|---|---|
| G515 TKL (wired) | `c358` | discovered at runtime | [HW] |
| G515 LIGHTSPEED TKL | `c355` / `40B4` | `0x0e` on one unit, `0x0f` on others | [g515-onboard], Solaar device dump `docs/devices/G515 LS TKL 40B4.text` |
| G915 X LIGHTSPEED | `c356` / `40B5` | `0x0f` | Solaar issues #3328, #3137, #3205 |

Devices from the same era that use `0x8100` instead (so not 0x8101): G502 X
PLUS (`0x8100` at index 11, Solaar #3205), G815, G903, PRO X 2 Superlight /
Superstrike / DEX (`0x8100` with profile formats 6 and 7, see the 0x8100
notes). So far 0x8101 has only been seen on keyboards (G515 family, G915 X).
The feature index differs between firmware versions: always look it up with
the root feature.

Feature set seen next to it on the G515 LS TKL: `0x8071` RGBEffects, `0x8081`
PerKeyLighting v2, `0x8040` BrightnessControl, `0x1b05`, `0x8051`
LogiModifiers, `0x4540` KeyboardLayout2.

## Functions

| fn | Name used here | Request | Reply | Source |
|---|---|---|---|---|
| 0 | GetInfo | none | 16 bytes, mostly unknown | [HW] |
| 1 | unknown | | | |
| 2 | StartWrite (buffer start) | `[length u16]` | none | [g515-onboard] [HW] |
| 3 | WriteBuffer (append) | up to 16 data bytes | none | [g515-onboard] [HW] |
| 4, 5 | unknown | | | |
| 6 | GetSetMode | `[flags]` | `[mode, active file u16]` | [ksanislo] [OpenRGB] [Solaar] [HW] |
| 7 | Save (commit buffer to flash) | `[address u16, length u16, checksum u32]` | none | [g515-onboard] [HW] |
| 8 | Load (flash to buffer) | `[address u16, length u16]` | none, or error | [g515-onboard] [OpenRGB] [HW] |
| 9 | Configure (activate) | `[81 01 01 00 01 00 63, crc32 u32]` for the G515 user profile | none | [ksanislo] [g515-onboard] [HW] |
| A, B | unknown | | | |
| C | ReadBuffer | `[offset u16]` | 16 bytes of the buffer | [g515-onboard] [OpenRGB] [HW] |
| D-F | unknown | | | |

The names are descriptive, not Logitech's. OpenRGB names them
`FN_8101_GET_SET_MODE` (0x60), `FN_8101_LOAD` (0x80, "load(partition,
sector, size)") and `FN_8101_READBUFFER` (0xC0). ksanislo calls fn6
`getSetMode` and fn9 `configure`, and mentions a `resetConfiguration` done
with `configure(fileID=0xFF00, size=0, hash=0)`.

No 0x8101 events are known. ksanislo mentioned in PR #3149 that idle and
activity reports come "out of the management hid, so long as we're properly
diddling feature 0x8101 into the right flags", but those reports belong to
`0x8071` (onUserActivity), not 0x8101.

## GetInfo (fn0)

**[HW]** G515 TKL wired:

```
01 01 00 80 05 80 7f 7f 07 14 ff ff ff 20 01 07
```

No source decodes this. Possible readings, all **[unverified]**: `00 80` a
128-byte buffer size (the 99-byte profile fits), the `ff ff ff` a
"no limit" or unused field. Logging this reply on other 0x8101 devices
(G915 X) would help.

## Read and write sequences

### Reading a file

```
fn8 [addr hi, addr lo, len hi, len lo]     load file into the buffer
fnC [00 00] -> 16 bytes                     read buffer at offset 0
fnC [00 10] -> 16 bytes                     offset 16 ...
```

**[HW] [g515-onboard]** Loading a user file that was never written fails
with HID++ error `0x04` (HW_ERROR). Both g515.py and this project treat that
as "not present" for user addresses only.

OpenRGB's `ReadActiveProfileSector` does exactly this for `0x0101` with
length `0x63` and logs a hex dump at debug level. It calls the two address
bytes "partition 0x01 (NVS/flash)" and "sector 0x01 (active profile)". That
label is misleading: `0x0101` is the factory profile, which is only the
active one while no user profile exists (see addressing below).

### Writing a file

```
fn2 [len hi, len lo]                        start a buffer of len bytes
fn3 [16 bytes]                              append, repeat (last chunk shorter)
fnC [offset]                                optional: read back the buffer
fn7 [addr u16, len u16, checksum u32]       commit the buffer to flash
fn8 [addr u16, len u16] + fnC ...           load the file back and compare
```

**[HW] [g515-onboard]** Both implementations verify the staged buffer with
fnC before fn7 and verify flash with fn8 + fnC afterwards. g515.py only allows
writes to `0x0000` and `0x0008`; this project does the same.

The checksum passed to fn7 is the file's CRC32 (standard IEEE / zlib CRC-32,
stored big endian):

- for the profile: CRC32 of all 99 bytes,
- for the directory entry: the entry's own CRC field, which is CRC32 of
  entry bytes 4-15 (not of all 16 bytes).

Whether the firmware checks the fn7 checksum, and what happens on a
mismatch, is **[unverified]**.

The two flash writes (profile, then directory) are not atomic. Write the
profile first; the old directory still describes the old profile until the
directory write lands. Back up all four files before writing.

## File addressing

| Address | Size | Contents | Writable |
|---|---|---|---|
| `0x0000` | 16 | user directory entry | yes |
| `0x0008` | 99 | user profile (G515) | yes |
| `0x0100` | 16 | factory directory entry | no (never tried) |
| `0x0101` | 99 | factory profile | no (never tried) |

The high byte looks like a partition (`0x00` user, `0x01` factory, mirroring
0x8100's user/ROM split) and the low byte a slot. ksanislo describes the
active-file field returned by fn6 as `(partition, sector)`. He also says
effect cards (multi-color or animated lighting saved by G HUB) are per-key
frame recordings "saved into another partition in the flash"; their
addresses are unknown.

Physical slot versus logical file ID: **[HW] [g515-onboard]** the user
profile is stored at `0x0008`, but after activation fn6 reports the active
file as `00 01` (logical file ID 1), not `00 08`. ksanislo reported `00 08`
"when a user-modified profile is loaded" and `01 01` while the factory profile
runs, and `ff 00` for "no profile active" after a reset. Solaar maintainer
pfps saw `03 01 01` on a G515 that had never been configured. G HUB probably
activates with a different file ID than the one used here; the exact rule is
**[unverified]**.

## Directory entry (16 bytes)

| Offset | Size | Factory (`0x0100`) | User (`0x0000`, this project) | Meaning |
|---|---|---|---|---|
| 0 | 4 | `5e 38 09 25` | recomputed | CRC32 of bytes 4-15, big endian |
| 4 | 2 | `01 01` | `01 01` | unknown, constant |
| 6 | 2 | `81 01` | `81 01` | the feature ID 0x8101, maybe a file type tag **[unverified]** |
| 8 | 1 | `40` | `40` | unknown |
| 9 | 1 | `63` | `63` | 99, likely the profile length **[unverified]** |
| 10 | 4 | `99 6d ec 01` | CRC32 of the profile | profile checksum |
| 14 | 1 | `01` | `08` | profile location (slot low byte: `0x0101` versus `0x0008`) |
| 15 | 1 | `ff` | `ff` | terminator or padding |

**[HW]** Factory directory read from the G515 TKL wired:
`5e 38 09 25 01 01 81 01 40 63 99 6d ec 01 01 ff`. Its own CRC (bytes 0-3)
validates. A user directory already present from an earlier tool was
`eb aa 70 c3 01 01 81 01 40 63 9d f9 3c 91 08 ff`.

**Unresolved [HW] [g515-onboard]:** the factory directory's profile
checksum `99 6d ec 01` does **not** equal the CRC32 of the 99 factory profile
bytes (`14 ec 98 48` on our unit). Both projects saw this independently. For
user profiles a freshly computed CRC32 works. Maybe the factory hash covers a
different length or the factory profile was patched after the hash was made.

Building the user entry (both g515.py and this project): clone the factory
entry, put CRC32(profile) at 10-13, set byte 14 to `0x08`, recompute the CRC
at 0-3. Both validate bytes 4-9 equal `01 01 81 01 40 63` and byte 15 is
`ff` before writing, and refuse otherwise.

```python
# logitech-g515-onboard g515.py (GPL-3.0), build_directory()
result[10:14] = zlib.crc32(profile).to_bytes(4, "big")
result[14] = 8
result[:4] = zlib.crc32(result[4:]).to_bytes(4, "big")
```

## G515 profile layout (99 bytes)

| Offset | Size | Field | Notes |
|---|---|---|---|
| 0 | 2 | header `01 01` | both tools require it; maybe format and version |
| 2 | 2 | idle timeout, seconds | factory `00 3c` (60). Firmware dims to record 3 after this. **[HW]** |
| 4 | 2 | sleep timeout, seconds | factory `01 2c` (300). Firmware turns lighting off after this. **[HW]** |
| 6 | 24 | name | all `ff` in the factory profile; encoding unknown (UTF-16LE like 0x8100 would give 12 characters) |
| 30 | 11 | lighting record 0 | factory: `18 ff 00 81 80 aa ff 00 00 64 00` |
| 41 | 11 | lighting record 1 | factory: zero |
| 52 | 11 | lighting record 2 | factory: `19 ff 00 81 80 aa ff 00 00 64 00` |
| 63 | 11 | lighting record 3 | factory: zero |
| 74 | 1 | lighting flags | factory `03`; `00` to use records 1 and 3 |
| 75 | 16 | "disabled-control bitmap" (g515-onboard name) | factory `00 00 00 00 00 00 00 00 10 00 01 00 00 00 00 ff`, preserved |
| 91 | 8 | unknown | factory `ff`, preserved |

**[HW]** Full factory profile `0x0101` from the G515 TKL wired:

```
01 01 00 3c 01 2c ff ff ff ff ff ff ff ff ff ff   header, idle 60, sleep 300, name...
ff ff ff ff ff ff ff ff ff ff ff ff ff ff 18 ff   ...name, rec0
00 81 80 aa ff 00 00 64 00 00 00 00 00 00 00 00   rec0 cont., rec1 (zero)
00 00 00 00 19 ff 00 81 80 aa ff 00 00 64 00 00   rec1, rec2
00 00 00 00 00 00 00 00 00 00 03 00 00 00 00 00   rec3, flags 03, ...
00 00 00 10 00 01 00 00 00 00 ff ff ff ff ff ff
ff ff ff
```

OpenRGB's comment on the same file: it is "the canonical storage for
persisted device state on G-series devices: idle/sleep timers, baseline RGB
effect, FKC enable, and more". What "FKC" covers (probably a function key
or game mode setting in bytes 75-98) is **[unverified]**.

### Lighting records

Each record is `[effect ID, 10 parameter bytes]`, the same shape as the
0x8071 RGBEffects effect parameters (and the 0x8100 LED blocks). The record
stores the effect **ID**, not the cluster's effect index.

| Record | Used when (flags `00`) | Verified |
|---|---|---|
| 0 | not used with flags `00`; factory effect `0x18` with flags `03` | |
| 1 | active lighting | **[HW]** |
| 2 | not used with flags `00`; factory effect `0x19` with flags `03` | |
| 3 | idle (dimmed) lighting, shown after the idle timeout | **[HW]** |

Static color, as written by this project **[HW]**:

```
rec1: 01 RR GG BB 02 00 00 00 00 00 00     static, ramp byte 02
rec3: 01 rr gg bb 02 00 00 00 00 00 00     the same, dimmed (for example 30%)
flags: 00
```

g515.py writes the same with `00` instead of `02` in byte 4 and record 3 at
half brightness, records 0 and 2 zeroed; that also worked on its hardware.
The ramp byte follows the 0x8071 static effect (Solaar: 0 default, 1 ramp,
2 no ramp).

Effects `0x18` and `0x19` are not in any public effect table (Solaar's list
stops at `0x17`). With flags `03` the factory profile uses records 0 and 2.
The working theory, **[unverified]**: flag bit 0 selects record 0 instead of
1 for the active state and bit 1 selects record 2 instead of 3 for idle, and
`0x18` / `0x19` are the firmware's built-in default animations or effect
card references. g515-onboard describes `00` as "the parameter path",
citing ksanislo's original profile writer.

Other effects (breathe `0x0a`, cycle `0x15`, wave `0x16`, ripple `0x17`) in
records 1 and 3 should work the same way since this project encodes them
with the device's effect IDs, but only static has been tested on hardware.

### Timers

**[HW]** With idle 10 s and sleep 25 s in the user profile, the G515 TKL
showed the record 1 color, dimmed to record 3 after 10 s without input,
switched the lighting off after 25 s, woke on a key press, and did all of
this again after the USB cable was unplugged and replugged, with no software
running.

These profile timers are the persistent ones. `0x8071` fn7
(Get/SetRgbPowerModeConfig) has runtime idle and sleep values that reset on
power cycle (OpenRGB comment). pfps's G515 log in PR #3149 read
`3c 01 2c` (60, 300) from 0x8071 fn7 at power up, which matches the factory
profile's bytes 2-5, so the firmware likely loads the runtime timers from
the active profile **[unverified]**.

ksanislo stated (quoted in PR #3149) that there is "no hardware timer for the
sleep timeout under any mode" and that G HUB tracks it in software. That
holds under software control; in onboard mode with timers in the profile
the firmware did turn the lighting off by itself in our test.

## GetSetMode (fn6)

### Request byte

From ksanislo's notes in Solaar PR #3149 (hardware tested on G515 LS TKL):

| Bit | Meaning |
|---|---|
| 0 | write the onboard bit (0 = read only) |
| 1 | onboard value (1 onboard, 0 host), applied if bit 0 set |
| 2 | write the power-management bit |
| 3 | power-management value, applied if bit 2 set |

| Request | Result | Used by |
|---|---|---|
| `00` | read only | everyone |
| `03` | onboard | Solaar and OpenRGB release, this project's `Release` |
| `05` | **host mode**, the only value that gives host mode; also stops the firmware power timers | G HUB, Solaar and OpenRGB claim, this project's `Claim` |
| `07` | onboard (power bit ignored) | |
| `0b` | onboard | what G HUB sends before `configure`; this project's activation |
| `0f` | onboard | G HUB after deep sleep wake |
| `0c` / `04` | set / clear the power bit without changing mode | pfps, PR #3149 |

### Reply

`[mode, active file hi, active file lo]`, mode bit 0 = onboard, bit 1 =
firmware power management.

| Seen | Meaning | Source |
|---|---|---|
| `03 01 01` | onboard, power bit set, factory profile `0x0101` | pfps, PR #3149 |
| `00 01 01` | after `fn6 [05]`: host | pfps, PR #3149 |
| `03 00 01` | onboard, power bit set, file 1 | [HW] on first read |
| `01 00 01` | onboard, file 1 | [HW] later |
| `00 00 01` | after `fn6 [05]` | [HW] |
| `03 00 01` | after activation | [g515-onboard] |

The mode is runtime state. After a power cycle or a wake from deep sleep the
keyboard comes back in onboard mode with the stored profile active
([ksanislo], [HW]).

## Configure / activate (fn9)

**[HW] [g515-onboard]** After saving the profile and directory:

```
fn6 [0b]                                         onboard mode
fn9 [81 01 01 00 01 00 63 c1 c2 c3 c4]           c1..c4 = CRC32(profile), big endian
fn6 [00] -> [x1 00 01]                           bit 0 set, active file 0x0001
```

Possible field split, **[unverified]**: `81 01` (the feature ID again, a type
tag), `01` (count or flags), `00 01` file ID (matches what fn6 then
reports), `00 63` length 99, then the 32-bit hash. That fits ksanislo's
description of `configure(fileID, size, hash)` and of a reset with
`fileID = 0xFF00, size = 0, hash = 0`. The reset form has not been tried
here.

Configure is needed to make the firmware reload the profile right away.
Without it the new profile only takes effect on the next power up
**[unverified]**.

## Interaction with software control (0x8071)

Claim, as G HUB, Solaar (`settings_templates.py` `_claim_sw_control`),
OpenRGB (`SetHostMode`) and this project do it:

```
8101 fn6 [05]                 host mode, firmware power management off
8071 fn5 [01 03 xx]           SetSWControl: mode 3, flags (Solaar 05 / 04, OpenRGB 06 then 05)
```

Release:

```
8071 fn5 [01 00 00]
8101 fn6 [03]                 (G HUB uses 0b + configure)
```

Consequences:

- In host mode the firmware no longer dims or sleeps the lighting; the host
  must do it (Solaar's `rgb_power.py`, OpenRGB's power state machine).
- Live effects set with 0x8071 are lost at the next power cycle or deep
  sleep wake, when the keyboard boots back into its onboard profile. OpenRGB
  re-claims on every reconnect and mentions a "10s firmware watchdog" that
  drops an unconfirmed claim back to onboard mode.
- Writing the profile (this file) is the only way found so far to make
  lighting and timers persist without software running.

## Project status and tools

| Tool | 0x8101 support |
|---|---|
| Solaar | constant `PROFILE_MANAGEMENT = 0x8101`; only `fn6 [05]` on claim and `fn6 [03]` on release (`settings_templates.py`, `rgb_power.py`), added through PR #3165 / #3149 |
| OpenRGB | `LogitechHIDPP20Controller`: fn6 host/onboard switching, and a debug-only fn8 + fnC dump of `0x0101` (99 bytes) |
| logitech-g515-onboard | full read, backup, write and activate of one static color (Python, stdlib only, GPL-3.0) |
| this project | `lighting/profilemgmt.go`: same file handling, plus any 0x8071 effect in record 1, a dimmed copy in record 3, and idle/sleep timers |
| libratbag | none (no issue or code mentions 0x8101) |

## Pitfalls

- **Feature index varies** (0x0e versus 0x0f on G515 LS TKL firmware).
- **Unwritten user files fail to load with error 0x04.** Do not treat that
  as a fatal error for `0x0000` and `0x0008`, but do for factory files.
- **Half-written user storage** (profile present without directory or the
  reverse) should stop any write; both tools refuse.
- **Factory directory hash mismatch** is normal; do not "fix" it.
- **Never write `0x0100` or `0x0101`.** Nobody knows whether factory files are
  write protected.
- **Readback of active file is the logical ID (0x0001)**, not the slot
  (0x0008).
- **fn6 values other than 05 all mean onboard**; there is no third mode.
- **The layout is only known for the G515.** g515-onboard refuses other
  models, and warns explicitly against the wired-only G515 and G915. This
  project validated the layout on the wired G515 TKL (`c358`) and checks the
  header and directory signature before every write; other devices with
  0x8101 (G915 X) are untested.

## Sources

- logitech-g515-onboard (Federico De Malmayne Duppa, GPL-3.0),
  `PROTOCOL.md`, `g515.py`, `NOTICE.md`:
  https://github.com/federico-duppa/logitech-g515-onboard
- Fork used by this project: https://github.com/mevdschee/logitech-g515-onboard
  (`PROTOCOL.md` and `g515.py` are identical to upstream)
- Solaar PR #3149, "Add RGB lighting persistence and software LED power
  management for G515", including ksanislo's fn6 write-up, pfps's hidconsole
  log and the `set_color.py` attachment that the g515-onboard tool is based
  on: https://github.com/pwr-Solaar/Solaar/pull/3149
- Solaar PR #3165 (minimal SW control claim, adds the 0x8101 constant):
  https://github.com/pwr-Solaar/Solaar/pull/3165
- Solaar source: `lib/logitech_receiver/hidpp20_constants.py`,
  `settings_templates.py` (`RGBControl._claim_sw_control`,
  `_release_sw_control`), `rgb_power.py` (`cleanup`):
  https://github.com/pwr-Solaar/Solaar
- Solaar device dump `docs/devices/G515 LS TKL 40B4.text` (0x8101 at index 15)
- Solaar issues with G915 X feature lists:
  https://github.com/pwr-Solaar/Solaar/issues/3328,
  https://github.com/pwr-Solaar/Solaar/issues/3137,
  https://github.com/pwr-Solaar/Solaar/issues/3205
- Solaar issue #2791 (G515 LS TKL lighting resets):
  https://github.com/pwr-Solaar/Solaar/issues/2791
- OpenRGB `Controllers/LogitechController/LogitechHIDPP20Controller.cpp/.h`
  (`FN_8101_*`, `SetHostMode`, `ReadActiveProfileSector`,
  `WritePowerConfig`, `ReclaimSWControl`): https://gitlab.com/CalcProgrammer1/OpenRGB,
  merge requests https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/3270
  and https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/3352
- This project: `lighting/profilemgmt.go`, `lighting/controller.go`
  (`Claim`, `Release`), `research/hardware-log.md`
- Related 0x8100 notes: [8100-onboard-profiles.md](8100-onboard-profiles.md)
