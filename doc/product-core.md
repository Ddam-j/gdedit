# Delivery Plan: Multi-platform Support and Release Operations

## Scope Note

This document is a historical delivery and release-planning document.

- it does not define the current product identity of `gdedit`
- the live product philosophy is editing-first assistance around the active text surface
- durable memos, file intent, and scoped edit-agent interaction now define the assistant direction more than generic project operations language

When this document conflicts with newer product docs, prefer:

- `docs/product-vision.md`
- `docs/control-panel.md`
- `docs/edit-agent.md`
- `docs/product-core-map.md`

## TL;DR
> **Summary**: Define a portability contract and set up repeatable CI/release automation so `gdedit` can ship on Windows (PowerShell terminals), Linux, and macOS (incl. SSH usage) with public-commit hygiene.
> **Deliverables**: Terminal compatibility contract + `--doctor`, CI validate workflow (3 OS), GoReleaser-based release workflow, GitHub Releases assets (archives + checksums + SBOM), Homebrew tap + Scoop bucket (manual update flow by default).
> **Effort**: Medium
> **Parallel**: YES - 3 waves
> **Critical Path**: Terminal compatibility contract → CI validate → GoReleaser snapshot → Release publishing → Package managers

## Context
### Original Request
- "멀티 플랫폼을 통해서 Windows의 파워셀, Linux, mac의 SSH 터미널용으로 구성할 수 있는지 확인" + "프로젝트를 어떻게 운용할지 논의".

### Interview Summary
- Platform baseline: Modern only (Windows 10/11 + Windows Terminal/WezTerm; macOS 12+; Linux SSH terminals with xterm-256color/truecolor).
- Panel strategy: In-app panels (consistent TUI across terminals/SSH).
- Distribution: GitHub Releases + package managers.
- Constraint: GitHub connected; all commits are public.

### Research Findings
- `go.mod` exists (`module gdedit`, `go 1.25`) but there are no `*.go` files yet.
- No `.github/workflows`, no `Makefile`/task runner, no GoReleaser config, no tests yet.
- Product direction lives in `dev-guide/2026-03-06_18-07-11_ChatGPT_1._터미널 에디터 기획(gdedit).md` and commit rules in `dev-guide/COWORK_COMMIT_GUIDE.md`.

### Metis Review (gaps addressed)
- Added guardrails for scope creep (codesigning/notarization deferred; limit pkg managers).
- Added explicit decisions/risks: Go version baseline, release topology for Homebrew/Scoop, CGO policy, terminal contract (tmux/TERM/truecolor), and non-TTY behavior.

## Work Objectives
### Core Objective
- Make multi-platform support a *contract* (documented + testable) and make releases *repeatable* (CI + automation) without relying on emulator-specific behavior.

### Key Decisions (locked for execution)
- **Go baseline**: `go 1.24` in `go.mod`; CI uses `actions/setup-go` with `go-version-file: go.mod` (avoids pinning a specific patch here).
- **Terminal/TUI stack**: Use `tcell` as the low-level terminal I/O + rendering foundation (portable, editor-friendly, good Windows story).
- **CGO policy**: `CGO_ENABLED=0` for all release builds in v0 ops baseline.
- **Terminal contract**: "Modern terminal" required; legacy conhost / dumb terminals are best-effort and may refuse to start.
- **Panels**: In-app panels only; no WezTerm-pane automation in v0.
- **Package managers**: Homebrew (macOS) + Scoop (Windows) supported via *manual manifest updates* initially; automate later when publishing permissions are clear.
- **Supply chain**: SBOM is REQUIRED for release assets; provenance/signing is DEFERRED (no codesigning/notarization in v0).

### Deliverables
- Terminal compatibility contract (document) + minimal automated verification.
- CI Validate workflow for Windows/macOS/Linux.
- Release workflow using GoReleaser with snapshot and tagged release modes.
- Release artifacts: per-target archives + checksums (+ SBOM if enabled).
- Package manager lane: Homebrew + Scoop (automated or documented manual update).

