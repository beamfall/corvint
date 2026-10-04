package tracerecordrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/trace"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func aggregateTestExpected() AggregateExpected {
	return AggregateExpected{ObjectFormat: "sha1", Base: strings.Repeat("1", 40), Target: strings.Repeat("2", 40),
		Tree: strings.Repeat("3", 40), AdmissionPolicy: AggregateAdmissionPolicy,
		Task: "Local completion " + strings.Repeat("a", 64), Verification: []string{"go test ./internal/parser"}, Outcome: "passed"}
}

func aggregateTestPaths(n int) []string {
	paths := make([]string, n)
	for i := range paths {
		paths[i] = fmt.Sprintf("source/f%04d.go", i)
	}
	return paths
}

func aggregateTestReceipt(t *testing.T) (AggregateOutcome, []byte) {
	t.Helper()
	paths := aggregateTestPaths(201)
	v, err := newAggregateOutcome(aggregateTestExpected(), paths, paths)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeAggregateOutcome(v)
	if err != nil {
		t.Fatal(err)
	}
	return v, raw
}

// ALO-V0-005/006: independent standard-JSON ASCII oracle pins both hash domains
// and the omission of envelopeSha256 from its own preimage.
func TestAggregateOutcomeWireAndDigest(t *testing.T) {
	v, raw := aggregateTestReceipt(t)
	got, err := ParseAggregateOutcome(raw)
	if err != nil || !reflect.DeepEqual(got, v) {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	var members map[string]any
	if err := json.Unmarshal(raw, &members); err != nil {
		t.Fatal(err)
	}
	delete(members, "envelopeSha256")
	preimage, err := json.Marshal(members)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(append([]byte("corvint-dogfood-aggregate-outcome/v0\x00"), preimage...))
	if v.EnvelopeSHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatal("envelope preimage/domain mismatch")
	}
	pathPreimage := []byte("corvint-record-path-admission/v0\x00")
	for _, p := range v.Candidates {
		pathPreimage = append(pathPreimage, []byte(p+"\x00")...)
	}
	sum = sha256.Sum256(pathPreimage)
	if v.CandidateSHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatal("path preimage/domain mismatch")
	}
	for _, name := range []string{"profile", "state", "candidates", "admitted"} {
		delete(members, name)
	}
	preimage, _ = json.Marshal(members)
	sum = sha256.Sum256(append([]byte("corvint-dogfood-aggregate-binding/v0\x00"), preimage...))
	binding, err := AggregateBinding(v)
	if err != nil || binding != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("binding preimage/domain mismatch: %s %v", binding, err)
	}
}

