# TODO 015 - Cleaner interfaces for CLI, TUI, and Go API

Context: `mob-consensus` is moving to an explicit, conventional CLI:
- explicit subcommands: `init`, `start`, `join`, `status`, `merge`, `branch create` (TODO 006, TODO 015)
- hard breaks (intended): no “no-args discovery”, no positional merge, no `-b` branch-create flag.

As we add multi-remote support (TODO 008), claiming (TODO 009), richer
review/approval UX (TODO 011/012), and a possible TUI (TODO 014), the
current “mode by positional arg” approach becomes harder to learn and
harder to extend without surprises.

Goal: define a clean, conventional command hierarchy and a shared “engine” API so:
- humans get an intuitive CLI/TUI,
- AI agents can use the CLI non-interactively *or* call a Go API programmatically,
- tests (`go test`, `scripts/mc-test`) can drive stable interfaces.

## Design principles

- Prefer explicit verbs over implicit modes (ex: `mob-consensus merge
  <ref>` instead of `mob-consensus <ref>`).
- Preserve scriptability: `--plan`, `--dry-run`, `--yes` must remain
  first-class and never require a TTY.
- Single source of truth: build a **structured plan** (steps +
  explanations) once; UI layers only render/confirm/execute it.
- Avoid remote/name magic (align with TODO 006/008): only choose
  defaults when unambiguous; otherwise prompt or error with exact
  commands.

## Recommendation: adopt Cobra for the CLI

Pros:
- Conventional subcommands, flag scoping, and help output
  (`mob-consensus merge -h`).
- Easier to add logically-grouped features (`config`, `claim`, `tui`)
  without overloading positional args.
- Shell completion and clearer error messages.

Cons / risks:
- Adds a dependency and changes help text formatting (tests and docs
  must adapt).
- Requires a compatibility plan for existing users/scripts (see
  below).

## Proposed command tree (strawman)

Core workflow:
- `mob-consensus status` (or `discover`): show related branches +
  ahead/behind/diverged/synced.
- `mob-consensus merge <ref>`: merge `<ref>` into the current branch
  (supports multi-remote shorthand resolution per TODO 008).
- `mob-consensus branch create <twig> [--from <ref>]`:
  create/switch to `<user>/<twig>` from a base ref (replaces `-b`).
  - Recommended default: if `--from` is not provided, default it to
    the current local branch name (like a real user would do before
    running mob-consensus). If the repo is in detached HEAD, error
    with a clear hint to pass `--from <ref>`.
  - Recommended UX: print the exact operation (ex: “create
    alice/feature-x from feature-x”) and ask for confirmation unless
    `--yes` is set.

Onboarding (keep existing, but consider grouping):
- `mob-consensus init` (suggest start vs join)
- `mob-consensus start` (first member)
- `mob-consensus join` (next members)
- Optional grouping: `mob-consensus twig init|start|join` with
  top-level aliases for convenience.

Future (to align TODOs with namespacing):
- `mob-consensus config …` (repo-tracked collaborator config; TODO 008)
- `mob-consensus claim …` (work-item claiming; TODO 009)
- `mob-consensus tui` or `--tui` (optional; TODO 014)
  - Open question: should the TUI always be opt-in, or should
    `mob-consensus` with no args on a TTY always launch it?

### Flag conventions

Make these consistent across commands (global where possible):
- `--plan`, `--dry-run`, `--yes`
- `--no-push`, `--commit-dirty`
- `--remote` / `--push-remote` / `--fetch` (as needed for TODO 008)
- `--twig`, `--base` (onboarding and branch creation)

## Recommendation: define a Go “engine” API (shared by CLI/TUI/tests/agents)

Align with TODO 001.9: factor non-interactive logic into a small
package (no `internal/` or `pkg/` per repo conventions).

Suggested shape:
- Package name: short and explicit (e.g., `consensus` or `mc`).
- Input: structured options + a `Git` interface (shellouts behind an
  interface; go-git hybrid is optional per TODO 005).
- Output: structured `Plan` and/or structured “state” objects (related
  branches, merge target resolution, push advice).
- No direct prompting/printing inside the engine; UI layers own I/O.

This enables:
- CLI and Bubble Tea TUI to share the same plan/state machine.
- AI agents to call Go directly without parsing text output.
- Tests to run in-process for coverage (no need to spawn the CLI
  binary).

## Compatibility / migration plan

Decision: **hard break** (remove legacy modes immediately).

Rationale: today the main consumers are `go test` and `scripts/mc-test`,
so it’s better to pay the churn cost now and end up with a simpler,
less-surprising CLI.

## Testing impact

If command names/args change:
- Update `main_integration_test.go` to call new subcommands (ex:
  `merge bob/feature-x`).
- Update `scripts/mc-test` scenarios and any golden output parsing.
- Keep an explicit “CLI contract” section in tests to prevent
  accidental churn in UX-critical strings (especially `--plan`
  output).

## Subtasks

- [ ] 015.1 Write the CLI contract (commands, args, exit codes).
- [ ] 015.2 Decide naming: `status` vs `discover`, and `branch create` UX details:
  - Recommended: `mob-consensus branch create <twig> [--from <ref>]`.
  - Recommended: default `--from` to the current branch when possible;
    require `--from` in detached HEAD.
- [ ] 015.3 Pick a compatibility strategy (hard break vs soft migrate) and document it in `usage.tmpl`.
- [x] 015.8 Hard break step 1: require explicit `mob-consensus status` (no “no-args discovery”).
- [x] 015.9 Hard break step 2: require explicit `mob-consensus merge <ref>` (no positional merge).
- [x] 015.10 Hard break step 3: replace `-b` with `mob-consensus branch create <twig> [--from <ref>]`.
- [ ] 015.4 Introduce Cobra scaffolding and map commands to existing logic.
- [ ] 015.5 Define the engine package boundary (types + interfaces) and move non-interactive logic out of `main`.
- [ ] 015.6 Add TUI entrypoint hooks that call the same engine (coordinate with TODO 014).
- [ ] 015.7 Update tests + harness:
  - [ ] 015.7.1 `go test` integration tests updated for new CLI.
  - [ ] 015.7.2 `scripts/mc-test` scenarios updated.
  - [ ] 015.7.3 Coverage reports still written only under the harness root.
