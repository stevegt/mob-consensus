package main

import (
	"errors"
	"fmt"
	"io"
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

func TestParseArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		wantHelp bool
		wantOpts options
		wantErr  bool
	}{
		{
			name:     "empty",
			args:     nil,
			wantHelp: false,
			wantOpts: options{},
		},
		{
			name:     "help_short",
			args:     []string{"-h"},
			wantHelp: true,
			wantOpts: options{},
		},
		{
			name:     "help_long",
			args:     []string{"--help"},
			wantHelp: true,
			wantOpts: options{},
		},
		{
			name: "flags_and_other",
			args: []string{"-F", "-c", "-n", "-b", "feature-x", "bob/feature-x"},
			wantOpts: options{
				force:       true,
				baseBranch:  "feature-x",
				noPush:      true,
				commitDirty: true,
				otherBranch: "bob/feature-x",
			},
		},
		{
			name:    "unknown_flag",
			args:    []string{"--nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, help, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs() err=%v, wantErr=%v", err, tt.wantErr)
			}
			if help != tt.wantHelp {
				t.Fatalf("parseArgs() help=%v, want %v", help, tt.wantHelp)
			}
			if tt.wantErr {
				return
			}
			if opts != tt.wantOpts {
				t.Fatalf("parseArgs() opts=%+v, want %+v", opts, tt.wantOpts)
			}
		})
	}
}

func TestConfirm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want bool
	}{
		{in: "y\n", want: true},
		{in: "yes\n", want: true},
		{in: "Y\n", want: true},
		{in: "n\n", want: false},
		{in: "\n", want: false},
		{in: "y", want: true},       // EOF without newline
		{in: "maybe", want: false},  // EOF without newline
		{in: " yes ", want: true},   // whitespace + EOF
		{in: "nope\n", want: false}, // unknown token
	}

	for _, tt := range tests {
		got, err := confirm(strings.NewReader(tt.in), io.Discard, "prompt: ")
		if err != nil {
			t.Fatalf("confirm() err=%v", err)
		}
		if got != tt.want {
			t.Fatalf("confirm(%q)=%v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestUsageErrorUnwrap(t *testing.T) {
	t.Parallel()

	underlying := errors.New("underlying")
	err := usageError{Err: underlying}

	if got := errors.Unwrap(err); got != underlying {
		t.Fatalf("errors.Unwrap(usageError)=%v, want %v", got, underlying)
	}
	if !errors.Is(err, underlying) {
		t.Fatalf("errors.Is(usageError, underlying)=false, want true")
	}
}

func TestPrintErrorNil(t *testing.T) {
	t.Parallel()

	var out strings.Builder
	printError(&out, nil)
	if got := out.String(); got != "" {
		t.Fatalf("printError(nil)=%q, want empty string", got)
	}
}

func TestRequireUserBranch(t *testing.T) {
	t.Parallel()

	if err := requireUserBranch(true, "alice", "main"); err != nil {
		t.Fatalf("requireUserBranch(force=true) err=%v, want nil", err)
	}
	if err := requireUserBranch(false, "alice", "alice/feature-x"); err != nil {
		t.Fatalf("requireUserBranch(on user branch) err=%v, want nil", err)
	}
	if err := requireUserBranch(false, "alice", "bob/feature-x"); err == nil {
		t.Fatalf("requireUserBranch(on non-user branch) err=nil, want error")
	}
}
