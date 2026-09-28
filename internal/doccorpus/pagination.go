package doccorpus

import "sort"

func recordID(value any) string {
	switch x := value.(type) {
	case Subject:
		return x.ID
	case Claim:
		return x.ID
	case Relation:
		return x.ID
	case Journey:
		return x.ID
	case Observation:
		return x.Link.ID
	}
	return ""
}
func recordAnchors(value any) []Anchor {
	switch x := value.(type) {
	case Subject:
		return x.Evidence.Anchors
	case Claim:
		return x.Evidence.Anchors
	case Relation:
		return x.Evidence.Anchors
	case Journey:
		return x.Evidence.Anchors
	case Observation:
		return x.Link.StepEvidence
	}
	return nil
}
func boundedCapabilities(caps []Capability) []Capability {
	out := append([]Capability{}, caps...)
	for i := range out {
		if len(out[i].Records) > 0 {
			out[i].RecordsOmitted = len(out[i].Records)
			out[i].Records = []string{}
		}
	}
	return out
}

// Pagination is bound to the immutable artifact digest and deterministic result
// order. Only the visible page contributes typed details and citations.
func pageReceipt(a *Artifact, original Receipt, request Request) (Receipt, error) {
	total := len(original.Results)
	start := min(request.Offset, total)
	end := min(start+request.Limit, total)
	for {
		r := original
		r.Capabilities = boundedCapabilities(a.Capabilities)
		r.Offset = request.Offset
		r.Results = original.Results[start:end]
		r.Details = nil
		r.Citations = []Anchor{}
		r.Limitations = append([]string{}, original.Limitations...)
		r.Omitted = total - len(r.Results)
		if end < total {
			next := end
			r.NextOffset = &next
			r.Limitations = append(r.Limitations, "result page withheld records; continue at next_offset")
		}
		if len(r.Results) == 0 {
			r.State = "empty"
			r.Miss = "no-match"
		}
		unique := map[string]Anchor{}
		for _, value := range r.Results {
			if d, ok := a.Details[recordID(value)]; ok {
				if r.Details == nil {
					r.Details = map[string]RecordDetails{}
				}
				r.Details[recordID(value)] = d
			}
			for _, anchor := range recordAnchors(value) {
				unique[anchorKey(anchor)] = anchor
			}
		}
		keys := make([]string, 0, len(unique))
		for k := range unique {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) > MaxResults {
			r.Limitations = append(r.Limitations, "citation limit withheld anchors; full record retains evidence")
			keys = keys[:MaxResults]
		}
		for _, key := range keys {
			r.Citations = append(r.Citations, unique[key])
		}
		if _, err := Encode(r); err != nil {
			if end-start > 1 {
				end = start + (end-start)/2
				continue
			}
			return Receipt{}, err
		}
		return r, nil
	}
}

// Admission guarantees a single record and its typed details can be retrieved
// within the independent receipt limit, including duplicated citation metadata.
func validateReadableRecords(a *Artifact) error {
	check := func(value any) error {
		projection := map[string]any{"record": value}
		if d, ok := a.Details[recordID(value)]; ok {
			projection["details"] = d
		}
		raw, err := Encode(projection)
		if err != nil || len(raw) > 1<<20 {
			return fail("individual corpus record exceeds read bound")
		}
		return nil
	}
	for _, x := range a.Subjects {
		if err := check(x); err != nil {
			return err
		}
	}
	for _, x := range a.Claims {
		if err := check(x); err != nil {
			return err
		}
	}
	for _, x := range a.Relations {
		if err := check(x); err != nil {
			return err
		}
	}
	for _, x := range a.Journeys {
		if err := check(x); err != nil {
			return err
		}
	}
	for _, x := range a.Observations {
		if err := check(x); err != nil {
			return err
		}
	}
	return nil
}
