// Package flows holds the `corvint-docs flows` and `corvint-docs how` command handlers
// (application-flow intent, appmap, navigate, request-path and coverage); cmd/corvint-docs keeps
// the preamble, dispatch and help, and `corvint flows` / `corvint how` forward to it (decision 0595).
package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// Env carries the root-owned behaviour the flows handlers share with every other command: the
// JSON error envelope written to the diagnostic stream, and the affected-revision refusal for a
// --base that does not name a commit.
type Env struct {
	EmitError   func(diagnostic io.Writer, err error)
	BaseRefusal func(base string) error
}

// Run executes `corvint-docs flows ARGS` against root and returns the process exit code. The root
// preamble, the `how` alias and help stay with the caller.
func Run(ctx context.Context, root string, args []string, out, diagnostic io.Writer, env Env) int {
	return runFlows(ctx, root, args, out, diagnostic, env)
}

func argumentError(message string) error {
	return &gokernel.Error{Code: "invalid-arguments", Message: message}
}

// unresolvedBase marks a --base that is not a commit; runFlowSubcommand turns it into
// Env.BaseRefusal so the refusal itself stays defined once, by the root.
type unresolvedBase struct{ base string }

func (err *unresolvedBase) Error() string {
	return "base " + err.base + " is not a commit in this repository"
}

func runFlows(ctx context.Context, root string, args []string, out, diagnostic io.Writer, env Env) int {
	if len(args) > 0 && args[0] == "coverage" {
		return runFlowCoverage(ctx, root, args[1:], out, diagnostic)
	}
	if len(args) > 0 && flowSubcommands[args[0]] != nil {
		return runFlowSubcommand(ctx, flowSubcommands[args[0]], root, args[1:], out, diagnostic, env)
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
		env.EmitError(diagnostic, argumentError("invalid flows arguments"))
		return 2
	}
	if *manifest == "" || f.NArg() != 0 {
		env.EmitError(diagnostic, argumentError("flows requires --manifest FILE"))
		return 2
	}
	if !record && *output != "" {
		env.EmitError(diagnostic, argumentError("--output requires flows record"))
		return 2
	}
	if record && (*evidence == "" || *output == "") {
		env.EmitError(diagnostic, argumentError("flows record requires --evidence and --output"))
		return 2
	}
	root, err := filepath.Abs(root)
	if err != nil {
		env.EmitError(diagnostic, argumentError("invalid flow root"))
		return 2
	}
	in, err := appflows.Capture(ctx, root, *manifest)
	if err != nil {
		env.EmitError(diagnostic, argumentError(err.Error()))
		return 2
	}
	var e *appflows.Evidence
	var raw []byte
	if *evidence != "" {
		raw, err = appflows.ReadFile(*evidence)
		if err != nil {
			env.EmitError(diagnostic, argumentError(err.Error()))
			return 2
		}
		e = new(appflows.Evidence)
		if err = appflows.Decode(raw, e); err != nil {
			env.EmitError(diagnostic, argumentError(err.Error()))
			return 2
		}
	}
	r, err := appflows.ReportFor(in, e)
	if err != nil {
		env.EmitError(diagnostic, argumentError(err.Error()))
		return 2
	}
	if record {
		if err = appflows.Record(in, raw, *output); err != nil {
			env.EmitError(diagnostic, argumentError(err.Error()))
			return 2
		}
	}
	if json.NewEncoder(out).Encode(r) != nil {
		return 2
	}
	return 0
}

type flowSubcommand func(ctx context.Context, root string, args []string, out, diag io.Writer) error

var flowSubcommands = map[string]flowSubcommand{"export": runFlowsExport, "import": runFlowsImport, "map": runFlowsMap,
	"gaps": runFlowsGaps, "impact": runFlowsImpact, "ingest": runFlowsIngest, "docs": runFlowsDocs,
	"stability": runFlowsStability, "ingest-coverage": runFlowsIngestCoverage, "test-evidence": refuseFlowsTestEvidence}

