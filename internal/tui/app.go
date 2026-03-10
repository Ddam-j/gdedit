package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"gdedit/internal/agent"
	"gdedit/internal/config"
	"gdedit/internal/memo"
	sysclipboard "github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"
)

var clipboardRead = func() (string, error) {
	return sysclipboard.ReadAll()
}

var clipboardWrite = func(text string) error {
	return sysclipboard.WriteAll(text)
}

const (
	minWidth      = 60
	minHeight     = 12
	defaultIndent = 2
	tabGlyph      = "»"
)

var spinnerFrames = []string{"-", "\\", "|", "/"}

type spinnerTick struct{}

type controlExecutionResult struct {
	response agent.Response
	err      error
}

type focusArea int

const (
	focusEditor focusArea = iota
	focusControl
)

type Tab struct {
	Title            string
	FilePath         string
	Content          []string
	TrailingNewline  bool
	Dirty            bool
	LastAgentReply   string
	LastAgentResult  string
	History          []historyEntry
	Locked           map[int]bool
	ViewTop          int
	ViewLeft         int
	CursorX          int
	CursorY          int
	SelectionAnchorX int
	SelectionAnchorY int
}

type lineRange struct {
	start int
	end   int
}

type historyEntry struct {
	Prompt string
	Reply  string
}

type textPos struct {
	x int
	y int
}

type App struct {
	screen                tcell.Screen
	tabs                  []Tab
	activeTab             int
	focus                 focusArea
	controlInput          []rune
	controlCursor         int
	controlSelectAnchor   int
	controlViewportLeft   int
	controlViewportWidth  int
	preview               Preview
	cursorX               int
	cursorY               int
	selectionAnchorCol    int
	selectionAnchor       int
	statusMessage         string
	lastAgentReply        string
	lastAgentResult       string
	executingControlInput string
	voiceState            string
	lastCommand           string
	helpVisible           bool
	helpScroll            int
	historyVisible        bool
	historyScroll         int
	quitConfirm           bool
	indentWidth           int
	clipboard             string
	agentProfile          string
	agentExecutor         agent.Executor
	workspaceRoot         string
	systemMemoRoot        string
	agentRunning          bool
	spinnerIndex          int
	spinnerStop           chan struct{}
	viewportTop           int
	editorViewportHeight  int
	viewportLeft          int
	editorViewportWidth   int
}

func New() *App {
	return NewWithAgent("", nil, "", "")
}

func NewScratch() *App {
	return NewScratchWithAgent("", nil, "", "")
}

func NewWithAgentProfile(agentProfile string) *App {
	return NewWithAgent(agentProfile, nil, "", "")
}

func NewWithAgent(agentProfile string, executor agent.Executor, workspaceRoot, systemMemoRoot string) *App {
	return newApp(sampleTabs(), agentProfile, executor, workspaceRoot, systemMemoRoot, "Ready. Ctrl+G focuses the control hub.")
}

func NewScratchWithAgent(agentProfile string, executor agent.Executor, workspaceRoot, systemMemoRoot string) *App {
	return newApp([]Tab{scratchTab()}, agentProfile, executor, workspaceRoot, systemMemoRoot, "Ready. Open a file or start typing. Ctrl+S saves file-backed tabs.")
}

func newApp(tabs []Tab, agentProfile string, executor agent.Executor, workspaceRoot, systemMemoRoot, statusMessage string) *App {
	if len(tabs) == 0 {
		tabs = []Tab{scratchTab()}
	}
	return &App{
		tabs:                tabs,
		focus:               focusEditor,
		selectionAnchor:     -1,
		controlSelectAnchor: -1,
		statusMessage:       statusMessage,
		voiceState:          "off",
		indentWidth:         defaultIndent,
		agentProfile:        agentProfile,
		agentExecutor:       executor,
		workspaceRoot:       workspaceRoot,
		systemMemoRoot:      systemMemoRoot,
	}
}

func NewWithFiles(agentProfile string, executor agent.Executor, workspaceRoot, systemMemoRoot string, filePaths []string) (*App, error) {
	app := NewScratchWithAgent(agentProfile, executor, workspaceRoot, systemMemoRoot)
	if len(filePaths) == 0 {
		return app, nil
	}
	tabs, err := loadTabsFromPaths(filePaths)
	if err != nil {
		return nil, err
	}
	app.tabs = tabs
	return app, nil
}

func scratchTab() Tab {
	return Tab{
		Title:            "untitled",
		Content:          []string{""},
		SelectionAnchorY: -1,
	}
}

func sampleTabs() []Tab {
	return []Tab{
		{
			Title: "main.go",
			Content: []string{
				"package main",
				"",
				"func route(input string) string {",
				"\tif input == \"agent\" {",
				"		return buildReply(input)",
				"	}",
				"	return fallback(input)",
				"}",
			},
			SelectionAnchorY: -1,
		},
		{
			Title: "worker.py",
			Content: []string{
				"def transform(task):",
				"    if task.ready:",
				"        for step in task.steps:",
				"            if step.enabled:",
				"                return step.name",
				"    return \"idle\"",
			},
			SelectionAnchorY: -1,
		},
		{
			Title: "panel.ts",
			Content: []string{
				"export function buildPanel(state: EditorState) {",
				"  if (state.mode === \"review\") {",
				"    return {",
				"      title: \"Review Queue\",",
				"      items: state.items.map((item) => item.label),",
				"    }",
				"  }",
				"  return { title: \"Editor\", items: [] }",
				"}",
			},
			SelectionAnchorY: -1,
		},
		{
			Title: "config.yaml",
			Content: []string{
				"workstyle:",
				"  name: focused-review",
				"  shortcuts:",
				"    select_block: ctrl-[",
				"    move_block_up: ctrl-up",
				"    move_block_down: ctrl-down",
			},
			SelectionAnchorY: -1,
		},
	}
}

func loadTabsFromPaths(filePaths []string) ([]Tab, error) {
	tabs := make([]Tab, 0, len(filePaths))
	for _, path := range filePaths {
		expandedPath, err := config.ExpandUserPath(path)
		if err != nil {
			return nil, err
		}
		absPath, err := filepath.Abs(filepath.FromSlash(expandedPath))
		if err != nil {
			return nil, err
		}
		content, err := os.ReadFile(absPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				tabs = append(tabs, Tab{
					Title:            filepath.Base(absPath),
					FilePath:         absPath,
					Content:          []string{""},
					SelectionAnchorY: -1,
				})
				continue
			}
			return nil, err
		}
		text := strings.ReplaceAll(string(content), "\r\n", "\n")
		trailingNewline := strings.HasSuffix(text, "\n")
		lines := strings.Split(text, "\n")
		if len(lines) == 0 {
			lines = []string{""}
		}
		if len(lines) > 1 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		tabs = append(tabs, Tab{
			Title:            filepath.Base(absPath),
			FilePath:         absPath,
			Content:          lines,
			TrailingNewline:  trailingNewline,
			SelectionAnchorY: -1,
		})
	}
	return tabs, nil
}

func (a *App) Run() (err error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("create screen: %w", err)
	}

	if err := screen.Init(); err != nil {
		return fmt.Errorf("init screen: %w", err)
	}

	defer func() {
		maybePanic := recover()
		screen.Fini()
		if maybePanic != nil {
			panic(maybePanic)
		}
	}()

	a.screen = screen
	a.ensureCursorInBounds()

	for {
		a.draw()

		event := screen.PollEvent()
		switch ev := event.(type) {
		case *tcell.EventResize:
			screen.Sync()
		case *tcell.EventInterrupt:
			a.handleInterrupt(ev)
		case *tcell.EventKey:
			if quit := a.handleKey(ev); quit {
				return nil
			}
		}
	}
}

func (a *App) handleInterrupt(ev *tcell.EventInterrupt) {
	switch data := ev.Data().(type) {
	case spinnerTick:
		if a.agentRunning {
			a.spinnerIndex = (a.spinnerIndex + 1) % len(spinnerFrames)
		}
	case controlExecutionResult:
		a.finishControlExecution(data)
	}
}

func (a *App) handleKey(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyCtrlQ {
		a.quitConfirm = true
		a.statusMessage = "Quit confirmation opened. Press Enter or y to exit, Esc or n to stay."
		return false
	}

	if ev.Key() == tcell.KeyF1 {
		a.helpVisible = true
		a.helpScroll = 0
		a.historyVisible = false
		a.quitConfirm = false
		a.statusMessage = "Help opened. Press Esc to close."
		return false
	}

	if ev.Key() == tcell.KeyF3 {
		a.historyVisible = true
		a.historyScroll = 0
		a.helpVisible = false
		a.quitConfirm = false
		a.statusMessage = "History opened. Press Esc to close."
		return false
	}

	if ev.Key() == tcell.KeyRune && ev.Modifiers()&tcell.ModAlt != 0 {
		switch ev.Rune() {
		case '.':
			a.nextTab()
			return false
		case ',':
			a.prevTab()
			return false
		}
	}

	if a.quitConfirm {
		switch ev.Key() {
		case tcell.KeyEsc:
			a.quitConfirm = false
			a.statusMessage = "Quit cancelled."
			return false
		case tcell.KeyEnter:
			return true
		case tcell.KeyRune:
			switch ev.Rune() {
			case 'y', 'Y':
				return true
			case 'n', 'N':
				a.quitConfirm = false
				a.statusMessage = "Quit cancelled."
				return false
			}
		}
		return false
	}

	if a.helpVisible {
		switch ev.Key() {
		case tcell.KeyEsc:
			a.helpVisible = false
			a.helpScroll = 0
			a.statusMessage = "Help closed."
			return false
		case tcell.KeyUp:
			if a.helpScroll > 0 {
				a.helpScroll--
			}
			return false
		case tcell.KeyDown:
			if a.helpScroll < a.maxHelpScroll() {
				a.helpScroll++
			}
			return false
		}
		return false
	}

	if a.historyVisible {
		switch ev.Key() {
		case tcell.KeyEsc:
			a.historyVisible = false
			a.historyScroll = 0
			a.statusMessage = "History closed."
			return false
		case tcell.KeyUp:
			if a.historyScroll > 0 {
				a.historyScroll--
			}
			return false
		case tcell.KeyDown:
			if a.historyScroll < a.maxHistoryScroll() {
				a.historyScroll++
			}
			return false
		}
		return false
	}

	switch a.focus {
	case focusEditor:
		return a.handleEditorKey(ev)
	case focusControl:
		return a.handleControlKey(ev)
	default:
		return false
	}
}