// ALO-V0-005/007: structural corruption and self-consistent metadata substitution
// cannot be accepted as an enrolled observation.
func TestAggregateOutcomeRejectsMutations(t *testing.T) {
	_, raw := aggregateTestReceipt(t)
	mutations := map[string]func(map[string]any){
		"missing":       func(v map[string]any) { delete(v, "tree") },
		"unknown":       func(v map[string]any) { v["mutates"] = false },
		"profile":       func(v map[string]any) { v["profile"] = "corvint-dogfood-aggregate-outcome/1" },
		"legacy-state":  func(v map[string]any) { v["state"] = "recorded" },
		"object-format": func(v map[string]any) { v["objectFormat"] = "sha256" },
		"count":         func(v map[string]any) { v["candidateCount"] = float64(202) },
		"float-count":   func(v map[string]any) { v["candidateCount"] = 201.5 },
		"null":          func(v map[string]any) { v["verification"] = nil },
		"digest":        func(v map[string]any) { v["admittedSha256"] = "sha256:" + strings.Repeat("0", 64) },
		"omitted-set":   func(v map[string]any) { v["admitted"] = v["admitted"].([]any)[1:] },
		"reordered-set": func(v map[string]any) { a := v["candidates"].([]any); a[0], a[1] = a[1], a[0] },
		"duplicate-set": func(v map[string]any) { a := v["admitted"].([]any); a[1] = a[0] },
		"foreign-path":  func(v map[string]any) { a := v["admitted"].([]any); a[0] = "foreign.go" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			if err := json.Unmarshal(raw, &v); err != nil {
				t.Fatal(err)
			}
			mutate(v)
			b, _ := json.Marshal(v)
			if _, err := ParseAggregateOutcome(append(b, '\n')); err == nil {
				t.Fatal("mutation accepted")
			}
		})
	}
	for _, b := range [][]byte{bytes.TrimSuffix(raw, []byte{'\n'}), append([]byte{' '}, raw...), append(raw, '\n'),
		bytes.Replace(raw, []byte(`"state":"aggregate-recorded"`), []byte(`"state":"aggregate-recorded","state":"aggregate-recorded"`), 1)} {
		if _, err := ParseAggregateOutcome(b); err == nil {
			t.Fatal("noncanonical/duplicate wire accepted")
		}
	}
	value, _ := aggregateTestReceipt(t)
	value.Task = "Another valid task"
	value.EnvelopeSHA256, _ = aggregateEnvelope(value)
	forged, _ := encodeAggregateOutcome(value)
	if _, err := ParseAggregateOutcome(forged); err != nil {
		t.Fatalf("control is not self-consistent: %v", err)
	}
	if _, err := VerifyAggregateOutcome(context.Background(), t.TempDir(), aggregateTestExpected(), forged); err == nil || err.Error() != "aggregate-binding-drift" {
		t.Fatalf("foreign self-consistent task must fail before source lookup: %v", err)
	}
}

// ALO-V0-004: path count is independent from encoded-byte admission.
func TestAggregateOutcomeBounds(t *testing.T) {
	for _, n := range []int{200, 201, 299, 512, 513} {
		p := aggregateTestPaths(n)
		_, err := newAggregateOutcome(aggregateTestExpected(), p, p)
		if (n > 200 && n <= 512) != (err == nil) {
			t.Fatalf("n=%d: %v", n, err)
		}
	}
	for _, n := range []int{AggregateReceiptLimit, AggregateReceiptLimit + 1} {
		_, err := ParseAggregateOutcome(bytes.Repeat([]byte{' '}, n))
		if err == nil || (err.Error() == "aggregate-receipt-byte-limit") != (n > AggregateReceiptLimit) {
			t.Fatalf("byte admission n=%d err=%v", n, err)
		}
	}
	expected := aggregateTestExpected()
	expected.Task = strings.Repeat("a", 2001)
	if _, err := newAggregateOutcome(expected, aggregateTestPaths(201), aggregateTestPaths(201)); err == nil {
		t.Fatal("oversize task accepted")
	}
	for _, n := range []int{512, 513} {
		expected = aggregateTestExpected()
		expected.Verification = []string{strings.Repeat("a", n)}
		_, err := newAggregateOutcome(expected, aggregateTestPaths(201), aggregateTestPaths(201))
		if (n == 512) != (err == nil) {
			t.Fatalf("command length=%d: %v", n, err)
		}
	}
}

