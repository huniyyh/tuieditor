package editor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ContentChangedMsg signals content was modified
type ContentChangedMsg struct {
	Path     string
	Modified bool
}

type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	Home       key.Binding
	End        key.Binding
	PageUp     key.Binding
	PageDown   key.Binding
	Save       key.Binding
	DeleteChar key.Binding
	Backspace  key.Binding
	NewLine    key.Binding
	WordLeft   key.Binding
	WordRight  key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up:         key.NewBinding(key.WithKeys("up")),
		Down:       key.NewBinding(key.WithKeys("down")),
		Left:       key.NewBinding(key.WithKeys("left")),
		Right:      key.NewBinding(key.WithKeys("right")),
		Home:       key.NewBinding(key.WithKeys("home", "ctrl+a")),
		End:        key.NewBinding(key.WithKeys("end", "ctrl+e")),
		PageUp:     key.NewBinding(key.WithKeys("pgup", "ctrl+b")),
		PageDown:   key.NewBinding(key.WithKeys("pgdown", "ctrl+f")),
		Save:       key.NewBinding(key.WithKeys("ctrl+s")),
		DeleteChar: key.NewBinding(key.WithKeys("delete", "ctrl+d")),
		Backspace:  key.NewBinding(key.WithKeys("backspace")),
		NewLine:    key.NewBinding(key.WithKeys("enter")),
		WordLeft:   key.NewBinding(key.WithKeys("alt+left", "ctrl+left")),
		WordRight:  key.NewBinding(key.WithKeys("alt+right", "ctrl+right")),
	}
}

// visualRow represents one screen row produced by word-wrap.
type visualRow struct {
	lineIdx  int // which logical line
	startCol int // rune index where this visual row starts in the logical line
	endCol   int // rune index where this visual row ends (exclusive)
}

type Model struct {
	lines     []string
	cursorRow int // logical line index
	cursorCol int // rune column in logical line

	// scrollVisRow is the first visible visual row index (into the flat visual rows list)
	scrollVisRow int

	path     string
	modified bool
	focused  bool

	width  int
	height int

	keymap KeyMap

	styleBorder    lipgloss.Style
	styleLineNum   lipgloss.Style
	styleCursor    lipgloss.Style
	styleEmpty     lipgloss.Style
	styleScrollbar lipgloss.Style
}

func New() Model {
	return Model{
		lines:  []string{""},
		keymap: DefaultKeyMap(),
		styleBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")),
		styleLineNum: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Width(4).
			Align(lipgloss.Right),
		styleCursor: lipgloss.NewStyle().
			Background(lipgloss.Color("255")).
			Foreground(lipgloss.Color("0")),
		styleEmpty: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true),
		styleScrollbar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
	}
}

func (m *Model) LoadFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		m.lines = []string{""}
		m.path = path
		m.modified = false
		m.cursorRow = 0
		m.cursorCol = 0
		m.scrollVisRow = 0
		return nil
	}
	text := string(content)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	m.lines = strings.Split(text, "\n")
	if len(m.lines) == 0 {
		m.lines = []string{""}
	}
	m.path = path
	m.modified = false
	m.cursorRow = 0
	m.cursorCol = 0
	m.scrollVisRow = 0
	return nil
}

func (m *Model) SaveFile() error {
	if m.path == "" {
		return nil
	}
	content := strings.Join(m.lines, "\n")
	err := os.WriteFile(m.path, []byte(content), 0644)
	if err != nil {
		return err
	}
	m.modified = false
	return nil
}

func (m Model) Path() string   { return m.path }
func (m Model) Modified() bool { return m.modified }
func (m Model) CursorRow() int { return m.cursorRow }
func (m Model) CursorCol() int { return m.cursorCol }
func (m Model) LineCount() int { return len(m.lines) }
func (m Model) Width() int     { return m.width }
func (m Model) Height() int    { return m.height }

func (m Model) Init() tea.Cmd { return nil }

