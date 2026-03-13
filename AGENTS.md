# Repository Guidelines

## Project Structure & Module Organization
- The Go CLI lives at the module root (`main.go`) (preferred).
- `x/` holds experimental prototypes and legacy helpers; `x/mob-consensus` is the original Bash implementation kept for reference.
- Keep packages at the module root or under `x/` (avoid `internal/` and `pkg/`).
- Do not commit local state (e.g., `.grok/`, `.grok`) or generated binaries; keep these ignored locally.

## Build, Test, and Development Commands
- `go test ./...`: run unit tests.
- `go run . -h`: run locally (requires Go 1.24.0+).
- `go install github.com/stevegt/mob-consensus@latest`: install/upgrade the CLI.
- `mob-consensus -h`: show help and flags.

Notes: the tool runs `git fetch`, uses `git mergetool`/`git difftool` (defaulting to `vimdiff`), and pushes after commits unless `-n` is set.

## Coding Style & Naming Conventions
- Bash: keep changes small and readable; validate with `bash -n x/mob-consensus` (and `shellcheck x/mob-consensus` if available).
- Go: run `gofmt`; keep package names short and lower-case. Minimum supported Go is 1.24.0 (see `go.mod`).
- Always add detailed comments to code.

## Comment Preservation Protocol (Required)
- Never remove existing code comments unless they are replaced in the same patch by equal-or-better explanatory comments near the same logic.
- When rewriting or refactoring code, port old explanatory intent first, then improve wording.
- If a touched non-trivial code block has no comments, add explanatory comments.
- Do not treat shorter comments as better unless they preserve all important intent.
- For any non-trivial behavior change, include a behavior-level comment with:
  - `Intent:` a short, clear rationale (a sentence or a few; no hard cap if more is needed for clarity).
  - `Source:` a DI ID in the format `DI-NNN-YYYYMMDD-HHMMSS`.
  - `NNN` is the TODO number of the TODO file where that DI entry resides.
  - Optional: TODO file/section reference for faster lookup.
- If a comment must be dropped with no replacement, stop and ask the user before proceeding.
- Before editing a file, review existing comments in that file.
- Maintain a `## Decision Intent Log` at the top of relevant `TODO/*.md` files.
- Treat DI logs as append-only history. Do not rewrite or delete prior entries.
- When intent evolves, add a new DI entry and set `Supersedes: <old-di-id>`.
- DI entries must include:
  - `ID: DI-NNN-YYYYMMDD-HHMMSS`
  - `Date: YYYY-MM-DD HH:MM:SS`
  - `Status: active|superseded`
  - `Decision:`
  - `Intent:`
  - `Constraints:`
  - `Affects:`
  - `Supersedes:` (optional)
- After editing, run a comment-delta audit on each touched code file using: `git diff -U0 -- <file> | rg -n '^-\\s*//|^-\\s*/\\*|^\\+\\s*//|^\\+\\s*/\\*'`.
- Resolve all removed-comment lines before finalizing unless explicit user approval was given.
- In the final response, include:
  - `Comment audit: PASS/FAIL`, with file list.
  - `Intent provenance audit: PASS/FAIL`, listing files with behavior changes and DI sources.
- Hard gate: behavior-changing work is incomplete unless comments preserve intent and include DI provenance.
- Do not remove comments or documentation; update them if outdated or incorrect.

### Comment + DI Examples
- Comment format example:
  - `// Intent: Keep per-client rotation state stable across reconnects to avoid cross-client session churn. Source: DI-016-20260309-093000`
- Decision Intent Log entry template (for TODO files):
  - `ID: DI-NNN-YYYYMMDD-HHMMSS`
  - `Date: YYYY-MM-DD HH:MM:SS`
  - `Status: active`
  - `Decision: <what was decided>`
  - `Intent: <short clear rationale>`
  - `Constraints: <hard limits, dependencies, assumptions>`
  - `Affects: <paths, modules, commands, docs>`
  - `Supersedes: <old DI ID, optional>`

## Testing Guidelines
- Prefer deterministic tests using Go’s standard `testing` package when adding Go code.
- When tests interact with Git workflows, keep them as realistic as possible by using the same commands shown in the `mob-consensus` help (`usage.tmpl`) where practical (e.g., `git switch -c`, `git fetch`, `git push -u`). If a test must deviate (compatibility, determinism, or focus), explain why in the test code comments.
- For shell changes, run `bash -n x/mob-consensus` and include a short manual repro in the PR description.

## TODO Tracking
- Track work in `TODO/` and keep an index at `TODO/TODO.md`.
- Number TODOs with 3 digits (e.g., `005`), do not renumber, and sort the index by priority (not number).
- In each `TODO/*.md`, use numbered checkboxes like `- [ ] 005.1 describe subtask`.

## Commit & Pull Request Guidelines
- Keep commit messages short and imperative; existing history often uses a `mob-consensus:` prefix for script changes.
- PRs should include a concise summary, test commands run (e.g., `bash -n x/mob-consensus`), and before/after notes for behavior or output changes.
- When staging, list files explicitly (avoid `git add .` / `git add -A`).
- Derive commit message content from `git diff` (not only from chat context).

## Commit Messages
- Subject: short, imperative, capitalized (e.g., "Refactor chat client", "Fix WS path").
- Body: include a section per changed file with bullet points summarizing the edits.
- Use a here-doc (`git commit -F -`) for multi-line bodies; do not use multiple `-m` flags.
- Keep messages readable (avoid literal `\n` escapes); base content on the actual diff.

## Agent-Specific Notes
- Check `~/.codex/AGENTS.md` for updated local workflows and keep `~/.codex/meta-context.md` current.
- Treat a line containing only `commit` as “add and commit all changes with an AGENTS.md-compliant message”.

## Post-Commit AI Reviewer (TODO 019)
- Purpose: run an AI-based, read-only architectural/code-quality/doc-consistency review after each new commit.
- Trigger model:
  - Record current `HEAD` hash as baseline.
  - Poll `HEAD` every 60 seconds.
  - When hash changes, run one full review cycle.
- Scope: review all tracked code/docs.
- Constraints:
  - Read-only analysis of repo content.
  - Do not modify code or docs.
  - Do not generate helper scripts.
  - Only report file updates are allowed for TODO 019 output.
- Output location:
  - Write report to `TODO/019-post-commit-ai-review.md`.
  - Replace file content each cycle (do not append).
  - Ensure `TODO/TODO.md` has an entry for TODO 019.
- Required report format:
  - Header with timestamp and commit hash.
  - `Most critical next fixes` with 3-7 bullets.
  - Numbered, referenceable findings using TODO-style IDs (for example `019.1`, `019.2`, ...).
  - Group findings by: `Code Smells`, `Architecture`, `Code/Doc Drift`.
  - Include a confidence note for inference-heavy findings.
- Quality bar:
  - Prioritize actionable, high-severity findings first.
  - Keep findings concise and concrete, with file/path references where possible.
- Operator runbook (future sessions):
  - Start: spawn an `awaiter` agent with the TODO 019 instructions above.
  - Record and share the spawned `agent_id` in chat for operational control.
  - Stop: call `close_agent` for that `agent_id`.
  - Resume: call `resume_agent` for the same `agent_id`, then re-send TODO 019 instructions if needed.
  - Verify output: confirm `TODO/019-post-commit-ai-review.md` was rewritten after the latest commit.
