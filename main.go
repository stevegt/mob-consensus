// mob-consensus is a Git workflow helper optimized for mob/pair sessions.
//
// The tool assumes a convention where each collaborator works on a personal
// branch named "<user>/<twig>", where:
//   - <user> is derived from repo-local `git config user.email` (left of '@')
//   - <twig> is a shared suffix used to group related branches (ex: "feature-x")
//
// This Go implementation intentionally shells out to `git` for all repository
// operations. All `git` commands run in the current working directory, so
// callers/tests must `chdir` into the target repo before invoking the CLI.
package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// usageTemplate is rendered for `mob-consensus -h` and for usage errors.
// It's embedded so the binary is self-contained and help text is maintained as
// a readable template rather than line-by-line print calls.
//
//go:embed usage.tmpl
var usageTemplate string

// command labels an onboarding subcommand. It's used for composing prompts and
// error messages that mention the originating verb (init/start/join).
type command string

const (
	cmdInit  command = "init"
	cmdStart command = "start"
	cmdJoin  command = "join"
)

// options holds parsed flags and arguments. It is shared across commands so
// core workflow functions can be tested in-process without spawning a binary.
type options struct {
	// force bypasses the "<user>/" branch requirement for commands that normally
	// insist you're working on a personal branch.
	force bool
	// noPush disables automatic pushes after commits/merges (including -c
	// auto-commits).
	noPush bool
	// commitDirty allows mob-consensus to commit existing working tree changes
	// before continuing. When true, and noPush is false, mob-consensus also
	// pushes the auto-commit via smartPush.
	commitDirty bool
	// otherBranch is the merge target passed to `mob-consensus merge`.
	otherBranch string

	// twig is the shared coordination branch name (suffix) such as "feature-x".
	twig string
	// base is a git ref used by onboarding and branch creation (e.g., "main",
	// "origin/feature-x", "HEAD", or a commit SHA).
	base string
	// remote is the selected remote name for onboarding operations.
	remote string // fetch-only (used by init/start/join to locate twig)
	// plan prints a structured plan (commands + explanations) and exits.
	plan bool
	// dryRun prints the git commands that would run without executing them.
	dryRun bool
	// yes accepts defaults and skips confirmation prompts.
	yes bool
}

// exitFunc exists so tests can stub process exit without terminating the test
// process.
var exitFunc = os.Exit

// main delegates to mainExit so tests can exercise the CLI logic in-process.
func main() {
	exitFunc(mainExit(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// mainExit is the top-level entrypoint for CLI execution. It returns an exit
// code (instead of calling os.Exit) so it can be used by tests.
//
// Errors wrapped in usageError cause the help text to be printed.
func mainExit(ctx context.Context, args []string, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			printPanic(stderr, r)
			code = 1
		}
	}()

	if err := run(ctx, args, stdout, stderr); err != nil {
		var uerr usageError
		if errors.As(err, &uerr) {
			printError(stderr, uerr.Err)
			_ = printUsage(ctx, stderr)
			return 1
		}
		printError(stderr, err)
		return 1
	}

	return 0
}

// printError prints a human-readable message. If err implements Msg() string,
// that message is used (allowing structured errors to control presentation).
func printError(w io.Writer, err error) {
	if err == nil {
		return
	}

	type msgError interface {
		Msg() string
	}
	var me msgError
	if errors.As(err, &me) {
		fmt.Fprintln(w, me.Msg())
		return
	}
	fmt.Fprintln(w, err)
}

// printPanic formats panic values consistently with printError.
func printPanic(w io.Writer, r any) {
	if err, ok := r.(error); ok {
		printError(w, err)
		return
	}
	fmt.Fprintln(w, r)
}

// usageError marks an error as "show usage". It is used for argument/flag
// mistakes and other situations where the user needs guidance.
type usageError struct {
	Err error
}

// Error implements the error interface.
func (e usageError) Error() string {
	return e.Err.Error()
}

// Unwrap exposes the underlying error for errors.Is / errors.As.
func (e usageError) Unwrap() error {
	return e.Err
}

// branchNotFoundError is returned when the user names a merge target that
// cannot be resolved as a local ref or an unambiguous remote ref.
type branchNotFoundError struct {
	Branch  string
	Remotes []string
}

// Error implements the error interface with a machine-friendly message.
func (e branchNotFoundError) Error() string {
	if len(e.Remotes) == 0 {
		return fmt.Sprintf("mob-consensus: branch %q not found locally and no remotes configured (hint: git remote -v)", e.Branch)
	}

	remotes := append([]string(nil), e.Remotes...)
	sort.Strings(remotes)
	return fmt.Sprintf(
		"mob-consensus: branch %q not found locally or on any remote (%s) (hint: git fetch --all; or use an explicit ref like <remote>/%s)",
		e.Branch,
		strings.Join(remotes, ", "),
		e.Branch,
	)
}

// Msg returns a friendlier message than Error() and includes a next-step hint.
func (e branchNotFoundError) Msg() string {
	return fmt.Sprintf(
		"mob-consensus: branch %q does not exist.\n\nPick a branch name from the list above (the same list shown by running `mob-consensus status`), then re-run:\n  mob-consensus merge <branch>",
		e.Branch,
	)
}

// usageData is the data model for `usage.tmpl`. Keep this structure stable:
// tests and scripts depend on the wording and examples in the rendered output.
type usageData struct {
	CurrentBranch string
	Twig          string
	ExampleTwig   string

	User       string
	UserBranch string
	PeerBranch string
	PeerRef    string

	DerivedUser      string
	DerivedUserValid bool

	UserName     string
	UserEmail    string
	UserEmailSet bool

	Remote              string
	HasRemotes          bool
	RemoteIsPlaceholder bool
	RemoteSource        string
	Remotes             string
}

// printUsage renders usage.tmpl with repo-specific context (current branch,
// derived <user>, and the available/configured remotes).
func printUsage(ctx context.Context, w io.Writer) error {
	currentBranch, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		currentBranch = ""
	}

	twig := "twig"
	if currentBranch != "" {
		twig = twigFromBranch(currentBranch)
	}

	exampleTwig := "feature-x"
	if currentBranch != "" && currentBranch != "HEAD" && twig != "" {
		switch twig {
		case "main", "master":
		default:
			exampleTwig = twig
		}
	}

	userName, _ := gitOutputTrimmed(ctx, "config", "--get", "user.name")
	userEmail, _ := gitOutputTrimmed(ctx, "config", "--get", "user.email")
	userEmailSet := userEmail != ""

	derivedUser := ""
	derivedUserValid := false
	if userEmailSet {
		derivedUser = strings.TrimSpace(userEmail)
		if at := strings.IndexByte(derivedUser, '@'); at >= 0 {
			derivedUser = derivedUser[:at]
		}
		derivedUser = strings.TrimSpace(derivedUser)
		if derivedUser != "" {
			probe := derivedUser + "/probe"
			if _, err := gitOutput(ctx, "check-ref-format", "--branch", probe); err == nil {
				derivedUserValid = true
			}
		}
	}

	user := "alice"
	if derivedUserValid {
		user = derivedUser
	}

	remote, remotes, remoteSource := suggestedRemote(ctx)
	remoteIsPlaceholder := remote == ""
	if remoteIsPlaceholder {
		remote = "<remote>"
	}

	peerUser := "bob"
	if user == peerUser {
		peerUser = "alice"
	}

	data := usageData{
		CurrentBranch: currentBranch,
		Twig:          twig,
		ExampleTwig:   exampleTwig,

		User:       user,
		UserBranch: user + "/" + exampleTwig,
		PeerBranch: peerUser + "/" + exampleTwig,
		PeerRef:    remote + "/" + peerUser + "/" + exampleTwig,

		DerivedUser:      derivedUser,
		DerivedUserValid: derivedUserValid,

		UserName:     userName,
		UserEmail:    userEmail,
		UserEmailSet: userEmailSet,

		Remote:              remote,
		HasRemotes:          len(remotes) > 0,
		RemoteIsPlaceholder: remoteIsPlaceholder,
		RemoteSource:        remoteSource,
		Remotes:             strings.Join(remotes, ", "),
	}

	tmpl, err := template.New("usage").Option("missingkey=error").Parse(usageTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, data)
}

