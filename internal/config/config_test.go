package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultForHome(t *testing.T) {
	cfg := DefaultForHome(`/home/tester`)

	if cfg.MemoRoot != "/home/tester/gdedit/" {
		t.Fatalf("unexpected memo root: %q", cfg.MemoRoot)
	}
	if cfg.EditAgent.Role != "edit-agent" {
		t.Fatalf("unexpected role: %q", cfg.EditAgent.Role)
	}
	if cfg.EditAgent.Provider != "openai" {
		t.Fatalf("unexpected provider: %q", cfg.EditAgent.Provider)
	}
	if cfg.EditAgent.Model != "gpt-5.4" {
		t.Fatalf("unexpected model: %q", cfg.EditAgent.Model)
	}
}

func TestLoadFromPathMergesAndExpandsHome(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte(`{
  "memoRoot": "~/portable-memos/",
  "editAgent": {
    "provider": "openai",
    "model": "gpt-5.4-mini",
    "apiKeyEnv": "TEST_OPENAI_API_KEY"
  }
}`), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	loaded, err := loadFromPath(path, `/home/tester`)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !loaded.Exists {
		t.Fatal("expected config to exist")
	}
	if loaded.Config.MemoRoot != "/home/tester/portable-memos/" {
		t.Fatalf("unexpected memo root: %q", loaded.Config.MemoRoot)
	}
	if loaded.Config.EditAgent.Role != "edit-agent" {
		t.Fatalf("expected default role, got %q", loaded.Config.EditAgent.Role)
	}
	if loaded.Config.EditAgent.Model != "gpt-5.4-mini" {
		t.Fatalf("unexpected model: %q", loaded.Config.EditAgent.Model)
	}
	if loaded.Config.EditAgent.APIKeyEnv != "TEST_OPENAI_API_KEY" {
		t.Fatalf("unexpected api key env: %q", loaded.Config.EditAgent.APIKeyEnv)
	}
}

func TestAgentStatus(t *testing.T) {
	t.Setenv("TEST_EDIT_AGENT_KEY", "secret")

	agent := AgentConfig{
		Enabled:   true,
		Role:      "edit-agent",
		Provider:  "openai",
		Model:     "gpt-5.4",
		APIKeyEnv: "TEST_EDIT_AGENT_KEY",
	}

	if got := agent.Status(); got != "ready" {
		t.Fatalf("unexpected status: %q", got)
	}
	if got := agent.Summary(); got != "edit-agent openai/gpt-5.4" {
		t.Fatalf("unexpected summary: %q", got)
	}
}

func TestExpandUserPathForHome(t *testing.T) {
	if got := ExpandUserPathForHome("~/.wezterm.lua", `C:\Users\tester`); got != "C:/Users/tester/.wezterm.lua" {
		t.Fatalf("unexpected expanded slash path: %q", got)
	}
	if got := ExpandUserPathForHome(`~\.wezterm.lua`, `C:\Users\tester`); got != "C:/Users/tester/.wezterm.lua" {
		t.Fatalf("unexpected expanded backslash path: %q", got)
	}
	if got := ExpandUserPathForHome(`D:\work\file.txt`, `C:\Users\tester`); got != "D:/work/file.txt" {
		t.Fatalf("unexpected passthrough path: %q", got)
	}
}