func (a *App) handleEditorKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyCtrlG:
		a.focus = focusControl
		a.voiceState = "ready"
		a.statusMessage = "Control hub focused. Type a command and press Enter for preview."
		return false
	case tcell.KeyTab:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			a.nextTab()
		} else if a.hasSelection() {
			a.indentSelection()
		} else {
			a.insertRune('\t')
		}
		return false
	case tcell.KeyBacktab:
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			a.prevTab()
		} else {
			a.outdentSelection()
		}
		return false
	case tcell.KeyF2, tcell.KeyCtrlLeftSq, tcell.KeyCtrlSpace:
		a.selectCodeBlock()
		return false
	case tcell.KeyEsc:
		if a.hasSelection() {
			a.clearSelection("Selection cleared.")
		}
		return false
	case tcell.KeyUp:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == (tcell.ModCtrl | tcell.ModAlt) {
			a.statusMessage = "Ctrl+Alt+Up is reserved and does nothing."
			return false
		}
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			if a.hasSelection() {
				a.moveSelectedBlock(-1)
			} else {
				a.removeBlankLineAbove()
			}
			return false
		}
		if hasSelectionModifier(ev.Modifiers()) {
			a.moveCaretVertical(-1, true)
			return false
		}
		a.moveCaretVertical(-1, false)
	case tcell.KeyDown:
		if ev.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == (tcell.ModCtrl | tcell.ModAlt) {
			a.statusMessage = "Ctrl+Alt+Down is reserved and does nothing."
			return false
		}
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			if a.hasSelection() {
				a.moveSelectedBlock(1)
			} else {
				a.insertLineAbove()
			}
			return false
		}
		if hasSelectionModifier(ev.Modifiers()) {
			a.moveCaretVertical(1, true)
			return false
		}
		a.moveCaretVertical(1, false)
	case tcell.KeyLeft:
		if hasWordSelectionModifier(ev.Modifiers()) {
			a.moveCaretWord(-1, true)
			return false
		}
		if hasWordMoveModifier(ev.Modifiers()) {
			a.moveCaretWord(-1, false)
			return false
		}
		if hasSelectionModifier(ev.Modifiers()) {
			a.moveCaretHorizontal(-1, true)
			return false
		}
		a.moveCaretHorizontal(-1, false)
	case tcell.KeyRight:
		if hasWordSelectionModifier(ev.Modifiers()) {
			a.moveCaretWord(1, true)
			return false
		}
		if hasWordMoveModifier(ev.Modifiers()) {
			a.moveCaretWord(1, false)
			return false
		}
		if hasSelectionModifier(ev.Modifiers()) {
			a.moveCaretHorizontal(1, true)
			return false
		}
		a.moveCaretHorizontal(1, false)
	case tcell.KeyHome:
		a.moveToLineBoundary(true, hasSelectionModifier(ev.Modifiers()))
		return false
	case tcell.KeyEnd:
		a.moveToLineBoundary(false, hasSelectionModifier(ev.Modifiers()))
		return false
	case tcell.KeyPgUp:
		a.moveByPage(-1, hasSelectionModifier(ev.Modifiers()))
		return false
	case tcell.KeyPgDn:
		a.moveByPage(1, hasSelectionModifier(ev.Modifiers()))
		return false
	case tcell.KeyEnter:
		a.insertNewLine()
		return false
	case tcell.KeyCtrlA:
		a.selectAllEditor()
		return false
	case tcell.KeyCtrlS:
		a.saveActiveTab()
		return false
	case tcell.KeyCtrlC:
		a.copySelection()
		return false
	case tcell.KeyCtrlX:
		a.cutSelection()
		return false
	case tcell.KeyCtrlV:
		a.pasteClipboard()
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.backspaceEditor()
		return false
	case tcell.KeyDelete:
		a.deleteEditor()
		return false
	case tcell.KeyRune:
		if ev.Modifiers()&tcell.ModAlt != 0 {
			switch ev.Rune() {
			case '.':
				a.nextTab()
				return false
			case ',':
				a.prevTab()
				return false
			case '0', '1', '2', '3', '4':
				a.setIndentWidth(int(ev.Rune() - '0'))
				return false
			}
			a.statusMessage = "Unhandled Alt-modified key was ignored."
			return false
		}
		a.insertRune(ev.Rune())
		return false
	}

	a.ensureCursorInBounds()
	return false
}

func (a *App) insertRune(r rune) {
	if a.hasSelection() {
		a.replaceSelection(string(r))
		return
	}

	line := []rune(a.tabs[a.activeTab].Content[a.cursorY])
	if a.cursorX < 0 {
		a.cursorX = 0
	}
	if a.cursorX > len(line) {
		a.cursorX = len(line)
	}

	line = append(line[:a.cursorX], append([]rune{r}, line[a.cursorX:]...)...)
	line, a.cursorX = normalizeInsertedRunes(line, a.cursorX+1)
	a.tabs[a.activeTab].Content[a.cursorY] = string(line)
	a.markActiveTabDirty()
	a.statusMessage = "Inserted text in active edit surface."
}

func (a *App) insertString(text string) {
	if a.hasSelection() {
		a.replaceSelection(text)
		return
	}
	if strings.Contains(text, "\n") {
		a.insertTextAtCaret(text)
		return
	}
	for _, r := range []rune(text) {
		a.insertRune(r)
	}
}

func (a *App) insertTextAtCaret(text string) {
	content := a.tabs[a.activeTab].Content
	originalY := a.cursorY
	line := []rune(content[originalY])
	before := string(line[:a.cursorX])
	after := string(line[a.cursorX:])
	parts := strings.Split(text, "\n")
	updated := make([]string, 0, len(content)+len(parts)-1)
	updated = append(updated, content[:originalY]...)
	if len(parts) == 1 {
		updated = append(updated, before+parts[0]+after)
		a.cursorX += len([]rune(parts[0]))
	} else {
		updated = append(updated, before+parts[0])
		for _, middle := range parts[1 : len(parts)-1] {
			updated = append(updated, middle)
		}
		updated = append(updated, parts[len(parts)-1]+after)
		a.cursorY = originalY + len(parts) - 1
		a.cursorX = len([]rune(parts[len(parts)-1]))
	}
	updated = append(updated, content[originalY+1:]...)
	a.tabs[a.activeTab].Content = updated
	a.markActiveTabDirty()
	a.statusMessage = "Inserted text in active edit surface."
	a.ensureCursorInBounds()
}

func (a *App) indentUnit() string {
	if a.indentWidth == 0 {
		return "\t"
	}
	return strings.Repeat(" ", a.indentWidth)
}

func (a *App) setIndentWidth(width int) {
	if width < 0 {
		width = 0
	}
	if width > 8 {
		width = 8
	}
	a.indentWidth = width
	if width == 0 {
		a.statusMessage = "Indentation mode set to literal tabs."
		return
	}
	a.statusMessage = fmt.Sprintf("Indent width set to %d spaces.", width)
}

func (a *App) indentModeLabel() string {
	if a.indentWidth == 0 {
		return "tabs"
	}
	return fmt.Sprintf("spaces-%d", a.indentWidth)
}

func (a *App) insertNewLine() {
	if a.hasSelection() {
		a.replaceSelection("\n")
		return
	}

	line := []rune(a.tabs[a.activeTab].Content[a.cursorY])
	if a.cursorX < 0 {
		a.cursorX = 0
	}
	if a.cursorX > len(line) {
		a.cursorX = len(line)
	}

	before := string(line[:a.cursorX])
	after := string(line[a.cursorX:])
	content := a.tabs[a.activeTab].Content
	updated := make([]string, 0, len(content)+1)
	updated = append(updated, content[:a.cursorY]...)
	updated = append(updated, before, after)
	updated = append(updated, content[a.cursorY+1:]...)
	a.tabs[a.activeTab].Content = updated
	a.markActiveTabDirty()
	a.cursorY++
	a.cursorX = 0
	a.statusMessage = "Inserted a new line."
}

func (a *App) insertLineAbove() {
	content := a.tabs[a.activeTab].Content
	originalLine := a.cursorY
	originalColumn := a.cursorX
	updated := make([]string, 0, len(content)+1)
	updated = append(updated, content[:a.cursorY]...)
	updated = append(updated, "")
	updated = append(updated, content[a.cursorY:]...)
	a.tabs[a.activeTab].Content = updated
	a.markActiveTabDirty()
	a.cursorY = originalLine + 1
	a.cursorX = originalColumn
	a.statusMessage = fmt.Sprintf("Inserted a blank line above line %d while keeping the caret on its content line.", a.cursorY+1)
}

func (a *App) removeBlankLineAbove() {
	if a.cursorY == 0 {
		a.statusMessage = "There is no line above the cursor."
		return
	}
	index := a.cursorY - 1
	if !isVisuallyBlank(a.tabs[a.activeTab].Content[index]) {
		a.statusMessage = "The line above is not blank enough to remove."
		return
	}
	a.tabs[a.activeTab].Content = append(a.tabs[a.activeTab].Content[:index], a.tabs[a.activeTab].Content[index+1:]...)
	a.markActiveTabDirty()
	a.cursorY--
	a.statusMessage = fmt.Sprintf("Removed the blank line above line %d.", a.cursorY+1)
}

func (a *App) backspaceEditor() {
	if a.hasSelection() {
		a.replaceSelection("")
		return
	}

	if a.cursorX > 0 {
		line := []rune(a.tabs[a.activeTab].Content[a.cursorY])
		line = append(line[:a.cursorX-1], line[a.cursorX:]...)
		a.tabs[a.activeTab].Content[a.cursorY] = string(line)
		a.markActiveTabDirty()
		a.cursorX--
		a.statusMessage = "Deleted previous character."
		return
	}

	if a.cursorY == 0 {
		return
	}

	prevIndex := a.cursorY - 1
	prev := a.tabs[a.activeTab].Content[prevIndex]
	current := a.tabs[a.activeTab].Content[a.cursorY]
	a.tabs[a.activeTab].Content[prevIndex] = prev + current
	a.tabs[a.activeTab].Content = append(a.tabs[a.activeTab].Content[:a.cursorY], a.tabs[a.activeTab].Content[a.cursorY+1:]...)
	a.markActiveTabDirty()
	a.cursorY = prevIndex
	a.cursorX = len([]rune(prev))
	a.statusMessage = "Merged the current line into the previous line."
}

func (a *App) deleteEditor() {
	if a.hasSelection() {
		a.replaceSelection("")
		return
	}

	line := []rune(a.tabs[a.activeTab].Content[a.cursorY])
	if a.cursorX < len(line) {
		line = append(line[:a.cursorX], line[a.cursorX+1:]...)
		a.tabs[a.activeTab].Content[a.cursorY] = string(line)
		a.markActiveTabDirty()
		a.statusMessage = "Deleted character at cursor."
		return
	}

	if a.cursorY >= len(a.tabs[a.activeTab].Content)-1 {
		return
	}

	nextIndex := a.cursorY + 1
	next := a.tabs[a.activeTab].Content[nextIndex]
	a.tabs[a.activeTab].Content[a.cursorY] = a.tabs[a.activeTab].Content[a.cursorY] + next
	a.tabs[a.activeTab].Content = append(a.tabs[a.activeTab].Content[:nextIndex], a.tabs[a.activeTab].Content[nextIndex+1:]...)
	a.markActiveTabDirty()
	a.statusMessage = "Merged the next line into the current line."
}