// suggestedRemote returns a default remote name when the choice is unambiguous,
// plus the full remote list and a short explanation of the selection.
//
// Policy: never assume `origin`. We only select a remote automatically when:
//   - the current branch has an upstream remote, or
//   - there is exactly one configured remote.
func suggestedRemote(ctx context.Context) (string, []string, string) {
	remotes, err := listRemotes(ctx)
	if err != nil {
		return "", nil, ""
	}
	if len(remotes) == 0 {
		return "", nil, ""
	}

	upstream, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil && upstream != "" {
		if i := strings.IndexByte(upstream, '/'); i > 0 {
			upstreamRemote := upstream[:i]
			for _, r := range remotes {
				if r == upstreamRemote {
					return upstreamRemote, remotes, "from current branch upstream"
				}
			}
		}
	}

	if len(remotes) == 1 {
		return remotes[0], remotes, "only configured remote"
	}

	return "", remotes, ""
}

// listRemotes returns configured remote names (as shown by `git remote`).
func listRemotes(ctx context.Context) ([]string, error) {
	remotesOut, err := gitOutputTrimmed(ctx, "remote")
	if err != nil {
		return nil, err
	}
	if remotesOut == "" {
		return nil, nil
	}

	var remotes []string
	for _, line := range strings.Split(remotesOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		remotes = append(remotes, line)
	}
	return remotes, nil
}

// registryRemoteURL returns the remote URL for the given collaborator id from
// the repo-tracked registry (.mob-consensus/u/<id>/remote.url). It returns an
// empty string if no entry exists.
func registryRemoteURL(id string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", nil
	}
	path := filepath.Join(".mob-consensus", "u", id, "remote.url")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ensurePushRemote picks (or creates) a push remote for the current user and
// returns the chosen remote. It does not write git config; pushes set upstream
// implicitly via `git push -u`.
//
// Selection order:
//  1. if only one remote exists, use it
//  2. if a remote named <user> exists, use it
//  3. if registry has .mob-consensus/u/<user>/remote.url, ensure a remote named
//     <user> exists with that URL (add or set-url), then use it
//
// Otherwise return an error listing available remotes and instructions to add
// the user's remote.
func ensurePushRemote(ctx context.Context, user string) (string, error) {
	remotes, err := listRemotes(ctx)
	if err != nil {
		return "", err
	}
	if len(remotes) == 1 {
		return remotes[0], nil
	}
	for _, r := range remotes {
		if r == user {
			return r, nil
		}
	}

	// Try collaborator registry.
	url, err := registryRemoteURL(user)
	if err != nil {
		return "", err
	}
	if url != "" {
		// Add or update the remote named <user>.
		if err := gitRun(ctx, "remote", "add", user, url); err != nil {
			// remote might already exist with a different URL; set-url in that case.
			_ = gitRun(ctx, "remote", "set-url", user, url)
		}
		return user, nil
	}

	if len(remotes) == 0 {
		return "", errors.New("mob-consensus: no git remotes configured; add your remote (e.g., git remote add origin <url>)")
	}

	sort.Strings(remotes)
	return "", fmt.Errorf("mob-consensus: cannot determine your push remote; add/rename a remote to %q (available: %s)", user, strings.Join(remotes, ", "))
}

