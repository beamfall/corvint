// SPDX-License-Identifier: Apache-2.0
package runnerprotocol

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/execfile"
	"github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func TestMain(m *testing.M) {
	if os.Args[0] == "/proc/self/fd/3" {
		fixtureProcess()
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func fixtureProcess() {
	mode, _ := os.ReadFile(".fixture-mode")
	if string(mode) == "reference" {
		if err := Serve(context.Background(), os.Stdin, os.Stdout, "reference", "1", map[string]registry.RunnerPlugin{ReferenceRunner: Reference{}}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		r, err := decodeRequest(bytes.TrimSuffix(line, []byte{'\n'}))
		if err != nil {
			return
		}
		var result any
		switch r.Method {
		case "hello":
			h := Hello{Protocol: Protocol, Plugin: "reference", Version: "1", Runners: []string{ReferenceRunner}, Pieces: map[string][]string{ReferenceRunner: {"inventory", "test-evidence", "qualification"}}, Features: []string{"example-feature"}}
			switch string(mode) {
			case "crash":
				os.Exit(7)
			case "timeout":
				time.Sleep(3 * time.Second)
				return
			case "oversize":
				fmt.Print(strings.Repeat("x", 8192))
				return
			case "protocol":
				fmt.Println(`{"id":"wrong","result":{}}`)
				continue
			case "duplicate":
				fmt.Printf("{\"id\":%q,\"id\":%q,\"result\":{}}\n", r.ID, r.ID)
				continue
			case "schema":
				fmt.Printf("{\"id\":%q,\"result\":{\"qualified\":true}}\n", r.ID)
				continue
			case "error":
				fmt.Printf("{\"id\":%q,\"error\":{\"code\":\"fixture\",\"message\":\"failed\"}}\n", r.ID)
				return
			case "version":
				h.Protocol = "corvint-runner/2"
			case "undeclared":
				h.Runners = []string{"unannounced"}
			}
			result = h
		case "shutdown":
			if string(mode) == "hook-mutation" {
				_ = execfile.WriteFile(".git/hooks/planted", []byte("#!/bin/sh\nexit 0\n"), 0700)
			}
			if string(mode) == "mutation" || string(mode) == "late-mutation" {
				_ = os.WriteFile("tracked.txt", []byte("changed again"), 0600)
			}
			if string(mode) == "untracked-mutation" {
				_ = os.WriteFile("new.txt", []byte("new"), 0600)
			}
			if string(mode) == "stderr" {
				fmt.Fprint(os.Stderr, strings.Repeat("x", 2048))
			}
			if string(mode) == "output-cap" {
				fmt.Fprint(os.Stderr, strings.Repeat("x", 10000))
			}
			if string(mode) == "shutdown-timeout" {
				time.Sleep(3 * time.Second)
				return
			}
			result = map[string]any{}
		default:
			if string(mode) == "forged-authority" {
				fmt.Printf("{\"id\":%q,\"result\":{\"records\":[],\"qualified\":true}}\n", r.ID)
				continue
			}
			if string(mode) == "wrong-response-runner" {
				fmt.Printf("{\"id\":%q,\"result\":{\"runner\":\"rogue\",\"results\":[]}}\n", r.ID)
				continue
			}
			result, err = serveMethod(context.Background(), map[string]registry.RunnerPlugin{ReferenceRunner: Reference{}}, r)
			if err != nil {
				fmt.Printf("{\"id\":%q,\"error\":{\"code\":\"fixture\",\"message\":%q}}\n", r.ID, err.Error())
				return
			}
		}
		if r.Method == "command" && string(mode) == "nul-environment" {
			result = commandResult{Argv: []string{"go", "test"}, Cwd: ".", Env: map[string]string{"VALUE": "before\x00after"}, Artifacts: []string{}}
		}
		data, _ := json.Marshal(result)
		encoded, _ := json.Marshal(response{ID: r.ID, Result: data})
		fmt.Println(string(encoded))
		if r.Method == "shutdown" {
			return
		}
	}
}
func requireLinux(t testing.TB) {
	t.Helper()
	if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" && runtime.GOARCH != "riscv64") {
		t.Skip("descriptor execution unavailable on this platform")
	}
}
func write(t testing.TB, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func gitFixture(t testing.TB, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, b)
	}
}
func fixtureRepo(t testing.TB, mode string) string {
	t.Helper()
	root := t.TempDir()
	gitFixture(t, root, "init", "-q")
	write(t, filepath.Join(root, "tracked.txt"), "original")
	write(t, filepath.Join(root, ".fixture-mode"), mode)
	gitFixture(t, root, "add", ".")
	gitFixture(t, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	return root
}
func selfPin(t testing.TB) (Pin, string) {
	t.Helper()
	requireLinux(t)
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		t.Fatal(err)
	}
	return Pin{Name: "reference", Protocol: Protocol, Executable: path, SHA256: hex.EncodeToString(h.Sum(nil)), Runners: []string{ReferenceRunner}}, path
}
func fixtureResolved(t *testing.T) Resolved {
	t.Helper()
	pin, source := selfPin(t)
	store := t.TempDir()
	installed, err := Install(store, pin, source)
	if err != nil {
		t.Fatal(err)
	}
	// This repository source label must never be opened or resolved on PATH.
	pin.Executable = "./corvint-runner-reference"
	resolved, err := Resolve(pin, Discovery{Schema: DiscoverySchema, Plugins: []Pin{installed}}, Discovery{})
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
func corpus(t testing.TB, runDir string) Corpus {
	t.Helper()
	inventory := filepath.Join(runDir, "inventory.json")
	report := filepath.Join(runDir, "go.json")
	write(t, inventory, `{"config_digest":"`+strings.Repeat("a", 64)+`","tests":[{"id":"p::TestOne","path":"one_test.go"}]}`)
	write(t, report, "{\"Action\":\"start\",\"Package\":\"p\"}\n{\"Action\":\"run\",\"Package\":\"p\",\"Test\":\"TestOne\"}\n{\"Action\":\"pass\",\"Package\":\"p\",\"Test\":\"TestOne\"}\n{\"Action\":\"pass\",\"Package\":\"p\"}\n")
	return Corpus{Key: testrunner.InventoryKey{Checkout: strings.Repeat("b", 40), Runner: ReferenceRunner, ConfigDigest: strings.Repeat("a", 64), OS: runtime.GOOS, Arch: runtime.GOARCH}, Inventory: inventory, Report: report, RunDir: runDir, Artifacts: []string{inventory}}
}
func openFixture(t *testing.T, mode string, limits Limits) (*Session, Corpus) {
	t.Helper()
	resolved := fixtureResolved(t)
	runDir := t.TempDir()
	c := corpus(t, runDir)
	s, err := Open(context.Background(), resolved, Options{Repository: fixtureRepo(t, mode), RunDir: runDir, CoreVersion: "test", Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s, c
}
func requireFailure(t *testing.T, err error, reason string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), reason) {
		t.Fatalf("want %q, got %v", reason, err)
	}
}

// RSP-V0-025: one corpus, unchanged T3 SPI, both implementations.
func TestRSPV0025TwinParity(t *testing.T) {
	s, c := openFixture(t, "reference", Limits{})
	plugin, err := s.Plugin(ReferenceRunner)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Conform(context.Background(), Reference{}, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	process, err := Conform(context.Background(), plugin, c, s.Close)
	if err != nil {
		t.Fatal(err)
	}
	if err = CompareConformance(compiled, process); err != nil {
		t.Fatalf("%v\ncompiled=%+v\nprocess=%+v", err, compiled, process)
	}
	if !reflect.DeepEqual(Reference{}.Pieces(), plugin.Pieces()) {
		t.Fatal("piece parity")
	}
	if len(process.Results) != 1 || process.Results[0].Outcome != "passed" || len(process.Evidence) != 1 {
		t.Fatalf("empty conformance result: %+v", process)
	}
}
func TestRSPV0025UnknownAndLegacy(t *testing.T) {
	if _, ok := registry.SelectionPlugin("unknown"); ok {
		t.Fatal("unknown plugin")
	}
	p, ok := registry.SelectionPlugin("go-test")
	if !ok || p.Pieces().FullReason(false) == "" {
		t.Fatal("compiled-in hot path changed")
	}
	a := ConformanceResult{}
	b := ConformanceResult{Results: []testrunner.RunnerResult{{ID: "extra"}}}
	requireFailure(t, CompareConformance(a, b), "parity-mismatch")
}

// RSP-V0-026: strict frames and payloads, including duplicate nested members.
func TestRSPV0026StrictPositive(t *testing.T) {
	r, err := decodeRequest([]byte(`{"id":"one","method":"hello","params":{"protocols":["corvint-runner/1"],"core":"1"}}`))
	if err != nil || r.ID != "one" {
		t.Fatal(r, err)
	}
	var result map[string]any
	if err := decodeResponse([]byte(`{"id":"one","result":{}}`), "one", "shutdown", &result); err != nil {
		t.Fatal(err)
	}
}
func TestRSPV0026StrictNegative(t *testing.T) {
	for _, raw := range []string{
		`{"id":"1","id":"2","method":"shutdown","params":{}}`,
		`{"id":"1","method":"hello","params":{"protocols":[],"core":"x","core":"y"}}`,
		`{"id":"1","method":"shutdown","params":{},"unknown":true}`,
		`{"id":2,"method":"shutdown","params":{}}`,
		`{"id":null,"method":"shutdown","params":{}}`,
		`{"id":"1","method":"shutdown","params":null}`,
		`{"id":"1","method":"shutdown","params":{}} {}`,
		"{\"id\":\"\xff\",\"method\":\"shutdown\",\"params\":{}}",
	} {
		if _, err := decodeRequest([]byte(raw)); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	var out any
	for _, raw := range []string{`{"id":"1","result":{},"error":{"code":"x","message":"y"}}`, `{"id":"2","result":{}}`, `{"id":"1","result":null}`, `{"id":"1","error":null}`, `{"id":"1","error":{"code":"x"}}`} {
		if err := decodeResponse([]byte(raw), "1", "shutdown", &out); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

// RSP-V0-027: methods produce data only and all request/result shapes are closed.
func TestRSPV0027Methods(t *testing.T) {
	s, c := openFixture(t, "reference", Limits{})
	p, _ := s.Plugin(ReferenceRunner)
	for _, purpose := range []string{"inventory", "run", "collect"} {
		cmd, err := p.Command(context.Background(), registry.CommandRequest{Key: c.Key, Purpose: purpose, Tests: []string{}, RunDir: c.RunDir})
		if err != nil || len(cmd.Argv) == 0 || !cmd.Superset {
			t.Fatal(cmd, err)
		}
	}
	if _, err := os.Stat(filepath.Join(s.options.Repository, "runner-started")); !os.IsNotExist(err) {
		t.Fatal("unexpected runner launch")
	}
	requests, results := Schemas()
	if len(requests) != 6 || len(results) != 6 {
		t.Fatal("missing method schema")
	}
}
func TestRSPV0027UnknownMethodAndPath(t *testing.T) {
	var out any
	requireFailure(t, validatePayload("execute", []byte(`{}`), false, &out), "unknown-method")
	s, c := openFixture(t, "reference", Limits{})
	p, _ := s.Plugin(ReferenceRunner)
	_, err := p.Parse(context.Background(), registry.ParseRequest{Runner: c.Key.Runner, Report: "../outside"})
	requireFailure(t, err, "report-path")
	if s.FullPlans()[ReferenceRunner] == "" {
		t.Fatal("missing FULL reason")
	}
}

// RSP-V0-028: strict declared pins and immutable trusted-base selection.
func TestRSPV0028DiscoveryPositive(t *testing.T) {
	pin, _ := selfPin(t)
	other := pin
	other.Name = "another"
	other.Runners = []string{"another"}
	d := Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin, other}}
	b, err := d.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeDiscovery(b)
	if err != nil || decoded.Plugins[0].Name != "another" {
		t.Fatal(decoded, err)
	}
	p := PlanDiscovery(b, b, true)
	if p.FullAll != "" || len(p.Full) != 0 {
		t.Fatal(p)
	}
	resolved := fixtureResolved(t)
	shadow := t.TempDir()
	write(t, filepath.Join(shadow, "corvint-runner-reference"), "not executable")
	t.Setenv("PATH", shadow)
	f, err := executableFile(resolved)
	if err != nil {
		t.Fatal("consulted PATH:", err)
	}
	f.Close()
}
func TestRSPV0028DiscoveryNegative(t *testing.T) {
	pin, _ := selfPin(t)
	b, _ := json.Marshal(Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin}})
	for _, data := range [][]byte{[]byte(`{"schema":"x","plugins":[],"extra":1}`), []byte(`{"schema":"x","schema":"y","plugins":[]}`), []byte(`{"schema":"x","plugins":null}`), []byte(`{"plugins":[]}`), bytes.Repeat([]byte("x"), MaxDiscoveryBytes+1)} {
		if _, err := DecodeDiscovery(data); err == nil {
			t.Fatalf("accepted malformed discovery")
		}
	}
	for _, other := range []Pin{pin, {Name: "different", Protocol: Protocol, Executable: "x", SHA256: pin.SHA256, Runners: pin.Runners}} {
		data, _ := json.Marshal(Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin, other}})
		if _, err := DecodeDiscovery(data); err == nil {
			t.Fatal("duplicate accepted")
		}
	}
	changed := pin
	changed.Name = "new"
	changed.Runners = []string{"new-runner"}
	candidate, _ := json.Marshal(Discovery{Schema: DiscoverySchema, Plugins: []Pin{changed}})
	plan := PlanDiscovery(b, candidate, true)
	if plan.Full[ReferenceRunner] != "candidate-pins-changed" || plan.Full["new-runner"] != "candidate-pins-changed" {
		t.Fatal(plan)
	}
	if PlanDiscovery(b, []byte(`{}`), true).FullAll == "" {
		t.Fatal("malformed candidate narrowed")
	}
	unknown, _ := json.Marshal(Discovery{Schema: "future", Plugins: []Pin{pin}})
	if PlanDiscovery(unknown, unknown, false).Full[ReferenceRunner] != "unsupported-schema" {
		t.Fatal("unknown schema skipped")
	}
	missing := PlanDiscovery(b, b, false)
	admitted, plan := ResolvePlan(missing, Discovery{}, Discovery{})
	if len(admitted) != 0 || plan.Full[ReferenceRunner] != "missing-executable" {
		t.Fatal(plan)
	}
}
func TestRSPV0028DigestAndSymlink(t *testing.T) {
	pin, source := selfPin(t)
	store := t.TempDir()
	installed, err := Install(store, pin, source)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(installed.Executable, 0700); err != nil {
		t.Fatal(err)
	}
	write(t, installed.Executable, "wrong bytes")
	_, err = Resolve(pin, Discovery{Schema: DiscoverySchema, Plugins: []Pin{installed}}, Discovery{})
	requireFailure(t, err, "digest-mismatch")
	link := filepath.Join(t.TempDir(), "link")
	if err = os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if _, err = openRegular(link); err == nil {
		t.Fatal("followed executable link")
	}
}

