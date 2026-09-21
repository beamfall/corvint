package migrationratchet

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"time"
)

var recordKinds = set("legacy-case test-execution behavior-contract coverage-target owner reviewer review evidence-manifest")
var comparableDimensions = set("schema repository policy provider stale partial duplicate contradictory")
var exceptionDeltas = set("addition removal content-change state-advance state-regression state-lateral stale-evidence broken-reverse-link unknown")

type recordKey struct{ kind, id string }

type pair struct {
	baseline  *Record
	candidate *Record
}

func Decode(data []byte) (Profile, error) {
	if len(data) == 0 || len(data) > MaxBytes {
		return Profile{}, errors.New("migration-ratchet input size refused")
	}
	var profile Profile
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&profile); err != nil {
		return Profile{}, fmt.Errorf("migration-ratchet input refused: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Profile{}, errors.New("migration-ratchet input refused: trailing JSON")
	}
	return profile, nil
}

func Encode(value any) ([]byte, error) {
	return json.Marshal(value)
}

func Digest(value any) string {
	encoded, _ := Encode(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func SnapshotDigest(snapshot Snapshot) string {
	snapshot.ArtifactSHA256 = ""
	return Digest(snapshot)
}

func PolicyDigest(policy Policy) string {
	policy.SHA256 = ""
	return Digest(policy)
}

func RecordDigest(record Record) string {
	record.SHA256 = ""
	return Digest(record)
}

func ExceptionDigest(exception Exception) string {
	exception.SHA256 = ""
	return Digest(exception)
}

func RuleDigest(rule ComparabilityRule) string {
	rule.SHA256 = ""
	return Digest(rule)
}

func DeltaDigest(delta Delta) string {
	delta.ExceptedBy = ""
	return Digest(delta)
}

func StateDeltaDigest(delta StateDelta) string {
	delta.ExceptedBy = ""
	return Digest(delta)
}

func Compare(profile Profile) (Receipt, error) {
	if profile.Schema != ProfileSchema {
		return Receipt{}, errors.New("migration-ratchet profile schema refused")
	}
	if _, err := time.Parse("2006-01-02", profile.EvaluatedOn); err != nil {
		return Receipt{}, errors.New("migration-ratchet evaluation date refused")
	}
	if err := validateSnapshot("baseline", profile.Baseline); err != nil {
		return Receipt{}, err
	}
	if err := validateSnapshot("candidate", profile.Candidate); err != nil {
		return Receipt{}, err
	}
	allowed, err := validateComparability(profile)
	if err != nil {
		return Receipt{}, err
	}
	baseline, err := selectRecords("baseline", profile.Baseline.Records, profile.MigrationRule)
	if err != nil {
		return Receipt{}, err
	}
	candidate, err := selectRecords("candidate", profile.Candidate.Records, profile.MigrationRule)
	if err != nil {
		return Receipt{}, err
	}
	pairs, err := pairRecords(baseline, candidate, profile.MigrationRule)
	if err != nil {
		return Receipt{}, err
	}
	receipt := newReceipt(profile, baseline, candidate)
	states := stateRules(profile.Candidate.Policy)
	for _, item := range pairs {
		comparePair(&receipt, item, states)
	}
	detectRenameMasquerades(&receipt, baseline, candidate)
	inspectLinks(&receipt, candidate, baselineByCandidate(pairs), profile.Candidate.Policy)
	inspectExceptions(&receipt, profile)
	applyPolicy(&receipt, profile, candidate, states)
	if len(allowed) > 0 {
		receipt.Identities = append(receipt.Identities, IdentityDelta{Kind: "comparability-rule", CandidateID: profile.MigrationRule.ID, Changes: allowed})
	}
	normalizeReceipt(&receipt)
	switch {
	case hasUnexceptedUnknown(receipt.Unknowns):
		receipt.Verdict = "unknown"
	case len(receipt.PolicyFailures) > 0:
		receipt.Verdict = "fail"
	default:
		receipt.Verdict = "pass"
	}
	return receipt, nil
}

func validateSnapshot(side string, snapshot Snapshot) error {
	if snapshot.Schema == "" || !text(snapshot.Repository.ID) || !objectID(snapshot.Repository.Revision) || !objectID(snapshot.Repository.Tree) {
		return fmt.Errorf("migration-ratchet %s identity refused", side)
	}
	if !text(snapshot.Policy.ID) || !digest(snapshot.Policy.SHA256) || PolicyDigest(snapshot.Policy) != snapshot.Policy.SHA256 {
		return fmt.Errorf("migration-ratchet %s policy digest refused", side)
	}
	if !text(snapshot.Provider.ID) || !text(snapshot.Provider.Schema) || !digest(snapshot.Provider.SHA256) {
		return fmt.Errorf("migration-ratchet %s provider identity refused", side)
	}
	if !digest(snapshot.ArtifactSHA256) || SnapshotDigest(snapshot) != snapshot.ArtifactSHA256 {
		return fmt.Errorf("migration-ratchet %s artifact digest refused", side)
	}
	if len(snapshot.Records) > MaxRecords || len(snapshot.Exceptions) > MaxRecords {
		return fmt.Errorf("migration-ratchet %s bound refused", side)
	}
	if err := validatePolicy(snapshot.Policy); err != nil {
		return fmt.Errorf("migration-ratchet %s %w", side, err)
	}
	if !slices.IsSortedFunc(snapshot.Records, compareRecords) {
		return fmt.Errorf("migration-ratchet %s records are not canonical", side)
	}
	for _, record := range snapshot.Records {
		if !recordKinds[record.Kind] || !text(record.ID) || !digest(record.ContentSHA256) || !digest(record.SHA256) || RecordDigest(record) != record.SHA256 || !text(record.State) {
			return fmt.Errorf("migration-ratchet %s record refused", side)
		}
		if !slices.IsSortedFunc(record.Links, compareLinks) {
			return fmt.Errorf("migration-ratchet %s record links are not canonical", side)
		}
		for i := 1; i < len(record.Links); i++ {
			if compareLinks(record.Links[i-1], record.Links[i]) == 0 {
				return fmt.Errorf("migration-ratchet %s duplicate link refused", side)
			}
		}
		for _, link := range record.Links {
			if !text(link.Relation) || !recordKinds[link.TargetKind] || !text(link.TargetID) || !digest(link.TargetSHA256) || !text(link.ReverseRelation) {
				return fmt.Errorf("migration-ratchet %s link refused", side)
			}
		}
	}
	seenExceptions := map[string]bool{}
	if !slices.IsSortedFunc(snapshot.Exceptions, compareExceptions) {
		return fmt.Errorf("migration-ratchet %s exceptions are not canonical", side)
	}
	records := map[recordKey]bool{}
	for _, record := range snapshot.Records {
		records[recordKey{record.Kind, record.ID}] = true
	}
	for _, exception := range snapshot.Exceptions {
		if seenExceptions[exception.ID] || !text(exception.ID) || !digest(exception.SHA256) || ExceptionDigest(exception) != exception.SHA256 || !text(exception.Owner) || len(exception.Reviewers) == 0 || !text(exception.Reason) || !exceptionDeltas[exception.Scope.Delta] || !recordKinds[exception.Scope.Kind] || !text(exception.Scope.Identity) || !digest(exception.Scope.DeltaSHA256) {
			return fmt.Errorf("migration-ratchet %s exception refused", side)
		}
		seenExceptions[exception.ID] = true
		if _, err := time.Parse("2006-01-02", exception.ExpiresOn); err != nil || !strictStrings(exception.Reviewers) {
			return fmt.Errorf("migration-ratchet %s exception refused", side)
		}
		if !records[recordKey{"owner", exception.Owner}] {
			return fmt.Errorf("migration-ratchet %s exception owner refused", side)
		}
		for _, reviewer := range exception.Reviewers {
			if !records[recordKey{"reviewer", reviewer}] {
				return fmt.Errorf("migration-ratchet %s exception reviewer refused", side)
			}
		}
	}
	return nil
}

func validatePolicy(policy Policy) error {
	if len(policy.States) == 0 || len(policy.States) > 64 {
		return errors.New("policy states refused")
	}
	names := map[string]bool{}
	ranks := map[int]bool{}
	for _, state := range policy.States {
		if !text(state.Name) || names[state.Name] || ranks[state.Rank] {
			return errors.New("policy states refused")
		}
		names[state.Name] = true
		ranks[state.Rank] = true
	}
	return nil
}

func validateComparability(profile Profile) ([]string, error) {
	checks := []struct {
		name string
		same bool
	}{
		{"schema", profile.Baseline.Schema == profile.Candidate.Schema && profile.Baseline.Schema == SnapshotSchema},
		{"repository", profile.Baseline.Repository.ID == profile.Candidate.Repository.ID},
		{"policy", profile.Baseline.Policy.ID == profile.Candidate.Policy.ID && profile.Baseline.Policy.SHA256 == profile.Candidate.Policy.SHA256},
		{"provider", profile.Baseline.Provider == profile.Candidate.Provider},
		{"stale", profile.Baseline.Fresh && profile.Candidate.Fresh},
		{"partial", profile.Baseline.Complete && profile.Candidate.Complete},
	}
	required := []string{}
	for _, check := range checks {
		if !check.same {
			required = append(required, check.name)
		}
	}
	if len(required) == 0 && profile.MigrationRule == nil {
		return nil, nil
	}
	if profile.MigrationRule == nil {
		return nil, fmt.Errorf("migration-ratchet incomparable %s refused", strings.Join(required, ","))
	}
	rule := *profile.MigrationRule
	if !text(rule.ID) || !text(rule.Owner) || !text(rule.Reason) || len(rule.Reviewers) == 0 || !strictStrings(rule.Reviewers) || !digest(rule.SHA256) || RuleDigest(rule) != rule.SHA256 || rule.BaselineArtifactSHA256 != profile.Baseline.ArtifactSHA256 || rule.CandidateArtifactSHA256 != profile.Candidate.ArtifactSHA256 {
		return nil, errors.New("migration-ratchet migration rule refused")
	}
	if len(rule.Allows) > 0 && !strictStrings(rule.Allows) {
		return nil, errors.New("migration-ratchet migration rule allows refused")
	}
	allowed := set(strings.Join(rule.Allows, " "))
	for name := range allowed {
		if !comparableDimensions[name] {
			return nil, errors.New("migration-ratchet migration rule allows refused")
		}
	}
	for _, name := range required {
		if !allowed[name] {
			return nil, fmt.Errorf("migration-ratchet incomparable %s refused", name)
		}
	}
	if !snapshotHasIdentity(profile.Candidate, "owner", rule.Owner) {
		return nil, errors.New("migration-ratchet migration rule owner refused")
	}
	for _, reviewer := range rule.Reviewers {
		if !snapshotHasIdentity(profile.Candidate, "reviewer", reviewer) {
			return nil, errors.New("migration-ratchet migration rule reviewer refused")
		}
	}
	if err := validateRuleSelections(profile, rule); err != nil {
		return nil, err
	}
	return required, nil
}

func validateRuleSelections(profile Profile, rule ComparabilityRule) error {
	type selectionKey struct{ side, kind, id string }
	seen := map[selectionKey]bool{}
	for _, selection := range rule.Selections {
		key := selectionKey{selection.Side, selection.Kind, selection.ID}
		if seen[key] || (selection.Side != "baseline" && selection.Side != "candidate") || !recordKinds[selection.Kind] || !text(selection.ID) || !digest(selection.SHA256) {
			return errors.New("migration-ratchet migration rule selection refused")
		}
		seen[key] = true
		records := profile.Baseline.Records
		if selection.Side == "candidate" {
			records = profile.Candidate.Records
		}
		matches := []Record{}
		for _, record := range records {
			if record.Kind == selection.Kind && record.ID == selection.ID {
				matches = append(matches, record)
			}
		}
		if len(matches) < 2 {
			return errors.New("migration-ratchet migration rule selection has no duplicate")
		}
		issue := "duplicate"
		found := false
		for _, record := range matches {
			if record.SHA256 != matches[0].SHA256 {
				issue = "contradictory"
			}
			if record.SHA256 == selection.SHA256 {
				found = true
			}
		}
		if !found || !slices.Contains(rule.Allows, issue) {
			return errors.New("migration-ratchet migration rule selection refused")
		}
	}
	return nil
}

func selectRecords(side string, records []Record, rule *ComparabilityRule) (map[recordKey]Record, error) {
	groups := map[recordKey][]Record{}
	for _, record := range records {
		key := recordKey{record.Kind, record.ID}
		groups[key] = append(groups[key], record)
	}
	selected := map[recordKey]Record{}
	for key, group := range groups {
		if len(group) == 1 {
			selected[key] = group[0]
			continue
		}
		kind := "duplicate"
		for _, record := range group[1:] {
			if record.SHA256 != group[0].SHA256 {
				kind = "contradictory"
			}
		}
		choice, ok := selectedRecord(side, key, group, rule, kind)
		if !ok {
			return nil, fmt.Errorf("migration-ratchet %s %s record refused: %s/%s", side, kind, key.kind, key.id)
		}
		selected[key] = choice
	}
	return selected, nil
}

func selectedRecord(side string, key recordKey, records []Record, rule *ComparabilityRule, issue string) (Record, bool) {
	if rule == nil || !slices.Contains(rule.Allows, issue) {
		return Record{}, false
	}
	for _, selection := range rule.Selections {
		if selection.Side != side || selection.Kind != key.kind || selection.ID != key.id {
			continue
		}
		for _, record := range records {
			if record.SHA256 == selection.SHA256 {
				return record, true
			}
		}
	}
	return Record{}, false
}

func pairRecords(baseline, candidate map[recordKey]Record, rule *ComparabilityRule) ([]pair, error) {
	mappedBaseline := map[recordKey]recordKey{}
	mappedCandidate := map[recordKey]recordKey{}
	if rule != nil {
		for _, mapping := range rule.Mappings {
			from := recordKey{mapping.Kind, mapping.BaselineID}
			to := recordKey{mapping.Kind, mapping.CandidateID}
			if !recordKinds[mapping.Kind] || !text(mapping.BaselineID) || !text(mapping.CandidateID) || mappedBaseline[from] != (recordKey{}) || mappedCandidate[to] != (recordKey{}) {
				return nil, errors.New("migration-ratchet identity mapping refused")
			}
			if _, ok := baseline[from]; !ok {
				return nil, errors.New("migration-ratchet identity mapping source missing")
			}
			if _, ok := candidate[to]; !ok {
				return nil, errors.New("migration-ratchet identity mapping target missing")
			}
			mappedBaseline[from] = to
			mappedCandidate[to] = from
		}
	}
	result := make([]pair, 0, len(baseline)+len(candidate))
	claimedCandidates := map[recordKey]recordKey{}
	for _, key := range sortedRecordKeys(baseline) {
		base := baseline[key]
		candidateKey := key
		if mapped, ok := mappedBaseline[key]; ok {
			candidateKey = mapped
		}
		cand, candidateOK := candidate[candidateKey]
		item := pair{baseline: &base}
		if candidateOK {
			if claimedBy, claimed := claimedCandidates[candidateKey]; claimed {
				return nil, fmt.Errorf("migration-ratchet identity mapping reuses candidate %s/%s for %s/%s and %s/%s", candidateKey.kind, candidateKey.id, claimedBy.kind, claimedBy.id, key.kind, key.id)
			}
			claimedCandidates[candidateKey] = key
			item.candidate = &cand
		}
		result = append(result, item)
	}
	for _, key := range sortedRecordKeys(candidate) {
		if _, claimed := claimedCandidates[key]; claimed {
			continue
		}
		cand := candidate[key]
		result = append(result, pair{candidate: &cand})
	}
	return result, nil
}

func newReceipt(profile Profile, baseline, candidate map[recordKey]Record) Receipt {
	return Receipt{
		Schema: ReceiptSchema, ProfileSHA256: Digest(profile), EvaluatedOn: profile.EvaluatedOn,
		Baseline: binding(profile.Baseline), Candidate: binding(profile.Candidate),
		BaselineDenominator:  denominator(baseline, stateRules(profile.Baseline.Policy)),
		CandidateDenominator: denominator(candidate, stateRules(profile.Candidate.Policy)),
		Additions:            []Delta{}, Removals: []Delta{}, ContentChanges: []Delta{}, StateChanges: []StateDelta{},
		StaleEvidence: []Delta{}, BrokenReverseLinks: []Delta{}, Unknowns: []Delta{}, Identities: []IdentityDelta{}, PolicyFailures: []string{},
		Limitations: []string{"incremental ratchet pass does not prove behavioral adequacy", "incremental ratchet pass does not prove migration completeness", "provider review and evidence authenticity remain externally governed"},
	}
}

func comparePair(receipt *Receipt, item pair, states map[string]StateRule) {
	if item.baseline == nil {
		record := *item.candidate
		receipt.Additions = append(receipt.Additions, Delta{Kind: record.Kind, CandidateID: record.ID, Code: "added"})
		receipt.Identities = append(receipt.Identities, IdentityDelta{Kind: record.Kind, CandidateID: record.ID, Changes: []string{"addition"}})
		return
	}
	if item.candidate == nil {
		record := *item.baseline
		receipt.Removals = append(receipt.Removals, Delta{Kind: record.Kind, BaselineID: record.ID, Code: "removed"})
		receipt.Identities = append(receipt.Identities, IdentityDelta{Kind: record.Kind, BaselineID: record.ID, Changes: []string{"removal"}})
		return
	}
	base, candidate := *item.baseline, *item.candidate
	changes := []string{}
	if recordContentDigest(base) != recordContentDigest(candidate) {
		receipt.ContentChanges = append(receipt.ContentChanges, Delta{Kind: candidate.Kind, BaselineID: base.ID, CandidateID: candidate.ID, Code: "content-changed", Details: []string{base.ContentSHA256, candidate.ContentSHA256}})
		changes = append(changes, "content")
	}
	if base.State != candidate.State {
		direction := "unknown"
		from, fromOK := states[base.State]
		to, toOK := states[candidate.State]
		if fromOK && toOK {
			switch {
			case to.Rank > from.Rank:
				direction = "advance"
			case to.Rank < from.Rank:
				direction = "regression"
			default:
				direction = "lateral"
			}
		} else {
			receipt.Unknowns = append(receipt.Unknowns, Delta{Kind: candidate.Kind, BaselineID: base.ID, CandidateID: candidate.ID, Code: "state-uncomparable", Details: []string{base.State, candidate.State}})
		}
		receipt.StateChanges = append(receipt.StateChanges, StateDelta{Kind: candidate.Kind, BaselineID: base.ID, CandidateID: candidate.ID, From: base.State, To: candidate.State, Direction: direction})
		changes = append(changes, "state-"+direction)
	}
	if len(changes) > 0 {
		receipt.Identities = append(receipt.Identities, IdentityDelta{Kind: candidate.Kind, BaselineID: base.ID, CandidateID: candidate.ID, Changes: changes})
	}
}

func recordContentDigest(record Record) string {
	record.SHA256 = ""
	record.ID = ""
	record.State = ""
	return Digest(record)
}

func detectRenameMasquerades(receipt *Receipt, baseline, candidate map[recordKey]Record) {
	for _, removal := range receipt.Removals {
		for _, addition := range receipt.Additions {
			if removal.Kind != addition.Kind {
				continue
			}
			removed := baseline[recordKey{removal.Kind, removal.BaselineID}]
			added := candidate[recordKey{addition.Kind, addition.CandidateID}]
			if removed.ContentSHA256 == added.ContentSHA256 {
				receipt.Unknowns = append(receipt.Unknowns, Delta{Kind: addition.Kind, BaselineID: removal.BaselineID, CandidateID: addition.CandidateID, Code: "rename-masquerade"})
			}
		}
	}
}

func inspectLinks(receipt *Receipt, candidate, baselineByCandidate map[recordKey]Record, policy Policy) {
	for _, key := range sortedRecordKeys(candidate) {
		record := candidate[key]
		for _, link := range record.Links {
			target, ok := candidate[recordKey{link.TargetKind, link.TargetID}]
			if !ok || target.ContentSHA256 != link.TargetSHA256 {
				actual := "missing"
				if ok {
					actual = target.ContentSHA256
				}
				receipt.StaleEvidence = append(receipt.StaleEvidence, Delta{Kind: key.kind, CandidateID: key.id, Code: "stale-link", Details: []string{link.Relation, link.TargetKind, link.TargetID, link.TargetSHA256, actual, link.ReverseRelation, record.ContentSHA256}})
				continue
			}
			if !hasReverse(target, record, link) {
				receipt.BrokenReverseLinks = append(receipt.BrokenReverseLinks, Delta{Kind: key.kind, CandidateID: key.id, Code: "broken-reverse-link", Details: []string{link.Relation, link.TargetKind, link.TargetID, link.TargetSHA256, link.ReverseRelation, record.ContentSHA256, target.ContentSHA256}})
			}
			if !policy.InvalidateEvidenceOnContentChange || (record.Kind != "evidence-manifest" && record.Kind != "review") {
				continue
			}
			previous, recordExisted := baselineByCandidate[key]
			previousTarget, targetExisted := baselineByCandidate[recordKey{link.TargetKind, link.TargetID}]
			if recordExisted && targetExisted && previous.ContentSHA256 == record.ContentSHA256 && previousTarget.ContentSHA256 != target.ContentSHA256 {
				receipt.StaleEvidence = append(receipt.StaleEvidence, Delta{Kind: key.kind, BaselineID: previous.ID, CandidateID: key.id, Code: "evidence-not-renewed", Details: []string{link.Relation, link.TargetKind, previousTarget.ID, link.TargetID, link.ReverseRelation, previous.ContentSHA256, record.ContentSHA256, previousTarget.ContentSHA256, target.ContentSHA256}})
			}
		}
	}
}

func baselineByCandidate(pairs []pair) map[recordKey]Record {
	result := map[recordKey]Record{}
	for _, item := range pairs {
		if item.baseline == nil || item.candidate == nil {
			continue
		}
		result[recordKey{item.candidate.Kind, item.candidate.ID}] = *item.baseline
	}
	return result
}

func hasReverse(target, source Record, link Link) bool {
	for _, reverse := range target.Links {
		if reverse.Relation == link.ReverseRelation && reverse.TargetKind == source.Kind && reverse.TargetID == source.ID && reverse.TargetSHA256 == source.ContentSHA256 && reverse.ReverseRelation == link.Relation {
			return true
		}
	}
	return false
}

func inspectExceptions(receipt *Receipt, profile Profile) {
	baseline := map[string]Exception{}
	for _, exception := range profile.Baseline.Exceptions {
		baseline[exception.ID] = exception
	}
	for _, exception := range profile.Candidate.Exceptions {
		if old, ok := baseline[exception.ID]; ok && old.SHA256 != exception.SHA256 {
			receipt.PolicyFailures = append(receipt.PolicyFailures, "exception-widened:"+exception.ID)
			continue
		}
		if exception.ExpiresOn < profile.EvaluatedOn {
			receipt.PolicyFailures = append(receipt.PolicyFailures, "exception-expired:"+exception.ID)
			continue
		}
		applyException(receipt, exception)
	}
}

func applyException(receipt *Receipt, exception Exception) {
	apply := func(delta *Delta, name string) {
		identity := delta.CandidateID
		if identity == "" {
			identity = delta.BaselineID
		}
		if exception.Scope.Delta == name && exception.Scope.Kind == delta.Kind && exception.Scope.Identity == identity && exception.Scope.DeltaSHA256 == DeltaDigest(*delta) {
			delta.ExceptedBy = exception.ID
		}
	}
	for i := range receipt.Additions {
		apply(&receipt.Additions[i], "addition")
	}
	for i := range receipt.Removals {
		apply(&receipt.Removals[i], "removal")
	}
	for i := range receipt.ContentChanges {
		apply(&receipt.ContentChanges[i], "content-change")
	}
	for i := range receipt.StaleEvidence {
		apply(&receipt.StaleEvidence[i], "stale-evidence")
	}
	for i := range receipt.BrokenReverseLinks {
		apply(&receipt.BrokenReverseLinks[i], "broken-reverse-link")
	}
	for i := range receipt.Unknowns {
		apply(&receipt.Unknowns[i], "unknown")
	}
	for i := range receipt.StateChanges {
		state := &receipt.StateChanges[i]
		if exception.Scope.Delta == "state-"+state.Direction && exception.Scope.Kind == state.Kind && exception.Scope.Identity == state.CandidateID && exception.Scope.DeltaSHA256 == StateDeltaDigest(*state) {
			state.ExceptedBy = exception.ID
		}
	}
}

func applyPolicy(receipt *Receipt, profile Profile, candidate map[recordKey]Record, states map[string]StateRule) {
	policy := profile.Candidate.Policy
	for _, delta := range receipt.Additions {
		if delta.ExceptedBy != "" {
			continue
		}
		if policy.ForbidNewLegacy && delta.Kind == "legacy-case" {
			receipt.PolicyFailures = append(receipt.PolicyFailures, "new-legacy-forbidden:"+delta.CandidateID)
		}
		if policy.RequireContractForNewOrChangedTests && delta.Kind == "test-execution" && !hasReviewedContract(candidate[recordKey{delta.Kind, delta.CandidateID}], candidate) {
			receipt.PolicyFailures = append(receipt.PolicyFailures, "uncontracted-test:"+delta.CandidateID)
		}
	}
	for _, delta := range receipt.ContentChanges {
		if delta.ExceptedBy != "" {
			continue
		}
		if policy.RequireContractForNewOrChangedTests && delta.Kind == "test-execution" && !hasReviewedContract(candidate[recordKey{delta.Kind, delta.CandidateID}], candidate) {
			receipt.PolicyFailures = append(receipt.PolicyFailures, "uncontracted-changed-test:"+delta.CandidateID)
		}
		if delta.Kind == "behavior-contract" && !hasReview(candidate[recordKey{delta.Kind, delta.CandidateID}], candidate) {
			receipt.PolicyFailures = append(receipt.PolicyFailures, "unreviewed-contract-change:"+delta.CandidateID)
		}
	}
	for _, delta := range receipt.StateChanges {
		if delta.ExceptedBy != "" || !policy.TerminalStatesCannotRegress {
			continue
		}
		from, ok := states[delta.From]
		if ok && from.Terminal && delta.Direction == "regression" {
			receipt.PolicyFailures = append(receipt.PolicyFailures, "terminal-regression:"+delta.CandidateID)
		}
	}
	for _, delta := range receipt.StaleEvidence {
		if delta.ExceptedBy == "" {
			receipt.PolicyFailures = append(receipt.PolicyFailures, delta.Code+":"+delta.CandidateID)
		}
	}
	for _, delta := range receipt.BrokenReverseLinks {
		if delta.ExceptedBy == "" {
			receipt.PolicyFailures = append(receipt.PolicyFailures, delta.Code+":"+delta.CandidateID)
		}
	}
	if policy.UnresolvedDenominatorCannotGrow && receipt.CandidateDenominator.Unresolved > receipt.BaselineDenominator.Unresolved {
		receipt.PolicyFailures = append(receipt.PolicyFailures, "unresolved-denominator-grew")
	}
}

func hasReviewedContract(test Record, records map[recordKey]Record) bool {
	if !hasReview(test, records) {
		return false
	}
	for _, link := range test.Links {
		if link.Relation != "contract" || link.TargetKind != "behavior-contract" {
			continue
		}
		contract, ok := records[recordKey{link.TargetKind, link.TargetID}]
		if ok && contract.ContentSHA256 == link.TargetSHA256 && hasReview(contract, records) {
			return true
		}
	}
	return false
}

func hasReview(record Record, records map[recordKey]Record) bool {
	for _, link := range record.Links {
		if link.Relation != "review" || link.TargetKind != "review" {
			continue
		}
		review, ok := records[recordKey{link.TargetKind, link.TargetID}]
		if ok && review.ContentSHA256 == link.TargetSHA256 {
			return true
		}
	}
	return false
}

func normalizeReceipt(receipt *Receipt) {
	sortDeltas := func(values []Delta) {
		sort.Slice(values, func(i, j int) bool {
			a, b := values[i], values[j]
			return a.Kind < b.Kind || a.Kind == b.Kind && (a.BaselineID < b.BaselineID || a.BaselineID == b.BaselineID && (a.CandidateID < b.CandidateID || a.CandidateID == b.CandidateID && a.Code < b.Code))
		})
	}
	for _, values := range [][]Delta{receipt.Additions, receipt.Removals, receipt.ContentChanges, receipt.StaleEvidence, receipt.BrokenReverseLinks, receipt.Unknowns} {
		sortDeltas(values)
	}
	sort.Slice(receipt.StateChanges, func(i, j int) bool {
		a, b := receipt.StateChanges[i], receipt.StateChanges[j]
		return a.Kind < b.Kind || a.Kind == b.Kind && (a.BaselineID < b.BaselineID || a.BaselineID == b.BaselineID && a.CandidateID < b.CandidateID)
	})
	sort.Slice(receipt.PolicyFailures, func(i, j int) bool { return receipt.PolicyFailures[i] < receipt.PolicyFailures[j] })
	receipt.PolicyFailures = slices.Compact(receipt.PolicyFailures)
	sort.Slice(receipt.Identities, func(i, j int) bool {
		a, b := receipt.Identities[i], receipt.Identities[j]
		return a.Kind < b.Kind || a.Kind == b.Kind && (a.BaselineID < b.BaselineID || a.BaselineID == b.BaselineID && a.CandidateID < b.CandidateID)
	})
}

func binding(snapshot Snapshot) Binding {
	return Binding{Schema: snapshot.Schema, Repository: snapshot.Repository, PolicyID: snapshot.Policy.ID, PolicySHA256: snapshot.Policy.SHA256, Provider: snapshot.Provider, ArtifactSHA256: snapshot.ArtifactSHA256}
}

func denominator(records map[recordKey]Record, states map[string]StateRule) Denominator {
	result := Denominator{Total: len(records), ByKind: map[string]int{}, ByState: map[string]int{}}
	for _, record := range records {
		result.ByKind[record.Kind]++
		result.ByState[record.State]++
		if state, ok := states[record.State]; !ok || state.Unresolved {
			result.Unresolved++
		}
	}
	return result
}

func stateRules(policy Policy) map[string]StateRule {
	result := map[string]StateRule{}
	for _, state := range policy.States {
		result[state.Name] = state
	}
	return result
}

func sortedRecordKeys(records map[recordKey]Record) []recordKey {
	keys := make([]recordKey, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].kind < keys[j].kind || keys[i].kind == keys[j].kind && keys[i].id < keys[j].id
	})
	return keys
}

