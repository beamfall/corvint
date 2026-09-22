package trace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

const (
	SchemaVersion           = 1
	MaxTraces               = 1_000
	MaxTraceFiles           = 1_000
	MaxTraceRowBytes        = 256 * 1_024
	MaxTraceStoreBytes      = 16 * 1_024 * 1_024
	MaxTracePaths           = 200
	MaxAdmissionCandidates  = 200_000
	MaxVerificationCommands = 50
	MaxCommandCharacters    = 512
	MaxTaskCharacters       = 2_000
)

// Record is the schema-version 1 JSONL record shared with the Python runtime.
// Slices are always normalized to non-nil values by constructors and decoders.
type Record struct {
	SchemaVersion int
	Revision      string
	TraceID       string
	Task          string
	OpenedPaths   []string
	ChangedPaths  []string
	Verification  []string
	Outcome       string
}

// Input contains the caller-controlled fields of a new trace record.
type Input struct {
	Revision     string
	TreeRevision string
	Task         string
	OpenedPaths  []string
	ChangedPaths []string
	Verification []string
	Outcome      string
}

// NewRecord applies the Python writer's normalization and computes TraceID.
func NewRecord(input Input, trackedPaths []string) (Record, error) {
	tracked := stringSet(trackedPaths)
	task, err := validTask(safeText(input.Task, "trace task", MaxTaskCharacters))
	if err != nil {
		return Record{}, err
	}
	opened, err := normalizeCurrentPaths(input.OpenedPaths, "opened_paths", input.TreeRevision, tracked)
	if err != nil {
		return Record{}, err
	}
	changed, err := normalizeCurrentPaths(input.ChangedPaths, "changed_paths", input.TreeRevision, tracked)
	if err != nil {
		return Record{}, err
	}
	commands, err := normalizeCommands(input.Verification)
	if err != nil {
		return Record{}, err
	}
	if !validOutcome(input.Outcome) {
		return Record{}, fmt.Errorf("trace outcome must be one of: blocked, failed, passed")
	}
	record := Record{
		SchemaVersion: SchemaVersion,
		Revision:      input.Revision,
		Task:          task,
		OpenedPaths:   opened,
		ChangedPaths:  changed,
		Verification:  commands,
		Outcome:       input.Outcome,
	}
	record.TraceID, err = traceID(record)
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

func normalizeStoredRecord(record Record, expectedRevision string, tracked map[string]struct{}) (Record, error) {
	if record.SchemaVersion != SchemaVersion {
		return Record{}, fmt.Errorf("unsupported local trace schema")
	}
	if record.Revision != expectedRevision {
		return Record{}, fmt.Errorf("local trace contains mixed revision: %s != %s", record.Revision, expectedRevision)
	}
	if !validOutcome(record.Outcome) {
		return Record{}, fmt.Errorf("trace outcome must be one of: blocked, failed, passed")
	}
	task, err := validTask(safeStoredText(record.Task, "trace task", MaxTaskCharacters))
	if err != nil {
		return Record{}, err
	}
	opened, err := normalizeStoredHistoricalPaths(record.OpenedPaths, "opened_paths", expectedRevision, tracked)
	if err != nil {
		return Record{}, err
	}
	changed, err := normalizeStoredHistoricalPaths(record.ChangedPaths, "changed_paths", expectedRevision, tracked)
	if err != nil {
		return Record{}, err
	}
	commands, err := normalizeStoredCommands(record.Verification)
	if err != nil {
		return Record{}, err
	}
	normalized := Record{
		SchemaVersion: SchemaVersion,
		Revision:      expectedRevision,
		Task:          task,
		OpenedPaths:   opened,
		ChangedPaths:  changed,
		Verification:  commands,
		Outcome:       record.Outcome,
	}
	expected, err := traceID(normalized)
	if err != nil {
		return Record{}, err
	}
	if record.TraceID != expected {
		return Record{}, fmt.Errorf("local trace digest mismatch")
	}
	normalized.TraceID = expected
	return normalized, nil
}

func validateUnreachableRecord(record Record, expectedRevision string) error {
	if record.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported local trace schema")
	}
	if record.Revision != expectedRevision {
		return fmt.Errorf("local trace contains mixed revision: %s != %s", record.Revision, expectedRevision)
	}
	if !validOutcome(record.Outcome) {
		return fmt.Errorf("trace outcome must be one of: blocked, failed, passed")
	}
	task, err := validTask(safeStoredText(record.Task, "trace task", MaxTaskCharacters))
	if err != nil {
		return err
	}
	if task != record.Task {
		return fmt.Errorf("trace task is not normalized")
	}
	opened, err := normalizeStoredPaths(record.OpenedPaths, "opened_paths")
	if err != nil {
		return err
	}
	if !equalStrings(opened, record.OpenedPaths) {
		return fmt.Errorf("opened_paths are not normalized")
	}
	changed, err := normalizeStoredPaths(record.ChangedPaths, "changed_paths")
	if err != nil {
		return err
	}
	if !equalStrings(changed, record.ChangedPaths) {
		return fmt.Errorf("changed_paths are not normalized")
	}
	commands, err := normalizeStoredCommands(record.Verification)
	if err != nil {
		return err
	}
	if !equalStrings(commands, record.Verification) {
		return fmt.Errorf("verification commands are not normalized")
	}
	expected, err := traceID(record)
	if err != nil {
		return err
	}
	if record.TraceID != expected {
		return fmt.Errorf("local trace digest mismatch")
	}
	return nil
}

