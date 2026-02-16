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
- PRs should include: a concise summary, test commands run (e.g., `bash -n x/mob-consensus`), and before/after notes for behavior or output changes.
- When staging, list files explicitly (avoid `git add .` / `git add -A`).

## Agent-Specific Notes
- Check `~/.codex/AGENTS.md` for updated local workflows and keep `~/.codex/meta-context.md` current.
- Treat a line containing only `commit` as “add and commit all changes with an AGENTS.md-compliant message”.
