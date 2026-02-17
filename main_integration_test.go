package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type friendlyError struct{}

func (friendlyError) Error() string { return "raw error" }
func (friendlyError) Msg() string   { return "friendly message" }

type exitCode int

// These integration tests try to mirror the Git commands shown in `usage.tmpl`
// so the exercised workflows match what real users do. When a test must deviate
// (compatibility, determinism, or to keep the test focused), explain why in an
// inline comment.

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	val, ok := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if !ok {
			_ = os.Unsetenv(key)
			return
		}
		_ = os.Setenv(key, val)
	})
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

func setupIsolatedGitEnv(t *testing.T) {
	t.Helper()
	// Prevent user environment variables from pointing git at a non-temp repo.
	for _, key := range []string{
		"GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_COMMON_DIR",
		"GIT_CEILING_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM",
		"GIT_CONFIG_GLOBAL",
		"GIT_CONFIG_SYSTEM",
	} {
		unsetEnv(t, key)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_PAGER", "cat")

	// Keep tests non-interactive.
	t.Setenv("GIT_EDITOR", "true")
}

func requireTempDir(t *testing.T, dir string) {
	t.Helper()
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	absTmp, err := filepath.Abs(os.TempDir())
	if err != nil {
		t.Fatalf("abs tmp path: %v", err)
	}
	absTmp = filepath.Clean(absTmp)
	absDir = filepath.Clean(absDir)
	prefix := absTmp + string(os.PathSeparator)
	if absDir != absTmp && !strings.HasPrefix(absDir, prefix) {
		t.Fatalf("refusing to operate outside os.TempDir (%s): %s", absTmp, absDir)
	}
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	requireTempDir(t, dir)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func gitInitMain(t *testing.T, dir string) {
	t.Helper()
	requireTempDir(t, dir)
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err == nil {
		return
	} else if len(out) > 0 {
		// Fall through. Some git versions don't support -b/--initial-branch.
	}

	cmd = exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "branch", "-M", "main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch -M main failed: %v\n%s", err, out)
	}
}

func writeFile(t *testing.T, dir, relPath, contents string) {
	t.Helper()
	requireTempDir(t, dir)
	path := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func withCwd(t *testing.T, dir string) {
	t.Helper()
	requireTempDir(t, dir)
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(old)
	})
}

func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		_ = r.Close()
		_ = w.Close()
		t.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		_ = r.Close()
	})
}

func configureRepo(t *testing.T, dir, name, email string) {
	t.Helper()
	requireTempDir(t, dir)
	gitCmd(t, dir, "config", "--local", "user.name", name)
	gitCmd(t, dir, "config", "--local", "user.email", email)
	gitCmd(t, dir, "config", "--local", "commit.gpgSign", "false")
	gitCmd(t, dir, "config", "--local", "difftool.prompt", "false")
	gitCmd(t, dir, "config", "--local", "mergetool.prompt", "false")
	gitCmd(t, dir, "config", "--local", "difftool.vimdiff.cmd", "true")
	gitCmd(t, dir, "config", "--local", "mergetool.vimdiff.cmd", "true")
}

func cloneRepo(t *testing.T, remote, name, email string) string {
	t.Helper()
	requireGit(t)
	setupIsolatedGitEnv(t)

	dir := filepath.Join(t.TempDir(), "clone")
	requireTempDir(t, dir)
	cmd := exec.Command("git", "clone", remote, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git clone %s failed: %v\n%s", remote, err, out)
	}
	configureRepo(t, dir, name, email)
	return dir
}

func gitSwitchCreate(t *testing.T, dir, branch string, startPoint ...string) {
	t.Helper()
	requireTempDir(t, dir)

	// `usage.tmpl` recommends `git switch -c`. Use it when available, but fall
	// back to `git checkout -b` for older Git versions (<2.23) so the tests run
	// on a wider range of systems.
	args := []string{"switch", "-c", branch}
	if len(startPoint) > 0 {
		args = append(args, startPoint[0])
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return
	}
	if strings.Contains(string(out), "is not a git command") {
		args = []string{"checkout", "-b", branch}
		if len(startPoint) > 0 {
			args = append(args, startPoint[0])
		}
		cmd = exec.Command("git", args...)
		cmd.Dir = dir
		out, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
		return
	}
	t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
}

func initRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	setupIsolatedGitEnv(t)

	dir := t.TempDir()
	gitInitMain(t, dir)
	configureRepo(t, dir, "Alice", "alice@example.com")

	writeFile(t, dir, "README.md", "seed\n")
	gitCmd(t, dir, "add", "README.md")
	gitCmd(t, dir, "commit", "-m", "seed")
	return dir
}

func initBareRemote(t *testing.T) string {
	t.Helper()
	requireGit(t)

	dir := filepath.Join(t.TempDir(), "remote.git")
	requireTempDir(t, dir)
	cmd := exec.Command("git", "init", "--bare", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}
	// Make the bare remote deterministic for clones regardless of the user's
	// global init.defaultBranch config.
	cmd = exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/main")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git symbolic-ref HEAD refs/heads/main failed: %v\n%s", err, out)
	}
	return dir
}

func writeCommitMessageEditor(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	editorPath := filepath.Join(dir, "git-editor.sh")
	requireTempDir(t, editorPath)
	script := `#!/usr/bin/env bash
set -euo pipefail
msg_file="${1:-}"
if [[ -z "$msg_file" ]]; then
  exit 0
fi
printf '%s\n' "test auto commit" >"$msg_file"
`
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor: %v", err)
	}
	return editorPath
}

func TestRunHelpOutsideRepo(t *testing.T) {
	requireGit(t)
	setupIsolatedGitEnv(t)

	dir := t.TempDir()
	withCwd(t, dir)

	var out bytes.Buffer
	if err := run(context.Background(), []string{"-h"}, &out, io.Discard); err != nil {
		t.Fatalf("run(-h) err=%v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Usage:\n  mob-consensus") {
		t.Fatalf("help output missing usage header:\n%s", got)
	}
	if !strings.Contains(got, "git config --local user.email") {
		t.Fatalf("help output missing identity hint:\n%s", got)
	}
}

func TestMainUsesExitFunc(t *testing.T) {
	setupIsolatedGitEnv(t)
	dir := t.TempDir()
	withCwd(t, dir)

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open devnull: %v", err)
	}

	oldExit := exitFunc
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = devNull
	os.Stderr = devNull
	exitFunc = func(code int) { panic(exitCode(code)) }

	t.Cleanup(func() {
		exitFunc = oldExit
		os.Args = oldArgs
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = devNull.Close()
	})

	runMain := func(args ...string) int {
		os.Args = append([]string{"mob-consensus"}, args...)
		code := -1
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("expected exit for args=%v", args)
				}
				c, ok := r.(exitCode)
				if !ok {
					t.Fatalf("unexpected panic: %T %v", r, r)
				}
				code = int(c)
			}()
			main()
		}()
		return code
	}

	if got := runMain("-h"); got != 0 {
		t.Fatalf("main -h exit=%d, want 0", got)
	}
	if got := runMain("--nope"); got != 1 {
		t.Fatalf("main --nope exit=%d, want 1", got)
	}
}

