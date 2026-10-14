// Package tui is the interactive front end: a device list on the left and
// the lighting settings of the selected device on the right. Changes are
// applied to the hardware immediately and saved to the config file.
package tui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mevdschee/logirgb/hidpp"
	"github.com/mevdschee/logirgb/internal/app"
	"github.com/mevdschee/logirgb/internal/state"
	"github.com/mevdschee/logirgb/lighting"
)

// Run starts the TUI.
func Run() error {
	m, err := newModel()
	if err != nil {
		return err
	}
	defer func() { app.Close(m.entries) }()
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type focus int

const (
	focusDevices focus = iota
	focusSettings
)

type rowKind int

const (
	rowProfile rowKind = iota
	rowZone
	rowEffect
	rowParam
	rowRate
	rowDPI
	rowStartDPI
	rowShiftDPI
	rowBrightness
	rowIdle
	rowSleep
	rowDim
)

type row struct {
	kind  rowKind
	label string
	param lighting.ParamSpec
	zone  int // zone for rowZone, rowEffect and rowParam
	level int // DPI level for rowDPI
}

// selectable reports whether the cursor can stop on the row; zone rows are
// headings.
func (r row) selectable() bool { return r.kind != rowZone }

// devState is what the TUI shows for one device.
type devState struct {
	// profile is the onboard profile being edited (1-based), 0 for devices
	// without onboard profiles.
	profile  int
	profiles int
	active   int
	zone     int
	// effects holds the zone effects per profile, index 0 for devices
	// without onboard profiles.
	effects [][]lighting.Effect
	// dpiRange and rates limit the profile settings (report rate, DPI).
	dpiRange   lighting.DPIRange
	rates      []int
	brightness int
	bmin, bmax int
	idle       int
	sleep      int
	dim        int
	claimed    bool
	// dev holds the settings as read from the device plus the edits;
	// dirty is set while edits have not been written to onboard memory.
	dev   *state.Device
	dirty bool
}

type model struct {
	entries []*app.Entry
	states  []*devState

	focus  focus
	dev    int
	row    int
	width  int
	height int
	// devOff and rowOff are the first visible lines of the device and
	// settings panes when they scroll.
	devOff int
	rowOff int

	// showHelp replaces the panes with the help screen, scrolled to helpOff.
	showHelp bool
	helpOff  int

	// confirmQuit asks before quitting with unwritten changes.
	confirmQuit bool

	editing bool
	input   textinput.Model
	status  string
	isError bool
}

var timeoutSteps = []int{0, 15, 30, 60, 120, 300, 600, 900, 1800, 3600}

var palette = []string{"white", "red", "orange", "yellow", "green", "cyan", "blue", "purple", "magenta", "pink"}

func newModel() (*model, error) {
	m := &model{}
	if err := m.load(); err != nil {
		return nil, err
	}
	m.input = textinput.New()
	m.input.CharLimit = 16
	m.input.Prompt = ""
	return m, nil
}

func (m *model) load() error {
	app.Close(m.entries)
	entries, err := app.Scan(hidpp.DefaultSoftwareID)
	if err != nil {
		return err
	}
	m.entries = entries
	m.states = make([]*devState, len(entries))
	for i, e := range entries {
		m.states[i] = m.initState(e)
	}
	if m.dev >= len(entries) {
		m.dev = 0
	}
	m.firstRow()
	return nil
}

func (m *model) initState(e *app.Entry) *devState {
	s := &devState{}
	if !e.Usable() {
		return s
	}
	saved, err := app.Load(e)
	if err != nil {
		m.setStatus(fmt.Errorf("%s: read onboard memory: %w", e.Name, err))
	}
	s.dev = saved
	s.claimed, _ = e.Ctl.Claimed()
	if e.Ctl.HasOnboardProfiles() {
		s.profiles, _ = e.Ctl.ProfileCount()
		s.active, _ = e.Ctl.ActiveProfile()
		s.profile = 1
		if s.active >= 1 && s.active <= s.profiles {
			s.profile = s.active
		}
	}
	if e.Ctl.HasProfileSettings() {
		s.dpiRange = e.Ctl.DPIRange()
		s.rates = e.Ctl.ReportRates()
		slices.Sort(s.rates)
	}
	for p := 0; p <= s.profiles; p++ {
		var effects []lighting.Effect
		for i, z := range e.Ctl.Zones() {
			eff, ok := saved.ProfileZone(p, i)
			if !ok {
				eff = lighting.Effect{Name: "static"}
				if _, has := z.Find("static"); !has && len(z.Effects) > 0 {
					eff.Name = z.Effects[0].Type.Name
				}
			}
			effects = append(effects, eff)
		}
		s.effects = append(s.effects, effects)
	}
	if e.Ctl.HasBrightness() {
		s.bmin, s.bmax, _ = e.Ctl.BrightnessRange()
		s.brightness, _ = e.Ctl.Brightness()
	}
	if e.Ctl.HasTimers() {
		if saved.IdleTimeout != nil {
			s.idle = *saved.IdleTimeout
		}
		if saved.SleepTimeout != nil {
			s.sleep = *saved.SleepTimeout
		}
		s.dim = app.DefaultDimPercent
		if saved.DimPercent != nil {
			s.dim = *saved.DimPercent
		}
	}
	return s
}

// settings returns the report rate and DPI settings of the selected
// profile, or nil when there are none to edit.
func (m *model) settings() *lighting.ProfileSettings {
	e, s := m.current()
	if s.profile == 0 || !e.Ctl.HasProfileSettings() {
		return nil
	}
	return s.dev.Settings(s.profile)
}

// effect returns the effect being edited: the selected zone in the selected
// profile.
func (s *devState) effect() *lighting.Effect {
	return &s.effects[s.profile][s.zone]
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) current() (*app.Entry, *devState) {
	if len(m.entries) == 0 {
		return nil, nil
	}
	return m.entries[m.dev], m.states[m.dev]
}

func (m *model) rows() []row {
	e, s := m.current()
	if e == nil || !e.Usable() {
		return nil
	}
	var rows []row
	zones := e.Ctl.Zones()
	if s.profiles > 0 {
		rows = append(rows, row{kind: rowProfile, label: "Profile"})
	}
	for zi, z := range zones {
		if len(zones) > 1 {
			rows = append(rows, row{kind: rowZone, zone: zi})
		}
		rows = append(rows, row{kind: rowEffect, label: "Effect", zone: zi})
		if ze, ok := z.Find(s.effects[s.profile][zi].Name); ok {
			for _, p := range ze.Type.Params() {
				rows = append(rows, row{kind: rowParam, label: p.Label, param: p, zone: zi})
			}
		}
	}
	if ps := m.settings(); ps != nil {
		rows = append(rows, row{kind: rowRate, label: "Report rate"})
		for i := range ps.DPI {
			rows = append(rows, row{kind: rowDPI, label: fmt.Sprintf("DPI %d", i+1), level: i})
		}
		rows = append(rows,
			row{kind: rowStartDPI, label: "DPI at start"},
			row{kind: rowShiftDPI, label: "DPI shift"},
		)
	}
	if e.Ctl.HasBrightness() {
		rows = append(rows, row{kind: rowBrightness, label: "Brightness"})
	}
	if e.Ctl.HasTimers() {
		rows = append(rows,
			row{kind: rowIdle, label: "Dim after"},
			row{kind: rowDim, label: "Dim level"},
			row{kind: rowSleep, label: "Off after"},
		)
	}
	return rows
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.confirmQuit {
			return m.updateConfirmQuit(msg)
		}
		if m.showHelp {
			return m.updateHelp(msg)
		}
		if m.editing {
			return m.updateEditing(msg)
		}
		return m.updateKey(msg)
	}
	return m, nil
}

