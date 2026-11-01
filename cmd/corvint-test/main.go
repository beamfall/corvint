// Command corvint-test is the experimental test companion. Its `e2e` group compiles change-scoped
// E2E scenarios from admitted, revision-pinned blocks (CCE-V0). Core `corvint` does not import or
// execute it, and it never executes `corvint` (CCE-V0-015). Diagnostics carry the
// `corvint-test e2e:` prefix, disjoint from core `corvint test` diagnostics (`corvint-test:`).
// The `runner`, `falsify`, `minimize` and `accept` groups are folded companions (CTF-V0); their
// diagnostics name `corvint-test <group>`.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Beamfall/corvint/internal/appmap"
	falsifycli "github.com/Beamfall/corvint/internal/behaviorfalsify/cli"
	"github.com/Beamfall/corvint/internal/capcatalog"
	"github.com/Beamfall/corvint/internal/changee2e"
	minimizecli "github.com/Beamfall/corvint/internal/playwrightminimize/cli"
	acceptcli "github.com/Beamfall/corvint/internal/testacceptance/cli"
	"github.com/Beamfall/corvint/internal/testrunner/cli"
)

// proveReport is the proof report written under the prove --out directory.
const proveReport = "e2e-prove.json"

// proveCoverage is the e2e-runtime-coverage/0 document `prove --record-coverage` writes beside the
// proof (CCE-V0-053).
const proveCoverage = "e2e-runtime-coverage.json"

const usage = "usage: corvint-test e2e <catalog|harvest|obligations|compile|prove|shadow> [flags]\n" +
	"       (catalog|harvest|obligations|compile|prove) [--repo ALIAS=ABSOLUTE_ROOT]... [--manifest-repo ALIAS] [--repo-revision ALIAS=REV]...; compile and prove read the test tree from the pinned tests root (CCE-V0-130..136)\n" +
	"       (catalog|harvest|obligations|compile|prove) [--workspace FILE | --no-workspace]: the aliased roots from a corvint-workspace/0 file, or from --root/.corvint/workspace.json when no root flag is given (WSD-V0)\n" +
	"       (obligations|compile|prove) [--endpoint-source [ALIAS:]DIR]... [--rails-routes [ALIAS:]FILE]... [--openapi [ALIAS:]FILE]...: join declared server endpoints to client calls so a changed controller or endpoint declaration reaches the calling screens (CCE-V0-152); ALIAS: reads a --repo root at the map's pin (AMAP-V0-222)\n" +
	"       corvint-test e2e how-accept --answer FILE [--write PATH] [--evidence FILE] [--oracle FILE] (SKH-V0 stage 8)\n" +
	"       corvint-test e2e how-evidence --answer FILE --prove FILE --validity FILE [--root DIR] (SKH-V0 stage 8)\n" +
	"       corvint-test e2e discriminate --base REV --head REV --app-dir DIR --test-command JSON [flags] (external-suite mutation discrimination, CCE-V0-170..176)\n" +
	"       corvint-test runner <command> [flags] (folded corvint-test-runner, CTF-V0)\n" +
	"       corvint-test falsify <plan|execute|execute-receipt|verify-receipt> --experimental [flags] < input.json (folded corvint-behavior-falsify, CTF-V0)\n" +
	"       corvint-test minimize <plan|execute> --experimental [flags] < input.json (folded corvint-playwright-minimize, CTF-V0)\n" +
	"       corvint-test accept plan|accept --plan FILE [flags] (folded corvint-tests-accept, CTF-V0)\n" +
	"       accept and falsify against a caller-owned authenticated-disposable app: docs/EXTERNAL-APP-ACCEPTANCE.md\n" +
	"       corvint-test help [--json | --tree] | corvint-test COMMAND --help (each command's usage, flags, required inputs and example)"