func TestBranchUserFromEmail(t *testing.T) {
	requireGit(t)
	setupIsolatedGitEnv(t)

	dir := t.TempDir()
	gitInitMain(t, dir)
	withCwd(t, dir)

	ctx := context.Background()
	if _, err := branchUserFromEmail(ctx); err == nil {
		t.Fatalf("expected error when user.email is unset")
	}

	gitCmd(t, dir, "config", "--local", "user.email", "alice@example.com")
	user, err := branchUserFromEmail(ctx)
	if err != nil {
		t.Fatalf("branchUserFromEmail() err=%v", err)
	}
	if user != "alice" {
		t.Fatalf("branchUserFromEmail()=%q, want %q", user, "alice")
	}

	gitCmd(t, dir, "config", "--local", "user.email", "@example.com")
	if _, err := branchUserFromEmail(ctx); err == nil || !strings.Contains(err.Error(), "could not derive") {
		t.Fatalf("expected derive error, got: %v", err)
	}

	gitCmd(t, dir, "config", "--local", "user.email", "bad user@example.com")
	if _, err := branchUserFromEmail(ctx); err == nil || !strings.Contains(err.Error(), "invalid branch name") {
		t.Fatalf("expected invalid-branch error, got: %v", err)
	}
}

func TestValidateBranchName(t *testing.T) {
	requireGit(t)
	setupIsolatedGitEnv(t)

	dir := t.TempDir()
	withCwd(t, dir)

	ctx := context.Background()
	if err := validateBranchName(ctx, "twig", ""); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty error, got: %v", err)
	}
	if err := validateBranchName(ctx, "twig", "bad user"); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid error, got: %v", err)
	}
	if err := validateBranchName(ctx, "twig", "alice/feature-x"); err != nil {
		t.Fatalf("expected valid branch, got err=%v", err)
	}
}

func TestPrintPushAdvice(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)

	ctx := context.Background()

	{
		var out bytes.Buffer
		if err := printPushAdvice(ctx, &out, "alice/feature-x"); err != nil {
			t.Fatalf("printPushAdvice err=%v", err)
		}
		got := out.String()
		if !strings.Contains(got, "git push -u <remote> alice/feature-x") {
			t.Fatalf("expected placeholder push advice, got:\n%s", got)
		}
		if !strings.Contains(got, "Hint: git remote -v") {
			t.Fatalf("expected remote hint, got:\n%s", got)
		}
	}

	gitCmd(t, repo, "remote", "add", "origin", repo)
	{
		var out bytes.Buffer
		if err := printPushAdvice(ctx, &out, "alice/feature-x"); err != nil {
			t.Fatalf("printPushAdvice err=%v", err)
		}
		got := out.String()
		if !strings.Contains(got, "git push -u origin alice/feature-x") {
			t.Fatalf("expected origin push advice, got:\n%s", got)
		}
	}

	gitCmd(t, repo, "remote", "add", "jj", repo)
	{
		var out bytes.Buffer
		if err := printPushAdvice(ctx, &out, "alice/feature-x"); err != nil {
			t.Fatalf("printPushAdvice err=%v", err)
		}
		got := out.String()
		if !strings.Contains(got, "git push -u <remote> alice/feature-x") {
			t.Fatalf("expected placeholder push advice for multiple remotes, got:\n%s", got)
		}
		if !strings.Contains(got, "Available remotes:") || !strings.Contains(got, "origin") || !strings.Contains(got, "jj") {
			t.Fatalf("expected available remotes line, got:\n%s", got)
		}
	}
}

func TestPrintErrorAndPanic(t *testing.T) {
	{
		var out bytes.Buffer
		printError(&out, friendlyError{})
		if got := out.String(); got != "friendly message\n" {
			t.Fatalf("printError()=%q, want %q", got, "friendly message\n")
		}
	}
	{
		var out bytes.Buffer
		printPanic(&out, errors.New("boom"))
		if got := out.String(); got != "boom\n" {
			t.Fatalf("printPanic(error)=%q, want %q", got, "boom\n")
		}
	}
	{
		var out bytes.Buffer
		printPanic(&out, "boom")
		if got := out.String(); got != "boom\n" {
			t.Fatalf("printPanic(string)=%q, want %q", got, "boom\n")
		}
	}
}

func TestRunCreateBranchViaRun(t *testing.T) {
	repo := initRepo(t)
	gitSwitchCreate(t, repo, "feature-x")
	withCwd(t, repo)

	var out bytes.Buffer
	if err := run(context.Background(), []string{"-b", "feature-x"}, &out, io.Discard); err != nil {
		t.Fatalf("run(-b) err=%v\n%s", err, out.String())
	}
	if got := strings.TrimSpace(gitCmd(t, repo, "rev-parse", "--abbrev-ref", "HEAD")); got != "alice/feature-x" {
		t.Fatalf("current branch=%q, want %q", got, "alice/feature-x")
	}

	out.Reset()
	if err := run(context.Background(), []string{"-b", "feature-x"}, &out, io.Discard); err != nil {
		t.Fatalf("run(-b) second time err=%v\n%s", err, out.String())
	}
	if got := strings.TrimSpace(gitCmd(t, repo, "rev-parse", "--abbrev-ref", "HEAD")); got != "alice/feature-x" {
		t.Fatalf("current branch=%q, want %q", got, "alice/feature-x")
	}
}

func TestRunStartOnboardingFlow(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	withCwd(t, alice)

	var out bytes.Buffer
	if err := run(context.Background(), []string{"start", "--twig", "feature-x", "--yes"}, &out, io.Discard); err != nil {
		t.Fatalf("run(start) err=%v\n%s", err, out.String())
	}

	if got := strings.TrimSpace(gitCmd(t, alice, "rev-parse", "--abbrev-ref", "HEAD")); got != "alice/feature-x" {
		t.Fatalf("current branch=%q, want %q", got, "alice/feature-x")
	}

	// Shared twig and personal branch are pushed to the remote.
	if out := gitCmd(t, seed, "ls-remote", "--heads", "origin", "feature-x"); !strings.Contains(out, "refs/heads/feature-x") {
		t.Fatalf("expected remote to have feature-x, got:\n%s", out)
	}
	if out := gitCmd(t, seed, "ls-remote", "--heads", "origin", "alice/feature-x"); !strings.Contains(out, "refs/heads/alice/feature-x") {
		t.Fatalf("expected remote to have alice/feature-x, got:\n%s", out)
	}
}

func TestRunJoinOnboardingFlow(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	// Publish the shared twig as the first group member would.
	gitSwitchCreate(t, seed, "feature-x")
	gitCmd(t, seed, "push", "-u", "origin", "feature-x")

	bob := cloneRepo(t, origin, "Bob", "bob@example.com")
	withCwd(t, bob)

	var out bytes.Buffer
	if err := run(context.Background(), []string{"join", "--twig", "feature-x", "--yes"}, &out, io.Discard); err != nil {
		t.Fatalf("run(join) err=%v\n%s", err, out.String())
	}

	if got := strings.TrimSpace(gitCmd(t, bob, "rev-parse", "--abbrev-ref", "HEAD")); got != "bob/feature-x" {
		t.Fatalf("current branch=%q, want %q", got, "bob/feature-x")
	}

	if out := gitCmd(t, seed, "ls-remote", "--heads", "origin", "bob/feature-x"); !strings.Contains(out, "refs/heads/bob/feature-x") {
		t.Fatalf("expected remote to have bob/feature-x, got:\n%s", out)
	}
}

