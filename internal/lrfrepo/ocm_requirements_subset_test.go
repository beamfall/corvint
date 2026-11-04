package lrfrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// overBoundSpec is a spec whose Requirements section holds more requirement
// IDs than one OCM map admits (V1-2275).
func overBoundSpec(count int) []byte {
	var builder strings.Builder
	builder.WriteString("# Big\n\n## Requirements\n\n")
	for number := 1; number <= count; number++ {
		fmt.Fprintf(&builder, "- `BIG-V0-%03d`: requirement %d.\n", number, number)
	}
	builder.WriteString("\n## Non-goals\n\nNone.\n")
	return []byte(builder.String())
}

func subsetIDs(from, to int) []string {
	ids := make([]string, 0, to-from+1)
	for number := from; number <= to; number++ {
		ids = append(ids, fmt.Sprintf("BIG-V0-%03d", number))
	}
	return ids
}

// OIF-V0-670 and OIF-V0-671: the subset derives from a spec over the bound,
// keeps document order, binds the whole section span, and is itself bounded.
func TestOIFV0670RequirementsSubsetDerivesFromAnOverBoundSpec(t *testing.T) {
	data := overBoundSpec(300)
	if _, err := deriveIntent("", "docs/specs/big.md", "0123456789abcdef", data); CodeOf(err) != "too-many-obligations" {
		t.Fatalf("default form admitted 300 requirements: %v", err)
	}
	subset := []string{"BIG-V0-007", "BIG-V0-271", "BIG-V0-300"}
	derived, err := deriveRecordedIntent(IntentFormRequirementsSubset, subset, "docs/specs/big.md", "0123456789abcdef", data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(derived.requirements, subset) {
		t.Fatalf("requirements=%v", derived.requirements)
	}
	whole, _, scope, err := scanRequirements("docs/specs/big.md", "0123456789abcdef", data, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	if derived.intent.start != whole.start || derived.intent.end != whole.end || derived.intent.spanSHA256 != whole.spanSHA256 || string(derived.scope) != string(scope) {
		t.Fatalf("subset span differs from the whole Requirements section")
	}
	if derived.intent.form != IntentFormRequirementsSubset || derived.intent.subset != strings.Join(subset, ",") {
		t.Fatalf("intent=%+v", derived.intent)
	}
	statement, err := obligationStatement(derived.statements, derived.scope, "BIG-V0-271")
	if err != nil || !strings.Contains(string(statement), "requirement 271.") {
		t.Fatalf("statement=%q err=%v", statement, err)
	}
	if _, err := deriveRecordedIntent(IntentFormRequirementsSubset, subsetIDs(1, 256), "docs/specs/big.md", "0123456789abcdef", data); err != nil {
		t.Fatalf("256 listed IDs refused: %v", err)
	}
	refusals := []struct {
		name   string
		subset []string
		code   string
	}{
		{"over the bound", subsetIDs(1, 257), "too-many-obligations"},
		{"empty", nil, "missing-requirements"},
		{"absent ID", []string{"BIG-V0-007", "BIG-V0-301"}, "unknown-subset-requirement"},
		{"unsorted", []string{"BIG-V0-009", "BIG-V0-008"}, "invalid-intent-scope"},
		{"duplicate", []string{"BIG-V0-008", "BIG-V0-008"}, "invalid-intent-scope"},
		{"not an ID", []string{"big-v0-008"}, "invalid-intent-scope"},
	}
	for _, test := range refusals {
		if _, err := deriveRecordedIntent(IntentFormRequirementsSubset, test.subset, "docs/specs/big.md", "0123456789abcdef", data); CodeOf(err) != test.code {
			t.Fatalf("%s: err=%v want %s", test.name, err, test.code)
		}
	}
	if _, err := deriveIntent(IntentFormRequirementsSubset, "docs/specs/big.md", "0123456789abcdef", data); CodeOf(err) != "missing-requirements" {
		t.Fatalf("unlisted subset derived: %v", err)
	}
	if _, err := deriveRecordedIntent("", subset, "docs/specs/big.md", "0123456789abcdef", data); CodeOf(err) != "invalid-intent-scope" {
		t.Fatalf("default form took listed IDs: %v", err)
	}
	if err := refuseDeclaredIntentForm(&ocmDocument{intent: derived.intent}); CodeOf(err) != "unsupported-intent-form" {
		t.Fatalf("LRF admitted the subset form: %v", err)
	}
}

// OIF-V0-671: intentScope.requirements is present exactly for the subset form
// and round-trips; a malformed or over-bound list is refused on read.
func TestOIFV0671IntentScopeRequirementsWireMember(t *testing.T) {
	intent := ocmIntent{path: "docs/specs/big.md", blobOID: strings.Repeat("a", 40), start: 1, end: 2,
		spanSHA256: strings.Repeat("b", 64), form: IntentFormRequirementsSubset, subset: "BIG-V0-001,BIG-V0-300"}
	parsed, err := parseIntent(intentScopeValue(intent))
	if err != nil || parsed != intent {
		t.Fatalf("round trip=%+v err=%v", parsed, err)
	}
	listed := func(ids ...string) wire.Value {
		values := make([]wire.Value, 0, len(ids))
		for _, id := range ids {
			values = append(values, stringValue(id))
		}
		return wire.Value{Kind: wire.KindArray, Arr: values}
	}
	cases := []struct {
		name   string
		mutate func(wire.Value)
		code   string
	}{
		{"missing member", func(value wire.Value) {
			delete(value.Obj.Values, "requirements")
			value.Obj.Keys = value.Obj.Keys[:len(value.Obj.Keys)-1]
		}, "invalid-intent-scope"},
		{"unsorted", func(value wire.Value) { value.Obj.Values["requirements"] = listed("BIG-V0-300", "BIG-V0-001") }, "invalid-intent-scope"},
		{"not an array", func(value wire.Value) { value.Obj.Values["requirements"] = stringValue("BIG-V0-001") }, "invalid-intent-scope"},
		{"non-string", func(value wire.Value) {
			value.Obj.Values["requirements"] = wire.Value{Kind: wire.KindArray, Arr: []wire.Value{intValue(1)}}
		}, "invalid-intent-scope"},
		{"over the bound", func(value wire.Value) { value.Obj.Values["requirements"] = listed(subsetIDs(1, 257)...) }, "too-many-obligations"},
		{"empty", func(value wire.Value) { value.Obj.Values["requirements"] = listed() }, "missing-requirements"},
	}
	for _, test := range cases {
		value := intentScopeValue(intent)
		test.mutate(value)
		if _, err := parseIntent(value); CodeOf(err) != test.code {
			t.Fatalf("%s: err=%v want %s", test.name, err, test.code)
		}
	}
	other := intentScopeValue(ocmIntent{path: "docs/adr/0224-x.md", blobOID: strings.Repeat("a", 40), start: 1, end: 2,
		spanSHA256: strings.Repeat("b", 64), form: IntentFormADR})
	other.Obj.Keys = append(other.Obj.Keys, "requirements")
	other.Obj.Values["requirements"] = listed("BIG-V0-001")
	if _, err := parseIntent(other); CodeOf(err) != "invalid-intent-scope" {
		t.Fatalf("requirements accepted on another form: %v", err)
	}
	if _, err := parseObligations(wire.Value{Kind: wire.KindArray, Arr: []wire.Value{objectValue(
		[2]any{"id", stringValue("BIG-V0-001")}, [2]any{"disposition", stringValue("unknown")},
		[2]any{"reason", stringValue("unassessed")}, [2]any{"hunkIds", emptyArray()},
		[2]any{"claimIds", emptyArray()},
	)}}, IntentFormRequirementsSubset); err != nil {
		t.Fatalf("subset obligation ID refused: %v", err)
	}
}

// OIF-V0-672: prepare records the subset, mark/resume/read re-derive it, and a
// changed subset is a binding change.
func TestOIFV0672PrepareRecordsAndRederivesTheSubset(t *testing.T) {
	root, _, _ := makeOCMPrepareRepository(t)
	writeOCMPrepareFile(t, root, "docs/specs/big.md", string(overBoundSpec(300)))
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "big base")
	base := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	writeOCMPrepareFile(t, root, "internal/example.txt", "big change\n")
	gitOCMPrepare(t, root, "commit", "-qam", "big target")
	target := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareCEM(t, root, base, target, true)
	options := PrepareOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath,
		IntentPath: "docs/specs/big.md", ExpectedBase: base, Target: target,
		IntentForm: IntentFormRequirementsSubset, IntentRequirements: []string{"BIG-V0-002", "BIG-V0-299"},
	}
	first, err := PrepareOCM(context.Background(), root, options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Requirements, options.IntentRequirements) {
		t.Fatalf("requirements=%v", first.Requirements)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(options.MapPath)))
	if err != nil || !strings.Contains(string(raw), `"requirements":["BIG-V0-002","BIG-V0-299"]`) || !strings.Contains(string(raw), `"form":"requirements-subset"`) {
		t.Fatalf("subset not recorded: err=%v bytes=%s", err, raw)
	}
	if got := first.IntentScope["requirements"]; !reflect.DeepEqual(got, []any{"BIG-V0-002", "BIG-V0-299"}) {
		t.Fatalf("envelope intentScope requirements=%v", got)
	}
	if _, err := MarkOCM(context.Background(), root, MarkOptions{
		MapPath: options.MapPath, Obligation: "BIG-V0-299", Reason: "no-test-claim",
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
	changed := options
	changed.IntentRequirements = []string{"BIG-V0-002"}
	if _, err := PrepareOCM(context.Background(), root, changed); err == nil {
		t.Fatal("a changed subset resumed without --replace")
	}
	changed.Replace = true
	if replaced, err := PrepareOCM(context.Background(), root, changed); err != nil || !reflect.DeepEqual(replaced.Requirements, []string{"BIG-V0-002"}) {
		t.Fatalf("replace: requirements=%v err=%v", replaced, err)
	}
	options.IntentForm, options.IntentRequirements = "", nil
	options.Replace = true
	if _, err := PrepareOCM(context.Background(), root, options); CodeOf(err) != "too-many-obligations" {
		t.Fatalf("default form admitted the over-bound spec: %v", err)
	}
}
