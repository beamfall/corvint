package lrfrepo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Fixtures are short excerpts minimized from Beamfall docs/adr/0224-*.md and
// docs/plans/roadmap/41-*.md shapes at Beamfall 2a8e06b28.

const adrFixture = "---\nadr: 0224\ntitle: Plugin-first authority kernel\nstatus: accepted\n---\n\n" +
	"# ADR-0224 — Plugin-first authority kernel\n\n## Context\n\n### 1. Not a decision\n\n" +
	"## Decisions\n\n### 1. Core is the authority kernel\n\nCore owns authority.\n\n" +
	"```text\n### 9. fenced, not an item\n```\n\n#### Detail\n\n" +
	"### 2. Media processing uses narrow provider contracts\n\nOne envelope.\n\n" +
	"### 6a. Capability compatibility is a closed Core contract\n\nClosed.\n\n" +
	"## Verification implications\n\n### 3. Not a decision either\n"

const roadmapFixture = "# Podcast ad skip\n\n" +
	"- [x] **PODAD-2 — Closure: Core per-episode audio ad detector job (`podcast-ad-audio`).** Done.\n" +
	"  - **Allowed Paths:** `internal/store/**`\n" +
	"  - **Acceptance:** revoking a model version completes.\n" +
	"  - **Verify:** `[auto]` regression.\n\n" +
	"- [ ] **STORE-MIGRATE-USERVERSION-ORDER-1 — Stop `PRAGMA user_version` regressing.** Open.\n" +
	"  - **Acceptance:** `user_version` never decreases.\n" +
	"    across an open.\n" +
	"  - **Acceptance:** an earlier-sorting migration is refused.\n" +
	"  - **Verify:** `[auto]` reopen regression.\n" +
	"    - [ ] **NESTED-1 — nested ticket stays in the parent body.**\n" +
	"- [ ] **RELAY-PROD-ACCEPT-BACKOFF-1 — Keep the accept loops alive.**\n" +
	"  - **Acceptance:**\n" +
	"    - an injected temporary `Accept` error is followed by a successful accept\n" +
	"  - **Acceptance:**x is not an Acceptance line\n" +
	"  - **Verify:**\n" +
	"    - `[auto]` `go test -race ./internal/relay/`\n" +
	"- [ ] **PODAD-2-BENCH-EVIDENCE-POINTER-1 — repair the stale bench-transcript pointer.** Verify only.\n" +
	"  - **Verify:** `[auto]` pointer resolves.\n\n" +
	"```markdown\n- [ ] **FENCED-1 — fenced, not a ticket.**\n  - **Acceptance:** ignored.\n```\n"

func TestOIFV0ADRDecisionsDerivation(t *testing.T) {
	data := []byte(adrFixture)
	derived, err := deriveIntent(IntentFormADR, "docs/adr/0224-plugin-first-authority-kernel.md", "0123456789abcdef", data)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := derived.requirements, []string{"ADR-0224-D1", "ADR-0224-D2", "ADR-0224-D6a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requirements=%v want=%v", got, want)
	}
	if !bytes.HasPrefix(derived.scope, []byte("## Decisions\n")) || !bytes.HasSuffix(derived.scope, []byte("Closed.\n\n")) {
		t.Fatalf("scope=%q", derived.scope)
	}
	if derived.intent.form != IntentFormADR || derived.intent.spanSHA256 != sha256Hex(derived.scope) {
		t.Fatalf("intent=%+v", derived.intent)
	}
	statement, err := obligationStatement(derived.statements, derived.scope, "ADR-0224-D2")
	if err != nil || string(statement) != "### 2. Media processing uses narrow provider contracts\n\nOne envelope.\n\n" {
		t.Fatalf("statement=%q err=%v", statement, err)
	}
}

func TestOIFV0ADRDecisionsRefusals(t *testing.T) {
	decisions := "## Decisions\n\n### 1. One\n"
	cases := map[string]struct{ data, code string }{
		"duplicate decision":  {"---\nadr: 0001\n---\n## Decisions\n### 1. A\n### 1. B\n", "duplicate-requirement"},
		"D-prefixed item":     {"---\nadr: 0001\n---\n## Decisions\n### D1 — A\n", "invalid-decision-item"},
		"uppercase suffix":    {"---\nadr: 0001\n---\n## Decisions\n### 8A. A\n", "invalid-decision-item"},
		"dotted number":       {"---\nadr: 0001\n---\n## Decisions\n### 2.1 A\n", "invalid-decision-item"},
		"indented item":       {"---\nadr: 0001\n---\n## Decisions\n ### 1. A\n", "invalid-decision-item"},
		"two sections":        {"---\nadr: 0001\n---\n" + decisions + decisions, "invalid-decisions-section"},
		"singular heading":    {"---\nadr: 0001\n---\n## Decision\n### 1. A\n", "invalid-decisions-section"},
		"numbered heading":    {"---\nadr: 0001\n---\n## 2. Decisions\n### 1. A\n", "invalid-decisions-section"},
		"fenced section only": {"---\nadr: 0001\n---\n```\n## Decisions\n### 1. A\n```\n", "invalid-decisions-section"},
		"no front matter":     {decisions, "invalid-adr-number"},
		"two adr lines":       {"---\nadr: 0001\nadr: 0002\n---\n" + decisions, "invalid-adr-number"},
		"short adr number":    {"---\nadr: 224\n---\n" + decisions, "invalid-adr-number"},
		"unclosed matter":     {"---\nadr: 0001\n" + decisions, "invalid-adr-number"},
		"no items":            {"---\nadr: 0001\n---\n## Decisions\n\nProse only.\n", "missing-requirements"},
		"invalid UTF-8":       {"---\nadr: 0001\n---\n## Decisions\n### 1. \xff\n", "invalid-intent"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := deriveIntent(IntentFormADR, "docs/adr/0001-x.md", "0123456789abcdef", []byte(test.data))
			if got := CodeOf(err); got != test.code {
				t.Fatalf("code=%q want=%q err=%v", got, test.code, err)
			}
		})
	}
}

