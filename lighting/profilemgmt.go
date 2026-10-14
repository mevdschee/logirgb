package lighting

import (
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/mevdschee/logitech-rgb/hidpp"
)

// Keyboards with 0x8071 RGBEffects (G515) keep their onboard profile in
// files managed by 0x8101 ProfileManagement. The layout below was observed on
// the G515 TKL (wired and LIGHTSPEED) and is validated before every write.
//
// Profile (99 bytes):
//
//	0-1    header 01 01
//	2-3    idle timeout in seconds (the firmware dims), big endian
//	4-5    sleep timeout in seconds (the firmware turns the lighting off)
//	6-29   name
//	30-73  four 11 byte [effect ID, params] lighting records; with the flags
//	       byte at 0 the firmware shows record 1 and switches to record 3
//	       while idle
//	74     lighting flags
//	75-98  key mask and other data, preserved
//
// Directory entry (16 bytes): CRC32 of bytes 4-15 at 0-3, a fixed header
// at 4-9, the CRC32 of the profile at 10-13 and the profile location at 14.
const (
	pmFactoryProfile = 0x0101
	pmFactoryDir     = 0x0100
	pmUserProfile    = 0x0008
	pmUserDir        = 0x0000
	pmProfileSize    = 99
	pmDirSize        = 16
	pmRecords        = 30
	pmFlags          = 74
)

var pmDirHeader = []byte{0x01, 0x01, 0x81, 0x01, 0x40, 0x63}

// HasProfileStorage reports whether lighting and idle timers can be stored
// in a keyboard's onboard profile, which makes the firmware dim and switch
// off the lighting by itself.
func (c *Controller) HasProfileStorage() bool {
	return c.feature == hidpp.FeatureRGBEffects && c.dev.HasFeature(hidpp.FeatureProfileManagement)
}

func (c *Controller) pmCall(fn byte, params ...byte) ([]byte, error) {
	return c.dev.Call(hidpp.FeatureProfileManagement, fn, params...)
}

// pmRead loads a file into the device buffer and reads it back. It returns
// nil without error for user files that were never written.
func (c *Controller) pmRead(addr, size int) ([]byte, error) {
	if _, err := c.pmCall(8, byte(addr>>8), byte(addr), byte(size>>8), byte(size)); err != nil {
		var herr *hidpp.Error
		if errors.As(err, &herr) && herr.Code == 0x04 && addr < pmFactoryDir {
			return nil, nil
		}
		return nil, fmt.Errorf("load file %#04x: %w", addr, err)
	}
	return c.pmBuffer(size)
}

func (c *Controller) pmBuffer(size int) ([]byte, error) {
	out := make([]byte, 0, size+16)
	for o := 0; o < size; o += 16 {
		r, err := c.pmCall(0xC, byte(o>>8), byte(o))
		if err != nil {
			return nil, fmt.Errorf("read buffer: %w", err)
		}
		out = append(out, r...)
	}
	return out[:size], nil
}

// pmSave stages data in the device buffer, verifies it, commits it to flash
// and verifies the flash copy.
func (c *Controller) pmSave(addr int, data []byte, checksum uint32) error {
	if addr != pmUserProfile && addr != pmUserDir {
		return fmt.Errorf("refusing to write file %#04x", addr)
	}
	n := len(data)
	if _, err := c.pmCall(2, byte(n>>8), byte(n)); err != nil {
		return fmt.Errorf("start buffer: %w", err)
	}
	for o := 0; o < n; o += 16 {
		if _, err := c.pmCall(3, data[o:min(o+16, n)]...); err != nil {
			return fmt.Errorf("fill buffer: %w", err)
		}
	}
	staged, err := c.pmBuffer(n)
	if err != nil {
		return err
	}
	if !bytes.Equal(staged, data) {
		return errors.New("buffer verification failed, flash not written")
	}
	if _, err := c.pmCall(7, byte(addr>>8), byte(addr), byte(n>>8), byte(n),
		byte(checksum>>24), byte(checksum>>16), byte(checksum>>8), byte(checksum)); err != nil {
		return fmt.Errorf("save file %#04x: %w", addr, err)
	}
	back, err := c.pmRead(addr, n)
	if err != nil {
		return err
	}
	if !bytes.Equal(back, data) {
		return fmt.Errorf("flash readback of file %#04x differs", addr)
	}
	return nil
}

func validProfile(p []byte) bool {
	return len(p) == pmProfileSize && p[0] == 0x01 && p[1] == 0x01
}

func validDir(d []byte) bool {
	return len(d) == pmDirSize && bytes.Equal(d[4:10], pmDirHeader) && d[15] == 0xFF &&
		crc32.ChecksumIEEE(d[4:]) == be32(d[0:4])
}

func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func putBE32(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
}

type pmFiles struct {
	factoryProfile, factoryDir, userProfile, userDir []byte
}

func (c *Controller) pmSnapshot() (*pmFiles, error) {
	var f pmFiles
	var err error
	if f.factoryProfile, err = c.pmRead(pmFactoryProfile, pmProfileSize); err != nil {
		return nil, err
	}
	if f.factoryDir, err = c.pmRead(pmFactoryDir, pmDirSize); err != nil {
		return nil, err
	}
	if !validProfile(f.factoryProfile) || !validDir(f.factoryDir) {
		return nil, errors.New("unknown onboard profile layout, not touching it")
	}
	if f.userProfile, err = c.pmRead(pmUserProfile, pmProfileSize); err != nil {
		return nil, err
	}
	if f.userDir, err = c.pmRead(pmUserDir, pmDirSize); err != nil {
		return nil, err
	}
	if (f.userProfile == nil) != (f.userDir == nil) {
		return nil, errors.New("user profile storage is half written, not touching it")
	}
	if f.userProfile != nil {
		if !validProfile(f.userProfile) || !validDir(f.userDir) || f.userDir[14] != pmUserProfile ||
			crc32.ChecksumIEEE(f.userProfile) != be32(f.userDir[10:14]) {
			return nil, errors.New("user profile does not match its directory entry, not touching it")
		}
	}
	return &f, nil
}