func (a *App) handleControlKey(ev *tcell.EventKey) bool {
	if a.agentRunning {
		a.statusMessage = "Edit agent request is still running. Please wait."
		return false
	}
	switch ev.Key() {
	case tcell.KeyEsc:
		if a.hasControlSelection() {
			a.clearControlSelection()
			a.statusMessage = "Control selection cleared."
			return false
		}
		if a.preview.Pending {
			a.preview = Preview{}
			a.statusMessage = "Preview cleared. You can edit the command before submitting again."
			return false
		}
		a.focus = focusEditor
		a.voiceState = "off"
		a.statusMessage = "Returned to editor focus."
		return false
	case tcell.KeyCtrlG:
		a.focus = focusEditor
		a.voiceState = "off"
		a.statusMessage = "Returned to editor focus."
		return false
	case tcell.KeyCtrlA:
		if len(a.controlInput) == 0 {
			a.statusMessage = "Control hub is empty."
			return false
		}
		a.controlSelectAnchor = 0
		a.controlCursor = len(a.controlInput)
		a.ensureControlCursorInBounds()
		a.statusMessage = "Selected all control input."
		return false
	case tcell.KeyEnter:
		input := strings.TrimSpace(string(a.controlInput))
		if input == "" {
			a.statusMessage = "Control hub is empty."
			return false
		}

		if !a.preview.Pending {
			a.preview = BuildPreview(input, a.currentScope(), a.tabs[a.activeTab].Title)
			if !commandRequiresConfirmation(a.preview.Kind) {
				if a.startControlExecution(input) {
					return false
				}
				response, err := a.executePreview(input)
				if err != nil {
					a.agentRunning = false
					a.stopSpinner()
					a.voiceState = "ready"
					a.statusMessage = err.Error()
					return false
				}
				a.completeControlExecution(response)
				return false
			}
			a.statusMessage = "Preview ready for the current scope. Press Enter again to confirm or Esc to edit."
			a.voiceState = "captured"
			return false
		}

		if a.startControlExecution(input) {
			return false
		}
		response, err := a.executePreview(input)
		if err != nil {
			a.agentRunning = false
			a.stopSpinner()
			a.voiceState = "ready"
			a.statusMessage = err.Error()
			return false
		}
		a.completeControlExecution(response)
		return false
	case tcell.KeyCtrlC:
		a.copyControlInput()
		return false
	case tcell.KeyCtrlX:
		a.cutControlInput()
		return false
	case tcell.KeyCtrlV:
		a.pasteControlInput()
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if a.hasControlSelection() {
			a.replaceControlSelection("")
			return false
		}
		if a.controlCursor > 0 {
			a.clearLastAgentReply()
			a.controlInput = append(a.controlInput[:a.controlCursor-1], a.controlInput[a.controlCursor:]...)
			a.controlCursor--
			a.ensureControlCursorInBounds()
			a.preview = Preview{}
		}
		return false
	case tcell.KeyDelete:
		if a.hasControlSelection() {
			a.replaceControlSelection("")
			return false
		}
		if a.controlCursor < len(a.controlInput) {
			a.clearLastAgentReply()
			a.controlInput = append(a.controlInput[:a.controlCursor], a.controlInput[a.controlCursor+1:]...)
			a.ensureControlCursorInBounds()
			a.preview = Preview{}
		}
		return false
	case tcell.KeyLeft:
		if hasWordSelectionModifier(ev.Modifiers()) {
			a.moveControlWord(-1, true)
			return false
		}
		if hasWordMoveModifier(ev.Modifiers()) {
			a.moveControlWord(-1, false)
			return false
		}
		if hasSelectionModifier(ev.Modifiers()) {
			a.moveControlCursor(-1, true)
			return false
		}
		if a.controlCursor > 0 {
			a.controlCursor--
		}
		a.clearControlSelection()
		a.ensureControlCursorInBounds()
		return false
	case tcell.KeyRight:
		if hasWordSelectionModifier(ev.Modifiers()) {
			a.moveControlWord(1, true)
			return false
		}
		if hasWordMoveModifier(ev.Modifiers()) {
			a.moveControlWord(1, false)
			return false
		}
		if hasSelectionModifier(ev.Modifiers()) {
			a.moveControlCursor(1, true)
			return false
		}
		if a.controlCursor < len(a.controlInput) {
			a.controlCursor++
		}
		a.clearControlSelection()
		a.ensureControlCursorInBounds()
		return false
	case tcell.KeyHome:
		a.moveControlBoundary(true, hasSelectionModifier(ev.Modifiers()))
		return false
	case tcell.KeyEnd:
		a.moveControlBoundary(false, hasSelectionModifier(ev.Modifiers()))
		return false
	case tcell.KeyRune:
		if ev.Modifiers()&tcell.ModAlt != 0 {
			a.statusMessage = "Unhandled Alt-modified control key was ignored."
			return false
		}
		r := ev.Rune()
		if a.hasControlSelection() {
			a.clearLastAgentReply()
			a.replaceControlSelection(string(r))
			return false
		}
		a.clearLastAgentReply()
		a.controlInput = append(a.controlInput[:a.controlCursor], append([]rune{r}, a.controlInput[a.controlCursor:]...)...)
		a.controlInput, a.controlCursor = normalizeInsertedRunes(a.controlInput, a.controlCursor+1)
		a.ensureControlCursorInBounds()
		a.preview = Preview{}
		a.voiceState = "listening"
		return false
	}

	return false
}

func (a *App) executePreview(input string) (agent.Response, error) {
	if a.preview.Kind == CommandMemo {
		return a.saveCurrentFileMemo(input)
	}
	if a.preview.Kind == CommandOpen {
		return a.openFileCommand(input)
	}
	if a.preview.Kind == CommandWrite {
		return a.writeFileCommand(input)
	}
	if a.agentExecutor == nil {
		return agent.Response{}, errors.New("edit agent is not configured or ready")
	}

	response, err := a.agentExecutor.Execute(context.Background(), agent.Request{
		Command:   input,
		Action:    a.preview.Action,
		Kind:      string(a.preview.Kind),
		Scope:     a.preview.Target,
		Tab:       a.preview.Tab,
		Selection: a.selectedTextOrEmpty(),
		Document:  a.currentDocumentText(),
		Workspace: a.workspaceRoot,
	})
	if err != nil {
		return agent.Response{}, fmt.Errorf("edit agent failed: %w", err)
	}

	if err := a.applyAgentResponse(response); err != nil {
		return agent.Response{}, err
	}

	return response, nil
}

func (a *App) beginControlExecution() {
	a.executingControlInput = string(a.controlInput)
	a.agentRunning = true
	a.spinnerIndex = 0
	a.voiceState = "sending"
	switch a.preview.Kind {
	case CommandMemo:
		a.statusMessage = "Saving memo for the current target..."
	case CommandOpen:
		a.statusMessage = "Opening file in a new tab..."
	case CommandWrite:
		a.statusMessage = "Saving file to the requested path..."
	default:
		a.statusMessage = "Sending request to the edit agent..."
	}
	if a.screen != nil {
		a.draw()
	}
}

func (a *App) startSpinner() {
	if a.screen == nil {
		return
	}
	if a.spinnerStop != nil {
		close(a.spinnerStop)
	}
	stop := make(chan struct{})
	a.spinnerStop = stop
	go func() {
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.screen.PostEventWait(tcell.NewEventInterrupt(spinnerTick{}))
			case <-stop:
				return
			}
		}
	}()
}

func (a *App) stopSpinner() {
	if a.spinnerStop != nil {
		close(a.spinnerStop)
		a.spinnerStop = nil
	}
	a.spinnerIndex = 0
}

func (a *App) executeAgentRequest(req agent.Request) (agent.Response, error) {
	if a.agentExecutor == nil {
		return agent.Response{}, errors.New("edit agent is not configured or ready")
	}
	response, err := a.agentExecutor.Execute(context.Background(), req)
	if err != nil {
		return agent.Response{}, fmt.Errorf("edit agent failed: %w", err)
	}
	return response, nil
}

func (a *App) startControlExecution(input string) bool {
	if a.screen == nil || a.preview.Kind == CommandMemo || a.preview.Kind == CommandOpen || a.preview.Kind == CommandWrite {
		a.beginControlExecution()
		return false
	}
	request := agent.Request{
		Command:   input,
		Action:    a.preview.Action,
		Kind:      string(a.preview.Kind),
		Scope:     a.preview.Target,
		Tab:       a.preview.Tab,
		Selection: a.selectedTextOrEmpty(),
		Document:  a.currentDocumentText(),
		Workspace: a.workspaceRoot,
	}
	a.beginControlExecution()
	a.startSpinner()
	go func() {
		response, err := a.executeAgentRequest(request)
		if a.screen != nil {
			a.screen.PostEventWait(tcell.NewEventInterrupt(controlExecutionResult{response: response, err: err}))
		}
	}()
	return true
}

func (a *App) finishControlExecution(result controlExecutionResult) {
	a.agentRunning = false
	a.stopSpinner()
	if result.err != nil {
		a.statusMessage = result.err.Error()
		a.voiceState = "ready"
		return
	}
	if err := a.applyAgentResponse(result.response); err != nil {
		a.statusMessage = err.Error()
		a.voiceState = "ready"
		return
	}
	a.completeControlExecution(result.response)
}

func (a *App) completeControlExecution(response agent.Response) {
	a.agentRunning = false
	a.stopSpinner()
	a.lastCommand = a.preview.Summary()
	kind := a.preview.Kind
	a.lastAgentReply = response.Message
	if kind != CommandMemo && kind != CommandOpen && kind != CommandWrite {
		a.lastAgentResult = response.Message
	}
	a.appendActiveTabHistory(a.executingControlInput, response.Message)
	a.statusMessage = response.Message
	a.preview = Preview{}
	a.executingControlInput = ""
	a.controlInput = nil
	a.controlCursor = 0
	a.clearControlSelection()
	a.ensureControlCursorInBounds()
	a.focus = focusControl
	a.voiceState = "ready"
}

func (a *App) saveCurrentFileMemo(input string) (agent.Response, error) {
	current := a.tabs[a.activeTab]
	if strings.TrimSpace(current.FilePath) == "" {
		return agent.Response{}, errors.New("current tab is not backed by a real file")
	}
	note := extractMemoNote(input)
	if strings.TrimSpace(note) == "" {
		return agent.Response{}, errors.New("memo note is empty")
	}
	note = a.resolveMemoContent(input, note)
	target, err := memo.SaveFileMemoDetailed(a.systemMemoRoot, a.workspaceRoot, current.FilePath, note)
	if err != nil {
		return agent.Response{}, fmt.Errorf("failed to save memo: %w", err)
	}
	return agent.Response{Mode: agent.ModeMessage, Message: formatMemoSaveMessage(target)}, nil
}

func (a *App) openFileCommand(input string) (agent.Response, error) {
	path, ok := openCommandPayload(input)
	if !ok {
		return agent.Response{}, errors.New("open path is empty")
	}
	resolvedPath, err := config.ExpandUserPath(path)
	if err != nil {
		return agent.Response{}, fmt.Errorf("failed to resolve open path: %w", err)
	}
	absPath, err := filepath.Abs(filepath.FromSlash(resolvedPath))
	if err != nil {
		return agent.Response{}, fmt.Errorf("failed to resolve open path: %w", err)
	}
	for index, tab := range a.tabs {
		if filepath.Clean(tab.FilePath) != filepath.Clean(absPath) {
			continue
		}
		a.saveCurrentTabState()
		a.activeTab = index
		a.loadCurrentTabState()
		return agent.Response{Mode: agent.ModeMessage, Message: "Switched to " + filepath.ToSlash(absPath)}, nil
	}
	newTabs, err := loadTabsFromPaths([]string{absPath})
	if err != nil {
		return agent.Response{}, fmt.Errorf("failed to open file: %w", err)
	}
	if len(newTabs) == 0 {
		return agent.Response{}, errors.New("no tab was created for the open path")
	}
	a.saveCurrentTabState()
	a.tabs = append(a.tabs, newTabs[0])
	a.activeTab = len(a.tabs) - 1
	a.loadCurrentTabState()
	return agent.Response{Mode: agent.ModeMessage, Message: "Opened " + filepath.ToSlash(absPath)}, nil
}

func (a *App) saveActiveTab() {
	current := &a.tabs[a.activeTab]
	if strings.TrimSpace(current.FilePath) == "" {
		a.statusMessage = "Current tab is not backed by a file. Open a file path to save changes."
		return
	}
	if err := a.writeTabToPath(current, current.FilePath); err != nil {
		a.statusMessage = fmt.Sprintf("Failed to save %s: %v", filepath.ToSlash(current.FilePath), err)
		return
	}
	current.Dirty = false
	a.statusMessage = "Saved " + filepath.ToSlash(current.FilePath)
}

func (a *App) writeTabToPath(tab *Tab, targetPath string) error {
	if strings.TrimSpace(targetPath) == "" {
		return errors.New("target path is empty")
	}
	parentDir := filepath.Dir(targetPath)
	if strings.TrimSpace(parentDir) != "" && parentDir != "." {
		if err := os.MkdirAll(parentDir, 0o755); err != nil {
			return err
		}
	}
	content := strings.Join(tab.Content, "\n")
	if tab.TrailingNewline {
		content += "\n"
	}
	perm := os.FileMode(0o644)
	if info, err := os.Stat(targetPath); err == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(targetPath, []byte(content), perm); err != nil {
		return err
	}
	return nil
}

