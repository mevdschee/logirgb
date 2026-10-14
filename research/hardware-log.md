# Hardware log

Everything in this file was observed on real hardware while building
logirgb, on 2026-10-14. Requests are written as `feature fn params`, where
`fn` is the function number (the byte on the wire is `fn<<4 | softwareId`),
and replies show the first parameter bytes.

## Setup

| | |
|---|---|
| OS | Ubuntu, kernel 6.17.0-42-generic, Go 1.26 |
| Keyboard | Logitech G515 TKL, wired, USB `046d:c358` |
| Mouse | Logitech G502 SE HERO, wired, USB `046d:c08b` (reports itself as "G502 HERO Gaming Mouse") |
| Transport | `/dev/hidrawN`, HID++ long reports (`0x11`, 20 bytes), device index `0xFF` |

## Device discovery

`/sys/class/hidraw/*/device/uevent` gives `HID_NAME` and `HID_ID`
(`bus:vendor:product`). The keyboard has three hidraw nodes and the mouse
two. Only the node whose report descriptor declares the vendor collection
answers HID++:

```
G515 hidraw2 descriptor:
0600ff 0901 a101 8510 9506 7508 150026ff00 0901 8100 0901 9100 c0
0600ff 0902 a101 8511 9513 7508 150026ff00 0902 8100 0902 9100 c0
```

That is usage page `0xFF00`, report `0x10` (6 data bytes) and report `0x11`
(19 data bytes). On the G502 the same collection sits at the end of the
second node's descriptor. logirgb selects nodes that contain `06 00 ff`
followed by `85 11`.

Default permissions were `crw------- root root` for the G502 node. The G515
HID++ node happened to have a `uaccess` ACL already. This rule fixed both:

```
KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="046d", TAG+="uaccess"
```

## G515 TKL (046d:c358)

### Features

`0x0001 GetCount` / `GetFeatureID` list, in index order:

```
0000 0001 0003 0005 0020 0011 8071 8081 1b10 4523 4540 8040 8101 1b05
8051 00c3 00d0 1802 1803 1807 1817 1805 1890 1e00 1e02 1602 1eb0 1801 18b0
```

`0x0005` device name: `G515 TKL`.

### 0x8071 RGBEffects

```
8071 0 ff ff 00   -> ff 00 01 00 63 00 26 06          1 cluster
8071 0 00 ff 00   -> 00 00 00 01 06                   cluster 0, location 1 (primary), 6 effects
8071 0 00 NN 00   -> cluster, index, effect id, caps, period:
   idx 0: 00 00 00 00 ...                            0x0000 off
   idx 1: 00 01 00 01 00 00 00 00                    0x0001 static
   idx 2: 00 02 00 0a c0 01 1e 0b                    0x000a breathe
   idx 3: 00 03 00 15 c0 05 03 e0                    0x0015 cycle
   idx 4: 00 04 00 16 dc e1 00 1e                    0x0016 wave
   idx 5: 00 05 00 17 00 00 00 32                    0x0017 ripple
8071 0 00 06 00   -> error 0x02 invalid argument
8071 0 01 ff 00   -> error 0x02 invalid argument     no cluster 1
```

Setting an effect (`fn1 [cluster, effectIndex, params10, 0x01]`):

| State | Result |
|---|---|
| No software control | error `0x05` (logitech internal) |
| After `8101 6 05` and `8071 5 01 03 04` | accepted and visible |

Static green that was confirmed visible:
`8071 1 00 01 00 ff 00 02 00 00 00 00 00 00 01`. Breathe blue with a
3000 ms period was also confirmed: color at params 0-2, period big endian at
3-4, intensity at 6.

### Software control (fn5) matrix

`8071 5 01 MM FF` sets mode MM with flags FF, and `8071 5 00` reads it back.
After each set, a static color write was attempted:

| Mode | Flags 0-7 | Effect write | What the keyboard shows |
|---|---|---|---|
| 1 | all accepted | accepted | onboard lighting, the host color is **not** shown |
| 2 | all accepted | error `0x05` | onboard lighting |
| 3 | all accepted | accepted | host color, the firmware **never dims or sleeps** (tested with flags 0 and 4) |
| 0 | release | error `0x05` | onboard lighting, firmware dims and sleeps |

### Idle events (mode 3, flags 0x04)

The firmware idle timer was set to 15 s and sleep to 40 s, and a host
process listened for events:

```
14:24:53 connected
14:24:53 idle         <- spurious idle/active pair right after claiming
14:24:53 active
14:25:08 idle         <- 15 s after the last key press
14:25:33 (host) sleep <- host timer, 40 s
14:25:51 active       <- key press
```

