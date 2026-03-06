package tui

import "testing"

func TestBuildPreviewUsesScopeAndTab(t *testing.T) {
	preview := BuildPreview("inspect recent change", "selection:L4", "main.go", false)

	if !preview.Pending {
		t.Fatal("expected preview to be pending")
	}
	if preview.Target != "selection:L4" {
		t.Fatalf("unexpected target: %s", preview.Target)
	}
	if preview.Tab != "main.go" {
		t.Fatalf("unexpected tab: %s", preview.Tab)
	}
	if preview.Action != "inspect current context" {
		t.Fatalf("unexpected action: %s", preview.Action)
	}
	if preview.Kind != CommandInspect {
		t.Fatalf("unexpected kind: %s", preview.Kind)
	}
}

func TestBuildPreviewDeniesLockedEdits(t *testing.T) {
	preview := BuildPreview("rename this symbol", "selection:L1", "main.go", true)

	if preview.Kind != CommandDenied {
		t.Fatalf("unexpected kind: %s", preview.Kind)
	}
	if preview.Action != "deny change in locked region" {
		t.Fatalf("unexpected action: %s", preview.Action)
	}
	if preview.ProposalID != "" {
		t.Fatalf("expected no proposal id, got %s", preview.ProposalID)
	}
}

func TestParseAction(t *testing.T) {
	tests := []struct {
		input    string
		wantKind CommandKind
		wantText string
	}{
		{input: "rename this symbol", wantKind: CommandPropose, wantText: "prepare rename preview"},
		{input: "simplify this block", wantKind: CommandPropose, wantText: "propose bounded refactor"},
		{input: "show diff only", wantKind: CommandReview, wantText: "show diff review"},
		{input: "hold this proposal", wantKind: CommandReview, wantText: "hold for review"},
		{input: "approve this proposal", wantKind: CommandApprove, wantText: "approve pending proposal"},
		{input: "switch tab", wantKind: CommandSwitch, wantText: "switch tab context"},
		{input: "do something surprising", wantKind: CommandRoute, wantText: "route command through control hub"},
	}

	for _, test := range tests {
		gotKind, gotText := parseAction(test.input)
		if gotKind != test.wantKind || gotText != test.wantText {
			t.Fatalf("parseAction(%q) = (%s, %q), want (%s, %q)", test.input, gotKind, gotText, test.wantKind, test.wantText)
		}
	}
}
