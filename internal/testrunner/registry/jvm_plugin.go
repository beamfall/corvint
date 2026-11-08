// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/Beamfall/corvint/internal/langplugin/runnernames"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testevidence"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/platform"
)

// JVMSourceSet is a core-captured build-graph observation for one native test
// task. Complete means task type, source-set membership, compile classpath and
// class-to-tracked-source mappings were all resolved for Key. A Gradle aggregate
// is never a native Test task. Surefire uses the Maven test source set.
// These are in-process inputs, not an extension of the frozen runner wires.
type JVMSourceSet struct {
	Name       string
	Task       string
	NativeTest bool
	Complete   bool
	Reason     string
	Classes    map[string]testevidence.Blob
	Classpath  []testevidence.Blob
	Sources    map[string]testevidence.Blob // JaCoCo package/sourcefile names
}

// JVMSelectionConfig binds immutable core observations to a single runner/task.
// Console is an independently pinned ConsoleLauncher jar. DiscoveryClasspath is
// the exact test runtime classpath captured by the build graph, not a guessed
// source directory. Native coverage is captured separately for each test tree.
type JVMSelectionConfig struct {
	Key                tr.InventoryKey
	Request            tr.Request
	Console            tr.Tool
	Agent              tr.Tool // optional independently pinned JaCoCo agent for collect
	DiscoveryClasspath []string
	SourceSet          JVMSourceSet
	Captures           map[string]JVMCapture
}

type jvmSelectionPlugin struct {
	config          JVMSelectionConfig
	javaToolOptions string
}

// NewJVMSelectionPlugin extends the existing declarations rather than adding
// another runner. The core must stage the returned invocation's Files before
// executing Command; this factory and the SPI perform no I/O or process launch.
// The configured factory mirrors ScriptPlugins.Plugin: SelectionPlugin's
// unconfigured legacy lookup remains absent. No qualification is manufactured.
func NewJVMSelectionPlugin(c JVMSelectionConfig) (RunnerPlugin, tr.Invocation, error) {
	fail := func(reason string) (RunnerPlugin, tr.Invocation, error) {
		return nil, tr.Invocation{}, fmt.Errorf("jvm-selection:%s", reason)
	}
	if c.Key.Runner != c.Request.Runner || (c.Key.Runner != runnernames.RunnerGradleJunit && c.Key.Runner != runnernames.RunnerMavenSurefire && c.Key.Runner != runnernames.RunnerJunitPlatform) {
		return fail("runner-unsupported")
	}
	if _, err := hex.DecodeString(c.Key.Checkout); err != nil || (len(c.Key.Checkout) != 40 && len(c.Key.Checkout) != 64) || c.Key.Checkout != strings.ToLower(c.Key.Checkout) {
		return fail("checkout-not-immutable")
	}
	if _, err := tr.CanonicalInventory(tr.RunnerInventory{Key: c.Key, Complete: true}, c.Key); err != nil {
		return fail(err.Error())
	}
	if len(c.Request.Selectors) != 0 || c.Request.ExpectedSelection != nil || len(c.Request.ExpectedTests) != 0 {
		return fail("unfiltered-request-required")
	}
	for _, name := range []string{c.Request.Root, c.Request.ReportDir, c.Request.Executable, c.Console.Executable} {
		if !filepath.IsAbs(name) || filepath.Clean(name) != name || strings.ContainsAny(name, "\x00\r\n") {
			return fail("tool-root-or-staging-path-invalid")
		}
	}
	s := c.SourceSet
	if !s.Complete || s.Reason != "" || s.Name == "" || !s.NativeTest || s.Task != c.Request.Target || len(s.Classes) > tr.MaxTests || len(s.Sources) > tr.MaxTests || len(s.Classpath) > tr.MaxTests {
		return fail("source-set-incomplete-or-not-native-test")
	}
	if c.Key.Runner != runnernames.RunnerGradleJunit && s.Name != "test" {
		return fail("source-set-unsupported")
	}
	for class, blob := range s.Classes {
		if !jvmClass.MatchString(class) || !jvmBlob(blob) || (filepath.Ext(blob.Path.Resolved) != ".java" && filepath.Ext(blob.Path.Resolved) != ".kt") {
			return fail("class-source-unresolved")
		}
	}
	for source, blob := range s.Sources {
		if !jvmRelative(source) || !jvmBlob(blob) || (filepath.Ext(blob.Path.Resolved) != ".java" && filepath.Ext(blob.Path.Resolved) != ".kt") {
			return fail("coverage-source-unresolved")
		}
	}
	for _, blob := range s.Classpath {
		if !jvmBlob(blob) {
			return fail("compile-classpath-unresolved")
		}
	}
	if !filepath.IsAbs(c.Console.Executable) || !jvmDigest(c.Console.Sha256) || len(c.DiscoveryClasspath) == 0 || len(c.DiscoveryClasspath) > tr.MaxTests {
		return fail("console-or-runtime-classpath-unresolved")
	}
	for _, name := range c.DiscoveryClasspath {
		if !filepath.IsAbs(name) || strings.ContainsAny(name, "\x00\r\n"+string(filepath.ListSeparator)) {
			return fail("runtime-classpath-invalid")
		}
	}
	if c.Key.Runner == runnernames.RunnerJunitPlatform && (c.Request.Reporter != c.Console.Executable || c.Request.ReporterSha256 != c.Console.Sha256) {
		return fail("console-pin-mismatch")
	}
	if c.Key.Runner == runnernames.RunnerJunitPlatform && (len(c.DiscoveryClasspath) != 1 || c.DiscoveryClasspath[0] != filepath.Join(c.Request.Root, c.Request.Project)) {
		return fail("console-discovery-classpath-mismatch")
	}
	if c.Agent.Executable != "" && (!filepath.IsAbs(c.Agent.Executable) || !jvmDigest(c.Agent.Sha256) || strings.ContainsAny(c.Agent.Executable, ",\"\x00\r\n")) {
		return fail("jacoco-agent-invalid")
	}
	// Copy the complete input, including nested artifact slices and maps. Caller
	// mutation after admission cannot silently change the plugin's observations.
	raw, err := json.Marshal(c)
	if err != nil || len(raw) > testevidence.MaxRecordBytes {
		return fail("capture-bound")
	}
	// Decode into an empty value: decoding into c reuses caller-owned slices
	// and map entries, defeating the artifact snapshot.
	c = JVMSelectionConfig{}
	if err := json.Unmarshal(raw, &c); err != nil {
		return fail("capture-copy-failed")
	}
	inv, err := tr.BuildWith(c.Request, nil, platform.Build)
	if err != nil {
		return fail(err.Error())
	}
	return &jvmSelectionPlugin{config: c, javaToolOptions: os.Getenv("JAVA_TOOL_OPTIONS")}, inv, nil
}

