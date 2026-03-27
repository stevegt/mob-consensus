# TODO 022 - Evaluate `git consensus` subcommand architecture

Context: we are considering exposing mob-consensus as a Git subcommand (for
example, `git consensus ...`) while preserving the current workflow model and
algorithm.

## Decision Intent Log

ID: DI-022-20260326-225113
Date: 2026-03-26 22:51:13
Status: active
Decision: Create an architecture-only memo (no implementation) evaluating
`git consensus` as a first-class interface candidate.
Intent: Decide whether Git-subcommand UX improves adoption and clarity without
adding avoidable complexity or behavior drift.
Constraints: Same binary and same algorithm/codepath as `mob-consensus`; no
code changes in this TODO; focus on design tradeoffs and rollout shape.
Affects: `TODO/022-git-consensus-subcommand.md`, `TODO/TODO.md`.

## Scope

- Analyze `git consensus` as the primary command name candidate.
- Assume one binary with dual entry names (`mob-consensus`,
  `git-consensus`) and identical behavior.
- Compare architecture/UX tradeoffs only (not implementation steps).

## Pros and cons to analyze

- [ ] 022.1 UX and discoverability
  - [ ] 022.1.1 Pros: Git-native invocation, easier mental model in Git-heavy teams.
  - [ ] 022.1.2 Cons: transition confusion if both names coexist without clear policy.
- [ ] 022.2 Packaging and installation
  - [ ] 022.2.1 Pros: possible zero-drift dual-name model from one binary.
  - [ ] 022.2.2 Cons: extra install/symlink/PATH requirements for `git-consensus`.
- [ ] 022.3 Operational impact
  - [ ] 022.3.1 Docs/help churn (`README`, `usage.tmpl`, examples).
  - [ ] 022.3.2 Test and harness churn (`go test` expectations, `scripts/mc-test` command calls).
- [ ] 022.4 Compatibility strategy options
  - [ ] 022.4.1 Dual-entry steady state.
  - [ ] 022.4.2 Eventual hard switch.
  - [ ] 022.4.3 Keep subcommand as optional alias.

## Deliverable

- [ ] 022.5 Publish a recommendation paragraph with rationale and risks:
  - preferred interface policy (`git consensus` primary vs dual-entry),
  - migration posture (if any),
  - explicit non-goals for the next implementation step.
