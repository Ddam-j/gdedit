package main

import (
	"os"
	"strings"
	"testing"

	"gdedit/internal/config"
	"gdedit/internal/processsync"
)

func TestRunDoctorReportsEditAgentConfig(t *testing.T) {
	originalLoader := loadConfig
	t.Cleanup(func() {
		loadConfig = originalLoader
	})

	loadConfig = func() (config.Loaded, error) {
		return config.Loaded{
			Path:   "/home/tester/.config/gdedit/config.json",
			Exists: true,
			Config: config.Config{
				MemoRoot: "/home/tester/gdedit/",
				EditAgent: config.AgentConfig{
					Enabled:   true,
					Role:      "edit-agent",
					Provider:  "openai",
					Model:     "gpt-5.4",
					APIKeyEnv: "TEST_GDEDIT_KEY",
				},
			},
		}, nil
	}

	t.Setenv("TEST_GDEDIT_KEY", "present")

	stdout, stderr, err := tempFiles(t)
	if err != nil {
		t.Fatalf("temp files: %v", err)
	}
	t.Cleanup(func() {
		_ = stdout.Close()
		_ = stderr.Close()
	})

	code := run([]string{"--doctor"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}

	out := readTempFile(t, stdout)
	if !strings.Contains(out, "edit_agent_role: edit-agent") {
		t.Fatalf("missing role in output: %s", out)
	}
	if !strings.Contains(out, "edit_agent_provider: openai") {
		t.Fatalf("missing provider in output: %s", out)
	}
	if !strings.Contains(out, "edit_agent_model: gpt-5.4") {
		t.Fatalf("missing model in output: %s", out)
	}
	if !strings.Contains(out, "edit_agent_api_key_present: true") {
		t.Fatalf("missing api key presence in output: %s", out)
	}
	if !strings.Contains(out, "edit_agent_status: ready") {
		t.Fatalf("missing status in output: %s", out)
	}
}

func TestResolveLaunch(t *testing.T) {
	tests := []struct {
		args     []string
		wantMode launchMode
		wantPath string
		wantID   string
		wantName string
		wantErr  bool
	}{
		{args: nil, wantMode: launchScratch},
		{args: []string{"--tui"}, wantMode: launchScratch},
		{args: []string{"--test"}, wantMode: launchTest},
		{args: []string{"--help"}, wantMode: launchHelp},
		{args: []string{"--version"}, wantMode: launchVersion},
		{args: []string{"--doctor"}, wantMode: launchDoctor},
		{args: []string{"--sync", "mynamr", "demo-rule"}, wantMode: launchSync, wantID: "mynamr", wantName: "demo-rule"},
		{args: []string{"--sync-register", "mynamr", "--read", "show {name}", "--write", "update {name}"}, wantMode: launchSyncRegister, wantID: "mynamr"},
		{args: []string{"--sync-list"}, wantMode: launchSyncList},
		{args: []string{"--sync-remove", "mynamr"}, wantMode: launchSyncRemove, wantID: "mynamr"},
		{args: []string{"note.txt"}, wantMode: launchFile, wantPath: "note.txt"},
		{args: []string{"one", "two"}, wantErr: true},
	}

	for _, tt := range tests {
		req, err := resolveLaunch(tt.args)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for args %v", tt.args)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for args %v: %v", tt.args, err)
		}
		if req.mode != tt.wantMode {
			t.Fatalf("unexpected mode for args %v: got %v want %v", tt.args, req.mode, tt.wantMode)
		}
		if req.filePath != tt.wantPath {
			t.Fatalf("unexpected path for args %v: got %q want %q", tt.args, req.filePath, tt.wantPath)
		}
		if req.syncID != tt.wantID {
			t.Fatalf("unexpected sync id for args %v: got %q want %q", tt.args, req.syncID, tt.wantID)
		}
		if req.syncName != tt.wantName {
			t.Fatalf("unexpected sync name for args %v: got %q want %q", tt.args, req.syncName, tt.wantName)
		}
	}
}