func TestOIFV0RoadmapAcceptanceDerivation(t *testing.T) {
	data := []byte(roadmapFixture)
	derived, err := deriveIntent(IntentFormRoadmap, "docs/plans/roadmap/41-podcast.md", "0123456789abcdef", data)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := derived.requirements, []string{"PODAD-2", "STORE-MIGRATE-USERVERSION-ORDER-1", "RELAY-PROD-ACCEPT-BACKOFF-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requirements=%v want=%v", got, want)
	}
	if got, want := derived.excluded, []string{"PODAD-2-BENCH-EVIDENCE-POINTER-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("excluded=%v want=%v", got, want)
	}
	if derived.intent.start != 0 || derived.intent.end != int64(len(data)) {
		t.Fatalf("span=%d..%d", derived.intent.start, derived.intent.end)
	}
	want := "  - **Acceptance:** `user_version` never decreases.\n    across an open.\n" +
		"  - **Acceptance:** an earlier-sorting migration is refused.\n"
	if got := string(derived.statements["STORE-MIGRATE-USERVERSION-ORDER-1"]); got != want {
		t.Fatalf("statement=%q want=%q", got, want)
	}
	want = "  - **Acceptance:**\n    - an injected temporary `Accept` error is followed by a successful accept\n"
	if got := string(derived.statements["RELAY-PROD-ACCEPT-BACKOFF-1"]); got != want {
		t.Fatalf("block statement=%q want=%q", got, want)
	}
}

func TestOIFV0RoadmapAcceptanceRefusals(t *testing.T) {
	accepted := "  - **Acceptance:** holds.\n"
	cases := map[string]struct{ data, code string }{
		"composite ID":     {"- [ ] **CLOUD-1 / FND-E — x.**\n" + accepted, "invalid-ticket-item"},
		"unclosed bold":    {"- [ ] **ABC-1 — title without close\n" + accepted, "invalid-ticket-item"},
		"no em dash":       {"- [ ] **ABC-1 - title.**\n" + accepted, "invalid-ticket-item"},
		"empty title":      {"- [ ] **ABC-1 — **\n" + accepted, "invalid-ticket-item"},
		"lowercase ID":     {"- [ ] **abc-1 — title.**\n" + accepted, "invalid-ticket-item"},
		"no dash in ID":    {"- [ ] **ABC — title.**\n" + accepted, "invalid-ticket-item"},
		"plain task item":  {"- [ ] write the thing\n" + accepted, "invalid-ticket-item"},
		"overlong ID":      {"- [ ] **A-" + strings.Repeat("B", 63) + " — title.**\n" + accepted, "invalid-ticket-item"},
		"duplicate ticket": {"- [ ] **ABC-1 — a.**\n" + accepted + "- [x] **ABC-1 — b.**\n", "duplicate-requirement"},
		"no acceptance":    {"- [ ] **ABC-1 — a.**\n  - **Verify:** runs.\n", "missing-requirements"},
		"no tickets":       {"# Empty shard\n", "missing-requirements"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := deriveIntent(IntentFormRoadmap, "docs/plans/roadmap/x.md", "0123456789abcdef", []byte(test.data))
			if got := CodeOf(err); got != test.code {
				t.Fatalf("code=%q want=%q err=%v", got, test.code, err)
			}
		})
	}
}

func TestOIFV0DefaultFormIsUnchanged(t *testing.T) {
	data := []byte("# Spec\n\n## Requirements\n\n- `ABC-V0-001`: holds.\n\n## Non-goals\n")
	intent, requirements, scope, err := requirementsFromBlob("docs/specs/a.md", "0123456789abcdef", data)
	if err != nil {
		t.Fatal(err)
	}
	derived, err := deriveIntent("", "docs/specs/a.md", "0123456789abcdef", data)
	if err != nil || derived.intent != intent || !reflect.DeepEqual(derived.requirements, requirements) ||
		!bytes.Equal(derived.scope, scope) || derived.statements != nil {
		t.Fatalf("default derivation drifted: %+v err=%v", derived, err)
	}
	encoded := canonicalValue(intentScopeValue(intent))
	if bytes.Contains(encoded, []byte("form")) {
		t.Fatalf("default intentScope carries a form: %s", encoded)
	}
	if _, err := deriveIntent("requirements", "docs/specs/a.md", "0123456789abcdef", data); CodeOf(err) != "invalid-intent-form" {
		t.Fatalf("library accepted a second encoding of the default form: %v", err)
	}
}