// contentWidth returns the usable width for text content (inside border, after line-num gutter).
func (m Model) contentWidth() int {
	lineNumW := len(fmt.Sprintf("%d", len(m.lines))) + 1
	// inner = m.width - 2 (border), then - lineNumW - 1 (separator │) - 1 (scrollbar)
	w := m.width - 2 - lineNumW - 1 - 1
	if w < 1 {
		w = 1
	}
	return w
}

// lineNumWidth returns width of the line-number gutter.
func (m Model) lineNumWidth() int {
	return len(fmt.Sprintf("%d", len(m.lines))) + 1
}

// expandTabsAnsi expands tab characters in an ANSI-colored string to spaces,
// using 4-space tab stops, while preserving ANSI escape sequences.
func expandTabsAnsi(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var out strings.Builder
	col := 0
	r := []rune(s)
	i := 0
	for i < len(r) {
		if r[i] == '\x1b' && i+1 < len(r) && r[i+1] == '[' {
			// Copy ANSI escape sequence verbatim
			start := i
			i += 2
			for i < len(r) && r[i] != 'm' {
				i++
			}
			if i < len(r) {
				i++
			}
			out.WriteString(string(r[start:i]))
			continue
		}
		if r[i] == '\t' {
			spaces := 4 - (col % 4)
			out.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else {
			out.WriteRune(r[i])
			col++
		}
		i++
	}
	return out.String()
}

// expandedCol converts a rune-index cursorCol in m.lines[row] to the
// corresponding column position in the tab-expanded version of that line.
func (m Model) expandedCol(row, runeCol int) int {
	if row < 0 || row >= len(m.lines) {
		return runeCol
	}
	runes := []rune(m.lines[row])
	col := 0
	for i := 0; i < runeCol && i < len(runes); i++ {
		if runes[i] == '\t' {
			col += 4 - (col % 4)
		} else {
			col++
		}
	}
	return col
}

// runeColFromExpanded converts an expanded-col position back to a rune-index
// in m.lines[row].
func (m Model) runeColFromExpanded(row, expandedColPos int) int {
	if row < 0 || row >= len(m.lines) {
		return expandedColPos
	}
	runes := []rune(m.lines[row])
	col := 0
	for i, r := range runes {
		if col >= expandedColPos {
			return i
		}
		if r == '\t' {
			col += 4 - (col % 4)
		} else {
			col++
		}
	}
	return len(runes)
}

// expandTabs replaces tab characters with spaces (tab stop every 4 columns).
func expandTabs(s string) string {
	if !strings.ContainsRune(s, '\t') {
		return s
	}
	var out strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := 4 - (col % 4)
			out.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else {
			out.WriteRune(r)
			col++
		}
	}
	return out.String()
}

// expandedLines returns m.lines with tabs expanded to spaces (for display only).
func (m Model) expandedLines() []string {
	result := make([]string, len(m.lines))
	for i, line := range m.lines {
		result[i] = expandTabs(line)
	}
	return result
}

// buildVisualRows computes all visual rows for the current content width.
// Each logical line wraps into one or more visual rows.
func (m Model) buildVisualRows() []visualRow {
	cw := m.contentWidth()
	expanded := m.expandedLines()
	var rows []visualRow
	for i, line := range expanded {
		runes := []rune(line)
		if len(runes) == 0 {
			rows = append(rows, visualRow{lineIdx: i, startCol: 0, endCol: 0})
			continue
		}
		start := 0
		for start < len(runes) {
			end := start + cw
			if end >= len(runes) {
				end = len(runes)
			}
			rows = append(rows, visualRow{lineIdx: i, startCol: start, endCol: end})
			start = end
		}
	}
	return rows
}

