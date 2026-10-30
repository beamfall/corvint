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

// Inline blobs follow the OpenSpec 1.14.3 and Spec Kit v1.2.1 shapes the STA-V0
// manifest pins; TestSTAV0010_FixturesDerive reads the committed fixtures.

const openSpecMainFixture = "# auth Specification\n\n## Purpose\nAuthenticate users.\n\n" +
	"### Requirement: Not in the section\n\n" +
	"## Requirements\n" +
	"### Requirement: User Login\nThe system SHALL sign users in.\n\n#### Scenario: Valid\n- **WHEN** valid\n- **THEN** signed in\n\n" +
	"```markdown\n### Requirement: Fenced, not a block\n## Fenced heading\n```\n\n" +
	"### REQUIREMENT:   C# Support ##  \nThe system SHALL parse C#.\n" +
	"## Notes\n### Requirement: After the section\n"

func TestSTAV0002_OpenSpecRequirementBlocks(t *testing.T) {
	data := []byte(openSpecMainFixture)
	derived, err := deriveIntent(IntentFormOpenSpecSpec, "openspec/specs/auth/spec.md", "0123456789abcdef", data)
	if err != nil {
		t.Fatal(err)
	}
	login, csharp := openSpecObligation("User Login"), openSpecObligation("C# Support")
	if login != "OSR-"+strings.ToUpper(sha256Hex([]byte("User Login"))[:12]) || !openSpecObligationID.MatchString(login) {
		t.Fatalf("obligation ID=%q", login)
	}
	if got, want := derived.requirements, []string{login, csharp}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requirements=%v want=%v", got, want)
	}
	want := "### Requirement: User Login\nThe system SHALL sign users in.\n\n#### Scenario: Valid\n- **WHEN** valid\n- **THEN** signed in\n\n" +
		"```markdown\n### Requirement: Fenced, not a block\n## Fenced heading\n```\n\n"
	if got := string(derived.statements[login]); got != want {
		t.Fatalf("statement=%q want=%q", got, want)
	}
	if got := string(derived.statements[csharp]); got != "### REQUIREMENT:   C# Support ##  \nThe system SHALL parse C#.\n" {
		t.Fatalf("closing-run statement=%q", got)
	}
	start := int64(strings.Index(openSpecMainFixture, "## Requirements\n"))
	if derived.intent.start != start || derived.intent.end != int64(strings.Index(openSpecMainFixture, "## Notes\n")) {
		t.Fatalf("span=%d..%d", derived.intent.start, derived.intent.end)
	}
	if got, want := derived.nativeNames, map[string]string{login: "User Login", csharp: "C# Support"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nativeNames=%v want=%v", got, want)
	}
}

func TestSTAV0002_OpenSpecSpecRefusals(t *testing.T) {
	block := "### Requirement: A\nThe system SHALL a.\n"
	cases := []struct{ name, blob, code string }{
		{"no section", "# x\n\n" + block, "invalid-requirements-section"},
		{"two sections", "## Requirements\n" + block + "## Requirements\n", "invalid-requirements-section"},
		{"other level-3 heading", "## Requirements\n" + block + "### Notes\n", "invalid-requirement-item"},
		{"empty name", "## Requirements\n### Requirement:  \n", "invalid-requirement-item"},
		{"repeated name", "## Requirements\n" + block + "\n" + "### Requirement: A ##\n", "duplicate-requirement"},
		{"no requirements", "## Requirements\nprose only\n", "missing-requirements"},
	}
	for _, test := range cases {
		if _, err := deriveIntent(IntentFormOpenSpecSpec, "openspec/specs/a/spec.md", "0123456789abcdef", []byte(test.blob)); CodeOf(err) != test.code {
			t.Fatalf("%s: err=%v want %s", test.name, err, test.code)
		}
	}
}

const openSpecDeltaFixture = "## ADDED Requirements\n### Requirement: Idle Warning\nThe system SHALL warn.\n\n#### Scenario: Shown\n- shown\n\n" +
	"## modified requirements\n### Requirement: Session Expiry\nThe system SHALL expire.\n\n" +
	"## REMOVED Requirements\n### Requirement: Remember Me\n**Reason**: gone.\n\n" +
	"## RENAMED Requirements\n- FROM: `### Requirement: User Login`\n- TO: `### Requirement: Credential Sign-In`\n\n" +
	"## ADDED Requirements\n### Requirement: Second Added\nThe system SHALL add.\n\n" +
	"## Removed Requirements\n### Requirement: Old Export\n\n" +
	"## REMOVED Requirements\n- `### Requirement: Bullet Ticked`\n- ### Requirement: Bullet Plain\n- not a requirement\n"

