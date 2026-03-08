# Product Vision

## One-Line Definition

`gdedit` is a terminal cowork editor where a human and an edit agent share context and control to refine text files together.

## Short Definition

`gdedit` is a terminal editor for an editing-focused assistant.

## Core Promise

- `gdedit` helps a human refine code, config, notes, and skill files with a connected edit agent on the same editing surface.
- It favors bounded edits, visible state, and reviewable changes over opaque generation.
- It keeps control deliberate: editing stays direct, and agent interaction happens through an explicit control path.

## Product Direction

- **Refining over generating**: `gdedit` is built around patching, adjustment, inspection, and coherence checks rather than broad rewrites.
- **Co-editing over instructing**: the human and the edit agent share an editing surface, not a detached chat loop.
- **Editing-first assistance over orchestration-first AI**: the product should deepen file-aware assistance before turning into a generic external-agent console.
- **Visible state over hidden magic**: mode, target scope, command state, and edit-agent involvement should be visible on screen.
- **One control hub**: control is concentrated in one scoped control hub rather than scattered across many command surfaces.
- **Workstyle over fixed keymap**: shortcuts are part of an editing style, not just a static command list.

## What Makes `gdedit` Different

- It is not a generic terminal editor with AI bolted on.
- It is not a coding agent shell optimized for large autonomous rewrites.
- It is a cowork editing environment designed for local, contextual, reviewable change and durable file intent.

## Edit Agent, Not Coding Agent

In `gdedit`, the AI is an edit agent.

- It reads the current editing surface, cursor, selection, recent edits, diffs, and visible state.
- It proposes bounded patches instead of acting as a general-purpose code generator.
- It helps with locating, patching, impact checking, verification, and explanation.
- It can also help preserve user intent in dedicated text artifacts such as notes, memos, and handoff summaries.
- It should reduce over-recoding, not amplify it.

## Control Philosophy

- Normal editing remains direct and keyboard-first.
- Control is explicit and intentional.
- Control mode is not a large modal universe; it is primarily experienced as the control hub being focused.
- Voice is not ambient. It is one input channel for the control hub.

## Collaboration Philosophy

- The human remains an active editor, not a passive reviewer of AI output.
- The edit agent should collaborate through scope, preview, and confirmation.
- Highlighting is part of the collaboration model because it exposes editing intent, risk, and active scope.
- The best result is a semi-automatic workflow where human local judgment and agent structural reasoning reinforce each other.

## Assistant-Oriented Text Work

`gdedit` should become especially strong at text work that benefits from local meaning and durable intent.

- understanding what a file is for
- recording why a setting, block, or file matters
- keeping notes that help future edits stay aligned with user intent
- preparing assistant-readable text artifacts for later outside delegation when needed

Memo storage should follow context.

- system and app memos should live under the system memo root configured in `~/.config/gdedit/config.json`
- project memos should live with the relevant project inside its `.gdedit/` directory
- memo files should remain portable and AI-readable so they can help recreate or adapt setup on another machine

This matters most for settings files, code, personal notes, and skill files where tiny details often carry long-lived meaning.

External-agent interaction is a later layer. When it arrives, it should flow through file-backed notes, memos, and scoped text artifacts rather than replace the editing-first core.

## Design Principles

- **Keep the center stable**: one control hub, a clear active editing context, and a readable status surface.
- **Prefer bounded scope**: current tab, cursor, selection, and recent local context matter more than vague whole-project actions.
- **Make confirmation cheap**: users should be able to inspect, confirm, adjust, or narrow agent actions without losing flow.
- **Preserve intent durably**: if user rationale matters later, the product should help store it in plain text artifacts.
- **Limit surprise**: avoid ambient listening, hidden mode shifts, and unrequested broad edits.
- **Support growth in workstyle**: the editor should eventually help the user shape and share editing habits without taking ownership away.

## Anti-Goals

- `gdedit` is not an always-listening voice interface.
- `gdedit` is not a chat-first AI IDE.
- `gdedit` is not a tool for opaque multi-file rewrites as the default path.
- `gdedit` is not built around many competing control windows.
- `gdedit` is not meant to force style changes on the user.
- `gdedit` is not primarily a generic multi-agent operations console.

## Decision Filters

When deciding whether a feature belongs in `gdedit`, ask:

- Does it help a human and an edit agent share editing context?
- Does it strengthen local, reviewable, scope-aware editing?
- Does it keep control visible and understandable?
- Does it avoid unnecessary mode burden and AI noise?
- Does it support the product identity of a cowork editor rather than drift into a generic IDE?
- Does it improve file understanding, user memo capture, or durable intent preservation?

## Source Notes

This document is distilled from the product direction recorded in `dev-guide/2026-03-06_18-07-11_ChatGPT_1._터미널 에디터 기획(gdedit).md`, especially the sections defining `gdedit` as an edit-agent centered cowork editor, restricting voice to control mode, and treating shortcuts and highlighting as part of the collaboration model.
