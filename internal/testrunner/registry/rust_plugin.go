// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/native"
)

// RustSource is a core-captured immutable source, not a path the plugin opens.
type RustSource struct {
	Blob testevidence.Blob
	Data []byte
}

// RustPluginOptions binds a compiled-in Rust runner to captured inputs. The core
// resolves each test's source (libtest and nextest listings lack source paths),
// supplies the complete tracked file universe and captures artifacts under its
// own bounds/containment. Artifact keys are opaque handles, never opened here.
// Each instance serves one run/process tree. Without qualified per-test markers
// its coverage is conservatively attributed to every Tests entry in that tree.
// Metadata is cargo metadata --format-version 1; BuildMessages is cargo's JSON
// build stream, including build-script-executed and a successful build-finished.
// BuildScripts holds each such script's captured stdout by package ID.
type RustPluginOptions struct {
	Key                     testrunner.InventoryKey
	Root, Manifest          string
	TestSources             map[string]string
	Sources                 map[string]RustSource // repository-relative names
	NativeSources           map[string]string     // native coverage name -> repository path
	Metadata, BuildMessages []byte
	BuildScripts            map[string][]byte
	Artifacts               map[string][]byte
	Tests                   []string
	SourcesComplete         bool
	CoverageComplete        bool // core observed all instrumented processes dump
	// CompilerEnvironment is the core-captured effective compiler environment.
	// Collection requires an explicit RUSTFLAGS or CARGO_ENCODED_RUSTFLAGS,
	// including an empty value. Config-only flags must first be resolved by
	// core into one of those variables; absent capture refuses collection.
	CompilerEnvironment map[string]string
}

type rustPlugin struct{ o RustPluginOptions }