func (m *model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q", "esc":
		if len(m.unwritten()) > 0 {
			m.confirmQuit = true
			return m, nil
		}
		return m, tea.Quit
	case "?":
		m.showHelp, m.helpOff = true, 0
		return m, nil
	case "tab", "shift+tab":
		if m.focus == focusDevices && len(m.rows()) > 0 {
			m.focus = focusSettings
		} else {
			m.focus = focusDevices
		}
		return m, nil
	case "ctrl+r", "f5":
		if err := m.load(); err != nil {
			m.setStatus(err)
		} else {
			m.setStatus(nil, "rescanned devices")
		}
		return m, nil
	case "o":
		if e, s := m.current(); e != nil && e.Usable() {
			if err := e.Ctl.Release(); err != nil {
				m.setStatus(err)
				return m, nil
			}
			// Drop the edits and show what the device stores again.
			ns := m.initState(e)
			ns.profile = s.profile
			m.states[m.dev] = ns
			m.setStatus(nil, "back to the stored lighting; unwritten changes dropped")
		}
		return m, nil
	case "w":
		if e, _ := m.current(); e != nil && e.Usable() {
			if !writable(e) {
				m.setStatus(errors.New("this device has no onboard memory; changes are live only"))
				return m, nil
			}
			m.write(m.dev)
		}
		return m, nil
	}

	if m.focus == focusDevices {
		switch msg.String() {
		case "up", "k":
			if m.dev > 0 {
				m.dev--
				m.firstRow()
			}
		case "down", "j":
			if m.dev < len(m.entries)-1 {
				m.dev++
				m.firstRow()
			}
		case "enter", "right", "l":
			if len(m.rows()) > 0 {
				m.focus = focusSettings
			}
		}
		return m, nil
	}

	rows := m.rows()
	switch msg.String() {
	case "up", "k":
		m.moveRow(-1)
	case "down", "j":
		m.moveRow(1)
	case "left", "h":
		m.adjust(rows[m.row], -1, false)
	case "right", "l":
		m.adjust(rows[m.row], 1, false)
	case "shift+left", "H", "pgdown":
		m.adjust(rows[m.row], -1, true)
	case "shift+right", "L", "pgup":
		m.adjust(rows[m.row], 1, true)
	case "enter", "e":
		r := rows[m.row]
		if r.kind == rowParam && r.param.Name.IsColor() {
			_, s := m.current()
			s.zone = r.zone
			eff := s.effect()
			ze, _ := m.entries[m.dev].Ctl.Zones()[s.zone].Find(eff.Name)
			m.input.SetValue(lighting.FormatColor(eff.Get(ze.Type, r.param.Name)))
			m.input.CursorEnd()
			m.input.Focus()
			m.editing = true
		}
	}
	return m, nil
}

