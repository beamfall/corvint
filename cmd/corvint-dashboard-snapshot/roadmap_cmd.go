package main

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/dashboard/roadmap"
)

// roadmapOptions is the parsed `corvint-dashboard-snapshot roadmap` argument
// set, in the same explicit-flag style as `snapshot`'s options.
type roadmapOptions struct {
	Root            string
	AtmBinary       string
	StoreRoot       string
	ReceiptsDir     string
	DocsStatePath   string
	RequirementsTSV string
	GeneratedAt     string
}

func parseRoadmapArguments(arguments []string) (roadmapOptions, error) {
	var parsed roadmapOptions
	seen := make(map[string]bool)
	var tasksBinary, legacyTasksBinary string
	for index := 1; index < len(arguments); {
		flag := arguments[index]
		if index+1 >= len(arguments) || !strings.HasPrefix(flag, "--") {
			return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
		}
		value := arguments[index+1]
		index += 2
		switch flag {
		case "--root":
			if seen[flag] || !validRoot(value) {
				return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			parsed.Root = value
		case "--tasks":
			if seen[flag] || !validArgumentString(value) {
				return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			tasksBinary = value
		case "--atm":
			if seen[flag] || !validArgumentString(value) {
				return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			legacyTasksBinary = value
		case "--store":
			if seen[flag] || !validRoot(value) {
				return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			parsed.StoreRoot = value
		case "--receipts-dir":
			if seen[flag] || !validRoot(value) {
				return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			parsed.ReceiptsDir = value
		case "--docs-state":
			if seen[flag] || !validRoot(value) {
				return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			parsed.DocsStatePath = value
		case "--requirements":
			if seen[flag] || !validRoot(value) {
				return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			parsed.RequirementsTSV = value
		case "--generated-at":
			if seen[flag] || !validGeneratedAt(value) {
				return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			parsed.GeneratedAt = value
		default:
			return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
		}
	}
	selected, err := selectRoadmapTasks(tasksBinary, seen["--tasks"], legacyTasksBinary, seen["--atm"])
	if err != nil {
		return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
	}
	parsed.AtmBinary = selected
	if parsed.Root == "" || parsed.AtmBinary == "" || parsed.StoreRoot == "" || parsed.GeneratedAt == "" {
		return roadmapOptions{}, &dashboardError{code: errorInvalidArgument}
	}
	if parsed.RequirementsTSV == "" {
		parsed.RequirementsTSV = filepath.Join(parsed.Root, "docs", "specs", "REQUIREMENTS.tsv")
	}
	return parsed, nil
}

func selectRoadmapTasks(tasks string, tasksSet bool, legacy string, legacySet bool) (string, error) {
	if tasksSet && tasks == "" {
		return "", &dashboardError{code: errorInvalidArgument}
	}
	if legacySet && legacy == "" {
		return "", &dashboardError{code: errorInvalidArgument}
	}
	if tasksSet && legacySet && tasks != legacy {
		return "", &dashboardError{code: errorInvalidArgument}
	}
	if tasksSet {
		return tasks, nil
	}
	if legacySet {
		return legacy, nil
	}
	return "", &dashboardError{code: errorInvalidArgument}
}

// runRoadmapContext is the `roadmap` subcommand's runContext counterpart: it
// parses arguments, resolves the current tree digest (best-effort — its
// absence only widens receipts to NOT_OBSERVED, it never aborts the join),
// compiles the snapshot, and writes it exactly like `snapshot` does.
func runRoadmapContext(ctx context.Context, arguments []string, stdout, stderr io.Writer) (exit int) {
	defer func() {
		if recover() != nil {
			emitError(stderr, errorInternal)
			exit = 2
		}
	}()
	parsed, err := parseRoadmapArguments(arguments)
	if err != nil {
		emitError(stderr, errorInvalidArgument)
		return 2
	}
	if err := ctx.Err(); err != nil {
		emitError(stderr, errorInterrupted)
		return 2
	}
	treeDigest, digestErr := roadmap.CurrentTreeDigest(ctx, parsed.Root, 10*time.Second)
	if digestErr != nil {
		treeDigest = ""
	}
	_, encoded, compileErr := roadmap.Compile(ctx, roadmap.Options{
		AtmBinary: parsed.AtmBinary, StoreRoot: parsed.StoreRoot, RepoRoot: parsed.Root,
		RequirementsTSV: parsed.RequirementsTSV, ReceiptsDir: parsed.ReceiptsDir,
		DocsStatePath: parsed.DocsStatePath, TreeDigest: treeDigest,
		GeneratedAt: parsed.GeneratedAt, Timeout: 30 * time.Second,
	})
	// A cancellation during Compile surfaces as a NOT_OBSERVED atm read, not
	// an error, so it is checked before any compile outcome is trusted.
	if err := ctx.Err(); err != nil {
		emitError(stderr, errorInterrupted)
		return 2
	}
	if compileErr != nil {
		emitError(stderr, errorInternal)
		return 2
	}
	if len(encoded) > maxSnapshotBytes {
		emitError(stderr, errorResource)
		return 2
	}
	if !writeSnapshot(stdout, encoded) {
		emitError(stderr, errorOutputWrite)
		return 2
	}
	return 0
}