// NewRustPlugin is the artifact-bound factory, like the script runner factory.
// It changes neither legacy Build/Parse nor the default absent declarations.
// Qualification is exclusively core-owned and remains absent/unqualified here.
func NewRustPlugin(o RustPluginOptions) (RunnerPlugin, error) {
	if o.Key.Runner != runnernames.RunnerCargoTest && o.Key.Runner != runnernames.RunnerNextest {
		return nil, fmt.Errorf("rust-runner-unsupported")
	}
	if !filepath.IsAbs(o.Root) || filepath.Clean(o.Root) != o.Root || !rustPath(o.Manifest) || path.Base(o.Manifest) != "Cargo.toml" {
		return nil, fmt.Errorf("rust-project-invalid")
	}
	if _, err := testrunner.CanonicalInventory(testrunner.RunnerInventory{Key: o.Key, Complete: true}, o.Key); err != nil {
		return nil, err
	}
	oid, err := hex.DecodeString(o.Key.Checkout)
	if err != nil || len(oid) != 20 && len(oid) != 32 || strings.ToLower(o.Key.Checkout) != o.Key.Checkout {
		return nil, fmt.Errorf("rust-checkout-not-pinned")
	}
	if len(o.TestSources) > testrunner.MaxTests || len(o.Sources) > 16384 || len(o.NativeSources) > 16384 || len(o.Artifacts) > testrunner.MaxReports || len(o.BuildScripts) > testrunner.MaxTests || len(o.Tests) > testrunner.MaxTests {
		return nil, fmt.Errorf("rust-input-cap")
	}
	// Copy all caller-owned maps/bytes: a later caller mutation cannot change
	// the evidence or checkout/configuration this factory captured.
	compilerEnvironment := make(map[string]string, 2)
	for _, name := range []string{"RUSTFLAGS", "CARGO_ENCODED_RUSTFLAGS"} {
		if value, ok := o.CompilerEnvironment[name]; ok {
			if len(value) > testrunner.MaxReportBytes || strings.ContainsRune(value, '\x00') {
				return nil, fmt.Errorf("rust-input-cap")
			}
			compilerEnvironment[name] = value
		}
	}
	o.CompilerEnvironment = compilerEnvironment
	totalBytes := len(o.Metadata) + len(o.BuildMessages)
	copyBytes := func(src map[string][]byte) (map[string][]byte, error) {
		out := make(map[string][]byte, len(src))
		for k, v := range src {
			if !rustText(k) || len(v) > testrunner.MaxReportBytes {
				return nil, fmt.Errorf("rust-artifact-invalid-or-capped")
			}
			totalBytes += len(v)
			if totalBytes > 256<<20 {
				return nil, fmt.Errorf("rust-input-aggregate-cap")
			}
			out[k] = bytes.Clone(v)
		}
		return out, nil
	}
	if len(o.Metadata) > testrunner.MaxReportBytes || len(o.BuildMessages) > testrunner.MaxReportBytes {
		return nil, fmt.Errorf("rust-build-input-cap")
	}
	o.Metadata, o.BuildMessages = bytes.Clone(o.Metadata), bytes.Clone(o.BuildMessages)
	if o.Artifacts, err = copyBytes(o.Artifacts); err != nil {
		return nil, err
	}
	if o.BuildScripts, err = copyBytes(o.BuildScripts); err != nil {
		return nil, err
	}
	sources := make(map[string]RustSource, len(o.Sources))
	for name, src := range o.Sources {
		if !rustPath(name) || src.Blob.Path.Lexical != name || src.Blob.Path.Resolved != name || len(src.Data) > testrunner.MaxReportBytes || !rustBlobMatches(src) {
			return nil, fmt.Errorf("rust-source-unbound:%s", name)
		}
		totalBytes += len(src.Data)
		if totalBytes > 256<<20 {
			return nil, fmt.Errorf("rust-input-aggregate-cap")
		}
		src.Data = bytes.Clone(src.Data)
		sources[name] = src
	}
	o.Sources = sources
	tests := make(map[string]string, len(o.TestSources))
	for id, name := range o.TestSources {
		if !rustText(id) || !strings.HasSuffix(name, ".rs") {
			return nil, fmt.Errorf("rust-test-source-invalid")
		}
		if _, ok := o.Sources[name]; !ok {
			return nil, fmt.Errorf("rust-test-source-missing:%s", id)
		}
		tests[id] = name
	}
	o.TestSources = tests
	mapping := make(map[string]string, len(o.NativeSources))
	for native, name := range o.NativeSources {
		if !rustText(native) || !rustPath(name) {
			return nil, fmt.Errorf("rust-native-source-invalid")
		}
		if _, ok := o.Sources[name]; !ok {
			return nil, fmt.Errorf("rust-native-source-missing")
		}
		mapping[native] = name
	}
	o.NativeSources = mapping
	seen := map[string]bool{}
	for _, id := range o.Tests {
		if _, ok := tests[id]; !ok || seen[id] {
			return nil, fmt.Errorf("rust-attribution-invalid")
		}
		seen[id] = true
	}
	o.Tests = append([]string(nil), o.Tests...)
	sort.Strings(o.Tests)
	return &rustPlugin{o}, nil
}

func rustPath(s string) bool {
	return rustText(s) && s != "." && s != ".." && path.Clean(s) == s && !strings.HasPrefix(s, "/") && !strings.HasPrefix(s, "../") && !strings.ContainsAny(s, "\\:")
}

func rustText(s string) bool {
	return s != "" && len(s) <= 4096 && strings.TrimSpace(s) == s && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}

func rustBlobMatches(src RustSource) bool {
	r := testevidence.New("unused", "unused", "unused")
	r.Loaded = testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{src.Blob}}
	if r.ValidateComponents() != nil {
		return false
	}
	prefix := []byte("blob " + strconv.Itoa(len(src.Data)) + "\x00")
	if len(src.Blob.OID) == 40 {
		h := sha1.New()
		h.Write(prefix)
		h.Write(src.Data)
		return hex.EncodeToString(h.Sum(nil)) == src.Blob.OID
	}
	h := sha256.New()
	h.Write(prefix)
	h.Write(src.Data)
	return hex.EncodeToString(h.Sum(nil)) == src.Blob.OID
}

