package lighting

import (
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"

	"github.com/mevdschee/logirgb/hidpp"
)

// Keyboards with 0x8071 RGBEffects (G515) keep their onboard profiles in
// files managed by 0x8101 ProfileManagement. The layout below was observed on
// the G515 TKL (wired and LIGHTSPEED) and the G515 Rapid TKL, and is
// validated before every write.
//
// Profile (99 bytes on the G515 TKL, 109 on the Rapid):
//
//	0      profile number (G HUB writes 00 for user profile 1)
//	1      format 01
//	2-3    idle timeout in seconds (the firmware dims), big endian,
//	       0xFFFF for never
//	4-5    sleep timeout in seconds (the firmware turns the lighting off),
//	       0xFFFF for never
//	6-29   name
//	30-73  four 11 byte [effect ID, params] lighting records; with the flags
//	       byte at 0 the firmware shows record 1 and switches to record 3
//	       while idle
//	74     lighting flags
//	75-    key mask, references to other files and other data, preserved
//
// A directory file is [crc32 u32, count, count 10 byte entries, ff], each
// entry [id, type u16, flags, length, crc32 u32, location]. The CRC at 0-3
// covers bytes 4 to the end, the entry CRC covers the whole file it points
// to. Lighting profiles have type 0x8101; the Rapid also lists 0x1B08
// analog key files, which are left alone. The factory directory is at
// 0x0100 and its files at 0x01xx, the user directory at 0x0000 and its files
// at 0x00xx.
const (
	pmFactoryDir   = 0x0100
	pmUserProfile  = 0x0008
	pmUserDir      = 0x0000
	pmProfileSize  = 99 // the smallest known profile, G515 TKL
	pmDirSize      = 16
	pmRecords      = 30
	pmFlags        = 74
	pmDirEntrySize = 10
	pmDirMaxCount  = 24
)

