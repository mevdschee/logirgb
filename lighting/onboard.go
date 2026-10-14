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

// ProfileCount returns the number of onboard profiles.
func (c *Controller) ProfileCount() (int, error) {
	if !c.HasOnboardProfiles() {
		return 0, errors.New("device has no onboard profiles")
	}
	info, err := c.profileInfo()
	return info.count, err
}

// ActiveProfile returns the number (1-based) of the profile the device is
// running.
func (c *Controller) ActiveProfile() (int, error) {
	if !c.HasOnboardProfiles() {
		return 0, errors.New("device has no onboard profiles")
	}
	r, err := c.dev.Call(hidpp.FeatureOnboardProfiles, 4)
	if err != nil {
		return 0, err
	}
	sector := int(r[0])<<8 | int(r[1])
	if sector >= romSectorBase {
		return sector - romSectorBase, nil
	}
	info, err := c.profileInfo()
	if err != nil {
		return 0, err
	}
	entries, err := c.userDirectory(info)
	if err != nil {
		return 0, err
	}
	for i, e := range entries {
		if e[0] == sector {
			return i + 1, nil
		}
	}
	return sector, nil
}

// Profile is the content of an onboard profile that this package edits.
type Profile struct {
	// Effects holds the zone effects. Zones without an effect keep their
	// stored LED block when writing, and are left out when reading a block
	// with an effect the zone does not support.
	Effects map[int]Effect
	// Settings holds the report rate and DPI levels; nil keeps the stored
	// ones when writing.
	Settings *ProfileSettings
	// Enabled is whether the profile button cycles to the profile; nil
	// keeps the stored flag when writing.
	Enabled *bool
}

// ReadProfiles returns what each onboard profile holds; element i is
// profile i+1.
func (c *Controller) ReadProfiles() ([]Profile, error) {
	if !c.HasOnboardProfiles() {
		return nil, errors.New("device has no onboard profiles")
	}
	info, err := c.profileInfo()
	if err != nil {
		return nil, err
	}
	entries, err := c.userDirectory(info)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		for i := 1; i <= info.count; i++ {
			entries = append(entries, [2]int{romSectorBase + i, 1})
		}
	}
	var out []Profile
	for _, entry := range entries {
		data, err := c.readSector(entry[0], info.sectorSize)
		if err != nil {
			return nil, err
		}
		s := decodeSettings(data)
		enabled := entry[1] != 0
		out = append(out, Profile{Effects: c.decodeLEDs(data), Settings: &s, Enabled: &enabled})
	}
	return out, nil
}

// decodeLEDs reads the zone effects from a profile's LED blocks.
func (c *Controller) decodeLEDs(data []byte) map[int]Effect {
	effects := map[int]Effect{}
	for zone := range c.zones {
		led := data[ledOffset+c.ledSlot(zone)*ledSize:][:ledSize]
		if ze, ok := c.zoneEffect(zone, led[0]); ok {
			effects[zone] = decode(ze.Type, led[1:])
		}
	}
	return effects
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

// SaveToProfiles writes zone effects, settings and enabled flags into the
// onboard profiles, so they survive profile switches and power cycles.
// profiles[i] is profile i+1. On a device that still runs its factory
// profiles, these are first copied to the user sectors. The active profile
// and the last enabled one cannot be disabled. It returns the number of
// profiles written.
func (c *Controller) SaveToProfiles(profiles []Profile) (int, error) {
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
	leds := make([]map[int][]byte, len(profiles))
	settings := make([]*ProfileSettings, len(profiles))
	for p, prof := range profiles {
		if prof.Settings != nil {
			s := prof.Settings.compact()
			if err := c.validate(s); err != nil {
				return 0, fmt.Errorf("profile %d: %w", p+1, err)
			}
			settings[p] = &s
		}
		leds[p] = map[int][]byte{}
		for zone, e := range prof.Effects {
			if zone < 0 || zone >= len(c.zones) {
				return 0, fmt.Errorf("no zone %d", zone)
			}
			ze, ok := c.zones[zone].Find(e.Name)
			if !ok {
				return 0, fmt.Errorf("zone %d does not support %q", zone, e.Name)
			}
			params := e.encode(ze.Type)
			leds[p][zone] = append([]byte{byte(ze.Type.ID)}, params[:]...)
		}
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
	flagsChanged := false
	for i := range entries {
		if i >= len(profiles) || profiles[i].Enabled == nil {
			continue
		}
		flag := 0
		if *profiles[i].Enabled {
			flag = 1
		}
		if flag != entries[i][1] {
			entries[i][1] = flag
			flagsChanged = true
		}
	}
	if flagsChanged {
		enabled := 0
		for i, e := range entries {
			if e[1] == 0 && current != nil && int(current[1]) == i+1 {
				return 0, fmt.Errorf("profile %d is active and cannot be disabled", i+1)
			}
			if e[1] != 0 {
				enabled++
			}
		}
		if enabled == 0 {
			return 0, errors.New("at least one profile has to stay enabled")
		}
	}
	// Re-selecting the profile below resets the DPI to the profile default,
	// so remember the current DPI level and restore it afterwards.
	dpi, dpiErr := c.dev.Call(hidpp.FeatureOnboardProfiles, 0xB)

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
		if i < len(leds) {
			for zone, led := range leds[i] {
				copy(data[ledOffset+c.ledSlot(zone)*ledSize:], led)
			}
			if settings[i] != nil {
				encodeSettings(data, *settings[i])
			}
		}
		sealSector(data)
		if err := c.writeSector(sector, data); err != nil {
			return i, err
		}
	}

	switch {
	case fromROM:
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
	case flagsChanged:
		// Change only the enabled flags; the profile button skips the
		// profiles with flag 0.
		dir, err := c.readSector(0, info.sectorSize)
		if err != nil {
			return len(entries), err
		}
		for i, e := range entries {
			dir[i*dirEntrySize+2] = byte(e[1])
		}
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
			if dpiErr == nil {
				// Stay on the same level, or the last one if the profile
				// now has fewer levels.
				level := dpi[0]
				if idx <= len(settings) && settings[idx-1] != nil {
					level = byte(min(int(level), settings[idx-1].Levels()-1))
				}
				c.dev.Call(hidpp.FeatureOnboardProfiles, 0xC, level)
			}
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