// writable reports whether the device has onboard memory to write to.
func writable(e *app.Entry) bool {
	return e.Ctl.HasOnboardProfiles() || e.Ctl.HasProfileStorage()
}

// unwritten returns the indexes of the devices with unwritten changes.
func (m *model) unwritten() []int {
	var out []int
	for i, s := range m.states {
		if s.dirty {
			out = append(out, i)
		}
	}
	return out
}

// write stores the settings of device i in its onboard memory.
func (m *model) write(i int) error {
	e, s := m.entries[i], m.states[i]
	n, dir, err := app.SaveToProfiles(e, s.dev)
	if err != nil {
		m.setStatus(fmt.Errorf("%s: %w", e.Name, err))
		return err
	}
	s.dirty = false
	if e.Ctl.HasProfileStorage() {
		s.claimed = false
		m.setStatus(nil, "stored in the keyboard; it now dims and turns off by itself (backup: "+dir+")")
	} else {
		m.setStatus(nil, fmt.Sprintf("stored in %d onboard profiles (backup: %s)", n, dir))
	}
	return nil
}

func (m *model) updateConfirmQuit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "y":
		return m, tea.Quit
	case "w":
		for _, i := range m.unwritten() {
			if m.write(i) != nil {
				m.confirmQuit = false
				return m, nil
			}
		}
		return m, tea.Quit
	}
	m.confirmQuit = false
	return m, nil
}

func (m *model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.helpOff--
	case "down", "j":
		m.helpOff++
	case "pgup":
		m.helpOff -= 10
	case "pgdown", " ":
		m.helpOff += 10
	default:
		m.showHelp = false
	}
	m.helpOff = max(m.helpOff, 0)
	return m, nil
}

// helpSections is the content of the help screen.
var helpSections = []struct {
	title string
	keys  [][2]string
}{
	{"Moving around", [][2]string{
		{"tab", "switch between the device list and the settings"},
		{"↑ ↓  k j", "select a device or a setting"},
		{"← →  h l", "change the selected setting"},
		{"shift+← →  H L", "change it in bigger steps (also pgup, pgdown)"},
		{"enter  e", "type a color: #rrggbb, r,g,b or a name like orange"},
	}},
	{"Device", [][2]string{
		{"w", "write the settings to the device's onboard memory, after a backup"},
		{"o", "drop unwritten changes and show the stored lighting again"},
		{"ctrl+r  F5", "rescan the devices"},
		{"?", "show this help; any key closes it"},
		{"q  esc", "quit; asks first when changes are not written"},
	}},
	{"Good to know", [][2]string{
		{"lighting", "shows right away, but a profile switch or power cycle undoes it until you press w"},
		{"DPI, rate", "only change on the mouse when you press w"},
		{"profiles", "start on the profile the mouse is running, marked active"},
		{"settings", "are read from the device; edits are kept until you write them with w"},
		{"*", "marks a device with changes that are not written yet"},
		{"backups", "w first saves the onboard memory in ~/.local/share/logirgb/backups"},
	}},
}

