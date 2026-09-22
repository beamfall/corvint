//go:build corvint_development

package frontier

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// Independently frozen proposed-policy table; no accepted authority or adapter.
type finitePolicyOracle struct {
	Priorities []struct{ Reason, Resolution, Action string }
	Sources    []struct {
		Source   string
		Priority int
	}
}

func finitePolicyRows(oracle finitePolicyOracle) error {
	if len(oracle.Priorities) != 13 || len(oracle.Sources) != 18 {
		return fmt.Errorf("incomplete domain")
	}
	seenReasons, seenSources := map[string]bool{}, map[string]bool{}
	for _, row := range oracle.Priorities {
		if row.Reason == "" || row.Resolution == "" || row.Action == "" || seenReasons[row.Reason] {
			return fmt.Errorf("empty or duplicate priority")
		}
		seenReasons[row.Reason] = true
	}
	for _, row := range oracle.Sources {
		if row.Source == "" || seenSources[row.Source] || row.Priority < 0 || row.Priority >= 13 {
			return fmt.Errorf("invalid source domain")
		}
		seenSources[row.Source] = true
	}
	return nil
}

func finitePolicyActual(sources []string) (Item, *Error) {
	claim := TCQClaimResult{ObligationID: "CF-V0-001", ClaimID: claimRef("one"), AuthorityClass: AuthorityCallerReported}
	for _, source := range sources {
		if source == "@relation" {
			claim.Relation = "test-report-matched-v0"
		} else {
			claim.Reasons = append(claim.Reasons, source)
		}
	}
	obligation := Obligation{ID: "CF-V0-001", Disposition: OCMLinked, ClaimIDs: []string{claimRef("two"), claimRef("one"), claimRef("two")}}
	return intentTestItem("frontier-universe:sha256:"+repeat("01", 32), obligation, []TCQClaimResult{claim})
}

func finitePolicyCheck(oracle finitePolicyOracle) error {
	if err := finitePolicyRows(oracle); err != nil {
		return err
	}
	for _, row := range oracle.Sources {
		actual, err := finitePolicyActual([]string{row.Source})
		want := oracle.Priorities[row.Priority]
		if err != nil || !reflect.DeepEqual(actual.Reasons, []string{want.Reason}) || actual.ResolutionClass != want.Resolution || actual.NextAction != want.Action {
			return fmt.Errorf("counterexample %s: %+v / %v", row.Source, actual, err)
		}
	}
	return nil
}

func TestFinitePolicyDevelopmentScreen(t *testing.T) {
	t.Run("EAF-V0-008", func(t *testing.T) {
		data, err := os.ReadFile("../../benchmarks/expert-audit-followup/finite-policy.json")
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != "d1742435bba786e23b84afff95dfb97e3e181f524932855f2237d405df30c2e5" {
			t.Fatal("independently frozen oracle changed; register a new experiment")
		}
		var oracle finitePolicyOracle
		if err := json.Unmarshal(data, &oracle); err != nil {
			t.Fatal(err)
		}
		if err := finitePolicyCheck(oracle); err != nil {
			t.Fatal(err)
		}
		// Every distinct nonempty reason set in this finite profile is checked.
		canonical := make([]string, 13)
		for _, row := range oracle.Sources {
			if canonical[row.Priority] == "" {
				canonical[row.Priority] = row.Source
			}
		}
		for mask := 1; mask < 1<<13; mask++ {
			sources, want := []string{}, []string{}
			first := -1
			for i, row := range oracle.Priorities {
				if mask&(1<<i) != 0 {
					if first < 0 {
						first = i
					}
					sources = append(sources, canonical[i])
					want = append(want, row.Reason)
				}
			}
			actual, err := finitePolicyActual(sources)
			if err != nil {
				t.Fatal(err)
			}
			head := oracle.Priorities[first]
			if !reflect.DeepEqual(actual.Reasons, want) || actual.ResolutionClass != head.Resolution || actual.NextAction != head.Action || actual.AuthorityClass != "CALLER_REPORTED" || actual.Kind != "INTENT_TEST" || actual.SubjectID != "CF-V0-001" {
				t.Fatalf("mask %d: %+v", mask, actual)
			}
			if !reflect.DeepEqual(actual.RelatedIDs, sortedUnique([]string{claimRef("one"), claimRef("two")})) {
				t.Fatalf("mask %d dropped selected claims", mask)
			}
		}
		// A duplicate alias/reversed-input control exercises union canonicalization.
		a, failure := finitePolicyActual([]string{"test-error", "repeated-test-rows", "execution-identity-ambiguous"})
		if failure != nil {
			t.Fatal(failure)
		}
		b, failure := finitePolicyActual([]string{"execution-identity-ambiguous", "repeated-test-rows", "test-error", "test-error"})
		if failure != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("aliases/order changed the item")
		}
		// Corrupt the frozen table; ordinary tests must expose every control.
		controls := 0
		for n := 0; n < 18; n++ {
			for _, kind := range []string{"missing", "duplicate", "wrong"} {
				var changed finitePolicyOracle
				if err := json.Unmarshal(data, &changed); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "missing":
					changed.Sources = append(changed.Sources[:n], changed.Sources[n+1:]...)
				case "duplicate":
					changed.Sources[n] = changed.Sources[(n+1)%18]
				case "wrong":
					changed.Sources[n].Priority = (changed.Sources[n].Priority + 1) % 13
				}
				if finitePolicyCheck(changed) == nil {
					t.Fatalf("guard accepted %s row %d", kind, n)
				}
				controls++
			}
		}
		if finitePolicyCheck(finitePolicyOracle{}) == nil {
			t.Fatal("empty domain accepted")
		}
		oracle.Priorities[0].Action = ""
		if finitePolicyCheck(oracle) == nil {
			t.Fatal("empty expected cell accepted")
		}
		if _, err := finitePolicyActual([]string{"unsupported-invented-predicate"}); err == nil {
			t.Fatal("unsupported predicate admitted")
		}
		t.Logf("oracle_sha256=%s atomic=18 combinations=8191 alias_controls=1 malformed_table_controls=%d unsupported_controls=1 result=PASS_BOUNDED_PROPOSED_POLICY adapter=NOT_NEEDED_BY_THIS_SCREEN", hex.EncodeToString(digest[:]), controls+2)
	})
}
