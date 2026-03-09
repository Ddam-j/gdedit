package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"gdedit/internal/agent"
	"gdedit/internal/memo"
	"github.com/gdamore/tcell/v2"
)

type stubAgentExecutor struct {
	response  agent.Response
	err       error
	request   agent.Request
	onExecute func()
}

func (s *stubAgentExecutor) Execute(_ context.Context, req agent.Request) (agent.Response, error) {
	s.request = req
	if s.onExecute != nil {
		s.onExecute()
	}
	if s.err != nil {
		return agent.Response{}, s.err
	}
	return s.response, nil
}

func withClipboardMocks(read func() (string, error), write func(string) error, run func()) {
	originalRead := clipboardRead
	originalWrite := clipboardWrite
	clipboardRead = read
	clipboardWrite = write
	defer func() {
		clipboardRead = originalRead
		clipboardWrite = originalWrite
	}()
	run()
}

func TestCtrlQOpensQuitConfirmation(t *testing.T) {
	app := New()

	quit := app.handleKey(tcell.NewEventKey(tcell.KeyCtrlQ, 0, 0))
	if quit {
		t.Fatal("expected Ctrl+Q to open confirmation instead of quitting immediately")
	}
	if !app.quitConfirm {
		t.Fatal("expected quit confirmation to be visible")
	}
}

func TestNewIncludesRepresentativeSampleTabs(t *testing.T) {
	app := New()
	want := []string{"main.go", "worker.py", "panel.ts", "config.yaml"}
	if len(app.tabs) != len(want) {
		t.Fatalf("unexpected tab count: got %d want %d", len(app.tabs), len(want))
	}
	for i, title := range want {
		if app.tabs[i].Title != title {
			t.Fatalf("unexpected tab %d title: got %q want %q", i, app.tabs[i].Title, title)
		}
	}
}

func TestNewScratchStartsWithUntitledBuffer(t *testing.T) {
	app := NewScratch()
	if len(app.tabs) != 1 {
		t.Fatalf("unexpected tab count: got %d want 1", len(app.tabs))
	}
	if app.tabs[0].Title != "untitled" {
		t.Fatalf("unexpected scratch tab title: %q", app.tabs[0].Title)
	}
	if len(app.tabs[0].Content) != 1 || app.tabs[0].Content[0] != "" {
		t.Fatalf("unexpected scratch content: %#v", app.tabs[0].Content)
	}
}

func TestSelectionStateIsScopedPerTab(t *testing.T) {
	app := New()
	app.activeTab = 0
	app.cursorY = 4
	app.cursorX = 0
	app.handleEditorKey(tcell.NewEventKey(tcell.KeyF2, 0, 0))
	if got := app.currentScope(); got != "selection:L4:C1-L6:C3" {
		t.Fatalf("unexpected initial tab selection: %s", got)
	}

	app.nextTab()
	if got := app.currentScope(); got != "caret:L1:C1" {
		t.Fatalf("selection leaked into next tab: %s", got)
	}

	app.cursorY = 4
	app.cursorX = 0
	app.handleEditorKey(tcell.NewEventKey(tcell.KeyF2, 0, 0))
	if got := app.currentScope(); got != "selection:L4:C1-L5:C33" {
		t.Fatalf("unexpected second tab selection: %s", got)
	}

	app.prevTab()
	if got := app.currentScope(); got != "selection:L4:C1-L6:C3" {
		t.Fatalf("first tab selection was not restored: %s", got)
	}
}

func TestTabInsertsTabWhenNoSelection(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 0

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyTab, 0, 0))
	if quit {
		t.Fatal("did not expect Tab to quit")
	}
	if got := app.tabs[app.activeTab].Content[3]; got != "\t\tif input == \"agent\" {" {
		t.Fatalf("unexpected line after literal tab insert: %q", got)
	}
	if app.cursorX != 1 {
		t.Fatalf("expected cursor to move after tab insert, got %d", app.cursorX)
	}
}

func TestAltNumberSetsIndentWidth(t *testing.T) {
	app := New()

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+2 to quit")
	}
	if app.indentWidth != 2 {
		t.Fatalf("expected indent width 2, got %d", app.indentWidth)
	}

	app.cursorY = 3
	app.cursorX = 0
	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyTab, 0, 0))
	if quit {
		t.Fatal("did not expect Tab to quit after width change")
	}
	if got := app.tabs[app.activeTab].Content[3]; got != "\t\tif input == \"agent\" {" {
		t.Fatalf("unexpected line after width change tab insert: %q", got)
	}
}

func TestAltZeroSetsTabIndentModeForSelection(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha", "beta", "gamma"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+0 to quit")
	}
	if app.indentWidth != 0 {
		t.Fatalf("expected indent width 0 for tab mode, got %d", app.indentWidth)
	}

	app.cursorY = 0
	app.cursorX = 0
	app.handleEditorKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift))
	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyTab, 0, 0))
	if quit {
		t.Fatal("did not expect Tab to quit in tab indent mode")
	}
	if got := app.tabs[app.activeTab].Content[0]; got != "\talpha" {
		t.Fatalf("unexpected first tab-indented line: %q", got)
	}
	if got := app.tabs[app.activeTab].Content[1]; got != "\tbeta" {
		t.Fatalf("unexpected second tab-indented line: %q", got)
	}
}

func TestShiftArrowCreatesCharacterSelectionAndCtrlCPastesAtCaret(t *testing.T) {
	withClipboardMocks(
		func() (string, error) { return "", nil },
		func(text string) error { return nil },
		func() {
			app := New()
			app.activeTab = 3
			app.tabs[app.activeTab] = Tab{
				Title:            "flat.txt",
				Content:          []string{"alpha"},
				Locked:           map[int]bool{},
				SelectionAnchorY: -1,
			}
			app.loadCurrentTabState()
			app.cursorY = 0
			app.cursorX = 1

			quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
			if quit {
				t.Fatal("did not expect Shift+Right to quit")
			}
			quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
			if quit {
				t.Fatal("did not expect second Shift+Right to quit")
			}
			if got := app.selectedText(); got != "lp" {
				t.Fatalf("unexpected selected text: %q", got)
			}

			quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+C to quit")
			}
			if app.clipboard != "lp" {
				t.Fatalf("unexpected clipboard: %q", app.clipboard)
			}

			app.clearSelection("Selection cleared.")
			app.cursorX = 5
			quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlV, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+V to quit")
			}
			if got := app.tabs[app.activeTab].Content[0]; got != "alphalp" {
				t.Fatalf("unexpected pasted line: %q", got)
			}
		},
	)
}

