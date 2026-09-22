package adapters

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/dashboard/authority"
	"github.com/Beamfall/corvint/internal/dashboard/model"
	"github.com/Beamfall/corvint/internal/dashboard/source"
)

const (
	maxTraceRows         = 1_000
	maxTraceRowBytes     = 256 * 1_024
	maxTraceStoreBytes   = 16 * 1_024 * 1_024
	maxTracePaths        = 200
	maxTracePathBytes    = 4_096
	maxVerificationItems = 50
	maxCommandRunes      = 512
	maxTaskRunes         = 2_000
)

type AdapterIssueCode string

const (
	IssueVerifierRejected            AdapterIssueCode = "VERIFIER_REJECTED"
	IssueSourceInvalidSchema         AdapterIssueCode = "SOURCE_INVALID_SCHEMA"
	IssueSourceInvalidIdentity       AdapterIssueCode = "SOURCE_INVALID_IDENTITY"
	IssueRepositoryObjectUnavailable AdapterIssueCode = "REPOSITORY_OBJECT_UNAVAILABLE"
	IssueTraceAncestryBound          AdapterIssueCode = "TRACE_ANCESTRY_BOUND"
	IssueTraceStoreBound             AdapterIssueCode = "TRACE_STORE_BOUND"
)

const (
	AuthorityQualified       = authority.TraceQualified
	AuthorityUnavailable     = authority.TraceObjectUnavailable
	AuthorityAncestryBound   = authority.TraceAncestryBound
	AuthorityPathUntracked   = authority.TracePathUntracked
	AuthorityInvalidIdentity = authority.TraceInvalidIdentity
	AuthorityInterrupted     = authority.TraceInterrupted
)

type AuthorityCode = authority.TraceCode
type TraceAuthorityResult = authority.TraceQualification
type TracePathWitness = authority.PathWitness
type TraceAuthority = authority.TraceQualifier

type TraceOutcomeCount struct {
	Outcome string
	Count   uint64
}

// TraceSummary is the only trace-derived adapter output. Raw trace IDs, task
// text, paths, commands, prompts, and errors are deliberately unrepresentable.
type TraceSummary struct {
	Revision            string
	TreeRevision        string
	ObjectFormat        string
	RetainedRows        uint64
	Outcomes            []TraceOutcomeCount
	repositoryWitnesses []model.RepositoryWitness
	traceIDs            []string
}

// ValidateStableTraceContent consumes immutable bytes during source.Read's
// StableConsumer callback. StableContent must not escape that callback; the
// caller retains only this normalized summary and the eventual source.Result.
func ValidateStableTraceContent(ctx context.Context, content source.StableContent, expectedRevision string, authority TraceAuthority) (TraceSummary, AdapterIssueCode, *ScanError) {
	summary, code, _, failure := validateStableTraceContentDetailed(ctx, content, expectedRevision, authority)
	return summary, code, failure
}

func validateStableTraceContentDetailed(ctx context.Context, content source.StableContent, expectedRevision string, authority TraceAuthority) (TraceSummary, AdapterIssueCode, uint64, *ScanError) {
	if content.Len() > maxTraceStoreBytes || !validSHA256Digest(content.SHA256()) {
		return TraceSummary{}, IssueSourceInvalidIdentity, 0, nil
	}
	body, err := io.ReadAll(io.LimitReader(content.Reader(), maxTraceStoreBytes+1))
	if err != nil || uint64(len(body)) != content.Len() {
		return TraceSummary{}, IssueSourceInvalidIdentity, 0, nil
	}
	return validateTraceArtifactDetailed(ctx, body, expectedRevision, authority)
}

// ValidateTraceArtifact verifies exactly one revision-named local trace
// artifact. The expected revision comes from the qualified filename grammar,
// never from a default or current-path discovery.
func ValidateTraceArtifact(ctx context.Context, body []byte, expectedRevision string, authority TraceAuthority) (TraceSummary, AdapterIssueCode, *ScanError) {
	summary, code, _, failure := validateTraceArtifactDetailed(ctx, body, expectedRevision, authority)
	return summary, code, failure
}

