package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// This file defines the CLI surface area using Cobra. The goal is to keep
// parsing and command routing here, while the Git-centric logic lives in
// main.go so it can be exercised by integration tests.

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	root := newRootCmd(stdout, stderr)
	root.SetArgs(args)
	root.SetContext(ctx)
	if err := root.ExecuteContext(ctx); err != nil {
		var uerr usageError
		if errors.As(err, &uerr) {
			return err
		}
		// Cobra returns plain errors for unknown commands/args. Treat those as
		// usage errors so mainExit prints help.
		if isCobraUsageError(err) {
			return usageError{Err: err}
		}
		return err
	}
	return nil
}

func isCobraUsageError(err error) bool {
	if err == nil {
		return false
	}
	var uerr usageError
	if errors.As(err, &uerr) {
		return true
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "unknown command ") ||
		strings.Contains(msg, "unknown flag: ") ||
		strings.Contains(msg, "accepts ") ||
		strings.Contains(msg, "requires at least") ||
		strings.Contains(msg, "requires exactly") ||
		strings.Contains(msg, "invalid argument")
}

func newRootCmd(stdout, stderr io.Writer) *cobra.Command {
	var (
		force       bool
		noPush      bool
		commitDirty bool
		baseBranch  string
	)

	cmd := &cobra.Command{
		Use:           "mob-consensus",
		Short:         "Git workflow helper for mob/pair consensus merges",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		// Root command supports the legacy `-b` branch creation flow until TODO 015
		// step 3 (branch create subcommand) is implemented.
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := options{
				force:       force,
				noPush:      noPush,
				commitDirty: commitDirty,
				baseBranch:  baseBranch,
			}

			// Hard break: `mob-consensus` with no args no longer does discovery or a
			// merge. Use explicit subcommands (ex: `status`, `merge`).
			if opts.baseBranch == "" {
				return usageError{Err: errors.New("mob-consensus: missing command (hint: run `mob-consensus -h`)")}
			}

			user, err := branchUserFromEmail(cmd.Context())
			if err != nil {
				return err
			}

			// `-b` implicitly allows running from any branch (it creates/switches to
			// a personal branch). Keep the legacy behavior.
			opts.force = true
			return runCreateBranch(cmd.Context(), opts, user, cmd.OutOrStdout())
		},
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return usageError{Err: err}
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		if err := printUsage(cmd.Context(), cmd.OutOrStdout()); err != nil {
			printError(cmd.ErrOrStderr(), err)
		}
	})
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		return printUsage(cmd.Context(), cmd.OutOrStdout())
	})

	cmd.PersistentFlags().BoolVarP(&force, "force", "F", false, "force run even if not on a <user>/ branch")
	cmd.PersistentFlags().BoolVarP(&commitDirty, "commit-dirty", "c", false, "commit existing uncommitted changes")
	cmd.PersistentFlags().BoolVarP(&noPush, "no-push", "n", false, "no automatic push after commits")
	cmd.Flags().StringVarP(&baseBranch, "base-branch", "b", "", "create new <user>/<twig> branch based on base branch")

	cmd.AddCommand(newStatusCmd(&force, &noPush, &commitDirty))
	cmd.AddCommand(newMergeCmd(&force, &noPush, &commitDirty))
	cmd.AddCommand(newInitCmd(&commitDirty))
	cmd.AddCommand(newStartCmd(&commitDirty))
	cmd.AddCommand(newJoinCmd(&commitDirty))

	return cmd
}

func newStatusCmd(force, noPush, commitDirty *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Fetch and list related branches for the current twig",
		Long:  "Fetch remote refs, then list related branches ending in */<twig> and show whether each is ahead/behind/diverged/synced.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageError{Err: fmt.Errorf("unexpected argument: %s", args[0])}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := options{
				force:       *force,
				noPush:      *noPush,
				commitDirty: *commitDirty,
			}

			currentBranch, err := gitOutputTrimmed(cmd.Context(), "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return err
			}
			user, err := branchUserFromEmail(cmd.Context())
			if err != nil {
				return err
			}

			if err := requireUserBranch(opts.force, user, currentBranch); err != nil {
				return usageError{Err: err}
			}
			if err := fetchSuggestedRemote(cmd.Context(), ""); err != nil {
				return err
			}
			return runDiscovery(cmd.Context(), opts, currentBranch, cmd.OutOrStdout())
		},
	}
	return cmd
}