func validateAppendRecord(record Record, tracked map[string]struct{}) error {
	if record.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported local trace schema")
	}
	if !validOutcome(record.Outcome) {
		return fmt.Errorf("trace outcome must be one of: blocked, failed, passed")
	}
	task, err := validTask(safeText(record.Task, "trace task", MaxTaskCharacters))
	if err != nil {
		return err
	}
	if task != record.Task {
		return fmt.Errorf("trace task is not normalized")
	}
	opened, err := normalizeHistoricalPaths(record.OpenedPaths, "opened_paths", record.Revision, tracked)
	if err != nil {
		return err
	}
	if !equalStrings(opened, record.OpenedPaths) {
		return fmt.Errorf("opened_paths are not normalized")
	}
	changed, err := normalizeHistoricalPaths(record.ChangedPaths, "changed_paths", record.Revision, tracked)
	if err != nil {
		return err
	}
	if !equalStrings(changed, record.ChangedPaths) {
		return fmt.Errorf("changed_paths are not normalized")
	}
	if len(record.Verification) > MaxVerificationCommands {
		return fmt.Errorf("verification exceeds %d commands", MaxVerificationCommands)
	}
	for _, command := range record.Verification {
		cleaned, err := safeText(command, "verification command", MaxCommandCharacters)
		if err != nil {
			return err
		}
		if cleaned != command {
			return fmt.Errorf("verification command is not normalized")
		}
		if err := safeCommand(command); err != nil {
			return verificationFailure("unsupported-verify-syntax", err)
		}
	}
	expected, err := traceID(record)
	if err != nil {
		return err
	}
	if record.TraceID != expected {
		return fmt.Errorf("local trace digest mismatch")
	}
	return nil
}

