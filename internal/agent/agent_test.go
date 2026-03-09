package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gdedit/internal/config"
)

func TestParseResponse(t *testing.T) {
	response, err := parseResponse("```json\n{\n  \"mode\": \"replace_selection\",\n  \"message\": \"updated\",\n  \"content\": \"hello\"\n}\n```")
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if response.Mode != ModeReplaceSelection {
		t.Fatalf("unexpected mode: %q", response.Mode)
	}
	if response.Content != "hello" {
		t.Fatalf("unexpected content: %q", response.Content)
	}
}

func TestClientExecute(t *testing.T) {
	t.Setenv("TEST_OPENAI_API_KEY", "secret")
	systemRoot := filepath.Join(t.TempDir(), "system")
	workspace := t.TempDir()
	projectRoot := filepath.Join(workspace, ".gdedit")
	if err := os.MkdirAll(systemRoot, 0o755); err != nil {
		t.Fatalf("mkdir system root: %v", err)
	}
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(systemRoot, "global.md"), []byte("global memo"), 0o600); err != nil {
		t.Fatalf("write system memo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "local.md"), []byte("local memo"), 0o600); err != nil {
		t.Fatalf("write project memo: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"model":"gpt-5.4"`) {
			t.Fatalf("missing model in request: %s", text)
		}
		if !strings.Contains(text, "selection:L1:C1-L1:C4") {
			t.Fatalf("missing scope in request: %s", text)
		}
		if !strings.Contains(text, "global memo") || !strings.Contains(text, "local memo") {
			t.Fatalf("missing memo context in request: %s", text)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"mode\":\"message\",\"message\":\"inspected\"}"}}]}`))
	}))
	defer server.Close()

	executor, err := New(config.AgentConfig{
		Enabled:   true,
		Role:      "edit-agent",
		Provider:  "openai",
		Model:     "gpt-5.4",
		APIKeyEnv: "TEST_OPENAI_API_KEY",
		BaseURL:   server.URL,
	}, systemRoot)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	response, err := executor.Execute(context.Background(), Request{
		Command:   "inspect this",
		Action:    "inspect current scope",
		Kind:      "inspect",
		Scope:     "selection:L1:C1-L1:C4",
		Tab:       "main.go",
		Selection: "main",
		Document:  "package main",
		Workspace: workspace,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if response.Mode != ModeMessage {
		t.Fatalf("unexpected mode: %q", response.Mode)
	}
	if response.Message != "inspected" {
		t.Fatalf("unexpected message: %q", response.Message)
	}
}