// cursorVisRow returns the visual row index that contains the cursor.
func (m Model) cursorVisRow(vrows []visualRow) int {
	// vrow startCol/endCol are in expanded-column space.
	// Convert cursorCol (rune index) to expanded col for comparison.
	expCursorCol := m.expandedCol(m.cursorRow, m.cursorCol)
	best := 0
	for i, vr := range vrows {
		if vr.lineIdx != m.cursorRow {
			if vr.lineIdx > m.cursorRow {
				break
			}
			continue
		}
		// Determine if cursor belongs to this vrow.
		// A cursor at expCursorCol belongs to this vrow if:
		//   startCol <= expCursorCol < endCol  (for non-last vrows of this line)
		// OR
		//   startCol <= expCursorCol           (for the last vrow of this line)
		// We achieve this by: pick the last vrow where startCol <= expCursorCol,
		// BUT skip a vrow if expCursorCol == its endCol AND there's a next vrow for the same line.
		isLastVrow := i == len(vrows)-1 || vrows[i+1].lineIdx != vr.lineIdx
		if vr.startCol <= expCursorCol {
			if expCursorCol == vr.endCol && !isLastVrow {
				// Cursor is exactly at the boundary — belongs to the next vrow
				continue
			}
			best = i
		}
	}
	return best
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			m.handleMouseClick(msg.X, msg.Y)
		}
		return m, nil
	}

	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Save):
			if err := m.SaveFile(); err == nil {
				return m, func() tea.Msg {
					return ContentChangedMsg{Path: m.path, Modified: false}
				}
			}

		case key.Matches(msg, m.keymap.Up):
			m.moveCursorUp()
		case key.Matches(msg, m.keymap.Down):
			m.moveCursorDown()
		case key.Matches(msg, m.keymap.Left):
			m.moveCursorLeft()
		case key.Matches(msg, m.keymap.Right):
			m.moveCursorRight()
		case key.Matches(msg, m.keymap.Home):
			m.cursorCol = 0
		case key.Matches(msg, m.keymap.End):
			m.cursorCol = len([]rune(m.lines[m.cursorRow]))
		case key.Matches(msg, m.keymap.PageUp):
			for i := 0; i < m.height-2; i++ {
				m.moveCursorUp()
			}
		case key.Matches(msg, m.keymap.PageDown):
			for i := 0; i < m.height-2; i++ {
				m.moveCursorDown()
			}
		case key.Matches(msg, m.keymap.WordLeft):
			m.moveWordLeft()
		case key.Matches(msg, m.keymap.WordRight):
			m.moveWordRight()
		case key.Matches(msg, m.keymap.NewLine):
			m.insertNewLine()
			m.clampScroll()
			return m, m.changedCmd()
		case key.Matches(msg, m.keymap.Backspace):
			m.deleteBackward()
			m.clampScroll()
			return m, m.changedCmd()
		case key.Matches(msg, m.keymap.DeleteChar):
			m.deleteForward()
			m.clampScroll()
			return m, m.changedCmd()
		default:
			if msg.String() == "tab" {
				m.insertRune('\t')
				m.clampScroll()
				return m, m.changedCmd()
			}
			r := []rune(msg.String())
			if len(r) == 1 && !isControl(r[0]) {
				m.insertRune(r[0])
				m.clampScroll()
				return m, m.changedCmd()
			}
		}
		m.clampScroll()
	}

	return m, nil
}

func (m Model) changedCmd() tea.Cmd {
	return func() tea.Msg {
		return ContentChangedMsg{Path: m.path, Modified: true}
	}
}

func isControl(r rune) bool {
	return unicode.IsControl(r) && r != '\t'
}

func (m *Model) moveCursorUp() {
	vrows := m.buildVisualRows()
	curVis := m.cursorVisRow(vrows)
	if curVis == 0 {
		return
	}
	// Save expanded col before changing row
	expCol := m.expandedCol(m.cursorRow, m.cursorCol)
	prevVis := vrows[curVis-1]
	m.cursorRow = prevVis.lineIdx
	expLineLen := len([]rune(expandTabs(m.lines[m.cursorRow])))
	isLastVrowOfLine := prevVis.endCol >= expLineLen
	// Clamp expCol to target visual row's range
	if expCol < prevVis.startCol {
		expCol = prevVis.startCol
	}
	if isLastVrowOfLine {
		if expCol > expLineLen {
			expCol = expLineLen
		}
	} else {
		// On non-last vrow, cursor can only be at startCol..endCol-1
		if expCol >= prevVis.endCol {
			expCol = prevVis.endCol - 1
		}
	}
	m.cursorCol = m.runeColFromExpanded(m.cursorRow, expCol)
}

