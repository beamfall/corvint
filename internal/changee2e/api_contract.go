package changee2e

import (
	"bytes"
	"context"
	"path"
	"sort"

	"github.com/Beamfall/corvint/internal/appmap"
)

// APIContractBasis names the declared API contract join reach source (CCE-V0-114).
const APIContractBasis = "api-contract"

// APIContractRelation is the relation an obligations report names when the base declarations
// declare API contracts.
const APIContractRelation = "e2e-blocks:api_contracts@base"

// maxContractReads bounds the head blobs one obligations build reads for API contract joins.
// Beyond it the join adds no reach, so every changed path keeps the span impact left it.
const maxContractReads = 512

// apiContractHits widens reach through the API contracts the base declarations declare
// (CCE-V0-114): a changed path matching a contract glob reaches each screen that places a head-tree
// file naming the contract's endpoint literal. Hits only add reach: a pair already reached is not
// repeated, and they are never counted as placed, so no unknown span is removed (CCE-V0-115).
func apiContractHits(ctx context.Context, root, head string, m *appmap.Map, contracts []APIContract, changed []string, prior []appmap.ImpactScreen) ([]appmap.ImpactScreen, error) {
	matched := map[int][]string{} // contract index -> the changed paths its globs match
	for i, c := range contracts {
		for _, p := range changed {
			for _, g := range c.Paths {
				if ok, _ := path.Match(g, p); ok {
					matched[i] = append(matched[i], p)
					break
				}
			}
		}
	}
	if len(matched) == 0 {
		return nil, nil
	}
	placements := screenPlacements(m)
	files := make([]string, 0, len(placements))
	for f := range placements {
		files = append(files, f)
	}
	if len(files) > maxContractReads {
		return nil, nil
	}
	sort.Strings(files)
	blobs := map[string][]byte{}
	for _, f := range files {
		data, _, ok, err := blobAt(ctx, root, head, f)
		if err != nil {
			return nil, err
		}
		if ok {
			blobs[f] = data
		}
	}
	already := map[string]bool{}
	for _, h := range prior {
		for _, p := range h.Paths {
			already[h.ID+"\x00"+p] = true
		}
	}
	type key struct{ screen, basis string }
	byKey := map[key]map[string]bool{}
	for i, c := range contracts {
		if len(matched[i]) == 0 {
			continue
		}
		for _, f := range files {
			if !namesEndpoint(blobs[f], c.Endpoint) {
				continue
			}
			for _, screen := range placements[f] {
				k := key{screen, APIContractBasis + ":" + c.ContractID + ":" + f}
				for _, p := range matched[i] {
					if p == f || already[screen+"\x00"+p] {
						continue
					}
					already[screen+"\x00"+p] = true
					if byKey[k] == nil {
						byKey[k] = map[string]bool{}
					}
					byKey[k][p] = true
				}
			}
		}
	}
	var hits []appmap.ImpactScreen
	for k, paths := range byKey {
		ps := make([]string, 0, len(paths))
		for p := range paths {
			ps = append(ps, p)
		}
		sort.Strings(ps)
		hits = append(hits, appmap.ImpactScreen{ID: k.screen, Basis: k.basis, Paths: ps})
	}
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].ID < hits[j].ID || (hits[i].ID == hits[j].ID && hits[i].Basis < hits[j].Basis)
	})
	return hits, nil
}

// namesEndpoint reports whether data holds the endpoint literal not followed by an identifier
// byte, so "/api/slots" names "/api/slots" and "/api/slots/1" but not "/api/slotsx".
func namesEndpoint(data []byte, endpoint string) bool {
	e := []byte(endpoint)
	for i := 0; ; {
		j := bytes.Index(data[i:], e)
		if j < 0 {
			return false
		}
		end := i + j + len(e)
		if end == len(data) || !isIdentByte(data[end]) {
			return true
		}
		i = i + j + 1
	}
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '-' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
