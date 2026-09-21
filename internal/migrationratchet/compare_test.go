package migrationratchet

import (
	"slices"
	"strings"
	"testing"
)

func TestRevisionBoundReceiptEndToEnd(t *testing.T) {
	profile := fixtureProfile()
	receipt, err := Compare(profile)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Verdict != "pass" || receipt.Baseline.Repository.Revision == receipt.Candidate.Repository.Revision {
		t.Fatalf("receipt = %+v", receipt)
	}
	if receipt.BaselineDenominator.Unresolved != 1 || receipt.CandidateDenominator.Unresolved != 0 || len(receipt.StateChanges) != 1 || receipt.StateChanges[0].Direction != "advance" {
		t.Fatalf("denominators/delta = %+v %+v %+v", receipt.BaselineDenominator, receipt.CandidateDenominator, receipt.StateChanges)
	}
	first, err := Encode(receipt)
	if err != nil {
		t.Fatal(err)
	}
	secondReceipt, err := Compare(profile)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := Encode(secondReceipt)
	if string(first) != string(second) {
		t.Fatalf("receipt is nondeterministic\n%s\n%s", first, second)
	}
}

func TestMigrationEvidenceNegativeControls(t *testing.T) {
	tests := map[string]struct {
		edit        func(*Profile)
		wantVerdict string
		want        string
	}{
		"renamed test masquerades as delete add": {
			edit: func(p *Profile) {
				for i := range p.Candidate.Records {
					if p.Candidate.Records[i].Kind == "test-execution" {
						p.Candidate.Records[i].ID = "test:new-name"
						sealRecord(&p.Candidate.Records[i])
					}
				}
			},
			wantVerdict: "unknown", want: "rename-masquerade",
		},
		"weakened criterion with unchanged count": {
			edit:        func(p *Profile) { changeContent(p, "behavior-contract", "contract:checkout") },
			wantVerdict: "fail", want: "stale-link",
		},
		"removed reverse link": {
			edit: func(p *Profile) {
				for i := range p.Candidate.Records {
					r := &p.Candidate.Records[i]
					if r.Kind == "behavior-contract" {
						r.Links = slices.DeleteFunc(r.Links, func(link Link) bool { return link.Relation == "tested-by" })
						sealRecord(r)
					}
				}
			},
			wantVerdict: "fail", want: "broken-reverse-link",
		},
		"changed test body retains old evidence": {
			edit:        func(p *Profile) { changeContent(p, "test-execution", "test:checkout:chromium") },
			wantVerdict: "fail", want: "stale-link",
		},
		"new uncontracted test": {
			edit: func(p *Profile) {
				r := record("test-execution", "test:orphan", "reviewed")
				p.Candidate.Records = append(p.Candidate.Records, r)
			},
			wantVerdict: "fail", want: "uncontracted-test",
		},
		"expired exception": {
			edit: func(p *Profile) {
				r := record("legacy-case", "legacy:new", "reviewed")
				p.Candidate.Records = append(p.Candidate.Records, r)
				delta := Delta{Kind: r.Kind, CandidateID: r.ID, Code: "added"}
				exception := Exception{ID: "exception:legacy-new", Scope: ExceptionScope{Delta: "addition", Kind: r.Kind, Identity: r.ID, DeltaSHA256: DeltaDigest(delta)}, Owner: "owner:migration", Reviewers: []string{"reviewer:primary"}, Reason: "bounded transition", ExpiresOn: "2026-09-19"}
				exception.SHA256 = ExceptionDigest(exception)
				p.Candidate.Exceptions = append(p.Candidate.Exceptions, exception)
			},
			wantVerdict: "fail", want: "exception-expired",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			profile := fixtureProfile()
			test.edit(&profile)
			sealSnapshot(&profile.Candidate)
			receipt, err := Compare(profile)
			if err != nil {
				t.Fatal(err)
			}
			if receipt.Verdict != test.wantVerdict || !receiptContains(receipt, test.want) {
				t.Fatalf("want %s/%s, got %+v", test.wantVerdict, test.want, receipt)
			}
		})
	}
}