func TestMultilinePasteCreatesRealLines(t *testing.T) {
	withClipboardMocks(
		func() (string, error) { return "X\nY", nil },
		func(text string) error { return nil },
		func() {
			app := New()
			app.activeTab = 3
			app.tabs[app.activeTab] = Tab{
				Title:            "flat.txt",
				Content:          []string{"alpha"},
				Locked:           map[int]bool{},
				SelectionAnchorY: -1,
			}
			app.loadCurrentTabState()
			app.cursorY = 0
			app.cursorX = 2

			quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlV, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+V to quit")
			}
			if len(app.tabs[app.activeTab].Content) != 2 {
				t.Fatalf("expected 2 lines after multiline paste, got %d", len(app.tabs[app.activeTab].Content))
			}
			if got := app.tabs[app.activeTab].Content[0]; got != "alX" {
				t.Fatalf("unexpected first pasted line: %q", got)
			}
			if got := app.tabs[app.activeTab].Content[1]; got != "Ypha" {
				t.Fatalf("unexpected second pasted line: %q", got)
			}
		},
	)
}

func TestCopyFallsBackWhenSystemClipboardUnavailable(t *testing.T) {
	withClipboardMocks(
		func() (string, error) { return "", nil },
		func(text string) error { return errors.New("clipboard unavailable") },
		func() {
			app := New()
			app.activeTab = 3
			app.tabs[app.activeTab] = Tab{
				Title:            "flat.txt",
				Content:          []string{"alpha"},
				Locked:           map[int]bool{},
				SelectionAnchorY: -1,
			}
			app.loadCurrentTabState()
			app.cursorY = 0
			app.cursorX = 1
			_ = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
			_ = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
			_ = app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
			if app.clipboard != "lp" {
				t.Fatalf("unexpected internal clipboard after fallback copy: %q", app.clipboard)
			}
		},
	)
}

func TestCtrlASelectsWholeDocumentAndAllowsReplace(t *testing.T) {
	withClipboardMocks(
		func() (string, error) { return "REPLACED", nil },
		func(text string) error { return nil },
		func() {
			app := New()
			quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlA, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+A to quit")
			}
			if !app.hasSelection() {
				t.Fatal("expected Ctrl+A to select the document")
			}
			quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlV, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+V after Ctrl+A to quit")
			}
			if len(app.tabs[app.activeTab].Content) != 1 {
				t.Fatalf("expected whole document replacement to collapse to one line, got %d", len(app.tabs[app.activeTab].Content))
			}
			if got := app.tabs[app.activeTab].Content[0]; got != "REPLACED" {
				t.Fatalf("unexpected document after whole replace: %q", got)
			}
		},
	)
}

func TestCtrlXCutRemovesSelectionAndStoresClipboard(t *testing.T) {
	withClipboardMocks(
		func() (string, error) { return "", nil },
		func(text string) error { return nil },
		func() {
			app := New()
			app.activeTab = 3
			app.tabs[app.activeTab] = Tab{
				Title:            "flat.txt",
				Content:          []string{"alpha"},
				Locked:           map[int]bool{},
				SelectionAnchorY: -1,
			}
			app.loadCurrentTabState()
			app.cursorY = 0
			app.cursorX = 1
			_ = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
			_ = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))

			quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlX, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+X to quit")
			}
			if app.clipboard != "lp" {
				t.Fatalf("unexpected clipboard after cut: %q", app.clipboard)
			}
			if got := app.tabs[app.activeTab].Content[0]; got != "aha" {
				t.Fatalf("unexpected line after cut: %q", got)
			}
		},
	)
}

func TestCtrlXCutSupportsMultilineSelection(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha", "beta"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.setSelectionAnchor(2, 0)
	app.cursorY = 1
	app.cursorX = 2

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlX, 0, 0))
	if quit {
		t.Fatal("did not expect multiline Ctrl+X to quit")
	}
	if app.clipboard != "pha\nbe" {
		t.Fatalf("unexpected multiline clipboard: %q", app.clipboard)
	}
	if len(app.tabs[app.activeTab].Content) != 1 || app.tabs[app.activeTab].Content[0] != "alta" {
		t.Fatalf("unexpected content after multiline cut: %#v", app.tabs[app.activeTab].Content)
	}
}

func TestHomeAndEndMoveCaretAndExpandSelection(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 3

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyHome, 0, 0))
	if quit {
		t.Fatal("did not expect Home to quit")
	}
	if app.cursorX != 0 {
		t.Fatalf("expected Home to move caret to column 0, got %d", app.cursorX)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModShift))
	if quit {
		t.Fatal("did not expect Shift+End to quit")
	}
	if !app.hasSelection() {
		t.Fatal("expected Shift+End to create selection")
	}
	if got := app.selectedText(); got != "\tif input == \"agent\" {" {
		t.Fatalf("unexpected selected text after Shift+End: %q", got)
	}
}

func TestPageUpAndPageDownMoveCaretAndSelection(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12", "l13", "l14", "l15"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 10
	app.cursorX = 1

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyPgUp, 0, 0))
	if quit {
		t.Fatal("did not expect PageUp to quit")
	}
	if app.cursorY != 0 {
		t.Fatalf("expected PageUp to clamp to top, got %d", app.cursorY)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModShift))
	if quit {
		t.Fatal("did not expect Shift+PageDown to quit")
	}
	if !app.hasSelection() {
		t.Fatal("expected Shift+PageDown to create selection")
	}
	if got := app.currentScope(); got != "selection:L1:C2-L13:C2" {
		t.Fatalf("unexpected scope after Shift+PageDown: %s", got)
	}
}