// ALO-V0-001/002/003/007: real immutable Git inputs produce and independently
// revalidate all 201 admitted paths, preserving current source policy.
func TestAggregateOutcomeCompleteImmutableSource(t *testing.T) {
	oldOpen, oldRecover := openTraceStore, recoverTraceAppend
	calls := 0
	openTraceStore = func(string, map[string]trace.Revision, func() error) (*trace.Store, error) {
		calls++
		return nil, fmt.Errorf("unexpected trace store open")
	}
	recoverTraceAppend = func(string, func() error, func() error) error {
		calls++
		return fmt.Errorf("unexpected trace recovery")
	}
	t.Cleanup(func() {
		openTraceStore, recoverTraceAppend = oldOpen, oldRecover
		if calls != 0 {
			t.Errorf("aggregate attempted %d trace store operations", calls)
		}
	})
	root := newAdapterFixture(t)
	base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	for i, path := range aggregateTestPaths(201) {
		writeAdapterFixture(t, root, path, fmt.Sprintf("package source\nconst V%d = %d\n", i, i))
	}
	gitAdapterFixture(t, root, "add", "source")
	gitAdapterFixture(t, root, "commit", "-qm", "aggregate fixture")
	expected := aggregateTestExpected()
	expected.Base = base
	expected.Target = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	expected.Tree = gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
	raw, err := ProduceAggregateOutcome(context.Background(), root, expected)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyAggregateOutcome(context.Background(), root, expected, raw)
	if err != nil || got.AdmittedCount != 201 {
		t.Fatalf("complete validation: count=%d err=%v", got.AdmittedCount, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".context-corvint")); !os.IsNotExist(err) {
		t.Fatalf("aggregate created trace directory: %v", err)
	}
	writeAdapterFixture(t, root, "source/f0000.go", "package source\nconst Changed = true\n")
	if _, err := VerifyAggregateOutcome(context.Background(), root, expected, raw); err == nil {
		t.Fatal("dirty source accepted")
	}
}

func TestAggregateChangedPathsPhysicalBounds(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("Unix executable fixture")
	}
	bin := t.TempDir()
	payload := filepath.Join(bin, "names")
	script := "#!/bin/sh\nexec /bin/cat \"" + payload + "\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, n := range []int{4096, 4097} {
		raw := []byte(strings.Join(aggregateTestPaths(n), "\x00") + "\x00")
		if err := os.WriteFile(payload, raw, 0600); err != nil {
			t.Fatal(err)
		}
		budget, _ := gitrun.NewOperationBudget(64, time.Now().Add(time.Minute), false)
		paths, err := aggregateChangedPaths(gitrun.WithOperationBudget(context.Background(), budget), bin, "base", "target")
		if n == 4096 && (err != nil || len(paths) != n) {
			t.Fatalf("4096: %d %v", len(paths), err)
		}
		if n == 4097 && (err == nil || err.Error() != "aggregate-candidate-limit") {
			t.Fatalf("4097: %v", err)
		}
		if budget.Used() != 1 {
			t.Fatalf("spawn count %d", budget.Used())
		}
	}
	for _, n := range []int{AggregateReceiptLimit, AggregateReceiptLimit + 1, AggregateReceiptLimit * 2} {
		raw := bytes.Repeat([]byte("a"), n)
		raw[n-1] = 0
		if err := os.WriteFile(payload, raw, 0600); err != nil {
			t.Fatal(err)
		}
		budget, _ := gitrun.NewOperationBudget(64, time.Now().Add(time.Minute), false)
		paths, err := aggregateChangedPaths(gitrun.WithOperationBudget(context.Background(), budget), bin, "base", "target")
		if n == AggregateReceiptLimit && (err != nil || len(paths) != 1) {
			t.Fatalf("exact byte boundary: %v", err)
		}
		if n > AggregateReceiptLimit && err == nil {
			t.Fatalf("accepted %d bytes", n)
		}
	}
}

func TestAggregateOutcomeExactEncodedByteBoundary(t *testing.T) {
	candidates := append(aggregateTestPaths(201), "z.md")
	admitted := aggregateTestPaths(201)
	value, err := newAggregateOutcome(aggregateTestExpected(), candidates, admitted)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeAggregateOutcome(value)
	if err != nil {
		t.Fatal(err)
	}
	padding := AggregateReceiptLimit - len(raw)
	for _, extra := range []int{0, 1} {
		candidates[len(candidates)-1] = "z" + strings.Repeat("a", padding+extra) + ".md"
		value, err = newAggregateOutcome(aggregateTestExpected(), candidates, admitted)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := aggregateCanonical(value)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(canonical, '\n')
		if len(raw) != AggregateReceiptLimit+extra {
			t.Fatalf("boundary fixture=%d", len(raw))
		}
		_, err = ParseAggregateOutcome(raw)
		if extra == 0 && err != nil {
			t.Fatalf("exact valid canonical receipt: %v", err)
		}
		if extra == 1 && (err == nil || err.Error() != "aggregate-receipt-byte-limit") {
			t.Fatalf("plus-one canonical receipt: %v", err)
		}
		_, err = encodeAggregateOutcome(value)
		if (extra == 0) != (err == nil) {
			t.Fatalf("encoder boundary: %v", err)
		}
	}
}