// acceptWorker selects the accept group's worker when the executable re-enters itself.
var acceptWorker = []string{"accept", "worker"}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	// The BBF control hook re-enters this executable with no arguments (CCE-V0-009).
	if role := os.Getenv("CORVINT_BEHAVIOR_COMMAND_ROLE"); role != "" && len(os.Args) == 1 {
		if err := changee2e.RunHook(ctx, role, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "corvint-test e2e hook:", err)
			os.Exit(1)
		}
		return
	}
	// Every node answers --help and -h with its own help before any group parses it (CAP-V0-042).
	if handled, err := helpRequest(os.Args[1:], os.Stdout); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, "corvint-test help:", err)
			os.Exit(2)
		}
		return
	}
	// The runner group shares the old companion's entry, so its output is byte-identical
	// apart from the invocation name in diagnostics (CTF-V0-002, CTF-V0-004).
	if len(os.Args) > 1 && os.Args[1] == "runner" {
		os.Exit(cli.Command(ctx, "corvint-test runner", os.Args[2:], os.Stdout, os.Stderr))
	}
	// The accept group exits 2 like the old companion, and its worker re-enters this
	// executable as `corvint-test accept worker` (CTF-V0-006).
	if len(os.Args) > 1 && os.Args[1] == "accept" {
		if err := acceptcli.Run(ctx, "corvint-test accept", acceptWorker, os.Args[2:], os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	folded := map[string]func(context.Context, string, []string, io.Reader, io.Writer) error{
		"falsify":  falsifycli.Run,
		"minimize": minimizecli.Run,
	}
	if len(os.Args) > 1 && folded[os.Args[1]] != nil {
		if err := folded[os.Args[1]](ctx, "corvint-test "+os.Args[1], os.Args[2:], os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "corvint-test e2e:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if (len(args) == 1 && helpWord(args[0])) || (len(args) == 2 && args[0] == "e2e" && helpWord(args[1])) {
		return writeGroupHelp(stdout)
	}
	if len(args) < 2 || args[0] != "e2e" {
		return errors.New(usage)
	}
	sub := args[1]
	if sub == "how-accept" {
		return howAccept(args[2:], stdout)
	}
	if sub == "how-evidence" {
		return howEvidence(ctx, args[2:], stdout)
	}
	if sub == "discriminate" {
		return discriminate(ctx, args[2:], stdout)
	}
	fs := flag.NewFlagSet("corvint-test e2e "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository root")
	manifest := fs.String("manifest", "appmap.json", "repository-relative application-map manifest")
	out := fs.String("out", "", "output file (catalog, harvest, obligations) or directory (compile, prove), outside the repository")
	var rev, base, head, coverage, ledger *string
	var repeats *int
	var recordCoverage, moduleGraph, remedySummary *bool
	var remedyCommits *int
	var endpoints *appmap.EndpointsOptions
	var sources []string
	var nodeModules, nodeBin, packageJSON, lockfile, window, hostedTest, hostedTitle, oracleAcceptance *string
	// Every map-reading verb takes the aliased-root flags of `flows appmap build` (CCE-V0-130).
	roots := &appmap.Roots{}
	if sub != "shadow" {
		roots = appmap.BindRootFlags(fs)
	}
	switch sub {
	case "shadow":
		window = fs.String("window", "", "frozen shadow window file (CCE-V0-011)")
	case "catalog", "harvest":
		rev = fs.String("rev", "", "revision")
		if sub == "harvest" {
			fs.Func("source", "[ALIAS:]DIR site-knowledge source directory whose IR binds proposals, repeatable (CCE-V0-310)",
				func(v string) error { sources = append(sources, v); return nil })
		}
	case "obligations", "compile", "prove":
		base = fs.String("base", "", "base revision")
		head = fs.String("head", "", "head revision")
		coverage = fs.String("coverage", "", "runtime coverage document recorded at head; it may only widen reach (CCE-V0-045)")
		ledger = fs.String("ledger", "", "application-edge-ledger/0 document built from the head map at head, bound to the impact relation (CCE-V0-103)")
		moduleGraph = fs.Bool("module-graph", false, "widen reach through the head tree's TypeScript/JavaScript module graph (CCE-V0-054)")
		endpoints = &appmap.EndpointsOptions{}
		repeat := func(dst *[]string) func(string) error {
			return func(v string) error { *dst = append(*dst, v); return nil }
		}
		fs.Func("endpoint-source", "[ALIAS:]DIR client source directory the endpoint join reads, repeatable (CCE-V0-152, AMAP-V0-222)", repeat(&endpoints.Sources))
		fs.Func("rails-routes", "rails-routes/0 file the endpoint join reads, repeatable (CCE-V0-152)", repeat(&endpoints.RailsRoutes))
		fs.Func("openapi", "OpenAPI 2/3 document (JSON or block YAML) the endpoint join reads, repeatable (CCE-V0-152)", repeat(&endpoints.OpenAPI))
		if sub == "obligations" {
			remedySummary = fs.Bool("remedy-summary", false, "rank the remedies that would bound the most first-parent merges of base..head, each against its parent (CCE-V0-151)")
			remedyCommits = fs.Int("remedy-commits", 20, "the newest first-parent commits --remedy-summary evaluates, 1 to 50")
		}
		if sub == "prove" {
			repeats = fs.Int("repeats", changee2e.DefaultRepeats, "repeats per side")
			nodeModules = fs.String("node-modules", os.Getenv("CORVINT_PLAYWRIGHT_MODULES"), "node_modules holding @playwright/test")
			nodeBin = fs.String("node-bin", os.Getenv("CORVINT_E2E_NODE_BIN"), "absolute path of the node binary, with npx beside it")
			packageJSON = fs.String("package-json", "", "package.json binding the runner identity")
			lockfile = fs.String("lockfile", "", "lockfile binding the runner identity")
			hostedTest = fs.String("hosted-test", "", "prove this existing hosted Playwright test file as given, without compiling (CCE-V0-034)")
			hostedTitle = fs.String("hosted-title", "", "the hosted test's title; optional when the file declares one test")
			recordCoverage = fs.Bool("record-coverage", false, "record the application paths each compiled scenario's head pool served and write "+proveCoverage+" under --out for --coverage (CCE-V0-053)")
			oracleAcceptance = fs.String("oracle-acceptance", "", "e2e-oracle-acceptance/0 document, outside the repository, recording an independent acceptance of edited outcomes (CCE-V0-051)")
		}
	default:
		return errors.New(usage)
	}
	if err := fs.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// The subcommand's own help, exit 0 (CCE-V0-118).
			return writeStepHelp(stdout, sub, fs)
		}
		return fmt.Errorf("%v (%s)", err, usage)
	}
	if fs.NArg() != 0 {
		return errors.New(usage)
	}
	if err := roots.Check(); err != nil {
		return fmt.Errorf("%v (%s)", err, usage)
	}
	if sub != "shadow" {
		// --workspace FILE, or the discovered .corvint/workspace.json, binds the aliased roots (WSD-V0-004).
		if err := roots.Resolve(*root); err != nil {
			return err
		}
	}
	outPath := ""
	if *out != "" || sub == "compile" || sub == "prove" {
		p, err := changee2e.CheckOut(*root, *out)
		if err != nil {
			return err
		}
		outPath = p
		for _, dir := range roots.Repos {
			// Scenarios and proofs are written outside every tree they are read from (CCE-V0-136).
			if _, err := changee2e.CheckOut(dir, *out); err != nil {
				return err
			}
		}
	}
	if sub == "prove" {
		// A bad destination must not cost a whole run (CCE-V0-137).
		if err := checkOutDir(outPath); err != nil {
			return err
		}
	}
	diff := func() changee2e.DiffOptions {
		d := changee2e.DiffOptions{Root: *root, Base: *base, Head: *head, Manifest: *manifest, Coverage: *coverage, ModuleGraph: *moduleGraph, Ledger: *ledger, Roots: *roots}
		if endpoints != nil && len(endpoints.Sources)+len(endpoints.RailsRoutes)+len(endpoints.OpenAPI) != 0 {
			d.Endpoints = endpoints
		}
		return d
	}
	var doc []byte
	switch sub {
	case "shadow":
		data, err := os.ReadFile(*window)
		if err != nil {
			return &changee2e.Refusal{Code: "e2e-shadow-window-invalid", Detail: err.Error()}
		}
		if doc, err = changee2e.Shadow(data); err != nil {
			return err
		}
	case "catalog":
		c, err := changee2e.BuildCatalog(ctx, changee2e.CatalogOptions{Root: *root, Rev: *rev, Manifest: *manifest, Roots: *roots})
		if err != nil {
			return err
		}
		doc = c.Encoded()
	case "harvest":
		p, err := changee2e.Harvest(ctx, changee2e.HarvestOptions{Root: *root, Rev: *rev, Manifest: *manifest, Roots: *roots, Sources: sources})
		if err != nil {
			return err
		}
		doc = p.Encoded()
	case "obligations":
		if *remedySummary {
			s, err := changee2e.BuildRemedySummary(ctx, diff(), *remedyCommits)
			if err != nil {
				return err
			}
			doc = s
			break
		}
		o, err := changee2e.BuildObligations(ctx, diff())
		if err != nil {
			return err
		}
		doc = o.Encoded()
	case "compile":
		r, err := changee2e.Compile(ctx, diff())
		if err != nil {
			return err
		}
		if err := writeScenarios(outPath, r.Compiled()); err != nil {
			return err
		}
		_, err = stdout.Write(r.Encoded())
		return err
	case "prove":
		if *hostedTest != "" || *hostedTitle != "" {
			if roots.Aliased() {
				// A hosted test is read from --root at head (CCE-V0-136).
				// It names every alias it would ignore, from flags or a workspace (WSD-V0-005).
				return &changee2e.Refusal{Code: "e2e-roots-unsupported", Detail: "a hosted test is read from --root and ignores alias " + strings.Join(roots.Aliases(), ", ") +
					"; aliased tests roots (--repo or a workspace) prove compiled scenarios only (pass --no-workspace to prove a hosted test without a discovered workspace)"}
			}
			if *hostedTest == "" {
				return &changee2e.Refusal{Code: "e2e-hosted-test-invalid", Detail: "--hosted-title needs --hosted-test"}
			}
			if *coverage != "" || *ledger != "" {
				return &changee2e.Refusal{Code: "e2e-hosted-test-invalid", Detail: "--coverage and --ledger compile reach; a hosted test is proved as given"}
			}
			doc, err := changee2e.Prove(ctx, changee2e.ProveOptions{Root: *root, Base: *base, Head: *head, Repeats: *repeats,
				NodeModules: *nodeModules, NodeBin: *nodeBin, PackageJSON: *packageJSON, Lockfile: *lockfile,
				Hosted: &changee2e.HostedSelection{File: *hostedTest, Title: *hostedTitle}, OracleAcceptance: *oracleAcceptance,
				RecordCoverage: *recordCoverage})
			if err != nil {
				return err
			}
			return writeProof(outPath, doc, *recordCoverage, stdout)
		}
		r, err := changee2e.Compile(ctx, diff())
		if err != nil {
			return err
		}
		if err := writeScenarios(outPath, r.Compiled(), proveReport, proveCoverage); err != nil {
			return err
		}
		doc, err = changee2e.Prove(ctx, changee2e.ProveOptions{Root: *root, Base: r.Base, Head: r.Head, Repeats: *repeats,
			NodeModules: *nodeModules, NodeBin: *nodeBin, PackageJSON: *packageJSON, Lockfile: *lockfile, Scenarios: r.Compiled(),
			OracleAcceptance: *oracleAcceptance, RecordCoverage: *recordCoverage, Tests: r.TestsRoot()})
		if err != nil {
			return err
		}
		return writeProof(outPath, doc, *recordCoverage, stdout)
	}
	if outPath != "" {
		return writeFile(outPath, doc, nil)
	}
	_, err := stdout.Write(doc)
	return err
}

// e2eWorkflow names the corvint help --json workflow whose steps the e2e subcommands are.
const e2eWorkflow = "change-compiled-e2e"

func helpWord(word string) bool { return word == "--help" || word == "-h" }

// adoptionWorkflow reads the e2e chain from the committed capability catalog, the one source
// `corvint help --json` and `corvint flows appmap build --help` also print (CCE-V0-118).
func adoptionWorkflow() (capcatalog.Workflow, error) {
	var catalog capcatalog.Catalog
	if err := json.Unmarshal(capcatalog.Snapshot(), &catalog); err != nil {
		return capcatalog.Workflow{}, err
	}
	for _, workflow := range catalog.Workflows {
		if workflow.Name == e2eWorkflow {
			return workflow, nil
		}
	}
	return capcatalog.Workflow{}, errors.New("the capability catalog has no " + e2eWorkflow + " workflow")
}

// writeGroupHelp prints the group usage, the chain and the adoption guide.
func writeGroupHelp(stdout io.Writer) error {
	workflow, err := adoptionWorkflow()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n\nChange-compiled E2E chain (corvint-test e2e SUBCOMMAND --help prints each step):\n  %s\nGuide: %s\nEvery command of corvint-test: %s\n%s",
		usage, strings.Join(workflow.Chain, " -> "), workflow.Guide, commandTree().HelpJSON, treeTrailer(true, "e2e"))
	return err
}

