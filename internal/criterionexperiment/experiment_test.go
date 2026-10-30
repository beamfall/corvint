package criterionexperiment

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

func requestFixture() Request {
	return Request{Schema: Profile, Base: strings.Repeat("a", 40), Target: strings.Repeat("b", 40), CEM: "/tmp/cem.json", Ticket: "ticket:q:main:T1", Attempt: "attempt:q:main:a", Module: "sample", GoBinary: "/usr/bin/go", GoSha256: strings.Repeat("c", 64), TimeoutSeconds: 120, Criteria: []Criterion{{Index: 0, AcceptanceSha256: strings.Repeat("d", 64), Relation: "repair", Test: "TestRepair", Package: ".", Assertion: "EXPECTED", Hunks: []string{"hunk"}, Oracle: Selector{strings.Repeat("a", 40), "sample/oracle_test.go"}, Authority: Authority{"reviewer", strings.Repeat("a", 40), "contract.txt", strings.Repeat("e", 64), true}, Controls: []string{strings.Repeat("c", 40)}}}}
}

// CEX-V0-001: bounded, closed, duplicate/case rejecting admission.
func TestSchemaAdmissionClosedAndBounded(t *testing.T) {
	r := requestFixture()
	if e := r.validate(); e != nil {
		t.Fatal(e)
	}
	raw := Encode(r)
	var got Request
	if e := Decode(raw, &got); e != nil {
		t.Fatal(e)
	}
	for name, b := range map[string][]byte{
		"duplicate":  bytes.Replace(raw, []byte(`"schema":`), []byte(`"schema":"x","schema":`), 1),
		"case":       bytes.Replace(raw, []byte(`"schema":`), []byte(`"Schema":`), 1),
		"extra":      append([]byte(`{"extra":0,`), raw[1:]...),
		"null index": bytes.Replace(raw, []byte(`"index":0`), []byte(`"index":null`), 1),
		"trailing":   append(append([]byte{}, raw...), []byte(`{}`)...),
	} {
		t.Run(name, func(t *testing.T) {
			if Decode(b, &got) == nil {
				t.Fatal("admitted malformed request")
			}
		})
	}
	for name, change := range map[string]func(*Request){"missing": func(r *Request) { r.Criteria = nil }, "index": func(r *Request) { r.Criteria[0].Index = 1 }, "circular": func(r *Request) { r.Criteria[0].Authority.AnchorCommit = r.Target }, "unreviewed": func(r *Request) { r.Criteria[0].Authority.Independent = false }, "pattern": func(r *Request) { r.Criteria[0].Package = "..." }, "timeout": func(r *Request) { r.TimeoutSeconds = 121 }, "surplus": func(r *Request) { r.Criteria = append(r.Criteria, r.Criteria[0]) }} {
		t.Run(name, func(t *testing.T) {
			r := requestFixture()
			change(&r)
			if r.validate() == nil {
				t.Fatal("admitted")
			}
		})
	}
	receipt := Receipt{Schema: Profile, Scenarios: []Scenario{{Complete: false}}}
	receiptRaw := Encode(receipt)
	for _, replacement := range [][2]string{{`"complete":false`, `"complete":null`}, {`"role":""`, `"role":null`}, {`"exitCode":0`, `"exitCode":null`}} {
		var parsed Receipt
		if Decode(bytes.Replace(receiptRaw, []byte(replacement[0]), []byte(replacement[1]), 1), &parsed) == nil {
			t.Fatal("null primitive receipt field admitted")
		}
	}
	spaced := append([]byte("\n"), raw...)
	if e := Decode(spaced, &got); e != nil || CanonicalDigest(got) != CanonicalDigest(r) {
		t.Fatal("canonical digest changed with whitespace")
	}
}
func events(status, marker string) []byte {
	outputType := "frame"
	if marker != "" {
		outputType = "error"
	}
	rows := []map[string]any{{"Action": "start", "Package": "example.test/sample"}, {"Action": "run", "Package": "example.test/sample", "Test": "TestRepair"}, {"Action": "output", "Package": "example.test/sample", "Test": "TestRepair", "Output": marker, "OutputType": outputType}, {"Action": status, "Package": "example.test/sample", "Test": "TestRepair"}, {"Action": status, "Package": "example.test/sample"}}
	var out []byte
	for _, r := range rows {
		out = append(out, Encode(r)...)
		out = append(out, '\n')
	}
	return out
}

// CEX-V0-005: only complete named error diagnostics can kill controls.
func TestClassifierNamedAssertionAndCompleteEvents(t *testing.T) {
	fail := events("fail", "    file_test.go:4: EXPECTED\n")
	classify := func(b []byte, exit int, complete bool) string {
		return Classify(b, exit, complete, "TestRepair", "EXPECTED", "example.test/sample")
	}
	if classify(events("pass", ""), 0, true) != "pass" || classify(fail, 1, true) != "expected-failure" {
		t.Fatal("honest complete outcomes refused")
	}
	for name, b := range map[string][]byte{
		"wrong assertion":        events("fail", "WRONG"),
		"marker substring":       events("fail", "    file_test.go:4: UNEXPECTED\n"),
		"marker filename":        events("fail", "    EXPECTED_test.go:4: WRONG\n"),
		"nested source location": events("fail", "    wrong_test.go:4: WRONG nested.go:9: EXPECTED\n"),
		"contradictory pass":     events("pass", "    file_test.go:4: EXPECTED\n"),
		"unparsed diagnostic":    events("fail", "EXPECTED\n"), "logged marker": bytes.ReplaceAll(fail, []byte(`"OutputType":"error"`), []byte(`"OutputType":"log"`)), "skip": events("skip", "EXPECTED"), "panic": events("fail", "panic: EXPECTED"), "duplicate": append(append([]byte{}, fail...), fail...), "malformed": []byte("{\n"), "truncated": fail[:len(fail)-1], "other test": bytes.ReplaceAll(fail, []byte("TestRepair"), []byte("TestOther")), "other package": bytes.ReplaceAll(fail, []byte("example.test/sample"), []byte("wrong")), "duplicate key": bytes.Replace(fail, []byte(`"Action":"run"`), []byte(`"Action":"pass","Action":"run"`), 1), "case key": bytes.Replace(fail, []byte(`"Action"`), []byte(`"action"`), 1), "extra object": bytes.Replace(fail, []byte("}\n"), []byte("}{}\n"), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if got := classify(b, 1, true); got == "expected-failure" || got == "pass" {
				t.Fatalf("admitted %s", got)
			}
		})
	}
	if classify(fail, 1, false) == "expected-failure" || classify(fail, 2, true) == "expected-failure" {
		t.Fatal("incomplete/crashed control counted kill")
	}
}

