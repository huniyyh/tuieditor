package toolbar

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// InsertCharMsg is sent when user clicks a toolbar button.
type InsertCharMsg struct {
	Char string
}

// buttons defines the special characters shown in the toolbar.
// Each entry is the display label and the character to insert.
type button struct {
	label string
	char  string
}

var buttons = []button{
	{"{", "{"}, {"}", "}"},
	{"[", "["}, {"]", "]"},
	{"(", "("}, {")", ")"},
	{"<", "<"}, {">", ">"},
	{"/", "/"}, {"\\", "\\"},
	{"|", "|"}, {"&", "&"},
	{";", ";"}, {":", ":"},
	{"=", "="}, {"!", "!"},
	{"?", "?"}, {"#", "#"},
	{"@", "@"}, {`"`, `"`},
	{"'", "'"}, {"`", "`"},
	{"~", "~"}, {"$", "$"},
	{"%", "%"}, {"^", "^"},
	{"*", "*"}, {"+", "+"},
	{"-", "-"}, {"_", "_"},
}

type Model struct {
	width int

	styleBtn  lipgloss.Style
	styleBar  lipgloss.Style
	btnRanges []btnRange // computed per View(); recomputed on each Update()
}

type btnRange struct {
	start int
	end   int
	char  string
}

func New() Model {
	return Model{
		styleBtn: lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 0).
			Margin(0, 0),
		styleBar: lipgloss.NewStyle().
			Background(lipgloss.Color("234")).
			Foreground(lipgloss.Color("245")),
	}
}

func (m *Model) SetWidth(w int) {
	m.width = w
}

// computeButtonRanges returns the list of button X ranges for the current width.
// Buttons are rendered as " X " with a single space separator between them.
// Buttons that don't fit are omitted.
func (m Model) computeButtonRanges() []btnRange {
	var ranges []btnRange
	pos := 0
	for _, b := range buttons {
		label := " " + b.label + " "
		w := len([]rune(label))
		if pos+w > m.width {
			break
		}
		ranges = append(ranges, btnRange{start: pos, end: pos + w, char: b.char})
		pos += w
		// one space separator
		if pos < m.width {
			pos++
		}
	}
	return ranges
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			x := msg.X
			ranges := m.computeButtonRanges()
			for _, r := range ranges {
				if x >= r.start && x < r.end {
					ch := r.char
					return m, func() tea.Msg { return InsertCharMsg{Char: ch} }
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var sb strings.Builder
	pos := 0
	for i, b := range buttons {
		label := " " + b.label + " "
		w := len([]rune(label))
		if pos+w > m.width {
			break
		}
		sb.WriteString(m.styleBtn.Render(label))
		pos += w
		if pos < m.width && i < len(buttons)-1 {
			sb.WriteString(" ")
			pos++
		}
	}

	// Fill remaining width
	remaining := m.width - pos
	if remaining > 0 {
		sb.WriteString(strings.Repeat(" ", remaining))
	}

	return lipgloss.NewStyle().
		Background(lipgloss.Color("234")).
		Width(m.width).
		Render(sb.String())
}