### Definition of Done (verifiable)
- `gh workflow list` shows Validate + Release workflows.
- Validate workflow runs on PR/push and completes green for all OS.
- GoReleaser snapshot build runs in CI and produces expected cross-platform artifacts.
- Tagged release produces GitHub Release assets with checksums (and SBOM if required).
- Homebrew + Scoop installation flows are executable and verify `gdedit --version` output (once a runnable binary exists).

### Must Have
- Public-commit hygiene: no secrets; add guardrails so accidental commits are caught early.
- Portability-first: avoid WezTerm-only dependencies; in-app panels must render in generic terminals.
- Release pipeline produces deterministic artifacts (pinned toolchain policy).

### Must NOT Have
- No early codesigning/notarization/AuthentiCode in this milestone.
- No additional pkg managers beyond Homebrew + Scoop in this milestone.
- No CGO requirements in v0 operations baseline unless explicitly approved.

## Verification Strategy
> ZERO HUMAN INTERVENTION — all verification is agent-executed.
- Test decision: tests-after (start with `go test ./...` + `go vet ./...` even before substantial code exists).
- CI policy: Validate (3 OS) always required before merge.
- Evidence policy: store command outputs as `.sisyphus/evidence/task-{N}-{slug}.txt`.

## Execution Strategy
### Parallel Execution Waves
Wave 1: Contract + repo hygiene foundations (docs/guardrails/toolchain decisions)
Wave 2: CI Validate + GoReleaser snapshot
Wave 3: Tagged Release + Homebrew/Scoop integration

### Dependency Matrix (full, all tasks)
- 1 blocks 3,5,9 (contract drives doctor wording + feature gates + TUI smoke scope)
- 2 blocks 6,7,8 (toolchain pinned before CI/release)
- 3 blocks 6,7,8,10 (needs runnable binary for CI)
- 6+7 block 8 (release job depends on validate baseline + goreleaser)
- 8 blocks 11 (must have actual release assets before pkg manifests)
- 11 blocks 12,13 (must have tap/bucket before install verification)

### Agent Dispatch Summary
- Wave 1: 5 tasks (writing + repo ops)
- Wave 2: 5 tasks (CI + release automation)
- Wave 3: 3 tasks (pkg managers + verification)

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task includes agent-executed QA scenarios.

- [ ] 1. Publish Terminal Compatibility Contract (baseline + degradations)

  **What to do**: Create `docs/terminal-compat.md` defining: required capabilities (ANSI/VT, alt-screen, cursor addressing, UTF-8), best-effort capabilities (truecolor, mouse, bracketed paste), supported terminals (Windows Terminal/WezTerm/VSCode; iTerm2/Terminal.app; xterm-compatible over SSH; tmux), and non-goals (legacy/dumb terminals). Include a small manual test matrix list.
  **Must NOT do**: Do not promise pixel-perfect behavior across all terminals; explicitly document degradations.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: contract doc must be crisp and unambiguous
  - Skills: []
  - Omitted: [`playwright`] — no browser interaction required

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 2,3,4,6 | Blocked By: none

  **References**:
  - Product direction: `dev-guide/2026-03-06_18-07-11_ChatGPT_1._터미널 에디터 기획(gdedit).md` — portability posture, panels, mouse policy
  - README: `README.md` — WezTerm recommended but not required

  **Acceptance Criteria** (agent-executable only):
  - [ ] `docs/terminal-compat.md` exists and includes: Required vs Best-effort table; supported terminal list; tmux/ssh notes; explicit out-of-scope list.

  **QA Scenarios**:
  ```
  Scenario: Contract completeness check
    Tool: Bash
    Steps: Open and scan `docs/terminal-compat.md` for required sections.
    Expected: All required sections present with explicit language (no TBD).
    Evidence: .sisyphus/evidence/task-1-terminal-compat-doc.txt

  Scenario: Anti-scope check
    Tool: Bash
    Steps: Search for "supports all terminals" / "perfect" wording.
    Expected: No over-promising language remains.
    Evidence: .sisyphus/evidence/task-1-terminal-compat-anti-scope.txt
  ```

  **Commit**: YES | Message: `docs(terminal): define compatibility contract` | Files: `docs/terminal-compat.md`