func (m *Model) moveCursorDown() {
	vrows := m.buildVisualRows()
	curVis := m.cursorVisRow(vrows)
	if curVis >= len(vrows)-1 {
		return
	}
	// Save expanded col before changing row
	expCol := m.expandedCol(m.cursorRow, m.cursorCol)
	nextVis := vrows[curVis+1]
	m.cursorRow = nextVis.lineIdx
	expLineLen := len([]rune(expandTabs(m.lines[m.cursorRow])))
	isLastVrowOfLine := nextVis.endCol >= expLineLen
	// Clamp expCol to target visual row's range
	if expCol < nextVis.startCol {
		expCol = nextVis.startCol
	}
	if isLastVrowOfLine {
		if expCol > expLineLen {
			expCol = expLineLen
		}
	} else {
		// On non-last vrow, cursor can only be at startCol..endCol-1
		if expCol >= nextVis.endCol {
			expCol = nextVis.endCol - 1
		}
	}
	m.cursorCol = m.runeColFromExpanded(m.cursorRow, expCol)
}

func (m *Model) moveCursorLeft() {
	if m.cursorCol > 0 {
		m.cursorCol--
	} else if m.cursorRow > 0 {
		m.cursorRow--
		m.cursorCol = len([]rune(m.lines[m.cursorRow]))
	}
}

func (m *Model) moveCursorRight() {
	lineLen := len([]rune(m.lines[m.cursorRow]))
	if m.cursorCol < lineLen {
		m.cursorCol++
	} else if m.cursorRow < len(m.lines)-1 {
		m.cursorRow++
		m.cursorCol = 0
	}
}

func (m *Model) moveWordLeft() {
	if m.cursorCol == 0 {
		if m.cursorRow > 0 {
			m.cursorRow--
			m.cursorCol = len([]rune(m.lines[m.cursorRow]))
		}
		return
	}
	runes := []rune(m.lines[m.cursorRow])
	col := m.cursorCol
	for col > 0 && unicode.IsSpace(runes[col-1]) {
		col--
	}
	for col > 0 && !unicode.IsSpace(runes[col-1]) {
		col--
	}
	m.cursorCol = col
}

func (m *Model) moveWordRight() {
	runes := []rune(m.lines[m.cursorRow])
	col := m.cursorCol
	for col < len(runes) && !unicode.IsSpace(runes[col]) {
		col++
	}
	for col < len(runes) && unicode.IsSpace(runes[col]) {
		col++
	}
	m.cursorCol = col
	if col >= len(runes) && m.cursorRow < len(m.lines)-1 {
		m.cursorRow++
		m.cursorCol = 0
	}
}

func (m *Model) insertRune(r rune) {
	runes := []rune(m.lines[m.cursorRow])
	newRunes := make([]rune, len(runes)+1)
	copy(newRunes, runes[:m.cursorCol])
	newRunes[m.cursorCol] = r
	copy(newRunes[m.cursorCol+1:], runes[m.cursorCol:])
	m.lines[m.cursorRow] = string(newRunes)
	m.cursorCol++
	m.modified = true
}

func (m *Model) insertNewLine() {
	runes := []rune(m.lines[m.cursorRow])
	before := string(runes[:m.cursorCol])
	after := string(runes[m.cursorCol:])
	m.lines[m.cursorRow] = before
	newLines := make([]string, len(m.lines)+1)
	copy(newLines, m.lines[:m.cursorRow+1])
	newLines[m.cursorRow+1] = after
	copy(newLines[m.cursorRow+2:], m.lines[m.cursorRow+1:])
	m.lines = newLines
	m.cursorRow++
	m.cursorCol = 0
	m.modified = true
}

