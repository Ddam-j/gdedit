package main

import (
	"fmt"
	"os"
	"strings"

	"gdedit/internal/agent"
	"gdedit/internal/config"
	"gdedit/internal/tui"
	"gdedit/internal/version"
)

var loadConfig = config.Load

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout, stderr *os.File) int {
	loadedConfig, err := loadConfig()
	if err != nil {
		fprintf(stderr, "failed to load gdedit config: %v\n", err)
		return 1
	}

	workspace, err := os.Getwd()
	if err != nil {
		fprintf(stderr, "failed to resolve current workspace: %v\n", err)
		return 1
	}

	editAgent, err := agent.New(loadedConfig.Config.EditAgent, loadedConfig.Config.MemoRoot)
	if err != nil {
		fprintf(stderr, "failed to initialize edit agent: %v\n", err)
		return 1
	}

	if len(args) == 0 {
		if err := tui.NewWithAgent(loadedConfig.Config.EditAgent.Summary(), editAgent, workspace, loadedConfig.Config.MemoRoot).Run(); err != nil {
			fprintf(stderr, "failed to run gdedit shell: %v\n", err)
			return 1
		}
		return 0
	}

	if len(args) > 1 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "--version":
		fmt.Fprintln(stdout, version.String())
		return 0
	case "--doctor":
		reportDoctor(stdout, loadedConfig)
		return 0
	case "--tui":
		if err := tui.NewWithAgent(loadedConfig.Config.EditAgent.Summary(), editAgent, workspace, loadedConfig.Config.MemoRoot).Run(); err != nil {
			fprintf(stderr, "failed to run gdedit shell: %v\n", err)
			return 1
		}
		return 0
	case "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		app, err := tui.NewWithFiles(loadedConfig.Config.EditAgent.Summary(), editAgent, workspace, loadedConfig.Config.MemoRoot, []string{args[0]})
		if err != nil {
			fprintf(stderr, "failed to open file: %v\n", err)
			return 1
		}
		if err := app.Run(); err != nil {
			fprintf(stderr, "failed to run gdedit shell: %v\n", err)
			return 1
		}
		return 0
	}
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "Usage: gdedit [--version|--doctor|--tui|<file>]")
	fmt.Fprintln(out, "Run without arguments to start the minimal shell, or pass a file path to open it directly.")
}

func reportDoctor(out *os.File, loaded config.Loaded) {
	stdinTTY := isTTY(os.Stdin)
	stdoutTTY := isTTY(os.Stdout)
	term := os.Getenv("TERM")
	colorTerm := os.Getenv("COLORTERM")
	noColor := os.Getenv("NO_COLOR")
	sshConnection := os.Getenv("SSH_CONNECTION")

	fmt.Fprintln(out, "gdedit doctor")
	fmt.Fprintf(out, "version: %s\n", version.String())
	fmt.Fprintf(out, "stdin_tty: %t\n", stdinTTY)
	fmt.Fprintf(out, "stdout_tty: %t\n", stdoutTTY)
	fmt.Fprintf(out, "term: %s\n", envOrUnset(term))
	fmt.Fprintf(out, "colorterm: %s\n", envOrUnset(colorTerm))
	fmt.Fprintf(out, "no_color: %s\n", envOrUnset(noColor))
	fmt.Fprintf(out, "ssh_connection: %s\n", envOrUnset(sshConnection))
	fmt.Fprintf(out, "config_path: %s\n", loaded.Path)
	fmt.Fprintf(out, "config_exists: %t\n", loaded.Exists)
	fmt.Fprintf(out, "memo_root: %s\n", loaded.Config.MemoRoot)
	fmt.Fprintf(out, "edit_agent_role: %s\n", envOrUnset(loaded.Config.EditAgent.Role))
	fmt.Fprintf(out, "edit_agent_provider: %s\n", envOrUnset(loaded.Config.EditAgent.Provider))
	fmt.Fprintf(out, "edit_agent_model: %s\n", envOrUnset(loaded.Config.EditAgent.Model))
	fmt.Fprintf(out, "edit_agent_api_key_env: %s\n", envOrUnset(loaded.Config.EditAgent.APIKeyEnv))
	fmt.Fprintf(out, "edit_agent_api_key_present: %t\n", loaded.Config.EditAgent.HasAPIKey())
	fmt.Fprintf(out, "edit_agent_status: %s\n", loaded.Config.EditAgent.Status())
	fmt.Fprintf(out, "interpretation: %s\n", interpretEnvironment(stdoutTTY, term, sshConnection, noColor, colorTerm))
}

func isTTY(file *os.File) bool {
	if file == nil {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func envOrUnset(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(unset)"
	}

	return value
}

func interpretEnvironment(stdoutTTY bool, term, sshConnection, noColor, colorTerm string) string {
	if !stdoutTTY {
		return "stdout is not a TTY; this looks non-interactive and terminal capability checks are limited"
	}

	normalizedTERM := strings.ToLower(strings.TrimSpace(term))
	if normalizedTERM == "" || normalizedTERM == "dumb" {
		return "TERM is empty or dumb; this does not meet the terminal contract and gdedit may refuse to start"
	}

	if strings.TrimSpace(sshConnection) != "" {
		return "SSH session detected; xterm-compatible terminals are supported, best-effort features depend on remote and tmux setup"
	}

	if strings.TrimSpace(noColor) != "" {
		return "local interactive terminal detected; NO_COLOR requests reduced color output"
	}

	if strings.TrimSpace(colorTerm) != "" {
		return "local interactive terminal detected; COLORTERM is set, so enhanced color is likely available"
	}

	return "local interactive terminal detected; required capabilities depend on your terminal emulator"
}

func fprintf(file *os.File, format string, args ...any) {
	_, _ = fmt.Fprintf(file, format, args...)
}