func TestAltPageDownAlsoExpandsSelection(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10", "l11", "l12", "l13", "l14", "l15"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 0
	app.cursorX = 1

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+PageDown to quit")
	}
	if !app.hasSelection() {
		t.Fatal("expected Alt+PageDown to create selection")
	}
	if got := app.currentScope(); got != "selection:L1:C2-L13:C2" {
		t.Fatalf("unexpected scope after Alt+PageDown: %s", got)
	}
}

func TestAltAndShiftAltAlsoExpandSelectionOnLineKeys(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 1

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+Right to quit")
	}
	if !app.hasSelection() {
		t.Fatal("expected Alt+Right to create selection")
	}
	if got := app.selectedText(); got != "i" {
		t.Fatalf("unexpected selected text after Alt+Right: %q", got)
	}

	app.clearSelection("Selection cleared.")
	app.cursorX = 1
	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModShift|tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Shift+Alt+End to quit")
	}
	if !app.hasSelection() {
		t.Fatal("expected Shift+Alt+End to create selection")
	}
	if got := app.selectedText(); got != "if input == \"agent\" {" {
		t.Fatalf("unexpected selected text after Shift+Alt+End: %q", got)
	}
}

func TestUnhandledAltRuneDoesNotInsertText(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 0
	original := app.tabs[app.activeTab].Content[3]

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+x to quit")
	}
	if got := app.tabs[app.activeTab].Content[3]; got != original {
		t.Fatalf("expected Alt+x to leave line unchanged, got %q", got)
	}
}

func TestCtrlAndCtrlAltArrowMoveByWord(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha beta_gamma delta"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 0
	app.cursorX = 0

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect Ctrl+Right to quit")
	}
	if app.cursorX != 6 {
		t.Fatalf("expected Ctrl+Right to jump to next word, got %d", app.cursorX)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl|tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Ctrl+Alt+Right to quit")
	}
	if app.cursorX != 17 {
		t.Fatalf("expected Ctrl+Alt+Right to jump by word, got %d", app.cursorX)
	}
}

func TestCtrlShiftAndCtrlAltArrowSelectByWord(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha beta_gamma delta"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 0
	app.cursorX = 6

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl|tcell.ModShift))
	if quit {
		t.Fatal("did not expect Ctrl+Shift+Right to quit")
	}
	if got := app.selectedText(); got != "beta_gamma " {
		t.Fatalf("unexpected Ctrl+Shift word selection: %q", got)
	}

	app.clearSelection("Selection cleared.")
	app.cursorX = 6
	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl|tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Ctrl+Alt+Right selection path to quit")
	}
	if app.cursorX != 17 {
		t.Fatalf("expected Ctrl+Alt+Right to move by word, got %d", app.cursorX)
	}

	app.cursorX = 6
	app.clearSelection("Selection cleared.")
	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl|tcell.ModShift|tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Ctrl+Shift+Alt+Right to quit")
	}
	if got := app.selectedText(); got != "beta_gamma " {
		t.Fatalf("unexpected Ctrl+Shift+Alt word selection: %q", got)
	}
}

func TestCtrlDownInsertsBlankLineAboveWithoutSelection(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha", "beta"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 1
	app.cursorX = 2

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect Ctrl+Down to quit")
	}
	if got := app.tabs[app.activeTab].Content[0]; got != "alpha" {
		t.Fatalf("unexpected first line: %q", got)
	}
	if got := app.tabs[app.activeTab].Content[1]; got != "" {
		t.Fatalf("expected inserted blank line, got %q", got)
	}
	if got := app.tabs[app.activeTab].Content[2]; got != "beta" {
		t.Fatalf("unexpected shifted line: %q", got)
	}
	if app.cursorY != 2 || app.cursorX != 2 {
		t.Fatalf("expected caret to stay on original content line, got y=%d x=%d", app.cursorY, app.cursorX)
	}
}

func TestCtrlUpRemovesBlankLineAboveWithoutSelection(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha", "  \t", "beta"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 2
	app.cursorX = 1

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect Ctrl+Up to quit")
	}
	if len(app.tabs[app.activeTab].Content) != 2 {
		t.Fatalf("expected blank line removal, got %d lines", len(app.tabs[app.activeTab].Content))
	}
	if got := app.tabs[app.activeTab].Content[1]; got != "beta" {
		t.Fatalf("unexpected remaining line: %q", got)
	}
	if app.cursorY != 1 {
		t.Fatalf("expected caret to follow original content, got y=%d", app.cursorY)
	}
}

func TestCtrlAltUpAndDownDoNothing(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha", "beta"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 1
	app.cursorX = 2

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModCtrl|tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Ctrl+Alt+Down to quit")
	}
	if len(app.tabs[app.activeTab].Content) != 2 || app.tabs[app.activeTab].Content[0] != "alpha" || app.tabs[app.activeTab].Content[1] != "beta" {
		t.Fatalf("expected Ctrl+Alt+Down to leave content unchanged: %#v", app.tabs[app.activeTab].Content)
	}
	if app.cursorY != 1 || app.cursorX != 2 {
		t.Fatalf("expected Ctrl+Alt+Down to leave caret unchanged, got y=%d x=%d", app.cursorY, app.cursorX)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModCtrl|tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Ctrl+Alt+Up to quit")
	}
	if len(app.tabs[app.activeTab].Content) != 2 || app.tabs[app.activeTab].Content[0] != "alpha" || app.tabs[app.activeTab].Content[1] != "beta" {
		t.Fatalf("expected Ctrl+Alt+Up to leave content unchanged: %#v", app.tabs[app.activeTab].Content)
	}
	if app.cursorY != 1 || app.cursorX != 2 {
		t.Fatalf("expected Ctrl+Alt+Up to leave caret unchanged, got y=%d x=%d", app.cursorY, app.cursorX)
	}
}

