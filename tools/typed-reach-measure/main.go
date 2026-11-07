// Command typed-reach-measure measures AFP-V0-2370 test-function narrowing
// on a real change (AFP-V0-2378): it plans the kept test functions of one
// package for the Go change between two commits of a checkout whose working
// tree is the head, and with -oracle runs every test function of the package
// alone, at head and with each changed function made to panic on entry, and
// reports every test whose outcome the fault changes that the plan dropped.
//
// Usage:
//
//	typed-reach-measure -root DIR -base REV -pkg DIR [-oracle] [-run-timeout 2m]
//
// Example:
//
//	typed-reach-measure -root /tmp/wt -base HEAD^ -pkg internal/tasks/store -oracle
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected/golang/typedreach"
)

var (
	droppedOnly bool
	alsoRun     string
)

type report struct {
	Root       string            `json:"root"`
	Base       string            `json:"base"`
	Head       string            `json:"head"`
	Package    string            `json:"package"`
	Changed    []string          `json:"changed"`
	Config     string            `json:"config"`
	Whole      string            `json:"whole,omitempty"`
	Tests      int               `json:"tests"`
	Kept       map[string]string `json:"kept"`
	Dropped    []string          `json:"dropped"`
	PlanMillis int64             `json:"planMillis"`
	Oracle     *oracle           `json:"oracle,omitempty"`
}

type oracle struct {
	Faulted   []string `json:"faultedFunctions"`
	Ran       int      `json:"ran"`
	Observers []string `json:"observers"`
	Missed    []string `json:"missed"`
	Recall    string   `json:"recall"`
	Errors    []string `json:"errors,omitempty"`
}

func main() {
	root := flag.String("root", ".", "checkout whose working tree is the head")
	base := flag.String("base", "", "base revision")
	pkg := flag.String("pkg", "", "test package directory, relative to root")
	runOracle := flag.Bool("oracle", false, "run the fault-injection oracle")
	runTimeout := flag.Duration("run-timeout", 2*time.Minute, "bound of one test run")
	flag.BoolVar(&droppedOnly, "oracle-dropped-only", false, "run the oracle over the dropped tests only (a missed observer can only be one of them)")
	flag.StringVar(&alsoRun, "oracle-also", "", "comma-separated kept tests the dropped-only oracle also runs, to show the fault is observable")
	flag.Parse()
	if *base == "" || *pkg == "" {
		fmt.Fprintln(os.Stderr, "usage: typed-reach-measure -root DIR -base REV -pkg DIR [-oracle]")
		os.Exit(2)
	}
	out, err := measure(*root, *base, *pkg, *runOracle, *runTimeout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "typed-reach-measure:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(out)
}

func git(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, stderr.String())
	}
	return output, nil
}

func measure(root, base, pkg string, runOracle bool, runTimeout time.Duration) (report, error) {
	module, err := modulePath(root)
	if err != nil {
		return report{}, err
	}
	head, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return report{}, err
	}
	names, err := git(root, "diff", "--name-only", "--no-renames", base, "HEAD", "--", "*.go")
	if err != nil {
		return report{}, err
	}
	out := report{Root: root, Base: base, Head: strings.TrimSpace(string(head)), Package: pkg, Kept: map[string]string{}}
	var changed []typedreach.Declaration
	baseFiles := map[string][]byte{}
	for _, name := range strings.Fields(string(names)) {
		if strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "testdata/") || strings.Contains(name, "/testdata/") {
			continue
		}
		before, _ := git(root, "show", base+":"+name)
		baseFiles[name] = before
		after, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			after = nil
		}
		importPath := typedreach.ImportPath(module, name)
		for _, declared := range typedreach.DiffFile(name, before, after).Declarations(importPath) {
			changed = append(changed, declared)
			out.Changed = append(out.Changed, importPath+"."+declared.Name)
		}
	}
	start := time.Now()
	// The oracle builds with this process's environment and no flags, so
	// constraints are evaluated under that configuration (AFP-V0-2372).
	config, err := typedreach.ProbeConfig(context.Background(), root, os.Environ(), nil)
	if err != nil {
		return report{}, err
	}
	out.Config = config.Identity()
	program, err := typedreach.LoadFor(config, root, module, pkg)
	if err != nil {
		return report{}, err
	}
	result := typedreach.Analyze(program, changed, nil)
	if result.Whole == "" {
		before, err := typedreach.LoadBaseFor(config, root, module, pkg, baseFiles)
		if err != nil {
			result.Whole = typedreach.WholeBaseSource + ": " + err.Error()
		} else {
			result.Whole = typedreach.BaseEffects(before, changed)
		}
	}
	out.PlanMillis = time.Since(start).Milliseconds()
	out.Whole, out.Tests, out.Dropped = result.Whole, result.Tests, result.Dropped
	for test, why := range result.Kept {
		out.Kept[test] = why
	}
	if runOracle {
		o, err := runFaultOracle(root, module, pkg, changed, result, runTimeout)
		if err != nil {
			return out, err
		}
		out.Oracle = o
	}
	return out, nil
}

