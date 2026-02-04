package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type options struct {
	force       bool
	baseBranch  string
	noPush      bool
	commitDirty bool
	otherBranch string
}

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var uerr usageError
		if errors.As(err, &uerr) {
			fmt.Fprintln(os.Stderr, uerr.Err)
			_ = printUsage(ctx, os.Stderr)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type usageError struct {
	Err error
}

func (e usageError) Error() string {
	return e.Err.Error()
}

func (e usageError) Unwrap() error {
	return e.Err
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	opts, showHelp, err := parseArgs(args)
	if err != nil {
		return usageError{Err: err}
	}
	if showHelp {
		return printUsage(ctx, stdout)
	}

	currentBranch, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	user := os.Getenv("USER")
	if user == "" {
		return errors.New("USER is not set")
	}

	if opts.baseBranch != "" {
		opts.force = true
	}

	if opts.baseBranch == "" {
		if err := requireUserBranch(opts.force, user, currentBranch); err != nil {
			return usageError{Err: err}
		}
	}

	if err := gitRun(ctx, "fetch"); err != nil {
		return err
	}

	switch {
	case opts.baseBranch != "":
		return runCreateBranch(ctx, opts, user)
	case opts.otherBranch == "":
		return runDiscovery(ctx, opts, currentBranch, stdout)
	default:
		return runMerge(ctx, opts, currentBranch, stdout)
	}
}

func parseArgs(args []string) (options, bool, error) {
	var opts options
	fs := flag.NewFlagSet("mob-consensus", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	help := fs.Bool("h", false, "show help")
	fs.BoolVar(&opts.force, "F", false, "force run even if not on a $USER/ branch")
	fs.StringVar(&opts.baseBranch, "b", "", "create new $USER/<twig> branch based on base branch")
	fs.BoolVar(&opts.noPush, "n", false, "no automatic push after commits")
	fs.BoolVar(&opts.commitDirty, "c", false, "commit existing uncommitted changes")

	if err := fs.Parse(args); err != nil {
		return options{}, false, err
	}
	rest := fs.Args()
	if len(rest) > 0 {
		opts.otherBranch = rest[0]
	}
	return opts, *help, nil
}

func printUsage(ctx context.Context, w io.Writer) error {
	currentBranch, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		currentBranch = "CURRENT_BRANCH"
	}
	twig := twigFromBranch(currentBranch)
	_, _ = fmt.Fprintf(w, `Usage: mob-consensus [-cFn] [-b BASE_BRANCH] [OTHER_BRANCH]

With no arguments, compare %s with other branches named */%s.

If OTHER_BRANCH is given, do a manual merge of OTHER_BRANCH into %s.

-F force run even if not on a $USER/ branch
-b create new branch named $USER/%s based on BASE_BRANCH
-n no automatic push after commit
-c commit existing uncommitted changes
`, currentBranch, twig, currentBranch, twig)
	return nil
}

func requireUserBranch(force bool, user, currentBranch string) error {
	if force {
		return nil
	}
	if strings.HasPrefix(currentBranch, user+"/") {
		return nil
	}
	return fmt.Errorf("mob-consensus: you aren't on a '%s/' branch", user)
}

func runCreateBranch(ctx context.Context, opts options, user string) error {
	if err := ensureClean(ctx, opts, true); err != nil {
		return err
	}

	twig := twigFromBranch(opts.baseBranch)
	newBranch := user + "/" + twig
	baseBranch := opts.baseBranch
	if strings.Contains(baseBranch, "remotes/") {
		if err := gitRun(ctx, "checkout", "-b", twig, baseBranch); err != nil {
			return err
		}
		baseBranch = twig
	}
	if err := gitRun(ctx, "checkout", "-b", newBranch, baseBranch); err != nil {
		return err
	}
	return gitRun(ctx, "push", "--set-upstream", "origin", newBranch)
}

func runDiscovery(ctx context.Context, opts options, currentBranch string, stdout io.Writer) error {
	if opts.commitDirty {
		if err := ensureClean(ctx, opts, false); err != nil {
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

		switch {
		case ahead != "":
			fmt.Fprintf(stdout, "%40s is ahead: %s\n", b, ahead)
		case behind != "":
			fmt.Fprintf(stdout, "%40s is behind: %s\n", b, behind)
		default:
			fmt.Fprintf(stdout, "%40s is synced\n", b)
		}
	}
	return nil
}

func runMerge(ctx context.Context, opts options, currentBranch string, stdout io.Writer) error {
	if err := ensureClean(ctx, opts, true); err != nil {
		return err
	}

	msgPath := filepath.Join(os.TempDir(), "mob-consensus.msg")
	if err := writeMergeMessage(ctx, msgPath, opts.otherBranch, currentBranch); err != nil {
		return err
	}

	if err := gitRun(ctx, "merge", "--no-commit", "--no-ff", opts.otherBranch); err != nil {
		if err := gitRun(ctx, "mergetool", "-t", "vimdiff"); err != nil {
			return err
		}
	}

	mergeMsgPath, err := gitOutputTrimmed(ctx, "rev-parse", "--git-path", "MERGE_MSG")
	if err != nil {
		return err
	}
	mergeMsgPath, err = filepath.Abs(mergeMsgPath)
	if err != nil {
		return err
	}
	mergeMsg, err := os.ReadFile(msgPath)
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
	return gitRun(ctx, "push")
}

func ensureClean(ctx context.Context, opts options, requireClean bool) error {
	status, err := gitOutputTrimmed(ctx, "status", "--porcelain")
	if err != nil {
		return err
	}
	if status == "" {
		return nil
	}

	fmt.Fprintln(os.Stdout, "you have uncommitted changes")
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
	return gitRun(ctx, "push")
}

func twigFromBranch(branch string) string {
	return path.Base(strings.TrimSpace(branch))
}

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

func writeMergeMessage(ctx context.Context, msgPath, otherBranch, currentBranch string) error {
	var buf strings.Builder
	fmt.Fprintf(&buf, "mob-consensus merge from %s onto %s\n\n", otherBranch, currentBranch)

	userEmail, err := gitOutputTrimmed(ctx, "config", "--get", "user.email")
	if err != nil {
		userEmail = ""
	}
	logOut, err := gitOutput(ctx, "log", ".."+otherBranch, "--pretty=format:Co-authored-by: %an <%ae>")
	if err != nil {
		return err
	}

	coauthors := coAuthorLines(logOut, userEmail)
	for _, line := range coauthors {
		buf.WriteString(line)
		buf.WriteString("\n")
	}

	return os.WriteFile(msgPath, []byte(buf.String()), 0o644)
}

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

func gitOutputTrimmed(ctx context.Context, args ...string) (string, error) {
	out, err := gitOutput(ctx, args...)
	return strings.TrimSpace(out), err
}

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
