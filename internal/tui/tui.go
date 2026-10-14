// Package tui is the interactive front end: a device list on the left and
// the lighting settings of the selected device on the right. Changes are
// applied to the hardware immediately and saved to the config file.
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mevdschee/logirgb/hidpp"
	"github.com/mevdschee/logirgb/internal/app"
	"github.com/mevdschee/logirgb/internal/config"
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
	rowZone rowKind = iota
	rowEffect
	rowParam
	rowBrightness
	rowIdle
	rowSleep
	rowDim
)

type row struct {
	kind  rowKind
	label string
	param lighting.ParamSpec
}

// devState is what the TUI shows for one device.
type devState struct {
	zone       int
	effects    []lighting.Effect // per zone
	brightness int
	bmin, bmax int
	idle       int
	sleep      int
	dim        int
	claimed    bool
}

type model struct {
	entries []*app.Entry
	states  []*devState
	cfg     *config.Config

	focus  focus
	dev    int
	row    int
	width  int
	height int

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
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	m.entries, m.cfg = entries, cfg
	m.states = make([]*devState, len(entries))
	for i, e := range entries {
		m.states[i] = m.initState(e)
	}
	if m.dev >= len(entries) {
		m.dev = 0
	}
	m.row = 0
	return nil
}

func (m *model) initState(e *app.Entry) *devState {
	s := &devState{}
	if !e.Usable() {
		return s
	}
	saved := m.cfg.Device(e.Key)
	s.claimed, _ = e.Ctl.Claimed()
	for i, z := range e.Ctl.Zones() {
		eff, ok := saved.Zone(i)
		if !ok {
			eff = lighting.Effect{Name: "static"}
			if _, has := z.Find("static"); !has && len(z.Effects) > 0 {
				eff.Name = z.Effects[0].Type.Name
			}
		}
		s.effects = append(s.effects, eff)
	}
	if e.Ctl.HasBrightness() {
		s.bmin, s.bmax, _ = e.Ctl.BrightnessRange()
		s.brightness, _ = e.Ctl.Brightness()
	}
	if e.Ctl.HasTimers() {
		s.idle, s.sleep, _ = e.Ctl.Timers()
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
	if len(zones) > 1 {
		rows = append(rows, row{kind: rowZone, label: "Zone"})
	}
	rows = append(rows, row{kind: rowEffect, label: "Effect"})
	if ze, ok := zones[s.zone].Find(s.effects[s.zone].Name); ok {
		for _, p := range ze.Type.Params() {
			rows = append(rows, row{kind: rowParam, label: p.Label, param: p})
		}
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
		if m.editing {
			return m.updateEditing(msg)
		}
		return m.updateKey(msg)
	}
	return m, nil
}

func (m *model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
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
			err := e.Ctl.Release()
			s.claimed = err != nil
			m.setStatus(err, "lighting returned to the onboard profile")
		}
		return m, nil
	case "w":
		if e, s := m.current(); e != nil && e.Usable() && (e.Ctl.HasOnboardProfiles() || e.Ctl.HasProfileStorage()) {
			m.setStatus(nil, "writing onboard memory...")
			n, dir, err := app.SaveToProfiles(e, m.cfg.Device(e.Key))
			if e.Ctl.HasProfileStorage() {
				s.claimed = err != nil && s.claimed
				m.setStatus(err, "stored in the keyboard; it now dims and turns off by itself (backup: "+dir+")")
			} else {
				m.setStatus(err, fmt.Sprintf("stored in %d onboard profiles (backup: %s)", n, dir))
			}
		}
		return m, nil
	case "a":
		if e, s := m.current(); e != nil && e.Usable() {
			err := app.Apply(e, m.cfg.Device(e.Key))
			s.claimed = err == nil
			m.setStatus(err, "applied saved settings")
		}
		return m, nil
	}

	if m.focus == focusDevices {
		switch msg.String() {
		case "up", "k":
			if m.dev > 0 {
				m.dev--
				m.row = 0
			}
		case "down", "j":
			if m.dev < len(m.entries)-1 {
				m.dev++
				m.row = 0
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
		if m.row > 0 {
			m.row--
		}
	case "down", "j":
		if m.row < len(rows)-1 {
			m.row++
		}
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
			eff := s.effects[s.zone]
			ze, _ := m.entries[m.dev].Ctl.Zones()[s.zone].Find(eff.Name)
			m.input.SetValue(lighting.FormatColor(eff.Get(ze.Type, r.param.Name)))
			m.input.CursorEnd()
			m.input.Focus()
			m.editing = true
		}
	}
	return m, nil
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
		eff := &s.effects[s.zone]
		eff.Set(r.param.Name, v)
		m.applyEffect()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
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
	switch r.kind {
	case rowZone:
		s.zone = (s.zone + dir + len(zones)) % len(zones)
		m.row = min(m.row, len(m.rows())-1)
	case rowEffect:
		z := zones[s.zone]
		idx := 0
		for i, ze := range z.Effects {
			if ze.Type.Name == s.effects[s.zone].Name {
				idx = i
			}
		}
		idx = (idx + dir + len(z.Effects)) % len(z.Effects)
		// Keep colors and other values when switching effects.
		s.effects[s.zone].Name = z.Effects[idx].Type.Name
		m.applyEffect()
	case rowParam:
		eff := &s.effects[s.zone]
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
	case rowBrightness:
		step := max((s.bmax-s.bmin)/20, 1)
		if big {
			step *= 4
		}
		s.brightness = clamp(s.brightness+dir*step, s.bmin, s.bmax)
		d := m.cfg.Device(e.Key)
		d.Name = e.Name
		d.Brightness = config.IntPtr(s.brightness)
		m.setStatus(e.Ctl.SetBrightness(s.brightness), fmt.Sprintf("brightness %d", s.brightness))
		m.save()
	case rowIdle, rowSleep, rowDim:
		switch r.kind {
		case rowIdle:
			s.idle = stepThrough(timeoutSteps, s.idle, dir)
		case rowSleep:
			s.sleep = stepThrough(timeoutSteps, s.sleep, dir)
		case rowDim:
			s.dim = clamp(s.dim+dir*5, 0, 100)
		}
		d := m.cfg.Device(e.Key)
		d.Name = e.Name
		d.IdleTimeout = config.IntPtr(s.idle)
		d.SleepTimeout = config.IntPtr(s.sleep)
		d.DimPercent = config.IntPtr(s.dim)
		m.setStatus(app.ApplyTimers(e, d), "saved; press w to store in the keyboard")
		m.save()
	}
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
	eff := s.effects[s.zone]
	err := e.Ctl.Claim()
	if err == nil {
		err = e.Ctl.SetEffect(s.zone, eff)
	}
	if err == nil {
		s.claimed = true
		d := m.cfg.Device(e.Key)
		d.Name = e.Name
		d.SetZone(s.zone, eff)
		m.save()
	}
	m.setStatus(err, fmt.Sprintf("%s: %s", e.Ctl.Zones()[s.zone].Location, eff.Name))
}

