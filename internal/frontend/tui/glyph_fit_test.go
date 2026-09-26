package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"reasonix/internal/contract/eventwire"
)

// conhostCJKCells is what a legacy conhost window with the default Chinese
// console font measured through WriteConsoleW + cursor read-back (#10540).
var conhostCJKCells = map[rune]int{
	'·': 2, '…': 2, '“': 2, '”': 2, '‘': 2, '’': 2, '—': 2, '–': 2,
	'●': 2, '○': 2, '←': 2, '↑': 2, '→': 2, '↓': 2, '※': 2, '★': 2,
	'■': 2, '◆': 2, 'α': 2, 'β': 2, 'Ω': 2, 'Ж': 2, 'ж': 2, '°': 2,
	'±': 2, '×': 2, '€': 2,
	'─': 1, '│': 1, '┌': 1, '•': 1, '█': 1, 'ü': 1, 'é': 1, '™': 1,
	'￩': 1, '￪': 1, '￫': 1, '￬': 1, '￭': 1, '￮': 1, '￨': 1,
}

// conhostBestFit is the same machine's WideCharToMultiByte(20127) answer.
var conhostBestFit = map[rune]rune{
	'·': '.', '…': '.', '“': '"', '”': '"', '‘': '\'', '’': '\'', '—': '-', '–': '-',
	'•': '.', 'ü': 'u', 'é': 'e', '™': 'T',
}

func conhostTableFit() *glyphFit {
	return newGlyphFit(conhostCellsOf, func(r rune) rune {
		if c, ok := conhostBestFit[r]; ok {
			return c
		}
		return '?'
	})
}

func conhostCellsOf(r rune) int {
	if w, ok := conhostCJKCells[r]; ok {
		return w
	}
	if r < 0x80 {
		return 1
	}
	return ansi.StringWidth(string(r))
}

func conhostRowCells(row string) int {
	n := 0
	for _, r := range ansi.Strip(row) {
		n += conhostCellsOf(r)
	}
	return n
}

func TestFrameFitsConhostMeasuredWidths(t *testing.T) {
	const cols = 120
	m, _ := testModel(t)
	m.glyphs = conhostTableFit()
	m.Update(tea.WindowSizeMsg{Width: cols, Height: 30})
	m.tr.AddUser("go")
	apply(m, eventwire.Event{Kind: "turn_started"})
	apply(m, eventwire.Event{Kind: "text", Text: strings.Repeat("“引号”…—→·", 12) + "\n"})
	apply(m, eventwire.Event{Kind: "message", Text: ""}, eventwire.Event{Kind: "turn_done"})

	frame := m.View().Content
	for i, row := range strings.Split(frame, "\n") {
		if got := conhostRowCells(row); got > cols {
			t.Fatalf("row %d draws %d cells on conhost, terminal has %d:\n%s", i, got, cols, ansi.Strip(row))
		}
	}
	if !strings.Contains(ansi.Strip(frame), `"引号".-￫.`) {
		t.Fatalf("the answer should carry the measured stand-ins:\n%s", ansi.Strip(frame))
	}
}

func TestGlyphFitLeavesEscapePayloadsAlone(t *testing.T) {
	link := ansi.SetHyperlink("https://example.com/·") + "a·b" + ansi.ResetHyperlink()
	got := conhostTableFit().apply(link)
	want := ansi.SetHyperlink("https://example.com/·") + "a.b" + ansi.ResetHyperlink()
	if got != want {
		t.Fatalf("apply(%q) = %q, want %q", link, got, want)
	}
}

func TestGlyphFitKeepsRunesTheConsoleAgreesOn(t *testing.T) {
	in := "│─┌ 中文 • ü"
	if got := conhostTableFit().apply(in); got != in {
		t.Fatalf("apply(%q) = %q, want it unchanged", in, got)
	}
}
