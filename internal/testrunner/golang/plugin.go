// SPDX-License-Identifier: Apache-2.0

// Package golang interprets core-captured Go discovery and coverage artifacts.
// It launches no processes and cannot write the evidence store or qualify itself.
package golang

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

const MaxArtifactBytes = 8 << 20

// Configuration describes ONE package under ONE CI build constraint. The core
// must instantiate it for every declared CI constraint; a host instance cannot
// stand in for another target. Environment is the complete captured environment,
// including CGO_ENABLED and GOFLAGS. The core admits the executable digest.
// Root is the module directory; its enclosing Git boundary determines the
// repository-relative prefix of immutable Sources and emitted evidence paths.
type Configuration struct {
	Checkout, Runner, OS, Arch                    string
	Executable, ExecutableDigest, RunnerVersion   string
	Package, ImportPath, Module, Repository, Root string
	Tags                                          []string
	Race                                          bool
	Environment                                   map[string]string
}

// Capture is core-owned output, bound to the exact key before interpretation.
// Complete means successful exit and no output truncation, for BOTH commands.
// Sources are immutable tree bytes, keyed by repository-relative path. The
// dependency graph supplies the active test files for this build constraint.
type Capture struct {
	Key                testrunner.InventoryKey
	Complete           bool
	Reason             string
	List, Dependencies string
	Sources            map[string]Source
	Coverage           []linecov.Record
}

type Plugin struct {
	config                       Configuration
	key                          testrunner.InventoryKey
	capture                      Capture
	repositoryRoot, modulePrefix string
}

var _ registry.RunnerPlugin = (*Plugin)(nil)

// New constructs an isolated SPI instance. It neither discovers CI constraints
// nor assumes the current host's build defaults apply to a different target.
func New(c Configuration, capture Capture) (*Plugin, error) {
	if !wire.IsGitOid(c.Checkout) || (c.Runner != "go-test" && c.Runner != "ginkgo-v2") || c.OS == "" || c.Arch == "" || c.RunnerVersion == "" || !filepath.IsAbs(c.Executable) || !filepath.IsAbs(c.Root) || filepath.Clean(c.Root) != c.Root || c.Module == "" || c.Repository == "" || !localPath(c.Package, true) || c.ImportPath == "" || strings.ContainsAny(c.ImportPath, " \t\r\n\\") {
		return nil, fmt.Errorf("go-configuration-invalid")
	}
	digest, err := hex.DecodeString(c.ExecutableDigest)
	if err != nil || len(digest) != 32 || strings.ToLower(c.ExecutableDigest) != c.ExecutableDigest {
		return nil, fmt.Errorf("go-executable-digest-invalid")
	}
	c.Tags = append([]string(nil), c.Tags...)
	sort.Strings(c.Tags)
	for i, tag := range c.Tags {
		if tag == "" || strings.ContainsAny(tag, " ,\t\r\n\x00") || i > 0 && tag == c.Tags[i-1] {
			return nil, fmt.Errorf("go-build-tags-invalid")
		}
	}
	env := make(map[string]string, len(c.Environment))
	for k, v := range c.Environment {
		if k == "" || strings.ContainsAny(k, "=\x00") || strings.ContainsRune(v, 0) {
			return nil, fmt.Errorf("go-environment-invalid")
		}
		env[k] = v
	}
	// Ambient flags can replace discovery/filtering or inject overlays. Callers
	// must express supported constraints explicitly, never silently discard them.
	if env["GOFLAGS"] != "" || env["GOOS"] != c.OS || env["GOARCH"] != c.Arch || (env["CGO_ENABLED"] != "0" && env["CGO_ENABLED"] != "1") {
		return nil, fmt.Errorf("go-build-environment-unsupported")
	}
	expectedImport := c.Module
	if c.Package != "." {
		expectedImport += "/" + c.Package
	}
	if expectedImport != c.ImportPath {
		return nil, fmt.Errorf("go-module-package-mismatch")
	}
	repositoryRoot, err := repositoryBoundary(c.Root)
	if err != nil {
		return nil, err
	}
	prefix, err := filepath.Rel(repositoryRoot, c.Root)
	if err != nil || !localPath(filepath.ToSlash(prefix), true) {
		return nil, fmt.Errorf("go-repository-root-unresolved")
	}
	prefix = filepath.ToSlash(prefix)
	c.Environment = env
	// Checkout binds the inventory, not the reusable execution configuration.
	stable := c
	stable.Checkout = ""
	raw, err := json.Marshal(struct {
		Configuration
		RepositoryRoot string
	}{stable, repositoryRoot})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	key := testrunner.InventoryKey{Checkout: c.Checkout, Runner: c.Runner, ConfigDigest: hex.EncodeToString(sum[:]), OS: c.OS, Arch: c.Arch}
	sources := make(map[string]Source, len(capture.Sources))
	total := len(capture.List) + len(capture.Dependencies)
	for name, source := range capture.Sources {
		total += len(source.Bytes)
		if total > MaxArtifactBytes {
			return nil, fmt.Errorf("go-artifact-cap")
		}
		source.Bytes = append([]byte(nil), source.Bytes...)
		sources[name] = source
	}
	if total > MaxArtifactBytes {
		return nil, fmt.Errorf("go-artifact-cap")
	}
	capture.Sources = sources
	capture.Coverage = append([]linecov.Record(nil), capture.Coverage...)
	for i := range capture.Coverage {
		if len(capture.Coverage[i].Segments) != 0 {
			return nil, fmt.Errorf("go-segmented-capture-unsupported")
		}
		total += len(capture.Coverage[i].Profile)
		if total > MaxArtifactBytes {
			return nil, fmt.Errorf("go-artifact-cap")
		}
		capture.Coverage[i].Profile = append([]byte(nil), capture.Coverage[i].Profile...)
	}
	return &Plugin{config: c, key: key, capture: capture, repositoryRoot: repositoryRoot, modulePrefix: prefix}, nil
}