The event is feature `0x8071`, function 1, software id 0, with
`data[0] == 0` for idle and non-zero for active. While idle, mode 3 flags
`0x02` makes the firmware report the next activity. No dimming happens
unless the host does it.

### 0x8071 fn7 power mode config (idle/sleep timers)

```
8071 7 00                      -> 00 00 00 00 3c 01 2c   idle 60 s, sleep 300 s
8071 7 01 00 00 00 0a 00 19    -> 01 ...                 idle 10 s, sleep 25 s
```

With software control released, and the onboard profile driving the
lighting, the firmware dimmed after 10 s, switched off after 25 s and woke
on a key press. In mode 3 it did neither.

### 0x8040 BrightnessControl

```
8040 0      -> 00 64 05 03 00 00    max 100, 5 steps, caps 0x03, min 0
8040 1      -> 00 32                current 50
8040 2 00 64                        set 100, visible immediately
```

The brightness later read 55 (`0x37`) without logirgb writing it, most
likely from the FN brightness key or the firmware.

### 0x8081 PerKeyLighting v2 (read only)

```
8081 0 00 00 -> 00 00 ff ff ff ff ff ff ff ff ff ff 00 00 fe ff
8081 0 00 01 -> 00 01 00 00 ...
8081 0 00 02 -> 00 02 00 00 00 40 ...
```

### 0x8101 ProfileManagement

```
8101 0      -> 01 01 00 80 05 80 7f 7f 07 14 ff ff ff 20 01 07
8101 6 00   -> 03 00 01   (onboard, file 0x0001), later 01 00 01
8101 6 05   -> 00 00 01   used before claiming 0x8071
8101 6 03   -> used when releasing
```

To read a file, `fn8 [addr u16, len u16]` loads it into a buffer, and
`fnC [offset u16]` then reads that buffer back 16 bytes at a time:

```
factory directory 0x0100 (16):
5e380925 010181014063 996dec01 01 ff
crc32(bytes 4..15) = 5e380925 (matches bytes 0..3)

factory profile 0x0101 (99), crc32 14ec9848 (does not match 996dec01 in the directory):
0101 003c 012c ffff...ff(24 name bytes)
rec0 18 ff 00 81 80 aa ff 00 00 64 00
rec1 00 00 00 00 00 00 00 00 00 00 00
rec2 19 ff 00 81 80 aa ff 00 00 64 00
rec3 00 00 00 00 00 00 00 00 00 00 00
flags 03
00 00 00 00 00 00 00 00 10 00 01 00 00 00 00 ff ff ff ff ff ff ff ff

user directory 0x0000 (16), already present before logirgb touched it:
ebaa70c3 010181014063 9df93c91 08 ff

user profile 0x0008 (99), crc32 9df93c91 (static white written by an earlier tool):
0101 003c 012c ff..ff
rec0 00.., rec1 01 ff ff ff 00.., rec2 00.., rec3 01 7f 7f 7f 00.., flags 00, rest as factory
```

A save writes the file through the buffer: `fn2 [len u16]`, then
`fn3 [16 bytes]` repeated, a verify with `fnC`, and finally
`fn7 [addr u16, len u16, crc32]`. Activation is `fn6 [0b]`, then
`fn9 [81 01 01 00 01 00 63 + crc32(profile)]`, then `fn6 [00]`, which
reads back `x1 00 01`.

The stored profile was static orange `#ff8000` (`rec1 01 ff 80 00 02 ...`),
with rec3 at 30% (`01 4c 26 00 02 ...`), idle 10 s and sleep 25 s. The
result:

- the keyboard showed orange,
- dimmed to the rec3 color after 10 s,
- turned off after 25 s and woke on a key press,
- and did all of this again after the USB cable was unplugged and replugged.

## G502 SE HERO (046d:c08b)

### 0x8070 ColorLEDEffects

```
8070 0      -> 02 00 03                2 zones, nvCapabilities 0x0003 (see 8070-color-led-effects.md)
8070 1 00   -> 00 00 01 04             zone 0, location 1 (primary), 4 effects
8070 1 01   -> 01 00 02 04             zone 1, location 2 (logo), 4 effects
effects per zone: 0x00 off, 0x01 static, 0x0a breathe, 0x03 cycle
8070 7      -> 00                      GetSWControl
8070 8 01   -> accepted                SetSWControl
8070 e 00   -> 00 03 00 00 00 00 00 1f 40   GetZoneEffect: zone 0, effect idx 3 (cycle 8000 ms)
```

`8070 3 [zone, effectIndex, params10]` works in onboard mode, without
switching to host mode and without `fn8`. The zone effect is overwritten
whenever the firmware loads a profile, with or without `fn8 [01]`.

Zones as they appear on the mouse:

