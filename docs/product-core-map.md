# Product Core Map

## Purpose

This document turns the scattered product concepts in `dev-guide/2026-03-06_18-07-11_ChatGPT_1._터미널 에디터 기획(gdedit).md` into a compact reference. It is the bridge between product vision and later implementation documents.

## Core Product Statement

- `gdedit` is a cowork editor centered on an edit agent.
- The human and the edit agent share editing context and control on the same editing surface.
- The product is organized around visible state, bounded edits, and deliberate control.

## Concept Map

### 1. Control Hub

Definition:
- The control hub is the single representative control window for agent invocation, text commands, voice-injected commands, confirmation, and review entry.

Why it exists:
- It gives `gdedit` one stable place where control happens.
- It prevents command behavior from being split across multiple competing control surfaces.
- It keeps voice and text on the same path.

Downstream impact:
- Later UI docs must preserve exactly one primary control hub.
- Control commands, previews, and approvals should route through the same place.
- Voice should land in the control hub before execution.

### 2. Edit Tabs

Definition:
- Edit tabs are multiple editing contexts separated by task, file, comparison target, or work thread.

Why it exists:
- Editing context changes naturally across tasks.
- Tabs separate work contexts while keeping control shared.
- The split is: tabs hold editing context; the control hub stays common.

Downstream impact:
- Later docs must define tab, buffer, session, and task-context relationships.
- Agent actions should default to the active tab and its local context.

### 3. Status Surface

Definition:
- The status surface is the persistent state summary area that reports what the editor knows right now.

Why it exists:
- `gdedit` needs explicit visibility of mode, scope, agent state, review state, and command results.
- It is not just a thin status bar; it is a compact context dashboard.

Primary examples:
- active tab
- project name
- selection range
- agent state
- last proposal result
- voice status
- target scope
- highlight summary

Downstream impact:
- Later docs should keep this area compact and dashboard-like, not a log dump.
- Status vocabulary needs to be standardized early.

### 4. Edit Agent

Definition:
- The AI in `gdedit` is an edit agent, not a coding agent.

Why it exists:
- `gdedit` is built around local refinement, patching, and review.
- The product rejects the idea that the best AI collaborator is a broad rewrite engine.

Primary responsibilities:
- understand current editing context
- find relevant positions
- propose partial edits
- inspect likely impact range
- verify local coherence
- explain the change

Non-responsibilities:
- broad opaque rewrites by default
- detached chat-only generation as the primary flow
- ignoring current cursor, selection, diff, or locked state

Downstream impact:
- Later agent docs must define hard scope boundaries and proposal flow.
- Review-first patch handling should remain the default.

### 5. Control Handoff

Definition:
- Control handoff is the visible sharing and transfer of editing control between the human and the edit agent.

Why it exists:
- The product is not fully manual and not fully automatic.
- The value comes from semi-automatic collaboration with explicit control exchange.

Typical states:
- human-led editing
- agent-suggesting
- review-pending
- approved/applying
- locked/denied

Downstream impact:
- Later docs need a state model for handoff.
- Agent actions must be previewable and reversible.
- The user should always understand who currently has initiative and where.

### 6. Control Mode

Definition:
- Control mode is the state where the control hub is active.

Why it exists:
- `gdedit` wants clear control without heavy modal complexity.
- The user should feel “I focused the control hub,” not “I entered a separate world.”

Operational meaning:
- editing state = direct document editing
- control state = control hub focused

Downstream impact:
- Focus transitions matter more than adding many named modes.
- The status surface must make the current state explicit.

### 7. Voice Control

Definition:
- Voice is a control-mode-only input channel.

Why it exists:
- Ambient listening creates noise, ambiguity, and trust problems.
- Voice is strongest when it carries intent into a controlled, reviewable path.

Expected flow:
- focus control hub
- speak command
- transcript appears in control hub
- user confirms or edits text
- execute

Good command classes:
- ask for inspection
- ask for bounded refactor
- request diff/review
- confirm/hold proposal
- switch tab or panel

Downstream impact:
- Later voice docs should optimize for command intent, not dictation-first coding.
- Voice output must not bypass the control hub.

### 8. Highlighting Layers

Definition:
- Highlighting is a shared visual language for editing and collaboration state.

Why it exists:
- In `gdedit`, highlighting is not just syntax color; it expresses intent, risk, and shared focus.
- Visual state can communicate faster than text in a cowork editing flow.

Layer model:
- base highlighting: syntax, symbol, search, diagnostics
- editing highlighting: recent edits, selection, diff, unsaved changes
- cowork highlighting: agent focus, proposed changes, review needed, impact range, locked region, conflict risk, approval wait

Downstream impact:
- Later UI docs need a stable taxonomy for state overlays.
- Reduced-color terminals need a fallback marker strategy.

### 9. Workstyle System

Definition:
- Shortcuts in `gdedit` are part of a workstyle package, not just a command map.

Why it exists:
- The product aims to encode editing methodology, not only key bindings.
- A workstyle can bundle keymap, panel habits, review flow, agent behavior, and task preference.

Example dimensions:
- keymap
- panel/tab habits
- agent invocation rules
- review defaults
- language-specific editing habits
- mouse usage level

Downstream impact:
- Later docs should define a stable workstyle schema.
- Agent guidance may recommend styles, but ownership stays with the user.

## Canonical Layout Direction

Current preferred direction from the product source:

- top: tab bar
- center: active edit tab
- bottom-left: control input hub
- bottom-right: status surface
- optional later panels: diff, agent result, references, highlight details

This is a direction, not a frozen implementation spec. The precise layout belongs in later UI architecture docs.

## Product Rules Implied by the Map

- one primary control hub
- multiple edit tabs
- control routed by active tab, cursor, selection, and current project context
- voice only through the control hub
- reviewable patch flow over opaque rewrite flow
- highlighting used as collaboration-state semantics
- workstyle recommendations allowed, forced style mutation not allowed

## What Needs Its Own Follow-Up Document

- `docs/ui-architecture.md`
- `docs/control-panel.md`
- `docs/tab-model.md`
- `docs/edit-agent.md`
- `docs/control-handoff.md`
- `docs/guarded-editing.md`
- `docs/highlighting-model.md`
- `docs/status-language.md`
- `docs/voice-control.md`
- `docs/workstyle-system.md`

## Source Notes

This map is derived from the product planning discussion in `dev-guide/2026-03-06_18-07-11_ChatGPT_1._터미널 에디터 기획(gdedit).md`, especially the sections on edit agents, control-mode-as-focus, representative control hub, voice restriction, collaborative highlighting, and workstyle-oriented shortcuts.