func (m *model) save() {
	if err := m.cfg.Save(); err != nil {
		m.setStatus(err)
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
	accent     = lipgloss.AdaptiveColor{Light: "#0b7a75", Dark: "#4fd1c5"}
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

func (m *model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("logirgb") + dimStyle.Render("  Logitech RGB control") + "\n")

	if denied := app.PermissionProblems(m.entries); len(denied) > 0 {
		b.WriteString(warnBox.Render(errStyle.Render("Permission problem") + "\n" + strings.TrimRight(app.PermissionHelp(denied), "\n")))
		b.WriteString("\n")
	}

	left := m.viewDevices()
	right := m.viewSettings()
	lp, rp := paneStyle, paneStyle
	if m.focus == focusDevices {
		lp = focusPane
	} else {
		rp = focusPane
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, lp.Width(30).Render(left), rp.Width(max(m.width-36, 44)).Render(right)))
	b.WriteString("\n")

	if m.status != "" {
		if m.isError {
			b.WriteString(errStyle.Render("✗ "+m.status) + "\n")
		} else {
			b.WriteString(selStyle.Render("✓ ") + m.status + "\n")
		}
	}
	help := "tab switch pane · ↑↓ select · ←→ change (shift: faster) · enter edit color · a apply saved · w store in device · o onboard lighting · ctrl+r rescan · q quit"
	if m.editing {
		help = "type #rrggbb, r,g,b or a color name · enter confirm · esc cancel"
	}
	b.WriteString(dimStyle.Render(help))
	return b.String()
}

func (m *model) viewDevices() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Devices") + "\n")
	if len(m.entries) == 0 {
		b.WriteString(dimStyle.Render("no Logitech devices found\nctrl+r to rescan"))
		return b.String()
	}
	for i, e := range m.entries {
		mark, style := "●", lipgloss.NewStyle()
		if !e.Usable() {
			mark, style = "✗", errStyle
		}
		name := e.Name
		if len(name) > 22 {
			name = name[:21] + "…"
		}
		line := fmt.Sprintf("%s %s", style.Render(mark), name)
		if i == m.dev {
			line = selStyle.Render("› ") + line
		} else {
			line = "  " + line
		}
		b.WriteString(line + "\n")
		b.WriteString(dimStyle.Render("    "+e.Info.Path) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) viewSettings() string {
	e, s := m.current()
	if e == nil {
		return dimStyle.Render("nothing selected")
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(e.Name) + dimStyle.Render("  "+e.Key) + "\n")
	if !e.Usable() {
		if e.Err == app.ErrPermission {
			b.WriteString(errStyle.Render("No access to "+e.Info.Path) + "\n" + dimStyle.Render("See the fix above, then press ctrl+r."))
		} else {
			b.WriteString(errStyle.Render(e.Err.Error()))
		}
		return b.String()
	}
	switch {
	case !s.claimed:
		b.WriteString(dimStyle.Render("showing the onboard lighting") + "\n")
	case e.Ctl.HasProfileStorage():
		b.WriteString(errStyle.Render("live preview, not stored: ") + dimStyle.Render("press w to store it") + "\n")
	}
	b.WriteString("\n")

	zones := e.Ctl.Zones()
	eff := s.effects[s.zone]
	ze, _ := zones[s.zone].Find(eff.Name)
	for i, r := range m.rows() {
		var val string
		switch r.kind {
		case rowZone:
			val = fmt.Sprintf("%s (%d/%d)", zones[s.zone].Location, s.zone+1, len(zones))
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
		if r.kind == rowBrightness || r.kind == rowIdle {
			b.WriteString("\n")
		}
		if i == m.row && m.focus == focusSettings {
			b.WriteString(selStyle.Render("› "+label) + " " + selStyle.Render("‹ ") + val + selStyle.Render(" ›") + "\n")
		} else {
			b.WriteString("  " + label + "   " + val + "\n")
		}
	}
	if e.Ctl.HasOnboardProfiles() {
		b.WriteString("\n" + dimStyle.Render("changes last until a profile switch; press w to store them in all onboard profiles"))
	}
	if e.Ctl.HasProfileStorage() {
		b.WriteString("\n" + dimStyle.Render("the keyboard only dims and turns off stored lighting; press w to store"))
	}
	return strings.TrimRight(b.String(), "\n")
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
