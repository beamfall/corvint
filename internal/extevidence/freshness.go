package extevidence

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// Freshness states from EEP-V0-009.
const (
	FreshnessEqual               = "equal"
	FreshnessRepositoryAhead     = "repository-ahead"
	FreshnessProviderAhead       = "provider-ahead"
	FreshnessUnrelatedHistory    = "unrelated-history"
	FreshnessRevisionUnavailable = "revision-unavailable"
)

const gitOperationBudget = 5 * time.Second

// Freshness compares the provider's observed revision with the captured
// commit revision by Git ancestry alone (EEP-V0-009).
func Freshness(ctx context.Context, root, captured, provider string) string {
	if provider == captured {
		return FreshnessEqual
	}
	if !isKnownCommit(ctx, root, provider) {
		return FreshnessRevisionUnavailable
	}
	providerIsAncestor, ok := isAncestor(ctx, root, provider, captured)
	if !ok {
		return FreshnessRevisionUnavailable
	}
	if providerIsAncestor {
		return FreshnessRepositoryAhead
	}
	capturedIsAncestor, ok := isAncestor(ctx, root, captured, provider)
	if !ok {
		return FreshnessRevisionUnavailable
	}
	if capturedIsAncestor {
		return FreshnessProviderAhead
	}
	return FreshnessUnrelatedHistory
}

func isKnownCommit(ctx context.Context, root, revision string) bool {
	if !blobPattern.MatchString(revision) {
		return false
	}
	output, err := runGit(ctx, root, nil, "rev-parse", "--verify", "--quiet", revision+"^{commit}")
	return err == nil && strings.TrimSpace(string(output)) == revision
}

// isAncestor reports whether ancestor reaches descendant; ok is false when Git
// answered with anything other than its yes/no exit codes.
func isAncestor(ctx context.Context, root, ancestor, descendant string) (bool, bool) {
	_, err := runGit(ctx, root, nil, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, true
	}
	details, exited := cemcode.GitExitFailureDetails(err)
	if exited && details.ExitCode == 1 {
		return false, true
	}
	return false, false
}

func runGit(ctx context.Context, root string, stdin []byte, args ...string) ([]byte, error) {
	options := gitrun.Options{Dir: root, Env: gitEnvironment(), Stdin: stdin, StdoutLimit: 1 << 20}
	return gitrun.RunReserved(ctx, gitOperationBudget, options, args...)
}

// gitEnvironment is the same scrubbed child environment the other gitrun
// callers pass: an allowlist of what Git needs plus determinism settings.
func gitEnvironment() []string {
	environment := make([]string, 0, 16)
	for _, name := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if value, exists := os.LookupEnv(name); exists {
			environment = append(environment, name+"="+value)
		}
	}
	return append(environment,
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0",
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}