// fetchSuggestedRemote updates remotes so subsequent operations see fresh refs.
//
// Policy:
//   - If otherBranch is prefixed with a remote (ex: "jj/alice/feature-x"),
//     fetch that remote.
//   - Else if exactly one remote exists, fetch it.
//   - Else fetch all remotes (`git fetch --all`) so multi-remote discovery/merge
//     can see peer branches.
//
// Fetch failures are fatal.
func fetchSuggestedRemote(ctx context.Context, otherBranch string) error {
	remotes, err := listRemotes(ctx)
	if err != nil {
		return err
	}
	if len(remotes) == 0 {
		return errors.New("mob-consensus: no remotes configured (hint: git remote -v)")
	}

	if otherBranch != "" {
		if i := strings.IndexByte(otherBranch, '/'); i > 0 {
			prefix := otherBranch[:i]
			for _, r := range remotes {
				if r == prefix {
					return gitRun(ctx, "fetch", r)
				}
			}
		}
	}

	upstream, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil && upstream != "" {
		if i := strings.IndexByte(upstream, '/'); i > 0 {
			upstreamRemote := upstream[:i]
			for _, r := range remotes {
				if r == upstreamRemote {
					return gitRun(ctx, "fetch", upstreamRemote)
				}
			}
		}
	}

	if len(remotes) == 1 {
		return gitRun(ctx, "fetch", remotes[0])
	}

	// Multi-remote: update all so peer branches are visible.
	return gitRun(ctx, "fetch", "--all")
}

// branchUserFromEmail derives the "<user>" branch prefix from repo-local
// `git config user.email` (left of '@') and validates that it can be used in a
// branch name.
func branchUserFromEmail(ctx context.Context) (string, error) {
	email, err := gitOutputTrimmed(ctx, "config", "--get", "user.email")
	if err != nil || strings.TrimSpace(email) == "" {
		return "", errors.New("mob-consensus: git user.email is not set (hint: git config --local user.email alice@example.com)")
	}

	email = strings.TrimSpace(email)
	user := email
	if at := strings.IndexByte(email, '@'); at >= 0 {
		user = email[:at]
	}
	user = strings.TrimSpace(user)
	if user == "" {
		return "", fmt.Errorf("mob-consensus: could not derive a username from git user.email=%q", email)
	}

	probe := user + "/probe"
	if _, err := gitOutput(ctx, "check-ref-format", "--branch", probe); err != nil {
		return "", fmt.Errorf("mob-consensus: derived username %q (from git user.email=%q) produces an invalid branch name", user, email)
	}

	return user, nil
}

// requireUserBranch enforces the "<user>/" personal-branch convention for
// commands that operate on a collaborator branch. Use -F/--force to override.
func requireUserBranch(force bool, user, currentBranch string) error {
	if force {
		return nil
	}
	if strings.HasPrefix(currentBranch, user+"/") {
		return nil
	}
	return fmt.Errorf("mob-consensus: you aren't on a '%s/' branch", user)
}

// gitPlanStep is one step in an onboarding plan. Steps are expressed as git
// subcommand args and can be printed (--plan/--dry-run) or executed.
type gitPlanStep struct {
	Explain string
	Pre     func(ctx context.Context) error
	Args    func(ctx context.Context) ([]string, error)
}

// runGitPlan prints or executes an ordered list of git commands with
// explanations. This powers init/start/join so the tool can both:
//   - show an exact copy/paste plan (`--plan`) and
//   - execute the same plan interactively (default) or non-interactively (`--yes`).
func runGitPlan(ctx context.Context, opts options, title string, steps []gitPlanStep, stdout, stderr io.Writer) error {
	if opts.plan {
		fmt.Fprintln(stdout, title)
		for i, step := range steps {
			args, err := step.Args(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "  %d) %s\n", i+1, step.Explain)
			fmt.Fprintf(stdout, "       git %s\n", strings.Join(args, " "))
		}
		return nil
	}
	if opts.dryRun {
		for _, step := range steps {
			args, err := step.Args(ctx)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "git %s\n", strings.Join(args, " "))
		}
		return nil
	}

	fmt.Fprintln(stdout, title)
	for i, step := range steps {
		if step.Pre != nil {
			if err := step.Pre(ctx); err != nil {
				return err
			}
		}
		args, err := step.Args(ctx)
		if err != nil {
			return err
		}

		fmt.Fprintf(stdout, "\nStep %d/%d: %s\n", i+1, len(steps), step.Explain)
		fmt.Fprintf(stdout, "  git %s\n", strings.Join(args, " "))

		if !opts.yes {
			ok, err := confirm(os.Stdin, stderr, "Run this? [y/N]: ")
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("mob-consensus: aborted")
			}
		}

		if err := gitRun(ctx, args...); err != nil {
			return err
		}
	}

	return nil
}

