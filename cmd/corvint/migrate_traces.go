package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracemigraterepo"
)

func parseMigrateTracesInvocation(arguments []string) (string, []string, bool, error) {
	index := 0
	root := ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
	}
	if index >= len(arguments) || arguments[index] != "migrate-traces" {
		return "", nil, false, nil
	}
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return "", nil, true, argumentError("cannot resolve current directory")
		}
		root = workingDirectory
	} else {
		resolved, err := resolveExplicitRoot(root)
		if err != nil {
			return "", nil, true, argumentError("argument --root: " + err.Error())
		}
		root = resolved
	}
	return root, arguments[index+1:], true, nil
}

func parseMigrateTracesFlags(arguments []string) (tracemigraterepo.Options, error) {
	var options tracemigraterepo.Options
	mode := ""
	for index := 0; index < len(arguments); {
		argument := arguments[index]
		name, value, inline := strings.Cut(argument, "=")
		switch name {
		case "--dry-run", "--apply":
			if inline {
				return options, argumentError("unrecognized arguments: " + argument)
			}
			if mode != "" && mode != name {
				return options, argumentError("argument " + name + ": not allowed with argument " + mode)
			}
			mode = name
			options.Apply = name == "--apply"
			index++
		case "--plan-digest":
			if !inline {
				if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
					return options, argumentError("argument --plan-digest: expected one argument")
				}
				value = arguments[index+1]
				index += 2
			} else {
				index++
			}
			options.PlanDigest = &value
		default:
			return options, argumentError("unrecognized arguments: " + argument)
		}
	}
	if mode == "" {
		return options, argumentError("one of the arguments --dry-run --apply is required")
	}
	return options, nil
}

func runMigrateTraces(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseMigrateTracesFlags(arguments)
	if err != nil {
		emitError(stderr, err)
		return 2
	}
	result, err := tracemigraterepo.Evaluate(ctx, root, options)
	if err != nil {
		emitMigrateTracesError(stderr, err)
		return 2
	}
	encoded, err := contextindex.CanonicalJSON(migrateTracesPayload(result))
	if err != nil {
		emitError(stderr, argumentError("cannot encode trace migration output"))
		return 2
	}
	written, err := fmt.Fprintf(stdout, "%s\n", encoded)
	if err != nil || written != len(encoded)+1 {
		emitError(stderr, argumentError("cannot write trace migration output"))
		return 2
	}
	return 0
}

func migrateTracesPayload(result trace.MigrationResult) map[string]any {
	entries := make([]any, 0, len(result.Entries))
	for _, entry := range result.Entries {
		entries = append(entries, map[string]any{
			"tree_revision": entry.TreeRevision, "commit_revision": entry.CommitRevision,
			"source_sha256": entry.SourceSHA256, "target_sha256": entry.TargetSHA256, "row_count": entry.RowCount,
		})
	}
	return map[string]any{
		"ok": true, "tool": "migrate-traces", "mutates": result.Mutates, "mode": result.Mode,
		"plan_digest":           result.PlanDigest,
		"repository":            map[string]any{"commit_revision": result.CommitRevision, "tree_revision": result.TreeRevision},
		"candidate_trace_files": result.CandidateTraceFiles, "legacy_trace_files": result.LegacyTraceFiles,
		"trace_rows": result.TraceRows, "entries": entries,
	}
}

func emitMigrateTracesError(stderr io.Writer, err error) {
	_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"ok\": false}\n", pythonJSONString(err.Error()))
}
