# TODO 006 - Simplify getting started flow

Goal: reduce the “getting started” steps for both the **first** group member and **next** group members by adding code that can:
- detect a sensible default base branch / remote / twig,
- suggest choices,
- show a proposed step plan (exact commands + short explanations),
- ask for confirmation (per-step), and
- perform the branch + push steps for the user.

Motivation: current onboarding requires several manual git commands, and the shared twig must be pushed for others to join.

## Pros / cons of doing this TODO now

Pros:
- Big UX win: fewer error-prone onboarding steps; enforce “push the shared twig” as a required gate.
- `--plan` output becomes executable documentation (and great for bug reports: “here’s what it proposed”).
- Safety improvements early: detect collisions/partial progress and recover cleanly from aborted starts.
- Helps testing: the same planner can drive manual testing (TODO 002) and later automated system tests (TODO 003).

Cons / risks:
- More surface area before core behaviors are fully battle-tested (many repo/remote/branch edge cases).
- Interactive prompting is harder to test; without a clean planner/executor split it can get brittle.
- Wrong heuristics are costly (creating/pushing the wrong branch to the wrong remote); users may “yes” through prompts.

Recommended sequencing if doing it now:
- Implement planner-only first (`init --plan` / `start --dry-run` / `join --dry-run`, no side effects).
- Add step-by-step execution only after the plan output is solid.
- If remote selection is ambiguous, require an explicit choice (never guess).

- [x] 006.1 Define the UX and CLI surface (minimize flags, keep existing modes intact).
  - [x] 006.1.1 Prefer explicit subcommands: `mob-consensus start` (first member) and `mob-consensus join` (next member).
  - [x] 006.1.2 Add `mob-consensus init` to analyze the repo and suggest `start` vs `join` (ask for confirmation).
  - [x] 006.1.3 Two-phase operation: `--plan` prints the plan; without it the tool executes the same plan.
  - [x] 006.1.4 Confirm-before-running: show each command + 1–2 line explanation and ask “Run this? [y/N]”.
  - [x] 006.1.5 Non-interactive modes:
    - [x] 006.1.5.1 `--yes` to accept defaults + run without prompts.
    - [x] 006.1.5.2 `--dry-run` to print commands only (no prompts, no execution).
  - [x] 006.1.6 Define the required inputs: `twig`, `base`, `remote`.
- [ ] 006.2 Implement detection + prompting helpers.
  - [x] 006.2.1 Detect current base branch (default: current branch).
  - [x] 006.2.2 Detect remotes; if none → error; if one → default; if many → prompt/require flag.
    - [x] 006.2.2.1 Prefer the remote from the current branch upstream (if present).
    - [x] 006.2.2.2 Never assume `origin` (only use it if it’s the upstream/only remote or the user explicitly selects it).
  - [x] 006.2.3 Detect whether the shared twig exists on the selected remote (`<remote>/<twig>`):
    - [x] 006.2.3.1 exists → suggest “join”
    - [x] 006.2.3.2 missing → suggest “start”
  - [x] 006.2.4 Validate twig name and derived `<user>` prefix (`check-ref-format`).
  - [ ] 006.2.5 Detect collisions and partial progress (for safe “resume” behavior).
    - [ ] 006.2.5.1 Local twig branch exists? Is it tracking `<remote>/<twig>`?
    - [x] 006.2.5.2 Remote twig exists but local twig missing?
    - [ ] 006.2.5.3 Local `<user>/<twig>` exists? Has an upstream set?
    - [x] 006.2.5.4 Remote `<user>/<twig>` exists? Does local branch match it?
  - [x] 006.2.6 Make steps idempotent (avoid state files):
    - [x] 006.2.6.1 Each step has a pre-check (“already done?”) and post-check (“did it work?”).
    - [x] 006.2.6.2 Re-running recomputes “what’s missing” to recover from aborted runs.
- [x] 006.3 First member (“start”) automation.
  - [x] 006.3.1 Create shared twig branch from base (`git switch -c <twig> <base>`).
  - [x] 006.3.2 Push shared twig branch (required; others can’t join until this is pushed) (`git push -u <remote> <twig>`).
  - [x] 006.3.3 Create personal branch from the local twig (`mob-consensus -b <twig>` or internal equivalent).
  - [x] 006.3.4 Push personal branch (`git push -u <remote> <user>/<twig>`).
- [x] 006.4 Next members (“join”) automation.
  - [x] 006.4.1 Fetch (`git fetch <remote>`).
  - [x] 006.4.2 Create local twig tracking remote (`git switch -c <twig> <remote>/<twig>`).
  - [x] 006.4.3 Create and push personal branch (same as 006.3.3–006.3.4).
- [x] 006.5 Add non-interactive flags (for scripts/tests).
  - [x] 006.5.1 `--twig <name>`, `--base <ref>`, `--remote <name>`.
  - [x] 006.5.2 `--yes` to accept defaults and skip prompts.
  - [x] 006.5.3 `--plan` for “print the plan and exit” (safe for copy/paste).
  - [x] 006.5.4 `--dry-run` for “print commands only, no execution”.
- [ ] 006.6 System tests.
  - [x] 006.6.0 Add Go integration tests exercising `init`/`start`/`join` via `run()` in temp repos (covers onboarding paths in `main.go`).
  - [ ] 006.6.1 Extend TODO 003 harness to cover `start` and `join` flows end-to-end (tracked in TODO 010).

## Notes / pitfalls

- Must refuse to proceed with a dirty working tree unless `-c` (or equivalent) is used.
- Must not assume `origin`; choose a remote deterministically or ask.
- Must handle existing local branches (`<twig>` or `<user>/<twig>`) gracefully (offer to reuse, rename, or abort).
- Avoid “helpful” pushes in non-interactive mode unless explicitly confirmed.
- Prefer printing commands that users can paste verbatim (and make `--plan` output match the executed plan).
