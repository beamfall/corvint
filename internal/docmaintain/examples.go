package docmaintain

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// exampleWaitDelay bounds how long a cancelled example run waits for a descendant that still
// holds its output pipes after the go command itself was killed.
const exampleWaitDelay = time.Second

// Example declarations (SDD-V0-014). An owner binds a generated block to Go examples with one
// prose line outside every fence and every block body:
//
//	<!-- corvint:docmaintain example source="owner.md" package="internal/foo" name="ExampleBar" -->
//
// The line is ordinary prose to Preview, Apply and Watch: they never parse, rewrite or run it, so
// the block stays idempotent and the declaration survives every refresh. Only CheckExamples, an
// operator-invoked check that is never part of a read command, runs the declared examples, and it
// runs them through the Go test runner rather than importing or evaluating anything itself.

const exampleMarkerPrefix = "<!-- corvint:docmaintain example "

var exampleLine = regexp.MustCompile(`^<!-- corvint:docmaintain example source="([^"]*)" package="([^"]*)" name="(Example[A-Za-z0-9_]*)" -->$`)

// ExampleCheckProfile names the closed example-check report.
const ExampleCheckProfile = "corvint-docs-example-check/0"

// Example check states and declaration problem codes.
const (
	ExamplePass   = "PASS"
	ExampleFail   = "FAIL"
	ExampleNotRun = "NOT_RUN"

	ExampleMalformed      = "example-malformed"
	ExampleInBlockBody    = "example-in-generated-body"
	ExampleUnboundBlock   = "example-unbound-block"
	ExamplePackageInvalid = "example-package-invalid"
)

// ExampleDecl is one declared example: the block it belongs to (Source, Package) and the Go
// example function name, with the 1-based page line that declares it.
type ExampleDecl struct {
	Source  string `json:"source"`
	Package string `json:"package"`
	Name    string `json:"name"`
	Line    int    `json:"line"`
}

// ExampleProblem is a declaration line that cannot be checked; it fails the check, never silently.
type ExampleProblem struct {
	Line int    `json:"line"`
	Code string `json:"code"`
}

// ExampleResult is one declared example's observed outcome.
type ExampleResult struct {
	ExampleDecl
	Status string `json:"status"`
}

// ExampleReport is the closed result of one CheckExamples run. Verdict is PASS only when every
// declaration parsed, bound to a block on the page, and its example passed.
type ExampleReport struct {
	Profile  string           `json:"profile"`
	Verdict  string           `json:"verdict"`
	Results  []ExampleResult  `json:"results"`
	Problems []ExampleProblem `json:"problems"`
}

// ExampleRunner runs the named examples of one package directory (relative to root) through an
// existing test provider and returns each observed name's state (ExamplePass or ExampleFail). A
// name absent from the map was not run.
type ExampleRunner func(ctx context.Context, root, pkg string, names []string) (map[string]string, error)

// DeclaredExamples reads the example declarations of page without running anything. It returns
// the page refusal code from scanPage when the page's blocks are malformed. A declaration must sit
// outside fences and block bodies, parse exactly, name a generated block present on the page, and
// name a package directory that is relative and inside the repository.
func DeclaredExamples(page []byte) ([]ExampleDecl, []ExampleProblem, string) {
	layout, refusal := scanPage(page)
	if refusal != "" {
		return nil, nil, refusal
	}
	blocks := map[[2]string]bool{}
	for _, b := range layout.blocks {
		blocks[[2]string{b.source, b.pkg}] = true
	}
	decls, problems := []ExampleDecl{}, []ExampleProblem{}
	var fenceChar byte
	fenceLen, lineNo := 0, 0
	for offset := 0; offset < len(page); {
		lineNo++
		end := bytes.IndexByte(page[offset:], '\n')
		next := len(page)
		if end < 0 {
			end = len(page)
		} else {
			end += offset
			next = end + 1
		}
		line := string(page[offset:end])
		inBody := false
		for _, b := range layout.blocks {
			if offset > b.start && offset < b.end {
				inBody = true
			}
		}
		offset = next
		switch {
		case fenceLen > 0:
			if closesFence(line, fenceChar, fenceLen) {
				fenceLen = 0
			}
			continue
		case inBody:
			if isExampleMarker(line) {
				problems = append(problems, ExampleProblem{Line: lineNo, Code: ExampleInBlockBody})
			}
			continue
		}
		if char, length, ok := opensFence(line); ok {
			fenceChar, fenceLen = char, length
			continue
		}
		if !isExampleMarker(line) {
			continue
		}
		m := exampleLine.FindStringSubmatch(strings.TrimLeft(line, " "))
		switch {
		case m == nil:
			problems = append(problems, ExampleProblem{Line: lineNo, Code: ExampleMalformed})
		case !blocks[[2]string{m[1], m[2]}]:
			problems = append(problems, ExampleProblem{Line: lineNo, Code: ExampleUnboundBlock})
		case !validExamplePackage(m[2]):
			problems = append(problems, ExampleProblem{Line: lineNo, Code: ExamplePackageInvalid})
		default:
			decls = append(decls, ExampleDecl{Source: m[1], Package: m[2], Name: m[3], Line: lineNo})
		}
	}
	return decls, problems, ""
}