// viewHelpScreen renders the help screen for a pane with the given inner
// width and height.
func (m *model) viewHelpScreen(width, height int) string {
	keyWidth := 0
	for _, sec := range helpSections {
		for _, k := range sec.keys {
			keyWidth = max(keyWidth, lipgloss.Width(k[0]))
		}
	}
	var lines []string
	for i, sec := range helpSections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, titleStyle.Render(sec.title))
		for _, k := range sec.keys {
			// Wrap the description in its own column next to the key.
			desc := lipgloss.NewStyle().Width(max(width-keyWidth-4, 10)).Render(k[1])
			key := selStyle.Render(fmt.Sprintf("  %-*s  ", keyWidth, k[0]))
			lines = append(lines, strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, key, desc), "\n")...)
		}
	}
	if height <= 0 || len(lines) <= height {
		m.helpOff = 0
		return strings.Join(lines, "\n")
	}
	m.helpOff = min(m.helpOff, len(lines)-height)
	out := slices.Clone(lines[m.helpOff : m.helpOff+height])
	if m.helpOff > 0 {
		out[0] = dimStyle.Render("  ↑ more")
	}
	if m.helpOff+height < len(lines) {
		out[height-1] = dimStyle.Render("  ↓ more")
	}
	return strings.Join(out, "\n")
}

func (m *model) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = false
		m.input.Blur()
		return m, nil
	case "enter":
		v, err := lighting.ParseColor(m.input.Value())
		if err != nil {
			m.setStatus(err)
			return m, nil
		}
		m.editing = false
		m.input.Blur()
		r := m.rows()[m.row]
		_, s := m.current()
		eff := s.effect()
		eff.Set(r.param.Name, v)
		m.applyEffect()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// moveRow moves the cursor to the next selectable row in direction dir.
func (m *model) moveRow(dir int) {
	rows := m.rows()
	for i := m.row + dir; i >= 0 && i < len(rows); i += dir {
		if rows[i].selectable() {
			m.row = i
			return
		}
	}
}

// firstRow puts the cursor on the first selectable row.
func (m *model) firstRow() {
	m.row = 0
	if rows := m.rows(); len(rows) > 0 && !rows[0].selectable() {
		m.moveRow(1)
	}
}

func clamp(v, lo, hi int) int {
	return max(lo, min(hi, v))
}

func stepThrough(steps []int, cur, dir int) int {
	if dir > 0 {
		for _, s := range steps {
			if s > cur {
				return s
			}
		}
		return steps[len(steps)-1]
	}
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i] < cur {
			return steps[i]
		}
	}
	return steps[0]
}

func (m *model) adjust(r row, dir int, big bool) {
	e, s := m.current()
	zones := e.Ctl.Zones()
	if r.kind == rowEffect || r.kind == rowParam {
		s.zone = r.zone
	}
	switch r.kind {
	case rowProfile:
		s.profile = (s.profile-1+dir+s.profiles)%s.profiles + 1
		m.row = min(m.row, len(m.rows())-1)
		m.previewProfile()
	case rowEffect:
		z := zones[s.zone]
		idx := 0
		for i, ze := range z.Effects {
			if ze.Type.Name == s.effect().Name {
				idx = i
			}
		}
		idx = (idx + dir + len(z.Effects)) % len(z.Effects)
		// Keep colors and other values when switching effects.
		s.effect().Name = z.Effects[idx].Type.Name
		m.applyEffect()
	case rowParam:
		eff := s.effect()
		ze, _ := zones[s.zone].Find(eff.Name)
		cur := eff.Get(ze.Type, r.param.Name)
		var v int
		switch {
		case r.param.Name.IsColor():
			v = cyclePalette(cur, dir)
		case r.param.Choices != nil:
			n := r.param.Max - r.param.Min + 1
			v = r.param.Min + (cur-r.param.Min+dir+n)%n
		default:
			step := max(r.param.Step, 1)
			if big {
				step *= 10
			}
			v = clamp(cur+dir*step, r.param.Min, r.param.Max)
		}
		eff.Set(r.param.Name, v)
		m.applyEffect()
	case rowRate, rowDPI, rowStartDPI, rowShiftDPI:
		m.adjustSettings(r, dir, big)
	case rowBrightness:
		step := max((s.bmax-s.bmin)/20, 1)
		if big {
			step *= 4
		}
		s.brightness = clamp(s.brightness+dir*step, s.bmin, s.bmax)
		s.dev.Brightness = state.IntPtr(s.brightness)
		m.setStatus(e.Ctl.SetBrightness(s.brightness), fmt.Sprintf("brightness %d", s.brightness))
	case rowIdle, rowSleep, rowDim:
		switch r.kind {
		case rowIdle:
			s.idle = stepThrough(timeoutSteps, s.idle, dir)
		case rowSleep:
			s.sleep = stepThrough(timeoutSteps, s.sleep, dir)
		case rowDim:
			s.dim = clamp(s.dim+dir*5, 0, 100)
		}
		d := s.dev
		d.IdleTimeout = state.IntPtr(s.idle)
		d.SleepTimeout = state.IntPtr(s.sleep)
		d.DimPercent = state.IntPtr(s.dim)
		m.changed()
		m.setStatus(app.ApplyTimers(e, d), "press w to store it in the keyboard")
	}
}

