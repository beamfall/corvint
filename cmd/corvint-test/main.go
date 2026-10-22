// Command corvint-test is the experimental test companion. Its `e2e` group compiles change-scoped
// E2E scenarios from admitted, revision-pinned blocks (CCE-V0). Core `corvint` does not import or
// execute it, and it never executes `corvint` (CCE-V0-015). Diagnostics carry the
// `corvint-test e2e:` prefix, disjoint from core `corvint test` diagnostics (`corvint-test:`).
// The `runner` group is the folded runner companion (CTF-V0); its diagnostics name
// `corvint-test runner`.
package main

import (
	"context"
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

	"github.com/Beamfall/corvint/internal/changee2e"
	"github.com/Beamfall/corvint/internal/testrunner/cli"
)

// proveReport is the proof report written under the prove --out directory.
const proveReport = "e2e-prove.json"

const usage = "usage: corvint-test e2e <catalog|harvest|obligations|compile|prove|shadow> [flags]\n" +
	"       corvint-test runner <command> [flags] (folded corvint-test-runner, CTF-V0)"

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
	// The runner group shares the old companion's entry, so its output is byte-identical
	// apart from the invocation name in diagnostics (CTF-V0-002, CTF-V0-004).
	if len(os.Args) > 1 && os.Args[1] == "runner" {
		os.Exit(cli.Command(ctx, "corvint-test runner", os.Args[2:], os.Stdout, os.Stderr))
	}
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "corvint-test e2e:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	if len(args) < 2 || args[0] != "e2e" {
		return errors.New(usage)
	}
	sub := args[1]
	fs := flag.NewFlagSet("corvint-test e2e "+sub, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "repository root")
	manifest := fs.String("manifest", "appmap.json", "repository-relative application-map manifest")
	out := fs.String("out", "", "output file (catalog, harvest, obligations) or directory (compile, prove), outside the repository")
	var rev, base, head, coverage *string
	var repeats *int
	var nodeModules, nodeBin, packageJSON, lockfile, window, hostedTest, hostedTitle *string
	switch sub {
	case "shadow":
		window = fs.String("window", "", "frozen shadow window file (CCE-V0-011)")
	case "catalog", "harvest":
		rev = fs.String("rev", "", "revision")
	case "obligations", "compile", "prove":
		base = fs.String("base", "", "base revision")
		head = fs.String("head", "", "head revision")
		coverage = fs.String("coverage", "", "runtime coverage document recorded at head; it may only widen reach (CCE-V0-045)")
		if sub == "prove" {
			repeats = fs.Int("repeats", changee2e.DefaultRepeats, "repeats per side")
			nodeModules = fs.String("node-modules", os.Getenv("CORVINT_PLAYWRIGHT_MODULES"), "node_modules holding @playwright/test")
			nodeBin = fs.String("node-bin", os.Getenv("CORVINT_E2E_NODE_BIN"), "directory holding node and npx")
			packageJSON = fs.String("package-json", "", "package.json binding the runner identity")
			lockfile = fs.String("lockfile", "", "lockfile binding the runner identity")
			hostedTest = fs.String("hosted-test", "", "prove this existing hosted Playwright test file as given, without compiling (CCE-V0-034)")
			hostedTitle = fs.String("hosted-title", "", "the hosted test's title; optional when the file declares one test")
		}
	default:
		return errors.New(usage)
	}
	if err := fs.Parse(args[2:]); err != nil {
		return fmt.Errorf("%v (%s)", err, usage)
	}
	if fs.NArg() != 0 {
		return errors.New(usage)
	}
	outPath := ""
	if *out != "" || sub == "compile" || sub == "prove" {
		p, err := changee2e.CheckOut(*root, *out)
		if err != nil {
			return err
		}
		outPath = p
	}
	diff := func() changee2e.DiffOptions {
		return changee2e.DiffOptions{Root: *root, Base: *base, Head: *head, Manifest: *manifest, Coverage: *coverage}
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
		c, err := changee2e.BuildCatalog(ctx, changee2e.CatalogOptions{Root: *root, Rev: *rev, Manifest: *manifest})
		if err != nil {
			return err
		}
		doc = c.Encoded()
	case "harvest":
		p, err := changee2e.Harvest(ctx, changee2e.HarvestOptions{Root: *root, Rev: *rev})
		if err != nil {
			return err
		}
		doc = p.Encoded()
	case "obligations":
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
			if *hostedTest == "" {
				return &changee2e.Refusal{Code: "e2e-hosted-test-invalid", Detail: "--hosted-title needs --hosted-test"}
			}
			if *coverage != "" {
				return &changee2e.Refusal{Code: "e2e-hosted-test-invalid", Detail: "--coverage compiles reach; a hosted test is proved as given"}
			}
			doc, err := changee2e.Prove(ctx, changee2e.ProveOptions{Root: *root, Base: *base, Head: *head, Repeats: *repeats,
				NodeModules: *nodeModules, NodeBin: *nodeBin, PackageJSON: *packageJSON, Lockfile: *lockfile,
				Hosted: &changee2e.HostedSelection{File: *hostedTest, Title: *hostedTitle}})
			if err != nil {
				return err
			}
			return writeUnder(outPath, proveReport, doc, stdout)
		}
		r, err := changee2e.Compile(ctx, diff())
		if err != nil {
			return err
		}
		if err := writeScenarios(outPath, r.Compiled(), proveReport); err != nil {
			return err
		}
		doc, err = changee2e.Prove(ctx, changee2e.ProveOptions{Root: *root, Base: r.Base, Head: r.Head, Repeats: *repeats,
			NodeModules: *nodeModules, NodeBin: *nodeBin, PackageJSON: *packageJSON, Lockfile: *lockfile, Scenarios: r.Compiled()})
		if err != nil {
			return err
		}
		return writeUnder(outPath, proveReport, doc, stdout)
	}
	if outPath != "" {
		return writeFile(outPath, doc, nil)
	}
	_, err := stdout.Write(doc)
	return err
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
