# HID++ transport on Linux hidraw

Research notes on how HID++ messages are framed, addressed and routed on Linux,
and how to share a device with the kernel and with other userspace tools. The
lighting features themselves (0x8070, 0x8071, 0x8080, 0x8081, 0x8100, 0x8101)
are out of scope here.

## Legend

Every claim is tagged with where it comes from:

| Tag | Meaning |
|---|---|
| **[HW]** | Verified on hardware in this project (G515 TKL wired 046d:c358, G502 HERO 046d:c08b, Ubuntu, kernel 6.17) |
| **[Spec]** | Logitech documentation: the HID++ 2.0 draft (2012-06-04), the HID++ 1.0 spec for Unifying receivers, Logitech's `cpg-docs` repo, or the per-feature HTML docs mirrored by Peter Wu |
| **[Kernel]** | Linux `drivers/hid/hid-logitech-hidpp.c`, `hid-logitech-dj.c`, `hid-core.c`, `hidraw.c` (torvalds/linux master, fetched 2026-10) |
| **[Solaar]** | Solaar source (`lib/logitech_receiver/base.py`, `base_usb.py`, `hidpp20.py`, `hidapi/udev_impl.py`, `rules.d/`), commit e7304c4 (2026-08) |
| **[OpenRGB]** | OpenRGB source (`Controllers/LogitechController/*`, unified HID++ 2.0 controller), master 2026 |
| **[libratbag]** | libratbag source (`src/hidpp20.c`, `src/hidpp-generic.c`, `src/driver-hidpp20.c`, `src/libratbag-hidraw.c`), master 2026 |
| **[Unverified]** | Plausible, seen in forum posts or inferred, not confirmed by source or hardware |

## 1. Report framing

HID++ rides on vendor defined HID reports. The first byte is the HID report
ID, which also fixes the length.

| Report ID | Name | Total length | Payload after the 4 byte header | Notes |
|---|---|---|---|---|
| `0x10` | short | 7 bytes | 3 bytes | HID++ 1.0 and 2.0 [Spec] |
| `0x11` | long | 20 bytes | 16 bytes | HID++ 1.0 and 2.0 [Spec] |
| `0x12` | very long | up to 64 bytes | up to 60 bytes | Not in the 2012 draft. Length is taken from the report descriptor by the kernel [Kernel]; OpenRGB fixes it at 64 [OpenRGB]. Used by some per-key keyboards for 0x8080/0x8081 bulk writes [OpenRGB] |
| `0x20` / `0x21` | DJ short / DJ long | 15 / 32 bytes | | Unifying receiver "DJ" collection, not HID++ itself [Kernel][Solaar] |

From the kernel driver (GPL-2.0, `hid-logitech-hidpp.c`):

```c
#define REPORT_ID_HIDPP_SHORT			0x10
#define REPORT_ID_HIDPP_LONG			0x11
#define REPORT_ID_HIDPP_VERY_LONG		0x12

#define HIDPP_REPORT_SHORT_LENGTH		7
#define HIDPP_REPORT_LONG_LENGTH		20
#define HIDPP_REPORT_VERY_LONG_MAX_LENGTH	64
```

The kernel only enables very long reports when the descriptor declares report
`0x12`, and accepts any length between 20 and 64 for it
(`hidpp_validate_device`) [Kernel].

**[HW]** Neither the G515 TKL (wired) nor the G502 HERO declares report `0x12`
in its descriptor. Both declare `0x10` (6 data bytes) and `0x11` (19 data
bytes), see section 6.

### HID++ 2.0 header (feature access, "FAP")

| Byte | Field | Notes |
|---|---|---|
| 0 | Report ID | `0x10` / `0x11` / `0x12` |
| 1 | Device index | `0xFF` for a corded/direct device or the receiver itself, `0x01`..`0x06` (up to `0x07`) for devices behind a receiver |
| 2 | Feature index | Device specific index from the feature table. `0x00` is always IRoot |
| 3 | `function << 4 \| softwareId` | High nibble: function (0..15). Low nibble: software ID (1..15) |
| 4.. | Parameters / results | Big endian unless a feature says otherwise |

Logitech's own description of the software ID, from `cpg-docs/hidpp20/README.rst` [Spec]:

> This ID is used in case multiple applications want to interact with the
> device. When you send a request with a software ID, the reponse's software
> ID will have the same value. You can use it to make sure a reply is meant
> for you.

And from the 2012 draft [Spec]:

> The device index, feature index, function identifier, and software
> identifier are always returned unchanged.

> 0 Do not use (allows to distinguish a notification from a response).

### HID++ 1.0 header (register access, "RAP")

Receivers and old devices speak HID++ 1.0: byte 2 is a Sub ID instead of a
feature index [Spec].

| Sub ID | Meaning |
|---|---|
| `0x00`..`0x7F` | HID++ reports and notifications (e.g. `0x40` device disconnection, `0x41` device connection) |
| `0x80` / `0x81` | Set / get short register |
| `0x82` / `0x83` | Set / get long register |
| `0x8F` | Error message |

Example (HID++ 1.0 spec): read register `0x02` (connection state) of the
receiver is `10 FF 81 02 00 00 00`. Writing `0x02` to register `0x02`
(`10 FF 80 02 02 00 00`) makes the receiver send fake `0x41` connection
notifications for all paired devices; Solaar calls this `notify_devices()`
[Solaar]:

```python
# Solaar, lib/logitech_receiver/receiver.py (GPL-2.0)
def notify_devices(self):
    """Scan all devices."""
    if self.handle:
        if not self.write_register(Registers.RECEIVER_CONNECTION, 0x02):
```

## 2. Device index

| Value | Meaning | Source |
|---|---|---|
| `0xFF` | The receiver itself, or a corded / directly attached device (USB cable, Bluetooth) | [Spec] "`0xff` is reserved for corded devices and receivers" |
| `0x01`..`0x06` | Paired device slot on a Unifying or Bolt receiver | [Spec] HID++ 1.0: "1 to 6 for Unifying devices" |
| `0x07` | Some receivers report devices beyond their nominal maximum | [Kernel] `DJ_DEVICE_INDEX_MAX 7`; [Solaar] scans `range(1, 8)` with comment "some receivers have devices past their max # devices" |
| `0x00` | Some Bluetooth devices reply with index `0x00` to a request sent to `0xFF` | [Solaar] `reply_devnumber == devnumber ^ 0xFF  # BT device returning 0x00` |

