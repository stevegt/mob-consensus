# TODO 010 - Extend `mc-test` for deeper system coverage

Context: `scripts/mc-test` is now a useful smoke test
(bootstrap/join/discovery/merge), but it does not yet exercise **all**
user-facing behavior in `mob-consensus` or cover important edge cases
(dirty trees, multi-remote ambiguity, smartPush behavior, conflict
merges, etc.). We also want measurable test coverage.

Goal: extend `scripts/mc-test` so it can validate (non-interactively
by default) the full `mob-consensus` workflow with multiple simulated
users who **actually create, modify, and merge files across clones**,
and produce a repeatable coverage report.

## What we must cover

- Branch creation:
  - `mob-consensus -b <twig>` creates `<user>/<twig>` from local
    `<twig>`.
  - Re-running `-b` is idempotent (switches to existing
    `<user>/<twig>`).
- Discovery:
  - Shows `ahead`, `behind`, `has diverged`, and `synced` correctly.
- Merge:
  - Clean merge produces a merge commit with `Co-authored-by:`
    trailers.
  - No-op merge (“Already up to date”) exits 0 and does not create a
    commit.
  - Shorthand `bob/<twig>` resolves to exactly-one
    `<remote>/bob/<twig>` across remotes and asks for confirmation.
- Dirty-tree behavior:
  - `-b` / merge require clean trees unless `-c` is provided.
  - `-c` commits dirty work (and pushes unless `-n`).
- Remote ambiguity:
  - Fetch errors are fatal.
  - Multiple remotes without an upstream should produce a clear error
    (no guessing).
- Push behavior (`smartPush`):
  - With upstream: `git push`.
  - Without upstream: `git push -u <remote> <branch>` only when
    unambiguous (`branch.*.pushRemote`, `remote.pushDefault`, or sole
    remote); otherwise error with exact commands.

## Plan (extend `scripts/mc-test`)

- [ ] 010.1 Add scenario coverage matrix in `scripts/mc-test --help` (brief list of scenarios and what they assert).
- [x] 010.2 Add scenario `branch`:
  - [x] 010.2.1 Run `mob-consensus -b <twig>` twice; assert branch unchanged and no error.
  - [x] 010.2.2 Verify push advice contains the selected remote or a placeholder when ambiguous.
- [x] 010.3 Add scenario `dirty`:
  - [x] 010.3.1 Make an uncommitted tracked change; run `mob-consensus OTHER_BRANCH`; assert non-zero + “dirty” message.
  - [x] 010.3.2 Repeat with `-c`; assert an auto-commit is created before the merge and the worktree ends clean.
- [x] 010.4 Add scenario `smartpush`:
  - [x] 010.4.1 No upstream + single remote: after merge/commit, assert `-u` upstream gets set.
  - [x] 010.4.2 Add a second remote (e.g. `jj`) and assert `smartPush` errors until `remote.pushDefault` or `branch.<name>.pushRemote` is configured.
- [ ] 010.5 Add scenario `multi-remote-fetch`:
  - [x] 010.5.1 With 2 remotes and no upstream, assert discovery fails with a clear “multiple remotes” error.
  - [x] 010.5.2 With upstream set, assert discovery proceeds.
  - [ ] 010.5.3 Add merge-mode coverage under multi-remote setups (both “ambiguous remotes” errors and “explicit remote works”).
- [x] 010.6 Add scenario `converge` (real multi-user workflow):
  - [x] 010.6.1 Each user makes at least one commit touching real files (not just empty commits) and pushes.
  - [x] 010.6.2 Leader merges peers; peers merge leader; everyone pushes.
  - [x] 010.6.3 Final discovery output on each user reports peer branches are `synced`.
- [ ] 010.7 Add conflict coverage (two tiers):
  - [ ] 010.7.1 Automated: configure a non-interactive `mergetool.vimdiff.cmd` that resolves deterministically (e.g., choose “theirs”) so CI can exercise the conflict path without opening editors.
  - [ ] 010.7.2 Manual: add a `--interactive` recipe that intentionally creates a conflict and documents the expected UX (mergetool + difftool + commit).

## Coverage reporting

We need two kinds of coverage:

1) **Unit coverage** (fast): `go test -coverprofile=... ./...`
- [x] 010.8 Add `mc-test coverage` to write `ROOT/coverage.out` and
  `ROOT/coverage.html` (via `go tool cover -html`), plus
  `ROOT/coverage.func.txt` (via `go tool cover -func`) and derived
  summaries (`coverage.total.txt`, `coverage.zero.txt`, `coverage.low.txt`).

2) **System/integration coverage** (meaningful): execute `run()` paths
while creating real repos.
- [ ] 010.9 Implement Go “system tests” (see TODO 003) that create the
  same harness layout via `t.TempDir()` and drive `mob-consensus`
  logic (ideally in-process) so the merge/discovery/create-branch
  paths contribute to the coverprofile.
- [ ] 010.10 Decide/record an initial minimum coverage target (start
  low, raise over time).

### Current baseline (from `mc-test coverage`)

Baseline run: `scripts/mc-test coverage --root /tmp/tmp.LVlJXTGvxj/`

- Coverage is **7.0%** overall (30/430 statements), and the only file
  is `main.go` (7.0%).
- Covered code is mostly the pure helpers:
  `twigFromBranch`, `relatedBranches`, `coAuthorLines`,
  `diffStatusLine`.
- Most user-facing CLI behavior is currently uncovered in Go tests:
  arg parsing, usage rendering, remote selection/fetching, branch
  creation, discovery, merge paths, and `smartPush`.

### Plan to raise coverage (beyond `mc-test` scenarios)

- [ ] 010.11 Add Go tests that exercise real CLI paths via `run()` in a temp git repo:
  - [x] 010.11.1 `-h/--help` renders usage without error (in a temp repo with/without remotes).
  - [x] 010.11.2 `-b <base>` creates `<user>/<twig>` from local `<twig>` (create `twig` first) and prints push advice.
  - [x] 010.11.3 Discovery mode prints the expected status lines (ahead/behind/diverged/synced) for arranged histories.
  - [x] 010.11.4 Merge mode: clean merge creates a merge commit with `Co-authored-by:` trailers; repeat merge is a no-op success.
  - [ ] 010.11.5 Error paths produce human-readable errors:
    - [x] 010.11.5.1 Missing `user.email`
    - [x] 010.11.5.2 Detached HEAD
    - [x] 010.11.5.3 No remotes
    - [x] 010.11.5.4 Multiple remotes (push ambiguity)
    - [x] 010.11.5.5 Ambiguous merge target across remotes
- [ ] 010.12 Extend `mc-test coverage` to optionally include system tests:
  - [ ] 010.12.1 Add `mc-test coverage --system` (or `--tags system`) to run `go test -tags=system -coverprofile=... ./...`.
  - [ ] 010.12.2 If we keep unit-only as the default, update usage text so users know the difference.

## Notes / risks

- Conflict merges are hard to automate unless we provide a
  deterministic non-interactive mergetool; simply setting
  `mergetool.*.cmd=true` is not sufficient because it won’t resolve
  conflicts.
- Some paths depend on interactive confirmation; tests should run
  those paths with a controlled stdin (e.g., piping `y\n`) and assert
  the prompt appears.