func (*rustPlugin) Pieces() testrunner.RunnerPieces {
	return testrunner.RunnerPieces{
		Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true},
		Qualification: testrunner.RunnerPiece{Reason: "unqualified"},
	}
}

func (p *rustPlugin) Inventory(ctx context.Context, req InventoryRequest) (testrunner.RunnerInventory, error) {
	fail := func(err error) (testrunner.RunnerInventory, error) {
		return testrunner.RunnerInventory{Key: req.Key, Reason: err.Error()}, err
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if req.Key != p.o.Key {
		return fail(fmt.Errorf("rust-inventory-key-mismatch"))
	}
	if err := p.libraryScope(); err != nil {
		return fail(err)
	}
	if len(req.Output) == 0 || len(req.Output) > testrunner.MaxReportBytes || !utf8.ValidString(req.Output) {
		return fail(fmt.Errorf("rust-inventory-missing-malformed-or-capped"))
	}
	var ids []string
	var err error
	if p.o.Key.Runner == runnernames.RunnerCargoTest {
		ids, err = rustCargoList(req.Output)
	} else {
		ids, err = rustNextestList([]byte(req.Output))
	}
	if err != nil {
		return fail(err)
	}
	in := testrunner.RunnerInventory{Key: req.Key, Complete: true}
	for _, id := range ids {
		name, ok := p.o.TestSources[id]
		if !ok {
			return fail(fmt.Errorf("rust-inventory-source-unresolved:%s", id))
		}
		in.Tests = append(in.Tests, testrunner.InventoryTest{ID: id, Path: name})
	}
	in, err = testrunner.CanonicalInventory(in, req.Key)
	if err != nil {
		return fail(err)
	}
	return in, nil
}

// Stable libtest lists one explicitly selected --lib target. A mandatory final
// count detects truncation; compiler diagnostics belong to separately captured
// stderr. Benchmarks/custom harnesses and multiple target streams are unsupported.
func rustCargoList(output string) ([]string, error) {
	var ids []string
	done := false
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		if done {
			return nil, fmt.Errorf("rust-inventory-trailing-output")
		}
		if id, ok := strings.CutSuffix(line, ": test"); ok {
			if !rustText(id) || len(ids) >= testrunner.MaxTests {
				return nil, fmt.Errorf("rust-inventory-test-invalid-or-capped")
			}
			ids = append(ids, id)
			continue
		}
		count, tail, ok := strings.Cut(line, " ")
		n, err := strconv.Atoi(count)
		if !ok || err != nil || strconv.Itoa(n) != count || n != len(ids) || tail != "tests, 0 benchmarks" && tail != "test, 0 benchmarks" {
			return nil, fmt.Errorf("rust-inventory-unparseable-or-truncated")
		}
		done = true
	}
	if !done {
		return nil, fmt.Errorf("rust-inventory-summary-missing")
	}
	return ids, nil
}