func (m *Model) deleteBackward() {
	if m.cursorCol > 0 {
		runes := []rune(m.lines[m.cursorRow])
		m.lines[m.cursorRow] = string(append(runes[:m.cursorCol-1], runes[m.cursorCol:]...))
		m.cursorCol--
	} else if m.cursorRow > 0 {
		prevLen := len([]rune(m.lines[m.cursorRow-1]))
		m.lines[m.cursorRow-1] += m.lines[m.cursorRow]
		m.lines = append(m.lines[:m.cursorRow], m.lines[m.cursorRow+1:]...)
		m.cursorRow--
		m.cursorCol = prevLen
	}
	m.modified = true
}

func (m *Model) deleteForward() {
	runes := []rune(m.lines[m.cursorRow])
	if m.cursorCol < len(runes) {
		m.lines[m.cursorRow] = string(append(runes[:m.cursorCol], runes[m.cursorCol+1:]...))
	} else if m.cursorRow < len(m.lines)-1 {
		m.lines[m.cursorRow] += m.lines[m.cursorRow+1]
		m.lines = append(m.lines[:m.cursorRow+1], m.lines[m.cursorRow+2:]...)
	}
	m.modified = true
}

func (m *Model) clampScroll() {
	innerH := m.height - 2
	if innerH < 1 {
		innerH = 1
	}
	vrows := m.buildVisualRows()
	curVis := m.cursorVisRow(vrows)

	if curVis < m.scrollVisRow {
		m.scrollVisRow = curVis
	}
	if curVis >= m.scrollVisRow+innerH {
		m.scrollVisRow = curVis - innerH + 1
	}
	// clamp to valid range
	if m.scrollVisRow < 0 {
		m.scrollVisRow = 0
	}
	maxScroll := len(vrows) - innerH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollVisRow > maxScroll {
		m.scrollVisRow = maxScroll
	}
}

// handleMouseClick moves the cursor to the clicked position (wrap-aware).
func (m *Model) handleMouseClick(localX, localY int) {
	lnW := m.lineNumWidth()
	// left border(1) + lineNum(lnW) + separator(1) = lnW+2
	contentX := localX - 1 - lnW - 1
	contentY := localY - 1 // top border

	if contentY < 0 {
		return
	}

	vrows := m.buildVisualRows()
	visIdx := m.scrollVisRow + contentY
	if visIdx >= len(vrows) {
		visIdx = len(vrows) - 1
	}
	if visIdx < 0 {
		return
	}

	vr := vrows[visIdx]
	m.cursorRow = vr.lineIdx

	// col is in expanded-column space; convert back to rune col
	expCol := vr.startCol + contentX
	if expCol < vr.startCol {
		expCol = vr.startCol
	}
	expLineLen := len([]rune(expandTabs(m.lines[m.cursorRow])))
	if expCol > expLineLen {
		expCol = expLineLen
	}
	// Don't go past the end of this visual row (except on last row of line)
	if vr.endCol < expLineLen && expCol > vr.endCol {
		expCol = vr.endCol
	}
	m.cursorCol = m.runeColFromExpanded(m.cursorRow, expCol)
}

func (m Model) highlightedLines() []string {
	if m.path == "" {
		return m.lines
	}

	content := strings.Join(m.lines, "\n")

	// Detect language by file extension
	lexer := lexers.Match(filepath.Base(m.path))
	if lexer == nil {
		lexer = lexers.Analyse(content)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return m.lines
	}

	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return m.lines
	}

	highlighted := buf.String()
	// Remove trailing newline added by formatter
	highlighted = strings.TrimRight(highlighted, "\n")
	result := strings.Split(highlighted, "\n")
	// Expand tabs in highlighted output so display widths are consistent
	for i, line := range result {
		result[i] = expandTabsAnsi(line)
	}
	return result
}