// RSP-V0-029: positive defaults, bounded retention, capped and timed failures.
func TestRSPV0029LimitsAndDiagnostics(t *testing.T) {
	defaults, err := (Limits{}).normalized()
	if err != nil || defaults.Message != 1<<20 || defaults.Output != 256<<20 || defaults.Stderr != 64<<10 || defaults.Hello != 5*time.Second || defaults.Request != 60*time.Second {
		t.Fatal(defaults, err)
	}
	s, _ := openFixture(t, "stderr", Limits{Stderr: 16})
	if err = s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(s.Diagnostics()) != 16 {
		t.Fatal("stderr retention unbounded")
	}
}
func TestRSPV0029LimitsNegative(t *testing.T) {
	for _, l := range []Limits{{Message: 8<<20 + 1}, {Output: 1<<30 + 1}, {Stderr: 65537}, {Hello: 6 * time.Second}, {Request: 601 * time.Second}, {Message: -1}} {
		if _, err := l.normalized(); err == nil {
			t.Fatal("accepted limits", l)
		}
	}
	s, _ := openFixture(t, "output-cap", Limits{Output: 2048})
	requireFailure(t, s.Close(context.Background()), "session-output-cap")
}

// RSP-V0-030: data is staged, trace/qualification cannot be supplied by a plugin.
func TestRSPV0030EvidencePublication(t *testing.T) {
	s, c := openFixture(t, "reference", Limits{})
	p, _ := s.Plugin(ReferenceRunner)
	r := registry.EvidenceRequest{Runner: c.Key.Runner, Artifacts: c.Artifacts}
	records, err := p.Evidence(context.Background(), r)
	requireFailure(t, err, "pending-session-close")
	if len(records) != 0 {
		t.Fatal("published early")
	}
	if p.Pieces().FullReason(false) == "" {
		t.Fatal("plugin qualified itself")
	}
	if err = s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	records, err = p.Evidence(context.Background(), r)
	if err != nil || len(records) != 1 {
		t.Fatal(records, err)
	}
	var executed map[string]any
	_ = json.Unmarshal(records[0].Executed, &executed)
	if executed["state"] != "absent" {
		t.Fatal("fabricated evidence")
	}
}
func TestRSPV0030DiscardAndForgedAuthority(t *testing.T) {
	s, c := openFixture(t, "late-mutation", Limits{})
	p, _ := s.Plugin(ReferenceRunner)
	r := registry.EvidenceRequest{Runner: c.Key.Runner, Artifacts: c.Artifacts}
	_, err := p.Evidence(context.Background(), r)
	requireFailure(t, err, "pending-session-close")
	requireFailure(t, s.Close(context.Background()), "repository-mutation")
	records, err := p.Evidence(context.Background(), r)
	if err == nil || len(records) != 0 {
		t.Fatal("failed session published evidence")
	}
	for _, name := range []string{"qualified", "excluded", "reads", "class"} {
		var out any
		raw := []byte(`{"records":[],"` + name + `":true}`)
		if err := validatePayload("evidence", raw, true, &out); err == nil {
			t.Fatal("accepted authority/trace", name)
		}
	}
}

