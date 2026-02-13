# TODO 009 - Use mob-consensus for work-item claiming

Problem: some teams track work items as repo-local Markdown TODO docs
(often with short numeric IDs). Two recurring pain points:

1) Assignment/volunteerism tends to be **merge-hostile** when it’s
   recorded in shared files (e.g., a single TODO index).
2) File-based assignment state still depends on everyone pushing/pulling
   frequently; if someone forgets, the “who is working on what” view is
   wrong.

Goal: extend `mob-consensus` so it can act as an external, Git-native
claiming tool. Claim state should be:
- visible after `git fetch`,
- merge-conflict-free (no repo file edits required),
- safe under concurrency (clear failure modes; no silent stealing).

## Prior art (common patterns) and why they hurt

1) `Owner:` fields inside each TODO file
- Pros: ownership “lives with the work item”; changes touch one file.
- Cons: still depends on push/pull discipline; can conflict if multiple
  people edit the same TODO file.

2) Per-person “working set” files (e.g., `TODO/WORKING-alice.md`)
- Pros: each person edits their own file; low merge conflicts.
- Cons: still depends on push/pull discipline; easy to forget.

3) Branch-name signaling (e.g., `user/<who>/todo-<id>-<slug>`)
- Pros: no repo file churn; naturally reflects active work.
- Cons: only works if branches are pushed; hard to discover consistently
  without a tool that queries remotes.

4) External issue tracker for assignments
- Pros: good UI; avoids Git merge conflicts entirely.
- Cons: “two sources of truth”; may not be acceptable for some teams.

## Proposal: represent claims as remote refs (no file edits)

Represent “claiming item X” as the existence of a remote ref in a
reserved namespace.

Two semantics to support (pick one for MVP):

### Option A: exclusive claim (one claimant at a time)
- Ref: `refs/heads/claims/<item>`
- Claim: create the ref; fail if it already exists.
- Unclaim: delete the ref.
- Reassign: delete then re-create (or a `--force` “steal”).

### Option B: non-exclusive claim (multiple claimants allowed)
- Ref: `refs/heads/claims/<item>/<who>`
- Claim: create the per-claimer ref; no contention.
- Unclaim: delete your own ref.
- Reassign: add/remove claimers by adding/removing refs.

Implementation detail (state payload):
- MVP: the ref can point at the claimant’s current `HEAD`.
- If we need better “claim age” signaling, introduce a dedicated
  “claim marker” commit or tag later.

## CLI sketch

Keep the current `mob-consensus` flows intact; add a small “claim” API:

- `mob-consensus claim <item> [--remote <r>] [--who <label>]`
- `mob-consensus unclaim <item> [--remote <r>] [--who <label>]`
- `mob-consensus claims [--remote <r>] [--item <item>]`

Expected behavior:
- always `git fetch` before evaluating claim state
- return a clear error when a claim already exists (exclusive mode)
- print exact remediation steps (who claimed it; how to unclaim/steal)

## Stalled claims / reassignment

New problem: if a work item is claimed but work stalls, the claim can
block progress until it is removed or reassigned.

Design ideas (MVP-friendly):
- show “staleness” as the age of the commit the claim ref points to
  (good enough if claim refs are renewed when work continues)
- add `mob-consensus claim renew <item>` to fast-forward/update the claim
  ref (and make “keeping it fresh” a habit like pushing branches)
- add `mob-consensus claim steal <item> --force` with an explicit
  confirmation prompt, and require `--reason` text for auditability

## Extra merge-conflict reduction ideas for Markdown TODO systems

Even with claiming solved, new work items can still collide in shared
indexes:
- Keep an append-only “Inbox / Untriaged” section at the bottom of the
  TODO index so concurrent additions don’t conflict as often.
- If we add a `newtodo` helper, have it insert into the Inbox (triage
  later).

## Subtasks

- [ ] 009.1 Decide exclusive vs non-exclusive semantics for MVP.
- [ ] 009.2 Decide the reserved ref namespace and naming conventions.
- [ ] 009.3 Implement `claims` listing (remote discovery + formatting).
- [ ] 009.4 Implement `claim` creation with safe failure modes.
- [ ] 009.5 Implement `unclaim` deletion with safe failure modes.
- [ ] 009.6 Add stalled-claim UX (age display; optional `renew`/`steal`).
- [ ] 009.7 Add tests (unit tests for parsing/formatting; integration tests optional).
- [ ] 009.8 Document usage in `README.md`.
