# TODO 021 - Branch setup bakeoff (spec-first, decision-first)

Context: We need to simplify branch setup behavior so it matches the
many-to-many mob-consensus model: each collaborator pushes only their own
branch to their own remote, and fetches from peers. Before changing code, we
will run a spec-first bakeoff.

## Decision Intent Log

ID: DI-021-20260326-205942
Date: 2026-03-26 20:59:42
Status: superseded
Decision: Run a two-phase branch-setup bakeoff (happy paths first, then
failure paths) and block implementation until a winner is locked.
Intent: Reduce churn and rework by selecting one coherent branch setup model
before porting stash changes.
Constraints: No implementation prototypes during bakeoff; compare only
decision-level command contracts and expected outcomes.
Affects: `TODO/021-branch-setup-bakeoff.md`,
`TODO/020-recover-jj-main-into-stevegt-main.md`, `TODO/TODO.md`.

ID: DI-021-20260330-235100
Date: 2026-03-30 23:51:00
Status: active
Decision: Use a stabilization-first implementation plan: never push peer
branches, warn on risky local peer branches, and checkpoint this behavior
before the LLM/DF refactor.
Intent: Finish reconciliation safely with clear push ownership rules while
avoiding another long churn loop in push/branch semantics.
Constraints: No auto-creation of local peer branches; keep peer integration
based on remote-tracking refs; run peer hygiene checks in `init`, `start`,
`join`, `status`, and `branch sync` (skip `merge`); include exact git stderr in
guided failures; preserve existing command hierarchy.
Affects: `main.go`, `cli.go`, `main_integration_test.go`, `scripts/mc-test`,
`README.md`, `TODO/020-recover-jj-main-into-stevegt-main.md`,
`TODO/012-llm-assisted-review-and-approval.md`, `TODO/TODO.md`.
Supersedes: DI-021-20260326-205942

## Archived candidates

- A) Current `branch create` flow.
- B) `branch from <twig>` convenience flow.
- C) Predeclared twig mapping + user apply command.
- D) Current `start/join` onboarding baseline.

## Evaluation criteria (priority order)

1. Simplicity of user workflow.
2. Correctness/safety of branch linkage and push behavior.
3. Intent preservation at branch creation time.
4. Clarity of user-facing errors (plain English, low jargon).
5. Testability and deterministic behavior.

## Locked stabilization decisions

- Never push peer-owned branches from `mob-consensus`.
- Warn if a local peer branch has local modifications.
- Warn if a local peer branch is likely unpushed:
  - if it has upstream and is ahead, warn;
  - if it has no upstream, assume unpushed and warn.
- Do not auto-create local peer branches.
- If a local peer branch exists, enforce no-push with marker
  `branch.<peer>.pushRemote=__mob_consensus_no_push__`.
- In interactive mode, prompt before applying marker per branch.
- In non-interactive mode, apply marker automatically.

## Archived bakeoff checklist (superseded by DI-021-20260330-235100)

- [x] 021.1 Write explicit command contracts for A/B/C/D (superseded).
- [x] 021.2 Build side-by-side comparison matrix for all criteria (superseded).
- [x] 021.3 Run happy-path bakeoff on all candidates (superseded).
- [x] 021.4 Select provisional winner and runner-up (superseded).
- [x] 021.5 Run required failure-path bakeoff on winner vs runner-up (superseded).
  - [x] 021.5.1 Missing branch linkage (superseded).
  - [x] 021.5.2 Wrong remote target (superseded).
  - [x] 021.5.3 No write permission (superseded).
  - [x] 021.5.4 Detached HEAD (superseded).
  - [x] 021.5.5 Remote branch-name conflict (superseded).
  - [x] 021.5.6 Multi-remote ambiguity (superseded).
- [x] 021.6 Publish final winning model with rationale and rejected alternatives (superseded).
- [x] 021.7 Convert winning model into implementation DI locks for TODO 020 (superseded).

## Implementation plan (active)

- [ ] 021.8 Add strict no-peer-push guard in push paths.
- [ ] 021.9 Add local peer-branch hygiene scanner:
  - [ ] 021.9.1 Detect peer locals by `<owner>/<twig>` where `owner != current user`.
  - [ ] 021.9.2 Check and warn on dirty peer branch state.
  - [ ] 021.9.3 Check and warn on ahead/no-upstream peer branch state.
- [ ] 021.10 Wire checks into command scope:
  - [ ] 021.10.1 `init`
  - [ ] 021.10.2 `start`
  - [ ] 021.10.3 `join`
  - [ ] 021.10.4 `status`
  - [ ] 021.10.5 `branch sync`
- [ ] 021.11 Implement poisoning behavior for existing local peer branches:
  - [ ] 021.11.1 Interactive prompt per branch.
  - [ ] 021.11.2 Non-interactive auto-poison.
- [ ] 021.12 Update tests (`go test` and `scripts/mc-test`) for the new guardrails.
- [ ] 021.13 Update docs/help text with the stabilized push-ownership model.
- [ ] 021.14 Cut a stabilization commit, then hand off to TODO 012 AI/DF refactor.

## Gate

TODO 020 reconciliation may continue only after 021.13 is complete and reviewed.
