# TODO 003 - Automated system testing plan

Goal: add repeatable, non-networked system tests that exercise `mob-consensus` end-to-end with multiple simulated users.

Constraints:
- Do not modify `x/mob-consensus`.
- Avoid interactive tools in CI (editor/difftool/mergetool prompts).

- [ ] 003.1 Add a build-tagged system test suite (so `go test ./...` stays fast).
- [ ] 003.2 Implement a helper harness in tests (create bare remote, seed, clone users, run commands).
- [ ] 003.3 Cover `-b` branch creation and push advice.
- [ ] 003.4 Cover discovery mode output (ahead/behind/diverged/synced).
- [ ] 003.5 Cover merge mode for clean/no-op merges (non-interactive).
- [ ] 003.6 Add one “conflict merge fails cleanly” test (expected non-zero exit).

## Proposed structure

- `system_test.go` (or `system/system_test.go`) guarded by a build tag:
  - `//go:build system`
  - Run with: `go test -tags=system ./...`
- Use `exec.Command` to run `git` and the built `mob-consensus` binary with `cmd.Dir` pointing at each clone.
- Use `t.TempDir()` and a **local bare repo** as `origin` to avoid network/credentials.

## Making merge mode non-interactive (without new flags)

Until TODO 001.6 adds tool/editor overrides, tests can neutralize interactivity via per-repo config + env:

- `git config difftool.prompt false`
- `git config mergetool.prompt false`
- `git config difftool.vimdiff.cmd true`
- `git config mergetool.vimdiff.cmd true`
- Env: `GIT_EDITOR=true`

This avoids prompts, avoids requiring `vimdiff` to be installed, and makes `git commit -e` return immediately.

## Test cases (minimum viable)

1) **Branch creation (no push required)**
- Arrange: in `alice/`, create `feature-x` from `main`.
- Act: run `mob-consensus -b feature-x`.
- Assert: current branch is `alice/feature-x`; stdout contains a `git push -u` suggestion.

2) **Discovery statuses**
- Arrange: make/push commits such that `bob/feature-x` is ahead; then create diverged histories.
- Act: run `mob-consensus`.
- Assert: output contains “is ahead”, “is behind”, and “has diverged” in the expected scenarios.

3) **Merge no-op is success**
- Arrange: merge `origin/bob/feature-x` once into `alice/feature-x`.
- Act: run the same merge again.
- Assert: exit code 0 and `HEAD` unchanged.

4) **Merge clean creates a merge commit**
- Arrange: `bob/feature-x` has a commit that cleanly merges.
- Act: run merge from `alice/feature-x`.
- Assert: `git log -1` is a merge commit and includes at least one `Co-authored-by:` line for Bob.

5) **Conflict merge exits non-zero**
- Arrange: create a real content conflict between `alice/feature-x` and `bob/feature-x`.
- Act: run merge with tools neutralized.
- Assert: non-zero exit and repo is left in a merge state (e.g., `MERGE_HEAD` exists).

## Follow-ups

If automated merge testing remains brittle, prefer implementing TODO 001.6 so tests can run with explicit flags like `--no-tools` and `--no-edit` (or an `--editor` override) rather than depending on git config tricks.
