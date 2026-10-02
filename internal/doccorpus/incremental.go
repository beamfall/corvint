package doccorpus

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"sort"
)

const IncrementalSchema = "corvint-corpus-incremental-cache/0"

// ShardIdentity is an explicit policy-admitted reuse unit, never inferred scope.
type ShardIdentity struct {
	Provider string `json:"provider"`
	Path     string `json:"path"`
}
type ShardContribution struct {
	Subjects         []Subject                `json:"subjects"`
	Claims           []Claim                  `json:"claims"`
	Relations        []Relation               `json:"relations"`
	Journeys         []Journey                `json:"journeys"`
	Details          map[string]RecordDetails `json:"details"`
	Declarations     []CapabilityDeclaration  `json:"declarations"`
	AdoptionProvider bool                     `json:"adoption_provider"`
	ImportParity     []ImportParity           `json:"import_parity"`
}
type CachedShard struct {
	Identity     ShardIdentity     `json:"identity"`
	Fingerprint  string            `json:"fingerprint"`
	Contribution ShardContribution `json:"contribution"`
}
type IncrementalCache struct {
	Schema            string        `json:"schema"`
	Builder           Builder       `json:"builder"`
	EligibilitySHA256 string        `json:"eligibility_sha256"`
	Shards            []CachedShard `json:"shards"`
}
type IncrementalStats struct {
	Compiled       []ShardIdentity `json:"compiled"`
	Reused         []ShardIdentity `json:"reused"`
	Fallback       []ShardIdentity `json:"fallback"`
	FallbackReason string          `json:"fallback_reason"`
}
type incrementalBuild struct {
	eligible map[ShardIdentity]bool
	prior    map[ShardIdentity]CachedShard
	cache    *IncrementalCache
	stats    *IncrementalStats
}

func ValidateEligibility(ids []ShardIdentity) error {
	if ids == nil || len(ids) > 128 {
		return fail("explicit bounded reuse eligibility required")
	}
	for i, id := range ids {
		if !identifier(id.Provider) || !validPath(id.Path) || i > 0 && (ids[i-1].Provider+"\x00"+ids[i-1].Path >= id.Provider+"\x00"+id.Path) {
			return fail("reuse eligibility must be sorted unique identities")
		}
	}
	return nil
}
func EligibilityDigest(ids []ShardIdentity) (string, error) {
	if err := ValidateEligibility(ids); err != nil {
		return "", err
	}
	return hashValue(ids)
}

