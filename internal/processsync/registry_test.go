package processsync

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRegisterAndResolve(t *testing.T) {
	home := t.TempDir()
	originalHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	defer func() { userHomeDir = originalHome }()

	path, err := Register("mynamr", "mynamr rule show {name} --spec-only", "mynamr rule update {name} --spec-stdin")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if got := filepath.ToSlash(path); got != filepath.ToSlash(filepath.Join(home, ".config", "gdedit", "process_sync.json")) {
		t.Fatalf("unexpected registry path: %s", path)
	}
	entry, err := Resolve("mynamr")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if entry.ReadFormat != "mynamr rule show {name} --spec-only" {
		t.Fatalf("unexpected read format: %q", entry.ReadFormat)
	}
	if entry.WriteFormat != "mynamr rule update {name} --spec-stdin" {
		t.Fatalf("unexpected write format: %q", entry.WriteFormat)
	}
}

func TestRegisterRejectsConflictingDefinition(t *testing.T) {
	home := t.TempDir()
	originalHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	defer func() { userHomeDir = originalHome }()

	if _, err := Register("demo", "read {name}", "write {name}"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := Register("demo", "read other {name}", "write {name}"); err == nil {
		t.Fatal("expected conflicting registration to fail")
	}
}

func TestExpandReplacesNamePlaceholder(t *testing.T) {
	got := Expand("mynamr rule show {name} --spec-only", "demo-rule")
	if got != "mynamr rule show demo-rule --spec-only" {
		t.Fatalf("unexpected expanded format: %q", got)
	}
}

func TestListAndRemove(t *testing.T) {
	home := t.TempDir()
	originalHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	defer func() { userHomeDir = originalHome }()

	if _, err := Register("b", "read {name}", "write {name}"); err != nil {
		t.Fatalf("register b: %v", err)
	}
	if _, err := Register("a", "read {name}", "write {name}"); err != nil {
		t.Fatalf("register a: %v", err)
	}
	entries, _, err := List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	ids := SortedIDs(entries)
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("unexpected sorted ids: %#v", ids)
	}
	_, removed, err := Remove("a")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !removed {
		t.Fatal("expected sync id to be removed")
	}
	entries, _, err = List()
	if err != nil {
		t.Fatalf("list after remove: %v", err)
	}
	if _, ok := entries["a"]; ok {
		t.Fatal("expected removed sync id to disappear")
	}
}

func TestShellCommandUsesComspecOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only shell resolution")
	}
	original := os.Getenv("COMSPEC")
	t.Setenv("COMSPEC", `C:\Windows\System32\cmd.exe`)
	defer func() {
		_ = os.Setenv("COMSPEC", original)
	}()
	shell, args := ShellCommand("echo hello")
	if shell != `C:\Windows\System32\cmd.exe` {
		t.Fatalf("unexpected windows shell: %q", shell)
	}
	if len(args) != 2 || args[0] != "/C" || args[1] != "echo hello" {
		t.Fatalf("unexpected windows shell args: %#v", args)
	}
}
