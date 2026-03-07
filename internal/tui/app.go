package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

const (
	minWidth      = 60
	minHeight     = 12
	defaultIndent = 2
	tabGlyph      = "»"
)

type focusArea int

const (
	focusEditor focusArea = iota
	focusControl
)

type Tab struct {
	Title            string
	Content          []string
	Locked           map[int]bool
	CursorX          int
	CursorY          int
	SelectionAnchorX int
	SelectionAnchorY int
}

type handoffState string

const (
	handoffHumanLed      handoffState = "human-led"
	handoffAgentSuggest  handoffState = "agent-suggesting"
	handoffReviewPending handoffState = "review-pending"
	handoffApplying      handoffState = "applying"
	handoffDenied        handoffState = "locked/denied"
)

type reviewItem struct {
	ID     string
	Action string
	Target string
	State  handoffState
}

type lineRange struct {
	start int
	end   int
}

type textPos struct {
	x int
	y int
}

type App struct {
	screen             tcell.Screen
	tabs               []Tab
	activeTab          int
	focus              focusArea
	controlInput       []rune
	controlCursor      int
	preview            Preview
	cursorX            int
	cursorY            int
	selectionAnchorCol int
	selectionAnchor    int
	statusMessage      string
	voiceState         string
	reviewCount        int
	handoff            handoffState
	reviewItems        []reviewItem
	lastApplied        string
	helpVisible        bool
	quitConfirm        bool
	indentWidth        int
	clipboard          string
}

