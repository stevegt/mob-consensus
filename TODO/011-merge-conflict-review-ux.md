# TODO 011 - Improve merge-conflict review UX (mergetool vs difftool)

Context: in `mob-consensus` merge mode, when `git merge --no-commit` hits a
conflict, we currently run **both**:

1) `git mergetool ...` (to resolve conflicts), then
2) `git difftool ... HEAD` (to review the merge result before committing).

That often shows the **same changes twice** (especially for conflicted files),
which is noisy and slows down the workflow. It also assumes users can run the
chosen tool(s) (today we force `vimdiff`), which may not be true on headless
systems or for users with different preferences.

Goal: keep a safe, reviewable merge flow, but avoid duplicate views and avoid
assuming a particular tool is installed/configured.

## Tradeoffs and options

### Keep current behavior (mergetool + difftool always)
Pros:
- Always provides a final review step before committing.
- Uses familiar Git flows.
Cons:
- Duplicate review for conflicted files.
- Requires both tools to work; `vimdiff` may not be installed/wanted.

### Skip difftool when conflicts occurred
Pros:
- No duplication.
- Faster for conflict-heavy merges.
Cons:
- Loses “final result” review (especially non-conflicted files).

### Run difftool conditionally (recommended)
Idea:
- If merge is clean: run difftool for review (as today).
- If merge had conflicts: run mergetool, then show a summary (`git status`,
  `git diff --stat HEAD`, maybe `git diff --name-only HEAD`) and **ask** whether
  to open difftool for final review.
- Optionally: run difftool only for **non-conflicted** files by capturing the
  conflicted file list before running mergetool (`git diff --name-only
  --diff-filter=U`) and excluding those paths from the review step.
Pros:
- Avoids duplication while preserving safety/review.
- Makes the approval step explicit.
Cons:
- More code paths; needs careful handling of file lists and quoting.

## Tool configuration questions

We need a stance on how tools are selected:

- Honor user config: prefer `git mergetool` / `git difftool` without `-t`
  (uses `merge.tool`, `diff.tool`) and document how to set them.
  - Pros: respects user preference.
  - Cons: if not configured, Git may prompt or fail.

- Provide explicit flags (e.g. `--mergetool TOOL`, `--difftool TOOL`,
  `--no-difftool`, `--no-mergetool`) with sensible defaults.
  - Pros: deterministic and scriptable.
  - Cons: more CLI surface area to maintain.

- Provide a fallback “no external tools” review: print `git diff --stat` and
  optionally `git diff` to stdout, then ask for confirmation to commit.
  - Pros: works everywhere; great for headless/CI.
  - Cons: can be overwhelming for large diffs.

## Better “approval” mechanisms (beyond difftool)

- Explicit confirmation after review: show summary + ask “Commit this merge?”
  (and optionally “Push now?”).
- LLM-assisted review (optional):
  - Feed `git diff --stat`, `git diff`, and/or conflict summaries to an LLM to
    produce a short “what changed” explanation and ask for approval.
  - Also useful for generating merge commit messages (already planned).
  - Concerns: privacy, secrets in diffs, network dependency/cost; must be
    opt-in with clear warnings and local/offline options where possible.

## Plan

- [ ] 011.1 Decide tool-selection policy (honor git config vs force tool vs flags).
- [ ] 011.2 Add a “merge had conflicts” detection flag so the flow can branch.
- [ ] 011.3 Change conflict flow to avoid duplicate review:
  - [ ] 011.3.1 After mergetool, show summary (`status`, `diff --stat`) and ask whether to open difftool.
  - [ ] 011.3.2 (Optional) Review only non-conflicted files, or review conflicts only on request.
- [ ] 011.4 Improve docs/help: how to configure mergetool/difftool, and what happens if none is configured.
- [ ] 011.5 Extend tests:
  - [ ] 011.5.1 Non-interactive conflict-path test in `go test` (scripted mergetool).
  - [ ] 011.5.2 Manual `mc-test --interactive` recipe for real tool UX.