func TestIncomparableInputsRefuseWithoutReviewedRule(t *testing.T) {
	tests := map[string]func(*Profile){
		"schema":     func(p *Profile) { p.Candidate.Schema = "corvint-migration-evidence-snapshot/2" },
		"repository": func(p *Profile) { p.Candidate.Repository.ID = "repository:other" },
		"stale":      func(p *Profile) { p.Candidate.Fresh = false },
		"partial":    func(p *Profile) { p.Candidate.Complete = false },
		"duplicate":  func(p *Profile) { p.Candidate.Records = append(p.Candidate.Records, p.Candidate.Records[0]) },
		"contradictory": func(p *Profile) {
			r := p.Candidate.Records[0]
			r.State = "unresolved"
			sealRecord(&r)
			p.Candidate.Records = append(p.Candidate.Records, r)
		},
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			profile := fixtureProfile()
			edit(&profile)
			sealSnapshot(&profile.Candidate)
			if _, err := Compare(profile); err == nil {
				t.Fatal("incomparable input passed")
			}
		})
	}
}

func TestReviewedRulesAndScopedException(t *testing.T) {
	t.Run("schema rule binds exact artifacts", func(t *testing.T) {
		profile := fixtureProfile()
		profile.Candidate.Schema = "corvint-migration-evidence-snapshot/2"
		sealSnapshot(&profile.Candidate)
		profile.MigrationRule = reviewedRule(profile, []string{"schema"})
		receipt, err := Compare(profile)
		if err != nil || receipt.Verdict != "pass" {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
	})

	t.Run("duplicate rule selects an exact record", func(t *testing.T) {
		profile := fixtureProfile()
		duplicate := profile.Candidate.Records[0]
		profile.Candidate.Records = append(profile.Candidate.Records, duplicate)
		sealSnapshot(&profile.Candidate)
		rule := reviewedRule(profile, []string{"duplicate"})
		rule.Selections = []RecordSelection{{Side: "candidate", Kind: duplicate.Kind, ID: duplicate.ID, SHA256: duplicate.SHA256}}
		rule.SHA256 = RuleDigest(*rule)
		profile.MigrationRule = rule
		receipt, err := Compare(profile)
		if err != nil || receipt.Verdict != "pass" {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
	})

	t.Run("identity mapping admits a reviewed rename", func(t *testing.T) {
		profile := fixtureProfile()
		for i := range profile.Candidate.Records {
			if profile.Candidate.Records[i].Kind == "owner" {
				profile.Candidate.Records[i].ID = "owner:migration-v2"
				sealRecord(&profile.Candidate.Records[i])
			}
		}
		sealSnapshot(&profile.Candidate)
		rule := reviewedRule(profile, nil)
		rule.Owner = "owner:migration-v2"
		rule.Mappings = []IdentityMapping{{Kind: "owner", BaselineID: "owner:migration", CandidateID: "owner:migration-v2"}}
		rule.SHA256 = RuleDigest(*rule)
		profile.MigrationRule = rule
		receipt, err := Compare(profile)
		if err != nil || receipt.Verdict != "pass" {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
	})

	t.Run("mapping cannot reuse an implicit candidate", func(t *testing.T) {
		profile := fixtureProfile()
		other := record("owner", "owner:other", "reviewed")
		profile.Baseline.Records = append(profile.Baseline.Records, other)
		profile.Candidate.Records = append(profile.Candidate.Records, other)
		sealSnapshot(&profile.Baseline)
		sealSnapshot(&profile.Candidate)
		rule := reviewedRule(profile, nil)
		rule.Mappings = []IdentityMapping{{Kind: "owner", BaselineID: "owner:migration", CandidateID: "owner:other"}}
		rule.SHA256 = RuleDigest(*rule)
		profile.MigrationRule = rule
		if _, err := Compare(profile); err == nil {
			t.Fatal("candidate identity was paired twice")
		}
	})

	t.Run("contradictory selections are unique", func(t *testing.T) {
		profile := fixtureProfile()
		contradiction := profile.Candidate.Records[0]
		contradiction.State = "unresolved"
		sealRecord(&contradiction)
		profile.Candidate.Records = append(profile.Candidate.Records, contradiction)
		sealSnapshot(&profile.Candidate)
		rule := reviewedRule(profile, []string{"contradictory"})
		rule.Selections = []RecordSelection{
			{Side: "candidate", Kind: contradiction.Kind, ID: contradiction.ID, SHA256: contradiction.SHA256},
			{Side: "candidate", Kind: contradiction.Kind, ID: contradiction.ID, SHA256: profile.Candidate.Records[0].SHA256},
		}
		rule.SHA256 = RuleDigest(*rule)
		profile.MigrationRule = rule
		if _, err := Compare(profile); err == nil {
			t.Fatal("contradictory selections were order-dependent")
		}
	})

	t.Run("exact unexpired exception does not erase delta", func(t *testing.T) {
		profile := fixtureProfile()
		legacy := record("legacy-case", "legacy:new", "reviewed")
		profile.Candidate.Records = append(profile.Candidate.Records, legacy)
		delta := Delta{Kind: legacy.Kind, CandidateID: legacy.ID, Code: "added"}
		exception := Exception{ID: "exception:legacy-new", Scope: ExceptionScope{Delta: "addition", Kind: legacy.Kind, Identity: legacy.ID, DeltaSHA256: DeltaDigest(delta)}, Owner: "owner:migration", Reviewers: []string{"reviewer:primary"}, Reason: "bounded transition", ExpiresOn: "2026-10-01"}
		exception.SHA256 = ExceptionDigest(exception)
		profile.Candidate.Exceptions = append(profile.Candidate.Exceptions, exception)
		sealSnapshot(&profile.Candidate)
		receipt, err := Compare(profile)
		if err != nil || receipt.Verdict != "pass" || receipt.Additions[len(receipt.Additions)-1].ExceptedBy != exception.ID {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
	})
}

func TestDecodeRejectsEveryTrailingValue(t *testing.T) {
	encoded, err := Encode(fixtureProfile())
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"true", "[]", "garbage"} {
		if _, err := Decode(append(encoded, suffix...)); err == nil {
			t.Fatalf("accepted trailing %q", suffix)
		}
	}
}

func TestCanonicalRecordClosure(t *testing.T) {
	t.Run("unordered links", func(t *testing.T) {
		profile := fixtureProfile()
		for i := range profile.Candidate.Records {
			record := &profile.Candidate.Records[i]
			if len(record.Links) > 1 {
				slices.Reverse(record.Links)
				record.SHA256 = RecordDigest(*record)
				break
			}
		}
		profile.Candidate.ArtifactSHA256 = SnapshotDigest(profile.Candidate)
		if _, err := Compare(profile); err == nil {
			t.Fatal("unordered links accepted")
		}
	})

	t.Run("duplicate link", func(t *testing.T) {
		profile := fixtureProfile()
		for i := range profile.Candidate.Records {
			record := &profile.Candidate.Records[i]
			if len(record.Links) > 0 {
				record.Links = append(record.Links, record.Links[0])
				sealRecord(record)
				break
			}
		}
		sealSnapshot(&profile.Candidate)
		if _, err := Compare(profile); err == nil {
			t.Fatal("duplicate link accepted")
		}
	})
}

func TestExceptionBindsOneConcreteDelta(t *testing.T) {
	first := Delta{Kind: "evidence-manifest", CandidateID: "evidence:checkout", Code: "stale-link", Details: []string{"test-source", "test-execution", "test:checkout", Digest("expected-a"), Digest("actual"), "evidenced-by", Digest("evidence")}}
	second := Delta{Kind: "evidence-manifest", CandidateID: "evidence:checkout", Code: "stale-link", Details: []string{"test-source", "test-execution", "test:checkout", Digest("expected-b"), Digest("actual"), "evidenced-by", Digest("evidence")}}
	receipt := Receipt{StaleEvidence: []Delta{first, second}}
	exception := Exception{ID: "exception:one-link", Scope: ExceptionScope{Delta: "stale-evidence", Kind: first.Kind, Identity: first.CandidateID, DeltaSHA256: DeltaDigest(first)}}
	applyException(&receipt, exception)
	if receipt.StaleEvidence[0].ExceptedBy != exception.ID || receipt.StaleEvidence[1].ExceptedBy != "" {
		t.Fatalf("exception scope widened: %+v", receipt.StaleEvidence)
	}
}

func TestMappedEvidenceIdentityStillRequiresRenewal(t *testing.T) {
	baseSource := record("test-execution", "test:checkout", "reviewed")
	baseEvidence := record("evidence-manifest", "evidence:old", "reviewed")
	baseSource.Links = []Link{{Relation: "evidenced-by", TargetKind: baseEvidence.Kind, TargetID: baseEvidence.ID, TargetSHA256: baseEvidence.ContentSHA256, ReverseRelation: "test-source"}}
	baseEvidence.Links = []Link{{Relation: "test-source", TargetKind: baseSource.Kind, TargetID: baseSource.ID, TargetSHA256: baseSource.ContentSHA256, ReverseRelation: "evidenced-by"}}
	sealRecord(&baseSource)
	sealRecord(&baseEvidence)

	candidateSource := baseSource
	candidateSource.Links = append([]Link{}, baseSource.Links...)
	candidateSource.ContentSHA256 = Digest("changed-test-source")
	candidateEvidence := baseEvidence
	candidateEvidence.ID = "evidence:new"
	candidateEvidence.Links = []Link{{Relation: "test-source", TargetKind: candidateSource.Kind, TargetID: candidateSource.ID, TargetSHA256: candidateSource.ContentSHA256, ReverseRelation: "evidenced-by"}}
	candidateSource.Links = []Link{{Relation: "evidenced-by", TargetKind: candidateEvidence.Kind, TargetID: candidateEvidence.ID, TargetSHA256: candidateEvidence.ContentSHA256, ReverseRelation: "test-source"}}
	sealRecord(&candidateSource)
	sealRecord(&candidateEvidence)

	pairs := []pair{{baseline: &baseSource, candidate: &candidateSource}, {baseline: &baseEvidence, candidate: &candidateEvidence}}
	candidate := map[recordKey]Record{{candidateSource.Kind, candidateSource.ID}: candidateSource, {candidateEvidence.Kind, candidateEvidence.ID}: candidateEvidence}
	receipt := Receipt{StaleEvidence: []Delta{}, BrokenReverseLinks: []Delta{}}
	inspectLinks(&receipt, candidate, baselineByCandidate(pairs), Policy{InvalidateEvidenceOnContentChange: true})
	if len(receipt.StaleEvidence) != 1 || receipt.StaleEvidence[0].Code != "evidence-not-renewed" || receipt.StaleEvidence[0].BaselineID != baseEvidence.ID {
		t.Fatalf("mapped evidence renewal escaped: %+v", receipt.StaleEvidence)
	}
}

func fixtureProfile() Profile {
	policy := Policy{
		ID: "policy:migration", States: []StateRule{{Name: "unresolved", Rank: 0, Unresolved: true}, {Name: "reviewed", Rank: 1}, {Name: "terminal", Rank: 2, Terminal: true}},
		ForbidNewLegacy: true, RequireContractForNewOrChangedTests: true, TerminalStatesCannotRegress: true,
		InvalidateEvidenceOnContentChange: true, UnresolvedDenominatorCannotGrow: true,
	}
	policy.SHA256 = PolicyDigest(policy)
	provider := ProviderBinding{ID: "provider:migration", Schema: "golf-e2e-migration/1", SHA256: Digest("provider")}
	review := record("review", "review:checkout", "reviewed")
	contract := record("behavior-contract", "contract:checkout", "reviewed")
	test := record("test-execution", "test:checkout:chromium", "reviewed")
	evidence := record("evidence-manifest", "evidence:checkout", "reviewed")
	contract.Links = []Link{
		{Relation: "review", TargetKind: review.Kind, TargetID: review.ID, TargetSHA256: review.ContentSHA256, ReverseRelation: "reviews"},
		{Relation: "tested-by", TargetKind: test.Kind, TargetID: test.ID, TargetSHA256: test.ContentSHA256, ReverseRelation: "contract"},
	}
	review.Links = []Link{{Relation: "reviews", TargetKind: contract.Kind, TargetID: contract.ID, TargetSHA256: contract.ContentSHA256, ReverseRelation: "review"}}
	test.Links = []Link{
		{Relation: "contract", TargetKind: contract.Kind, TargetID: contract.ID, TargetSHA256: contract.ContentSHA256, ReverseRelation: "tested-by"},
		{Relation: "evidenced-by", TargetKind: evidence.Kind, TargetID: evidence.ID, TargetSHA256: evidence.ContentSHA256, ReverseRelation: "test-source"},
		{Relation: "review", TargetKind: review.Kind, TargetID: review.ID, TargetSHA256: review.ContentSHA256, ReverseRelation: "reviews"},
	}
	review.Links = append(review.Links, Link{Relation: "reviews", TargetKind: test.Kind, TargetID: test.ID, TargetSHA256: test.ContentSHA256, ReverseRelation: "review"})
	evidence.Links = []Link{{Relation: "test-source", TargetKind: test.Kind, TargetID: test.ID, TargetSHA256: test.ContentSHA256, ReverseRelation: "evidenced-by"}}
	records := []Record{record("legacy-case", "legacy:checkout", "unresolved"), test, contract, record("coverage-target", "coverage:checkout", "reviewed"), record("owner", "owner:migration", "reviewed"), record("reviewer", "reviewer:primary", "reviewed"), review, evidence}
	for i := range records {
		sealRecord(&records[i])
	}
	baseline := Snapshot{Schema: SnapshotSchema, Repository: RepositoryBinding{ID: "repository:golf-e2e", Revision: hexID('1'), Tree: hexID('2')}, Policy: policy, Provider: provider, Complete: true, Fresh: true, Records: records, Exceptions: []Exception{}}
	sealSnapshot(&baseline)
	candidate := baseline
	candidate.Repository.Revision = hexID('3')
	candidate.Repository.Tree = hexID('4')
	candidate.Records = cloneRecords(baseline.Records)
	for i := range candidate.Records {
		if candidate.Records[i].ID == "legacy:checkout" {
			candidate.Records[i].State = "terminal"
			sealRecord(&candidate.Records[i])
		}
	}
	sealSnapshot(&candidate)
	return Profile{Schema: ProfileSchema, EvaluatedOn: "2026-09-20", Baseline: baseline, Candidate: candidate}
}

func record(kind, id, state string) Record {
	r := Record{Kind: kind, ID: id, ContentSHA256: Digest(kind + ":" + id + ":content"), State: state, Links: []Link{}}
	sealRecord(&r)
	return r
}

func sealRecord(record *Record) {
	slices.SortFunc(record.Links, compareLinks)
	record.SHA256 = RecordDigest(*record)
}

func sealSnapshot(snapshot *Snapshot) {
	slices.SortFunc(snapshot.Records, compareRecords)
	slices.SortFunc(snapshot.Exceptions, compareExceptions)
	snapshot.ArtifactSHA256 = SnapshotDigest(*snapshot)
}

func changeContent(profile *Profile, kind, id string) {
	for i := range profile.Candidate.Records {
		r := &profile.Candidate.Records[i]
		if r.Kind == kind && r.ID == id {
			r.ContentSHA256 = Digest(r.ContentSHA256 + ":changed")
			sealRecord(r)
		}
	}
}

func receiptContains(receipt Receipt, want string) bool {
	for _, failure := range receipt.PolicyFailures {
		if stringsContains(failure, want) {
			return true
		}
	}
	for _, delta := range receipt.Unknowns {
		if delta.Code == want {
			return true
		}
	}
	return false
}

func stringsContains(value, want string) bool { return strings.Contains(value, want) }

func hexID(value byte) string { return string(slices.Repeat([]byte{value}, 40)) }

func cloneRecords(records []Record) []Record {
	cloned := append([]Record{}, records...)
	for i := range cloned {
		cloned[i].Links = append([]Link{}, cloned[i].Links...)
	}
	return cloned
}

func reviewedRule(profile Profile, allows []string) *ComparabilityRule {
	rule := &ComparabilityRule{
		ID: "rule:migration", BaselineArtifactSHA256: profile.Baseline.ArtifactSHA256, CandidateArtifactSHA256: profile.Candidate.ArtifactSHA256,
		Owner: "owner:migration", Reviewers: []string{"reviewer:primary"}, Reason: "reviewed exact migration", Allows: allows, Mappings: []IdentityMapping{}, Selections: []RecordSelection{},
	}
	rule.SHA256 = RuleDigest(*rule)
	return rule
}
