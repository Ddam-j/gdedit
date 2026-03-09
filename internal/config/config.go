package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var userHomeDir = os.UserHomeDir

type Config struct {
	MemoRoot  string      `json:"memoRoot"`
	EditAgent AgentConfig `json:"editAgent"`
}

type AgentConfig struct {
	Enabled   bool   `json:"enabled"`
	Role      string `json:"role"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"apiKeyEnv,omitempty"`
	BaseURL   string `json:"baseURL,omitempty"`
}

type Loaded struct {
	Config Config
	Path   string
	Exists bool
}

func DefaultPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("home directory is empty")
	}
	return filepath.Join(home, ".config", "gdedit", "config.json"), nil
}

func DefaultForHome(home string) Config {
	memoRoot := ensureTrailingSlash(filepath.ToSlash(filepath.Join(home, "gdedit")))
	return Config{
		MemoRoot: memoRoot,
		EditAgent: AgentConfig{
			Enabled:   true,
			Role:      "edit-agent",
			Provider:  "openai",
			Model:     "gpt-5.4",
			APIKeyEnv: "OPENAI_API_KEY",
		},
	}
}

func Load() (Loaded, error) {
	home, err := userHomeDir()
	if err != nil {
		return Loaded{}, err
	}
	path, err := DefaultPath()
	if err != nil {
		return Loaded{}, err
	}
	return loadFromPath(path, home)
}

func loadFromPath(path, home string) (Loaded, error) {
	loaded := Loaded{
		Config: DefaultForHome(home),
		Path:   path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return loaded, nil
		}
		return Loaded{}, err
	}

	loaded.Exists = true

	var incoming Config
	if err := json.Unmarshal(data, &incoming); err != nil {
		return Loaded{}, err
	}

	loaded.Config = merge(DefaultForHome(home), incoming, home)
	return loaded, nil
}

func merge(base, incoming Config, home string) Config {
	merged := base
	if strings.TrimSpace(incoming.MemoRoot) != "" {
		merged.MemoRoot = normalizeMemoRoot(incoming.MemoRoot, home)
	}

	if incoming.EditAgent.Enabled != base.EditAgent.Enabled {
		merged.EditAgent.Enabled = incoming.EditAgent.Enabled
	}
	if strings.TrimSpace(incoming.EditAgent.Role) != "" {
		merged.EditAgent.Role = strings.TrimSpace(incoming.EditAgent.Role)
	}
	if strings.TrimSpace(incoming.EditAgent.Provider) != "" {
		merged.EditAgent.Provider = strings.TrimSpace(incoming.EditAgent.Provider)
	}
	if strings.TrimSpace(incoming.EditAgent.Model) != "" {
		merged.EditAgent.Model = strings.TrimSpace(incoming.EditAgent.Model)
	}
	if strings.TrimSpace(incoming.EditAgent.APIKeyEnv) != "" {
		merged.EditAgent.APIKeyEnv = strings.TrimSpace(incoming.EditAgent.APIKeyEnv)
	}
	if strings.TrimSpace(incoming.EditAgent.BaseURL) != "" {
		merged.EditAgent.BaseURL = strings.TrimSpace(incoming.EditAgent.BaseURL)
	}

	return merged
}

func normalizeMemoRoot(value, home string) string {
	return ensureTrailingSlash(ExpandUserPathForHome(value, home))
}

func ExpandUserPath(value string) (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	return ExpandUserPathForHome(value, home), nil
}

func ExpandUserPathForHome(value, home string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	replaced := strings.ReplaceAll(trimmed, `\`, `/`)
	if replaced == "~" {
		return filepath.ToSlash(home)
	}
	if strings.HasPrefix(replaced, "~/") {
		return filepath.ToSlash(filepath.Join(home, strings.TrimPrefix(replaced, "~/")))
	}
	return filepath.ToSlash(trimmed)
}

func ensureTrailingSlash(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	if strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}

func (a AgentConfig) Summary() string {
	parts := []string{}
	if role := strings.TrimSpace(a.Role); role != "" {
		parts = append(parts, role)
	}
	providerModel := strings.Trim(strings.TrimSpace(a.Provider)+"/"+strings.TrimSpace(a.Model), "/")
	if providerModel != "" {
		parts = append(parts, providerModel)
	}
	if len(parts) == 0 {
		return "unconfigured"
	}
	return strings.Join(parts, " ")
}

func (a AgentConfig) HasAPIKey() bool {
	if strings.TrimSpace(a.APIKeyEnv) == "" {
		return false
	}
	_, ok := os.LookupEnv(a.APIKeyEnv)
	return ok
}

func (a AgentConfig) Status() string {
	if !a.Enabled {
		return "disabled"
	}
	if strings.TrimSpace(a.Role) == "" || strings.TrimSpace(a.Provider) == "" || strings.TrimSpace(a.Model) == "" {
		return "incomplete"
	}
	if strings.TrimSpace(a.APIKeyEnv) != "" && !a.HasAPIKey() {
		return "missing_api_key"
	}
	return "ready"
}
