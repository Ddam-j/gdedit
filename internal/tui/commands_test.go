package tui

import "testing"

func TestBuildPreviewUsesScopeAndTab(t *testing.T) {
	preview := BuildPreview("inspect recent change", "selection:L4", "main.go")

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
}

func TestParseAction(t *testing.T) {
	tests := map[string]string{
		"rename this symbol":      "prepare rename preview",
		"simplify this block":     "propose bounded refactor",
		"show diff only":          "show diff review",
		"hold this proposal":      "hold for review",
		"approve this proposal":   "approve pending proposal",
		"switch tab":              "switch tab context",
		"do something surprising": "route command through control hub",
	}

	for input, want := range tests {
		got := parseAction(input)
		if got != want {
			t.Fatalf("parseAction(%q) = %q, want %q", input, got, want)
		}
	}
}