func TestTabIndentsSelectedRange(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha", "beta", "gamma"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 0
	app.cursorX = 0
	app.handleEditorKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift))

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyTab, 0, 0))
	if quit {
		t.Fatal("did not expect Tab to quit")
	}
	if got := app.tabs[app.activeTab].Content[0]; got != "  alpha" {
		t.Fatalf("unexpected first indented line: %q", got)
	}
	if got := app.tabs[app.activeTab].Content[1]; got != "  beta" {
		t.Fatalf("unexpected second indented line: %q", got)
	}
}

func TestBacktabOutdentsSelectedRange(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"\talpha", "\tbeta", "gamma"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.cursorY = 0
	app.cursorX = 0
	app.handleEditorKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift))

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyBacktab, 0, 0))
	if quit {
		t.Fatal("did not expect Shift+Tab to quit")
	}
	if got := app.tabs[app.activeTab].Content[0]; got != "alpha" {
		t.Fatalf("unexpected first outdented line: %q", got)
	}
	if got := app.tabs[app.activeTab].Content[1]; got != "beta" {
		t.Fatalf("unexpected second outdented line: %q", got)
	}
}

func TestStyleTabGlyphDiffersFromNormalText(t *testing.T) {
	if styleTabGlyph() == styleNormal() {
		t.Fatal("expected literal tab glyph style to differ from normal text")
	}
}

func TestCtrlTabNavigatesTabsWithoutIndenting(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 0
	original := app.tabs[0].Content[3]

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect Ctrl+Tab to quit")
	}
	if app.activeTab != 1 {
		t.Fatalf("expected active tab 1, got %d", app.activeTab)
	}
	if got := app.tabs[0].Content[3]; got != original {
		t.Fatalf("Ctrl+Tab should not indent source tab, got %q", got)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect Ctrl+Shift+Tab to quit")
	}
	if app.activeTab != 0 {
		t.Fatalf("expected active tab 0 after reverse navigation, got %d", app.activeTab)
	}
}

func TestEnsureCursorInBoundsAdjustsViewportForLowerLines(t *testing.T) {
	app := New()
	app.tabs[app.activeTab] = Tab{
		Title:            "long.txt",
		Content:          []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.editorViewportHeight = 5
	app.cursorY = 8
	app.ensureCursorInBounds()

	if app.viewportTop != 4 {
		t.Fatalf("expected viewportTop 4, got %d", app.viewportTop)
	}
}

func TestMoveByPageUpdatesViewport(t *testing.T) {
	app := New()
	content := make([]string, 30)
	for i := range content {
		content[i] = fmt.Sprintf("line %d", i)
	}
	app.tabs[app.activeTab] = Tab{
		Title:            "long.txt",
		Content:          content,
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.editorViewportHeight = 5

	app.moveByPage(1, false)

	if app.cursorY != 12 {
		t.Fatalf("expected cursorY 12 after page move, got %d", app.cursorY)
	}
	if app.viewportTop != 8 {
		t.Fatalf("expected viewportTop 8 after page move, got %d", app.viewportTop)
	}
}

func TestEnsureCursorInBoundsAdjustsHorizontalViewport(t *testing.T) {
	app := New()
	app.tabs[app.activeTab] = Tab{
		Title:            "wide.txt",
		Content:          []string{"0123456789abcdefghij"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.editorViewportWidth = 8
	app.cursorX = 12
	app.ensureCursorInBounds()

	if app.viewportLeft != 5 {
		t.Fatalf("expected viewportLeft 5, got %d", app.viewportLeft)
	}
}

func TestHorizontalViewportPersistsPerTab(t *testing.T) {
	app := New()
	app.tabs[0] = Tab{Title: "wide-a.txt", Content: []string{"0123456789abcdefghij"}, Locked: map[int]bool{}, SelectionAnchorY: -1}
	app.tabs[1] = Tab{Title: "wide-b.txt", Content: []string{"abcdefghij0123456789"}, Locked: map[int]bool{}, SelectionAnchorY: -1}
	app.loadCurrentTabState()
	app.editorViewportWidth = 8
	app.cursorX = 12
	app.ensureCursorInBounds()

	app.nextTab()
	if app.viewportLeft != 0 {
		t.Fatalf("expected fresh tab viewportLeft 0, got %d", app.viewportLeft)
	}
	app.editorViewportWidth = 8
	app.cursorX = 10
	app.ensureCursorInBounds()
	app.prevTab()

	if app.viewportLeft != 5 {
		t.Fatalf("expected first tab viewportLeft restored to 5, got %d", app.viewportLeft)
	}
}

func TestEnsureControlCursorInBoundsAdjustsHorizontalViewport(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("0123456789abcdefghij")
	app.controlCursor = 12
	app.controlViewportWidth = 8

	app.ensureControlCursorInBounds()

	if app.controlViewportLeft != 5 {
		t.Fatalf("expected controlViewportLeft 5, got %d", app.controlViewportLeft)
	}
}

func TestControlViewportTracksCursorMovementAndHome(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("0123456789abcdefghij")
	app.controlViewportWidth = 8
	app.ensureControlCursorInBounds()

	for range 12 {
		quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyRight, 0, 0))
		if quit {
			t.Fatal("did not expect Right in control hub to quit")
		}
	}

	if app.controlCursor != 12 {
		t.Fatalf("expected controlCursor 12, got %d", app.controlCursor)
	}
	if app.controlViewportLeft != 5 {
		t.Fatalf("expected scrolled controlViewportLeft 5, got %d", app.controlViewportLeft)
	}

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyHome, 0, 0))
	if quit {
		t.Fatal("did not expect Home in control hub to quit")
	}
	if app.controlCursor != 0 {
		t.Fatalf("expected controlCursor 0 after Home, got %d", app.controlCursor)
	}
	if app.controlViewportLeft != 0 {
		t.Fatalf("expected controlViewportLeft 0 after Home, got %d", app.controlViewportLeft)
	}
}

func TestControlViewportUsesVisualWidthForWideCharacters(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("가나다라마바사")
	app.controlCursor = 4
	app.controlViewportWidth = 6

	app.ensureControlCursorInBounds()

	if app.controlViewportLeft != 3 {
		t.Fatalf("expected controlViewportLeft 3 for wide characters, got %d", app.controlViewportLeft)
	}
}

