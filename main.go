package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"tuieditor/internal/editor"
	"tuieditor/internal/filetree"
	"tuieditor/internal/statusbar"
	"tuieditor/internal/tabs"
	"tuieditor/internal/toolbar"
)

type FocusPanel int

const (
	FocusTree FocusPanel = iota
	FocusEditor
)

type KeyMap struct {
	FocusTree   key.Binding
	FocusEditor key.Binding
	Quit        key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		FocusTree: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("Ctrl+T", "focus tree"),
		),
		FocusEditor: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("Ctrl+E", "focus editor"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+q"),
			key.WithHelp("Ctrl+Q", "quit"),
		),
	}
}

type Model struct {
	tree      filetree.Model
	editor    editor.Model
	tabs      tabs.Model
	statusbar statusbar.Model
	toolbar   toolbar.Model

	focus  FocusPanel
	keymap KeyMap

	width  int
	height int

	// Open editors keyed by path
	editors map[string]editor.Model
}

func initialModel(rootPath string) Model {
	m := Model{
		tree:      filetree.New(rootPath),
		editor:    editor.New(),
		tabs:      tabs.New(),
		statusbar: statusbar.New(),
		toolbar:   toolbar.New(),
		focus:     FocusTree,
		keymap:    DefaultKeyMap(),
		editors:   make(map[string]editor.Model),
	}
	m.tree.Focus()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	tabConsumed := false

	switch typedMsg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = typedMsg.Width
		m.height = typedMsg.Height
		m.layout()

	case tea.MouseMsg:
		cmds = append(cmds, m.routeMouse(typedMsg)...)

	case tea.KeyMsg:
		switch {
		case key.Matches(typedMsg, m.keymap.Quit):
			return m, tea.Quit

		case key.Matches(typedMsg, m.keymap.FocusTree):
			m.setFocus(FocusTree)
			return m, nil

		case key.Matches(typedMsg, m.keymap.FocusEditor):
			m.setFocus(FocusEditor)
			return m, nil
		}

		// Tab switching: always global, regardless of focus
		tabModel, tabCmd := m.tabs.Update(typedMsg)
		m.tabs = tabModel
		if tabCmd != nil {
			cmds = append(cmds, tabCmd)
			tabConsumed = true
		}

	case filetree.OpenFileMsg:
		m.openFile(typedMsg.Path)

	case tabs.SwitchTabMsg:
		// Remove closed editor from cache
		if typedMsg.ClosedPath != "" {
			delete(m.editors, typedMsg.ClosedPath)
		}
		path := m.tabs.ActivePath()
		if path != "" {
			m.switchToFile(path)
			m.setFocus(FocusEditor)
		} else {
			// All tabs closed: reset editor to blank state
			m.editor = editor.New()
			m.editor.SetSize(m.width-m.treeWidth(), m.editorHeight())
		}

	case tabs.ExitRequestMsg:
		return m, tea.Quit

	case tabs.SaveRequestMsg:
		// Same as Ctrl+S: save current editor
		if err := m.editor.SaveFile(); err == nil {
			m.tabs.SetModified(m.editor.Path(), false)
			if m.editor.Path() != "" {
				m.editors[m.editor.Path()] = m.editor
			}
		}

	case tabs.CloseRequestMsg:
		// Same as Ctrl+W: close active tab
		if len(m.tabs.Tabs()) > 0 {
			closedPath := m.tabs.ActivePath()
			activeIdx := m.tabs.ActiveIndex()
			m.tabs.CloseTab(activeIdx)
			delete(m.editors, closedPath)
			path := m.tabs.ActivePath()
			if path != "" {
				m.switchToFile(path)
				m.setFocus(FocusEditor)
			} else {
				m.editor = editor.New()
				m.editor.SetSize(m.width-m.treeWidth(), m.editorHeight())
			}
		}

	case toolbar.InsertCharMsg:
		// Insert the character into the editor (if a file is open)
		if m.editor.Path() != "" {
			cmd := m.editor.InsertChars(typedMsg.Char)
			m.editors[m.editor.Path()] = m.editor
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case editor.ContentChangedMsg:
		m.tabs.SetModified(typedMsg.Path, typedMsg.Modified)
		// Save modified state
		if ed, ok := m.editors[typedMsg.Path]; ok {
			m.editors[typedMsg.Path] = ed
		}
	}

	// Route messages to focused panel (only if tab key wasn't consumed)
	if !tabConsumed {
		switch m.focus {
		case FocusTree:
			newTree, cmd := m.tree.Update(msg)
			m.tree = newTree
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case FocusEditor:
			newEditor, cmd := m.editor.Update(msg)
			// Save editor state
			if m.editor.Path() != "" {
				m.editors[m.editor.Path()] = newEditor
			}
			m.editor = newEditor
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	// Always re-apply layout to ensure all components have correct sizes
	m.layout()

	// Update status bar
	m.statusbar.SetFile(
		m.editor.Path(),
		m.editor.CursorRow(),
		m.editor.CursorCol(),
		m.editor.LineCount(),
		m.editor.Modified(),
	)

	return m, tea.Batch(cmds...)
}

func (m *Model) openFile(path string) {
	// Save current editor state
	if m.editor.Path() != "" {
		m.editors[m.editor.Path()] = m.editor
	}

	editorW := m.width - m.treeWidth()
	editorH := m.editorHeight()

	if existing, ok := m.editors[path]; ok {
		existing.SetSize(editorW, editorH)
		m.editor = existing
	} else {
		newEditor := editor.New()
		newEditor.SetSize(editorW, editorH)
		if err := newEditor.LoadFile(path); err != nil {
			return
		}
		m.editors[path] = newEditor
		m.editor = newEditor
	}

	m.tabs.AddTab(path)
	m.setFocus(FocusEditor)
}

func (m *Model) switchToFile(path string) {
	if path == "" {
		return
	}
	if m.editor.Path() != "" {
		m.editors[m.editor.Path()] = m.editor
	}

	editorW := m.width - m.treeWidth()
	editorH := m.editorHeight()

	if existing, ok := m.editors[path]; ok {
		existing.SetSize(editorW, editorH)
		m.editor = existing
	} else {
		newEditor := editor.New()
		newEditor.SetSize(editorW, editorH)
		_ = newEditor.LoadFile(path)
		m.editors[path] = newEditor
		m.editor = newEditor
	}
}

func (m *Model) setFocus(f FocusPanel) {
	m.focus = f
	switch f {
	case FocusTree:
		m.tree.Focus()
		m.editor.Blur()
	case FocusEditor:
		m.tree.Blur()
		m.editor.Focus()
	}
}

func (m *Model) treeWidth() int {
	w := m.width / 4
	if w < 20 {
		w = 20
	}
	if w > 40 {
		w = 40
	}
	return w
}

func (m *Model) editorHeight() int {
	// height = tabbar(1) + editor(editorH) + toolbar(1) + statusbar(1)
	// → editorH = height - 3
	h := m.height - 3
	if h < 5 {
		h = 5
	}
	return h
}

func (m *Model) layout() {
	treeW := m.treeWidth()
	editorW := m.width - treeW
	editorH := m.editorHeight()

	m.tree.SetSize(treeW, editorH+1) // tabbar(1) + editor(editorH) — same height as right panel excluding toolbar/statusbar
	m.editor.SetSize(editorW, editorH)
	m.tabs.SetWidth(editorW)
	m.toolbar.SetWidth(m.width)
	m.statusbar.SetWidth(m.width)
}

// routeMouse dispatches mouse events to the correct panel based on coordinates.
// Layout (y coordinates):
//
//	x:  0..treeW-1           → tree panel
//	x:  treeW..width-1       → editor area
//	  y: 0                   →   tab bar   (local x = x - treeW)
//	  y: 1..editorHeight+2   →   editor    (local x = x - treeW, local y = y - 1)
//	  y: editorHeight+3      →   toolbar   (local x = x)
func (m *Model) routeMouse(msg tea.MouseMsg) []tea.Cmd {
	var cmds []tea.Cmd
	treeW := m.treeWidth()
	editorH := m.editorHeight()
	x, y := msg.X, msg.Y

	toolbarRow := 1 + editorH // tabbar(1) + editor(editorH rows)

	if x < treeW {
		// Tree panel — pass with local coords
		localMsg := msg
		localMsg.X = x
		localMsg.Y = y
		newTree, cmd := m.tree.Update(localMsg)
		m.tree = newTree
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Focus tree on click
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m.setFocus(FocusTree)
		}
	} else {
		// Editor area
		localX := x - treeW
		if y == 0 {
			// Tab bar — pass local x (y is always 0 in tab bar)
			localMsg := msg
			localMsg.X = localX
			localMsg.Y = 0
			newTabs, cmd := m.tabs.Update(localMsg)
			m.tabs = newTabs
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else if y == toolbarRow {
			// Toolbar row — local x is full width
			localMsg := msg
			localMsg.X = x
			localMsg.Y = 0
			newToolbar, cmd := m.toolbar.Update(localMsg)
			m.toolbar = newToolbar
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else if y > 0 && y < toolbarRow {
			// Editor — local y starts at 1 (tab bar = 0)
			localMsg := msg
			localMsg.X = localX
			localMsg.Y = y - 1 // subtract tab bar row
			newEditor, cmd := m.editor.Update(localMsg)
			if m.editor.Path() != "" {
				m.editors[m.editor.Path()] = newEditor
			}
			m.editor = newEditor
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			// Focus editor on click
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
				m.setFocus(FocusEditor)
			}
		}
	}
	return cmds
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	treeW := m.treeWidth()
	editorW := m.width - treeW
	editorH := m.editorHeight()

	// Tab bar (1 row)
	tabBar := lipgloss.NewStyle().
		Width(editorW).
		Height(1).
		Background(lipgloss.Color("232")).
		Render(m.tabs.View())

	// Editor panel — clamp to exactly editorH rows
	editorPanel := lipgloss.NewStyle().
		Width(editorW).
		Height(editorH).
		Render(m.editor.View())

	// Right side: tabbar(1) + editor(editorH) = editorH+1 rows
	rightPanel := lipgloss.JoinVertical(lipgloss.Left, tabBar, editorPanel)

	// Tree panel: same total height as rightPanel
	treePanel := lipgloss.NewStyle().
		Width(treeW).
		Height(editorH + 1).
		Render(m.tree.View())

	// Main area: tree | editor
	main := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, rightPanel)

	// Toolbar (full width, 1 row)
	toolbarView := m.toolbar.View()

	// Status bar (full width, 1 row)
	statusView := m.statusbar.View()

	return lipgloss.JoinVertical(lipgloss.Left, main, toolbarView, statusView)
}

func main() {
	rootPath := "."
	if len(os.Args) > 1 {
		rootPath = os.Args[1]
	}

	// Verify path exists
	if _, err := os.Stat(rootPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	m := initialModel(rootPath)

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
