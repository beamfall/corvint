package model

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
	sha256Pattern  = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

var issueCodes = enumSet(
	"SOURCE_NOT_PRESENT", "SOURCE_INACCESSIBLE", "SOURCE_UNSUPPORTED", "SOURCE_DISABLED",
	"SOURCE_EXPIRED", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK",
	"SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_CHANGED_DURING_READ", "SOURCE_INVALID_SCHEMA",
	"SOURCE_INVALID_IDENTITY", "SOURCE_WRONG_COHORT", "VERIFIER_REJECTED",
	"LIMIT_ARTIFACTS", "LIMIT_INPUT_BYTES", "LIMIT_SAMPLES", "LIMIT_SNAPSHOT_BYTES",
	"OBSERVATION_TIME_UNKNOWN", "MIXED_COHORT_EXCLUDED", "REPOSITORY_OBJECT_UNAVAILABLE",
	"UNSUPPORTED_OBJECT_ALTERNATES", "STORE_CHANGED", "TRACE_ANCESTRY_BOUND", "TRACE_STORE_BOUND",
)

var memberTerminalIssueCodes = enumSet(
	"REPOSITORY_OBJECT_UNAVAILABLE", "SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_INVALID_IDENTITY",
	"SOURCE_INVALID_SCHEMA", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_OVERSIZED",
	"SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK", "TRACE_ANCESTRY_BOUND", "VERIFIER_REJECTED",
)

func invalidArgument() *Error   { return &Error{Code: "DASHBOARD_INVALID_ARGUMENT"} }
func resourceExhausted() *Error { return &Error{Code: "DASHBOARD_RESOURCE_EXHAUSTED"} }
func internalError() *Error     { return &Error{Code: "DASHBOARD_INTERNAL_ERROR"} }

func validateDecimal(value string) bool { return decimalPattern.MatchString(value) }
func validateSHA256(value string) bool  { return sha256Pattern.MatchString(value) }

func validateTimestamp(value string) bool {
	parsed, err := time.Parse("2006-01-02T15:04:05.000000000Z", value)
	return err == nil && parsed.Location() == time.UTC && parsed.Format("2006-01-02T15:04:05.000000000Z") == value
}

func validateIssue(issue Issue) error {
	if _, ok := issueCodes[issue.Code]; !ok || !oneOf(issue.Severity, SeverityInfo, SeverityWarning, SeverityError) {
		return invalidArgument()
	}
	if issue.SourceID != nil && !strings.HasPrefix(*issue.SourceID, "dashboard-source:sha256:") {
		return invalidArgument()
	}
	for _, value := range []*string{issue.Observed, issue.Limit} {
		if value != nil && !validateDecimal(*value) {
			return invalidArgument()
		}
	}
	if _, terminal := memberTerminalIssueCodes[issue.Code]; terminal && issue.Observed != nil {
		if *issue.Observed == "0" || issue.Severity != SeverityError || issue.Limit != nil {
			return invalidArgument()
		}
	}
	return nil
}

func normalizeSet(values []string) ([]string, error) {
	copy := append([]string(nil), values...)
	for _, value := range copy {
		if value == "" || validateWireString(value) != nil {
			return nil, invalidArgument()
		}
	}
	sort.Strings(copy)
	if hasDuplicate(copy) {
		return nil, invalidArgument()
	}
	return copy, nil
}

func hasDuplicate(values []string) bool {
	for index := 1; index < len(values); index++ {
		if values[index-1] == values[index] {
			return true
		}
	}
	return false
}

func enumSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func oneOf[T comparable](value T, allowed ...T) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func decimalUint(value uint64) string { return strconv.FormatUint(value, 10) }

func parseDecimal(value string) (uint64, bool) {
	if !validateDecimal(value) {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil
}