func validateTraceArtifactDetailed(ctx context.Context, body []byte, expectedRevision string, authority TraceAuthority) (TraceSummary, AdapterIssueCode, uint64, *ScanError) {
	if !validAnyObjectID(expectedRevision) || authority == nil {
		return TraceSummary{}, IssueSourceInvalidIdentity, 0, nil
	}
	summary, paths, code, limit := preflightTraceArtifact(body, expectedRevision)
	if code != "" {
		return TraceSummary{}, code, limit, nil
	}
	return qualifyTraceSummary(ctx, summary, paths, authority)
}

func preflightStableTraceContent(content source.StableContent, expectedRevision string) (TraceSummary, []string, AdapterIssueCode, uint64) {
	if content.Len() > maxTraceStoreBytes || !validSHA256Digest(content.SHA256()) {
		return TraceSummary{}, nil, IssueSourceInvalidIdentity, 0
	}
	body, err := io.ReadAll(io.LimitReader(content.Reader(), maxTraceStoreBytes+1))
	if err != nil || uint64(len(body)) != content.Len() {
		return TraceSummary{}, nil, IssueSourceInvalidIdentity, 0
	}
	return preflightTraceArtifact(body, expectedRevision)
}

func preflightTraceArtifact(body []byte, expectedRevision string) (TraceSummary, []string, AdapterIssueCode, uint64) {
	if len(body) > maxTraceStoreBytes {
		return TraceSummary{}, nil, IssueTraceStoreBound, maxTraceStoreBytes
	}
	if !validAnyObjectID(expectedRevision) {
		return TraceSummary{}, nil, IssueSourceInvalidIdentity, 0
	}
	rawRowCount := uint64(len(splitTraceLines(body)))
	rows, code, limit := parseTraceRows(body, expectedRevision)
	if code != "" {
		return TraceSummary{Revision: expectedRevision, RetainedRows: rawRowCount}, nil, code, limit
	}
	paths := make(map[string]struct{})
	counts := map[string]uint64{"blocked": 0, "failed": 0, "passed": 0}
	for _, row := range rows {
		for _, value := range row.openedPaths {
			paths[value] = struct{}{}
		}
		for _, value := range row.changedPaths {
			paths[value] = struct{}{}
		}
		counts[row.outcome]++
	}
	pathList := make([]string, 0, len(paths))
	for value := range paths {
		pathList = append(pathList, value)
	}
	sort.Strings(pathList)
	outcomes := make([]TraceOutcomeCount, 0, 3)
	traceIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		traceIDs = append(traceIDs, row.traceID)
	}
	sort.Strings(traceIDs)
	for _, outcome := range []string{"blocked", "failed", "passed"} {
		if count := counts[outcome]; count != 0 {
			outcomes = append(outcomes, TraceOutcomeCount{Outcome: outcome, Count: count})
		}
	}
	return TraceSummary{
		Revision: expectedRevision, RetainedRows: uint64(len(rows)), Outcomes: outcomes,
		traceIDs: traceIDs,
	}, pathList, "", 0
}

func qualifyTraceSummary(ctx context.Context, summary TraceSummary, paths []string, traceAuthority TraceAuthority) (TraceSummary, AdapterIssueCode, uint64, *ScanError) {
	if traceAuthority == nil || !validAnyObjectID(summary.Revision) {
		return TraceSummary{}, IssueSourceInvalidIdentity, 0, nil
	}
	qualified := traceAuthority.QualifyTrace(ctx, summary.Revision, append([]string(nil), paths...))
	if qualified.Code == AuthorityInterrupted {
		return TraceSummary{}, "", 0, &ScanError{Code: "DASHBOARD_INTERRUPTED"}
	}
	witnesses, code := validateAuthority(summary.Revision, paths, qualified)
	if code != "" {
		return TraceSummary{}, code, 0, nil
	}
	summary.TreeRevision = qualified.TreeRevision
	summary.ObjectFormat = qualified.ObjectFormat
	summary.repositoryWitnesses = witnesses
	return summary, "", 0, nil
}

