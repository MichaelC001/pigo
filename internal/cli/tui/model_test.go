package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/smallnest/pigo/internal/cli/ui"
)

// TestModelQuitKeys verifies the root model returns tea.Quit on the standard
// exit keys (Ctrl+C / Ctrl+D), which is how Bubble Tea tears down the program
// and restores the terminal from the alt-screen.
func TestModelQuitKeys(t *testing.T) {
	for _, key := range []string{"ctrl+c", "ctrl+d"} {
		m := NewModel(Options{})
		got, cmd := m.Update(keyPress(key))
		if cmd == nil {
			t.Fatalf("%s: expected a quit command, got nil", key)
		}
		if msg := cmd(); msg != (tea.QuitMsg{}) {
			t.Errorf("%s: cmd produced %T, want tea.QuitMsg", key, msg)
		}
		if !got.(Model).quitting {
			t.Errorf("%s: model should be marked quitting", key)
		}
	}
}

// TestModelViewShell verifies the empty shell renders on the alt-screen and,
// once a size is known, occupies the full terminal height (empty transcript rows
// + status bar + input line), with the real status bar (#386) painting its
// fields.
func TestModelViewShell(t *testing.T) {
	m := NewModel(Options{Model: "test-model"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	view := next.View()
	if !view.AltScreen {
		t.Error("View should request the alt-screen")
	}
	if got := strings.Count(view.Content, "\n"); got != 9 {
		t.Errorf("newline count = %d, want 9 (10 rows)", got)
	}
	if !strings.Contains(view.Content, "test-model") {
		t.Errorf("status bar model field missing from view: %q", view.Content)
	}
}

// TestModelNewlineKeys verifies that Shift+Enter inserts a line break at the
// cursor and preserves the already-typed text, rather than submitting. Plain
// Enter still submits, so it does not leave a newline in the buffer. Shift+Enter
// is the primary newline key (reported distinctly by terminals speaking the
// Kitty disambiguate protocol, which Bubble Tea enables by default); Ctrl+J and
// Alt+Enter are fallbacks for terminals that collapse Shift+Enter to a bare CR.
func TestModelNewlineKeys(t *testing.T) {
	var mm tea.Model = NewModel(Options{})
	mm, _ = mm.Update(tea.WindowSizeMsg{Width: 60, Height: 10})
	for _, r := range "abc" {
		mm, _ = mm.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	mm, _ = mm.Update(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	for _, r := range "def" {
		mm, _ = mm.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if got := mm.(Model).input.Value(); got != "abc\ndef" {
		t.Errorf("input = %q, want %q", got, "abc\ndef")
	}
}

// TestModelSelectionCopy drives a mouse selection over a transcript line and
// asserts Ctrl+C copies the selected text (over OSC52) and clears the selection.
func TestModelSelectionCopy(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 40, Height: 12})
	m.transcript.addUser("hello world")

	// Locate the rendered screen cell where the text begins so the test does not
	// hard-code the transcript's bottom-stick row.
	rows := strings.Split(m.renderContent(), "\n")
	y, x := -1, -1
	for i, r := range rows {
		plain := stripANSI(r)
		if idx := strings.Index(plain, "hello world"); idx >= 0 {
			y = i
			x = ui.Width(plain[:idx])
			break
		}
	}
	if y < 0 {
		t.Fatal("rendered screen did not contain the transcript text")
	}

	// Select exactly "hello world" (11 display cells) on that row.
	m.sel = selection{active: true, anchor: point{x, y}, cursor: point{x + 11, y}}
	next, cmd := m.Update(keyPress("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c with a selection should emit a clipboard command")
	}
	if got := fmt.Sprintf("%s", cmd()); got != "hello world" {
		t.Errorf("copied %q, want %q", got, "hello world")
	}
	if !next.(Model).sel.empty() {
		t.Error("selection should be cleared after Ctrl+C copies it")
	}
}

// TestModelCtrlCFallsBackToQuit verifies Ctrl+C with no selection keeps its
// interrupt/quit role (idle → quit).
func TestModelCtrlCFallsBackToQuit(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 40, Height: 12})
	next, cmd := m.Update(keyPress("ctrl+c"))
	if cmd == nil || cmd() != (tea.QuitMsg{}) {
		t.Fatal("ctrl+c without a selection should quit when idle")
	}
	if !next.(Model).quitting {
		t.Error("model should be marked quitting")
	}
}

// keyPress builds a KeyPressMsg matching String()==s for the simple keys used
// in these tests (ctrl+<letter>).
func keyPress(s string) tea.KeyPressMsg {
	switch s {
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+d":
		return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	case "ctrl+y":
		return tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl}
	case "super+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModSuper}
	case "super+v":
		return tea.KeyPressMsg{Code: 'v', Mod: tea.ModSuper}
	default:
		return tea.KeyPressMsg{}
	}
}

