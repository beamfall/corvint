package betarung

import (
	"encoding/json"
	"strings"
	"testing"
)

// admittedFixture is one syntactically complete record that admits `harness` on
// the `cli` surface. Tests mutate a copy of it to show exactly which mutation
// each rule refuses.
func admittedFixture() Record {
	passing := Finding{Verdict: Pass, Sources: []string{"conformance/fixture.json"}, Detail: "fixture"}
	return Record{
		Profile:    Profile,
		PriorLabel: Fallback,
		OpenDivergences: []OpenDivergence{
			{ID: "DR-0005", Commands: []string{"harness"}, Summary: "parked spec-gap"},
		},
		Admissions: []Admission{{
			Command:  "harness",
			Surfaces: []string{"cli"},
			Admitted: true,
			Evidence: Evidence{
				GPKV0017Criteria:      passing,
				W11ReproducibleBuild:  passing,
				W12DogfoodAndRollback: passing,
			},
			DisclosedDivergences: []string{"DR-0005"},
		}},
	}
}

func parseFixture(t *testing.T, record Record) (Record, error) {
	t.Helper()
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return Parse(encoded)
}

func TestAdmittedCommandStatesBetaOnItsOwnSurfaceOnly(t *testing.T) {
	record, err := parseFixture(t, admittedFixture())
	if err != nil {
		t.Fatalf("parse admitted fixture: %v", err)
	}
	if label := record.Label("harness", "cli"); label != Beta {
		t.Fatalf("admitted pair states %q, want %q", label, Beta)
	}
	if label := record.Label("harness", "plugin"); label != Fallback {
		t.Fatalf("unadmitted surface states %q, want %q; GPK-V0-042 admits no global label", label, Fallback)
	}
	if label := record.Label("query", "cli"); label != Fallback {
		t.Fatalf("unadmitted command states %q, want %q", label, Fallback)
	}
}

// TestUndisclosedOpenEntryInvalidatesAdmission is the demonstrated red for the
// disclosure obligation: the same record is accepted with the disclosure and
// refused without it, so the rule discriminates rather than passing quietly.
func TestUndisclosedOpenEntryInvalidatesAdmission(t *testing.T) {
	if _, err := parseFixture(t, admittedFixture()); err != nil {
		t.Fatalf("disclosed admission must be valid: %v", err)
	}

	undisclosed := admittedFixture()
	undisclosed.Admissions[0].DisclosedDivergences = nil
	_, err := parseFixture(t, undisclosed)
	if err == nil {
		t.Fatal("an admission holding an undisclosed open entry was accepted; GPK-V0-042 makes it invalid")
	}
	if !strings.Contains(err.Error(), "DR-0005") {
		t.Fatalf("refusal does not name the undisclosed entry: %v", err)
	}
}

// An open entry obliges disclosure; it never blocks admission. GPK-V0-042 term 5.
func TestOpenEntryDoesNotBlockAdmission(t *testing.T) {
	record, err := parseFixture(t, admittedFixture())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := record.AdmittedCommands(); len(got) != 1 || got[0] != "harness" {
		t.Fatalf("admitted commands %v, want [harness]", got)
	}
}

func TestStaleDisclosureIsRefused(t *testing.T) {
	stale := admittedFixture()
	stale.Admissions[0].DisclosedDivergences = []string{"DR-0005", "DR-0006"}
	_, err := parseFixture(t, stale)
	if err == nil {
		t.Fatal("a disclosure naming a closed entry was accepted")
	}
	if !strings.Contains(err.Error(), "DR-0006") {
		t.Fatalf("refusal does not name the stale disclosure: %v", err)
	}
}

func TestAdmissionRequiresPassOnAllThreeNamedSources(t *testing.T) {
	for _, weakened := range []struct {
		name   string
		mutate func(*Record)
	}{
		{"gpkV0017", func(r *Record) { r.Admissions[0].Evidence.GPKV0017Criteria.Verdict = NotRun }},
		{"w11", func(r *Record) { r.Admissions[0].Evidence.W11ReproducibleBuild.Verdict = NotRun }},
		{"w12", func(r *Record) { r.Admissions[0].Evidence.W12DogfoodAndRollback.Verdict = Fail }},
	} {
		t.Run(weakened.name, func(t *testing.T) {
			record := admittedFixture()
			weakened.mutate(&record)
			if _, err := parseFixture(t, record); err == nil {
				t.Fatal("admission accepted on weaker evidence than GPK-V0-042 names")
			}
		})
	}
}

func TestVerdictWithoutAnEvidenceFileIsRefused(t *testing.T) {
	record := admittedFixture()
	record.Admissions[0].Evidence.W11ReproducibleBuild.Sources = nil
	if _, err := parseFixture(t, record); err == nil {
		t.Fatal("a verdict citing no evidence file was accepted")
	}
}

func TestPriorLabelMustStayFallback(t *testing.T) {
	for _, label := range []Label{Beta, Full, "EXPERIMENTAL"} {
		record := admittedFixture()
		record.PriorLabel = label
		if _, err := parseFixture(t, record); err == nil {
			t.Fatalf("priorLabel %q accepted; GPK-V0-023 keeps a partial slice at FALLBACK", label)
		}
	}
}

func TestAdmissionWithoutASurfaceIsRefused(t *testing.T) {
	record := admittedFixture()
	record.Admissions[0].Surfaces = nil
	if _, err := parseFixture(t, record); err == nil {
		t.Fatal("an admission with no integration surface was accepted")
	}
}

