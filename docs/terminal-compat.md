# Terminal Compatibility Contract

This document defines the terminal requirements for gdedit. It clarifies what capabilities are required for the editor to function, which features are best-effort, and what terminals are explicitly out of scope.

## Required Capabilities

These capabilities are essential. Without them, gdedit will not start or will refuse to operate.

| Capability | Description | Why It Matters |
|------------|-------------|----------------|
| ANSI/VT escape sequences | Support for standard VT100/ANSI control sequences | Core rendering depends on this |
| Alternate screen buffer | Ability to switch to alternate screen and return | Prevents clobbering the user's terminal history |
| Cursor addressing | CUP (cursor position) escape sequences | Required for precise screen updates |
| UTF-8 encoding | Full Unicode text support | gdedit is a modern editor targeting modern environments |
| Minimum 80x24 geometry | Terminal must report at least 80 columns and 24 rows | UI layout assumes this baseline |

A terminal that lacks any of these capabilities is not supported. gdedit will display an error message and exit rather than produce broken output.

## Best-Effort Capabilities

These features enhance the experience but are not required. gdedit detects them at startup and falls back gracefully when unavailable.

| Capability | Description | Fallback Behavior |
|------------|-------------|-------------------|
| Truecolor (24-bit color) | RGB color specification via OSC 10/110/130 etc. | Falls back to standard 256-color palette |
| Mouse tracking | xterm mouse protocol support | Mouse clicks are ignored, keyboard works as primary |
| Bracketed paste mode | ESC [200~ ... ESC [201~ delimiters | Pasted text works but may contain unexpected whitespace |
| Primary selection | Access to system clipboard via escape sequences | Copy/paste operations use internal buffer only |
| Title setting | OSC 0/1/2 sequences for window title | Window title remains unchanged |
| Unicode wide character support | Proper width calculation for CJK characters | Characters may render with incorrect width |

The editor is designed to work without these. If your terminal does not support a particular feature, the core editing experience remains intact.

## Key Delivery Notes

Terminal key delivery is not uniform. `gdedit` treats some shortcuts as best-effort aliases only when the terminal forwards them reliably.

| Key | Reliability | Current Role |
|-----|-------------|--------------|
| `F1` | High | Help dialog |
| `F2` | High | Hierarchical block selection |
| `Ctrl+Q` | High | Quit confirmation |
| `Ctrl+C` / `Ctrl+X` / `Ctrl+V` | Medium | Internal copy, cut, and paste |
| `Alt+.` / `Alt+,` | Medium to high | Primary tab navigation |
| `Shift`, `Alt`, `Shift+Alt` with arrows | Medium | Selection expansion modifiers; any one may be missing depending on terminal |
| `Ctrl+Tab` / `Ctrl+Shift+Tab` | Low | Best-effort tab navigation alias |
| `Ctrl+[` | Low | Best-effort block-selection alias; often delivered as `Esc` |
| `Ctrl+Space` | Low | Best-effort block-selection alias; often dropped |

When terminal behavior is inconsistent, gdedit prefers keys that arrive predictably rather than desktop-editor conventions that are frequently swallowed by tmux, ssh, or terminal emulators.

## Visible Tab Policy

`gdedit` does not currently treat literal tab characters as layout-width cells. Instead:

- literal `\t` is stored in the buffer
- literal `\t` is rendered as a visible `»` marker with distinct styling
- selection indentation is handled separately through configurable indentation units

This keeps storage and display separate, avoids cursor-width ambiguity, and makes tab characters auditable inside a terminal-first editor.

## Selection Contract

`gdedit` now treats selection as a keyboard-driven character range, not a line-only highlight.

- `Shift`, `Alt`, and `Shift+Alt` are all accepted as selection modifiers for movement keys.
- The visible selection is the true text range between the anchor caret and the active caret.
- Structural edits still work on covered lines by projecting the text selection to its start and end lines.
- Terminal mouse drag remains an external terminal selection, not an internal gdedit edit selection.

## Supported Terminals

The following terminals are tested and known to work well with gdedit. Other modern terminals with the required capabilities may work but are not explicitly tested.

### Windows

- WezTerm (recommended)
- Windows Terminal
- VSCode integrated terminal

### macOS

- iTerm2
- Terminal.app

### Linux / BSD

- xterm and xterm-compatible terminals (urxvt, st, etc.)
- GNOME Terminal
- Konsole
- kitty

### Remote Environments

- xterm-compatible terminals over SSH
- tmux sessions (see notes below)

## tmux and Screen Notes

gdedit works inside tmux, but some features depend on terminal passthrough. Truecolor and mouse tracking require tmux version 3.2 or newer with proper configuration.

Add this to your tmux.conf:

```
set -g allow-passthrough on
set -g default-terminal "tmux-256color"
```

For truecolor support, also ensure your tmux config includes:

```
set -ga terminal-overrides ",*:Tc"
```

Without these settings, colors may appear wrong and mouse input may not work. The editor itself will function, but the experience degrades.

Screen sessions are not recommended. Screen has limited escape sequence support and many features will not work. Use tmux instead.

## Out of Scope

These environments are not supported and will not be made to work. Do not file bugs related to these terminals.

- Windows Command Prompt (conhost)
- Windows PowerShell console (not Windows Terminal)
- legacy dumb terminals (VT52, Tektronix 4100 series)
- any terminal reporting fewer than 80x24 cells
- terminals without UTF-8 support
- terminal emulators that do not implement the alternate screen buffer

If you attempt to run gdedit in one of these environments, it will likely display a clear error and exit. We do not provide workarounds or compatibility shims for these cases.

## Manual Test Matrix

This matrix lists basic checks you can perform to verify terminal compatibility. Run these manually if you are unsure whether your terminal works.

| Test | Expected Result |
|------|-----------------|
| Launch gdedit | Editor starts, displays empty buffer or welcome screen |
| Type characters | Characters appear correctly with proper encoding |
| Arrow keys | Cursor moves in all four directions |
| Enter/Backspace | Newlines and deletions work as expected |
| Resize terminal to 80x24 | UI adapts without breaking |
| Resize below 80x24 | Editor shows warning or error |
| Mouse click in editor | Cursor moves to clicked position (if mouse supported) |
| Copy text | Selected text copies to clipboard (if supported) |
| Paste text | Pasted text appears in buffer |
| Quit editor | Returns cleanly to shell, original screen content restored |

If all required tests pass, your terminal is compatible. Best-effort features are optional.

## Version History

- 2026-03-06: Initial contract

This document may be updated as gdedit evolves. Check the docs folder for the latest version.