// writeStepHelp prints one subcommand's step help with the options its flag set declares.
func writeStepHelp(stdout io.Writer, sub string, fs *flag.FlagSet) error {
	workflow, err := adoptionWorkflow()
	if err != nil {
		return err
	}
	text := capcatalog.StepHelp(workflow, "e2e-"+sub, flagOptions(fs))
	if text == "" {
		return errors.New("the " + e2eWorkflow + " workflow has no e2e-" + sub + " step")
	}
	_, err = io.WriteString(stdout, text+treeTrailer(false, "e2e", sub))
	return err
}

// writeCommandHelp prints the help of a subcommand outside the adoption chain (how-accept,
// how-evidence): its text, then the options its flag set declares (CCE-V0-118).
func writeCommandHelp(stdout io.Writer, sub, help string, fs *flag.FlagSet) error {
	_, err := io.WriteString(stdout, strings.TrimRight(help, "\n")+"\n\nOptions:\n"+flagOptions(fs)+"\n"+treeTrailer(false, "e2e", sub))
	return err
}

// treeTrailer gives the e2e group and step help the required inputs of their command-tree node
// and, for the group, which has no authored example, the node's example (CAP-V0-044).
func treeTrailer(example bool, path ...string) string {
	node, ok := commandTree().Find(path)
	if !ok || len(node.Path) != len(path) {
		return ""
	}
	required := node.Required
	if len(required) == 0 {
		required = []string{"none"}
	}
	trailer := "\nRequired: " + strings.Join(required, ", ") + "\n"
	if example {
		trailer += "\nExample:\n  " + node.Example + "\n"
	}
	return trailer
}