func TestRunInitSuggestsStartThenJoin(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	{
		alice := cloneRepo(t, origin, "Alice", "alice@example.com")
		withCwd(t, alice)

		var out bytes.Buffer
		if err := run(context.Background(), []string{"init", "--twig", "feature-x", "--yes"}, &out, io.Discard); err != nil {
			t.Fatalf("run(init) first member err=%v\n%s", err, out.String())
		}
		if got := strings.TrimSpace(gitCmd(t, alice, "rev-parse", "--abbrev-ref", "HEAD")); got != "alice/feature-x" {
			t.Fatalf("current branch=%q, want %q", got, "alice/feature-x")
		}
	}

	{
		bob := cloneRepo(t, origin, "Bob", "bob@example.com")
		withCwd(t, bob)

		var out bytes.Buffer
		if err := run(context.Background(), []string{"init", "--twig", "feature-x", "--yes"}, &out, io.Discard); err != nil {
			t.Fatalf("run(init) next member err=%v\n%s", err, out.String())
		}
		if got := strings.TrimSpace(gitCmd(t, bob, "rev-parse", "--abbrev-ref", "HEAD")); got != "bob/feature-x" {
			t.Fatalf("current branch=%q, want %q", got, "bob/feature-x")
		}
	}
}

func TestRunInitJoinDetachedHeadDoesNotRequireBase(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	// Publish the shared twig as the first group member would.
	gitSwitchCreate(t, seed, "feature-x")
	gitCmd(t, seed, "push", "-u", "origin", "feature-x")

	bob := cloneRepo(t, origin, "Bob", "bob@example.com")

	// Put the clone into a detached-HEAD state. This simulates real-world cases
	// like `git checkout <sha>`, `git bisect`, or CI checkouts. `mob-consensus init`
	// should still be able to *join* an existing twig without needing --base.
	gitCmd(t, bob, "checkout", "--detach", "HEAD")
	if got := strings.TrimSpace(gitCmd(t, bob, "rev-parse", "--abbrev-ref", "HEAD")); got != "HEAD" {
		t.Fatalf("expected detached HEAD, got %q", got)
	}

	withCwd(t, bob)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"init", "--twig", "feature-x", "--yes"}, &out, io.Discard); err != nil {
		t.Fatalf("run(init) detached HEAD err=%v\n%s", err, out.String())
	}
	if got := strings.TrimSpace(gitCmd(t, bob, "rev-parse", "--abbrev-ref", "HEAD")); got != "bob/feature-x" {
		t.Fatalf("current branch=%q, want %q", got, "bob/feature-x")
	}
}

func TestRunInitPlanDetachedHeadShowsBaseHint(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	bob := cloneRepo(t, origin, "Bob", "bob@example.com")
	gitCmd(t, bob, "checkout", "--detach", "HEAD")

	withCwd(t, bob)
	var out bytes.Buffer
	if err := run(context.Background(), []string{"init", "--twig", "feature-x", "--plan"}, &out, io.Discard); err != nil {
		t.Fatalf("run(init --plan) err=%v\n%s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "git fetch origin") {
		t.Fatalf("init plan missing fetch:\n%s", got)
	}
	if !strings.Contains(got, "mob-consensus start --twig feature-x --base <ref>") || !strings.Contains(got, "(hint: pass --base <ref>)") {
		t.Fatalf("init plan missing detached-HEAD base hint:\n%s", got)
	}
}

func TestRunInitAbortAfterFetch(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	withCwd(t, alice)

	// init has two interactive confirmations: one for the fetch step, and one
	// for the suggested start/join action. Approve the fetch, then abort.
	withStdin(t, "y\nn\n")
	var out bytes.Buffer
	err := run(context.Background(), []string{"init", "--twig", "feature-x"}, &out, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("expected init abort, got err=%v\n%s", err, out.String())
	}
}

func TestRunInitDetachedHeadStartRequiresBase(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	bob := cloneRepo(t, origin, "Bob", "bob@example.com")
	gitCmd(t, bob, "checkout", "--detach", "HEAD")
	withCwd(t, bob)

	var out bytes.Buffer
	err := run(context.Background(), []string{"init", "--twig", "feature-x", "--yes"}, &out, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "could not determine a base ref") {
		t.Fatalf("expected init detached-HEAD start to require --base, got err=%v\n%s", err, out.String())
	}
}

func TestRunStartPlanOutput(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	withCwd(t, alice)

	var out bytes.Buffer
	if err := run(context.Background(), []string{"start", "--twig", "feature-x", "--base", "main", "--plan"}, &out, io.Discard); err != nil {
		t.Fatalf("run(start --plan) err=%v\n%s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{
		"git fetch origin",
		"git checkout -b feature-x main",
		"git push -u origin feature-x",
		"git checkout -b alice/feature-x feature-x",
		"git push -u origin alice/feature-x",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("start plan missing %q:\n%s", want, got)
		}
	}
}

func TestRunStartFailsWhenTwigExistsOnRemote(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")
	gitSwitchCreate(t, seed, "feature-x")
	gitCmd(t, seed, "push", "-u", "origin", "feature-x")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	withCwd(t, alice)

	var out bytes.Buffer
	err := run(context.Background(), []string{"start", "--twig", "feature-x", "--base", "main", "--yes"}, &out, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "shared twig") || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected start to fail when twig exists on remote, got err=%v\n%s", err, out.String())
	}
}

func TestRunJoinPlanOutput(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	withCwd(t, alice)

	var out bytes.Buffer
	if err := run(context.Background(), []string{"join", "--twig", "feature-x", "--plan"}, &out, io.Discard); err != nil {
		t.Fatalf("run(join --plan) err=%v\n%s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{
		"git fetch origin",
		"git checkout -b feature-x origin/feature-x",
		"git checkout -b alice/feature-x feature-x",
		"git push -u origin alice/feature-x",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("join plan missing %q:\n%s", want, got)
		}
	}
}

func TestRunJoinFailsWhenTwigMissingOnRemote(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	bob := cloneRepo(t, origin, "Bob", "bob@example.com")
	withCwd(t, bob)

	var out bytes.Buffer
	err := run(context.Background(), []string{"join", "--twig", "feature-x", "--yes"}, &out, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "shared twig") || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected join to fail when twig missing on remote, got err=%v\n%s", err, out.String())
	}
}

func TestRunJoinUsesExistingRemotePersonalBranch(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	// Publish the shared twig as the first group member would.
	gitSwitchCreate(t, seed, "feature-x")
	gitCmd(t, seed, "push", "-u", "origin", "feature-x")

	// Publish a peer personal branch with a commit not present on the twig, so we
	// can verify join checks it out instead of re-creating it from the twig.
	gitSwitchCreate(t, seed, "bob/feature-x", "feature-x")
	writeFile(t, seed, "bob.txt", "hello from bob\n")
	gitCmd(t, seed, "add", "bob.txt")
	gitCmd(t, seed, "-c", "user.name=Bob", "-c", "user.email=bob@example.com", "commit", "-m", "bob change")
	gitCmd(t, seed, "push", "-u", "origin", "bob/feature-x")

	bob := cloneRepo(t, origin, "Bob", "bob@example.com")
	withCwd(t, bob)

	var out bytes.Buffer
	if err := run(context.Background(), []string{"join", "--twig", "feature-x", "--yes"}, &out, io.Discard); err != nil {
		t.Fatalf("run(join) err=%v\n%s", err, out.String())
	}
	if got := strings.TrimSpace(gitCmd(t, bob, "rev-parse", "--abbrev-ref", "HEAD")); got != "bob/feature-x" {
		t.Fatalf("current branch=%q, want %q", got, "bob/feature-x")
	}
	if got := gitCmd(t, bob, "show", "HEAD:bob.txt"); !strings.Contains(got, "hello from bob") {
		t.Fatalf("expected bob/feature-x to include bob.txt, got:\n%s", got)
	}
}

