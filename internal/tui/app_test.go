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
	"gdedit/internal/processsync"
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

func TestHelpScrollsWithUpDownAndResetsOnClose(t *testing.T) {
	app := New()

	quit := app.handleKey(tcell.NewEventKey(tcell.KeyF1, 0, 0))
	if quit {
		t.Fatal("did not expect F1 to quit")
	}
	if !app.helpVisible {
		t.Fatal("expected help to become visible")
	}
	if app.helpScroll != 0 {
		t.Fatalf("expected help scroll to start at 0, got %d", app.helpScroll)
	}

	quit = app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	if quit {
		t.Fatal("did not expect Down in help to quit")
	}
	if app.helpScroll != 1 {
		t.Fatalf("expected help scroll to advance to 1, got %d", app.helpScroll)
	}

	quit = app.handleKey(tcell.NewEventKey(tcell.KeyUp, 0, 0))
	if quit {
		t.Fatal("did not expect Up in help to quit")
	}
	if app.helpScroll != 0 {
		t.Fatalf("expected help scroll to return to 0, got %d", app.helpScroll)
	}

	for range 200 {
		_ = app.handleKey(tcell.NewEventKey(tcell.KeyDown, 0, 0))
	}
	if app.helpScroll != app.maxHelpScroll() {
		t.Fatalf("expected help scroll to clamp at %d, got %d", app.maxHelpScroll(), app.helpScroll)
	}

	quit = app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, 0))
	if quit {
		t.Fatal("did not expect Esc in help to quit")
	}
	if app.helpVisible {
		t.Fatal("expected help to close on Esc")
	}
	if app.helpScroll != 0 {
		t.Fatalf("expected help scroll reset to 0 after close, got %d", app.helpScroll)
	}
}

func TestHistoryDialogOpensAndClosesWithF3AndEsc(t *testing.T) {
	app := New()
	quit := app.handleKey(tcell.NewEventKey(tcell.KeyF3, 0, 0))
	if quit {
		t.Fatal("did not expect F3 to quit")
	}
	if !app.historyVisible {
		t.Fatal("expected history dialog to become visible")
	}
	quit = app.handleKey(tcell.NewEventKey(tcell.KeyEsc, 0, 0))
	if quit {
		t.Fatal("did not expect Esc in history to quit")
	}
	if app.historyVisible {
		t.Fatal("expected history dialog to close")
	}
}

func TestAgentReplyAndHistoryAreScopedPerTab(t *testing.T) {
	app := New()
	app.activeTab = 0
	app.preview = Preview{Kind: CommandTalk, Pending: true, Action: "talk with edit agent", Tab: app.tabs[0].Title}
	app.executingControlInput = "hello main"
	app.completeControlExecution(agent.Response{Mode: agent.ModeMessage, Message: "reply for main"})
	if len(app.tabs[0].History) != 1 {
		t.Fatalf("expected first tab history length 1, got %d", len(app.tabs[0].History))
	}
	if app.tabs[0].History[0].Prompt != "hello main" {
		t.Fatalf("unexpected first tab history prompt: %q", app.tabs[0].History[0].Prompt)
	}

	app.nextTab()
	if got := app.lastAgentReply; got != "" {
		t.Fatalf("expected second tab to have empty last reply, got %q", got)
	}
	if len(app.tabs[app.activeTab].History) != 0 {
		t.Fatalf("expected second tab history to be empty, got %d", len(app.tabs[app.activeTab].History))
	}
	if got := strings.Join(app.historyLines(), "\n"); !strings.Contains(got, "(no conversation history for this tab yet)") {
		t.Fatalf("expected empty-history lines for second tab, got %q", got)
	}

	app.prevTab()
	if got := app.lastAgentReply; got != "reply for main" {
		t.Fatalf("expected first tab reply to restore, got %q", got)
	}
	if len(app.tabs[app.activeTab].History) != 1 {
		t.Fatalf("expected first tab history to restore, got %d", len(app.tabs[app.activeTab].History))
	}
	if got := strings.Join(app.historyLines(), "\n"); !strings.Contains(got, "hello main") || !strings.Contains(got, "reply for main") {
		t.Fatalf("expected first tab history lines to restore, got %q", got)
	}
}

func TestEllipsizeRunesAddsTrailingDots(t *testing.T) {
	got := ellipsizeRunes("this reply is too long", 10)
	if got != "this re..." {
		t.Fatalf("unexpected ellipsized text: %q", got)
	}
}