func TestAggregateOutcomeMetadataBounds(t *testing.T) {
	paths := aggregateTestPaths(201)
	for _, n := range []int{2000, 2001} {
		expected := aggregateTestExpected()
		expected.Task = strings.Repeat("a", n)
		_, err := newAggregateOutcome(expected, paths, paths)
		if (n == 2000) != (err == nil) {
			t.Fatalf("task %d: %v", n, err)
		}
	}
	for _, n := range []int{50, 51} {
		expected := aggregateTestExpected()
		expected.Verification = nil
		for i := 0; i < n; i++ {
			expected.Verification = append(expected.Verification, fmt.Sprintf("test command %02d", i))
		}
		_, err := newAggregateOutcome(expected, paths, paths)
		if (n == 50) != (err == nil) {
			t.Fatalf("verification count %d: %v", n, err)
		}
	}
}

func TestAggregateOutcomeNativeAdmissionSizes(t *testing.T) {
	for _, n := range []int{299, 512, 513} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			root := newAdapterFixture(t)
			base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
			for i, path := range aggregateTestPaths(n) {
				writeAdapterFixture(t, root, path, fmt.Sprintf("package source\nconst V%d = %d\n", i, i))
			}
			gitAdapterFixture(t, root, "add", "source")
			gitAdapterFixture(t, root, "commit", "-qm", "aggregate boundary")
			expected := aggregateTestExpected()
			expected.Base = base
			expected.Target = gitAdapterFixture(t, root, "rev-parse", "HEAD")
			expected.Tree = gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
			raw, err := ProduceAggregateOutcome(context.Background(), root, expected)
			if n == 513 {
				if err == nil || trace.AdmissionFailureReason(err) != "aggregate-admitted-limit" {
					t.Fatalf("513: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := VerifyAggregateOutcome(context.Background(), root, expected, raw)
			if err != nil || got.AdmittedCount != n {
				t.Fatalf("native count %d got %d: %v", n, got.AdmittedCount, err)
			}
			// A self-consistent proper subset still disagrees with current authority.
			forged, err := newAggregateOutcome(expected, got.Candidates[1:], got.Admitted[1:])
			if err != nil {
				t.Fatal(err)
			}
			altered, err := encodeAggregateOutcome(forged)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = VerifyAggregateOutcome(context.Background(), root, expected, altered); err == nil || err.Error() != "aggregate-source-set-drift" {
				t.Fatalf("self-consistent omission: %v", err)
			}
		})
	}
}

