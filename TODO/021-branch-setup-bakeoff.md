# TODO 021 - Branch setup bakeoff (spec-first, decision-first)

Context: We need to simplify branch setup behavior so it matches the
many-to-many mob-consensus model: each collaborator pushes only their own
branch to their own remote, and fetches from peers. Before changing code, we
will run a spec-first bakeoff.

## Decision Intent Log

ID: DI-021-20260326-205942
Date: 2026-03-26 20:59:42
Status: active
Decision: Run a two-phase branch-setup bakeoff (happy paths first, then
failure paths) and block implementation until a winner is locked.
Intent: Reduce churn and rework by selecting one coherent branch setup model
before porting stash changes.
Constraints: No implementation prototypes during bakeoff; compare only
decision-level command contracts and expected outcomes.
Affects: `TODO/021-branch-setup-bakeoff.md`,
`TODO/020-recover-jj-main-into-stevegt-main.md`, `TODO/TODO.md`.

## Candidates

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

## Plan

- [ ] 021.1 Write explicit command contracts for A/B/C/D.
- [ ] 021.2 Build side-by-side comparison matrix for all criteria.
- [ ] 021.3 Run happy-path bakeoff on all candidates.
- [ ] 021.4 Select provisional winner and runner-up.
- [ ] 021.5 Run required failure-path bakeoff on winner vs runner-up:
  - [ ] 021.5.1 Missing branch linkage.
  - [ ] 021.5.2 Wrong remote target.
  - [ ] 021.5.3 No write permission.
  - [ ] 021.5.4 Detached HEAD.
  - [ ] 021.5.5 Remote branch-name conflict.
  - [ ] 021.5.6 Multi-remote ambiguity.
- [ ] 021.6 Publish final winning model with rationale and rejected
      alternatives.
- [ ] 021.7 Convert winning model into implementation DI locks for TODO 020.

## Gate

No further TODO 020 code reconciliation work starts until 021.6 is complete.
