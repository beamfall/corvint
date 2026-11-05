//go:build linux

// SPDX-License-Identifier: Apache-2.0
package nativelinecov

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// TestNativeLiveCMocka is opt-in: tools are installed and checksum-verified by
// the operator. Native output and event-driven ptrace observations are retained
// outside the repository, never substituted with fixture XML or process IDs.
func TestNativeLiveCMocka(t *testing.T) {
	t.Run("LPCV-V0-9400 native inventories", func(t *testing.T) {
		tools, evidence := os.Getenv("CORVINT_NATIVE_LIVE_TOOLS"), os.Getenv("CORVINT_NATIVE_LIVE_EVIDENCE")
		if tools == "" || evidence == "" {
			t.Skip("operator-supplied native toolchains and evidence directory required")
		}
		ctx := context.Background()
		run := func(name string, args ...string) []byte {
			t.Helper()
			c := exec.CommandContext(ctx, name, args...)
			b, e := c.CombinedOutput()
			if e != nil {
				t.Fatalf("%s %v: %v\n%s", name, args, e, b)
			}
			return b
		}
		if err := os.MkdirAll(evidence, 0700); err != nil {
			t.Fatal(err)
		}
		evidence, err := os.MkdirTemp(evidence, "cmocka-")
		if err != nil {
			t.Fatal(err)
		}
		source, err := filepath.Abs("testdata/live")
		if err != nil {
			t.Fatal(err)
		}
		originalSource := source
		source = t.TempDir()
		for _, name := range []string{"CMakeLists.txt", "probe.c", "child.c"} {
			b, e := os.ReadFile(filepath.Join(originalSource, name))
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(source, name), b, 0600); e != nil {
				t.Fatal(e)
			}
		}
		build := filepath.Join(t.TempDir(), "build")
		cmake := filepath.Join(tools, "cmake-4.4.5-linux-x86_64/bin/cmake")
		run(cmake, "-S", source, "-B", build, "-DCMOCKA_ROOT="+filepath.Join(tools, "cmocka"))
		run(cmake, "--build", build)
		trace := filepath.Join(t.TempDir(), "trace")
		run("gcc", "-O2", "-Wall", "-Wextra", filepath.Join(originalSource, "trace.c"), "-o", trace)
		exe := filepath.Join(build, "probe")
		child := filepath.Join(build, "child")
		binary, err := os.ReadFile(exe)
		if err != nil {
			t.Fatal(err)
		}
		sources := map[string]string{filepath.Join(source, "probe.c"): liveSourcePath("cmocka", "probe.c"), filepath.Join(source, "child.c"): liveSourcePath("cmocka", "child.c")}
		o := Options{Module: "live", Package: "tests", Work: t.TempDir(), Version: "cmocka 2.0.2; cmake 4.4.5; gcc " + strings.TrimSpace(string(run("gcc", "-dumpfullversion"))) + "; gcov; binary " + tr.Digest(binary), Sources: sources,
			Request: tr.Request{Runner: "cmocka-xml", Root: build, Executable: exe, ExecutableSha256: tr.Digest(binary), Target: "probe", ExpectedTests: []string{"probe::alpha", "probe::beta"}, TimeoutSeconds: 120}}
		call := 0
		o.Run = func(ctx context.Context, r tr.Request, inv tr.Invocation) (Capture, error) {
			ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()
			call++
			prefix := filepath.Join(evidence, fmt.Sprintf("cmocka-%02d", call))
			// Each owned build is fresh. Counters are reset before every invocation.
			for _, file := range []string{"probe.dir/probe.c.gcda", "child.dir/child.c.gcda"} {
				if e := os.Remove(filepath.Join(build, "CMakeFiles", file)); e != nil && !os.IsNotExist(e) {
					return Capture{}, e
				}
			}
			if err := os.MkdirAll(r.ReportDir, 0700); err != nil {
				return Capture{}, err
			}
			argv := inv.Argv
			env := inv.Environment
			if len(inv.Phases) == 1 {
				argv = inv.Phases[0].Argv
				env = inv.Phases[0].Environment
			}
			c := exec.CommandContext(ctx, trace, append([]string{prefix + ".trace", r.Executable}, argv...)...)
			c.Env = append(os.Environ(), "COVLIVE_CHILD="+child)
			for k, v := range env {
				c.Env = append(c.Env, k+"="+v)
			}
			var stdout, stderr bytes.Buffer
			c.Stdout = &stdout
			c.Stderr = &stderr
			err := c.Run()
			code := 0
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else if err != nil {
				return Capture{}, err
			}
			input := tr.Input{Runner: r.Runner, Target: r.Target, SuccessExitCodes: inv.SuccessExitCodes, FailureExitCodes: inv.FailureExitCodes, ExitCode: code, Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Reports: map[string][]byte{}, Expected: r.ExpectedTests, Selectors: r.Selectors}
			for _, p := range inv.ReportPaths {
				b, e := os.ReadFile(filepath.Join(r.ReportDir, p))
				if e != nil {
					return Capture{}, e
				}
				input.Reports[p] = b
			}
			// Export gcov's real line counters, retaining zero-hit entries. Each
			// binary is launched once and writes its own independently built mapping.
			exports := map[string]Report{}
			for _, item := range []struct{ exe, obj string }{{exe, "probe.dir/probe.c"}, {child, "child.dir/child.c"}} {
				object := filepath.Join(build, "CMakeFiles", item.obj)
				if _, e := os.Stat(object + ".gcda"); os.IsNotExist(e) {
					continue
				}
				raw := run("gcov", "--json-format", "--stdout", object+".gcda")
				var data struct {
					Files []struct {
						File  string `json:"file"`
						Lines []struct {
							Number int   `json:"line_number"`
							Count  int64 `json:"count"`
						} `json:"lines"`
					} `json:"files"`
				}
				if e := json.Unmarshal(raw, &data); e != nil {
					return Capture{}, e
				}
				var lcov strings.Builder
				for _, f := range data.Files {
					fmt.Fprintf(&lcov, "SF:%s\n", f.File)
					for _, l := range f.Lines {
						fmt.Fprintf(&lcov, "DA:%d,%d\n", l.Number, l.Count)
					}
					lcov.WriteString("end_of_record\n")
				}
				exports[item.exe] = Report{Format: "lcov", Data: []byte(lcov.String())}
				if e := os.WriteFile(prefix+"-"+filepath.Base(item.exe)+".gcov.json", raw, 0600); e != nil {
					return Capture{}, e
				}
			}
			raw, e := os.ReadFile(prefix + ".trace")
			if e != nil {
				return Capture{}, e
			}
			capture, e := liveTrace(raw, exports)
			if e != nil {
				return Capture{}, e
			}
			capture.Input = input
			for suffix, b := range map[string][]byte{".stdout": input.Stdout, ".stderr": input.Stderr, ".xml": input.Reports["cmocka.xml"]} {
				if e := os.WriteFile(prefix+suffix, b, 0600); e != nil {
					return Capture{}, e
				}
			}
			b, e := json.MarshalIndent(capture, "", "  ")
			if e != nil {
				return Capture{}, e
			}
			e = os.WriteFile(prefix+".capture.json", append(b, '\n'), 0600)
			return capture, e
		}
		outcomes, err := Collect(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		if len(outcomes) != 3 || call != 3 {
			t.Fatalf("outcomes=%+v calls=%d", outcomes, call)
		}
		for _, out := range outcomes {
			if out.Record == nil {
				t.Fatalf("%s: %s", out.Name, out.Divergent)
			}
		}
		oids := map[string]string{}
		for _, name := range []string{"probe.c", "child.c"} {
			b, e := os.ReadFile(filepath.Join(source, name))
			if e != nil {
				t.Fatal(e)
			}
			oids[liveSourcePath("cmocka", name)] = liveBlobOID(b)
		}
		ix, err := Build(o, liveSourceCommit(), oids, outcomes)
		if err != nil {
			t.Fatal(err)
		}
		liveIndexEvidence(t, evidence, ix)
		if len(ix.Tests) != 2 || len(ix.Unordered) != 1 {
			t.Fatalf("unqualified index %+v", ix)
		}
		// The changed covered statement belongs only to alpha. The unexecuted
		// return has an explicit zero count. Child coverage also belongs to alpha.
		bodies := map[string]Bodies{liveSourcePath("cmocka", "probe.c"): {OID: oids[liveSourcePath("cmocka", "probe.c")], Spans: []Body{{10, 18}}}, liveSourcePath("cmocka", "child.c"): {OID: oids[liveSourcePath("cmocka", "child.c")], Spans: []Body{{2, 4}}}}
		for _, tc := range []struct {
			file, oid string
			line      int
			want      string
		}{{"probe.c", oids[liveSourcePath("cmocka", "probe.c")], 12, "probe::alpha"}, {"probe.c", oids[liveSourcePath("cmocka", "probe.c")], 17, ""}, {"child.c", oids[liveSourcePath("cmocka", "child.c")], 3, "probe::alpha"}} {
			diff := linecov.FileDiff{Path: liveSourcePath("cmocka", tc.file), OldOID: tc.oid, NewOID: "changed", Hunks: []linecov.Hunk{{Start: tc.line, End: tc.line}}}
			// These are explicit hypothetical statement edits in the controlled fixture.
			eligible := map[string]EligibleChange{diff.Path: {OldOID: diff.OldOID, NewOID: diff.NewOID, Hunks: diff.Hunks}}
			effect := Analyze(ix, []linecov.FileDiff{diff}, bodies, eligible)
			var got []string
			for id := range effect.Tests {
				got = append(got, ix.Tests[id].Name)
			}
			if effect.Fallback != "" || strings.Join(got, ",") != tc.want {
				t.Fatalf("%s:%d effect=%+v selected=%v", tc.file, tc.line, effect, got)
			}
		}
		effect := Analyze(ix, []linecov.FileDiff{{Path: liveSourcePath("cmocka", "probe.c"), OldOID: oids[liveSourcePath("cmocka", "probe.c")], NewOID: "changed", Hunks: []linecov.Hunk{{Start: 10, End: 10}}}}, bodies, nil)
		if effect.Fallback == "" {
			t.Fatal("declaration did not keep static plan")
		}
		liveMutants(t, o, ix, bodies, source, evidence, "cmocka", []liveMutant{
			{"probe.c", 12, "return 11;", "return 10;"},
			{"probe.c", 15, "return 22;", "return 21;"},
			{"probe.c", 17, "return 33;", "return 34;"},
			{"child.c", 3, "return 42;", "return 43;"},
		}, func() { run(cmake, "--build", build) })
		t.Logf("three complete native captures retained in %s; production exclusion remains disabled", evidence)
	})
}