**[HW]** Both the G515 TKL wired and the G502 HERO wired answer HID++ 2.0 on
device index `0xFF`.

Note for this project: `hidpp/device.go` drops replies whose byte 1 does not
equal the request's index, so a Bluetooth device that answers on `0x00` would
time out. Accepting `index ^ 0xFF` as Solaar does would cover that case
[Unverified for any G5xx over Bluetooth].

## 3. Core features used by the transport

### 3.1 IRoot `0x0000` (always at index 0)

| Fn | Name | Request params | Reply |
|---|---|---|---|
| 0 | `GetFeature` | featureId MSB, LSB | featureIndex, featureType, featureVersion |
| 1 | `GetProtocolVersion` (also ping) | 0, 0, pingData | protocolMajor, protocolMinor, pingData echoed |

Event 0 of IRoot is `NoOperation`, which a device "may send to prevent the
device from entering low power mode" and which host software can ignore
[Spec, cpg-docs 0x0000-IRoot.rst].

`featureIndex == 0` in a `GetFeature` reply means "not supported" [libratbag:
"Returns the feature index or 0x00 if it is not found"]. `featureType` bit 7 is
obsolete, bit 6 hidden [Spec], and Solaar also names bit 5 `INTERNAL`
(engineering) [Solaar `FeatureFlag`].

Ping and version detection, from the 2012 draft [Spec]:

```
Request:     0x10.DeviceIndex.0x00.0x1n.0x00.0x00.0xUU   (n = SwID, UU = ping data)
HID++ 1.0:   0x10.DeviceIndex.0x8F.0x00.0xFn.0x01.0x00   (ERR_INVALID_SUBID)
HID++ 2.0:   0x10.DeviceIndex.0x00.0x1n,0x02.0x00.0xUU
```

`protocolMajor` values documented by Logitech: `0x02` HID++ 2.0 legacy,
`0x04` HID++ 2.0 (cpg-docs lists `0x8f` as "HID++ 1.0", which is really the
error Sub ID that a 1.0 device answers with) [Spec]. Peter Wu's notes on
protocol 4 add that byte 1 is `targetSw` when `protocolNum >= 3` [Spec mirror].

How the tools use it:

* Solaar `ping()` sends function 1 with a random mark byte and checks it in
  the reply. It treats a `0x8F` reply with `INVALID_SUB_ID_COMMAND` as "HID++
  1.0 device", `RESOURCE_ERROR`/`CONNECTION_REQUEST_FAILED` as "unreachable",
  and `UNKNOWN_DEVICE` as "no device in this slot" [Solaar `base.py`].
* libratbag maps a HID++ 1.0 `INVALID_SUBID` reply to protocol 1.0 [libratbag
  `hidpp20_root_get_protocol_version`].
* The kernel sends the version request as a short RAP request because 27 MHz
  receivers only have an 8 byte HID++ endpoint and cannot carry long reports
  (kernel commit 096377525cdb, "use RAP instead of FAP to get the protocol
  version") [Kernel].
* This project pings with a long report, function 1, ping byte `0x5A`, 500 ms
  timeout, three tries on `0xFF` then indexes 1..6 [this repo `hidpp/device.go`].

### 3.2 IFeatureSet `0x0001`

| Fn | Name | Params | Reply |
|---|---|---|---|
| 0 | `GetCount` | none | count (number of features, not counting IRoot) |
| 1 | `GetFeatureID` | index (1..count) | featureId MSB, LSB, featureType, featureVersion |

Solaar walks it as `device.request((fs_index << 8) | 0x10, index)` [Solaar
`hidpp20.py`]. OpenRGB uses `LOGITECH_CMD_FEATURE_SET_GET_COUNT 0x01` and
`LOGITECH_CMD_FEATURE_SET_GET_ID 0x11` (function nibble already combined with
software ID 1 in its older shared code) [OpenRGB `LogitechProtocolCommon.h`].

### 3.3 Device Information `0x0003`

From Logitech's feature document (Peter Wu's mirror `x0003_deviceinfo.html`)
[Spec] and Solaar's parser [Solaar `get_ids`, `get_firmware`]:

| Fn | Name | Reply layout |
|---|---|---|
| 0 | `getDeviceInfo` | `[0]` entityCnt, `[1..4]` unitId, `[5..6]` transport bitfield (bit0 BT, bit1 BTLE, bit2 eQuad, bit3 USB), `[7..12]` modelId (one 16 bit PID per set transport bit, in bit order), `[13]` extended model ID (later versions) |
| 1 | `getFwInfo(entityIdx)` | `[0]` type (0 main app, 1 bootloader, 2 hardware), `[1..3]` fw name prefix (ASCII), `[4]` fw number (BCD), `[5]` revision (BCD), `[6..7]` build (BCD), `[8]` active flag, `[9..10]` transport PID, `[11..15]` extra version |
| 2 | `getDeviceSerialNumber` | 12 ASCII chars, only in newer versions of the feature [Unverified, not in the mirrored v1 doc] |

`unitId` is a 4 byte per-unit identifier; together with `modelId` it lets
software recognise the same device across USB, receiver and Bluetooth [Spec].

### 3.4 Device Name and Type `0x0005`

| Fn | Name | Params | Reply |
|---|---|---|---|
| 0 | `GetDeviceNameCount` | none | name length in bytes |
| 1 | `GetDeviceName` | char index | next chunk of the name (3 bytes in a short reply, 16 in a long one) |
| 2 | `GetDeviceType` | none | 0 keyboard, 1 remote control, 2 numpad, 3 mouse, 4 touchpad, 5 trackball, 6 presenter, 7 receiver [Solaar `DEVICE_KIND`] |

