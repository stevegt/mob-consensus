# TODO 020 - Recover `jj/main` work and reconcile onto `stevegt/main`

Context: We accidentally continued implementation on `jj/main` while a prior
stash from `main` still exists. We need a durable recovery flow that preserves
all work, does not disturb `jj/main`, and lands only decision-approved deltas
onto `stevegt/main`.

## Decision Intent Log

ID: DI-020-20260325-220434
Date: 2026-03-25 22:04:34
Status: active
Decision: Snapshot current `jj/main` worktree/index to `recover/jj-main-wip`,
keep `stash@{0}` untouched, and reconcile from a fresh
`recover/stevegt-main-reconcile` branch with decision-first approvals.
Intent: Preserve all in-flight work without rewriting or renaming `jj/main`,
while preventing accidental direct landing of unresolved deltas to
`stevegt/main`.
Constraints: Do not rename/move/delete `jj/main`; do not apply or drop
`stash@{0}` during bootstrap; use explicit file adds (no `git add .`).
Affects: `recover/jj-main-wip` branch, `recover/stevegt-main-reconcile` branch,
`TODO/020-recover-jj-main-into-stevegt-main.md`, `TODO/TODO.md`.

ID: DI-020-20260326-205942
Date: 2026-03-26 20:59:42
Status: superseded
Decision: Pause stash reconciliation implementation until TODO 021 branch-setup
bakeoff completes and a winning command model is locked.
Intent: Avoid repeated churn in `main.go`, `cli.go`, tests, and docs by
finalizing branch-link/push behavior first.
Constraints: Implementation remains blocked until TODO 021.6 is complete;
decision-first intake remains required before any resumed edits.
Affects: `TODO/020-recover-jj-main-into-stevegt-main.md`,
`TODO/021-branch-setup-bakeoff.md`, TODO 020 implementation sequencing.

ID: DI-020-20260330-235100
Date: 2026-03-30 23:51:00
Status: active
Decision: Run a narrow stabilization checkpoint from TODO 021 (never push peer
branches + peer branch hygiene warnings) before resuming the rest of stash/JJ
reconciliation and then immediately pivot to TODO 012 AI/DF refactor planning.
Intent: Preserve current WIP on a stable, reviewable baseline and avoid losing
work while still moving quickly toward AI-assisted merge/review workflows.
Constraints: Stabilization scope is limited to push ownership and local
peer-branch warnings; no broad CLI redesign in this checkpoint.
Affects: `TODO/020-recover-jj-main-into-stevegt-main.md`,
`TODO/021-branch-setup-bakeoff.md`, `TODO/012-llm-assisted-review-and-approval.md`.
Supersedes: DI-020-20260326-205942

ID: DI-020-20260406-193000
Date: 2026-04-06 19:30:00
Status: active
Decision: Begin reconciliation by porting the staged stash slice for `main.go`, `main_integration_test.go`, `scripts/mc-test`, and `usage.tmpl` onto `recover/stevegt-main-reconcile` while keeping `stash@{0}` untouched.
Intent: Recover high-impact push-guidance and system-test changes without disturbing `jj/main`, then continue with incremental DI-reviewed slices.
Constraints: Treat stash as read-only source; do not drop/apply stash; keep reconciliation branch isolated from `stevegt/main` until validated.
Affects: `main.go`, `main_integration_test.go`, `scripts/mc-test`, `usage.tmpl`, `TODO/020-recover-jj-main-into-stevegt-main.md`

- [ ] 020.1 Capture immutable recovery artifacts
  - [x] 020.1.1 Verify branch and stash baseline:
    - `git rev-parse --abbrev-ref HEAD`
    - `git status --short --branch`
    - `git stash list --max-count=1`
    - `git rev-parse --verify refs/heads/jj/main`
    - `git rev-parse --verify refs/heads/stevegt/main`
  - [x] 020.1.2 Create snapshot branch and commit exact current worktree state:
    - `git switch -c recover/jj-main-wip`
    - `git add AGENTS.md TODO/001-rewrite-mob-consensus-in-go.md TODO/008-support-fork-remotes.md TODO/010-extend-mc-test.md TODO/018-detached-head-and-push-permissions.md TODO/TODO.md main.go main_integration_test.go scripts/mc-test usage.tmpl`
    - `git commit -F -` (snapshot message with per-file bullets)
  - [x] 020.1.3 Record immutable recovery references:
    - `git rev-parse --verify recover/jj-main-wip`
    - `git rev-parse --verify refs/heads/jj/main`
    - `git rev-parse --verify stash@{0}`

- [ ] 020.2 Prepare clean target branch from `stevegt/main`
  - [x] 020.2.1 Create reconcile branch:
    - `git switch stevegt/main`
    - `git switch -c recover/stevegt-main-reconcile`
  - [ ] 020.2.2 Verify clean baseline before applying any recovered deltas:
    - `git status --short --branch`
    - `git log --oneline --decorate -n 5`

- [ ] 020.3 Run decision-first intake per target file before edits
  - [ ] 020.3.1 For each target file, lock architecture/behavior/implementation
        choices as DI entries before editing:
        - `main.go`
        - `main_integration_test.go`
        - `scripts/mc-test`
        - `usage.tmpl`
        - `AGENTS.md` and affected `TODO/*.md`
  - [ ] 020.3.2 Lock naming decisions (function/variable names) before code
        changes.
  - [ ] 020.3.3 Lock runtime path decisions for test/harness touches.

- [ ] 020.4 Reconcile source deltas onto `recover/stevegt-main-reconcile`
  - [x] 020.4.1 Use `recover/jj-main-wip` and `stash@{0}` as read-only sources.
  - [ ] 020.4.2 Port only DI-approved chunks; do not batch-apply everything.
  - [ ] 020.4.3 Keep behavior comments and add `Intent` + `Source: DI-...`
        markers for non-trivial behavior changes.

- [ ] 020.5 Validate and prepare handoff
  - [ ] 020.5.1 Verify no conflict markers remain:
    - `rg -n '^(<<<<<<<|=======|>>>>>>>)'`
  - [ ] 020.5.2 Verify branch safety invariants:
    - `git rev-parse --verify refs/heads/jj/main`
    - `git rev-parse --verify stash@{0}`
  - [ ] 020.5.3 Run Go tests and ask user to run `scripts/mc-test all`.
  - [ ] 020.5.4 Provide Decision Compliance and comment/provenance audits.

- [ ] 020.6 Branch-setup bakeoff gate (TODO 021)
  - [x] 020.6.1 Pause implementation while bakeoff spec is prepared.
  - [x] 020.6.2 Resume gate replaced by stabilization checkpoint (DI-020-20260330-235100).
  - [ ] 020.6.3 Resume reconciliation only after TODO 021.13 stabilization
        tasks are complete and reviewed.
  - [ ] 020.6.4 After stabilization commit, open/execute the TODO 012 AI/DF
        refactor intake before additional branch behavior changes.
