package tracerecordrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"reflect"
	"slices"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/trace"
)

const (
	AggregateOutcomeProfile  = "corvint-dogfood-aggregate-outcome/0"
	AggregateAdmissionPolicy = "corvint-contextindex-record-paths/0"
	AggregateReceiptLimit    = 4 << 20
	AggregateCandidateLimit  = trace.MaxAggregateCandidates
	AggregateAdmittedLimit   = trace.MaxAggregateAdmitted
)

// AggregateExpected comes from immutable source and the loaded enrollment, never
// from the receipt being checked. ALO-V0-007 requires all fields to match.
type AggregateExpected struct {
	ObjectFormat    string   `json:"objectFormat"`
	Base            string   `json:"base"`
	Target          string   `json:"target"`
	Tree            string   `json:"tree"`
	AdmissionPolicy string   `json:"admissionPolicy"`
	Task            string   `json:"task"`
	Verification    []string `json:"verification"`
	Outcome         string   `json:"outcome"`
}

// AggregateOutcome is nonlearning evidence. It has no store or trace identity.
type AggregateOutcome struct {
	AggregateExpected
	Profile         string   `json:"profile"`
	State           string   `json:"state"`
	Candidates      []string `json:"candidates"`
	Admitted        []string `json:"admitted"`
	CandidateCount  int      `json:"candidateCount"`
	AdmittedCount   int      `json:"admittedCount"`
	CandidateSHA256 string   `json:"candidateSha256"`
	AdmittedSHA256  string   `json:"admittedSha256"`
	EnvelopeSHA256  string   `json:"envelopeSha256"`
}

var aggregateMembers = []string{
	"admissionPolicy", "admitted", "admittedCount", "admittedSha256", "base",
	"candidateCount", "candidateSha256", "candidates", "envelopeSha256",
	"objectFormat", "outcome", "profile", "state", "target", "task", "tree", "verification",
}

func aggregateCanonical(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	parsed, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	return wire.CanonicalValue(parsed), nil
}

func aggregateHash(domain string, raw []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte(domain))
	_, _ = h.Write(raw)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func aggregateEnvelope(value AggregateOutcome) (string, error) {
	raw, err := aggregateCanonical(value)
	if err != nil {
		return "", err
	}
	parsed, err := wire.Parse(raw)
	if err != nil {
		return "", err
	}
	delete(parsed.Obj.Values, "envelopeSha256")
	keys := parsed.Obj.Keys[:0]
	for _, key := range parsed.Obj.Keys {
		if key != "envelopeSha256" {
			keys = append(keys, key)
		}
	}
	parsed.Obj.Keys = keys
	return aggregateHash("corvint-dogfood-aggregate-outcome/v0\x00", wire.CanonicalValue(parsed)), nil
}

// AggregateBinding identifies the logical observation, independently of history
// snapshots, publication progress, executable paths and wall-clock time.
func AggregateBinding(value AggregateOutcome) (string, error) {
	payload := struct {
		AggregateExpected
		CandidateSHA256 string `json:"candidateSha256"`
		AdmittedSHA256  string `json:"admittedSha256"`
		CandidateCount  int    `json:"candidateCount"`
		AdmittedCount   int    `json:"admittedCount"`
	}{value.AggregateExpected, value.CandidateSHA256, value.AdmittedSHA256, value.CandidateCount, value.AdmittedCount}
	raw, err := aggregateCanonical(payload)
	if err != nil {
		return "", err
	}
	return aggregateHash("corvint-dogfood-aggregate-binding/v0\x00", raw), nil
}

func aggregateExpectedValid(expected AggregateExpected) error {
	if expected.ObjectFormat != "sha1" {
		return errors.New("aggregate-object-format-unsupported")
	}
	if !lowerHex(expected.Base, 40) || !lowerHex(expected.Target, 40) || !lowerHex(expected.Tree, 40) || expected.AdmissionPolicy != AggregateAdmissionPolicy {
		return errors.New("aggregate-binding-drift")
	}
	// This pure constructor reuses exactly the current secret/text/command grammar;
	// it opens no store. Empty changed paths avoid borrowing a larger trace limit.
	normalized, err := trace.NewRecord(trace.Input{Producer: trace.ProducerDogfood, Revision: expected.Target, TreeRevision: expected.Tree,
		Task: expected.Task, Verification: expected.Verification, Outcome: expected.Outcome}, nil)
	if err != nil {
		return err
	}
	if expected.Task != normalized.Task {
		return schemaInvalid("task", expected.Task)
	}
	if len(expected.Verification) == 0 {
		return schemaInvalid("verification", "")
	}
	if !reflect.DeepEqual(expected.Verification, normalized.Verification) {
		return schemaInvalid("verification", firstUnnormalized(expected.Verification, normalized.Verification))
	}
	return nil
}