func TestAggregateOutcomeCurrentSourcePolicy(t *testing.T) {
	root := newAdapterFixture(t)
	writeAdapterFixture(t, root, ".gitignore", "ignored-local.go\n")
	writeAdapterFixture(t, root, "deleted.go", "package old\n")
	gitAdapterFixture(t, root, "add", ".")
	gitAdapterFixture(t, root, "commit", "-qm", "policy baseline")
	base := gitAdapterFixture(t, root, "rev-parse", "HEAD")
	admitted := aggregateTestPaths(201)
	for i, path := range admitted {
		writeAdapterFixture(t, root, path, fmt.Sprintf("package source\nconst V%d = %d\n", i, i))
	}
	excluded := map[string]string{
		"generated-output.go":  "// Code generated by fixture DO NOT EDIT.\npackage generated\n",
		"pointer.go":           "version https://git-lfs.github.com/spec/v1\noid sha256:" + strings.Repeat("a", 64) + "\nsize 100\n",
		"binary.go":            "package binary\x00\xff",
		"unsupported.csv":      "a,b,c\n",
		"vendor/dependency.go": "package dependency\n",
		"huge.go":              "package huge\n//" + strings.Repeat("x", 1000001),
	}
	for path, content := range excluded {
		writeAdapterFixture(t, root, path, content)
	}
	if err := os.Remove(filepath.Join(root, "deleted.go")); err != nil {
		t.Fatal(err)
	}
	gitAdapterFixture(t, root, "add", ".")
	gitAdapterFixture(t, root, "commit", "-qm", "policy target")
	writeAdapterFixture(t, root, "ignored-local.go", "package ignored\n")
	expected := aggregateTestExpected()
	expected.Base = base
	expected.Target = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	expected.Tree = gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
	raw, err := ProduceAggregateOutcome(context.Background(), root, expected)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyAggregateOutcome(context.Background(), root, expected, raw)
	if err != nil {
		t.Fatal(err)
	}
	candidates := append(append([]string{}, admitted...), "deleted.go")
	for path := range excluded {
		candidates = append(candidates, path)
	}
	sort.Strings(candidates)
	// Current trace path authority includes every index.Sources key, including
	// a retained source with Valid=false. Aggregate must preserve that policy;
	// parser validity is not an additional path-admission rule.
	admitted = append(admitted, "binary.go")
	sort.Strings(admitted)
	if !reflect.DeepEqual(got.Admitted, admitted) || !reflect.DeepEqual(got.Candidates, candidates) {
		t.Fatalf("full source policy admitted=%v candidates=%v", got.Admitted, got.Candidates)
	}
}

// These acquisition tests also pin the aggregate path's no-trace-write boundary.
func aggregateQualificationTraceGuard(t *testing.T, root string) {
	t.Helper()
	oldOpen, oldRecover := openTraceStore, recoverTraceAppend
	calls := 0
	openTraceStore = func(string, map[string]trace.Revision, func() error) (*trace.Store, error) {
		calls++
		return nil, fmt.Errorf("unexpected trace store open")
	}
	recoverTraceAppend = func(string, func() error, func() error) error {
		calls++
		return fmt.Errorf("unexpected trace recovery")
	}
	path := filepath.Join(root, ".git", "aggregate-qualification.trace")
	before := []byte("existing private trace evidence\n")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		openTraceStore, recoverTraceAppend = oldOpen, oldRecover
		after, err := os.ReadFile(path)
		if calls != 0 || err != nil || sha256.Sum256(after) != sha256.Sum256(before) {
			t.Errorf("trace boundary: calls=%d err=%v changed=%v", calls, err, !bytes.Equal(before, after))
		}
	})
}

