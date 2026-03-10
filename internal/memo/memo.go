package memo

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	maxMemoFiles   = 8
	maxMemoBytes   = 16384
	projectDirName = ".gdedit"
)

var userHomeDir = os.UserHomeDir

type Scope string

const (
	ScopeProject Scope = "project"
	ScopeApp     Scope = "app"
	ScopeSystem  Scope = "system"
)

type Target struct {
	Scope    Scope
	Name     string
	Dir      string
	FileName string
	Path     string
}

func LoadContext(systemRoot, workspace string) (string, error) {
	sections := []string{}

	if text, err := collectRoot(systemRoot, "system memos"); err != nil {
		return "", err
	} else if text != "" {
		sections = append(sections, text)
	}

	if strings.TrimSpace(workspace) != "" {
		projectRoot := filepath.Join(workspace, projectDirName)
		if text, err := collectRoot(projectRoot, "project memos"); err != nil {
			return "", err
		} else if text != "" {
			sections = append(sections, text)
		}
	}

	return strings.Join(sections, "\n\n"), nil
}

func collectRoot(root, label string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", nil
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if !info.IsDir() {
		return "", nil
	}

	paths := []string{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !isMemoFile(path) {
			return nil
		}
		paths = append(paths, path)
		if len(paths) >= maxMemoFiles {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil && err != fs.SkipAll {
		return "", err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return "", nil
	}

	b := strings.Builder{}
	b.WriteString(label)
	b.WriteString(":")
	written := 0
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		text := strings.TrimSpace(string(content))
		if text == "" {
			continue
		}
		remaining := maxMemoBytes - written
		if remaining <= 0 {
			break
		}
		if len(text) > remaining {
			text = text[:remaining]
		}
		b.WriteString("\n- ")
		b.WriteString(filepath.Base(path))
		b.WriteString(":\n")
		b.WriteString(indent(text, "  "))
		written += len(text)
	}
	return b.String(), nil
}

func isMemoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".txt", ".json", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func DebugRoots(systemRoot, workspace string) string {
	return fmt.Sprintf("system=%s workspace=%s", systemRoot, workspace)
}

func SaveFileMemo(systemRoot, workspace, filePath, note string) (string, error) {
	target, err := SaveFileMemoDetailed(systemRoot, workspace, filePath, note)
	if err != nil {
		return "", err
	}
	return target.Path, nil
}

func SaveFileMemoDetailed(systemRoot, workspace, filePath, note string) (Target, error) {
	if strings.TrimSpace(filePath) == "" {
		return Target{}, fmt.Errorf("file path is empty")
	}
	if strings.TrimSpace(note) == "" {
		return Target{}, fmt.Errorf("memo note is empty")
	}

	target, err := resolveMemoTarget(systemRoot, workspace, filePath)
	if err != nil {
		return Target{}, err
	}
	if err := os.MkdirAll(target.Dir, 0o755); err != nil {
		return Target{}, err
	}
	memoPath := filepath.Join(target.Dir, target.FileName)
	entry := buildMemoEntry(filePath, note)

	f, err := os.OpenFile(memoPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return Target{}, err
	}
	defer f.Close()
	if _, err := f.WriteString(entry); err != nil {
		return Target{}, err
	}
	target.Path = memoPath
	return target, nil
}

func resolveMemoTarget(systemRoot, workspace, filePath string) (Target, error) {
	cleanFilePath := filepath.Clean(filePath)
	projectMatch := isProjectFile(workspace, cleanFilePath)
	if isAppConfigFile(cleanFilePath) && (!projectMatch || appScopeOverridesProject(workspace, cleanFilePath)) {
		if strings.TrimSpace(systemRoot) == "" {
			return Target{}, fmt.Errorf("system memo root is empty")
		}
		name := appMemoName(cleanFilePath)
		return Target{Scope: ScopeApp, Name: name, Dir: filepath.Join(systemRoot, "app"), FileName: name + ".md"}, nil
	}
	if projectMatch {
		memoDir := filepath.Join(workspace, projectDirName, "memos")
		relPath := cleanFilePath
		if rel, err := filepath.Rel(workspace, cleanFilePath); err == nil && !strings.HasPrefix(rel, "..") {
			relPath = rel
		}
		return Target{Scope: ScopeProject, Name: sanitizeMemoName(relPath), Dir: memoDir, FileName: sanitizeMemoName(relPath) + ".md"}, nil
	}
	if strings.TrimSpace(systemRoot) == "" {
		return Target{}, fmt.Errorf("system memo root is empty")
	}
	name := systemMemoName(cleanFilePath)
	return Target{Scope: ScopeSystem, Name: name, Dir: filepath.Join(systemRoot, "system"), FileName: name + ".md"}, nil
}

func appScopeOverridesProject(workspace, filePath string) bool {
	if strings.TrimSpace(workspace) == "" {
		return false
	}
	home, err := userHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return false
	}
	workspace = filepath.Clean(workspace)
	home = filepath.Clean(home)
	if samePath(workspace, home) {
		return true
	}
	configRoot := filepath.Join(home, ".config")
	appDataRoot := filepath.Join(home, "AppData")
	return samePath(workspace, configRoot) || samePath(workspace, appDataRoot)
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func isProjectFile(workspace, filePath string) bool {
	if strings.TrimSpace(workspace) == "" {
		return false
	}
	rel, err := filepath.Rel(workspace, filePath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isAppConfigFile(filePath string) bool {
	home, err := userHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return false
	}
	rel, err := filepath.Rel(home, filePath)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	base := filepath.Base(filePath)
	if strings.HasPrefix(base, ".") {
		return true
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts {
		if part == ".config" || part == "AppData" {
			return true
		}
	}
	return false
}

func appMemoName(filePath string) string {
	home, err := userHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		if name := appMemoNameFromHomePath(home, filePath); name != "" {
			return name
		}
	}
	return appMemoNameFromBase(filePath)
}

func appMemoNameFromHomePath(home, filePath string) string {
	rel, err := filepath.Rel(home, filePath)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, part := range parts {
		switch part {
		case ".config":
			if i+1 < len(parts) {
				return sanitizeMemoName(parts[i+1])
			}
		case "AppData":
			if i+2 < len(parts) {
				return sanitizeMemoName(parts[i+2])
			}
		}
	}
	return ""
}

func appMemoNameFromBase(filePath string) string {
	base := strings.TrimSpace(filepath.Base(filePath))
	base = strings.TrimPrefix(base, ".")
	for {
		ext := filepath.Ext(base)
		if ext == "" {
			break
		}
		base = strings.TrimSuffix(base, ext)
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return "app"
	}
	return sanitizeMemoName(base)
}

func systemMemoName(filePath string) string {
	clean := strings.TrimSpace(filepath.Base(filePath))
	for {
		ext := filepath.Ext(clean)
		if ext == "" {
			break
		}
		clean = strings.TrimSuffix(clean, ext)
	}
	clean = strings.TrimPrefix(clean, ".")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "system"
	}
	return sanitizeMemoName(clean)
}

func sanitizeMemoName(path string) string {
	replacer := strings.NewReplacer("\\", "__", "/", "__", ":", "-", " ", "_")
	name := replacer.Replace(strings.TrimSpace(path))
	if name == "" {
		return "memo"
	}
	return name
}

func buildMemoEntry(filePath, note string) string {
	stamp := time.Now().Format(time.RFC3339)
	return fmt.Sprintf("## %s\nfile: %s\ndetails:\n%s\n\n", stamp, filepath.ToSlash(filePath), formatMemoDetails(note))
}

func formatMemoDetails(note string) string {
	segments := memoSegments(note)
	if len(segments) == 0 {
		return "- (empty)"
	}
	formatted := make([]string, 0, len(segments))
	for _, segment := range segments {
		formatted = append(formatted, bulletize(wrapText(segment, 72)))
	}
	return strings.Join(formatted, "\n")
}

func memoSegments(note string) []string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(note, "\r\n", "\n"))
	if trimmed == "" {
		return nil
	}

	lines := strings.Split(trimmed, "\n")
	segments := []string{}
	for _, line := range lines {
		for _, part := range splitExplicitMemoLine(line) {
			part = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(part, "-"), "*"))
			if part == "" {
				continue
			}
			segments = append(segments, splitSentenceLike(part)...)
		}
	}
	return compactSegments(segments)
}

