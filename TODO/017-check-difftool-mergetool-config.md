# TODO 017 - Check git difftool/mergetool config and assist setup

Problem: mob-consensus relies on `git mergetool` and `git difftool` for human review and conflict resolution. If a user has not configured these tools, Git may prompt interactively, pick surprising defaults, or fail in a confusing way.

Goal: detect missing/unsafe configuration early and guide the user through a one-time setup that matches their editor/workflow.

## What to check
- `git config --get merge.tool` and `git config --get diff.tool`
- If a tool is set, verify the corresponding `mergetool.<tool>.*` / `difftool.<tool>.*` keys are present (at least `cmd` if needed).
- Check `mergetool.prompt` / `difftool.prompt` (ensure mob-consensus can run with predictable prompting behavior).
- Optionally detect common editors on PATH (`nvim`, `vim`, `code`, `meld`) and offer tailored suggestions.

## Proposed UX
- Add a command like `mob-consensus tools check` (or run checks at the start of `merge` and onboarding commands).
- If missing:
  - explain why it matters (mob-consensus will invoke `git mergetool`/`git difftool`)
  - show exact `git config` commands to apply
  - prompt for confirmation before writing config
- Prefer writing **repo-local** config (`git config --local ...`) by default to avoid surprising global changes; offer `--global` as an explicit option.

## Interactions with other TODOs
- Complements TODO 011 (merge-conflict UX) and TODO 012 (LLM-assisted review): even if we later add richer built-in diff review, many teams will still want `mergetool` as an escape hatch.

## Implementation plan
- [ ] 017.1 Implement a `checkTools(ctx)` helper that returns a structured report (missing/ok + suggested commands).
- [ ] 017.2 Add a CLI entrypoint (`tools check` and possibly `tools setup`).
- [ ] 017.3 Integrate checks into `merge` (and possibly `init/start/join`) with a clear “skip” option.
- [ ] 017.4 Add tests covering: no config, partial config, and configured tool paths.
- [ ] 017.5 Update help text to mention the new checks and recommended setup.