func TestUnknownFieldIsRefused(t *testing.T) {
	if _, err := Parse([]byte(`{"profile":"corvint-beta-rung/0","priorLabel":"FALLBACK","surprise":1}`)); err == nil {
		t.Fatal("a record carrying an unknown field decided a label")
	}
}

const registerFixture = `# Divergence register

### DR-0001 - lrf: something adjudicated
- **Status:** LANDED - adjudicated ` + "`python-defect`" + `
- **Command:** ` + "`lrf`" + `. **Discovered:** 2026-08-28.

### DR-0005 - harness: something parked
- **Status:** OPEN - adjudicated ` + "`spec-gap`" + ` and therefore parked
- **Command:** ` + "`harness event --event user-prompt`" + `. **Discovered:** 2026-08-28.
`

func TestOpenRegisterEntriesReadOnlyTheStatusBullet(t *testing.T) {
	open := OpenRegisterEntries([]byte(registerFixture))
	if len(open) != 1 {
		t.Fatalf("open entries %d, want 1: %+v", len(open), open)
	}
	if open[0].ID != "DR-0005" {
		t.Fatalf("open entry %q, want DR-0005", open[0].ID)
	}
	if len(open[0].Commands) != 1 || open[0].Commands[0] != "harness" {
		t.Fatalf("open entry commands %v, want [harness]", open[0].Commands)
	}
}

// An entry is closed only by a status word the register uses for closure, so
// an emphasized, misspelled or missing status still obliges disclosure.
func TestOpenRegisterEntriesTreatAnUnrecognizedStatusAsOpen(t *testing.T) {
	for _, status := range []string{"- **Status:** **OPEN** - parked", "- **Status:** Open", "- **Status:**", "- no status bullet"} {
		markdown := strings.Replace(registerFixture, "- **Status:** OPEN - adjudicated `spec-gap` and therefore parked", status, 1)
		open := OpenRegisterEntries([]byte(markdown))
		if len(open) != 1 || open[0].ID != "DR-0005" {
			t.Fatalf("status %q: open entries %+v, want DR-0005", status, open)
		}
	}
}

func TestCheckRegisterFailsWhenTheRecordDriftsFromTheRegister(t *testing.T) {
	record, err := parseFixture(t, admittedFixture())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := record.CheckRegister([]byte(registerFixture)); err != nil {
		t.Fatalf("matching record refused: %v", err)
	}

	closed := strings.Replace(registerFixture, "- **Status:** OPEN", "- **Status:** LANDED", 1)
	if err := record.CheckRegister([]byte(closed)); err == nil {
		t.Fatal("a record still declaring a closed entry as open was accepted")
	}

	opened := registerFixture + "\n### DR-0099 - impact: newly opened\n- **Status:** OPEN\n- **Command:** `impact`.\n"
	if err := record.CheckRegister([]byte(opened)); err == nil {
		t.Fatal("a record missing a newly opened entry was accepted")
	}
}

func TestCheckRegisterFailsWhenADeclaredCommandIsNotInTheRegisterEntry(t *testing.T) {
	record := admittedFixture()
	record.OpenDivergences[0].Commands = []string{"query"}
	record.Admissions[0].DisclosedDivergences = nil
	record.Admissions[0].Admitted = false
	parsed, err := parseFixture(t, record)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := parsed.CheckRegister([]byte(registerFixture)); err == nil {
		t.Fatal("a record attributing an open entry to a command the register does not name was accepted")
	}
}

// A record this package cannot parse must never mint a BETA claim.
func TestBrokenRecordDegradesToFallback(t *testing.T) {
	broken := Record{}
	if label := broken.Label("harness", "cli"); label != Fallback {
		t.Fatalf("empty record states %q, want %q", label, Fallback)
	}
}

// A register entry that names more commands than the record declares would let
// an undeclared command reach BETA without disclosing the entry it holds.
func TestCheckRegisterFailsWhenTheRegisterNamesAnUndeclaredCommand(t *testing.T) {
	record := admittedFixture()
	record.Admissions = append(record.Admissions, Admission{
		Command:  "query",
		Surfaces: []string{"cli"},
		Admitted: true,
		Evidence: record.Admissions[0].Evidence,
	})
	parsed, err := parseFixture(t, record)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	widened := strings.Replace(registerFixture, "`harness event --event user-prompt`", "`harness event --event user-prompt`, `query`", 1)
	if err := parsed.CheckRegister([]byte(widened)); err == nil {
		t.Fatalf("query states %q while holding undeclared open DR-0005 without disclosing it", parsed.Label("query", "cli"))
	}
}

// The register wraps a long Command bullet onto indented continuation lines.
// A command named only there must still oblige disclosure, or an admitted
// command could hold the open entry without disclosing it.
func TestCheckRegisterReadsAWrappedCommandBullet(t *testing.T) {
	record := admittedFixture()
	record.OpenDivergences[0].Commands = []string{"--limit"}
	record.Admissions[0].DisclosedDivergences = nil
	parsed, err := parseFixture(t, record)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wrapped := strings.Replace(registerFixture,
		"- **Command:** `harness event --event user-prompt`. **Discovered:** 2026-08-28.",
		"- **Command:** every surface under a `--limit` ceiling with no\n  `--budget`: the `harness` paths. **Discovered:** 2026-09-04.", 1)
	if err := parsed.CheckRegister([]byte(wrapped)); err == nil {
		t.Fatalf("harness states %q while holding open DR-0005, named on a continuation line, without disclosing it", parsed.Label("harness", "cli"))
	}
}
