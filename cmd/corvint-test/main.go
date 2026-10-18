// Command corvint-test is the experimental test companion. Its `e2e` group compiles change-scoped
// E2E scenarios from admitted, revision-pinned blocks (CCE-V0). Core `corvint` does not import or
// execute it, and it never executes `corvint` (CCE-V0-015). Diagnostics carry the
// `corvint-test e2e:` prefix, disjoint from core `corvint test` diagnostics (`corvint-test:`).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Beamfall/corvint/internal/changee2e"
)

const usage = "usage: corvint-test e2e <catalog|obligations|compile|prove> [flags]"

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
	out := fs.String("out", "", "output file (catalog, obligations) or directory (compile, prove), outside the repository")
	var rev, base, head *string
	var repeats *int
	var nodeModules, nodeBin, packageJSON, lockfile *string
	switch sub {
	case "catalog":
		rev = fs.String("rev", "", "revision")
	case "obligations", "compile", "prove":
		base = fs.String("base", "", "base revision")
		head = fs.String("head", "", "head revision")
		if sub == "prove" {
			repeats = fs.Int("repeats", changee2e.DefaultRepeats, "repeats per side")
			nodeModules = fs.String("node-modules", os.Getenv("CORVINT_PLAYWRIGHT_MODULES"), "node_modules holding @playwright/test")
			nodeBin = fs.String("node-bin", os.Getenv("CORVINT_E2E_NODE_BIN"), "directory holding node and npx")
			packageJSON = fs.String("package-json", "", "package.json binding the runner identity")
			lockfile = fs.String("lockfile", "", "lockfile binding the runner identity")
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
		return changee2e.DiffOptions{Root: *root, Base: *base, Head: *head, Manifest: *manifest}
	}
	var doc []byte
	switch sub {
	case "catalog":
		c, err := changee2e.BuildCatalog(ctx, changee2e.CatalogOptions{Root: *root, Rev: *rev, Manifest: *manifest})
		if err != nil {
			return err
		}
		doc = c.Encoded()
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
		r, err := changee2e.Compile(ctx, diff())
		if err != nil {
			return err
		}
		if err := writeScenarios(outPath, r.Compiled()); err != nil {
			return err
		}
		doc, err = changee2e.Prove(ctx, changee2e.ProveOptions{Root: *root, Base: r.Base, Head: r.Head, Repeats: *repeats,
			NodeModules: *nodeModules, NodeBin: *nodeBin, PackageJSON: *packageJSON, Lockfile: *lockfile, Scenarios: r.Compiled()})
		if err != nil {
			return err
		}
		return writeFile(filepath.Join(outPath, "e2e-prove.json"), doc, stdout)
	}
	if outPath != "" {
		return writeFile(outPath, doc, nil)
	}
	_, err := stdout.Write(doc)
	return err
}

// writeScenarios writes each compiled scenario under dir at its placement (CCE-V0-007).
func writeScenarios(dir string, scenarios []changee2e.Scenario) error {
	for _, s := range scenarios {
		target := filepath.Join(dir, filepath.FromSlash(s.Placement))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, s.Text, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(path string, data []byte, echo io.Writer) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	if echo != nil {
		_, err := echo.Write(data)
		return err
	}
	return nil
}
