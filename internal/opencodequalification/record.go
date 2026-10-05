// Package opencodequalification produces local maintainer evidence, never execution authority.
package opencodequalification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
)

const Profile = "opencode-native-integration/1"
const recordLimit = 131072
const SupportedHostRange = ">=2.0.18 <2.1.0"

var focusedTests = []string{"TestHostAdapterJavaScriptHarnessInterruption", "TestHostAdapterJavaScriptHosts"}
var cases = map[string][]string{
	"install":  {"native-discovery", "native-host-image", "direct-install", "uninstall"},
	"snapshot": {"snapshot", "oversized-exclusion"}, "context": {"awaited-current-prompt", "native-query", "bounded-prompt", "governance"},
	"expansion": {"native-exact-expansion"}, "observations": {"native-edit", "verification-observation", "explicit-outcome", "supplied-evidence"},
	"frontier": {"advisory-completion", "bounded-stops"}, "degradation": {}, "privacy": {"environment-filter"}, "normalization": {},
	"recursion": {"bounded-stops"}, "compaction": {"native-compaction", "dirty-path-recovery"}, "latency": {"timed-delivery", "latency"},
	"recall": {"critical-recall-and-bytes"}, "cleanup": {"interruption-cleanup", "native-exit"},
}

type Object map[string]any

func object(v any) Object {
	if x, ok := v.(map[string]any); ok {
		return x
	}
	if x, ok := v.(Object); ok {
		return x
	}
	return Object{}
}
func array(v any) []any { x, _ := v.([]any); return x }
func str(v any) string  { x, _ := v.(string); return x }
func number(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		n, e := x.Float64()
		if e == nil {
			return n
		}
	}
	return math.NaN()
}
func truth(v any) bool { b, ok := v.(bool); return ok && b }

func ParseSupportedHostVersion(output string) (string, error) {
	version := strings.TrimSpace(output)
	version = strings.TrimPrefix(version, "opencode v")
	parts := strings.Split(version, ".")
	if len(parts) != 3 || parts[0] != "2" || parts[1] != "0" || !canonicalDecimal(parts[2]) || len(parts[2]) > 18 {
		return "", fmt.Errorf("unsupported OpenCode version %q; supported range is %s", strings.TrimSpace(output), SupportedHostRange)
	}
	if len(parts[2]) < 2 || (len(parts[2]) == 2 && parts[2] < "18") {
		return "", fmt.Errorf("unsupported OpenCode version %q; supported range is %s", strings.TrimSpace(output), SupportedHostRange)
	}
	return version, nil
}