func TestAltCommaAndAltDotNavigateTabs(t *testing.T) {
	app := New()

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRune, '.', tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+. to quit")
	}
	if app.activeTab != 1 {
		t.Fatalf("expected active tab 1 after Alt+., got %d", app.activeTab)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRune, ',', tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+, to quit")
	}
	if app.activeTab != 0 {
		t.Fatalf("expected active tab 0 after Alt+,, got %d", app.activeTab)
	}
}

func TestQuitConfirmationCanCancelAndConfirm(t *testing.T) {
	app := New()
	_ = app.handleKey(tcell.NewEventKey(tcell.KeyCtrlQ, 0, 0))

	quit := app.handleKey(tcell.NewEventKey(tcell.KeyRune, 'n', 0))
	if quit {
		t.Fatal("expected n to cancel quit")
	}
	if app.quitConfirm {
		t.Fatal("expected quit confirmation to close after cancel")
	}

	_ = app.handleKey(tcell.NewEventKey(tcell.KeyCtrlQ, 0, 0))
	quit = app.handleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if !quit {
		t.Fatal("expected Enter to confirm quit")
	}
}

func TestInsertRuneUpdatesUnlockedLine(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 1

	app.insertRune('X')

	got := app.tabs[app.activeTab].Content[3]
	if got != "	Xif input == \"agent\" {" {
		t.Fatalf("unexpected edited line: %q", got)
	}
}

func TestInsertRuneEditsAnyLineWithoutLockDenial(t *testing.T) {
	app := New()
	app.cursorY = 0
	app.cursorX = 0

	app.insertRune('X')

	if app.tabs[app.activeTab].Content[0] != "Xpackage main" {
		t.Fatalf("expected first line to be directly editable, got %q", app.tabs[app.activeTab].Content[0])
	}
}

func TestInsertNewLineSplitsUnlockedLine(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 1

	app.insertNewLine()

	if app.tabs[app.activeTab].Content[3] != "	" {
		t.Fatalf("unexpected first split line: %q", app.tabs[app.activeTab].Content[3])
	}
	if app.tabs[app.activeTab].Content[4] != "if input == \"agent\" {" {
		t.Fatalf("unexpected second split line: %q", app.tabs[app.activeTab].Content[4])
	}
}

func TestEditorPrintableRunesAreNotHijacked(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 0

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyRune, 'q', 0))
	if quit {
		t.Fatal("expected q to be inserted instead of quitting")
	}
	if got := app.tabs[app.activeTab].Content[3]; got != "q\tif input == \"agent\" {" {
		t.Fatalf("unexpected line after q insert: %q", got)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyRune, ':', 0))
	if quit {
		t.Fatal("expected : to be inserted instead of changing focus")
	}
	if app.focus != focusEditor {
		t.Fatalf("expected editor focus to remain active, got %v", app.focus)
	}
}

func TestAltDownCreatesLineSelection(t *testing.T) {
	app := New()
	app.cursorY = 3
	app.cursorX = 0

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift))
	if quit {
		t.Fatal("did not expect selection growth to quit")
	}
	if !app.hasSelection() {
		t.Fatal("expected line selection to start")
	}
	if got := app.currentScope(); got != "selection:L4:C1-L5:C1" {
		t.Fatalf("unexpected selection scope: %s", got)
	}
}

func TestCtrlDownMovesSelectedBlock(t *testing.T) {
	app := New()
	app.activeTab = 2
	app.tabs[app.activeTab] = Tab{
		Title:   "notes.md",
		Content: []string{"a", "b", "c", "d"},
		Locked:  map[int]bool{},
	}
	app.cursorY = 1
	app.cursorX = 0
	app.handleEditorKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift))

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect block move to quit")
	}

	want := []string{"a", "d", "b", "c"}
	for i, line := range want {
		if got := app.tabs[app.activeTab].Content[i]; got != line {
			t.Fatalf("unexpected content at %d: got %q want %q", i, got, line)
		}
	}
	if got := app.currentScope(); got != "selection:L3:C1-L4:C1" {
		t.Fatalf("unexpected moved selection scope: %s", got)
	}
}

func TestControlInputAllowsLeadingQ(t *testing.T) {
	app := New()
	app.focus = focusControl

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyRune, 'q', 0))
	if quit {
		t.Fatal("expected q to be typed into control input")
	}
	if got := string(app.controlInput); got != "q" {
		t.Fatalf("unexpected control input: %q", got)
	}
}

func TestControlHubClipboardCopyCutPaste(t *testing.T) {
	withClipboardMocks(
		func() (string, error) { return " pasted", nil },
		func(text string) error { return nil },
		func() {
			app := New()
			app.focus = focusControl
			app.controlInput = []rune("inspect")
			app.controlCursor = len(app.controlInput)

			quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+C in control hub to quit")
			}
			if app.clipboard != "inspect" {
				t.Fatalf("unexpected control clipboard: %q", app.clipboard)
			}

			quit = app.handleControlKey(tcell.NewEventKey(tcell.KeyCtrlX, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+X in control hub to quit")
			}
			if got := string(app.controlInput); got != "" {
				t.Fatalf("expected control input to be cleared after cut, got %q", got)
			}

			quit = app.handleControlKey(tcell.NewEventKey(tcell.KeyCtrlV, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+V in control hub to quit")
			}
			if got := string(app.controlInput); got != " pasted" {
				t.Fatalf("unexpected control input after paste: %q", got)
			}
		},
	)
}

func TestControlHubSelectionShortcuts(t *testing.T) {
	withClipboardMocks(
		func() (string, error) { return "ZZ", nil },
		func(text string) error { return nil },
		func() {
			app := New()
			app.focus = focusControl
			app.controlInput = []rune("inspect")
			app.controlCursor = len(app.controlInput)

			quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyCtrlA, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+A in control hub to quit")
			}
			if !app.hasControlSelection() {
				t.Fatal("expected Ctrl+A to select all control input")
			}
			if got := app.selectedControlText(); got != "inspect" {
				t.Fatalf("unexpected selected control text: %q", got)
			}

			app.clearControlSelection()
			app.controlCursor = 1
			quit = app.handleControlKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
			if quit {
				t.Fatal("did not expect Shift+Right in control hub to quit")
			}
			if got := app.selectedControlText(); got != "n" {
				t.Fatalf("unexpected shifted control selection: %q", got)
			}

			quit = app.handleControlKey(tcell.NewEventKey(tcell.KeyCtrlV, 0, 0))
			if quit {
				t.Fatal("did not expect Ctrl+V replacement in control hub to quit")
			}
			if got := string(app.controlInput); got != "iZZspect" {
				t.Fatalf("unexpected control input after replacing selection: %q", got)
			}
		},
	)
}