func (a *App) writeFileCommand(input string) (agent.Response, error) {
	path, ok := writeCommandPayload(input)
	if !ok {
		return agent.Response{}, errors.New("write path is empty")
	}
	resolvedPath, err := config.ExpandUserPath(path)
	if err != nil {
		return agent.Response{}, fmt.Errorf("failed to resolve write path: %w", err)
	}
	absPath, err := filepath.Abs(filepath.FromSlash(resolvedPath))
	if err != nil {
		return agent.Response{}, fmt.Errorf("failed to resolve write path: %w", err)
	}
	current := &a.tabs[a.activeTab]
	if err := a.writeTabToPath(current, absPath); err != nil {
		return agent.Response{}, fmt.Errorf("failed to write file: %w", err)
	}
	current.FilePath = absPath
	current.Title = filepath.Base(absPath)
	current.Dirty = false
	return agent.Response{Mode: agent.ModeMessage, Message: "Saved " + filepath.ToSlash(absPath)}, nil
}

func (a *App) resolveMemoContent(input, extracted string) string {
	trimmed := strings.TrimSpace(input)
	if _, payload, ok := explicitMemoPayload(trimmed); ok {
		return payload
	}
	if (strings.HasPrefix(trimmed, "memo ") || strings.HasPrefix(trimmed, "메모 ")) && strings.TrimSpace(extracted) != "" {
		return extracted
	}
	if isNaturalLanguageMemoRequest(trimmed) && strings.TrimSpace(a.lastAgentResult) != "" {
		return a.lastAgentResult
	}
	return extracted
}

func formatMemoSaveMessage(target memo.Target) string {
	label := string(target.Scope)
	if strings.TrimSpace(target.Name) != "" {
		label += "(" + target.Name + ")"
	}
	return "Saved " + label + " memo to " + filepath.ToSlash(target.Path)
}

func extractMemoNote(input string) string {
	payload, ok := memoCommandPayload(input)
	if ok {
		return payload
	}
	return ""
}

func (a *App) applyAgentResponse(response agent.Response) error {
	switch response.Mode {
	case agent.ModeMessage:
		return nil
	case agent.ModeReplaceSelection:
		if !a.hasSelection() {
			return errors.New("edit agent requested selection replacement, but no selection is active")
		}
		a.replaceSelection(response.Content)
		return nil
	case agent.ModeReplaceDocument:
		a.replaceDocument(response.Content)
		return nil
	default:
		return fmt.Errorf("edit agent returned unsupported mode: %s", response.Mode)
	}
}

func (a *App) selectedTextOrEmpty() string {
	if !a.hasSelection() {
		return ""
	}
	return a.selectedText()
}

func (a *App) currentDocumentText() string {
	return strings.Join(a.tabs[a.activeTab].Content, "\n")
}

func (a *App) replaceDocument(content string) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	a.tabs[a.activeTab].Content = lines
	a.markActiveTabDirty()
	a.cursorY = 0
	a.cursorX = 0
	a.selectionAnchor = -1
	a.selectionAnchorCol = 0
	a.ensureCursorInBounds()
}

func (a *App) copyControlInput() {
	text := string(a.controlInput)
	if a.hasControlSelection() {
		text = a.selectedControlText()
	}
	if text == "" {
		a.statusMessage = "Control hub is empty."
		return
	}
	a.clipboard = text
	if err := clipboardWrite(a.clipboard); err != nil {
		a.statusMessage = "Copied control input to internal clipboard; system clipboard unavailable."
		return
	}
	a.statusMessage = "Copied control input to system clipboard."
}

func (a *App) cutControlInput() {
	text := string(a.controlInput)
	if a.hasControlSelection() {
		text = a.selectedControlText()
	}
	if text == "" {
		a.statusMessage = "Control hub is empty."
		return
	}
	a.clipboard = text
	if err := clipboardWrite(a.clipboard); err != nil {
		a.statusMessage = "Cut control input to internal clipboard; system clipboard unavailable."
	} else {
		a.statusMessage = "Cut control input to system clipboard."
	}
	if a.hasControlSelection() {
		a.replaceControlSelection("")
	} else {
		a.controlInput = nil
		a.controlCursor = 0
		a.clearControlSelection()
		a.ensureControlCursorInBounds()
		a.preview = Preview{}
	}
}

func (a *App) pasteControlInput() {
	a.clearLastAgentReply()
	text := a.clipboard
	if external, err := clipboardRead(); err == nil && external != "" {
		text = external
		a.clipboard = external
	}
	if text == "" {
		a.statusMessage = "Clipboard is empty."
		return
	}
	if a.hasControlSelection() {
		a.replaceControlSelection(text)
		a.statusMessage = "Pasted clipboard into control hub."
		return
	}
	for _, r := range []rune(text) {
		a.controlInput = append(a.controlInput[:a.controlCursor], append([]rune{r}, a.controlInput[a.controlCursor:]...)...)
		a.controlCursor++
	}
	a.ensureControlCursorInBounds()
	a.preview = Preview{}
	a.voiceState = "listening"
	a.statusMessage = "Pasted clipboard into control hub."
}

func (a *App) hasControlSelection() bool {
	if a.controlSelectAnchor < 0 {
		return false
	}
	return a.controlSelectAnchor != a.controlCursor
}

func (a *App) clearControlSelection() {
	a.controlSelectAnchor = -1
}

func (a *App) controlSelectionRange() (int, int, bool) {
	if !a.hasControlSelection() {
		return 0, 0, false
	}
	start := a.controlSelectAnchor
	end := a.controlCursor
	if start > end {
		start, end = end, start
	}
	return start, end, true
}

func (a *App) moveControlCursor(delta int, selecting bool) {
	if selecting {
		if a.controlSelectAnchor < 0 {
			a.controlSelectAnchor = a.controlCursor
		}
	} else {
		a.clearControlSelection()
	}
	a.controlCursor += delta
	a.ensureControlCursorInBounds()
}

func (a *App) moveControlBoundary(toStart bool, selecting bool) {
	if selecting {
		if a.controlSelectAnchor < 0 {
			a.controlSelectAnchor = a.controlCursor
		}
	} else {
		a.clearControlSelection()
	}
	if toStart {
		a.controlCursor = 0
		a.ensureControlCursorInBounds()
		return
	}
	a.controlCursor = len(a.controlInput)
	a.ensureControlCursorInBounds()
}

func (a *App) moveControlWord(direction int, selecting bool) {
	if selecting {
		if a.controlSelectAnchor < 0 {
			a.controlSelectAnchor = a.controlCursor
		}
	} else {
		a.clearControlSelection()
	}
	if direction < 0 {
		if a.controlCursor == 0 {
			return
		}
		index := a.controlCursor
		for index > 0 && !isWordRune(a.controlInput[index-1]) {
			index--
		}
		for index > 0 && isWordRune(a.controlInput[index-1]) {
			index--
		}
		a.controlCursor = index
		a.ensureControlCursorInBounds()
		return
	}
	if a.controlCursor >= len(a.controlInput) {
		return
	}
	index := a.controlCursor
	for index < len(a.controlInput) && isWordRune(a.controlInput[index]) {
		index++
	}
	for index < len(a.controlInput) && !isWordRune(a.controlInput[index]) {
		index++
	}
	a.controlCursor = index
	a.ensureControlCursorInBounds()
}

func (a *App) ensureControlCursorInBounds() {
	if a.controlCursor < 0 {
		a.controlCursor = 0
	}
	if a.controlCursor > len(a.controlInput) {
		a.controlCursor = len(a.controlInput)
	}
	if a.controlSelectAnchor > len(a.controlInput) {
		a.controlSelectAnchor = len(a.controlInput)
	}
	a.syncControlViewport()
}

func (a *App) selectedControlText() string {
	start, end, ok := a.controlSelectionRange()
	if !ok {
		return ""
	}
	return string(a.controlInput[start:end])
}

func (a *App) replaceControlSelection(text string) {
	a.clearLastAgentReply()
	start, end, ok := a.controlSelectionRange()
	if !ok {
		return
	}
	replacement := []rune(text)
	a.controlInput = append(append([]rune(nil), a.controlInput[:start]...), append(replacement, a.controlInput[end:]...)...)
	a.controlCursor = start + len(replacement)
	a.clearControlSelection()
	a.ensureControlCursorInBounds()
	a.preview = Preview{}
	a.voiceState = "listening"
}

func (a *App) clearLastAgentReply() {
	a.lastAgentReply = ""
}

func (a *App) nextTab() {
	a.saveCurrentTabState()
	a.activeTab = (a.activeTab + 1) % len(a.tabs)
	a.loadCurrentTabState()
	a.historyScroll = 0
	a.statusMessage = "Switched to tab " + a.tabs[a.activeTab].Title
}

func (a *App) prevTab() {
	a.saveCurrentTabState()
	a.activeTab--
	if a.activeTab < 0 {
		a.activeTab = len(a.tabs) - 1
	}
	a.loadCurrentTabState()
	a.historyScroll = 0
	a.statusMessage = "Switched to tab " + a.tabs[a.activeTab].Title
}

func (a *App) saveCurrentTabState() {
	if len(a.tabs) == 0 || a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return
	}
	tab := &a.tabs[a.activeTab]
	tab.ViewTop = a.viewportTop
	tab.ViewLeft = a.viewportLeft
	tab.CursorX = a.cursorX
	tab.CursorY = a.cursorY
	tab.SelectionAnchorX = a.selectionAnchorX()
	tab.SelectionAnchorY = a.selectionAnchor
	tab.LastAgentReply = a.lastAgentReply
	tab.LastAgentResult = a.lastAgentResult
}

func (a *App) markActiveTabDirty() {
	if len(a.tabs) == 0 || a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return
	}
	a.tabs[a.activeTab].Dirty = true
}

func (a *App) appendActiveTabHistory(prompt, reply string) {
	if len(a.tabs) == 0 || a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return
	}
	prompt = strings.TrimSpace(prompt)
	reply = strings.TrimSpace(reply)
	if prompt == "" && reply == "" {
		return
	}
	tab := &a.tabs[a.activeTab]
	tab.History = append(tab.History, historyEntry{Prompt: prompt, Reply: reply})
	if len(tab.History) > 100 {
		tab.History = append([]historyEntry(nil), tab.History[len(tab.History)-100:]...)
	}
}

func (a *App) loadCurrentTabState() {
	if len(a.tabs) == 0 || a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return
	}
	tab := &a.tabs[a.activeTab]
	a.viewportTop = tab.ViewTop
	a.viewportLeft = tab.ViewLeft
	a.cursorX = tab.CursorX
	a.cursorY = tab.CursorY
	a.setSelectionAnchor(tab.SelectionAnchorX, tab.SelectionAnchorY)
	a.lastAgentReply = tab.LastAgentReply
	a.lastAgentResult = tab.LastAgentResult
	a.ensureCursorInBounds()
	a.syncViewport()
	tab.CursorX = a.cursorX
	tab.CursorY = a.cursorY
	tab.ViewTop = a.viewportTop
	tab.ViewLeft = a.viewportLeft
	tab.SelectionAnchorX = a.selectionAnchorX()
	tab.SelectionAnchorY = a.selectionAnchor
}

func (a *App) currentScope() string {
	if start, end, ok := a.selectionRange(); ok {
		return fmt.Sprintf("selection:L%d:C%d-L%d:C%d", start.y+1, start.x+1, end.y+1, end.x+1)
	}
	return fmt.Sprintf("caret:L%d:C%d", a.cursorY+1, a.cursorX+1)
}

func (a *App) hasSelection() bool {
	if a.selectionAnchor < 0 {
		return false
	}
	return a.selectionAnchor != a.cursorY || a.selectionAnchorCol != a.cursorX
}

func (a *App) selectionAnchorX() int {
	return a.selectionAnchorCol
}

func (a *App) setSelectionAnchor(x, y int) {
	a.selectionAnchorCol = x
	a.selectionAnchor = y
}