// adjustSettings changes the report rate or a DPI setting of the selected
// profile. These are not previewed; w writes them to the device.
func (m *model) adjustSettings(r row, dir int, big bool) {
	_, s := m.current()
	ps := m.settings()
	switch r.kind {
	case rowRate:
		i := max(slices.Index(s.rates, ps.ReportRate), 0)
		ps.ReportRate = s.rates[clamp(i+dir, 0, len(s.rates)-1)]
	case rowDPI:
		rng := s.dpiRange
		v := ps.DPI[r.level]
		step := rng.Step
		if big {
			step *= 10
		}
		switch {
		case v == 0 && dir > 0:
			v = rng.Min
		case v == 0:
		case v+dir*step < rng.Min:
			// Turn the level off, unless the profile needs it.
			if r.level != ps.DefaultDPI && r.level != ps.ShiftDPI && ps.Levels() > 1 {
				v = 0
			} else {
				v = rng.Min
			}
		default:
			v = clamp(v+dir*step, rng.Min, rng.Max)
		}
		ps.DPI[r.level] = v
	case rowStartDPI, rowShiftDPI:
		idx := &ps.DefaultDPI
		if r.kind == rowShiftDPI {
			idx = &ps.ShiftDPI
		}
		for i := *idx + dir; i >= 0 && i < len(ps.DPI); i += dir {
			if ps.DPI[i] > 0 {
				*idx = i
				break
			}
		}
	}
	m.changed()
	m.setStatus(nil, fmt.Sprintf("profile %d %s changed; press w to store it in the mouse", s.profile, strings.ToLower(r.label)))
}

// levelDPI formats the DPI level i of the profile settings.
func levelDPI(ps *lighting.ProfileSettings, i int) string {
	if i < 0 || i >= len(ps.DPI) {
		return "?"
	}
	return fmt.Sprintf("level %d (%d DPI)", i+1, ps.DPI[i])
}

func cyclePalette(cur, dir int) int {
	idx := -1
	for i, name := range palette {
		if v, _ := lighting.ParseColor(name); v == cur {
			idx = i
		}
	}
	if idx < 0 && dir < 0 {
		idx = 0
	}
	idx = (idx + dir + len(palette)) % len(palette)
	v, _ := lighting.ParseColor(palette[idx])
	return v
}

func (m *model) applyEffect() {
	e, s := m.current()
	eff := *s.effect()
	err := e.Ctl.Claim()
	if err == nil {
		err = e.Ctl.SetEffect(s.zone, eff)
	}
	if err == nil {
		s.claimed = true
		if s.profile == 0 {
			s.dev.SetZone(s.zone, eff)
		} else {
			s.dev.SetProfileZone(s.profile, s.zone, eff)
		}
		m.changed()
	}
	where := e.Ctl.Zones()[s.zone].Location
	if s.profile > 0 {
		where = fmt.Sprintf("profile %d %s", s.profile, where)
	}
	m.setStatus(err, fmt.Sprintf("%s: %s", where, eff.Name))
}

// previewProfile shows the lighting of the selected profile on the device.
func (m *model) previewProfile() {
	e, s := m.current()
	if err := e.Ctl.Claim(); err != nil {
		m.setStatus(err)
		return
	}
	for zone, eff := range s.effects[s.profile] {
		if err := e.Ctl.SetEffect(zone, eff); err != nil {
			m.setStatus(err)
			return
		}
	}
	s.claimed = true
	m.setStatus(nil, fmt.Sprintf("editing profile %d", s.profile))
}

// changed marks the selected device as having unwritten changes, when it
// has onboard memory to write them to.
func (m *model) changed() {
	if e, s := m.current(); writable(e) {
		s.dirty = true
	}
}