func validateAuthority(revision string, paths []string, result TraceAuthorityResult) ([]model.RepositoryWitness, AdapterIssueCode) {
	switch result.Code {
	case AuthorityUnavailable:
		return nil, IssueRepositoryObjectUnavailable
	case AuthorityAncestryBound:
		return nil, IssueTraceAncestryBound
	case AuthorityPathUntracked:
		return nil, IssueVerifierRejected
	case AuthorityInvalidIdentity:
		return nil, IssueSourceInvalidIdentity
	case AuthorityQualified:
	default:
		return nil, IssueVerifierRejected
	}
	if !validObjectID(revision, result.ObjectFormat) || !validObjectID(result.TreeRevision, result.ObjectFormat) {
		return nil, IssueSourceInvalidIdentity
	}
	witnesses, ok := normalizeAuthorityWitnesses(result.Witnesses)
	if !ok {
		return nil, IssueSourceInvalidIdentity
	}
	var headCount, revisionCount int
	pathObjects := make(map[string]struct{})
	for _, witness := range witnesses {
		if witness.ObjectFormat != result.ObjectFormat {
			return nil, IssueSourceInvalidIdentity
		}
		switch witness.Kind {
		case "SNAPSHOT_HEAD":
			headCount++
		case "TRACE_REVISION":
			if witness.Revision == nil || witness.ObjectID != revision || *witness.Revision != revision {
				return nil, IssueSourceInvalidIdentity
			}
			revisionCount++
		case "TRACE_PATH_OBJECT":
			if witness.Revision == nil || *witness.Revision != revision {
				return nil, IssueSourceInvalidIdentity
			}
			pathObjects[witnessKey(witness)] = struct{}{}
		}
	}
	if headCount != 1 || revisionCount != 1 || len(result.PathWitnesses) != len(paths) {
		return nil, IssueSourceInvalidIdentity
	}
	associated := make(map[string]struct{}, len(pathObjects))
	for index, association := range result.PathWitnesses {
		if association.Path != paths[index] || association.Witness.Kind != "TRACE_PATH_OBJECT" ||
			association.Witness.ObjectFormat != result.ObjectFormat || association.Witness.ObjectType != "blob" ||
			association.Witness.Revision == nil || *association.Witness.Revision != revision {
			return nil, IssueSourceInvalidIdentity
		}
		witness := modelWitness(association.Witness)
		key := witnessKey(witness)
		if _, exists := pathObjects[key]; !exists {
			return nil, IssueSourceInvalidIdentity
		}
		associated[key] = struct{}{}
	}
	if len(associated) != len(pathObjects) {
		return nil, IssueSourceInvalidIdentity
	}
	return witnesses, ""
}

func normalizeAuthorityWitnesses(input []authority.Witness) ([]model.RepositoryWitness, bool) {
	if input == nil {
		return nil, false
	}
	converted := make([]model.RepositoryWitness, len(input))
	for index, witness := range input {
		converted[index] = modelWitness(witness)
	}
	return normalizeWitnesses(converted)
}

func modelWitness(witness authority.Witness) model.RepositoryWitness {
	return model.RepositoryWitness{
		Kind: witness.Kind, ObjectFormat: witness.ObjectFormat, ObjectID: witness.ObjectID,
		ObjectType: witness.ObjectType, Revision: cloneOptional(witness.Revision),
	}
}

func normalizeWitnesses(input []model.RepositoryWitness) ([]model.RepositoryWitness, bool) {
	if input == nil {
		return nil, false
	}
	witnesses := make([]model.RepositoryWitness, len(input))
	for index, witness := range input {
		witnesses[index] = witness
		if witness.Revision != nil {
			revision := *witness.Revision
			witnesses[index].Revision = &revision
		}
	}
	if _, err := model.ComputeRepositoryReadsSHA256(witnesses); err != nil {
		return nil, false
	}
	sort.Slice(witnesses, func(left, right int) bool {
		leftJSON, leftErr := canonicalWitnessJSON(witnesses[left])
		rightJSON, rightErr := canonicalWitnessJSON(witnesses[right])
		return leftErr == nil && rightErr == nil && bytes.Compare(leftJSON, rightJSON) < 0
	})
	normalized := witnesses[:0]
	previous := ""
	for index, witness := range witnesses {
		encoded, err := canonicalWitnessJSON(witness)
		if err != nil {
			return nil, false
		}
		key := string(encoded)
		if index == 0 || key != previous {
			normalized = append(normalized, witness)
			previous = key
		}
	}
	return normalized, true
}

