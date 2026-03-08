# Guarded Editing

## Purpose

This document defines the safety rules for edit-agent collaboration in `gdedit`. It exists to prevent small requested changes from turning into broad, untrusted rewrites.

## Core Definition

- `gdedit` uses guarded editing, not blind automatic rewriting.
- The edit agent acts against the currently visible scope.
- Scope boundaries must stay narrow, explicit, and context-aware.

## Why Guarded Editing Exists

Many coding-agent workflows fail in the same way:

- a small requested change expands into a large rewrite
- unrelated behavior gets touched
- human local intent gets lost
- confidence drops because the system stops feeling controllable

`gdedit` exists to reduce that failure mode.

## Primary Safety Principle

The agent must prefer patch-sized action over document-sized action.

Examples of valid constraints:

- change only this line
- rewrite only this block
- keep this function signature
- do not modify outside this selection
- inspect only the areas that conflict with my manual edit

## Guard Rails

### 1. Scope Boundaries

Agent action should be bounded by explicit or inferred scope.

Typical scope anchors:

- current selection
- current cursor
- current block or function
- explicitly allowed range
- active tab / current context

If the requested action cannot be safely bounded, `gdedit` should fall back to preview and scope clarification rather than widen scope silently.

### 2. Scope Before Apply

The live prototype uses visible scope rather than line-level locks or proposal queues.

- selection, caret, and active tab define the working boundary
- the user should be able to preview the intended scoped action before confirming it
- the system should never silently widen beyond the visible working scope

### 4. Preserve Human Intent

Human direct editing is part of the context, not something to overwrite casually.

The agent should read:

- recent human edits
- manual deletions
- explicit selections
- already approved diffs

and use them as constraints on its own proposals.

### 5. Explain Scope and Impact

The system should be able to show:

- what range the proposal will modify
- what nearby regions may be affected
- whether the intended action stays inside the requested scope

## Proposal Types

### Single-Range Patch

- one local target range
- clear low-risk patch proposal
- easiest case for fast review

### Multi-Range Patch

- several bounded ranges related to one local intent
- must remain reviewable as a connected proposal
- should expose impact and conflict risk clearly

### Rejected Scope

- blocked by invalid scope or over-broad impact
- should surface reason rather than silently rewriting the request

## Shared State Required for Guarding

Guarded editing depends on shared edit state, not just conversation state.

Important state includes:

- current file
- current selection
- current mode/control state
- recent diff
- editable range
- human-touched region

## Anti-Patterns

Guarded editing is violated when the system does any of the following:

- rewrites the whole file for a local request
- changes unrelated areas without explicit review
- ignores the current visible scope
- applies agent output without a visible scoped preview for non-trivial changes
- hides impact range from the user
- erases or overrides recent human edits without acknowledgment

## Relationship to Highlighting

Highlighting is one of the main safety channels for guarded editing.

It should help expose:

- proposed patch ranges
- impact ranges
- selected region
- current caret line
- human recent edits

## Relationship to Control Handoff

Guarded editing supports healthy collaboration.

- the agent can discuss or edit without taking over the entire surface
- the human can refine scope before confirming
- rejected scope outcomes stay visible as part of the shared state

## Default Policy

Until proven safe otherwise, the default policy should be:

- narrow scope
- preview first
- confirm meaningful edits
- never widen scope silently

## Example Flows

### Single-Range Edit

1. user selects a block
2. user asks for simplification
3. system previews the scoped action for that block
4. user confirms, edits the request, or narrows the scope

### Multi-Range Edit

1. user asks for a local rename with reference updates in current scope
2. agent identifies several linked ranges
3. system shows the scoped impact summary
4. user confirms before apply

### Rejected Edit

1. user request crosses an over-broad boundary
2. agent cannot safely comply under current constraints
3. system asks for narrower scope through preview flow

## Phase 2 Acceptance

Phase 2 guarded-editing documentation is complete when it clearly defines:

- patch-oriented editing as the default
- scope-before-apply behavior
- preview-before-apply flow
- single-range, multi-range, and rejected-scope cases
- anti-patterns that `gdedit` must avoid