func TestCtrlLeftBracketSelectsCurrentAndParentBlock(t *testing.T) {
	app := New()
	app.activeTab = 0
	app.tabs[app.activeTab] = Tab{
		Title: "main.go",
		Content: []string{
			"func outer() {",
			"\tif ready {",
			"\t\tprintln(\"go\")",
			"\t}",
			"}",
		},
		Locked: map[int]bool{},
	}
	app.cursorY = 2
	app.cursorX = 0

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlLeftSq, 0, 0))
	if quit {
		t.Fatal("did not expect Ctrl+[ to quit")
	}
	if got := app.currentScope(); got != "selection:L2:C1-L4:C3" {
		t.Fatalf("unexpected first block scope: %s", got)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlLeftSq, 0, 0))
	if quit {
		t.Fatal("did not expect second Ctrl+[ to quit")
	}
	if got := app.currentScope(); got != "selection:L1:C1-L5:C2" {
		t.Fatalf("unexpected parent block scope: %s", got)
	}
}

func TestCtrlSpaceSelectsCurrentAndParentBlock(t *testing.T) {
	app := New()
	app.activeTab = 0
	app.tabs[app.activeTab] = Tab{
		Title: "main.go",
		Content: []string{
			"func outer() {",
			"\tif ready {",
			"\t\tprintln(\"go\")",
			"\t}",
			"}",
		},
		Locked: map[int]bool{},
	}
	app.cursorY = 2
	app.cursorX = 0

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlSpace, 0, 0))
	if quit {
		t.Fatal("did not expect Ctrl+Space to quit")
	}
	if got := app.currentScope(); got != "selection:L2:C1-L4:C3" {
		t.Fatalf("unexpected first block scope: %s", got)
	}

	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlSpace, 0, 0))
	if quit {
		t.Fatal("did not expect second Ctrl+Space to quit")
	}
	if got := app.currentScope(); got != "selection:L1:C1-L5:C2" {
		t.Fatalf("unexpected parent block scope: %s", got)
	}
}

func TestF2SelectsCurrentAndParentBlockOnDefaultSamples(t *testing.T) {
	app := New()
	app.activeTab = 0
	app.cursorY = 4
	app.cursorX = 0

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyF2, 0, 0))
	if quit {
		t.Fatal("did not expect F2 to quit")
	}
	if got := app.currentScope(); got != "selection:L4:C1-L6:C3" {
		t.Fatalf("unexpected Go block scope: %s", got)
	}

	app.activeTab = 1
	app.selectionAnchor = -1
	app.cursorY = 4
	app.cursorX = 0
	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyF2, 0, 0))
	if quit {
		t.Fatal("did not expect F2 to quit on Python sample")
	}
	if got := app.currentScope(); got != "selection:L4:C1-L5:C33" {
		t.Fatalf("unexpected Python block scope: %s", got)
	}

	app.activeTab = 2
	app.selectionAnchor = -1
	app.cursorY = 4
	app.cursorX = 0
	quit = app.handleEditorKey(tcell.NewEventKey(tcell.KeyF2, 0, 0))
	if quit {
		t.Fatal("did not expect F2 to quit on TypeScript sample")
	}
	if got := app.currentScope(); got != "selection:L3:C1-L6:C6" {
		t.Fatalf("unexpected TypeScript block scope: %s", got)
	}
}

func TestCtrlLeftBracketFallsBackToCurrentLineWithoutBlock(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:   "flat.txt",
		Content: []string{"alpha", "beta", "gamma"},
		Locked:  map[int]bool{},
	}
	app.cursorY = 1
	app.cursorX = 0

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlLeftSq, 0, 0))
	if quit {
		t.Fatal("did not expect Ctrl+[ to quit")
	}
	if got := app.currentScope(); got != "selection:L2:C1-L2:C5" {
		t.Fatalf("unexpected fallback scope: %s", got)
	}
}

func TestControlHubConfirmInvokesAgentAndReplacesSelection(t *testing.T) {
	var app *App
	executor := &stubAgentExecutor{
		response: agent.Response{
			Mode:    agent.ModeReplaceSelection,
			Content: "return buildAgentReply(input)",
			Message: "Edit agent updated the current selection.",
		},
		onExecute: func() {
			if app.voiceState != "sending" {
				t.Fatalf("expected voiceState to indicate sending, got %q", app.voiceState)
			}
			if app.statusMessage != "Sending request to the edit agent..." {
				t.Fatalf("unexpected sending status: %q", app.statusMessage)
			}
		},
	}
	app = NewWithAgent("edit-agent openai/gpt-5.4", executor, "", "")
	app.focus = focusControl
	app.activeTab = 0
	line := app.tabs[app.activeTab].Content[4]
	app.setSelectionAnchor(0, 4)
	app.cursorY = 4
	app.cursorX = len([]rune(line))
	app.controlInput = []rune("simplify this block")
	app.controlCursor = len(app.controlInput)

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if quit {
		t.Fatal("did not expect preview enter to quit")
	}
	if !app.preview.Pending {
		t.Fatal("expected preview to become pending")
	}

	quit = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if quit {
		t.Fatal("did not expect confirm enter to quit")
	}
	if got := executor.request.Command; got != "simplify this block" {
		t.Fatalf("unexpected command sent to agent: %q", got)
	}
	if got := executor.request.Tab; got != "main.go" {
		t.Fatalf("unexpected tab sent to agent: %q", got)
	}
	if got := executor.request.Scope; got == "" {
		t.Fatal("expected non-empty scope")
	}
	if got := app.tabs[app.activeTab].Content[4]; got != "return buildAgentReply(input)" {
		t.Fatalf("unexpected edited line: %q", got)
	}
	if app.preview.Pending {
		t.Fatal("expected preview to clear after execution")
	}
	if app.focus != focusControl {
		t.Fatal("expected focus to remain in control hub after agent execution")
	}
	if got := app.statusMessage; got != "Edit agent updated the current selection." {
		t.Fatalf("unexpected status message: %q", got)
	}
}