func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("no module line in go.mod")
}

// runFaultOracle builds the package's test binary twice, at head and with a
// panic at the entry of each changed function, runs every test function
// alone in both, and names the tests whose outcome differs.
func runFaultOracle(root, module, pkg string, changed []typedreach.Declaration, result typedreach.Result, runTimeout time.Duration) (*oracle, error) {
	o := &oracle{}
	scratch, err := os.MkdirTemp("", "typed-reach-oracle-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(scratch)
	clean := filepath.Join(scratch, "clean.test")
	if err := goTestBuild(root, pkg, clean); err != nil {
		return nil, err
	}
	restore, faulted, err := injectFaults(root, module, changed)
	if err != nil {
		restore()
		return nil, err
	}
	o.Faulted = faulted
	faultBinary := filepath.Join(scratch, "fault.test")
	buildErr := goTestBuild(root, pkg, faultBinary)
	restore()
	if buildErr != nil {
		return nil, buildErr
	}
	tests := append([]string(nil), result.Dropped...)
	for test := range result.Kept {
		if !droppedOnly {
			tests = append(tests, test)
		}
	}
	if droppedOnly && alsoRun != "" {
		tests = append(tests, strings.Split(alsoRun, ",")...)
	}
	o.Ran = len(tests)
	sort.Strings(tests)
	directory := filepath.Join(root, pkg)
	for _, test := range tests {
		a := runOne(clean, directory, test, runTimeout)
		b := runOne(faultBinary, directory, test, runTimeout)
		if a != b {
			o.Observers = append(o.Observers, test+" ("+a+" -> "+b+")")
			if _, kept := result.Kept[test]; !kept && result.Whole == "" {
				o.Missed = append(o.Missed, test)
			}
		}
	}
	o.Recall = fmt.Sprintf("%d/%d", len(o.Observers)-len(o.Missed), len(o.Observers))
	return o, nil
}

func goTestBuild(root, pkg, output string) error {
	command := exec.Command("go", "test", "-c", "-o", output, "./"+pkg)
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "CGO_ENABLED=0")
	if out, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("go test -c: %v\n%s", err, out)
	}
	return nil
}

// runOne runs one top-level test function and classifies its outcome.
func runOne(binary, directory, test string, timeout time.Duration) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, binary, "-test.run", "^"+test+"$", "-test.count=1", "-test.v")
	command.Dir = directory
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := command.CombinedOutput()
	switch {
	case ctx.Err() != nil:
		return "timeout"
	case err != nil:
		return "fail"
	case bytes.Contains(output, []byte("no tests to run")):
		return "none"
	case bytes.Contains(output, []byte("--- SKIP: "+test+" ")):
		return "skip"
	case bytes.Contains(output, []byte("--- PASS: "+test+" ")):
		return "pass"
	}
	return "unclassified"
}

// injectFaults inserts `panic("typed-reach fault")` as the first statement of
// every changed function or method with a body, rewriting the files in place;
// restore puts the original bytes back.
func injectFaults(root, module string, changed []typedreach.Declaration) (func(), []string, error) {
	originals := map[string][]byte{}
	restore := func() {
		for name, data := range originals {
			_ = os.WriteFile(name, data, 0o644)
		}
	}
	wanted := map[string]map[string]bool{}
	for _, declaration := range changed {
		directory := strings.TrimPrefix(strings.TrimPrefix(declaration.Package, module), "/")
		if wanted[directory] == nil {
			wanted[directory] = map[string]bool{}
		}
		wanted[directory][declaration.Name] = true
	}
	var faulted []string
	for directory, names := range wanted {
		entries, err := os.ReadDir(filepath.Join(root, directory))
		if err != nil {
			return restore, nil, err
		}
		for _, entry := range entries {
			name := filepath.Join(root, directory, entry.Name())
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			source, err := os.ReadFile(name)
			if err != nil {
				return restore, nil, err
			}
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, name, source, parser.SkipObjectResolution)
			if err != nil {
				continue
			}
			var offsets []int
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil || fn.Name.Name == "init" {
					continue
				}
				key := fn.Name.Name
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					key = receiverKey(fn.Recv.List[0].Type) + "." + key
				}
				if names[key] {
					offsets = append(offsets, fset.Position(fn.Body.Lbrace).Offset+1)
					faulted = append(faulted, directory+"."+key)
				}
			}
			if len(offsets) == 0 {
				continue
			}
			originals[name] = source
			sort.Sort(sort.Reverse(sort.IntSlice(offsets)))
			rewritten := append([]byte(nil), source...)
			for _, offset := range offsets {
				rewritten = append(rewritten[:offset], append([]byte(` panic("typed-reach fault");`), rewritten[offset:]...)...)
			}
			if err := os.WriteFile(name, rewritten, 0o644); err != nil {
				return restore, nil, err
			}
		}
	}
	sort.Strings(faulted)
	return restore, faulted, nil
}

func receiverKey(expr ast.Expr) string {
	for {
		switch typed := expr.(type) {
		case *ast.StarExpr:
			expr = typed.X
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return "?"
		}
	}
}
