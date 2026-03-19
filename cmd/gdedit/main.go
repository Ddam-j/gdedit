package main

import (
	"fmt"
	"os"
	"strings"

	"gdedit/internal/agent"
	"gdedit/internal/config"
	"gdedit/internal/processsync"
	"gdedit/internal/tui"
	"gdedit/internal/version"
)

var loadConfig = config.Load

type launchMode int

const (
	launchScratch launchMode = iota
	launchVersion
	launchDoctor
	launchHelp
	launchFile
	launchSync
	launchSyncRegister
	launchSyncList
	launchSyncRemove
	launchTest
)

type launchRequest struct {
	mode        launchMode
	filePath    string
	syncID      string
	syncName    string
	readFormat  string
	writeFormat string
}

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout, stderr *os.File) int {
	req, err := resolveLaunch(args)
	if err != nil {
		printUsage(stderr)
		return 2
	}

	switch req.mode {
	case launchVersion:
		fmt.Fprintln(stdout, version.String())
		return 0
	case launchHelp:
		printUsage(stdout)
		return 0
	}
	if req.mode == launchSyncRegister {
		path, err := processsync.Register(req.syncID, req.readFormat, req.writeFormat)
		if err != nil {
			fprintf(stderr, "failed to register sync: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "registered sync %q in %s\n", req.syncID, path)
		return 0
	}
	if req.mode == launchSyncList {
		entries, path, err := processsync.List()
		if err != nil {
			fprintf(stderr, "failed to list sync entries: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "process sync registry: %s\n", path)
		for _, id := range processsync.SortedIDs(entries) {
			entry := entries[id]
			fmt.Fprintf(stdout, "- %s\n  read: %s\n  write: %s\n", id, entry.ReadFormat, entry.WriteFormat)
		}
		if len(entries) == 0 {
			fmt.Fprintln(stdout, "(no registered sync entries)")
		}
		return 0
	}
	if req.mode == launchSyncRemove {
		path, removed, err := processsync.Remove(req.syncID)
		if err != nil {
			fprintf(stderr, "failed to remove sync: %v\n", err)
			return 1
		}
		if !removed {
			fprintf(stderr, "sync id %q was not registered in %s\n", req.syncID, path)
			return 1
		}
		fmt.Fprintf(stdout, "removed sync %q from %s\n", req.syncID, path)
		return 0
	}

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

	switch req.mode {
	case launchDoctor:
		reportDoctor(stdout, loadedConfig)
		return 0
	case launchScratch:
		if err := tui.NewScratchWithAgent(loadedConfig.Config.EditAgent.Summary(), editAgent, workspace, loadedConfig.Config.MemoRoot).Run(); err != nil {
			fprintf(stderr, "failed to run gdedit shell: %v\n", err)
			return 1
		}
		return 0
	case launchTest:
		if err := tui.NewWithAgent(loadedConfig.Config.EditAgent.Summary(), editAgent, workspace, loadedConfig.Config.MemoRoot).Run(); err != nil {
			fprintf(stderr, "failed to run gdedit shell: %v\n", err)
			return 1
		}
		return 0
	case launchSync:
		app, err := tui.NewWithSync(loadedConfig.Config.EditAgent.Summary(), editAgent, workspace, loadedConfig.Config.MemoRoot, req.syncID, req.syncName)
		if err != nil {
			fprintf(stderr, "failed to open sync target: %v\n", err)
			return 1
		}
		if err := app.Run(); err != nil {
			fprintf(stderr, "failed to run gdedit shell: %v\n", err)
			return 1
		}
		return 0
	case launchFile:
		app, err := tui.NewWithFiles(loadedConfig.Config.EditAgent.Summary(), editAgent, workspace, loadedConfig.Config.MemoRoot, []string{req.filePath})
		if err != nil {
			fprintf(stderr, "failed to open file: %v\n", err)
			return 1
		}
		if err := app.Run(); err != nil {
			fprintf(stderr, "failed to run gdedit shell: %v\n", err)
			return 1
		}
		return 0
	default:
		printUsage(stderr)
		return 2
	}
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "gdedit")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Editing-first terminal editor with a Control Hub for AI interaction,")
	fmt.Fprintln(out, "slash commands, memo capture, and subprocess-backed sync buffers.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage: gdedit")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Commands")
	fmt.Fprintln(out, "  gdedit")
	fmt.Fprintln(out, "      Start a scratch buffer.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  gdedit <file>")
	fmt.Fprintln(out, "      Open a file. If it does not exist, gdedit opens an empty file-backed tab")
	fmt.Fprintln(out, "      and creates the file when you save with Ctrl+S.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  gdedit --tui")
	fmt.Fprintln(out, "      Start a scratch buffer explicitly.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  gdedit --doctor")
	fmt.Fprintln(out, "      Check terminal environment, config path, memo root, and edit-agent readiness.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  gdedit --version")
	fmt.Fprintln(out, "      Print the current version.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Process Sync")
	fmt.Fprintln(out, "  gdedit --sync-register <id> --read <format> --write <format>")
	fmt.Fprintln(out, "      Register a subprocess-backed sync target in ~/.config/gdedit/process_sync.json.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  gdedit --sync-list")
	fmt.Fprintln(out, "      Show registered sync entries.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  gdedit --sync <id> <name>")
	fmt.Fprintln(out, "      Open a sync-backed buffer using the registered read command.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "  gdedit --sync-remove <id>")
	fmt.Fprintln(out, "      Remove a registered sync entry.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Examples")
	fmt.Fprintln(out, "  gdedit README.md")
	fmt.Fprintln(out, "  gdedit --doctor")
	fmt.Fprintln(out, "  gdedit --sync-register mynamr --read \"mynamr rule show {name} --spec-only\" --write \"mynamr rule update {name} --spec-stdin\"")
	fmt.Fprintln(out, "  gdedit --sync mynamr demo-rule")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Inside the editor")
	fmt.Fprintln(out, "  Ctrl+G  focus the Control Hub")
	fmt.Fprintln(out, "  F1      open in-app help")
	fmt.Fprintln(out, "  F3      open per-file conversation history")
	fmt.Fprintln(out, "  Ctrl+S  save the current file-backed or sync-backed tab")
}

func resolveLaunch(args []string) (launchRequest, error) {
	if len(args) == 0 {
		return launchRequest{mode: launchScratch}, nil
	}
	if args[0] == "--sync" {
		if len(args) != 3 {
			return launchRequest{}, fmt.Errorf("--sync requires <id> and <name>")
		}
		return launchRequest{mode: launchSync, syncID: args[1], syncName: args[2]}, nil
	}
	if args[0] == "--sync-register" {
		if len(args) != 6 {
			return launchRequest{}, fmt.Errorf("--sync-register requires <id> --read <format> --write <format>")
		}
		if args[2] != "--read" || args[4] != "--write" {
			return launchRequest{}, fmt.Errorf("--sync-register requires <id> --read <format> --write <format>")
		}
		return launchRequest{mode: launchSyncRegister, syncID: args[1], readFormat: args[3], writeFormat: args[5]}, nil
	}
	if args[0] == "--sync-list" {
		if len(args) != 1 {
			return launchRequest{}, fmt.Errorf("--sync-list takes no arguments")
		}
		return launchRequest{mode: launchSyncList}, nil
	}
	if args[0] == "--sync-remove" {
		if len(args) != 2 {
			return launchRequest{}, fmt.Errorf("--sync-remove requires <id>")
		}
		return launchRequest{mode: launchSyncRemove, syncID: args[1]}, nil
	}
	if len(args) > 1 {
		return launchRequest{}, fmt.Errorf("too many arguments")
	}
	switch args[0] {
	case "--version":
		return launchRequest{mode: launchVersion}, nil
	case "--doctor":
		return launchRequest{mode: launchDoctor}, nil
	case "--tui":
		return launchRequest{mode: launchScratch}, nil
	case "--test":
		return launchRequest{mode: launchTest}, nil
	case "-h", "--help":
		return launchRequest{mode: launchHelp}, nil
	default:
		return launchRequest{mode: launchFile, filePath: args[0]}, nil
	}
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