func traceID(record Record) (string, error) {
	encoded, err := canonicalRecord(record, false)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func semanticID(record Record) (string, error) {
	record.Revision = ""
	record.TraceID = ""
	encoded, err := canonicalSemantic(record)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func safeText(value, label string, maximum int) (string, error) {
	return safeTextWithMatcher(value, label, maximum, secretscreen.MatchString)
}

func safeStoredText(value, label string, maximum int) (string, error) {
	return safeTextWithMatcher(value, label, maximum, secretscreen.MatchStoredV1String)
}

func safeTextWithMatcher(value, label string, maximum int, containsSecret func(string) bool) (string, error) {
	cleaned, count, err := trimPythonSpace(value)
	if err != nil || cleaned == "" {
		return "", fmt.Errorf("%s must be a non-empty string", label)
	}
	if count > maximum {
		return "", fmt.Errorf("%s exceeds %d characters", label, maximum)
	}
	if containsSecret(cleaned) {
		return "", fmt.Errorf("%s contains secret-shaped content", label)
	}
	return cleaned, nil
}

func normalizeCurrentPaths(values []string, label, treeRevision string, tracked map[string]struct{}) ([]string, error) {
	paths := sortedUnique(values)
	if len(paths) > MaxTracePaths {
		return nil, fmt.Errorf("%s exceeds %d paths", label, MaxTracePaths)
	}
	for _, value := range paths {
		admission, err := classifyCurrentPath(value, label, tracked)
		if err != nil {
			return nil, err
		}
		switch admission {
		case currentPathForbidden:
			return nil, fmt.Errorf("%s contains forbidden path: %s", label, value)
		case currentPathNonSource:
			if treeRevision == "" {
				return nil, fmt.Errorf("%s path was not tracked at trace revision: %s", label, value)
			}
			return nil, fmt.Errorf("%s path is not a tracked source at %s: %s", label, treeRevision, value)
		}
	}
	return paths, nil
}

// AdmissibleCurrentPaths classifies a bounded complete changed-path set through
// the same per-path admission primitive NewRecord uses. Malformed input fails;
// valid paths outside the exact tracked source set are filtered.
func AdmissibleCurrentPaths(values, trackedPaths []string) ([]string, error) {
	candidates := sortedUnique(values)
	if len(candidates) > MaxAdmissionCandidates {
		return nil, admissionFailure("candidate-limit", fmt.Errorf("changed-path candidates exceed %d paths", MaxAdmissionCandidates))
	}
	tracked := stringSet(trackedPaths)
	admitted := make([]string, 0, len(candidates))
	for _, value := range candidates {
		admission, err := classifyCurrentPath(value, "changed_paths", tracked)
		if err != nil {
			return nil, err
		}
		if admission == currentPathAdmitted {
			admitted = append(admitted, value)
		}
	}
	if len(admitted) > MaxTracePaths {
		return nil, admissionFailure("admitted-path-limit", fmt.Errorf("changed_paths exceeds %d paths", MaxTracePaths))
	}
	return admitted, nil
}

type admissionError struct {
	reason string
	cause  error
}

func (err *admissionError) Error() string { return err.cause.Error() }
func (err *admissionError) Unwrap() error { return err.cause }

func admissionFailure(reason string, cause error) error {
	return &admissionError{reason: reason, cause: cause}
}

// AdmissionFailureReason returns the bounded reason for an exact changed-path
// admission failure. It is empty for errors outside that classifier.
func AdmissionFailureReason(err error) string {
	var admission *admissionError
	if errors.As(err, &admission) {
		return admission.reason
	}
	return ""
}

type verificationError struct {
	reason string
	cause  error
}

func (err *verificationError) Error() string { return err.cause.Error() }
func (err *verificationError) Unwrap() error { return err.cause }

func verificationFailure(reason string, cause error) error {
	return &verificationError{reason: reason, cause: cause}
}

// VerificationFailureReason returns the bounded reason for an exact
// verification-command failure. It is empty for other errors.
func VerificationFailureReason(err error) string {
	var verification *verificationError
	if errors.As(err, &verification) {
		return verification.reason
	}
	return ""
}

func normalizeHistoricalPaths(values []string, label, revision string, tracked map[string]struct{}) ([]string, error) {
	return normalizeHistoricalPathsWithMatcher(values, label, revision, tracked, secretscreen.MatchString)
}

func normalizeStoredHistoricalPaths(values []string, label, revision string, tracked map[string]struct{}) ([]string, error) {
	return normalizeHistoricalPathsWithMatcher(values, label, revision, tracked, secretscreen.MatchStoredV1String)
}

func normalizeHistoricalPathsWithMatcher(values []string, label, revision string, tracked map[string]struct{}, containsSecret func(string) bool) ([]string, error) {
	paths, err := normalizePathsWithMatcher(values, label, containsSecret)
	if err != nil {
		return nil, err
	}
	for _, value := range paths {
		if _, ok := tracked[value]; !ok {
			return nil, fmt.Errorf("%s path was not tracked at trace revision %s: %s", label, revision, value)
		}
	}
	return paths, nil
}

func normalizePaths(values []string, label string) ([]string, error) {
	return normalizePathsWithMatcher(values, label, secretscreen.MatchString)
}

func normalizeStoredPaths(values []string, label string) ([]string, error) {
	return normalizePathsWithMatcher(values, label, secretscreen.MatchStoredV1String)
}

func normalizePathsWithMatcher(values []string, label string, containsSecret func(string) bool) ([]string, error) {
	values = sortedUnique(values)
	if len(values) > MaxTracePaths {
		return nil, fmt.Errorf("%s exceeds %d paths", label, MaxTracePaths)
	}
	for _, value := range values {
		admission, err := classifyPath(value, label, nil, containsSecret)
		if err != nil {
			return nil, err
		}
		if admission == currentPathForbidden {
			return nil, fmt.Errorf("%s contains forbidden path: %s", label, value)
		}
	}
	return values, nil
}

type currentPathAdmission uint8

const (
	currentPathAdmitted currentPathAdmission = iota
	currentPathNonSource
	currentPathForbidden
)

func classifyCurrentPath(value, label string, tracked map[string]struct{}) (currentPathAdmission, error) {
	return classifyPath(value, label, tracked, secretscreen.MatchString)
}

func classifyPath(value, label string, tracked map[string]struct{}, containsSecret func(string) bool) (currentPathAdmission, error) {
	if value == "" || !pythonString(value) {
		return currentPathNonSource, admissionFailure("malformed-path", fmt.Errorf("%s contains an invalid path", label))
	}
	if containsSecret(value) {
		return currentPathNonSource, admissionFailure("secret-shaped-path", fmt.Errorf("%s contains secret-shaped content", label))
	}
	if path.IsAbs(value) || strings.Contains(value, "//") || path.Clean(value) != value || hasParentPart(value) {
		return currentPathNonSource, admissionFailure("unnormalized-path", fmt.Errorf("%s path must be normalized and repository-relative: %q", label, value))
	}
	if contextindex.ForbiddenPathReason(value) != "" {
		return currentPathForbidden, nil
	}
	if tracked != nil {
		if _, ok := tracked[value]; !ok {
			return currentPathNonSource, nil
		}
	}
	if !witnessPath(value) {
		return currentPathNonSource, admissionFailure("malformed-path", fmt.Errorf("%s contains an invalid path", label))
	}
	return currentPathAdmitted, nil
}

func normalizeCommands(values []string) ([]string, error) {
	return normalizeCommandsWithMatcher(values, secretscreen.MatchString)
}

func normalizeStoredCommands(values []string) ([]string, error) {
	return normalizeCommandsWithMatcher(values, secretscreen.MatchStoredV1String)
}

func normalizeCommandsWithMatcher(values []string, containsSecret func(string) bool) ([]string, error) {
	values = sortedUnique(values)
	if len(values) > MaxVerificationCommands {
		return nil, fmt.Errorf("verification exceeds %d commands", MaxVerificationCommands)
	}
	normalized := make([]string, len(values))
	for index, value := range values {
		command, err := safeTextWithMatcher(value, "verification command", MaxCommandCharacters, containsSecret)
		if err != nil {
			return nil, err
		}
		if err := safeCommand(command); err != nil {
			return nil, verificationFailure("unsupported-verify-syntax", err)
		}
		normalized[index] = command
	}
	return sortedUnique(normalized), nil
}

func sortedUnique(values []string) []string {
	result := append([]string(nil), values...)
	if result == nil {
		result = []string{}
	}
	sort.Strings(result)
	write := 0
	for _, value := range result {
		if write == 0 || result[write-1] != value {
			result[write] = value
			write++
		}
	}
	return result[:write]
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

// admittedCommandPunctuation is the only non-alphanumeric ASCII a verification
// command may carry; shell operators, quotes, and parentheses stay refused.
const admittedCommandPunctuation = "_./:@=+, -"

func safeCommand(value string) error {
	if value == "" {
		return fmt.Errorf("verification command contains unsupported shell syntax: the command is empty")
	}
	for offset, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune(admittedCommandPunctuation, rune(character)) {
			continue
		}
		return fmt.Errorf("verification command contains unsupported shell syntax: byte %d %q is outside the admitted set of ASCII letters, digits, and %q",
			offset, character, admittedCommandPunctuation)
	}
	return nil
}

func validOutcome(value string) bool {
	return value == "passed" || value == "failed" || value == "blocked"
}

func hasParentPart(value string) bool {
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