// firstUnnormalized is the first verify command the recorder's normal form
// changes, or "" when the lists differ only in length.
func firstUnnormalized(got, normalized []string) string {
	for i, command := range got {
		if i >= len(normalized) || command != normalized[i] {
			return command
		}
	}
	return ""
}

func aggregatePathsValid(candidates, admitted []string) error {
	if len(candidates) > AggregateCandidateLimit {
		return errors.New("aggregate-candidate-limit")
	}
	if len(admitted) > AggregateAdmittedLimit {
		return errors.New("aggregate-admitted-limit")
	}
	if len(admitted) <= trace.MaxTracePaths {
		return errors.New("aggregate-not-required")
	}
	if len(candidates) == 0 || !sort.StringsAreSorted(candidates) {
		return schemaInvalid("candidates", "")
	}
	if !sort.StringsAreSorted(admitted) {
		return schemaInvalid("admitted", "")
	}
	for _, set := range []struct {
		field string
		paths []string
	}{{"candidates", candidates}, {"admitted", admitted}} {
		for i, path := range set.paths {
			if path == "" || i > 0 && path == set.paths[i-1] {
				return schemaInvalid(set.field, path)
			}
		}
	}
	// Complete source authority is separately reacquired by the worker. This pure
	// shape check still applies the actual path classifier and requires a subset.
	classified, err := trace.AdmissibleAggregateCurrentPaths(candidates, admitted)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(classified, admitted) {
		return errors.New("aggregate-source-set-drift")
	}
	return nil
}

func newAggregateOutcome(expected AggregateExpected, candidates, admitted []string) (AggregateOutcome, error) {
	if err := aggregateExpectedValid(expected); err != nil {
		return AggregateOutcome{}, err
	}
	if err := aggregatePathsValid(candidates, admitted); err != nil {
		return AggregateOutcome{}, err
	}
	expected.Verification = append([]string(nil), expected.Verification...)
	value := AggregateOutcome{AggregateExpected: expected, Profile: AggregateOutcomeProfile, State: "aggregate-recorded",
		Candidates: append([]string(nil), candidates...), Admitted: append([]string(nil), admitted...),
		CandidateCount: len(candidates), AdmittedCount: len(admitted), CandidateSHA256: digestPaths(candidates), AdmittedSHA256: digestPaths(admitted)}
	var err error
	value.EnvelopeSHA256, err = aggregateEnvelope(value)
	return value, err
}

func encodeAggregateOutcome(value AggregateOutcome) ([]byte, error) {
	raw, err := aggregateCanonical(value)
	if err != nil {
		return nil, err
	}
	if len(raw)+1 > AggregateReceiptLimit {
		return nil, errors.New("aggregate-receipt-byte-limit")
	}
	return append(raw, '\n'), nil
}

// ParseAggregateOutcome checks the closed canonical wire and its internal
// identities. Callers still need VerifyAggregateOutcome against fresh authority.
func ParseAggregateOutcome(raw []byte) (AggregateOutcome, error) {
	if len(raw) > AggregateReceiptLimit {
		return AggregateOutcome{}, errors.New("aggregate-receipt-byte-limit")
	}
	parsed, err := wire.Parse(raw)
	if err != nil || parsed.Kind != wire.KindObject {
		return AggregateOutcome{}, schemaInvalid("receipt", "")
	}
	keys := append([]string(nil), parsed.Obj.Keys...)
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, aggregateMembers) || !bytes.Equal(raw, append(wire.CanonicalValue(parsed), '\n')) {
		return AggregateOutcome{}, schemaInvalid("receipt", "")
	}
	var value AggregateOutcome
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return AggregateOutcome{}, schemaInvalid("receipt", "")
	}
	if value.Profile != AggregateOutcomeProfile {
		return AggregateOutcome{}, errors.New("aggregate-profile-unsupported")
	}
	if value.State != "aggregate-recorded" {
		return AggregateOutcome{}, schemaInvalid("receipt", "")
	}
	expected, err := newAggregateOutcome(value.AggregateExpected, value.Candidates, value.Admitted)
	if err != nil {
		return AggregateOutcome{}, err
	}
	want, err := encodeAggregateOutcome(expected)
	if err != nil {
		return AggregateOutcome{}, err
	}
	if !bytes.Equal(raw, want) {
		return AggregateOutcome{}, schemaInvalid("receipt", "")
	}
	return value, nil
}

