package playwrightruns

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const baselineRev = "0123456789abcdef0123456789abcdef01234567"

// bt is a one-result test of a baseline fixture.
func bt(file, title, status string) fxTest {
	r := pass()
	switch status {
	case "unexpected":
		r = fail("Error: boom", fixtureRoot+"/"+file, 4)
	case "flaky":
		return fxTest{file: file, title: title, status: status, results: []fxResult{fail("Error: boom", fixtureRoot+"/"+file, 4), {status: "passed", retry: 1}}}
	}
	return fxTest{file: file, title: title, status: status, results: []fxResult{r}}
}

// named parses a report and writes it to dir/name so an EvidenceLoader can re-read it.
func named(t *testing.T, dir, name string, tests ...fxTest) NamedRun {
	t.Helper()
	raw := reportJSON(t, "1.63.0", nil, tests...)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return NamedRun{Path: p, Run: mustParse(t, raw)}
}

func fileLoader(p string) (*Run, error) { return ReadFile(p) }

func entryTitles(list []BaselineEntry) string {
	out := []string{}
	for _, e := range list {
		out = append(out, strings.Join(e.TitlePath, ">"))
	}
	return strings.Join(out, ",")
}

func diffReasons(list []DiffTest) string {
	out := []string{}
	for _, d := range list {
		out = append(out, strings.Join(d.TitlePath, ">")+"="+d.Reason)
	}
	return strings.Join(out, ",")
}

var notObserved = Staleness{Status: "NOT_OBSERVED", MaxAge: DefaultBaselineMaxAge, Reason: "BASE_NOT_GIVEN"}

// PWR-V0-041: known needs a failure in both the full run and the isolated re-run; a failure (or a
// flaky result) in the full run only is a suspected flake.
func TestPWRV0041_RecordKnownNeedsBothRuns(t *testing.T) {
	dir := t.TempDir()
	full := named(t, dir, "full.json", bt("a.spec.ts", "both", "unexpected"), bt("a.spec.ts", "fullonly", "unexpected"),
		bt("a.spec.ts", "retried", "flaky"), bt("a.spec.ts", "ok", "expected"))
	iso := named(t, dir, "iso.json", bt("a.spec.ts", "both", "unexpected"), bt("a.spec.ts", "fullonly", "expected"))
	b := RecordBaseline(baselineRev, []NamedRun{full}, []NamedRun{iso})
	if b.Profile != BaselineProfile || b.Revision != baselineRev {
		t.Fatalf("profile/revision: %+v", b)
	}
	if got := entryTitles(b.Known); got != "both" {
		t.Fatalf("known = %q", got)
	}
	if got := entryTitles(b.Flaky); got != "fullonly,retried" {
		t.Fatalf("flaky = %q", got)
	}
	k := b.Known[0]
	if k.Full.Path != full.Path || k.Full.Sha256 != full.Run.Sha256 || k.Isolated == nil || k.Isolated.Sha256 != iso.Run.Sha256 {
		t.Fatalf("evidence binding: %+v", k)
	}
	for _, f := range b.Flaky {
		if f.Isolated != nil {
			t.Fatalf("flake carries isolated evidence: %+v", f)
		}
	}
}