// liveTrace trusts only the bounded observer's complete event stream. A traced
// executable with no native dump stays uninstrumented; no process is inferred
// from a coverage directory. Thread events are not process launch evidence.
func liveTrace(raw []byte, exports map[string]Report) (Capture, error) {
	c := Capture{}
	ids := map[int]int{}
	executions := map[int]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			return c, fmt.Errorf("empty trace event")
		}
		if f[0] == "COMPLETE" {
			c.TreeComplete = true
			continue
		}
		if len(f) != 3 {
			return c, fmt.Errorf("invalid trace event")
		}
		id, e := strconv.Atoi(f[1])
		if e != nil {
			return c, e
		}
		switch f[0] {
		case "START":
			parent, e := strconv.Atoi(f[2])
			if e != nil {
				return c, e
			}
			if _, ok := ids[id]; ok {
				return c, fmt.Errorf("duplicate launch")
			}
			ids[id] = len(c.Processes)
			c.Processes = append(c.Processes, Process{ID: id, Parent: parent})
		case "CLONE": // JVM/runtime threads share their process's counters.
		case "EXEC":
			i, ok := ids[id]
			if !ok {
				return c, fmt.Errorf("exec without process launch")
			}
			executions[id]++
			report, ok := exports[f[2]]
			c.Processes[i].Instrumented = ok && executions[id] == 1
			if ok {
				c.Processes[i].Reports = append(c.Processes[i].Reports, report)
			}
		case "EXIT":
			if i, ok := ids[id]; ok {
				c.Processes[i].Exited = true
			}
		default:
			return c, fmt.Errorf("unknown trace event")
		}
	}
	return c, nil
}