- [ ] 2. Lock Go Toolchain Baseline (public, reproducible)

  **What to do**: Update `go.mod` from `go 1.25` to `go 1.24` (baseline) and document that CI uses `actions/setup-go` with `go-version-file: go.mod`.
  **Must NOT do**: Do not rely on unreleased Go versions.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: small, surgical repo config change
  - Skills: []
  - Omitted: [`git-master`] — simple change; optional

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 6,7,8 | Blocked By: 1

  **References**:
  - Current module file: `go.mod`

  **Acceptance Criteria**:
  - [ ] `go.mod` uses `go 1.24`.
  - [ ] `docs/dev-setup.md` (if created/updated here) states Go 1.24.x as the baseline.

  **QA Scenarios**:
  ```
  Scenario: Local toolchain sanity
    Tool: Bash
    Steps: Run `go env GOVERSION`.
    Expected: Reports Go 1.24.x when using the documented toolchain.
    Evidence: .sisyphus/evidence/task-2-go-toolchain.txt

  Scenario: go.mod baseline check
    Tool: Bash
    Steps: Read `go.mod`.
    Expected: Contains `go 1.24`.
    Evidence: .sisyphus/evidence/task-2-go-mod.txt
  ```

  **Commit**: YES | Message: `build(go): baseline on go1.24` | Files: `go.mod`, `docs/dev-setup.md`

- [ ] 3. Add Minimal Runnable CLI Skeleton (`--version`, `--doctor`)

  **What to do**: Create a minimal entrypoint `cmd/gdedit/main.go` plus a version package (e.g., `internal/version`). Implement:
  - `gdedit --version` prints semver if embedded via ldflags (fallback `dev`).
  - `gdedit --doctor` prints terminal/environment signals: TTY detection, `TERM`, `COLORTERM`, `NO_COLOR`, `SSH_CONNECTION`, and a short interpretation aligned with `docs/terminal-compat.md`.
  Keep output stable for CI verification.
  **Must NOT do**: Do not implement editor UI yet; no TUI library required in this task.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: establishes initial executable contract cleanly
  - Skills: []
  - Omitted: [`playwright`] — CLI only

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 6,7,8,9,11 | Blocked By: 1

  **References**:
  - Product direction: `dev-guide/2026-03-06_18-07-11_ChatGPT_1._터미널 에디터 기획(gdedit).md` — `--doctor` aligns with "state visible" philosophy

  **Acceptance Criteria**:
  - [ ] `go run ./cmd/gdedit --version` exits 0 and prints a single line.
  - [ ] `go run ./cmd/gdedit --doctor` exits 0 and prints a readable capability report.

  **QA Scenarios**:
  ```
  Scenario: Version output
    Tool: Bash
    Steps: Run `go run ./cmd/gdedit --version`.
    Expected: Exits 0; output contains either `dev` or a semver-like token.
    Evidence: .sisyphus/evidence/task-3-version.txt

  Scenario: Doctor output in non-SSH
    Tool: Bash
    Steps: Run `go run ./cmd/gdedit --doctor` locally.
    Expected: Exits 0; prints TERM and TTY status.
    Evidence: .sisyphus/evidence/task-3-doctor.txt
  ```

  **Commit**: YES | Message: `feat(cli): add --version and --doctor` | Files: `cmd/gdedit/main.go`, `internal/version/*`

