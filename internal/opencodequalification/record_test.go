package opencodequalification

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func recordFixture(t *testing.T) (Object, Object, Identity) {
	t.Helper()
	native, e := readObject("../../integrations/fixtures/opencode-native-report.json")
	if e != nil {
		t.Fatal(e)
	}
	id := Identity{SourceCommit: str(native["sourceCommit"]), HostSHA256: str(native["hostSHA256"]), CorvintSHA256: str(native["corvintSHA256"]), SourceFiles: map[string]string{}, Inputs: map[string]string{"collector.go": strings.Repeat("c", 64)}, HarnessSHA256: strings.Repeat("d", 64), QualificationBinarySHA256: strings.Repeat("e", 64), Tuple: map[string]string{}}
	for k, v := range object(native["sourceFiles"]) {
		id.SourceFiles[k] = str(v)
	}
	for _, k := range []string{"adapterVersion", "hostVersion", "os", "architecture"} {
		id.Tuple[k] = str(native[k])
	}
	object(native["cleanupEvidence"])["absentBeforeSupervisorCleanup"] = true
	native["harnessSHA256"] = id.HarnessSHA256
	native["qualificationBinarySHA256"] = id.QualificationBinarySHA256
	var lines []string
	for _, a := range []string{"run", "pass"} {
		for _, test := range focusedTests {
			b, _ := jsonBytes(Object{"Action": a, "Test": test, "Package": "github.com/Beamfall/corvint/cmd/corvint"})
			lines = append(lines, string(b))
		}
	}
	return native, Object{"exitCode": 0, "stdout": strings.Join(lines, "\n"), "stderr": ""}, id
}
func TestRecordValidation(t *testing.T) {
	t.Run("GOC-V0-008 AHI-032 preserved producer validation", func(t *testing.T) {
		n, f, id := recordFixture(t)
		r, e := BuildRecord(n, f, id, id)
		if e != nil {
			t.Fatal(e)
		}
		if object(r["conformance"])["compaction"] != "PASS" {
			t.Fatal(r)
		}
		mutations := map[string]func(Object, Object, *Identity){
			"nonfinite latency": func(n, f Object, id *Identity) { array(object(n["metrics"])["query"])[0] = math.Inf(1) },
			"nan latency":       func(n, f Object, id *Identity) { array(object(n["metrics"])["query"])[0] = math.NaN() },
			"too few samples":   func(n, f Object, id *Identity) { object(n["metrics"])["query"] = []any{1} },
			"threshold exceeded": func(n, f Object, id *Identity) {
				m := object(n["metrics"])
				v := make([]any, 20)
				for i := range v {
					v[i] = 501
				}
				m["query"] = v
				m["queryP95Ms"] = 501
			},

			"missing check":     func(n, f Object, id *Identity) { delete(object(n["checks"]), "native-compaction") },
			"string check":      func(n, f Object, id *Identity) { object(n["checks"])["latency"] = "PASS" },
			"focused missing":   func(n, f Object, id *Identity) { delete(f, "stdout") },
			"focused skipped":   func(n, f Object, id *Identity) { f["stdout"] = strings.Replace(str(f["stdout"]), "pass", "skip", 1) },
			"focused failed":    func(n, f Object, id *Identity) { f["exitCode"] = 1 },
			"focused bool exit": func(n, f Object, id *Identity) { f["exitCode"] = false },
			"focused malformed": func(n, f Object, id *Identity) { f["stdout"] = "not json" },
			"stale binary":      func(n, f Object, id *Identity) { id.QualificationBinarySHA256 = "changed" },
			"stale observer":    func(n, f Object, id *Identity) { id.HarnessSHA256 = "changed" },
			"wrong host":        func(n, f Object, id *Identity) { n["hostVersion"] = "2.0.19" },
			"wrong files":       func(n, f Object, id *Identity) { n["sourceFiles"] = Object{} },
			"bool latency":      func(n, f Object, id *Identity) { array(object(n["metrics"])["query"])[0] = true },
			"string latency":    func(n, f Object, id *Identity) { array(object(n["metrics"])["query"])[0] = "1" },
			"negative latency":  func(n, f Object, id *Identity) { array(object(n["metrics"])["query"])[0] = -1 },
			"p95 mismatch":      func(n, f Object, id *Identity) { object(n["metrics"])["queryP95Ms"] = -1 },
			"recall loss":       func(n, f Object, id *Identity) { object(object(n["metrics"])["criticalRecall"])["injected"] = 0 },
			"bytes worse":       func(n, f Object, id *Identity) { m := object(n["metrics"]); m["injectedBytes"] = m["manualBytes"] },
			"cleanup rescued":   func(n, f Object, id *Identity) { object(n["cleanupEvidence"])["absentBeforeSupervisorCleanup"] = false },
			"cleanup missing":   func(n, f Object, id *Identity) { delete(object(n["cleanupEvidence"]), "survivors") },
			"cleanup survivor":  func(n, f Object, id *Identity) { object(n["cleanupEvidence"])["survivors"] = []any{1} },
		}
		for name, mutate := range mutations {
			t.Run(name, func(t *testing.T) {
				n, f, before := recordFixture(t)
				_, _, after := recordFixture(t)
				mutate(n, f, &after)
				if _, e := BuildRecord(n, f, before, after); e == nil {
					t.Fatal("accepted malformed or stale evidence")
				}
			})
		}
	})
}
func TestAtomicRecord(t *testing.T) {
	t.Run("GOC-V0-008 AHI-032 interrupted publication invalidates old PASS", func(t *testing.T) {
		dir := t.TempDir()
		p, b := filepath.Join(dir, "record.json"), filepath.Join(dir, "previous.json")
		ctx := context.Background()
		if e := writeRecord(ctx, p, Object{"result": "PASS"}, nil); e != nil {
			t.Fatal(e)
		}
		if e := beginRecord(ctx, p, b); e != nil {
			t.Fatal(e)
		}
		old, _ := readObject(b)
		if old["result"] != "PASS" {
			t.Fatal(old)
		}
		cancelled, cancel := context.WithCancel(ctx)
		if e := writeRecord(cancelled, p, Object{"result": "PASS"}, cancel); e == nil {
			t.Fatal("cancelled publication succeeded")
		}
		current, _ := readObject(p)
		if current["result"] != "INCOMPLETE" {
			t.Fatal(current)
		}
		matches, _ := filepath.Glob(filepath.Join(dir, ".opencode-qualification-*"))
		if len(matches) != 0 {
			t.Fatal(matches)
		}
		if e := writeRecord(ctx, p, Object{"large": strings.Repeat("x", recordLimit)}, nil); e == nil {
			t.Fatal("oversize accepted")
		}
		if e := os.Symlink(p, filepath.Join(dir, "symlink")); e != nil {
			t.Fatal(e)
		}
		if e := beginRecord(ctx, filepath.Join(dir, "symlink"), b); e == nil {
			t.Fatal("symlink accepted")
		}
	})
}
func TestArchitecture(t *testing.T) {
	t.Run("GOC-V0-008 AHI-032 architecture normalization", func(t *testing.T) {
		for in, want := range map[string]string{"x86_64": "x64", "AMD64": "x64", "aarch64": "arm64", "arm64": "arm64"} {
			got, e := NodeArchitecture(in)
			if e != nil || got != want {
				t.Fatalf("%s: %s %v", in, got, e)
			}
		}
		if _, e := NodeArchitecture("unknown"); e == nil {
			t.Fatal("unknown accepted")
		}
		if _, e := NodeArchitecture(runtime.GOARCH); e != nil {
			t.Fatal(e)
		}
	})
}