// TestModelPasteInsertsIntoInput verifies a bracketed paste (tea.PasteMsg) is
// routed into the editor and inserted whole, including embedded newlines.
func TestModelPasteInsertsIntoInput(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 40, Height: 12})
	m = apply(t, m, tea.PasteMsg{Content: "hello\nworld"})
	if got := m.input.Value(); got != "hello\nworld" {
		t.Errorf("input after paste = %q, want %q", got, "hello\nworld")
	}
}

// TestModelClipboardReadInsertsIntoInput verifies an OSC52 clipboard read reply
// (tea.ClipboardMsg, the response to Ctrl+V) is inserted into the editor.
func TestModelClipboardReadInsertsIntoInput(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 40, Height: 12})
	m = apply(t, m, tea.ClipboardMsg{Content: "pasted"})
	if got := m.input.Value(); got != "pasted" {
		t.Errorf("input after clipboard read = %q, want %q", got, "pasted")
	}
}

// TestModelCopyToClipboard verifies Ctrl+Y emits an OSC52 SetClipboard command
// carrying the current buffer, and is a no-op on an empty buffer.
func TestModelCopyToClipboard(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 40, Height: 12})

	// Empty buffer: no command.
	if _, cmd := m.Update(keyPress("ctrl+y")); cmd != nil {
		t.Errorf("ctrl+y on empty buffer should be a no-op, got a command")
	}

	m = apply(t, m, tea.PasteMsg{Content: "copy me"})
	_, cmd := m.Update(keyPress("ctrl+y"))
	if cmd == nil {
		t.Fatal("ctrl+y with content should emit a clipboard command")
	}
	// SetClipboard yields an unexported string-underlying message; format it to
	// read its payload without depending on the tea-internal type.
	if got := fmt.Sprintf("%s", cmd()); got != "copy me" {
		t.Errorf("clipboard command carried %q, want %q", got, "copy me")
	}
}

// TestModelSuperCCopiesSelection verifies Cmd+C (super+c) copies the mouse
// selection just like Ctrl+C, but with an empty buffer and no selection it is a
// no-op rather than quitting — Cmd+C is "copy" on macOS, never interrupt/quit.
func TestModelSuperCCopiesSelection(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 40, Height: 12})

	// No selection, empty buffer: no-op, and never a quit command.
	if next, cmd := m.Update(keyPress("super+c")); cmd != nil {
		t.Errorf("super+c with nothing to copy should be a no-op, got a command")
	} else if next.(Model).quitting {
		t.Error("super+c must never quit")
	}

	// No selection, non-empty buffer: copies the whole buffer.
	m = apply(t, m, tea.PasteMsg{Content: "buffer text"})
	if _, cmd := m.Update(keyPress("super+c")); cmd == nil {
		t.Fatal("super+c with buffer content should emit a clipboard command")
	} else if got := fmt.Sprintf("%s", cmd()); got != "buffer text" {
		t.Errorf("super+c copied %q, want %q", got, "buffer text")
	}
}

// TestModelSuperVPastes verifies Cmd+V (super+v) requests the clipboard over
// OSC52 when idle, like Ctrl+V.
func TestModelSuperVPastes(t *testing.T) {
	m := apply(t, NewModel(Options{}), tea.WindowSizeMsg{Width: 40, Height: 12})
	if _, cmd := m.Update(keyPress("super+v")); cmd == nil {
		t.Fatal("super+v should emit a clipboard read command when idle")
	}
}