func (m *model) setStatus(err error, ok ...string) {
	if err != nil {
		m.status, m.isError = err.Error(), true
		return
	}
	m.status, m.isError = strings.Join(ok, " "), false
}

// Styles.
var (
	accent     = lipgloss.AdaptiveColor{Light: "#0057d9", Dark: "#3d8bfd"}
	dim        = lipgloss.AdaptiveColor{Light: "#7a7a7a", Dark: "#8a8a8a"}
	errColor   = lipgloss.AdaptiveColor{Light: "#c0362c", Dark: "#ff6b6b"}
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	paneStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dim).Padding(0, 1)
	focusPane  = paneStyle.BorderForeground(accent)
	selStyle   = lipgloss.NewStyle().Bold(true).Foreground(accent)
	dimStyle   = lipgloss.NewStyle().Foreground(dim)
	errStyle   = lipgloss.NewStyle().Foreground(errColor)
	warnBox    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(errColor).Padding(0, 1)
)

// Pane layout: the device pane takes leftPercent of the terminal width and
// the settings pane the rest; both fill the height left by the header and
// footer.
const (
	leftPercent   = 30
	minLeftWidth  = 24
	minRightWidth = 44
	defaultWidth  = 100
)

// paneWidths returns the outer widths (with borders) of the two panes.
func (m *model) paneWidths() (int, int) {
	w := m.width
	if w <= 0 {
		w = defaultWidth
	}
	left := max(w*leftPercent/100, minLeftWidth)
	return left, max(w-left, minRightWidth)
}

func (m *model) View() string {
	width := m.width
	if width <= 0 {
		width = defaultWidth
	}
	var top strings.Builder
	top.WriteString(titleStyle.Render("logirgb") + dimStyle.Render("  Logitech RGB control"))
	if denied := app.PermissionProblems(m.entries); len(denied) > 0 {
		top.WriteString("\n" + warnBox.Width(width-2).Render(errStyle.Render("Permission problem")+"\n"+strings.TrimRight(app.PermissionHelp(denied), "\n")))
	}

	var bottom strings.Builder
	if m.confirmQuit {
		var names []string
		for _, i := range m.unwritten() {
			names = append(names, m.entries[i].Name)
		}
		bottom.WriteString(errStyle.Width(width).Render("not written to "+strings.Join(names, ", ")+"; quit anyway?") + "\n")
	} else if m.status != "" {
		if m.isError {
			bottom.WriteString(errStyle.Width(width).Render("✗ "+m.status) + "\n")
		} else {
			bottom.WriteString(lipgloss.NewStyle().Width(width).Render(selStyle.Render("✓ ")+m.status) + "\n")
		}
	}
	bottom.WriteString(m.viewHelp(width))

	lw, rw := m.paneWidths()
	lp, rp := paneStyle, paneStyle
	if m.focus == focusDevices {
		lp = focusPane
	} else {
		rp = focusPane
	}
	// Width and Height include the padding but not the border.
	lp, rp = lp.Width(lw-2), rp.Width(rw-2)
	if m.height > 0 {
		h := m.height - lipgloss.Height(top.String()) - lipgloss.Height(bottom.String()) - 2
		lp, rp = lp.Height(max(h, 0)), rp.Height(max(h, 0))
	}
	if m.showHelp {
		hp := focusPane.Width(lw + rw - 2)
		h := -1
		if m.height > 0 {
			h = m.height - lipgloss.Height(top.String()) - lipgloss.Height(bottom.String()) - 2
			hp = hp.Height(max(h, 0))
		}
		return top.String() + "\n" + hp.Render(m.viewHelpScreen(lw+rw-4, h)) + "\n" + bottom.String()
	}
	left, devSel := m.viewDevices(lw - 4)
	right, rowSel := m.viewSettings(rw - 4)
	if m.height > 0 {
		h := m.height - lipgloss.Height(top.String()) - lipgloss.Height(bottom.String()) - 2
		left = scroll(left, lw-4, devSel, h, &m.devOff)
		right = scroll(right, rw-4, rowSel, h, &m.rowOff)
	}
	panes := lipgloss.JoinHorizontal(lipgloss.Top, lp.Render(left), rp.Render(right))
	return top.String() + "\n" + panes + "\n" + bottom.String()
}