func TestSupportedHostVersion(t *testing.T) {
	t.Run("AHI-009 AHI-032 bounded OpenCode 2.0 compatibility", func(t *testing.T) {
		for _, in := range []string{"opencode v2.0.18\n", "2.0.19", "2.0.999"} {
			if _, e := ParseSupportedHostVersion(in); e != nil {
				t.Fatalf("%q: %v", in, e)
			}
		}
		for _, in := range []string{"2.0.17", "2.1.0", "1.18.31", "2.0.19-beta", "2.0.019", "02.00.19", "2.0.9999999999999999999", "unknown"} {
			if _, e := ParseSupportedHostVersion(in); e == nil {
				t.Fatalf("accepted unsupported version %q", in)
			}
		}
	})
}

func TestQualificationRequiresCleanCheckout(t *testing.T) {
	t.Run("AHI-032 dirty and mixed checkouts remain unqualified", func(t *testing.T) {
		root := t.TempDir()
		ctx := context.Background()
		for _, args := range [][]string{{"git", "init", "-q"}, {"git", "config", "user.name", "Qualification"}, {"git", "config", "user.email", "qualification@example.invalid"}} {
			if _, e := capture(ctx, root, nil, args); e != nil {
				t.Fatal(e)
			}
		}
		tracked := filepath.Join(root, "tracked.txt")
		if e := os.WriteFile(tracked, []byte("clean\n"), 0600); e != nil {
			t.Fatal(e)
		}
		for _, args := range [][]string{{"git", "add", "tracked.txt"}, {"git", "commit", "-qm", "fixture"}} {
			if _, e := capture(ctx, root, nil, args); e != nil {
				t.Fatal(e)
			}
		}
		if e := requireCleanCheckout(ctx, root); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(tracked, []byte("dirty\n"), 0600); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("mixed\n"), 0600); e != nil {
			t.Fatal(e)
		}
		if e := requireCleanCheckout(ctx, root); e == nil || !strings.Contains(e.Error(), "clean committed checkout") {
			t.Fatalf("dirty checkout admitted: %v", e)
		}
	})
}

