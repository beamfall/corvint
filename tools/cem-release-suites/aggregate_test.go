package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

// aggregateFixture commits the sealed declaration to a fresh repository and
// returns it with two passing run records of that commit, darwin/arm64 and
// linux/amd64, each carrying the sealed neutral-criterion ACCEPT.
func aggregateFixture(t *testing.T) (string, string, []runRecord) {
	t.Helper()
	accept, err := os.ReadFile("../../protocol/cem-1.0/stable/neutral-criterion/results/accept.json")
	if err != nil {
		t.Fatal(err)
	}
	declaration, err := os.ReadFile("READINESS.json")
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	gitT(t, repo, "init", "-q")
	path := "tools/cem-release-suites/READINESS.json"
	if err := os.MkdirAll(filepath.Join(repo, filepath.Dir(path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, path), declaration, 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", ".")
	gitT(t, repo, "commit", "-q", "-m", "declare")
	head, err := gitOutput(repo, "", nil, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	var records []runRecord
	for i, platform := range []string{"darwin/arm64", "linux/amd64"} {
		runner, platforms, _ := observedRows(platform, map[string]string{"corvint": "PASSED", "cem01-go": "PASSED"})
		records = append(records, runRecord{Profile: runRecordProfile, Revision: head, Platform: platform,
			Declaration: recordFile{Path: path, Sha256: digest(declaration)},
			Binaries:    []recordBinary{{Name: "corvint", Main: corvintMain, Sha256: strings.Repeat(fmt.Sprint(2*i+1), 64)}, {Name: "cem01-go", Main: cem01Main, Sha256: strings.Repeat(fmt.Sprint(2*i+2), 64)}},
			Runner:      runner, Platforms: platforms, Accepted: []string{string(accept)}})
	}
	return repo, path, records
}

func cloneRecord(t *testing.T, rec runRecord) runRecord {
	t.Helper()
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var out runRecord
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCEMV1611RunRecordIsCanonical pins the cem-release-run/1 encoding: a
// written record reads back unchanged, and an unknown member, a trailing value
// or a non-canonical spelling is refused.
func TestCEMV1611RunRecordIsCanonical(t *testing.T) {
	_, _, records := aggregateFixture(t)
	raw, err := encodeRunRecord(records[0])
	if err != nil {
		t.Fatal(err)
	}
	got, err := readRunRecord(raw)
	if err != nil || got.Platform != "darwin/arm64" || len(got.Accepted) != 1 {
		t.Fatalf("readRunRecord = %+v, %v", got, err)
	}
	trimmed := bytes.TrimSuffix(raw, []byte("\n"))
	for name, bad := range map[string][]byte{
		"unknown member": append(append([]byte(`{"extra":1,`), trimmed[1:]...), '\n'),
		"trailing value": append(append([]byte{}, raw...), []byte("{}\n")...),
		"no newline":     trimmed,
		"spaced":         append(bytes.ReplaceAll(trimmed, []byte(`,"`), []byte(`, "`)), '\n'),
		"empty":          nil,
	} {
		if _, err := readRunRecord(bad); err == nil {
			t.Fatalf("%s record was read", name)
		}
	}
}

// TestCEMV1612AggregationRefusesMismatchedRuns pins the record-set refusals:
// another revision, a judged declaration other than the committed one, a
// binary set other than corvint and cem01-go, a reused binary digest, a
// repeated platform, a record observing any platform but its own PASSED,
// different runner rows, or no accepted result. A runner row passes the merge
// only when every record observed it PASSED.
func TestCEMV1612AggregationRefusesMismatchedRuns(t *testing.T) {
	_, path, records := aggregateFixture(t)
	sha := records[0].Declaration.Sha256
	cases := map[string]func(*runRecord){
		"profile":            func(r *runRecord) { r.Profile = "cem-release-run/0" },
		"revision":           func(r *runRecord) { r.Revision = strings.Repeat("e", 40) },
		"declaration digest": func(r *runRecord) { r.Declaration.Sha256 = strings.Repeat("0", 64) },
		"declaration path":   func(r *runRecord) { r.Declaration.Path = "READINESS.json" },
		"binary count":       func(r *runRecord) { r.Binaries = r.Binaries[:1] },
		"binary order":       func(r *runRecord) { r.Binaries[0], r.Binaries[1] = r.Binaries[1], r.Binaries[0] },
		"binary main":        func(r *runRecord) { r.Binaries[1].Main = corvintMain },
		"binary digest":      func(r *runRecord) { r.Binaries[1].Sha256 = "x" },
		"reused binary":      func(r *runRecord) { r.Binaries[0].Sha256 = strings.Repeat("1", 64) },
		"repeated platform": func(r *runRecord) {
			r.Platform, r.Platforms = "darwin/arm64", map[string]string{"darwin/arm64": "PASSED"}
		},
		"foreign platform":    func(r *runRecord) { r.Platforms["darwin/arm64"] = "PASSED" },
		"platform not passed": func(r *runRecord) { r.Platforms["linux/amd64"] = "NOT_RUN" },
		"no platform":         func(r *runRecord) { r.Platform, r.Platforms = "", map[string]string{} },
		"runner rows":         func(r *runRecord) { r.Runner["corvint:extra"] = "PASSED" },
		"no accepted":         func(r *runRecord) { r.Accepted = []string{} },
	}
	for name, mutate := range cases {
		second := cloneRecord(t, records[1])
		mutate(&second)
		if _, _, _, err := checkRecords([]runRecord{records[0], second}, path, sha); err == nil {
			t.Fatalf("%s mismatch was aggregated", name)
		}
	}
	if _, _, _, err := checkRecords(nil, path, sha); err == nil {
		t.Fatal("an empty record set was aggregated")
	}
	second := cloneRecord(t, records[1])
	second.Runner["cem01-go:repository-envelope"] = "NOT_RUN"
	runner, platforms, accepted, err := checkRecords([]runRecord{records[0], second}, path, sha)
	if err != nil {
		t.Fatal(err)
	}
	if runner["cem01-go:repository-envelope"] != "NOT_RUN" || runner["corvint:repository-envelope"] != "PASSED" || len(platforms) != 2 || len(accepted) != 2 {
		t.Fatalf("merged runner %v platforms %v accepted %d; a row not PASSED on every run must not pass", runner, platforms, len(accepted))
	}
}

// TestCEMV1612BothPlatformRowsFromTwoRuns pins the aggregation end to end:
// two records of one revision observe both declared platform rows PASSED, the
// verdict is refused only at the external row, and the outcome names both
// records by digest. A record of another revision is refused with exit 1 and
// writes no outcome, and an existing outcome path is never overwritten.
func TestCEMV1612BothPlatformRowsFromTwoRuns(t *testing.T) {
	repo, path, records := aggregateFixture(t)
	dir := t.TempDir()
	var paths, sums []string
	for i, rec := range records {
		raw, err := encodeRunRecord(rec)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, fmt.Sprintf("run-%d.json", i))
		if err := os.WriteFile(p, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		paths, sums = append(paths, p), append(sums, digest(raw))
	}
	outcome := filepath.Join(dir, "outcome.json")
	aggregate := func(out string) []string {
		return append([]string{"--aggregate", "--root", repo, "--readiness", path, "--outcome", out}, paths...)
	}
	var stdout, stderr bytes.Buffer
	if code := run(aggregate(outcome), &stdout, &stderr); code != 3 {
		t.Fatalf("aggregate exit %d, want 3: %s%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"readiness: platform darwin/arm64 PASSED", "readiness: platform linux/amd64 PASSED", "readiness: runner cem01-go:repository-envelope PASSED", "readiness: external independent-external-consumer NOT_OBSERVED", "REFUSED (2 accepted stable results read): release-external-incomplete"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("aggregate output lacks %q:\n%s", want, stdout.String())
		}
	}
	raw, err := os.ReadFile(outcome)
	if err != nil {
		t.Fatal(err)
	}
	o, err := companionrelease.ReadStableGateOutcome(raw)
	if err != nil || o.Verdict != companionrelease.StableGateRefused || o.Revision != records[0].Revision || fmt.Sprint(o.Records) != fmt.Sprint(sums) || !strings.Contains(o.Reason, "release-external-incomplete") {
		t.Fatalf("outcome %+v, %v", o, err)
	}
	if code := run(aggregate(outcome), io.Discard, io.Discard); code != 2 {
		t.Fatalf("an existing outcome path exit %d, want 2", code)
	}
	other := cloneRecord(t, records[1])
	other.Revision = strings.Repeat("e", 40)
	if raw, err = encodeRunRecord(other); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[1], raw, 0o644); err != nil {
		t.Fatal(err)
	}
	refused := filepath.Join(dir, "refused.json")
	stderr.Reset()
	if code := run(aggregate(refused), io.Discard, &stderr); code != 1 || !strings.Contains(stderr.String(), "is not record 1's revision") {
		t.Fatalf("mismatched revision exit %d: %s", code, stderr.String())
	}
	if _, err := os.Lstat(refused); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused record set wrote an outcome")
	}
	for _, args := range [][]string{{"--aggregate", "--readiness", path}, {"--aggregate", paths[0]}, {"--aggregate", "--readiness", path, "--corvint", "c", paths[0]}, {"--corvint", "c", "--cem01", "p", "--record", "r"}, {"--corvint", "c", "--cem01", "p", paths[0]}} {
		if code := run(args, io.Discard, io.Discard); code != 2 {
			t.Fatalf("usage %v exit %d, want 2", args, code)
		}
	}
}

// TestAFPV0432CIScopeExcludesOnlyAttestedExternalRows pins the PR and
// main-push scope of the cem-release-readiness job (V1-2736): with
// --ci-unobservable naming exactly the declaration's external rows, the
// aggregate exits 0 when every runner and platform row is PASSED, prints the
// external row NOT_OBSERVED as a known blocker and still writes the strict
// REFUSED outcome. A runner row that is not PASSED, or a missing platform
// row, still exits 3; an attestation other than the declared external rows is
// refused with exit 1 and no outcome; the flag outside --aggregate is a usage
// error; and without the flag the aggregate stays strict.
func TestAFPV0432CIScopeExcludesOnlyAttestedExternalRows(t *testing.T) {
	repo, path, records := aggregateFixture(t)
	dir := t.TempDir()
	n := 0
	write := func(recs ...runRecord) []string {
		var paths []string
		for _, rec := range recs {
			raw, err := encodeRunRecord(rec)
			if err != nil {
				t.Fatal(err)
			}
			n++
			p := filepath.Join(dir, fmt.Sprintf("run-%d.json", n))
			if err := os.WriteFile(p, raw, 0o644); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, p)
		}
		return paths
	}
	const external = "independent-external-consumer"
	aggregate := func(flag string, paths []string) (int, string, string, string) {
		n++
		out := filepath.Join(dir, fmt.Sprintf("outcome-%d.json", n))
		args := []string{"--aggregate", "--root", repo, "--readiness", path, "--outcome", out}
		if flag != "" {
			args = append(args, "--ci-unobservable", flag)
		}
		var stdout, stderr bytes.Buffer
		code := run(append(args, paths...), &stdout, &stderr)
		return code, stdout.String(), stderr.String(), out
	}
	strictOutcome := func(name, out, class string) {
		t.Helper()
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		o, err := companionrelease.ReadStableGateOutcome(raw)
		if err != nil || o.Verdict != companionrelease.StableGateRefused || !strings.HasPrefix(o.Reason, class+":") {
			t.Fatalf("%s: outcome %+v, %v; the outcome must stay the strict %s refusal", name, o, err, class)
		}
	}
	passing := write(records...)

	code, stdout, stderr, out := aggregate(external, passing)
	if code != 0 {
		t.Fatalf("CI scope over passing records exit %d, want 0: %s%s", code, stdout, stderr)
	}
	for _, want := range []string{
		"readiness: external " + external + " NOT_OBSERVED\n",
		"REFUSED (2 accepted stable results read): release-external-incomplete",
		"readiness: known blocker: external " + external + " NOT_OBSERVED (caller-attested not CI-observable)",
		"readiness: CI-OBSERVABLE PASSED",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("CI scope output lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, external+" PASSED") || strings.Contains(stdout, "readiness: READY") {
		t.Fatalf("CI scope reported the unobserved row or the release as passed:\n%s", stdout)
	}
	strictOutcome("CI scope", out, companionrelease.StableRefusalExternal)

	if code, stdout, _, out = aggregate("", passing); code != 3 || strings.Contains(stdout, "known blocker") {
		t.Fatalf("strict aggregate exit %d, want 3:\n%s", code, stdout)
	}
	strictOutcome("strict", out, companionrelease.StableRefusalExternal)

	failing := cloneRecord(t, records[1])
	failing.Runner["cem01-go:repository-envelope"] = "NOT_RUN"
	code, stdout, _, out = aggregate(external, write(records[0], failing))
	if code != 3 || strings.Contains(stdout, "CI-OBSERVABLE PASSED") || !strings.Contains(stdout, "readiness: runner cem01-go:repository-envelope NOT_RUN") {
		t.Fatalf("CI scope with a runner row NOT_RUN exit %d, want 3:\n%s", code, stdout)
	}
	strictOutcome("runner NOT_RUN", out, companionrelease.StableRefusalRunner)

	if code, stdout, _, out = aggregate(external, write(records[0])); code != 3 || strings.Contains(stdout, "CI-OBSERVABLE PASSED") {
		t.Fatalf("CI scope with one platform unobserved exit %d, want 3:\n%s", code, stdout)
	}
	strictOutcome("platform missing", out, companionrelease.StableRefusalPlatform)

	for _, bad := range []string{"darwin/arm64", "corvint:tamper-matrix", external + "," + external, external + ",other", ","} {
		code, _, stderr, out := aggregate(bad, passing)
		if code != 1 || !strings.Contains(stderr, "--ci-unobservable") {
			t.Fatalf("--ci-unobservable %q exit %d, want 1: %s", bad, code, stderr)
		}
		if _, err := os.Lstat(out); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("--ci-unobservable %q wrote an outcome", bad)
		}
	}
	if code := run([]string{"--corvint", "c", "--cem01", "p", "--ci-unobservable", external}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("--ci-unobservable without --aggregate exit %d, want 2", code)
	}
}
