package tui

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/smallnest/pigo/internal/agentcore"
	"github.com/smallnest/pigo/internal/cli/ui"
)

// ansiRE strips SGR escape sequences so tests can inspect the raw text the
// transcript stored, independent of the theme's coloring.
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// apply runs one Update tick and returns the concrete Model, failing on an
// unexpected model type. It keeps the streaming tests terse.
func apply(t *testing.T, m tea.Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", next)
	}
	return got
}

// TestTranscriptStreamingConcat feeds a run of text deltas then a turn end and
// asserts the assistant block accumulates the deltas in order and the joined
// text is rendered in the View.
func TestTranscriptStreamingConcat(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 40, Height: 12})

	m = apply(t, m, textDeltaMsg{delta: "Hello "})
	m = apply(t, m, textDeltaMsg{delta: "world"})
	m = apply(t, m, turnEndMsg{msg: agentcore.AssistantMessage{
		Content: agentcore.ContentList{agentcore.NewTextContent("Hello world")},
	}})

	if n := len(m.transcript.blocks); n != 1 {
		t.Fatalf("block count = %d, want 1 assistant block", n)
	}
	if got := m.transcript.blocks[0]; got.role != roleAssistant || got.text != "Hello world" {
		t.Errorf("assistant block = %+v, want role assistant text %q", got, "Hello world")
	}
	if content := stripANSI(m.View().Content); !strings.Contains(content, "Hello world") {
		t.Errorf("rendered View missing streamed text; got:\n%s", content)
	}
	// The turn was finalized, so a fresh delta starts a NEW assistant block.
	if m.transcript.activeAssistant != -1 {
		t.Errorf("activeAssistant = %d after turn end, want -1", m.transcript.activeAssistant)
	}
}

// TestTranscriptAutoStick verifies the stick-to-bottom rule: while the viewport
// is at the bottom, new content keeps it pinned there; once the user scrolls up,
// new content no longer forces a jump to the bottom.
func TestTranscriptAutoStick(t *testing.T) {
	tr := newTranscript(DefaultTheme())
	tr.setSize(20, 3) // 3 visible rows

	for i := 0; i < 6; i++ {
		tr.addUser("line")
	}
	if !tr.vp.AtBottom() {
		t.Fatal("transcript should stick to the bottom while at the bottom")
	}

	// More content while pinned keeps it pinned.
	tr.addUser("more")
	if !tr.vp.AtBottom() {
		t.Fatal("new content should keep a bottom-pinned transcript at the bottom")
	}

	// Simulate a user scroll-up through the viewport's key handling.
	tr.update(tea.KeyPressMsg{Code: tea.KeyUp})
	if tr.vp.AtBottom() {
		t.Fatal("scrolling up should move the viewport off the bottom")
	}

	// Content arriving while scrolled up must NOT yank the view back to bottom.
	tr.addUser("streamed while reading history")
	if tr.vp.AtBottom() {
		t.Error("auto-stick should stay paused after the user scrolls up")
	}
}

// TestTranscriptCJKWrap feeds a long CJK line into a narrow transcript and
// asserts every wrapped line fits the width in display columns (not bytes) and
// that no rune was dropped or split.
func TestTranscriptCJKWrap(t *testing.T) {
	const width = 10
	tr := newTranscript(DefaultTheme())
	tr.setSize(width, 20)

	line := strings.Repeat("你好世界", 5) // 20 CJK runes = 40 display columns
	tr.addUser(line)

	content := tr.vp.GetContent()
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the CJK line to wrap onto multiple rows, got %d line(s)", len(lines))
	}
	for i, ln := range lines {
		if w := ui.Width(ln); w > width {
			t.Errorf("wrapped line %d width = %d columns, want <= %d: %q", i, w, width, stripANSI(ln))
		}
	}
	// No rune was cut or dropped: every source rune survives the wrap.
	if got := strings.Count(stripANSI(content), "你"); got != 5 {
		t.Errorf("counted %d 你 runes after wrap, want 5", got)
	}
}