func TestHistoryLinesForWidthWrapsKoreanWithoutOverflow(t *testing.T) {
	app := New()
	app.tabs[app.activeTab].History = []historyEntry{{
		Prompt: "메모를 읽어서 장점을 말해줘",
		Reply:  "메모와 현재 설정을 보면 장점은 분명합니다. 여러 배경 이미지와 배경 전환 단축키가 설정되어 있습니다.",
	}}
	lines := app.historyLinesForWidth(24)
	if len(lines) < 4 {
		t.Fatalf("expected wrapped history lines, got %#v", lines)
	}
	for _, line := range lines {
		if visualWidthString(line) > 24 {
			t.Fatalf("history line exceeds target width: %q (width=%d)", line, visualWidthString(line))
		}
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

func TestTabIndentSelectionMovesSelectionColumnsWithContent(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"alpha", "beta", "gamma"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.setSelectionAnchor(1, 0)
	app.cursorY = 1
	app.cursorX = 2

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
	if app.selectionAnchorCol != 3 {
		t.Fatalf("expected anchor column to move to 3, got %d", app.selectionAnchorCol)
	}
	if app.cursorX != 4 {
		t.Fatalf("expected cursor column to move to 4, got %d", app.cursorX)
	}
	if got := app.selectedText(); got != "lpha\n  be" {
		t.Fatalf("unexpected selected text after indent: %q", got)
	}
}

func TestShiftTabOutdentSelectionMovesSelectionColumnsWithContent(t *testing.T) {
	app := New()
	app.activeTab = 3
	app.tabs[app.activeTab] = Tab{
		Title:            "flat.txt",
		Content:          []string{"  alpha", "  beta", "gamma"},
		Locked:           map[int]bool{},
		SelectionAnchorY: -1,
	}
	app.loadCurrentTabState()
	app.setSelectionAnchor(3, 0)
	app.cursorY = 1
	app.cursorX = 4

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
	if app.selectionAnchorCol != 1 {
		t.Fatalf("expected anchor column to move to 1, got %d", app.selectionAnchorCol)
	}
	if app.cursorX != 2 {
		t.Fatalf("expected cursor column to move to 2, got %d", app.cursorX)
	}
	if got := app.selectedText(); got != "lpha\nbe" {
		t.Fatalf("unexpected selected text after outdent: %q", got)
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

func TestAltCommaAndAltDotNavigateTabsFromControlHub(t *testing.T) {
	app := New()
	app.focus = focusControl

	quit := app.handleKey(tcell.NewEventKey(tcell.KeyRune, '.', tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+. in control hub to quit")
	}
	if app.activeTab != 1 {
		t.Fatalf("expected active tab 1 after Alt+. in control hub, got %d", app.activeTab)
	}

	quit = app.handleKey(tcell.NewEventKey(tcell.KeyRune, ',', tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt+, in control hub to quit")
	}
	if app.activeTab != 0 {
		t.Fatalf("expected active tab 0 after Alt+, in control hub, got %d", app.activeTab)
	}
}

func TestControlHubIgnoresUnhandledAltModifiedRunes(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("inspect")
	app.controlCursor = len(app.controlInput)

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Alt-modified control input to quit")
	}
	if got := string(app.controlInput); got != "inspect" {
		t.Fatalf("expected control input to remain unchanged, got %q", got)
	}
	if got := app.statusMessage; got != "Unhandled Alt-modified control key was ignored." {
		t.Fatalf("unexpected status message: %q", got)
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

func TestControlHubDeleteRemovesSelectedText(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("inspect")
	app.controlCursor = 7
	app.controlSelectAnchor = 2

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyDelete, 0, 0))
	if quit {
		t.Fatal("did not expect Delete in control hub to quit")
	}
	if got := string(app.controlInput); got != "in" {
		t.Fatalf("unexpected control input after Delete: %q", got)
	}
	if app.hasControlSelection() {
		t.Fatal("expected control selection to clear after Delete")
	}
}

func TestControlHubCtrlRightMovesByWord(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("open file_name now")

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect Ctrl+Right in control hub to quit")
	}
	if app.controlCursor != 5 {
		t.Fatalf("expected control cursor at 5 after first word jump, got %d", app.controlCursor)
	}

	quit = app.handleControlKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect second Ctrl+Right in control hub to quit")
	}
	if app.controlCursor != 15 {
		t.Fatalf("expected control cursor at 15 after second word jump, got %d", app.controlCursor)
	}
}

func TestControlHubCtrlLeftMovesByWord(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("open file_name now")
	app.controlCursor = len(app.controlInput)

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect Ctrl+Left in control hub to quit")
	}
	if app.controlCursor != 15 {
		t.Fatalf("expected control cursor at 15 after first backward word jump, got %d", app.controlCursor)
	}

	quit = app.handleControlKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl))
	if quit {
		t.Fatal("did not expect second Ctrl+Left in control hub to quit")
	}
	if app.controlCursor != 5 {
		t.Fatalf("expected control cursor at 5 after second backward word jump, got %d", app.controlCursor)
	}
}

