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
	if preview.Action != "inspect current scope" {
		t.Fatalf("unexpected action: %s", preview.Action)
	}
	if preview.Kind != CommandInspect {
		t.Fatalf("unexpected kind: %s", preview.Kind)
	}
}

func TestBuildPreviewTreatsRefactorAsScopedEdit(t *testing.T) {
	preview := BuildPreview("rename this symbol", "selection:L1", "main.go")

	if preview.Kind != CommandEdit {
		t.Fatalf("unexpected kind: %s", preview.Kind)
	}
	if preview.Action != "edit current scope" {
		t.Fatalf("unexpected action: %s", preview.Action)
	}
}

func TestCommandRequiresConfirmationPolicy(t *testing.T) {
	if commandRequiresConfirmation(CommandTalk) {
		t.Fatal("talk should execute without confirmation")
	}
	if commandRequiresConfirmation(CommandInspect) {
		t.Fatal("inspect should execute without confirmation")
	}
	if !commandRequiresConfirmation(CommandEdit) {
		t.Fatal("edit should still require confirmation")
	}
	if !commandRequiresConfirmation(CommandMemo) {
		t.Fatal("memo should still require confirmation")
	}
}

func TestParseAction(t *testing.T) {
	tests := []struct {
		input    string
		wantKind CommandKind
		wantText string
	}{
		{input: "memo remember this setting", wantKind: CommandMemo, wantText: "save memo for current file"},
		{input: "메모 이 파일은 유지", wantKind: CommandMemo, wantText: "save memo for current file"},
		{input: "설정 내용을 메모해줘 메모파일은 앱의 이름으로", wantKind: CommandMemo, wantText: "save memo for current file"},
		{input: "메모리 설정을 설명해줘", wantKind: CommandRoute, wantText: "resolve against active edit context"},
		{input: "안녕하세요", wantKind: CommandTalk, wantText: "talk with edit agent"},
		{input: "hello", wantKind: CommandTalk, wantText: "talk with edit agent"},
		{input: "rename this symbol", wantKind: CommandEdit, wantText: "edit current scope"},
		{input: "simplify this block", wantKind: CommandEdit, wantText: "edit current scope"},
		{input: "show diff only", wantKind: CommandRoute, wantText: "resolve against active edit context"},
		{input: "hold this proposal", wantKind: CommandRoute, wantText: "resolve against active edit context"},
		{input: "approve this proposal", wantKind: CommandRoute, wantText: "resolve against active edit context"},
		{input: "switch tab", wantKind: CommandSwitch, wantText: "switch tab context"},
		{input: "do something surprising", wantKind: CommandRoute, wantText: "resolve against active edit context"},
	}

	for _, test := range tests {
		gotKind, gotText := parseAction(test.input)
		if gotKind != test.wantKind || gotText != test.wantText {
			t.Fatalf("parseAction(%q) = (%s, %q), want (%s, %q)", test.input, gotKind, gotText, test.wantKind, test.wantText)
		}
	}
}