func TestIsDirtyCleanAndDirty(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)

	ctx := context.Background()
	dirty, err := isDirty(ctx)
	if err != nil {
		t.Fatalf("isDirty() err=%v", err)
	}
	if dirty {
		t.Fatalf("isDirty()=true, want false")
	}

	writeFile(t, repo, "untracked.txt", "dirty\n")
	dirty, err = isDirty(ctx)
	if err != nil {
		t.Fatalf("isDirty() err=%v", err)
	}
	if !dirty {
		t.Fatalf("isDirty()=false, want true")
	}
}

func TestIsDirtyOutsideRepoErrors(t *testing.T) {
	requireGit(t)
	setupIsolatedGitEnv(t)

	dir := t.TempDir()
	withCwd(t, dir)

	if _, err := isDirty(context.Background()); err == nil {
		t.Fatalf("expected isDirty() to error outside a git repo")
	}
}

func TestResolveTwigPrompting(t *testing.T) {
	{
		var stderr bytes.Buffer
		withStdin(t, "\n")
		twig, err := resolveTwig(cmdStart, options{}, "main", "alice", &stderr)
		if err != nil {
			t.Fatalf("resolveTwig(default) err=%v", err)
		}
		if twig != "feature-x" {
			t.Fatalf("resolveTwig(default)=%q, want %q", twig, "feature-x")
		}
		if !strings.Contains(stderr.String(), "Twig name") {
			t.Fatalf("expected prompt on stderr, got:\n%s", stderr.String())
		}
	}

	{
		var stderr bytes.Buffer
		withStdin(t, "dev\n")
		twig, err := resolveTwig(cmdStart, options{}, "main", "alice", &stderr)
		if err != nil {
			t.Fatalf("resolveTwig(custom) err=%v", err)
		}
		if twig != "dev" {
			t.Fatalf("resolveTwig(custom)=%q, want %q", twig, "dev")
		}
	}

	{
		// Non-interactive modes require --twig unless it can be inferred.
		var stderr bytes.Buffer
		_, err := resolveTwig(cmdStart, options{yes: true}, "main", "alice", &stderr)
		if err == nil || !strings.Contains(err.Error(), "requires --twig") {
			t.Fatalf("resolveTwig(noninteractive) err=%v, want requires --twig", err)
		}
	}

	{
		// When the current branch already includes a twig, infer it.
		var stderr bytes.Buffer
		twig, err := resolveTwig(cmdStart, options{}, "alice/feature-x", "alice", &stderr)
		if err != nil {
			t.Fatalf("resolveTwig(infer user/twig) err=%v", err)
		}
		if twig != "feature-x" {
			t.Fatalf("resolveTwig(infer user/twig)=%q, want %q", twig, "feature-x")
		}
	}
}

func TestResolveRemotePromptingAndErrors(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)

	ctx := context.Background()

	// Add multiple remotes but do not set an upstream; this forces the prompt
	// path in resolveRemote (no deterministic suggestion).
	gitCmd(t, repo, "remote", "add", "origin", repo)
	gitCmd(t, repo, "remote", "add", "jj", repo)

	{
		remote, err := resolveRemote(ctx, cmdStart, options{remote: "jj"}, io.Discard)
		if err != nil {
			t.Fatalf("resolveRemote(--remote jj) err=%v", err)
		}
		if remote != "jj" {
			t.Fatalf("resolveRemote(--remote jj)=%q, want %q", remote, "jj")
		}
	}

	{
		_, err := resolveRemote(ctx, cmdStart, options{remote: "nope"}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("resolveRemote(--remote nope) err=%v, want not found", err)
		}
	}

	{
		// Non-interactive modes require --remote when multiple remotes exist.
		var stderr bytes.Buffer
		_, err := resolveRemote(ctx, cmdStart, options{yes: true}, &stderr)
		if err == nil || !strings.Contains(err.Error(), "requires --remote") {
			t.Fatalf("resolveRemote(noninteractive) err=%v, want requires --remote", err)
		}
	}

	{
		// Interactive prompt picks an explicit remote.
		var stderr bytes.Buffer
		withStdin(t, "origin\n")
		remote, err := resolveRemote(ctx, cmdStart, options{}, &stderr)
		if err != nil {
			t.Fatalf("resolveRemote(prompt) err=%v", err)
		}
		if remote != "origin" {
			t.Fatalf("resolveRemote(prompt)=%q, want %q", remote, "origin")
		}
		if !strings.Contains(stderr.String(), "Pick remote") {
			t.Fatalf("expected prompt on stderr, got:\n%s", stderr.String())
		}
	}

	{
		// Unknown remote should error with a clear message.
		var stderr bytes.Buffer
		withStdin(t, "nope\n")
		_, err := resolveRemote(ctx, cmdStart, options{}, &stderr)
		if err == nil || !strings.Contains(err.Error(), "unknown remote") {
			t.Fatalf("resolveRemote(unknown) err=%v, want unknown remote", err)
		}
	}
}

func TestRunGitPlanModesAndConfirm(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)

	ctx := context.Background()

	steps := []gitPlanStep{
		{
			Explain: "Verify we're in a Git worktree",
			Args: func(ctx context.Context) ([]string, error) {
				return []string{"rev-parse", "--is-inside-work-tree"}, nil
			},
		},
	}

	{
		var out bytes.Buffer
		if err := runGitPlan(ctx, options{plan: true}, "plan title", steps, &out, io.Discard); err != nil {
			t.Fatalf("runGitPlan(plan) err=%v", err)
		}
		if !strings.Contains(out.String(), "plan title") || !strings.Contains(out.String(), "git rev-parse") {
			t.Fatalf("runGitPlan(plan) output missing expected lines:\n%s", out.String())
		}
	}

	{
		var out bytes.Buffer
		if err := runGitPlan(ctx, options{dryRun: true}, "dry run title", steps, &out, io.Discard); err != nil {
			t.Fatalf("runGitPlan(dry-run) err=%v", err)
		}
		if strings.TrimSpace(out.String()) != "git rev-parse --is-inside-work-tree" {
			t.Fatalf("runGitPlan(dry-run) output=%q", out.String())
		}
	}

	{
		// Confirmed execution runs the step.
		var out bytes.Buffer
		var stderr bytes.Buffer
		withStdin(t, "y\n")
		if err := runGitPlan(ctx, options{}, "exec title", steps, &out, &stderr); err != nil {
			t.Fatalf("runGitPlan(exec) err=%v\n%s", err, out.String())
		}
		if !strings.Contains(out.String(), "Step 1/1") {
			t.Fatalf("runGitPlan(exec) output missing step header:\n%s", out.String())
		}
		if !strings.Contains(stderr.String(), "Run this?") {
			t.Fatalf("runGitPlan(exec) expected confirmation prompt, got:\n%s", stderr.String())
		}
	}

	{
		// Declining confirmation aborts.
		var out bytes.Buffer
		var stderr bytes.Buffer
		withStdin(t, "n\n")
		err := runGitPlan(ctx, options{}, "exec title", steps, &out, &stderr)
		if err == nil || !strings.Contains(err.Error(), "aborted") {
			t.Fatalf("runGitPlan(abort) err=%v, want aborted", err)
		}
	}
}

