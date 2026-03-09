package memo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadContextIncludesSystemAndProjectMemos(t *testing.T) {
	systemRoot := filepath.Join(t.TempDir(), "system-memos")
	workspace := t.TempDir()
	projectRoot := filepath.Join(workspace, ".gdedit")
	if err := os.MkdirAll(systemRoot, 0o755); err != nil {
		t.Fatalf("mkdir system root: %v", err)
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(systemRoot, "windows.md"), []byte("terminal setup note"), 0o600); err != nil {
		t.Fatalf("write system memo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "feature.md"), []byte("project intent note"), 0o600); err != nil {
		t.Fatalf("write project memo: %v", err)
	}

	context, err := LoadContext(systemRoot, workspace)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if !strings.Contains(context, "system memos:") {
		t.Fatalf("missing system memo section: %s", context)
	}
	if !strings.Contains(context, "terminal setup note") {
		t.Fatalf("missing system memo content: %s", context)
	}
	if !strings.Contains(context, "project memos:") {
		t.Fatalf("missing project memo section: %s", context)
	}
	if !strings.Contains(context, "project intent note") {
		t.Fatalf("missing project memo content: %s", context)
	}
}

func TestSaveFileMemoWritesProjectMemo(t *testing.T) {
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "configs", "app.yaml")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("mkdir file dir: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("name: app\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	memoPath, err := SaveFileMemo("", workspace, filePath, "Keep this setting aligned with the production proxy.")
	if err != nil {
		t.Fatalf("save file memo: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(memoPath), "/.gdedit/memos/") {
		t.Fatalf("unexpected memo path: %s", memoPath)
	}
	content, err := os.ReadFile(memoPath)
	if err != nil {
		t.Fatalf("read memo file: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "file: "+filepath.ToSlash(filePath)) {
		t.Fatalf("missing file path in memo: %s", text)
	}
	if !strings.Contains(text, "details:") {
		t.Fatalf("missing details section in memo: %s", text)
	}
	if !strings.Contains(text, "- Keep this setting aligned with the production proxy.") {
		t.Fatalf("missing note in memo: %s", text)
	}
}

func TestSaveFileMemoRoutesAppConfigToSystemAppMemo(t *testing.T) {
	home := t.TempDir()
	originalHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	defer func() { userHomeDir = originalHome }()

	systemRoot := filepath.Join(t.TempDir(), "memo-root")
	workspace := t.TempDir()
	filePath := filepath.Join(home, ".wezterm.lua")
	if err := os.WriteFile(filePath, []byte("return {}\n"), 0o600); err != nil {
		t.Fatalf("write app config: %v", err)
	}

	memoPath, err := SaveFileMemo(systemRoot, workspace, filePath, "Resolved WezTerm settings summary.")
	if err != nil {
		t.Fatalf("save app memo: %v", err)
	}
	if got := filepath.ToSlash(memoPath); !strings.Contains(got, "/app/wezterm.md") {
		t.Fatalf("unexpected app memo path: %s", memoPath)
	}
	content, err := os.ReadFile(memoPath)
	if err != nil {
		t.Fatalf("read app memo: %v", err)
	}
	if !strings.Contains(string(content), "Resolved WezTerm settings summary.") {
		t.Fatalf("missing app memo content: %s", string(content))
	}
}

func TestBuildMemoEntryFormatsReadableDetails(t *testing.T) {
	entry := buildMemoEntry("/tmp/app.conf", "First sentence. Second sentence with more detail! Third sentence?")
	if !strings.Contains(entry, "details:") {
		t.Fatalf("missing details section: %s", entry)
	}
	if !strings.Contains(entry, "- First sentence.") {
		t.Fatalf("missing first bullet: %s", entry)
	}
	if !strings.Contains(entry, "- Second sentence with more detail!") {
		t.Fatalf("missing second bullet: %s", entry)
	}
	if !strings.Contains(entry, "- Third sentence?") {
		t.Fatalf("missing third bullet: %s", entry)
	}
}
