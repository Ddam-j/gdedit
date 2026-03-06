package main

import (
	"fmt"
	"os"
	"strings"

	"gdedit/internal/tui"
	"gdedit/internal/version"
)

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		if err := tui.New().Run(); err != nil {
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
		reportDoctor(stdout)
		return 0
	case "--tui":
		if err := tui.New().Run(); err != nil {
			fprintf(stderr, "failed to run gdedit shell: %v\n", err)
			return 1
		}
		return 0
	case "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		printUsage(stderr)
		return 2
	}
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "Usage: gdedit [--version|--doctor|--tui]")
	fmt.Fprintln(out, "Run without arguments to start the minimal shell.")
}

func reportDoctor(out *os.File) {
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