func rustNextestList(data []byte) ([]string, error) {
	var list struct {
		BuildMeta json.RawMessage `json:"rust-build-meta"`
		Count     *int            `json:"test-count"`
		Suites    map[string]struct {
			PackageName   string `json:"package-name"`
			BinaryName    string `json:"binary-name"`
			PackageID     string `json:"package-id"`
			BinaryPath    string `json:"binary-path"`
			BuildPlatform string `json:"build-platform"`
			Cwd           string `json:"cwd"`
			BinaryID      string `json:"binary-id"`
			Kind          string `json:"kind"`
			Status        string `json:"status"`
			Cases         map[string]struct {
				Kind    *string `json:"kind"`
				Ignored *bool   `json:"ignored"`
				Match   struct {
					Status string `json:"status"`
					Reason string `json:"reason"`
				} `json:"filter-match"`
			} `json:"testcases"`
		} `json:"rust-suites"`
	}
	// The build metadata is opaque here; it does not decide inventory. Unknown
	// test/suite fields, duplicate keys and invalid UTF-8 refuse discovery.
	if err := jsonv2.Unmarshal(data, &list, jsonv2.RejectUnknownMembers(true)); err != nil || list.Count == nil || *list.Count < 0 || *list.Count > testrunner.MaxTests || list.Suites == nil || len(list.Suites) > testrunner.MaxTests {
		return nil, fmt.Errorf("rust-nextest-inventory-malformed-or-capped")
	}
	var ids []string
	for key, suite := range list.Suites {
		if (suite.Kind != "lib" && suite.Kind != "bin" && suite.Kind != "test") || key != suite.BinaryID || !rustText(key) || suite.Status != "listed" || suite.Cases == nil || len(suite.Cases) > testrunner.MaxTests {
			return nil, fmt.Errorf("rust-nextest-suite-unlisted-or-invalid")
		}
		for name, test := range suite.Cases {
			if !rustText(name) || test.Ignored == nil || test.Match.Status != "matches" || test.Kind != nil && *test.Kind != "test" {
				return nil, fmt.Errorf("rust-nextest-test-unlisted-or-unsupported")
			}
			ids = append(ids, key+"::"+name)
			if len(ids) > testrunner.MaxTests {
				return nil, fmt.Errorf("rust-inventory-cap")
			}
		}
	}
	if len(ids) != *list.Count {
		return nil, fmt.Errorf("rust-nextest-inventory-count-mismatch")
	}
	sort.Strings(ids)
	return ids, nil
}

func (p *rustPlugin) Command(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return CommandResponse{}, err
	}
	if req.Key != p.o.Key {
		return CommandResponse{}, fmt.Errorf("rust-command-key-mismatch")
	}
	if req.Tests != nil && len(req.Tests) == 0 {
		return CommandResponse{}, fmt.Errorf("rust-command-empty-run-set")
	}
	if len(req.Tests) > testrunner.MaxTests {
		return CommandResponse{}, fmt.Errorf("rust-command-test-cap")
	}
	for _, id := range req.Tests {
		if _, ok := p.o.TestSources[id]; !ok {
			return CommandResponse{}, fmt.Errorf("rust-command-test-unknown")
		}
	}
	if err := p.libraryScope(); err != nil {
		return CommandResponse{}, err
	}
	cmd := CommandResponse{Cwd: p.o.Root}
	if req.Purpose == "inventory" {
		if req.Tests != nil {
			return CommandResponse{}, fmt.Errorf("rust-inventory-filter-unsupported")
		}
		if p.o.Key.Runner == runnernames.RunnerCargoTest {
			cmd.Argv = []string{"cargo", "test", "--offline", "--locked", "--manifest-path", p.o.Manifest, "--lib", "--", "--list", "--format=pretty", "--color=never"}
		} else {
			cmd.Argv = []string{"cargo", runnernames.RunnerNextest, "list", "--offline", "--locked", "--manifest-path", p.o.Manifest, "--message-format", "json"}
		}
		return cmd, nil
	}
	if req.Purpose != "run" && req.Purpose != "collect" {
		return CommandResponse{}, fmt.Errorf("rust-command-purpose-unsupported")
	}
	// The SPI describes one command. Multi-test cargo filters and nextest's
	// expression grammar cannot encode every identity; run the entire runner
	// as a declared superset rather than risk a subset or interpolate a filter.
	if p.o.Key.Runner == runnernames.RunnerCargoTest {
		cmd.Argv = []string{"cargo", "test", "--offline", "--locked", "--manifest-path", p.o.Manifest, "--lib", "--", "--format=pretty", "--color=never", "--test-threads=1"}
		if len(req.Tests) == 1 && !strings.HasPrefix(req.Tests[0], "-") {
			cmd.Argv = append(cmd.Argv, "--exact", req.Tests[0])
		} else {
			cmd.Superset = req.Tests != nil
		}
	} else {
		if !filepath.IsAbs(req.RunDir) || filepath.Clean(req.RunDir) != req.RunDir {
			return CommandResponse{}, fmt.Errorf("rust-run-directory-invalid")
		}
		inv, err := native.Build(testrunner.Request{Runner: runnernames.RunnerNextest, Executable: "cargo", Project: p.o.Manifest, ReportDir: req.RunDir})
		if err != nil {
			return CommandResponse{}, err
		}
		cmd.Argv = append([]string{"cargo"}, inv.Argv...)
		cmd.Artifacts = inv.ReportPaths
		cmd.Superset = req.Tests != nil
	}
	if req.Purpose == "collect" {
		if !filepath.IsAbs(req.RunDir) || filepath.Clean(req.RunDir) != req.RunDir || strings.ContainsAny(req.RunDir, "%\x00\r\n") {
			return CommandResponse{}, fmt.Errorf("rust-collection-directory-invalid")
		}
		flags, encoded := p.o.CompilerEnvironment["CARGO_ENCODED_RUSTFLAGS"]
		if !encoded {
			plain, captured := p.o.CompilerEnvironment["RUSTFLAGS"]
			if !captured {
				return CommandResponse{}, fmt.Errorf("rust-command-purpose-unsupported")
			}
			flags = strings.Join(strings.Fields(plain), "\x1f")
		}
		if flags != "" {
			flags += "\x1f"
		}
		flags += "-C\x1finstrument-coverage"
		// Encoded flags win over both inherited environment and Cargo config.
		// Pin them even when the captured effective flags were empty.
		cmd.Env = map[string]string{"RUSTFLAGS": "-C instrument-coverage", "CARGO_ENCODED_RUSTFLAGS": flags, "LLVM_PROFILE_FILE": filepath.Join(req.RunDir, "coverage-%p-%m.profraw")}
		cmd.Artifacts = append(cmd.Artifacts, filepath.Join(req.RunDir, "coverage-*.profraw"))
	}
	return cmd, nil
}