func TestProducerConsumer(t *testing.T) {
	t.Run("GOC-V0-008 AHI-032 Go producer to Node consumer", func(t *testing.T) {
		root := t.TempDir()
		pkg := filepath.Join(root, "integrations/opencode")
		if e := os.MkdirAll(pkg, 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.CopyFS(pkg, os.DirFS("../../integrations/opencode")); e != nil {
			t.Fatal(e)
		}
		node, e := exec.LookPath("node")
		if e != nil {
			t.Fatal(e)
		}
		node, e = filepath.EvalSymlinks(node)
		if e != nil {
			t.Fatal(e)
		}
		n, f, id := recordFixture(t)
		id.SourceFiles, e = packageFiles(root)
		if e != nil {
			t.Fatal(e)
		}
		id.HostSHA256, e = digest(node)
		if e != nil {
			t.Fatal(e)
		}
		id.CorvintSHA256 = id.HostSHA256
		manifest, e := readObject(filepath.Join(pkg, "package.json"))
		if e != nil {
			t.Fatal(e)
		}
		arch, e := NodeArchitecture(runtime.GOARCH)
		if e != nil {
			t.Fatal(e)
		}
		id.Tuple = map[string]string{"adapterVersion": str(manifest["version"]), "hostVersion": "2.0.19", "os": runtime.GOOS, "architecture": arch}
		n["hostVersion"] = "2.0.19"
		for k, v := range identityFields(id) {
			n[k] = v
		}
		for k, v := range id.Tuple {
			n[k] = v
		}
		record, e := BuildRecord(n, f, id, id)
		if e != nil {
			t.Fatal(e)
		}
		p := filepath.Join(root, "integrations/opencode-qualification.json")
		ctx := context.Background()
		if e := writeRecord(ctx, p, record, nil); e != nil {
			t.Fatal(e)
		}
		module, _ := jsonBytes(filepath.Join(pkg, "src/qualification.js"))
		rootJSON, _ := jsonBytes(root)
		nodeJSON, _ := jsonBytes(node)
		program := func(version string) string {
			versionJSON, _ := jsonBytes(version)
			return `import {pathToFileURL} from 'node:url'; const {qualificationStatus}=await import(pathToFileURL(` + string(module) + `).href); console.log(JSON.stringify(await qualificationStatus({hostVersion:` + string(versionJSON) + `,corvintBinary:` + string(nodeJSON) + `,environment:process.env,root:` + string(rootJSON) + `})))`
		}
		status := func(version string) Object {
			out, e := capture(ctx, root, nil, []string{node, "--input-type=module", "-e", program(version)})
			if e != nil {
				t.Fatal(e)
			}
			x, e := decode([]byte(out))
			if e != nil {
				t.Fatal(e)
			}
			return x
		}
		got := status("2.0.19")
		if got["integrationSupport"] != "FULL" || got["executionAuthority"] != "NONE" || got["frontier"] != "UNAVAILABLE" || got["legacyReceiptSupport"] != "FALLBACK" {
			t.Fatal(got)
		}
		if got["supportedHostRange"] != SupportedHostRange || got["hostVersion"] != "2.0.19" {
			t.Fatal(got)
		}
		for _, version := range []string{"2.1.0", "2.0.019", "02.00.19", "2.0.9999999999999999999", "2.0.19-beta"} {
			unsupported := status(version)
			if unsupported["integrationSupport"] != "UNQUALIFIED" || unsupported["reason"] != "unsupported-host-version" || !strings.Contains(str(unsupported["qualificationAction"]), "go run ./tools/qualify-opencode") {
				t.Fatalf("%s: %v", version, unsupported)
			}
		}
		record["hostSHA256"] = strings.Repeat("0", 64)
		if e := writeRecord(ctx, p, record, nil); e != nil {
			t.Fatal(e)
		}
		if status("2.0.19")["integrationSupport"] != "UNQUALIFIED" {
			t.Fatal("changed image qualified")
		}
		record["hostSHA256"] = id.HostSHA256
		if e := writeRecord(ctx, p, record, nil); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(pkg, "README.md"), []byte("changed"), 0600); e != nil {
			t.Fatal(e)
		}
		if status("2.0.19")["integrationSupport"] != "UNQUALIFIED" {
			t.Fatal("changed package qualified")
		}
	})
}