func TestRunSyncListAndRemove(t *testing.T) {
	originalHome := processsync.UserHomeDirForTest()
	home := t.TempDir()
	processsync.SetUserHomeDirForTest(func() (string, error) { return home, nil })
	defer processsync.SetUserHomeDirForTest(originalHome)

	stdout, stderr, err := tempFiles(t)
	if err != nil {
		t.Fatalf("temp files: %v", err)
	}
	t.Cleanup(func() {
		_ = stdout.Close()
		_ = stderr.Close()
	})

	code := run([]string{"--sync-register", "demo", "--read", "show {name}", "--write", "write {name}"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("unexpected register exit code: %d", code)
	}
	code = run([]string{"--sync-list"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("unexpected list exit code: %d", code)
	}
	out := readTempFile(t, stdout)
	if !strings.Contains(out, "demo") || !strings.Contains(out, "show {name}") {
		t.Fatalf("unexpected sync list output: %s", out)
	}
	stdout2, stderr2, err := tempFiles(t)
	if err != nil {
		t.Fatalf("temp files remove: %v", err)
	}
	t.Cleanup(func() {
		_ = stdout2.Close()
		_ = stderr2.Close()
	})
	code = run([]string{"--sync-remove", "demo"}, stdout2, stderr2)
	if code != 0 {
		t.Fatalf("unexpected remove exit code: %d", code)
	}
	out = readTempFile(t, stdout2)
	if !strings.Contains(out, "removed sync \"demo\"") {
		t.Fatalf("unexpected sync remove output: %s", out)
	}
}

func TestRunHelpDoesNotRequireConfig(t *testing.T) {
	originalLoader := loadConfig
	t.Cleanup(func() {
		loadConfig = originalLoader
	})

	loadConfig = func() (config.Loaded, error) {
		t.Fatal("did not expect config load for --help")
		return config.Loaded{}, nil
	}

	stdout, stderr, err := tempFiles(t)
	if err != nil {
		t.Fatalf("temp files: %v", err)
	}
	t.Cleanup(func() {
		_ = stdout.Close()
		_ = stderr.Close()
	})

	code := run([]string{"--help"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	out := readTempFile(t, stdout)
	if !strings.Contains(out, "Usage: gdedit") {
		t.Fatalf("unexpected help output: %s", out)
	}
}

func TestRunVersionDoesNotRequireConfig(t *testing.T) {
	originalLoader := loadConfig
	t.Cleanup(func() {
		loadConfig = originalLoader
	})

	loadConfig = func() (config.Loaded, error) {
		t.Fatal("did not expect config load for --version")
		return config.Loaded{}, nil
	}

	stdout, stderr, err := tempFiles(t)
	if err != nil {
		t.Fatalf("temp files: %v", err)
	}
	t.Cleanup(func() {
		_ = stdout.Close()
		_ = stderr.Close()
	})

	code := run([]string{"--version"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("unexpected exit code: %d", code)
	}
	out := strings.TrimSpace(readTempFile(t, stdout))
	if out == "" {
		t.Fatal("expected non-empty version output")
	}
}

func TestResolveLaunchTreatsMissingPathAsFileTarget(t *testing.T) {
	req, err := resolveLaunch([]string{"missing-file.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.mode != launchFile {
		t.Fatalf("unexpected mode: got %v want %v", req.mode, launchFile)
	}
	if req.filePath != "missing-file.txt" {
		t.Fatalf("unexpected path: %q", req.filePath)
	}
}

func tempFiles(t *testing.T) (*os.File, *os.File, error) {
	t.Helper()
	stdout, err := os.CreateTemp(t.TempDir(), "stdout-*.txt")
	if err != nil {
		return nil, nil, err
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr-*.txt")
	if err != nil {
		return nil, nil, err
	}
	return stdout, stderr, nil
}

func readTempFile(t *testing.T, file *os.File) string {
	t.Helper()
	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatalf("read temp file by name: %v", err)
	}
	return string(content)
}