// isDirty reports whether the working tree has changes (tracked or untracked)
// as reported by `git status --porcelain`.
func isDirty(ctx context.Context) (bool, error) {
	status, err := gitOutputTrimmed(ctx, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return status != "", nil
}

// resolveTwig determines the shared twig name for onboarding commands.
//
// Priority:
//  1. explicit --twig
//  2. infer from the current branch name
//     - if on "<user>/<twig>", use that twig
//     - if on a non-main branch, reuse its basename as the twig
//  3. prompt the user (interactive mode only)
//
// In non-interactive plan/dry-run/--yes mode, twig must be unambiguous or
// passed explicitly.
func resolveTwig(cmd command, opts options, currentBranch, user string, stderr io.Writer) (string, error) {
	if strings.TrimSpace(opts.twig) != "" {
		return strings.TrimSpace(opts.twig), nil
	}

	inferFromCurrent := func() string {
		if currentBranch == "" || currentBranch == "HEAD" {
			return ""
		}
		if strings.HasPrefix(currentBranch, user+"/") {
			return twigFromBranch(currentBranch)
		}
		twig := twigFromBranch(currentBranch)
		switch twig {
		case "", "main", "master":
			return ""
		default:
			return twig
		}
	}

	if inferred := inferFromCurrent(); inferred != "" {
		return inferred, nil
	}

	interactive := !opts.yes && !opts.plan && !opts.dryRun
	if !interactive {
		return "", fmt.Errorf("mob-consensus: %s requires --twig (example: mob-consensus %s --twig feature-x)", cmd, cmd)
	}

	def := "feature-x"
	fmt.Fprintf(stderr, "Twig name (shared branch): [%s]: ", def)
	in, err := promptString(os.Stdin)
	if err != nil {
		return "", err
	}
	in = strings.TrimSpace(in)
	if in == "" {
		return def, nil
	}
	return in, nil
}

// resolveBase selects the base ref for `start`. It defaults to the current
// branch name (when not detached) unless --base is provided.
func resolveBase(opts options, currentBranch string) string {
	if strings.TrimSpace(opts.base) != "" {
		return strings.TrimSpace(opts.base)
	}
	return strings.TrimSpace(currentBranch)
}

// resolveRemote selects a remote to use for onboarding fetch operations.
//
// Priority:
//  1. explicit --remote (must exist)
//  2. if unambiguous: upstream remote or only remote
//  3. prompt the user (interactive mode only)
//
// In non-interactive plan/dry-run/--yes mode, the remote must be unambiguous or
// passed explicitly.
func resolveRemote(ctx context.Context, cmd command, opts options, stderr io.Writer) (string, error) {
	remotes, err := listRemotes(ctx)
	if err != nil {
		return "", err
	}
	if len(remotes) == 0 {
		return "", errors.New("mob-consensus: no remotes configured (hint: git remote -v)")
	}

	if strings.TrimSpace(opts.remote) != "" {
		r := strings.TrimSpace(opts.remote)
		for _, remote := range remotes {
			if remote == r {
				return r, nil
			}
		}
		sort.Strings(remotes)
		return "", fmt.Errorf("mob-consensus: remote %q not found; available remotes: %s", r, strings.Join(remotes, ", "))
	}

	remote, remotes, _ := suggestedRemote(ctx)
	if remote != "" {
		return remote, nil
	}

	interactive := !opts.yes && !opts.plan && !opts.dryRun
	sort.Strings(remotes)
	if !interactive {
		return "", fmt.Errorf("mob-consensus: %s requires --remote when multiple remotes exist (%s)", cmd, strings.Join(remotes, ", "))
	}

	fmt.Fprintf(stderr, "Pick remote for fetch (%s): ", strings.Join(remotes, ", "))
	in, err := promptString(os.Stdin)
	if err != nil {
		return "", err
	}
	in = strings.TrimSpace(in)
	for _, r := range remotes {
		if in == r {
			return r, nil
		}
	}
	return "", fmt.Errorf("mob-consensus: unknown remote %q (available: %s)", in, strings.Join(remotes, ", "))
}

// validateBranchName validates a *branch name* (not an arbitrary ref) using
// `git check-ref-format --branch`.
func validateBranchName(ctx context.Context, label, branch string) error {
	if strings.TrimSpace(branch) == "" {
		return fmt.Errorf("mob-consensus: %s is empty", label)
	}
	if _, err := gitOutput(ctx, "check-ref-format", "--branch", branch); err != nil {
		return fmt.Errorf("mob-consensus: invalid %s %q", label, branch)
	}
	return nil
}

// gitRefExists returns whether a fully-qualified ref name exists, using
// `git show-ref --verify --quiet`.
func gitRefExists(ctx context.Context, ref string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", ref)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git show-ref --verify --quiet %s: %w", ref, err)
}

// localBranchExists returns whether refs/heads/<branch> exists.
func localBranchExists(ctx context.Context, branch string) (bool, error) {
	return gitRefExists(ctx, "refs/heads/"+branch)
}

// remoteTrackingBranchExists returns whether refs/remotes/<remote>/<branch>
// exists locally.
func remoteTrackingBranchExists(ctx context.Context, remote, branch string) (bool, error) {
	return gitRefExists(ctx, "refs/remotes/"+remote+"/"+branch)
}

// runInit implements `mob-consensus init`. It fetches remote refs, checks
// whether the shared twig exists on the remote, then suggests (or runs) either
// `start` (first member) or `join` (next members).
//
// `init` intentionally does not assume a base ref unless it needs to run the
// `start` path; this keeps `init` usable in detached-HEAD repos when the twig
// already exists and `join` is sufficient.
func runInit(ctx context.Context, opts options, user, currentBranch string, stdout, stderr io.Writer) error {
	if opts.plan || opts.dryRun {
		dirty, err := isDirty(ctx)
		if err != nil {
			return err
		}
		if dirty {
			return usageError{Err: errors.New("mob-consensus: working tree is dirty (clean it before using --plan/--dry-run)")}
		}
	} else {
		execOpts := opts
		execOpts.noPush = true
		if err := ensureClean(ctx, execOpts, true, stdout); err != nil {
			return err
		}
	}

	twig, err := resolveTwig(cmdInit, opts, currentBranch, user, stderr)
	if err != nil {
		return usageError{Err: err}
	}
	if err := validateBranchName(ctx, "twig", twig); err != nil {
		return usageError{Err: err}
	}

	remote, err := resolveRemote(ctx, cmdInit, opts, stderr)
	if err != nil {
		return usageError{Err: err}
	}

	title := fmt.Sprintf("mob-consensus init (twig=%s, remote=%s)", twig, remote)
	if opts.plan || opts.dryRun {
		baseSuggestion := resolveBase(opts, currentBranch)
		baseHint := ""
		if baseSuggestion == "" || baseSuggestion == "HEAD" {
			baseSuggestion = "<ref>"
			baseHint = " (hint: pass --base <ref>)"
		}

		fmt.Fprintln(stdout, title)
		fmt.Fprintf(stdout, "  1) Fetch remote refs:\n       git fetch %s\n", remote)
		fmt.Fprintf(stdout, "  2) If %s/%s exists, run: mob-consensus join --twig %s\n", remote, twig, twig)
		fmt.Fprintf(stdout, "     Otherwise run:        mob-consensus start --twig %s --base %s%s\n", twig, baseSuggestion, baseHint)
		return nil
	}

	fetchStep := []gitPlanStep{
		{
			Explain: fmt.Sprintf("Fetch remote refs from %s", remote),
			Args: func(ctx context.Context) ([]string, error) {
				return []string{"fetch", remote}, nil
			},
		},
	}
	if err := runGitPlan(ctx, opts, title, fetchStep, stdout, stderr); err != nil {
		return err
	}

	exists, err := remoteTrackingBranchExists(ctx, remote, twig)
	if err != nil {
		return err
	}

	nextCmd := cmdStart
	if exists {
		nextCmd = cmdJoin
	}

	base := ""
	if nextCmd == cmdStart {
		base = resolveBase(opts, currentBranch)
		if base == "" || base == "HEAD" {
			return usageError{Err: errors.New("mob-consensus: could not determine a base ref (hint: pass --base <ref>)")}
		}
	}

	if !opts.yes {
		action := "start"
		if nextCmd == cmdJoin {
			action = "join"
		}
		ok, err := confirm(os.Stdin, stderr, fmt.Sprintf("Suggested: mob-consensus %s --twig %s (remote=%s). Continue? [y/N]: ", action, twig, remote))
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("mob-consensus: aborted")
		}
	}

	next := opts
	next.twig = twig
	next.remote = remote
	next.base = base

	switch nextCmd {
	case cmdJoin:
		return runJoin(ctx, next, user, currentBranch, stdout, stderr)
	default:
		return runStart(ctx, next, user, currentBranch, stdout, stderr)
	}
}

