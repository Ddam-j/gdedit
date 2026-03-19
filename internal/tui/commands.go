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
	CommandOpen    CommandKind = "open"
	CommandWrite   CommandKind = "write"
	CommandSync    CommandKind = "sync"
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
	kind, action := parseAction(input)
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
	case CommandTalk, CommandInspect, CommandOpen, CommandWrite, CommandSync:
		return false
	default:
		return true
	}
}

func parseAction(input string) (CommandKind, string) {
	if _, ok := openCommandPayload(input); ok {
		return CommandOpen, "open file in a new tab"
	}
	if _, _, ok := syncCommandPayload(input); ok {
		return CommandSync, "open sync-backed buffer in a new tab"
	}
	if _, ok := writeCommandPayload(input); ok {
		return CommandWrite, "save current tab to path"
	}
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch {
	case normalized == "":
		return CommandRoute, "no-op"
	case isMemoCommand(normalized):
		return CommandMemo, "save memo for current file"
	case isGreeting(normalized):
		return CommandTalk, "talk with edit agent"
	case strings.Contains(normalized, "inspect") || strings.Contains(normalized, "check") || strings.Contains(normalized, "explain"):
		return CommandInspect, "inspect current scope"
	case strings.Contains(normalized, "rename") || strings.Contains(normalized, "simplify") || strings.Contains(normalized, "refactor") || strings.Contains(normalized, "patch") || strings.Contains(normalized, "edit") || strings.Contains(normalized, "rewrite"):
		return CommandEdit, "edit current scope"
	case strings.Contains(normalized, "switch") && strings.Contains(normalized, "tab"):
		return CommandSwitch, "switch tab context"
	default:
		return CommandRoute, "resolve against active edit context"
	}
}

func openCommandPayload(input string) (string, bool) {
	return slashPathPayload(input, "open")
}

func writeCommandPayload(input string) (string, bool) {
	if payload, ok := slashPathPayload(input, "write"); ok {
		return payload, true
	}
	return slashPathPayload(input, "saveas")
}

func syncCommandPayload(input string) (string, string, bool) {
	trimmed := strings.TrimSpace(input)
	for _, command := range []string{"/sync", "/rule", "/mynamr"} {
		if !strings.HasPrefix(strings.ToLower(trimmed), command) {
			continue
		}
		payload := strings.TrimSpace(trimmed[len(command):])
		parts := strings.Fields(payload)
		if command == "/sync" {
			if len(parts) < 2 {
				return "", "", false
			}
			return parts[0], strings.Join(parts[1:], " "), true
		}
		if len(parts) < 1 {
			return "", "", false
		}
		return "mynamr", strings.Join(parts, " "), true
	}
	return "", "", false
}

func slashPathPayload(input string, command string) (string, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", false
	}
	prefix := "/" + strings.ToLower(command)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, prefix) {
		return "", false
	}
	payload := strings.TrimSpace(trimmed[len(prefix):])
	if payload == "" {
		return "", false
	}
	if path, ok := unquoteSlashPayload(payload); ok {
		return path, true
	}
	return payload, true
}

func unquoteSlashPayload(payload string) (string, bool) {
	if len(payload) < 2 {
		return "", false
	}
	quote := payload[0]
	if quote != '"' && quote != '\'' {
		return "", false
	}
	if payload[len(payload)-1] != quote {
		return "", false
	}
	unquoted := payload[1 : len(payload)-1]
	if strings.TrimSpace(unquoted) == "" {
		return "", false
	}
	return unquoted, true
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
	if _, payload, ok := explicitMemoPayload(trimmed); ok {
		return payload, true
	}
	if strings.HasPrefix(trimmed, "memo ") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "memo ")), true
	}
	if strings.HasPrefix(trimmed, "메모 ") {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, "메모 ")), true
	}
	if isNaturalLanguageMemoRequest(trimmed) {
		return trimmed, true
	}
	return "", false
}

func isNaturalLanguageMemoRequest(input string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "메모해") {
		return true
	}
	koreanMemoIntent := hasKoreanMemoNoun(trimmed) && containsAny(trimmed,
		"추가", "저장", "기록", "남겨", "적어", "써줘", "작성", "정리", "반영",
	)
	englishMemoIntent := strings.Contains(trimmed, "memo") && containsAny(trimmed,
		"add", "save", "record", "write", "store", "append", "keep",
	)
	return koreanMemoIntent || englishMemoIntent
}

func explicitMemoPayload(input string) (string, string, bool) {
	trimmed := strings.TrimSpace(input)
	parts := strings.SplitN(trimmed, "->", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if right == "" {
		return "", "", false
	}
	if left == "" {
		return "", right, true
	}
	if strings.HasPrefix(left, "/") {
		return "", "", false
	}
	return left, right, true
}

func hasKoreanMemoNoun(input string) bool {
	return containsAny(input,
		"메모 ", "메모를", "메모가", "메모는", "메모에", "메모로", "메모도",
		"메모파일", "메모 파일", "메모내용", "메모 내용",
	)
}

func containsAny(input string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(input, needle) {
			return true
		}
	}
	return false
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