// RSP-V0-031: every session failure ends in a named FULL, never a record.
func TestRSPV0031CleanSession(t *testing.T) {
	s, _ := openFixture(t, "reference", Limits{})
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.FullPlans()[ReferenceRunner] != "runner-not-qualified" {
		t.Fatal("new digest qualified itself")
	}
}
func TestRSPV0031FailureFixtures(t *testing.T) {
	resolved := fixtureResolved(t)
	for _, tc := range []struct{ mode, reason string }{{"crash", "crash"}, {"timeout", "timeout"}, {"oversize", "oversize-message"}, {"protocol", "protocol-error"}, {"duplicate", "protocol-error"}, {"schema", "schema-violation"}, {"error", "error-response"}, {"version", "version-mismatch"}, {"undeclared", "undeclared-runner"}} {
		t.Run(tc.mode, func(t *testing.T) {
			s, err := Open(context.Background(), resolved, Options{Repository: fixtureRepo(t, tc.mode), RunDir: t.TempDir(), CoreVersion: "test", Limits: Limits{Message: 4096, Hello: time.Second}})
			if s != nil {
				_ = s.Close(context.Background())
				t.Fatal("failed hello admitted")
			}
			requireFailure(t, err, tc.reason)
		})
	}
	for _, mode := range []string{"mutation", "untracked-mutation", "late-mutation"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := openFixture(t, mode, Limits{})
			if mode == "mutation" {
				write(t, filepath.Join(s.options.Repository, "tracked.txt"), "already dirty")
			}
			requireFailure(t, s.Close(context.Background()), "repository-mutation")
			if s.FullPlans()[ReferenceRunner] != "repository-mutation" {
				t.Fatal(s.FullPlans())
			}
			if len(s.staged) != 0 {
				t.Fatal("retained failed evidence")
			}
		})
	}
	t.Run("launch-failure", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "bad")
		if err := execfile.WriteFile(file, []byte("not executable format"), 0700); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256([]byte("not executable format"))
		pin := Pin{Name: "reference", Protocol: Protocol, Executable: file, SHA256: hex.EncodeToString(digest[:]), Runners: []string{ReferenceRunner}}
		r, err := Resolve(pin, Discovery{Schema: DiscoverySchema, Plugins: []Pin{pin}}, Discovery{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = Open(context.Background(), r, Options{Repository: fixtureRepo(t, "reference"), RunDir: t.TempDir(), CoreVersion: "test"})
		requireFailure(t, err, "launch-failure")
	})
}