// runStart implements the "first group member" onboarding flow:
//  1. fetch
//  2. create the shared twig branch from a base ref
//  3. push the twig (required so others can join)
//  4. create/switch to the user's personal branch (<user>/<twig>)
//  5. push the personal branch
func runStart(ctx context.Context, opts options, user, currentBranch string, stdout, stderr io.Writer) error {
	if opts.plan || opts.dryRun {
		dirty, err := isDirty(ctx)
		if err != nil {
			return err
		}
		if dirty {
			return usageError{Err: errors.New("mob-consensus: working tree is dirty (clean it before using --plan/--dry-run)")}
		}
	} else {
		execOpts := opts
		execOpts.noPush = true
		if err := ensureClean(ctx, execOpts, true, stdout); err != nil {
			return err
		}
	}

	twig, err := resolveTwig(cmdStart, opts, currentBranch, user, stderr)
	if err != nil {
		return usageError{Err: err}
	}
	if err := validateBranchName(ctx, "twig", twig); err != nil {
		return usageError{Err: err}
	}

	remote, err := resolveRemote(ctx, cmdStart, opts, stderr)
	if err != nil {
		return usageError{Err: err}
	}

	base := resolveBase(opts, currentBranch)
	if base == "" || base == "HEAD" {
		return usageError{Err: errors.New("mob-consensus: could not determine a base ref (hint: pass --base <ref>)")}
	}

	userBranch := user + "/" + twig
	if err := validateBranchName(ctx, "personal branch", userBranch); err != nil {
		return usageError{Err: err}
	}

	pushRemote := remote
	if !opts.plan && !opts.dryRun {
		pushRemote, err = ensurePushRemote(ctx, user)
		if err != nil {
			return err
		}
	}

	title := fmt.Sprintf("mob-consensus start (twig=%s, base=%s, remote=%s, user=%s)", twig, base, remote, user)
	steps := []gitPlanStep{
		{
			Explain: fmt.Sprintf("Fetch remote refs from %s", remote),
			Args: func(ctx context.Context) ([]string, error) {
				return []string{"fetch", remote}, nil
			},
		},
		{
			Explain: fmt.Sprintf("Create/switch to shared twig branch %q", twig),
			Pre: func(ctx context.Context) error {
				localExists, err := localBranchExists(ctx, twig)
				if err != nil {
					return err
				}
				if localExists {
					return nil
				}
				remoteExists, err := remoteTrackingBranchExists(ctx, remote, twig)
				if err != nil {
					return err
				}
				if remoteExists {
					return usageError{Err: fmt.Errorf("mob-consensus: shared twig %q already exists on %s (hint: use `mob-consensus join --twig %s`)", twig, remote, twig)}
				}
				return nil
			},
			Args: func(ctx context.Context) ([]string, error) {
				exists, err := localBranchExists(ctx, twig)
				if err != nil {
					return nil, err
				}
				if exists {
					return []string{"checkout", twig}, nil
				}
				return []string{"checkout", "-b", twig, base}, nil
			},
		},
		{
			Explain: fmt.Sprintf("Push shared twig %q (required so others can join)", twig),
			Args: func(ctx context.Context) ([]string, error) {
				return []string{"push", "-u", pushRemote, twig}, nil
			},
		},
		{
			Explain: fmt.Sprintf("Create/switch to your personal branch %q", userBranch),
			Args: func(ctx context.Context) ([]string, error) {
				exists, err := localBranchExists(ctx, userBranch)
				if err != nil {
					return nil, err
				}
				if exists {
					return []string{"checkout", userBranch}, nil
				}

				remoteExists, err := remoteTrackingBranchExists(ctx, remote, userBranch)
				if err != nil {
					return nil, err
				}
				if remoteExists {
					return []string{"checkout", "-b", userBranch, remote + "/" + userBranch}, nil
				}
				return []string{"checkout", "-b", userBranch, twig}, nil
			},
		},
		{
			Explain: fmt.Sprintf("Push your personal branch %q", userBranch),
			Args: func(ctx context.Context) ([]string, error) {
				return []string{"push", "-u", pushRemote, userBranch}, nil
			},
		},
	}
	return runGitPlan(ctx, opts, title, steps, stdout, stderr)
}

