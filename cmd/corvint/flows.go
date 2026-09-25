package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

func flowInvocation(args []string) (string, []string, bool) {
	root := "."
	for len(args) > 0 && strings.HasPrefix(args[0], "--root") {
		if strings.HasPrefix(args[0], "--root=") {
			root, args = strings.TrimPrefix(args[0], "--root="), args[1:]
			continue
		}
		if args[0] != "--root" || !rootPreambleValue(args, 1) {
			return "", nil, false
		}
		root, args = args[1], args[2:]
	}
	if len(args) == 0 || args[0] != "flows" {
		return "", nil, false
	}
	return root, args[1:], true
}

func runFlows(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	if len(args) > 0 && flowSubcommands[args[0]] != nil {
		return runFlowSubcommand(ctx, flowSubcommands[args[0]], root, args[1:], out, diagnostic)
	}
	record := false
	if len(args) > 0 && args[0] == "record" {
		record, args = true, args[1:]
	}
	f := flag.NewFlagSet("flows", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	manifest := f.String("manifest", "", "tracked relative manifest")
	evidence := f.String("evidence", "", "optional observation file")
	output := f.String("output", "", "explicit record destination")
	if err := f.Parse(args); err != nil {
		emitError(diagnostic, argumentError("invalid flows arguments"))
		return 2
	}
	if *manifest == "" || f.NArg() != 0 {
		emitError(diagnostic, argumentError("flows requires --manifest FILE"))
		return 2
	}
	if !record && *output != "" {
		emitError(diagnostic, argumentError("--output requires flows record"))
		return 2
	}
	if record && (*evidence == "" || *output == "") {
		emitError(diagnostic, argumentError("flows record requires --evidence and --output"))
		return 2
	}
	root, err := filepath.Abs(root)
	if err != nil {
		emitError(diagnostic, argumentError("invalid flow root"))
		return 2
	}
	in, err := appflows.Capture(ctx, root, *manifest)
	if err != nil {
		emitError(diagnostic, argumentError(err.Error()))
		return 2
	}
	var e *appflows.Evidence
	var raw []byte
	if *evidence != "" {
		raw, err = appflows.ReadFile(*evidence)
		if err != nil {
			emitError(diagnostic, argumentError(err.Error()))
			return 2
		}
		e = new(appflows.Evidence)
		if err = appflows.Decode(raw, e); err != nil {
			emitError(diagnostic, argumentError(err.Error()))
			return 2
		}
	}
	r, err := appflows.ReportFor(in, e)
	if err != nil {
		emitError(diagnostic, argumentError(err.Error()))
		return 2
	}
	if record {
		if err = appflows.Record(in, raw, *output); err != nil {
			emitError(diagnostic, argumentError(err.Error()))
			return 2
		}
	}
	if json.NewEncoder(out).Encode(r) != nil {
		return 2
	}
	return 0
}

type flowSubcommand func(ctx context.Context, root string, args []string, out io.Writer) error

var flowSubcommands = map[string]flowSubcommand{"export": runFlowsExport, "import": runFlowsImport, "map": runFlowsMap,
	"gaps": runFlowsGaps, "impact": runFlowsImpact, "ingest": runFlowsIngest, "docs": runFlowsDocs}

// flowExports maps each --emit value to the one document it writes to stdout.
var flowExports = map[string]func(ctx context.Context, root string, set appflows.IntentSet, envelope string) ([]byte, error){
	"inventory": func(_ context.Context, _ string, set appflows.IntentSet, _ string) ([]byte, error) {
		return appflows.CompileInventory(set)
	},
	"provider": func(ctx context.Context, root string, set appflows.IntentSet, _ string) ([]byte, error) {
		return appflows.ExportProvider(ctx, root, set)
	},
	"request": func(ctx context.Context, root string, set appflows.IntentSet, envelope string) ([]byte, error) {
		inventory, err := appflows.CompileInventory(set)
		if err != nil {
			return nil, err
		}
		raw, err := appflows.ReadFile(envelope)
		if err != nil {
			return nil, err
		}
		return appflows.ExportRequest(ctx, root, raw, inventory)
	},
}

func runFlowSubcommand(ctx context.Context, run flowSubcommand, root string, args []string, out, diagnostic io.Writer) int {
	root, err := filepath.Abs(root)
	if err == nil {
		err = run(ctx, root, args, out)
	}
	var coded *gokernel.Error
	if err != nil && !errors.As(err, &coded) {
		err = argumentError(err.Error())
	}
	if err != nil {
		emitError(diagnostic, err)
		return 2
	}
	return 0
}

func runFlowsExport(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows export", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("flows", "", "intent directory inside the root")
	emit := f.String("emit", "", "request, provider or inventory")
	envelope := f.String("envelope", "", "behavior-adapter request anchoring the committed inventory")
	parseErr := f.Parse(args)
	export, known := flowExports[*emit]
	if parseErr != nil || *dir == "" || f.NArg() != 0 || !known || (*emit == "request") != (*envelope != "") {
		return errors.New("flows export requires --flows DIR --emit request|provider|inventory, and --envelope FILE exactly when --emit request")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	data, err := export(ctx, root, set, *envelope)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func runFlowsImport(_ context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows import", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("flows", "", "intent directory inside the root")
	from := f.String("from", "", "source document")
	format := f.String("format", "", "behavior-adapter-request, openapi or playwright-list")
	if f.Parse(args) != nil || *dir == "" || *from == "" || *format == "" || f.NArg() != 0 {
		return errors.New("flows import requires --flows DIR --from FILE --format behavior-adapter-request|openapi|playwright-list")
	}
	raw, err := appflows.ReadFile(*from)
	if err != nil {
		return err
	}
	written, err := appflows.Import(root, *dir, raw, *format, *from)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, strings.Join(written, "\n"))
	return err
}

// flowQueryFlags declares the --flows directory and the repeatable --evidence files map and gaps share.
func flowQueryFlags(name string) (*flag.FlagSet, *string, *[]string) {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("flows", "", "intent directory inside the root")
	evidence := &[]string{}
	f.Func("evidence", "test-run-evidence/0 JSONL file", func(s string) error {
		*evidence = append(*evidence, s)
		return nil
	})
	return f, dir, evidence
}

func runFlowsMap(ctx context.Context, root string, args []string, out io.Writer) error {
	f, dir, evidence := flowQueryFlags("flows map")
	path := f.String("path", "", "reverse lookup from a source path")
	testKey := f.String("test-key", "", "reverse lookup from a test key")
	parseErr := f.Parse(args)
	lookup := *path != "" || *testKey != ""
	if parseErr != nil || *dir == "" || f.NArg() != 0 || (*path != "" && *testKey != "") || (lookup && len(*evidence) != 0) {
		return errors.New("flows map requires --flows DIR, then --evidence FILE (repeatable) or one of --path P or --test-key K")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	var data []byte
	if lookup {
		data, err = appflows.FlowLookup(ctx, root, set, *path, *testKey)
	} else {
		data, err = flowMap(ctx, root, set, *evidence)
	}
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func flowMap(ctx context.Context, root string, set appflows.IntentSet, evidence []string) ([]byte, error) {
	records, err := appflows.ReadRunEvidence(evidence)
	if err != nil {
		return nil, err
	}
	return appflows.FlowMap(ctx, root, set, records)
}

func runFlowsGaps(ctx context.Context, root string, args []string, out io.Writer) error {
	f, dir, evidence := flowQueryFlags("flows gaps")
	if f.Parse(args) != nil || *dir == "" || f.NArg() != 0 {
		return errors.New("flows gaps requires --flows DIR and optional repeatable --evidence FILE")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	records, err := appflows.ReadRunEvidence(*evidence)
	if err != nil {
		return err
	}
	data, err := appflows.FlowGaps(ctx, root, set, records)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// flowImpactDirtyWorktree is the graph unknown reason of `flows impact` on a worktree that differs
// from HEAD.
const flowImpactDirtyWorktree = "DIRTY_WORKTREE"

func runFlowsImpact(ctx context.Context, root string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows impact", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("flows", "", "intent directory inside the root")
	base := f.String("base", "", "base commit of the changed range")
	if f.Parse(args) != nil || *dir == "" || *base == "" || f.NArg() != 0 {
		return errors.New("flows impact requires --flows DIR --base SHA")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	resolved, err := appflows.ResolveRevision(ctx, root, *base)
	if err != nil {
		return affectedBaseRefusal(*base)
	}
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return errors.New("git executable unavailable")
	}
	changed, err := affectedRangePaths(ctx, gitExecutable, root, resolved)
	if err != nil {
		return err
	}
	dirty, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	graph, err := affected.Build(root, affectedLanguages()...)
	if err != nil {
		return err
	}
	recheck, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return &gokernel.Error{Code: "unsupported-affected-status", Message: err.Error()}
	}
	plan := affected.Select(graph, changed)
	// The graph is walked from the working tree while intents and changed paths come from HEAD; a
	// worktree that differs from HEAD may have built a different graph, so the hit list may be short.
	if differs := slices.Compact(slices.Sorted(slices.Values(append(dirty, recheck...)))); len(differs) != 0 {
		plan.Scope = affected.ScopeUnknown
		plan.Unknown = append(plan.Unknown, affected.Unknown{Reason: flowImpactDirtyWorktree,
			Detail: fmt.Sprintf("the impact graph was built from a worktree that differs from HEAD at %d paths, first %s", len(differs), differs[0])})
	}
	data, err := appflows.FlowImpact(ctx, root, set, resolved, graph, plan)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func runFlowsIngest(_ context.Context, _ string, args []string, out io.Writer) error {
	f := flag.NewFlagSet("flows ingest", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	format := f.String("format", "", "playwright-json, junit-xml or go-test-json")
	from := f.String("from", "", "test report")
	h := appflows.RunHeader{}
	f.StringVar(&h.RunID, "run-id", "", "run ID")
	f.StringVar(&h.RunnerVersion, "runner-version", "", "runner version")
	f.StringVar(&h.Source.Commit, "source-commit", "", "commit the run tested")
	f.StringVar(&h.Source.Tree, "source-tree", "", "tree the run tested")
	f.BoolVar(&h.Source.Clean, "source-clean", false, "the run's worktree was clean")
	f.StringVar(&h.BuildArtifactDigest, "build-artifact-digest", "", "build artifact digest")
	f.StringVar(&h.Environment.ID, "environment-id", "", "environment ID")
	f.StringVar(&h.Environment.Digest, "environment-digest", "", "environment digest")
	f.StringVar(&h.Fixture.ID, "fixture-id", "", "fixture ID")
	f.StringVar(&h.Fixture.Digest, "fixture-digest", "", "fixture digest")
	f.StringVar(&h.Cleanup, "cleanup", "not-declared", "done, failed or not-declared")
	f.Func("control", "SUBJECT<TAB>CONTROL<TAB>EXPECTED", func(s string) error {
		parts := strings.Split(s, "\t")
		if len(parts) != 3 {
			return errors.New("--control needs three tab-separated fields")
		}
		h.Controls = append(h.Controls, appflows.RunControl{Subject: parts[0], TestKey: parts[1], Expected: parts[2]})
		return nil
	})
	if f.Parse(args) != nil || *format == "" || *from == "" || f.NArg() != 0 {
		return errors.New("flows ingest requires --format playwright-json|junit-xml|go-test-json --from FILE and the run header flags")
	}
	ingested, err := appflows.IngestRunFile(*format, *from, h)
	if err != nil {
		return err
	}
	if ingested.Incomplete != "" {
		return &gokernel.Error{Code: ingested.Incomplete, Message: "run evidence is incomplete: " + ingested.Incomplete + " exceeded; no record written"}
	}
	var lines bytes.Buffer
	for _, r := range ingested.Records {
		line, err := appflows.EncodeRunEvidence(r)
		if err != nil {
			return err
		}
		lines.Write(line)
	}
	_, err = out.Write(lines.Bytes())
	return err
}

// flowsIntentHelp documents the AFU-V1 intent subcommands; help.go appends it to flowsHelp.
const flowsIntentHelp = `
Intent usage:
  corvint [--root PATH] flows export --flows DIR --emit inventory|provider|request [--envelope FILE]
  corvint [--root PATH] flows import --flows DIR --from FILE --format behavior-adapter-request|openapi|playwright-list

export reads application-flow-intent/1 files from DIR (one <flow_id>.json each, plus
retired.json) and writes one document to stdout: the application-flow-inventory/1
inventory, the EEP-V1 provider record evaluated at HEAD, or the
corvint-behavior-adapter-request/1 built from --envelope, whose application-flows
input must anchor the committed inventory bytes. Review anchors are self-attested:
review identity is not verified. import writes new proposed intents with inferred
links, never overwrites an intent, and prints the written paths.

Query usage:
  corvint [--root PATH] flows map --flows DIR [--evidence FILE]... [--path P | --test-key K]
  corvint [--root PATH] flows gaps --flows DIR [--evidence FILE]...
  corvint [--root PATH] flows impact --flows DIR --base SHA
  corvint [--root PATH] flows ingest --format playwright-json|junit-xml|go-test-json --from FILE [header flags]

map writes application-flow-map/1: every link with its basis, review state and the
self-attested review summary, and per variation the test-run-evidence/0 state and
authority at HEAD. --path or --test-key writes application-flow-lookup/1 instead.
gaps writes application-flow-gaps/1; any gap makes a flow incomplete. impact writes
application-flow-impact/1: flows, variations and test keys reached from base..HEAD
changes, with the path of each hop. ingest writes test-run-evidence/0 JSONL to
stdout; header flags are --run-id, --runner-version, --source-commit, --source-tree,
--source-clean, --build-artifact-digest, --environment-id, --environment-digest,
--fixture-id, --fixture-digest, --cleanup and repeatable
--control "SUBJECT<TAB>CONTROL<TAB>EXPECTED". A bound exceeded exits nonzero with
the incomplete code and writes nothing. None of these writes to the repository.

Documentation usage:
  corvint [--root PATH] flows docs --flows DIR --page FILE --claims FILE [--docs-root DIR] [--evidence FILE]...
  corvint [--root PATH] flows docs --check --flows DIR --page FILE --claims FILE [--docs-root DIR] [--waivers FILE] [--evidence FILE]...

docs renders the page from the committed intents with a fixed template, marks every
claim that is not PROVEN, and writes the flow-doc-claims/0 sidecar; both paths are
repository-relative and each is replaced through a temporary file and a rename.
--docs-root checks corvint-claim anchor comments in committed Markdown under DIR.
--check writes flow-doc-check/0 to stdout and writes nothing: it fails when a
committed PROVEN claim is no longer PROVEN or the committed bytes differ from
regeneration, unless an unexpired entry of the committed flow-doc-waivers/0 file
names the claim, and it fails on an unknown or invalid anchor.
`
