# Tab Model

## Purpose

This document defines what tabs mean in `gdedit` and how they relate to buffers, sessions, task context, and shared control.

## Core Definition

- A tab in `gdedit` represents an editing context.
- Tabs separate work context.
- The control hub remains shared across tabs.

In short:

- tab = work context
- control hub = common control entry

## Why Tabs Exist

- Code work naturally shifts across files, tasks, and comparison targets.
- A user needs multiple live contexts without fragmenting control.
- Tabs keep editing contexts separate while preserving one stable control model.

## Tab vs Buffer vs Session

### Tab

- a visible work context
- what the user actively switches between
- may reflect one file, one review target, one comparison target, or one focused task thread

### Buffer

- the underlying editable content representation
- a tab may show one primary buffer
- future designs may allow multiple views onto related content, but Phase 1 assumes one main buffer context per tab

### Session

- the broader workspace state that contains many tabs, selection states, and collaboration history
- sessions outlive a single tab switch

## What a Tab Carries

Each tab should carry enough local context for control and agent actions.

Minimum conceptual payload:

- active buffer/file identity
- local cursor and selection state
- local recent edit context
- local review/proposal context
- local collaboration highlights relevant to that tab

## Shared vs Local State

### Shared Across Tabs

- one control hub
- one product control philosophy
- one workspace/project identity
- global workstyle rules

### Local To Each Tab

- editing content
- cursor/selection
- local review state
- local proposal state
- local visual context

## Control Routing Rule

The control hub is global, but commands resolve through the active tab.

That means control should follow:

- active tab
- current cursor in that tab
- current selection in that tab
- current project state

The user learns one control system, while the active tab supplies the local target context.

## Tab Lifecycle

### Open

- create a new editing context for a file, task, comparison, or review target

### Activate

- move visible editing focus to that tab
- make that tab the target context for control commands

### Update

- maintain local cursor, selection, recent edits, and pending review state

### Close

- remove the visible editing context while preserving broader session rules
- if the closed tab has unsaved or unreviewed state, later implementation must define guardrails

## Naming Guidance

Tab names should help users reason about context quickly.

Prefer names that reflect:

- file identity
- task identity
- review identity
- comparison identity

Avoid vague generic names when a clearer local purpose exists.

## Cross-Tab Agent Behavior

Default rule:

- the edit agent acts on the active tab unless the user explicitly broadens scope

Implications:

- an agent request should not silently jump to another tab
- a control action should not silently affect multiple tabs by default
- cross-tab or multi-context actions must be made explicit and reviewable

## Tabs and Panels Are Not The Same

`gdedit` should keep a clear distinction:

- tabs = work contexts
- panels = supporting views or supporting surfaces

Examples:

- tab: `main.go`
- tab: `review patch`
- panel later: diff detail
- panel later: reference detail

This distinction helps avoid turning the UI into a confusing multi-pane IDE before the core model stabilizes.

## Phase 1 Constraints

- tabs are first-class and central
- tabs do not become separate control hubs
- tab switching must preserve clear control targeting
- tab behavior should stay lightweight and context-focused

## Non-Goals

- tabs are not mini workspaces with their own separate command systems
- tabs are not a replacement for all future supporting panels
- tabs are not hidden session containers the user cannot reason about

## Phase 1 Acceptance

Phase 1 tab-model documentation is complete when it clearly defines:

- tab as editing context
- tab vs buffer vs session
- shared control hub with active-tab routing
- local vs shared state
- cross-tab agent behavior defaults
