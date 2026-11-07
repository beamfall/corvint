// SPDX-License-Identifier: Apache-2.0
package nativelinecov

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/linecov/v8"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/native"
	"github.com/Beamfall/corvint/internal/testrunner/platform"
)

// Process is caller-observed process lifecycle and native coverage, including
// the runner and every descendant, even a launcher. Parent is zero only for
// the one root. No dump or launch completion is inferred from a passing test.
type Process struct {
	ID, Parent           int
	Exited, Instrumented bool
	Reports              []Report
}

// Capture is the native execution/export adapter's result. TreeComplete is
// independently observed complete launch/retirement accounting, not a promise
// derived from finding some coverage files (LPCV-V0-8205).
type Capture struct {
	Input        tr.Input
	TreeComplete bool
	Processes    []Process
}

// Options binds native execution to one module/package. Run must execute the
// supplied request/invocation and export coverage from the complete process
// tree before returning. It may use testrunner.Execute for admitted execution;
// that executor alone does not attest instrumentation or export coverage.
// These in-process hooks are deliberately not a new report/wire protocol.
type Options struct {
	Module, Package, Work, Version string
	Request                        tr.Request
	Sources                        map[string]string
	Agent                          string // independently admitted absolute JaCoCo agent jar
	// Selectors resolves a native reported ID where a runner needs another
	// exact selector (e.g. Gradle class#method). Nil uses the reported ID.
	Selectors map[string]string
	Run       func(context.Context, tr.Request, tr.Invocation) (Capture, error)
}

// Family returns the coverage family of an existing runner profile.
func Family(r string) string {
	switch r {
	case "junit-platform", "junit4", "testng", "kotest", "kotlin-test", "robolectric", "gradle-junit", "maven-surefire":
		return "jvm"
	case "dotnet-vstest-nunit", "dotnet-vstest-xunit", "dotnet-vstest-mstest", "dotnet-mtp-nunit", "dotnet-mtp-xunit", "dotnet-mtp-mstest":
		return "dotnet"
	case "ctest", "googletest", "catch2", "boost-test-junit", "cmocka-xml":
		return "cfamily"
	case "cargo-test", "cargo-integration", "cargo-bin", "cargo-doctest", "nextest":
		return "rust"
	case "swift-xctest", "swift-testing":
		return "swift"
	}
	return ""
}

// Identity binds runner version and instrumentation configuration to an index.
// The caller includes all binary/build/tool identities in version.
func Identity(runner, version string) string { return "native-lines/1 " + runner + " " + version }

// Collect obtains the ordinary suite record and isolates every passing native
// identity. Records are emitted only for complete passing native observations
// with exact isolation and complete instrumented process coverage.
func Collect(ctx context.Context, o Options) ([]linecov.Outcome, error) {
	if Family(o.Request.Runner) == "" || o.Module == "" || o.Version == "" || o.Run == nil || !filepath.IsAbs(o.Work) || !filepath.IsAbs(o.Request.Root) || o.Package == "" {
		return nil, fmt.Errorf("native collector requires runner, identity, absolute paths and execution/export adapter")
	}
	if len(o.Request.Selectors) > 0 {
		return nil, fmt.Errorf("native collector requires an unfiltered suite")
	}
	if err := os.MkdirAll(o.Work, 0700); err != nil {
		return nil, err
	}
	work, err := os.MkdirTemp(o.Work, "n-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(work)
	suite, observed, err := o.run(ctx, work, 0, linecov.InOrder, "")
	if err != nil {
		return nil, err
	}
	out := []linecov.Outcome{suite}
	// A coverage refusal does not erase the observed test identities: isolated
	// runs can still produce useful records, while the suite keeps static scope.
	if !observed.Complete {
		return out, nil
	}
	for _, test := range observed.Tests {
		if test.ID == linecov.InOrder {
			out = append(out, linecov.Outcome{Package: o.Package, Name: test.ID, Divergent: "reserved native test identity"})
			continue
		}
		if test.State != tr.Passed {
			continue
		}
		selector := test.ID
		// ConsoleLauncher does not admit --select-unique-id on the pinned
		// native console. Standard method IDs have an exact native method
		// selector; dynamic/nested IDs still need the caller's explicit map.
		switch o.Request.Runner {
		case "junit-platform", "junit4", "kotlin-test", "robolectric":
			if method, ok := junitMethodSelector(test.ID); ok {
				selector = method
			}
		}
		if strings.HasPrefix(o.Request.Runner, "dotnet-mtp-") {
			selector, _, _ = strings.Cut(test.ID, "::")
		}
		if o.Request.Runner == "swift-testing" {
			selector, _, _ = strings.Cut(test.ID, "/")
		}
		if o.Selectors != nil {
			var ok bool
			selector, ok = o.Selectors[test.ID]
			if !ok {
				out = append(out, linecov.Outcome{Package: o.Package, Name: test.ID, Divergent: "missing exact selector"})
				continue
			}
		}
		one, _, err := o.run(ctx, work, len(out), test.ID, selector)
		if err != nil {
			return nil, err
		}
		out = append(out, one)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// junitMethodSelector translates only the simple, unambiguous discovery ID.
// Extra segments, escapes and dynamic invocations retain the old refusal path.
func junitMethodSelector(id string) (string, bool) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], "[engine:") || !strings.HasSuffix(parts[0], "]") {
		return "", false
	}
	class, ok := strings.CutPrefix(parts[1], "[class:")
	if !ok || !strings.HasSuffix(class, "]") {
		return "", false
	}
	class = strings.TrimSuffix(class, "]")
	method, ok := strings.CutPrefix(parts[2], "[method:")
	if !ok || !strings.HasSuffix(method, "()]") {
		return "", false
	}
	method = strings.TrimSuffix(method, "()]")
	if class == "" || method == "" || strings.ContainsAny(class+method, "[]\\%#") {
		return "", false
	}
	return class + "#" + method, true
}

