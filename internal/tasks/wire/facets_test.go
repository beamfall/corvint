package wire

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// CAL-V0-206 (amendment A26): the facets member is absent-only and optional.
// A result without it keeps its earlier bytes, a result with it round-trips,
// and a non-object facets member is refused.
func TestCALV0206_FacetsMemberIsAbsentOnlyAndOptional(t *testing.T) {
	plain := &Result{Command: []string{"roadmap"}, Outcome: OutcomeOK}
	raw, err := plain.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(`"facets"`)) {
		t.Fatalf("facets rendered without a summary: %s", raw)
	}
	if got, err := DecodeResult(raw); err != nil || got.Facets != nil {
		t.Fatalf("earlier bytes: %v %+v", err, got)
	}
	summary := validFacets()
	with := &Result{Command: []string{"roadmap"}, Outcome: OutcomeOK, Facets: &summary}
	raw2, err := with.Encode()
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeResult(raw2)
	if err != nil || got.Facets == nil || !bytes.Equal(Encode(*got.Facets), Encode(summary)) {
		t.Fatalf("round trip: %v %+v\n%s", err, got, raw2)
	}
	bad := strings.Replace(string(raw2), string(Encode(summary)), `"3"`, 1)
	if _, err := DecodeResult([]byte(bad)); err == nil {
		t.Fatalf("non-object facets decoded: %s", bad)
	}
}

// validFacets is a closed CAL-V0-206 summary over three tickets.
func validFacets() Value {
	counts := func(enum []string, set map[string]string) Value {
		o := NewObject()
		for _, k := range enum {
			n := "0"
			if v, ok := set[k]; ok {
				n = v
			}
			o.Set(k, String(n))
		}
		return ObjectValue(o)
	}
	o := NewObject()
	o.Set("total", String("3"))
	o.Set("byStatus", counts(FacetStatuses, map[string]string{"OPEN": "2", "COMPLETED": "1"}))
	o.Set("byPriority", counts(FacetPriorities, map[string]string{"P1": "3"}))
	o.Set("byKind", counts(FacetKinds, map[string]string{"BUG": "1", "FEATURE": "2"}))
	o.Set("byExecutionClass", counts(FacetExecutionClasses, map[string]string{"AUTONOMOUS": "3"}))
	o.Set("byEligibility", counts(FacetEligibilities, map[string]string{"BLOCKED": "1", "UNKNOWN": "2"}))
	o.Set("byNextAction", ObjectValue(NewObject().Set("admit", String("2")).Set("reopen", String("1"))))
	o.Set("byBlockerCode", ObjectValue(NewObject().Set(CodeDependencyMissing, String("1"))))
	o.Set("byMilestone", ObjectValue(NewObject().Set("v1-0", String("2"))))
	o.Set("milestonesOmitted", String("0"))
	o.Set("withoutMilestone", String("1"))
	o.Set("byLabel", ObjectValue(NewObject().Set("bugs", String("1"))))
	o.Set("labelsOmitted", String("0"))
	return ObjectValue(o)
}

// V1-1052: the facets member is the closed CAL-V0-206 summary, not any
// object. Every payload below passed Encode and DecodeResult before the fix.
func TestCALV0206_FacetsMemberIsTheClosedSummary(t *testing.T) {
	encode := func(f Value) ([]byte, error) {
		return (&Result{Command: []string{"roadmap"}, Outcome: OutcomeOK, Facets: &f}).Encode()
	}
	if _, err := encode(validFacets()); err != nil {
		t.Fatalf("valid summary refused: %v", err)
	}
	edit := func(fn func(o *Object)) Value {
		v := validFacets()
		fn(v.Obj)
		return v
	}
	labels := func(n int) Value {
		o := NewObject()
		for i := 0; i < n; i++ {
			o.Set(fmt.Sprintf("l%03d", i), String("1"))
		}
		return ObjectValue(o)
	}
	cases := map[string]Value{
		"empty object":        ObjectValue(NewObject()),
		"reviewer bool total": ObjectValue(NewObject().Set("total", Bool(true)).Set("unexpected", String("accepted"))),
		"reviewer negative":   ObjectValue(NewObject().Set("total", String("-1"))),
		"unknown member":      edit(func(o *Object) { o.Set("unexpected", String("1")) }),
		"negative total":      edit(func(o *Object) { o.Set("total", String("-1")) }),
		"null total":          edit(func(o *Object) { o.Set("total", Null()) }),
		"status missing enum": edit(func(o *Object) { o.Set("byStatus", ObjectValue(NewObject().Set("OPEN", String("3")))) }),
		"status unknown key":  edit(func(o *Object) { o.Vals["byStatus"].Obj.Set("WONTFIX", String("0")) }),
		"status sum mismatch": edit(func(o *Object) { o.Vals["byStatus"].Obj.Set("OPEN", String("1")) }),
		"next action unknown": edit(func(o *Object) { o.Vals["byNextAction"].Obj.Set("dance", String("0")) }),
		"next action zero": edit(func(o *Object) {
			o.Set("byNextAction", ObjectValue(NewObject().Set("admit", String("3")).Set("reopen", String("0"))))
		}),
		"next action sum":      edit(func(o *Object) { o.Set("byNextAction", ObjectValue(NewObject().Set("admit", String("1")))) }),
		"blocker unknown code": edit(func(o *Object) { o.Set("byBlockerCode", ObjectValue(NewObject().Set("NOT_A_CODE", String("1")))) }),
		"blocker above total": edit(func(o *Object) {
			o.Set("byBlockerCode", ObjectValue(NewObject().Set(CodeDependencyMissing, String("4"))))
		}),
		"milestone not label":  edit(func(o *Object) { o.Set("byMilestone", ObjectValue(NewObject().Set("", String("2")))) }),
		"milestone over total": edit(func(o *Object) { o.Set("withoutMilestone", String("2")) }),
		"labels over cap":      edit(func(o *Object) { o.Set("total", String("3")); o.Set("byLabel", labels(65)) }),
		"omitted under cap":    edit(func(o *Object) { o.Set("labelsOmitted", String("1")) }),
		"omitted not count":    edit(func(o *Object) { o.Set("milestonesOmitted", Null()) }),
		"label map not object": edit(func(o *Object) { o.Set("byLabel", String("bugs")) }),
	}
	for name, f := range cases {
		if raw, err := encode(f); err == nil {
			t.Errorf("%s: encoded %s", name, raw)
		}
		ok, err := encode(validFacets())
		if err != nil {
			t.Fatal(err)
		}
		bad := strings.Replace(string(ok), string(Encode(validFacets())), string(Encode(f)), 1)
		if _, err := DecodeResult([]byte(bad)); err == nil || CodeOf(err) != CodeMalformed {
			t.Errorf("%s: decoded (%v)", name, err)
		}
	}
	// The cap itself is admitted: 64 labels with one omitted.
	if _, err := encode(edit(func(o *Object) { o.Set("byLabel", labels(64)); o.Set("labelsOmitted", String("1")) })); err != nil {
		t.Fatalf("64 labels with one omitted refused: %v", err)
	}
}
