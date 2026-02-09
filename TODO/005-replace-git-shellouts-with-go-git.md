# TODO 005 - Replace git shellouts with go-git

Goal: evaluate replacing (some or all) `mob-consensus` `git` shellouts with the Go library `github.com/go-git/go-git/v5`.

Why: fewer subprocesses, easier unit testing/mocking, and less dependence on an external `git` binary.

- [ ] 005.1 Inventory current `git` shellouts and categorize:
  - [ ] 005.1.1 Read-only plumbing (branch listing, ahead/behind/diff stats).
  - [ ] 005.1.2 Worktree mutation (checkout/create branch, commit, push/fetch).
  - [ ] 005.1.3 Interactive UX (merge, mergetool, difftool, editor).
- [ ] 005.2 Prototype a “hybrid” engine:
  - [ ] 005.2.1 Use go-git for discovery/status (read-only) where feasible.
  - [ ] 005.2.2 Keep `git` shellouts for interactive steps (merge + tools) and any missing features.
  - [ ] 005.2.3 Add a small interface boundary so logic can be tested without spawning `git`.
- [ ] 005.3 Decide: full replacement, hybrid, or keep shellouts.

## Findings (go-git compatibility vs what mob-consensus needs)

As of the go-git `COMPATIBILITY.md` in the module cache:

- `merge` support is **partial** and effectively **fast-forward only**.
- `mergetool` is **not supported**.
- `diff` exists (unified diff output), but there is no equivalent of `git difftool` (interactive UI) in go-git.

Also: go-git is a library; it does not “call your mergetool/difftool”. To keep the current UX, we would still shell out to `git mergetool` / `git difftool` (or reimplement that orchestration ourselves by reading git config and launching external tools).

Implication: a full replacement would change core behavior of `mob-consensus` (manual non-FF merges + interactive mergetool/difftool). A hybrid approach (go-git for read-only status; keep `git` for merge/tooling) is more realistic.

## Pros

- **Fewer subprocesses**: avoids repeated `git ...` invocations for status/discovery.
- **More unit-testable**: inject a repo/worktree object or a small interface instead of mocking `exec.Command`.
- **Potential portability**: works even where `git` is missing (for read-only cases).

## Cons / risks

- **Feature gaps**: go-git does not cover `mergetool` and does not implement the merge style `mob-consensus` relies on.
- **Behavior drift**: subtle semantic differences vs the real `git` CLI (especially around worktrees, index, and merge edge-cases).
- **Auth/credential integration**: `git` CLI typically uses user-configured credential helpers/SSH config; go-git may require explicit auth wiring to match real-world setups.
- **Maintenance cost**: adds a major dependency and two “git engines” if we go hybrid.

## Suggested acceptance criteria (if pursued)

- Status/discovery output matches current behavior for typical repos.
- Merge mode retains `git merge --no-commit --no-ff` + external tools (no UX regression).
- No additional per-user setup for remotes/auth beyond what `git` already uses.
