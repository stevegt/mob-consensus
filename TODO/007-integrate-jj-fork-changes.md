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
- `usage.tmpl` and `scripts/mc-test`: exist on `main` but are
  deleted/rewritten on JJ’s branch.

Key design choice differences to preserve from `main`:
- Branch prefix must come from `git config user.email` local-part (not
  `$USER`) to avoid leaking local usernames.
- Do not assume remote name `origin`; only auto-select a remote when
  unambiguous (upstream remote or sole remote).
- Help text must be maintainable: edit a template and print once (no
  line-by-line `fmt.Fprintln` help blocks).

## JJ changes worth porting (adapted to `main` constraints)

- Branch creation idempotency: re-running `mob-consensus branch create <twig>`
  should switch to existing `<user>/<twig>` instead of failing.
- Output plumbing: `ensureClean` should write to the provided writer,
  not directly to `os.Stdout`.
- Push behavior when no upstream is set: handle this gracefully after
  commits/merges.
  - Prefer: if upstream is missing, run `git push -u <remote>
    <branch>` **only when** a push remote can be selected unambiguously:
    - `branch.<name>.pushRemote`, or
    - `remote.pushDefault`, or
    - only configured remote.
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

Constraint: do **not** cherry-pick and do **not** merge JJ’s branch
directly. Codex should port the *effects* of JJ’s changes into our code
while preserving this repo’s current design decisions.

Credit approach:
- Use `Co-authored-by:` trailers in the porting commits (so JJ is visibly
  credited on GitHub).
- Include a short “Ported from JJ fork” note listing the source commit
  hashes (example: `93e99f3`) in the commit message
  body.

### Detailed steps

- [x] 007.1 Create an integration branch from `main` (recommended):
  `git switch -c jj-fixed`.
- [x] 007.2 Codex ports code changes from `remotes/jj/main` into `main.go`
  (no cherry-pick).
  - [x] 007.2.1 Idempotent branch creation: if `<user>/<twig>` exists,
    switch to it; otherwise create it.
  - [x] 007.2.2 `ensureClean` output: accept `stdout io.Writer` and
    stop writing to `os.Stdout`.
  - [x] 007.2.3 Upstreamless push: implement a `smartPush` that:
    - [x] 007.2.3.1 Uses plain `git push` when upstream is set.
    - [x] 007.2.3.2 Uses `git push -u <remote> <branch>` only when the
      push remote is unambiguous (`branch.<name>.pushRemote`, or
      `remote.pushDefault`, or only remote).
    - [x] 007.2.3.3 Otherwise returns a human-readable error with exact
      suggested `git push -u ...` command(s).
  - [x] 007.2.4 Merge target resolution: accept shorthand (e.g.
    `bob/<twig>`) by auto-resolving to exactly-one `<remote>/bob/<twig>`
    across remotes; ask for confirmation before merging.
  - [x] 007.2.5 Add `*.lock` to `.gitignore` (note: this is broad; revisit
    if we ever want to track something like `flake.lock`).
- [ ] 007.3 Keep `usage.tmpl` and docs aligned with `main`’s decisions
  (no `$USER` or `origin` assumptions).
- [x] 007.4 Verify formatting + tests: `gofmt -w main.go` and `go test
  ./...`.
- [ ] 007.5 Run system/manual harness (TODO 002) to validate the
  ported behaviors end-to-end.
- [ ] 007.6 Coordinate with JJ:
  - [ ] 007.6.1 Ask for a short list of “must have” fixes.
  - [ ] 007.6.2 Prefer follow-up PRs that are narrowly scoped (one fix
    per PR) to reduce conflicts.

## Porting on `jj-fixed` branch vs doing it on `main`

Pros of a dedicated branch (`jj-fixed`):
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

Recommendation: use `jj-fixed` unless you can complete the port in one small, fully-tested commit.
