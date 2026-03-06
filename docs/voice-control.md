# Voice Control

## Purpose

This document defines the role of voice in `gdedit`. Voice is a secondary control interface for conveying intent and control, not a primary replacement for keyboard editing.

## Core Definition

- Voice is used only in control mode.
- Voice is an input channel for the control hub.
- Voice exists to convey editing intent, scope, review requests, and control transitions.

## Why Voice Exists

- `gdedit` is a cowork editing environment, not a chat-first AI tool.
- Voice fits best when it helps the user express short intent against visible edit context.
- Voice becomes stronger when it combines with cursor, selection, highlighting, and current scope.

## Voice Is Not The Main Interface

Voice is intentionally secondary.

- the core editing flow is still visual, local, keyboard-first editing
- mouse remains limited and supporting
- voice should not replace direct code editing as the default path

If voice becomes the main interface, the product becomes noisier and less controlled.

## Control-Mode-Only Rule

Voice is available only while the control hub is active.

That means:

- no ambient always-listening behavior
- no voice capture during normal editing state
- no separate voice world outside the control hub

This keeps intent explicit and reduces accidental interpretation.

## Voice Flow

The canonical flow is:

1. user edits normally
2. user focuses the control hub
3. user speaks a short instruction
4. transcript appears in the control hub
5. user verifies or edits the text
6. `gdedit` resolves scope from current context
7. user confirms execution or review path
8. focus may return to the edit surface

## Why This Flow Matters

- it keeps voice reviewable
- it makes misrecognition recoverable
- it keeps text and voice on the same command path
- it makes control mode feel like focusing the control hub, not entering a separate world

## Scope Resolution

Voice commands are interpreted through the current editing context.

Priority should follow the control model:

1. current selection
2. current cursor
3. current block / function / file when inferable
4. preview before apply when ambiguity remains

This is why short phrases like “simplify here” can become meaningful in `gdedit`.

## Recommended First Command Classes

Phase 4 should keep the first voice actions constrained.

### Inspection

- inspect recent change
- explain this function
- check the part I just edited

### Scope-Limited Proposal

- refactor only this selection
- rename this symbol
- simplify this block

### Review / Approval

- show diff only
- hold this proposal
- approve this proposal

### Navigation / Control

- switch tab
- move to control target
- change target scope

## Priority of Voice Roles

### Primary Role

Voice is best as a command-and-intent channel.

- call the edit agent
- specify scope
- request inspection
- request or hold proposal
- approve or defer
- move tabs or panels

### Secondary Role

Voice may help with light drafting.

- note a TODO
- draft a short explanation
- draft a comment or commit message idea

### Tertiary Role

Direct code dictation is possible later, but should remain low priority.

## Command Set Strategy

The early voice system should start with a limited action set rather than fully open-ended interpretation.

Good initial categories:

- explain
- inspect
- highlight
- propose
- hold
- approve
- move
- switch

This reduces interpretation drift and keeps the system teachable.

## Privacy and Processing Assumptions

The product direction strongly favors local processing where possible.

Why:

- spoken code context and project details should leak less often
- local processing fits the broader local-LLM direction
- responsiveness improves when voice stays close to the editor

## Non-Goals

- no ambient voice listening
- no always-on microphone workflow
- no freeform dictation-first editing as the primary model
- no bypass of control-hub preview and confirmation

## Design Rules

- voice must reinforce control, not blur it
- voice must stay tied to visible context
- voice should express intent more than literal code text
- voice should remain compatible with review-first editing

## Phase 4 Acceptance

Phase 4 voice-control documentation is complete when it clearly defines:

- voice as control-mode-only input
- voice-to-control-hub flow
- constrained early command classes
- local/privacy-oriented processing assumptions
- why voice is secondary to direct editing rather than a replacement for it
