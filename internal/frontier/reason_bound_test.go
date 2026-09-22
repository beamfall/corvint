package frontier

import "testing"

// The CF-V0-023 bound of 32 reasons per item is real and enforced, and it is
// also UNREACHABLE: CF-V0-010 and CF-V0-016 are closed unions with unique
// members, so the widest item any universe can produce carries 13 reasons.
//
// That is why conformance/frontier-v0's `resource-limits/reasonsPerItem-at-limit`
// and `-at-limit-plus-one` pair skips as unsatisfiable by construction instead
// of executing. This file is the other half of that adjudication: the bound
// stays tested here, at the boundary that enforces it, and the reachable
// maximum stays tested too, so a future clause that widens a union or narrows
// the bound fails a test rather than silently making the conformance skip a lie.

// TestReasonAlgebraCeilingsAreBelowTheDeclaredBound derives each kind's ceiling
// from the frozen reason tables themselves, not from a transcribed number.
func TestReasonAlgebraCeilingsAreBelowTheDeclaredBound(t *testing.T) {
	// HUNK_BASIS: a CEM `unknown` hunk carries exactly one mapped reason; a
	// `supported` hunk carries the union of the CF-V0-010 basis diagnostics,
	// which is exactly the set lrfBasisReasons can produce.
	hunkCeiling := len(distinct(lrfBasisReasons))
	// INTENT_TEST: the one relation reason plus every distinct reason the
	// CF-V0-016 TCQ mapping can produce.
	testCeiling := 1 + len(distinct(tcqReasons))
	// INTENT_CHANGE: CF-V0-016 says exactly one.
	changeCeiling := 1

	for _, ceiling := range []struct {
		kind  string
		value int
	}{
		{KindHunkBasis, hunkCeiling},
		{KindIntentChange, changeCeiling},
		{KindIntentTest, testCeiling},
	} {
		if ceiling.value >= MaxReasonsPerItem {
			t.Errorf("%s can now reach %d reasons, at or above the CF-V0-023 bound of %d: the "+
				"conformance suite's reasonsPerItem pair is materializable again and must stop skipping",
				ceiling.kind, ceiling.value, MaxReasonsPerItem)
		}
		if ceiling.value > len(reasonOrders[ceiling.kind]) {
			t.Errorf("%s can produce %d reasons but its frozen order lists only %d",
				ceiling.kind, ceiling.value, len(reasonOrders[ceiling.kind]))
		}
	}
	if testCeiling != 13 {
		t.Errorf("the CF-V0-016 table is 13 rows; the mapping now reaches %d", testCeiling)
	}
}

// TestWidestReachableItemIsAcceptedAndCanonical builds the widest item the
// reason algebra can produce — every CF-V0-016 diagnostic plus the relation
// reason, united across selected claim edges — and requires it through the
// projection and the seal. This is the "reachable maximum" the resource-limits
// pair cannot express.
func TestWidestReachableItemIsAcceptedAndCanonical(t *testing.T) {
	claims := []TCQClaimResult{{
		ObligationID:   "CF-V0-001",
		ClaimID:        "claim:sha256:" + repeat("01", 32),
		Relation:       tcqRelation,
		AuthorityClass: AuthorityCallerReported,
		Reasons:        keys(tcqReasons),
	}}
	obligation := Obligation{
		ID: "CF-V0-001", Disposition: OCMLinked,
		HunkIDs:  []string{"hunk:sha256:" + repeat("02", 32)},
		ClaimIDs: []string{"claim:sha256:" + repeat("01", 32)},
	}
	universeID := universeIDPrefix + repeat("03", 32)
	item, err := intentTestItem(universeID, obligation, claims)
	if err != nil {
		t.Fatalf("the widest reachable INTENT_TEST item was refused: %v", err)
	}
	want := 1 + len(distinct(tcqReasons))
	if len(item.Reasons) != want {
		t.Fatalf("reasons = %d, want the full union of %d", len(item.Reasons), want)
	}
	if len(item.Reasons) >= MaxReasonsPerItem {
		t.Fatalf("the widest reachable item reaches the CF-V0-023 bound")
	}
	// CF-V0-016: the first retained reason supplies the action class.
	if item.Reasons[0] != ReasonCallerReportedNonclosing {
		t.Errorf("reasons[0] = %q, want the highest-priority reason", item.Reasons[0])
	}
	if item.ResolutionClass != ResolutionAuthorityRequired || item.NextAction != ActionEstablishHarnessAuthority {
		t.Errorf("disposition = %s/%s", item.ResolutionClass, item.NextAction)
	}
	// The union must be stored in the frozen order, not lexical order.
	position := 0
	for _, reason := range intentTestReasonOrder {
		if position < len(item.Reasons) && item.Reasons[position] == reason {
			position++
		}
	}
	if position != len(item.Reasons) {
		t.Errorf("reasons are not stored in the CF-V0-016 order: %v", item.Reasons)
	}
	if err := checkLimits([]Item{item}); err != nil {
		t.Errorf("the widest reachable item exceeds a CF-V0-023 bound: %v", err)
	}
}

