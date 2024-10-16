package ci

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/dev/ci/gitops"
	"github.com/sourcegraph/sourcegraph/dev/ci/images"
	"github.com/sourcegraph/sourcegraph/dev/ci/internal/ci/changed"
	"github.com/sourcegraph/sourcegraph/dev/ci/runtype"
	"github.com/sourcegraph/sourcegraph/internal/execute"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Config is the set of configuration parameters that determine the structure of the CI build. These
// parameters are extracted from the build environment (branch name, commit hash, timestamp, etc.)
type Config struct {
	// RunType indicates what kind of pipeline run should be generated, based on various
	// bits of metadata
	RunType runtype.RunType

	// Build metadata
	Time        time.Time
	Branch      string
	Version     string
	Commit      string
	BuildNumber int

	// Diff denotes what has changed since the merge-base with origin/main.
	Diff changed.Diff
	// ChangedFiles lists files that have changed, group by diff type
	ChangedFiles changed.ChangedFiles

	// MustIncludeCommit, if non-empty, is a list of commits at least one of which must be present
	// in the branch. If empty, then no check is enforced.
	MustIncludeCommit []string

	// MessageFlags contains flags parsed from commit messages.
	MessageFlags MessageFlags

	// Notify declares configuration required to generate notifications.
	Notify SlackNotification
}

type SlackNotification struct {
	// An Buildkite "Notification service" must exist for this channel in order for notify
	// to work. This is configured here: https://buildkite.com/organizations/sourcegraph/services
	//
	// Under "Choose notifications to send", uncheck the option for "Failed" state builds.
	// Failure notifications will be generated by the pipeline generator.
	Channel string
	// This Slack token is used for retrieving Slack user data to generate messages.
	SlackToken string
}

// NewConfig computes configuration for the pipeline generator based on Buildkite environment
// variables.
func NewConfig(now time.Time) Config {
	var (
		commit = os.Getenv("BUILDKITE_COMMIT")
		branch = os.Getenv("BUILDKITE_BRANCH")
		tag    = os.Getenv("BUILDKITE_TAG")
		// evaluates what type of pipeline run this is
		runType = runtype.Compute(tag, branch, map[string]string{
			"BEXT_NIGHTLY":       os.Getenv("BEXT_NIGHTLY"),
			"RELEASE_NIGHTLY":    os.Getenv("RELEASE_NIGHTLY"),
			"VSCE_NIGHTLY":       os.Getenv("VSCE_NIGHTLY"),
			"WOLFI_BASE_REBUILD": os.Getenv("WOLFI_BASE_REBUILD"),
			"RELEASE_INTERNAL":   os.Getenv("RELEASE_INTERNAL"),
			"RELEASE_PUBLIC":     os.Getenv("RELEASE_PUBLIC"),
			"CLOUD_EPHEMERAL":    os.Getenv("CLOUD_EPHEMERAL"),
		})
		// defaults to 0
		buildNumber, _ = strconv.Atoi(os.Getenv("BUILDKITE_BUILD_NUMBER"))
	)

	var mustIncludeCommits []string
	if rawMustIncludeCommit := os.Getenv("MUST_INCLUDE_COMMIT"); rawMustIncludeCommit != "" {
		mustIncludeCommits = strings.Split(rawMustIncludeCommit, ",")
		for i := range mustIncludeCommits {
			mustIncludeCommits[i] = strings.TrimSpace(mustIncludeCommits[i])
		}
	}

	// detect changed files
	var changedFiles []string
	var err error
	if commit != "" {
		if runType.Is(runtype.MainBranch) {
			// We run builds on every commit in main, so on main, just look at the diff of the current commit.
			changedFiles, err = gitops.GetHEADChangedFiles()
		} else {
			baseBranch := os.Getenv("BUILDKITE_PULL_REQUEST_BASE_BRANCH")
			changedFiles, err = gitops.GetBranchChangedFiles(baseBranch, commit)
		}
	} else {
		out, giterr := execute.Git(context.Background(), "rev-parse", "HEAD")
		if giterr != nil {
			panic(giterr)
		}

		commit = strings.TrimSpace(string(out))
		changedFiles, err = gitops.GetBranchChangedFiles("main", commit)
	}

	if err != nil {
		panic(err)
	}

	diff, changedFilesByDiffType := changed.ParseDiff(changedFiles)

	fmt.Fprintf(os.Stderr, "Parsed diff:\n\tchanged files: %v\n\tdiff changes: %q\n",
		changedFiles,
		diff.String(),
	)
	fmt.Fprint(os.Stderr, "The generated build pipeline will now follow, see you next time!\n")

	return Config{
		RunType: runType,

		Time:              now,
		Branch:            branch,
		Version:           inferVersion(runType, tag, commit, buildNumber, branch, now),
		Commit:            commit,
		MustIncludeCommit: mustIncludeCommits,
		Diff:              diff,
		ChangedFiles:      changedFilesByDiffType,
		BuildNumber:       buildNumber,

		// get flags from commit message
		MessageFlags: parseMessageFlags(os.Getenv("BUILDKITE_MESSAGE")),

		Notify: SlackNotification{
			Channel:    "#buildkite-main",
			SlackToken: os.Getenv("SLACK_INTEGRATION_TOKEN"),
		},
	}
}