func TestAggregateOutcomeChangedTrackedIgnorePolicy(t *testing.T) {
	root := newAdapterFixture(t)
	aggregateQualificationTraceGuard(t, root)
	writeAdapterFixture(t, root, ".gitignore", "old-root-local.go\n")
	writeAdapterFixture(t, root, "config/local/.gitignore", "old-nested-local.go\n")
	gitAdapterFixture(t, root, "add", ".")
	gitAdapterFixture(t, root, "commit", "-qm", "tracked ignore baseline")
	expected := aggregateTestExpected()
	expected.Base = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	paths := aggregateTestPaths(201)
	for i, path := range paths {
		writeAdapterFixture(t, root, path, fmt.Sprintf("package source\nconst V%d = %d\n", i, i))
	}
	writeAdapterFixture(t, root, ".gitignore", "ignored-root-local.go\n")
	writeAdapterFixture(t, root, "config/local/.gitignore", "ignored-nested-local.go\n")
	gitAdapterFixture(t, root, "add", ".")
	gitAdapterFixture(t, root, "commit", "-qm", "changed tracked ignore policy")
	expected.Target = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	expected.Tree = gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
	writeAdapterFixture(t, root, "ignored-root-local.go", "package ignored\n")
	writeAdapterFixture(t, root, "config/local/ignored-nested-local.go", "package ignored\n")
	want := append(append([]string{}, paths...), ".gitignore", "config/local/.gitignore")
	sort.Strings(want)
	raw, err := ProduceAggregateOutcome(context.Background(), root, expected)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyAggregateOutcome(context.Background(), root, expected, raw)
	if err != nil || got.CandidateCount != 203 || got.AdmittedCount != 203 || !reflect.DeepEqual(got.Candidates, want) || !reflect.DeepEqual(got.Admitted, want) {
		t.Fatalf("complete changed-ignore authority: %+v err=%v", got, err)
	}
	preimage := []byte("corvint-record-path-admission/v0\x00")
	for _, path := range want {
		preimage = append(preimage, []byte(path+"\x00")...)
	}
	sum := sha256.Sum256(preimage)
	if got.CandidateSHA256 != fmt.Sprintf("sha256:%x", sum) || got.AdmittedSHA256 != fmt.Sprintf("sha256:%x", sum) {
		t.Fatal("independent path digest mismatch")
	}
	index, _, err := recordIndex(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".gitignore", "config/local/.gitignore", "ignored-root-local.go", "config/local/ignored-nested-local.go"} {
		if _, exists := index.Sources[path]; exists {
			t.Fatalf("policy-only/ignored file indexed: %s", path)
		}
	}
}

func TestAggregateOutcomeSecretChangedPathRefusal(t *testing.T) {
	root := newAdapterFixture(t)
	aggregateQualificationTraceGuard(t, root)
	expected := aggregateTestExpected()
	expected.Base = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	paths := aggregateTestPaths(201)
	for i, path := range paths {
		writeAdapterFixture(t, root, path, fmt.Sprintf("package source\nconst V%d = %d\n", i, i))
	}
	writeAdapterFixture(t, root, "source/token=abcdefghijklmnopqrstuvwxyz.go", "package source\n")
	gitAdapterFixture(t, root, "add", "source")
	gitAdapterFixture(t, root, "commit", "-qm", "secret-shaped changed path")
	expected.Target = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	expected.Tree = gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
	raw, err := ProduceAggregateOutcome(context.Background(), root, expected)
	if len(raw) != 0 || trace.AdmissionFailureReason(err) != "secret-shaped-path" {
		t.Fatalf("producer raw=%d err=%v reason=%s", len(raw), err, trace.AdmissionFailureReason(err))
	}
	omitted, err := newAggregateOutcome(expected, paths, paths)
	if err != nil {
		t.Fatal(err)
	}
	forged, err := encodeAggregateOutcome(omitted)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseAggregateOutcome(forged); err != nil {
		t.Fatalf("self-consistent omitted control: %v", err)
	}
	_, err = VerifyAggregateOutcome(context.Background(), root, expected, forged)
	if trace.AdmissionFailureReason(err) != "secret-shaped-path" {
		t.Fatalf("verifier failed to reacquire omitted path: %v", err)
	}
}

