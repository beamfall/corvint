// Package tracemigraterepo binds trace migration to stable Git repository authority.
package tracemigraterepo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
)

const (
	maximumAncestry       = 10_000
	maximumAncestryBytes  = 2 << 20
	maximumTreeBytes      = 64 << 20
	maximumHistoricalPath = 200_000
)

// Options are the public migrate-traces mode and optional confirmation digest.
type Options struct {
	Apply      bool
	PlanDigest *string
}

// Evaluate plans or applies one trace migration against frozen repository authority.
func Evaluate(ctx context.Context, root string, options Options) (trace.MigrationResult, error) {
	if options.Apply {
		if options.PlanDigest == nil || !lowerHex(*options.PlanDigest, 64) {
			return trace.MigrationResult{}, fmt.Errorf("trace migration apply requires a dry-run plan digest")
		}
	} else if options.PlanDigest != nil {
		return trace.MigrationResult{}, fmt.Errorf("dry-run trace migration does not accept a plan digest")
	}
	authority, stable, err := bindAuthority(ctx, root)
	if err != nil {
		return trace.MigrationResult{}, err
	}
	if options.Apply {
		plan, err := trace.ApplyMigration(root, authority, *options.PlanDigest, stable)
		if err != nil {
			return trace.MigrationResult{}, err
		}
		return plan.Result(true), nil
	}
	plan, err := trace.PlanMigration(root, authority, stable)
	if err != nil {
		return trace.MigrationResult{}, err
	}
	return plan.Result(false), nil
}

func bindAuthority(ctx context.Context, root string) (trace.MigrationAuthority, func() error, error) {
	index, err := contextindex.Build(ctx, root)
	if err != nil {
		return trace.MigrationAuthority{}, nil, err
	}
	budget := gitrun.NewDefaultBudget()
	raw, err := runGit(ctx, budget, root, maximumAncestryBytes,
		"log", fmt.Sprintf("--max-count=%d", maximumAncestry+1), "--format=%H %T", "HEAD")
	if err != nil {
		return trace.MigrationAuthority{}, nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	if len(lines) > maximumAncestry {
		return trace.MigrationAuthority{}, nil, fmt.Errorf("Git ancestry exceeds bounded trace replay limit %d", maximumAncestry)
	}
	length := objectIDLength(index.ObjectFormat)
	commits := make(map[string]trace.Revision, len(lines))
	trees := make(map[string][]string)
	currentPaths := sortedSourcePaths(index.Sources)
	for _, line := range lines {
		fields := strings.Split(line, " ")
		if len(fields) != 2 || !lowerHex(fields[0], length) || !lowerHex(fields[1], length) {
			return trace.MigrationAuthority{}, nil, fmt.Errorf("Git ancestry contains malformed commit/tree identity")
		}
		paths := []string(nil)
		if fields[1] == index.Revision {
			paths = currentPaths
		}
		commits[fields[0]] = trace.Revision{TreeRevision: fields[1], TrackedPaths: paths}
		trees[fields[1]] = append(trees[fields[1]], fields[0])
	}
	for tree := range trees {
		sort.Strings(trees[tree])
	}
	dirty := append([]string(nil), index.DirtyPaths...)
	authority := trace.MigrationAuthority{
		CommitRevision: index.CommitRevision, TreeRevision: index.Revision,
		ObjectFormat: index.ObjectFormat, DirtyPaths: dirty, ProfileID: index.ProfileID,
		Commits: commits, Trees: trees,
	}
	trackedCache := map[string][]string{index.Revision: currentPaths}
	authority.TrackedPaths = func(tree string) ([]string, error) {
		if cached, ok := trackedCache[tree]; ok {
			return append([]string(nil), cached...), nil
		}
		raw, err := runGit(ctx, budget, root, maximumTreeBytes+1, "ls-tree", "-r", "--name-only", "-z", tree)
		if err != nil {
			return nil, err
		}
		if len(raw) > maximumTreeBytes {
			return nil, fmt.Errorf("historical trace tree exceeds %d bytes", maximumTreeBytes)
		}
		if bytes.Count(raw, []byte{0}) > maximumHistoricalPath {
			return nil, fmt.Errorf("historical trace tree exceeds %d paths", maximumHistoricalPath)
		}
		paths := make([]string, 0, bytes.Count(raw, []byte{0}))
		for _, path := range bytes.Split(raw, []byte{0}) {
			if len(path) != 0 {
				paths = append(paths, string(path))
			}
		}
		trackedCache[tree] = paths
		return append([]string(nil), paths...), nil
	}
	// The check reads only header fields, so it observes them rather than
	// compiling an index whose sources it would discard. ProfileID is not
	// compared because it is a pure function of the tree: an equal Revision
	// already implies an equal ProfileID.
	stable := func() error {
		current, err := contextindex.Observe(ctx, root)
		if err != nil {
			return err
		}
		if current.CommitRevision != index.CommitRevision || current.Revision != index.Revision ||
			current.ObjectFormat != index.ObjectFormat ||
			!equalStrings(current.DirtyPaths, dirty) {
			return fmt.Errorf("repository identity changed")
		}
		return nil
	}
	return authority, stable, nil
}

func runGit(ctx context.Context, budget *gitrun.Budget, root string, limit int, arguments ...string) ([]byte, error) {
	args := []string{
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false",
		"-c", "core.excludesFile=", "-c", "credential.helper=", "-c", "submodule.recurse=false", "-C", root,
		"-c", "advice.graftFileDeprecated=false",
	}
	args = append(args, arguments...)
	return gitrun.Run(ctx, budget, gitrun.Options{Env: gitEnvironment(), StdoutLimit: limit}, args...)
}

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
		"GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE="+os.DevNull,
		"GCM_INTERACTIVE=never", "GIT_ASKPASS=",
	)
}

func sortedSourcePaths(sources map[string]contextindex.Source) []string {
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func objectIDLength(format string) int {
	if format == "sha256" {
		return 64
	}
	return 40
}

func lowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return false
		}
	}
	return true
}
