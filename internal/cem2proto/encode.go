package cem2proto

import (
	"github.com/Beamfall/corvint/internal/cem/wire"
)

func indexOf(table []string, value string) int {
	for i, item := range table {
		if item == value {
			return i
		}
	}
	return -1
}

// Encode splits a canonical cem/1.0 map into a 2.0 prototype header and body.
// Pinned rows also carry each blob OID and span digest. Encode never guesses:
// a map it cannot reproduce byte-identically from Git is an UNSUPPORTED
// abstention (CEM2-V0-020), not a lossy encoding.
//
// Hunk order is not checked here, because only the patch knows it: a map
// whose hunks are not in canonical-patch order projects to different bytes,
// which the projection comparison reports.
func Encode(m *wire.StableMap, pinned bool) (header, body []byte, err error) {
	b, err := encodeBody(m, pinned)
	if err != nil {
		return nil, nil, err
	}
	body, err = EncodeBody(b)
	if err != nil {
		return nil, nil, err
	}
	header, err = EncodeHeader(Header{
		Spec:        ProtoSpec,
		Base:        m.Change.BaseRevision,
		PatchSha256: m.Change.PatchSha256,
		BodySha256:  Sha256Hex(body),
		Role:        m.EffectiveRole(),
	})
	if err != nil {
		return nil, nil, err
	}
	return header, body, nil
}

func encodeBody(m *wire.StableMap, pinned bool) (Body, error) {
	switch {
	case m == nil || m.Change.Spec != wire.StableSpec:
		return Body{}, fail(CodeUnsupported, "only cem/1.0 maps are encoded")
	case len(m.Criteria)+len(m.Receipts)+len(m.Links)+len(m.Artifacts) > 0:
		return Body{}, fail(CodeUnsupported, "criterion, receipt, link and artifact members are outside the prototype")
	case m.Role == wire.StableRoleChange:
		return Body{}, fail(CodeUnsupported, "an explicit role \"change\" cannot be told apart from an absent role")
	case m.Change.ExcludedPath != wire.ExcludedCEMPath:
		return Body{}, fail(CodeUnsupported, "excludedPath %q", m.Change.ExcludedPath)
	}
	b := Body{Paths: []string{}, Evidence: []EvidenceRow{}, Hunks: []HunkRow{}, Provenance: m.Provenance}
	pathIndex := map[string]int{}
	evidenceIndex := map[string]int{}
	for _, e := range m.Change.Evidence {
		if _, dup := evidenceIndex[e.ID]; dup {
			return Body{}, fail(CodeUnsupported, "evidence %s repeats", e.ID)
		}
		if wire.EvidenceIdentity(e.BlobOid, e.Path, e.Span, e.SpanSha256) != e.ID {
			return Body{}, fail(CodeUnsupported, "evidence %s is not its derived identity", e.ID)
		}
		p, ok := pathIndex[e.Path]
		if !ok {
			p = len(b.Paths)
			pathIndex[e.Path] = p
			b.Paths = append(b.Paths, e.Path)
		}
		row := EvidenceRow{Path: p, Start: e.Span.Start, End: e.Span.End}
		if pinned {
			row.BlobOid, row.SpanSha256 = e.BlobOid, e.SpanSha256
		}
		evidenceIndex[e.ID] = len(b.Evidence)
		b.Evidence = append(b.Evidence, row)
	}
	for _, h := range m.Change.Hunks {
		if h.Coverage != nil || h.Discriminates != nil {
			return Body{}, fail(CodeUnsupported, "hunk %s carries a witness", h.ID)
		}
		reason := indexOf(Reasons, h.Reason)
		if reason < 0 || h.Disposition != disposition(h.Reason) {
			return Body{}, fail(CodeUnsupported, "hunk %s reason %q / disposition %q", h.ID, h.Reason, h.Disposition)
		}
		row := HunkRow{Reason: reason}
		for _, basis := range h.Basis {
			e, ok := evidenceIndex[basis.EvidenceID]
			relation := indexOf(Relations, basis.Relation)
			if !ok || relation < 0 {
				return Body{}, fail(CodeUnsupported, "hunk %s basis %s/%s", h.ID, basis.EvidenceID, basis.Relation)
			}
			row.Basis = append(row.Basis, BasisRow{Evidence: e, Relation: relation})
		}
		b.Hunks = append(b.Hunks, row)
	}
	return b, nil
}
