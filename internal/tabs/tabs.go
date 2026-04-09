package tabs

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SwitchTabMsg is sent when user selects a tab
type SwitchTabMsg struct {
	Index      int
	ClosedPath string // set when a tab was closed
}

// SaveRequestMsg is sent when the [Save] button is clicked
type SaveRequestMsg struct{}

// CloseRequestMsg is sent when the [Close] button is clicked
type CloseRequestMsg struct{}

// ExitRequestMsg is sent when the [Exit] button is clicked
type ExitRequestMsg struct{}

type Tab struct {
	Path     string
	Modified bool
}

type KeyMap struct {
	Prev  key.Binding
	Next  key.Binding
	Close key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Prev: key.NewBinding(
			key.WithKeys("alt+["),
			key.WithHelp("Alt+[", "prev tab"),
		),
		Next: key.NewBinding(
			key.WithKeys("alt+]"),
			key.WithHelp("Alt+]", "next tab"),
		),
		Close: key.NewBinding(
			key.WithKeys("ctrl+w"),
			key.WithHelp("C-w", "close tab"),
		),
	}
}

type Model struct {
	tabs    []Tab
	active  int
	width   int
	focused bool
	keymap  KeyMap

	styleActive   lipgloss.Style
	styleInactive lipgloss.Style
	styleModified lipgloss.Style
	styleBtn      lipgloss.Style
	styleBtnExit  lipgloss.Style
}

// btnLabels are the action buttons rendered on the right side of the tab bar.
// Order: Save, Close, Exit
const (
	btnLabelSave  = "[Save]"
	btnLabelClose = "[Close]"
	btnLabelExit  = "[Exit]"
)

func New() Model {
	return Model{
		keymap: DefaultKeyMap(),
		styleActive: lipgloss.NewStyle().
			Background(lipgloss.Color("99")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1),
		styleInactive: lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("245")).
			Padding(0, 1),
		styleModified: lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")),
		styleBtn: lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1),
		styleBtnExit: lipgloss.NewStyle().
			Background(lipgloss.Color("52")).
			Foreground(lipgloss.Color("203")).
			Padding(0, 1),
	}
}

func (m *Model) AddTab(path string) {
	// Check if already open
	for i, t := range m.tabs {
		if t.Path == path {
			m.active = i
			return
		}
	}
	m.tabs = append(m.tabs, Tab{Path: path})
	m.active = len(m.tabs) - 1
}

func (m *Model) CloseTab(index int) {
	if index < 0 || index >= len(m.tabs) {
		return
	}
	m.tabs = append(m.tabs[:index], m.tabs[index+1:]...)
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
	if m.active < 0 {
		m.active = 0
	}
}

func (m *Model) SetModified(path string, modified bool) {
	for i, t := range m.tabs {
		if t.Path == path {
			m.tabs[i].Modified = modified
			return
		}
	}
}

func (m Model) ActivePath() string {
	if m.active < 0 || m.active >= len(m.tabs) {
		return ""
	}
	return m.tabs[m.active].Path
}

func (m Model) Tabs() []Tab {
	return m.tabs
}

func (m Model) ActiveIndex() int {
	return m.active
}

func (m Model) Init() tea.Cmd {
	return nil
}

type btnRange struct {
	start, end int
}

type actionBtns struct {
	save, close, exit btnRange
}

// actionButtonRanges computes the X ranges of the Save/Close/Exit buttons.
// Must match View() layout exactly.
func (m Model) actionButtonRanges() actionBtns {
	saveR := m.styleBtn.Render(btnLabelSave)
	closeR := m.styleBtn.Render(btnLabelClose)
	exitR := m.styleBtnExit.Render(btnLabelExit)
	saveW := lipgloss.Width(saveR)
	closeW := lipgloss.Width(closeR)
	exitW := lipgloss.Width(exitR)
	btnsW := saveW + 1 + closeW + 1 + exitW // spaces between buttons

	tabBarW := m.visibleTabBarWidth(btnsW)

	gap := m.width - tabBarW - btnsW
	if gap < 0 {
		gap = 0
	}

	saveStart := tabBarW + gap
	closeStart := saveStart + saveW + 1
	exitStart := closeStart + closeW + 1
	return actionBtns{
		save:  btnRange{saveStart, saveStart + saveW},
		close: btnRange{closeStart, closeStart + closeW},
		exit:  btnRange{exitStart, exitStart + exitW},
	}
}