Solaar reads the name with `feature_request(DEVICE_NAME)` for the length and
then `feature_request(DEVICE_NAME, 0x10, len(name))` until complete [Solaar
`hidpp20.py`]. This project does the same in `Device.Name()`.

## 4. Error replies

### 4.1 HID++ 2.0 errors (`0xFF`)

Layout: `11 <dev> FF <featIdx> <fn|swid> <errCode> ...` (short form `10 ...`
also possible). Byte 2 is `0xFF` where the feature index would be, bytes 3 and
4 echo the request's feature index and function/software ID [Kernel
`hidpp_match_error`; libratbag; Solaar].

```c
/* Linux, hid-logitech-hidpp.c (GPL-2.0) */
static inline bool hidpp_match_error(struct hidpp_report *question,
		struct hidpp_report *answer)
{
	return ((answer->rap.sub_id == HIDPP_ERROR) ||
	    (answer->fap.feature_index == HIDPP20_ERROR)) &&
	    (answer->fap.funcindex_clientid == question->fap.feature_index) &&
	    (answer->fap.params[0] == question->fap.funcindex_clientid);
}
```

| Code | 2012 draft name | Kernel define | Solaar | libratbag |
|---|---|---|---|---|
| `0x00` | NoError | `HIDPP20_ERROR_NO_ERROR` | | `ERR_NO_ERROR` |
| `0x01` | Unknown | `..._UNKNOWN` | `UNKNOWN` | `ERR_UNKNOWN` |
| `0x02` | InvalidArgument | `..._INVALID_ARGS` | `INVALID_ARGUMENT` | `ERR_INVALID_ARGUMENT` |
| `0x03` | OutOfRange | `..._OUT_OF_RANGE` | `OUT_OF_RANGE` | `ERR_OUT_OF_RANGE` |
| `0x04` | HWError | `..._HW_ERROR` | `HARDWARE_ERROR` | `ERR_HARDWARE_ERROR` |
| `0x05` | Logitech internal | `..._NOT_ALLOWED` | `LOGITECH_ERROR` | `ERR_LOGITECH_INTERNAL` |
| `0x06` | INVALID_FEATURE_INDEX | `..._INVALID_FEATURE_INDEX` | `INVALID_FEATURE_INDEX` | `ERR_INVALID_FEATURE_INDEX` |
| `0x07` | INVALID_FUNCTION_ID | `..._INVALID_FUNCTION_ID` | `INVALID_FUNCTION` | `ERR_INVALID_FUNCTION_ID` |
| `0x08` | Busy | `..._BUSY` | `BUSY` | `ERR_BUSY` |
| `0x09` | Unsupported | `..._UNSUPPORTED` | `UNSUPPORTED` | `ERR_UNSUPPORTED` |

The draft describes Busy as "Device (or receiver) cannot answer immediately
to this request for any reason i.e: already processing a request from the
same or another SW, pipe full" [Spec]. Retrying on Busy is reasonable; the
kernel retries `HIDPP20_ERROR_BUSY` and OpenRGB has a `retry_on_busy` policy
[Kernel `hidpp_send_message_sync`; OpenRGB `SendAcked`].

Note that the kernel calls `0x05` "not allowed" while the spec says "Logitech
internal". This project's `errorNames` follows the spec.

### 4.2 HID++ 1.0 errors (`0x8F`)

Layout: `10 <dev> 8F <subId> <address> <errCode> 00` [Spec HID++ 1.0 §2.9.9].
A HID++ 2.0 request to a 1.0 device or receiver comes back as an `0x8F` error
whose Sub ID/address bytes are the request's feature index and
function/software ID, which is why Solaar compares `reply_data[1:3] ==
request_data[:2]` for both error kinds [Solaar].

| Code | HID++ 1.0 spec name | Meaning (spec) |
|---|---|---|
| `0x00` | ERR_SUCCESS | No error / undefined |
| `0x01` | ERR_INVALID_SUBID | Invalid SubID / command (also: "this is a HID++ 1.0 device" on ping) |
| `0x02` | ERR_INVALID_ADDRESS | Invalid address |
| `0x03` | ERR_INVALID_VALUE | Invalid value |
| `0x04` | ERR_CONNECT_FAIL | Connection request failed (receiver) |
| `0x05` | ERR_TOO_MANY_DEVICES | Too many devices connected (receiver) |
| `0x06` | ERR_ALREADY_EXISTS | Already exists (receiver) |
| `0x07` | ERR_BUSY | Busy (receiver) |
| `0x08` | ERR_UNKNOWN_DEVICE | Unknown device (receiver), i.e. nothing paired in that slot |
| `0x09` | ERR_RESOURCE_ERROR | Resource error (receiver), Solaar treats it as "device unreachable" |
| `0x0A` | ERR_REQUEST_UNAVAILABLE | Request not valid in current context |
| `0x0B` | ERR_INVALID_PARAM_VALUE | Request parameter has unsupported value |
| `0x0C` | ERR_WRONG_PIN_CODE | PIN code entered on the device was wrong |

The same table appears in the kernel (`HIDPP_ERROR_*`), Solaar
(`hidpp10_constants.ErrorCode`) and libratbag (`hidpp10_errors[]`).

## 5. Notifications and events

* HID++ 2.0 **broadcast events** use software ID `0`. The draft reserves 0 for
  exactly this purpose [Spec].
* **Application specific events (ASEs)** are events tied to an earlier
  request; the draft says "the software identifier associated with the
  function is repeated in the ASE" [Spec]. So a non-zero software ID does not
  always mean "reply"; it can also be an ASE meant for the requester.
* HID++ 1.0 notifications from receivers use Sub IDs `0x40`..`0x7F`, e.g.
  `0x41` device connection with the wireless PID in bytes 5..6, and are only
  sent when the host enabled "wireless notifications" in register `0x00`
  [Spec HID++ 1.0 §3].

Solaar's classifier (GPL-2.0, `base.py`):

