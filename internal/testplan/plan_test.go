package testplan

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

var update = flag.Bool("update", false, "rewrite the golden plan files")

// workedExample is the spec's worked example: V1 and V6 duplicate, V2 books slot 42 which V4
// needs free, V3 has a passing witness and V5 is destructive.
func workedExample() ([]map[string]any, Evidence) {
	witness := []any{map[string]any{"test_id": "teesheet-book", "project": "chromium", "basis": "reviewed"}}
	variations := []map[string]any{
		variation("V1"),
		variation("V2", set("changes", []any{"slot.42=booked"})),
		variation("V3", set("witnesses", witness)),
		variation("V4", set("requires", []any{"slot.42=free"})),
		variation("V5", set("destructive", true)),
		variation("V6", set("action", []any{"element:V1.act"}), set("assertion", []any{"element:V1.see"})),
	}
	evidence := Evidence{
		Revision:    fixedRevision,
		TestsDigest: digestList([]string{strings.Repeat("ab", 32)}),
		Documents:   []testvaliditydoc.Document{e2eDocument("teesheet-book", "chromium", passing())},
	}
	return variations, evidence
}

// TestGoldenWorkedExample pins the table and JSON bytes of the worked example (TCN-V0-009,
// TCN-V0-010).
func TestGoldenWorkedExample(t *testing.T) {
	variations, evidence := workedExample()
	p := build(t, evidence, DefaultMaxSteps, variations...)
	want := []string{
		"| T001 | tests/e2e/teesheet.spec.ts | V1, V4, V2 | - |",
		"| T002 | tests/e2e/teesheet.spec.ts | V5 | destructive-change |",
		"| REUSE teesheet-book@chromium | tests/e2e/teesheet.spec.ts | V3 | reused strength=NOT_MEASURED |",
		"| DUPLICATE V1 | tests/e2e/teesheet.spec.ts | V6 | duplicate |",
	}
	if got := rows(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}
	if p.Authority != "candidate" || p.Status != statusComplete || p.AnchorValidation != anchorsNotRun {
		t.Fatalf("authority %s status %s anchors %s", p.Authority, p.Status, p.AnchorValidation)
	}
	if p.TableDigest != sha(p.Table()) {
		t.Fatal("table_digest does not name the table bytes")
	}
	golden(t, "worked-example.table", p.Table())
	golden(t, "worked-example.json", p.JSON())
	for i := 0; i < 3; i++ {
		again := build(t, evidence, DefaultMaxSteps, variations...)
		if !bytes.Equal(again.JSON(), p.JSON()) || !bytes.Equal(again.Table(), p.Table()) {
			t.Fatal("repeated runs differ")
		}
	}
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from the golden file:\n%s", name, got)
	}
}

// TestDuplicateClasses: classes keep the reused member as representative; a differing context,
// requires, changes or destructive never classes a duplicate (TCN-V0-005).
func TestDuplicateClasses(t *testing.T) {
	same := func(id string) []func(map[string]any) {
		return []func(map[string]any){set("action", []any{"element:" + id + ".act"}), set("assertion", []any{"element:" + id + ".see"})}
	}
	witness := []any{map[string]any{"test_id": "t1", "basis": "declared"}}
	p := build(t, Evidence{Revision: none, TestsDigest: none, Documents: []testvaliditydoc.Document{e2eDocument("t1", "", passing())}}, 8,
		variation("V1"),
		variation("V2", same("V1")...),
		variation("V3", same("V3")...),
		variation("V4", append(same("V3"), set("witnesses", witness))...),
		variation("V5", same("V3")...),
		variation("V6", append(same("V1"), set("user", "member"))...),
		variation("V7", append(same("V1"), set("requires", []any{"k=1"}))...),
		variation("V8", append(same("V1"), set("changes", []any{"k=1"}))...),
		variation("V9", append(same("V1"), set("destructive", true))...),
		variation("W1", append(same("V1"), set("action", []any{"element:V1.act", "element:V1.act"}))...),
	)
	want := []Duplicate{{"V2", "V1"}, {"V3", "V4"}, {"V5", "V4"}}
	if !reflect.DeepEqual(p.Duplicates, want) {
		t.Fatalf("duplicates %+v", p.Duplicates)
	}
	if len(p.Reused) != 1 || p.Reused[0].VariationID != "V4" {
		t.Fatalf("reused %+v", p.Reused)
	}
	placed := map[string]bool{}
	for _, test := range p.Tests {
		for _, s := range test.Steps {
			placed[s.VariationID] = true
		}
	}
	for _, id := range []string{"V1", "V6", "V7", "V8", "V9", "W1"} {
		if !placed[id] {
			t.Fatalf("%s was not planned: %s", id, p.Table())
		}
	}
}