func canonicalDecimal(value string) bool {
	if value == "0" {
		return true
	}
	if value == "" || value[0] < '1' || value[0] > '9' {
		return false
	}
	for _, r := range value[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func equal(a, b any) bool {
	x, e := json.Marshal(a)
	y, f := json.Marshal(b)
	return e == nil && f == nil && string(x) == string(y)
}
func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func digest(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// maxEvidenceBytes bounds every host- or harness-produced file this package reads.
const maxEvidenceBytes = 16 << 20

var errEvidenceBound = errors.New("evidence stream exceeds bound")

// readBounded reads at most limit bytes, refusing a longer file after limit+1 bytes
// instead of allocating all of it (V1-0747).
func readBounded(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, errEvidenceBound
	}
	return b, nil
}

// decode admits any JSON object. Evidence rows and reports are open Object maps by
// design: their producers (the OpenCode host, the harness and the gate probes) may add
// fields, so unknown members are accepted, and every gate reads only the fields it names.
func decode(b []byte) (Object, error) {
	var x Object
	e := json.Unmarshal(b, &x)
	if e == nil && x == nil {
		e = errors.New("expected object")
	}
	return x, e
}
func readObject(path string) (Object, error) {
	b, e := readBounded(path, maxEvidenceBytes)
	if e != nil {
		return nil, e
	}
	return decode(b)
}
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func NodeArchitecture(machine string) (string, error) {
	switch strings.ToLower(machine) {
	case "amd64", "x86_64", "x64":
		return "x64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	}
	return "", fmt.Errorf("unsupported qualification architecture: %s", machine)
}

type Identity struct {
	SourceCommit              string            `json:"sourceCommit"`
	SourceFiles               map[string]string `json:"sourceFiles"`
	HostSHA256                string            `json:"hostSHA256"`
	CorvintSHA256             string            `json:"corvintSHA256"`
	HarnessSHA256             string            `json:"harnessSHA256"`
	QualificationBinarySHA256 string            `json:"qualificationBinarySHA256"`
	Inputs                    map[string]string `json:"inputs"`
	Tuple                     map[string]string `json:"tuple"`
}

func packageFiles(source string) (map[string]string, error) {
	out := map[string]string{}
	e := filepath.WalkDir(filepath.Join(source, "integrations/opencode"), func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type().IsRegular() {
			rel, e := filepath.Rel(source, p)
			if e != nil {
				return e
			}
			h, e := digest(p)
			if e != nil {
				return e
			}
			out[filepath.ToSlash(rel)] = h
		}
		return nil
	})
	return out, e
}

func requireCleanCheckout(ctx context.Context, source string) error {
	status, e := capture(ctx, source, nil, []string{"git", "status", "--porcelain", "--untracked-files=all"})
	if e != nil {
		return e
	}
	if strings.TrimSpace(status) != "" {
		return errors.New("qualification requires a clean committed checkout")
	}
	return nil
}

func identities(ctx context.Context, c Config, clean bool) (Identity, error) {
	var id Identity
	if clean {
		if e := requireCleanCheckout(ctx, c.Source); e != nil {
			return id, e
		}
	}
	var e error
	id.SourceCommit, e = capture(ctx, c.Source, nil, []string{"git", "rev-parse", "HEAD"})
	if e != nil {
		return id, e
	}
	id.SourceCommit = strings.TrimSpace(id.SourceCommit)
	id.SourceFiles, e = packageFiles(c.Source)
	if e != nil {
		return id, e
	}
	for path, target := range map[string]*string{c.Host: &id.HostSHA256, c.Corvint: &id.CorvintSHA256, c.Self: &id.QualificationBinarySHA256} {
		*target, e = digest(path)
		if e != nil {
			return id, e
		}
	}
	names, e := capture(ctx, c.Source, nil, []string{"git", "ls-files", "integrations", "conformance/harness-event-v0", "cmd/corvint/host_adapter_javascript_test.go", "internal/opencodequalification", "internal/procgroup", "internal/liveverify/processidentity", "tools/qualify-opencode", "go.mod", "go.sum"})
	if e != nil {
		return id, e
	}
	id.Inputs = map[string]string{}
	collector := map[string]string{}
	for _, p := range strings.Fields(names) {
		h, e := digest(filepath.Join(c.Source, p))
		if e != nil {
			return id, e
		}
		id.Inputs[p] = h
		if strings.HasPrefix(p, "internal/") || strings.HasPrefix(p, "tools/") || strings.HasPrefix(p, "go.") {
			collector[p] = h
		}
	}
	b, e := json.Marshal(collector)
	if e != nil {
		return id, e
	}
	id.HarnessSHA256 = hash(b)
	manifest, e := readObject(filepath.Join(c.Source, "integrations/opencode/package.json"))
	if e != nil {
		return id, e
	}
	arch, e := NodeArchitecture(runtime.GOARCH)
	if e != nil {
		return id, e
	}
	if _, e = ParseSupportedHostVersion(c.HostVersion); e != nil {
		return id, e
	}
	id.Tuple = map[string]string{"hostVersion": c.HostVersion, "adapterVersion": str(manifest["version"]), "os": runtime.GOOS, "architecture": arch}
	return id, nil
}
func identityFields(id Identity) Object {
	return Object{"sourceCommit": id.SourceCommit, "sourceFiles": id.SourceFiles, "hostSHA256": id.HostSHA256, "corvintSHA256": id.CorvintSHA256, "harnessSHA256": id.HarnessSHA256, "qualificationBinarySHA256": id.QualificationBinarySHA256}
}
func BuildRecord(native, focused Object, before, after Identity) (Object, error) {
	fail := func(s string) (Object, error) { return nil, errors.New(s) }
	if !reflect.DeepEqual(before, after) {
		return fail("qualification inputs changed during the gates")
	}
	if native["profile"] != Profile || native["result"] != "PASS" {
		return fail("native campaign did not pass")
	}
	if native["executionAuthority"] != "NONE" || native["frontier"] != "UNAVAILABLE" || native["legacyReceiptSupport"] != "FALLBACK" {
		return fail("authority boundary mismatch")
	}
	for k, v := range identityFields(before) {
		if !equal(native[k], v) {
			return fail("native identity mismatch: " + k)
		}
	}
	for k, v := range before.Tuple {
		if native[k] != v {
			return fail("native tuple mismatch")
		}
	}
	checks := object(native["checks"])
	if !truth(checks["frozen-identities"]) {
		return fail("collector identities changed")
	}
	for _, names := range cases {
		for _, name := range names {
			if !truth(checks[name]) {
				return fail("missing or failed native check: " + name)
			}
		}
	}
	if number(focused["exitCode"]) != 0 {
		return fail("focused suite did not pass")
	}
	seen := map[string]map[string]bool{"run": {}, "pass": {}}
	for _, line := range strings.Split(str(focused["stdout"]), "\n") {
		if line == "" {
			continue
		}
		ev, e := decode([]byte(line))
		if e != nil {
			return fail("malformed focused event")
		}
		a := str(ev["Action"])
		if a == "fail" || a == "skip" {
			return fail("focused suite failed or skipped")
		}
		if ev["Package"] == "github.com/Beamfall/corvint/cmd/corvint" && seen[a] != nil {
			seen[a][str(ev["Test"])] = true
		}
	}
	for action, tests := range seen {
		for _, test := range focusedTests {
			if !tests[test] {
				return fail("focused suite missing required test: " + action)
			}
		}
	}
	metrics := object(native["metrics"])
	for key, limit := range map[string]float64{"query": 500, "lifecycle": 250} {
		values := array(metrics[key])
		if len(values) < 20 {
			return fail("invalid latency samples")
		}
		nums := make([]float64, len(values))
		for i, v := range values {
			nums[i] = number(v)
			if math.IsNaN(nums[i]) || math.IsInf(nums[i], 0) || nums[i] < 0 {
				return fail("invalid latency samples")
			}
		}
		p := p95(nums)
		if p != number(metrics[key+"P95Ms"]) || p > limit {
			return fail("latency gate failed")
		}
	}
	recall := object(metrics["criticalRecall"])
	total := number(recall["total"])
	if !(total > 0 && number(recall["injected"]) == total && number(recall["manual"]) == total && number(metrics["injectedBytes"]) > 0 && number(metrics["injectedBytes"]) < number(metrics["manualBytes"])) {
		return fail("recall/bytes gate failed")
	}
	cleanup := object(native["cleanupEvidence"])
	survivors, ok := cleanup["survivors"].([]any)
	if !truth(cleanup["absentBeforeSupervisorCleanup"]) || number(cleanup["exit"]) != 143 || len(array(cleanup["observedDescendants"])) == 0 || !ok || len(survivors) != 0 {
		return fail("cleanup evidence incomplete")
	}
	out := Object{}
	for k, v := range native {
		out[k] = v
	}
	conformance, evidence := Object{}, Object{}
	for name, names := range cases {
		conformance[name] = "PASS"
		evidence[name] = Object{"nativeChecks": names, "focusedTests": focusedTests}
	}
	out["conformance"] = conformance
	out["conformanceEvidence"] = evidence
	out["qualificationInputs"] = before.Inputs
	out["focusedSuite"] = Object{"exitCode": 0, "tests": focusedTests, "stdoutSHA256": hash([]byte(str(focused["stdout"]))), "stderrSHA256": hash([]byte(str(focused["stderr"])))}
	return out, nil
}
func p95(values []float64) float64 {
	if len(values) == 0 {
		return math.Inf(1)
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	return v[int(math.Ceil(float64(len(v))*.95))-1]
}
func writeRecord(ctx context.Context, path string, record Object, beforePublish func()) error {
	b, e := json.MarshalIndent(record, "", "  ")
	if e != nil {
		return e
	}
	b = append(b, '\n')
	if len(b) > recordLimit {
		return errors.New("qualification record exceeds consumer bound")
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".opencode-qualification-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, e = f.Write(b); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if beforePublish != nil {
		beforePublish()
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	return os.Rename(f.Name(), path)
}
func beginRecord(ctx context.Context, path, backup string) error {
	info, e := os.Lstat(path)
	if e == nil {
		if !info.Mode().IsRegular() || info.Size() > recordLimit {
			return errors.New("invalid previous qualification record")
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if e = os.WriteFile(backup, b, 0600); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	return writeRecord(ctx, path, Object{"profile": Profile, "result": "INCOMPLETE", "reason": "qualification-in-progress"}, nil)
}