- [ ] 4. Add Public Repo Hygiene Docs (no secrets, commit/PR flow)

  **What to do**: Add `docs/dev-setup.md` and `docs/contributing.md` capturing:
  - Public commits: secrets policy + examples of forbidden files
  - Conventional Commit format pointing to `dev-guide/COWORK_COMMIT_GUIDE.md`
  - Local verification commands (`go test ./...`, `go vet ./...`, `gdedit --doctor`)
  **Must NOT do**: Do not add heavy contributor bureaucracy; keep it short and actionable.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: policy clarity
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 6 | Blocked By: none

  **References**:
  - Commit guide: `dev-guide/COWORK_COMMIT_GUIDE.md`

  **Acceptance Criteria**:
  - [ ] `docs/dev-setup.md` and `docs/contributing.md` exist.
  - [ ] Both explicitly mention that all commits are public and forbid committing secrets.

  **QA Scenarios**:
  ```
  Scenario: Docs presence
    Tool: Bash
    Steps: Verify files exist and include "public" + "secrets" + commit format.
    Expected: Clear, short instructions present.
    Evidence: .sisyphus/evidence/task-4-docs.txt

  Scenario: Commit guide linkage
    Tool: Bash
    Steps: Check docs link or reference `dev-guide/COWORK_COMMIT_GUIDE.md`.
    Expected: Reference present.
    Evidence: .sisyphus/evidence/task-4-commit-guide-link.txt
  ```

  **Commit**: YES | Message: `docs(contrib): add public repo hygiene and verification steps` | Files: `docs/dev-setup.md`, `docs/contributing.md`

- [ ] 5. Decide and Document Terminal Feature Gates (future-proof)

  **What to do**: Create `docs/terminal-features.md` listing feature gates and their detection sources (env vars, `--doctor` checks). Include at least: truecolor, mouse, bracketed paste, alt-screen, tmux.
  **Must NOT do**: Do not bind gates to WezTerm-only features.

  **Recommended Agent Profile**:
  - Category: `writing` — Reason: drives future implementation decisions
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: 9,10 | Blocked By: 1,3

  **References**:
  - Oracle findings (captured in plan): modern terminal contract and feature-gating rationale

  **Acceptance Criteria**:
  - [ ] `docs/terminal-features.md` exists with a table of gates + detection + default behavior.

  **QA Scenarios**:
  ```
  Scenario: Feature gate table check
    Tool: Bash
    Steps: Open `docs/terminal-features.md`.
    Expected: Each gate has detection and fallback behavior described.
    Evidence: .sisyphus/evidence/task-5-feature-gates.txt
  ```

  **Commit**: YES | Message: `docs(terminal): define feature gates and fallbacks` | Files: `docs/terminal-features.md`

- [ ] 6. Add GitHub Actions Validate Workflow (win/mac/linux)

  **What to do**: Add `.github/workflows/validate.yml` triggered on `push` and `pull_request`.
  - Matrix: `windows-latest`, `ubuntu-latest`, `macos-latest`
  - Steps: checkout, setup-go (`go-version-file: go.mod`), `go test ./...`, `go vet ./...`, `go build ./cmd/gdedit`
  **Must NOT do**: Do not require secrets; keep permissions minimal.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: straightforward workflow wiring
  - Skills: []
  - Omitted: [`git-master`] — optional

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 7,8 | Blocked By: 2,3

  **References**:
  - Go baseline: `go.mod`

  **Acceptance Criteria**:
  - [ ] Validate workflow runs successfully on all 3 OS.
  - [ ] Validate includes `go test`, `go vet`, and `go build`.
  - [ ] Validate pins the same Go 1.24.x baseline as `go.mod`.

  **QA Scenarios**:
  ```
  Scenario: Local workflow lint
    Tool: Bash
    Steps: Review `.github/workflows/validate.yml` for matrix + go commands.
    Expected: Matrix contains 3 OS; commands present.
    Evidence: .sisyphus/evidence/task-6-validate-yml.txt

  Scenario: CI run
    Tool: Bash
    Steps: Push branch and view GitHub Actions status.
    Expected: All jobs green.
    Evidence: .sisyphus/evidence/task-6-actions-green.txt
  ```

  **Commit**: YES | Message: `ci(validate): add go test/vet/build matrix` | Files: `.github/workflows/validate.yml`

