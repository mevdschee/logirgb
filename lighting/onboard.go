package lighting

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/mevdschee/logirgb/hidpp"
)

// Onboard profiles (0x8100) live in flash sectors. Sector 0 is the profile
// directory, sectors 1..n hold user profiles and sectors 0x0101.. hold the
// read-only factory (ROM) profiles. While the directory is empty or has a
// bad CRC the device runs the ROM profiles. Each profile stores one 11 byte
// LED block per zone at offset 208 in the same [effect ID, params] format
// that SetZoneEffect uses (see ledSlot for the order).
const (
	romSectorBase   = 0x0100
	ledOffset       = 208
	ledSize         = 11
	dirEntrySize    = 4
	dirEnd          = 0xFFFF
	minProfileBytes = ledOffset + 2*ledSize + 2
)

// HasOnboardProfiles reports whether lighting can be stored in the
// device's onboard profiles.
func (c *Controller) HasOnboardProfiles() bool {
	return c.feature == hidpp.FeatureColorLEDEffects && c.dev.HasFeature(hidpp.FeatureOnboardProfiles)
}

type profileInfo struct {
	count      int
	sectorSize int
}

func (c *Controller) profileInfo() (profileInfo, error) {
	r, err := c.dev.Call(hidpp.FeatureOnboardProfiles, 0)
	if err != nil {
		return profileInfo{}, err
	}
	if r[0] != 0x01 {
		return profileInfo{}, fmt.Errorf("unsupported onboard memory model %d", r[0])
	}
	info := profileInfo{count: int(r[3]), sectorSize: int(r[7])<<8 | int(r[8])}
	if info.sectorSize < minProfileBytes || info.count == 0 {
		return profileInfo{}, fmt.Errorf("unexpected profile layout (%d profiles, sector size %d)", info.count, info.sectorSize)
	}
	return info, nil
}

func (c *Controller) readSector(sector, size int) ([]byte, error) {
	out := make([]byte, size)
	for o := 0; o < size; o += 16 {
		// Reads return 16 bytes and may not run past the sector end, so
		// the last read overlaps the previous one.
		off := min(o, size-16)
		r, err := c.dev.Call(hidpp.FeatureOnboardProfiles, 5, byte(sector>>8), byte(sector), byte(off>>8), byte(off))
		if err != nil {
			return nil, fmt.Errorf("read sector %#04x: %w", sector, err)
		}
		copy(out[off:], r)
	}
	return out, nil
}

func (c *Controller) writeSector(sector int, data []byte) error {
	n := len(data)
	if _, err := c.dev.Call(hidpp.FeatureOnboardProfiles, 6, byte(sector>>8), byte(sector), 0, 0, byte(n>>8), byte(n)); err != nil {
		return fmt.Errorf("start write sector %#04x: %w", sector, err)
	}
	for o := 0; o < n; o += 16 {
		chunk := make([]byte, 16)
		copy(chunk, data[o:])
		if _, err := c.dev.Call(hidpp.FeatureOnboardProfiles, 7, chunk...); err != nil {
			return fmt.Errorf("write sector %#04x: %w", sector, err)
		}
	}
	if _, err := c.dev.Call(hidpp.FeatureOnboardProfiles, 8); err != nil {
		return fmt.Errorf("finish write sector %#04x: %w", sector, err)
	}
	back, err := c.readSector(sector, n)
	if err != nil {
		return err
	}
	if !bytes.Equal(back, data) {
		return fmt.Errorf("verify sector %#04x: read back differs from what was written", sector)
	}
	return nil
}

// crc16 is CRC-16/CCITT-FALSE, which the firmware uses to validate sectors.
func crc16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func validSector(data []byte) bool {
	n := len(data)
	return crc16(data[:n-2]) == uint16(data[n-2])<<8|uint16(data[n-1])
}

func sealSector(data []byte) {
	n := len(data)
	crc := crc16(data[:n-2])
	data[n-2], data[n-1] = byte(crc>>8), byte(crc)
}

// userDirectory returns the user profile sectors and their enabled flags,
// or nil when the device is running its ROM profiles.
func (c *Controller) userDirectory(info profileInfo) ([][2]int, error) {
	dir, err := c.readSector(0, info.sectorSize)
	if err != nil {
		return nil, err
	}
	if !validSector(dir) {
		return nil, nil
	}
	var entries [][2]int
	for i := 0; i+dirEntrySize <= info.sectorSize-2 && len(entries) < info.count; i += dirEntrySize {
		sector := int(dir[i])<<8 | int(dir[i+1])
		if sector == dirEnd {
			break
		}
		entries = append(entries, [2]int{sector, int(dir[i+2])})
	}
	return entries, nil
}