func compareTextPos(left, right textPos) int {
	if left.y != right.y {
		if left.y < right.y {
			return -1
		}
		return 1
	}
	if left.x < right.x {
		return -1
	}
	if left.x > right.x {
		return 1
	}
	return 0
}

func hasSelectionModifier(mod tcell.ModMask) bool {
	return mod&(tcell.ModShift|tcell.ModAlt) != 0
}

func hasWordSelectionModifier(mod tcell.ModMask) bool {
	return mod&tcell.ModCtrl != 0 && mod&(tcell.ModShift|tcell.ModAlt) != 0
}

func hasWordMoveModifier(mod tcell.ModMask) bool {
	return mod&tcell.ModCtrl != 0
}

func (a *App) caretPos() textPos {
	return textPos{x: a.cursorX, y: a.cursorY}
}

func (a *App) anchorPos() textPos {
	return textPos{x: a.selectionAnchorCol, y: a.selectionAnchor}
}

func (a *App) selectionRange() (textPos, textPos, bool) {
	if !a.hasSelection() {
		return textPos{}, textPos{}, false
	}
	start := a.anchorPos()
	end := a.caretPos()
	if compareTextPos(start, end) > 0 {
		start, end = end, start
	}
	return start, end, true
}

func (a *App) projectedLineRange() (int, int, bool) {
	start, end, ok := a.selectionRange()
	if !ok {
		return 0, 0, false
	}
	return start.y, end.y, true
}

func (a *App) startSelectionIfNeeded() {
	if a.selectionAnchor < 0 {
		a.setSelectionAnchor(a.cursorX, a.cursorY)
	}
}

func (a *App) moveCaretHorizontal(delta int, selecting bool) {
	if selecting {
		a.startSelectionIfNeeded()
	} else if a.hasSelection() {
		a.clearSelection("Selection cleared.")
	}
	line := []rune(a.tabs[a.activeTab].Content[a.cursorY])
	if delta < 0 {
		if a.cursorX > 0 {
			a.cursorX--
		} else if a.cursorY > 0 {
			a.cursorY--
			a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
		}
	} else if delta > 0 {
		if a.cursorX < len(line) {
			a.cursorX++
		} else if a.cursorY < len(a.tabs[a.activeTab].Content)-1 {
			a.cursorY++
			a.cursorX = 0
		}
	}
	a.ensureCursorInBounds()
}

func (a *App) moveCaretVertical(delta int, selecting bool) {
	if selecting {
		a.startSelectionIfNeeded()
	} else if a.hasSelection() {
		a.clearSelection("Selection cleared.")
	}
	a.cursorY += delta
	a.ensureCursorInBounds()
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func (a *App) moveCaretWord(direction int, selecting bool) {
	if selecting {
		a.startSelectionIfNeeded()
	} else if a.hasSelection() {
		a.clearSelection("Selection cleared.")
	}
	line := []rune(a.tabs[a.activeTab].Content[a.cursorY])
	if direction < 0 {
		if a.cursorX == 0 {
			return
		}
		index := a.cursorX
		for index > 0 && !isWordRune(line[index-1]) {
			index--
		}
		for index > 0 && isWordRune(line[index-1]) {
			index--
		}
		a.cursorX = index
		return
	}
	if a.cursorX >= len(line) {
		return
	}
	index := a.cursorX
	for index < len(line) && isWordRune(line[index]) {
		index++
	}
	for index < len(line) && !isWordRune(line[index]) {
		index++
	}
	a.cursorX = index
}

func (a *App) moveToLineBoundary(toStart bool, selecting bool) {
	if selecting {
		a.startSelectionIfNeeded()
	} else if a.hasSelection() {
		a.clearSelection("Selection cleared.")
	}
	if toStart {
		a.cursorX = 0
		return
	}
	a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
}

func (a *App) moveByPage(direction int, selecting bool) {
	if selecting {
		a.startSelectionIfNeeded()
	} else if a.hasSelection() {
		a.clearSelection("Selection cleared.")
	}
	step := 12
	a.cursorY += direction * step
	a.ensureCursorInBounds()
}

func (a *App) selectionContains(pos textPos) bool {
	start, end, ok := a.selectionRange()
	if !ok {
		return false
	}
	return compareTextPos(start, pos) <= 0 && compareTextPos(pos, end) < 0
}

func (a *App) selectedText() string {
	start, end, ok := a.selectionRange()
	if !ok {
		return ""
	}
	content := a.tabs[a.activeTab].Content
	if start.y == end.y {
		line := []rune(content[start.y])
		return string(line[start.x:end.x])
	}
	parts := make([]string, 0, end.y-start.y+1)
	first := []rune(content[start.y])
	parts = append(parts, string(first[start.x:]))
	for index := start.y + 1; index < end.y; index++ {
		parts = append(parts, content[index])
	}
	last := []rune(content[end.y])
	parts = append(parts, string(last[:end.x]))
	return strings.Join(parts, "\n")
}

func (a *App) replaceSelection(replacement string) {
	start, end, ok := a.selectionRange()
	if !ok {
		return
	}
	content := a.tabs[a.activeTab].Content
	first := []rune(content[start.y])
	last := []rune(content[end.y])
	before := string(first[:start.x])
	after := string(last[end.x:])
	replacementLines := strings.Split(replacement, "\n")
	newLines := make([]string, 0, len(content)-(end.y-start.y)+len(replacementLines))
	newLines = append(newLines, content[:start.y]...)
	if len(replacementLines) == 1 {
		newLines = append(newLines, before+replacementLines[0]+after)
		a.cursorY = start.y
		a.cursorX = len([]rune(before + replacementLines[0]))
	} else {
		newLines = append(newLines, before+replacementLines[0])
		for _, line := range replacementLines[1 : len(replacementLines)-1] {
			newLines = append(newLines, line)
		}
		newLines = append(newLines, replacementLines[len(replacementLines)-1]+after)
		a.cursorY = start.y + len(replacementLines) - 1
		a.cursorX = len([]rune(replacementLines[len(replacementLines)-1]))
	}
	newLines = append(newLines, content[end.y+1:]...)
	a.tabs[a.activeTab].Content = newLines
	a.markActiveTabDirty()
	a.clearSelection("Selection replaced.")
	a.ensureCursorInBounds()
	if replacement == "" {
		a.statusMessage = "Deleted selected text."
	} else {
		a.statusMessage = "Inserted text in active edit surface."
	}
}

func (a *App) copySelection() {
	if !a.hasSelection() {
		a.statusMessage = "No selection to copy."
		return
	}
	a.clipboard = a.selectedText()
	if err := clipboardWrite(a.clipboard); err != nil {
		a.statusMessage = "Copied selection to internal clipboard; system clipboard unavailable."
		return
	}
	a.statusMessage = "Copied selection to system clipboard."
}

func (a *App) cutSelection() {
	if !a.hasSelection() {
		a.statusMessage = "No selection to cut."
		return
	}
	a.clipboard = a.selectedText()
	if err := clipboardWrite(a.clipboard); err != nil {
		a.statusMessage = "Cut selection to internal clipboard; system clipboard unavailable."
		a.replaceSelection("")
		return
	}
	a.replaceSelection("")
	a.statusMessage = "Cut selection to system clipboard."
}

func (a *App) pasteClipboard() {
	clipboardText := a.clipboard
	if external, err := clipboardRead(); err == nil && external != "" {
		clipboardText = external
		a.clipboard = external
	}
	if clipboardText == "" {
		a.statusMessage = "Clipboard is empty."
		return
	}
	a.insertString(clipboardText)
	a.statusMessage = "Pasted clipboard at caret."
}

func (a *App) selectAllEditor() {
	if len(a.tabs[a.activeTab].Content) == 0 {
		a.statusMessage = "Nothing to select."
		return
	}
	lastLine := len(a.tabs[a.activeTab].Content) - 1
	lastCol := len([]rune(a.tabs[a.activeTab].Content[lastLine]))
	a.setSelectionAnchor(0, 0)
	a.cursorY = lastLine
	a.cursorX = lastCol
	a.statusMessage = "Selected entire document."
}

func (a *App) clearSelection(message string) {
	a.selectionAnchor = -1
	a.selectionAnchorCol = 0
	a.statusMessage = message
}

func (a *App) expandSelection(delta int) {
	if !a.hasSelection() {
		a.setSelectionAnchor(a.cursorX, a.cursorY)
	}
	a.cursorY += delta
	a.ensureCursorInBounds()
	start, end, _ := a.selectionRange()
	if compareTextPos(start, end) == 0 {
		a.statusMessage = fmt.Sprintf("Selection anchored on line %d.", start.y+1)
		return
	}
	a.statusMessage = fmt.Sprintf("Selection spans lines %d-%d.", start.y+1, end.y+1)
}

func (a *App) moveSelectedBlock(delta int) {
	if delta == 0 {
		return
	}
	start, end, ok := a.projectedLineRange()
	if !ok {
		a.setSelectionAnchor(a.cursorX, a.cursorY)
		start, end, _ = a.projectedLineRange()
	}

	content := a.tabs[a.activeTab].Content
	if delta < 0 && start == 0 {
		a.statusMessage = "Selection is already at the top edge."
		return
	}
	if delta > 0 && end >= len(content)-1 {
		a.statusMessage = "Selection is already at the bottom edge."
		return
	}

	if delta < 0 {
		aboveLine := content[start-1]
		selected := append([]string(nil), content[start:end+1]...)
		copy(content[start-1:], append(selected, aboveLine))
		start--
		end--
	} else {
		belowLine := content[end+1]
		selected := append([]string(nil), content[start:end+1]...)
		copy(content[start:], append([]string{belowLine}, selected...))
		start++
		end++
	}

	a.tabs[a.activeTab].Content = content
	a.markActiveTabDirty()
	a.setSelectionAnchor(a.selectionAnchorX(), start)
	a.cursorY = end
	a.ensureCursorInBounds()
	a.statusMessage = fmt.Sprintf("Moved selected block to lines %d-%d.", start+1, end+1)
}

func (a *App) activeLineRange() (int, int) {
	if start, end, ok := a.projectedLineRange(); ok {
		return start, end
	}
	return a.cursorY, a.cursorY
}

func (a *App) indentSelection() {
	start, end := a.activeLineRange()
	indentWidth := len([]rune(a.indentUnit()))

	for index := start; index <= end; index++ {
		a.tabs[a.activeTab].Content[index] = a.indentUnit() + a.tabs[a.activeTab].Content[index]
	}
	a.markActiveTabDirty()
	if a.hasSelection() {
		a.selectionAnchorCol += indentWidth
		a.cursorX += indentWidth
	} else {
		a.cursorX += indentWidth
	}
	a.ensureCursorInBounds()
	if start == end {
		a.statusMessage = fmt.Sprintf("Indented line %d.", start+1)
	} else {
		a.statusMessage = fmt.Sprintf("Indented lines %d-%d.", start+1, end+1)
	}
}

func (a *App) outdentSelection() {
	start, end := a.activeLineRange()

	removedOnAnchor := 0
	removedOnCursor := 0
	changed := false
	for index := start; index <= end; index++ {
		updated, removed := a.removeSingleIndent(a.tabs[a.activeTab].Content[index])
		a.tabs[a.activeTab].Content[index] = updated
		if removed > 0 {
			changed = true
		}
		if index == a.cursorY {
			removedOnCursor = removed
		}
		if index == a.selectionAnchor {
			removedOnAnchor = removed
		}
	}
	if changed {
		a.markActiveTabDirty()
	}
	if a.hasSelection() {
		a.selectionAnchorCol -= removedOnAnchor
		if a.selectionAnchorCol < 0 {
			a.selectionAnchorCol = 0
		}
		a.cursorX -= removedOnCursor
		if a.cursorX < 0 {
			a.cursorX = 0
		}
	} else {
		a.cursorX -= removedOnCursor
		if a.cursorX < 0 {
			a.cursorX = 0
		}
	}
	a.ensureCursorInBounds()
	if start == end {
		a.statusMessage = fmt.Sprintf("Outdented line %d.", start+1)
	} else {
		a.statusMessage = fmt.Sprintf("Outdented lines %d-%d.", start+1, end+1)
	}
}

func (a *App) selectCodeBlock() {
	content := a.tabs[a.activeTab].Content
	if len(content) == 0 {
		return
	}

	currentStart, currentEnd, hasSelection := a.projectedLineRange()
	delimiterCandidates := uniqueSortedRanges(a.delimiterBlockCandidates(content))
	indentCandidates := uniqueSortedRanges(a.indentBlockCandidates(content))
	candidates := delimiterCandidates
	if hasSelection {
		if _, ok := a.nextEnclosingBlock(delimiterCandidates, currentStart, currentEnd); !ok {
			if _, ok := a.smallestBlockForLine(delimiterCandidates, currentStart); !ok {
				candidates = indentCandidates
			}
		}
	} else if _, ok := a.smallestBlockForLine(delimiterCandidates, a.cursorY); !ok {
		candidates = indentCandidates
	}
	if len(candidates) == 0 {
		a.setSelectionAnchor(0, a.cursorY)
		a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
		a.statusMessage = fmt.Sprintf("No enclosing block found; selected line %d.", a.cursorY+1)
		return
	}

	if !hasSelection {
		candidate, ok := a.smallestBlockForLine(candidates, a.cursorY)
		if !ok {
			a.setSelectionAnchor(0, a.cursorY)
			a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
			a.statusMessage = fmt.Sprintf("No enclosing block found; selected line %d.", a.cursorY+1)
			return
		}
		a.setSelectionAnchor(0, candidate.start)
		a.cursorY = candidate.end
		a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
		a.ensureCursorInBounds()
		a.statusMessage = fmt.Sprintf("Selected code block lines %d-%d.", candidate.start+1, candidate.end+1)
		return
	}

	if candidate, ok := a.nextEnclosingBlock(candidates, currentStart, currentEnd); ok {
		a.setSelectionAnchor(0, candidate.start)
		a.cursorY = candidate.end
		a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
		a.ensureCursorInBounds()
		a.statusMessage = fmt.Sprintf("Expanded to parent block lines %d-%d.", candidate.start+1, candidate.end+1)
		return
	}

	a.statusMessage = "No larger parent block found for the current selection."
}

func (a *App) blockCandidates() []lineRange {
	content := a.tabs[a.activeTab].Content
	if len(content) == 0 {
		return nil
	}

	var candidates []lineRange
	candidates = append(candidates, a.delimiterBlockCandidates(content)...)
	candidates = append(candidates, a.indentBlockCandidates(content)...)
	return uniqueSortedRanges(candidates)
}

func (a *App) smallestBlockForLine(candidates []lineRange, line int) (lineRange, bool) {
	for _, candidate := range candidates {
		if candidate.start <= line && line <= candidate.end {
			return candidate, true
		}
	}
	return lineRange{}, false
}

func (a *App) nextEnclosingBlock(candidates []lineRange, start, end int) (lineRange, bool) {
	for _, candidate := range candidates {
		if candidate.start <= start && candidate.end >= end && (candidate.start < start || candidate.end > end) {
			return candidate, true
		}
	}
	return lineRange{}, false
}

func (a *App) delimiterBlockCandidates(content []string) []lineRange {
	closers := map[rune]rune{')': '(', ']': '[', '}': '{'}
	type opener struct {
		line int
		r    rune
	}
	var stack []opener
	var candidates []lineRange

	for lineIndex, line := range content {
		for _, r := range []rune(line) {
			switch r {
			case '(', '[', '{':
				stack = append(stack, opener{line: lineIndex, r: r})
			case ')', ']', '}':
				expected := closers[r]
				for len(stack) > 0 {
					last := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if last.r == expected {
						if last.line < lineIndex {
							candidates = append(candidates, lineRange{start: last.line, end: lineIndex})
						}
						break
					}
				}
			}
		}
	}

	return candidates
}

func (a *App) indentBlockCandidates(content []string) []lineRange {
	anchor := a.nearestCodeLine(a.cursorY)
	if anchor < 0 {
		return nil
	}

	var candidates []lineRange
	seen := map[int]bool{}

	for lineIndex := anchor; lineIndex >= 0; lineIndex-- {
		trimmed := strings.TrimSpace(content[lineIndex])
		if trimmed == "" {
			continue
		}
		indent := lineIndent(content[lineIndex])
		if lineIndex != anchor && !looksLikeBlockHeader(trimmed) {
			nextLine := a.nearestCodeLineBelow(lineIndex + 1)
			if nextLine < 0 {
				continue
			}
			nextIndent := lineIndent(content[nextLine])
			if nextIndent <= indent {
				continue
			}
		}
		end := findIndentedBlockEnd(content, lineIndex)
		if end <= lineIndex || anchor > end {
			continue
		}
		if !seen[lineIndex] {
			candidates = append(candidates, lineRange{start: lineIndex, end: end})
			seen[lineIndex] = true
		}
	}

	return candidates
}

func (a *App) nearestCodeLine(start int) int {
	content := a.tabs[a.activeTab].Content
	if len(content) == 0 {
		return -1
	}
	if start < 0 {
		start = 0
	}
	if start >= len(content) {
		start = len(content) - 1
	}
	for distance := 0; distance < len(content); distance++ {
		up := start - distance
		if up >= 0 && strings.TrimSpace(content[up]) != "" {
			return up
		}
		down := start + distance
		if distance > 0 && down < len(content) && strings.TrimSpace(content[down]) != "" {
			return down
		}
	}
	return -1
}

func (a *App) nearestCodeLineBelow(start int) int {
	content := a.tabs[a.activeTab].Content
	for index := start; index < len(content); index++ {
		if strings.TrimSpace(content[index]) != "" {
			return index
		}
	}
	return -1
}

func uniqueSortedRanges(ranges []lineRange) []lineRange {
	seen := make(map[string]bool)
	unique := make([]lineRange, 0, len(ranges))
	for _, candidate := range ranges {
		if candidate.start < 0 || candidate.end < candidate.start {
			continue
		}
		key := fmt.Sprintf("%d:%d", candidate.start, candidate.end)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, candidate)
	}

	sort.Slice(unique, func(i, j int) bool {
		leftSize := unique[i].end - unique[i].start
		rightSize := unique[j].end - unique[j].start
		if leftSize != rightSize {
			return leftSize < rightSize
		}
		if unique[i].start != unique[j].start {
			return unique[i].start > unique[j].start
		}
		return unique[i].end < unique[j].end
	})

	return unique
}