func New() *App {
	return &App{
		tabs: []Tab{
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
				Locked:           map[int]bool{0: true, 2: true},
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
				Locked:           map[int]bool{0: true},
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
				Locked:           map[int]bool{0: true},
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
				Locked:           map[int]bool{0: true},
				SelectionAnchorY: -1,
			},
		},
		focus:           focusEditor,
		selectionAnchor: -1,
		statusMessage:   "Ready. Ctrl+G focuses the control hub.",
		voiceState:      "off",
		handoff:         handoffHumanLed,
		indentWidth:     defaultIndent,
	}
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
		case *tcell.EventKey:
			if quit := a.handleKey(ev); quit {
				return nil
			}
		}
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
		a.quitConfirm = false
		a.statusMessage = "Help opened. Press Esc to close."
		return false
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
		if ev.Key() == tcell.KeyEsc {
			a.helpVisible = false
			a.statusMessage = "Help closed."
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
		a.handoff = handoffAgentSuggest
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
	if a.currentLineLocked() {
		a.handoff = handoffDenied
		a.statusMessage = "Current line is locked and cannot be edited directly."
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
	a.tabs[a.activeTab].Content[a.cursorY] = string(line)
	a.cursorX++
	a.handoff = handoffHumanLed
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
	if a.currentLineLocked() {
		a.handoff = handoffDenied
		a.statusMessage = "Current line is locked and cannot be edited directly."
		return
	}
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
		a.shiftLockedLines(originalY+1, len(parts)-1)
		a.cursorY = originalY + len(parts) - 1
		a.cursorX = len([]rune(parts[len(parts)-1]))
	}
	updated = append(updated, content[originalY+1:]...)
	a.tabs[a.activeTab].Content = updated
	a.handoff = handoffHumanLed
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
	if a.currentLineLocked() {
		a.handoff = handoffDenied
		a.statusMessage = "Current line is locked and cannot be split."
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
	a.shiftLockedLines(a.cursorY+1, 1)
	a.cursorY++
	a.cursorX = 0
	a.handoff = handoffHumanLed
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
	a.shiftLockedLines(a.cursorY, 1)
	a.cursorY = originalLine + 1
	a.cursorX = originalColumn
	a.handoff = handoffHumanLed
	a.statusMessage = fmt.Sprintf("Inserted a blank line above line %d while keeping the caret on its content line.", a.cursorY+1)
}

func (a *App) removeBlankLineAbove() {
	if a.cursorY == 0 {
		a.statusMessage = "There is no line above the cursor."
		return
	}
	index := a.cursorY - 1
	locked := a.tabs[a.activeTab].Locked
	if locked != nil && locked[index] {
		a.handoff = handoffDenied
		a.statusMessage = "The line above is locked and cannot be removed."
		return
	}
	if !isVisuallyBlank(a.tabs[a.activeTab].Content[index]) {
		a.statusMessage = "The line above is not blank enough to remove."
		return
	}
	a.tabs[a.activeTab].Content = append(a.tabs[a.activeTab].Content[:index], a.tabs[a.activeTab].Content[index+1:]...)
	a.shiftLockedLines(a.cursorY, -1)
	a.cursorY--
	a.handoff = handoffHumanLed
	a.statusMessage = fmt.Sprintf("Removed the blank line above line %d.", a.cursorY+1)
}

func (a *App) backspaceEditor() {
	if a.hasSelection() {
		a.replaceSelection("")
		return
	}
	if a.currentLineLocked() {
		a.handoff = handoffDenied
		a.statusMessage = "Current line is locked and cannot be edited directly."
		return
	}

	if a.cursorX > 0 {
		line := []rune(a.tabs[a.activeTab].Content[a.cursorY])
		line = append(line[:a.cursorX-1], line[a.cursorX:]...)
		a.tabs[a.activeTab].Content[a.cursorY] = string(line)
		a.cursorX--
		a.handoff = handoffHumanLed
		a.statusMessage = "Deleted previous character."
		return
	}

	if a.cursorY == 0 {
		return
	}

	prevIndex := a.cursorY - 1
	locked := a.tabs[a.activeTab].Locked
	if locked != nil && locked[prevIndex] {
		a.handoff = handoffDenied
		a.statusMessage = "Previous line is locked and cannot be merged."
		return
	}

	prev := a.tabs[a.activeTab].Content[prevIndex]
	current := a.tabs[a.activeTab].Content[a.cursorY]
	a.tabs[a.activeTab].Content[prevIndex] = prev + current
	a.tabs[a.activeTab].Content = append(a.tabs[a.activeTab].Content[:a.cursorY], a.tabs[a.activeTab].Content[a.cursorY+1:]...)
	a.shiftLockedLines(a.cursorY, -1)
	a.cursorY = prevIndex
	a.cursorX = len([]rune(prev))
	a.handoff = handoffHumanLed
	a.statusMessage = "Merged the current line into the previous line."
}

func (a *App) deleteEditor() {
	if a.hasSelection() {
		a.replaceSelection("")
		return
	}
	if a.currentLineLocked() {
		a.handoff = handoffDenied
		a.statusMessage = "Current line is locked and cannot be edited directly."
		return
	}

	line := []rune(a.tabs[a.activeTab].Content[a.cursorY])
	if a.cursorX < len(line) {
		line = append(line[:a.cursorX], line[a.cursorX+1:]...)
		a.tabs[a.activeTab].Content[a.cursorY] = string(line)
		a.handoff = handoffHumanLed
		a.statusMessage = "Deleted character at cursor."
		return
	}

	if a.cursorY >= len(a.tabs[a.activeTab].Content)-1 {
		return
	}

	nextIndex := a.cursorY + 1
	locked := a.tabs[a.activeTab].Locked
	if locked != nil && locked[nextIndex] {
		a.handoff = handoffDenied
		a.statusMessage = "Next line is locked and cannot be merged."
		return
	}

	next := a.tabs[a.activeTab].Content[nextIndex]
	a.tabs[a.activeTab].Content[a.cursorY] = a.tabs[a.activeTab].Content[a.cursorY] + next
	a.tabs[a.activeTab].Content = append(a.tabs[a.activeTab].Content[:nextIndex], a.tabs[a.activeTab].Content[nextIndex+1:]...)
	a.shiftLockedLines(nextIndex, -1)
	a.handoff = handoffHumanLed
	a.statusMessage = "Merged the next line into the current line."
}

func (a *App) shiftLockedLines(fromIndex, delta int) {
	locked := a.tabs[a.activeTab].Locked
	if locked == nil || delta == 0 {
		return
	}

	updated := make(map[int]bool, len(locked))
	for index, value := range locked {
		if !value {
			continue
		}
		if index >= fromIndex {
			updated[index+delta] = value
		} else {
			updated[index] = value
		}
	}
	a.tabs[a.activeTab].Locked = updated
}

func (a *App) handleControlKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEsc:
		if a.preview.Pending {
			if a.preview.Kind == CommandDenied {
				a.handoff = handoffDenied
			} else {
				a.handoff = handoffHumanLed
			}
			a.preview = Preview{}
			a.reviewItems = nil
			a.reviewCount = 0
			a.statusMessage = "Preview cleared. You can edit the command before submitting again."
			return false
		}
		a.focus = focusEditor
		a.voiceState = "off"
		a.handoff = handoffHumanLed
		a.statusMessage = "Returned to editor focus."
		return false
	case tcell.KeyCtrlG:
		a.focus = focusEditor
		a.voiceState = "off"
		a.handoff = handoffHumanLed
		a.statusMessage = "Returned to editor focus."
		return false
	case tcell.KeyEnter:
		input := strings.TrimSpace(string(a.controlInput))
		if input == "" {
			a.statusMessage = "Control hub is empty."
			return false
		}

		if !a.preview.Pending {
			a.preview = BuildPreview(input, a.currentScope(), a.tabs[a.activeTab].Title, a.currentLineLocked())
			a.reviewItems = a.buildReviewItems(a.preview)
			a.reviewCount = len(a.reviewItems)
			a.handoff = a.previewState(a.preview)
			if a.preview.Kind == CommandDenied {
				a.statusMessage = "Preview denied. Current scope is locked; narrow the target or inspect only."
			} else {
				a.statusMessage = "Preview ready. Press Enter again to confirm or Esc to edit."
			}
			a.voiceState = "captured"
			return false
		}

		if a.preview.Kind == CommandDenied {
			a.statusMessage = "Denied change kept in review. Locked scope remains unchanged."
			a.focus = focusEditor
			a.voiceState = "off"
			a.preview = Preview{}
			return false
		}

		a.lastApplied = a.preview.ProposalID
		a.handoff = handoffApplying
		a.statusMessage = "Applied to " + a.preview.Target + " in " + a.preview.Tab + ": " + a.preview.Action
		a.reviewItems = nil
		a.preview = Preview{}
		a.controlInput = nil
		a.controlCursor = 0
		a.reviewCount = 0
		a.focus = focusEditor
		a.voiceState = "off"
		a.handoff = handoffHumanLed
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if a.controlCursor > 0 {
			a.controlInput = append(a.controlInput[:a.controlCursor-1], a.controlInput[a.controlCursor:]...)
			a.controlCursor--
			a.preview = Preview{}
		}
		return false
	case tcell.KeyLeft:
		if a.controlCursor > 0 {
			a.controlCursor--
		}
		return false
	case tcell.KeyRight:
		if a.controlCursor < len(a.controlInput) {
			a.controlCursor++
		}
		return false
	case tcell.KeyRune:
		r := ev.Rune()
		a.controlInput = append(a.controlInput[:a.controlCursor], append([]rune{r}, a.controlInput[a.controlCursor:]...)...)
		a.controlCursor++
		a.preview = Preview{}
		a.reviewItems = nil
		a.reviewCount = 0
		a.voiceState = "listening"
		a.handoff = handoffAgentSuggest
		return false
	}

	return false
}

