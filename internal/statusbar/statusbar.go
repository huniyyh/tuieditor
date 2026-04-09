package statusbar

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	filePath string
	row      int
	col      int
	total    int
	modified bool
	mode     string
	width    int

	stylePath     lipgloss.Style
	stylePosition lipgloss.Style
	styleModified lipgloss.Style
	styleMode     lipgloss.Style
	styleBar      lipgloss.Style
}

func New() Model {
	return Model{
		mode: "NORMAL",
		stylePath: lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Padding(0, 1),
		stylePosition: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Padding(0, 1),
		styleModified: lipgloss.NewStyle().
			Foreground(lipgloss.Color("220")).
			Bold(true).
			Padding(0, 1),
		styleMode: lipgloss.NewStyle().
			Background(lipgloss.Color("99")).
			Foreground(lipgloss.Color("255")).
			Bold(true).
			Padding(0, 1),
		styleBar: lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Width(0),
	}
}

func (m *Model) SetFile(path string, row, col, total int, modified bool) {
	m.filePath = path
	m.row = row
	m.col = col
	m.total = total
	m.modified = modified
}

func (m *Model) SetMode(mode string) {
	m.mode = mode
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m Model) View() string {
	mode := m.styleMode.Render(" " + m.mode + " ")

	var pathPart string
	if m.filePath != "" {
		name := filepath.Base(m.filePath)
		dir := filepath.Dir(m.filePath)
		pathPart = m.stylePath.Render(dir + "/" + name)
	} else {
		pathPart = m.stylePath.Render("No file")
	}

	var modPart string
	if m.modified {
		modPart = m.styleModified.Render("[+]")
	}

	pos := m.stylePosition.Render(fmt.Sprintf("Ln %d, Col %d  %d lines", m.row+1, m.col+1, m.total))

	// Key hints
	hints := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 1).
		Render("Ctrl+S: Save  Ctrl+W: Close  Ctrl+T: Tree  Ctrl+E: Editor  Alt+]/[: Tab")

	left := mode + pathPart + modPart
	right := pos

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	hintsW := lipgloss.Width(hints)
	gap := m.width - leftW - rightW - hintsW
	if gap < 0 {
		gap = 0
		hints = ""
	}

	bar := left + hints + strings.Repeat(" ", gap) + right

	return lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Width(m.width).
		Render(bar)
}
