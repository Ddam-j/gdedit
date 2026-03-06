# UI Architecture

## Purpose

This document defines the Phase 1 UI structure for `gdedit`. It freezes the layout direction and screen-role boundaries before richer TUI implementation begins.

## Core Layout Principle

`gdedit` has one representative control hub and multiple edit contexts.

- control is centralized
- editing contexts are separated
- status remains visible
- optional supporting panels stay secondary

## Canonical Layout

Primary layout direction:

- top: tab bar
- center: active edit surface
- bottom-left: control input hub
- bottom-right: status surface
- optional later panels: diff, agent result, references, highlight detail

## Canonical Desktop / Wide Terminal Sketch

```text
+--------------------------------------------------------------+
| Tabs: [main.go] [doctor-report] [patch-review] [notes]       |
+--------------------------------------------------------------+
|                                                              |
|                      Active Edit Surface                     |
|                                                              |
|         current file / buffer / task context lives here      |
|                                                              |
+-----------------------------------+--------------------------+
| Control Hub                       | Status Surface           |
| > inspect recent edit             | tab: main.go             |
|   current scope: selection        | scope: selection         |
|   preview: pending                | agent: idle              |
|                                   | review: 1 pending        |
+-----------------------------------+--------------------------+
```

## Compact Terminal Sketch

```text
+--------------------------------------------------+
| Tabs: [main.go] [review] [notes]                 |
+--------------------------------------------------+
|                                                  |
|                Active Edit Surface               |
|                                                  |
+-----------------------------+--------------------+
| Control Hub                 | Status             |
| > rename this function      | tab: main.go       |
|                             | agent: suggest     |
+-----------------------------+--------------------+
```

## Region Roles

### Tab Bar

- Shows multiple editing contexts.
- Represents task or work context, not a second control layer.
- Keeps context switching visible and lightweight.

### Active Edit Surface

- Holds the current editable document context.
- Remains the primary focus during normal editing.
- Must not be overloaded with control-only UI.

### Control Hub

- The single place for control input.
- Accepts typed commands and voice-injected text.
- Supports preview, confirmation, and agent invocation.
- Operates against the active tab and its current local context.

### Status Surface

- Displays compact, high-value state.
- Must behave like a dashboard, not a log dump.
- Should summarize current context, agent status, review state, and target scope.

### Optional Secondary Panels

- Diff view
- agent result details
- reference/inspection details
- highlight detail or impact explanation

These are not part of the primary Phase 1 center of gravity. They may exist later, but they must not compete with the single control hub.

## Fixed in v1

- exactly one primary control hub
- multiple edit contexts via tabs
- one active edit surface at a time
- a persistent status surface
- active-tab context determines control target
- bottom split with input on the left and state on the right

## Not Fixed Yet

- exact height/width ratios of the bottom region
- whether optional secondary panels live to the side or in temporary overlays
- detailed keybindings for navigation and layout changes
- final rendering details for low-width terminals

## Context Routing Rule

The control hub is shared, but it is not context-free.

Every control action should resolve against:

- active tab
- current cursor
- current selection
- current project/workspace state

This lets users learn one control hub without sacrificing context precision.

## Layout Philosophy

- The user edits in the center.
- The user issues intent from the lower-left.
- The system answers with compact state on the lower-right.
- The center remains the editing plane; the bottom remains the control-and-state plane.

## Design Constraints

- Do not introduce multiple competing control hubs.
- Do not let the status surface turn into a scrolling log or chat transcript.
- Do not hide the active editing context behind side-panel complexity.
- Do not require optional side panels for core editing and control flow.

## Follow-On Documents

- `docs/control-panel.md` defines control-hub behavior.
- `docs/tab-model.md` defines tab semantics.
- `docs/status-language.md` will define stable status vocabulary.
