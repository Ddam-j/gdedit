package main

import (
	"os"
	"strings"
	"testing"

	"gdedit/internal/config"
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
		wantErr  bool
	}{
		{args: nil, wantMode: launchScratch},
		{args: []string{"--tui"}, wantMode: launchScratch},
		{args: []string{"--test"}, wantMode: launchTest},
		{args: []string{"--help"}, wantMode: launchHelp},
		{args: []string{"--version"}, wantMode: launchVersion},
		{args: []string{"--doctor"}, wantMode: launchDoctor},
		{args: []string{"note.txt"}, wantMode: launchFile, wantPath: "note.txt"},
		{args: []string{"one", "two"}, wantErr: true},
	}

	for _, tt := range tests {
		mode, path, err := resolveLaunch(tt.args)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("expected error for args %v", tt.args)
			}
			continue
		}
		if err != nil {
			t.Fatalf("unexpected error for args %v: %v", tt.args, err)
		}
		if mode != tt.wantMode {
			t.Fatalf("unexpected mode for args %v: got %v want %v", tt.args, mode, tt.wantMode)
		}
		if path != tt.wantPath {
			t.Fatalf("unexpected path for args %v: got %q want %q", tt.args, path, tt.wantPath)
		}
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
	mode, path, err := resolveLaunch([]string{"missing-file.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != launchFile {
		t.Fatalf("unexpected mode: got %v want %v", mode, launchFile)
	}
	if path != "missing-file.txt" {
		t.Fatalf("unexpected path: %q", path)
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