// flagOptions renders a flag set's defaults with the double-dash spelling the usage lines use.
func flagOptions(fs *flag.FlagSet) string {
	var options strings.Builder
	fs.SetOutput(&options)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
	lines := strings.Split(strings.TrimRight(options.String(), "\n"), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "  -") {
			lines[i] = "  --" + line[3:]
		}
	}
	return strings.Join(lines, "\n")
}

// writeProof writes the proof under dir and, when coverage was recorded, the coverage document
// derived from it (CCE-V0-053).
func writeProof(dir string, doc []byte, recorded bool, stdout io.Writer) error {
	if recorded {
		cov, err := changee2e.CoverageFromProof(doc)
		if err != nil {
			return err
		}
		if err := writeUnder(dir, proveCoverage, cov, nil); err != nil {
			return err
		}
	}
	return writeUnder(dir, proveReport, doc, stdout)
}

// writeScenarios writes each compiled scenario under dir at its placement (CCE-V0-007). Every
// placement, and each name in reserved, is validated before the first write (CCE-V0-001).
func writeScenarios(dir string, scenarios []changee2e.Scenario, reserved ...string) error {
	seen := map[string]bool{}
	rels := append([]string(nil), reserved...)
	for _, s := range scenarios {
		rels = append(rels, s.Placement)
	}
	for _, rel := range rels {
		clean := filepath.Clean(filepath.FromSlash(rel))
		if seen[clean] {
			return unsafeOut("duplicate placement %s", rel)
		}
		seen[clean] = true
		if _, err := checkUnder(dir, rel); err != nil {
			return err
		}
	}
	for _, s := range scenarios {
		if err := writeUnder(dir, s.Placement, s.Text, nil); err != nil {
			return err
		}
	}
	return nil
}