// TestIsolationReasons covers every reason on one fixture, including a destructive variation whose
// context is shared only by other users (TCN-V0-006).
func TestIsolationReasons(t *testing.T) {
	p := build(t, Evidence{}, 8,
		variation("V1", set("requires", []any{"x=0"})),
		variation("V2", set("requires", []any{"x=1"})),
		variation("V3", set("user", "member")),
		variation("V4", set("user", "guest"), set("destructive", true)),
		variation("V5", set("screen", "screen:admin:other")),
		variation("V6", set("screen", "screen:admin:group")),
		variation("V7", set("screen", "screen:admin:group")),
		variation("V8", set("screen", "screen:admin:org"), set("org", "")),
		variation("V9", set("screen", "screen:admin:org"), set("org", "club-b"), set("requires", []any{"y=1"})),
	)
	want := []string{
		"| T001 | tests/e2e/teesheet.spec.ts | V1 | conflicting-state |",
		"| T002 | tests/e2e/teesheet.spec.ts | V2 | conflicting-state |",
		"| T003 | tests/e2e/teesheet.spec.ts | V3 | different-user-or-org |",
		"| T004 | tests/e2e/teesheet.spec.ts | V4 | destructive-change |",
		"| T005 | tests/e2e/teesheet.spec.ts | V5 | unique-context |",
		"| T006 | tests/e2e/teesheet.spec.ts | V6, V7 | - |",
		"| T007 | tests/e2e/teesheet.spec.ts | V8 | different-user-or-org |",
		"| T008 | tests/e2e/teesheet.spec.ts | V9 | different-user-or-org |",
	}
	if got := rows(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}
	if p.EvaluatedRevision != none || p.TestsDigest != none || p.MapsDigest != none {
		t.Fatalf("absent evidence must read none: %+v", p)
	}
}

// TestPrecedenceOrderAndSets: B precedes A when A changes a key B requires; a self-change adds no
// edge; requires x=0, x=1, x=1 gives two tests (TCN-V0-007).
func TestPrecedenceOrderAndSets(t *testing.T) {
	p := build(t, Evidence{}, 8,
		variation("V1", set("changes", []any{"k=2"})),
		variation("V2", set("requires", []any{"k=1"})),
		variation("V3", set("requires", []any{"j=1"}), set("changes", []any{"j=2"})),
		variation("V4"),
	)
	if got := rows(p); !reflect.DeepEqual(got, []string{"| T001 | tests/e2e/teesheet.spec.ts | V2, V1, V3, V4 | - |"}) {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}
	p = build(t, Evidence{}, 8,
		variation("V1", set("requires", []any{"x=0"})),
		variation("V2", set("requires", []any{"x=1"})),
		variation("V3", set("requires", []any{"x=1"})),
	)
	want := []string{
		"| T001 | tests/e2e/teesheet.spec.ts | V1 | conflicting-state |",
		"| T002 | tests/e2e/teesheet.spec.ts | V2, V3 | - |",
	}
	if got := rows(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}
}

// TestCycleReplanned: the greatest ID of a cycle is removed and planned again alone (TCN-V0-007).
func TestCycleReplanned(t *testing.T) {
	p := build(t, Evidence{}, 8,
		variation("V1", set("requires", []any{"c=1"}), set("changes", []any{"a=1"})),
		variation("V2", set("requires", []any{"a=1"}), set("changes", []any{"b=1"})),
		variation("V3", set("requires", []any{"b=1"}), set("changes", []any{"c=1"})),
	)
	want := []string{
		"| T001 | tests/e2e/teesheet.spec.ts | V2, V1 | - |",
		"| T002 | tests/e2e/teesheet.spec.ts | V3 | conflicting-state |",
	}
	if got := rows(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}
}

// TestEdgeBoundRefuses: more than 1,048,576 precedence edges refuse, never truncate (TCN-V0-007).
func TestEdgeBoundRefuses(t *testing.T) {
	vs := make([]map[string]any, 1100)
	for i := range vs {
		vs[i] = variation(fmt.Sprintf("V%04d", i), set("requires", []any{"k=1"}), set("changes", []any{"k=2"}))
	}
	_, err := Build(decode(t, inputJSON(t, vs...)), Evidence{}, 8)
	if code(err) != "test-plan-bound-exceeded" {
		t.Fatalf("got %v", err)
	}
}

