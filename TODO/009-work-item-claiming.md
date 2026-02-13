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

Constraints:
- This repo is public; do not require exposing real identities. Support
  an explicit `--who <label>` (e.g., `alice`, `team-a`, `anon1`).
- Do not assume remote name `origin`; require explicit remote selection
  when ambiguous (reuse the existing remote-selection logic).
- Must work with fork-based collaboration (see TODO 008).

## Terminology: “remote refs” vs local refs

Git refs always live *somewhere*:
- **Remote refs** live in a *remote repository* (server-side). You only
  see them after you `git fetch` that remote.
- **Remote-tracking refs** live in your local clone under
  `refs/remotes/<remote>/...`; they are your local cached view of a
  remote’s refs.

So “represent claims as remote refs” should be read as:
- store claim state as refs **pushed to a repo that collaborators can
  fetch**, and
- collaborators run `git fetch` (or `git fetch --all`) to update their
  local remote-tracking view of those claim refs.

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

Represent “claiming item X” as the existence of a ref **pushed to a
remote** in a reserved namespace (so other collaborators can see it
after `git fetch`).

### Where do the claim refs live?

Two viable models:

1) **Single coordination remote** (exclusive claims possible)
- Everyone can push to the same remote that holds the claim refs.
- This could be the shared repo (when access is shared), or a small
  dedicated bare repo used only for coordination.
- Pros: “exclusive claim” can be enforced in one place.
- Cons: requires shared write access somewhere.

2) **Per-fork claims** (distributed; exclusivity is advisory)
- Each collaborator pushes their claim ref(s) to their own fork.
- Everyone adds each other’s forks as remotes and fetches them (TODO 008).
- Pros: works even when upstream is read-only.
- Cons: cannot strongly enforce exclusivity; only detect collisions
  (multiple claim refs exist for the same item across remotes).

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

In fork workflows, Option B maps naturally to “push to your fork; others
fetch your fork” because each claimant controls their own namespace.

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

## How collaborators see each other’s claims (fork workflow)

Conceptual example:
- Alice pushes a claim ref to her fork (which Bob has added as remote `alice`).
- Bob runs `git fetch alice` (or `git fetch --all`) and sees the claim as a
  remote-tracking ref like `remotes/alice/claims/<item>/alice`.
- `mob-consensus claims` lists claim refs across the configured collaborator remotes
  (TODO 008 “repo-tracked collaborator configuration”).

Note: if we ever use a non-branch namespace (anything outside `refs/heads/*`),
we’ll need an explicit fetch refspec; using `refs/heads/claims/*` keeps it simple.

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
