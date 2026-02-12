# TODO 006 - Simplify getting started flow

Goal: reduce the “getting started” steps for both the **first** group member and **next** group members by adding code that can:
- detect a sensible default base branch / remote / twig,
- suggest choices,
- ask a small number of confirmation questions, and
- perform the branch + push steps for the user.

Motivation: current onboarding requires several manual git commands, and the shared twig must be pushed for others to join.

- [ ] 006.1 Define the UX and CLI surface (minimize flags, keep existing modes intact).
  - [ ] 006.1.1 Choose a command/flag name (examples: `mob-consensus init`, `mob-consensus start`, `mob-consensus join`, or `--wizard`).
  - [ ] 006.1.2 Decide interactive vs non-interactive defaults (ex: prompt unless `--yes`).
  - [ ] 006.1.3 Define the required inputs: `twig`, `base`, `remote`.
- [ ] 006.2 Implement detection + prompting helpers.
  - [ ] 006.2.1 Detect current base branch (default: current branch).
  - [ ] 006.2.2 Detect remotes; if none → error; if one → default; if many → prompt/require flag.
  - [ ] 006.2.3 Detect whether `<remote>/<twig>` already exists:
    - exists → suggest “join”
    - missing → suggest “start”
  - [ ] 006.2.4 Validate twig name and derived `<user>` prefix (`check-ref-format`).
- [ ] 006.3 First member (“start”) automation.
  - [ ] 006.3.1 Create shared twig branch from base (`git switch -c <twig> <base>`).
  - [ ] 006.3.2 Push shared twig branch (`git push -u <remote> <twig>`).
  - [ ] 006.3.3 Create personal branch (`mob-consensus -b <twig>` or internal equivalent).
  - [ ] 006.3.4 Push personal branch (`git push -u <remote> <user>/<twig>`).
- [ ] 006.4 Next members (“join”) automation.
  - [ ] 006.4.1 Fetch (`git fetch <remote>`).
  - [ ] 006.4.2 Create local twig tracking remote (`git switch -c <twig> <remote>/<twig>`).
  - [ ] 006.4.3 Create and push personal branch (same as 006.3.3–006.3.4).
- [ ] 006.5 Add non-interactive flags (for scripts/tests).
  - [ ] 006.5.1 `--twig <name>`, `--base <ref>`, `--remote <name>`.
  - [ ] 006.5.2 `--yes` to accept defaults and skip prompts.
- [ ] 006.6 System tests.
  - [ ] 006.6.1 Extend TODO 003 harness to cover `start` and `join` flows end-to-end.

## Notes / pitfalls

- Must refuse to proceed with a dirty working tree unless `-c` (or equivalent) is used.
- Must not assume `origin`; choose a remote deterministically or ask.
- Must handle existing local branches (`<twig>` or `<user>/<twig>`) gracefully (offer to reuse, rename, or abort).
- Avoid “helpful” pushes in non-interactive mode unless explicitly confirmed.