// LPCV-V0-9401: original native reports and actual process/export gaps stay
// distinct. Stock coverage tools do not instrument the build/runner launchers;
// these refusals are evidence, never permission to omit them from the tree.
func TestNativeLiveFamilies(t *testing.T) {
	tools, evidence := os.Getenv("CORVINT_NATIVE_LIVE_TOOLS"), os.Getenv("CORVINT_NATIVE_LIVE_EVIDENCE")
	if tools == "" || evidence == "" {
		t.Skip("operator-supplied native toolchains and evidence directory required")
	}
	driver, err := filepath.Abs("testdata/live/driver.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"jvm", "dotnet", "rust", "swift"} {
		t.Run(family, func(t *testing.T) {
			evidence, err := os.MkdirTemp(evidence, family+"-")
			if err != nil {
				t.Fatal(err)
			}
			work := t.TempDir()
			ctx := context.Background()
			setup := exec.CommandContext(ctx, "python3", driver, family, work, "setup")
			setup.Env = append(os.Environ(), "CORVINT_NATIVE_LIVE_EVIDENCE="+evidence)
			var setupErr bytes.Buffer
			setup.Stderr = &setupErr
			raw, err := setup.Output()
			if err != nil {
				if e := os.WriteFile(filepath.Join(evidence, family+"-setup.stderr"), setupErr.Bytes(), 0600); e != nil {
					t.Fatal(e)
				}
				if family == "dotnet" && bytes.Contains(setupErr.Bytes(), []byte("System.IO.IOException")) && bytes.Contains(setupErr.Bytes(), []byte("NuGet-Migrations")) {
					t.Skip("PARTIAL: host named mutex NuGet-Migrations is unavailable; native .NET runs NOT_RUN; original setup failure retained")
				}
				t.Fatalf("setup: %v\n%s", err, setupErr.Bytes())
			}
			var request tr.Request
			if err = json.Unmarshal(raw, &request); err != nil {
				t.Fatalf("setup request: %v\n%s", err, raw)
			}
			sources := map[string]string{}
			oids := map[string]string{}
			err = filepath.WalkDir(filepath.Join(work, "project"), func(path string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if d.IsDir() {
					if d.Name() == "target" || d.Name() == ".build" || d.Name() == "obj" || d.Name() == "bin" || d.Name() == "classes" {
						return filepath.SkipDir
					}
					return nil
				}
				rel, e := filepath.Rel(filepath.Join(work, "project"), path)
				if e != nil {
					return e
				}
				rel = filepath.ToSlash(rel)
				sources[path] = liveSourcePath(family, rel)
				sources[rel] = liveSourcePath(family, rel)
				b, e := os.ReadFile(path)
				if e != nil {
					return e
				}
				oids[liveSourcePath(family, rel)] = liveBlobOID(b)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			o := Options{Module: "live", Package: "tests", Work: t.TempDir(), Version: map[string]string{"jvm": "Temurin 21.0.13+9; JUnit 1.11.4; JaCoCo 0.8.13; ", "dotnet": "SDK 8.0.427; xUnit 2.9.2; coverlet 6.0.2; ", "rust": "rustc 1.99.1; llvm-tools-preview; ", "swift": "Swift 6.4.2; LLVM instrumentation; "}[family] + request.ExecutableSha256, Sources: sources, Request: request, Agent: filepath.Join(tools, "org.jacoco.agent-0.8.13-runtime.jar")}
			calls := 0
			o.Run = func(ctx context.Context, r tr.Request, inv tr.Invocation) (Capture, error) {
				calls++
				packet, e := json.Marshal(struct {
					Request    tr.Request    `json:"request"`
					Invocation tr.Invocation `json:"invocation"`
				}{r, inv})
				if e != nil {
					return Capture{}, e
				}
				c := exec.CommandContext(ctx, "python3", driver, family, work)
				c.Env = append(os.Environ(), "CORVINT_NATIVE_LIVE_EVIDENCE="+evidence)
				c.Stdin = bytes.NewReader(packet)
				var stderr bytes.Buffer
				c.Stderr = &stderr
				raw, e := c.Output()
				if e != nil {
					return Capture{}, fmt.Errorf("live adapter: %w: %s", e, stderr.Bytes())
				}
				var capture Capture
				if e = json.Unmarshal(raw, &capture); e != nil {
					return Capture{}, e
				}
				return capture, nil
			}
			outcomes, err := Collect(ctx, o)
			if err != nil {
				t.Fatal(err)
			}
			b, err := json.MarshalIndent(outcomes, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(evidence, family+"-outcomes.json"), append(b, '\n'), 0600); err != nil {
				t.Fatal(err)
			}
			if calls != 3 || len(outcomes) != 3 {
				t.Fatalf("native identities/isolation incomplete: calls=%d outcomes=%+v", calls, outcomes)
			}
			if family == "jvm" {
				for _, out := range outcomes {
					if out.Record == nil {
						t.Fatalf("%s: %s", out.Name, out.Divergent)
					}
				}
				ix, e := Build(o, liveSourceCommit(), oids, outcomes)
				if e != nil {
					t.Fatal(e)
				}
				liveIndexEvidence(t, evidence, ix)
				if len(ix.Unordered) != 1 || len(ix.Tests) != 2 {
					t.Fatalf("unqualified JVM index %+v", ix)
				}
				bodies := map[string]Bodies{
					liveSourcePath("jvm", "p/Logic.java"): {OID: oids[liveSourcePath("jvm", "p/Logic.java")], Spans: []Body{{4, 12}}},
					liveSourcePath("jvm", "p/Child.java"): {OID: oids[liveSourcePath("jvm", "p/Child.java")], Spans: []Body{{9, 11}}},
				}
				liveMutants(t, o, ix, bodies, filepath.Join(work, "project"), evidence, "jvm", []liveMutant{
					{"p/Logic.java", 6, "return 11;", "return 10;"},
					{"p/Logic.java", 9, "return 22;", "return 21;"},
					{"p/Logic.java", 11, "return 33;", "return 34;"},
					{"p/Child.java", 10, "return 42;", "return 43;"},
				}, func() {
					cmd := exec.CommandContext(ctx, "python3", driver, family, work, "rebuild")
					if b, e := cmd.CombinedOutput(); e != nil {
						t.Fatalf("JVM rebuild: %v\n%s", e, b)
					}
				})
			} else {
				for _, out := range outcomes {
					if out.Record != nil || (out.Divergent != "uninstrumented or undumped process" && out.Divergent != "incomplete process coverage") {
						t.Fatalf("launcher coverage gap lost: %+v", out)
					}
				}
			}
			// Standalone native exports must decode without losing zero-hit lines,
			// even where the collector correctly refuses a complete process record.
			exports, e := filepath.Glob(filepath.Join(evidence, family+"-*.coverage-*"))
			if e != nil {
				t.Fatal(e)
			}
			if len(exports) == 0 {
				t.Fatal("no real native coverage exports retained")
			}
			for _, path := range exports {
				b, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				format := strings.TrimPrefix(filepath.Ext(path), ".")
				if _, e = Decode(Report{Format: format, Data: b}, sources); e != nil {
					t.Fatalf("native %s: %v", path, e)
				}
			}
			t.Logf("%s: %d native suite/isolated invocations; captures and exports in %s", family, calls, evidence)
		})
	}
}

// LPCV-V0-9401: the live observer cannot turn a missing dump into coverage.
func TestLiveTraceRefusals(t *testing.T) {
	t.Run("LPCV-V0-9401 process evidence", func(t *testing.T) {
		report := reports("lcov", "src", 1)
		for _, tc := range []struct {
			name, trace string
			exports     map[string]Report
			valid       bool
		}{
			{"complete", "START 1 0\nEXEC 1 /probe\nEXIT 1 0\nCOMPLETE\n", map[string]Report{"/probe": report}, true},
			{"unobserved completion", "START 1 0\nEXEC 1 /probe\nEXIT 1 0\n", map[string]Report{"/probe": report}, false},
			{"undumped launcher", "START 1 0\nEXEC 1 /launcher\nEXIT 1 0\nCOMPLETE\n", map[string]Report{"/probe": report}, false},
			{"child missing dump", "START 1 0\nEXEC 1 /probe\nSTART 2 1\nEXEC 2 /helper\nEXIT 2 0\nEXIT 1 0\nCOMPLETE\n", map[string]Report{"/probe": report}, false},
			{"child not retired", "START 1 0\nEXEC 1 /probe\nSTART 2 1\nEXEC 2 /helper\nEXIT 1 0\nCOMPLETE\n", map[string]Report{"/probe": report, "/helper": report}, false},
			{"disconnected child", "START 1 0\nEXEC 1 /probe\nSTART 2 3\nEXEC 2 /helper\nEXIT 2 0\nEXIT 1 0\nCOMPLETE\n", map[string]Report{"/probe": report, "/helper": report}, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				capture, err := liveTrace([]byte(tc.trace), tc.exports)
				if err != nil {
					t.Fatal(err)
				}
				_, err = processLines(capture, map[string]string{"src": "src"})
				if (err == nil) != tc.valid {
					t.Fatalf("coverage admission %v want valid=%t", err, tc.valid)
				}
			})
		}
	})
}

// LPCV-V0-8205, LPCV-V0-9401: exercise the Python reducer used by the real
// adapter, including the jspawnhelper -> Java history seen in retained evidence.
func TestLiveImageHistory(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("Python live adapter runtime unavailable")
	}
	script := `import json, sys
from pathlib import Path
sys.path.insert(0, sys.argv[1])
from images import capture_images
p = json.load(sys.stdin)
print(json.dumps(capture_images(p['trace'], 'jvm', Path('/java'), [('lcov', p['report'])], b'Session "1"\nSession "1147"')))
`
	for _, tc := range []struct {
		name, trace string
		valid       bool
	}{
		{"single Java image", "START 1 0\nEXEC 1 /java\nEXIT 1 0\nCOMPLETE\n", true},
		{"launcher then Java", "START 1 0\nEXEC 1 /java\nSTART 1147 1\nEXEC 1147 /jspawnhelper\nEXEC 1147 /java\nEXIT 1147 0\nEXIT 1 0\nCOMPLETE\n", false},
		{"Java then launcher", "START 1 0\nEXEC 1 /java\nEXEC 1 /jspawnhelper\nEXIT 1 0\nCOMPLETE\n", false},
		{"repeated Java session ambiguous", "START 1 0\nEXEC 1 /java\nEXEC 1 /java\nEXIT 1 0\nCOMPLETE\n", false},
		{"no executable image", "START 1 0\nEXIT 1 0\nCOMPLETE\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			packet, err := json.Marshal(map[string]string{"trace": tc.trace, "report": base64.StdEncoding.EncodeToString(reports("lcov", "src", 1).Data)})
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("python3", "-B", "-c", script, "testdata/live")
			cmd.Stdin = bytes.NewReader(packet)
			raw, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("live image adapter: %v: %s", err, raw)
			}
			var capture Capture
			if err = json.Unmarshal(raw, &capture); err != nil {
				t.Fatal(err)
			}
			_, err = processLines(capture, map[string]string{"src": "src"})
			if (err == nil) != tc.valid {
				t.Fatalf("coverage admission %v want valid=%t: %+v", err, tc.valid, capture)
			}
			// A later uncovered image must not erase the earlier native export.
			if tc.name == "Java then launcher" && len(capture.Processes[0].Reports) != 1 {
				t.Fatal("earlier executable image export erased")
			}
		})
	}
}