func newMergeCmd(force, noPush, commitDirty *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge OTHER_BRANCH",
		Short: "Merge a related branch onto the current branch",
		Long: "Merge OTHER_BRANCH onto the current branch, adding Co-authored-by trailers, opening tools for review/conflict resolution, then committing and (optionally) pushing.\n\n" +
			"If OTHER_BRANCH isn't a local ref, mob-consensus will try to resolve it to <remote>/OTHER_BRANCH and ask for confirmation.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := options{
				force:       *force,
				noPush:      *noPush,
				commitDirty: *commitDirty,
				otherBranch: args[0],
			}

			currentBranch, err := gitOutputTrimmed(cmd.Context(), "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return err
			}
			user, err := branchUserFromEmail(cmd.Context())
			if err != nil {
				return err
			}

			if err := requireUserBranch(opts.force, user, currentBranch); err != nil {
				return usageError{Err: err}
			}
			if err := fetchSuggestedRemote(cmd.Context(), opts.otherBranch); err != nil {
				return err
			}
			return runMerge(cmd.Context(), opts, currentBranch, cmd.OutOrStdout())
		},
	}
	return cmd
}

type onboardingFlags struct {
	twig   string
	base   string
	remote string
	plan   bool
	dryRun bool
	yes    bool
}

func addOnboardingFlags(cmd *cobra.Command, flags *onboardingFlags, includeBase bool) {
	cmd.Flags().StringVar(&flags.twig, "twig", "", "shared twig branch name")
	if includeBase {
		cmd.Flags().StringVar(&flags.base, "base", "", "base ref (default: current branch)")
	}
	cmd.Flags().StringVar(&flags.remote, "remote", "", "remote name to use for fetch/push")
	cmd.Flags().BoolVar(&flags.plan, "plan", false, "print the plan (commands + explanations) and exit")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "print commands only; no prompts or execution")
	cmd.Flags().BoolVar(&flags.yes, "yes", false, "accept defaults and run non-interactively")
}

func validateOnboardingFlags(flags onboardingFlags) error {
	if flags.plan && flags.dryRun {
		return usageError{Err: errors.New("--plan and --dry-run are mutually exclusive")}
	}
	return nil
}

func newInitCmd(commitDirty *bool) *cobra.Command {
	var flags onboardingFlags
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Fetch and suggest start vs join, then optionally run it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOnboardingFlags(flags); err != nil {
				return err
			}

			currentBranch, err := gitOutputTrimmed(cmd.Context(), "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return err
			}
			user, err := branchUserFromEmail(cmd.Context())
			if err != nil {
				return err
			}

			opts := options{
				commitDirty: *commitDirty,
				twig:        flags.twig,
				base:        flags.base,
				remote:      flags.remote,
				plan:        flags.plan,
				dryRun:      flags.dryRun,
				yes:         flags.yes,
			}
			return runInit(cmd.Context(), opts, user, currentBranch, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	addOnboardingFlags(cmd, &flags, true)
	return cmd
}

func newStartCmd(commitDirty *bool) *cobra.Command {
	var flags onboardingFlags
	cmd := &cobra.Command{
		Use:   "start",
		Short: "First member flow: create/push twig, create/push personal branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOnboardingFlags(flags); err != nil {
				return err
			}

			currentBranch, err := gitOutputTrimmed(cmd.Context(), "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return err
			}
			user, err := branchUserFromEmail(cmd.Context())
			if err != nil {
				return err
			}

			opts := options{
				commitDirty: *commitDirty,
				twig:        flags.twig,
				base:        flags.base,
				remote:      flags.remote,
				plan:        flags.plan,
				dryRun:      flags.dryRun,
				yes:         flags.yes,
			}
			return runStart(cmd.Context(), opts, user, currentBranch, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	addOnboardingFlags(cmd, &flags, true)
	return cmd
}

func newJoinCmd(commitDirty *bool) *cobra.Command {
	var flags onboardingFlags
	cmd := &cobra.Command{
		Use:   "join",
		Short: "Next member flow: fetch twig, create/push personal branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateOnboardingFlags(flags); err != nil {
				return err
			}

			currentBranch, err := gitOutputTrimmed(cmd.Context(), "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return err
			}
			user, err := branchUserFromEmail(cmd.Context())
			if err != nil {
				return err
			}

			opts := options{
				commitDirty: *commitDirty,
				twig:        flags.twig,
				remote:      flags.remote,
				plan:        flags.plan,
				dryRun:      flags.dryRun,
				yes:         flags.yes,
			}
			return runJoin(cmd.Context(), opts, user, currentBranch, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	addOnboardingFlags(cmd, &flags, false)
	return cmd
}