func (p *rustPlugin) Parse(ctx context.Context, req ParseRequest) ([]testrunner.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Runner != p.o.Key.Runner || len(req.Report) == 0 || len(req.Report) > testrunner.MaxReportBytes || !utf8.ValidString(req.Report) {
		return nil, fmt.Errorf("rust-report-invalid-or-capped")
	}
	input := testrunner.Input{Runner: req.Runner, Stdout: []byte(req.Report)}
	if req.Runner == runnernames.RunnerNextest {
		input.Reports = map[string][]byte{"nextest.xml": []byte(req.Report)}
		input.Stdout = nil
	}
	o, nativeErr := native.Parse(input)
	skipReasons := map[string]string{}
	if req.Runner == runnernames.RunnerNextest {
		skipReasons = rustSkipReasons(req.Report)
	}
	var results []testrunner.RunnerResult
	for _, test := range o.Tests {
		r := testrunner.RunnerResult{ID: test.ID}
		switch test.State {
		case testrunner.Passed:
			r.Outcome = "passed"
		case testrunner.Failed:
			r.Outcome = "failed"
		case testrunner.Skipped:
			r.Outcome = "skipped"
			r.Reason = skipReasons[test.ID]
			// Stable libtest itself reports 'ignored'; preserve any explanation.
			for _, line := range strings.Split(req.Report, "\n") {
				if s, ok := strings.CutPrefix(line, "test "+test.ID+" ... ignored"); ok {
					r.Reason = "ignored" + s
					break
				}
			}
		default:
			return results, errors.Join(nativeErr, fmt.Errorf("rust-report-nonterminal"))
		}
		results = append(results, r)
	}
	// ParseRequest intentionally contains no process exit code. Executor errors
	// are reconciled by the core. The legacy parser's synthetic zero-exit
	// comparison cannot add or remove a native terminal result here.
	for _, problem := range o.Problems {
		if problem.Code != "EXIT_REPORT_CONTRADICTION" {
			return results, errors.Join(nativeErr, fmt.Errorf("rust-report-incomplete:%s", problem.Code))
		}
	}
	return results, nativeErr
}

// Ensure the implementation remains on the common SPI.
var _ RunnerPlugin = (*rustPlugin)(nil)

