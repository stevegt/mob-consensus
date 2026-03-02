# Post-Commit AI Reviews

Latest report only (replaces prior content).

## 2026-03-02 00:00:00 UTC — commit bfbe8d0b014901388279fa45f4ab973ff88c4d8e

Most critical next fixes (map to item IDs below):
- Address auto-push doc/code clarity (019.5)
- Add detached-HEAD and invalid email branch tests (019.2)
- Tame multi-remote fetch strategy (019.1)
- Broaden merge flow integration coverage (019.4)

Code Smells
- [019.1] The current `fetchSuggestedRemote` behavior falls back to `git fetch --all` whenever multiple remotes are configured and no single remote is inferred. This is functionally broad but operationally risky: it can make `status`/`merge` noticeably slower in multi-fork setups, can fail when one remote is intentionally inaccessible, and can introduce noisy errors unrelated to the branch the user is actually targeting. The command should prefer fetching only the remote needed for the active operation and keep `--all` as an explicit fallback path with clearer user intent.
- [019.2] `branchUserFromEmail` correctly enforces branch-safe usernames, but the failure modes around missing or invalid `user.email` are still brittle for real workflows and under-tested for recovery. Detached HEAD and malformed-email paths are especially important because they show up in CI, rebases, and ad-hoc maintenance sessions; right now the test surface does not fully prove that guidance remains consistent and actionable in these edge cases. Adding focused tests for those states would reduce regressions in onboarding and branch-create flows.
- [019.3] The test harness uses a global `exitFunc` override with panic-based capture, which is practical but creates shared mutable state across tests and subtests. If cleanup is missed in any path, later tests can inherit the overridden behavior and fail in confusing ways that look unrelated to the real defect. Isolating exit behavior per test helper or hardening reset discipline around every override would improve determinism and reduce flake risk as the integration suite grows.

Architecture
- [019.4] The CLI remains heavily shell-out centric, with workflow logic and command wiring still closely coupled in the same runtime path. That keeps the implementation straightforward, but it raises long-term architecture cost: unit testing is harder without process-level mocking, behavior can vary by host git environment, and future reuse as a library/API surface is constrained by Cobra-oriented control flow. A thinner command adapter over a testable git/workflow interface would preserve behavior while reducing maintenance and portability risk.

Code/Doc Drift
- [019.5] Documentation around push semantics is directionally correct but still easy to misread at speed, especially during conflict-heavy sessions when users expect conservative defaults. The implementation auto-pushes after commit paths unless `-n` is explicitly set, and that behavior needs to be called out in a more prominent, unambiguous way in both README and help text. Without that emphasis, users may assume a local-only merge/commit and accidentally publish intermediate states.
- [019.6] The docs still frame collaborator-registry behavior as planned, while the code already consults `.mob-consensus/u/<id>/remote.url` during push-remote selection. This mismatch can confuse users debugging remote resolution because they may not realize registry files are already active inputs to runtime behavior. Updating docs to describe what is currently implemented, how precedence works, and what errors look like when registry entries are absent or stale will close that drift and improve operator understanding.

Confidence: Medium — static read only; edge behaviors inferred.
