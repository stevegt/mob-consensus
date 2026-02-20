# mob-consensus

`mob-consensus` is a Git workflow helper optimized for mob/pair sessions where each collaborator works on their own `<user>/<twig>` branch and repeatedly merges to converge.

The `<user>` prefix is derived from `git config user.email` (the part left of `@`). For testing you can use addresses like `alice@example.com`, `bob@example.com`, etc.

## Install / Upgrade

- Install latest: `go install github.com/stevegt/mob-consensus@latest`
- Install a version: `go install github.com/stevegt/mob-consensus@vX.Y.Z`

Minimum supported Go is 1.24.0.

## Usage

```
mob-consensus status [-cF]
mob-consensus merge  [-cFn] OTHER_BRANCH
mob-consensus branch create [-cn] TWIG [--from REF]
mob-consensus init  [-c] [--twig NAME] [--base REF] [--remote NAME] [--plan|--dry-run] [--yes]
mob-consensus start [-c] [--twig NAME] [--base REF] [--remote NAME] [--plan|--dry-run] [--yes]
mob-consensus join  [-c] [--twig NAME]            [--remote NAME] [--plan|--dry-run] [--yes]
```

- `status`: `git fetch`, then list related branches ending in `/<twig>` and show whether each is ahead/behind/diverged/synced.
- `merge OTHER_BRANCH`: perform a manual merge of `OTHER_BRANCH` onto the current branch, populate `MERGE_MSG` with `Co-authored-by:` lines, open mergetool/difftool, then commit and (optionally) push.
- `branch create TWIG [--from REF]`: create `<user>/<twig>` and switch to it. By default it branches from the current local branch (does not push; it prints a suggested `git push -u ...`).
- `start`: first group member onboarding (create + push shared twig, then create + push your `<user>/<twig>`).
- `join`: next group member onboarding (fetch, create local twig from `<remote>/<twig>`, then create + push your `<user>/<twig>`).
- `init`: fetch and suggest `start` vs `join`, then (optionally) run it.

Flags:
- `-F`: force run even if not on a `<user>/` branch
- `-c`: commit existing uncommitted changes (required for merge/branch-creation if the tree is dirty)
- `-n`: no automatic push after commits
- `--twig`, `--base`, `--remote`: inputs for `init`/`start`/`join`
- `--plan`: print the onboarding plan (commands + explanations) and exit
- `--dry-run`: print commands only; no prompts or execution
- `--yes`: accept defaults and run non-interactively