func TestControlHubCtrlAltRightSelectsByWord(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("open file_name now")
	app.controlCursor = 5

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModCtrl|tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Ctrl+Alt+Right in control hub to quit")
	}
	if !app.hasControlSelection() {
		t.Fatal("expected Ctrl+Alt+Right to create a control selection")
	}
	if got := app.selectedControlText(); got != "file_name " {
		t.Fatalf("unexpected control selection after Ctrl+Alt+Right: %q", got)
	}
}

func TestControlHubCtrlAltLeftSelectsByWord(t *testing.T) {
	app := New()
	app.focus = focusControl
	app.controlInput = []rune("open file_name now")
	app.controlCursor = len(app.controlInput)

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModCtrl|tcell.ModAlt))
	if quit {
		t.Fatal("did not expect Ctrl+Alt+Left in control hub to quit")
	}
	if !app.hasControlSelection() {
		t.Fatal("expected Ctrl+Alt+Left to create a control selection")
	}
	if got := app.selectedControlText(); got != "now" {
		t.Fatalf("unexpected control selection after Ctrl+Alt+Left: %q", got)
	}
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

func TestControlHubOpenExecutesOnFirstEnter(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "note.txt"
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	app := NewScratchWithAgent("", nil, workspace, "")
	app.focus = focusControl
	app.controlInput = []rune("/open " + filePath)
	app.controlCursor = len(app.controlInput)

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if quit {
		t.Fatal("did not expect /open to quit")
	}
	if app.preview.Pending {
		t.Fatal("did not expect /open to remain in preview")
	}
	if len(app.tabs) != 2 {
		t.Fatalf("expected second tab to be opened, got %d tabs", len(app.tabs))
	}
	if app.activeTab != 1 {
		t.Fatalf("expected new tab to become active, got %d", app.activeTab)
	}
	if app.tabs[app.activeTab].Title != "note.txt" {
		t.Fatalf("unexpected open tab title: %q", app.tabs[app.activeTab].Title)
	}
	if got := app.statusMessage; !strings.Contains(got, "Opened") {
		t.Fatalf("unexpected status message: %q", got)
	}
	if app.voiceState != "ready" {
		t.Fatalf("expected voice state ready after /open, got %q", app.voiceState)
	}
}

func TestControlHubOpenSwitchesToExistingTab(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "note.txt"
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	app, err := NewWithFiles("", nil, workspace, "", []string{filePath})
	if err != nil {
		t.Fatalf("new with files: %v", err)
	}
	app.tabs = append(app.tabs, scratchTab())
	app.activeTab = 1
	app.focus = focusControl
	app.controlInput = []rune("/open " + filePath)
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if app.activeTab != 0 {
		t.Fatalf("expected existing file tab to become active, got %d", app.activeTab)
	}
	if got := app.statusMessage; !strings.Contains(got, "Switched to") {
		t.Fatalf("unexpected switch status: %q", got)
	}
}

func TestControlHubOpenCanCreateMissingFileBackedTab(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "missing.txt"

	app := NewScratchWithAgent("", nil, workspace, "")
	app.focus = focusControl
	app.controlInput = []rune("/open " + filePath)
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if app.tabs[app.activeTab].FilePath != filePath {
		t.Fatalf("unexpected opened file path: %q", app.tabs[app.activeTab].FilePath)
	}
	if len(app.tabs[app.activeTab].Content) != 1 || app.tabs[app.activeTab].Content[0] != "" {
		t.Fatalf("unexpected missing-file tab content: %#v", app.tabs[app.activeTab].Content)
	}
}