func (a *App) nextTab() {
	a.saveCurrentTabState()
	a.activeTab = (a.activeTab + 1) % len(a.tabs)
	a.loadCurrentTabState()
	a.statusMessage = "Switched to tab " + a.tabs[a.activeTab].Title
}

func (a *App) prevTab() {
	a.saveCurrentTabState()
	a.activeTab--
	if a.activeTab < 0 {
		a.activeTab = len(a.tabs) - 1
	}
	a.loadCurrentTabState()
	a.statusMessage = "Switched to tab " + a.tabs[a.activeTab].Title
}

func (a *App) saveCurrentTabState() {
	if len(a.tabs) == 0 || a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return
	}
	tab := &a.tabs[a.activeTab]
	tab.CursorX = a.cursorX
	tab.CursorY = a.cursorY
	tab.SelectionAnchorX = a.selectionAnchorX()
	tab.SelectionAnchorY = a.selectionAnchor
}

func (a *App) loadCurrentTabState() {
	if len(a.tabs) == 0 || a.activeTab < 0 || a.activeTab >= len(a.tabs) {
		return
	}
	tab := &a.tabs[a.activeTab]
	a.cursorX = tab.CursorX
	a.cursorY = tab.CursorY
	a.setSelectionAnchor(tab.SelectionAnchorX, tab.SelectionAnchorY)
	a.ensureCursorInBounds()
	tab.CursorX = a.cursorX
	tab.CursorY = a.cursorY
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
	a.statusMessage = "Copied selection."
}