func isExampleMarker(line string) bool {
	trimmed := strings.TrimLeft(line, " ")
	return len(line)-len(trimmed) <= 3 && strings.HasPrefix(trimmed, strings.TrimRight(exampleMarkerPrefix, " "))
}

// validExamplePackage admits a slash-separated relative directory with no empty, dot, dot-dot or
// Git metadata segment and no leading dash, so it can never be read as a go flag.
func validExamplePackage(pkg string) bool {
	if pkg == "" || strings.HasPrefix(pkg, "-") || strings.HasPrefix(pkg, "/") || strings.ContainsAny(pkg, "\\ \t") {
		return false
	}
	for _, s := range strings.Split(pkg, "/") {
		if s == "" || s == "." || s == ".." || strings.EqualFold(s, ".git") {
			return false
		}
	}
	return true
}

// CheckExamples runs every example declared on page through run, one call per package, and
// reports each declaration's state. It is an operator check, never called by Preview, Apply,
// Watch or docs consume.
func CheckExamples(ctx context.Context, root string, page []byte, run ExampleRunner) (ExampleReport, error) {
	decls, problems, refusal := DeclaredExamples(page)
	if refusal != "" {
		return ExampleReport{}, refuse(refusal)
	}
	byPkg := map[string][]string{}
	for _, d := range decls {
		byPkg[d.Package] = append(byPkg[d.Package], d.Name)
	}
	pkgs := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	observed := map[[2]string]string{}
	for _, p := range pkgs {
		states, err := run(ctx, root, p, uniqueSorted(byPkg[p]))
		if err != nil {
			return ExampleReport{}, err
		}
		for name, s := range states {
			observed[[2]string{p, name}] = s
		}
	}
	report := ExampleReport{Profile: ExampleCheckProfile, Verdict: ExamplePass, Results: []ExampleResult{}, Problems: problems}
	if len(problems) > 0 || len(decls) == 0 {
		report.Verdict = ExampleFail
	}
	for _, d := range decls {
		s, ok := observed[[2]string{d.Package, d.Name}]
		if !ok {
			s = ExampleNotRun
		}
		if s != ExamplePass {
			report.Verdict = ExampleFail
		}
		report.Results = append(report.Results, ExampleResult{ExampleDecl: d, Status: s})
	}
	return report, nil
}

func uniqueSorted(names []string) []string {
	sort.Strings(names)
	out := names[:0]
	for i, n := range names {
		if i == 0 || n != names[i-1] {
			out = append(out, n)
		}
	}
	return out
}

// GoTestRunner is the ExampleRunner backed by the Go toolchain's own test provider: it runs
// `go test -count=1 -json -run '^(A|B)$' ./pkg` under root and reads each example's pass or fail
// event. Go checks an example's `// Output:` comment; an example without one only compiles.
func GoTestRunner(goBin string) ExampleRunner {
	return func(ctx context.Context, root, pkg string, names []string) (map[string]string, error) {
		pattern := "^(" + strings.Join(names, "|") + ")$"
		cmd := exec.CommandContext(ctx, goBin, "test", "-count=1", "-json", "-run", pattern, "./"+pkg)
		cmd.Dir = root
		// CommandContext kills only the go command; a descendant can keep the pipes open, so a
		// cancelled run bounds Wait's pipe drain (as procgroup and testrunner do).
		cmd.WaitDelay = exampleWaitDelay
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		var stderr bytes.Buffer
		cmd.Stderr = &limitedWriter{w: &stderr, n: 64 << 10}
		if err := cmd.Start(); err != nil {
			return nil, err
		}
		// Cancellation closes the read end, so the scan and drain below cannot block on a
		// descendant that inherited stdout; the run then reports ctx.Err().
		stopClose := context.AfterFunc(ctx, func() { _ = out.Close() })
		defer stopClose()
		states := map[string]string{}
		want := map[string]bool{}
		for _, n := range names {
			want[n] = true
		}
		scan := bufio.NewScanner(io.LimitReader(out, 64<<20))
		scan.Buffer(make([]byte, 0, 64<<10), 1<<20)
		for scan.Scan() {
			var ev struct{ Action, Test string }
			if json.Unmarshal(scan.Bytes(), &ev) != nil || !want[ev.Test] {
				continue
			}
			switch ev.Action {
			case "pass":
				states[ev.Test] = ExamplePass
			case "fail":
				states[ev.Test] = ExampleFail
			}
		}
		// Drain what the scanner did not read (past its bound or an over-long line), so a child
		// that keeps writing is never blocked on a full pipe and Wait returns.
		_, _ = io.Copy(io.Discard, out)
		waitErr := cmd.Wait()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if waitErr != nil {
			if _, ok := waitErr.(*exec.ExitError); !ok {
				return nil, fmt.Errorf("go test %s: %w", pkg, waitErr)
			}
			// A nonzero exit that recorded no FAIL (a build, TestMain or teardown failure) is
			// not a passing run: every declared example stays NOT_RUN.
			failed := false
			for _, s := range states {
				failed = failed || s == ExampleFail
			}
			if !failed {
				return map[string]string{}, nil
			}
		}
		return states, nil
	}
}

type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n > 0 {
		k := min(len(p), l.n)
		_, _ = l.w.Write(p[:k])
		l.n -= k
	}
	return len(p), nil
}