// Only locate the checkout boundary; source bytes still come exclusively from
// the core's immutable capture. Both ordinary checkouts and Git worktrees have
// a .git marker. Missing, inaccessible or symlinked markers cannot establish it.
func repositoryBoundary(root string) (string, error) {
	for dir := root; ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(filepath.Join(dir, ".git"))
		if err == nil {
			if info.IsDir() || info.Mode().IsRegular() {
				return dir, nil
			}
			break
		}
		if !os.IsNotExist(err) || filepath.Dir(dir) == dir {
			break
		}
	}
	return "", fmt.Errorf("go-repository-root-unresolved")
}

func (p *Plugin) Key() testrunner.InventoryKey { return p.key }
func (p *Plugin) Pieces() testrunner.RunnerPieces {
	if p.config.Runner == "ginkgo-v2" {
		return testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Reason: "go-ginkgo-inventory-unsupported"}, TestEvidence: testrunner.RunnerPiece{Reason: "go-ginkgo-inventory-unsupported"}, Qualification: testrunner.RunnerPiece{Reason: "unqualified"}}
	}
	return testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Reason: "unqualified"}}
}

// BindCapture attaches output collected using this instance's commands. It
// returns a new instance; neither caller maps nor source byte slices are shared.
func (p *Plugin) BindCapture(c Capture) (*Plugin, error) {
	if c.Key != p.key {
		return nil, fmt.Errorf("go-capture-key-mismatch")
	}
	return New(p.config, c)
}