// runJoin implements the "next group member" onboarding flow:
//  1. fetch
//  2. create a local twig branch tracking <remote>/<twig> (if needed)
//  3. create/switch to the user's personal branch (<user>/<twig>)
//  4. push the personal branch
func runJoin(ctx context.Context, opts options, user, currentBranch string, stdout, stderr io.Writer) error {
	if opts.plan || opts.dryRun {
		dirty, err := isDirty(ctx)
		if err != nil {
			return err
		}
		if dirty {
			return usageError{Err: errors.New("mob-consensus: working tree is dirty (clean it before using --plan/--dry-run)")}
		}
	} else {
		execOpts := opts
		execOpts.noPush = true
		if err := ensureClean(ctx, execOpts, true, stdout); err != nil {
			return err
		}
	}

	twig, err := resolveTwig(cmdJoin, opts, currentBranch, user, stderr)
	if err != nil {
		return usageError{Err: err}
	}
	if err := validateBranchName(ctx, "twig", twig); err != nil {
		return usageError{Err: err}
	}

	remote, err := resolveRemote(ctx, cmdJoin, opts, stderr)
	if err != nil {
		return usageError{Err: err}
	}

	userBranch := user + "/" + twig
	if err := validateBranchName(ctx, "personal branch", userBranch); err != nil {
		return usageError{Err: err}
	}

	pushRemote := remote
	if !opts.plan && !opts.dryRun {
		pushRemote, err = ensurePushRemote(ctx, user)
		if err != nil {
			return err
		}
	}

	title := fmt.Sprintf("mob-consensus join (twig=%s, remote=%s, user=%s)", twig, remote, user)
	steps := []gitPlanStep{
		{
			Explain: fmt.Sprintf("Fetch remote refs from %s", remote),
			Args: func(ctx context.Context) ([]string, error) {
				return []string{"fetch", remote}, nil
			},
		},
		{
			Explain: fmt.Sprintf("Create/switch to shared twig branch %q tracking %s/%s", twig, remote, twig),
			Pre: func(ctx context.Context) error {
				remoteExists, err := remoteTrackingBranchExists(ctx, remote, twig)
				if err != nil {
					return err
				}
				if !remoteExists {
					return usageError{Err: fmt.Errorf("mob-consensus: shared twig %q not found on %s (hint: ask the first member to run `mob-consensus start --twig %s`)", twig, remote, twig)}
				}
				return nil
			},
			Args: func(ctx context.Context) ([]string, error) {
				exists, err := localBranchExists(ctx, twig)
				if err != nil {
					return nil, err
				}
				if exists {
					return []string{"checkout", twig}, nil
				}
				return []string{"checkout", "-b", twig, remote + "/" + twig}, nil
			},
		},
		{
			Explain: fmt.Sprintf("Create/switch to your personal branch %q", userBranch),
			Args: func(ctx context.Context) ([]string, error) {
				exists, err := localBranchExists(ctx, userBranch)
				if err != nil {
					return nil, err
				}
				if exists {
					return []string{"checkout", userBranch}, nil
				}

				remoteExists, err := remoteTrackingBranchExists(ctx, remote, userBranch)
				if err != nil {
					return nil, err
				}
				if remoteExists {
					return []string{"checkout", "-b", userBranch, remote + "/" + userBranch}, nil
				}
				return []string{"checkout", "-b", userBranch, twig}, nil
			},
		},
		{
			Explain: fmt.Sprintf("Push your personal branch %q", userBranch),
			Args: func(ctx context.Context) ([]string, error) {
				return []string{"push", "-u", pushRemote, userBranch}, nil
			},
		},
	}
	return runGitPlan(ctx, opts, title, steps, stdout, stderr)
}

