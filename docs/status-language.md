# Status Language

## Purpose

This document defines the stable language and content model for the `gdedit` status surface. The status surface is the compact dashboard for current editing, agent, and scope state.

## Core Definition

- The status surface is not just a traditional status bar.
- It is a compact situation summary panel.
- It should show the most important current state without turning into a log or chat window.

## Status Surface Role

The status surface exists to answer, at a glance:

- where am I working?
- what scope is active?
- what is the agent doing?
- what scoped command is active?
- is voice active?
- is the current scope clear?

## Primary Content Categories

### Context Identity

- active tab name
- current project name
- current selection range or current target scope

### Collaboration State

- agent state
- current preview or last scoped command
- highlight summary

### Edit State

- unsaved change presence
- current target range
- current editable scope indication

### Voice / Control State

- control hub active or not
- voice waiting / listening / complete

## Recommended First-Class Items

The `dev-guide` strongly suggests these as the initial status set:

- active tab
- current project
- current selection or target scope
- agent state
- current preview or last scoped command
- dirty/changed indicator
- voice input state
- highlight state summary

## Stable Labels

The exact wording may evolve, but the semantics should stay stable.

Recommended compact labels:

- `tab:` active editing context
- `project:` current project identity
- `scope:` current cursor/selection/target scope
- `agent:` idle / scoping / preview
- `command:` current preview or last confirmed scoped command
- `voice:` off / ready / listening / captured
- `changes:` clean / dirty
- `highlight:` summary of current visual collaboration state

## Compact Mode Behavior

When horizontal space is limited:

- prioritize active tab
- prioritize scope
- prioritize agent state
- collapse secondary fields into short forms
- hide explanation text before hiding core state

Compact mode should preserve state meaning, not just truncate randomly.

## Dashboard, Not Log

This is the most important rule for the status surface:

- it must remain short
- it must remain current
- it must remain relevant to active editing

It must not become:

- a scrolling log
- a chat transcript
- a dump of long explanations

If longer explanation is needed, it belongs in a separate panel or temporary detail view.

## Status Semantics Rules

- status should reflect the present state, not historical noise
- status should prefer summary over narration
- status should use consistent vocabulary across tabs and flows
- status should align with highlighting and scope states

## Relationship to the Control Hub

- the control hub is where the user expresses intent
- the status surface is where the system reflects compact state back

A good mental model is:

- left: I speak
- right: the editor answers

## Relationship to Highlighting

The status surface should summarize what highlighting already shows spatially.

Examples:

- `highlight: selection`
- `highlight: caret+scope`
- `highlight: human-edit`

The status surface should not replace the editing-surface markers; it should help users interpret them quickly.

## Relationship to Control Handoff

Status language must make handoff visible.

Examples:

- `agent: scoping`
- `agent: preview`
- `command: edit current scope`
- `scope: selection`

This helps users understand who currently leads and what stage the edit is in.

## Non-Goals

- no verbose log dumping in the status surface
- no duplicating the full control panel in the status surface
- no unstable vocabulary that changes meaning by context
- no hiding current scope or command state when space gets tight

## Phase 3 Acceptance

Phase 3 status-language documentation is complete when it clearly defines:

- what the status surface always shows
- a stable compact vocabulary
- dashboard-not-log behavior
- compact-mode prioritization
- the relationship between status, highlighting, and scope