// The literal table is independent of aggregateMembers; adding a wire member
// requires extending this oracle instead of silently extending coverage.
func TestAggregateOutcomeAllMemberShapes(t *testing.T) {
	members := []string{"admissionPolicy", "admitted", "admittedCount", "admittedSha256", "base", "candidateCount", "candidateSha256", "candidates", "envelopeSha256", "objectFormat", "outcome", "profile", "state", "target", "task", "tree", "verification"}
	if !reflect.DeepEqual(members, aggregateMembers) {
		t.Fatal("closed schema changed; revise frozen oracle")
	}
	_, raw := aggregateTestReceipt(t)
	for _, member := range members {
		for _, shape := range []string{"missing", "null", "wrong-type"} {
			// These exact cells already have accepted witnesses.
			if member == "tree" && shape == "missing" || member == "verification" && shape == "null" {
				continue
			}
			t.Run(member+"/"+shape, func(t *testing.T) {
				var value map[string]any
				if err := json.Unmarshal(raw, &value); err != nil {
					t.Fatal(err)
				}
				switch shape {
				case "missing":
					delete(value, member)
				case "null":
					value[member] = nil
				case "wrong-type":
					value[member] = true
				}
				mutant, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = ParseAggregateOutcome(append(mutant, '\n')); err == nil {
					t.Fatal("closed member mutation accepted")
				}
			})
		}
	}
	for member, wrong := range map[string]any{"admissionPolicy": "foreign/0", "base": strings.Repeat("4", 40), "target": strings.Repeat("5", 40), "tree": strings.Repeat("6", 40), "task": "Foreign valid task", "verification": []string{"go test ./other"}, "outcome": "failed", "admittedCount": 202, "candidateSha256": "sha256:" + strings.Repeat("0", 64), "envelopeSha256": "sha256:" + strings.Repeat("0", 64)} {
		t.Run(member+"/same-type", func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			value[member] = wrong
			mutant, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ParseAggregateOutcome(append(mutant, '\n')); err == nil {
				t.Fatal("same-type mutation accepted")
			}
		})
	}
}

func TestAggregateOutcomeAllExpectedBindings(t *testing.T) {
	_, raw := aggregateTestReceipt(t)
	mutations := map[string]func(*AggregateExpected){
		"objectFormat":    func(v *AggregateExpected) { v.ObjectFormat = "sha256" },
		"base":            func(v *AggregateExpected) { v.Base = strings.Repeat("4", 40) },
		"target":          func(v *AggregateExpected) { v.Target = strings.Repeat("5", 40) },
		"tree":            func(v *AggregateExpected) { v.Tree = strings.Repeat("6", 40) },
		"admissionPolicy": func(v *AggregateExpected) { v.AdmissionPolicy = "foreign/0" },
		"task":            func(v *AggregateExpected) { v.Task = "Foreign valid task" },
		"verification":    func(v *AggregateExpected) { v.Verification = []string{"go test ./other"} },
		"outcome":         func(v *AggregateExpected) { v.Outcome = "failed" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			expected := aggregateTestExpected()
			mutate(&expected)
			// The receipt is independently well-formed; an empty root proves this is
			// an enrollment comparison rather than a later source-acquisition failure.
			if _, err := ParseAggregateOutcome(raw); err != nil {
				t.Fatal(err)
			}
			if _, err := VerifyAggregateOutcome(context.Background(), t.TempDir(), expected, raw); err == nil || err.Error() != "aggregate-binding-drift" {
				t.Fatalf("expected authority %s: %v", name, err)
			}
		})
	}
	for _, name := range []string{"base", "target", "tree", "verification"} {
		t.Run(name+"/self-consistent-receipt", func(t *testing.T) {
			value, _ := aggregateTestReceipt(t)
			mutations[name](&value.AggregateExpected)
			var err error
			value.EnvelopeSHA256, err = aggregateEnvelope(value)
			if err != nil {
				t.Fatal(err)
			}
			forged, err := encodeAggregateOutcome(value)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ParseAggregateOutcome(forged); err != nil {
				t.Fatalf("invalid control: %v", err)
			}
			if _, err = VerifyAggregateOutcome(context.Background(), t.TempDir(), aggregateTestExpected(), forged); err == nil || err.Error() != "aggregate-binding-drift" {
				t.Fatalf("receipt authority %s: %v", name, err)
			}
		})
	}
}

