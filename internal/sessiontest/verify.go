package sessiontest

import (
	"fmt"
	"sort"
)

// Verify is the STR-V0-009 verify-shards check: the union of shard receipts
// is accepted only when every shard index 0..N-1 appears exactly once,
// every receipt binds this manifest, source and partition, and every
// manifest unit has exactly one terminal outcome on its assigned shard. It
// returns INCOMPLETE before FAIL before PASS with the reasons that decided
// it; unreadable names receipts that could not be read.
func Verify(m Manifest, digest string, receipts []Receipt, unreadable []string) (string, []string) {
	var incomplete, failed []string
	incomplete = append(incomplete, unreadable...)
	for _, reason := range m.Incomplete {
		incomplete = append(incomplete, "manifest: "+reason)
	}
	if !sourceMatchesDigest(m.Source, m.SourceDigest) {
		incomplete = append(incomplete, "manifest: the source identity does not match its digest")
	}
	shardOf := map[string]int{}
	for _, unit := range m.Units {
		shardOf[unit.Key()] = unit.Shard
	}
	registered := AdapterManifests()
	seenShard := map[int]bool{}
	seenUnit := map[string]bool{}
	for index, receipt := range receipts {
		name := fmt.Sprintf("receipt %d (shard %d)", index, receipt.ShardIndex)
		switch {
		case receipt.ManifestDigest != digest:
			incomplete = append(incomplete, name+": binds another manifest")
			continue
		case receipt.SourceDigest != m.SourceDigest || receipt.PartitionDigest != m.PartitionDigest || receipt.ShardCount != m.ShardCount:
			incomplete = append(incomplete, name+": source, partition or shard count differs from the manifest")
			continue
		case !sourceMatchesDigest(receipt.Source, receipt.SourceDigest) || receipt.Revision != m.Source.Head:
			incomplete = append(incomplete, name+": the source identity does not match its digest or the manifest")
			continue
		case receipt.ShardIndex < 0 || receipt.ShardIndex >= m.ShardCount:
			incomplete = append(incomplete, name+": shard index out of range")
			continue
		case seenShard[receipt.ShardIndex]:
			incomplete = append(incomplete, name+": duplicate shard")
			continue
		}
		seenShard[receipt.ShardIndex] = true
		switch receipt.Result {
		case ResultPass:
		case ResultFail:
			failed = append(failed, name+": result FAIL")
		default:
			incomplete = append(incomplete, name+": result "+receipt.Result)
		}
		for _, invocation := range receipt.Invocations {
			// STR-V0-059: an invocation counts only under the capability
			// manifest version this binary registers for its adapter.
			if manifest, ok := registered[invocation.Adapter]; !ok || invocation.AdapterManifest != manifest.Profile {
				incomplete = append(incomplete, fmt.Sprintf("%s: %s ran under adapter manifest %q, not the registered one", name, invocation.Adapter, invocation.AdapterManifest))
			}
			if invocation.Exit != 0 {
				failed = append(failed, fmt.Sprintf("%s: %s exited %d", name, invocation.Adapter, invocation.Exit))
			}
			for _, outcome := range invocation.Outcomes {
				key := outcome.Unit + " " + outcome.Slice
				assigned, planned := shardOf[key]
				switch {
				case !planned:
					incomplete = append(incomplete, name+": unplanned unit "+key)
				case assigned != receipt.ShardIndex:
					incomplete = append(incomplete, fmt.Sprintf("%s: unit %s belongs to shard %d", name, key, assigned))
				case seenUnit[key]:
					incomplete = append(incomplete, name+": duplicate unit "+key)
				case outcome.Outcome == OutcomeNotRun:
					incomplete = append(incomplete, name+": unit "+key+" NOT_RUN")
				case outcome.Outcome != OutcomePass && outcome.Outcome != OutcomeNoTests:
					failed = append(failed, name+": unit "+key+" "+outcome.Outcome)
				}
				seenUnit[key] = true
			}
		}
	}
	for shard := 0; shard < m.ShardCount; shard++ {
		if !seenShard[shard] {
			incomplete = append(incomplete, fmt.Sprintf("shard %d has no accepted receipt", shard))
		}
	}
	var missing []string
	for key := range shardOf {
		if !seenUnit[key] {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	for _, key := range missing {
		incomplete = append(incomplete, "unit "+key+" has no outcome")
	}
	switch {
	case len(incomplete) != 0:
		return ResultIncomplete, append(incomplete, failed...)
	case len(failed) != 0:
		return ResultFail, failed
	}
	return ResultPass, []string{}
}

// sourceMatchesDigest recomputes the STR-V0-017 source digest, so a receipt
// cannot keep the manifest's digest string over different identities.
func sourceMatchesDigest(source SourceIdentity, digest string) bool {
	computed, err := Digest(source)
	return err == nil && computed == digest
}