// RSP-V0-032: version 1 has no prior defined major; only named features usable.
func TestRSPV0032NegotiatedFeatures(t *testing.T) {
	s, _ := openFixture(t, "features", Limits{})
	if err := s.RequireFeature("example-feature"); err != nil {
		t.Fatal(err)
	}
	if !s.Supports("example-feature") {
		t.Fatal("lost declared feature")
	}
}
func TestRSPV0032UnnegotiatedFeature(t *testing.T) {
	s, _ := openFixture(t, "features", Limits{})
	before := s.counter
	requireFailure(t, s.RequireFeature("not-offered"), "feature-not-negotiated")
	if s.counter != before {
		t.Fatal("sent unnamed feature")
	}
	pin, _ := selfPin(t)
	pin.Protocol = "corvint-runner/99"
	_, err := Resolve(pin, Discovery{}, Discovery{})
	requireFailure(t, err, "unsupported-protocol")
}

// RSP-V0-041: an explicit operator copy does not qualify or edit repository pins.
func TestRSPV0041OperatorInstall(t *testing.T) {
	pin, source := selfPin(t)
	store := t.TempDir()
	installed, err := Install(store, pin, source)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(installed.Executable) != filepath.Join(store, pin.SHA256) {
		t.Fatal("not digest keyed")
	}
	if _, err = Install(store, pin, source); err != nil {
		t.Fatal("idempotent install", err)
	}
	config, err := ReadDiscovery(filepath.Join(store, "runner-plugins.json"))
	if err != nil || len(config.Plugins) != 1 {
		t.Fatal(config, err)
	}
	if !reflect.DeepEqual(config.Plugins[0], installed) {
		t.Fatal("store pin mismatch")
	}
}
func TestRSPV0041InstallNegative(t *testing.T) {
	pin, source := selfPin(t)
	_, err := Install(fixtureRepo(t, "reference"), pin, source)
	requireFailure(t, err, "inside-repository")
	pin.SHA256 = strings.Repeat("0", 64)
	store := t.TempDir()
	_, err = Install(store, pin, source)
	requireFailure(t, err, "digest-mismatch")
	if _, err = os.Stat(filepath.Join(store, "runner-plugins.json")); !os.IsNotExist(err) {
		t.Fatal("published invalid store")
	}
}
