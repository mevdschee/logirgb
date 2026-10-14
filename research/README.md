# Research notes

Background material on the RGB lighting protocols of Logitech G5xx devices
(the G502 HERO mouse family, the G512/G513/G515 keyboards and their
relatives), collected while building logirgb. Each file marks where a claim
comes from:

- **Logitech spec:** the published HID++ 2.0 feature documents.
- **Hardware:** verified on a G515 TKL (`046d:c358`) or a G502 SE HERO
  (`046d:c08b`).
- **Tool source:** read in a named tool, such as Solaar, OpenRGB, libratbag,
  g810-led or logitech-g515-onboard.
- **Unverified:** everything else.

| File | Contents |
|---|---|
| [hardware-log.md](hardware-log.md) | Raw results of every test run on the two devices: dumps, replies, what was visible |
| [devices.md](devices.md) | Catalog of G5xx products: USB IDs, HID++ features, zones, effects, Linux tool support |
| [hidpp-transport.md](hidpp-transport.md) | HID++ framing, device indexes, root and feature set, errors, notifications, finding the hidraw node, receivers, udev, sharing a device between tools |
| [8070-color-led-effects.md](8070-color-led-effects.md) | 0x8070 ColorLEDEffects (G502 HERO): functions, effects, parameter layouts |
| [8071-rgb-effects.md](8071-rgb-effects.md) | 0x8071 RGBEffects (G515, G502 X PLUS): functions, effects, software control, power modes and timers, NvConfig, events |
| [8080-8081-per-key-lighting.md](8080-8081-per-key-lighting.md) | Per-key lighting, including the G515 zone ID map |
| [8040-brightness-control.md](8040-brightness-control.md) | 0x8040 BrightnessControl |
| [8100-onboard-profiles.md](8100-onboard-profiles.md) | 0x8100 OnboardProfiles: sectors, directory, profile format, LED blocks, CRC, decoded G502 HERO ROM profiles |
| [8101-profile-management.md](8101-profile-management.md) | 0x8101 ProfileManagement (G515): files, directory entries, the 99 byte profile with lighting records and idle/sleep timers |
| [other-protocols-and-tools.md](other-protocols-and-tools.md) | g810-led protocol, LIGHTSYNC, the LED SDK, G HUB behavior, Linux tool comparison, known issues |

## Logitech specifications

Logitech has published HID++ 2.0 feature documents, including x8040, x8070,
x8071 and x8081, in a public folder that Solaar links to:
<https://drive.google.com/drive/folders/0BxbRzx7vEV7eWmgwazJ3NUFfQ28>.
They are not copied into this repository. Nothing public exists for 0x8080,
0x8100 or 0x8101; those notes are based on tool sources and hardware tests.
