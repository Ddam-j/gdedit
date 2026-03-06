# WORKLOG

## Session

- Session ID: `ses_33d533b68ffedk0BojcwN3zL5N`

## Scope

- Consolidated the product-core planning work through Phase 5.
- Captured product definition, UI structure, edit-agent rules, shared visual language, voice/workstyle rules, and the first runnable TUI shell.

## Completed Work

### Phase 0 - Product Freeze

- Added `docs/product-vision.md`.
- Added `docs/product-core-map.md`.
- Fixed the product definition around a cowork editor with an edit agent, one control hub, visible state, and bounded editing.

### Phase 1 - UI Structure Freeze

- Added `docs/ui-architecture.md`.
- Added `docs/control-panel.md`.
- Added `docs/tab-model.md`.
- Locked the canonical structure: top tab bar, center edit surface, bottom-left control hub, bottom-right status surface.

### Phase 2 - Edit-Agent Protocol

- Added `docs/edit-agent.md`.
- Added `docs/control-handoff.md`.
- Added `docs/guarded-editing.md`.
- Defined the edit agent as a patch actor, formalized review-first handoff, and documented locked-scope / anti-overrewrite rules.

### Phase 3 - Shared Visual Language

- Added `docs/highlighting-model.md`.
- Added `docs/status-language.md`.
- Defined the three-layer highlighting model and stabilized the status-surface vocabulary.

### Phase 4 - Voice and Workstyle

- Added `docs/voice-control.md`.
- Added `docs/workstyle-system.md`.
- Restricted voice to control mode and defined workstyles as user-owned methodology packages.

### Phase 5 - Minimal Product Shell Implementation

- Added `internal/tui/app.go`.
- Added `internal/tui/commands.go`.
- Added `internal/tui/commands_test.go`.
- Updated `cmd/gdedit/main.go` to launch the minimal shell by default and support `--tui`.
- Added `tcell` to `go.mod` / `go.sum`.
- Implemented the first visible shell with:
  - fixed layout regions
  - visible tab switching
  - control-hub focus flow
  - preview/confirm command loop
  - status updates for focus, scope, review, and voice state

### Phase 6 - Cowork Editing MVP

- Extended `internal/tui/commands.go` with bounded preview kinds, proposal IDs, review labels, and locked-scope denial behavior.
- Extended `internal/tui/app.go` with visible handoff states, review queue wiring, cowork line markers, and locked-region handling.
- Added first cowork-state rendering on the edit surface:
  - `L!` locked region
  - `P>` agent proposal
  - `R?` review-needed or inspect state
  - `H*` human-selected edit line
  - `A+` approved state
  - `X!` denied change in locked scope
- Added Phase 6 tests for preview kinds and locked-scope denial in `internal/tui/commands_test.go`.
- Verified three tmux-driven shell paths:
  - locked-scope denial preview
  - unlocked proposal preview with review queue
  - proposal apply returning focus to editor

## Verification

- `go test ./...` passed.
- `go build ./cmd/gdedit` passed.
- `go run ./cmd/gdedit --help` worked.
- `go run ./cmd/gdedit --version` worked.
- `go run ./cmd/gdedit --doctor` worked.
- Verified TUI startup and quit inside tmux after tmux became available.

## Notes

- Temporary tmux verification scripts were removed after validation.
- Phase 5 is complete in the product-core plan and the next implementation target is Phase 6.

## Post-Phase 6 Usability Refinement

- Reworked the editor keymap so printable characters are no longer stolen by mnemonic shortcuts.
- Added a visible help flow (`F1` / `Esc`) and a quit confirmation flow on `Ctrl+C`.
- Added direct editor mutations for insert, delete, newline split, merge, and visible caret handling.
- Added example tabs for Go, Python, TypeScript, and YAML so block-selection behavior can be tested in-place.
- Added hierarchical block selection with a terminal-safe default on `F2`; `Ctrl+[` and `Ctrl+Space` remain best-effort aliases.
- Scoped cursor and selection state per tab so tab switching preserves the active tab's own selection instead of leaking selection across tabs.
- Moved tab navigation to `Alt+.` and `Alt+,` because `Ctrl+Tab` is unreliable in many terminal stacks.
- Changed indentation policy to user-directed editing:
  - `Tab` inserts a literal `\t` when there is no selection.
  - `Tab` indents only the active selection.
  - `Shift+Tab` outdents the active selection.
  - `Alt+0` switches selection indentation to literal tabs.
  - `Alt+1` through `Alt+4` switch selection indentation to 1-4 spaces.
- Split stored tab characters from displayed tab markers: literal `\t` is saved in the buffer, but renders as a styled `»` marker in the edit surface.