```python
if (
    # standard HID++ 1.0 notification, SubId may be 0x40 - 0x7F
    (sub_id >= 0x40)  # noqa: E131
    ...
    or
    # HID++ 2.0 feature notifications have the SoftwareID 0
    (address & 0x0F == 0x00)
):  # noqa: E129
```

The kernel treats a report as a connect event when it comes from the
`0x1D4B` Wireless Device Status feature index or is a short `0x41`
notification, and handles it in a work queue [Kernel
`hidpp_report_is_connect_event`].

Notification-like traffic a lighting tool should expect while waiting for its
own reply:

| Source | Example | How to recognise |
|---|---|---|
| Device firmware | IRoot `NoOperation`, battery, wireless status, onboard profile change | software ID 0 |
| Kernel `hid-logitech-hidpp` (if bound) | requests and replies of the kernel itself | software ID `0x1` |
| Other userspace tools | Solaar, OpenRGB, ratbagd, G HUB under Wine | their software ID, see section 9 |
| Receiver | `0x40` / `0x41` connection notifications | report `0x10`, Sub ID `0x40`/`0x41` |
| Keyboard/mouse input | key or button reports on other report IDs of the same node | report ID not `0x10`/`0x11`/`0x12` |

**[HW]** On the G502 HERO the HID++ collection shares a hidraw node with a
keyboard collection (report IDs 1, 3, 4), so input reports for the G-keys and
consumer keys arrive on the same fd as HID++ replies and must be skipped.

## 6. Finding the right hidraw node

A USB HID device gets one hidraw node per HID interface. Only one of them
carries the HID++ collection.

### 6.1 What the descriptors look like

**[HW]** G515 TKL wired (046d:c358): 3 hidraw nodes (interfaces 0, 1, 2). Only
interface 2 has the HID++ collection. Its complete report descriptor:

```
06 00 ff   Usage Page (Vendor 0xFF00)
09 01      Usage (1)
a1 01      Collection (Application)
85 10        Report ID (0x10)
95 06        Report Count (6)
75 08        Report Size (8)
15 00 26 ff 00  Logical 0..255
09 01 81 00     Input
09 01 91 00     Output
c0         End Collection
06 00 ff   Usage Page (Vendor 0xFF00)
09 02      Usage (2)
a1 01      Collection (Application)
85 11        Report ID (0x11)
95 13        Report Count (19)
75 08 15 00 26 ff 00 09 02 81 00 09 02 91 00
c0
```

**[HW]** G502 HERO (046d:c08b): 2 hidraw nodes. Interface 0 is the mouse.
Interface 1 holds a keyboard collection (report 1), consumer control (report
3), system control (report 4) and then the same two vendor collections with
report IDs `0x10` and `0x11` at the end of the descriptor.

This layout is identical to the HID++ collection the kernel synthesises for
devices behind a DJ receiver (`hidpp_descriptor[]` in `hid-logitech-dj.c`:
`0x06, 0x00, 0xff` / `0x85, 0x10` / `0x95, 0x06` ... `0x85, 0x11` / `0x95,
0x13`) [Kernel].

### 6.2 Usage pages in use

| Usage page | Where | Source |
|---|---|---|
| `0xFF00` usage 1 (short) / usage 2 (long) | Receivers, mice, most modern keyboards over USB, G915 family, DJ child devices, **[HW]** G515 TKL wired and G502 HERO | [Kernel], [OpenRGB], [HW] |
| `0xFF43` | Older G-series keyboards (G213, G512, G610, G810, G813, G815, G910, G Pro), G560 speaker, G933/G935 headsets, and G515 over Bluetooth. OpenRGB opens usage `0x0602` on the G213 and notes that the very long `0x12` report sits in a separate usage `0x0604` collection on Windows | [OpenRGB `LogitechControllerDetect.cpp`] |
| `0xFF43` usage `0x0202`, long reports only | G515 LIGHTSPEED TKL over Bluetooth (046d:b389), per the description of OpenRGB MR !3496 | [OpenRGB MR] |
| `0xFF80` | G600 (not HID++ 2.0) | [OpenRGB] |
| `0xFFA0` | "Centurion" transport (G522, PRO X 2 headsets), 64 byte frames with report IDs `0x50`/`0x51`, no device index byte | [OpenRGB], [Solaar] |

OpenRGB's summary of its own detectors (GPL-2.0-or-later):

```
|  0xFF00 usage 2   standard HID++ long report (modern keyboards/mice, receivers, G915 family, wired Lightspeed mice).
|  0xFF43 any usage keyboards (G213/G512/G610/G810/G813/G815/G910/G Pro), the G560 speaker, the G933 headset
|  0xFFA0 usage 1   Centurion (G522, PRO X 2)
| Bluetooth, which reports no interface number:
|  0xFF43 any usage G-series keyboards (G515)
```

Consequence for this project: `hidpp/discover.go` looks for the byte sequence
`06 00 FF` followed by `85 11`. A keyboard that puts HID++ on usage page
`0xFF43` (descriptor bytes `06 43 FF`) is not found. Matching any usage page
`>= 0xFF00` that contains report ID `0x11`, as libratbag does, would be more
general.

### 6.3 How each tool selects the node

| Tool | Method |
|---|---|
| Solaar | Parses the descriptor (`hid_parser`): short if report `0x10` is an input report of 6×8 bits, long if `0x11` is 19×8 bits; the usage page check is commented out "be more permissive". For receivers it also requires the USB interface number from `base_usb.py` (2 for Unifying, Bolt and Lightspeed) [Solaar `udev_impl.py`] |
| libratbag | Reads the descriptor with `HIDIOCGRDESC`, `hidpp20drv_test_hidraw()` just checks `ratbag_hidraw_has_report(device, REPORT_ID_LONG)`; `ratbag_hidraw_has_vendor_page()` accepts any usage page `>= 0xff00` [libratbag] |
| OpenRGB | hidapi enumeration filtered by VID, interface number, usage page and usage (see table above) [OpenRGB] |
| This project | sysfs `/sys/class/hidraw/*/device/report_descriptor`, VID `046d`, bytes `06 00 FF` then `85 11` [this repo] |