- [ ] 7. Add GoReleaser Config (snapshot + release) with checksums + SBOM

  **What to do**: Add `.goreleaser.yaml` configured to:
  - Build targets: windows/amd64, darwin/amd64+arm64, linux/amd64+arm64
  - `CGO_ENABLED=0`
  - Archive: Windows `.zip`, others `.tar.gz`
  - Archive naming: `gdedit_<version>_<os>_<arch>` (stable for Homebrew/Scoop)
  - Generate checksums (sha256) with name template: `{{ .ProjectName }}_{{ .Version }}_checksums.txt`
  - Generate SBOMs via GoReleaser `sboms` for archive artifacts (requires `syft` in PATH)
  - Inject version via `ldflags` into `internal/version`.
  **Must NOT do**: Do not add signing/notarization.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: release automation correctness matters
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 8,9 | Blocked By: 2,3,6

  **References**:
  - Go module: `go.mod`

  **Acceptance Criteria**:
  - [ ] `goreleaser check` passes locally.
  - [ ] `goreleaser build --snapshot --clean` produces artifacts for all targets.
  - [ ] `gdedit_<version>_checksums.txt` exists in dist output and validates.
  - [ ] Artifact names follow the stable template (no random suffixes).

  **QA Scenarios**:
  ```
  Scenario: Snapshot build
    Tool: Bash
    Steps: Run `goreleaser check` and `goreleaser release --snapshot --clean`.
    Expected: Exit 0; `dist/` contains per-target archives + checksums file.
    Evidence: .sisyphus/evidence/task-7-goreleaser-snapshot.txt

  Scenario: SBOM presence
    Tool: Bash
    Steps: List `dist/` for SBOM artifacts.
    Expected: SBOM files present per archive (as configured).
    Evidence: .sisyphus/evidence/task-7-sbom.txt
  ```

  **Commit**: YES | Message: `build(release): add goreleaser config with checksums and sbom` | Files: `.goreleaser.yaml`

- [ ] 8. Add GitHub Actions Release Workflow (tag → GitHub Release)

  **What to do**: Add `.github/workflows/release.yml` with two triggers:
  - `workflow_dispatch`: runs GoReleaser in `--snapshot` mode and uploads `dist/` as a workflow artifact (no publish)
  - `push` tags `v*`: runs GoReleaser publish to GitHub Releases
  Keep permissions minimal; no secrets; omit `id-token` (provenance deferred).
  Add a step before GoReleaser to install Syft for SBOM generation: `uses: anchore/sbom-action/download-syft@v0`.
  **Must NOT do**: Do not attempt Homebrew/Scoop publishing in this workflow yet.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: wiring around a known tool
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 11 | Blocked By: 6,7

  **References**:
  - Release automation: `.goreleaser.yaml`

  **Acceptance Criteria**:
  - [ ] `workflow_dispatch` path produces a downloadable `dist/` artifact.
  - [ ] Tag push path is wired for publishing (no manual steps besides creating the tag).

  **QA Scenarios**:
  ```
  Scenario: Snapshot workflow run
    Tool: Bash
    Steps: Trigger `workflow_dispatch` for Release workflow; download artifacts.
    Expected: `dist/` artifact contains archives + `gdedit_<version>_checksums.txt` (+ SBOM).
    Evidence: .sisyphus/evidence/task-8-snapshot-artifact.txt

  Scenario: Tag release asset listing
    Tool: Bash
    Steps: Create tag `v0.0.1` and run `gh release view v0.0.1 --json assets --jq '.assets[].name'`.
    Expected: Lists archives + checksums (+ SBOM).
    Evidence: .sisyphus/evidence/task-8-release-assets.txt
  ```

  **Commit**: YES | Message: `ci(release): publish goreleaser on tags` | Files: `.github/workflows/release.yml`