func (a *App) cutSelection() {
	if !a.hasSelection() {
		a.statusMessage = "No selection to cut."
		return
	}
	a.clipboard = a.selectedText()
	a.replaceSelection("")
	a.statusMessage = "Cut selection."
}

func (a *App) pasteClipboard() {
	if a.clipboard == "" {
		a.statusMessage = "Clipboard is empty."
		return
	}
	a.insertString(a.clipboard)
	a.statusMessage = "Pasted clipboard at caret."
}

func (a *App) clearSelection(message string) {
	a.selectionAnchor = -1
	a.selectionAnchorCol = 0
	a.handoff = handoffHumanLed
	a.statusMessage = message
}

func (a *App) expandSelection(delta int) {
	if !a.hasSelection() {
		a.setSelectionAnchor(a.cursorX, a.cursorY)
	}
	a.cursorY += delta
	a.ensureCursorInBounds()
	a.handoff = handoffHumanLed
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

	flags := a.lockedFlags()
	if a.rangeLocked(start, end, flags) {
		a.handoff = handoffDenied
		a.statusMessage = "Locked lines cannot be moved as a selection block."
		return
	}

	adjacent := start - 1
	if delta > 0 {
		adjacent = end + 1
	}
	if adjacent >= 0 && adjacent < len(flags) && flags[adjacent] {
		a.handoff = handoffDenied
		a.statusMessage = "Cannot swap selection across a locked line."
		return
	}

	if delta < 0 {
		aboveLine := content[start-1]
		selected := append([]string(nil), content[start:end+1]...)
		copy(content[start-1:], append(selected, aboveLine))

		aboveLocked := flags[start-1]
		selectedLocked := append([]bool(nil), flags[start:end+1]...)
		copy(flags[start-1:], append(selectedLocked, aboveLocked))
		start--
		end--
	} else {
		belowLine := content[end+1]
		selected := append([]string(nil), content[start:end+1]...)
		copy(content[start:], append([]string{belowLine}, selected...))

		belowLocked := flags[end+1]
		selectedLocked := append([]bool(nil), flags[start:end+1]...)
		copy(flags[start:], append([]bool{belowLocked}, selectedLocked...))
		start++
		end++
	}

	a.tabs[a.activeTab].Content = content
	a.applyLockedFlags(flags)
	a.setSelectionAnchor(a.selectionAnchorX(), start)
	a.cursorY = end
	a.ensureCursorInBounds()
	a.handoff = handoffHumanLed
	a.statusMessage = fmt.Sprintf("Moved selected block to lines %d-%d.", start+1, end+1)
}

func (a *App) lockedFlags() []bool {
	content := a.tabs[a.activeTab].Content
	flags := make([]bool, len(content))
	for index := range content {
		flags[index] = a.tabs[a.activeTab].Locked[index]
	}
	return flags
}

func (a *App) applyLockedFlags(flags []bool) {
	updated := make(map[int]bool)
	for index, locked := range flags {
		if locked {
			updated[index] = true
		}
	}
	a.tabs[a.activeTab].Locked = updated
}

func (a *App) rangeLocked(start, end int, flags []bool) bool {
	for index := start; index <= end; index++ {
		if flags[index] {
			return true
		}
	}
	return false
}