func canonicalWitnessJSON(witness model.RepositoryWitness) ([]byte, error) {
	var revision any
	if witness.Revision != nil {
		revision = *witness.Revision
	}
	return contextindex.CanonicalJSON(map[string]any{
		"kind": witness.Kind, "objectFormat": witness.ObjectFormat,
		"objectId": witness.ObjectID, "objectType": witness.ObjectType,
		"revision": revision,
	})
}

type traceRow struct {
	openedPaths  []string
	changedPaths []string
	outcome      string
	traceID      string
}

type rawMembers map[string]jsontext.Value

var traceFields = map[string]struct{}{
	"schema_version": {}, "revision": {}, "trace_id": {}, "task": {},
	"opened_paths": {}, "changed_paths": {}, "verification": {}, "outcome": {},
}

func parseTraceRows(body []byte, revision string) ([]traceRow, AdapterIssueCode, uint64) {
	lines := splitTraceLines(body)
	if len(lines) > maxTraceRows {
		return nil, IssueTraceStoreBound, maxTraceRows
	}
	for _, line := range lines {
		if len(line.raw) > maxTraceRowBytes {
			return nil, IssueTraceStoreBound, maxTraceRowBytes
		}
	}
	if !utf8.Valid(body) {
		return nil, IssueSourceInvalidSchema, 0
	}
	rows := make([]traceRow, 0, len(lines))
	ids := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		row, code := parseTraceRow(line.value, revision)
		if code != "" {
			return nil, code, 0
		}
		if _, duplicate := ids[row.traceID]; duplicate {
			return nil, IssueVerifierRejected, 0
		}
		ids[row.traceID] = struct{}{}
		rows = append(rows, row)
	}
	return rows, "", 0
}

type traceLine struct {
	raw   []byte
	value []byte
}

func splitTraceLines(body []byte) []traceLine {
	if len(body) == 0 {
		return nil
	}
	capacity := min(maxTraceRows+1, 1+bytes.Count(body, []byte{'\n'}))
	result := make([]traceLine, 0, capacity)
	for len(body) != 0 {
		end := len(body)
		if index := bytes.IndexByte(body, '\n'); index >= 0 {
			end = index + 1
		}
		raw := body[:end]
		value := raw
		for len(value) != 0 && (value[len(value)-1] == '\r' || value[len(value)-1] == '\n') {
			value = value[:len(value)-1]
		}
		result = append(result, traceLine{raw: raw, value: value})
		if len(result) > maxTraceRows {
			break
		}
		body = body[end:]
	}
	return result
}

func parseTraceRow(data []byte, revision string) (traceRow, AdapterIssueCode) {
	if len(data) < 2 || data[0] != '{' || data[len(data)-1] != '}' {
		return traceRow{}, IssueSourceInvalidSchema
	}
	var raw rawMembers
	if err := json.Unmarshal(data, &raw); err != nil {
		if errors.Is(err, jsontext.ErrDuplicateName) {
			return traceRow{}, IssueSourceInvalidSchema
		}
		return traceRow{}, IssueSourceInvalidSchema
	}
	if len(raw) != len(traceFields) {
		return traceRow{}, IssueSourceInvalidSchema
	}
	for name := range raw {
		if _, ok := traceFields[name]; !ok {
			return traceRow{}, IssueSourceInvalidSchema
		}
	}
	if string(raw["schema_version"]) != "1" {
		return traceRow{}, IssueSourceInvalidSchema
	}
	rowRevision, ok := strictString(raw["revision"])
	if !ok || rowRevision != revision {
		return traceRow{}, IssueSourceInvalidIdentity
	}
	task, ok := strictString(raw["task"])
	if !ok {
		return traceRow{}, IssueSourceInvalidSchema
	}
	task = strings.TrimSpace(task)
	if task == "" || utf8.RuneCountInString(task) > maxTaskRunes || secretPattern.MatchString(task) {
		return traceRow{}, IssueVerifierRejected
	}
	opened, ok := normalizedPaths(raw["opened_paths"])
	if !ok {
		return traceRow{}, IssueVerifierRejected
	}
	changed, ok := normalizedPaths(raw["changed_paths"])
	if !ok {
		return traceRow{}, IssueVerifierRejected
	}
	verification, ok := normalizedCommands(raw["verification"])
	if !ok {
		return traceRow{}, IssueVerifierRejected
	}
	outcome, ok := strictString(raw["outcome"])
	if !ok || (outcome != "passed" && outcome != "failed" && outcome != "blocked") {
		return traceRow{}, IssueVerifierRejected
	}
	traceID, ok := strictString(raw["trace_id"])
	if !ok || !lowerHex(traceID, 64) {
		return traceRow{}, IssueSourceInvalidIdentity
	}
	normalized := map[string]any{
		"schema_version": 1, "revision": revision, "task": task,
		"opened_paths": opened, "changed_paths": changed,
		"verification": verification, "outcome": outcome,
	}
	encoded, err := contextindex.CanonicalJSON(normalized)
	if err != nil {
		return traceRow{}, IssueVerifierRejected
	}
	digest := sha256.Sum256(encoded)
	if hex.EncodeToString(digest[:]) != traceID {
		return traceRow{}, IssueSourceInvalidIdentity
	}
	return traceRow{openedPaths: opened, changedPaths: changed, outcome: outcome, traceID: traceID}, ""
}