func (p *Plugin) Inventory(ctx context.Context, request registry.InventoryRequest) (testrunner.RunnerInventory, error) {
	fail := func(reason string) (testrunner.RunnerInventory, error) {
		return testrunner.RunnerInventory{Key: p.key, Reason: reason}, fmt.Errorf("%s", reason)
	}
	if err := ctx.Err(); err != nil {
		return fail("go-inventory-cancelled")
	}
	if request.Key != p.key || p.capture.Key != p.key {
		return fail("go-inventory-key-mismatch")
	}
	if !p.capture.Complete || p.capture.Reason != "" {
		return fail("go-inventory-capture-incomplete")
	}
	if p.config.Runner == "ginkgo-v2" {
		return fail("go-ginkgo-inventory-unsupported")
	}
	if request.Output != p.capture.List {
		return fail("go-inventory-output-mismatch")
	}
	packages, err := decodeDependencies(p.capture.Dependencies)
	if err != nil {
		return fail(err.Error())
	}
	var target *goPackage
	for i := range packages {
		if packages[i].ImportPath == p.config.ImportPath {
			if target != nil {
				return fail("go-inventory-package-duplicate")
			}
			target = &packages[i]
		}
	}
	if target == nil {
		return fail("go-inventory-package-missing")
	}
	if filepath.Clean(target.Dir) != filepath.Join(p.config.Root, filepath.FromSlash(p.config.Package)) {
		return fail("go-inventory-package-directory-mismatch")
	}
	names := map[string]string{}
	for _, name := range append(append([]string{}, target.TestGoFiles...), target.XTestGoFiles...) {
		if !localPath(name, false) || path.Base(name) != name {
			return fail("go-inventory-source-invalid")
		}
		filePath := path.Join(p.modulePrefix, p.config.Package, name)
		source, ok := p.capture.Sources[filePath]
		if !ok || !source.valid(filePath) {
			return fail("go-inventory-source-unbound")
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filePath, source.Bytes, parser.SkipObjectResolution)
		if err != nil {
			return fail("go-inventory-source-unparseable")
		}
		runnableExamples := map[string]bool{}
		for _, example := range doc.Examples(parsed) {
			if example.Output != "" || example.EmptyOutput {
				runnableExamples["Example"+example.Name] = true
			}
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !testName(fn.Name.Name) || strings.HasPrefix(fn.Name.Name, "Example") && !runnableExamples[fn.Name.Name] {
				continue
			}
			if _, exists := names[fn.Name.Name]; exists {
				return fail("go-inventory-source-ambiguous")
			}
			names[fn.Name.Name] = filePath
		}
	}
	events, err := decodeEvents(request.Output, p.config.ImportPath, false)
	if err != nil {
		return fail(err.Error())
	}
	var output strings.Builder
	for _, event := range events {
		if event.Test != "" {
			return fail("go-inventory-executed-test")
		}
		if event.Action == "output" {
			output.WriteString(event.Output)
		}
	}
	tests := []testrunner.InventoryTest{}
	for _, name := range strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n") {
		if strings.HasPrefix(name, "ok\t") || strings.HasPrefix(name, "ok  \t") || strings.HasPrefix(name, "?\t") || strings.HasPrefix(name, "?   \t") {
			continue
		}
		if name == "" && len(names) == 0 {
			continue
		}
		// Benchmarks are listed but never run by the admitted argv (no -bench).
		if strings.HasPrefix(name, "Benchmark") {
			continue
		}
		source, ok := names[name]
		if !ok {
			return fail("go-inventory-unparseable-list")
		}
		tests = append(tests, testrunner.InventoryTest{ID: p.config.Package + "/" + name, Path: source})
	}
	listed := map[string]bool{}
	for _, test := range tests {
		listed[strings.TrimPrefix(test.ID, p.config.Package+"/")] = true
	}
	for name := range names {
		if !listed[name] {
			return fail("go-inventory-declared-test-missing")
		}
	}
	return testrunner.CanonicalInventory(testrunner.RunnerInventory{Key: p.key, Tests: tests, Complete: true}, request.Key)
}