// TestTranscriptScrollbar verifies the scrollbar policy: the gutter is hidden
// while the content fits (nothing to scroll) and appears only once the content
// overflows. When overflowing, a rounded pill thumb (body "█" with half-block
// caps "▄"/"▀") sits alongside the thin groove "│".
func TestTranscriptScrollbar(t *testing.T) {
	tr := newTranscript(DefaultTheme())
	tr.setSize(20, 4) // 4 visible rows

	// Two short lines fit in 4 rows: no scrollbar at all — no thumb, no groove.
	tr.addUser("one")
	tr.addUser("two")
	if tr.overflowing() {
		t.Fatal("transcript should not overflow while content fits")
	}
	fit := stripANSI(tr.view())
	if strings.ContainsAny(fit, "█▄▀│") {
		t.Errorf("expected no scrollbar glyphs while content fits; got:\n%q", fit)
	}

	// Enough lines to exceed 4 rows: now it overflows, thumb shrinks and the
	// groove appears.
	for i := 0; i < 10; i++ {
		tr.addUser("line")
	}
	if !tr.overflowing() {
		t.Fatal("transcript should overflow once content exceeds the viewport")
	}
	view := stripANSI(tr.view())
	if !strings.Contains(view, "▄") || !strings.Contains(view, "▀") {
		t.Errorf("expected a rounded pill thumb (▄ top, ▀ bottom) while overflowing; got:\n%q", view)
	}
	if !strings.Contains(view, "│") {
		t.Errorf("expected a groove │ while overflowing; got:\n%q", view)
	}
	if strings.Contains(view, "░") {
		t.Errorf("scrollbar no longer uses the shaded track ░; got:\n%q", view)
	}
}

// TestTranscriptScrollToRow checks the click/drag mapping: pressing the top of
// the gutter scrolls to the top, the bottom scrolls to the bottom, and it is a
// no-op when the content fits.
func TestTranscriptScrollToRow(t *testing.T) {
	tr := newTranscript(DefaultTheme())
	tr.setSize(20, 4)

	// Content fits: dragging must not move a non-scrollable viewport.
	tr.addUser("only line")
	tr.scrollToRow(3)
	if tr.vp.YOffset() != 0 {
		t.Errorf("scrollToRow on non-overflowing viewport moved offset to %d, want 0", tr.vp.YOffset())
	}

	for i := 0; i < 20; i++ {
		tr.addUser("line")
	}
	if !tr.overflowing() {
		t.Fatal("expected overflow after filling the transcript")
	}

	tr.scrollToRow(0)
	if !tr.vp.AtTop() {
		t.Errorf("dragging to row 0 should scroll to the top; YOffset=%d", tr.vp.YOffset())
	}

	tr.scrollToRow(tr.viewportHeight() - 1)
	if !tr.vp.AtBottom() {
		t.Errorf("dragging to the last row should scroll to the bottom; YOffset=%d", tr.vp.YOffset())
	}
}

// TestModelScrollbarDrag drives the model with mouse press/motion/release on the
// scrollbar column and asserts the drag state toggles and the viewport scrolls.
func TestModelScrollbarDrag(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 30, Height: 8})
	for i := 0; i < 40; i++ {
		m.transcript.addUser("line")
	}
	if !m.transcript.overflowing() {
		t.Fatal("expected the transcript to overflow")
	}

	col := m.width - 1
	// Press at the top of the gutter: drag begins and the view jumps to the top.
	m = apply(t, m, tea.MouseClickMsg{X: col, Y: 0, Button: tea.MouseLeft})
	if !m.draggingScrollbar {
		t.Fatal("left press on the scrollbar column should start dragging")
	}
	if !m.transcript.vp.AtTop() {
		t.Errorf("press at row 0 should scroll to top; YOffset=%d", m.transcript.vp.YOffset())
	}

	// Motion to the bottom row while held drags the thumb down.
	m = apply(t, m, tea.MouseMotionMsg{X: col, Y: m.transcript.viewportHeight() - 1, Button: tea.MouseLeft})
	if !m.transcript.vp.AtBottom() {
		t.Errorf("motion to the last row while dragging should scroll to bottom; YOffset=%d", m.transcript.vp.YOffset())
	}

	// Release ends the drag.
	m = apply(t, m, tea.MouseReleaseMsg{X: col, Y: 3, Button: tea.MouseLeft})
	if m.draggingScrollbar {
		t.Error("release should end the scrollbar drag")
	}

	// A press away from the gutter column must not start a drag.
	m = apply(t, m, tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if m.draggingScrollbar {
		t.Error("press off the scrollbar column should not start dragging")
	}
}