func (o Options) run(ctx context.Context, work string, seq int, name, selector string) (linecov.Outcome, tr.Observation, error) {
	out := linecov.Outcome{Package: o.Package, Name: name}
	var obs tr.Observation
	if err := ctx.Err(); err != nil {
		return out, obs, err
	}
	r := o.Request
	r.ReportDir = filepath.Join(work, fmt.Sprintf("%d", seq))
	r.Selectors = nil
	// Inventory-based native protocols cannot discover disabled or missing
	// identities from terminal reports alone. Keep the caller's inventory;
	// cmocka admits exactly the one expected identity for an isolated run.
	if r.Runner == "cmocka-xml" || r.Runner == "boost-test-junit" {
		r.ExpectedTests = append([]string(nil), o.Request.ExpectedTests...)
		if r.Runner == "cmocka-xml" && selector != "" {
			r.ExpectedTests = []string{name}
		}
	} else {
		r.ExpectedTests = nil
	}
	r.ExpectedSelection = nil
	if selector != "" {
		r.Selectors = []string{selector}
	}
	var inv tr.Invocation
	var err error
	if r.Runner == "testng" && selector != "" {
		// Preserve suite/test names in the native ID while selecting one method.
		parts := strings.Split(name, "/")
		if len(parts) != 3 {
			out.Divergent = "ambiguous TestNG identity"
			return out, obs, nil
		}
		class, method, ok := strings.Cut(parts[2], "#")
		if !ok || class == "" || method == "" {
			out.Divergent = "ambiguous TestNG identity"
			return out, obs, nil
		}
		esc := func(s string) string { var b strings.Builder; _ = xml.EscapeText(&b, []byte(s)); return b.String() }
		data := []byte(`<suite name="` + esc(parts[0]) + `"><test name="` + esc(parts[1]) + `"><classes><class name="` + esc(class) + `"><methods><include name="` + esc(method) + `"/></methods></class></classes></test></suite>`)
		r.Config = filepath.Join(work, fmt.Sprintf("testng-%d.xml", seq))
		if err = os.WriteFile(r.Config, data, 0600); err != nil {
			return out, obs, err
		}
		sum := sha256.Sum256(data)
		r.ConfigSha256 = hex.EncodeToString(sum[:])
		r.Selectors = nil
	}
	family := Family(r.Runner)
	if family == "jvm" || family == "swift" {
		inv, err = platform.Build(r)
	} else {
		inv, err = native.Build(r)
	}
	if err != nil {
		out.Divergent = "native invocation: " + err.Error()
		return out, obs, nil
	}
	if err = instrument(o, r.ReportDir, &inv); err != nil {
		return out, obs, err
	}
	start := time.Now()
	capture, err := o.Run(ctx, r, inv)
	if err != nil {
		out.Divergent = "native execution: " + err.Error()
		return out, obs, nil
	}
	if err = ctx.Err(); err != nil {
		return out, obs, err
	}
	input := capture.Input
	if input.Runner != r.Runner {
		out.Divergent = "native runner mismatch"
		return out, obs, nil
	}
	if input.TimedOut || input.Interrupted || input.Overflow || len(input.ExecutionProblems) > 0 {
		out.Divergent = "incomplete native execution"
		return out, obs, nil
	}
	if family == "jvm" || family == "swift" {
		obs, err = platform.Parse(input)
	} else {
		obs, err = native.Parse(input)
	}
	if err != nil {
		out.Divergent = "native observation: " + err.Error()
		return out, obs, nil
	}
	if !obs.Complete || len(obs.Problems) > 0 {
		out.Divergent = "incomplete native observation"
		return out, obs, nil
	}
	passed := 0
	for _, test := range obs.Tests {
		if test.State == tr.Skipped {
			continue
		}
		if test.State != tr.Passed || name != linecov.InOrder && test.ID != name {
			out.Divergent = "native filter/status mismatch"
			return out, obs, nil
		}
		passed++
	}
	if passed == 0 || name != linecov.InOrder && passed != 1 {
		out.Divergent = "native filter did not run exactly one passing test"
		return out, obs, nil
	}
	lines, err := processLines(capture, o.Sources)
	if err != nil {
		out.Divergent = err.Error()
		return out, obs, nil
	}
	out.Record = &linecov.Record{Package: o.Package, Name: name, Profile: lines.Profile(o.Module), Elapsed: time.Since(start)}
	return out, obs, nil
}