// visibleTabBarWidth returns the rendered width of the tab portion only.
func (m Model) visibleTabBarWidth(btnsW int) int {
	if len(m.tabs) == 0 {
		left := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" No open files")
		return lipgloss.Width(left)
	}
	avail := m.width - btnsW - 1
	if avail < 0 {
		avail = 0
	}
	tabW := make([]int, len(m.tabs))
	for i, t := range m.tabs {
		name := filepath.Base(t.Path)
		if t.Modified {
			name = name + " ●"
		}
		var s string
		if i == m.active {
			s = m.styleActive.Render(name)
		} else {
			s = m.styleInactive.Render(name)
		}
		tabW[i] = lipgloss.Width(s)
	}
	used := tabW[m.active]
	lo, hi := m.active, m.active
	for {
		extended := false
		if hi+1 < len(m.tabs) {
			if extra := 1 + tabW[hi+1]; used+extra <= avail {
				used += extra
				hi++
				extended = true
			}
		}
		if lo-1 >= 0 {
			if extra := 1 + tabW[lo-1]; used+extra <= avail {
				used += extra
				lo--
				extended = true
			}
		}
		if !extended {
			break
		}
	}
	_ = lo
	return used
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			x := msg.X
			btns := m.actionButtonRanges()
			if x >= btns.save.start && x < btns.save.end {
				return m, func() tea.Msg { return SaveRequestMsg{} }
			}
			if x >= btns.close.start && x < btns.close.end {
				return m, func() tea.Msg { return CloseRequestMsg{} }
			}
			if x >= btns.exit.start && x < btns.exit.end {
				return m, func() tea.Msg { return ExitRequestMsg{} }
			}
			if idx := m.tabAtX(msg.X); idx >= 0 {
				m.active = idx
				return m, func() tea.Msg { return SwitchTabMsg{Index: idx} }
			}
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Prev):
			if len(m.tabs) > 0 {
				m.active = (m.active - 1 + len(m.tabs)) % len(m.tabs)
				return m, func() tea.Msg { return SwitchTabMsg{Index: m.active} }
			}
		case key.Matches(msg, m.keymap.Next):
			if len(m.tabs) > 0 {
				m.active = (m.active + 1) % len(m.tabs)
				return m, func() tea.Msg { return SwitchTabMsg{Index: m.active} }
			}
		case key.Matches(msg, m.keymap.Close):
			if len(m.tabs) > 0 {
				closedPath := m.tabs[m.active].Path
				m.CloseTab(m.active)
				idx := m.active
				return m, func() tea.Msg {
					return SwitchTabMsg{Index: idx, ClosedPath: closedPath}
				}
			}
		}
	}
	return m, nil
}

// tabAtX returns the tab index at the given X offset (local to tab bar), or -1.
func (m Model) tabAtX(x int) int {
	if len(m.tabs) == 0 {
		return -1
	}
	saveR := m.styleBtn.Render(btnLabelSave)
	closeR := m.styleBtn.Render(btnLabelClose)
	exitR := m.styleBtnExit.Render(btnLabelExit)
	btnsW := lipgloss.Width(saveR) + 1 + lipgloss.Width(closeR) + 1 + lipgloss.Width(exitR)

	avail := m.width - btnsW - 1
	if avail < 0 {
		avail = 0
	}
	tabW := make([]int, len(m.tabs))
	for i, t := range m.tabs {
		name := filepath.Base(t.Path)
		if t.Modified {
			name = name + " ●"
		}
		var s string
		if i == m.active {
			s = m.styleActive.Render(name)
		} else {
			s = m.styleInactive.Render(name)
		}
		tabW[i] = lipgloss.Width(s)
	}
	used := tabW[m.active]
	lo, hi := m.active, m.active
	for {
		extended := false
		if hi+1 < len(m.tabs) {
			if extra := 1 + tabW[hi+1]; used+extra <= avail {
				used += extra
				hi++
				extended = true
			}
		}
		if lo-1 >= 0 {
			if extra := 1 + tabW[lo-1]; used+extra <= avail {
				used += extra
				lo--
				extended = true
			}
		}
		if !extended {
			break
		}
	}
	pos := 0
	for i := lo; i <= hi; i++ {
		if i > lo {
			pos++
		}
		if x >= pos && x < pos+tabW[i] {
			return i
		}
		pos += tabW[i]
	}
	return -1
}

func (m *Model) View() string {
	saveR := m.styleBtn.Render(btnLabelSave)
	closeR := m.styleBtn.Render(btnLabelClose)
	exitR := m.styleBtnExit.Render(btnLabelExit)
	btns := saveR + " " + closeR + " " + exitR
	btnsW := lipgloss.Width(btns)

	if len(m.tabs) == 0 {
		left := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render(" No open files")
		leftW := lipgloss.Width(left)
		gap := m.width - leftW - btnsW
		if gap < 0 {
			gap = 0
		}
		return left + strings.Repeat(" ", gap) + btns
	}

	avail := m.width - btnsW - 1
	if avail < 0 {
		avail = 0
	}

	type renderedTab struct {
		s string
		w int
	}
	rendered := make([]renderedTab, len(m.tabs))
	for i, t := range m.tabs {
		name := filepath.Base(t.Path)
		if t.Modified {
			name = name + m.styleModified.Render(" ●")
		}
		var s string
		if i == m.active {
			s = m.styleActive.Render(name)
		} else {
			s = m.styleInactive.Render(name)
		}
		rendered[i] = renderedTab{s: s, w: lipgloss.Width(s)}
	}

	used := rendered[m.active].w
	lo, hi := m.active, m.active
	for {
		extended := false
		if hi+1 < len(m.tabs) {
			if extra := 1 + rendered[hi+1].w; used+extra <= avail {
				used += extra
				hi++
				extended = true
			}
		}
		if lo-1 >= 0 {
			if extra := 1 + rendered[lo-1].w; used+extra <= avail {
				used += extra
				lo--
				extended = true
			}
		}
		if !extended {
			break
		}
	}

	var parts []string
	for i := lo; i <= hi; i++ {
		parts = append(parts, rendered[i].s)
	}
	tabBar := strings.Join(parts, " ")
	tabBarW := lipgloss.Width(tabBar)

	gap := m.width - tabBarW - btnsW
	if gap < 0 {
		gap = 0
	}
	return tabBar + strings.Repeat(" ", gap) + btns
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m *Model) Focus() {
	m.focused = true
}

func (m *Model) Blur() {
	m.focused = false
}
