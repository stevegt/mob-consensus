# TODO 008 - Support fork-based collaboration (multi-remote)

Context: `mob-consensus` was originally used where collaborators all
have write access to a single shared remote. We also need to support
the case where each collaborator only has write access to **their own
fork**, and other collaborators can only fetch from it.

Goal: make discovery/merge/push workflows work predictably with multiple
remotes (ex: `origin` = your fork, `upstream` = canonical, plus `jj`,
`bob`, etc).

## Can mob-consensus do this already?

Partially:
- Merge mode can merge from any ref that `git merge` can see, including
  remote-tracking refs like `jj/alice/feature-x`, *if the remote has been
  fetched*.
- Discovery lists local + remote-tracking branches from `git branch -a`,
  so it can show peers across multiple remotes *once those remotes are
  present and up to date locally*.

Gaps:
- The tool currently runs `git fetch` with no remote specified; this does
  not necessarily update all remotes in a multi-fork setup.
- Auto-push behavior can fail if the current branch’s upstream remote is
  not writable (common when tracking a shared twig on someone else’s
  fork, or tracking `upstream/*`).
  - XXX we should only ever push to our own fork
- Help/examples currently assume a single “chosen remote” for peer refs;
  in fork workflows the peer remote varies by collaborator.

## Proposed behavior (high level)

- Separate concerns:
  - **Fetch remotes**: which remotes to update for discovery/merge inputs.
  - **Push remote**: where *your* branch should be pushed (your fork).
  - **Twig remote** (optional): where the shared twig lives (may be a
    designated collaborator’s fork if `upstream` is read-only).
    - XXX we should only ever push to our own fork

- Prefer deterministic behavior and clear errors over guessing.

## Subtasks

- [ ] 008.1 Define CLI/config knobs (keep defaults safe).
  - [ ] 008.1.1 Add `--fetch <remote>` (repeatable) and/or `--fetch-all`
    (`git fetch --all`) for multi-remote discovery/merge.
  - [ ] 008.1.2 Add `--push-remote <remote>` (or support using
    `git config remote.pushDefault`) for upstreamless pushes.
    - XXX we should only ever push to our own fork
  - [ ] 008.1.3 (Optional) Add `--twig-remote <remote>` for onboarding:
    where the shared `<twig>` branch is created/pushed/fetched.
- [ ] 008.2 Update fetch logic.
  - [ ] 008.2.1 Default behavior: fetch the current branch upstream
    remote if set; otherwise error if the remote choice is ambiguous.
    - XXX no, this is not how the mob-consensus algo should work - we
      should only ever fetch from the remotes of our collaborators,
      not a generic upstream
  - [ ] 008.2.2 If `--fetch-all`, fetch all remotes and error if any
    remote fetch fails (consistent with “fetch failures are errors”).
  - [ ] 008.2.3 If `--fetch <remote>`, fetch only those remotes.
- [ ] 008.3 Improve merge target ergonomics in multi-remote setups.
  - [ ] 008.3.1 Keep accepting explicit refs like `jj/bob/feature-x`.
  - [ ] 008.3.2 (Optional) If user passes `bob/feature-x`, resolve it to
    a unique `*/bob/feature-x` across remotes; if ambiguous, error and
    list candidates.
- [ ] 008.4 Make push behavior fork-friendly.
  - XXX this should be 'origin', not 'upstream' - we should never push to upstream
  - [ ] 008.4.1 If upstream is set: `git push` as usual.
  - [ ] 008.4.2 If upstream is not set: use `--push-remote` or
    `remote.pushDefault` if configured; otherwise error with exact `git
    push -u ...` suggestions.
  - [ ] 008.4.3 Ensure we never silently push to a guessed remote when
    multiple remotes exist.
- [ ] 008.5 Update help/docs to cover forks explicitly.
  - [ ] 008.5.1 Add a short “fork workflow” section to `usage.tmpl`:
    add collaborator remotes and merge from `REMOTE/<user>/<twig>`.
  - [ ] 008.5.2 Explain how to configure push remote for fork workflows
    (example: `git config --local remote.pushDefault <your-fork-remote>`).
  - [ ] 008.5.3 Ensure examples don’t assume `origin`.
- [ ] 008.6 Testing
  - [ ] 008.6.1 Extend TODO 002 harness to add multiple remotes.
  - [ ] 008.6.2 Simulate “read-only upstream” by setting an invalid push
    URL for the upstream remote (so pushes fail deterministically).
    - XXX we should only ever push to our own fork
  - [ ] 008.6.3 Add system tests (TODO 003) for: merge from a
    collaborator remote and push to a configured push-remote.

## Notes / pitfalls

- In multi-remote setups, the same branch name may exist in multiple
  remotes; any shorthand resolution must handle ambiguity safely.
- Fork workflows may not have a truly “shared” twig unless the group
  designates a twig remote; TODO 006 onboarding should incorporate
  this concept.