func TestOIFV0IntentScopeWireForm(t *testing.T) {
	intent := ocmIntent{path: "docs/adr/0224-x.md", blobOID: strings.Repeat("a", 40), start: 1, end: 2,
		spanSHA256: strings.Repeat("b", 64), form: IntentFormADR}
	parsed, err := parseIntent(intentScopeValue(intent))
	if err != nil || parsed != intent {
		t.Fatalf("round trip=%+v err=%v", parsed, err)
	}
	for _, form := range []string{"requirements", "", "bogus"} {
		value := intentScopeValue(intent)
		value.Obj.Values["form"] = stringValue(form)
		if _, err := parseIntent(value); CodeOf(err) != "invalid-intent-form" {
			t.Fatalf("form %q accepted: %v", form, err)
		}
	}
	value := intentScopeValue(intent)
	value.Obj.Values["form"] = intValue(1)
	if _, err := parseIntent(value); CodeOf(err) != "invalid-intent-form" {
		t.Fatalf("non-string form accepted: %v", err)
	}
}

func TestOIFV0ObligationIDsFollowTheDeclaredForm(t *testing.T) {
	obligation := func(id string) wire.Value {
		return wire.Value{Kind: wire.KindArray, Arr: []wire.Value{objectValue(
			[2]any{"id", stringValue(id)}, [2]any{"disposition", stringValue("unknown")},
			[2]any{"reason", stringValue("unassessed")}, [2]any{"hunkIds", emptyArray()},
			[2]any{"claimIds", emptyArray()},
		)}}
	}
	cases := []struct {
		form, id string
		ok       bool
	}{
		{IntentFormADR, "ADR-0224-D6a", true},
		{"", "ADR-0224-D6a", false},
		{IntentFormADR, "OCM-V0-001", false},
		{IntentFormRoadmap, "PODAD-2-BENCH-EVIDENCE-POINTER-1", true},
		{"", "PODAD-2", false},
	}
	for _, test := range cases {
		_, err := parseObligations(obligation(test.id), test.form)
		if (err == nil) != test.ok {
			t.Fatalf("form=%q id=%q err=%v", test.form, test.id, err)
		}
	}
}

func TestOIFV0LRFRefusesDeclaredForms(t *testing.T) {
	if err := refuseDeclaredIntentForm(&ocmDocument{}); err != nil {
		t.Fatal(err)
	}
	err := refuseDeclaredIntentForm(&ocmDocument{intent: ocmIntent{form: IntentFormRoadmap}})
	if CodeOf(err) != "unsupported-intent-form" {
		t.Fatalf("err=%v", err)
	}
}

func TestOIFV0PrepareRecordsAndRederivesTheDeclaredForm(t *testing.T) {
	root, _, _ := makeOCMPrepareRepository(t)
	writeOCMPrepareFile(t, root, "docs/adr/0224-kernel.md", adrFixture)
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "adr base")
	base := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	writeOCMPrepareFile(t, root, "internal/example.txt", "adr change\n")
	gitOCMPrepare(t, root, "commit", "-qam", "adr target")
	target := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareCEM(t, root, base, target, true)
	options := PrepareOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/adr/0224-kernel.md", ExpectedBase: base, Target: target, IntentForm: IntentFormADR,
	}
	first, err := PrepareOCM(context.Background(), root, options)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := first.Requirements, []string{"ADR-0224-D1", "ADR-0224-D2", "ADR-0224-D6a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requirements=%v want=%v", got, want)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(options.MapPath)))
	if err != nil || !strings.Contains(string(raw), `"form":"adr-decisions"`) || first.IntentScope["form"] != IntentFormADR {
		t.Fatalf("form not recorded: err=%v bytes=%s", err, raw)
	}
	if _, err := MarkOCM(context.Background(), root, MarkOptions{
		MapPath: options.MapPath, Obligation: "ADR-0224-D6a", Reason: "no-test-claim",
	}); err != nil {
		t.Fatal(err)
	}
	resumed, err := PrepareOCM(context.Background(), root, options)
	if err != nil || !resumed.Resumed {
		t.Fatalf("resume: err=%v", err)
	}
	if _, err := ReadOCM(context.Background(), root, OCMReadOptions{
		OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: base, Target: target,
		ExpectedBaseGiven: true, TargetGiven: true,
	}); err != nil {
		t.Fatal(err)
	}
	options.IntentForm = ""
	if _, err := PrepareOCM(context.Background(), root, options); CodeOf(err) != "invalid-requirements-section" {
		t.Fatalf("default form read an ADR: %v", err)
	}
}