func findIndentedBlockEnd(content []string, start int) int {
	if start < 0 || start >= len(content) {
		return start
	}
	baseIndent := lineIndent(content[start])
	end := start
	for index := start + 1; index < len(content); index++ {
		trimmed := strings.TrimSpace(content[index])
		if trimmed == "" {
			if end >= start {
				end = index
			}
			continue
		}
		if lineIndent(content[index]) <= baseIndent {
			break
		}
		end = index
	}
	return end
}

func lineIndent(line string) int {
	indent := 0
	for _, r := range []rune(line) {
		switch r {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			return indent
		}
	}
	return indent
}

func (a *App) removeSingleIndent(line string) (string, int) {
	if strings.HasPrefix(line, "\t") {
		return strings.TrimPrefix(line, "\t"), 1
	}
	indentUnit := a.indentUnit()
	if strings.HasPrefix(line, indentUnit) {
		return strings.TrimPrefix(line, indentUnit), len([]rune(indentUnit))
	}
	spaces := 0
	for _, r := range []rune(line) {
		if r != ' ' || spaces == len([]rune(indentUnit)) {
			break
		}
		spaces++
	}
	if spaces == 0 {
		return line, 0
	}
	return line[spaces:], spaces
}

func isVisuallyBlank(line string) bool {
	return strings.Trim(line, " \t") == ""
}

func looksLikeBlockHeader(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	return strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, ":")
}

func (a *App) ensureCursorInBounds() {
	content := a.tabs[a.activeTab].Content
	if len(content) == 0 {
		a.cursorX = 0
		a.cursorY = 0
		return
	}
	if a.cursorY < 0 {
		a.cursorY = 0
	}
	if a.cursorY >= len(content) {
		a.cursorY = len(content) - 1
	}
	lineWidth := len([]rune(content[a.cursorY]))
	if a.cursorX < 0 {
		a.cursorX = 0
	}
	if a.cursorX > lineWidth {
		a.cursorX = lineWidth
	}
	a.syncViewport()
}

func (a *App) syncViewport() {
	content := a.tabs[a.activeTab].Content
	if len(content) == 0 {
		a.viewportTop = 0
		a.viewportLeft = 0
		return
	}
	visibleHeight := a.editorViewportHeight
	if visibleHeight <= 0 {
		visibleHeight = 10
	}
	maxTop := len(content) - visibleHeight
	if maxTop < 0 {
		maxTop = 0
	}
	if a.viewportTop < 0 {
		a.viewportTop = 0
	}
	if a.viewportTop > maxTop {
		a.viewportTop = maxTop
	}
	if a.cursorY < a.viewportTop {
		a.viewportTop = a.cursorY
	}
	if a.cursorY >= a.viewportTop+visibleHeight {
		a.viewportTop = a.cursorY - visibleHeight + 1
	}
	if a.viewportTop < 0 {
		a.viewportTop = 0
	}
	if a.viewportTop > maxTop {
		a.viewportTop = maxTop
	}

	visibleWidth := a.editorViewportWidth
	if visibleWidth <= 0 {
		visibleWidth = 20
	}
	line := []rune(content[a.cursorY])
	lineWidth := visualColumnForRunes(line, len(line))
	maxLeft := lineWidth - visibleWidth
	if maxLeft < 0 {
		maxLeft = 0
	}
	if a.viewportLeft < 0 {
		a.viewportLeft = 0
	}
	if a.viewportLeft > maxLeft {
		a.viewportLeft = maxLeft
	}
	cursorVisualX := visualColumnForRunes(line, a.cursorX)
	if cursorVisualX < a.viewportLeft {
		a.viewportLeft = cursorVisualX
	}
	if cursorVisualX >= a.viewportLeft+visibleWidth {
		a.viewportLeft = cursorVisualX - visibleWidth + 1
	}
	if a.viewportLeft < 0 {
		a.viewportLeft = 0
	}
	if a.viewportLeft > maxLeft {
		a.viewportLeft = maxLeft
	}
}

func (a *App) syncControlViewport() {
	visibleWidth := a.controlViewportWidth
	if visibleWidth <= 0 {
		visibleWidth = 20
	}
	lineWidth := visualColumnForRunes(a.controlInput, len(a.controlInput))
	maxLeft := lineWidth - visibleWidth
	if maxLeft < 0 {
		maxLeft = 0
	}
	if a.controlViewportLeft < 0 {
		a.controlViewportLeft = 0
	}
	if a.controlViewportLeft > maxLeft {
		a.controlViewportLeft = maxLeft
	}
	cursorVisualX := visualColumnForRunes(a.controlInput, a.controlCursor)
	if cursorVisualX < a.controlViewportLeft {
		a.controlViewportLeft = cursorVisualX
	}
	if cursorVisualX >= a.controlViewportLeft+visibleWidth {
		a.controlViewportLeft = cursorVisualX - visibleWidth + 1
	}
	if a.controlViewportLeft < 0 {
		a.controlViewportLeft = 0
	}
	if a.controlViewportLeft > maxLeft {
		a.controlViewportLeft = maxLeft
	}
}