func (p *jvmSelectionPlugin) Pieces() tr.RunnerPieces {
	return tr.RunnerPieces{Inventory: tr.RunnerPiece{Present: true}, TestEvidence: tr.RunnerPiece{Present: true}, Qualification: tr.RunnerPiece{Reason: "unqualified"}}
}

func (p *jvmSelectionPlugin) Inventory(ctx context.Context, r InventoryRequest) (tr.RunnerInventory, error) {
	if err := ctx.Err(); err != nil {
		return tr.RunnerInventory{Reason: "jvm-inventory-cancelled"}, err
	}
	if r.Key != p.config.Key {
		return tr.RunnerInventory{Reason: "jvm-inventory-key-mismatch"}, fmt.Errorf("jvm-inventory-key-mismatch")
	}
	if r.Key.Runner == runnernames.RunnerGradleJunit {
		return jvmGradleInventory(r.Key, r.Output, p.config.SourceSet.Classes)
	}
	return jvmInventory(r.Key, r.Output, p.config.SourceSet.Classes)
}

func (p *jvmSelectionPlugin) Command(ctx context.Context, r CommandRequest) (CommandResponse, error) {
	if err := ctx.Err(); err != nil {
		return CommandResponse{}, err
	}
	if r.Key != p.config.Key || r.RunDir != p.config.Request.ReportDir {
		return CommandResponse{}, fmt.Errorf("jvm-command-key-or-staging-mismatch")
	}
	inv, err := tr.BuildWith(p.config.Request, nil, platform.Build)
	if err != nil {
		return CommandResponse{}, err
	}
	if r.Purpose == "inventory" && r.Key.Runner == runnernames.RunnerGradleJunit {
		// Gradle 8.3+ dry-run reports use the native task's actual candidates,
		// engines, tags and filters. Console scanning cannot reproduce those.
		// The host stages inv.Files, requires a successful uncapped process and
		// supplies all fresh XML artifacts as Output, never the console log.
		argv := append([]string{p.config.Request.Executable}, inv.Argv...)
		argv = append(argv, "--test-dry-run")
		return CommandResponse{Argv: argv, Cwd: p.config.Request.Root, Env: inv.Environment, Artifacts: inv.ReportPatterns}, nil
	}
	if r.Purpose == "inventory" {
		java := p.config.Request.Executable
		if r.Key.Runner != runnernames.RunnerJunitPlatform {
			java = p.config.Request.Tools[runnernames.LanguageJava].Executable
		}
		argv := []string{java, "-jar", p.config.Console.Executable, "discover", "--disable-banner", "--disable-ansi-colors", "--details=flat", "--class-path", strings.Join(p.config.DiscoveryClasspath, string(filepath.ListSeparator)), "--scan-class-path"}
		if r.Key.Runner == runnernames.RunnerJunitPlatform {
			argv = append(argv, "--include-engine", "junit-jupiter")
		}
		return CommandResponse{Argv: argv, Cwd: p.config.Request.Root}, nil
	}
	if r.Purpose != "run" && r.Purpose != "collect" {
		return CommandResponse{}, fmt.Errorf("jvm-command-purpose-unsupported")
	}
	if r.Tests != nil && (len(r.Tests) == 0 || len(r.Tests) > tr.MaxTests) {
		return CommandResponse{}, fmt.Errorf("jvm-command-empty-run-set")
	}
	// Console IDs and build-tool IDs differ, and parameterized/dynamic filters
	// are not generally exact. Omit filters and report the FULL superset. Core
	// reconciliation still owes every planned identity a terminal result.
	argv := append([]string{p.config.Request.Executable}, inv.Argv...)
	env := make(map[string]string, len(inv.Environment))
	for k, v := range inv.Environment {
		env[k] = v
	}
	artifacts := append(append([]string{}, inv.ReportPaths...), inv.ReportPatterns...)
	if r.Purpose == "collect" {
		if p.config.Agent.Executable == "" || strings.ContainsAny(r.RunDir, ",\"\x00\r\n") {
			return CommandResponse{}, fmt.Errorf("jvm-collect-pinned-agent-required")
		}
		env["JAVA_TOOL_OPTIONS"] = `"-javaagent:` + p.config.Agent.Executable + `=destfile=` + filepath.Join(r.RunDir, "jacoco.exec") + `,append=true,dumponexit=true,output=file"`
		if p.javaToolOptions != "" {
			env["JAVA_TOOL_OPTIONS"] = p.javaToolOptions + " " + env["JAVA_TOOL_OPTIONS"]
		}
		artifacts = append(artifacts, "jacoco.exec")
	}
	return CommandResponse{Argv: argv, Cwd: p.config.Request.Root, Env: env, Artifacts: artifacts, Superset: r.Tests != nil}, nil
}

