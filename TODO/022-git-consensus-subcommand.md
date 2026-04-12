# TODO 022 - Define `git consensus sync` workflow (LLM-assisted, DF/DI/DR-gated)

Context: this TODO now defines a concrete `git consensus sync` workflow, not
an abstract naming memo. This workflow is Git-centered convergence: use
`git consensus sync`
as an orchestrated operation that combines LLM-assisted analysis with required
human review and mandatory DF/DI/DR gates.

This TODO remains planning/spec-only. It does not commit the project to
session/relay runtime features in this repo.

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

ID: DI-022-20260406-191500  
Date: 2026-04-06 19:15:00  
Status: active  
Decision: Align `git consensus` evaluation criteria with peer-to-peer/no-PR
philosophy and DF/DI/DR facilitation boundaries.  
Intent: Ensure command naming/interface debate does not drift into a PR-centric
workflow model or protocol-engine scope.  
Constraints: Documentation/planning-only change; keep evaluation
implementation-neutral.  
Affects: `TODO/022-git-consensus-subcommand.md`,
`TODO/023-mob-consensus-philosophy.md`.

ID: DI-022-20260412-120000  
Date: 2026-04-12 12:00:00  
Status: active  
Decision: Define `git consensus sync` as an explicit orchestrated command,
`git consensus sync`, with mandatory DF/DI/DR merge gates, built-in side-by-side
diff review, LLM-assisted context/commit drafting, and push-to-own-branch
safety invariants.  
Intent: Remove roadmap ambiguity so this workflow can be estimated, tested, and
executed before any session/relay architecture decisions.  
Constraints: Planning/documentation only in this TODO update; no runtime code
changes; no PR-based workflow assumptions; human approval remains required.  
Affects: `TODO/022-git-consensus-subcommand.md`.  
Supersedes: DI-022-20260326-225113, DI-022-20260406-191500

## Workflow definition (locked)

This is a Git-convergence workflow with this primary command:

- `git consensus sync [PEER_REF ...]`

This workflow is not a generic chat/session runtime. It is a safer,
context-aware merge operation that:

- keeps convergence in Git history,
- enforces peer-to-peer ownership boundaries,
- records decision provenance (DF/DI/DR),
- uses LLM assistance for analysis and commit drafting, with human approval.

## Command contract

### Primary behavior

`git consensus sync` performs an orchestrated flow:

1. Prepare local state for sync:
   - evaluate dirty tree state;
   - if configured/approved, create a local LLM-assisted pre-sync commit.
2. Fetch peers needed for this sync.
3. Build comparison sets for each requested peer (or discovered related peers).
4. Run LLM-assisted analysis of peer-vs-local diffs.
5. Enter mandatory DF gate:
   - unresolved decisions become DR items;
   - approved decisions become/extend DI items;
   - unresolved DR blocks merge completion.
6. Present built-in colorized side-by-side diffs for required comparisons:
   - peer branch vs current local branch;
   - previous local commit vs new generated merge changes.
7. Require explicit human approval before integration.
8. Merge approved peer changes onto local branch.
9. Add merge attribution:
   - merged peer branch(es) in commit header/body;
   - `Co-authored-by` trailers derived from merged history.
10. Draft commit message with LLM assistance (editable by user).
11. Push only to user-owned branch/remote according to safety invariants.

### Safety invariants

- Never push peer-owned branches.
- Never auto-resolve ambiguous push targets.
- If ownership/remote intent is unclear, stop with explicit guidance.
- Human approval is required before final integration commit/push.

## Required DF/DI/DR behavior

- DF is mandatory in every `sync` run that proposes integration.
- DR is required for unresolved/contested design decisions.
- DI is required for accepted non-trivial decisions affecting behavior.
- Sync may continue only when blocking DR items are resolved or explicitly
  deferred with user approval.

## LLM role boundaries

LLM is allowed to:

- summarize/compare changes,
- explain probable impact and context,
- propose merge plan ordering,
- draft commit message text.

LLM is not allowed to:

- bypass approval gates,
- silently finalize merge/push decisions,
- replace required DF/DI/DR records.

## Use cases (normative examples)

### Use case 1: shared-write team repo

1. User runs `git consensus sync bob/twig carol/twig`.
2. Tool fetches peers, analyzes diffs, opens DF gate.
3. User reviews side-by-side diffs and resolves DR items.
4. Tool merges approved peer branches, records attribution, drafts commit.
5. User approves and pushes own branch.

### Use case 2: forked multi-remote collaboration

1. User runs `git consensus sync bob/twig`.
2. Tool fetches collaborator remotes, compares peer vs local.
3. DF/DI/DR gate captures decisions and unresolved issues.
4. User approves integration via built-in side-by-side diff views.
5. Tool pushes only to the user’s own remote/branch, never to peer remotes.

Both use cases require LLM-assisted analysis + human review + DF/DI/DR gates.

## Failure modes and required handling

- Dirty tree without approved pre-sync commit policy → stop with guidance.
- Detached HEAD in integration path → stop with concrete recovery commands.
- Ambiguous peer refs across remotes → stop and list exact candidates.
- Push permission failure → show exact stderr + safe remediation.
- Blocking DR unresolved → do not finalize merge commit.

## Explicit non-goals

- No requirement to introduce relay/session networking runtime here.
- No requirement to introduce TUI/session shell architecture here.
- No PromiseGrid node-mode commitment in this TODO.

Those belong to separate decisions and TODO tracks.

## Acceptance criteria for workflow readiness

The workflow is ready to execute when:

1. The `git consensus sync` contract is fully documented in user/help docs.
2. Tests cover shared-write and forked workflows with DF/DI/DR gates.
3. Built-in side-by-side diff requirements are specified and testable.
4. Push ownership safety invariants are enforced and documented.
5. Recommendation and rollout notes are explicit and implementation-ready.

## Implementation checklist

- [ ] 022.1 Publish user-facing workflow contract for `git consensus sync`.
- [ ] 022.2 Define exact DF/DI/DR artifacts required per sync run.
- [ ] 022.3 Define built-in side-by-side diff UX contract and approval rules.
- [ ] 022.4 Define attribution format (peer branch headers + co-author trailers).
- [ ] 022.5 Define hard-fail behavior for ambiguous/unsafe push and unresolved DR.
- [ ] 022.6 Add shared-write and forked end-to-end workflow examples to docs/help.
- [ ] 022.7 Add acceptance-test matrix for `git consensus sync` command behavior.
- [ ] 022.8 Publish recommendation paragraph and migration posture:
  - primary command naming: `git consensus sync`,
  - compatibility/rollout notes,
  - explicit non-goals for this phase.