func strictString(raw jsontext.Value) (string, bool) {
	if len(raw) < 2 || raw[0] != '"' {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !utf8.ValidString(value) {
		return "", false
	}
	return value, true
}

func normalizedPaths(raw jsontext.Value) ([]string, bool) {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, false
	}
	values = sortedUnique(values)
	if len(values) > maxTracePaths {
		return nil, false
	}
	for _, value := range values {
		if value == "" || len(value) > maxTracePathBytes || !utf8.ValidString(value) || path.IsAbs(value) ||
			path.Clean(value) != value || strings.Contains(value, "\\") || hasVolumePrefix(value) ||
			containsControl(value) || secretPattern.MatchString(value) || contextindex.ForbiddenPathReason(value) != "" {
			return nil, false
		}
		for _, part := range strings.Split(value, "/") {
			if part == "" || part == "." || part == ".." {
				return nil, false
			}
		}
	}
	return values, true
}

func hasVolumePrefix(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func normalizedCommands(raw jsontext.Value) ([]string, bool) {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, false
	}
	values = sortedUnique(values)
	if len(values) > maxVerificationItems {
		return nil, false
	}
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > maxCommandRunes ||
			secretPattern.MatchString(value) || !safeCommandPattern.MatchString(value) {
			return nil, false
		}
		values[index] = value
	}
	return values, true
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func validAnyObjectID(value string) bool { return lowerHex(value, 40) || lowerHex(value, 64) }

func validObjectID(value, objectFormat string) bool {
	switch objectFormat {
	case "sha1":
		return lowerHex(value, 40)
	case "sha256":
		return lowerHex(value, 64)
	default:
		return false
	}
}

func lowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

var (
	secretPattern      = regexp.MustCompile(`(?i)[a-z0-9_.-]*(?:api[_-]?key|access[_-]?key|authorization|password|private[_-]?key|secret|token)[a-z0-9_.-]*\s*[:=]\s*\S+|-----BEGIN [A-Z ]*PRIVATE KEY-----|-----BEGIN PGP PRIV[A]TE KEY BLOCK-----|[a-z][a-z0-9+.-]*://[^\s/:]+:[^\s/@]+@|gh[pousr]_[a-z0-9]{20,}|github_pat_[a-z0-9_]{20,}|glpat-[a-z0-9_-]{20,}|xox[a-z]-[a-z0-9-]{10,}|(?:akia|asia)[a-z0-9]{16}|aiza[a-z0-9_-]{30,}|npm_[a-z0-9]{20,}|\bsk-[a-z0-9_-]{20,}|(?:sk|rk)_(?:live|test)_[a-z0-9]{16,}|pypi-ageichlwasi5vcmc[a-z0-9_-]{20,}|sg\.[a-z0-9_-]{16,}\.[a-z0-9_-]{32,}|sk[0-9a-f]{32}|bearer\s+[a-z0-9][a-z0-9_.~+/-]{15,}|[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}`)
	safeCommandPattern = regexp.MustCompile(`^[A-Za-z0-9_./:@=+, \-]+$`)
)