Solaar's check (GPL-2.0, `hidapi/udev_impl.py`):

```python
hidpp_short = 0x10 in rd.input_report_ids and 6 * 8 == int(rd.get_input_report_size(0x10))
# and _Usage(0xFF00, 0x0001) in rd.get_input_items(0x10)[0].usages  # be more permissive
hidpp_long = 0x11 in rd.input_report_ids and 19 * 8 == int(rd.get_input_report_size(0x11))
```

Short versus long in practice: some mice do not declare report `0x10` at all;
libratbag upgrades short requests to long in that case ("some mice don't
support short reports"), the kernel does the same via `supported_reports`,
and OpenRGB falls back to long when a short write fails (G560, G933) and
always writes long on Windows because HIDClass exposes the short and long
collections as separate devices [libratbag, Kernel, OpenRGB]. Sending every
request as a long report, as this project does, avoids the issue on Linux.

## 7. Kernel drivers and how they coexist with hidraw

### 7.1 Which driver binds

| Device | Kernel driver | Source |
|---|---|---|
| Unifying C52B, C532 | `hid-logitech-dj` (type `recvr_type_dj`) | [Kernel `logi_dj_receivers[]`] |
| Lightspeed C539, C53A (Powerplay), C53F, C543 | `hid-logitech-dj` (`recvr_type_gaming_hidpp`) | [Kernel] |
| Lightspeed C545, C547, C54D | `hid-logitech-dj` (`recvr_type_gaming_hidpp_ls_1_3`) | [Kernel] |
| Bolt C548 | not in the DJ table; `hid-generic` with `HID_QUIRK_ALWAYS_POLL` | [Kernel `hid-quirks.c`] |
| Devices behind a DJ receiver | `hid-logitech-hidpp` via `LDJ_DEVICE(HID_ANY_ID)` | [Kernel] |
| Wired gaming mice listed explicitly (G403 C082, G502 Lightspeed C08D, G502 X Plus C095, G502 X Lightspeed C098, G703, G900, G903, G Pro, G Pro X Superlight 1/2, G915 TKL C343, G920/G923 wheels) | `hid-logitech-hidpp` | [Kernel `hidpp_devices[]`] |
| G502 HERO C08B, G515 TKL C358 | not listed, so `hid-generic` | [Kernel]; **[HW]** `/sys/bus/hid/devices/0003:046D:C08B.*` and `0003:046D:C358.*` are bound to `hid-generic` on kernel 6.17 |

So on the two tested devices the kernel sends no HID++ traffic of its own.

### 7.2 What hidraw sees when a HID++ driver is bound

The kernel driver sends with software ID 1:

```c
/* Linux, hid-logitech-hidpp.c (GPL-2.0) */
#define LINUX_KERNEL_SW_ID			0x01
...
	message->fap.funcindex_clientid = funcindex_clientid | LINUX_KERNEL_SW_ID;
```

When a reply matches the kernel's pending question, the driver returns 1
from `raw_event` with the comment "We return 1 to hid-core to avoid
forwarding the command upstream". In `hid-core.c`, however, only a negative
return value skips further processing:

```c
/* Linux, hid-core.c (GPL-2.0) */
	if (hdrv && hdrv->raw_event && hid_match_report(hid, report)) {
		ret = hdrv->raw_event(hid, report, data, size);
		if (ret < 0)
			goto unlock;
	}

	ret = hid_report_raw_event(hid, type, data, bufsize, size, interrupt);
```

and `hid_report_raw_event()` passes the report to `hidraw_report_event()`
whenever hidraw is attached. Reading the current source, userspace therefore
also sees the kernel's own replies (software ID 1) and connect events
[Kernel, reading of source; not tested on a device with `hid-logitech-hidpp`
bound].

The DJ driver says the same for receivers (GPL-2.0, `hid-logitech-dj.c`):

```c
	/*
	 * Data is from the HID++ collection, in this case, we forward the
	 * data to the corresponding child dj device and return 0 to hid-core
	 * so he data also goes to the hidraw device of the receiver. This
	 * allows a user space application to implement the full HID++ routing
	 * via the receiver.
	 */
```

The kernel serialises its own requests with `send_mutex` and only matches
replies by feature index plus function/software ID [Kernel
`hidpp_match_answer`], so a userspace request with a different software ID
cannot be mistaken for a kernel reply.

### 7.3 hidraw fan-out and buffers

Every open file descriptor on a hidraw node has its own ring buffer and gets
its own copy of every input report:

```c
/* Linux, hidraw.c (GPL-2.0) */
	list_for_each_entry(list, &dev->list, node) {
		int new_head = (list->head + 1) & (HIDRAW_BUFFER_SIZE - 1);

		if (hidraw_is_revoked(list) || new_head == list->tail)
			continue;
```

Consequences:

* One process reading a report does not take it away from another. Two tools
  cannot "steal" each other's replies at the kernel level; the problem is
  that a tool may *misattribute* someone else's reply to its own outstanding
  request if both use the same feature index, function and software ID.
* `HIDRAW_BUFFER_SIZE` is 64 reports per reader. A process that keeps the node
  open without reading loses new reports silently (`new_head == list->tail`
  → `continue`) but does not block others.
* Stale reports accumulate in your own buffer while you are not reading. Solaar
  drains its buffer before every write (`_read_input_buffer`), and treats
  what it drains as possible notifications [Solaar].
* `HIDIOCREVOKE` (kernel 6.12+) lets a privileged process such as logind
  revoke an fd; it is not an exclusive-open mechanism [Kernel, Peter Hutterer's
  blog].
* There is no `O_EXCL` or exclusive open on hidraw.

## 8. Receivers and paired devices

| PID | Receiver | Kind in Solaar | HID++ interface | Max devices | Kernel |
|---|---|---|---|---|---|
| C52B | Unifying | unifying | USB interface 2 | 6 | `hid-logitech-dj` |
| C532 | Unifying (2nd PID) | unifying | 2 | 6 | `hid-logitech-dj` |
| C539 | Lightspeed | lightspeed | 2 | read from receiver | `hid-logitech-dj` |
| C53A | Lightspeed (Powerplay) | lightspeed | 2 | read from receiver | `hid-logitech-dj` |
| C53D, C53F, C541, C543, C545, C54D | Lightspeed variants | lightspeed | 2 | read from receiver | DJ for C53F, C543, C545, C54D |
| C547 | Lightspeed (G915 X / G515 family, "ls 1.3" quirks) | lightspeed | 2 | read from receiver | `hid-logitech-dj` |
| C548 | Bolt | bolt | 2 | 6 | `hid-generic` |

Sources: [Solaar `base_usb.py`], [Kernel `hid-logitech-dj.c`, `hid-ids.h`].

Two ways to talk to a device behind a receiver:

1. **Through the receiver node** (what Solaar does): open the receiver's HID++
   hidraw node and put the slot number (1..6, sometimes 7) in byte 1. The
   receiver itself answers on `0xFF` with HID++ 1.0 registers. Use register
   `0x02` write `0x02` or the `0x41` notifications to learn which slots are
   populated, and long register `0xB5` sub-registers `0x2N`/`0x4N` (Unifying,
   Lightspeed) or `0x5N` (Bolt) for pairing info and names [Spec HID++ 1.0;
   Solaar; OpenRGB `getWirelessDeviceName` reads "reg 0xB5 sub 0x4N"].
2. **Through the DJ child node** (what OpenRGB does for Lightspeed mice): with
   `hid-logitech-dj` bound, each paired device gets its own HID device (USB ID
   shows the wireless PID, e.g. `046d:407f` for a G502 Lightspeed, `046d:40b4`
   for a G515 Lightspeed TKL per OpenRGB) and its own hidraw node with a
   synthetic HID++ collection. Whatever device index you write, the kernel
   rewrites byte 1 to the child's slot before forwarding:

```c
/* Linux, hid-logitech-dj.c (GPL-2.0), logi_dj_ll_raw_request() */
		else
			buf[1] = djdev->device_index;
		return hid_hw_raw_request(djrcv_dev->hidpp, reportnum, buf,
				count, report_type, reqtype);
```

Replies come back with the real slot number in byte 1, not `0xFF`
[Unverified, follows from the receiver forwarding raw data]. A sleeping
wireless device produces HID++ 1.0 `0x8F` errors from the receiver
(`ERR_RESOURCE_ERROR` / `ERR_CONNECT_FAIL`) or times out [Solaar `ping()`].

For Bolt (`hid-generic`) only option 1 exists.

This project's `Open()` tries `0xFF` three times and then slots 1..6 on the
same node, which covers wired devices and the receiver node, and on a DJ
child node the first ping already lands on the right slot because the kernel
rewrites the index.

## 9. Permissions

### 9.1 Defaults

**[HW]** On Ubuntu with kernel 6.17 the hidraw nodes of both devices were
`crw-------` root only by default. The G515 HID++ node happened to carry a
`uaccess` ACL from some other rule; the G502 nodes did not.

systemd only tags a few hidraw classes for `uaccess` itself (FIDO security
tokens via `ID_SECURITY_TOKEN`, AV production controllers), so a generic
Logitech hidraw node stays root only unless a package installs a rule
[local `/usr/lib/udev/rules.d/70-uaccess.rules`, systemd 255].

### 9.2 Rules in the wild

| Rule | Match | Grant |
|---|---|---|
| This project (**[HW]** fixed access) | `KERNEL=="hidraw*", SUBSYSTEM=="hidraw", ATTRS{idVendor}=="046d", TAG+="uaccess"` | ACL for the active seat user |
| Solaar `rules.d/42-logitech-unify-permissions.rules` | every hidraw with `idVendor 046d`, the Lenovo nano receiver `17ef:6042`, and Bluetooth `KERNELS == "0005:046D:*"`; also `uinput` | `TAG+="uaccess"`, with a commented out `MODE="0660", GROUP="plugdev"` alternative for SSH users |
| OpenRGB `60-openrgb.rules` (generated by `scripts/build-udev-rules.sh`) | one line per supported VID:PID | `SUBSYSTEMS=="usb\|hidraw", ATTRS{idVendor}=="...", ATTRS{idProduct}=="...", TAG+="uaccess", TAG+="<device_name>"`, plus `KERNEL=="i2c-[0-99]*", TAG+="uaccess"` and `KERNEL=="port", TAG+="uaccess"` |
| libratbag / ratbagd | none needed | ratbagd is a system D-Bus service running as root; Piper talks to it over D-Bus |
| g810-led | ships its own udev rules for its keyboards (see other-protocols-and-tools.md) | |

Solaar's rule (GPL-2.0):

```
# Allows non-root users to have raw access to Logitech devices.
# Allowing users to write to the device is potentially dangerous
# because they could perform firmware updates.
...
# Allow any seated user to access the receiver.
# uaccess: modern ACL-enabled udev
TAG+="uaccess"

# Grant members of the "plugdev" group access to receiver (useful for SSH users)
#MODE="0660", GROUP="plugdev"
```

OpenRGB's generator line (GPL-2.0-or-later):

```bash
udev_line=$(printf 'SUBSYSTEMS=="%s|hidraw", ATTRS{idVendor}=="%s", ATTRS{idProduct}=="%s", TAG+="uaccess", TAG+="%s"\n' ...)
```

### 9.3 Practical notes

* `uaccess` is evaluated by `73-seat-late.rules`, so a rule that adds the tag
  must sort before 73 (Solaar uses 42, OpenRGB 60, this project's README 70).
  A rule in `99-*.rules` does not work [picotool PR #199, Arch bug FS#63164,
  NixOS discourse].
* `uaccess` only grants the user of the active local session. SSH sessions or
  system services need `MODE="0660", GROUP="plugdev"` (or similar) instead.
* After installing a rule: `udevadm control --reload-rules && udevadm trigger
  --subsystem-match=hidraw --action=add`, or replug.
* The ACL is applied slightly after the node appears. Solaar retries `open()`
  three times with 100 ms sleeps on `EACCES` [Solaar `open_path`]. A hotplug
  aware tool should do the same.
* Write access to a Logitech hidraw node allows firmware update commands
  (DFU features `0x00C0`..`0x00D0`), as Solaar's rule warns. A rule limited to
  `046d` is still broad.

## 10. Concurrency with Solaar, OpenRGB, ratbagd and G HUB

### 10.1 Software IDs in use

| SW ID | User | Source |
|---|---|---|
| `0x0` | Device notifications (broadcast events) | [Spec] |
| `0x1` | Linux kernel `hid-logitech-hidpp` | [Kernel `LINUX_KERNEL_SW_ID`] |
| `0x7` | OpenRGB (unified HID++ 2.0 controller, `HIDPP20_SW_ID`) | [OpenRGB `LogitechHIDPP20Controller.h`], [Solaar comment] |
| `0x1` | OpenRGB older shared Logitech code (`LOGITECH_CMD_ROOT_GET_FEATURE 0x01`, `..._GET_PROTOCOL 0x11` etc.) | [OpenRGB `LogitechProtocolCommon.h`] |
| `0x8` | libratbag / ratbagd (`DEVICE_SW_ID = 0x8`, comment "kernel uses 0x1") | [libratbag `hidpp20.c`] |
| `0xA` | LGSTrayEx | [Solaar comment] |
| `0xB` | Solaar since PR #3218 (merged 2026-05-14). Before that Solaar rotated `0x2`..`0xF` per request, and before 2024-08 used `0x8` + random 3 bits | [Solaar `base.py`, PR #3218, commit 4578f5f6] |
| `0xD` | Logitech G HUB (host side) | [Solaar comment] |
| `0xF` | Logitech firmware, sub-device self-enumeration on wired transports | [Solaar comment] |
| `0xA` | **this project** (`SoftwareID: 0x0A` in `hidpp/device.go`) | [this repo] |

Solaar's comment (GPL-2.0, `base.py`):

```python
# HID++ Software ID claimed by Solaar. Fixed (not rotated) so cooperative
# userspace HID++ clients sharing the same device can pick a different value
# and reliably filter Solaar's traffic out of their reply stream.
#
# Known values in use by other tools at the time of writing:
#
#   0x07  OpenRGB
#   0x0A  LGSTrayEx
#   0x0D  Logitech G HUB (host-side)
#   0x0F  Logitech firmware (sub-device self-enumeration on wired transports)
SOLAAR_SOFTWARE_ID = 0x0B
```

OpenRGB's header comment (GPL-2.0-or-later) predates the Solaar change and
says "There are only 16 values (4-bit field), and all are claimed. We pick a
fixed value and will coordinate with Solaar to exclude it from its cycle."

**Recommendation:** this project uses `0x0A`, which Solaar lists as
LGSTrayEx's value. `0x0C` or `0x0E` are unclaimed in every list above and
keep bit 3 set (Solaar's convention to keep replies visually distinct from
notifications; the spec only requires non-zero). Making it configurable
would help on unusual setups.

**[HW]** Using a different software ID (low nibble of byte 3) per process lets
two processes talk to the same hidraw node at the same time without picking
up each other's replies.

### 10.2 How each tool matches replies

| Tool | Reply match | Risk with another process |
|---|---|---|
| Kernel | feature index + fn/swid, under `send_mutex` | none for userspace with swid != 1 |
| Solaar ≥ May 2026 | `reply_data[:2] == request_data[:2]` (feature index + fn/swid `0xB`), ping also checks a random mark byte | none if others avoid `0xB` |
| Solaar 2024-08 to 2026-05 | same, but swid rotates through `0x2`..`0xF` | eventually uses every ID, can misattribute replies meant for OpenRGB, ratbagd or this project, and vice versa (the reason for PR #3218) |
| libratbag | `sub_id` (feature index) + `address` (fn/swid `0x8`) | collides with older Solaar versions |
| OpenRGB | feature index + function + `HIDPP20_SW_ID` (`0x7`), discards "foreign error / non-matching frame" | collides with older Solaar versions |
| This project | report 0x10/0x11, device index, feature index, fn/swid `0xA` | collides with LGSTrayEx and older Solaar |

### 10.3 Exclusive access and locking

* hidraw has no exclusive open; Solaar opens with `O_RDWR | O_SYNC`, libratbag
  with `O_RDWR`, OpenRGB through hidapi's hidraw backend. None of them use
  `flock(2)` or any other lock on the node [Solaar, libratbag, OpenRGB
  sources: no `flock`/`LOCK_EX` found].
* A `flock` taken by this project would therefore only coordinate between
  instances of this project. It is still useful to stop two `logirgb`
  processes from interleaving multi-step writes (e.g. a 0x8100 profile
  write sequence), but it does not protect against other tools.
* Tools that use **libusb** instead of hidraw (for example g810-led's libusb
  build) detach the kernel driver from the interface; while they hold it, the
  hidraw node for that interface disappears and other tools lose access
  [Unverified for current g810-led builds; general libusb behaviour].
* Within one process, serialise requests per device (this project holds
  `d.mu` for the whole request/reply cycle; Solaar holds a per-handle lock
  with a 10 s timeout; OpenRGB uses a shared `std::mutex` per receiver).

### 10.4 State conflicts (the bigger problem)

Software IDs solve reply routing, not state. Things other tools do that can
undo or fight with a lighting change:

* **Solaar** re-applies its saved settings when a device (re)connects
  (`settings.apply_all_settings(self)` in `device.py`), including "LED
  Control", "LED Zone Effects", "Onboard Profiles" and per-key colours, so it
  can overwrite lighting set by another tool after a replug or wake [Solaar].
  Its `LEDControl` setting switches 0x8070 between "device" and "Solaar"
  control, and `RGBControl` claims 0x8071 software control [Solaar
  `settings_templates.py`].
* **ratbagd/libratbag** forces onboard mode when it loads a device with 0x8100
  ("not on the correct mode ... set_onboard_mode(device,
  HIDPP20_ONBOARD_MODE)") [libratbag `hidpp20.c`], which undoes host mode set
  by OpenRGB or this project.
* **OpenRGB** claims host/software mode for direct control and keeps writing
  frames; it has a `KEEP_ONBOARD_MODE` quirk for the G915 TKL because "host
  mode breaks the onboard-profile-mapped F-row" [OpenRGB]. Details of the
  G502/G515 quirks are in other-protocols-and-tools.md.
* **G HUB** (Windows, or under Wine) uses software ID `0xD` and also owns the
  device state while running [Solaar comment; Unverified under Wine].

## 11. Summary for this project's transport

| Topic | Current behaviour (`hidpp/*.go`) | Note |
|---|---|---|
| Report type | Always long `0x11`, 20 bytes | Works for all devices that declare `0x11`; very long `0x12` not supported, not needed on the tested devices **[HW]** |
| Discovery | sysfs descriptor contains `06 00 FF` then `85 11` | Misses `0xFF43` keyboards (G213, G512, G610, G810, G815, G910, G Pro, and the G515 over Bluetooth, per OpenRGB) |
| Device index | `0xFF` ×3, then 1..6 | Add 7 for receivers that use it; accept `0x00` replies for Bluetooth |
| Software ID | `0x0A` | Same as LGSTrayEx; prefer `0x0C` or `0x0E` |
| Errors | `0xFF` and `0x8F` handled | Matches kernel and Solaar layouts |
| Notifications | swid 0 queued, others dropped | ASEs carry the requester's swid; kernel traffic (swid 1) is dropped, which is fine |
| Locking | in-process mutex only | Optional `flock` for multi-instance safety |

## Sources

Logitech documentation:

* Logitech `cpg-docs`, HID++ 2.0 README: https://github.com/Logitech/cpg-docs/blob/master/hidpp20/README.rst
* Logitech `cpg-docs`, IRoot 0x0000: https://github.com/Logitech/cpg-docs/blob/master/hidpp20/features/0x0000-IRoot.rst
* HID++ 2.0 draft specification 2012-06-04 (mirror by Peter Wu): https://lekensteyn.nl/files/logitech/logitech_hidpp_2.0_specification_draft_2012-06-04.pdf
* HID++ 1.0 specification for Unifying receivers (mirror): https://lekensteyn.nl/files/logitech/logitech_hidpp10_specification_for_Unifying_Receivers.pdf
* Device Information 0x0003 feature doc (mirror): https://lekensteyn.nl/files/logitech/x0003_deviceinfo.html
* GetProtocolVersion notes (mirror): https://lekensteyn.nl/files/logitech/hidpp4-getprotocolversion.txt
* Mirror index and provenance: https://lekensteyn.nl/files/logitech/

Linux kernel:

* hid-logitech-hidpp.c: https://github.com/torvalds/linux/blob/master/drivers/hid/hid-logitech-hidpp.c
* hid-logitech-dj.c: https://github.com/torvalds/linux/blob/master/drivers/hid/hid-logitech-dj.c
* hid-core.c: https://github.com/torvalds/linux/blob/master/drivers/hid/hid-core.c
* hidraw.c: https://github.com/torvalds/linux/blob/master/drivers/hid/hidraw.c
* hid-ids.h: https://github.com/torvalds/linux/blob/master/drivers/hid/hid-ids.h
* hid-quirks.c (Bolt `ALWAYS_POLL`): https://github.com/torvalds/linux/blob/master/drivers/hid/hid-quirks.c
* "use RAP instead of FAP to get the protocol version": https://lkml.iu.edu/hypermail/linux/kernel/1905.2/06420.html
* HIDIOCREVOKE patch series: https://lkml.rescloud.iu.edu/2408.3/01910.html
* Peter Hutterer, "HIDIOCREVOKE merged for kernel 6.12": https://who-t.blogspot.com/2024/10/

Solaar:

* Repository: https://github.com/pwr-Solaar/Solaar
* base.py: https://github.com/pwr-Solaar/Solaar/blob/master/lib/logitech_receiver/base.py
* base_usb.py: https://github.com/pwr-Solaar/Solaar/blob/master/lib/logitech_receiver/base_usb.py
* udev_impl.py: https://github.com/pwr-Solaar/Solaar/blob/master/lib/hidapi/udev_impl.py
* udev rule: https://github.com/pwr-Solaar/Solaar/blob/master/rules.d/42-logitech-unify-permissions.rules
* PR #3218 "fix sw_id at 0x0B instead of rotating 0x2..0xF": https://github.com/pwr-Solaar/Solaar/pull/3218

OpenRGB:

* Repository: https://gitlab.com/CalcProgrammer1/OpenRGB
* Logitech controllers: https://gitlab.com/CalcProgrammer1/OpenRGB/-/tree/master/Controllers/LogitechController
* udev rule generator: https://gitlab.com/CalcProgrammer1/OpenRGB/-/blob/release_candidate_1.0rc2/scripts/build-udev-rules.sh
* Issue #3675 (60-openrgb.rules format examples): https://gitlab.com/CalcProgrammer1/OpenRGB/-/issues/3675
* MR !3496 (Bluetooth HID++ 2.0, G515 on page 0xFF43): https://gitlab.com/CalcProgrammer1/OpenRGB/-/merge_requests/3496

libratbag:

* hidpp20.c: https://github.com/libratbag/libratbag/blob/master/src/hidpp20.c
* hidpp-generic.c: https://github.com/libratbag/libratbag/blob/master/src/hidpp-generic.c
* driver-hidpp20.c: https://github.com/libratbag/libratbag/blob/master/src/driver-hidpp20.c
* libratbag-hidraw.c: https://github.com/libratbag/libratbag/blob/master/src/libratbag-hidraw.c

udev ordering:

* picotool PR #199: https://github.com/raspberrypi/picotool/pull/199
* Arch bug FS#63164: https://bugs.archlinux.org/task/63164
* NixOS discourse on 99-local.rules and uaccess: https://discourse.nixos.org/t/adding-rules-to-udev-easy-or-a-consenus-best-practice/78323/12
