package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// OpenFileMsg is sent when a file is selected in the tree
type OpenFileMsg struct {
	Path string
}

// ChangeRootMsg is sent when the tree root changes (go to parent)
type ChangeRootMsg struct {
	Path string
}

type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Children []*Node
	Expanded bool
	Depth    int
}

type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Enter    key.Binding
	Toggle   key.Binding
	GoParent key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Toggle: key.NewBinding(
			key.WithKeys("space"),
			key.WithHelp("space", "toggle"),
		),
		GoParent: key.NewBinding(
			key.WithKeys("-", "backspace"),
			key.WithHelp("-/BS", "parent dir"),
		),
	}
}

type Model struct {
	root     *Node
	rootPath string
	items    []*Node // flattened visible items
	cursor   int
	width    int
	height   int
	focused  bool
	keymap   KeyMap

	// double-click detection
	lastClickRow  int
	lastClickTime time.Time

	// styles
	styleNormal   lipgloss.Style
	styleFocused  lipgloss.Style
	styleSelected lipgloss.Style
	styleBorder   lipgloss.Style
	styleDir      lipgloss.Style
	styleFile     lipgloss.Style
}

func New(rootPath string) Model {
	abs, err := filepath.Abs(rootPath)
	if err != nil {
		abs = rootPath
	}
	m := Model{
		rootPath: abs,
		keymap:   DefaultKeyMap(),
		styleNormal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")),
		styleFocused: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
		styleSelected: lipgloss.NewStyle().
			Background(lipgloss.Color("237")).
			Foreground(lipgloss.Color("255")),
		styleBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")),
		styleDir: lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			Bold(true),
		styleFile: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")),
	}

	root := buildTree(abs, 0)
	if root != nil {
		root.Expanded = true
	}
	m.root = root
	m.flatten()
	return m
}

func buildTree(path string, depth int) *Node {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	node := &Node{
		Name:  info.Name(),
		Path:  path,
		IsDir: info.IsDir(),
		Depth: depth,
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return node
		}
		// dirs first, then files
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if e.IsDir() {
				child := buildTree(filepath.Join(path, e.Name()), depth+1)
				if child != nil {
					node.Children = append(node.Children, child)
				}
			}
		}
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if !e.IsDir() {
				child := buildTree(filepath.Join(path, e.Name()), depth+1)
				if child != nil {
					node.Children = append(node.Children, child)
				}
			}
		}
	}
	return node
}

func (m *Model) flatten() {
	m.items = nil
	if m.root == nil {
		return
	}
	// Don't show root itself, show its children
	flattenNode(m.root, &m.items, true)
}

func flattenNode(n *Node, items *[]*Node, isRoot bool) {
	if !isRoot {
		*items = append(*items, n)
	}
	if n.IsDir && n.Expanded {
		for _, child := range n.Children {
			flattenNode(child, items, false)
		}
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			return m.handleClick(msg.X, msg.Y)
		}

	case tea.KeyMsg:
		if !m.focused {
			break
		}
		switch {
		case key.Matches(msg, m.keymap.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keymap.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keymap.GoParent):
			parent := filepath.Dir(m.rootPath)
			if parent != m.rootPath {
				m.rootPath = parent
				root := buildTree(parent, 0)
				if root != nil {
					root.Expanded = true
				}
				m.root = root
				m.cursor = 0
				m.flatten()
				return m, func() tea.Msg {
					return ChangeRootMsg{Path: parent}
				}
			}
		case key.Matches(msg, m.keymap.Enter), key.Matches(msg, m.keymap.Toggle):
			if m.cursor < len(m.items) {
				selected := m.items[m.cursor]
				if selected.IsDir {
					selected.Expanded = !selected.Expanded
					m.flatten()
				} else {
					return m, func() tea.Msg {
						return OpenFileMsg{Path: selected.Path}
					}
				}
			}
		}
	}
	return m, nil
}

// handleClick processes a mouse click at absolute terminal coordinates (x, y).
// offsetY is the Y position of the tree panel's top-left corner.
// We receive local coords already translated by main before calling.
func (m Model) handleClick(localX, localY int) (Model, tea.Cmd) {
	// Border takes 1 row top, header takes 2 rows (name + separator), border 1 col left
	// layout inside border: row0=header, row1=separator, row2..=items
	contentY := localY - 1  // subtract top border
	itemRow := contentY - 2 // subtract header(1) + separator(1)

	if itemRow < 0 {
		// clicked header area — just focus
		return m, nil
	}

	visibleStart := 0
	innerHeight := m.height - 4
	if innerHeight < 1 {
		innerHeight = 1
	}
	if m.cursor >= innerHeight {
		visibleStart = m.cursor - innerHeight + 1
	}

	targetIdx := visibleStart + itemRow
	if targetIdx < 0 || targetIdx >= len(m.items) {
		return m, nil
	}

	now := time.Now()
	doubleClick := targetIdx == m.lastClickRow && now.Sub(m.lastClickTime) < 400*time.Millisecond
	m.lastClickRow = targetIdx
	m.lastClickTime = now
	m.cursor = targetIdx

	selected := m.items[targetIdx]
	if doubleClick || !selected.IsDir {
		// double-click or file: open/toggle
		if selected.IsDir {
			selected.Expanded = !selected.Expanded
			m.flatten()
		} else {
			path := selected.Path
			return m, func() tea.Msg {
				return OpenFileMsg{Path: path}
			}
		}
	}
	// single click on dir: just move cursor
	return m, nil
}

func (m Model) View() string {
	var sb strings.Builder

	// Root path header (1 line) + separator
	rootName := filepath.Base(m.rootPath)
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("33")).
		Bold(true)
	header := headerStyle.Render("  " + rootName + "/")
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("237")).Render(strings.Repeat("─", m.width-4)))
	sb.WriteString("\n")

	visibleStart := 0
	innerHeight := m.height - 4 // account for border + header + separator
	if innerHeight < 1 {
		innerHeight = 1
	}

	// simple scroll: keep cursor visible
	if m.cursor >= innerHeight {
		visibleStart = m.cursor - innerHeight + 1
	}

	for i := visibleStart; i < len(m.items) && i < visibleStart+innerHeight; i++ {
		node := m.items[i]
		indent := strings.Repeat("  ", node.Depth-1)

		var icon string
		var name string
		if node.IsDir {
			if node.Expanded {
				icon = "▾ "
			} else {
				icon = "▸ "
			}
			name = m.styleDir.Render(node.Name)
		} else {
			icon = "  "
			name = m.styleFile.Render(node.Name)
		}

		line := indent + icon + name

		if i == m.cursor {
			// pad to width for full-width highlight
			innerWidth := m.width - 2
			if innerWidth < 0 {
				innerWidth = 0
			}
			raw := indent + icon + node.Name
			padLen := innerWidth - lipgloss.Width(raw)
			if padLen < 0 {
				padLen = 0
			}
			line = m.styleSelected.Render(indent+icon+node.Name) + strings.Repeat(" ", padLen)
		}

		sb.WriteString(line)
		if i < len(m.items)-1 {
			sb.WriteString("\n")
		}
	}

	borderColor := lipgloss.Color("240")
	if m.focused {
		borderColor = lipgloss.Color("99")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		BorderTop(true).
		Width(m.width - 2).
		Height(m.height - 2).
		Render(sb.String())
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m *Model) Focus() {
	m.focused = true
}

func (m *Model) Blur() {
	m.focused = false
}

func (m Model) Focused() bool {
	return m.focused
}

func (m Model) RootPath() string {
	return m.rootPath
}
