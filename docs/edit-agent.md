# Edit Agent

## Purpose

This document defines the edit agent in `gdedit`. It fixes the product boundary between an edit agent and a coding agent so later implementation stays aligned with the cowork editing model.

## Core Definition

- The AI in `gdedit` is an edit agent.
- It is not a general code generator.
- It works from the current editing surface and shared editing state.

In product terms:

- coding agent: generate broadly from prompt-driven intent
- edit agent: refine locally from edit-surface context

## Why `gdedit` Needs an Edit Agent

- `gdedit` is built around local refinement, not broad autonomous rewriting.
- Human edits, cursor movement, selection, recent diffs, and local focus are part of the agent context.
- The edit agent is meant to continue and inspect human editing behavior, not replace it with detached generation.

## What the Edit Agent Reads

The edit agent should treat the editing surface as primary input.

Minimum context:

- current file or active buffer
- current tab / work context
- current cursor
- current selection
- recent diffs
- recent human edits
- visible highlighting and review state
- locked regions and allowed edit scope

## Primary Responsibilities

The edit agent should stay within a small, legible responsibility set.

### 1. Locate

- find relevant positions in the current editing context
- map local intent to the right symbol, block, or selected region

### 2. Propose Patch

- suggest bounded edits rather than broad rewrites
- prefer selection, cursor, or explicitly targeted scope

### 3. Analyze Impact

- inspect likely impact range
- surface potential conflicts or related references

### 4. Verify Coherence

- check local consistency after a change
- help detect mismatches between human edits and agent-proposed edits

### 5. Explain

- explain what changed and why in local, reviewable terms
- keep explanations tied to visible edit state, not generic chat narration

## Secondary Responsibilities

- interpret human local-intent signals from direct editing behavior
- help identify where a small patch should land
- help constrain changes when the user wants narrow modification

## Hard Boundaries

The edit agent must not drift into broad coding-agent behavior.

- no default whole-file rewrites for small edits
- no detached chat-first workflow as the primary path
- no ignoring current selection, cursor, diff, or locked regions
- no silent multi-context edits without explicit scope widening
- no treating human edits as disposable when reconciling changes

## Core Operating Principle

The edit agent is a patch actor.

That means:

- it acts on bounded ranges
- it preserves context when possible
- it returns reviewable proposals
- it suppresses over-recoding
- it avoids hallucination-style rewrite expansion

## Human Signals Matter

In `gdedit`, human editing behavior is part of the command language.

Examples of meaningful signals:

- current selection
- a recent manual deletion
- a renamed symbol under the cursor
- a recently approved diff
- a highlighted region marked for review

The edit agent should treat these as intent-bearing signals, not as noise around a prompt.

## Relationship to the Control Hub

- The control hub is the main explicit entry point for edit-agent invocation.
- The edit agent should receive scope and intent through control-hub actions plus current edit state.
- The control hub may preview or constrain the intended action before the agent proposes a patch.

## Relationship to Highlighting and Review

- The edit agent should produce outputs that can be visualized in shared collaboration layers.
- Proposed ranges, impact ranges, review-needed state, and locked-region denial should be visible on screen.
- The edit agent should support cheap review instead of forcing opaque acceptance.

## Non-Responsibilities

The edit agent is not responsible for:

- replacing the editor with a full autonomous coding workflow
- broad speculative generation across the project by default
- controlling the UI through hidden state transitions
- overriding user ownership of patch acceptance

## Design Tests

When evaluating an edit-agent feature, ask:

- Does it strengthen local patch-oriented editing?
- Does it read real edit-surface context?
- Does it reduce over-recoding risk?
- Does it keep the human in an active editing role?
- Does it remain reviewable and bounded?

## Phase 2 Acceptance

Phase 2 edit-agent documentation is complete when it clearly defines:

- edit agent vs coding agent
- 3-5 primary responsibilities
- scope and context expectations
- hard boundaries against broad rewrite behavior
- patch-actor identity as the default operating model