// SaveToProfiles writes the given zone effects into every onboard profile,
// so they survive profile switches and power cycles. On a device that still
// runs its factory profiles, these are first copied to the user sectors.
// It returns the number of profiles written.
func (c *Controller) SaveToProfiles(effects map[int]Effect) (int, error) {
	if !c.HasOnboardProfiles() {
		return 0, errors.New("device has no onboard profiles")
	}
	info, err := c.profileInfo()
	if err != nil {
		return 0, err
	}
	if len(c.zones) > 2 {
		return 0, fmt.Errorf("profiles hold 2 LED zones, device has %d", len(c.zones))
	}

	// Build the LED blocks first so invalid input fails before any write.
	leds := map[int][]byte{}
	for zone, e := range effects {
		if zone < 0 || zone >= len(c.zones) {
			return 0, fmt.Errorf("no zone %d", zone)
		}
		ze, ok := c.zones[zone].Find(e.Name)
		if !ok {
			return 0, fmt.Errorf("zone %d does not support %q", zone, e.Name)
		}
		params := e.encode(ze.Type)
		leds[zone] = append([]byte{byte(ze.Type.ID)}, params[:]...)
	}

	entries, err := c.userDirectory(info)
	if err != nil {
		return 0, err
	}
	fromROM := entries == nil
	if fromROM {
		for i := 1; i <= info.count; i++ {
			entries = append(entries, [2]int{i, 1})
		}
	}

	current, _ := c.dev.Call(hidpp.FeatureOnboardProfiles, 4)

	for i, entry := range entries {
		sector := entry[0]
		if sector >= romSectorBase {
			return 0, fmt.Errorf("profile %d points at read-only sector %#04x", i+1, sector)
		}
		src := sector
		if fromROM {
			src = romSectorBase + sector
		}
		data, err := c.readSector(src, info.sectorSize)
		if err != nil {
			return i, err
		}
		if !fromROM && !validSector(data) {
			return i, fmt.Errorf("profile %d (sector %#04x) has a bad checksum, not touching it", i+1, sector)
		}
		for zone, led := range leds {
			copy(data[ledOffset+c.ledSlot(zone)*ledSize:], led)
		}
		sealSector(data)
		if err := c.writeSector(sector, data); err != nil {
			return i, err
		}
	}

	if fromROM {
		// Only now point the directory at the new user profiles; until this
		// write the device keeps running the untouched ROM profiles.
		dir := bytes.Repeat([]byte{0xFF}, info.sectorSize)
		o := 0
		for _, e := range entries {
			copy(dir[o:], []byte{byte(e[0] >> 8), byte(e[0]), byte(e[1]), 0x00})
			o += dirEntrySize
		}
		copy(dir[o:], []byte{0xFF, 0xFF, 0x00, 0x00})
		sealSector(dir)
		if err := c.writeSector(0, dir); err != nil {
			return len(entries), err
		}
	}

	// Re-select the active profile so the firmware reloads it from flash.
	// The reply is the active sector; ROM sector 0x01NN became user sector
	// 0x00NN, so the low byte is the profile number either way.
	if current != nil {
		if idx := int(current[1]); idx >= 1 && idx <= len(entries) {
			c.dev.Call(hidpp.FeatureOnboardProfiles, 3, 0x00, byte(idx))
		}
	}
	return len(entries), nil
}

// ledSlot maps a zone to its LED block in a profile. The profile stores the
// zones in reverse order: on the G502 HERO block 0 drives zone 1 (logo) and
// block 1 drives zone 0 (primary, the DPI indicator).
func (c *Controller) ledSlot(zone int) int {
	return len(c.zones) - 1 - zone
}

// ResetProfiles clears the profile directory so the device goes back to its
// factory (ROM) profiles. The user sectors are left as they are.
func (c *Controller) ResetProfiles() error {
	if !c.HasOnboardProfiles() {
		return errors.New("device has no onboard profiles")
	}
	info, err := c.profileInfo()
	if err != nil {
		return err
	}
	current, _ := c.dev.Call(hidpp.FeatureOnboardProfiles, 4)
	if err := c.writeSector(0, bytes.Repeat([]byte{0xFF}, info.sectorSize)); err != nil {
		return err
	}
	if current != nil && current[1] >= 1 {
		c.dev.Call(hidpp.FeatureOnboardProfiles, 3, 0x01, current[1])
	}
	return nil
}