func splitExplicitMemoLine(line string) []string {
	parts := strings.Split(line, "-.")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func splitSentenceLike(text string) []string {
	parts := []string{}
	start := 0
	runes := []rune(strings.TrimSpace(text))
	for i, r := range runes {
		if !isSentenceBoundary(r) {
			continue
		}
		segment := strings.TrimSpace(string(runes[start : i+1]))
		if segment != "" {
			parts = append(parts, segment)
		}
		start = i + 1
	}
	if start < len(runes) {
		segment := strings.TrimSpace(string(runes[start:]))
		if segment != "" {
			parts = append(parts, segment)
		}
	}
	if len(parts) == 0 {
		return []string{strings.TrimSpace(text)}
	}
	return parts
}

func isSentenceBoundary(r rune) bool {
	switch r {
	case '.', '!', '?', ';':
		return true
	default:
		return false
	}
}

func compactSegments(segments []string) []string {
	result := []string{}
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		result = append(result, segment)
	}
	return result
}

func bulletize(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i == 0 {
			lines[i] = "- " + line
			continue
		}
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	lines := []string{}
	current := words[0]
	currentWidth := runeCount(words[0])
	for _, word := range words[1:] {
		wordWidth := runeCount(word)
		if currentWidth+1+wordWidth > width {
			lines = append(lines, current)
			current = word
			currentWidth = wordWidth
			continue
		}
		current += " " + word
		currentWidth += 1 + wordWidth
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func runeCount(text string) int {
	count := 0
	for _, r := range text {
		if unicode.IsControl(r) {
			continue
		}
		count++
	}
	return count
}