// Subcommands names the registered `flows` subcommands for the help catalog; `coverage` and the
// base manifest/record form are not registered subcommands.
func Subcommands() []string {
	names := slices.Collect(maps.Keys(flowSubcommands))
	slices.Sort(names)
	return names
}

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

func runFlowSubcommand(ctx context.Context, run flowSubcommand, root string, args []string, out, diagnostic io.Writer, env Env) int {
	root, err := filepath.Abs(root)
	if err == nil {
		err = run(ctx, root, args, out, diagnostic)
	}
	var unresolved *unresolvedBase
	if errors.As(err, &unresolved) {
		err = env.BaseRefusal(unresolved.base)
	}
	if errors.Is(err, errAppmapEdgesUnproven) {
		return 1
	}
	var coded *gokernel.Error
	var gate0 *gateFailure
	if err != nil && !errors.As(err, &coded) && !errors.As(err, &gate0) {
		err = argumentError(err.Error())
	}
	var gate *gateFailure
	if errors.As(err, &gate) {
		fmt.Fprintln(diagnostic, gate.Error())
		return 1
	}
	if err != nil {
		env.EmitError(diagnostic, err)
		return 2
	}
	return 0
}

func runFlowsExport(ctx context.Context, root string, args []string, out, diag io.Writer) error {
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

func runFlowsImport(_ context.Context, root string, args []string, out, diag io.Writer) error {
	f := flag.NewFlagSet("flows import", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("flows", "", "intent directory inside the root")
	from := f.String("from", "", "source document")
	format := f.String("format", "", appflows.ImportFormats)
	if f.Parse(args) != nil || *dir == "" || *from == "" || *format == "" || f.NArg() != 0 {
		return errors.New("flows import requires --flows DIR --from FILE --format behavior-adapter-request|extractor-inventory|openapi|playwright-list")
	}
	raw, err := appflows.ReadFile(*from)
	if err != nil {
		return err
	}
	written, err := appflows.Import(root, *dir, raw, *format, *from)
	var skipped *appflows.ImportSkipped
	if err != nil && !errors.As(err, &skipped) {
		return err
	}
	if len(written) != 0 || skipped == nil {
		if _, err = fmt.Fprintln(out, strings.Join(written, "\n")); err != nil {
			return err
		}
	}
	// A partial playwright-list import exits 1 with one line per skipped test (AFU-V1-129).
	if skipped != nil {
		return &gateFailure{message: skipped.Error()}
	}
	return nil
}

// flowQueryFlags declares the --flows directory and the repeatable --evidence files map and gaps share.
func flowQueryFlags(name string) (*flag.FlagSet, *string, *[]string, *string) {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	dir := f.String("flows", "", "intent directory inside the root")
	evidence := &[]string{}
	f.Func("evidence", "test-run-evidence/0 JSONL file", func(s string) error {
		*evidence = append(*evidence, s)
		return nil
	})
	return f, dir, evidence, f.String("registry", "", "committed flows-run-registry/0 stability policy")
}

func runFlowsMap(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	f, dir, evidence, registry := flowQueryFlags("flows map")
	path := f.String("path", "", "reverse lookup from a source path")
	testKey := f.String("test-key", "", "reverse lookup from a test key")
	parseErr := f.Parse(args)
	lookup := *path != "" || *testKey != ""
	if parseErr != nil || *dir == "" || f.NArg() != 0 || (*path != "" && *testKey != "") || (lookup && (len(*evidence) != 0 || *registry != "")) {
		return errors.New("flows map requires --flows DIR, then --registry FILE and/or --evidence FILE (repeatable) or one of --path P or --test-key K")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	var data []byte
	if lookup {
		data, err = appflows.FlowLookup(ctx, root, set, *path, *testKey)
	} else {
		data, err = flowMap(ctx, root, set, *evidence, *registry)
	}
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func flowMap(ctx context.Context, root string, set appflows.IntentSet, evidence []string, registry string) ([]byte, error) {
	records, err := appflows.ReadRunEvidence(evidence)
	if err != nil {
		return nil, err
	}
	return appflows.FlowMap(ctx, root, set, records, registry)
}

func runFlowsGaps(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	f, dir, evidence, registry := flowQueryFlags("flows gaps")
	provider := f.String("provider", "", "repository-relative application-flow-selection-provider/1 file whose inventory is compared")
	if f.Parse(args) != nil || *dir == "" || f.NArg() != 0 {
		return errors.New("flows gaps requires --flows DIR and optional --registry FILE, --provider FILE and repeatable --evidence FILE")
	}
	set, err := appflows.LoadIntentsAt(ctx, root, *dir, "HEAD")
	if err != nil {
		return err
	}
	records, err := appflows.ReadRunEvidence(*evidence)
	if err != nil {
		return err
	}
	// AFU-V1-150: the E2E inventory is the provider's; without one the report says it is absent.
	var inventory *appflows.InventoryProvider
	if *provider != "" {
		if inventory, err = appflows.ReadInventoryProvider(root, *provider, *dir); err != nil {
			return err
		}
	}
	data, err := appflows.FlowGaps(ctx, root, set, records, *registry, inventory)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// runFlowsStability aggregates repeated runs through the run registry committed at HEAD (AFU-V1-042).
func runFlowsStability(ctx context.Context, root string, args []string, out, diag io.Writer) error {
	f := flag.NewFlagSet("flows stability", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	registry := f.String("registry", "", "repository-relative flows-run-registry/0 file read at HEAD")
	evidence := &[]string{}
	f.Func("evidence", "test-run-evidence/0 JSONL file", func(s string) error {
		*evidence = append(*evidence, s)
		return nil
	})
	if f.Parse(args) != nil || *registry == "" || f.NArg() != 0 {
		return errors.New("flows stability requires --registry FILE and optional repeatable --evidence FILE")
	}
	records, err := appflows.ReadRunEvidence(*evidence)
	if err != nil {
		return err
	}
	data, err := appflows.FlowStability(ctx, root, *registry, records)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func runFlowsImpact(ctx context.Context, root string, args []string, out, diag io.Writer) error {
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
		return &unresolvedBase{*base}
	}
	data, err := appflows.FlowImpactAt(ctx, root, set, resolved)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

func runFlowsIngest(_ context.Context, root string, args []string, out, diag io.Writer) error {
	f := flag.NewFlagSet("flows ingest", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	format := f.String("format", "", "playwright-json, playwright-receipt, junit-xml or go-test-json")
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
	f.Func("coverage-artifact", "ID=FILE coverage file the run wrote", func(s string) error {
		id, file, ok := strings.Cut(s, "=")
		if !ok || id == "" || file == "" {
			return errors.New("--coverage-artifact needs ID=FILE")
		}
		raw, err := appflows.ReadFile(file)
		if err != nil {
			return err
		}
		h.CoverageArtifacts = append(h.CoverageArtifacts, appflows.RunDigestRef{ID: id, Digest: appflows.Digest(raw)})
		return nil
	})
	if f.Parse(args) != nil || *format == "" || *from == "" || f.NArg() != 0 {
		return errors.New("flows ingest requires --format playwright-json|playwright-receipt|junit-xml|go-test-json --from FILE and the run header flags")
	}
	// AFU-V1-904 (V1-3937): every format needs the whole run header; one refusal names each
	// missing or invalid flag with the format it accepts, before the report is read.
	if problems := appflows.CheckRunHeader(h); len(problems) != 0 {
		named := make([]string, 0, len(problems))
		for _, p := range problems {
			state := "invalid"
			if p.Missing {
				state = "missing"
			}
			named = append(named, fmt.Sprintf("--%s %s (%s)", strings.ReplaceAll(p.Field, "_", "-"), state, p.Format))
		}
		return &gokernel.Error{Code: "invalid-arguments", Message: "flows ingest run header: " + strings.Join(named, "; ")}
	}
	h.Root = root
	ingested, err := appflows.IngestRunFile(*format, *from, h)
	if err != nil {
		return err
	}
	// An unjoined test is still recorded; the diagnostic names it and why its key cannot equal the
	// key a playwright-list import gives it (AFU-V1-131, V1-2873).
	for _, u := range ingested.Unjoined {
		fmt.Fprintf(diag, "flows ingest: %s: test %q does not join playwright-list import keys: %s\n", *from, u.TestKey, u.Reason)
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

// IntentHelp documents the AFU-V1 intent subcommands; help.go appends it to flowsHelp.
const IntentHelp = `
Flow variation coverage:
  corvint-docs [--root PATH] flows coverage --denominator FILE --receipts FILE [--revision SHA]
    [--offset N --limit N] [--write-back FRESH_DIR | --check DIR]
Complete accepted matrices and original /3 receipts; exit1 for gaps,2 for refusal.
PROVEN binds immutable bytes and declared app identity, not attested deployment lineage.

Intent usage:
  corvint-docs [--root PATH] flows export --flows DIR --emit inventory|provider|request [--envelope FILE]
  corvint-docs [--root PATH] flows import --flows DIR --from FILE --format behavior-adapter-request|extractor-inventory|openapi|playwright-list

export reads application-flow-intent/1 files from DIR (one <flow_id>.json each, plus
retired.json) and writes one document to stdout: the application-flow-inventory/1
inventory, the EEP-V1 provider record evaluated at HEAD, or the
corvint-behavior-adapter-request/1 built from --envelope, whose application-flows
input must anchor the committed inventory bytes. Review anchors are self-attested:
review identity is not verified. import writes new proposed intents with inferred
links, never overwrites an intent, and prints the written paths. A playwright-list
import (Playwright's JSON test list) supplies test links only: one proposed ui flow
per listed test, its file resolved against config.rootDir under the root and keyed
"path > describe > title"; steps, actors and outcomes are authored by hand. A listed test whose flow ID collides is reported and skipped, the rest are
written, and the command exits 1.

Query usage:
  corvint-docs [--root PATH] flows map --flows DIR [--evidence FILE]... [--registry FILE]
  corvint-docs [--root PATH] flows map --flows DIR [--path P | --test-key K]
  corvint-docs [--root PATH] flows gaps --flows DIR [--evidence FILE]... [--registry FILE] [--provider FILE]
  corvint-docs [--root PATH] flows status --flows DIR [--evidence FILE]... [--base SHA]
  corvint-docs [--root PATH] flows impact --flows DIR --base SHA
  corvint-docs [--root PATH] flows stability --registry FILE [--evidence FILE]...
  corvint-docs [--root PATH] flows ingest --format playwright-json|playwright-receipt|junit-xml|go-test-json --from FILE
    --run-id ID --runner-version VERSION --source-commit SHA --source-tree SHA [--source-clean]
    --build-artifact-digest DIGEST --environment-id ID --environment-digest DIGEST
    --fixture-id ID --fixture-digest DIGEST [--cleanup STATE] [--control SPEC]... [--coverage-artifact ID=FILE]...

map writes application-flow-map/1: every link with its basis, review state and the
self-attested review summary, and per variation the test-run-evidence/0 state and
authority at HEAD. --path or --test-key writes application-flow-lookup/1 instead.
gaps writes application-flow-gaps/1; any gap makes a flow incomplete. With --provider FILE
(a repository-relative application-flow-selection-provider/1 file that declares the same
--flows DIR) it also lists, as unlinked_tests with code unlinked-test, every test of the
provider's E2E inventory that no flow variation links with a non-inferred test link, at most
1024 in test-key order with the total, and says in inventory whether that inventory was
absent (no --provider), complete, partial, stale or unverified against its declared
discovery receipt; an absent inventory is never a complete answer (experimental, AFU-V1-150).
Example: corvint-docs flows gaps --flows flows --provider .corvint/e2e-provider.json
status writes application-flow-status/0: per flow and variation, the attached tests and
the last recorded test-run-evidence/0 result of each (passed, failed, flaky, timedOut,
skipped, interrupted or not-run) or NOT_OBSERVED, stale with its source_commit when the
newest record is bound to another tree; --base SHA limits the flows to exactly those
flows impact names and carries its impact_graph scope; at most 8192 test rows, then
truncated with full totals (experimental, AFU-V1-160).
Example: corvint-docs flows status --flows flows --evidence runs/runs.jsonl
impact writes application-flow-impact/1: flows, variations and test keys reached from
base..HEAD changes, with the path of each hop. stability writes flows-run-stability/0: per
aggregate of the flows-run-registry/0 FILE committed at HEAD, the raw repeated-run
counts, the applied threshold and a clean or not-stable verdict. ingest writes test-run-evidence/0 JSONL to
stdout. Every --format, playwright-json included, requires the run header: --run-id,
--environment-id and --fixture-id are IDs of 1-128 letters, digits, '.', '_' or '-';
--runner-version is printable text; --source-commit and --source-tree are full lowercase hex
object IDs; --build-artifact-digest, --environment-digest and --fixture-digest are bare
64-hex sha256 digests (no sha256: prefix); --cleanup is done, failed or not-declared (the
default). One refusal (exit 2, invalid-arguments) names every missing or invalid header flag
with its format. Optional: --source-clean, repeatable
--control "SUBJECT<TAB>CONTROL<TAB>EXPECTED" and repeatable --coverage-artifact
ID=FILE, which pins a coverage file the run wrote by sha256. A bound exceeded exits nonzero with
the incomplete code and writes nothing. None of these writes to the repository.

Coverage usage:
  corvint-docs [--root PATH] flows ingest-coverage --format v8|istanbul|rspec|lcov --from FILE
    --evidence FILE... --revision FULL_COMMIT [--test-key KEY] [--links FILE] [--strip-prefix P]

ingest-coverage writes corvint-coverage-ingest/1 to stdout: the coverage, run-evidence
and links inputs pinned by digest; each covered test bound to its one run record, and
joined only when that record ran clean at the revision and pins the coverage input's
sha256 as a --coverage-artifact; observed tested_by relations
from executed source files with executed line spans anchored to the committed blobs;
links the coverage contradicts as coverage-contradiction conflicts; and every path or
test it cannot classify as an unknown with a reason. Source maps are not resolved.

Documentation usage:
  corvint-docs [--root PATH] flows docs --flows DIR --page FILE --claims FILE [--docs-root DIR] [--waivers FILE] [--evidence FILE]...
  corvint-docs [--root PATH] flows docs --check --flows DIR --page FILE --claims FILE [--docs-root DIR] [--waivers FILE] [--evidence FILE]...

docs renders the page from the committed intents with a fixed template, marks every
claim that is not PROVEN, and writes the flow-doc-claims/0 sidecar; both paths are
repository-relative and each is replaced through a temporary file and a rename.
--docs-root checks corvint-claim anchor comments in committed Markdown under DIR.
--waivers reads the committed flow-doc-waivers/0 file in generation and --check
alike: a claim an unexpired waiver names that is not PROVEN renders on the page and
in the sidecar as "WAIVED until DATE (now STATE)"; an expired waiver never applies.
--check writes flow-doc-check/0 to stdout and writes nothing: it fails when a
committed PROVEN claim is no longer PROVEN (unless an unexpired waiver names it) or
the committed bytes differ from regeneration, and it fails on an unknown or invalid
anchor; expired_waivers names each expired waiver it found.

Experimental maintenance (FDM-V0, proposed):
  corvint-docs [--root PATH] flows docs maintain --flows DIR --page FILE --claims FILE
    --preview | --apply --expected-proposal SHA256 [--receipt FILE | --adopt]
preview writes nothing and prints a digest-bound proposal; apply rederives it, refuses
drift, publishes the pair without clobbering and prints a receipt to save and commit.
`
