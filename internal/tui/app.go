package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

const (
	minWidth  = 60
	minHeight = 12
)

type focusArea int

const (
	focusEditor focusArea = iota
	focusControl
)

type Tab struct {
	Title   string
	Content []string
	Locked  map[int]bool
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

type App struct {
	screen        tcell.Screen
	tabs          []Tab
	activeTab     int
	focus         focusArea
	controlInput  []rune
	controlCursor int
	preview       Preview
	cursorX       int
	cursorY       int
	selectionOn   bool
	statusMessage string
	voiceState    string
	reviewCount   int
	handoff       handoffState
	reviewItems   []reviewItem
	lastApplied   string
}

func New() *App {
	return &App{
		tabs: []Tab{
			{
				Title: "main.go",
				Content: []string{
					"package main",
					"",
					"func main() {",
					"	// Phase 5 shell placeholder",
					"	println(\"gdedit\")",
					"}",
				},
				Locked: map[int]bool{0: true, 2: true},
			},
			{
				Title: "review.diff",
				Content: []string{
					"@@ preview",
					"- old line",
					"+ proposed line",
					"",
					"Use the control hub to inspect or hold proposals.",
				},
				Locked: map[int]bool{0: true},
			},
			{
				Title: "notes.md",
				Content: []string{
					"- One control hub",
					"- Multiple edit tabs",
					"- Voice flows into control",
					"- Preview before apply",
				},
				Locked: map[int]bool{},
			},
		},
		focus:         focusEditor,
		statusMessage: "Ready. Ctrl+G focuses the control hub.",
		voiceState:    "off",
		handoff:       handoffHumanLed,
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
	if ev.Key() == tcell.KeyCtrlC {
		return true
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
		a.nextTab()
		return false
	case tcell.KeyBacktab:
		a.prevTab()
		return false
	case tcell.KeyUp:
		a.cursorY--
	case tcell.KeyDown:
		a.cursorY++
	case tcell.KeyLeft:
		a.cursorX--
	case tcell.KeyRight:
		a.cursorX++
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q':
			return true
		case 'v':
			a.selectionOn = !a.selectionOn
			if a.selectionOn {
				a.statusMessage = "Selection enabled for current cursor line."
			} else {
				a.statusMessage = "Selection cleared."
			}
			a.handoff = handoffHumanLed
		case ']':
			a.nextTab()
		case '[':
			a.prevTab()
		case ':':
			a.focus = focusControl
			a.voiceState = "ready"
			a.handoff = handoffAgentSuggest
			a.statusMessage = "Control hub focused from editor shortcut."
		}
	}

	a.ensureCursorInBounds()
	return false
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
		if r == 'q' && len(a.controlInput) == 0 && !a.preview.Pending {
			return true
		}
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
	a.activeTab = (a.activeTab + 1) % len(a.tabs)
	a.ensureCursorInBounds()
	a.statusMessage = "Switched to tab " + a.tabs[a.activeTab].Title
}

func (a *App) prevTab() {
	a.activeTab--
	if a.activeTab < 0 {
		a.activeTab = len(a.tabs) - 1
	}
	a.ensureCursorInBounds()
	a.statusMessage = "Switched to tab " + a.tabs[a.activeTab].Title
}

func (a *App) currentScope() string {
	if a.selectionOn {
		return fmt.Sprintf("selection:L%d", a.cursorY+1)
	}
	return fmt.Sprintf("cursor:L%d:C%d", a.cursorY+1, a.cursorX+1)
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
		line := content[i]
		gutter, style := a.lineVisual(i)
		a.drawText(x+1, lineY, styleGutter(), gutter)
		a.drawText(x+4, lineY, style, trimRunes(line, width-5))
	}
	footer := fmt.Sprintf("Focus:%s  Scope:%s  Tabs:[Tab/Shift+Tab]  Control:[Ctrl+G/:]  Toggle Selection:[v]  Quit:[q/Ctrl+C]", a.focusLabel(), a.currentScope())
	a.drawText(x+1, y+height-1, styleMuted(), trimRunes(footer, width-2))
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
	} else {
		a.screen.HideCursor()
	}
}

func (a *App) drawStatusSurface(x, y, width, height int) {
	a.drawBox(x, y, width, height, styleBox(), "Status Surface")
	lines := []string{
		"tab: " + a.tabs[a.activeTab].Title,
		"scope: " + a.currentScope(),
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
	if a.selectionOn && index == a.cursorY {
		return "H* ", styleSelection()
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
