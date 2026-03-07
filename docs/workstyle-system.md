# Workstyle System

## Purpose

This document defines workstyles in `gdedit`. A workstyle is a shareable editing methodology package, not just a keybinding preset.

## Core Definition

- A workstyle is a package of editing habits, control rules, review defaults, and shortcut organization.
- Workstyles encode how a user prefers to work, not only what keys are bound.
- The edit agent may recommend workstyles, but ownership stays with the user.

## Why Workstyles Exist

Traditional editors usually treat shortcuts as fixed command maps that the user must memorize. `gdedit` takes a different direction:

- usage patterns accumulate first
- style becomes visible and shareable
- shortcuts are organized around editing philosophy
- the edit agent can guide adaptation

That makes shortcuts a workstyle interface layer rather than a static mapping table.

## What A Workstyle Contains

A workstyle may bundle:

- keymap choices
- panel and tab habits
- control-hub habits
- agent invocation patterns
- review and approval defaults
- selection-first or cursor-first targeting habits
- language- or task-specific editing conventions
- mouse involvement level

## Workstyle Dimensions

### Keymap

- preferred shortcuts
- mnemonic vs modal emphasis
- navigation rhythm
- terminal-safe fallback priorities when classic desktop shortcuts do not survive terminal transport

### Panel / Layout Habits

- preferred tab switching behavior
- side-panel usage habits later
- focus movement preferences

### Agent Involvement Rules

- when to inspect
- when to propose
- when to require review
- how strongly to prefer patch preview

### Review Defaults

- diff-first vs apply-first tendency
- approval expectations
- guarded-editing strictness

### Language / Task Preferences

- Go-focused review patterns
- Rust structural movement patterns
- pair-edit style preferences

## Core Philosophy

Workstyles are not “command lists.”

They are compressed editing methodologies.

That means a workstyle expresses:

- how the user explores
- how the user patches
- how the user reviews
- how the user collaborates with the edit agent

## Example Workstyles

### Local Patch Review Style

- favors small block edits
- emphasizes diff review and impact checking
- keeps agent proposals narrow

### Exploration-Centered Style

- emphasizes symbol movement and reference tracing
- favors navigation and structure discovery
- uses the agent more for explanation than patching

### Agent Pair-Edit Style

- uses frequent selection-based analysis
- requests patch suggestions often
- keeps handoff and review state highly visible

### Panel-Operation Style

- emphasizes tab and panel focus movement
- uses layout as part of the method
- pairs well with terminal split workflows later

### Terminal-Safe Editing Style

- prefers predictable keys such as function keys or `Alt`-modified runes over desktop-only shortcut assumptions
- treats best-effort aliases as optional, not canonical
- separates storage characters from rendered markers when the terminal cannot faithfully represent width-sensitive text
- accepts redundant modifier paths when terminals inconsistently forward `Shift`, `Alt`, or combined variants

### Keyboard-First Selection Style

- treats selection as a character-range operation anchored by carets
- keeps structural edits line-aware by projecting the text selection to covered lines
- uses clipboard operations on the selected text, not on implicit line blocks
- prefers keyboard selection over mouse-based internal editing semantics in terminal environments

## Shareability

A workstyle should be shareable as a package, not just exported as raw keybindings.

What users may share:

- keymap choices
- mode/control placement habits
- panel operation habits
- agent call patterns
- review/approval flow
- selection-based command combinations
- language-specific editing practices

This is the basis for a future style-sharing ecosystem.

## Agent Guidance Role

The edit agent may help users shape and learn workstyles.

Examples of appropriate guidance:

- suggest a shortcut for a repeated action
- suggest a better control-hub command pattern
- suggest review-first behavior for a risky style
- suggest panel focus movement for a chosen layout habit

## Ownership Rule

This is the critical constraint:

- the agent may recommend
- the agent may explain
- the agent may teach
- the agent must not impose

User ownership remains primary.

## Anti-Patterns

The workstyle system fails when:

- the agent silently rewrites shortcuts
- style changes are forced on the user
- recommendations become noisy interruptions
- workstyle becomes a hidden system the user cannot inspect

## Recommendation Rules

Recommendations should be:

- contextual
- rare enough to avoid fatigue
- clearly framed as suggestions
- easy to dismiss
- grounded in observed editing behavior

For terminal-first workstyles, recommendations should also preserve transport reliability. A shortcut that looks elegant on paper but fails in tmux or ssh should not be promoted as the default style.

## Relationship to Other Systems

### Relationship to Control Hub

- workstyles may shape how commands are entered and confirmed
- they may prefer particular command rhythms or focus transitions

### Relationship to Control Handoff

- some workstyles may prefer stronger review gating
- others may prefer quicker proposal turnover

### Relationship to Guarded Editing

- workstyles can tune how conservative proposal review feels
- they must not weaken the core safety model invisibly

## Non-Goals

- no forced style mutation
- no opaque personalization that changes editor behavior without consent
- no reducing workstyle to a simple hotkey export

## Phase 4 Acceptance

Phase 4 workstyle-system documentation is complete when it clearly defines:

- workstyle as a methodology package
- the major dimensions of a workstyle
- several concrete example workstyles
- shareability beyond raw keymaps
- the agent recommendation rule: guide, but do not impose