func TestControlHubConfirmShowsAgentFailure(t *testing.T) {
	executor := &stubAgentExecutor{err: errors.New("boom")}
	app := NewWithAgent("edit-agent openai/gpt-5.4", executor, "", "")
	app.focus = focusControl
	app.controlInput = []rune("inspect this")
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if got := app.statusMessage; got != "edit agent failed: boom" {
		t.Fatalf("unexpected status message: %q", got)
	}
	if !app.preview.Pending {
		t.Fatal("expected preview to remain pending after failure")
	}
	if app.voiceState != "ready" {
		t.Fatalf("expected voice state to recover after failure, got %q", app.voiceState)
	}
}

func TestControlHubConfirmStoresVisibleAgentReplyForMessageMode(t *testing.T) {
	executor := &stubAgentExecutor{
		response: agent.Response{
			Mode:    agent.ModeMessage,
			Message: "Hello. I am ready to help with this file.",
		},
	}
	app := NewWithAgent("edit-agent openai/gpt-5.4", executor, "", "")
	app.focus = focusControl
	app.controlInput = []rune("hello")
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if got := app.lastAgentReply; got != "Hello. I am ready to help with this file." {
		t.Fatalf("unexpected last agent reply: %q", got)
	}
	if got := app.lastAgentResult; got != "Hello. I am ready to help with this file." {
		t.Fatalf("unexpected last agent result: %q", got)
	}
	if got := app.statusMessage; got != "Hello. I am ready to help with this file." {
		t.Fatalf("unexpected status message: %q", got)
	}
	if app.focus != focusControl {
		t.Fatal("expected focus to remain in control hub after message reply")
	}
}

func TestControlHubTalkExecutesOnFirstEnter(t *testing.T) {
	var app *App
	executor := &stubAgentExecutor{
		response: agent.Response{Mode: agent.ModeMessage, Message: "Hello from the edit agent."},
		onExecute: func() {
			if app.voiceState != "sending" {
				t.Fatalf("expected sending state for immediate talk execution, got %q", app.voiceState)
			}
		},
	}
	app = NewWithAgent("edit-agent openai/gpt-5.4", executor, "", "")
	app.focus = focusControl
	app.controlInput = []rune("hello")
	app.controlCursor = len(app.controlInput)

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if quit {
		t.Fatal("did not expect immediate talk execution to quit")
	}
	if app.preview.Pending {
		t.Fatal("did not expect talk to remain in preview")
	}
	if got := app.statusMessage; got != "Hello from the edit agent." {
		t.Fatalf("unexpected status message: %q", got)
	}
	if got := executor.request.Kind; got != string(CommandTalk) {
		t.Fatalf("unexpected command kind: %q", got)
	}
}

func TestControlHubInspectExecutesOnFirstEnter(t *testing.T) {
	executor := &stubAgentExecutor{response: agent.Response{Mode: agent.ModeMessage, Message: "Current scope inspected."}}
	app := NewWithAgent("edit-agent openai/gpt-5.4", executor, "", "")
	app.focus = focusControl
	app.controlInput = []rune("explain this file")
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if app.preview.Pending {
		t.Fatal("did not expect inspect to remain in preview")
	}
	if got := executor.request.Kind; got != string(CommandInspect) {
		t.Fatalf("unexpected inspect request kind: %q", got)
	}
	if got := app.statusMessage; got != "Current scope inspected." {
		t.Fatalf("unexpected status message: %q", got)
	}
}

func TestTypingAfterReplyClearsStaleAgentReply(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.lastAgentReply = "Previous reply"

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyRune, 'a', 0))
	if quit {
		t.Fatal("did not expect typing in control hub to quit")
	}
	if app.lastAgentReply != "" {
		t.Fatalf("expected stale reply to clear, got %q", app.lastAgentReply)
	}
	if app.lastAgentResult != "" {
		t.Fatalf("did not expect last agent result to change in this test, got %q", app.lastAgentResult)
	}
}

func TestNormalizeInsertedRunesComposesHangulJamo(t *testing.T) {
	runes, cursor := normalizeInsertedRunes([]rune{'ᄒ', 'ᅡ'}, 2)
	if got := string(runes); got != "하" {
		t.Fatalf("unexpected normalized text: %q", got)
	}
	if cursor != 1 {
		t.Fatalf("unexpected cursor after normalization: %d", cursor)
	}
}

func TestVisualColumnForRunesCountsWideCharacters(t *testing.T) {
	if got := visualColumnForRunes([]rune("가a"), 1); got != 2 {
		t.Fatalf("unexpected visual column after Hangul rune: %d", got)
	}
	if got := visualColumnForRunes([]rune("가a"), 2); got != 3 {
		t.Fatalf("unexpected visual column after Hangul and ASCII: %d", got)
	}
}

