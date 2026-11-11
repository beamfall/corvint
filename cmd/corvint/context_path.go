package main

import (
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/pathbrief"
	"github.com/Beamfall/corvint/internal/testevidencedoc"
)

// pathContextEvidenceReads bounds the retained test-evidence reads of one path brief.
const pathContextEvidenceReads = 512

// isPathContext reports whether the `context` arguments ask for the path brief (TBC-V1-009).
func isPathContext(rest []string) bool {
	return slices.ContainsFunc(rest, func(argument string) bool {
		return argument == "--path" || strings.HasPrefix(argument, "--path=")
	})
}

// parsePathContextArguments reads `context --path PATH [--path PATH ...]`. The path brief takes
// no ticket, task text or change range, so every other argument is refused (TBC-V1-009).
func parsePathContextArguments(options taskContextOptions, rest []string) (taskContextOptions, error) {
	for index := 0; index < len(rest); index++ {
		if !argparseOptionLike(rest[index]) {
			return options, pathContextRangeRefusal("a task text")
		}
		flag, value, inline := strings.Cut(rest[index], "=")
		switch flag {
		case "--path":
		case "--task", "--base":
			return options, pathContextRangeRefusal(flag)
		case "--ticket":
			return options, argumentError("argument --path: not allowed with --ticket; ask for the ticket brief or the path brief, not both")
		default:
			return options, argumentError("argument --path: not allowed with " + flag)
		}
		if !inline {
			if index+1 >= len(rest) || argparseOptionLike(rest[index+1]) {
				return options, argumentError("argument --path: expected one argument")
			}
			value, index = rest[index+1], index+1
		}
		options.paths = append(options.paths, value)
	}
	return options, nil
}

func pathContextRangeRefusal(what string) error {
	return argumentError("argument --path: not allowed with " + what + "; the path brief reads the committed tree at HEAD and takes no task text or change range; the report for a change range is: corvint impact --base <sha>")
}

// runPathContext prints the path brief. It writes nothing, and a missing optional source never
// fails it (TBC-V1-011).
func runPathContext(ctx context.Context, options taskContextOptions, stdout, stderr io.Writer) int {
	paths := make([]string, len(options.paths))
	for index, value := range options.paths {
		normalized, err := pathbrief.NormalizePath(options.root, value)
		if err != nil {
			emitError(stderr, &gokernel.Error{Code: "path-outside-repository", Message: "argument --path: " + strconv.Quote(value) + " is not a path inside the repository"})
			return 2
		}
		paths[index] = normalized
	}
	brief, err := pathbrief.Build(ctx, options.root, paths, pathbrief.Sources{
		Tests:   pathContextTests(options.root),
		Tickets: pathbrief.ExecTickets(options.root),
	})
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "path-brief-unavailable", Message: "cannot read the committed tree at HEAD"})
		return 2
	}
	if err := gokernel.WriteCanonicalJSON(stdout, brief); err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write path brief"})
		return 2
	}
	return 0
}

// pathContextTests is the tests source of the path brief: the affected plan of each file and the
// retained test evidence that names it.
func pathContextTests(root string) func(context.Context, string, [][]string) []pathbrief.TestsResult {
	return func(ctx context.Context, revision string, groups [][]string) []pathbrief.TestsResult {
		results := make([]pathbrief.TestsResult, len(groups))
		graph, planMissing := pathContextGraph(ctx, root, revision, groups)
		reads, evidenceMissing := 0, ""
		for index, files := range groups {
			result := &results[index]
			if result.PlanMissing = planMissing; graph != nil {
				pathContextPlan(graph, files, result)
			}
			for _, file := range files {
				if evidenceMissing == "" && reads == pathContextEvidenceReads {
					evidenceMissing = "over-" + strconv.Itoa(pathContextEvidenceReads) + "-files"
				}
				if evidenceMissing != "" {
					break
				}
				document := testevidencedoc.Read(ctx, root, testevidencedoc.Query{Path: file, Revision: revision})
				if reads++; len(document.Records) == 0 {
					// No record is retained at all, so no other path has one either.
					evidenceMissing = pathContextReasons(document.Reasons, "no-retained-record")
					break
				}
				if document.Status != "OBSERVED" {
					result.EvidenceMissing = pathContextReasons(document.Reasons, "unknown")
				}
				for _, match := range document.Matches {
					result.Evidenced = append(result.Evidenced, pathbrief.EvidencedTest{Test: match.Test, Component: match.Component, RecordCommit: match.RecordCommit})
				}
			}
			if evidenceMissing != "" {
				result.EvidenceMissing = evidenceMissing
			}
		}
		return results
	}
}

func pathContextReasons(reasons []string, fallback string) string {
	if len(reasons) == 0 {
		return fallback
	}
	return strings.Join(reasons, "+")
}

// pathContextGraph compiles the affected graph once for every file the brief asks about. It
// returns why the plan is missing when it cannot.
func pathContextGraph(ctx context.Context, root, revision string, groups [][]string) (*affected.Graph, string) {
	files := affected.NormalizePaths(slices.Concat(groups...))
	switch {
	case len(files) == 0:
		return nil, "no-file-at-head"
	case len(files) > affectedMaxPathList:
		return nil, "over-" + strconv.Itoa(affectedMaxPathList) + "-files"
	}
	arguments := []string{"--root", root, "affected"}
	for _, file := range files {
		arguments = append(arguments, "--path", file)
	}
	invocation, _, err := parseAffectedInvocation(arguments)
	if err != nil {
		return nil, pathContextRefusalCode(err)
	}
	receipt, graph, err := compileAffectedGraph(ctx, invocation)
	switch {
	case err != nil:
		return nil, pathContextRefusalCode(err)
	case receipt.Revision != revision:
		return nil, "revision-moved"
	}
	return graph, ""
}

func pathContextRefusalCode(err error) string {
	var refusal *gokernel.Error
	if errors.As(err, &refusal) && refusal.Code != "" {
		return refusal.Code
	}
	return "plan-unavailable"
}

// pathContextPlan selects each file alone over the shared graph and merges the selections. A
// unit selected only as an unbounded reader is counted, not listed, and a make target or CI job
// is not a test (the listing rule of the TBC-V1-013 coverage check).
func pathContextPlan(graph *affected.Graph, files []string, result *pathbrief.TestsResult) {
	reasons, shared := map[string]string{}, map[string]bool{}
	result.Scope = "BOUNDED"
	for _, file := range files {
		plan := affected.SelectLinked(graph, []string{file})
		if plan.Scope != "BOUNDED" {
			result.Scope = plan.Scope
		}
		for _, selection := range plan.Selected {
			switch {
			case selection.Kind != "" || selection.Witness.Kind == affected.WitnessFrontierSite:
				// A make target or CI job runs tests; it is not one.
			case selection.Witness.Kind == affected.WitnessUnboundedReader:
				shared[selection.UnitID] = true
			default:
				if _, listed := reasons[selection.UnitID]; !listed {
					reasons[selection.UnitID] = selection.Witness.Kind
				}
			}
		}
	}
	for unit, reason := range reasons {
		delete(shared, unit)
		result.Planned = append(result.Planned, pathbrief.PlannedTest{Test: unit, Reason: reason})
	}
	result.SharedReaders = len(shared)
}