func TestRunGitPlanYesAndErrors(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)

	ctx := context.Background()

	quietSteps := []gitPlanStep{
		{
			Explain: "Check status (no output when clean)",
			Args: func(ctx context.Context) ([]string, error) {
				return []string{"status", "--porcelain"}, nil
			},
		},
	}

	{
		var out bytes.Buffer
		var stderr bytes.Buffer
		if err := runGitPlan(ctx, options{yes: true}, "yes title", quietSteps, &out, &stderr); err != nil {
			t.Fatalf("runGitPlan(yes) err=%v\n%s", err, out.String())
		}
		if strings.Contains(stderr.String(), "Run this?") {
			t.Fatalf("runGitPlan(yes) unexpectedly prompted:\n%s", stderr.String())
		}
	}

	{
		badSteps := []gitPlanStep{
			{
				Explain: "Args error",
				Args: func(context.Context) ([]string, error) {
					return nil, errors.New("boom")
				},
			},
		}
		if err := runGitPlan(ctx, options{plan: true}, "plan", badSteps, io.Discard, io.Discard); err == nil {
			t.Fatalf("runGitPlan(plan args error) err=nil, want error")
		}
		if err := runGitPlan(ctx, options{dryRun: true}, "dry", badSteps, io.Discard, io.Discard); err == nil {
			t.Fatalf("runGitPlan(dry-run args error) err=nil, want error")
		}
		if err := runGitPlan(ctx, options{}, "exec", badSteps, io.Discard, io.Discard); err == nil {
			t.Fatalf("runGitPlan(exec args error) err=nil, want error")
		}
	}

	{
		preFailSteps := []gitPlanStep{
			{
				Explain: "Pre error",
				Pre: func(context.Context) error {
					return errors.New("pre fail")
				},
				Args: func(context.Context) ([]string, error) {
					return []string{"status", "--porcelain"}, nil
				},
			},
		}
		if err := runGitPlan(ctx, options{}, "exec", preFailSteps, io.Discard, io.Discard); err == nil {
			t.Fatalf("runGitPlan(pre error) err=nil, want error")
		}
	}
}

func TestRunCreateBranchDirtyFails(t *testing.T) {
	repo := initRepo(t)
	gitSwitchCreate(t, repo, "feature-x")
	writeFile(t, repo, "README.md", "dirty\n")
	withCwd(t, repo)

	var out bytes.Buffer
	err := run(context.Background(), []string{"-b", "feature-x"}, &out, io.Discard)
	if err == nil {
		t.Fatalf("expected error on dirty tree")
	}
	if !strings.Contains(err.Error(), "working tree is dirty") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "you have uncommitted changes") {
		t.Fatalf("expected dirty-tree message, got:\n%s", out.String())
	}
}

func TestEnsureCleanCommitDirtyNoPush(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)

	editor := writeCommitMessageEditor(t)
	t.Setenv("GIT_EDITOR", editor)

	writeFile(t, repo, "README.md", "dirty change\n")

	var out bytes.Buffer
	err := ensureClean(context.Background(), options{commitDirty: true, noPush: true}, true, &out)
	if err != nil {
		t.Fatalf("ensureClean err=%v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "you have uncommitted changes") {
		t.Fatalf("expected dirty-tree message, got:\n%s", out.String())
	}

	if st := strings.TrimSpace(gitCmd(t, repo, "status", "--porcelain")); st != "" {
		t.Fatalf("expected clean tree after auto-commit, got status:\n%s", st)
	}
	if subject := strings.TrimSpace(gitCmd(t, repo, "log", "-1", "--pretty=%s")); subject != "test auto commit" {
		t.Fatalf("commit subject=%q, want %q", subject, "test auto commit")
	}
}

func TestEnsureCleanAllowsDirtyWhenNotRequired(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)

	writeFile(t, repo, "README.md", "dirty\n")

	var out bytes.Buffer
	if err := ensureClean(context.Background(), options{}, false, &out); err != nil {
		t.Fatalf("ensureClean err=%v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "you have uncommitted changes") {
		t.Fatalf("expected dirty-tree message, got:\n%s", out.String())
	}
}

func TestEnsureCleanCommitDirtyPushes(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	withCwd(t, alice)

	editor := writeCommitMessageEditor(t)
	t.Setenv("GIT_EDITOR", editor)

	writeFile(t, alice, "README.md", "dirty change\n")

	var out bytes.Buffer
	if err := ensureClean(context.Background(), options{commitDirty: true}, true, &out); err != nil {
		t.Fatalf("ensureClean err=%v\n%s", err, out.String())
	}

	head := strings.TrimSpace(gitCmd(t, alice, "rev-parse", "HEAD"))
	remote := gitCmd(t, seed, "ls-remote", "--heads", "origin", "main")
	if !strings.Contains(remote, head) || !strings.Contains(remote, "refs/heads/main") {
		t.Fatalf("expected remote main to be updated to %s, got:\n%s", head, remote)
	}
}