func (a *App) draw() {
	w, h := a.screen.Size()
	a.screen.Clear()

	if w < minWidth || h < minHeight {
		a.drawText(0, 0, styleError(), "Window too small for gdedit shell")
		a.drawText(0, 1, styleMuted(), fmt.Sprintf("Need at least %dx%d, got %dx%d", minWidth, minHeight, w, h))
		a.screen.Show()
		return
	}

	bottomHeight := 5
	editTop := 1
	editBottom := h - bottomHeight - 1
	controlWidth := (w * 2) / 3
	statusX := controlWidth + 1

	a.drawTabs(w)
	a.drawEditSurface(0, editTop, w, editBottom-editTop+1)
	a.drawControlPanel(0, h-bottomHeight, controlWidth, bottomHeight)
	a.drawStatusSurface(statusX, h-bottomHeight, w-statusX, bottomHeight)
	if a.helpVisible {
		a.screen.HideCursor()
		a.drawHelpDialog(w, h)
	}
	if a.historyVisible {
		a.screen.HideCursor()
		a.drawHistoryDialog(w, h)
	}
	if a.quitConfirm {
		a.screen.HideCursor()
		a.drawQuitDialog(w, h)
	}
	a.screen.Show()
}

func (a *App) drawTabs(width int) {
	x := 0
	for i, tab := range a.tabs {
		label := "[" + tab.Title
		if tab.Dirty {
			label += " ●"
		}
		label += "]"
		style := styleMuted()
		if i == a.activeTab {
			style = styleTabActive()
		}
		a.drawText(x, 0, style, label)
		x += len([]rune(label)) + 1
		if x >= width {
			break
		}
	}
}

func (a *App) drawEditSurface(x, y, width, height int) {
	a.drawBox(x, y, width, height, styleBox(), "Active Edit Surface")
	content := a.tabs[a.activeTab].Content
	visibleHeight := height - 2
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	visibleWidth := width - 5
	if visibleWidth < 1 {
		visibleWidth = 1
	}
	a.editorViewportHeight = visibleHeight
	a.editorViewportWidth = visibleWidth
	a.syncViewport()
	for i := 0; i < visibleHeight && i+a.viewportTop < len(content); i++ {
		lineIndex := i + a.viewportTop
		lineY := y + 1 + i
		gutter, style := a.lineVisual(lineIndex)
		a.drawText(x+1, lineY, styleGutter(), gutter)
		a.drawTextWithVisibleTabs(x+4, lineY, lineIndex, style, []rune(content[lineIndex]), a.viewportLeft, visibleWidth)
	}
	footer := fmt.Sprintf("Focus:%s  Scope:%s  Save:[Ctrl+S]  Indent:[Tab/Shift+Tab]  Width:[Alt+0..4]  Tabs:[Alt+,/Alt+.]  Block:[F2]  Ctrl+Up/Down:[line or block]  Control:[Ctrl+G]  Quit:[Ctrl+Q]", a.focusLabel(), a.currentScope())
	if a.helpVisible {
		footer = fmt.Sprintf("Focus:%s  Scope:%s  Help:[Esc closes]", a.focusLabel(), a.currentScope())
	}
	a.drawText(x+1, y+height-1, styleMuted(), trimRunes(footer, width-2))
	if a.focus == focusEditor && !a.helpVisible && !a.historyVisible && !a.quitConfirm {
		cursorX := x + 4 + (visualColumnForRunes([]rune(content[a.cursorY]), a.cursorX) - a.viewportLeft)
		cursorY := y + 1 + (a.cursorY - a.viewportTop)
		maxX := x + width - 2
		maxY := y + height - 2
		if cursorX > maxX {
			cursorX = maxX
		}
		if cursorY > maxY {
			cursorY = maxY
		}
		a.screen.ShowCursor(cursorX, cursorY)
	}
}

func (a *App) drawControlPanel(x, y, width, height int) {
	title := "Control Hub"
	if a.focus == focusControl {
		title += " (focused)"
	}
	a.drawBox(x, y, width, height, styleBox(), title)
	visibleWidth := width - 4
	if visibleWidth < 1 {
		visibleWidth = 1
	}
	a.controlViewportWidth = visibleWidth
	a.syncControlViewport()
	a.drawControlInput(x+1, y+1, width-2)
	preview := "Preview: type a command and press Enter"
	if a.preview.Pending {
		preview = "Preview: " + a.preview.Summary()
	}
	a.drawText(x+1, y+2, stylePreview(), trimRunes(preview, width-2))
	footer := "Examples: /open README.md | /write notes.txt | inspect recent change"
	style := styleMuted()
	if a.agentRunning {
		footer = fmt.Sprintf("Sending %s", spinnerFrames[a.spinnerIndex])
		style = stylePreview()
	} else if strings.TrimSpace(a.lastAgentReply) != "" {
		footer = "Reply: " + ellipsizeRunes(a.lastAgentReply, max(0, width-10))
		style = stylePreview()
	}
	a.drawText(x+1, y+3, style, trimRunes(footer, width-2))
	if a.focus == focusControl {
		cursorX := x + 3 + (visualColumnForRunes(a.controlInput, a.controlCursor) - a.controlViewportLeft)
		maxX := x + width - 2
		if cursorX > maxX {
			cursorX = maxX
		}
		a.screen.ShowCursor(cursorX, y+1)
	} else if a.focus != focusEditor {
		a.screen.HideCursor()
	}
}

func (a *App) drawControlInput(x, y, width int) {
	prefix := []rune("> ")
	dx := x
	for _, r := range prefix {
		a.screen.SetContent(dx, y, r, nil, styleNormal())
		dx += runeCellWidth(r)
	}
	maxWidth := width - visualColumnForRunes(prefix, len(prefix))
	if maxWidth < 1 {
		maxWidth = 1
	}
	currentCol := 0
	for i, r := range a.controlInput {
		style := styleNormal()
		if start, end, ok := a.controlSelectionRange(); ok && i >= start && i < end {
			style = styleSelection()
		}
		cellWidth := runeCellWidth(r)
		nextCol := currentCol + cellWidth
		if nextCol <= a.controlViewportLeft {
			currentCol = nextCol
			continue
		}
		if currentCol >= a.controlViewportLeft+maxWidth {
			break
		}
		a.screen.SetContent(dx, y, r, nil, style)
		dx += cellWidth
		currentCol = nextCol
		if dx >= x+width {
			break
		}
	}
}

func (a *App) drawStatusSurface(x, y, width, height int) {
	a.drawBox(x, y, width, height, styleBox(), "Status Surface")
	lines := []string{
		"tab: " + a.tabs[a.activeTab].Title,
		"scope: " + a.currentScope(),
		"indent: " + a.indentModeLabel(),
		"slash: /open /write /saveas",
		"agent: " + a.agentState(),
		"model: " + a.agentProfileLabel(),
		"voice: " + a.voiceState,
		"command: " + a.previewOrLastCommand(),
	}
	for i := 0; i < len(lines) && i < height-2; i++ {
		a.drawText(x+1, y+1+i, styleMuted(), trimRunes(lines[i], width-2))
	}
	if height >= 4 {
		scopeLine := "selection: none"
		if a.hasSelection() {
			scopeLine = "selection: active"
		}
		a.drawText(x+1, y+height-2, stylePreview(), trimRunes(scopeLine, width-2))
	}
}

