package doccorpus

import "slices"

// suppliedTextCap is the byte cap textOK places on supplied free text (DCP-V1-322).
const suppliedTextCap = 64 << 10

// SearchIncludesClaims reports whether a search request asks for claim hits
// (DCP-V1-328): include is exactly ["claims"] and no section filter is named.
func SearchIncludesClaims(request Request) bool {
	return request.Operation == "search" && request.Sections == nil && slices.Equal(request.Include, []string{"claims"})
}

// resolveAlternateID returns the subject whose alternate ids hold id when id
// names no record and exactly one subject declares it (DCP-V1-330). An
// ambiguous or unknown id resolves to nothing, so the reader's existing miss
// applies.
func resolveAlternateID(a *Artifact, id string) (string, bool) {
	if x := a.RuntimeIndex; x != nil {
		if _, ok := x.IDs[id]; ok {
			return "", false
		}
		if offsets := x.Aliases[id]; len(offsets) == 1 {
			return a.Subjects[offsets[0]].ID, true
		}
		return "", false
	}
	match := -1
	for i, s := range a.Subjects {
		if s.ID == id {
			return "", false
		}
		if slices.Contains(s.AlternateIDs, id) {
			if match >= 0 {
				return "", false
			}
			match = i
		}
	}
	if match < 0 {
		return "", false
	}
	return a.Subjects[match].ID, true
}

// annotateVisibleResults adds the per-result annotations of the visible page:
// supplied text origins (DCP-V1-322), the record kind of each search hit that
// includes claims (DCP-V1-328) and the non-witness limitation of a test run
// whose association is not passed (DCP-V1-323). Each is absent when it does
// not apply, so receipts without them keep their bytes. A ranked search or
// claim-text query also shows each visible result's rank (DCS-V1-479). A
// claims receipt labels each visible claim with its kind (DCS-V1-591).
func annotateVisibleResults(a *Artifact, request Request, r *Receipt) {
	r.TextOrigins, r.ResultKinds, r.ResultRanks, r.ResultLocations, r.ClaimKinds = nil, nil, nil, nil, nil
	locateQuery := request.Operation == "locate" && request.Query != "" && request.Path == ""
	ranked := request.Operation == "search" || locateQuery || request.Operation == "claims" && request.Query != ""
	native := map[string]bool{}
	for _, p := range a.Manifest.Providers {
		if p.Kind == "native" {
			native[p.ID] = true
		}
	}
	untrusted := TextOrigin{Origin: "untrusted", MaxBytes: suppliedTextCap}
	mark := func(id, provider string, fields ...string) {
		if native[provider] {
			return
		}
		if r.TextOrigins == nil {
			r.TextOrigins = map[string]map[string]TextOrigin{}
		}
		if r.TextOrigins[id] == nil {
			r.TextOrigins[id] = map[string]TextOrigin{}
		}
		for _, f := range fields {
			r.TextOrigins[id][f] = untrusted
		}
	}
	tag := SearchIncludesClaims(request)
	witness := []string{}
	for _, value := range r.Results {
		if rank, ok := r.ranks[recordID(value)]; ranked && ok {
			if r.ResultRanks == nil {
				r.ResultRanks = map[string]ResultRank{}
			}
			if s, isSubject := value.(Subject); isSubject && r.matchClaim != nil {
				if c, found := r.matchClaim(s); found {
					anchor := c.Evidence.Anchors[0]
					rank.MatchClaim, rank.MatchAnchor = c.ID, &anchor
				}
			}
			r.ResultRanks[recordID(value)] = rank
		}
		switch v := value.(type) {
		case Subject:
			mark(v.ID, v.Provider, "name")
			if locateQuery {
				r.ResultLocations = setLocations(r.ResultLocations, v)
			}
			if tag {
				r.ResultKinds = setKind(r.ResultKinds, v.ID, "subject")
			}
		case Claim:
			mark(v.ID, v.Provider, "text")
			if tag {
				r.ResultKinds = setKind(r.ResultKinds, v.ID, "claim")
			}
			if request.Operation == "claims" {
				r.ClaimKinds = setKind(r.ClaimKinds, v.ID, claimParityKind(a.Details[v.ID]))
			}
			if run := a.Details[v.ID].TestRun; run != nil {
				for _, axis := range run.Validity {
					if axis.Axis == "association" && axis.Status != "passed" {
						witness = append(witness, "test run "+v.ID+" association is "+axis.Status+": the run does not witness its claim")
					}
				}
			}
		case SuppliedGap:
			mark(v.ID, v.Provider, "reason")
		}
	}
	r.Limitations = append(r.Limitations, witness...)
	// The matcher served its page; a returned receipt holds data only.
	r.matchClaim = nil
}

// setLocations records the line anchors of one locate query result, in
// anchor order; an anchor without a line span is not a location (DCP-V1-651).
func setLocations(locations map[string][]ResultLocation, s Subject) map[string][]ResultLocation {
	if locations == nil {
		locations = map[string][]ResultLocation{}
	}
	found := []ResultLocation{}
	for _, x := range s.Evidence.Anchors {
		if x.Start > 0 {
			found = append(found, ResultLocation{Repository: x.Repository, Path: x.Path, Start: x.Start, End: x.End, Symbol: x.Symbol})
		}
	}
	locations[s.ID] = found
	return locations
}

func setKind(kinds map[string]string, id, kind string) map[string]string {
	if kinds == nil {
		kinds = map[string]string{}
	}
	kinds[id] = kind
	return kinds
}