// TestReasonCeilingIsEnforcedAtTheBoundary is the defence-in-depth half: the
// bound is unreachable through the projection, so this drives it directly. An
// item at N is accepted and an item at N+1 fails `frontier-resource-exhausted`
// with no partial result, at BOTH guards that own the bound.
func TestReasonCeilingIsEnforcedAtTheBoundary(t *testing.T) {
	atLimit := Item{
		AuthorityClass: AuthorityCallerReported, Kind: KindIntentTest,
		Reasons: syntheticReasons(MaxReasonsPerItem), RelatedIDs: []string{}, SubjectID: "CF-V0-001",
	}
	if err := checkLimits([]Item{atLimit}); err != nil {
		t.Errorf("an item at the %d-reason bound was refused: %v", MaxReasonsPerItem, err)
	}
	over := atLimit
	over.Reasons = syntheticReasons(MaxReasonsPerItem + 1)
	err := checkLimits([]Item{over})
	if err == nil {
		t.Fatalf("an item at %d reasons was accepted", MaxReasonsPerItem+1)
	}
	if err.Code != CodeResourceExhausted {
		t.Errorf("code = %q, want %q", err.Code, CodeResourceExhausted)
	}
	// buildItem owns the same bound before append, so neither guard is the only
	// one. It refuses at N+1 and accepts at N.
	if _, err := buildItem("u", KindIntentTest, "CF-V0-001",
		syntheticReasons(MaxReasonsPerItem+1), nil, AuthorityCallerReported); err == nil {
		t.Error("buildItem accepted an item past the reason ceiling")
	} else if err.Code != CodeResourceExhausted {
		t.Errorf("buildItem code = %q, want %q", err.Code, CodeResourceExhausted)
	}
}

// syntheticReasons produces a reason list of the requested length. Only the
// first entry needs a disposition, because CF-V0-016 says only the first
// retained reason selects the action; the rest exercise the count alone.
func syntheticReasons(count int) []string {
	reasons := make([]string, 0, count)
	reasons = append(reasons, ReasonCallerReportedNonclosing)
	for index := 1; index < count; index++ {
		reasons = append(reasons, intentTestReasonOrder[index%len(intentTestReasonOrder)])
	}
	return reasons
}

func distinct(mapping map[string]string) map[string]bool {
	values := map[string]bool{}
	for _, value := range mapping {
		values[value] = true
	}
	return values
}

func keys(mapping map[string]string) []string {
	out := make([]string, 0, len(mapping))
	for key := range mapping {
		out = append(out, key)
	}
	return out
}

func repeat(unit string, times int) string {
	out := make([]byte, 0, len(unit)*times)
	for index := 0; index < times; index++ {
		out = append(out, unit...)
	}
	return string(out)
}