func (m Model) View() string {
	if m.width < 4 || m.height < 3 {
		return ""
	}

	innerH := m.height - 2 // subtract top and bottom border rows
	cw := m.contentWidth()
	lnW := m.lineNumWidth()

	hlLines := m.highlightedLines()
	vrows := m.buildVisualRows()
	totalVis := len(vrows)
	curVis := m.cursorVisRow(vrows)

	// Compute the focused border color
	borderColor := lipgloss.Color("240")
	if m.focused {
		borderColor = lipgloss.Color("99")
	}
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	// Top border: ╭──...──╮  (render as one string for correct width)
	topBorder := borderStyle.Render("╭" + strings.Repeat("─", m.width-2) + "╮")

	// Bottom border: ╰──...──╯
	botBorder := borderStyle.Render("╰" + strings.Repeat("─", m.width-2) + "╯")

	var screenRows []string
	screenRows = append(screenRows, topBorder)

	lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle := lipgloss.NewStyle().Background(lipgloss.Color("255")).Foreground(lipgloss.Color("0"))

	// Pre-render border chars as plain strings (no ANSI) to avoid width calculation issues
	vBar := "\x1b[38;5;240m│\x1b[0m"
	if m.focused {
		vBar = "\x1b[38;5;99m│\x1b[0m"
	}

	for row := 0; row < innerH; row++ {
		visIdx := m.scrollVisRow + row
		var lineNum string
		var cell string

		if visIdx < totalVis {
			vr := vrows[visIdx]
			// Line number: only on first visual row of a logical line
			isFirstRow := visIdx == 0 || vrows[visIdx-1].lineIdx != vr.lineIdx
			if isFirstRow {
				numStr := fmt.Sprintf("%*d", lnW, vr.lineIdx+1)
				lineNum = lineNumStyle.Render(numStr)
			} else {
				lineNum = strings.Repeat(" ", lnW)
			}
			// Ensure lineNum is exactly lnW visible chars
			lineNum = padToWidth(lineNum, lnW)

			// Get text segment from highlighted line
			hlLine := ""
			if vr.lineIdx < len(hlLines) {
				hlLine = hlLines[vr.lineIdx]
			}
			// extract the visual-row slice from the highlighted text
			plainLine := stripAnsi(hlLine)
			plainRunes := []rune(plainLine)
			startCol := vr.startCol
			endCol := vr.endCol
			if startCol > len(plainRunes) {
				startCol = len(plainRunes)
			}
			if endCol > len(plainRunes) {
				endCol = len(plainRunes)
			}
			segment := ansiSubstring(hlLine, startCol, endCol)

			// Render cursor if cursor is on this visual row
			if visIdx == curVis {
				segRunes := []rune(stripAnsi(segment))
				// cursorCol is rune-based; convert to expanded col for offset within visual row
				expCursorCol := m.expandedCol(m.cursorRow, m.cursorCol)
				cursorOffset := expCursorCol - vr.startCol
				if cursorOffset < 0 {
					cursorOffset = 0
				}
				// Build: before + cursor-char + after
				var cellBuf strings.Builder
				if cursorOffset > 0 {
					cellBuf.WriteString(ansiSubstring(hlLine, vr.startCol, vr.startCol+cursorOffset))
				}
				if cursorOffset < len(segRunes) {
					cursorCh := string(segRunes[cursorOffset])
					cellBuf.WriteString(cursorStyle.Render(cursorCh))
					// after cursor
					if cursorOffset+1 < len(segRunes) {
						cellBuf.WriteString(ansiSubstring(hlLine, vr.startCol+cursorOffset+1, endCol))
					}
				} else {
					// cursor at end of line: show space
					cellBuf.WriteString(cursorStyle.Render(" "))
				}
				segment = cellBuf.String()
			}

			cell = padToWidth(segment, cw)
		} else {
			// Empty row below content
			lineNum = strings.Repeat(" ", lnW)
			cell = strings.Repeat(" ", cw)
		}

		// Scrollbar character
		sb := m.scrollbarChar(row, innerH, totalVis)

		// Assemble row. Use raw ANSI strings to avoid lipgloss adding extra padding/resets
		// that mess up the width calculation.
		// Format: │ + lineNum(lnW chars) + │ + cell(cw chars) + sb(1 char) + │
		rowStr := vBar + lineNum + vBar + cell + sb + vBar

		screenRows = append(screenRows, rowStr)
	}

	screenRows = append(screenRows, botBorder)
	return strings.Join(screenRows, "\n")
}