// StoreProfile writes an effect, its dimmed variant (dimPercent of the
// colors and intensity) and the idle/sleep timeouts into the keyboard's
// onboard profile and activates it. The firmware then shows the effect,
// dims it after idle seconds and turns it off after sleep seconds.
func (c *Controller) StoreProfile(e Effect, dimPercent, idle, sleep int) error {
	if !c.HasProfileStorage() {
		return errors.New("device has no profile storage")
	}
	if len(c.zones) != 1 {
		return fmt.Errorf("expected 1 lighting zone, device has %d", len(c.zones))
	}
	ze, ok := c.zones[0].Find(e.Name)
	if !ok {
		return fmt.Errorf("effect %q is not supported", e.Name)
	}
	f, err := c.pmSnapshot()
	if err != nil {
		return err
	}

	base := f.userProfile
	if base == nil {
		base = f.factoryProfile
	}
	p := bytes.Clone(base)
	p[2], p[3] = byte(idle>>8), byte(idle)
	p[4], p[5] = byte(sleep>>8), byte(sleep)
	clear(p[pmRecords : pmRecords+4*ledSize])
	record := func(slot int, eff Effect) {
		params := eff.encode(ze.Type)
		p[pmRecords+slot*ledSize] = byte(ze.Type.ID)
		copy(p[pmRecords+slot*ledSize+1:], params[:])
	}
	record(1, e)
	record(3, e.dimmed(ze.Type, dimPercent))
	p[pmFlags] = 0x00

	dir := bytes.Clone(f.factoryDir)
	putBE32(dir[10:14], crc32.ChecksumIEEE(p))
	dir[14] = pmUserProfile
	putBE32(dir[0:4], crc32.ChecksumIEEE(dir[4:]))

	if !bytes.Equal(p, f.userProfile) {
		if err := c.pmSave(pmUserProfile, p, crc32.ChecksumIEEE(p)); err != nil {
			return err
		}
	}
	if !bytes.Equal(dir, f.userDir) {
		if err := c.pmSave(pmUserDir, dir, be32(dir[0:4])); err != nil {
			return err
		}
	}
	return c.activateProfile(p)
}

// activateProfile hands the lighting back to the firmware and makes it
// load the stored user profile.
func (c *Controller) activateProfile(p []byte) error {
	if err := c.setSWControl(swRelease); err != nil {
		return err
	}
	if _, err := c.pmCall(6, 0x0B); err != nil {
		return fmt.Errorf("onboard mode: %w", err)
	}
	cfg := []byte{0x81, 0x01, 0x01, 0x00, 0x01, 0x00, 0x63, 0, 0, 0, 0}
	putBE32(cfg[7:], crc32.ChecksumIEEE(p))
	if _, err := c.pmCall(9, cfg...); err != nil {
		return fmt.Errorf("activate profile: %w", err)
	}
	m, err := c.pmCall(6, 0x00)
	if err != nil {
		return err
	}
	if m[0]&1 == 0 || m[1] != 0x00 || m[2] != 0x01 {
		return fmt.Errorf("unexpected onboard state % x after activation", m[:3])
	}
	return nil
}

// dimmed returns the effect with its colors and intensity scaled.
func (e Effect) dimmed(t EffectType, percent int) Effect {
	out := Effect{Name: e.Name}
	for _, spec := range t.Params() {
		v := e.Get(t, spec.Name)
		switch {
		case spec.Name.IsColor():
			v = scaleColor(v, percent)
		case spec.Name == ParamIntensity:
			v = max(1, v*percent/100)
		}
		out.Set(spec.Name, v)
	}
	return out
}

func scaleColor(c, percent int) int {
	r := (c >> 16 & 0xFF) * percent / 100
	g := (c >> 8 & 0xFF) * percent / 100
	b := (c & 0xFF) * percent / 100
	return r<<16 | g<<8 | b
}

// BackupProfiles returns the raw onboard profile storage, keyed by a file
// name, so callers can keep a copy before writing.
func (c *Controller) BackupProfiles() (map[string][]byte, error) {
	out := map[string][]byte{}
	switch {
	case c.HasProfileStorage():
		f, err := c.pmSnapshot()
		if err != nil {
			return nil, err
		}
		out["factory-profile.bin"] = f.factoryProfile
		out["factory-directory.bin"] = f.factoryDir
		if f.userProfile != nil {
			out["user-profile.bin"] = f.userProfile
			out["user-directory.bin"] = f.userDir
		}
	case c.HasOnboardProfiles():
		info, err := c.profileInfo()
		if err != nil {
			return nil, err
		}
		sectors := []int{0}
		for i := 1; i <= info.count; i++ {
			sectors = append(sectors, i, romSectorBase+i)
		}
		for _, s := range sectors {
			data, err := c.readSector(s, info.sectorSize)
			if err != nil {
				return nil, err
			}
			out[fmt.Sprintf("sector-%04x.bin", s)] = data
		}
	default:
		return nil, errors.New("device has no onboard profiles")
	}
	return out, nil
}