type liveMutant struct {
	file     string
	line     int
	old, new string
}

// liveMutants is a deterministic controlled-project falsification, not the
// Beamfall repository qualification. It actually rebuilds each changed source
// and compares every full-suite native failure with the candidate selection.
func liveMutants(t *testing.T, o Options, ix *linecov.Index, bodies map[string]Bodies, source, evidence, family string, mutants []liveMutant, rebuild func()) {
	t.Helper()
	type row struct {
		File             string
		Line             int
		Selected, Failed []string
		Negative         bool
	}
	var rows []row
	for seq, m := range mutants {
		path := filepath.Join(source, m.file)
		base, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Count(base, []byte(m.old)) != 1 {
			t.Fatalf("nonunique mutant %s:%d", m.file, m.line)
		}
		mutant := bytes.Replace(base, []byte(m.old), []byte(m.new), 1)
		// The frozen mutant list admits only literal return-value replacements.
		// Its exact source bytes and diff are the fixture's classification, not
		// a general native source provider or a qualification claim.
		for _, statement := range []string{m.old, m.new} {
			fields := strings.Fields(statement)
			if len(fields) != 2 || fields[0] != "return" || !strings.HasSuffix(fields[1], ";") {
				t.Fatal("mutant requires source-provider classification")
			}
			if _, err := strconv.Atoi(strings.TrimSuffix(fields[1], ";")); err != nil {
				t.Fatal(err)
			}
		}
		if liveBlobOID(base) != bodies[liveSourcePath(family, m.file)].OID ||
			strings.TrimSpace(strings.Split(string(base), "\n")[m.line-1]) != m.old {
			t.Fatal("mutant source binding mismatch")
		}
		diff := linecov.FileDiff{Path: liveSourcePath(family, m.file), OldOID: liveBlobOID(base), NewOID: liveBlobOID(mutant), Hunks: []linecov.Hunk{{Start: m.line, End: m.line}}}
		eligible := map[string]EligibleChange{diff.Path: {OldOID: diff.OldOID, NewOID: diff.NewOID, Hunks: diff.Hunks}}
		effect := Analyze(ix, []linecov.FileDiff{diff}, bodies, eligible)
		if effect.Fallback != "" {
			t.Fatal(effect.Fallback)
		}
		selected := map[string]bool{}
		r := row{File: m.file, Line: m.line, Selected: []string{}, Failed: []string{}}
		for id := range effect.Tests {
			name := ix.Tests[id].Name
			selected[name] = true
			r.Selected = append(r.Selected, name)
		}
		if err = os.WriteFile(path, mutant, 0600); err != nil {
			t.Fatal(err)
		}
		rebuild()
		_, observation, err := o.run(context.Background(), t.TempDir(), seq, linecov.InOrder, "")
		if restoreErr := os.WriteFile(path, base, 0600); restoreErr != nil {
			t.Fatal(restoreErr)
		}
		if err != nil || !observation.Complete {
			t.Fatalf("mutant native observation: %+v %v", observation, err)
		}
		for _, test := range observation.Tests {
			if test.State == tr.Failed {
				r.Failed = append(r.Failed, test.ID)
				if !selected[test.ID] {
					t.Fatalf("MISSED mutant %s:%d test %s", m.file, m.line, test.ID)
				}
			}
		}
		r.Negative = len(r.Selected) == 0
		if r.Negative && len(r.Failed) != 0 {
			t.Fatal("negative control killed tests")
		}
		if !r.Negative && len(r.Failed) == 0 {
			t.Fatalf("covered mutant survived: %s:%d", m.file, m.line)
		}
		rows = append(rows, r)
	}
	rebuild()
	b, err := json.MarshalIndent(struct {
		Seed          int
		Mutants       []row
		Misses        []string
		Qualification string
	}{9401, rows, []string{}, "controlled project only; LPCV-V0-963 NOT_RUN on pinned real repository"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(evidence, family+"-mutants.json"), append(b, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func liveBlobOID(data []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d%c", len(data), 0)
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// The final operator run supplies the immutable commit containing these sources.
func liveSourceCommit() string {
	if commit := os.Getenv("CORVINT_NATIVE_LIVE_SOURCE_COMMIT"); commit != "" {
		return commit
	}
	return "experimental-uncommitted-fixture"
}

func liveSourcePath(family, name string) string {
	prefix := "internal/nativelinecov/testdata/live/"
	if family != "cmocka" {
		prefix += family + "/"
	}
	return prefix + name
}

func liveIndexEvidence(t *testing.T, evidence string, ix *linecov.Index) {
	t.Helper()
	var data bytes.Buffer
	if err := ix.Encode(&data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evidence, "records.index"), data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	view, err := json.MarshalIndent(ix, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(evidence, "records.json"), append(view, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