func snapshotHasIdentity(snapshot Snapshot, kind, id string) bool {
	for _, record := range snapshot.Records {
		if record.Kind == kind && record.ID == id {
			return true
		}
	}
	return false
}

func hasUnexceptedUnknown(unknowns []Delta) bool {
	for _, unknown := range unknowns {
		if unknown.ExceptedBy == "" {
			return true
		}
	}
	return false
}

func compareLinks(a, b Link) int {
	aKey := a.Relation + "\x00" + a.TargetKind + "\x00" + a.TargetID + "\x00" + a.TargetSHA256 + "\x00" + a.ReverseRelation
	bKey := b.Relation + "\x00" + b.TargetKind + "\x00" + b.TargetID + "\x00" + b.TargetSHA256 + "\x00" + b.ReverseRelation
	return strings.Compare(aKey, bKey)
}

func compareRecords(a, b Record) int {
	aKey := a.Kind + "\x00" + a.ID + "\x00" + a.SHA256
	bKey := b.Kind + "\x00" + b.ID + "\x00" + b.SHA256
	return strings.Compare(aKey, bKey)
}

func compareExceptions(a, b Exception) int {
	return strings.Compare(a.ID+"\x00"+a.SHA256, b.ID+"\x00"+b.SHA256)
}

func set(value string) map[string]bool {
	result := map[string]bool{}
	for _, item := range strings.Fields(value) {
		result[item] = true
	}
	return result
}

func strictStrings(values []string) bool {
	if len(values) == 0 || !slices.IsSorted(values) {
		return false
	}
	for i, value := range values {
		if !text(value) || i > 0 && values[i-1] == value {
			return false
		}
	}
	return true
}

func text(value string) bool {
	return value != "" && len(value) <= 1024 && strings.TrimSpace(value) == value
}
func digest(value string) bool {
	_, err := hex.DecodeString(value)
	return len(value) == 64 && err == nil && value == strings.ToLower(value)
}
func objectID(value string) bool {
	_, err := hex.DecodeString(value)
	return (len(value) == 40 || len(value) == 64) && err == nil && value == strings.ToLower(value)
}
