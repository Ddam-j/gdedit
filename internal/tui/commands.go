package tui

import "strings"

type Preview struct {
	Action  string
	Target  string
	Tab     string
	Pending bool
}

func (p Preview) Summary() string {
	if !p.Pending {
		return ""
	}

	return p.Action + " -> " + p.Target + " @ " + p.Tab
}

func BuildPreview(input, scope, tab string) Preview {
	normalized := strings.ToLower(strings.TrimSpace(input))
	action := parseAction(normalized)

	return Preview{
		Action:  action,
		Target:  scope,
		Tab:     tab,
		Pending: true,
	}
}

func parseAction(input string) string {
	switch {
	case input == "":
		return "no-op"
	case strings.Contains(input, "inspect") || strings.Contains(input, "check"):
		return "inspect current context"
	case strings.Contains(input, "rename"):
		return "prepare rename preview"
	case strings.Contains(input, "simplify") || strings.Contains(input, "refactor"):
		return "propose bounded refactor"
	case strings.Contains(input, "diff"):
		return "show diff review"
	case strings.Contains(input, "hold"):
		return "hold for review"
	case strings.Contains(input, "approve"):
		return "approve pending proposal"
	case strings.Contains(input, "switch") && strings.Contains(input, "tab"):
		return "switch tab context"
	default:
		return "route command through control hub"
	}
}