// pmDirHeader is the start of the single entry directory of the G515 TKL,
// the only layout for which a user directory is created from scratch.
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
	if addr < 0 || addr >= pmFactoryDir {
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

func validProfile(p []byte, size int) bool {
	return len(p) == size && size >= pmProfileSize && p[1] == 0x01
}

// validDir checks the structure of a full directory file and, with
// checkCRC, its own checksum. The factory directory of the Rapid has a
// checksum that does not match, like the factory file checksums do not.
func validDir(d []byte, checkCRC bool) bool {
	if len(d) < pmDirSize || pmDirLen(d) != len(d) || d[len(d)-1] != 0xFF {
		return false
	}
	return !checkCRC || crc32.ChecksumIEEE(d[4:]) == be32(d[0:4])
}

// pmProfileEntry returns the offset of the lighting profile entry with the
// given ID in a directory, or -1.
func pmProfileEntry(dir []byte, id byte) int {
	for i, e := range pmDirEntries(dir) {
		if e[0] == id && e[1] == 0x81 && e[2] == 0x01 {
			return 5 + i*pmDirEntrySize
		}
	}
	return -1
}

// pmDirWithFile returns a copy of dir with the checksum of the file of the
// entry at off replaced and the directory checksum recomputed.
func pmDirWithFile(dir []byte, off int, file []byte) []byte {
	d := bytes.Clone(dir)
	putBE32(d[off+5:off+9], crc32.ChecksumIEEE(file))
	putBE32(d[0:4], crc32.ChecksumIEEE(d[4:]))
	return d
}

func be32(b []byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func putBE32(b []byte, v uint32) {
	b[0], b[1], b[2], b[3] = byte(v>>24), byte(v>>16), byte(v>>8), byte(v)
}

// pmDirLen returns the length of a directory file from its first bytes.
func pmDirLen(head []byte) int {
	if len(head) < 5 || head[4] == 0 || head[4] > pmDirMaxCount {
		return len(head)
	}
	return 5 + int(head[4])*pmDirEntrySize + 1
}

// pmDirEntries returns the complete entries of a directory file.
func pmDirEntries(dir []byte) [][]byte {
	if len(dir) < 5 {
		return nil
	}
	var out [][]byte
	for i := range int(dir[4]) {
		o := 5 + i*pmDirEntrySize
		if o+pmDirEntrySize > len(dir) {
			break
		}
		out = append(out, dir[o:o+pmDirEntrySize])
	}
	return out
}

// DumpedFile is one file read by DumpProfileFiles.
type DumpedFile struct {
	Addr int
	Data []byte // nil for a user file that was never written
	Err  error
}

// ProfileDump is the raw onboard profile state of a 0x8101 keyboard.
type ProfileDump struct {
	Info, Mode []byte
	Files      []DumpedFile
}

// DumpProfileFiles reads the onboard profile files without validating
// them, so an unknown layout can be reported and studied. It reads each
// directory at its full length and every file it lists, with the length the
// directory gives. It only loads and reads, it never writes.
func (c *Controller) DumpProfileFiles() (*ProfileDump, error) {
	if !c.HasProfileStorage() {
		return nil, errors.New("device has no profile storage")
	}
	var d ProfileDump
	d.Info, _ = c.pmCall(0)
	d.Mode, _ = c.pmCall(6, 0x00)
	read := func(addr, size int) []byte {
		b, err := c.pmRead(addr, size)
		d.Files = append(d.Files, DumpedFile{Addr: addr, Data: b, Err: err})
		return b
	}
	for _, dirAddr := range []int{pmFactoryDir, pmUserDir} {
		dir, err := c.pmReadDir(dirAddr)
		d.Files = append(d.Files, DumpedFile{Addr: dirAddr, Data: dir, Err: err})
		if err != nil {
			continue
		}
		for _, e := range pmDirEntries(dir) {
			if e[4] > 0 {
				read(dirAddr&0xFF00|int(e[9]), int(e[4]))
			}
		}
	}
	return &d, nil
}

// pmReadDir reads a directory file at its full length. It returns nil
// without error for a user directory that was never written.
func (c *Controller) pmReadDir(addr int) ([]byte, error) {
	head, err := c.pmRead(addr, pmDirSize)
	if err != nil || head == nil {
		return head, err
	}
	if n := pmDirLen(head); n > len(head) {
		return c.pmRead(addr, n)
	}
	return head, nil
}

// pmFiles is the onboard state of one lighting profile: the factory copy,
// and the user copy with its directory if there is one.
type pmFiles struct {
	id             byte // the profile's directory entry ID
	size           int
	factoryProfile []byte
	factoryDir     []byte
	userDir        []byte // nil if never written
	userEntry      int    // offset of the profile's entry in userDir
	userAddr       int    // file of the user profile, -1 if it cannot be created
	userProfile    []byte // nil if never written
}

// pmActiveID returns the ID of the lighting profile the firmware runs, or
// profile 1 when it runs something that is not a lighting profile, like
// the customizable analog profile (00 fd) of the Rapid.
func (c *Controller) pmActiveID(factoryDir []byte) byte {
	m, err := c.pmCall(6, 0x00)
	if err == nil && len(m) >= 3 && m[1] <= 0x01 && pmProfileEntry(factoryDir, m[2]) >= 0 {
		return m[2]
	}
	return 1
}

var errUnknownLayout = errors.New("unknown onboard profile layout, not touching it")

func (c *Controller) pmSnapshot() (*pmFiles, error) {
	f := pmFiles{userAddr: -1}
	var err error
	if f.factoryDir, err = c.pmReadDir(pmFactoryDir); err != nil {
		return nil, err
	}
	if !validDir(f.factoryDir, false) {
		return nil, errUnknownLayout
	}
	f.id = c.pmActiveID(f.factoryDir)
	fe := pmProfileEntry(f.factoryDir, f.id)
	if fe < 0 {
		return nil, errUnknownLayout
	}
	f.size = int(f.factoryDir[fe+4])
	if f.factoryProfile, err = c.pmRead(pmFactoryDir|int(f.factoryDir[fe+9]), f.size); err != nil {
		return nil, err
	}
	if !validProfile(f.factoryProfile, f.size) {
		return nil, errUnknownLayout
	}
	if f.userDir, err = c.pmReadDir(pmUserDir); err != nil {
		return nil, err
	}
	if f.userDir == nil {
		// Only the single profile G515 TKL layout is known well enough to
		// create a user directory for; check that its profile slot is
		// empty too.
		if bytes.Equal(f.factoryDir[4:10], pmDirHeader) && len(f.factoryDir) == pmDirSize &&
			validDir(f.factoryDir, true) {
			if f.userProfile, err = c.pmRead(pmUserProfile, f.size); err != nil {
				return nil, err
			}
			if f.userProfile != nil {
				return nil, errors.New("user profile storage is half written, not touching it")
			}
			f.userAddr = pmUserProfile
		}
		return &f, nil
	}
	if !validDir(f.userDir, true) {
		return nil, errors.New("user profile directory is damaged, not touching it")
	}
	f.userEntry = pmProfileEntry(f.userDir, f.id)
	if f.userEntry < 0 || int(f.userDir[f.userEntry+4]) != f.size {
		return nil, fmt.Errorf("user profile directory has no %d byte profile %d, not touching it", f.size, f.id)
	}
	f.userAddr = int(f.userDir[f.userEntry+9])
	if f.userProfile, err = c.pmRead(f.userAddr, f.size); err != nil {
		return nil, err
	}
	if f.userProfile == nil {
		return nil, errors.New("user profile storage is half written, not touching it")
	}
	if !validProfile(f.userProfile, f.size) ||
		crc32.ChecksumIEEE(f.userProfile) != be32(f.userDir[f.userEntry+5:]) {
		return nil, errors.New("user profile does not match its directory entry, not touching it")
	}
	return &f, nil
}

// StoreProfile writes an effect, its dimmed variant (dimPercent of the
// colors and intensity) and the idle/sleep timeouts into the keyboard's
// active onboard profile and activates it. The firmware then shows the
// effect, dims it after idle seconds and turns it off after sleep seconds.
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
	if f.userAddr < 0 {
		return errors.New("this keyboard has no user profiles in its onboard memory yet " +
			"and logirgb cannot create them for this model; save a profile to the onboard memory once with G HUB")
	}

	base := f.userProfile
	if base == nil {
		base = f.factoryProfile
	}
	p := bytes.Clone(base)
	encodeTimer(p[2:4], idle)
	encodeTimer(p[4:6], sleep)
	clear(p[pmRecords : pmRecords+4*ledSize])
	record := func(slot int, eff Effect) {
		params := eff.encode(ze.Type)
		p[pmRecords+slot*ledSize] = byte(ze.Type.ID)
		copy(p[pmRecords+slot*ledSize+1:], params[:])
	}
	record(1, e)
	record(3, e.dimmed(ze.Type, dimPercent))
	p[pmFlags] = 0x00

	var dir []byte
	if f.userDir != nil {
		dir = pmDirWithFile(f.userDir, f.userEntry, p)
	} else {
		dir = bytes.Clone(f.factoryDir)
		dir[14] = pmUserProfile
		dir = pmDirWithFile(dir, 5, p)
	}

	// The profile goes first: until the directory is written the old
	// directory entry still describes the old profile.
	if !bytes.Equal(p, f.userProfile) {
		if err := c.pmSave(f.userAddr, p, crc32.ChecksumIEEE(p)); err != nil {
			return err
		}
	}
	if !bytes.Equal(dir, f.userDir) {
		if err := c.pmSave(pmUserDir, dir, be32(dir[0:4])); err != nil {
			return err
		}
	}
	return c.activateProfile(f.id, p)
}

// StoredProfile is the lighting a keyboard's onboard profile holds.
type StoredProfile struct {
	Effect     Effect
	DimPercent int // the idle effect relative to Effect, 0 if unknown
	Idle       int // seconds, 0 for never
	Sleep      int // seconds, 0 for never
}

// ReadProfile returns the lighting stored in the keyboard's onboard
// profile, the user profile if there is one, else the factory profile. ok
// is false when the stored effect is not one this zone supports (the factory
// profile uses effects 0x18 and 0x19).
func (c *Controller) ReadProfile() (sp StoredProfile, ok bool, err error) {
	if !c.HasProfileStorage() || len(c.zones) != 1 {
		return sp, false, errors.New("device has no profile storage")
	}
	f, err := c.pmSnapshot()
	if err != nil {
		return sp, false, err
	}
	p := f.userProfile
	if p == nil {
		p = f.factoryProfile
	}
	// Flag bit 0 selects record 0 instead of 1, bit 1 record 2 instead of 3
	// (see research/8101-profile-management.md).
	active, idle := 1, 3
	if p[pmFlags]&1 != 0 {
		active = 0
	}
	if p[pmFlags]&2 != 0 {
		idle = 2
	}
	rec := func(slot int) []byte { return p[pmRecords+slot*ledSize:][:ledSize] }
	ze, found := c.zoneEffect(0, rec(active)[0])
	if !found {
		return sp, false, nil
	}
	sp.Effect = decode(ze.Type, rec(active)[1:])
	sp.Idle, sp.Sleep = decodeTimer(p[2:4]), decodeTimer(p[4:6])
	if r := rec(idle); r[0] == rec(active)[0] {
		sp.DimPercent = dimRatio(ze.Type, sp.Effect, decode(ze.Type, r[1:]))
	}
	return sp, true, nil
}

// zoneEffect finds the zone's effect with the given effect ID.
func (c *Controller) zoneEffect(zone int, id byte) (ZoneEffect, bool) {
	for _, ze := range c.zones[zone].Effects {
		if ze.Type.ID == uint16(id) {
			return ze, true
		}
	}
	return ZoneEffect{}, false
}

// dimRatio estimates the percentage that turned e into the dimmed effect d,
// the inverse of dimmed, from the brightest color channel or the intensity.
func dimRatio(t EffectType, e, d Effect) int {
	for _, spec := range t.Params() {
		switch {
		case spec.Name.IsColor():
			a, b := maxChannel(e.Get(t, spec.Name)), maxChannel(d.Get(t, spec.Name))
			if a > 0 {
				return min(100, (b*100+a/2)/a)
			}
		case spec.Name == ParamIntensity:
			if a := e.Get(t, spec.Name); a > 0 {
				return min(100, d.Get(t, spec.Name)*100/a)
			}
		}
	}
	return 0
}

func maxChannel(c int) int {
	return max(c>>16&0xFF, c>>8&0xFF, c&0xFF)
}

// activateProfile hands the lighting back to the firmware and makes it
// load the stored user profile.
func (c *Controller) activateProfile(id byte, p []byte) error {
	if err := c.setSWControl(swRelease); err != nil {
		return err
	}
	if _, err := c.pmCall(6, 0x0B); err != nil {
		return fmt.Errorf("onboard mode: %w", err)
	}
	// [type 81 01, 01, file ID u16, length u16, crc32]; the field split is a
	// guess, verified with profile 1 on the G515 TKL and Rapid.
	cfg := []byte{0x81, 0x01, 0x01, 0x00, id, byte(len(p) >> 8), byte(len(p)), 0, 0, 0, 0}
	putBE32(cfg[7:], crc32.ChecksumIEEE(p))
	if _, err := c.pmCall(9, cfg...); err != nil {
		return fmt.Errorf("activate profile: %w", err)
	}
	m, err := c.pmCall(6, 0x00)
	if err != nil {
		return err
	}
	if m[0]&1 == 0 || m[1] != 0x00 || m[2] != id {
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
