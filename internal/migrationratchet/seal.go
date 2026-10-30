package migrationratchet

import "slices"

// SealSnapshot returns the snapshot in canonical order with every self-digest
// filled from the canonical encoding (MER-V0-014): the policy digest, each
// record's sorted links and digest, each exception digest, record and
// exception order, and finally the artifact digest. It never invents
// evidence: content digests, states, links and exception delta digests stay
// exactly as supplied, and Compare still validates the result.
func SealSnapshot(snapshot Snapshot) Snapshot {
	snapshot.Policy.SHA256 = PolicyDigest(snapshot.Policy)
	snapshot.Records = slices.Clone(snapshot.Records)
	for i := range snapshot.Records {
		record := &snapshot.Records[i]
		record.Links = slices.Clone(record.Links)
		slices.SortFunc(record.Links, compareLinks)
		record.SHA256 = RecordDigest(*record)
	}
	slices.SortFunc(snapshot.Records, compareRecords)
	snapshot.Exceptions = slices.Clone(snapshot.Exceptions)
	for i := range snapshot.Exceptions {
		snapshot.Exceptions[i].SHA256 = ExceptionDigest(snapshot.Exceptions[i])
	}
	slices.SortFunc(snapshot.Exceptions, compareExceptions)
	snapshot.ArtifactSHA256 = SnapshotDigest(snapshot)
	return snapshot
}

// SealProfile seals both snapshots and the migration rule's own digest. The
// rule's baseline and candidate artifact bindings are reviewed inputs and are
// deliberately left as supplied, so a rule cannot be re-bound to new
// artifacts by sealing.
func SealProfile(profile Profile) Profile {
	profile.Baseline = SealSnapshot(profile.Baseline)
	profile.Candidate = SealSnapshot(profile.Candidate)
	if profile.MigrationRule != nil {
		rule := *profile.MigrationRule
		rule.SHA256 = RuleDigest(rule)
		profile.MigrationRule = &rule
	}
	return profile
}

// DecodeSnapshot strictly decodes one snapshot document under the profile
// decoder's bounds.
func DecodeSnapshot(data []byte) (Snapshot, error) {
	var snapshot Snapshot
	if err := decodeStrict(data, &snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

// ValidateSealable refuses a snapshot whose policy a compare would refuse, so seal output passes
// the policy validation (MER-V0-014, MER-V0-098).
func ValidateSealable(snapshot Snapshot) error {
	return validatePolicy(snapshot.Policy)
}