func (a *App) activeLineRange() (int, int) {
	if start, end, ok := a.projectedLineRange(); ok {
		return start, end
	}
	return a.cursorY, a.cursorY
}

func (a *App) indentSelection() {
	start, end := a.activeLineRange()
	flags := a.lockedFlags()
	if a.rangeLocked(start, end, flags) {
		a.handoff = handoffDenied
		a.statusMessage = "Locked lines cannot be indented."
		return
	}

	for index := start; index <= end; index++ {
		a.tabs[a.activeTab].Content[index] = a.indentUnit() + a.tabs[a.activeTab].Content[index]
	}
	if a.hasSelection() {
		a.selectionAnchor = start
		a.cursorY = end
	} else {
		a.cursorX += len([]rune(a.indentUnit()))
	}
	a.handoff = handoffHumanLed
	if start == end {
		a.statusMessage = fmt.Sprintf("Indented line %d.", start+1)
	} else {
		a.statusMessage = fmt.Sprintf("Indented lines %d-%d.", start+1, end+1)
	}
}

func (a *App) outdentSelection() {
	start, end := a.activeLineRange()
	flags := a.lockedFlags()
	if a.rangeLocked(start, end, flags) {
		a.handoff = handoffDenied
		a.statusMessage = "Locked lines cannot be outdented."
		return
	}

	removedOnCursor := 0
	for index := start; index <= end; index++ {
		updated, removed := a.removeSingleIndent(a.tabs[a.activeTab].Content[index])
		a.tabs[a.activeTab].Content[index] = updated
		if index == a.cursorY {
			removedOnCursor = removed
		}
	}
	if !a.hasSelection() {
		a.cursorX -= removedOnCursor
		if a.cursorX < 0 {
			a.cursorX = 0
		}
	}
	a.handoff = handoffHumanLed
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
		a.handoff = handoffHumanLed
		a.statusMessage = fmt.Sprintf("No enclosing block found; selected line %d.", a.cursorY+1)
		return
	}

	if !hasSelection {
		candidate, ok := a.smallestBlockForLine(candidates, a.cursorY)
		if !ok {
			a.setSelectionAnchor(0, a.cursorY)
			a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
			a.handoff = handoffHumanLed
			a.statusMessage = fmt.Sprintf("No enclosing block found; selected line %d.", a.cursorY+1)
			return
		}
		a.setSelectionAnchor(0, candidate.start)
		a.cursorY = candidate.end
		a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
		a.ensureCursorInBounds()
		a.handoff = handoffHumanLed
		a.statusMessage = fmt.Sprintf("Selected code block lines %d-%d.", candidate.start+1, candidate.end+1)
		return
	}

	if candidate, ok := a.nextEnclosingBlock(candidates, currentStart, currentEnd); ok {
		a.setSelectionAnchor(0, candidate.start)
		a.cursorY = candidate.end
		a.cursorX = len([]rune(a.tabs[a.activeTab].Content[a.cursorY]))
		a.ensureCursorInBounds()
		a.handoff = handoffHumanLed
		a.statusMessage = fmt.Sprintf("Expanded to parent block lines %d-%d.", candidate.start+1, candidate.end+1)
		return
	}

	a.handoff = handoffHumanLed
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

func (a *App) currentLineLocked() bool {
	locked := a.tabs[a.activeTab].Locked
	return locked != nil && locked[a.cursorY]
}

func (a *App) buildReviewItems(preview Preview) []reviewItem {
	if !preview.Pending {
		return nil
	}

	state := a.previewState(preview)
	if preview.ProposalID == "" {
		return []reviewItem{{
			ID:     preview.ReviewLabel,
			Action: preview.Action,
			Target: preview.Target,
			State:  state,
		}}
	}

	return []reviewItem{{
		ID:     preview.ProposalID,
		Action: preview.Action,
		Target: preview.Target,
		State:  state,
	}}
}

