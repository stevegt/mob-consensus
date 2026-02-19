# TODO 014 - Consider Bubble Tea TUI for mob-consensus

Context: `mob-consensus` currently uses a traditional CLI UX: it
prints a plan (or runs steps), prompts with `y/N`, and shells out to
`git` (including `git mergetool` / `git difftool` / `$EDITOR`). As we
add richer “review + approve” workflows (TODO 011/012) and more
complex multi-remote setups (TODO 008), the prompt-and-print UI can
become hard to scan and error-prone.

Goal: decide whether to add an optional TUI using Bubble Tea
(Charmbracelet) to improve interactive flows **without breaking**
scriptability (`--yes`, `--plan`, `--dry-run`) or non-TTY usage.

## Pros

- Better interactive ergonomics:
  - Structured “wizard” flows for `init/start/join` (step list,
    current step, clear next actions).
  - Pickers for remotes/branches (filterable list instead of typing).
  - Rich confirmation screens (“here’s what will happen; approve?”)
    that reduce mistakes.
  - Inline status/progress (spinners, captured command output, clearer
    errors).
- Better review UX:
  - Dedicated screens for “what changed since last sync?”, “what am I
    merging?”, “what will be pushed?”.
  - Can present a short summary + allow opening diffs or dropping to
    external tools.
- Cleaner maintainability for complex interactive states than ad-hoc
  print/prompt branching.

## Cons / risks

- Complexity + dependency surface area:
  - Event loop state machine, terminal quirks, and more UI code to
    maintain.
  - Harder to keep behavior identical across TTYs/platforms.
- Interaction with external interactive commands is tricky:
  - `git mergetool` / `git difftool` / `$EDITOR` want full terminal
    control.
  - We’d likely need a “suspend TUI, run command, resume” pattern, and
    a reliable way to detect failures and return to the right screen.
- Testing burden:
  - Unit-testing a TUI is feasible, but integration/system tests may
    still need PTY-driven automation (see TODO 013).
- Risk of harming scriptability if the TUI becomes the primary
  interface:
  - Must preserve (and test) non-TUI behavior for automation and for
    AI agents that don’t have a real terminal UI.

## Possible UI improvements vs current CLI

- `mob-consensus status` discovery:
  - Show related branches with ahead/behind/diverged badges; allow
    selecting a merge target.
- `merge` flow:
  - Show “resolved target” + preview of changes before executing
    merge.
  - After merge, show what will be committed/pushed; offer actions:
    open diff / open editor / push / abort.
- Multi-remote (fork) flow:
  - Show which remotes will be fetched and which remote is configured
    for push; avoid accidental pushes.
- Onboarding:
  - Interactive questionnaire that fills in `--twig`, `--base`,
    `--remote`, with review/confirmation before running commands
    (aligns with TODO 006).

## Implementation approach (if we do it)

- Keep CLI as source of truth; add TUI as an *optional* front-end:
  - Enable TUI only when stdin+stdout are TTY and `--tui` is set (or auto, but with a `--no-tui` escape hatch).
  - Preserve `--plan/--dry-run/--yes` semantics exactly (TUI should not activate in those modes).
- Treat external interactive tools as “terminal takeover” steps:
  - Bubble Tea app suspends itself, runs `git mergetool`/`difftool`/`$EDITOR`, then resumes and re-checks repo state (dirty? conflicts?).

## Subtasks

- [ ] 014.1 Decide scope: which commands get a TUI first (recommend: discovery + onboarding + merge confirm screens).
- [ ] 014.2 Prototype: a minimal Bubble Tea UI for branch selection + confirmation that calls existing code paths.
- [ ] 014.3 Define non-TTY behavior: ensure exact output and exit codes remain stable for scripts.
- [ ] 014.4 Investigate “suspend and exec” patterns for `git mergetool`/`difftool`/`$EDITOR`.
- [ ] 014.5 Define testing strategy:
  - [ ] 014.5.1 Unit tests for TUI state transitions (pure Bubble Tea model).
  - [ ] 014.5.2 PTY-driven integration tests only if needed (coordinate with TODO 013).
