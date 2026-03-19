package processsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var userHomeDir = os.UserHomeDir

func SetUserHomeDirForTest(fn func() (string, error)) {
	userHomeDir = fn
}

func UserHomeDirForTest() func() (string, error) {
	return userHomeDir
}

type Entry struct {
	ReadFormat  string `json:"read"`
	WriteFormat string `json:"write"`
}

type Registry struct {
	Sync map[string]Entry `json:"sync"`
}

func DefaultPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("home directory is empty")
	}
	return filepath.Join(home, ".config", "gdedit", "process_sync.json"), nil
}

func Load() (Registry, string, error) {
	path, err := DefaultPath()
	if err != nil {
		return Registry{}, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Registry{Sync: map[string]Entry{}}, path, nil
		}
		return Registry{}, "", err
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return Registry{}, "", err
	}
	if reg.Sync == nil {
		reg.Sync = map[string]Entry{}
	}
	return reg, path, nil
}

func Save(reg Registry, path string) error {
	if reg.Sync == nil {
		reg.Sync = map[string]Entry{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func Register(id, readFormat, writeFormat string) (string, error) {
	id = strings.TrimSpace(id)
	readFormat = strings.TrimSpace(readFormat)
	writeFormat = strings.TrimSpace(writeFormat)
	if id == "" {
		return "", errors.New("sync id is empty")
	}
	if readFormat == "" || writeFormat == "" {
		return "", errors.New("read and write formats are required")
	}
	reg, path, err := Load()
	if err != nil {
		return "", err
	}
	if existing, ok := reg.Sync[id]; ok {
		if existing.ReadFormat != readFormat || existing.WriteFormat != writeFormat {
			return "", fmt.Errorf("sync id %q already exists with different formats", id)
		}
		return path, nil
	}
	reg.Sync[id] = Entry{ReadFormat: readFormat, WriteFormat: writeFormat}
	if err := Save(reg, path); err != nil {
		return "", err
	}
	return path, nil
}

func Resolve(id string) (Entry, error) {
	reg, _, err := Load()
	if err != nil {
		return Entry{}, err
	}
	entry, ok := reg.Sync[strings.TrimSpace(id)]
	if !ok {
		return Entry{}, fmt.Errorf("sync id %q is not registered", id)
	}
	return entry, nil
}

func List() (map[string]Entry, string, error) {
	reg, path, err := Load()
	if err != nil {
		return nil, "", err
	}
	return reg.Sync, path, nil
}

func SortedIDs(entries map[string]Entry) []string {
	ids := make([]string, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func Remove(id string) (string, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", false, errors.New("sync id is empty")
	}
	reg, path, err := Load()
	if err != nil {
		return "", false, err
	}
	if _, ok := reg.Sync[id]; !ok {
		return path, false, nil
	}
	delete(reg.Sync, id)
	if err := Save(reg, path); err != nil {
		return "", false, err
	}
	return path, true, nil
}

func Expand(format, name string) string {
	return strings.ReplaceAll(format, "{name}", name)
}

func ShellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		shell := strings.TrimSpace(os.Getenv("COMSPEC"))
		if shell == "" {
			shell = "cmd.exe"
		}
		return shell, []string{"/C", command}
	}
	return "sh", []string{"-lc", command}
}
