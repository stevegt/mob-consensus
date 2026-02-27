# TODO 016 - ntfy.sh messaging integration

Goal: add optional push notifications so collaborators (humans and/or agents) can learn about mob-consensus events without polling.

## Scope / non-goals
- Keep this **optional** and **config-driven**.
- Do **not** send diffs or file contents by default (avoid leaking private code to public ntfy servers).
- Avoid hard-coding `https://ntfy.sh`; allow self-hosted ntfy instances.

## User stories
- As a collaborator, I want a notification when someone pushes/merges to `TWIG` or `user/TWIG`.
- As a group, I want a notification when a merge requires human attention (conflicts / manual review needed).
- As an agent, I want a machine-readable message so I can decide whether to pull/review.

## Proposed UX / configuration
- Add repo config knobs (ties into TODO 008/009 config work):
  - `ntfy.server` (e.g. `https://ntfy.sh` or `https://ntfy.example.com`)
  - `ntfy.topic` (or multiple topics: `ntfy.topic.merge`, `ntfy.topic.claims`, etc.)
  - `ntfy.enabled` boolean
- Secrets should be **local-only**:
  - env vars like `MC_NTFY_TOKEN` (preferred) or local git config keys.
  - never require committing tokens into the repo.

## Message content (default)
Send minimal metadata:
- repo identifier (best-effort; e.g. remote URL host/path when available)
- event type (`merge`, `push`, `conflict`, `claim`, etc.)
- twig + branch (`feature-x`, `alice/feature-x`)
- commit subject + short SHA (when applicable)

## Failure policy
- Default: notification failures are **warnings** (do not change git correctness).
- Optional: `--notify-strict` (or config) to make notify failures fatal for teams that depend on it.

## Implementation plan
- [ ] 016.1 Define config keys and how to load them (prefer repo-local config + env-var secrets).
- [ ] 016.2 Implement `notify(ctx, event)` with `net/http` (timeouts, context, clear errors).
- [ ] 016.3 Hook notifications into `merge`, `start/join/init`, and claim workflows (if/when claim is implemented).
- [ ] 016.4 Add `mob-consensus notify test` (or similar) to validate configuration.
- [ ] 016.5 Add unit tests using `httptest.Server` (no external network) and integration coverage where useful.
- [ ] 016.6 Update `usage.tmpl` / README: how to configure + security guidance (public topics, random topics, auth).

