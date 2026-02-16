# TODO 012 - LLM-assisted review and human-in-the-loop approval

Context: `mob-consensus` exists to support collaborative, AI-assisted software
development. In that world, the heavy lifting (writing code, resolving conflicts,
drafting messages) can be done by AI agents, but **humans must be able to review,
test, and explicitly approve** changes before they land. “We’re all testers now.”

Also: one or more “mob-consensus users” may themselves be AI agents. The workflow
should support both:

- a human-in-the-loop approval gate for commits/merges/pushes, and
- an automation-friendly mode for agent orchestration (but still with explicit
  confirmation when a human is present).

Goal: add an interactive, Codex-like review loop that can explain diffs in
context and guide users through “approve / edit / abort”, with optional LLM
assistance. This should reduce reliance on `mergetool`/`difftool` and avoid
duplicate review (see TODO 011).

## Proposed UX (interactive review loop)

After a merge (clean or conflicted) — or before committing/pushing any changes:

- Gather a change set:
  - `git status --porcelain`
  - `git diff --name-status`, `git diff --stat`, and `git diff` vs `HEAD`
  - conflict markers / `git diff --name-only --diff-filter=U` if conflicts exist
- Present changes incrementally (per file or per hunk):
  - show the raw diff (or open it in pager/editor),
  - optionally show an LLM explanation of *what changed and why*,
  - optionally answer: “how does this fit into the larger repo context?”
- Ask the user what to do next:
  - approve this change and continue,
  - open editor (`$EDITOR`) to adjust the file,
  - re-run a tool (`git mergetool`, `git add -p`, etc.),
  - abort.
- Only after approval should `mob-consensus` commit (and optionally push).

Key principle: the LLM can recommend, summarize, and draft text, but it must not
be able to “self-approve” or bypass confirmations.

## Risks / constraints

- Privacy & secrets: diffs may contain sensitive data. LLM usage must be opt-in
  with clear disclosure of what gets sent and where. Support “offline/no-LLM”.
- Prompt injection: diff content can be adversarial; never treat it as trusted
  instructions. Keep approval decisions strictly user-confirmed.
- Cost/latency/network: remote calls can be slow or unavailable; degrade
  gracefully.
- Wrong summaries: user must be encouraged to inspect the raw diff and/or open
  the file.

## Plan

- [ ] 012.1 Define approval semantics:
  - [ ] 012.1.1 Per file vs per hunk vs per commit approval.
  - [ ] 012.1.2 What constitutes “approved enough” to commit/push.
- [ ] 012.2 Define review surfaces (no-LLM baseline):
  - [ ] 012.2.1 Raw diff presentation (`git diff`, pager, editor, or TUI).
  - [ ] 012.2.2 Edit loop (`$EDITOR`, `git add -p`, re-run merge tools).
- [ ] 012.3 Add optional LLM “review assistant”:
  - [ ] 012.3.1 Summarize changes per file/hunk.
  - [ ] 012.3.2 Answer “why/how does this fit in?” based on repo context.
  - [ ] 012.3.3 Draft commit messages (and co-author trailers when appropriate).
- [ ] 012.4 Define configuration and policy:
  - [ ] 012.4.1 Opt-in config (provider/model, on/off, redaction policy).
  - [ ] 012.4.2 Safe defaults for headless/CI (no LLM, no editor).
- [ ] 012.5 Guardrails:
  - [ ] 012.5.1 Prevent any “auto-approve” behavior; require explicit user confirmation.
  - [ ] 012.5.2 Prevent push unless explicitly approved.
  - [ ] 012.5.3 Ensure LLM output can’t be treated as commands.
- [ ] 012.6 Testing:
  - [ ] 012.6.1 `go test` coverage for approval-state transitions.
  - [ ] 012.6.2 `mc-test --interactive` scenario exercising the full review loop.

