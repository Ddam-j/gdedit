# Control Handoff

## Purpose

This document defines how a human and an edit agent share, transfer, and recover control in `gdedit`.

## Core Definition

- `gdedit` is neither fully manual nor fully automatic.
- Its value comes from visible, explicit, semi-automatic collaboration.
- Control handoff means the current editing initiative can move between human and edit agent in a bounded, understandable way.

## Why Handoff Exists

If control stays fixed to one side, the product fails in one of two ways:

- human-only: slow, repetitive, and weak at broad impact tracing
- agent-only: hallucination risk, over-modification, weak reading of local human intent

The product works by combining human local judgment with agent structural analysis.

## Handoff Principle

Control must be shared, but the sharing must be explicit.

- the user should understand who currently leads
- the user should understand what scope is active
- the user should understand whether the current step is suggestive, reviewable, or applying

## Recommended State Model

Phase 2 should keep the handoff model small and visible.

### Human-Led

- the user edits directly
- the agent may observe or assist through later review requests
- the human currently owns the editing initiative

### Agent-Suggesting

- the user has asked the agent to inspect, propose, or analyze
- the agent returns suggestions or patch candidates
- the agent does not yet own final application authority

### Review-Pending

- a patch or suggested change is available for human review
- the user can accept, reject, or refine the proposal
- this is the default safety buffer for meaningful agent edits

### Applying

- the selected proposal is being applied to the allowed scope
- application should remain bounded and visible

### Locked / Denied

- the requested or proposed change conflicts with a lock, forbidden range, or explicit constraint
- the system must expose that denial rather than silently widening scope

## Transition Table

| From | Trigger | To |
|------|---------|----|
| Human-Led | ask for inspection or patch | Agent-Suggesting |
| Agent-Suggesting | proposal ready | Review-Pending |
| Review-Pending | user approves | Applying |
| Review-Pending | user rejects or cancels | Human-Led |
| Any | target hits locked scope | Locked / Denied |
| Applying | patch completes | Human-Led |

## Canonical Collaboration Patterns

These are the intended rhythms of work:

- human-led edit -> agent impact analysis
- agent proposal -> human approval
- human direct patch -> agent coherence check
- agent draft -> human fine adjustment
- human review of agent patch before merge into active state

## Explicitness Rules

The handoff model must not be hidden behind vague AI activity.

- current initiative should be visible in status language
- current target scope should be visible
- proposal vs applied state should be visible
- denied or locked actions should be visible

## Authority Model

In early `gdedit`, suggestion authority comes before automatic edit authority.

- the agent may propose first
- the human decides acceptance for meaningful edits
- the system should optimize for collaborative rhythm before automation power

## Relationship to Review

Review is not a bolt-on stage. It is part of the handoff contract.

- suggestion -> review -> apply is the normal path
- the review layer reduces confusion when control shifts from human to agent and back
- review protects local human intent from broad, opaque rewrite behavior

## Relationship to Locked Regions

Locked regions are not an edge case. They are a core handoff boundary.

- the human may reserve parts of the editing surface from agent modification
- the system may deny or constrain proposals that cross protected boundaries
- denied state must remain visible rather than silently falling back to broader behavior

## Failure Modes to Avoid

- unclear initiative ownership
- silent agent takeover
- reviewless patch application for non-trivial changes
- hidden scope widening
- multi-context edits without explicit transition

## State Machine Guidance

The actual implementation may later refine internal names, but the user-facing semantics should remain close to this model:

- human-led
- agent-suggesting
- review-pending
- applying
- locked/denied

If more states are added later, they should improve clarity rather than increase mode burden.

## Phase 2 Acceptance

Phase 2 control-handoff documentation is complete when it clearly defines:

- why shared control exists
- the visible handoff states
- how proposals move into review and then apply
- how denial and locking work
- why suggestion authority comes before automatic edit authority