// Marshal only the existing native component shape, never a Rust-specific wire.
func rustNative(test string, n testevidence.Native) NativeEvidence {
	executed, _ := json.Marshal(n.Executed)
	regions, _ := json.Marshal(n.Regions)
	loaded, _ := json.Marshal(n.Loaded)
	return NativeEvidence{Test: test, Executed: executed, Regions: regions, Loaded: loaded}
}

// RustRunFiles returns the existing native nextest profile's files for the core
// to materialize in its private RunDir before Command execution. It never writes
// them itself. Cargo needs no files. Keeping this separate preserves the frozen
// command SPI and the existing native profile's report/configuration shape.
func RustRunFiles(runner, manifest, runDir string) (map[string][]byte, error) {
	if runner == runnernames.RunnerCargoTest {
		return nil, nil
	}
	if runner != runnernames.RunnerNextest || !rustPath(manifest) || !filepath.IsAbs(runDir) || filepath.Clean(runDir) != runDir {
		return nil, fmt.Errorf("rust-run-files-invalid")
	}
	inv, err := native.Build(testrunner.Request{Runner: runner, Executable: "cargo", Project: manifest, ReportDir: runDir})
	return inv.Files, err
}

// The existing native parser validates XML structure/counts but intentionally
// keeps only failure messages. Preserve runner-provided skip reasons for the
// selection SPI's stricter terminal-result contract.
func rustSkipReasons(report string) map[string]string {
	out := map[string]string{}
	d := xml.NewDecoder(strings.NewReader(report))
	id := ""
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		} // native.Parse already validated this report
		if start, ok := token.(xml.StartElement); ok {
			if start.Name.Local == "testcase" {
				name, class := "", ""
				for _, a := range start.Attr {
					if a.Name.Local == "name" {
						name = a.Value
					}
					if a.Name.Local == "classname" {
						class = a.Value
					}
				}
				id = class + "::" + name
			}
			if start.Name.Local == "skipped" {
				var skip struct {
					Message string `xml:"message,attr"`
					Text    string `xml:",chardata"`
				}
				if d.DecodeElement(&skip, &start) == nil {
					out[id] = strings.TrimSpace(skip.Message + " " + skip.Text)
				}
			}
		}
	}
	return out
}

// The libtest parser binds one library target. Only a library and build scripts
// are understood; every other target shape leaves the FULL universe unproven.
func (p *rustPlugin) libraryScope() error {
	if p.o.Key.Runner != runnernames.RunnerCargoTest {
		return nil
	}
	if !p.o.SourcesComplete {
		return fmt.Errorf("rust-library-target-universe-incomplete")
	}
	for name := range p.o.Sources {
		if strings.HasSuffix(name, ".rs") && (strings.HasPrefix(name, "tests/") || strings.Contains(name, "/tests/")) {
			return fmt.Errorf("rust-integration-tests-unsupported")
		}
	}
	var metadata struct {
		Version  int           `json:"version"`
		Packages []rustPackage `json:"packages"`
	}
	if err := jsonv2.Unmarshal(p.o.Metadata, &metadata); err != nil || metadata.Version != 1 || len(metadata.Packages) == 0 {
		return fmt.Errorf("rust-library-target-universe-incomplete")
	}
	libraries := 0
	for _, pkg := range metadata.Packages {
		if pkg.Source != nil {
			continue
		}
		if len(pkg.Targets) == 0 {
			return fmt.Errorf("rust-library-target-universe-incomplete")
		}
		for _, target := range pkg.Targets {
			for _, kind := range target.Kind {
				if kind == "test" {
					return fmt.Errorf("rust-integration-tests-unsupported")
				}
			}
			if len(target.Kind) != 1 {
				return fmt.Errorf("rust-library-target-universe-incomplete")
			}
			switch target.Kind[0] {
			case "lib":
				libraries++
			case "custom-build":
			default:
				return fmt.Errorf("rust-library-target-universe-incomplete")
			}
		}
	}
	if libraries != 1 {
		return fmt.Errorf("rust-library-target-universe-incomplete")
	}
	return nil
}