func TestAggregateOutcomeForeignSelfConsistentSet(t *testing.T) {
	root := newAdapterFixture(t)
	aggregateQualificationTraceGuard(t, root)
	expected := aggregateTestExpected()
	expected.Base = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	paths := aggregateTestPaths(201)
	for i, path := range paths {
		writeAdapterFixture(t, root, path, fmt.Sprintf("package source\nconst V%d = %d\n", i, i))
	}
	gitAdapterFixture(t, root, "add", "source")
	gitAdapterFixture(t, root, "commit", "-qm", "foreign-set control")
	expected.Target = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	expected.Tree = gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
	foreign := append([]string{}, paths...)
	foreign[0] = "foreign.go"
	sort.Strings(foreign)
	value, err := newAggregateOutcome(expected, foreign, foreign)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := encodeAggregateOutcome(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseAggregateOutcome(raw); err != nil {
		t.Fatalf("invalid control: %v", err)
	}
	if _, err = VerifyAggregateOutcome(context.Background(), root, expected, raw); err == nil || err.Error() != "aggregate-source-set-drift" {
		t.Fatalf("foreign set: %v", err)
	}
}

func TestAggregateOutcomeSlowOpeningAndClosingAcquisition(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("native Darwin qualification")
	}
	root := newAdapterFixture(t)
	aggregateQualificationTraceGuard(t, root)
	expected := aggregateTestExpected()
	expected.Base = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	for i, path := range aggregateTestPaths(201) {
		writeAdapterFixture(t, root, path, fmt.Sprintf("package source\nconst V%d=%d\n", i, i))
	}
	gitAdapterFixture(t, root, "add", "source")
	gitAdapterFixture(t, root, "commit", "-qm", "slow acquisition fixture")
	expected.Target = gitAdapterFixture(t, root, "rev-parse", "HEAD")
	expected.Tree = gitAdapterFixture(t, root, "rev-parse", "HEAD^{tree}")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, which := range []int{1, 2} {
		t.Run(fmt.Sprint(which), func(t *testing.T) {
			bin := t.TempDir()
			counter := filepath.Join(bin, "tree-count")
			marker := filepath.Join(bin, "slow-acquisition")
			// ls-tree is the actual immutable source acquisition, exactly once per
			// opening/closing Build. The wrapper retains real Git for all reads.
			script := "#!/bin/sh\ncase \" $* \" in *' ls-tree '*) n=0; [ ! -f \"$QUAL_COUNT\" ] || n=$(/bin/cat \"$QUAL_COUNT\"); n=$((n+1)); printf '%s' \"$n\" > \"$QUAL_COUNT\"; if [ \"$n\" = \"$QUAL_SLOW\" ]; then printf '%s' \"$n\" > \"$QUAL_MARKER\"; /bin/sleep 2; fi;; esac\nexec \"$QUAL_REAL_GIT\" \"$@\"\n"
			quote := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\\''") + "'" }
			script = strings.NewReplacer("\"$QUAL_COUNT\"", quote(counter), "\"$QUAL_MARKER\"", quote(marker), "\"$QUAL_SLOW\"", quote(fmt.Sprint(which)), "\"$QUAL_REAL_GIT\"", quote(realGit)).Replace(script)
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("QUAL_COUNT", counter)
			t.Setenv("QUAL_MARKER", marker)
			t.Setenv("QUAL_SLOW", fmt.Sprint(which))
			t.Setenv("QUAL_REAL_GIT", realGit)
			deadline := time.Now().Add(time.Second)
			budget, err := gitrun.NewOperationBudget(64, deadline, false)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := ProduceAggregateOutcome(gitrun.WithOperationBudget(context.Background(), budget), root, expected)
			if err == nil || len(raw) != 0 {
				t.Fatal("slow acquisition published receipt")
			}
			observed, readErr := os.ReadFile(marker)
			if readErr != nil || string(observed) != fmt.Sprint(which) {
				t.Fatalf("wrong acquisition reached: %s %v; operation=%v", observed, readErr, err)
			}
			if time.Now().Before(deadline) || budget.Used() == 0 {
				t.Fatal("original operation budget was not consumed")
			}
			t.Logf("acquisition=%d attemptedGit=%d refusal=%v", which, budget.Used(), err)
		})
	}
}
