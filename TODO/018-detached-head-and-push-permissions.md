# TODO 018 - Guard detached HEAD branching & guide fork push permissions

Context: Users sometimes work in detached HEAD state or clone someone else’s
repo, then try to push without permission or accidentally push to the original
repo after forking. We need clearer detection and guided fixes.

- [ ] 018.1 Prevent branch creation from detached HEAD
  - [ ] 018.1.1 In `branch create`, detect `HEAD` base when current branch is
        detached; abort with a friendly message and instructions to switch to a
        real branch or pass `--from <ref>` explicitly.
  - [ ] 018.1.2 Add tests (Go + mc-test) covering detached HEAD rejection.

- [ ] 018.2 Detect push permission errors on cloned upstream
  - [ ] 018.2.1 When `git push` fails with permission/denied, detect the common
        case of pushing to someone else’s repo (no write access).
  - [ ] 018.2.2 Present a guided fix: add user’s own remote, set upstream via
        `git push -u <my-remote> <branch>`, retry push.
  - [ ] 018.2.3 Add tests simulating no-push permission (e.g., remote rejecting
        pushes) to ensure the guidance appears and the exit code is non-zero.

- [ ] 018.3 Detect “cloned upstream then forked” pushing to wrong remote
  - [ ] 018.3.1 Heuristic: origin URL differs from user’s fork URL (from
        registry or config), and push failures/remote HEAD owner mismatch.
  - [ ] 018.3.2 Guide user to add their fork remote (e.g., `git remote add <user> <url>`)
        and set upstream with `git push -u <user> <branch>`.
  - [ ] 018.3.3 Add docs + usage text to clarify the flow for forked setups.
  - [ ] 018.3.4 Add tests (mc-test scenario) that clones upstream, then sets a
        fork remote and verifies the guidance chooses the fork for pushes.