// scroll wraps content to width and, when it is taller than height, shows
// the window of height lines starting at *off, moved as little as needed to
// keep line sel visible with a line of context around it. Hidden lines
// above and below are marked.
func scroll(content string, width, sel, height int, off *int) string {
	lines := strings.Split(lipgloss.NewStyle().Width(width).Render(content), "\n")
	if height < 3 || len(lines) <= height {
		*off = 0
		return strings.Join(lines, "\n")
	}
	sel = clamp(sel, 0, len(lines)-1)
	// The first and last window lines hold the markers when there is more
	// above or below, so sel has to stay between them.
	if first := *off + min(*off, 1); sel <= first {
		*off = max(sel-2, 0)
	}
	if last := *off + height - 2; *off+height < len(lines) && sel >= last {
		*off = sel - height + 3
	}
	*off = clamp(*off, 0, len(lines)-height)
	out := slices.Clone(lines[*off : *off+height])
	if *off > 0 {
		out[0] = dimStyle.Render("  ↑ more")
	}
	if *off+height < len(lines) {
		out[height-1] = dimStyle.Render("  ↓ more")
	}
	return strings.Join(out, "\n")
}

// linesIn returns how many lines s takes when wrapped to width.
func linesIn(s string, width int) int {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return 0
	}
	return lipgloss.Height(lipgloss.NewStyle().Width(width).Render(s))
}

// helpItems are the key hints of the help line; when the line does not fit,
// the items in dropOrder are left out, in that order.
var (
	helpItems = [][2]string{
		{"tab", "pane"}, {"↑↓", "move"}, {"←→", "set"}, {"e", "color"}, {"w", "write"},
		{"o", "undo"}, {"^r", "scan"}, {"?", "help"}, {"q", "quit"},
	}
	editHelpItems = [][2]string{
		{"#rrggbb", "r,g,b or name"}, {"enter", "confirm"}, {"esc", "cancel"},
	}
	dropOrder = []string{"tab", "o", "^r"}
)

// viewHelp renders the key hints on a single line of at most width columns.
func (m *model) viewHelp(width int) string {
	items := helpItems
	switch {
	case m.editing:
		items = editHelpItems
	case m.confirmQuit:
		items = [][2]string{{"w", "write and quit"}, {"q", "quit without writing"}, {"esc", "cancel"}}
	case m.showHelp:
		items = [][2]string{{"↑↓", "scroll"}, {"any key", "close"}}
	}
	render := func(items [][2]string) (string, int) {
		var parts, plain []string
		for _, it := range items {
			parts = append(parts, selStyle.Render(it[0])+" "+dimStyle.Render(it[1]))
			plain = append(plain, it[0]+" "+it[1])
		}
		return strings.Join(parts, "  "), lipgloss.Width(strings.Join(plain, "  "))
	}
	line, w := render(items)
	for i := 0; w > width && i < len(dropOrder); i++ {
		items = slices.DeleteFunc(slices.Clone(items), func(it [2]string) bool { return it[0] == dropOrder[i] })
		line, w = render(items)
	}
	if w > width {
		var raw []string
		for _, it := range items {
			raw = append(raw, it[0]+" "+it[1])
		}
		return dimStyle.Render(truncate(strings.Join(raw, "  "), width))
	}
	return line
}

// truncate shortens s to at most n runes, ending in an ellipsis when cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:max(n, 0)])
	}
	return string(r[:n-1]) + "…"
}