func TestOpenSyncCommandLoadsConfiguredSyncIntoNewTab(t *testing.T) {
	originalHome := processsync.UserHomeDirForTest()
	home := t.TempDir()
	processsync.SetUserHomeDirForTest(func() (string, error) { return home, nil })
	defer processsync.SetUserHomeDirForTest(originalHome)
	if _, err := processsync.Register("mynamr", "mynamr rule show {name} --spec-only", "mynamr rule update {name} --spec-stdin"); err != nil {
		t.Fatalf("register sync: %v", err)
	}
	originalLoad := runSyncLoad
	defer func() { runSyncLoad = originalLoad }()
	runSyncLoad = func(entry processsync.Entry, name string) ([]byte, []byte, error) {
		if entry.ReadFormat != "mynamr rule show {name} --spec-only" {
			t.Fatalf("unexpected read format: %q", entry.ReadFormat)
		}
		if name != "demo-rule" {
			t.Fatalf("unexpected sync name: %q", name)
		}
		return []byte("kind: demo\nname: demo-rule\n"), nil, nil
	}

	app := NewScratchWithAgent("", nil, t.TempDir(), "")
	resp, err := app.openSyncCommand("/sync mynamr demo-rule")
	if err != nil {
		t.Fatalf("open sync command: %v", err)
	}
	if resp.Message != "Opened sync mynamr:demo-rule" {
		t.Fatalf("unexpected response message: %q", resp.Message)
	}
	if got := app.tabs[app.activeTab].SyncID; got != "mynamr" {
		t.Fatalf("unexpected sync id on tab: %q", got)
	}
	if got := app.tabs[app.activeTab].SyncName; got != "demo-rule" {
		t.Fatalf("unexpected sync name on tab: %q", got)
	}
	if got := app.tabs[app.activeTab].Title; got != "mynamr:demo-rule" {
		t.Fatalf("unexpected tab title: %q", got)
	}
	if got := strings.Join(app.tabs[app.activeTab].Content, "\n"); got != "kind: demo\nname: demo-rule" {
		t.Fatalf("unexpected rule content: %q", got)
	}
}

func TestSaveActiveTabWritesSyncThroughStdinContract(t *testing.T) {
	originalHome := processsync.UserHomeDirForTest()
	home := t.TempDir()
	processsync.SetUserHomeDirForTest(func() (string, error) { return home, nil })
	defer processsync.SetUserHomeDirForTest(originalHome)
	if _, err := processsync.Register("mynamr", "mynamr rule show {name} --spec-only", "mynamr rule update {name} --spec-stdin"); err != nil {
		t.Fatalf("register sync: %v", err)
	}
	originalSave := runSyncSave
	defer func() { runSyncSave = originalSave }()
	var gotName string
	var gotContent string
	runSyncSave = func(entry processsync.Entry, name, content string) ([]byte, error) {
		if entry.WriteFormat != "mynamr rule update {name} --spec-stdin" {
			t.Fatalf("unexpected write format: %q", entry.WriteFormat)
		}
		gotName = name
		gotContent = content
		return []byte("ok"), nil
	}

	app := NewScratchWithAgent("", nil, t.TempDir(), "")
	app.tabs = []Tab{{
		Title:           "mynamr:demo-rule",
		SyncID:          "mynamr",
		SyncName:        "demo-rule",
		Content:         []string{"kind: demo", "name: demo-rule"},
		TrailingNewline: true,
		Dirty:           true,
	}}
	app.activeTab = 0
	app.saveActiveTab()

	if gotName != "demo-rule" {
		t.Fatalf("unexpected saved sync name: %q", gotName)
	}
	if gotContent != "kind: demo\nname: demo-rule\n" {
		t.Fatalf("unexpected saved content: %q", gotContent)
	}
	if app.tabs[0].Dirty {
		t.Fatal("expected rule tab dirty state to clear after save")
	}
	if app.statusMessage != "Saved sync mynamr:demo-rule" {
		t.Fatalf("unexpected status message: %q", app.statusMessage)
	}
}

func TestSaveActiveTabShowsRawSyncError(t *testing.T) {
	originalHome := processsync.UserHomeDirForTest()
	home := t.TempDir()
	processsync.SetUserHomeDirForTest(func() (string, error) { return home, nil })
	defer processsync.SetUserHomeDirForTest(originalHome)
	if _, err := processsync.Register("mynamr", "mynamr rule show {name} --spec-only", "mynamr rule update {name} --spec-stdin"); err != nil {
		t.Fatalf("register sync: %v", err)
	}
	originalSave := runSyncSave
	defer func() { runSyncSave = originalSave }()
	runSyncSave = func(entry processsync.Entry, name, content string) ([]byte, error) {
		return []byte("validation failed on line 3"), errors.New("exit status 1")
	}

	app := NewScratchWithAgent("", nil, t.TempDir(), "")
	app.tabs = []Tab{{
		Title:    "mynamr:demo-rule",
		SyncID:   "mynamr",
		SyncName: "demo-rule",
		Content:  []string{"broken: true"},
		Dirty:    true,
	}}
	app.activeTab = 0
	app.saveActiveTab()

	if app.statusMessage != "validation failed on line 3" {
		t.Fatalf("unexpected raw stderr status: %q", app.statusMessage)
	}
	if !app.tabs[0].Dirty {
		t.Fatal("expected failed rule save to keep dirty state")
	}
}