// acquireAggregate performs complete current-policy acquisition, including an
// actual pure legacy refusal. It never calls recordWithIndex or opens a trace store.
func acquireAggregate(ctx context.Context, root string, expected AggregateExpected) (AggregateOutcome, error) {
	if err := aggregateExpectedValid(expected); err != nil {
		return AggregateOutcome{}, err
	}
	index, tracked, err := recordIndex(ctx, root)
	if err != nil {
		return AggregateOutcome{}, err
	}
	if index.ObjectFormat != expected.ObjectFormat || index.CommitRevision != expected.Target || index.Revision != expected.Tree {
		return AggregateOutcome{}, errors.New("aggregate-binding-drift")
	}
	base, err := resolveCommit(ctx, root, index.ObjectFormat, expected.Base)
	if err != nil {
		return AggregateOutcome{}, err
	}
	if base != expected.Base {
		return AggregateOutcome{}, errors.New("aggregate-binding-drift")
	}
	candidates, err := aggregateChangedPaths(ctx, root, expected.Base, expected.Target)
	if err != nil {
		return AggregateOutcome{}, err
	}
	_, legacyErr := trace.AdmissibleCurrentPaths(candidates, tracked)
	if legacyErr == nil {
		return AggregateOutcome{}, errors.New("aggregate-not-required")
	}
	if trace.AdmissionFailureReason(legacyErr) != "admitted-path-limit" {
		return AggregateOutcome{}, legacyErr
	}
	admitted, err := trace.AdmissibleAggregateCurrentPaths(candidates, tracked)
	if err != nil {
		return AggregateOutcome{}, err
	}
	return newAggregateOutcome(expected, candidates, admitted)
}

// AggregateExpectedFor resolves BASE and TARGET to full SHA-1 commit identities
// and TARGET's tree, for the dogfood-record opt-in aggregate (DCW-V0-374). The
// author's task, verification and outcome pass through aggregateExpectedValid.
// The verification list is put in the receipt's sorted unique byte order first,
// as the learning trace recorder normalizes the same VERIFY file, so any line
// order records the same receipt (DCW-V0-445); the receipt itself stays strict.
func AggregateExpectedFor(ctx context.Context, root, base, target, task string, verification []string, outcome string) (AggregateExpected, error) {
	sorted := append([]string(nil), verification...)
	sort.Strings(sorted)
	expected := AggregateExpected{ObjectFormat: "sha1", AdmissionPolicy: AggregateAdmissionPolicy, Task: task,
		Verification: slices.Compact(sorted), Outcome: outcome}
	var err error
	if expected.Base, err = resolveCommit(ctx, root, "sha1", base); err != nil {
		return AggregateExpected{}, err
	}
	if expected.Target, err = resolveCommit(ctx, root, "sha1", target); err != nil {
		return AggregateExpected{}, err
	}
	raw, err := runGit(ctx, gitrun.NewDefaultBudget(), root, 256, "rev-parse", "--verify", expected.Target+"^{tree}")
	if err != nil {
		return AggregateExpected{}, err
	}
	expected.Tree = string(bytes.TrimSuffix(raw, []byte{'\n'}))
	return expected, aggregateExpectedValid(expected)
}

// ProduceAggregateOutcome is the read-only worker operation. Both complete index
// acquisitions and diff reads must agree under the caller's single operation budget.
func ProduceAggregateOutcome(ctx context.Context, root string, expected AggregateExpected) ([]byte, error) {
	first, err := acquireAggregate(ctx, root, expected)
	if err != nil {
		return nil, err
	}
	second, err := acquireAggregate(ctx, root, expected)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(first, second) {
		return nil, errors.New("aggregate-source-set-drift")
	}
	return encodeAggregateOutcome(second)
}

// VerifyAggregateOutcome compares real expected enrollment fields and newly
// acquired complete arrays. A self-consistent arbitrary receipt cannot satisfy it.
func VerifyAggregateOutcome(ctx context.Context, root string, expected AggregateExpected, raw []byte) (AggregateOutcome, error) {
	value, err := ParseAggregateOutcome(raw)
	if err != nil {
		return AggregateOutcome{}, err
	}
	if !reflect.DeepEqual(value.AggregateExpected, expected) {
		return AggregateOutcome{}, errors.New("aggregate-binding-drift")
	}
	fresh, err := ProduceAggregateOutcome(ctx, root, expected)
	if err != nil {
		return AggregateOutcome{}, err
	}
	if !bytes.Equal(raw, fresh) {
		return AggregateOutcome{}, errors.New("aggregate-source-set-drift")
	}
	return value, nil
}

// Acquire at most the aggregate limit plus one sentinel byte. A large diff is
// never first buffered under the legacy recorder's larger tree allowance.
func aggregateChangedPaths(ctx context.Context, root, base, target string) ([]string, error) {
	raw, err := runGit(ctx, gitrun.NewDefaultBudget(), root, AggregateReceiptLimit+1,
		"diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--no-textconv",
		"--ignore-submodules=none", base, target, "--", ".")
	if err != nil {
		var overflow *gitrun.OperationOutputFailure
		if errors.As(err, &overflow) && overflow.Stream == "stdout" {
			return nil, errors.New("aggregate-receipt-byte-limit")
		}
		return nil, err
	}
	if len(raw) > AggregateReceiptLimit {
		return nil, errors.New("aggregate-receipt-byte-limit")
	}
	return parseChangedPaths(raw, AggregateCandidateLimit, "aggregate-candidate-limit")
}