func TestSTAV0003_OpenSpecDeltaSections(t *testing.T) {
	data := []byte(openSpecDeltaFixture)
	derived, err := deriveIntent(IntentFormOpenSpecDelta, "openspec/changes/x/specs/auth/spec.md", "0123456789abcdef", data)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"Idle Warning", "Session Expiry", "Second Added"}
	want := make([]string, 0, len(names))
	for _, name := range names {
		want = append(want, openSpecObligation(name))
	}
	if !reflect.DeepEqual(derived.requirements, want) {
		t.Fatalf("requirements=%v want=%v", derived.requirements, want)
	}
	if got := string(derived.statements[want[0]]); got != "### Requirement: Idle Warning\nThe system SHALL warn.\n\n#### Scenario: Shown\n- shown\n\n" {
		t.Fatalf("statement=%q", got)
	}
	excluded := []map[string]string{
		{"operation": "REMOVED", "name": "Remember Me"},
		{"operation": "RENAMED", "from": "User Login", "to": "Credential Sign-In"},
		{"operation": "REMOVED", "name": "Old Export"},
		{"operation": "REMOVED", "name": "Bullet Ticked"},
		{"operation": "REMOVED", "name": "Bullet Plain"},
	}
	if !reflect.DeepEqual(derived.excludedRequirements, excluded) {
		t.Fatalf("excludedRequirements=%v want=%v", derived.excludedRequirements, excluded)
	}
	if derived.intent.start != 0 || derived.intent.end != int64(len(data)) {
		t.Fatalf("span=%d..%d", derived.intent.start, derived.intent.end)
	}
	if derived.nativeNames[want[1]] != "Session Expiry" || len(derived.nativeNames) != 3 {
		t.Fatalf("nativeNames=%v", derived.nativeNames)
	}
}

func TestSTAV0003_OpenSpecDeltaRefusals(t *testing.T) {
	added := "## ADDED Requirements\n### Requirement: A\nThe system SHALL a.\n"
	cases := []struct{ name, blob, code string }{
		{"no delta section", "# x\n\n### Requirement: A\n", "missing-requirements"},
		{"fenced delta heading", "```\n" + added + "```\n", "missing-requirements"},
		{"block before sections", "### Requirement: Z\n" + added, "invalid-delta-section"},
		{"block in other section", added + "## Notes\n### Requirement: Z\n", "invalid-delta-section"},
		{"block in RENAMED", added + "## RENAMED Requirements\n### Requirement: Z\n", "invalid-delta-section"},
		{"FROM without TO", added + "## RENAMED Requirements\n- FROM: `### Requirement: A1`\n", "invalid-delta-section"},
		{"TO without FROM", added + "## RENAMED Requirements\n- TO: `### Requirement: A1`\n", "invalid-delta-section"},
		{"FROM displaced", added + "## RENAMED Requirements\n- FROM: `### Requirement: A1`\n- FROM: `### Requirement: A2`\n- TO: `### Requirement: A3`\n", "invalid-delta-section"},
		{"FROM names no requirement", added + "## RENAMED Requirements\n- FROM: ### Requirement:\n", "invalid-delta-section"},
		{"TO names no requirement", added + "## RENAMED Requirements\n- FROM: `### Requirement: A`\n- TO: `### Requirement:`\n", "invalid-delta-section"},
		{"repeat across sections", added + "## MODIFIED Requirements\n### Requirement: A\nThe system SHALL a.\n", "duplicate-requirement"},
		{"other level-3 in ADDED", added + "### Notes\n", "invalid-requirement-item"},
		{"only removed and renamed", "## REMOVED Requirements\n### Requirement: A\n", "missing-requirements"},
	}
	for _, test := range cases {
		if _, err := deriveIntent(IntentFormOpenSpecDelta, "openspec/changes/x/specs/a/spec.md", "0123456789abcdef", []byte(test.blob)); CodeOf(err) != test.code {
			t.Fatalf("%s: err=%v want %s", test.name, err, test.code)
		}
	}
}