func TestRequireUserBranchUsageError(t *testing.T) {
	repo := initRepo(t)
	gitSwitchCreate(t, repo, "feature-x")
	withCwd(t, repo)

	var out bytes.Buffer
	err := run(context.Background(), nil, &out, io.Discard)
	var uerr usageError
	if !errors.As(err, &uerr) {
		t.Fatalf("expected usageError, got: %T %v", err, err)
	}
	if !strings.Contains(err.Error(), "you aren't on a 'alice/' branch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMergeCleanAndNoop(t *testing.T) {
	repo := initRepo(t)

	gitSwitchCreate(t, repo, "alice/feature-x")
	gitSwitchCreate(t, repo, "bob/feature-x", "main")
	writeFile(t, repo, "bob.txt", "hello from bob\n")
	gitCmd(t, repo, "add", "bob.txt")
	gitCmd(t, repo, "-c", "user.name=Bob", "-c", "user.email=bob@example.com", "commit", "-m", "bob change")
	gitCmd(t, repo, "checkout", "alice/feature-x")

	withCwd(t, repo)
	ctx := context.Background()

	headBefore := strings.TrimSpace(gitCmd(t, repo, "rev-parse", "HEAD"))
	var out bytes.Buffer
	if err := runMerge(ctx, options{otherBranch: "bob/feature-x", noPush: true}, "alice/feature-x", &out); err != nil {
		t.Fatalf("runMerge err=%v\n%s", err, out.String())
	}
	headAfter := strings.TrimSpace(gitCmd(t, repo, "rev-parse", "HEAD"))
	if headAfter == headBefore {
		t.Fatalf("expected merge commit to change HEAD")
	}
	if !strings.Contains(out.String(), "skipping automatic push") {
		t.Fatalf("expected no-push message, got:\n%s", out.String())
	}

	parents := strings.Fields(strings.TrimSpace(gitCmd(t, repo, "rev-list", "--parents", "-n", "1", "HEAD")))
	if len(parents) != 3 {
		t.Fatalf("expected a merge commit with 2 parents, got: %v", parents)
	}

	msg := gitCmd(t, repo, "log", "-1", "--pretty=%B")
	if !strings.Contains(msg, "mob-consensus merge from bob/feature-x onto alice/feature-x") {
		t.Fatalf("merge commit message missing header:\n%s", msg)
	}
	if !strings.Contains(msg, "Co-authored-by: Bob <bob@example.com>") {
		t.Fatalf("merge commit message missing co-author:\n%s", msg)
	}

	out.Reset()
	headBefore = headAfter
	if err := runMerge(ctx, options{otherBranch: "bob/feature-x", noPush: true}, "alice/feature-x", &out); err != nil {
		t.Fatalf("runMerge no-op err=%v\n%s", err, out.String())
	}
	headAfter = strings.TrimSpace(gitCmd(t, repo, "rev-parse", "HEAD"))
	if headAfter != headBefore {
		t.Fatalf("expected no-op merge to leave HEAD unchanged")
	}
}

func TestRunMergeConflictRequiresResolution(t *testing.T) {
	repo := initRepo(t)

	// Make mergetool deterministic and non-interactive: when a conflict happens,
	// resolve by choosing our side for the known conflicted file.
	gitCmd(t, repo, "config", "--local", "mergetool.vimdiff.cmd", `sh -c 'git checkout --ours -- conflict.txt && git add conflict.txt'`)

	gitSwitchCreate(t, repo, "alice/feature-x")
	writeFile(t, repo, "conflict.txt", "alice\n")
	gitCmd(t, repo, "add", "conflict.txt")
	gitCmd(t, repo, "commit", "-m", "alice change")

	gitSwitchCreate(t, repo, "bob/feature-x", "main")
	writeFile(t, repo, "conflict.txt", "bob\n")
	gitCmd(t, repo, "add", "conflict.txt")
	gitCmd(t, repo, "-c", "user.name=Bob", "-c", "user.email=bob@example.com", "commit", "-m", "bob change")

	gitCmd(t, repo, "checkout", "alice/feature-x")

	withCwd(t, repo)
	var out bytes.Buffer
	if err := runMerge(context.Background(), options{otherBranch: "bob/feature-x", noPush: true}, "alice/feature-x", &out); err != nil {
		t.Fatalf("runMerge err=%v\n%s", err, out.String())
	}

	// Verify the merge completed and doesn't contain conflict markers.
	data, err := os.ReadFile(filepath.Join(repo, "conflict.txt"))
	if err != nil {
		t.Fatalf("read conflict.txt: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "<<<<<<<") || strings.Contains(got, ">>>>>>>") || strings.Contains(got, "=======") {
		t.Fatalf("expected conflict markers to be resolved, got:\n%s", got)
	}
	if strings.TrimSpace(got) != "alice" {
		t.Fatalf("expected our side to be chosen, got:\n%s", got)
	}
}

func TestRunDiscoveryStatusLines(t *testing.T) {
	repo := initRepo(t)

	gitSwitchCreate(t, repo, "alice/feature-x")
	writeFile(t, repo, "alice.txt", "alice\n")
	gitCmd(t, repo, "add", "alice.txt")
	gitCmd(t, repo, "commit", "-m", "alice change")

	gitSwitchCreate(t, repo, "carol/feature-x")
	writeFile(t, repo, "carol.txt", "carol\n")
	gitCmd(t, repo, "add", "carol.txt")
	gitCmd(t, repo, "-c", "user.name=Carol", "-c", "user.email=carol@example.com", "commit", "-m", "carol change")

	gitCmd(t, repo, "checkout", "alice/feature-x")
	gitCmd(t, repo, "branch", "eve/feature-x")
	gitSwitchCreate(t, repo, "dave/feature-x", "main")

	gitSwitchCreate(t, repo, "bob/feature-x", "main")
	writeFile(t, repo, "bob.txt", "bob\n")
	gitCmd(t, repo, "add", "bob.txt")
	gitCmd(t, repo, "-c", "user.name=Bob", "-c", "user.email=bob@example.com", "commit", "-m", "bob change")

	gitCmd(t, repo, "checkout", "alice/feature-x")

	withCwd(t, repo)
	var out bytes.Buffer
	if err := runDiscovery(context.Background(), options{}, "alice/feature-x", &out); err != nil {
		t.Fatalf("runDiscovery err=%v\n%s", err, out.String())
	}
	got := out.String()
	for _, want := range []string{
		"carol/feature-x is ahead:",
		"dave/feature-x is behind:",
		"bob/feature-x has diverged:",
		"eve/feature-x is synced",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("discovery output missing %q:\n%s", want, got)
		}
	}
}

func TestSmartPushErrors(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)
	ctx := context.Background()

	{
		err := smartPush(ctx)
		if err == nil || !strings.Contains(err.Error(), "no git remotes configured") {
			t.Fatalf("expected no-remotes error, got: %v", err)
		}
	}

	head := strings.TrimSpace(gitCmd(t, repo, "rev-parse", "HEAD"))
	gitCmd(t, repo, "checkout", head)
	{
		err := smartPush(ctx)
		if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
			t.Fatalf("expected detached-HEAD error, got: %v", err)
		}
	}

	gitCmd(t, repo, "checkout", "main")
	gitCmd(t, repo, "remote", "add", "origin", repo)
	gitCmd(t, repo, "remote", "add", "jj", repo)
	{
		err := smartPush(ctx)
		if err == nil || !strings.Contains(err.Error(), "multiple remotes exist") {
			t.Fatalf("expected multiple-remotes error, got: %v", err)
		}
	}
}

func TestResolveMergeTargetLocalAndMissing(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)
	ctx := context.Background()

	gitSwitchCreate(t, repo, "bob/feature-x")
	gitCmd(t, repo, "checkout", "main")

	got, needsConfirm, err := resolveMergeTarget(ctx, "bob/feature-x")
	if err != nil {
		t.Fatalf("resolveMergeTarget err=%v", err)
	}
	if needsConfirm {
		t.Fatalf("expected local ref to not need confirmation")
	}
	if got != "bob/feature-x" {
		t.Fatalf("resolveMergeTarget=%q, want %q", got, "bob/feature-x")
	}

	if _, _, err := resolveMergeTarget(ctx, "nope/feature-x"); err == nil || !strings.Contains(err.Error(), "no remotes configured") {
		t.Fatalf("expected no-remotes error, got: %v", err)
	}
}

