// Package frontiernext implements the additive experimental closing relation.
// Repository verification and current authority policy are independent inputs;
// neither producer EMPTY nor caller-reported test output is consumed.
package frontiernext

import (
	"context"
	"time"

	"github.com/Beamfall/corvint/internal/localauthority"
)

const Profile = "frontier/2-experimental"
const OCMProfile = "ocm/0.2-experimental"

type Edge struct {
	ClaimID      string `json:"claimId"`
	ObligationID string `json:"obligationId"`
	CheckID      string `json:"checkId"`
	Subject      string `json:"subject"`
	Function     string `json:"function"`
}
type Selection struct {
	Profile         string `json:"profile"`
	LegacyOCMSHA256 string `json:"legacyOCMSHA256"`
	Mode            string `json:"mode"`
	Edges           []Edge `json:"edges"`
}
type Item struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}
type Result struct {
	Profile        string `json:"profile"`
	Authority      string `json:"authority"`
	State          string `json:"state"`
	UniverseSHA256 string `json:"universeSHA256"`
	Open           []Item `json:"open"`
}
type Request struct {
	CEM        []byte
	OCM        []byte
	Selection  []byte
	Enrollment localauthority.Enrollment
	Receipt    localauthority.Receipt
}

// Universe is reconstructed by the repository adapter, including every
// obligation and every unresolved CEM item. Missing edges remain open.
type Universe struct {
	Identity    string
	Obligations []string
	Claims      map[string][]string
	Open        []Item
	Eligible    map[Edge]bool
}
type Recomputer interface {
	Recompute(context.Context, Request, Selection) (Universe, error)
}

func Compute(ctx context.Context, r Request, policy localauthority.PolicySnapshot, now time.Time, repo Recomputer) (Result, error) {
	return compute(ctx, r, policy, now, repo, false)
}

// ComputeFixture returns authority NONE even when the experimental relation is EMPTY.
func ComputeFixture(ctx context.Context, r Request, policy localauthority.PolicySnapshot, now time.Time, repo Recomputer) (Result, error) {
	return compute(ctx, r, policy, now, repo, true)
}
func compute(ctx context.Context, r Request, p localauthority.PolicySnapshot, now time.Time, repo Recomputer, fixture bool) (Result, error) {
	result := Result{Profile: Profile, Authority: "NONE", State: "OPEN", Open: []Item{}}
	var selection Selection
	if err := localauthority.Decode(r.Selection, &selection); err != nil {
		return result, err
	}
	if selection.Profile != OCMProfile || selection.Mode != localauthority.SelectionMode || len(selection.Edges) == 0 || len(selection.Edges) > 256 {
		return result, localauthority.ErrInvalid
	}
	if selection.LegacyOCMSHA256 != localauthority.BytesDigest(r.OCM) || r.Enrollment.Binding.OCMSHA256 != localauthority.BytesDigest(r.Selection) || r.Enrollment.Binding.CEMSHA256 != localauthority.BytesDigest(r.CEM) {
		return result, localauthority.ErrInvalid
	}
	if repo == nil {
		return result, localauthority.ErrUnavailable
	}
	universe, err := repo.Recompute(ctx, r, selection)
	if err != nil {
		return result, err
	}
	result.UniverseSHA256 = universe.Identity
	result.Open = append(result.Open, universe.Open...)
	verify := localauthority.Verify
	if fixture {
		verify = localauthority.VerifyFixture
	}
	authorityErr := verify(r.Receipt, r.Enrollment, p, now)
	if authorityErr == nil && !fixture {
		result.Authority = "VERIFIED"
	}
	passed := map[string]bool{}
	for _, row := range r.Receipt.Payload.Rows {
		passed[row.Check.ID] = row.Status == "PASS"
	}
	checks := map[string]localauthority.Check{}
	for _, check := range r.Enrollment.Checks {
		checks[check.ID] = check
	}
	seen := map[Edge]bool{}
	linked := map[string]int{}
	selectedChecks := map[string]bool{}
	claimed := map[string]map[string]bool{}
	for _, edge := range selection.Edges {
		if seen[edge] {
			return result, localauthority.ErrInvalid
		}
		seen[edge] = true
		linked[edge.ObligationID]++
		selectedChecks[edge.CheckID] = true
		if claimed[edge.ObligationID] == nil {
			claimed[edge.ObligationID] = map[string]bool{}
		}
		claimed[edge.ObligationID][edge.ClaimID] = true
		check, exists := checks[edge.CheckID]
		if authorityErr != nil || !exists || !passed[edge.CheckID] || check.Subject != edge.Subject || !universe.Eligible[edge] {
			result.Open = append(result.Open, Item{edge.ObligationID, "INTENT_TEST"})
		}
	}
	for _, check := range r.Enrollment.Checks {
		if !selectedChecks[check.ID] {
			result.Open = append(result.Open, Item{check.ID, "INTENT_TEST"})
		}
	}
	for _, id := range universe.Obligations {
		if linked[id] == 0 {
			result.Open = append(result.Open, Item{id, "INTENT_TEST"})
		}
	}
	for _, obligation := range universe.Obligations {
		for _, id := range universe.Claims[obligation] {
			if !claimed[obligation][id] {
				result.Open = append(result.Open, Item{obligation, "INTENT_TEST"})
			}
		}
	}
	if authorityErr != nil {
		result.Open = append(result.Open, Item{"authority", authorityErr.Error()})
	}
	if len(result.Open) == 0 {
		result.State = "EMPTY"
	}
	return result, nil
}
