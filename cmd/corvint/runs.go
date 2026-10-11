package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/playwrightruns"
)

const runsHelp = `Read Playwright JSON reports across runs (experimental, PWR-V0).

Usage:
  corvint [--root PATH] runs stability --runs FILE[,FILE...] [--runs ...]
    [--min-runs N]
  corvint [--root PATH] runs compare --baseline FILE[,FILE...]
    --candidate FILE[,FILE...]

stability pools each test (project, file, title path) across runs whose
identity inputs match: Playwright version plus the applicationRevision,
configDigest and testFileBlobs[file] a run declares under
config.metadata.corvintRunIdentity. Per pool: DETERMINISTIC (same outcome in
at least --min-runs runs, default 3, minimum 2), INTERMITTENT (outcomes differ,
or a run was flaky; differing first-error lines listed) or INSUFFICIENT with a
reason (BELOW_THRESHOLD, IDENTITY_INPUT_MISSING, IDENTITY_AMBIGUOUS,
RUN_INTERRUPTED, NOT_RUN). An intermittent test.fail() test is flagged
EXPECTED_FAIL_NOT_DETERMINISTIC with the alternatives.

compare matches tests by the same identity across the baseline and candidate
report sets (N repeats per side give k/N stability). Each test is SAME,
CHANGED (outcome, retries, or failed assertions raised in the test file or
without a location), MISSING or NEW; failed assertions raised in other files
(page objects, helpers, fixtures) are listed separately as a support
difference and do not change the verdict. The overall verdict is PARITY,
DIVERGED or INCOMPARABLE with reasons (REPORT_UNREADABLE,
PLAYWRIGHT_VERSION_DIFFERS, PROJECTS_DIFFER, CONFIG_DIFFERS,
INPUTS_DIFFER_WITHIN_SIDE, REPORT_HAS_ERRORS, NO_TESTS, IDENTITY_AMBIGUOUS,
RUN_INTERRUPTED). Per report it lists worker indexes and result durations;
fixture durations and cleanup are NOT_OBSERVED in the JSON reporter.

Reads only the named regular files (relative to --root when given; at most 64
per option, 64 MiB each); writes nothing and uses no network. Exit 0 with JSON
on stdout. Refusals: runs-invalid-arguments, runs-report-unavailable
(stability only), runs-output-unavailable.
Contract: docs/specs/playwright-run-evidence-v0.md (proposed; V1-1230,
V1-1229).
`

var errRunsArgument = errors.New("runs-invalid-arguments")

func runsInvocation(args []string) (string, []string, bool) {
	root := ""
	for len(args) > 0 {
		if args[0] == "--root" {
			if !rootPreambleValue(args, 1) {
				return "", nil, false
			}
			root = args[1]
			args = args[2:]
			continue
		}
		if strings.HasPrefix(args[0], "--root=") {
			root = strings.TrimPrefix(args[0], "--root=")
			args = args[1:]
			continue
		}
		break
	}
	if len(args) == 0 || args[0] != "runs" {
		return "", nil, false
	}
	return root, args[1:], true
}

func runsRefuse(diagnostic io.Writer, code string, fields map[string]any) int {
	body := map[string]any{"code": code, "ok": false}
	for k, v := range fields {
		body[k] = v
	}
	_ = json.NewEncoder(diagnostic).Encode(body)
	return 2
}

// runsFiles appends a comma-separated list of report paths, resolving relative ones against root.
func runsFiles(root string, dst *[]string) func(string) error {
	return func(v string) error {
		for _, name := range strings.Split(v, ",") {
			if name == "" || len(*dst) >= playwrightruns.MaxRuns {
				return errRunsArgument
			}
			if root != "" && !filepath.IsAbs(name) {
				name = filepath.Join(root, name)
			}
			*dst = append(*dst, name)
		}
		return nil
	}
}

func runRuns(_ context.Context, root string, args []string, out, diagnostic io.Writer) int {
	if len(args) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	switch args[0] {
	case "stability":
		return runRunsStability(root, args[1:], out, diagnostic)
	case "compare":
		return runRunsCompare(root, args[1:], out, diagnostic)
	}
	return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
}

func runRunsStability(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs stability", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	files := []string{}
	minRuns, minSet := playwrightruns.DefaultMinRuns, false
	f.Func("runs", "Playwright JSON reports", runsFiles(root, &files))
	f.Func("min-runs", "pool size for DETERMINISTIC", func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || minSet || n < playwrightruns.MinMinRuns || n > playwrightruns.MaxRuns {
			return errRunsArgument
		}
		minRuns, minSet = n, true
		return nil
	})
	if f.Parse(args) != nil || f.NArg() != 0 || len(files) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	runs := []*playwrightruns.Run{}
	for i, name := range files {
		run, err := playwrightruns.ReadFile(name)
		if err != nil {
			return runsRefuse(diagnostic, "runs-report-unavailable", map[string]any{"reason": err.Error(), "run": i + 1})
		}
		runs = append(runs, run)
	}
	return runsEmit(playwrightruns.Stability(runs, minRuns), out, diagnostic)
}

func runRunsCompare(root string, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("runs compare", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	baseline, candidate := []string{}, []string{}
	f.Func("baseline", "baseline Playwright JSON reports", runsFiles(root, &baseline))
	f.Func("candidate", "candidate Playwright JSON reports", runsFiles(root, &candidate))
	if f.Parse(args) != nil || f.NArg() != 0 || len(baseline) == 0 || len(candidate) == 0 {
		return runsRefuse(diagnostic, "runs-invalid-arguments", nil)
	}
	load := func(names []string) []playwrightruns.Loaded {
		out := []playwrightruns.Loaded{}
		for _, name := range names {
			run, err := playwrightruns.ReadFile(name)
			out = append(out, playwrightruns.Loaded{Run: run, Err: err})
		}
		return out
	}
	return runsEmit(playwrightruns.Compare(load(baseline), load(candidate)), out, diagnostic)
}

func runsEmit(value any, out, diagnostic io.Writer) int {
	raw, err := json.Marshal(value)
	if err == nil {
		_, err = out.Write(append(raw, '\n'))
	}
	if err != nil {
		return runsRefuse(diagnostic, "runs-output-unavailable", nil)
	}
	return 0
}