// scrollbarChar returns a single-character scrollbar indicator for the given screen row.
func (m Model) scrollbarChar(screenRow, innerH, totalVis int) string {
	if totalVis <= innerH {
		return " " // no scrollbar needed
	}
	// Scrollbar thumb position and size
	thumbSize := innerH * innerH / totalVis
	if thumbSize < 1 {
		thumbSize = 1
	}
	thumbStart := m.scrollVisRow * innerH / totalVis
	thumbEnd := thumbStart + thumbSize

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	if screenRow >= thumbStart && screenRow < thumbEnd {
		return style.Foreground(lipgloss.Color("99")).Render("▓")
	}
	return style.Render("░")
}

func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	// ANSI-safe truncation: walk rune by rune, skip escape sequences,
	// count visible width, stop when we reach the limit.
	var out strings.Builder
	visW := 0
	r := []rune(s)
	hadEscape := false
	i := 0
	for i < len(r) {
		if r[i] == '\x1b' && i+1 < len(r) && r[i+1] == '[' {
			start := i
			i += 2
			for i < len(r) && r[i] != 'm' {
				i++
			}
			if i < len(r) {
				i++
			}
			out.WriteString(string(r[start:i]))
			hadEscape = true
			continue
		}
		// Measure this rune's display width
		rw := lipgloss.Width(string(r[i]))
		if visW+rw > width {
			break
		}
		out.WriteRune(r[i])
		visW += rw
		i++
	}
	if hadEscape {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

// padToWidth truncates s if wider than width, or pads with spaces on the right to reach exactly width.
func padToWidth(s string, width int) string {
	s = truncateToWidth(s, width)
	pad := width - lipgloss.Width(s)
	if pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// stripAnsi removes ANSI escape sequences from a string, returning plain text.
func stripAnsi(s string) string {
	var out strings.Builder
	i := 0
	r := []rune(s)
	for i < len(r) {
		if r[i] == '\x1b' && i+1 < len(r) && r[i+1] == '[' {
			// skip until 'm'
			i += 2
			for i < len(r) && r[i] != 'm' {
				i++
			}
			i++ // skip 'm'
			continue
		}
		out.WriteRune(r[i])
		i++
	}
	return out.String()
}

// ansiSubstring extracts visible runes [startVis, endVis) from an ANSI-colored string,
// preserving the surrounding ANSI escape codes so colors are not lost.
// It re-emits all escape sequences encountered before the first visible rune,
// emits visible runes in the range, and appends a reset at the end if any escapes were seen.
func ansiSubstring(s string, startVis, endVis int) string {
	if startVis >= endVis {
		return ""
	}
	var out strings.Builder
	var pendingEscapes strings.Builder // accumulate escapes before we start emitting runes
	r := []rune(s)
	vis := 0 // visible rune counter
	emitting := false
	hadEscape := false

	i := 0
	for i < len(r) {
		// Parse ANSI escape sequence
		if r[i] == '\x1b' && i+1 < len(r) && r[i+1] == '[' {
			start := i
			i += 2
			for i < len(r) && r[i] != 'm' {
				i++
			}
			if i < len(r) {
				i++ // include 'm'
			}
			esc := string(r[start:i])
			hadEscape = true
			if emitting {
				out.WriteString(esc)
			} else {
				pendingEscapes.WriteString(esc)
			}
			continue
		}

		if vis >= endVis {
			break
		}

		if vis == startVis {
			// Flush pending escapes before first visible rune
			out.WriteString(pendingEscapes.String())
			pendingEscapes.Reset()
			emitting = true
		}

		if emitting {
			out.WriteRune(r[i])
		}

		vis++
		i++
	}

	// Flush pending escapes even if we never started emitting (edge case)
	if !emitting {
		out.WriteString(pendingEscapes.String())
	}

	// Reset colors at the end to avoid bleed
	if hadEscape {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// InsertChars inserts the given string at the current cursor position,
// regardless of focus state. Used for toolbar button clicks.
func (m *Model) InsertChars(s string) tea.Cmd {
	for _, r := range []rune(s) {
		m.insertRune(r)
	}
	m.clampScroll()
	return m.changedCmd()
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