func (a *App) drawHelpDialog(screenWidth, screenHeight int) {
	lines := a.helpLines()

	maxLen := 0
	for _, line := range lines {
		if n := visualWidthString(line); n > maxLen {
			maxLen = n
		}
	}

	width := maxLen + 4
	height := len(lines) + 2
	if width > screenWidth-4 {
		width = screenWidth - 4
	}
	if height > screenHeight-2 {
		height = screenHeight - 2
	}
	x := (screenWidth - width) / 2
	y := (screenHeight - height) / 2
	visibleLines := height - 2
	showScrollFooter := len(lines) > visibleLines && visibleLines > 1
	if showScrollFooter {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll := len(lines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.helpScroll > maxScroll {
		a.helpScroll = maxScroll
	}
	if a.helpScroll < 0 {
		a.helpScroll = 0
	}

	for dy := y; dy < y+height; dy++ {
		for dx := x; dx < x+width; dx++ {
			a.screen.SetContent(dx, dy, ' ', nil, styleDialogFill())
		}
	}

	a.drawBox(x, y, width, height, styleDialogBorder(), "Help")
	for i := 0; i < visibleLines && i+a.helpScroll < len(lines); i++ {
		style := styleDialogText()
		if i+a.helpScroll == 0 {
			style = styleDialogTitle()
		}
		a.drawText(x+2, y+1+i, style, trimRunes(lines[i+a.helpScroll], width-4))
	}
	if showScrollFooter {
		footer := fmt.Sprintf("Scroll: %d/%d  Up/Down moves  Esc closes", a.helpScroll+1, maxScroll+1)
		a.drawText(x+2, y+height-2, styleMuted(), trimRunes(footer, width-4))
	}
}

func (a *App) drawHistoryDialog(screenWidth, screenHeight int) {
	width := screenWidth - 4
	if width > 100 {
		width = 100
	}
	if width < 20 {
		width = 20
	}
	contentWidth := width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}
	lines := a.historyLinesForWidth(contentWidth)
	maxLen := 0
	for _, line := range lines {
		if n := visualWidthString(line); n > maxLen {
			maxLen = n
		}
	}
	width = maxLen + 4
	height := len(lines) + 2
	if width > screenWidth-4 {
		width = screenWidth - 4
	}
	if height > screenHeight-2 {
		height = screenHeight - 2
	}
	x := (screenWidth - width) / 2
	y := (screenHeight - height) / 2
	visibleLines := height - 2
	showScrollFooter := len(lines) > visibleLines && visibleLines > 1
	if showScrollFooter {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll := len(lines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.historyScroll > maxScroll {
		a.historyScroll = maxScroll
	}
	if a.historyScroll < 0 {
		a.historyScroll = 0
	}
	for dy := y; dy < y+height; dy++ {
		for dx := x; dx < x+width; dx++ {
			a.screen.SetContent(dx, dy, ' ', nil, styleDialogFill())
		}
	}
	title := "History"
	if len(a.tabs) > 0 {
		title = "History: " + a.tabs[a.activeTab].Title
	}
	a.drawBox(x, y, width, height, styleDialogBorder(), title)
	for i := 0; i < visibleLines && i+a.historyScroll < len(lines); i++ {
		style := styleDialogText()
		if i+a.historyScroll == 0 {
			style = styleDialogTitle()
		}
		a.drawText(x+2, y+1+i, style, lines[i+a.historyScroll])
	}
	if showScrollFooter {
		footer := fmt.Sprintf("Scroll: %d/%d  Up/Down moves  Esc closes", a.historyScroll+1, maxScroll+1)
		a.drawText(x+2, y+height-2, styleMuted(), trimRunes(footer, width-4))
	}
}

func (a *App) historyLines() []string {
	return a.historyLinesForWidth(72)
}

func (a *App) historyLinesForWidth(maxWidth int) []string {
	lines := []string{"conversation history", ""}
	if len(a.tabs) == 0 || a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return append(lines, "(no active tab)")
	}
	history := a.tabs[a.activeTab].History
	if len(history) == 0 {
		return append(lines, "(no conversation history for this tab yet)")
	}
	if maxWidth < 8 {
		maxWidth = 8
	}
	for i, entry := range history {
		lines = append(lines, wrapPrefixedLines(fmt.Sprintf("[%d] You: ", i+1), entry.Prompt, maxWidth)...)
		lines = append(lines, wrapPrefixedLines("    Reply: ", entry.Reply, maxWidth)...)
		lines = append(lines, "")
	}
	return lines
}

func (a *App) maxHistoryScroll() int {
	visibleLines := 18
	if a.screen != nil {
		_, h := a.screen.Size()
		visibleLines = h - 4
	}
	lines := a.historyLinesForWidth(72)
	if len(lines) > visibleLines && visibleLines > 1 {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll := len(lines) - visibleLines
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func wrapPrefixedLines(prefix, text string, width int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return []string{prefix}
	}
	available := width - visualWidthString(prefix)
	if available < 8 {
		available = 8
	}
	wrapped := wrapText(text, available)
	parts := strings.Split(wrapped, "\n")
	lines := make([]string, 0, len(parts))
	indent := strings.Repeat(" ", visualWidthString(prefix))
	for i, part := range parts {
		if i == 0 {
			lines = append(lines, prefix+part)
			continue
		}
		lines = append(lines, indent+part)
	}
	return lines
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	lines := []string{}
	firstParts := splitByVisualWidth(words[0], width)
	current := firstParts[0]
	for _, extra := range firstParts[1:] {
		lines = append(lines, current)
		current = extra
	}
	currentWidth := visualWidthString(current)
	for _, word := range words[1:] {
		wordParts := splitByVisualWidth(word, width)
		wordHead := wordParts[0]
		wordWidth := visualWidthString(wordHead)
		if currentWidth+1+wordWidth > width {
			lines = append(lines, current)
			current = wordHead
			currentWidth = wordWidth
		} else {
			current += " " + wordHead
			currentWidth += 1 + wordWidth
		}
		for _, extra := range wordParts[1:] {
			lines = append(lines, current)
			current = extra
			currentWidth = visualWidthString(extra)
		}
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func splitByVisualWidth(text string, width int) []string {
	if width <= 0 || text == "" {
		return []string{text}
	}
	parts := []string{}
	current := []rune{}
	currentWidth := 0
	for _, r := range []rune(text) {
		rw := runeCellWidth(displayRune(r))
		if currentWidth+rw > width && len(current) > 0 {
			parts = append(parts, string(current))
			current = []rune{r}
			currentWidth = rw
			continue
		}
		current = append(current, r)
		currentWidth += rw
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	if len(parts) == 0 {
		return []string{text}
	}
	return parts
}

func (a *App) helpLines() []string {
	return []string{
		"gdedit help",
		"",
		"F1 opens this help dialog.",
		"F3 opens conversation history for the active tab.",
		"Esc closes the help dialog, clears a preview, or clears a selection.",
		"",
		"Common",
		"  Alt+.            next tab",
		"  Alt+,            previous tab",
		"  Ctrl+A           select all in the active surface",
		"  Ctrl+C / Ctrl+X  copy or cut the current selection",
		"  Ctrl+V           paste, or replace the current selection",
		"  Ctrl+Q           open quit confirmation",
		"  F3               open history for the active tab",
		"  Shift/Alt+Arrow  expand or shrink selection",
		"  Home / End       move caret to start or end of line",
		"  Ctrl+Arrow       move caret by word",
		"  Ctrl+Shift/Alt   select by word with Left/Right",
		"  Shift/Alt+Home   select to line start",
		"  Shift/Alt+End    select to line end",
		"  Shift/Alt+Page   select by larger vertical steps",
		"",
		"Editor",
		"  Ctrl+G           focus control hub",
		"  No selection: Tab inserts a literal tab",
		"  With selection: Tab indents the selected lines",
		"  With selection: Shift+Tab outdents the selected lines",
		"  Alt+0            use literal tabs for selection indentation",
		"  Alt+1..Alt+4     set indentation width in spaces",
		"  colored »        visible marker for literal tab characters",
		"  F2               select current block, then parent block",
		"  PageUp / PageDn  move caret by larger vertical steps",
		"  No selection: Ctrl+Down inserts a blank line above",
		"  No selection: Ctrl+Up removes a blank line above",
		"  With selection: Ctrl+Down moves the selected block down",
		"  With selection: Ctrl+Up moves the selected block up",
		"  Ctrl+S           save the current file-backed tab",
		"",
		"Control hub",
		"  Ctrl+G           return to the editor",
		"  Talk, inspect, /open, and /write run on Enter.",
		"  Edit and memo commands preview first, then confirm on Enter.",
		"  Example: /open README.md",
		"  Example: /write notes.txt",
		"  Example: /saveas \"notes with spaces.txt\"",
		"  Example: inspect recent change",
	}
}

func (a *App) maxHelpScroll() int {
	visibleLines := 18
	if a.screen != nil {
		_, h := a.screen.Size()
		visibleLines = h - 4
	}
	if len(a.helpLines()) > visibleLines && visibleLines > 1 {
		visibleLines--
	}
	if visibleLines < 1 {
		visibleLines = 1
	}
	maxScroll := len(a.helpLines()) - visibleLines
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

func (a *App) drawQuitDialog(screenWidth, screenHeight int) {
	lines := []string{
		"Quit gdedit?",
		"",
		"Ctrl+Q is the dedicated quit key.",
		"gdedit still asks before quitting to avoid accidental exits.",
		"",
		"Press Enter or y to exit.",
		"Press Esc or n to return to the editor.",
	}

	maxLen := 0
	for _, line := range lines {
		if n := len([]rune(line)); n > maxLen {
			maxLen = n
		}
	}

	width := maxLen + 4
	height := len(lines) + 2
	if width > screenWidth-6 {
		width = screenWidth - 6
	}
	if height > screenHeight-4 {
		height = screenHeight - 4
	}
	x := (screenWidth - width) / 2
	y := (screenHeight - height) / 2

	for dy := y; dy < y+height; dy++ {
		for dx := x; dx < x+width; dx++ {
			a.screen.SetContent(dx, dy, ' ', nil, styleDialogFill())
		}
	}

	a.drawBox(x, y, width, height, styleQuitBorder(), "Confirm Exit")
	for i := 0; i < len(lines) && i < height-2; i++ {
		style := styleDialogText()
		if i == 0 {
			style = styleQuitTitle()
		}
		a.drawText(x+2, y+1+i, style, trimRunes(lines[i], width-4))
	}
}

func (a *App) agentState() string {
	if a.preview.Pending {
		return "preview"
	}
	if a.focus == focusControl {
		return "scoping"
	}
	return "idle"
}

func (a *App) agentProfileLabel() string {
	if strings.TrimSpace(a.agentProfile) == "" {
		return "unconfigured"
	}
	return a.agentProfile
}

func (a *App) previewOrLastCommand() string {
	if a.preview.Pending {
		return a.preview.Summary()
	}
	if a.lastCommand != "" {
		return a.lastCommand
	}
	return "none"
}

func (a *App) lineVisual(index int) (string, tcell.Style) {
	if index == a.cursorY {
		return ">  ", styleCursorLine()
	}
	return "   ", styleNormal()
}

func (a *App) focusLabel() string {
	if a.focus == focusControl {
		return "control"
	}
	return "editor"
}

func (a *App) drawBox(x, y, width, height int, style tcell.Style, title string) {
	if width < 2 || height < 2 {
		return
	}
	for dx := 0; dx < width; dx++ {
		a.screen.SetContent(x+dx, y, tcell.RuneHLine, nil, style)
		a.screen.SetContent(x+dx, y+height-1, tcell.RuneHLine, nil, style)
	}
	for dy := 0; dy < height; dy++ {
		a.screen.SetContent(x, y+dy, tcell.RuneVLine, nil, style)
		a.screen.SetContent(x+width-1, y+dy, tcell.RuneVLine, nil, style)
	}
	a.screen.SetContent(x, y, tcell.RuneULCorner, nil, style)
	a.screen.SetContent(x+width-1, y, tcell.RuneURCorner, nil, style)
	a.screen.SetContent(x, y+height-1, tcell.RuneLLCorner, nil, style)
	a.screen.SetContent(x+width-1, y+height-1, tcell.RuneLRCorner, nil, style)
	if strings.TrimSpace(title) != "" && width > 4 {
		a.drawText(x+2, y, style, trimRunes(title, width-4))
	}
}

func (a *App) drawText(x, y int, style tcell.Style, text string) {
	dx := x
	for _, r := range []rune(text) {
		a.screen.SetContent(dx, y, r, nil, style)
		dx += runeCellWidth(r)
	}
}

func (a *App) drawTextWithVisibleTabs(x, y, lineIndex int, style tcell.Style, runes []rune, startCol, maxWidth int) {
	if maxWidth <= 0 {
		return
	}
	dx := x
	currentCol := 0
	for i, r := range runes {
		cellRune := r
		cellStyle := style
		if a.selectionContains(textPos{x: i, y: lineIndex}) {
			cellStyle = styleSelection()
		}
		if r == '\t' {
			cellRune = []rune(tabGlyph)[0]
			cellStyle = styleTabGlyph()
			if a.selectionContains(textPos{x: i, y: lineIndex}) {
				cellStyle = styleSelection()
			}
		}
		cellWidth := runeCellWidth(cellRune)
		nextCol := currentCol + cellWidth
		if nextCol <= startCol {
			currentCol = nextCol
			continue
		}
		if currentCol >= startCol+maxWidth {
			break
		}
		a.screen.SetContent(dx, y, cellRune, nil, cellStyle)
		dx += cellWidth
		currentCol = nextCol
		if dx >= x+maxWidth {
			break
		}
	}
}

func normalizeInsertedRunes(runes []rune, cursor int) ([]rune, int) {
	normalized := []rune(norm.NFC.String(string(runes)))
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(normalized) {
		cursor = len(normalized)
	}
	return normalized, cursor
}

func visualColumnForRunes(runes []rune, runeIndex int) int {
	if runeIndex < 0 {
		return 0
	}
	if runeIndex > len(runes) {
		runeIndex = len(runes)
	}
	width := 0
	for _, r := range runes[:runeIndex] {
		width += runeCellWidth(displayRune(r))
	}
	return width
}

func displayRune(r rune) rune {
	if r == '\t' {
		return []rune(tabGlyph)[0]
	}
	return r
}

func runeCellWidth(r rune) int {
	width := uniseg.StringWidth(string(r))
	if width <= 0 {
		return 1
	}
	return width
}

func trimRunes(input string, max int) string {
	if max <= 0 {
		return ""
	}
	if visualWidthString(input) <= max {
		return input
	}
	if max <= 3 {
		runes := []rune(input)
		out := []rune{}
		width := 0
		for _, r := range runes {
			rw := runeCellWidth(displayRune(r))
			if width+rw > max {
				break
			}
			out = append(out, r)
			width += rw
		}
		return string(out)
	}
	budget := max - 3
	runes := []rune(input)
	out := []rune{}
	width := 0
	for _, r := range runes {
		rw := runeCellWidth(displayRune(r))
		if width+rw > budget {
			break
		}
		out = append(out, r)
		width += rw
	}
	return string(out) + "..."
}

func ellipsizeRunes(input string, max int) string {
	return trimRunes(input, max)
}

func visualWidthString(input string) int {
	return visualColumnForRunes([]rune(input), len([]rune(input)))
}

func styleNormal() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorWhite)
}

func styleMuted() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorSilver)
}

func styleBox() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorTeal)
}

func styleTabActive() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorAqua)
}

func styleTabGlyph() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorLightCyan)
}

func styleSelection() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow)
}

func styleCursorLine() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.Color235)
}

func stylePreview() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorLightGreen)
}

func styleProposal() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorLightBlue)
}

func styleReviewNeeded() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorOrange)
}

func styleLocked() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorMaroon)
}

func styleApproved() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorLightGreen)
}

func styleDenied() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorPurple)
}

func styleGutter() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorAqua)
}

func styleError() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorMaroon)
}

func styleDialogFill() tcell.Style {
	return tcell.StyleDefault.Background(tcell.Color234).Foreground(tcell.ColorWhite)
}

func styleDialogBorder() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorLightCyan).Background(tcell.Color234)
}

func styleDialogText() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.Color234)
}

func styleDialogTitle() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorLightCyan)
}

func styleQuitBorder() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.Color234)
}

func styleQuitTitle() tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow)
}
