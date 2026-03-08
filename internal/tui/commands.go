package tui

import (
	"fmt"
	"strings"
)

type CommandKind string

const (
	CommandInspect CommandKind = "inspect"
	CommandEdit    CommandKind = "edit"
	CommandTalk    CommandKind = "talk"
	CommandMemo    CommandKind = "memo"
	CommandSwitch  CommandKind = "switch"
	CommandRoute   CommandKind = "route"
)

type Preview struct {
	Kind    CommandKind
	Action  string
	Target  string
	Tab     string
	Pending bool
}

func (p Preview) Summary() string {
	if !p.Pending {
		return ""
	}
	return fmt.Sprintf("%s -> %s @ %s", p.Action, p.Target, p.Tab)
}

func BuildPreview(input, scope, tab string) Preview {
	normalized := strings.ToLower(strings.TrimSpace(input))
	kind, action := parseAction(normalized)
	return Preview{
		Kind:    kind,
		Action:  action,
		Target:  scope,
		Tab:     tab,
		Pending: true,
	}
}

func commandRequiresConfirmation(kind CommandKind) bool {
	switch kind {
	case CommandTalk, CommandInspect:
		return false
	default:
		return true
	}
}

func parseAction(input string) (CommandKind, string) {
	switch {
	case input == "":
		return CommandRoute, "no-op"
	case isMemoCommand(input):
		return CommandMemo, "save memo for current file"
	case isGreeting(input):
		return CommandTalk, "talk with edit agent"
	case strings.Contains(input, "inspect") || strings.Contains(input, "check") || strings.Contains(input, "explain"):
		return CommandInspect, "inspect current scope"
	case strings.Contains(input, "rename") || strings.Contains(input, "simplify") || strings.Contains(input, "refactor") || strings.Contains(input, "patch") || strings.Contains(input, "edit") || strings.Contains(input, "rewrite"):
		return CommandEdit, "edit current scope"
	case strings.Contains(input, "switch") && strings.Contains(input, "tab"):
		return CommandSwitch, "switch tab context"
	default:
		return CommandRoute, "resolve against active edit context"
	}
}

func isMemoCommand(input string) bool {
	_, ok := memoCommandPayload(input)
	return ok
}

func memoCommandPayload(input string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "memo ") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "memo ")), true
	}
	if strings.HasPrefix(trimmed, "메모 ") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "메모 ")), true
	}
	if strings.Contains(trimmed, "메모해") {
		return trimmed, true
	}
	return "", false
}

func isGreeting(input string) bool {
	trimmed := strings.TrimSpace(input)
	switch trimmed {
	case "hello", "hi", "hey", "안녕", "안녕하세요", "반가워", "반갑습니다":
		return true
	default:
		return false
	}
}