func (p *jvmSelectionPlugin) Parse(ctx context.Context, r ParseRequest) ([]tr.RunnerResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.config.Key.Runner || len(r.Report) > tr.MaxReportBytes {
		return nil, fmt.Errorf("jvm-report-runner-or-bound")
	}
	// Report contains native XML bytes, including a native testsuites aggregate.
	suites, err := jvmSuites(r.Report)
	out := []tr.RunnerResult{}
	seen := map[string]bool{}
	for _, suite := range suites {
		name := "TEST-captured.xml"
		if r.Runner == runnernames.RunnerJunitPlatform {
			name = "TEST-junit-jupiter.xml"
		}
		in := tr.Input{Runner: r.Runner, Reports: map[string][]byte{name: []byte(suite)}}
		// Exit consistency belongs to the executor. Only choose the matching
		// parser view here; no passing process outcome is inferred from XML.
		var observation tr.Observation
		var parseErr error
		for _, code := range []int{0, 1} {
			in.ExitCode = code
			observation, parseErr = platform.Parse(in)
			if parseErr != nil || observation.Complete {
				break
			}
		}
		for _, test := range observation.Tests {
			state := map[string]string{tr.Passed: "passed", tr.Failed: "failed", tr.Skipped: "skipped"}[test.State]
			out = append(out, tr.RunnerResult{ID: test.ID, Outcome: state, Reason: test.Message})
			if seen[test.ID] || len(out) > tr.MaxTests || state == "" || (state == "skipped" && strings.TrimSpace(test.Message) == "") {
				return out, fmt.Errorf("jvm-result-duplicate-capped-or-nonterminal")
			}
			seen[test.ID] = true
		}
		if parseErr != nil || !observation.Complete {
			return out, fmt.Errorf("jvm-native-report-incomplete:%v", parseErr)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, err
}

// Split a native testsuites aggregate without rewriting its suite bytes or
// relaxing either existing native schema parser. The complete wrapper and every
// child must be well formed; a truncated last suite never hides missing tests.
func jvmSuites(raw string) ([]string, error) {
	out := []string{}
	// A bad suffix invalidates the envelope, not failures already observed in
	// complete suites. Parse validates each retained suite before returning it.
	fail := func() ([]string, error) { return out, fmt.Errorf("jvm-native-suite-envelope-invalid") }
	d := xml.NewDecoder(strings.NewReader(raw))
	started, ended := false, false
	for {
		offset := d.InputOffset()
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fail()
		}
		switch t := token.(type) {
		case xml.StartElement:
			if ended || t.Name.Space != "" {
				return fail()
			}
			if !started {
				if t.Name.Local == "testsuite" {
					return []string{raw}, nil
				}
				if t.Name.Local != "testsuites" || len(t.Attr) != 0 {
					return fail()
				}
				started = true
			} else {
				if t.Name.Local != "testsuite" || len(out) >= tr.MaxReports {
					return fail()
				}
				if err := d.Skip(); err != nil {
					return fail()
				}
				out = append(out, raw[offset:d.InputOffset()])
			}
		case xml.EndElement:
			if !started || ended || t.Name.Local != "testsuites" {
				return fail()
			}
			ended = true
		case xml.CharData:
			if strings.TrimSpace(string(t)) != "" {
				return fail()
			}
		case xml.ProcInst:
			if started || t.Target != "xml" {
				return fail()
			}
		case xml.Comment:
		default:
			return fail()
		}
	}
	if !started || !ended || len(out) == 0 {
		return fail()
	}
	return out, nil
}

func (p *jvmSelectionPlugin) Evidence(ctx context.Context, r EvidenceRequest) ([]NativeEvidence, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Runner != p.config.Key.Runner || len(r.Artifacts) != 0 {
		return nil, fmt.Errorf("jvm-evidence-requires-core-captured-artifacts")
	}
	ids := make([]string, 0, len(p.config.Captures))
	for id := range p.config.Captures {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 || len(ids) > tr.MaxTests {
		return nil, fmt.Errorf("jvm-evidence-captures-missing-or-capped")
	}
	out := make([]NativeEvidence, 0, len(ids))
	bytes := 0
	for _, id := range ids {
		capture := p.config.Captures[id]
		native := jvmMissing("jvm-capture-key-or-attribution-mismatch")
		seen := map[string]bool{}
		valid := capture.Key == p.config.Key && len(capture.Tests) > 0 && len(capture.Tests) <= tr.MaxTests
		for _, test := range capture.Tests {
			valid = valid && jvmKnownTest(p.config, test) && !seen[test]
			seen[test] = true
			// Shared trees carry the same union on every attributed test. A
			// plugin cannot invent per-test precision by splitting one tree.
			valid = valid && tr.Identity(p.config.Captures[test]) == tr.Identity(capture)
		}
		if valid && seen[id] {
			native = jvmEvidence(p.config.SourceSet, capture)
		}
		e, _ := json.Marshal(native.Executed)
		r, _ := json.Marshal(native.Regions)
		l, _ := json.Marshal(native.Loaded)
		bytes += len(id) + len(e) + len(r) + len(l)
		if bytes > testevidence.MaxRecordBytes {
			return nil, fmt.Errorf("jvm-native-evidence-cap")
		}
		out = append(out, NativeEvidence{Test: id, Executed: e, Regions: r, Loaded: l})
	}
	return out, nil
}

func jvmKnownTest(c JVMSelectionConfig, id string) bool {
	class := ""
	if c.Key.Runner == runnernames.RunnerJunitPlatform {
		match := jvmMethodID.FindStringSubmatch(id)
		if match == nil {
			return false
		}
		class = match[1]
	} else {
		var method string
		class, method, _ = strings.Cut(id, "#")
		if !jvmClass.MatchString(method) || strings.Contains(method, ".") {
			return false
		}
	}
	return jvmBlob(c.SourceSet.Classes[class])
}

var _ RunnerPlugin = (*jvmSelectionPlugin)(nil)