func (a *App) previewState(preview Preview) handoffState {
	switch preview.Kind {
	case CommandDenied:
		return handoffDenied
	case CommandApprove:
		return handoffApplying
	default:
		return handoffReviewPending
	}
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
	if a.quitConfirm {
		a.screen.HideCursor()
		a.drawQuitDialog(w, h)
	}
	a.screen.Show()
}

func (a *App) drawTabs(width int) {
	x := 0
	for i, tab := range a.tabs {
		label := "[" + tab.Title + "]"
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
	for i := 0; i < len(content) && i < height-2; i++ {
		lineY := y + 1 + i
		line := trimRunes(content[i], width-5)
		gutter, style := a.lineVisual(i)
		a.drawText(x+1, lineY, styleGutter(), gutter)
		a.drawTextWithVisibleTabs(x+4, lineY, i, style, line)
	}
	footer := fmt.Sprintf("Focus:%s  Scope:%s  Indent:[Tab/Shift+Tab]  Width:[Alt+0..4]  Tabs:[Alt+,/Alt+. or Ctrl+Tab/Ctrl+Shift+Tab]  Block:[F2/Ctrl+Space/Ctrl+[]  Ctrl+Up/Down:[line or block]  Control:[Ctrl+G]  Quit:[Ctrl+Q]", a.focusLabel(), a.currentScope())
	if a.helpVisible {
		footer = fmt.Sprintf("Focus:%s  Scope:%s  Help:[Esc closes]", a.focusLabel(), a.currentScope())
	}
	a.drawText(x+1, y+height-1, styleMuted(), trimRunes(footer, width-2))
	if a.focus == focusEditor && !a.helpVisible && !a.quitConfirm {
		cursorX := x + 4 + a.cursorX
		cursorY := y + 1 + a.cursorY
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
	input := string(a.controlInput)
	a.drawText(x+1, y+1, styleNormal(), trimRunes("> "+input, width-2))
	preview := "Preview: type a command and press Enter"
	if a.preview.Pending {
		preview = "Preview: " + a.preview.Summary()
	}
	a.drawText(x+1, y+2, stylePreview(), trimRunes(preview, width-2))
	a.drawText(x+1, y+3, styleMuted(), trimRunes("Examples: inspect recent change | simplify this block | show diff only", width-2))
	if a.focus == focusControl {
		cursorX := x + 3 + a.controlCursor
		maxX := x + width - 2
		if cursorX > maxX {
			cursorX = maxX
		}
		a.screen.ShowCursor(cursorX, y+1)
	} else if a.focus != focusEditor {
		a.screen.HideCursor()
	}
}

func (a *App) drawStatusSurface(x, y, width, height int) {
	a.drawBox(x, y, width, height, styleBox(), "Status Surface")
	lines := []string{
		"tab: " + a.tabs[a.activeTab].Title,
		"scope: " + a.currentScope(),
		"indent: " + a.indentModeLabel(),
		"agent: " + a.agentState(),
		"handoff: " + string(a.handoff),
		fmt.Sprintf("review: %d pending", a.reviewCount),
		"voice: " + a.voiceState,
		"proposal: " + a.previewIDOrLastApplied(),
	}
	for i := 0; i < len(lines) && i < height-2; i++ {
		a.drawText(x+1, y+1+i, styleMuted(), trimRunes(lines[i], width-2))
	}
	if height >= 4 {
		reviewLine := "queue: none"
		if len(a.reviewItems) > 0 {
			item := a.reviewItems[0]
			reviewLine = fmt.Sprintf("queue: %s %s", item.ID, item.Target)
		}
		a.drawText(x+1, y+height-2, stylePreview(), trimRunes(reviewLine, width-2))
	}
}

func (a *App) drawHelpDialog(screenWidth, screenHeight int) {
	lines := []string{
		"gdedit help",
		"",
		"F1 opens this help dialog.",
		"Esc closes the help dialog, clears a preview, or clears a selection.",
		"",
		"Editor",
		"  Tab              insert literal tab, or indent active selection",
		"  Shift+Tab        outdent current line or selection",
		"  Alt+.            next tab",
		"  Alt+,            previous tab",
		"  Alt+0            use literal tabs for selection indentation",
		"  Alt+1..Alt+4     set indentation width in spaces",
		"  colored »        visible marker for literal tab characters",
		"  Ctrl+Tab         same intent, but many terminals swallow it",
		"  Ctrl+Shift+Tab   same intent, but many terminals swallow it",
		"  F2               select current block, then parent block",
		"  Ctrl+Space       same intent, but some terminals drop it",
		"  Ctrl+[           same intent, but some terminals treat it as Esc",
		"  Ctrl+Down        insert a blank line above, or move selected block",
		"  Ctrl+Up          remove a blank line above, or move selected block",
		"  Home / End       move caret to start or end of line",
		"  PageUp / PageDn  move caret by larger vertical steps",
		"  Ctrl+G           focus control hub",
		"  Shift/Alt+Arrow  expand or shrink text selection",
		"  Ctrl+Arrow       move caret by word",
		"  Ctrl+Shift/Alt   select by word with Left/Right",
		"  Shift/Alt+Home   select to line start",
		"  Shift/Alt+End    select to line end",
		"  Shift/Alt+Page   select by larger vertical steps",
		"  Ctrl+C / Ctrl+X  copy or cut selection",
		"  Ctrl+V           paste clipboard at caret",
		"  Ctrl+Up/Down     swap selected block with adjacent line",
		"  Ctrl+Q           open quit confirmation",
		"",
		"Control hub",
		"  Type a short command, then press Enter for preview.",
		"  Press Enter again to confirm apply.",
		"  Example: simplify this block",
		"  Example: inspect recent change",
		"",
		"Cowork states",
		"  L! locked region   P> proposal   R? review-needed",
		"  caret-range text selection   A+ approved   X! denied",
	}

	maxLen := 0
	for _, line := range lines {
		if n := len([]rune(line)); n > maxLen {
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

	for dy := y; dy < y+height; dy++ {
		for dx := x; dx < x+width; dx++ {
			a.screen.SetContent(dx, dy, ' ', nil, styleDialogFill())
		}
	}

	a.drawBox(x, y, width, height, styleDialogBorder(), "Help")
	for i := 0; i < len(lines) && i < height-2; i++ {
		style := styleDialogText()
		if i == 0 {
			style = styleDialogTitle()
		}
		a.drawText(x+2, y+1+i, style, trimRunes(lines[i], width-4))
	}
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
		return "review"
	}
	if a.handoff == handoffDenied {
		return "denied"
	}
	if a.focus == focusControl {
		return "suggesting"
	}
	return "idle"
}

func (a *App) previewIDOrLastApplied() string {
	if a.preview.ProposalID != "" {
		return a.preview.ProposalID
	}
	if a.lastApplied != "" {
		return a.lastApplied
	}
	if a.preview.Kind == CommandDenied {
		return "locked-scope"
	}
	return "none"
}

func (a *App) lineVisual(index int) (string, tcell.Style) {
	locked := a.tabs[a.activeTab].Locked[index]
	if locked {
		return "L! ", styleLocked()
	}
	if a.preview.Pending && index == a.cursorY {
		switch a.preview.Kind {
		case CommandDenied:
			return "X! ", styleDenied()
		case CommandPropose:
			return "P> ", styleProposal()
		case CommandReview, CommandInspect:
			return "R? ", styleReviewNeeded()
		case CommandApprove:
			return "A+ ", styleApproved()
		}
	}
	if index == a.cursorY {
		return ">  ", styleCursorLine()
	}
	if a.lastApplied != "" && index == a.cursorY {
		return "A+ ", styleApproved()
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
	for i, r := range []rune(text) {
		a.screen.SetContent(x+i, y, r, nil, style)
	}
}

func (a *App) drawTextWithVisibleTabs(x, y, lineIndex int, style tcell.Style, text string) {
	for i, r := range []rune(text) {
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
		a.screen.SetContent(x+i, y, cellRune, nil, cellStyle)
	}
}

func trimRunes(input string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= max {
		return input
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "..."
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
