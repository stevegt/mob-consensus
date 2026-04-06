# TODO 018 - Guard detached HEAD branching & guide fork push permissions

Context: Users sometimes work in detached HEAD state or clone someone else’s
repo, then try to push without permission or accidentally push to the original
repo after forking. We need clearer detection and guided fixes.

## Decision Intent Log

ID: DI-018-20260309-174820
Date: 2026-03-09 17:48:20
Status: superseded
Decision: Detect push permission/auth failures in `smartPush` and return guided fork-remediation instructions instead of raw git failure text.
Intent: Keep collaborator workflows unblocked when a user clones a repo they cannot write to by providing concrete next-step commands.
Constraints: Preserve existing push policy (upstream > branch.pushRemote > sole remote), keep analysis read-only beyond runtime behavior, and avoid changing unrelated merge/status semantics.
Affects: `main.go` (`smartPush`, git push error handling), `main_integration_test.go` (permission-denied push coverage), TODO 018 tracking.

ID: DI-018-20260309-175205
Date: 2026-03-09 17:52:05
Status: superseded
Decision: On push permission/auth failure, present guidance for both shared-write and fork workflows instead of assuming fork-only remediation.
Intent: Avoid incorrect assumptions in mixed collaboration models where local state cannot reliably determine whether the repository is shared-write or fork-based.
Constraints: Keep `smartPush` policy unchanged, keep remediation commands concise, and preserve deterministic integration coverage for denied push behavior.
Affects: `main.go` (`pushPermissionGuidanceError` messaging), `main_integration_test.go` (`TestSmartPushPermissionDeniedGuidance` assertions), TODO 018 decision trail.
Supersedes: DI-018-20260309-174820

ID: DI-018-20260312-185440
Date: 2026-03-12 18:54:40
Status: superseded
Decision: Include exact captured git stderr in push permission/auth guidance errors, in addition to remediation steps.
Intent: Preserve low-level debugging evidence while still providing high-level workflow guidance for shared-write and fork cases.
Constraints: Keep existing `smartPush` decision policy unchanged and avoid masking non-permission failures.
Affects: `main.go` (`gitRunWithOutput`, `gitPushWithGuidance`, `pushPermissionGuidanceError`), `main_integration_test.go` (`TestSmartPushPermissionDeniedGuidance`), TODO 018 decision log.
Supersedes: DI-018-20260309-175205

ID: DI-018-20260330-235100
Date: 2026-03-30 23:51:00
Status: active
Decision: Align push-permission guidance with TODO 021 stabilization:
`mob-consensus` must never push peer-owned branches; when push is attempted on
own branch and fails, keep original git stderr and add guided remediation text.
Intent: Separate ownership policy errors from permission/auth failures and
avoid misleading guidance when branch ownership is already invalid.
Constraints: Enforce ownership guard before push attempt; preserve exact stderr
when push does run and fails; keep guidance concise and workflow-oriented.
Affects: `main.go` push ownership guard + permission guidance flow,
`main_integration_test.go`, `scripts/mc-test`, TODO 018/021 tracking.
Supersedes: DI-018-20260312-185440

- [ ] 018.1 Prevent branch creation from detached HEAD
  - [x] 018.1.1 In `branch create`, detect `HEAD` base when current branch is
        detached; abort with a friendly message and instructions to switch to a
        real branch or pass `--from <ref>` explicitly.
  - [x] 018.1.2 Add tests (Go + mc-test) covering detached HEAD rejection.
        Note: Added `mc-test` scenario `detached-branch` to cover rejection,
        idempotent existing-branch switch, and explicit `--from` recovery.

- [ ] 018.2 Detect push permission errors on cloned upstream
  - [x] 018.2.1 When `git push` fails with permission/denied, detect the common
        case of pushing to someone else’s repo (no write access).
  - [x] 018.2.2 Present a guided fix: add user’s own remote, set upstream via
        `git push -u <my-remote> <branch>`, retry push.
  - [x] 018.2.3 Add tests simulating no-push permission (e.g., remote rejecting
        pushes) to ensure the guidance appears and the exit code is non-zero.

- [ ] 018.3 Detect “cloned upstream then forked” pushing to wrong remote
  - [ ] 018.3.1 Heuristic: origin URL differs from user’s fork URL (from
        registry or config), and push failures/remote HEAD owner mismatch.
  - [ ] 018.3.2 Guide user to add their fork remote (e.g., `git remote add <user> <url>`)
        and set upstream with `git push -u <user> <branch>`.
  - [ ] 018.3.3 Add docs + usage text to clarify the flow for forked setups.
  - [ ] 018.3.4 Add tests (mc-test scenario) that clones upstream, then sets a
        fork remote and verifies the guidance chooses the fork for pushes.