func TestNewWithFilesLoadsRealFileTabs(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "note.txt"
	if err := os.WriteFile(filePath, []byte("hello\nworld\n"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	app, err := NewWithFiles("edit-agent openai/gpt-5.4", nil, workspace, "", []string{filePath})
	if err != nil {
		t.Fatalf("new with files: %v", err)
	}
	if len(app.tabs) != 1 {
		t.Fatalf("unexpected tab count: %d", len(app.tabs))
	}
	if app.tabs[0].Title != "note.txt" {
		t.Fatalf("unexpected tab title: %q", app.tabs[0].Title)
	}
	if app.tabs[0].FilePath == "" {
		t.Fatal("expected real file path on tab")
	}
	if got := app.tabs[0].Content[0]; got != "hello" {
		t.Fatalf("unexpected first line: %q", got)
	}
}

func TestNewWithFilesCreatesEmptyFileBackedTabForMissingPath(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "missing.txt"

	app, err := NewWithFiles("", nil, workspace, "", []string{filePath})
	if err != nil {
		t.Fatalf("new with missing file: %v", err)
	}
	if len(app.tabs) != 1 {
		t.Fatalf("unexpected tab count: %d", len(app.tabs))
	}
	if app.tabs[0].FilePath != filePath {
		t.Fatalf("unexpected missing-file tab path: %q", app.tabs[0].FilePath)
	}
	if app.tabs[0].Title != "missing.txt" {
		t.Fatalf("unexpected missing-file tab title: %q", app.tabs[0].Title)
	}
	if len(app.tabs[0].Content) != 1 || app.tabs[0].Content[0] != "" {
		t.Fatalf("unexpected missing-file tab content: %#v", app.tabs[0].Content)
	}
	if _, err := os.Stat(filePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to stay absent until save, got err=%v", err)
	}
}

func TestCtrlSSavesActiveFile(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "note.txt"
	if err := os.WriteFile(filePath, []byte("hello\nworld\n"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	app, err := NewWithFiles("", nil, workspace, "", []string{filePath})
	if err != nil {
		t.Fatalf("new with files: %v", err)
	}
	app.cursorY = 0
	app.cursorX = len([]rune("hello"))
	app.insertRune('!')
	if !app.tabs[0].Dirty {
		t.Fatal("expected tab to become dirty after edit")
	}

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, 0))
	if quit {
		t.Fatal("did not expect Ctrl+S to quit")
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(content) != "hello!\nworld\n" {
		t.Fatalf("unexpected saved file content: %q", string(content))
	}
	if !strings.Contains(app.statusMessage, "Saved") {
		t.Fatalf("unexpected save status: %q", app.statusMessage)
	}
	if app.tabs[0].Dirty {
		t.Fatal("expected tab dirty state to clear after save")
	}
}

func TestCtrlSCreatesMissingFileFromEmptyTab(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "missing.txt"
	app, err := NewWithFiles("", nil, workspace, "", []string{filePath})
	if err != nil {
		t.Fatalf("new with missing file: %v", err)
	}
	app.insertRune('a')
	if !app.tabs[0].Dirty {
		t.Fatal("expected missing-file tab to become dirty after edit")
	}

	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, 0))
	if quit {
		t.Fatal("did not expect Ctrl+S to quit")
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if string(content) != "a" {
		t.Fatalf("unexpected created file content: %q", string(content))
	}
	if !strings.Contains(app.statusMessage, "Saved") {
		t.Fatalf("unexpected create-and-save status: %q", app.statusMessage)
	}
	if app.tabs[0].Dirty {
		t.Fatal("expected missing-file tab dirty state to clear after save")
	}
}

func TestMarkActiveTabDirtySetsDirtyFlag(t *testing.T) {
	app := NewScratch()
	if app.tabs[0].Dirty {
		t.Fatal("did not expect new scratch tab to start dirty")
	}
	app.insertRune('x')
	if !app.tabs[0].Dirty {
		t.Fatal("expected tab to become dirty after insert")
	}
}

func TestCtrlSRejectsScratchBuffer(t *testing.T) {
	app := NewScratch()
	quit := app.handleEditorKey(tcell.NewEventKey(tcell.KeyCtrlS, 0, 0))
	if quit {
		t.Fatal("did not expect Ctrl+S on scratch buffer to quit")
	}
	if !strings.Contains(app.statusMessage, "not backed by a file") {
		t.Fatalf("unexpected scratch save status: %q", app.statusMessage)
	}
}

func TestControlHubMemoCommandSavesCurrentFileMemo(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "config.yaml"
	if err := os.WriteFile(filePath, []byte("mode: safe\n"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	app, err := NewWithFiles("", nil, workspace, "", []string{filePath})
	if err != nil {
		t.Fatalf("new with files: %v", err)
	}
	app.focus = focusControl
	app.controlInput = []rune("memo keep this aligned with production")
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if !strings.Contains(app.statusMessage, "Saved memo to") {
		t.Fatalf("unexpected status message: %q", app.statusMessage)
	}
	contextText, err := memo.LoadContext("", workspace)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if !strings.Contains(contextText, "keep this aligned with production") {
		t.Fatalf("missing saved memo content: %s", contextText)
	}
}

func TestResolveMemoContentPrefersLastAgentReplyForNaturalLanguageRequest(t *testing.T) {
	app := New()
	app.lastAgentReply = "visible reply that may disappear"
	app.lastAgentResult = "WezTerm uses pwsh, custom backgrounds, and key bindings."
	input := "설정 내용을 메모해줘 메모파일은 앱의 이름으로"
	got := app.resolveMemoContent(input, extractMemoNote(input))
	if got != app.lastAgentResult {
		t.Fatalf("expected last agent reply, got %q", got)
	}
}

func TestBeginControlExecutionUsesMemoStatus(t *testing.T) {
	app := New()
	app.preview = Preview{Kind: CommandMemo, Pending: true}
	app.beginControlExecution()
	if app.voiceState != "sending" {
		t.Fatalf("unexpected voice state: %q", app.voiceState)
	}
	if app.statusMessage != "Saving memo for the current target..." {
		t.Fatalf("unexpected memo status: %q", app.statusMessage)
	}
}

func TestResolveMemoContentStillUsesLastAgentResultAfterVisibleReplyClears(t *testing.T) {
	app := New()
	app.lastAgentReply = ""
	app.lastAgentResult = "Persisted explanation for memo storage."
	input := "설정 내용을 메모해줘 메모파일은 앱의 이름으로"
	got := app.resolveMemoContent(input, extractMemoNote(input))
	if got != app.lastAgentResult {
		t.Fatalf("expected persisted last agent result, got %q", got)
	}
}

func TestExtractMemoNoteFallsBackToNaturalLanguageMemoRequest(t *testing.T) {
	input := "설정 내용을 메모해줘 메모파일은 앱의 이름으로"
	if got := extractMemoNote(input); got != input {
		t.Fatalf("unexpected memo note extraction: %q", got)
	}
}
