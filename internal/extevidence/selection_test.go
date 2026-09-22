package extevidence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

type selectionCase struct {
	Name         string            `json:"name"`
	Record       string            `json:"record"`
	Values       map[string]string `json:"values"`
	Profile      string            `json:"profile"`
	Changed      []string          `json:"changed"`
	Worktree     []string          `json:"worktree"`
	Incomplete   []string          `json:"incomplete"`
	Bind         []string          `json:"bind"`
	State        string            `json:"state"`
	Codes        []string          `json:"codes"`
	Selected     []string          `json:"selected"`
	Relevant     []string          `json:"relevant"`
	SafeToNarrow bool              `json:"safe_to_narrow"`
	Checkout     string            `json:"checkout"`
}

const (
	selectionFixtures = "conformance-selection"
	pathFixtures      = "conformance-path"
)

func selectionCases(t *testing.T) []selectionCase {
	t.Helper()
	return fixtureCases(t, selectionFixtures)
}

// fixtureCases reads one manifest; its defaults fill any placeholder a case
// leaves unset.
func fixtureCases(t *testing.T, dir string) []selectionCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Defaults map[string]string `json:"defaults"`
		Cases    []selectionCase   `json:"cases"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for i := range manifest.Cases {
		values := map[string]string{}
		for key, value := range manifest.Defaults {
			values[key] = value
		}
		for key, value := range manifest.Cases[i].Values {
			values[key] = value
		}
		manifest.Cases[i].Values = values
	}
	return manifest.Cases
}

// selectionRecord reads one fixture: "v0" is the V0 mock record, "" names a
// source that does not exist, anything else is a V1 selection fixture.
func selectionRecord(t *testing.T, p pair, name string) string {
	t.Helper()
	return fixtureRecord(t, p, selectionFixtures, selectionCase{Record: name})
}

// fixtureRecord writes a case's record with its own values substituted before
// the pair's, so a value may itself name a pair placeholder.
func fixtureRecord(t *testing.T, p pair, fixtures string, c selectionCase) string {
	t.Helper()
	name := c.Record
	dir := t.TempDir()
	switch name {
	case "":
		return filepath.Join(dir, "absent.json")
	case "v0":
		return writeRecord(t, dir, "provider.json", fixture(t, p.app.head))
	}
	data, err := os.ReadFile(filepath.Join("testdata", fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range c.Values {
		data = bytes.ReplaceAll(data, []byte("{{"+key+"}}"), []byte(value))
	}
	values := with(with(with(p.values(), "APP_FIRST", p.app.first), "E2E_FIRST", p.e2e.first), "E2E_ORPHAN", p.e2e.orphan)
	for key, value := range values {
		data = bytes.ReplaceAll(data, []byte("{{"+key+"}}"), []byte(value))
	}
	return writeRecord(t, dir, "provider.json", data)
}

func selectionInput(c selectionCase) SelectionInput {
	return SelectionInput{Changed: c.Changed, Worktree: c.Worktree, Incomplete: c.Incomplete, Profile: c.Profile, Limit: 20, CheckoutStatus: checkoutStatus[c.Checkout]}
}

// checkoutStatus stubs a bound checkout's worktree by case label; an unset
// label leaves every checkout uninspected, as V0 did.
var checkoutStatus = map[string]func(context.Context, string) ([]string, error){
	"clean":      func(context.Context, string) ([]string, error) { return nil, nil },
	"dirty":      func(context.Context, string) ([]string, error) { return []string{"tests/account.spec.ts"}, nil },
	"unreadable": func(context.Context, string) ([]string, error) { return nil, errors.New("status unavailable") },
}

func runSelection(t *testing.T, p pair, source string, checkouts []Checkout, input SelectionInput) (map[string]any, []byte) {
	t.Helper()
	out, err := contextindex.CanonicalJSON(Selection(context.Background(), p.app.root, p.app.head, []string{source}, checkouts, input))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded, out
}

// bindCase binds each id to the e2e checkout, or ID=app to the application
// root, a checkout of different history.
func bindCase(p pair, c selectionCase) []Checkout {
	var checkouts []Checkout
	for _, id := range c.Bind {
		source := p.e2e.root
		if name, found := strings.CutSuffix(id, "=app"); found {
			id, source = name, p.app.root
		}
		checkouts = append(checkouts, Checkout{ID: id, Source: source})
	}
	return checkouts
}

// selectedTests lists every selected row as repository:path; a V0 row has no
// repository member.
func selectedTests(selection map[string]any) []string {
	var out []string
	for _, raw := range selection["selected"].([]any) {
		test := raw.(map[string]any)["test"].(map[string]any)
		repository, _ := test["repository"].(string)
		out = append(out, repository+":"+test["path"].(string))
	}
	sort.Strings(out)
	return out
}

// selectionCodes is every reason code the advice reports anywhere.
func selectionCodes(selection map[string]any) map[string]bool {
	codes := map[string]bool{}
	for _, list := range []string{"blocking_reasons", "candidates", "unknowns"} {
		for _, raw := range selection[list].([]any) {
			if code, ok := raw.(map[string]any)["code"].(string); ok {
				codes[code] = true
			}
		}
	}
	for _, list := range []string{"uncovered_paths", "uncovered_entities"} {
		for _, raw := range selection[list].([]any) {
			for _, code := range raw.(map[string]any)["reasons"].([]any) {
				codes[code.(string)] = true
			}
		}
	}
	return codes
}

// allCases pairs every labelled case with its fixture directory: the entity
// fixtures (ETS-V0) and the independent path-to-path fixture (EEP-V2).
func allCases(t *testing.T) (cases []selectionCase, dirs []string) {
	t.Helper()
	for _, dir := range []string{selectionFixtures, pathFixtures} {
		for _, c := range fixtureCases(t, dir) {
			cases, dirs = append(cases, c), append(dirs, dir)
		}
	}
	return cases, dirs
}

// TestSelectionConformance runs every labelled fixture (ETS-V0-003..006,
// ETS-V0-008, ETS-V0-011, EEP-V2-006..010).
func TestSelectionConformance(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	cases, dirs := allCases(t)
	for i, c := range cases {
		selection, _ := runSelection(t, p, fixtureRecord(t, p, dirs[i], c), bindCase(p, c), selectionInput(c))
		if selection["state"] != c.State {
			t.Errorf("%s: state %v, want %s (reason %v, blocking %v, uncovered %v %v)", c.Name, selection["state"], c.State,
				selection["state_reason"], selection["blocking_reasons"], selection["uncovered_paths"], selection["uncovered_entities"])
		}
		codes := selectionCodes(selection)
		for _, code := range c.Codes {
			if !codes[code] {
				t.Errorf("%s: reason %s missing from %v", c.Name, code, codes)
			}
		}
		if got := selectedTests(selection); strings.Join(got, ",") != strings.Join(c.Selected, ",") {
			t.Errorf("%s: selected %v, want %v", c.Name, got, c.Selected)
		}
		for _, raw := range selection["candidates"].([]any) {
			if row := raw.(map[string]any); row["code"] == "" || row["reason"] == "" {
				t.Errorf("%s: an excluded relation must carry a code and a reason: %v", c.Name, row)
			}
		}
	}
}

// TestSelectionEvaluation reports the ETS-V0-012 measures over the labelled
// fixtures and fails on any unsafe narrowing.
func TestSelectionEvaluation(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	var selected, relevant, unsafe, unsafeDenominator, abstain, abstainCorrect, largest int
	var latencies []time.Duration
	cases, dirs := allCases(t)
	for i, c := range cases {
		source, checkouts := fixtureRecord(t, p, dirs[i], c), bindCase(p, c)
		started := time.Now()
		selection, out := runSelection(t, p, source, checkouts, selectionInput(c))
		latencies = append(latencies, time.Since(started))
		largest = max(largest, len(out))
		labelled := map[string]bool{}
		for _, test := range c.Relevant {
			labelled[test] = true
		}
		for _, test := range selectedTests(selection) {
			selected++
			if labelled[test] {
				relevant++
			}
		}
		if !c.SafeToNarrow {
			unsafeDenominator++
			if selection["state"] == SelectionNarrow {
				unsafe++
			}
		}
		if c.State == SelectionUnknown || c.State == SelectionBlocked {
			abstain++
			if selection["state"] == c.State {
				abstainCorrect++
			}
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	precision := float64(relevant) / float64(max(selected, 1))
	t.Logf("cases=%d precision=%.3f (%d/%d) unsafe-narrowing=%d/%d abstention-accuracy=%d/%d latency-p50=%s latency-max=%s receipt-max-bytes=%d",
		len(latencies), precision, relevant, selected, unsafe, unsafeDenominator, abstainCorrect, abstain,
		latencies[len(latencies)/2], latencies[len(latencies)-1], largest)
	if unsafe != 0 || relevant != selected || abstainCorrect != abstain {
		t.Fatalf("unsafe narrowing %d, precision %d/%d, abstention %d/%d", unsafe, relevant, selected, abstainCorrect, abstain)
	}
}

// TestSelectionRowProvenance checks the ETS-V0-007 members of a selected row.
func TestSelectionRowProvenance(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	c := selectionCases(t)[0]
	selection, _ := runSelection(t, p, selectionRecord(t, p, c.Record), bindCase(p, c), selectionInput(c))
	for _, raw := range selection["selected"].([]any) {
		row := raw.(map[string]any)
		for _, member := range []string{"authority", "confidence", "provider", "provider_revision", "entity", "entity_kind", "test", "relation", "relation_type", "evidence", "verification", "limitations", "identity", "binding", "freshness", "test_revision", "source_revision", "relation_state", "crosses_repositories", "reason"} {
			if _, present := row[member]; !present {
				t.Errorf("selected row lacks %s: %v", member, row)
			}
		}
		if row["relation_state"] != RelationFresh || row["confidence"] != confidenceUnscored {
			t.Errorf("a selected row must be fresh and unscored: %v", row)
		}
	}
	crossing := selection["selected"].([]any)[1].(map[string]any)
	if crossing["crosses_repositories"] != true || crossing["binding"] != BindingCheckout || crossing["test_revision"] != p.e2e.head || crossing["source_revision"] != p.app.head {
		t.Fatalf("a cross-repository row must carry each side's own revision and binding: %v", crossing)
	}
}

// TestSelectionMandatoryEchoedUnchanged: evidence can widen the obligations
// but the mandatory set is echoed verbatim whatever the state (ETS-V0-009).
func TestSelectionMandatoryEchoedUnchanged(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	mandatory := []any{map[string]any{"command": "make gate", "kind": "mandatory", "reason": "Makefile gate target", "source": "Makefile"}}
	want, _ := json.Marshal(mandatory)
	cases, dirs := allCases(t)
	for i, c := range cases {
		input := selectionInput(c)
		input.Mandatory = mandatory
		selection, _ := runSelection(t, p, fixtureRecord(t, p, dirs[i], c), bindCase(p, c), input)
		if got, _ := json.Marshal(selection["mandatory"]); !bytes.Equal(got, want) {
			t.Fatalf("%s: mandatory %s, want %s", c.Name, got, want)
		}
	}
}

// TestSelectionDeterministic: repeated runs and a reordered record give the
// same bytes (ETS-V0-010).
func TestSelectionDeterministic(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	c := selectionCases(t)[0]
	source := selectionRecord(t, p, c.Record)
	_, first := runSelection(t, p, source, bindCase(p, c), selectionInput(c))
	_, second := runSelection(t, p, source, bindCase(p, c), selectionInput(c))
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	reversed := writeRecord(t, t.TempDir(), "provider.json", mutate(t, data, func(record map[string]any) {
		list := relations(record)
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}))
	third, _ := runSelection(t, p, reversed, bindCase(p, c), selectionInput(c))
	delete(third, "provider_evidence")
	thirdBytes, _ := contextindex.CanonicalJSON(third)
	firstMap := map[string]any{}
	_ = json.Unmarshal(first, &firstMap)
	delete(firstMap, "provider_evidence")
	firstBytes, _ := contextindex.CanonicalJSON(firstMap)
	if !bytes.Equal(first, second) || !bytes.Equal(firstBytes, thirdBytes) {
		t.Fatalf("selection must not depend on run or record order:\n%s\n%s", firstBytes, thirdBytes)
	}
}

// TestSelectionOmissionAccounting: every list is bounded and the remainder
// counted (ETS-V0-010).
func TestSelectionOmissionAccounting(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	c := selectionCases(t)[0]
	input := selectionInput(c)
	input.Limit = 1
	selection, _ := runSelection(t, p, selectionRecord(t, p, c.Record), bindCase(p, c), input)
	omitted := selection["omitted"].(map[string]any)
	if len(selection["selected"].([]any)) != 1 || omitted["selected"] != float64(1) {
		t.Fatalf("two selected rows at limit 1 must keep one and count one: %v", omitted)
	}
	if selection["state"] != SelectionNarrow {
		t.Fatalf("omission must not change the state: %v", selection["state"])
	}
}

// TestSelectionPrivate: a checkout is echoed as given, never resolved, no
// file body reaches the advice, and provider free text is named untrusted
// (ETS-V0-013).
func TestSelectionPrivate(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	c := selectionCases(t)[0]
	checkouts := []Checkout{{ID: "e2e", Source: relativeTo(t, p.app.root, p.e2e.root)}}
	selection, out := runSelection(t, p, selectionRecord(t, p, c.Record), checkouts, selectionInput(c))
	canonical, err := filepath.EvalSymlinks(p.e2e.root)
	if err != nil {
		t.Fatal(err)
	}
	if selection["state"] != SelectionNarrow {
		t.Fatalf("a relative checkout must bind as an absolute one does: %v", selection["state_reason"])
	}
	for _, secret := range []string{p.app.root, canonical, "test('account')", "package main"} {
		if bytes.Contains(out, []byte(secret)) {
			t.Fatalf("advice leaks %q", secret)
		}
	}
	if len(selection["untrusted_text_fields"].([]any)) == 0 {
		t.Fatal("rule and reference must be named untrusted text")
	}
}

// TestSelectionWalkBudget: a walk that would pass the entity bound stops,
// blocks the entity at its edge, and reports the cut (ETS-V1-002, ETS-V1-003).
func TestSelectionWalkBudget(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	c := selectionCase{Record: "multihop-covered.json", Profile: ProfileStrict, Changed: []string{"pkg/main.go"}}
	data, err := os.ReadFile(fixtureRecord(t, p, selectionFixtures, c))
	if err != nil {
		t.Fatal(err)
	}
	source := writeRecord(t, t.TempDir(), "provider.json", mutate(t, data, func(record map[string]any) {
		for i := range MaxObligationEntities {
			id := fmt.Sprintf("leaf-%03d", i)
			record["entities"] = append(record["entities"].([]any), map[string]any{"id": id, "kind": "capability", "summary": "Leaf."})
			record["relations"] = append(relations(record), map[string]any{
				"from": map[string]any{"provider": "mockdocs", "entity": "cap-a"}, "to": map[string]any{"provider": "mockdocs", "entity": id},
				"type": "depends-on", "evidence": "declared", "rule": "dependency-map", "reference": "ci/dependency-map",
			})
		}
	}))
	selection, _ := runSelection(t, p, source, nil, selectionInput(c))
	if selection["state"] != SelectionFull || !selectionCodes(selection)["obligation-budget-exhausted"] {
		t.Fatalf("a walk past the entity bound must widen and say so: %v %v", selection["state"], selection["unknowns"])
	}
}

// TestSelectionExtensionsOnlyWiden: reading a checkout can only keep or widen
// a case's state, never narrow it (ETS-V1-008).
func TestSelectionExtensionsOnlyWiden(t *testing.T) {
	t.Parallel()
	p := newPair(t)
	cases, dirs := allCases(t)
	for i, c := range cases {
		source, checkouts := fixtureRecord(t, p, dirs[i], c), bindCase(p, c)
		c.Checkout = ""
		uninspected, _ := runSelection(t, p, source, checkouts, selectionInput(c))
		for _, label := range []string{"clean", "dirty", "unreadable"} {
			c.Checkout = label
			inspected, _ := runSelection(t, p, source, checkouts, selectionInput(c))
			if inspected["state"] == SelectionNarrow && uninspected["state"] != SelectionNarrow {
				t.Errorf("%s: a %s checkout narrowed %v", c.Name, label, uninspected["state"])
			}
			if label != "clean" && len(checkouts) != 0 && len(inspected["selected"].([]any)) > len(uninspected["selected"].([]any)) {
				t.Errorf("%s: a %s checkout selected more tests", c.Name, label)
			}
		}
	}
}
