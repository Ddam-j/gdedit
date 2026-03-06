package tui

import (
	"fmt"
	"strings"
)

type CommandKind string

const (
	CommandInspect CommandKind = "inspect"
	CommandPropose CommandKind = "propose"
	CommandReview  CommandKind = "review"
	CommandApprove CommandKind = "approve"
	CommandSwitch  CommandKind = "switch"
	CommandDenied  CommandKind = "denied"
	CommandRoute   CommandKind = "route"
)

type Preview struct {
	Kind        CommandKind
	Action      string
	Target      string
	Tab         string
	Pending     bool
	ProposalID  string
	ReviewLabel string
}

func (p Preview) Summary() string {
	if !p.Pending {
		return ""
	}

	base := p.Action + " -> " + p.Target + " @ " + p.Tab
	if p.ProposalID == "" {
		return base
	}

	return fmt.Sprintf("%s [%s]", base, p.ProposalID)
}

func BuildPreview(input, scope, tab string, locked bool) Preview {
	normalized := strings.ToLower(strings.TrimSpace(input))
	kind, action := parseAction(normalized)
	preview := Preview{
		Kind:        kind,
		Action:      action,
		Target:      scope,
		Tab:         tab,
		Pending:     true,
		ReviewLabel: reviewLabelFor(kind),
	}

	if locked && kind != CommandInspect && kind != CommandReview {
		preview.Kind = CommandDenied
		preview.Action = "deny change in locked region"
		preview.ReviewLabel = "locked"
		return preview
	}

	if preview.Kind == CommandPropose || preview.Kind == CommandReview || preview.Kind == CommandApprove {
		preview.ProposalID = proposalIDFor(tab, scope)
	}

	return preview
}

func parseAction(input string) (CommandKind, string) {
	switch {
	case input == "":
		return CommandRoute, "no-op"
	case strings.Contains(input, "inspect") || strings.Contains(input, "check") || strings.Contains(input, "explain"):
		return CommandInspect, "inspect current context"
	case strings.Contains(input, "rename"):
		return CommandPropose, "prepare rename preview"
	case strings.Contains(input, "simplify") || strings.Contains(input, "refactor") || strings.Contains(input, "patch"):
		return CommandPropose, "propose bounded refactor"
	case strings.Contains(input, "diff") || strings.Contains(input, "review") || strings.Contains(input, "highlight"):
		return CommandReview, "show diff review"
	case strings.Contains(input, "hold"):
		return CommandReview, "hold for review"
	case strings.Contains(input, "approve") || strings.Contains(input, "apply"):
		return CommandApprove, "approve pending proposal"
	case strings.Contains(input, "switch") && strings.Contains(input, "tab"):
		return CommandSwitch, "switch tab context"
	default:
		return CommandRoute, "route command through control hub"
	}
}

func reviewLabelFor(kind CommandKind) string {
	switch kind {
	case CommandInspect:
		return "analysis"
	case CommandPropose:
		return "proposal"
	case CommandReview:
		return "review"
	case CommandApprove:
		return "approval"
	case CommandSwitch:
		return "navigation"
	case CommandDenied:
		return "denied"
	default:
		return "control"
	}
}

func proposalIDFor(tab, scope string) string {
	tabToken := strings.ToUpper(strings.TrimSuffix(tab, ".go"))
	tabToken = strings.TrimSuffix(tabToken, ".DIFF")
	tabToken = strings.TrimSuffix(tabToken, ".MD")
	tabToken = strings.ReplaceAll(tabToken, " ", "-")
	if tabToken == "" {
		tabToken = "TAB"
	}
	scopeToken := strings.NewReplacer(":", "-", ".", "-", "/", "-").Replace(scope)
	return tabToken + "-" + scopeToken
}