// checkOutDir refuses a prove destination that is absent, not a directory or not
// writable, before any run starts (CCE-V0-137). The probe file is removed at once.
func checkOutDir(dir string) error {
	fi, err := os.Lstat(dir)
	switch {
	case err != nil:
		return unsafeOut("--out %s is unavailable: %v", dir, err)
	case !fi.IsDir():
		return unsafeOut("--out %s is not a directory", dir)
	}
	f, err := os.CreateTemp(dir, ".corvint-out-probe-*")
	if err != nil {
		return unsafeOut("--out %s is not writable: %v", dir, err)
	}
	name := f.Name()
	f.Close()
	return os.Remove(name)
}

func unsafeOut(format string, args ...any) error {
	return &changee2e.Refusal{Code: "e2e-out-unsafe", Detail: fmt.Sprintf(format, args...)}
}

// checkUnder returns the components of rel below dir after checking that rel stays lexically under
// dir and that no existing component below dir is a symlink or (except the last) a non-directory.
func checkUnder(dir, rel string) ([]string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if rel == "" || filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, unsafeOut("%s is not under the output directory", rel)
	}
	parts := strings.Split(clean, string(filepath.Separator))
	p := dir
	for i, part := range parts {
		p = filepath.Join(p, part)
		fi, err := os.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			break
		}
		if err != nil {
			return nil, err
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			return nil, unsafeOut("%s is a symlink", p)
		}
		if i < len(parts)-1 && !fi.IsDir() {
			return nil, unsafeOut("%s is not a directory", p)
		}
	}
	return parts, nil
}

// writeUnder writes data to rel under dir, creating directories one component at a time and never
// following a symlink below dir.
func writeUnder(dir, rel string, data []byte, echo io.Writer) error {
	parts, err := checkUnder(dir, rel)
	if err != nil {
		return err
	}
	p := dir
	for _, part := range parts[:len(parts)-1] {
		p = filepath.Join(p, part)
		if err := os.Mkdir(p, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		if fi, err := os.Lstat(p); err != nil {
			return err
		} else if !fi.IsDir() {
			return unsafeOut("%s is not a directory", p)
		}
	}
	return writeFile(filepath.Join(p, parts[len(parts)-1]), data, echo)
}

// writeFile writes path without following a final-component symlink.
func writeFile(path string, data []byte, echo io.Writer) error {
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		return unsafeOut("%s is a symlink", path)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|openNoFollow, 0o644)
	if err != nil {
		if fi, lerr := os.Lstat(path); lerr == nil && fi.Mode()&fs.ModeSymlink != 0 {
			return unsafeOut("%s is a symlink", path)
		}
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if echo != nil {
		_, err := echo.Write(data)
		return err
	}
	return nil
}
