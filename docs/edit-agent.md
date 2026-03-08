# Edit Agent

## Purpose

This document defines the edit agent in `gdedit`. It fixes the product boundary between an edit agent and a coding agent so later implementation stays aligned with the cowork editing model.

The current product direction is editing-first assistance, not generic agent orchestration.

## Core Definition

- The AI in `gdedit` is an edit agent.
- It is not a general code generator.
- It works from the current editing surface and shared editing state.
- It should act like a practical assistant for text artifacts in the workspace.

In product terms:

- coding agent: generate broadly from prompt-driven intent
- edit agent: refine locally from edit-surface context and preserve durable intent around files

## Why `gdedit` Needs an Edit Agent

- `gdedit` is built around local refinement, not broad autonomous rewriting.
- Human edits, cursor movement, selection, recent diffs, and local focus are part of the agent context.
- The edit agent is meant to continue and inspect human editing behavior, not replace it with detached generation.
- It should help users manage meaning around text files, not just emit patch-like output.

## What the Edit Agent Reads

The edit agent should treat the editing surface as primary input.

Minimum context:

- current file or active buffer
- current tab / work context
- current cursor
- current selection
- recent diffs
- recent human edits
- visible highlighting and scope state
- durable notes or memos attached to the current file when they exist

## Primary Responsibilities

The edit agent should stay within a small, legible responsibility set.

### 1. Locate

- find relevant positions in the current editing context
- map local intent to the right symbol, block, or selected region

### 2. Propose Patch

- suggest bounded edits rather than broad rewrites
- prefer selection, cursor, or explicitly targeted scope

### 3. Capture Intent

- turn user notes into durable text artifacts tied to a file or scope
- preserve rationale that should survive beyond the immediate edit session
- help the user leave assistant-readable context for later work
- choose memo destinations according to context: system/app history under the system memo root configured in `~/.config/gdedit/config.json`, project intent inside the project's `.gdedit/` directory

### 4. Analyze Impact

- inspect likely impact range
- surface potential conflicts or related references

### 5. Verify Coherence

- check local consistency after a change
- help detect mismatches between human edits and agent-proposed edits

### 6. Explain

- explain what changed and why in local, reviewable terms
- keep explanations tied to visible edit state, not generic chat narration

## Secondary Responsibilities

- interpret human local-intent signals from direct editing behavior
- help identify where a small patch should land
- help constrain changes when the user wants narrow modification
- help maintain file-level notes, memos, and handoff documents for settings, code, and skill files
- help keep those records portable enough to reuse when the same setup or configuration must be recreated on another system
- help keep memo files readable enough that AI models can reliably use them as execution context

## Hard Boundaries

The edit agent must not drift into broad coding-agent behavior.

- no default whole-file rewrites for small edits
- no detached chat-first workflow as the primary path
- no ignoring current selection, cursor, diff, or current file context
- no silent multi-context edits without explicit scope widening
- no treating human edits as disposable when reconciling changes
- no drift into a generic remote-agent command broker as the primary product identity

## Core Operating Principle

The edit agent is a patch actor and file-aware assistant.

That means:

- it acts on bounded ranges
- it preserves context when possible
- it can preserve durable notes and assistant-facing context when useful
- it suppresses over-recoding
- it avoids hallucination-style rewrite expansion

## Human Signals Matter

In `gdedit`, human editing behavior is part of the command language.

Examples of meaningful signals:

- current selection
- a recent manual deletion
- a renamed symbol under the cursor
- a file memo explaining user intent
- a handoff note prepared for later assistance

The edit agent should treat these as intent-bearing signals, not as noise around a prompt.

## Relationship to the Control Hub

- The control hub is the main explicit entry point for edit-agent invocation.
- The edit agent should receive scope and intent through control-hub actions plus current edit state.
- The control hub may preview or constrain the intended action before the agent edits or records a durable note.

## Relationship to Highlighting and Stored Context

- The edit agent should produce outputs that can be visualized in shared collaboration layers.
- Scope, impact ranges, and durable file intent should remain visible or recoverable.
- The edit agent should support cheap confirmation instead of forcing opaque acceptance.

## Non-Responsibilities

The edit agent is not responsible for:

- replacing the editor with a full autonomous coding workflow
- broad speculative generation across the project by default
- controlling the UI through hidden state transitions
- overriding user ownership of patch acceptance
- becoming a generic external-agent dispatcher before the editing assistant layer is mature

## Design Tests

When evaluating an edit-agent feature, ask:

- Does it strengthen local patch-oriented editing?
- Does it read real edit-surface context?
- Does it reduce over-recoding risk?
- Does it keep the human in an active editing role?
- Does it preserve meaningful file intent when needed?
- Does it remain bounded and scope-aware?

## Phase 2 Acceptance

Phase 2 edit-agent documentation is complete when it clearly defines:

- edit agent vs coding agent
- 3-5 primary responsibilities
- scope and context expectations
- hard boundaries against broad rewrite behavior
- patch-actor identity as the default operating model