func TestFetchSuggestedRemoteSelection(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)
	ctx := context.Background()

	if err := fetchSuggestedRemote(ctx, ""); err == nil {
		t.Fatalf("expected error with no remotes")
	}

	origin := initBareRemote(t)
	gitCmd(t, repo, "remote", "add", "origin", origin)
	gitCmd(t, repo, "push", "-u", "origin", "main")
	gitCmd(t, repo, "branch", "--unset-upstream")
	if err := fetchSuggestedRemote(ctx, ""); err != nil {
		t.Fatalf("fetchSuggestedRemote (sole remote) err=%v", err)
	}

	jj := initBareRemote(t)
	gitCmd(t, repo, "remote", "add", "jj", jj)
	if err := fetchSuggestedRemote(ctx, "jj/bob/feature-x"); err != nil {
		t.Fatalf("fetchSuggestedRemote (remote prefix) err=%v", err)
	}

	if err := fetchSuggestedRemote(ctx, ""); err == nil || !strings.Contains(err.Error(), "multiple remotes configured") {
		t.Fatalf("expected multiple-remotes error, got: %v", err)
	}

	gitCmd(t, repo, "push", "-u", "origin", "main")
	gitCmd(t, repo, "fetch", "origin")
	if upstream := strings.TrimSpace(gitCmd(t, repo, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")); !strings.HasPrefix(upstream, "origin/") {
		t.Fatalf("expected origin upstream, got %q", upstream)
	}
	if err := fetchSuggestedRemote(ctx, ""); err != nil {
		t.Fatalf("fetchSuggestedRemote (upstream remote) err=%v", err)
	}
}

func TestGitOutputErrorIncludesStderr(t *testing.T) {
	repo := initRepo(t)
	withCwd(t, repo)

	_, err := gitOutput(context.Background(), "rev-parse", "--verify", "refs/heads/does-not-exist")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "git rev-parse --verify refs/heads/does-not-exist") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "fatal:") {
		t.Fatalf("expected fatal message in error: %v", err)
	}
}

func TestResolveMergeTargetRemoteCandidates(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	// Follow `usage.tmpl`: publish the shared twig so others can base their
	// personal branches on it.
	gitSwitchCreate(t, seed, "feature-x")
	gitCmd(t, seed, "push", "-u", "origin", "feature-x")

	// Create a peer branch on the remote to exercise remote-ref resolution.
	// We create it directly (instead of running mob-consensus as Bob) to keep
	// this test focused on resolveMergeTarget behavior.
	gitSwitchCreate(t, seed, "bob/feature-x", "feature-x")
	writeFile(t, seed, "bob.txt", "hello from bob\n")
	gitCmd(t, seed, "add", "bob.txt")
	gitCmd(t, seed, "-c", "user.name=Bob", "-c", "user.email=bob@example.com", "commit", "-m", "bob change")
	gitCmd(t, seed, "push", "-u", "origin", "bob/feature-x")

	// Use a clone to match the user-facing workflow; using `git init` here can
	// produce an unrelated history and make merge-related behavior flaky.
	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	gitCmd(t, alice, "fetch", "origin")
	withCwd(t, alice)

	ctx := context.Background()

	{
		got, needsConfirm, err := resolveMergeTarget(ctx, "bob/feature-x")
		if err != nil {
			t.Fatalf("resolveMergeTarget err=%v", err)
		}
		if !needsConfirm {
			t.Fatalf("expected remote resolution to require confirmation")
		}
		if got != "origin/bob/feature-x" {
			t.Fatalf("resolveMergeTarget=%q, want %q", got, "origin/bob/feature-x")
		}
	}

	{
		_, _, err := resolveMergeTarget(ctx, "nobody/feature-x")
		if err == nil || !strings.Contains(err.Error(), "not found locally or on any remote") {
			t.Fatalf("expected not-found error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "origin") {
			t.Fatalf("expected error to mention origin, got: %v", err)
		}
	}

	jj := initBareRemote(t)
	gitCmd(t, seed, "remote", "add", "jj", jj)
	gitCmd(t, seed, "push", "-u", "jj", "main")
	gitCmd(t, seed, "push", "-u", "jj", "bob/feature-x")

	gitCmd(t, alice, "remote", "add", "jj", jj)
	gitCmd(t, alice, "fetch", "jj")

	{
		_, _, err := resolveMergeTarget(ctx, "bob/feature-x")
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected ambiguous error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "origin/bob/feature-x") || !strings.Contains(err.Error(), "jj/bob/feature-x") {
			t.Fatalf("expected ambiguous error to include both candidates, got: %v", err)
		}
	}
}

