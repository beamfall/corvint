package main

import (
	"encoding/json"
	"github.com/Beamfall/corvint/internal/contextindex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixture struct {
	t      *testing.T
	root   string
	ledger map[string]any
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/ledger-v0.json")
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: t.TempDir()}
	if err = json.Unmarshal(raw, &f.ledger); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *fixture) write(p string, raw []byte) {
	f.t.Helper()
	p = filepath.Join(f.root, p)
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, raw, 0600); err != nil {
		f.t.Fatal(err)
	}
}
func (f *fixture) row(i int) map[string]any { return f.ledger["useCases"].([]any)[i].(map[string]any) }
func (f *fixture) save() {
	raw, err := json.Marshal(f.ledger)
	if err != nil {
		f.t.Fatal(err)
	}
	f.write("ledger.json", raw)
}
func (f *fixture) check(want ...string) map[string]any {
	f.t.Helper()
	f.save()
	r := validate(f.root, "ledger.json")
	if len(want) == 0 {
		if r["valid"] != true {
			f.t.Fatal(r)
		}
	} else {
		if r["valid"] != false {
			f.t.Fatal("unexpected promotion", r)
		}
		raw, _ := json.Marshal(r["errors"])
		for _, w := range want {
			if !strings.Contains(string(raw), w) {
				f.t.Fatalf("missing %s: %s", w, raw)
			}
		}
	}
	return r
}
func (f *fixture) receipt(class string) map[string]any {
	id := "UC-AI-CODING"
	subjects := []any{}
	subject := func(name string) string {
		p := "evidence/" + class + "/" + name
		raw := []byte(class + ":" + name + "\n")
		f.write(p, raw)
		d := digest(raw)
		subjects = append(subjects, map[string]any{"path": p, "sha256": d})
		return d
	}
	a := map[string]any{}
	switch class {
	case "contract":
		a["requirementIds"] = []string{"UCV0-001"}
	case "implementation":
		a["entrypoints"] = []string{"corvint use-case run"}
	case "hostile-tests":
		a["cases"] = []string{"abstention", "hostile", "negative"}
	case "corvint-dogfood", "beamfall-dogfood":
		a["outcome"] = "PASS"
		a["repository"] = strings.TrimSuffix(class, "-dogfood")
	case "sealed-benchmark":
		a["outcome"] = "PASS"
		for _, name := range []string{"preregistration", "corpus", "result"} {
			a[name+"Sha256"] = subject(name)
		}
	}
	if class != "sealed-benchmark" {
		subject("subject")
	}
	r := map[string]any{"attestation": a, "evidenceClass": class, "repositoryRevision": strings.Repeat("a", 40), "result": "PASS", "spec": evidenceProfile, "subjects": subjects, "useCaseId": id}
	f.bind(class, r)
	return r
}
func (f *fixture) bind(class string, r map[string]any) {
	raw, err := json.Marshal(r)
	if err != nil {
		f.t.Fatal(err)
	}
	p := "evidence/" + class + "/receipt.json"
	f.write(p, raw)
	f.row(0)["evidence"].(map[string]any)[class] = map[string]any{"path": p, "sha256": digest(raw)}
}
func TestUCV0LedgerAndPromotion(t *testing.T) {
	f := newFixture(t)
	rows := f.ledger["useCases"].([]any)
	if len(rows) != 19 {
		t.Fatal(len(rows))
	}
	for i, r := range rows {
		m := r.(map[string]any)
		if m["id"] != useCaseIDs[i] || m["status"] != "specified" || m["claim"] != "UNPROVEN" || len(m["evidence"].(map[string]any)) != 0 {
			t.Fatal(m)
		}
	}
	result := f.check()
	raw, err := contextindex.CanonicalJSON(result)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"claimCounts":{"UNPROVEN":19,"VERIFIED":0},"evidenceCount":0,"ledgerSha256":"` + str(result["ledgerSha256"]) + `","profile":"corvint-use-case-conformance-result/0","statusCounts":{"experimental":0,"specified":19,"verified":0},"useCaseCount":19,"valid":true}`
	if string(raw) != want {
		t.Fatalf("noncanonical %s", raw)
	}
	f.row(0)["status"] = "verified"
	f.row(0)["claim"] = "VERIFIED"
	missing := []string{}
	for _, class := range evidenceClasses {
		missing = append(missing, "verified-missing-evidence:"+class)
	}
	f.check(missing...)
	f.row(0)["status"] = "experimental"
	f.row(0)["claim"] = "UNPROVEN"
	f.check("experimental-missing-evidence:contract", "experimental-missing-evidence:implementation")
	f.receipt("contract")
	f.receipt("implementation")
	f.check()
	for _, class := range evidenceClasses {
		f.receipt(class)
	}
	f.row(0)["status"] = "verified"
	f.row(0)["claim"] = "VERIFIED"
	result = f.check()
	if result["evidenceCount"] != 6 || result["claimCounts"].(map[string]any)["VERIFIED"] != 1 {
		t.Fatal(result)
	}
}
func TestUCV0HostileEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(*fixture, map[string]any)
	}{
		{"receipt-tamper", "receipt-digest-mismatch", func(f *fixture, r map[string]any) { f.write("evidence/contract/receipt.json", []byte("changed")) }},
		{"subject-tamper", "subject-digest-mismatch", func(f *fixture, r map[string]any) { f.write("evidence/contract/subject", []byte("changed")) }},
		{"traversal", "invalid-path", func(f *fixture, r map[string]any) {
			f.row(0)["evidence"].(map[string]any)["contract"].(map[string]any)["path"] = "../receipt.json"
		}},
		{"symlink", "symlink", func(f *fixture, r map[string]any) {
			if err := os.Symlink(filepath.Join(f.root, "evidence/contract/receipt.json"), filepath.Join(f.root, "link")); err != nil {
				t.Fatal(err)
			}
			f.row(0)["evidence"].(map[string]any)["contract"].(map[string]any)["path"] = "link"
		}},
		{"reuse", "duplicate-receipt-reuse", func(f *fixture, r map[string]any) {
			f.row(1)["evidence"].(map[string]any)["contract"] = f.row(0)["evidence"].(map[string]any)["contract"]
		}},
		{"class", "evidence-class-mismatch", func(f *fixture, r map[string]any) { r["evidenceClass"] = "implementation"; f.bind("contract", r) }},
		{"extra", "extra-field:unexpected", func(f *fixture, r map[string]any) { f.ledger["unexpected"] = true }},
		{"oversize", "file-too-large", func(f *fixture, r map[string]any) { f.write("evidence/contract/subject", make([]byte, maxFileBytes+1)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			r := f.receipt("contract")
			f.check()
			tc.mutate(f, r)
			f.check(tc.want)
		})
	}
	for _, tc := range []struct {
		class, key string
		value      any
		want       string
	}{{"hostile-tests", "cases", []string{"hostile", "negative"}, "incomplete-hostile-cases"}, {"sealed-benchmark", "resultSha256", strings.Repeat("f", 64), "unbound-benchmark-seal"}, {"contract", "requirementIds", []string{"invalid"}, "invalid-requirement-id"}, {"corvint-dogfood", "repository", "beamfall", "repository-mismatch"}, {"beamfall-dogfood", "outcome", "UNPROVEN", "outcome-not-pass"}} {
		t.Run(tc.want, func(t *testing.T) {
			f := newFixture(t)
			r := f.receipt(tc.class)
			r["attestation"].(map[string]any)[tc.key] = tc.value
			f.bind(tc.class, r)
			f.check(tc.want)
		})
	}
	t.Run("colliding-seals", func(t *testing.T) {
		f := newFixture(t)
		r := f.receipt("sealed-benchmark")
		a := r["attestation"].(map[string]any)
		a["resultSha256"] = a["corpusSha256"]
		f.bind("sealed-benchmark", r)
		f.check("colliding-benchmark-seal")
	})
	t.Run("duplicate-json", func(t *testing.T) {
		f := newFixture(t)
		f.save()
		p := filepath.Join(f.root, "ledger.json")
		raw, _ := os.ReadFile(p)
		f.write("ledger.json", []byte(strings.Replace(string(raw), "{", `{"spec":"duplicate",`, 1)))
		r := validate(f.root, "ledger.json")
		raw, _ = json.Marshal(r)
		if !strings.Contains(string(raw), "ledger:duplicate-key:spec") {
			t.Fatal(string(raw))
		}
	})
}

// UCV0-013: a new closed job set cannot silently redefine historical /0 admission.
func TestUCV0ProfileMigration(t *testing.T) {
	t.Run("UCV0-013 profile migration", testUCV0ProfileMigration)
}

func testUCV0ProfileMigration(t *testing.T) {
	legacy := newFixture(t)
	legacy.check()
	raw, err := os.ReadFile("ledger.json")
	if err != nil {
		t.Fatal(err)
	}
	current := newFixture(t)
	if err := json.Unmarshal(raw, &current.ledger); err != nil {
		t.Fatal(err)
	}
	result := current.check()
	if result["useCaseCount"] != 22 || result["claimCounts"].(map[string]any)["UNPROVEN"] != 22 || result["evidenceCount"] != 0 {
		t.Fatal(result)
	}
	rows := current.ledger["useCases"].([]any)
	oldRows := legacy.ledger["useCases"].([]any)
	for i, old := range oldRows {
		a, _ := json.Marshal(old)
		b, _ := json.Marshal(rows[i])
		if string(a) != string(b) {
			t.Fatalf("historical row %d changed", i)
		}
	}
	current.ledger["spec"] = legacyProfile
	current.check("unknown-id", "unexpected-use-case-count")
	legacy.ledger["spec"] = profile
	legacy.check("missing-use-case:UC-TASK-ORIENTATION", "missing-use-case:UC-CHANGE-CONSEQUENCE", "missing-use-case:UC-EVIDENCE-CARRYING-COMPLETION")
	legacy.ledger["spec"] = "corvint-use-case-conformance/2"
	legacy.check("wrong-spec")
	current.ledger["spec"] = profile
	for i := len(useCaseIDs); i < len(rows); i++ {
		row := current.row(i)
		row["status"], row["claim"] = "verified", "VERIFIED"
		missing := []string{}
		for _, class := range evidenceClasses {
			missing = append(missing, "use-case:"+str(row["id"])+":verified-missing-evidence:"+class)
		}
		current.check(missing...)
		row["status"], row["claim"] = "specified", "UNPROVEN"
	}
}
