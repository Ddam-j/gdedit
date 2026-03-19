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
	if commandRequiresConfirmation(CommandOpen) {
		t.Fatal("open should execute without confirmation")
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
		{input: "이 내용을 메모를 추가해줘", wantKind: CommandMemo, wantText: "save memo for current file"},
		{input: "wezterm 설정 메모 저장해줘", wantKind: CommandMemo, wantText: "save memo for current file"},
		{input: "그러면 메모를 추가해줘 -> -.test", wantKind: CommandMemo, wantText: "save memo for current file"},
		{input: "내가 작성한 것을 추가해줄 수 있어? -> -.Test", wantKind: CommandMemo, wantText: "save memo for current file"},
		{input: "이 파일에 해당하는 메모를 읽어서 장점을 말해줘", wantKind: CommandRoute, wantText: "resolve against active edit context"},
		{input: "메모리 설정을 설명해줘", wantKind: CommandRoute, wantText: "resolve against active edit context"},
		{input: "/open notes.txt", wantKind: CommandOpen, wantText: "open file in a new tab"},
		{input: "/sync mynamr demo-rule", wantKind: CommandSync, wantText: "open sync-backed buffer in a new tab"},
		{input: "/write notes.txt", wantKind: CommandWrite, wantText: "save current tab to path"},
		{input: "/saveas notes.txt", wantKind: CommandWrite, wantText: "save current tab to path"},
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

func TestIsNaturalLanguageMemoRequest(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "설정 내용을 메모해줘", want: true},
		{input: "이 내용을 메모를 추가해줘", want: true},
		{input: "wezterm 설정 메모 저장해줘", want: true},
		{input: "add memo for this file", want: true},
		{input: "이 파일에 해당하는 메모를 읽어서 장점을 말해줘", want: false},
		{input: "메모리 설정을 설명해줘", want: false},
		{input: "please explain this config", want: false},
	}

	for _, test := range tests {
		if got := isNaturalLanguageMemoRequest(test.input); got != test.want {
			t.Fatalf("isNaturalLanguageMemoRequest(%q) = %t, want %t", test.input, got, test.want)
		}
	}
}

func TestMemoCommandPayloadPrefersExplicitArrowPayload(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{input: "그러면 메모를 추가해줘 -> -.test", want: "-.test", wantOK: true},
		{input: "내가 작성한 것을 추가해줄 수 있어? -> -.Test", want: "-.Test", wantOK: true},
		{input: "/open foo -> bar", want: "", wantOK: false},
	}

	for _, test := range tests {
		got, ok := memoCommandPayload(test.input)
		if ok != test.wantOK || got != test.want {
			t.Fatalf("memoCommandPayload(%q) = (%q, %t), want (%q, %t)", test.input, got, ok, test.want, test.wantOK)
		}
	}
}

func TestOpenCommandPayload(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{input: "/open notes.txt", want: "notes.txt", wantOK: true},
		{input: " /open ./docs/readme.md ", want: "./docs/readme.md", wantOK: true},
		{input: "/open", want: "", wantOK: false},
		{input: "open notes.txt", want: "", wantOK: false},
	}

	for _, test := range tests {
		got, ok := openCommandPayload(test.input)
		if ok != test.wantOK || got != test.want {
			t.Fatalf("openCommandPayload(%q) = (%q, %t), want (%q, %t)", test.input, got, ok, test.want, test.wantOK)
		}
	}
}

func TestWriteCommandPayload(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{input: "/write notes.txt", want: "notes.txt", wantOK: true},
		{input: "/saveas \"notes with spaces.txt\"", want: "notes with spaces.txt", wantOK: true},
		{input: "/write './docs/file name.md'", want: "./docs/file name.md", wantOK: true},
		{input: "/write", want: "", wantOK: false},
		{input: "/saveas", want: "", wantOK: false},
	}

	for _, test := range tests {
		got, ok := writeCommandPayload(test.input)
		if ok != test.wantOK || got != test.want {
			t.Fatalf("writeCommandPayload(%q) = (%q, %t), want (%q, %t)", test.input, got, ok, test.want, test.wantOK)
		}
	}
}

func TestSyncCommandPayload(t *testing.T) {
	tests := []struct {
		input  string
		wantID string
		want   string
		wantOK bool
	}{
		{input: "/sync mynamr demo-rule", wantID: "mynamr", want: "demo-rule", wantOK: true},
		{input: "/rule demo-rule", wantID: "mynamr", want: "demo-rule", wantOK: true},
		{input: "/mynamr sample.rule", wantID: "mynamr", want: "sample.rule", wantOK: true},
		{input: "/sync", wantID: "", want: "", wantOK: false},
	}

	for _, test := range tests {
		gotID, got, ok := syncCommandPayload(test.input)
		if ok != test.wantOK || got != test.want || gotID != test.wantID {
			t.Fatalf("syncCommandPayload(%q) = (%q, %q, %t), want (%q, %q, %t)", test.input, gotID, got, ok, test.wantID, test.want, test.wantOK)
		}
	}
}
