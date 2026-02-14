# TODO 007 - Integrate JJ fork changes

Context: JJ has been working on her fork (`remotes/jj/main`) and has
implemented fixes and docs that overlap with changes on `main`. These
branches have diverged since merge-base
`d36df4486e6c1230146e7b12d0b4c2ea5b17caf5` (as of 2026-02-13).

Goal: integrate the **useful fixes** from `remotes/jj/main` without
regressing the current design decisions on `main` (email-derived
`<user>`, no `origin` assumptions, templated help via `usage.tmpl`).

## Summary of differences / likely conflicts

High-conflict files:
- `main.go`: both branches changed core flow logic.
- `go.mod`: JJ removes `github.com/stevegt/goadapt` while `main` still
  uses it (`Ck`).
- `.gitignore`: JJ adds `*.lock`.
- `TODO/TODO.md` and `TODO/00{1,2,3,4}.md`: JJ reorganizes TODOs and
  reverts some docs to `$USER`/`origin` assumptions.
- `usage.tmpl` and `scripts/mc-init`: exist on `main` but are
  deleted/rewritten on JJ’s branch.

Key design choice differences to preserve from `main`:
- Branch prefix must come from `git config user.email` local-part (not
  `$USER`) to avoid leaking local usernames.
- Do not assume remote name `origin`; only auto-select a remote when
  unambiguous (upstream remote or sole remote).
- Help text must be maintainable: edit a template and print once (no
  line-by-line `fmt.Fprintln` help blocks).

## JJ changes worth porting (adapted to `main` constraints)

- Branch creation idempotency: re-running `mob-consensus -b <twig>`
  should switch to existing `<user>/<twig>` instead of failing.
- Output plumbing: `ensureClean` should write to the provided writer,
  not directly to `os.Stdout`.
- Push behavior when no upstream is set: handle this gracefully after
  commits/merges.
  - Prefer: if upstream is missing, run `git push -u <remote>
    <branch>` **only when** a remote can be selected unambiguously
    (reuse `suggestedRemote`).
  - Otherwise: return a clear error telling the user how to push
    and/or set upstream.
- Optional: merge target resolution convenience (only if it doesn’t
  add ambiguity):
  - If user passes `bob/<twig>`, consider resolving it to
    `<remote>/bob/<twig>` when there is exactly one plausible remote.
- Optional: `.gitignore` `*.lock` (verify it won’t hide important repo
  artifacts).

## Recommended approach

Do not merge `remotes/jj/main` directly. Instead, port JJ’s fixes
selectively onto `main` while keeping `main`’s UX + docs consistent.

## Preserving JJ credit

Preferred: keep JJ as the commit author by cherry-picking the relevant
commits (with `-x`), then add follow-up commits for any local policy or
design adaptations.

Why:
- JJ stays the author of the commits she wrote (best attribution).
- `-x` leaves a trace to the original commit hash in JJ’s fork.
- Follow-up commits make it clear what was changed during integration.

Suggested workflow:
- Cherry-pick JJ’s focused commits (example hashes will change as her
  branch evolves):
  - `git cherry-pick -x <jj-commit>`
- If a cherry-pick needs conflict resolution, the resulting commit still
  preserves JJ as author; you are the committer.
- If you substantially modify or extend a cherry-picked change, add a
  follow-up commit and (optionally) include a `Co-authored-by:` trailer
  for JJ.

Avoid: “merge JJ’s branch, then revert lots of content”. That preserves
credit, but creates noisy history and makes the integration hard to
review because docs/design regressions and fixes arrive interleaved.

### Detailed steps

- [ ] 007.1 Create an integration branch from `main` (recommended):
  `git switch -c jj-fixes`.
- [ ] 007.2 Port code changes from `remotes/jj/main` into `main.go`
  (either cherry-pick or manual).
  - XXX i want codex to port the changes.  cherry-picking will create
    a lot of conflicts and make it hard to reason about the changes.
    LLM-based porting will be cleaner and easier to review.
  - [ ] 007.2.1 Idempotent branch creation: if `<user>/<twig>` exists,
    switch to it; otherwise create it.
  - [ ] 007.2.2 `ensureClean` output: accept `stdout io.Writer` and
    stop writing to `os.Stdout`.
  - [ ] 007.2.3 Upstreamless push: implement a `smartPush` that:
    - [ ] 007.2.3.1 Uses plain `git push` when upstream is set.
    - [ ] 007.2.3.2 Uses `git push -u <remote> <branch>` only when
      `suggestedRemote` returns a real remote (not `<remote>`).
    - [ ] 007.2.3.3 Otherwise returns a human-readable error with
      exact suggested `git push -u ...` command(s).
  - [ ] 007.2.4 Merge target resolution: decide whether to support
    shorthand (e.g. `bob/<twig>`) and implement only if deterministic.
  - [ ] 007.2.5 Decide whether to add `*.lock` to `.gitignore`
    (document why).
- [ ] 007.3 Keep `usage.tmpl` and docs aligned with `main`’s decisions
  (no `$USER` or `origin` assumptions).
- [ ] 007.4 Verify formatting + tests: `gofmt -w main.go` and `go test
  ./...`.
- [ ] 007.5 Run system/manual harness (TODO 002) to validate the
  ported behaviors end-to-end.
- [ ] 007.6 Coordinate with JJ:
  - [ ] 007.6.1 Ask for a short list of “must have” fixes.
  - [ ] 007.6.2 Prefer follow-up PRs that are narrowly scoped (one fix
    per PR) to reduce conflicts.

## Porting on `jj-fixes` branch vs doing it on `main`

Pros of a dedicated branch (`jj-fixes`):
- Safer: avoids leaving `main` in a half-integrated or broken state.
- Easier review: the diff is focused on the integration work and can be iterated/rebased.
- Easy rollback: abandon the branch if the approach is wrong.
- Better collaboration: JJ can review a single branch/PR instead of watching `main` churn.

Cons of a dedicated branch:
- Slight overhead: branch management + an extra merge when finished.
- Risk of drift: if `main` changes during the port, you may need to rebase/resolve again.

Pros of doing it directly on `main`:
- Fastest path if you’re confident and working solo.
- No extra merge step.

Cons of doing it directly on `main`:
- Higher risk: conflicts and partial ports can break other work immediately.
- Harder to reason about: “what changed because of JJ integration?” becomes mixed with unrelated edits.
- Harder to revert cleanly without backing out multiple commits.

Recommendation: use `jj-fixes` unless you can complete the port in one small, fully-tested commit.
