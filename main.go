package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
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
	"text/template"
)

//go:embed usage.tmpl
var usageTemplate string

type options struct {
	force       bool
	baseBranch  string
	noPush      bool
	commitDirty bool
	otherBranch string
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			printPanic(os.Stderr, r)
			os.Exit(1)
		}
	}()

	ctx := context.Background()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var uerr usageError
		if errors.As(err, &uerr) {
			printError(os.Stderr, uerr.Err)
			_ = printUsage(ctx, os.Stderr)
			os.Exit(1)
		}
		printError(os.Stderr, err)
		os.Exit(1)
	}
}

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

func printPanic(w io.Writer, r any) {
	if err, ok := r.(error); ok {
		printError(w, err)
		return
	}
	fmt.Fprintln(w, r)
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
	user, err := branchUserFromEmail(ctx)
	if err != nil {
		return err
	}

	if opts.baseBranch != "" {
		opts.force = true
	}

	if opts.baseBranch == "" {
		if err := requireUserBranch(opts.force, user, currentBranch); err != nil {
			return usageError{Err: err}
		}
	}

	switch {
	case opts.baseBranch != "":
		return runCreateBranch(ctx, opts, user, stdout)
	case opts.otherBranch == "":
		if err := fetchSuggestedRemote(ctx, ""); err != nil {
			return err
		}
		return runDiscovery(ctx, opts, currentBranch, stdout)
	default:
		if err := fetchSuggestedRemote(ctx, opts.otherBranch); err != nil {
			return err
		}
		return runMerge(ctx, opts, currentBranch, stdout)
	}
}

func parseArgs(args []string) (options, bool, error) {
	var opts options
	fs := flag.NewFlagSet("mob-consensus", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	help := fs.Bool("h", false, "show help")
	helpLong := fs.Bool("help", false, "show help")
	fs.BoolVar(&opts.force, "F", false, "force run even if not on a <user>/ branch")
	fs.StringVar(&opts.baseBranch, "b", "", "create new <user>/<twig> branch based on base branch")
	fs.BoolVar(&opts.noPush, "n", false, "no automatic push after commits")
	fs.BoolVar(&opts.commitDirty, "c", false, "commit existing uncommitted changes")

	if err := fs.Parse(args); err != nil {
		return options{}, false, err
	}
	rest := fs.Args()
	if len(rest) > 0 {
		opts.otherBranch = rest[0]
	}
	return opts, *help || *helpLong, nil
}

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

	return fmt.Errorf("mob-consensus: multiple remotes configured (%s); set an upstream or fetch explicitly (e.g., git fetch <remote>)", strings.Join(remotes, ", "))
}

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

func requireUserBranch(force bool, user, currentBranch string) error {
	if force {
		return nil
	}
	if strings.HasPrefix(currentBranch, user+"/") {
		return nil
	}
	return fmt.Errorf("mob-consensus: you aren't on a '%s/' branch", user)
}

func runCreateBranch(ctx context.Context, opts options, user string, stdout io.Writer) error {
	if err := ensureClean(ctx, opts, true, stdout); err != nil {
		return err
	}

	twig := twigFromBranch(opts.baseBranch)
	newBranch := user + "/" + twig
	baseBranch := opts.baseBranch

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

	if err := gitRun(ctx, "checkout", "-b", newBranch, baseBranch); err != nil {
		return err
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next: push your branch when you're ready.")
	return printPushAdvice(ctx, stdout, newBranch)
}

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

func runMerge(ctx context.Context, opts options, currentBranch string, stdout io.Writer) error {
	if err := ensureClean(ctx, opts, true, stdout); err != nil {
		return err
	}

	mergeTarget, needsConfirm, err := resolveMergeTarget(ctx, opts.otherBranch)
	if err != nil {
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

func smartPush(ctx context.Context) error {
	upstream, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err == nil && upstream != "" {
		return gitRun(ctx, "push")
	}

	currentBranch, err := gitOutputTrimmed(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	if currentBranch == "" || currentBranch == "HEAD" {
		return errors.New("mob-consensus: cannot push from detached HEAD")
	}

	branchPushRemote, err := gitOutputTrimmed(ctx, "config", "--get", "branch."+currentBranch+".pushRemote")
	if err == nil && branchPushRemote != "" {
		return gitRun(ctx, "push", "-u", branchPushRemote, currentBranch)
	}

	pushDefault, err := gitOutputTrimmed(ctx, "config", "--get", "remote.pushDefault")
	if err == nil && pushDefault != "" {
		return gitRun(ctx, "push", "-u", pushDefault, currentBranch)
	}

	remotesOut, err := gitOutputTrimmed(ctx, "remote")
	if err != nil {
		return errors.New("mob-consensus: cannot push: no git remotes configured (hint: git remote -v)")
	}

	var remotes []string
	for _, line := range strings.Split(remotesOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		remotes = append(remotes, line)
	}

	if len(remotes) == 1 {
		return gitRun(ctx, "push", "-u", remotes[0], currentBranch)
	}

	sort.Strings(remotes)
	return fmt.Errorf(
		"mob-consensus: cannot push: no upstream is set for branch %q and multiple remotes exist: %s (hint: git push -u <remote> %s; or: git config --local remote.pushDefault <remote>)",
		currentBranch,
		strings.Join(remotes, ", "),
		currentBranch,
	)
}

func resolveMergeTarget(ctx context.Context, otherBranch string) (string, bool, error) {
	if _, err := gitOutput(ctx, "rev-parse", "--verify", otherBranch); err == nil {
		return otherBranch, false, nil
	}

	remotesOut, err := gitOutputTrimmed(ctx, "remote")
	if err != nil {
		return "", false, fmt.Errorf("mob-consensus: branch %q not found locally and no remotes configured (hint: git remote -v)", otherBranch)
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
		return "", false, fmt.Errorf("mob-consensus: branch %q not found locally and no remotes configured (hint: git remote -v)", otherBranch)
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
		sort.Strings(remotes)
		return "", false, fmt.Errorf(
			"mob-consensus: branch %q not found locally or on any remote (%s) (hint: git fetch --all; or use an explicit ref like <remote>/%s)",
			otherBranch,
			strings.Join(remotes, ", "),
			otherBranch,
		)
	default:
		sort.Strings(candidates)
		return "", false, fmt.Errorf(
			"mob-consensus: branch %q is ambiguous; found multiple candidates: %s (use an explicit ref)",
			otherBranch,
			strings.Join(candidates, ", "),
		)
	}
}

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