func (p *Plugin) Command(ctx context.Context, r registry.CommandRequest) (registry.CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return registry.CommandResponse{}, err
	}
	if r.Key != p.key || r.RunDir != p.config.Root {
		return registry.CommandResponse{}, fmt.Errorf("go-command-key-or-directory-invalid")
	}
	if p.config.Runner != "go-test" {
		return registry.CommandResponse{}, fmt.Errorf("ginkgo-spec-command-unavailable")
	}
	argv := []string{p.config.Executable}
	switch r.Purpose {
	case "inventory":
		argv = append(argv, "test", "-json", "-count=1", "-list=.")
	case "loaded":
		argv = append(argv, "list", "-json", "-deps", "-test")
	case "run", "collect":
		argv = append(argv, "test", "-json", "-count=1", "-timeout=30m")
	default:
		return registry.CommandResponse{}, fmt.Errorf("go-command-purpose-unsupported")
	}
	if len(p.config.Tags) > 0 {
		argv = append(argv, "-tags="+strings.Join(p.config.Tags, ","))
	}
	if p.config.Race {
		argv = append(argv, "-race")
	}
	if r.Purpose == "collect" {
		return registry.CommandResponse{}, fmt.Errorf("go-coverage-use-linecov-collector")
	}
	if r.Tests != nil && (r.Purpose == "run") {
		inventory, err := p.Inventory(ctx, registry.InventoryRequest{Key: p.key, Output: p.capture.List})
		if err != nil {
			return registry.CommandResponse{}, err
		}
		known := map[string]bool{}
		for _, t := range inventory.Tests {
			known[t.ID] = true
		}
		names := []string{}
		for _, test := range r.Tests {
			if !known[test] {
				return registry.CommandResponse{}, fmt.Errorf("go-command-test-unknown")
			}
			names = append(names, regexp.QuoteMeta(strings.TrimPrefix(test, p.config.Package+"/")))
		}
		pattern := "^$"
		if len(names) > 0 {
			pattern = "^(" + strings.Join(names, "|") + ")$"
		}
		argv = append(argv, "-run="+pattern)
	}
	pkg := "."
	if p.config.Package != "." {
		pkg = "./" + p.config.Package
	}
	argv = append(argv, pkg)
	env := map[string]string{}
	for k, v := range p.config.Environment {
		env[k] = v
	}
	return registry.CommandResponse{Argv: argv, Cwd: r.RunDir, Env: env}, nil
}

func (p *Plugin) Parse(ctx context.Context, r registry.ParseRequest) ([]testrunner.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.config.Runner || r.Runner != "go-test" {
		return nil, fmt.Errorf("go-result-runner-unsupported")
	}
	events, err := decodeEvents(r.Report, p.config.ImportPath, true)
	// Completed results survive a malformed or truncated suffix. The error
	// still forces reconciliation and cannot certify the report as complete.
	var results []testrunner.RunnerResult
	outputs := map[string]string{}
	var childFailures []string
	failedOwners := map[string]bool{}
	for _, event := range events {
		if event.Test == "" {
			continue
		}
		if owner, _, child := strings.Cut(event.Test, "/"); child {
			// RSP-V0-003: a child can prove failure, never completion of
			// its owning top-level test. Keep failures even without its terminal.
			if event.Action == "fail" && !failedOwners[owner] {
				childFailures = append(childFailures, owner)
				failedOwners[owner] = true
			}
			continue
		}
		if event.Action == "output" {
			outputs[event.Test] += event.Output
			continue
		}
		outcome := ""
		switch event.Action {
		case "pass":
			outcome = "passed"
		case "fail":
			outcome = "failed"
		case "skip":
			outcome = "skipped"
		}
		if outcome != "" {
			reason := ""
			if outcome == "skipped" {
				reason = goSkipReason(outputs[event.Test])
				if reason == "" {
					if err == nil {
						err = fmt.Errorf("go-skip-reason-missing")
					}
					continue
				}
			}
			results = append(results, testrunner.RunnerResult{ID: p.config.Package + "/" + event.Test, Outcome: outcome, Reason: reason})
		}
	}
	parentTerminals := map[string]bool{}
	for i := range results {
		owner := strings.TrimPrefix(results[i].ID, p.config.Package+"/")
		parentTerminals[owner] = true
		if failedOwners[owner] && results[i].Outcome != "failed" {
			results[i].Outcome, results[i].Reason = "failed", ""
			if err == nil {
				err = fmt.Errorf("go-output-package-or-order-invalid")
			}
		}
	}
	for _, owner := range childFailures {
		if !parentTerminals[owner] {
			results = append(results, testrunner.RunnerResult{ID: p.config.Package + "/" + owner, Outcome: "failed"})
			if err == nil {
				err = fmt.Errorf("go-output-truncated")
			}
		}
	}
	if err != nil {
		return results, err
	}
	if len(events) > 0 && events[len(events)-1].Action == "fail" {
		return results, fmt.Errorf("go-package-failed")
	}
	return results, nil
}