// runCreateBranch implements `mob-consensus branch create`.
//
// It creates (or switches to) the user's personal branch "<user>/<twig>" based
// on opts.base. This command does not push; instead it prints a suggested
// `git push -u ...` so the user can choose the remote explicitly when needed.
func runCreateBranch(ctx context.Context, opts options, user string, stdout io.Writer) error {
	twig := strings.TrimSpace(opts.twig)
	if twig == "" {
		return errors.New("mob-consensus: twig is empty")
	}
	if err := validateBranchName(ctx, "twig", twig); err != nil {
		return err
	}

	baseRef := strings.TrimSpace(opts.base)
	if baseRef == "" {
		return errors.New("mob-consensus: base ref is empty")
	}

	newBranch := user + "/" + twig
	if err := validateBranchName(ctx, "personal branch", newBranch); err != nil {
		return err
	}

	if err := ensureClean(ctx, opts, true, stdout); err != nil {
		return err
	}

	existingBranches, err := gitOutput(ctx, "branch", "--list", newBranch)
	if err != nil {
		return err
	}
	if strings.TrimSpace(existingBranches) != "" {
		if err := gitRun(ctx, "checkout", newBranch); err != nil {
			return err
		}
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Next: push your branch when you're ready.")
		return printPushAdvice(ctx, stdout, newBranch)
	}

	if err := gitRun(ctx, "checkout", "-b", newBranch, baseRef); err != nil {
		return err
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next: push your branch when you're ready.")
	return printPushAdvice(ctx, stdout, newBranch)
}

// printPushAdvice prints an explicit `git push -u ...` suggestion. If the
// remote choice is unambiguous (upstream remote or only remote) we print that
// remote; otherwise we print a placeholder and list available remotes.
func printPushAdvice(ctx context.Context, w io.Writer, branch string) error {
	remote, remotes, _ := suggestedRemote(ctx)
	if remote != "" {
		fmt.Fprintf(w, "  git push -u %s %s\n", remote, branch)
		return nil
	}

	fmt.Fprintf(w, "  git push -u <remote> %s\n", branch)
	switch len(remotes) {
	case 0:
		fmt.Fprintln(w, "  (Hint: git remote -v)")
	default:
		fmt.Fprintf(w, "  Available remotes: %s\n", strings.Join(remotes, ", "))
	}
	return nil
}

// runDiscovery implements `mob-consensus status`. It lists branches that end in
// "/<twig>" (for the current twig) and prints whether each branch is ahead,
// behind, diverged, or synced relative to the current branch.
func runDiscovery(ctx context.Context, opts options, currentBranch string, stdout io.Writer) error {
	if opts.commitDirty {
		if err := ensureClean(ctx, opts, false, stdout); err != nil {
			return err
		}
	}

	twig := twigFromBranch(currentBranch)
	out, err := gitOutput(ctx, "branch", "-a")
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Related branches and their diffs (if any):")
	fmt.Fprintln(stdout)

	for _, b := range relatedBranches(out, twig) {
		if b == currentBranch {
			continue
		}
		ahead, err := gitOutput(ctx, "diff", "--shortstat", "..."+b)
		if err != nil {
			return err
		}
		behind, err := gitOutput(ctx, "diff", "--shortstat", b+"...")
		if err != nil {
			return err
		}
		ahead = strings.TrimSpace(ahead)
		behind = strings.TrimSpace(behind)

		fmt.Fprintln(stdout, diffStatusLine(b, ahead, behind))
	}
	return nil
}

// diffStatusLine formats a single discovery line based on symmetric-diff
// shortstat outputs.
func diffStatusLine(branch, ahead, behind string) string {
	switch {
	case ahead != "" && behind != "":
		return fmt.Sprintf("%40s has diverged: ahead: %s; behind: %s", branch, ahead, behind)
	case ahead != "":
		return fmt.Sprintf("%40s is ahead: %s", branch, ahead)
	case behind != "":
		return fmt.Sprintf("%40s is behind: %s", branch, behind)
	default:
		return fmt.Sprintf("%40s is synced", branch)
	}
}

// runMerge implements `mob-consensus merge`.
//
// It resolves the merge target (including remote shorthand), enforces a clean
// tree (or auto-commits with -c), performs a no-ff/no-commit merge, launches
// tools for conflict resolution and review, writes MERGE_MSG, commits, and
// optionally pushes.
func runMerge(ctx context.Context, opts options, currentBranch string, stdout io.Writer) error {
	mergeTarget, needsConfirm, err := resolveMergeTarget(ctx, opts.otherBranch)
	if err != nil {
		var nf branchNotFoundError
		if errors.As(err, &nf) {
			// Mirror `mob-consensus status` by showing the related branch
			// list, so the user can pick a valid branch.
			_ = runDiscovery(ctx, options{}, currentBranch, stdout)
		}
		return err
	}

	if err := ensureClean(ctx, opts, true, stdout); err != nil {
		return err
	}
	if needsConfirm {
		ok, err := confirm(os.Stdin, os.Stderr, fmt.Sprintf("Resolved %q to %q. Merge this branch? [y/N]: ", opts.otherBranch, mergeTarget))
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("mob-consensus: merge aborted")
		}
	}

	mergeMsg, err := buildMergeMessage(ctx, mergeTarget, currentBranch)
	if err != nil {
		return err
	}

	gitDir, err := gitOutputTrimmed(ctx, "rev-parse", "--git-dir")
	if err != nil {
		return err
	}
	gitDir, err = filepath.Abs(gitDir)
	if err != nil {
		return err
	}
	msgFile, err := os.CreateTemp(gitDir, "mob-consensus-*.msg")
	if err != nil {
		return err
	}
	msgPath := msgFile.Name()
	defer os.Remove(msgPath)
	if _, err := msgFile.Write(mergeMsg); err != nil {
		_ = msgFile.Close()
		return err
	}
	if err := msgFile.Close(); err != nil {
		return err
	}

	mergeHeadPath, err := gitOutputTrimmed(ctx, "rev-parse", "--git-path", "MERGE_HEAD")
	if err != nil {
		return err
	}
	mergeHeadPath, err = filepath.Abs(mergeHeadPath)
	if err != nil {
		return err
	}

	mergeErr := gitRun(ctx, "merge", "--no-commit", "--no-ff", mergeTarget)
	if mergeErr != nil {
		if _, err := os.Stat(mergeHeadPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return mergeErr
			}
			return err
		}
		if err := gitRun(ctx, "mergetool", "-t", "vimdiff"); err != nil {
			return err
		}
		unmerged, _ := gitOutputTrimmed(ctx, "ls-files", "--unmerged")
		if unmerged != "" {
			return errors.New("mob-consensus: unresolved merge conflicts remain after mergetool")
		}
	}

	if _, err := os.Stat(mergeHeadPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	mergeMsgPath, err := gitOutputTrimmed(ctx, "rev-parse", "--git-path", "MERGE_MSG")
	if err != nil {
		return err
	}
	mergeMsgPath, err = filepath.Abs(mergeMsgPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(mergeMsgPath, mergeMsg, 0o644); err != nil {
		return err
	}

	if err := gitRun(ctx, "difftool", "-t", "vimdiff", "HEAD"); err != nil {
		return err
	}

	if err := gitRun(ctx, "commit", "-e", "-F", msgPath); err != nil {
		fmt.Fprintln(stdout, "don't forget to push")
		return err
	}

	if opts.noPush {
		fmt.Fprintln(stdout, "skipping automatic push -- don't forget to push later")
		return nil
	}
	return smartPush(ctx)
}

// ensureClean enforces a clean working tree before running an operation.
//
// If requireClean is false, the function will print a warning but allow the
// caller to continue. If opts.commitDirty is true, it will auto-commit dirty
// changes (and push unless opts.noPush is set).
func ensureClean(ctx context.Context, opts options, requireClean bool, stdout io.Writer) error {
	status, err := gitOutputTrimmed(ctx, "status", "--porcelain")
	if err != nil {
		return err
	}
	if status == "" {
		return nil
	}

	fmt.Fprintln(stdout, "you have uncommitted changes")
	if !opts.commitDirty {
		if requireClean {
			return errors.New("working tree is dirty (use -c to commit)")
		}
		return nil
	}

	if err := gitRun(ctx, "diff", "HEAD"); err != nil {
		return err
	}
	if err := gitRun(ctx, "commit", "-a"); err != nil {
		return err
	}
	if opts.noPush {
		return nil
	}
	return smartPush(ctx)
}

// smartPush pushes the current branch.
//
// If an upstream is already configured, it runs `git push`. Otherwise it will
// set an upstream with `git push -u <remote> <branch>` only when the remote is
// unambiguous (branch.<name>.pushRemote or a sole remote). If the remote choice
// is ambiguous it returns a clear error with exact commands the user can run.
func smartPush(ctx context.Context) error {
	currentBranch, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if currentBranch == "" || currentBranch == "HEAD" {
		return errors.New("mob-consensus: cannot push from detached HEAD")
	}

	upstream, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil && upstream != "" && upstream != "HEAD" {
		return gitRun(ctx, "push")
	}

	branchPushRemote, err := gitOutputTrimmed(ctx, "config", "--get", "branch."+currentBranch+".pushRemote")
	if err == nil && branchPushRemote != "" {
		return gitRun(ctx, "push", "-u", branchPushRemote, currentBranch)
	}

	remotes, err := listRemotes(ctx)
	if err != nil {
		return fmt.Errorf("mob-consensus: cannot list git remotes: %w", err)
	}

	if len(remotes) == 0 {
		return errors.New("mob-consensus: cannot push: no git remotes configured (hint: git remote -v)")
	}
	if len(remotes) == 1 {
		return gitRun(ctx, "push", "-u", remotes[0], currentBranch)
	}

	sort.Strings(remotes)
	return fmt.Errorf(
		"mob-consensus: cannot push: no upstream configured for %q and multiple remotes exist: %s (hint: git push -u <remote> %s)",
		currentBranch,
		strings.Join(remotes, ", "),
		currentBranch,
	)
}

// resolveMergeTarget resolves a user-supplied merge target.
//
// If otherBranch is a valid local ref, it is returned as-is. Otherwise we try
// to resolve it to exactly one "<remote>/<otherBranch>" among the configured
// remotes. When a remote candidate is selected, needsConfirm is true so the UI
// can ask the user to confirm the resolution.
func resolveMergeTarget(ctx context.Context, otherBranch string) (string, bool, error) {
	if _, err := gitOutput(ctx, "rev-parse", "--verify", otherBranch); err == nil {
		return otherBranch, false, nil
	}

	remotesOut, err := gitOutputTrimmed(ctx, "remote")
	if err != nil {
		return "", false, branchNotFoundError{Branch: otherBranch}
	}

	var remotes []string
	for _, line := range strings.Split(remotesOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		remotes = append(remotes, line)
	}
	if len(remotes) == 0 {
		return "", false, branchNotFoundError{Branch: otherBranch}
	}

	var candidates []string
	for _, remote := range remotes {
		candidate := remote + "/" + otherBranch
		if _, err := gitOutput(ctx, "rev-parse", "--verify", candidate); err == nil {
			candidates = append(candidates, candidate)
		}
	}

	switch len(candidates) {
	case 1:
		return candidates[0], true, nil
	case 0:
		return "", false, branchNotFoundError{Branch: otherBranch, Remotes: remotes}
	default:
		sort.Strings(candidates)
		return "", false, fmt.Errorf(
			"mob-consensus: branch %q is ambiguous; found multiple candidates: %s (use an explicit ref)",
			otherBranch,
			strings.Join(candidates, ", "),
		)
	}
}

// promptString reads one line (up to '\n') from in and returns it trimmed. It's
// used for interactive prompts.
func promptString(in io.Reader) (string, error) {
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// confirm prints a prompt and reads a yes/no response from in.
// Only "y" and "yes" (case-insensitive) are treated as confirmation.
func confirm(in io.Reader, out io.Writer, prompt string) (bool, error) {
	fmt.Fprint(out, prompt)
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	line = strings.TrimSpace(line)
	switch strings.ToLower(line) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// twigFromBranch extracts the twig from a branch name by taking the final path
// element. Examples:
//   - "alice/feature-x" => "feature-x"
//   - "feature-x"       => "feature-x"
func twigFromBranch(branch string) string {
	return path.Base(strings.TrimSpace(branch))
}

// relatedBranches filters the output of `git branch -a` to branches that end in
// "/<twig>". It ignores the current-branch marker "*" and symbolic-ref lines
// like "remotes/origin/HEAD -> origin/main".
func relatedBranches(branchAOutput, twig string) []string {
	var out []string
	for _, line := range strings.Split(branchAOutput, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if line == "" {
			continue
		}
		if strings.Contains(line, "->") {
			continue
		}
		if !strings.HasSuffix(line, "/"+twig) {
			continue
		}
		out = append(out, line)
	}
	return out
}

// buildMergeMessage builds the merge commit message used by runMerge.
//
// It includes a stable subject line (used by tests and tooling) and a
// deterministic set of `Co-authored-by:` trailers derived from commits in
// HEAD..otherBranch (excluding the current user's email when available).
func buildMergeMessage(ctx context.Context, otherBranch, currentBranch string) ([]byte, error) {
	var buf strings.Builder
	fmt.Fprintf(&buf, "mob-consensus merge from %s onto %s\n\n", otherBranch, currentBranch)

	userEmail, err := gitOutputTrimmed(ctx, "config", "--get", "user.email")
	if err != nil {
		userEmail = ""
	}
	logOut, err := gitOutput(ctx, "log", ".."+otherBranch, "--pretty=format:Co-authored-by: %an <%ae>")
	if err != nil {
		return nil, err
	}

	coauthors := coAuthorLines(logOut, userEmail)
	for _, line := range coauthors {
		buf.WriteString(line)
		buf.WriteString("\n")
	}

	return []byte(buf.String()), nil
}

// coAuthorLines parses `git log` output lines already formatted as
// `Co-authored-by: ...` and returns a sorted, de-duplicated list, optionally
// excluding lines containing excludeEmail.
func coAuthorLines(gitLogOutput, excludeEmail string) []string {
	seen := make(map[string]struct{})
	for _, line := range strings.Split(gitLogOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if excludeEmail != "" && strings.Contains(line, excludeEmail) {
			continue
		}
		seen[line] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for line := range seen {
		out = append(out, line)
	}
	sort.Strings(out)
	return out
}

// gitOutputTrimmed is gitOutput with surrounding whitespace trimmed.
func gitOutputTrimmed(ctx context.Context, args ...string) (string, error) {
	out, err := gitOutput(ctx, args...)
	return strings.TrimSpace(out), err
}

// gitOutput runs `git <args...>` and returns stdout.
//
// It captures stderr for error messages. The command runs in the current
// working directory; callers must ensure they have chdir'd into the target repo.
func gitOutput(ctx context.Context, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// gitRun runs `git <args...>` connected to the current process stdio. This is
// used for interactive commands like commit/mergetool/difftool.
func gitRun(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}