// viewDevices renders the device list for a pane with the given inner
// width, and the line of the selected device.
func (m *model) viewDevices(width int) (string, int) {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Devices") + "\n")
	if len(m.entries) == 0 {
		b.WriteString(dimStyle.Render("no Logitech devices found\nctrl+r to rescan"))
		return b.String(), 0
	}
	sel := 0
	for i, e := range m.entries {
		mark, style := "●", lipgloss.NewStyle()
		if !e.Usable() {
			mark, style = "✗", errStyle
		}
		var line string
		if m.states[i].dirty {
			line = fmt.Sprintf("%s %s %s", style.Render(mark), truncate(e.Name, width-6), errStyle.Render("*"))
		} else {
			line = fmt.Sprintf("%s %s", style.Render(mark), truncate(e.Name, width-4))
		}
		if i == m.dev {
			// Keep the path line below the name in view too.
			sel = linesIn(b.String(), width) + 1
			line = selStyle.Render("› ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
		b.WriteString(dimStyle.Render(truncate("    "+e.Info.Path, width)) + "\n")
	}
	return strings.TrimRight(b.String(), "\n"), sel
}

// viewSettings renders the settings of the selected device for a pane with
// the given inner width, and the line of the selected row.
func (m *model) viewSettings(width int) (string, int) {
	e, s := m.current()
	if e == nil {
		return dimStyle.Render("nothing selected"), 0
	}
	sel := 0
	var b strings.Builder
	b.WriteString(titleStyle.Render(e.Name) + dimStyle.Render("  "+e.Key) + "\n")
	if !e.Usable() {
		if e.Err == app.ErrPermission {
			b.WriteString(errStyle.Render("No access to "+e.Info.Path) + "\n" + dimStyle.Render("See the fix above, then press ctrl+r."))
		} else {
			b.WriteString(errStyle.Render(e.Err.Error()))
		}
		return b.String(), 0
	}
	switch {
	case !s.claimed:
		b.WriteString(dimStyle.Render("showing the onboard lighting") + "\n")
	case e.Ctl.HasProfileStorage():
		b.WriteString(errStyle.Render("live preview, not stored: ") + dimStyle.Render("press w to store it") + "\n")
	}
	b.WriteString("\n")

	zones := e.Ctl.Zones()
	for i, r := range m.rows() {
		eff := s.effects[s.profile][r.zone]
		ze, _ := zones[r.zone].Find(eff.Name)
		var val string
		switch r.kind {
		case rowProfile:
			val = fmt.Sprintf("%d of %d", s.profile, s.profiles)
			if s.active > 0 && s.profile == s.active {
				val += "  " + dimStyle.Render("active")
			}
		case rowZone:
			z := zones[r.zone]
			heading := fmt.Sprintf("Zone %d · %s", r.zone, z.Location)
			if z.Description != "" {
				heading += "  " + dimStyle.Render(z.Description)
			}
			b.WriteString("\n" + lipgloss.NewStyle().PaddingLeft(2).Width(width).Render(titleStyle.Render(heading)) + "\n")
			continue
		case rowEffect:
			val = eff.Name
		case rowParam:
			v := eff.Get(ze.Type, r.param.Name)
			val = r.param.Format(v)
			if r.param.Name.IsColor() {
				if m.editing && i == m.row {
					val = m.input.View()
				}
				val += "  " + lipgloss.NewStyle().Background(lipgloss.Color(lighting.FormatColor(v))).Render("      ")
			}
		case rowRate:
			val = fmt.Sprintf("%d Hz", m.settings().ReportRate)
		case rowDPI:
			val = "off"
			if v := m.settings().DPI[r.level]; v > 0 {
				val = strconv.Itoa(v)
			}
		case rowStartDPI:
			val = levelDPI(m.settings(), m.settings().DefaultDPI)
		case rowShiftDPI:
			val = levelDPI(m.settings(), m.settings().ShiftDPI)
		case rowBrightness:
			val = fmt.Sprintf("%d  %s", s.brightness, bar(s.brightness-s.bmin, s.bmax-s.bmin))
		case rowIdle:
			val = seconds(s.idle)
		case rowSleep:
			val = seconds(s.sleep)
		case rowDim:
			val = fmt.Sprintf("%d%%  %s", s.dim, bar(s.dim, 100))
		}
		label := fmt.Sprintf("%-15s", r.label)
		if r.kind == rowBrightness || r.kind == rowIdle || r.kind == rowRate {
			b.WriteString("\n")
		}
		if i == m.row {
			sel = linesIn(b.String(), width)
		}
		if i == m.row && m.focus == focusSettings {
			b.WriteString(selStyle.Render("› "+label) + " " + selStyle.Render("‹ ") + val + selStyle.Render(" ›") + "\n")
		} else {
			b.WriteString("  " + label + "   " + val + "\n")
		}
	}
	if e.Ctl.HasOnboardProfiles() {
		b.WriteString("\n" + dimStyle.Render(truncate("not stored until w; lost on a profile switch", width)))
	}
	if e.Ctl.HasProfileStorage() {
		b.WriteString("\n" + dimStyle.Render(truncate("only stored lighting dims and turns off; press w", width)))
	}
	return strings.TrimRight(b.String(), "\n"), sel
}

func bar(v, total int) string {
	const width = 20
	if total <= 0 {
		return ""
	}
	n := clamp(v*width/total, 0, width)
	return selStyle.Render(strings.Repeat("█", n)) + dimStyle.Render(strings.Repeat("░", width-n))
}

func seconds(s int) string {
	switch {
	case s == 0:
		return "never"
	case s%60 == 0:
		return strconv.Itoa(s/60) + " min"
	}
	return strconv.Itoa(s) + " s"
}