// Harness lifecycle output is not a runner-reported explanation of a skip.
// User log lines remain intact, including their source location when present.
func goSkipReason(output string) string {
	var reason []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "=== RUN ") || strings.HasPrefix(line, "=== PAUSE ") || strings.HasPrefix(line, "=== CONT ") || strings.HasPrefix(line, "=== NAME ") || strings.HasPrefix(line, "--- SKIP: ") {
			continue
		}
		if line != "" {
			reason = append(reason, line)
		}
	}
	return strings.Join(reason, "\n")
}

// Evidence exposes only native components. Core traces are joined by Assemble;
// the plugin has no store writer or trace-supply capability in this SPI.
func (p *Plugin) Evidence(ctx context.Context, r registry.EvidenceRequest) ([]registry.NativeEvidence, error) {
	if r.Runner != p.config.Runner || len(r.Artifacts) != 0 {
		return nil, fmt.Errorf("go-native-capture-required")
	}
	records, err := p.native(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]registry.NativeEvidence, 0, len(records))
	for _, record := range records {
		executed, e := json.Marshal(record.Executed)
		if e != nil {
			return nil, e
		}
		regions, e := json.Marshal(record.Regions)
		if e != nil {
			return nil, e
		}
		loaded, e := json.Marshal(record.Loaded)
		if e != nil {
			return nil, e
		}
		out = append(out, registry.NativeEvidence{Test: record.Test, Executed: executed, Regions: regions, Loaded: loaded})
	}
	return out, nil
}

func localPath(s string, root bool) bool {
	return s != "" && (root || s != ".") && path.Clean(s) == s && !path.IsAbs(s) && s != ".." && !strings.HasPrefix(s, "../") && !strings.ContainsAny(s, "\\:\x00\r\n")
}
func testName(name string) bool {
	if name == "TestMain" {
		return false
	}
	for _, prefix := range []string{"Test", "Example", "Fuzz"} {
		if suffix, ok := strings.CutPrefix(name, prefix); ok {
			for _, r := range suffix {
				return !unicode.IsLower(r)
			}
			return true
		}
	}
	return false
}

type goEvent struct{ Action, Package, Test, Output string }

func decodeEvents(raw, pkg string, allowFailure bool) ([]goEvent, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("go-output-missing-or-capped")
	}
	var events []goEvent
	capped := len(raw) > MaxArtifactBytes
	if capped {
		raw = raw[:MaxArtifactBytes]
	}
	fail := func(reason string) ([]goEvent, error) {
		if capped {
			reason = "go-output-missing-or-capped"
		}
		return events, fmt.Errorf("%s", reason)
	}
	d := json.NewDecoder(strings.NewReader(raw))
	terminal := false
	for {
		var event goEvent
		var frame json.RawMessage
		err := d.Decode(&frame)
		if err == io.EOF {
			break
		}
		if err == nil {
			err = jsonv2.Unmarshal(frame, &event)
		}
		if err != nil {
			return fail("go-output-malformed")
		}
		if event.Package != pkg || terminal {
			return fail("go-output-package-or-order-invalid")
		}
		switch event.Action {
		case "start", "run", "pause", "cont", "output":
		case "pass", "skip":
			if event.Test == "" {
				terminal = true
			}
		case "fail":
			if !allowFailure {
				return fail("go-run-failed")
			}
			if event.Test == "" {
				terminal = true
			}
		default:
			return fail("go-output-action-unknown")
		}
		events = append(events, event)
		if len(events) > 100000 {
			return fail("go-output-event-cap")
		}
	}
	if !terminal {
		return fail("go-output-truncated")
	}
	if capped {
		return fail("go-output-missing-or-capped")
	}
	return events, nil
}