// TestMaxStepsCutAndNumbering: cuts keep the order, numbering follows spec then smallest ID, and
// the range is enforced (TCN-V0-008).
func TestMaxStepsCutAndNumbering(t *testing.T) {
	vs := []map[string]any{
		variation("V1"), variation("V2"), variation("V3"), variation("V4"), variation("V5"),
		variation("A1", set("spec", "tests/e2e/zz.spec.ts")),
	}
	p := build(t, Evidence{}, 2, vs...)
	want := []string{
		"| T001 | tests/e2e/teesheet.spec.ts | V1, V2 | - |",
		"| T002 | tests/e2e/teesheet.spec.ts | V3, V4 | - |",
		"| T003 | tests/e2e/teesheet.spec.ts | V5 | unique-context |",
		"| T004 | tests/e2e/zz.spec.ts | A1 | unique-context |",
	}
	if got := rows(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows:\n%s", strings.Join(got, "\n"))
	}
	if !strings.Contains(string(p.Table()), " max-steps=2 ") {
		t.Fatal("header does not carry max-steps")
	}
	for _, n := range []int{-1, 0, 1, 33} {
		if _, err := Build(decode(t, inputJSON(t, vs...)), Evidence{}, n); code(err) != "test-plan-invalid-arguments" {
			t.Fatalf("max-steps %d: got %v", n, err)
		}
	}
}

// TestEveryVariationAccountedOnce generates mixed inputs and checks each ID appears exactly once
// and the counts agree (TCN-V0-009).
func TestEveryVariationAccountedOnce(t *testing.T) {
	random := rand.New(rand.NewSource(1023))
	documents := []testvaliditydoc.Document{e2eDocument("t0", "", passing()), e2eDocument("t1", "", testvaliditydocFailing())}
	for round := 0; round < 40; round++ {
		n := 1 + random.Intn(60)
		vs := make([]map[string]any, n)
		for i := range vs {
			id := "V" + strconv.Itoa(i)
			edits := []func(map[string]any){
				set("user", []string{"a", "b"}[random.Intn(2)]),
				set("screen", []string{"s1", "s2", "s3"}[random.Intn(3)]),
				set("action", []any{"element:" + strconv.Itoa(random.Intn(4))}),
				set("assertion", []any{"element:see"}),
				set("requires", pick(random, []string{"a=0", "a=1", "b=0"})),
				set("changes", pick(random, []string{"a=1", "b=1", "c=1"})),
				set("destructive", random.Intn(8) == 0),
			}
			switch random.Intn(10) {
			case 0:
				edits = append(edits, drop("org"))
			case 1:
				edits = append(edits, set("witnesses", []any{map[string]any{"test_id": "t" + strconv.Itoa(random.Intn(3)), "basis": "declared"}}))
			}
			vs[i] = variation(id, edits...)
		}
		p := build(t, Evidence{Revision: none, TestsDigest: none, Documents: documents}, 2+random.Intn(4), vs...)
		seen := map[string]int{}
		for _, test := range p.Tests {
			if len(test.Steps) > p.MaxSteps {
				t.Fatalf("test %s exceeds max-steps", test.Test)
			}
			for _, s := range test.Steps {
				seen[s.VariationID]++
			}
		}
		for _, r := range p.Reused {
			seen[r.VariationID]++
		}
		for _, d := range p.Duplicates {
			seen[d.VariationID]++
		}
		for _, a := range p.Abstained {
			seen[a.VariationID]++
		}
		if len(seen) != n {
			t.Fatalf("round %d: %d of %d variations accounted", round, len(seen), n)
		}
		for id, count := range seen {
			if count != 1 {
				t.Fatalf("round %d: %s accounted %d times", round, id, count)
			}
		}
		c := p.Counts
		if c.Variations != n || c.BaselineOnePerRow != n || c.NewTests != len(p.Tests) {
			t.Fatalf("counts %+v", c)
		}
		var decoded map[string]any
		if err := json.Unmarshal(p.JSON(), &decoded); err != nil {
			t.Fatal(err)
		}
	}
}

func pick(random *rand.Rand, facts []string) []any {
	out := []any{}
	keys := map[string]bool{}
	for _, f := range facts {
		key, _, _ := strings.Cut(f, "=")
		if random.Intn(3) == 0 && !keys[key] {
			keys[key] = true
			out = append(out, f)
		}
	}
	return out
}

func testvaliditydocFailing() testvalidity.Projection {
	p := passing()
	p.Execution.State = "FAILED"
	return p
}

// TestOutputBoundRefuses: a plan over 8 MiB refuses, never truncates (TCN-V0-009).
func TestOutputBoundRefuses(t *testing.T) {
	vs := make([]map[string]any, 1150)
	for i := range vs {
		spec := fmt.Sprintf("tests/%04d/%s.spec.ts", i, strings.Repeat("s", 3980))
		vs[i] = variation(fmt.Sprintf("V%04d", i), set("spec", spec))
	}
	data := inputJSON(t, vs...)
	if len(data) > MaxInputBytes {
		t.Fatalf("fixture input is %d bytes", len(data))
	}
	_, err := Build(decode(t, data), Evidence{}, 8)
	if code(err) != "test-plan-bound-exceeded" {
		t.Fatalf("got %v", err)
	}
}