- [ ] 9. Introduce TUI Dependency (tcell) + No-op Screen Proof

  **What to do**: Add `tcell` dependency and a minimal runnable stub that enters/exits alt-screen cleanly (e.g., `gdedit --tui-smoke`) to validate portability early.
  **Must NOT do**: Do not build editor features; keep it a smoke test only.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: terminal I/O correctness across OS
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: 10 | Blocked By: 2,6

  **References**:
  - Oracle: recommends `tcell` for editor-grade control and Windows story
  - Contract: `docs/terminal-compat.md` (created in Task 1)

  **Acceptance Criteria**:
  - [ ] `go run ./cmd/gdedit --tui-smoke` enters alt-screen, draws a frame, exits on `q`.
  - [ ] Works on Windows Terminal, macOS Terminal/iTerm2, and Linux terminal.

  **QA Scenarios**:
  ```
  Scenario: Local interactive smoke (Windows)
    Tool: interactive_bash
    Steps: Run `go run ./cmd/gdedit --tui-smoke` in Windows Terminal; press `q`.
    Expected: Screen draws; exits cleanly; terminal restored.
    Evidence: .sisyphus/evidence/task-9-tui-smoke-windows.txt

  Scenario: Local interactive smoke (SSH)
    Tool: interactive_bash
    Steps: SSH into a Linux host; run `gdedit --tui-smoke`; press `q`.
    Expected: Works with acceptable latency; exits cleanly.
    Evidence: .sisyphus/evidence/task-9-tui-smoke-ssh.txt
  ```

  **Commit**: YES | Message: `feat(tui): add tcell smoke mode` | Files: `cmd/gdedit/main.go`, `go.mod`, `go.sum`, `internal/tui/*`

- [ ] 10. CI Terminal Contract Tests (non-interactive)

  **What to do**: Add unit tests that validate the `--doctor` interpretation logic (pure functions). Avoid interactive PTY tests initially.
  **Must NOT do**: Do not attempt flaky PTY integration tests in CI yet.

  **Recommended Agent Profile**:
  - Category: `quick` — Reason: pure unit tests
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: none | Blocked By: 3,5,9

  **References**:
  - Doctor implementation: `cmd/gdedit/main.go` (from Task 3)

  **Acceptance Criteria**:
  - [ ] `go test ./...` passes on all OS in Validate workflow.

  **QA Scenarios**:
  ```
  Scenario: Unit test run
    Tool: Bash
    Steps: Run `go test ./...`.
    Expected: Exit 0.
    Evidence: .sisyphus/evidence/task-10-tests.txt
  ```

  **Commit**: YES | Message: `test(cli): cover doctor interpretation logic` | Files: `**/*_test.go`

- [ ] 11. Package Manager Manifests (Homebrew + Scoop) + Manual Update Runbook

  **What to do**: Create a manual packaging lane (no secrets) using separate public repos:
  - Homebrew tap: `Ddam-j/homebrew-gdedit`
    - Formula path: `Formula/gdedit.rb`
    - Formula points to GitHub Release assets from `Ddam-j/gdedit` using the stable archive naming from Task 7
  - Scoop bucket: `Ddam-j/scoop-gdedit`
    - Manifest: `gdedit.json`
  Steps (explicit):
  1) `gh repo create Ddam-j/homebrew-gdedit --public --confirm`
  2) `gh repo create Ddam-j/scoop-gdedit --public --confirm`
  3) Clone each repo locally, add the formula/manifest + minimal README, commit, push.
  Also add `docs/packaging.md` in this repo documenting the exact update steps:
  1) Create tag release `vX.Y.Z`
  2) Fetch release asset URLs + sha256 from `gdedit_<version>_checksums.txt`
  3) Update formula + manifest
  4) Verify install (Task 13)
  Default is manual updates; no Actions pushing to tap/bucket.
  **Must NOT do**: Do not introduce PAT secrets for auto-publishing in v0.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: packaging correctness + reproducibility
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: 12,13 | Blocked By: 8

  **References**:
  - Release assets: GitHub Release from Task 8

  **Acceptance Criteria**:
  - [ ] `docs/packaging.md` exists with exact commands.
  - [ ] `Ddam-j/homebrew-gdedit` exists and contains `Formula/gdedit.rb` referencing GitHub Release URLs + sha256.
  - [ ] `Ddam-j/scoop-gdedit` exists and contains `gdedit.json` referencing GitHub Release URLs + hash.

  **QA Scenarios**:
  ```
  Scenario: Create tap/bucket repos
    Tool: Bash
    Steps: Use `gh repo create` to create `Ddam-j/homebrew-gdedit` and `Ddam-j/scoop-gdedit` (public) if missing.
    Expected: Repos exist and are reachable via `gh repo view`.
    Evidence: .sisyphus/evidence/task-11-repos.txt

  Scenario: Runbook walkthrough
    Tool: Bash
    Steps: Follow `docs/packaging.md` steps using an existing GitHub Release tag.
    Expected: Formula/manifest can be updated without undefined placeholders.
    Evidence: .sisyphus/evidence/task-11-runbook.txt
  ```

  **Commit**: YES | Message: `docs(packaging): add manual brew/scoop lane runbook` | Files: `docs/packaging.md`