// CEX-V0-006: repair/preservation/new behavior retain requested denominators.
func TestRelationsPreserveScenarioDenominators(t *testing.T) {
	r := requestFixture()
	r.Criteria = append(r.Criteria, r.Criteria[0], r.Criteria[0])
	r.Criteria[1].Relation = "preservation"
	r.Criteria[2].Relation = "new-behavior"
	rows := scenarios(Plan{Request: r})
	if len(rows) != 8 {
		t.Fatalf("rows %d", len(rows))
	}
	for _, row := range rows {
		if row.Criterion == 2 && row.Role == "base" {
			t.Fatal("new behavior incorrectly requires base")
		}
	}
}

// CEX-V0-010: no dependency, workspace, cgo or wider runner profile.
func TestSourceProfileRejectsBroadenedExecution(t *testing.T) {
	base := map[string][]byte{"go.mod": []byte("module example.test/sample\ngo 1.27.1\n"), "main.go": []byte("package sample\nimport \"strings\"\nvar _ = strings.TrimSpace")}
	if e := profile(base); e != nil {
		t.Fatal(e)
	}
	for name, entry := range map[string]sourceFile{"dependency": {"go.mod", "", "module example.test/sample\ngo 1.27.1\nrequire x v1.0.0"}, "nested": {"sub/go.mod", "", "module child"}, "workspace": {"go.work", "", "go 1.27.1"}, "cgo": {"x.go", "", "package sample\nimport \"C\""}, "external": {"x.go", "", "package sample\nimport \"example.com/foo\""}} {
		t.Run(name, func(t *testing.T) {
			m := map[string][]byte{}
			for p, b := range base {
				m[p] = b
			}
			m[entry.Path] = []byte(entry.Sha256)
			if profile(m) == nil {
				t.Fatal("broadened profile admitted")
			}
		})
	}
}
func gitTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-C", dir}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.invalid")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s %v", args, b, e)
	}
	return strings.TrimSpace(string(b))
}

// CEX-V0-003: immutable regular source inventory and identical oracle overlay.
func TestImmutableInventoryOverlayAndRefusals(t *testing.T) {
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q")
	if e := os.Mkdir(filepath.Join(dir, "sample"), 0700); e != nil {
		t.Fatal(e)
	}
	files := map[string]string{"sample/go.mod": "module example.test/sample\ngo 1.27.1\n", "sample/main.go": "package sample\nconst Value=0\n", "sample/oracle_test.go": "package sample\nimport \"testing\"\nfunc TestRepair(t *testing.T){if Value!=1{t.Fatal(\"EXPECTED\")}}\n", "contract.txt": "accepted criterion"}
	for p, b := range files {
		if e := os.WriteFile(filepath.Join(dir, p), []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(dir, "sample/helper"), []byte("helper"), 0755); e != nil {
		t.Fatal(e)
	}
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "base")
	base := gitTest(t, dir, "rev-parse", "HEAD")
	r := requestFixture()
	r.Criteria[0].Oracle.Commit = base
	r.Criteria[0].Authority.AnchorCommit = base
	r.Criteria[0].Authority.AnchorSha256 = Digest([]byte(files["contract.txt"]))
	repo, e := gitauth.Open(dir, gitrun.NewDefaultBudget())
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	source, modes, digest, e := inventory(ctx, repo, r, r.Criteria[0], base)
	if e != nil || len(source) != 4 || digest == "" || modes["main.go"] != "100644" || modes["helper"] != "100755" {
		t.Fatalf("inventory %v %d", e, len(source))
	}
	_ = os.WriteFile(filepath.Join(dir, "sample/oracle_test.go"), []byte("package sample // candidate corrupt oracle"), 0600)
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "candidate")
	target := gitTest(t, dir, "rev-parse", "HEAD")
	overlaid, _, digest2, e := inventory(ctx, repo, r, r.Criteria[0], target)
	if e != nil || digest2 != digest || !bytes.Equal(overlaid["oracle_test.go"], source["oracle_test.go"]) {
		t.Fatalf("immutable overlay changed %v", e)
	}
	_ = os.Symlink("/etc/passwd", filepath.Join(dir, "sample/link"))
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "special")
	if _, _, _, e = inventory(ctx, repo, r, r.Criteria[0], gitTest(t, dir, "rev-parse", "HEAD")); e == nil {
		t.Fatal("symlink admitted")
	}
	c := r.Criteria[0]
	c.Test = "TestMissing"
	if _, _, _, e = inventory(ctx, repo, r, c, base); e == nil {
		t.Fatal("unrelated oracle admitted")
	}
}
