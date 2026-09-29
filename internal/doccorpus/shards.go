package doccorpus

import (
	"sort"
)

func recordPaths(p Provider) []string {
	if len(p.Shards) > 0 {
		return p.Shards
	}
	return []string{p.Record}
}
func providerHasPath(p Provider, path string) bool {
	for _, s := range recordPaths(p) {
		if s == path {
			return true
		}
	}
	return false
}

// preloadProviders establishes all local-only restrictions before any provider
// can contribute generic evidence, including when a restriction is in a later shard.
func (c *compiler) preloadProviders() error {
	c.providerRecords = map[string]ProviderRecord{}
	seenPaths := map[string]bool{}
	totals := [5]int{}
	shards := 0
	for _, p := range c.manifest.Providers {
		if p.Kind != "records" {
			continue
		}
		shards += len(recordPaths(p))
		if shards > 128 {
			return fail("aggregate provider shard bound exceeded")
		}
		for _, path := range recordPaths(p) {
			key := inputKey(p.Revision, path)
			if seenPaths[key] {
				return fail("provider shard path reused")
			}
			seenPaths[key] = true
			source, ok := c.sources[key]
			if !ok {
				return fail("provider input missing")
			}
			var record ProviderRecord
			if err := decodeBounded(source.Data, &record, MaxProviderBytes); err != nil {
				return err
			}
			if len(source.Data) > encodingLimit(record) {
				return fail("provider input bound exceeded")
			}
			if record.ID != p.ID || record.Version != p.Version || record.Source != c.manifest.Repository {
				return fail("provider shard identity mismatch")
			}
			if len(p.Shards) > 0 && record.Schema != AdoptionProviderSchema {
				return fail("shards require adoption provider profile")
			}
			if record.Schema != AdoptionProviderSchema && (record.Details != nil || record.RestrictedFindings != nil) {
				return fail("adoption fields require their explicit provider schema")
			}
			counts := [5]int{len(record.Subjects), len(record.Claims), len(record.Relations), len(record.Journeys), len(record.Observations)}
			limits := [5]int{c.manifest.recordLimit(), c.manifest.recordLimit(), c.manifest.recordLimit(), c.manifest.journeyLimit(), MaxRecords}
			for i, n := range counts {
				if n > limits[i]-totals[i] {
					return fail("aggregate provider record bound exceeded")
				}
				totals[i] += n
			}
			if record.Schema == AdoptionProviderSchema {
				if c.manifest.Schema != ManifestSchemaV2 {
					return fail("adoption provider requires manifest /2")
				}
				if err := c.registerRestricted(record); err != nil {
					return err
				}
			}
			c.providerRecords[p.ID+":"+key] = record
		}
	}
	return nil
}

func (c *compiler) aggregateImportParity() {
	counts := map[string]ImportParity{}
	for _, r := range c.artifact.ImportParity {
		key := r.Provider + "\x00" + r.SourceKind
		x := counts[key]
		if x.Provider == "" {
			x = ImportParity{Provider: r.Provider, SourceKind: r.SourceKind, Reasons: []string{}}
		}
		x.RecordsIn += r.RecordsIn
		x.Admitted += r.Admitted
		x.Dropped += r.Dropped
		if len(r.Reasons) > 0 {
			x.Reasons = []string{"restricted-local-only"}
		}
		counts[key] = x
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	c.artifact.ImportParity = nil
	for _, key := range keys {
		c.artifact.ImportParity = append(c.artifact.ImportParity, counts[key])
	}
	summaries := map[string]RestrictedSummary{}
	for _, r := range c.artifact.RestrictedSummaries {
		x := summaries[r.Provider]
		if x.Provider == "" {
			x = RestrictedSummary{Provider: r.Provider, BySeverity: map[string]int{}}
		}
		x.Total += r.Total
		for k, n := range r.BySeverity {
			x.BySeverity[k] += n
		}
		summaries[r.Provider] = x
	}
	keys = keys[:0]
	for key := range summaries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	c.artifact.RestrictedSummaries = nil
	for _, key := range keys {
		c.artifact.RestrictedSummaries = append(c.artifact.RestrictedSummaries, summaries[key])
	}
}
