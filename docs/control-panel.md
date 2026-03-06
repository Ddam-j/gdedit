# Control Panel

## Purpose

This document defines the behavior of the `gdedit` control panel, also called the control hub. It is the primary path for explicit control, agent invocation, command preview, and voice-to-command flow.

## Core Definition

- The control panel is the single representative control hub of `gdedit`.
- Control mode is experienced as the control panel being focused.
- The control panel is not a detached command world; it is a context-aware entry point into the current editing surface.

## Main Responsibilities

- accept typed control input
- accept voice-injected text
- show command intent before execution
- route actions to the current editing context
- expose preview, confirmation, and review flow
- provide a stable place for agent invocation

## Focus Model

Two top-level interaction states are enough for Phase 1:

- editing state: the active edit surface has focus
- control state: the control panel has focus

The user-facing experience should be:

- not “I entered a separate mode”
- but “I focused the control panel”

## Focus Transitions

Expected transitions:

- editor -> control panel by shortcut or click
- control panel -> editor after execution, cancel, or explicit return
- voice input only when the control panel is active

## Command Flow

Base flow:

1. user focuses the control panel
2. user types or speaks a short instruction
3. `gdedit` resolves scope from visible context
4. the command is previewed or shown in actionable form
5. the user confirms, edits, or cancels
6. the result is routed back into the active editing context

## Scope Resolution Rules

Control commands must be resolved against the current editing context.

Priority order:

1. current selection
2. current cursor location
3. current block / function / file when inferable
4. preview before execution when ambiguity remains

The control panel should never behave like a context-blind shell.

## Voice Integration

Voice is a control-panel input path, not a parallel system.

- voice is allowed only when the control panel is active
- voice result should appear in the control panel before execution
- the user may correct the text before confirming
- direct blind execution is not the default

This keeps voice deliberate, reviewable, and aligned with the control philosophy.

## Canonical Command Categories

Phase 1 should keep the command set small and legible.

### Edit Intent

- rename this
- simplify this block
- insert project name here
- add TODO here

### Agent Invocation

- inspect recent change
- suggest a patch for this selection
- show agent draft

### Scope / Review

- show diff only
- highlight impact range
- hold for review
- approve this proposal

### Navigation / Context

- switch tab
- focus review panel later
- target current selection

## Command Language Style

The control panel favors short, context-aware intent rather than long chat prompts.

Good qualities:

- short
- local
- scope-aware
- preview-friendly

Bad qualities:

- broad whole-project ambiguity by default
- detached prompt style with no visible target
- opaque execution with no review path

## Preview-First Principle

The control panel should prefer preview-before-execute when:

- scope is ambiguous
- impact is non-local
- the edit touches multiple ranges
- the user is invoking an agent-generated patch

Immediate execution is acceptable only for clearly bounded, low-risk actions.

## Relationship to the Status Surface

- the control panel is where the user expresses intent
- the status surface is where `gdedit` summarizes state and result

The control panel should not absorb status responsibilities, and the status surface should not turn into a second command system.

## Relationship to the Edit Agent

- the control panel is the main entry point for explicit edit-agent requests
- the agent should return suggestions, previews, or patches that remain reviewable
- the control panel should be able to represent the current target scope before the agent acts

## Layout Role

Current preferred layout:

- bottom-left: control input panel
- bottom-right: status surface

This aligns with the product direction that the left side feels like the place where the user speaks and the right side feels like the place where the editor answers.

## Non-Goals

- no multiple primary control panels
- no ambient voice listening
- no giant chat transcript as the main control UI
- no command model that ignores cursor, selection, and active tab
- no hidden automatic broad rewrite path

## Phase 1 Acceptance

Phase 1 control-panel documentation is complete when it clearly defines:

- control mode as focus on the control panel
- text and voice using the same control path
- scope resolution priority
- preview-before-execute behavior
- compact canonical command categories