- **Zone 1 (location logo):** the G logo, the main visible light.
- **Zone 0 (location primary):** the DPI indicator. The three DPI LEDs are
  off and only light up for a moment when the DPI or profile buttons are
  pressed, in the zone 0 color. Setting zone 0 to static red made the DPI
  flash red (first tests looked like "no visible difference" because the
  LEDs are dark until a button press). Re-sending the current DPI index with
  `8100 c [index]` also triggers the flash.

### Profile switching

The button behind the scroll wheel cycles the onboard profiles
(`8100 4` went from `01 03` to `01 01`). It sends **no** HID++
notification: a listener on the hidraw node saw nothing for 60 seconds while
the button was pressed several times. Each switch re-applies that profile's
LED blocks.

### 0x8100 OnboardProfiles

```
8100 0 -> 01 02 01 05 05 0b 10 00 ff 0a 01
          memory model 1, profile format 2, macro format 1, 5 profiles,
          5 OOB profiles, 11 buttons, 16 sectors, sector size 255, 0x0a
8100 2 -> 01                     onboard mode
8100 4 -> 01 03                  current profile = ROM sector 0x0103
```

`fn5 [sector u16, offset u16]` returns 16 bytes, and the last read of a
sector has to start at `size-16`.

User directory sector `0x0000` was all `0xFF`, so the mouse ran its ROM
profiles. User sectors `0x0001` and `0x0002` were also all `0xFF`. ROM
sectors end in `ff ff` rather than a CRC.

ROM profile `0x0101`, full sector:

```
@0:   01 (rate) 01 (default dpi idx) 00 (shift dpi idx)
@3:   b004 6009 800c 0019 0000 (dpi 1200 2400 3200 6400 0, little endian)
@13:  ff ff ff (profile color) ff (power mode) 00 (angle snap) ff ff ...
@28:  ffff (power save timeout) ffff (power off timeout)
buttons @32: 80010001 80010002 80010004 80010008 80010010 9007ffff 9004ffff
             9003ffff 900affff 9002ffff 9001ffff ffffffff...
name @160: ff..ff
led0 @208: 03 00 00 00 00 00 1f 40 00 00 00
led1 @219: 03 00 00 00 00 00 1f 40 00 00 00
alt  @230: ff x22, tail ff ff ff
```

Decoded ROM profiles (report rate in ms, DPI list little endian):

| Sector | Rate | Default DPI index | DPI list | LED blocks 0 and 1 |
|---|---|---|---|---|
| 0x0101 | 1 | 1 | 1200, 2400, 3200, 6400 | `03 .. 1f 40` cycle 8000 ms |
| 0x0102 | 1 | 2 | 400, 800, 1600, 2400 | `0a 00 dc ff 1f 40` breathe `#00dcff` 8000 ms |
| 0x0103 | 2 | 0 | 2400 | `01 ff ff ff` static white |
| 0x0104 | 1 | 0 | 3200 | `01 ff dc 00` static `#ffdc00` |
| 0x0105 | 1 | 0 | 6400 | `01 80 00 ff` static `#8000ff` |

The profile color (13-15) is `ffffff`, power mode is `ff`, and the power
save and power off timeouts (28-31) are `ffff` in every ROM profile.

### Writing profiles

These steps were done with logirgb:

1. Copy ROM profile N (`0x010N`) to user sector N, for N = 1..5, changing
   only the LED blocks.
2. Store a CRC-16/CCITT (init `0xFFFF`, polynomial `0x1021`) big endian in
   the last two bytes.
3. Write each sector with `fn6 [sec u16, 00 00, len u16]`, then 16-byte
   `fn7` chunks, then `fn8`, and read it back with `fn5` to verify.
4. Write the directory sector 0 with entries `00 0N 01 00`, then
   `ff ff 00 00`, padded with `ff`, plus the CRC.
5. Select profile 1 with `fn3 [00 01]`; `fn4` then returned `00 01`.

**LED block order is reversed relative to the zones.** Writing green to
block 0 and red to block 1 gave `8070 e 00 -> 00 01 ff 00 00` (zone 0 red)
and `8070 e 01 -> 01 01 00 ff 00` (zone 1 green), and the logo was green. So
profile LED block 0 drives zone 1 (the logo) and block 1 drives zone 0.
After fixing the mapping, the logo kept its configured color across profile
switches.

### 0x8070 persistence byte

`8070 3 [zone, effectIndex, params10, persist]` with persist `01` (RAM and
EEPROM): logo set to magenta while all onboard profiles stored blue. After
unplugging and replugging, the logo came back magenta, so the stored zone
effect is what the mouse shows at power-up. The first profile switch loaded
the profile's LED block (blue). Writing the profiles and the persistent zone
effect together kept the logo color through profile switches and a replug.
The profile sectors were not changed by the persist write (block 0 still
read `01 00 00 ff 02`).