func TestRunMergeBranchNotFoundShowsDiscovery(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	// Follow `usage.tmpl`: publish the shared twig so others can base their
	// personal branches on it.
	gitSwitchCreate(t, seed, "feature-x")
	gitCmd(t, seed, "push", "-u", "origin", "feature-x")

	// Create a peer branch on the remote so discovery has a realistic branch to
	// show.
	gitSwitchCreate(t, seed, "bob/feature-x", "feature-x")
	writeFile(t, seed, "bob.txt", "hello from bob\n")
	gitCmd(t, seed, "add", "bob.txt")
	gitCmd(t, seed, "-c", "user.name=Bob", "-c", "user.email=bob@example.com", "commit", "-m", "bob change")
	gitCmd(t, seed, "push", "-u", "origin", "bob/feature-x")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	// Next group member flow from `usage.tmpl`.
	gitCmd(t, alice, "fetch", "origin")
	gitSwitchCreate(t, alice, "feature-x", "origin/feature-x")
	withCwd(t, alice)
	if err := run(context.Background(), []string{"-b", "feature-x"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run(-b) err=%v", err)
	}

	headBefore := strings.TrimSpace(gitCmd(t, alice, "rev-parse", "HEAD"))
	statusBefore := strings.TrimSpace(gitCmd(t, alice, "status", "--porcelain"))
	if statusBefore != "" {
		t.Fatalf("expected clean working tree, got status:\n%s", statusBefore)
	}

	var out bytes.Buffer
	err := run(context.Background(), []string{"nobody/feature-x"}, &out, io.Discard)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(out.String(), "Related branches and their diffs") {
		t.Fatalf("expected discovery output, got:\n%s", out.String())
	}

	var errOut bytes.Buffer
	printError(&errOut, err)
	if !strings.Contains(errOut.String(), "does not exist") {
		t.Fatalf("expected friendly not-found message, got:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Pick a branch name from the list above") {
		t.Fatalf("expected selection hint, got:\n%s", errOut.String())
	}

	headAfter := strings.TrimSpace(gitCmd(t, alice, "rev-parse", "HEAD"))
	if headAfter != headBefore {
		t.Fatalf("expected HEAD to be unchanged: before=%s after=%s", headBefore, headAfter)
	}
	statusAfter := strings.TrimSpace(gitCmd(t, alice, "status", "--porcelain"))
	if statusAfter != statusBefore {
		t.Fatalf("expected status to be unchanged: before=%q after=%q", statusBefore, statusAfter)
	}
}

func TestRunMergeRemoteResolutionConfirm(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	// Follow `usage.tmpl`: first publish the shared twig.
	gitSwitchCreate(t, seed, "feature-x")
	gitCmd(t, seed, "push", "-u", "origin", "feature-x")

	// Create a peer personal branch on the remote. We do this directly (instead
	// of running mob-consensus as Bob) to keep this test focused on the merge
	// confirmation path.
	gitSwitchCreate(t, seed, "bob/feature-x", "feature-x")
	writeFile(t, seed, "bob.txt", "hello from bob\n")
	gitCmd(t, seed, "add", "bob.txt")
	gitCmd(t, seed, "-c", "user.name=Bob", "-c", "user.email=bob@example.com", "commit", "-m", "bob change")
	gitCmd(t, seed, "push", "-u", "origin", "bob/feature-x")

	{
		alice := cloneRepo(t, origin, "Alice", "alice@example.com")
		// Next group member flow from `usage.tmpl`.
		gitCmd(t, alice, "fetch", "origin")
		gitSwitchCreate(t, alice, "feature-x", "origin/feature-x")
		withCwd(t, alice)
		if err := run(context.Background(), []string{"-b", "feature-x"}, io.Discard, io.Discard); err != nil {
			t.Fatalf("run(-b) err=%v", err)
		}
		withStdin(t, "n\n")

		var out bytes.Buffer
		err := runMerge(context.Background(), options{otherBranch: "bob/feature-x", noPush: true}, "alice/feature-x", &out)
		if err == nil || !strings.Contains(err.Error(), "merge aborted") {
			t.Fatalf("expected merge aborted error, got: %v", err)
		}
	}

	{
		alice := cloneRepo(t, origin, "Alice", "alice@example.com")
		// Next group member flow from `usage.tmpl`.
		gitCmd(t, alice, "fetch", "origin")
		gitSwitchCreate(t, alice, "feature-x", "origin/feature-x")
		withCwd(t, alice)
		if err := run(context.Background(), []string{"-b", "feature-x"}, io.Discard, io.Discard); err != nil {
			t.Fatalf("run(-b) err=%v", err)
		}
		withStdin(t, "y\n")

		var out bytes.Buffer
		if err := runMerge(context.Background(), options{otherBranch: "bob/feature-x", noPush: true}, "alice/feature-x", &out); err != nil {
			t.Fatalf("runMerge err=%v\n%s", err, out.String())
		}

		parents := strings.Fields(strings.TrimSpace(gitCmd(t, alice, "rev-list", "--parents", "-n", "1", "HEAD")))
		if len(parents) != 3 {
			t.Fatalf("expected a merge commit with 2 parents, got: %v", parents)
		}
		msg := gitCmd(t, alice, "log", "-1", "--pretty=%B")
		if !strings.Contains(msg, "Co-authored-by: Bob <bob@example.com>") {
			t.Fatalf("merge commit message missing co-author:\n%s", msg)
		}
	}
}

func TestSuggestedRemoteFromUpstream(t *testing.T) {
	repo := initRepo(t)
	origin := initBareRemote(t)

	gitCmd(t, repo, "remote", "add", "origin", origin)
	gitCmd(t, repo, "push", "-u", "origin", "main")

	withCwd(t, repo)
	remote, remotes, source := suggestedRemote(context.Background())
	if remote != "origin" {
		t.Fatalf("suggestedRemote() remote=%q, want %q", remote, "origin")
	}
	if len(remotes) != 1 || remotes[0] != "origin" {
		t.Fatalf("suggestedRemote() remotes=%v, want %v", remotes, []string{"origin"})
	}
	if !strings.Contains(source, "upstream") {
		t.Fatalf("suggestedRemote() source=%q, want it to mention upstream", source)
	}
}

func TestPrintUsageWithRemotes(t *testing.T) {
	repo := initRepo(t)
	origin := initBareRemote(t)

	gitCmd(t, repo, "remote", "add", "origin", origin)
	gitCmd(t, repo, "push", "-u", "origin", "main")

	withCwd(t, repo)
	var out bytes.Buffer
	if err := printUsage(context.Background(), &out); err != nil {
		t.Fatalf("printUsage err=%v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Available remotes:") || !strings.Contains(got, "origin") {
		t.Fatalf("usage output missing remotes:\n%s", got)
	}
	if !strings.Contains(got, "Using: origin") {
		t.Fatalf("usage output missing chosen remote:\n%s", got)
	}
}

func TestRunDiscoveryViaRun(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	// First group member flow from `usage.tmpl`.
	gitSwitchCreate(t, alice, "feature-x")
	gitCmd(t, alice, "push", "-u", "origin", "feature-x")
	withCwd(t, alice)
	if err := run(context.Background(), []string{"-b", "feature-x"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run(-b) err=%v", err)
	}
	gitCmd(t, alice, "push", "-u", "origin", "alice/feature-x")

	var out bytes.Buffer
	if err := run(context.Background(), nil, &out, io.Discard); err != nil {
		t.Fatalf("run discovery err=%v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Related branches and their diffs") {
		t.Fatalf("discovery output missing header:\n%s", out.String())
	}
}

func TestRunMergeViaRun(t *testing.T) {
	origin := initBareRemote(t)

	seed := initRepo(t)
	gitCmd(t, seed, "remote", "add", "origin", origin)
	gitCmd(t, seed, "push", "-u", "origin", "main")

	// Follow `usage.tmpl`: publish the shared twig, then a peer personal branch.
	// We create the peer branch directly to keep the test setup short.
	gitSwitchCreate(t, seed, "feature-x")
	gitCmd(t, seed, "push", "-u", "origin", "feature-x")
	gitSwitchCreate(t, seed, "bob/feature-x", "feature-x")
	writeFile(t, seed, "bob.txt", "hello from bob\n")
	gitCmd(t, seed, "add", "bob.txt")
	gitCmd(t, seed, "-c", "user.name=Bob", "-c", "user.email=bob@example.com", "commit", "-m", "bob change")
	gitCmd(t, seed, "push", "-u", "origin", "bob/feature-x")

	alice := cloneRepo(t, origin, "Alice", "alice@example.com")
	// Next group member flow from `usage.tmpl`.
	gitCmd(t, alice, "fetch", "origin")
	gitSwitchCreate(t, alice, "feature-x", "origin/feature-x")
	withCwd(t, alice)
	if err := run(context.Background(), []string{"-b", "feature-x"}, io.Discard, io.Discard); err != nil {
		t.Fatalf("run(-b) err=%v", err)
	}
	gitCmd(t, alice, "push", "-u", "origin", "alice/feature-x")
	withStdin(t, "y\n")

	var out bytes.Buffer
	if err := run(context.Background(), []string{"-n", "bob/feature-x"}, &out, io.Discard); err != nil {
		t.Fatalf("run merge err=%v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "skipping automatic push") {
		t.Fatalf("merge output missing -n message:\n%s", out.String())
	}
}

func TestSmartPushSuccessPaths(t *testing.T) {
	repo := initRepo(t)
	origin := initBareRemote(t)
	withCwd(t, repo)

	gitCmd(t, repo, "remote", "add", "origin", origin)
	gitCmd(t, repo, "push", "-u", "origin", "main")

	ctx := context.Background()
	if err := smartPush(ctx); err != nil {
		t.Fatalf("smartPush (upstream) err=%v", err)
	}

	gitCmd(t, repo, "branch", "--unset-upstream")
	gitCmd(t, repo, "config", "--local", "branch.main.pushRemote", "origin")
	if err := smartPush(ctx); err != nil {
		t.Fatalf("smartPush (branch.pushRemote) err=%v", err)
	}

	gitCmd(t, repo, "branch", "--unset-upstream")
	gitCmd(t, repo, "config", "--local", "--unset-all", "branch.main.pushRemote")
	gitCmd(t, repo, "config", "--local", "remote.pushDefault", "origin")
	if err := smartPush(ctx); err != nil {
		t.Fatalf("smartPush (remote.pushDefault) err=%v", err)
	}

	gitCmd(t, repo, "branch", "--unset-upstream")
	gitCmd(t, repo, "config", "--local", "--unset-all", "remote.pushDefault")
	if err := smartPush(ctx); err != nil {
		t.Fatalf("smartPush (sole remote) err=%v", err)
	}
}
