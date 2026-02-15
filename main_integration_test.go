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

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
}

func setupIsolatedGitEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_PAGER", "cat")

	// Keep tests non-interactive.
	t.Setenv("GIT_EDITOR", "true")
}

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
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

func initRepo(t *testing.T) string {
	t.Helper()
	requireGit(t)
	setupIsolatedGitEnv(t)

	dir := t.TempDir()
	gitInitMain(t, dir)
	gitCmd(t, dir, "config", "--local", "user.name", "Alice")
	gitCmd(t, dir, "config", "--local", "user.email", "alice@example.com")
	gitCmd(t, dir, "config", "--local", "commit.gpgSign", "false")
	gitCmd(t, dir, "config", "--local", "difftool.prompt", "false")
	gitCmd(t, dir, "config", "--local", "mergetool.prompt", "false")
	gitCmd(t, dir, "config", "--local", "difftool.vimdiff.cmd", "true")
	gitCmd(t, dir, "config", "--local", "mergetool.vimdiff.cmd", "true")

	writeFile(t, dir, "README.md", "seed\n")
	gitCmd(t, dir, "add", "README.md")
	gitCmd(t, dir, "commit", "-m", "seed")
	return dir
}

func initBareRemote(t *testing.T) string {
	t.Helper()
	requireGit(t)

	dir := filepath.Join(t.TempDir(), "remote.git")
	cmd := exec.Command("git", "init", "--bare", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}
	return dir
}

func writeCommitMessageEditor(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	editorPath := filepath.Join(dir, "git-editor.sh")
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
	if !strings.Contains(got, "Usage: mob-consensus") {
		t.Fatalf("help output missing usage header:\n%s", got)
	}
	if !strings.Contains(got, "git config --local user.email") {
		t.Fatalf("help output missing identity hint:\n%s", got)
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
	gitCmd(t, repo, "checkout", "-b", "feature-x")
	withCwd(t, repo)

	var out bytes.Buffer
	if err := run(context.Background(), []string{"-b", "feature-x"}, &out, io.Discard); err != nil {
		t.Fatalf("run(-b) err=%v\n%s", err, out.String())
	}
	if got := strings.TrimSpace(gitCmd(t, repo, "branch", "--show-current")); got != "alice/feature-x" {
		t.Fatalf("current branch=%q, want %q", got, "alice/feature-x")
	}

	out.Reset()
	if err := run(context.Background(), []string{"-b", "feature-x"}, &out, io.Discard); err != nil {
		t.Fatalf("run(-b) second time err=%v\n%s", err, out.String())
	}
	if got := strings.TrimSpace(gitCmd(t, repo, "branch", "--show-current")); got != "alice/feature-x" {
		t.Fatalf("current branch=%q, want %q", got, "alice/feature-x")
	}
}

func TestRunCreateBranchDirtyFails(t *testing.T) {
	repo := initRepo(t)
	gitCmd(t, repo, "checkout", "-b", "feature-x")
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

func TestRequireUserBranchUsageError(t *testing.T) {
	repo := initRepo(t)
	gitCmd(t, repo, "checkout", "-b", "feature-x")
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

	gitCmd(t, repo, "checkout", "-b", "alice/feature-x")
	gitCmd(t, repo, "checkout", "-b", "bob/feature-x", "main")
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

func TestRunDiscoveryStatusLines(t *testing.T) {
	repo := initRepo(t)

	gitCmd(t, repo, "checkout", "-b", "alice/feature-x")
	writeFile(t, repo, "alice.txt", "alice\n")
	gitCmd(t, repo, "add", "alice.txt")
	gitCmd(t, repo, "commit", "-m", "alice change")

	gitCmd(t, repo, "checkout", "-b", "carol/feature-x")
	writeFile(t, repo, "carol.txt", "carol\n")
	gitCmd(t, repo, "add", "carol.txt")
	gitCmd(t, repo, "-c", "user.name=Carol", "-c", "user.email=carol@example.com", "commit", "-m", "carol change")

	gitCmd(t, repo, "checkout", "alice/feature-x")
	gitCmd(t, repo, "branch", "eve/feature-x")
	gitCmd(t, repo, "checkout", "-b", "dave/feature-x", "main")

	gitCmd(t, repo, "checkout", "-b", "bob/feature-x", "main")
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

	gitCmd(t, repo, "checkout", "-b", "bob/feature-x")
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