const specKitFixture = "# Feature Specification: Session Timeout\n\n**Feature Branch**: `001-x`  \n**Status**: Draft  \n\n" +
	"## User Scenarios & Testing *(mandatory)*\n\n- **FR-099**: outside, not a requirement.\n\n" +
	"## Requirements *(mandatory)*\n\n### Functional Requirements\n\n" +
	"- **FR-001**: System MUST end idle sessions.\n  after 30 minutes.\n- **FR-002**: System MUST retain records for [NEEDS CLARIFICATION: period]\n" +
	"- **SC-009**: an SC line outside Success Criteria.\n- **FR-0003**: System MUST warn.\n" +
	"```\n- **FR-004**: fenced.\n```\n\n" +
	"## Success Criteria *(mandatory)*\n\n### Measurable Outcomes\n\n- **SC-001**: 95% end on time.\n- **FR-005**: an FR line outside Requirements.\n"

func TestSTAV0005_SpecKitRequirementLines(t *testing.T) {
	data := []byte(specKitFixture)
	derived, err := deriveIntent(IntentFormSpecKit, "specs/001-x/spec.md", "0123456789abcdef", data)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := derived.requirements, []string{"FR-001", "FR-0003", "SC-001"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requirements=%v want=%v", got, want)
	}
	if got := string(derived.statements["FR-001"]); got != "- **FR-001**: System MUST end idle sessions.\n  after 30 minutes.\n" {
		t.Fatalf("statement=%q", got)
	}
	if got, want := derived.excludedRequirements, []map[string]string{{"id": "FR-002", "reason": "needs-clarification"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("excludedRequirements=%v want=%v", got, want)
	}
	if derived.nativeStatus == nil || *derived.nativeStatus != "Draft" || derived.nativeNames != nil {
		t.Fatalf("nativeStatus=%v nativeNames=%v", derived.nativeStatus, derived.nativeNames)
	}
	if derived.intent.start != 0 || derived.intent.end != int64(len(data)) {
		t.Fatalf("span=%d..%d", derived.intent.start, derived.intent.end)
	}
	noStatus := strings.Replace(specKitFixture, "**Status**: Draft  \n", "", 1) + "\n**Status**: Late\n"
	derived, err = deriveIntent(IntentFormSpecKit, "specs/001-x/spec.md", "0123456789abcdef", []byte(noStatus))
	if err != nil || derived.nativeStatus != nil {
		t.Fatalf("body Status read as header: %v err=%v", derived.nativeStatus, err)
	}
}

func TestSTAV0005_SpecKitRefusals(t *testing.T) {
	line := "- **FR-001**: System MUST a.\n"
	cases := []struct{ name, blob, code string }{
		{"no Requirements", "# x\n" + line, "invalid-requirements-section"},
		{"two Requirements", "## Requirements\n" + line + "## Requirements *(mandatory)*\n", "invalid-requirements-section"},
		{"two Success Criteria", "## Requirements\n" + line + "## Success Criteria\n## Success Criteria\n", "invalid-requirements-section"},
		{"repeated ID", "## Requirements\n" + line + line, "duplicate-requirement"},
		{"overlong ID", "## Requirements\n- **FR-" + strings.Repeat("1", 70) + "**: x.\n", "invalid-obligation-id"},
		{"only clarifications", "## Requirements\n- **FR-001**: [NEEDS CLARIFICATION: x]\n", "missing-requirements"},
	}
	for _, test := range cases {
		if _, err := deriveIntent(IntentFormSpecKit, "specs/001-x/spec.md", "0123456789abcdef", []byte(test.blob)); CodeOf(err) != test.code {
			t.Fatalf("%s: err=%v want %s", test.name, err, test.code)
		}
	}
}

func TestSTAV0006_DeclaredFormsRegistry(t *testing.T) {
	for _, form := range []string{IntentFormOpenSpecSpec, IntentFormOpenSpecDelta, IntentFormSpecKit} {
		if intentFormParsers[form] == nil || intentFormIDs[form] == nil {
			t.Fatalf("form %q not registered", form)
		}
		if err := refuseDeclaredIntentForm(&ocmDocument{intent: ocmIntent{form: form}}); CodeOf(err) != "unsupported-intent-form" {
			t.Fatalf("LRF accepted %q: %v", form, err)
		}
		intent := ocmIntent{path: "a.md", blobOID: strings.Repeat("a", 40), start: 0, end: 1, spanSHA256: strings.Repeat("b", 64), form: form}
		if parsed, err := parseIntent(intentScopeValue(intent)); err != nil || parsed != intent {
			t.Fatalf("intentScope.form %q round trip=%+v err=%v", form, parsed, err)
		}
	}
	obligation := func(id string) wire.Value {
		return wire.Value{Kind: wire.KindArray, Arr: []wire.Value{objectValue(
			[2]any{"id", stringValue(id)}, [2]any{"disposition", stringValue("unknown")},
			[2]any{"reason", stringValue("unassessed")}, [2]any{"hunkIds", emptyArray()},
			[2]any{"claimIds", emptyArray()},
		)}}
	}
	ids := []struct {
		form, id string
		ok       bool
	}{
		{IntentFormOpenSpecSpec, "OSR-0123456789AB", true},
		{IntentFormOpenSpecDelta, "OSR-0123456789AB", true},
		{IntentFormOpenSpecSpec, "OSR-0123456789ab", false},
		{IntentFormOpenSpecSpec, "FR-001", false},
		{IntentFormSpecKit, "SC-0042", true},
		{IntentFormSpecKit, "FR-01", false},
		{IntentFormSpecKit, "OSR-0123456789AB", false},
		{"", "OSR-0123456789AB", false},
	}
	for _, test := range ids {
		if _, err := parseObligations(obligation(test.id), test.form); (err == nil) != test.ok {
			t.Fatalf("form=%q id=%q err=%v", test.form, test.id, err)
		}
	}
	// Never inferred: the default form reads neither toolchain shape.
	if _, err := deriveIntent("", "openspec/specs/auth/spec.md", "0123456789abcdef", []byte(openSpecMainFixture)); CodeOf(err) != "missing-requirements" {
		t.Fatalf("default form read an OpenSpec blob: %v", err)
	}
	if _, err := deriveIntent("", "specs/001-x/spec.md", "0123456789abcdef", []byte(specKitFixture)); CodeOf(err) != "invalid-requirements-section" {
		t.Fatalf("default form read a Spec Kit blob: %v", err)
	}
	var many strings.Builder
	many.WriteString("## Requirements\n")
	for index := 1; index <= 257; index++ {
		fmt.Fprintf(&many, "- **FR-%03d**: System MUST %d.\n", index, index)
	}
	if _, err := deriveIntent(IntentFormSpecKit, "specs/001-x/spec.md", "0123456789abcdef", []byte(many.String())); CodeOf(err) != "too-many-obligations" {
		t.Fatalf("257 obligations: %v", err)
	}
}

func TestSTAV0004_NativeNamesPrepareAndStatus(t *testing.T) {
	root, _, _ := makeOCMPrepareRepository(t)
	mainSpec := "## Requirements\n### Requirement: Session Expiry\nThe system SHALL expire.\n\n### Requirement: session expiry\nThe system SHALL differ by case.\n"
	writeOCMPrepareFile(t, root, "openspec/specs/auth/spec.md", mainSpec)
	writeOCMPrepareFile(t, root, "specs/001-x/spec.md", specKitFixture)
	gitOCMPrepare(t, root, "add", ".")
	gitOCMPrepare(t, root, "commit", "-qm", "toolchain base")
	base := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	writeOCMPrepareFile(t, root, "internal/example.txt", "toolchain change\n")
	gitOCMPrepare(t, root, "commit", "-qam", "toolchain target")
	target := gitOCMPrepare(t, root, "rev-parse", "HEAD")
	prepareCEM(t, root, base, target, true)
	options := PrepareOptions{
		MapPath: ".corvint/change.ocm.json", CEMPath: wire.ExcludedCEMPath,
		IntentPath: "openspec/specs/auth/spec.md", ExpectedBase: base, Target: target, IntentForm: IntentFormOpenSpecSpec,
	}
	first, err := PrepareOCM(context.Background(), root, options)
	if err != nil {
		t.Fatal(err)
	}
	upper, lower := openSpecObligation("Session Expiry"), openSpecObligation("session expiry")
	if upper == lower || !reflect.DeepEqual(first.Requirements, []string{upper, lower}) {
		t.Fatalf("case-sensitive names share an ID: %v", first.Requirements)
	}
	if got, want := first.NativeNames, map[string]string{upper: "Session Expiry", lower: "session expiry"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NativeNames=%v want=%v", got, want)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(options.MapPath)))
	if err != nil || !strings.Contains(string(raw), `"form":"openspec-spec"`) {
		t.Fatalf("form not recorded: err=%v bytes=%s", err, raw)
	}
	resumed, err := PrepareOCM(context.Background(), root, options)
	if err != nil || !resumed.Resumed || !reflect.DeepEqual(resumed.NativeNames, first.NativeNames) {
		t.Fatalf("resume: %+v err=%v", resumed, err)
	}
	if _, err := ReadOCM(context.Background(), root, OCMReadOptions{
		OCMPath: options.MapPath, CEMPath: options.CEMPath, ExpectedBase: base, Target: target,
		ExpectedBaseGiven: true, TargetGiven: true,
	}); err != nil {
		t.Fatal(err)
	}
	speckit := options
	speckit.MapPath, speckit.IntentPath, speckit.IntentForm = ".corvint/speckit.ocm.json", "specs/001-x/spec.md", IntentFormSpecKit
	prepared, err := PrepareOCM(context.Background(), root, speckit)
	if err != nil || prepared.NativeStatus == nil || *prepared.NativeStatus != "Draft" || prepared.NativeNames != nil ||
		!reflect.DeepEqual(prepared.ExcludedRequirements, []map[string]string{{"id": "FR-002", "reason": "needs-clarification"}}) {
		t.Fatalf("speckit prepare: %+v err=%v", prepared, err)
	}
}

func TestSTAV0010_FixturesDerive(t *testing.T) {
	fixtures := filepath.Join("..", "..", "interop", "spec-toolchains", "fixtures")
	read := func(path string) []byte {
		data, err := os.ReadFile(filepath.Join(fixtures, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	required := []string{
		"openspec/openspec/config.yaml",
		"openspec/openspec/changes/add-session-timeout/tasks.md",
		"openspec/openspec/changes/add-session-timeout/proposal.md",
		"openspec/openspec/changes/archive/2026-10-01-add-invoice-export/tasks.md",
		"speckit/.specify/memory/constitution.md",
		"speckit/specs/001-session-timeout/plan.md",
	}
	for _, path := range required {
		read(path)
	}
	cases := []struct {
		form, path string
		names      []string
	}{
		{IntentFormOpenSpecSpec, "openspec/openspec/specs/auth/spec.md", []string{"User Login", "Session Expiry", "Remember Me", "Password Reset"}},
		{IntentFormOpenSpecSpec, "openspec/openspec/specs/billing/invoices/spec.md", []string{"Invoice Issue", "Invoice Export"}},
		{IntentFormOpenSpecDelta, "openspec/openspec/changes/add-session-timeout/specs/auth/spec.md", []string{"Idle Timeout Warning", "Session Expiry"}},
		{IntentFormOpenSpecDelta, "openspec/openspec/changes/archive/2026-10-01-add-invoice-export/specs/billing/invoices/spec.md", []string{"Invoice Export"}},
	}
	for _, test := range cases {
		derived, err := deriveIntent(test.form, test.path, "0123456789abcdef", read(test.path))
		if err != nil {
			t.Fatalf("%s: %v", test.path, err)
		}
		want := make([]string, 0, len(test.names))
		for _, name := range test.names {
			want = append(want, openSpecObligation(name))
		}
		if !reflect.DeepEqual(derived.requirements, want) {
			t.Fatalf("%s: requirements=%v want=%v", test.path, derived.requirements, want)
		}
	}
	delta, _ := deriveIntent(IntentFormOpenSpecDelta, "delta", "0123456789abcdef", read(cases[2].path))
	operations := []map[string]string{
		{"operation": "REMOVED", "name": "Remember Me"},
		{"operation": "RENAMED", "from": "User Login", "to": "Credential Sign-In"},
	}
	if !reflect.DeepEqual(delta.excludedRequirements, operations) {
		t.Fatalf("delta excludedRequirements=%v", delta.excludedRequirements)
	}
	speckit, err := deriveIntent(IntentFormSpecKit, "specs/001-session-timeout/spec.md", "0123456789abcdef", read("speckit/specs/001-session-timeout/spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := speckit.requirements, []string{"FR-001", "FR-002", "FR-003", "SC-001", "SC-002"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("speckit requirements=%v want=%v", got, want)
	}
	if !reflect.DeepEqual(speckit.excludedRequirements, []map[string]string{{"id": "FR-004", "reason": "needs-clarification"}}) ||
		speckit.nativeStatus == nil || *speckit.nativeStatus != "Draft" {
		t.Fatalf("speckit excluded=%v status=%v", speckit.excludedRequirements, speckit.nativeStatus)
	}
}
