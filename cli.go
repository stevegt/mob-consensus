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
	)

	cmd := &cobra.Command{
		Use:           "mob-consensus",
		Short:         "Git workflow helper for mob/pair consensus merges",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return usageError{Err: errors.New("mob-consensus: missing command (hint: run `mob-consensus -h`)")}
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

	cmd.AddCommand(newStatusCmd(&force, &noPush, &commitDirty))
	cmd.AddCommand(newBranchCmd(&noPush, &commitDirty))
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

func newBranchCmd(noPush, commitDirty *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Branch helpers",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newBranchCreateCmd(noPush, commitDirty))
	return cmd
}

func newBranchCreateCmd(noPush, commitDirty *bool) *cobra.Command {
	var fromRef string
	cmd := &cobra.Command{
		Use:   "create TWIG",
		Short: "Create/switch to your personal <user>/<twig> branch",
		Long: "Create (or switch to) your personal <user>/<twig> branch for the given TWIG.\n\n" +
			"By default, the branch is created from the current local branch. Use --from to create it from an explicit ref.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			twig := args[0]
			if err := validateBranchName(cmd.Context(), "twig", twig); err != nil {
				return usageError{Err: err}
			}

			currentBranch, err := gitOutputTrimmed(cmd.Context(), "rev-parse", "--abbrev-ref", "HEAD")
			if err != nil {
				return err
			}

			baseRef := strings.TrimSpace(fromRef)
			if baseRef == "" {
				baseRef = currentBranch
			}
			if baseRef == "" || baseRef == "HEAD" {
				return usageError{Err: errors.New("mob-consensus: could not determine a base ref (hint: pass --from <ref>)")}
			}

			user, err := branchUserFromEmail(cmd.Context())
			if err != nil {
				return err
			}

			opts := options{
				noPush:      *noPush,
				commitDirty: *commitDirty,
				twig:        twig,
				base:        baseRef,
			}
			return runCreateBranch(cmd.Context(), opts, user, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&fromRef, "from", "", "base ref (default: current branch)")
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
