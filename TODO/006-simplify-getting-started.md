# TODO 006 - Simplify getting started flow

Goal: reduce the “getting started” steps for both the **first** group member and **next** group members by adding code that can:
- detect a sensible default base branch / remote / twig,
- suggest choices,
- show a proposed step plan (exact commands + short explanations),
- ask for confirmation (per-step), and
- perform the branch + push steps for the user.

Motivation: current onboarding requires several manual git commands, and the shared twig must be pushed for others to join.

- [ ] 006.1 Define the UX and CLI surface (minimize flags, keep existing modes intact).
  - [ ] 006.1.1 Prefer explicit subcommands: `mob-consensus start` (first member) and `mob-consensus join` (next member).
  - [ ] 006.1.2 Add `mob-consensus init` to analyze the repo and suggest `start` vs `join` (ask for confirmation).
  - [ ] 006.1.3 Two-phase operation: `--plan` prints the plan; without it the tool executes the same plan.
  - [ ] 006.1.4 Confirm-before-running: show each command + 1–2 line explanation and ask “Run this? [y/N]”.
  - [ ] 006.1.5 Non-interactive modes:
    - [ ] 006.1.5.1 `--yes` to accept defaults + run without prompts.
    - [ ] 006.1.5.2 `--dry-run` to print commands only (no prompts, no execution).
  - [ ] 006.1.6 Define the required inputs: `twig`, `base`, `remote`.
- [ ] 006.2 Implement detection + prompting helpers.
  - [ ] 006.2.1 Detect current base branch (default: current branch).
  - [ ] 006.2.2 Detect remotes; if none → error; if one → default; if many → prompt/require flag.
    - [ ] 006.2.2.1 Prefer the remote from the current branch upstream (if present).
    - [ ] 006.2.2.2 Never assume `origin` (only use it if it’s the upstream/only remote or the user explicitly selects it).
  - [ ] 006.2.3 Detect whether the shared twig exists on the selected remote (`<remote>/<twig>`):
    - [ ] 006.2.3.1 exists → suggest “join”
    - [ ] 006.2.3.2 missing → suggest “start”
  - [ ] 006.2.4 Validate twig name and derived `<user>` prefix (`check-ref-format`).
  - [ ] 006.2.5 Detect collisions and partial progress (for safe “resume” behavior).
    - [ ] 006.2.5.1 Local twig branch exists? Is it tracking `<remote>/<twig>`?
    - [ ] 006.2.5.2 Remote twig exists but local twig missing?
    - [ ] 006.2.5.3 Local `<user>/<twig>` exists? Has an upstream set?
    - [ ] 006.2.5.4 Remote `<user>/<twig>` exists? Does local branch match it?
  - [ ] 006.2.6 Make steps idempotent (avoid state files):
    - [ ] 006.2.6.1 Each step has a pre-check (“already done?”) and post-check (“did it work?”).
    - [ ] 006.2.6.2 Re-running recomputes “what’s missing” to recover from aborted runs.
- [ ] 006.3 First member (“start”) automation.
  - [ ] 006.3.1 Create shared twig branch from base (`git switch -c <twig> <base>`).
  - [ ] 006.3.2 Push shared twig branch (required; others can’t join until this is pushed) (`git push -u <remote> <twig>`).
  - [ ] 006.3.3 Create personal branch from the local twig (`mob-consensus -b <twig>` or internal equivalent).
  - [ ] 006.3.4 Push personal branch (`git push -u <remote> <user>/<twig>`).
- [ ] 006.4 Next members (“join”) automation.
  - [ ] 006.4.1 Fetch (`git fetch <remote>`).
  - [ ] 006.4.2 Create local twig tracking remote (`git switch -c <twig> <remote>/<twig>`).
  - [ ] 006.4.3 Create and push personal branch (same as 006.3.3–006.3.4).
- [ ] 006.5 Add non-interactive flags (for scripts/tests).
  - [ ] 006.5.1 `--twig <name>`, `--base <ref>`, `--remote <name>`.
  - [ ] 006.5.2 `--yes` to accept defaults and skip prompts.
  - [ ] 006.5.3 `--plan` for “print the plan and exit” (safe for copy/paste).
  - [ ] 006.5.4 `--dry-run` for “print commands only, no execution”.
- [ ] 006.6 System tests.
  - [ ] 006.6.1 Extend TODO 003 harness to cover `start` and `join` flows end-to-end.

## Notes / pitfalls

- Must refuse to proceed with a dirty working tree unless `-c` (or equivalent) is used.
- Must not assume `origin`; choose a remote deterministically or ask.
- Must handle existing local branches (`<twig>` or `<user>/<twig>`) gracefully (offer to reuse, rename, or abort).
- Avoid “helpful” pushes in non-interactive mode unless explicitly confirmed.
- Prefer printing commands that users can paste verbatim (and make `--plan` output match the executed plan).
