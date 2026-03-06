# Highlighting Model

## Purpose

This document defines highlighting in `gdedit` as a shared visual language for editing and collaboration state.

## Core Definition

- Highlighting in `gdedit` is not only syntax coloring.
- It is a collaboration-state layer.
- It exposes editing intent, risk, ownership, and review state on screen.

## Why Highlighting Matters

- People read visual state faster than long explanations.
- In cowork editing, users look at regions, differences, and risks before they read prose.
- Highlighting becomes one of the fastest communication channels between the human and the edit agent.

## Three-Layer Model

`gdedit` should treat highlighting as at least three layers.

### 1. Base Highlighting

This is the familiar editor layer.

- syntax
- symbols
- search results
- diagnostics

Purpose:
- help the user read code structure and baseline editor feedback

### 2. Editing Highlighting

This is the local editing state layer.

- recent edits
- current selection
- diff ranges
- unsaved changes

Purpose:
- help the user understand active local change and edit recency

### 3. Cowork Highlighting

This is the collaboration-state layer that makes `gdedit` distinct.

- agent focus
- proposed patch ranges
- review-needed regions
- impact ranges
- conflict-risk regions
- locked regions
- approval-wait regions
- human-led vs agent-proposed emphasis

Purpose:
- expose the shared state between human and edit agent directly on the editing surface

## Required Core States

Phase 3 must define stable meaning for at least these states:

### Human-Edited

- indicates recent human modification
- helps the agent and user see where direct intent has already been expressed

### Agent-Proposed

- indicates a patch or candidate change proposed by the edit agent
- should remain visibly distinct from already-applied edits

### Review-Needed

- indicates a proposal or area that still requires human attention
- should stay visible until resolved

### Impact-Range

- indicates nearby or related regions likely affected by a proposal or change
- helps prevent local intent from hiding non-local consequences

### Locked

- indicates regions that the agent must not modify under current constraints
- should read as a hard boundary, not a soft suggestion

### Approved

- indicates reviewed and accepted proposal state
- helps distinguish pending vs resolved collaboration state

## Additional Useful States

- agent-focus
- conflict-risk
- approval-wait
- selected-target scope
- diff-approved / diff-rejected

These may be layered later, but they should follow the same vocabulary contract.

## State Semantics Rules

- states must be readable at a glance
- states must carry meaning, not decoration
- the same state should mean the same thing everywhere
- cowork highlighting must not be confused with syntax highlighting

## Visual Priority

When layers overlap, the product should prioritize meaning over decoration.

Recommended semantic priority:

1. locked / denied
2. review-needed / proposal state
3. impact-range / conflict-risk
4. recent human edit
5. baseline syntax and symbol coloring

This keeps safety and collaboration visible before aesthetics.

## Default Marker Direction

The exact palette may evolve later, but the model should reserve distinct visual channels for:

- positive / accepted state
- pending / review state
- warning / impact state
- protected / locked state
- human-local edit state
- agent-proposed state

Possible channels include:

- background tint
- underline style
- gutter marker
- border accent
- inline marker

The important requirement is semantic distinction, not a specific color theme.

## Reduced-Color and Low-Capability Fallbacks

Highlighting must still work when color is limited.

Fallback strategies:

- pair color with marker shape or text symbol
- use gutter tokens for state emphasis
- reserve one strong marker for locked state
- keep review-needed and agent-proposed visually separable even without rich color

The model must never rely on truecolor alone to express safety-critical meaning.

## Relationship to Other Systems

### Relationship to Guarded Editing

- highlighting shows patch bounds, impact, and locked areas
- it makes guarded editing visible rather than implicit

### Relationship to Control Handoff

- highlighting exposes who initiated the current edit path and what still needs review

### Relationship to Status Surface

- the status surface summarizes collaboration state
- highlighting shows where that state lives on the editing surface

## Anti-Patterns

Highlighting is broken when:

- proposal state looks the same as applied state
- locked regions do not stand out clearly
- impact ranges are hidden
- syntax color overwhelms collaboration meaning
- the same marker means different things in different contexts

## Phase 3 Acceptance

Phase 3 highlighting documentation is complete when it clearly defines:

- the three-layer model
- required collaboration states
- semantic priority between overlapping states
- fallback behavior for reduced-color terminals
- the relationship between highlighting, handoff, and guarded editing