- [ ] 12. Add Package Install Verify Workflow (brew + scoop)

  **What to do**: Add `.github/workflows/package-verify.yml` (trigger: `workflow_dispatch`) that:
  - On `macos-latest`: taps `Ddam-j/homebrew-gdedit`, installs `gdedit`, runs `gdedit --version`
  - On `windows-latest`: installs Scoop (if missing), adds bucket `Ddam-j/scoop-gdedit`, installs `gdedit`, runs `gdedit --version`
  Accept an input `version` and assert output contains it.
  **Must NOT do**: Do not require secrets; do not add notarization.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: macOS packaging verification
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: none | Blocked By: 11

  **References**:
  - Runbook: `docs/packaging.md`

  **Acceptance Criteria**:
  - [ ] `package-verify` workflow exists and is runnable via `workflow_dispatch`.
  - [ ] Workflow asserts installed `gdedit --version` contains the provided version.

  **QA Scenarios**:
  ```
  Scenario: Workflow definition check
    Tool: Bash
    Steps: Inspect `.github/workflows/package-verify.yml`.
    Expected: Has macOS + Windows jobs and asserts `--version` contains input.
    Evidence: .sisyphus/evidence/task-12-package-verify-yml.txt
  ```

  **Commit**: YES | Message: `ci(packaging): add brew/scoop install verification workflow` | Files: `.github/workflows/package-verify.yml`

- [ ] 13. Run Package Install Verify Workflow (post-release)

  **What to do**: After a tag release + tap/bucket update, trigger `package-verify` workflow with the released version and capture evidence.
  **Must NOT do**: Do not require admin privileges.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` — Reason: Windows packaging verification
  - Skills: []

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: none | Blocked By: 11

  **References**:
  - Runbook: `docs/packaging.md`

  **Acceptance Criteria**:
  - [ ] `package-verify` workflow run is green on macOS + Windows.

  **QA Scenarios**:
  ```
  Scenario: Trigger verify workflow
    Tool: Bash
    Steps: Run `gh workflow run package-verify.yml -f version=vX.Y.Z`; then `gh run list --workflow package-verify.yml --limit 1`; then `gh run watch --exit-status <run-id>`.
    Expected: Workflow completes successfully on both OS.
    Evidence: .sisyphus/evidence/task-13-package-verify-run.txt
  ```

  **Commit**: NO | Message: `n/a` | Files: `n/a`

## Final Verification Wave (4 parallel agents, ALL must APPROVE)
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Commit message format: `type(scope): summary` per `dev-guide/COWORK_COMMIT_GUIDE.md`.
- Keep commits atomic: CI/workflows, release config, docs/contract each separate.
- No secrets committed; OIDC/provenance is deferred (keep workflows minimal-permissions).

## Success Criteria
- The project can be operated with a predictable PR→CI→Release loop.
- Multi-platform support is defined and verifiable (not aspirational).
- Release assets are reproducible and do not depend on WezTerm-specific features.