func TestControlHubWriteExecutesOnFirstEnter(t *testing.T) {
	workspace := t.TempDir()
	targetPath := workspace + string(os.PathSeparator) + "saved.txt"

	app := NewScratchWithAgent("", nil, workspace, "")
	app.insertRune('a')
	app.focus = focusControl
	app.controlInput = []rune("/write " + targetPath)
	app.controlCursor = len(app.controlInput)

	quit := app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	if quit {
		t.Fatal("did not expect /write to quit")
	}
	if app.preview.Pending {
		t.Fatal("did not expect /write to remain in preview")
	}
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(content) != "a" {
		t.Fatalf("unexpected written content: %q", string(content))
	}
	if app.tabs[app.activeTab].FilePath != targetPath {
		t.Fatalf("expected tab file path to update, got %q", app.tabs[app.activeTab].FilePath)
	}
	if app.tabs[app.activeTab].Title != "saved.txt" {
		t.Fatalf("expected tab title to update, got %q", app.tabs[app.activeTab].Title)
	}
	if app.tabs[app.activeTab].Dirty {
		t.Fatal("expected dirty state to clear after /write")
	}
	if got := app.statusMessage; !strings.Contains(got, "Saved") {
		t.Fatalf("unexpected status message: %q", got)
	}
}

func TestControlHubWriteSupportsQuotedPath(t *testing.T) {
	workspace := t.TempDir()
	targetPath := workspace + string(os.PathSeparator) + "file with spaces.txt"

	app := NewScratchWithAgent("", nil, workspace, "")
	app.insertRune('x')
	app.focus = focusControl
	app.controlInput = []rune(`/saveas "` + targetPath + `"`)
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read quoted-path file: %v", err)
	}
	if string(content) != "x" {
		t.Fatalf("unexpected quoted-path content: %q", string(content))
	}
}

func TestControlHubWriteCreatesMissingParentDirectories(t *testing.T) {
	workspace := t.TempDir()
	targetPath := workspace + string(os.PathSeparator) + "nested" + string(os.PathSeparator) + "dir" + string(os.PathSeparator) + "saved.txt"

	app := NewScratchWithAgent("", nil, workspace, "")
	app.insertRune('z')
	app.focus = focusControl
	app.controlInput = []rune("/write " + targetPath)
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read nested written file: %v", err)
	}
	if string(content) != "z" {
		t.Fatalf("unexpected nested written content: %q", string(content))
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

	if !strings.Contains(app.statusMessage, "Saved project(config.yaml) memo to") {
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

func TestResolveMemoContentDoesNotUseLastAgentResultForNonMemoRequest(t *testing.T) {
	app := New()
	app.lastAgentResult = "stale previous agent explanation"
	input := "explain this config"
	got := app.resolveMemoContent(input, extractMemoNote(input))
	if got != "" {
		t.Fatalf("expected non-memo request not to reuse last agent result, got %q", got)
	}
}

func TestResolveMemoContentPrefersExplicitArrowPayload(t *testing.T) {
	app := New()
	app.lastAgentResult = "stale previous agent explanation"
	input := "그러면 메모를 추가해줘 -> -.test"
	got := app.resolveMemoContent(input, extractMemoNote(input))
	if got != "-.test" {
		t.Fatalf("expected explicit memo payload, got %q", got)
	}
}

func TestMemoExecutionDoesNotOverwriteLastAgentResult(t *testing.T) {
	workspace := t.TempDir()
	filePath := workspace + string(os.PathSeparator) + "config.yaml"
	if err := os.WriteFile(filePath, []byte("mode: safe\n"), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	app, err := NewWithFiles("", nil, workspace, "", []string{filePath})
	if err != nil {
		t.Fatalf("new with files: %v", err)
	}
	app.lastAgentResult = "previous useful agent explanation"
	app.focus = focusControl
	app.controlInput = []rune("그러면 메모를 추가해줘 -> -.test")
	app.controlCursor = len(app.controlInput)

	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	_ = app.handleControlKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if app.lastAgentResult != "previous useful agent explanation" {
		t.Fatalf("expected memo execution not to overwrite lastAgentResult, got %q", app.lastAgentResult)
	}
	contextText, err := memo.LoadContext("", workspace)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if !strings.Contains(contextText, "- test") {
		t.Fatalf("missing explicit memo payload in saved context: %s", contextText)
	}
}
