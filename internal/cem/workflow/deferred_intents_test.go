package workflow

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// fakeIntentCounter counts lines that begin with "REQ-"; bytes without one are
// not an intent.
func fakeIntentCounter(data []byte) (int, error) {
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "REQ-") {
			count++
		}
	}
	if count == 0 {
		return 0, errors.New("no requirements")
	}
	return count, nil
}

const deferredIntentFixture = "# Intent\n\nREQ-1 first\nREQ-2 second\n"

// TestCEMV1643DeferredIntentsRecordEncoding pins the canonical record: compact
// alphabetical JSON with one trailing newline, the entry rules, and refusal of
// every non-canonical or out-of-bound form.
func TestCEMV1643DeferredIntentsRecordEncoding(t *testing.T) {
	t.Parallel()
	oid := strings.Repeat("a", 40)
	entries := []DeferredIntent{
		{BaseBlobOid: &oid, ObligationCount: 3, Path: "docs/a.md", Reason: "deferred to V1-1 <later>"},
		{ObligationCount: 1, Path: "docs/b.md", Reason: "new scope"},
	}
	encoded, err := EncodeDeferredIntents(entries)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"deferredIntents":[{"baseBlobOid":"` + oid + `","obligationCount":3,"path":"docs/a.md","reason":"deferred to V1-1 <later>"},{"baseBlobOid":null,"obligationCount":1,"path":"docs/b.md","reason":"new scope"}],"profile":"cem-deferred-intents/0"}` + "\n"
	if string(encoded) != want {
		t.Fatalf("encoding = %s", encoded)
	}
	parsed, err := ParseDeferredIntents(encoded)
	if err != nil || len(parsed) != 2 || *parsed[0].BaseBlobOid != oid || parsed[1].BaseBlobOid != nil {
		t.Fatalf("round trip = %+v, %v", parsed, err)
	}
	if path := DeferredRecordPath(encoded); !strings.HasPrefix(path, DeferredIntentsDir+"/") || len(path) != len(DeferredIntentsDir)+1+64+5 {
		t.Fatalf("record path = %q", path)
	}
	for name, data := range map[string]string{
		"pretty":        strings.Replace(want, `{"deferredIntents"`, `{ "deferredIntents"`, 1),
		"no newline":    strings.TrimSuffix(want, "\n"),
		"unknown field": strings.Replace(want, `"profile"`, `"extra":1,"profile"`, 1),
		"missing base":  strings.Replace(want, `"baseBlobOid":null,`, "", 1),
		"profile":       strings.Replace(want, "cem-deferred-intents/0", "cem-deferred-intents/1", 1),
		"empty":         `{"deferredIntents":[],"profile":"cem-deferred-intents/0"}` + "\n",
	} {
		if _, err := ParseDeferredIntents([]byte(data)); err == nil {
			t.Errorf("%s: parsed a non-canonical record", name)
		}
	}
	bad := map[string][]DeferredIntent{
		"unsorted":    {entries[1], entries[0]},
		"duplicate":   {entries[1], entries[1]},
		"five":        {{ObligationCount: 1, Path: "a", Reason: "r"}, {ObligationCount: 1, Path: "b", Reason: "r"}, {ObligationCount: 1, Path: "c", Reason: "r"}, {ObligationCount: 1, Path: "d", Reason: "r"}, {ObligationCount: 1, Path: "e", Reason: "r"}},
		"newline":     {{ObligationCount: 1, Path: "a", Reason: "two\nlines"}},
		"long reason": {{ObligationCount: 1, Path: "a", Reason: strings.Repeat("x", MaxDeferredIntentReasonBytes+1)}},
		"no reason":   {{ObligationCount: 1, Path: "a"}},
		"zero count":  {{ObligationCount: 0, Path: "a", Reason: "r"}},
		"record dir":  {{ObligationCount: 1, Path: DeferredIntentsDir + "/x.json", Reason: "r"}},
		"cem path":    {{ObligationCount: 1, Path: wire.ExcludedCEMPath, Reason: "r"}},
		"bad path":    {{ObligationCount: 1, Path: "../a", Reason: "r"}},
		"short oid":   {{BaseBlobOid: new(string), ObligationCount: 1, Path: "a", Reason: "r"}},
	}
	for name, entries := range bad {
		if _, err := EncodeDeferredIntents(entries); err == nil {
			t.Errorf("%s: encoded an invalid record", name)
		}
	}
}

// deferredRead runs one stable read with the fake counter installed.
func deferredRead(t *testing.T, root, action, base, head string, limit *int, counter IntentObligationCounter) map[string]any {
	t.Helper()
	session := openSession(t, root)
	session.SetIntentObligationCounter(counter)
	result, err := session.Read(ctx(), action, ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: head,
		ArtifactRoot: root, ArtifactsGiven: true, MaxDeferredIntents: limit,
		Format: map[bool]string{true: "json"}[action == "report"],
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// commitDeferredChange commits the worktree on top of the fixture target,
// prepares and cites the base-to-head map and commits it.
func commitDeferredChange(t *testing.T, root, base string) string {
	t.Helper()
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(wire.ExcludedCEMPath))); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-qm", "change with deferral")
	change := strings.TrimSpace(gitCmd(t, root, "rev-parse", "HEAD"))
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: change}); err != nil {
		t.Fatal(err)
	}
	citeAll(t, root)
	gitCmd(t, root, "add", wire.ExcludedCEMPath)
	gitCmd(t, root, "commit", "-qm", "candidate")
	return strings.TrimSpace(gitCmd(t, root, "rev-parse", "HEAD"))
}

func deferredMember(t *testing.T, result map[string]any, key string) map[string]any {
	t.Helper()
	member, ok := result[key].(map[string]any)
	if !ok {
		encoded, _ := json.Marshal(result)
		t.Fatalf("no %s member: %s", key, encoded)
	}
	return member
}

func deferredIssueCodes(member map[string]any) []string {
	var codes []string
	for _, item := range member["issues"].([]any) {
		codes = append(codes, item.(map[string]any)["code"].(string))
	}
	return codes
}

// TestCEMV1644To646DeferRecordsAndStatusChecks drives cem defer, then the
// sealed change through status, verify and report: the record is a created,
// content-named hunk the seal binds, status refuses it under the default
// bound 0 and admits it under --max-deferred-intents 1, verify refuses the
// bound, and the report carries the section.
func TestCEMV1644To646DeferRecordsAndStatusChecks(t *testing.T) {
	t.Parallel()
	root, base, target := makeRepo(t)
	session := openSession(t, root)
	if _, err := session.Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "docs/intent.md", deferredIntentFixture)
	if _, err := session.Defer(ctx(), DeferOptions{MapPath: wire.ExcludedCEMPath, Intent: "docs/intent.md", Reason: "first"}); err == nil {
		t.Fatal("defer without a counter was not refused")
	}
	session.SetIntentObligationCounter(fakeIntentCounter)
	if _, err := session.Defer(ctx(), DeferOptions{MapPath: wire.ExcludedCEMPath, Intent: "docs/rule.txt", Reason: "x"}); err == nil {
		t.Fatal("defer of a non-intent was not refused")
	}
	first, err := session.Defer(ctx(), DeferOptions{MapPath: wire.ExcludedCEMPath, Intent: "docs/intent.md", Reason: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := session.Defer(ctx(), DeferOptions{MapPath: wire.ExcludedCEMPath, Intent: "docs/intent.md", Reason: "scope deferred to V1-9"})
	if err != nil {
		t.Fatal(err)
	}
	record := second["record"].(string)
	if record == first["record"] || second["count"] != 1 || second["mutates"] != true {
		t.Fatalf("second defer = %v", second)
	}
	listing, _ := os.ReadDir(filepath.Join(root, filepath.FromSlash(DeferredIntentsDir)))
	if len(listing) != 1 || DeferredIntentsDir+"/"+listing[0].Name() != record {
		t.Fatalf("record directory holds %v, want only %s", listing, record)
	}
	data, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(record)))
	entries, err := ParseDeferredIntents(data)
	if err != nil || DeferredRecordPath(data) != record || len(entries) != 1 || entries[0].BaseBlobOid != nil || entries[0].ObligationCount != 2 {
		t.Fatalf("record %s = %+v, %v", data, entries, err)
	}
	head := commitDeferredChange(t, root, base)

	refused := deferredRead(t, root, "status", base, head, nil, fakeIntentCounter)
	member := deferredMember(t, refused, "deferredIntents")
	if refused["ok"] != false || stableOutcome(refused) != "ACCEPT" || member["state"] != "refused" ||
		strings.Join(deferredIssueCodes(member), ",") != DeferredIssueExceedBound || member["record"] != record {
		encoded, _ := json.Marshal(refused)
		t.Fatalf("default-bound status = %s", encoded)
	}
	admitted := deferredRead(t, root, "status", base, head, intLimit(1), fakeIntentCounter)
	member = deferredMember(t, admitted, "deferredIntents")
	row := member["entries"].([]any)[0].(map[string]any)
	if admitted["ok"] != true || member["state"] != "within-bound" || member["count"] != 1 ||
		row["countState"] != "VERIFIED" || row["baseState"] != "MATCHED" || row["reason"] != "scope deferred to V1-9" {
		encoded, _ := json.Marshal(admitted)
		t.Fatalf("bounded status = %s", encoded)
	}
	unchecked := deferredRead(t, root, "status", base, head, intLimit(1), nil)
	if row := deferredMember(t, unchecked, "deferredIntents")["entries"].([]any)[0].(map[string]any); unchecked["ok"] != true || row["countState"] != "NOT_CHECKED" {
		t.Fatalf("counterless status row = %v", row)
	}
	miscounted := deferredRead(t, root, "status", base, head, intLimit(1), func([]byte) (int, error) { return 7, nil })
	if codes := deferredIssueCodes(deferredMember(t, miscounted, "deferredIntents")); miscounted["ok"] != false || strings.Join(codes, ",") != DeferredIssueCountMismatch {
		t.Fatalf("miscounted status issues = %v", codes)
	}
	verified := deferredRead(t, root, "verify", base, head, nil, fakeIntentCounter)
	if _, present := verified["deferredIntents"]; present || verified["ok"] != true {
		t.Fatalf("verify carried the deferral check: %v", verified)
	}
	if _, err := openSession(t, root).Read(ctx(), "verify", ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: head,
		ArtifactRoot: root, ArtifactsGiven: true, MaxDeferredIntents: intLimit(1),
	}); err == nil {
		t.Fatal("verify accepted --max-deferred-intents")
	}
	report := deferredRead(t, root, "report", base, head, intLimit(1), fakeIntentCounter)
	if review := report["review"].(map[string]any); report["ok"] != true || deferredMember(t, review, "deferredIntents")["state"] != "within-bound" {
		t.Fatalf("JSON report = %v", report)
	}
	// The deferral member sits outside the record-set digest.
	review := maps.Clone(report["review"].(map[string]any))
	digest := review["recordSetSha256"]
	delete(review, "recordSetSha256")
	delete(review, "deferredIntents")
	if digest != reviewDigest(review) {
		t.Fatalf("recordSetSha256 %v covers the deferral member", digest)
	}
	preview, err := openSession(t, root).Read(ctx(), ActionReportPreview, ReadOptions{
		MapPath: wire.ExcludedCEMPath, ExpectedBase: base, Target: head, ArtifactRoot: root, ArtifactsGiven: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	markdown := preview["markdown"].(string)
	if !strings.Contains(markdown, "\n## Deferred intent scopes\n") || !strings.Contains(markdown, "`docs/intent.md`: 2 obligations") ||
		!strings.Contains(markdown, "`scope deferred to V1-9`") || !strings.Contains(markdown, "`"+DeferredIssueExceedBound+"`") {
		t.Fatalf("report preview lacks the deferral section:\n%s", markdown)
	}
}

// TestCEMV1645StatusRefusesMisnamedAndMismatchedRecords commits a misnamed
// record and a content-named record whose base blob is wrong: status names
// both issues and clears ok while the verifier still accepts.
func TestCEMV1645StatusRefusesMisnamedAndMismatchedRecords(t *testing.T) {
	t.Parallel()
	root, base, _ := makeRepo(t)
	writeFile(t, root, "docs/intent.md", deferredIntentFixture)
	wrong := strings.Repeat("b", 40)
	mismatched, err := EncodeDeferredIntents([]DeferredIntent{{BaseBlobOid: &wrong, ObligationCount: 2, Path: "docs/intent.md", Reason: "r"}})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, DeferredRecordPath(mismatched), string(mismatched))
	writeFile(t, root, DeferredIntentsDir+"/"+strings.Repeat("0", 64)+".json", string(mismatched))
	head := commitDeferredChange(t, root, base)
	result := deferredRead(t, root, "status", base, head, intLimit(4), fakeIntentCounter)
	codes := deferredIssueCodes(deferredMember(t, result, "deferredIntents"))
	if result["ok"] != false || stableOutcome(result) != "ACCEPT" ||
		strings.Join(codes, ",") != DeferredIssueRecordNameMismatch+","+DeferredIssueBaseMismatch {
		encoded, _ := json.Marshal(result)
		t.Fatalf("status = %s", encoded)
	}
}