// inferVersion constructs the Sourcegraph version from the given build state.
func inferVersion(runType runtype.RunType, tag string, commit string, buildNumber int, branch string, now time.Time) string {
	// If we're building a release, use the version that is being released regardless of
	// all other build attributes, such as tag, commit, build number, etc ...
	if runType.Is(runtype.InternalRelease, runtype.PromoteRelease) {
		return os.Getenv("VERSION")
	}

	if runType.Is(runtype.TaggedRelease) {
		// This tag is used for publishing versioned releases.
		//
		// The Git tag "v1.2.3" should map to the Docker image "1.2.3" (without v prefix).
		return strings.TrimPrefix(tag, "v")
	}

	// need to determine the tag
	latestTag, err := gitops.GetLatestTag()
	if err != nil {
		if errors.Is(err, gitops.ErrNoTags) {
			// use empty string if no tags are present
			fmt.Fprintf(os.Stderr, "no tags found, using empty string\n")
		} else {
			fmt.Fprintf(os.Stderr, "error determining latest tag: %v\n", err)
		}
		latestTag = ""
	}
	// "main" branch is used for continuous deployment and has a special-case format
	version := images.BranchImageTag(now, commit, buildNumber, branch, latestTag)

	// Add additional patch suffix
	if runType.Is(runtype.ImagePatch, runtype.ImagePatchNoTest, runtype.ExecutorPatchNoTest) {
		version = version + "_patch"
	}

	return version
}

func (c Config) shortCommit() string {
	// http://git-scm.com/book/en/v2/Git-Tools-Revision-Selection#Short-SHA-1
	if len(c.Commit) < 12 {
		return c.Commit
	}

	return c.Commit[:12]
}

func (c Config) ensureCommit() error {
	if len(c.MustIncludeCommit) == 0 {
		return nil
	}

	found, errs := gitops.HasIncludedCommit(c.MustIncludeCommit...)

	if !found {
		fmt.Printf("This branch %q at commit %s does not include any of these commits: %s.\n", c.Branch, c.Commit, strings.Join(c.MustIncludeCommit, ", "))
		fmt.Println("Rebase onto the latest main to get the latest CI fixes.")
		fmt.Printf("Errors from `git merge-base --is-ancestor $COMMIT HEAD`: %s", errs)
		return errs
	}
	return nil
}

// candidateImageTag provides the tag for a candidate image built for this Buildkite run.
//
// Note that the availability of this image depends on whether a candidate gets built,
// as determined in `addDockerImages()`.
func (c Config) candidateImageTag() string {
	return images.CandidateImageTag(c.Commit, c.BuildNumber)
}

// MessageFlags indicates flags that can be parsed out of commit messages to change
// pipeline behaviour. Use sparingly! If you are generating a new pipeline, please use
// RunType instead.
type MessageFlags struct {
	// ProfilingEnabled, if true, tells buildkite to print timing and resource utilization information
	// for each command
	ProfilingEnabled bool

	// SkipHashCompare, if true, tells buildkite to disable skipping of steps that compare
	// hash output.
	SkipHashCompare bool

	// NoBazel, if true prevents automatic replacement of job with their Bazel equivalents.
	NoBazel bool
}

// parseMessageFlags gets MessageFlags from the given commit message.
func parseMessageFlags(msg string) MessageFlags {
	return MessageFlags{
		ProfilingEnabled: strings.Contains(msg, "[buildkite-enable-profiling]"),
		SkipHashCompare:  strings.Contains(msg, "[skip-hash-compare]"),
	}
}