func instrument(o Options, dir string, inv *tr.Invocation) error {
	if inv.Environment == nil {
		inv.Environment = map[string]string{}
	}
	env := inv.Environment
	switch Family(o.Request.Runner) {
	case "jvm":
		if !filepath.IsAbs(o.Agent) || strings.ContainsAny(o.Agent, "\x00\r\n,=\"") || strings.ContainsAny(dir, "\x00\r\n,=\"") {
			return fmt.Errorf("native JVM collector requires an admitted absolute JaCoCo agent")
		}
		env["JAVA_TOOL_OPTIONS"] = `"-javaagent:` + o.Agent + `=destfile=` + filepath.Join(dir, "jacoco.exec") + `,append=true,dumponexit=true,output=file"`
	case "dotnet":
		if strings.HasPrefix(o.Request.Runner, "dotnet-mtp-") {
			inv.Argv = append(inv.Argv, "--coverage", "--coverage-output-format", "cobertura", "--coverage-output", filepath.Join(dir, "coverage.cobertura.xml"))
		} else {
			inv.Argv = append(inv.Argv, "--collect:XPlat Code Coverage")
		}
	case "rust":
		env["RUSTFLAGS"] = "-C instrument-coverage"
		fallthrough
	case "cfamily", "swift":
		env["LLVM_PROFILE_FILE"] = filepath.Join(dir, "coverage-%p-%m.profraw")
	}
	if Family(o.Request.Runner) == "swift" {
		inv.Argv = append(inv.Argv, "--enable-code-coverage")
	}
	// Multi-phase profiles carry their own environment. Propagate coverage only
	// to TEST phases, never changing pinned decoder tool invocations.
	for i := range inv.Phases {
		if inv.Phases[i].Kind == "TEST" {
			if inv.Phases[i].Environment == nil {
				inv.Phases[i].Environment = map[string]string{}
			}
			for k, v := range env {
				inv.Phases[i].Environment[k] = v
			}
		}
	}
	return nil
}

func processLines(c Capture, sources map[string]string) (v8.Lines, error) {
	if !c.TreeComplete || len(c.Processes) == 0 || len(c.Processes) > 32 {
		return nil, fmt.Errorf("incomplete process coverage")
	}
	ids := map[int]Process{}
	roots := 0
	reports := 0
	for _, p := range c.Processes {
		if p.ID < 1 || p.Parent < 0 || p.Parent == p.ID || !p.Exited || !p.Instrumented || len(p.Reports) == 0 {
			return nil, fmt.Errorf("uninstrumented or undumped process")
		}
		if _, ok := ids[p.ID]; ok {
			return nil, fmt.Errorf("duplicate process identity")
		}
		ids[p.ID] = p
		if p.Parent == 0 {
			roots++
		}
		reports += len(p.Reports)
	}
	if roots != 1 || reports > 32 {
		return nil, fmt.Errorf("invalid process coverage tree/bound")
	}
	lines := v8.Lines{}
	for _, p := range c.Processes {
		seen := map[int]bool{}
		node := p.ID
		for node != 0 {
			if seen[node] {
				return nil, fmt.Errorf("cyclic process coverage")
			}
			seen[node] = true
			parent, ok := ids[node]
			if !ok {
				return nil, fmt.Errorf("disconnected process coverage")
			}
			node = parent.Parent
		}
		for _, report := range p.Reports {
			one, err := Decode(report, sources)
			if err != nil {
				return nil, err
			}
			lines.Merge(one)
		}
	}
	return lines, nil
}

// Build binds accepted records to immutable source blobs. Every package stays
// unordered: native-family exclusion is not qualified (LPCV-V0-8207). This
// cannot accidentally promote fixture evidence into production exclusions.
func Build(o Options, commit string, oids map[string]string, outcomes []linecov.Outcome) (*linecov.Index, error) {
	var records []linecov.Record
	for _, out := range outcomes {
		if out.Record != nil {
			records = append(records, *out.Record)
		}
	}
	ix, err := linecov.Build(o.Module, commit, Identity(o.Request.Runner, o.Version), oids, records)
	if err != nil {
		return nil, err
	}
	pkgs := map[string]bool{}
	for _, t := range ix.Tests {
		pkgs[t.Package] = true
	}
	ix.Unordered = nil
	for pkg := range pkgs {
		ix.Unordered = append(ix.Unordered, pkg)
	}
	sort.Strings(ix.Unordered)
	return ix, nil
}