// BuildIncremental pins all original bytes and freshly preloads restrictions.
// The expected cache digest must come from an operator-pinned previous Result.
// Unsupported units use the ordinary import path; no speedup is asserted.
func BuildIncremental(ctx context.Context, root string, m Manifest, ids []ShardIdentity, priorRaw []byte, expected string) (*Artifact, *IncrementalCache, IncrementalStats, error) {
	stats := IncrementalStats{Compiled: []ShardIdentity{}, Reused: []ShardIdentity{}, Fallback: []ShardIdentity{}}
	eligibility, err := EligibilityDigest(ids)
	if err != nil {
		return nil, nil, stats, err
	}
	cache := &IncrementalCache{Schema: IncrementalSchema, Builder: currentBuilder(), EligibilitySHA256: eligibility, Shards: []CachedShard{}}
	r := &incrementalBuild{eligible: map[ShardIdentity]bool{}, prior: map[ShardIdentity]CachedShard{}, cache: cache, stats: &stats}
	for _, id := range ids {
		r.eligible[id] = true
		found := false
		for _, p := range m.Providers {
			found = found || p.ID == id.Provider && p.Kind == "records" && providerHasPath(p, id.Path)
		}
		if !found {
			return nil, nil, stats, fail("reuse unit absent from manifest")
		}
	}
	if len(priorRaw) > 0 {
		if !wire.IsSha256(expected) || expected != Digest(priorRaw) {
			return nil, nil, stats, fail("incremental cache digest mismatch")
		}
		var prior IncrementalCache
		if err := decodeBounded(priorRaw, &prior, MaxCorpusBytes); err != nil {
			return nil, nil, stats, err
		}
		canonical, err := EncodeIncrementalCache(prior)
		if err != nil || !bytes.Equal(canonical, priorRaw) {
			return nil, nil, stats, fail("noncanonical incremental cache")
		}
		if prior.Schema != IncrementalSchema || prior.Builder != cache.Builder || prior.EligibilitySHA256 != eligibility {
			return nil, nil, stats, fail("incremental cache profile builder or eligibility mismatch")
		}
		if len(prior.Shards) > 128 {
			return nil, nil, stats, fail("incremental cache shard bound exceeded")
		}
		for _, s := range prior.Shards {
			if !r.eligible[s.Identity] || r.prior[s.Identity].Fingerprint != "" || !wire.IsSha256(s.Fingerprint) {
				return nil, nil, stats, fail("invalid cached shard identity")
			}
			r.prior[s.Identity] = s
		}
	} else if expected != "" {
		return nil, nil, stats, fail("incremental cache missing")
	}
	a, err := build(ctx, root, m, r)
	sort.Slice(cache.Shards, func(i, j int) bool {
		return cache.Shards[i].Identity.Provider+"\x00"+cache.Shards[i].Identity.Path < cache.Shards[j].Identity.Provider+"\x00"+cache.Shards[j].Identity.Path
	})
	return a, cache, stats, err
}
func EncodeIncrementalCache(cache IncrementalCache) ([]byte, error) {
	b, e := json.Marshal(cache, json.Deterministic(true))
	if e != nil {
		return nil, e
	}
	if len(b)+1 > MaxCorpusBytes {
		return nil, fail("incremental cache byte bound exceeded")
	}
	return append(b, '\n'), nil
}
func (c *compiler) shardFingerprint(p Provider, id ShardIdentity, r *incrementalBuild) (string, error) {
	// Exclude unrelated provider shards and the moving docs commit. Original shard
	// content plus every non-provider dependency retain source/anchor bindings.
	inputs := []Input{}
	for _, in := range c.manifest.Inputs {
		if in.Purpose != "provider" {
			inputs = append(inputs, in)
		}
	}
	sort.Slice(inputs, func(i, j int) bool {
		return inputKey(inputs[i].Revision, inputs[i].Path) < inputKey(inputs[j].Revision, inputs[j].Path)
	})
	source := c.sources[inputKey(p.Revision, id.Path)]
	return hashValue(struct {
		Schema                                        string
		Builder                                       Builder
		Repository                                    Repository
		Profile                                       Profile
		MergeRule                                     string
		Eligibility, Provider, Version, Blob, Content string
		Inputs                                        []Input
	}{c.manifest.Schema, currentBuilder(), c.manifest.Repository, c.manifest.Profile, c.manifest.MergeRule, r.cache.EligibilitySHA256, p.ID, p.Version, source.BlobHash, Digest(source.Data), inputs})
}
func (c *compiler) importRecordsIncremental(p Provider, r *incrementalBuild) error {
	if r == nil {
		return c.importRecords(p)
	}
	for _, path := range recordPaths(p) {
		id := ShardIdentity{p.ID, path}
		part := p
		part.Record = path
		part.Shards = nil
		record := c.providerRecords[p.ID+":"+inputKey(p.Revision, path)]
		if !r.eligible[id] || record.Schema != AdoptionProviderSchema || len(record.Observations) > 0 || record.BehaviorContracts != nil {
			if r.eligible[id] {
				r.stats.Fallback = append(r.stats.Fallback, id)
				r.stats.FallbackReason = "unsupported incremental provider contribution"
			}
			if err := c.importRecord(part); err != nil {
				return err
			}
			continue
		}
		fp, err := c.shardFingerprint(p, id, r)
		if err != nil {
			return err
		}
		cached, ok := r.prior[id]
		var contribution ShardContribution
		if ok && cached.Fingerprint == fp {
			contribution = cached.Contribution
			if err := validateContribution(record, contribution); err != nil {
				return err
			}
			r.stats.Reused = append(r.stats.Reused, id)
		} else {
			// Import into fresh shard-local state; global preload and joins stay on c.
			a := &Artifact{Schema: c.artifact.Schema, Subjects: []Subject{}, Claims: []Claim{}, Relations: []Relation{}, Journeys: []Journey{}, Observations: []Observation{}}
			local := *c
			local.artifact = a
			local.declarations = nil
			local.adoptionProviders = nil
			if err := local.importRecord(part); err != nil {
				return err
			}
			contribution = ShardContribution{a.Subjects, a.Claims, a.Relations, a.Journeys, a.Details, local.declarations, local.adoptionProviders[p.ID], a.ImportParity}
			if err := validateContribution(record, contribution); err != nil {
				return err
			}
			r.stats.Compiled = append(r.stats.Compiled, id)
		}
		if err := c.restoreContribution(contribution); err != nil {
			return err
		}
		r.cache.Shards = append(r.cache.Shards, CachedShard{id, fp, contribution})
	}
	return nil
}
func validateContribution(record ProviderRecord, x ShardContribution) error {
	if !x.AdoptionProvider || len(x.Subjects) != len(record.Subjects) || len(x.Claims) != len(record.Claims) || len(x.Relations) != len(record.Relations) || len(x.Journeys) != len(record.Journeys) || len(x.Details) != len(record.Details) {
		return fail("incomplete cached adoption contribution")
	}
	// Details, declarations and complete unaggregated parity must survive reuse.
	same := func(a, b any) bool {
		aa, e := Encode(a)
		bb, f := Encode(b)
		return e == nil && f == nil && bytes.Equal(aa, bb)
	}
	if !same(x.Details, record.Details) || !same(x.Declarations, record.Capabilities) {
		return fail("cached details or declarations mismatch")
	}
	ids := func(a, b []string) bool { sort.Strings(a); sort.Strings(b); return same(a, b) }
	a, b := []string{}, []string{}
	for _, s := range x.Subjects {
		a = append(a, s.ID)
	}
	for _, s := range record.Subjects {
		b = append(b, s.ID)
	}
	if !ids(a, b) {
		return fail("cached subject inventory mismatch")
	}
	a, b = []string{}, []string{}
	for _, s := range x.Claims {
		a = append(a, s.ID)
	}
	for _, s := range record.Claims {
		b = append(b, s.ID)
	}
	if !ids(a, b) {
		return fail("cached claim inventory mismatch")
	}
	a, b = []string{}, []string{}
	for _, s := range x.Relations {
		a = append(a, s.ID)
	}
	for _, s := range record.Relations {
		b = append(b, s.ID)
	}
	if !ids(a, b) {
		return fail("cached relation inventory mismatch")
	}
	a, b = []string{}, []string{}
	for _, s := range x.Journeys {
		a = append(a, s.ID)
	}
	for _, s := range record.Journeys {
		b = append(b, s.ID)
	}
	if !ids(a, b) {
		return fail("cached journey inventory mismatch")
	}
	counts := map[string]int{"journey": len(record.Journeys), "observation": 0, "typed_detail": len(record.Details)}
	for _, s := range record.Subjects {
		counts["subject/"+s.Kind]++
	}
	for _, s := range record.Relations {
		counts["relation/"+s.Type]++
	}
	for _, s := range record.Claims {
		k := record.Details[s.ID].ClaimKind
		if k == "" {
			k = "unspecified"
		}
		counts["claim/"+k]++
	}
	if len(x.ImportParity) != len(counts) {
		return fail("cached import parity incomplete")
	}
	seen := map[string]bool{}
	for _, p := range x.ImportParity {
		n, ok := counts[p.SourceKind]
		if !ok || seen[p.SourceKind] || p.Provider != record.ID || p.RecordsIn != n || p.Admitted != n || p.Dropped != 0 || len(p.Reasons) > 0 {
			return fail("cached import parity mismatch")
		}
		seen[p.SourceKind] = true
	}
	return nil
}
func (c *compiler) restoreContribution(x ShardContribution) error {
	a := c.artifact
	if len(x.Subjects) > c.manifest.recordLimit()-len(a.Subjects) || len(x.Claims) > c.manifest.recordLimit()-len(a.Claims) || len(x.Relations) > c.manifest.recordLimit()-len(a.Relations) || len(x.Journeys) > c.manifest.journeyLimit()-len(a.Journeys) || len(x.Details) > MaxCorpusRecords-len(a.Details) {
		return fail("cached aggregate record bound exceeded")
	}
	if a.Details == nil {
		a.Details = map[string]RecordDetails{}
	}
	for id, d := range x.Details {
		if _, ok := a.Details[id]; ok {
			return fail("duplicate cached typed detail")
		}
		a.Details[id] = d
	}
	a.Subjects = append(a.Subjects, x.Subjects...)
	a.Claims = append(a.Claims, x.Claims...)
	a.Relations = append(a.Relations, x.Relations...)
	a.Journeys = append(a.Journeys, x.Journeys...)
	a.ImportParity = append(a.ImportParity, x.ImportParity...)
	c.declarations = append(c.declarations, x.Declarations...)
	if c.adoptionProviders == nil {
		c.adoptionProviders = map[string]bool{}
	}
	for _, p := range x.ImportParity {
		c.adoptionProviders[p.Provider] = true
	}
	return nil
}