// PWR-V0-041: ReadBaseline round-trips a recorded document and refuses malformed ones.
func TestPWRV0041_ReadBaselineShape(t *testing.T) {
	dir := t.TempDir()
	full := named(t, dir, "full.json", bt("a.spec.ts", "both", "unexpected"))
	iso := named(t, dir, "iso.json", bt("a.spec.ts", "both", "unexpected"))
	good := `{"profile":"corvint-run-baseline/0","revision":"` + baselineRev + `","known":[{"project":"chromium","file":"a.spec.ts","titlePath":["both"],` +
		`"full":{"path":"` + full.Path + `","sha256":"` + full.Run.Sha256 + `"},"isolated":{"path":"` + iso.Path + `","sha256":"` + iso.Run.Sha256 + `"}}],"flaky":[]}`
	write := func(body string) string {
		p := filepath.Join(dir, "b.json")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	b, sum, err := ReadBaseline(write(good))
	if err != nil || len(b.Known) != 1 || len(sum) != 64 {
		t.Fatalf("good baseline: %v %+v %q", err, b, sum)
	}
	bad := map[string]string{
		"unknown member": strings.Replace(good, `"flaky":[]`, `"flaky":[],"extra":1`, 1),
		"profile":        strings.Replace(good, "corvint-run-baseline/0", "corvint-run-baseline/9", 1),
		"short revision": strings.Replace(good, baselineRev, "0123abc", 1),
		"known no iso":   strings.Replace(good, `,"isolated":{"path":"`+iso.Path+`","sha256":"`+iso.Run.Sha256+`"}`, ``, 1),
		"bad digest":     strings.Replace(good, full.Run.Sha256, "nothex", 1),
		"no flaky list":  strings.Replace(good, `,"flaky":[]`, ``, 1),
		"trailing doc":   good + `{}`,
	}
	for name, body := range bad {
		if _, _, err := ReadBaseline(write(body)); !errors.Is(err, ErrBaselineShape) {
			t.Errorf("%s: err = %v, want ErrBaselineShape", name, err)
		}
	}
}

// PWR-V0-042, PWR-V0-043, PWR-V0-044: a verified known failure is excused; a new failure and a
// suspected flake block; passing baseline entries are fixed; the receipt binds the revision.
func TestPWRV0042_DiffExcusesOnlyVerifiedKnown(t *testing.T) {
	dir := t.TempDir()
	full := named(t, dir, "full.json", bt("a.spec.ts", "known", "unexpected"), bt("a.spec.ts", "flake", "unexpected"), bt("a.spec.ts", "healed", "unexpected"))
	iso := named(t, dir, "iso.json", bt("a.spec.ts", "known", "unexpected"), bt("a.spec.ts", "flake", "expected"), bt("a.spec.ts", "healed", "unexpected"))
	b := RecordBaseline(baselineRev, []NamedRun{full}, []NamedRun{iso})

	only := named(t, dir, "cand1.json", bt("a.spec.ts", "known", "unexpected"), bt("a.spec.ts", "healed", "expected"))
	d := DiffBaseline(b, "sum", []NamedRun{only}, nil, fileLoader, notObserved)
	if d.Verdict != VerdictBaselinePass || d.Counts.Excused != 1 || d.Counts.Fixed != 1 || d.Counts.Unobserved != 1 || d.Counts.Blocking != 0 {
		t.Fatalf("verified known: %+v", d)
	}
	if d.BaselineRevision != baselineRev || d.BaselineSha256 != "sum" || len(d.Runs) != 1 || d.Runs[0].Sha256 != only.Run.Sha256 {
		t.Fatalf("receipt binding: %+v", d)
	}

	cand := named(t, dir, "cand2.json", bt("a.spec.ts", "known", "unexpected"), bt("a.spec.ts", "flake", "unexpected"), bt("b.spec.ts", "fresh", "unexpected"))
	d = DiffBaseline(b, "sum", []NamedRun{cand}, nil, fileLoader, notObserved)
	if d.Verdict != VerdictBaselineBlocked || d.Counts.New != 1 || d.Counts.Flaky != 1 || d.Counts.Known != 1 || d.Counts.Excused != 1 || d.Counts.Blocking != 2 {
		t.Fatalf("blocking counts: %+v", d.Counts)
	}
	if got := diffReasons(d.Flaky); got != "flake="+ReasonSuspectedFlake {
		t.Fatalf("flaky = %q", got)
	}
	if got := diffReasons(d.New); got != "fresh="+ReasonNotInBaseline {
		t.Fatalf("new = %q", got)
	}
	if len(d.Reasons) != 1 || d.Reasons[0] != "BLOCKING_FAILURES" {
		t.Fatalf("reasons = %v", d.Reasons)
	}
}

// PWR-V0-042: missing, changed or passing evidence un-excuses a known failure.
func TestPWRV0042_EvidenceReReadUnexcuses(t *testing.T) {
	dir := t.TempDir()
	full := named(t, dir, "full.json", bt("a.spec.ts", "known", "unexpected"))
	iso := named(t, dir, "iso.json", bt("a.spec.ts", "known", "unexpected"))
	b := RecordBaseline(baselineRev, []NamedRun{full}, []NamedRun{iso})
	cand := named(t, dir, "cand.json", bt("a.spec.ts", "known", "unexpected"))
	cases := map[string]struct {
		load EvidenceLoader
		want string
	}{
		"missing": {func(p string) (*Run, error) {
			if p == iso.Path {
				return nil, os.ErrNotExist
			}
			return ReadFile(p)
		}, ReasonEvidenceMissing},
		"changed": {func(p string) (*Run, error) {
			r, err := ReadFile(p)
			if err == nil && p == full.Path {
				r.Sha256 = strings.Repeat("0", 64)
			}
			return r, err
		}, ReasonEvidenceChanged},
		"passing": {func(p string) (*Run, error) {
			if p == iso.Path {
				return mustParse(t, reportJSON(t, "1.63.0", nil, bt("a.spec.ts", "known", "expected"))), nil
			}
			return ReadFile(p)
		}, ReasonEvidenceChanged},
	}
	for name, c := range cases {
		d := DiffBaseline(b, "sum", []NamedRun{cand}, nil, c.load, notObserved)
		if d.Verdict != VerdictBaselineBlocked || d.Counts.Excused != 0 || diffReasons(d.Known) != "known="+c.want {
			t.Errorf("%s: %s %+v", name, diffReasons(d.Known), d.Counts)
		}
	}
	// A rewritten report that keeps its digest is impossible; a passing re-read at the recorded
	// digest is modelled by a loader that returns the recorded digest over a passing run.
	passingAtDigest := func(p string) (*Run, error) {
		r, err := ReadFile(p)
		if err != nil || p != iso.Path {
			return r, err
		}
		pr := mustParse(t, reportJSON(t, "1.63.0", nil, bt("a.spec.ts", "known", "expected")))
		pr.Sha256 = r.Sha256
		return pr, nil
	}
	d := DiffBaseline(b, "sum", []NamedRun{cand}, nil, passingAtDigest, notObserved)
	if diffReasons(d.Known) != "known="+ReasonEvidencePassing || d.Verdict != VerdictBaselineBlocked {
		t.Fatalf("passing evidence: %s", diffReasons(d.Known))
	}
}

// PWR-V0-043: a change's own test file fails even when its failure is known; report errors block.
func TestPWRV0043_OwnFileAndReportErrorsBlock(t *testing.T) {
	dir := t.TempDir()
	full := named(t, dir, "full.json", bt("a.spec.ts", "known", "unexpected"))
	iso := named(t, dir, "iso.json", bt("a.spec.ts", "known", "unexpected"))
	b := RecordBaseline(baselineRev, []NamedRun{full}, []NamedRun{iso})
	cand := named(t, dir, "cand.json", bt("a.spec.ts", "known", "unexpected"))
	d := DiffBaseline(b, "sum", []NamedRun{cand}, func(f string) bool { return f == "a.spec.ts" }, fileLoader, notObserved)
	if d.Verdict != VerdictBaselineBlocked || diffReasons(d.Known) != "known="+ReasonOwnFile {
		t.Fatalf("own file: %s %v", diffReasons(d.Known), d.Verdict)
	}
	cand.Run.ReportErrors = 1
	d = DiffBaseline(b, "sum", []NamedRun{cand}, nil, fileLoader, notObserved)
	if d.Verdict != VerdictBaselineBlocked || len(d.Reasons) != 1 || d.Reasons[0] != ReasonReportHasErrors {
		t.Fatalf("report errors: %+v", d)
	}
}
