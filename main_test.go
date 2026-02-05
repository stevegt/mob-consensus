package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestTwigFromBranch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		branch string
		want   string
	}{
		{branch: "stevegt/foo", want: "foo"},
		{branch: "remotes/origin/stevegt/foo", want: "foo"},
		{branch: "foo", want: "foo"},
		{branch: " foo \n", want: "foo"},
	}
	for _, tt := range tests {
		got := twigFromBranch(tt.branch)
		if got != tt.want {
			t.Fatalf("twigFromBranch(%q)=%q, want %q", tt.branch, got, tt.want)
		}
	}
}

func TestRelatedBranches(t *testing.T) {
	t.Parallel()

	in := strings.Join([]string{
		"* stevegt/twig",
		"  alice/twig",
		"  remotes/origin/alice/twig",
		"  remotes/origin/HEAD -> origin/master",
		"  remotes/origin/stevegt/other",
		"",
	}, "\n")

	got := relatedBranches(in, "twig")
	want := []string{
		"stevegt/twig",
		"alice/twig",
		"remotes/origin/alice/twig",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("relatedBranches()=%q, want %q", got, want)
	}
}

func TestCoAuthorLines(t *testing.T) {
	t.Parallel()

	in := strings.Join([]string{
		"Co-authored-by: Zed <zed@example.com>",
		"Co-authored-by: Me <me@example.com>",
		"Co-authored-by: Alice <alice@example.com>",
		"Co-authored-by: Alice <alice@example.com>",
		"",
	}, "\n")

	got := coAuthorLines(in, "me@example.com")
	want := []string{
		"Co-authored-by: Alice <alice@example.com>",
		"Co-authored-by: Zed <zed@example.com>",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("coAuthorLines()=%q, want %q", got, want)
	}
}

func TestDiffStatusLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		branch string
		ahead  string
		behind string
		want   string
	}{
		{
			branch: "alice/twig",
			ahead:  "AHEAD",
			behind: "BEHIND",
			want:   fmt.Sprintf("%40s has diverged: ahead: %s; behind: %s", "alice/twig", "AHEAD", "BEHIND"),
		},
		{
			branch: "alice/twig",
			ahead:  "AHEAD",
			want:   fmt.Sprintf("%40s is ahead: %s", "alice/twig", "AHEAD"),
		},
		{
			branch: "alice/twig",
			behind: "BEHIND",
			want:   fmt.Sprintf("%40s is behind: %s", "alice/twig", "BEHIND"),
		},
		{
			branch: "alice/twig",
			want:   fmt.Sprintf("%40s is synced", "alice/twig"),
		},
	}
	for _, tt := range tests {
		got := diffStatusLine(tt.branch, tt.ahead, tt.behind)
		if got != tt.want {
			t.Fatalf("diffStatusLine(%q,%q,%q)=%q, want %q", tt.branch, tt.ahead, tt.behind, got, tt.want)
		}
	}
}
