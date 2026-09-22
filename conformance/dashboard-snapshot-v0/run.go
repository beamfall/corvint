package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	profile          = "corvint-dashboard-snapshot-verifier/0"
	schema           = "corvint-dashboard-snapshot/0"
	maxSnapshotBytes = 4 << 20
	maxJSONDepth     = 128
)

var (
	decimalRE   = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
	digestRE    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	sourceIDRE  = regexp.MustCompile(`^dashboard-source:sha256:[0-9a-f]{64}$`)
	cohortIDRE  = regexp.MustCompile(`^dashboard-cohort:sha256:[0-9a-f]{64}$`)
	issueIDRE   = regexp.MustCompile(`^dashboard-issue:sha256:[0-9a-f]{64}$`)
	timestampRE = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{9}Z$`)
	gitSHA1RE   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	gitSHA256RE = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

const canonicalRegistryWithoutStoreChanged = `[{"acceptedProfiles":[],"adapterId":"beamfall-shadow-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"BEAMFALL_SHADOW","verifierId":"unsupported"},{"acceptedProfiles":["cem/0.1+ocm/0.1","cem/0.2+ocm/0.1"],"adapterId":"cem-ocm-bundle-v0","defaultLocation":null,"deliveryStage":"NOT_STARTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"CEM_OCM_BUNDLE","verifierId":"go-cem-ocm-bundle-v0"},{"acceptedProfiles":[],"adapterId":"frontier-usage-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"FRONTIER_RECEIPT","verifierId":"unsupported"},{"acceptedProfiles":[],"adapterId":"go-live-usage-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"GO_LIVE_RECEIPT","verifierId":"unsupported"},{"acceptedProfiles":[],"adapterId":"harness-usage-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"HARNESS_RECEIPT","verifierId":"unsupported"},{"acceptedProfiles":[],"adapterId":"head-spec-index-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"HEAD_SPEC_INDEX","verifierId":"unsupported"},{"acceptedProfiles":[],"adapterId":"impact-envelope-v1","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"IMPACT_ENVELOPE","verifierId":"unsupported"},{"acceptedProfiles":["corvint-local-trace/1"],"adapterId":"local-trace-v1","defaultLocation":".context-corvint/traces","deliveryStage":"NOT_STARTED","issueCodes":["OBSERVATION_TIME_UNKNOWN","REPOSITORY_OBJECT_UNAVAILABLE","SOURCE_CHANGED_DURING_READ","SOURCE_INACCESSIBLE","SOURCE_INVALID_IDENTITY","SOURCE_INVALID_SCHEMA","SOURCE_MULTILINK_UNQUALIFIED","SOURCE_NOT_PRESENT","SOURCE_OVERSIZED","SOURCE_SPECIAL_FILE","SOURCE_SYMLINK","TRACE_ANCESTRY_BOUND","TRACE_STORE_BOUND","UNSUPPORTED_OBJECT_ALTERNATES","VERIFIER_REJECTED"],"maxBytes":"16777216","sourceKind":"LOCAL_TRACE_STORE","verifierId":"go-local-trace-v1"},{"acceptedProfiles":[],"adapterId":"pulse-dogfood-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"PULSE_RECEIPT","verifierId":"unsupported"},{"acceptedProfiles":[],"adapterId":"query-envelope-v1","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"QUERY_ENVELOPE","verifierId":"unsupported"},{"acceptedProfiles":["dashboard-stable-read/0"],"adapterId":"stable-read-v0","defaultLocation":null,"deliveryStage":"NOT_STARTED","issueCodes":["SOURCE_CHANGED_DURING_READ","SOURCE_INACCESSIBLE","SOURCE_MULTILINK_UNQUALIFIED","SOURCE_NOT_PRESENT","SOURCE_OVERSIZED","SOURCE_SPECIAL_FILE","SOURCE_SYMLINK"],"maxBytes":"16777216","sourceKind":"INTERNAL","verifierId":"go-stable-read-v0"}]`

var canonicalRegistry = strings.Replace(canonicalRegistryWithoutStoreChanged, `"SOURCE_SYMLINK","TRACE_ANCESTRY_BOUND"`, `"SOURCE_SYMLINK","STORE_CHANGED","TRACE_ANCESTRY_BOUND"`, 1)

type registryEntry struct {
	profiles map[string]struct{}
	stage    string
	verifier string
	maxBytes uint64
}

var registry = map[string]registryEntry{
	"beamfall-shadow-v0": {profiles: set(), stage: "UNSUPPORTED", verifier: "unsupported"},
	"cem-ocm-bundle-v0":  {profiles: set("cem/0.1+ocm/0.1", "cem/0.2+ocm/0.1"), stage: "NOT_STARTED", verifier: "go-cem-ocm-bundle-v0"},
	"frontier-usage-v0":  {profiles: set(), stage: "UNSUPPORTED", verifier: "unsupported"},
	"go-live-usage-v0":   {profiles: set(), stage: "UNSUPPORTED", verifier: "unsupported"},
	"harness-usage-v0":   {profiles: set(), stage: "UNSUPPORTED", verifier: "unsupported"},
	"head-spec-index-v0": {profiles: set(), stage: "UNSUPPORTED", verifier: "unsupported"},
	"impact-envelope-v1": {profiles: set(), stage: "UNSUPPORTED", verifier: "unsupported"},
	"local-trace-v1":     {profiles: set("corvint-local-trace/1"), stage: "NOT_STARTED", verifier: "go-local-trace-v1", maxBytes: 16 << 20},
	"pulse-dogfood-v0":   {profiles: set(), stage: "UNSUPPORTED", verifier: "unsupported"},
	"query-envelope-v1":  {profiles: set(), stage: "UNSUPPORTED", verifier: "unsupported"},
	"stable-read-v0":     {profiles: set("dashboard-stable-read/0"), stage: "NOT_STARTED", verifier: "go-stable-read-v0", maxBytes: 16 << 20},
}

type rejectCode string

const (
	rejectSize        rejectCode = "SNAPSHOT_SIZE"
	rejectJSON        rejectCode = "SNAPSHOT_JSON"
	rejectDuplicate   rejectCode = "SNAPSHOT_DUPLICATE_KEY"
	rejectCanonical   rejectCode = "SNAPSHOT_NONCANONICAL"
	rejectUnicode     rejectCode = "SNAPSHOT_UNICODE"
	rejectRoot        rejectCode = "SNAPSHOT_ROOT"
	rejectFieldSet    rejectCode = "SNAPSHOT_FIELD_SET"
	rejectType        rejectCode = "SNAPSHOT_TYPE"
	rejectEnum        rejectCode = "SNAPSHOT_ENUM"
	rejectDecimal     rejectCode = "SNAPSHOT_DECIMAL"
	rejectTimestamp   rejectCode = "SNAPSHOT_TIMESTAMP"
	rejectDigest      rejectCode = "SNAPSHOT_DIGEST"
	rejectOrdering    rejectCode = "SNAPSHOT_ORDERING"
	rejectIdentity    rejectCode = "SNAPSHOT_IDENTITY"
	rejectTruthAxes   rejectCode = "SNAPSHOT_TRUTH_AXES"
	rejectMetric      rejectCode = "SNAPSHOT_METRIC"
	rejectMetricSet   rejectCode = "SNAPSHOT_METRIC_SET"
	rejectPrivacy     rejectCode = "SNAPSHOT_PRIVACY"
	rejectHash        rejectCode = "SNAPSHOT_HASH"
	rejectPrivacyText rejectCode = "SNAPSHOT_PRIVACY_TEXT"
	rejectInternal    rejectCode = "SNAPSHOT_INTERNAL"
)

type verificationError struct {
	code rejectCode
}

func (e verificationError) Error() string { return string(e.code) }

func reject(code rejectCode) error { return verificationError{code: code} }

func rejectionCode(err error) rejectCode {
	var verification verificationError
	if errors.As(err, &verification) {
		return verification.code
	}
	return rejectInternal
}

func set(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

var (
	validities       = set("VALID", "INVALID", "NOT_PRESENT", "INACCESSIBLE", "UNSUPPORTED", "DISABLED", "EXPIRED")
	epistemicClasses = set("OBSERVED", "DECLARED", "ADVISORY", "NOT_OBSERVED")
	authorityClasses = set("REPOSITORY_ACCEPTED", "OWNING_VERIFIER", "PROVIDER_QUALIFIED", "ADAPTER_QUALIFIED", "CALLER_REPORTED", "ADVISORY", "NONE")
	completenesses   = set("COMPLETE", "PARTIAL", "UNKNOWN")
	currencies       = set("VALIDATED_AT", "HISTORICAL", "STALE", "MIXED", "UNKNOWN")
	deliveryStages   = set("ACCEPTED", "VALIDATED", "IMPLEMENTED", "EXPERIMENTAL", "NOT_STARTED", "FAILED", "UNSUPPORTED")
	scopeClasses     = set("SINGLE_COHORT", "MULTI_COHORT_INVENTORY", "UNAVAILABLE")
	units            = set("COUNT", "BYTES", "NANOSECONDS", "RATIO", "STATE")
	ageBuckets       = set("LT_1H", "H1_TO_24H", "D1_TO_7D", "GE_7D", "UNKNOWN")
	issueCodes       = set(
		"SOURCE_NOT_PRESENT", "SOURCE_INACCESSIBLE", "SOURCE_UNSUPPORTED", "SOURCE_DISABLED",
		"SOURCE_EXPIRED", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK",
		"SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_CHANGED_DURING_READ", "SOURCE_INVALID_SCHEMA",
		"SOURCE_INVALID_IDENTITY", "SOURCE_WRONG_COHORT", "VERIFIER_REJECTED", "LIMIT_ARTIFACTS",
		"LIMIT_INPUT_BYTES", "LIMIT_SAMPLES", "LIMIT_SNAPSHOT_BYTES", "OBSERVATION_TIME_UNKNOWN",
		"MIXED_COHORT_EXCLUDED", "REPOSITORY_OBJECT_UNAVAILABLE", "STORE_CHANGED", "UNSUPPORTED_OBJECT_ALTERNATES",
		"TRACE_ANCESTRY_BOUND", "TRACE_STORE_BOUND",
	)
	adapterIDs = set(
		"stable-read-v0", "local-trace-v1", "cem-ocm-bundle-v0", "query-envelope-v1",
		"impact-envelope-v1", "head-spec-index-v0", "beamfall-shadow-v0", "pulse-dogfood-v0",
		"harness-usage-v0", "go-live-usage-v0", "frontier-usage-v0",
	)
)

var metricGroup = map[string]string{
	"data.artifact.count": "data", "data.artifact.bytes": "data",
	"usage.trace.retained": "usage", "usage.query.retained": "usage",
	"usage.impact.retained": "usage", "usage.harness.retained": "usage",
	"usage.response.bytes": "usage", "usage.latency.p50": "usage", "usage.latency.p95": "usage",
	"verification.cem.hunks": "verification", "verification.ocm.obligations": "verification",
	"verification.closure": "verification", "verification.live.runs": "verification",
	"verification.pulse.runs": "verification", "frontier.items": "frontier",
	"beamfall.shadow.runs": "beamfall",
}

var metricUnit = map[string]string{
	"data.artifact.count": "COUNT", "data.artifact.bytes": "BYTES",
	"usage.trace.retained": "COUNT", "usage.query.retained": "COUNT",
	"usage.impact.retained": "COUNT", "usage.harness.retained": "COUNT",
	"usage.response.bytes": "BYTES", "usage.latency.p50": "NANOSECONDS", "usage.latency.p95": "NANOSECONDS",
	"verification.cem.hunks": "COUNT", "verification.ocm.obligations": "COUNT",
	"verification.closure": "RATIO", "verification.live.runs": "COUNT",
	"verification.pulse.runs": "COUNT", "frontier.items": "COUNT",
	"beamfall.shadow.runs": "COUNT",
}

var baseUnavailable = set(
	"usage.query.retained", "usage.impact.retained", "usage.harness.retained", "usage.response.bytes",
	"usage.latency.p50", "usage.latency.p95", "verification.cem.hunks", "verification.ocm.obligations",
	"verification.closure", "verification.live.runs", "verification.pulse.runs", "frontier.items",
	"beamfall.shadow.runs",
)

func decode(raw []byte) (any, error) {
	return decodeWithTextPolicy(raw, true)
}

func decodeUTF8(raw []byte) (any, error) {
	return decodeWithTextPolicy(raw, false)
}

func decodeWithTextPolicy(raw []byte, asciiOnly bool) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder, 0, asciiOnly)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, reject(rejectJSON)
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder, depth int, asciiOnly bool) (any, error) {
	if depth > maxJSONDepth {
		return nil, reject(rejectJSON)
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, reject(rejectJSON)
	}
	switch typed := token.(type) {
	case nil, bool:
		return typed, nil
	case string:
		if !utf8.ValidString(typed) {
			return nil, reject(rejectUnicode)
		}
		for _, character := range typed {
			if character < 0x20 || asciiOnly && character > 0x7e {
				return nil, reject(rejectUnicode)
			}
		}
		return typed, nil
	case json.Number:
		return nil, reject(rejectType)
	case json.Delim:
		switch typed {
		case '{':
			object := make(map[string]any)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, reject(rejectJSON)
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, reject(rejectJSON)
				}
				if _, exists := object[key]; exists {
					return nil, reject(rejectDuplicate)
				}
				value, err := decodeValue(decoder, depth+1, asciiOnly)
				if err != nil {
					return nil, err
				}
				object[key] = value
			}
			if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
				return nil, reject(rejectJSON)
			}
			return object, nil
		case '[':
			array := make([]any, 0)
			for decoder.More() {
				value, err := decodeValue(decoder, depth+1, asciiOnly)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
				return nil, reject(rejectJSON)
			}
			return array, nil
		}
	}
	return nil, reject(rejectJSON)
}

func canonical(value any, trailingLF bool) ([]byte, error) {
	var output bytes.Buffer
	if err := appendCanonical(&output, value); err != nil {
		return nil, err
	}
	if trailingLF {
		output.WriteByte('\n')
	}
	return output.Bytes(), nil
}

func appendCanonical(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		output.WriteString(strconv.FormatBool(typed))
	case string:
		output.WriteString(strconv.Quote(typed))
	case json.Number:
		if !decimalRE.MatchString(string(typed)) {
			return reject(rejectDecimal)
		}
		output.WriteString(string(typed))
	case []any:
		output.WriteByte('[')
		for index, element := range typed {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := appendCanonical(output, element); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for index, key := range keys {
			if index > 0 {
				output.WriteByte(',')
			}
			output.WriteString(strconv.Quote(key))
			output.WriteByte(':')
			if err := appendCanonical(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return reject(rejectType)
	}
	return nil
}

func domainDigest(domain string, payload []byte) string {
	hash := sha256.New()
	hash.Write([]byte(domain))
	hash.Write([]byte{0})
	hash.Write(payload)
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func cloneWithout(value map[string]any, excluded string) map[string]any {
	result := make(map[string]any, len(value)-1)
	for key, field := range value {
		if key != excluded {
			result[key] = field
		}
	}
	return result
}

func object(value any) (map[string]any, error) {
	result, ok := value.(map[string]any)
	if !ok {
		return nil, reject(rejectType)
	}
	return result, nil
}

func array(value any) ([]any, error) {
	result, ok := value.([]any)
	if !ok {
		return nil, reject(rejectType)
	}
	return result, nil
}

func stringValue(value any) (string, error) {
	result, ok := value.(string)
	if !ok {
		return "", reject(rejectType)
	}
	return result, nil
}

func exactFields(value map[string]any, names ...string) error {
	if len(value) != len(names) {
		return reject(rejectFieldSet)
	}
	for _, name := range names {
		if _, exists := value[name]; !exists {
			return reject(rejectFieldSet)
		}
	}
	return nil
}

func enum(value any, allowed map[string]struct{}) (string, error) {
	text, err := stringValue(value)
	if err != nil {
		return "", err
	}
	if _, exists := allowed[text]; !exists {
		return "", reject(rejectEnum)
	}
	return text, nil
}

func nullableString(value any) (string, bool, error) {
	if value == nil {
		return "", false, nil
	}
	text, err := stringValue(value)
	return text, true, err
}

func decimal(value any, nullable bool) (string, bool, error) {
	text, present, err := nullableString(value)
	if err != nil {
		return "", false, err
	}
	if !present {
		if nullable {
			return "", false, nil
		}
		return "", false, reject(rejectDecimal)
	}
	if !decimalRE.MatchString(text) {
		return "", false, reject(rejectDecimal)
	}
	return text, true, nil
}

func digest(value any, nullable bool) (string, bool, error) {
	text, present, err := nullableString(value)
	if err != nil {
		return "", false, err
	}
	if !present {
		if nullable {
			return "", false, nil
		}
		return "", false, reject(rejectDigest)
	}
	if !digestRE.MatchString(text) {
		return "", false, reject(rejectDigest)
	}
	return text, true, nil
}

func timestamp(value any, nullable bool) (time.Time, bool, error) {
	text, present, err := nullableString(value)
	if err != nil {
		return time.Time{}, false, err
	}
	if !present {
		if nullable {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, reject(rejectTimestamp)
	}
	if !timestampRE.MatchString(text) {
		return time.Time{}, false, reject(rejectTimestamp)
	}
	parsed, err := time.Parse("2006-01-02T15:04:05.000000000Z", text)
	if err != nil || parsed.Format("2006-01-02T15:04:05.000000000Z") != text {
		return time.Time{}, false, reject(rejectTimestamp)
	}
	return parsed, true, nil
}

func sortedUniqueStrings(value any, matcher *regexp.Regexp) ([]string, error) {
	values, err := array(value)
	if err != nil {
		return nil, err
	}
	result := make([]string, len(values))
	for index, item := range values {
		text, err := stringValue(item)
		if err != nil {
			return nil, err
		}
		if matcher != nil && !matcher.MatchString(text) {
			return nil, reject(rejectIdentity)
		}
		if index > 0 && result[index-1] >= text {
			return nil, reject(rejectOrdering)
		}
		result[index] = text
	}
	return result, nil
}

func verifySnapshot(raw []byte) error {
	if len(raw) == 0 || len(raw) > maxSnapshotBytes {
		return reject(rejectSize)
	}
	if !utf8.Valid(raw) {
		return reject(rejectUnicode)
	}
	rootValue, err := decode(raw)
	if err != nil {
		return err
	}
	canonicalBytes, err := canonical(rootValue, true)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, canonicalBytes) {
		return reject(rejectCanonical)
	}
	root, err := object(rootValue)
	if err != nil {
		return reject(rejectRoot)
	}
	if err := exactFields(root, "schema", "generatedAt", "observation", "repository", "cohorts", "sources", "data", "usage", "verification", "frontier", "harnesses", "beamfall", "privacy", "issues", "snapshotSha256"); err != nil {
		return reject(rejectRoot)
	}
	if root["schema"] != schema {
		return reject(rejectRoot)
	}
	generatedAt, _, err := timestamp(root["generatedAt"], false)
	if err != nil {
		return err
	}
	repository, err := verifyRepository(root["repository"])
	if err != nil {
		return err
	}
	cohorts, err := verifyCohorts(root["cohorts"], repository)
	if err != nil {
		return err
	}
	sourceIDs, _, issueIDs, configuredRows, err := verifySources(root["sources"], generatedAt, cohorts, repository)
	if err != nil {
		return err
	}
	metricSources := verifiedMetricSources(root["sources"])
	if err := verifyObservation(root["observation"], generatedAt, configuredRows); err != nil {
		return err
	}
	allIssueIDs, err := verifyIssues(root["issues"], sourceIDs, issueIDs)
	if err != nil {
		return err
	}
	if err := verifyTraceIssueSemantics(root["sources"], root["issues"]); err != nil {
		return err
	}
	seenMetrics := make(map[string]int)
	for _, group := range []string{"data", "usage", "verification", "frontier", "harnesses", "beamfall"} {
		if err := verifyMetricGroup(group, root[group], metricSources, cohorts, allIssueIDs, repository, seenMetrics); err != nil {
			return err
		}
	}
	if err := verifyMetricCompleteness(seenMetrics); err != nil {
		return err
	}
	if err := verifyTraceInventory(root["data"], root["sources"], root["issues"], cohorts); err != nil {
		return err
	}
	if err := verifyScanState(root["observation"], root["sources"]); err != nil {
		return err
	}
	if err := verifyPrivacy(root["privacy"]); err != nil {
		return err
	}
	if err := verifySnapshotHash(root); err != nil {
		return err
	}
	return nil
}

func verifyObservation(value any, generatedAt time.Time, configuredRows []any) error {
	observation, err := object(value)
	if err != nil {
		return err
	}
	if err := exactFields(observation, "adapterRegistrySha256", "configuredSourceSetSha256", "end", "limitsProfile", "scanState", "start", "clockSource"); err != nil {
		return err
	}
	registryDigest, _, err := digest(observation["adapterRegistrySha256"], false)
	if err != nil {
		return err
	}
	if registryDigest != domainDigest("corvint-dashboard-adapter-registry/0", []byte(canonicalRegistry)) {
		return reject(rejectDigest)
	}
	configuredDigest, _, err := digest(observation["configuredSourceSetSha256"], false)
	if err != nil {
		return err
	}
	sort.Slice(configuredRows, func(left, right int) bool {
		leftBytes, _ := canonical(configuredRows[left], false)
		rightBytes, _ := canonical(configuredRows[right], false)
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
	configuredBytes, err := canonical(configuredRows, false)
	if err != nil || configuredDigest != domainDigest("corvint-dashboard-configured-sources/0", configuredBytes) {
		return reject(rejectDigest)
	}
	start, _, err := timestamp(observation["start"], false)
	if err != nil {
		return err
	}
	end, _, err := timestamp(observation["end"], false)
	if err != nil {
		return err
	}
	if end.Before(start) || generatedAt.Before(end) {
		return reject(rejectTimestamp)
	}
	if observation["limitsProfile"] != "corvint-dashboard-limits/0" {
		return reject(rejectEnum)
	}
	if _, err := enum(observation["scanState"], set("COMPLETE", "PARTIAL", "INVALID")); err != nil {
		return err
	}
	if _, err := enum(observation["clockSource"], set("PROCESS", "CALLER")); err != nil {
		return err
	}
	return nil
}

type repositoryState struct {
	objectFormat string
	headRevision string
}

type cohortState struct {
	adapterID        string
	profile          string
	producer         string
	observationEnd   string
	observationStart string
	sourceRevision   string
	sourceTree       string
	dirtyDigest      string
	objectFormat     string
}

type metricSourceState struct {
	adapterID string
	cohortIDs map[string]struct{}
}

func verifyCohorts(value any, repository repositoryState) (map[string]cohortState, error) {
	cohortValues, err := array(value)
	if err != nil {
		return nil, err
	}
	result := make(map[string]cohortState, len(cohortValues))
	previous := ""
	for _, rawCohort := range cohortValues {
		cohort, err := object(rawCohort)
		if err != nil {
			return nil, err
		}
		if err := exactFields(cohort, "adapterId", "cohortId", "dirtyPathsSha256", "producerIdentity", "profile", "repositoryObjectFormat", "sourceObservationEnd", "sourceObservationStart", "sourceRevision", "sourceTreeRevision"); err != nil {
			return nil, err
		}
		adapterID, err := enum(cohort["adapterId"], adapterIDs)
		if err != nil {
			return nil, err
		}
		id, err := stringValue(cohort["cohortId"])
		if err != nil || !cohortIDRE.MatchString(id) {
			return nil, reject(rejectIdentity)
		}
		if previous != "" && previous >= id {
			return nil, reject(rejectOrdering)
		}
		previous = id
		if _, exists := result[id]; exists {
			return nil, reject(rejectIdentity)
		}
		profileValue, err := stringValue(cohort["profile"])
		if err != nil || len(profileValue) == 0 || len(profileValue) > 128 {
			return nil, reject(rejectEnum)
		}
		producer, err := stringValue(cohort["producerIdentity"])
		if err != nil || !safeIdentifier(producer) {
			return nil, reject(rejectPrivacyText)
		}
		format, formatPresent, err := nullableString(cohort["repositoryObjectFormat"])
		if err != nil || formatPresent && format != "sha1" && format != "sha256" {
			return nil, reject(rejectEnum)
		}
		if formatPresent && repository.objectFormat != "" && format != repository.objectFormat {
			return nil, reject(rejectIdentity)
		}
		dirtyDigest, _, err := digest(cohort["dirtyPathsSha256"], true)
		if err != nil {
			return nil, err
		}
		sourceRevision := ""
		sourceTree := ""
		for _, field := range []string{"sourceRevision", "sourceTreeRevision"} {
			revision, present, err := nullableString(cohort[field])
			if err != nil {
				return nil, err
			}
			if present {
				matcher := gitSHA1RE
				if format == "sha256" {
					matcher = gitSHA256RE
				}
				if !formatPresent || !matcher.MatchString(revision) {
					return nil, reject(rejectIdentity)
				}
				if field == "sourceRevision" {
					sourceRevision = revision
				} else {
					sourceTree = revision
				}
			}
		}
		startText, startPresent, err := nullableString(cohort["sourceObservationStart"])
		if err != nil {
			return nil, err
		}
		endText, endPresent, err := nullableString(cohort["sourceObservationEnd"])
		if err != nil || startPresent != endPresent {
			return nil, reject(rejectTimestamp)
		}
		if startPresent {
			start, _, err := timestamp(startText, false)
			if err != nil {
				return nil, err
			}
			end, _, err := timestamp(endText, false)
			if err != nil || end.Before(start) {
				return nil, reject(rejectTimestamp)
			}
		}
		basis := cloneWithout(cohort, "cohortId")
		basisBytes, err := canonical(basis, false)
		if err != nil || id != "dashboard-cohort:"+domainDigest("corvint-dashboard-cohort/0", basisBytes) {
			return nil, reject(rejectIdentity)
		}
		result[id] = cohortState{adapterID: adapterID, profile: profileValue, producer: producer, observationStart: startText, observationEnd: endText, sourceRevision: sourceRevision, sourceTree: sourceTree, dirtyDigest: dirtyDigest, objectFormat: format}
	}
	return result, nil
}

func verifyRepository(value any) (repositoryState, error) {
	repository, err := object(value)
	if err != nil {
		return repositoryState{}, err
	}
	if err := exactFields(repository, "dirtyPathCount", "dirtyPathsSha256", "headRevision", "objectFormat", "treeRevision", "worktreeState"); err != nil {
		return repositoryState{}, err
	}
	if _, _, err := decimal(repository["dirtyPathCount"], true); err != nil {
		return repositoryState{}, err
	}
	if _, _, err := digest(repository["dirtyPathsSha256"], true); err != nil {
		return repositoryState{}, err
	}
	format, present, err := nullableString(repository["objectFormat"])
	if err != nil {
		return repositoryState{}, err
	}
	if present && format != "sha1" && format != "sha256" {
		return repositoryState{}, reject(rejectEnum)
	}
	head, headPresent, err := nullableString(repository["headRevision"])
	if err != nil {
		return repositoryState{}, err
	}
	tree, treePresent, err := nullableString(repository["treeRevision"])
	if err != nil {
		return repositoryState{}, err
	}
	if headPresent != present || treePresent != present {
		return repositoryState{}, reject(rejectIdentity)
	}
	if present {
		matcher := gitSHA1RE
		if format == "sha256" {
			matcher = gitSHA256RE
		}
		if !matcher.MatchString(head) || !matcher.MatchString(tree) {
			return repositoryState{}, reject(rejectIdentity)
		}
	}
	if _, err := enum(repository["worktreeState"], set("CLEAN", "MIXED", "UNKNOWN")); err != nil {
		return repositoryState{}, err
	}
	return repositoryState{objectFormat: format, headRevision: head}, nil
}

func verifySources(value any, generatedAt time.Time, cohorts map[string]cohortState, repository repositoryState) (map[string]struct{}, map[string]struct{}, map[string]struct{}, []any, error) {
	sources, err := array(value)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	sourceIDs := make(map[string]struct{}, len(sources))
	cohortIDs := make(map[string]struct{})
	issueIDs := make(map[string]struct{})
	configuredRows := make([]any, 0, len(sources))
	configuredOrdinals := make(map[string][]string)
	previous := ""
	for _, sourceValue := range sources {
		source, err := object(sourceValue)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if err := exactFields(source, "adapterId", "authorityClass", "byteCount", "cohortIds", "completeness", "configuredOrdinal", "contentSha256", "currency", "deliveryStage", "displayLabel", "epistemicClass", "exclusions", "id", "members", "observationEnd", "observationStart", "observationTime", "profile", "repositoryReadsSha256", "validity", "verifierId"); err != nil {
			return nil, nil, nil, nil, err
		}
		id, err := stringValue(source["id"])
		if err != nil || !sourceIDRE.MatchString(id) {
			return nil, nil, nil, nil, reject(rejectIdentity)
		}
		if previous >= id && previous != "" {
			return nil, nil, nil, nil, reject(rejectOrdering)
		}
		previous = id
		if _, exists := sourceIDs[id]; exists {
			return nil, nil, nil, nil, reject(rejectIdentity)
		}
		sourceIDs[id] = struct{}{}
		adapter, err := enum(source["adapterId"], adapterIDs)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		_ = adapter
		ordinal, _, err := decimal(source["configuredOrdinal"], false)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		validity, err := enum(source["validity"], validities)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		epistemic, err := enum(source["epistemicClass"], epistemicClasses)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if _, err := enum(source["authorityClass"], authorityClasses); err != nil {
			return nil, nil, nil, nil, err
		}
		completeness, err := enum(source["completeness"], completenesses)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if _, err := enum(source["currency"], currencies); err != nil {
			return nil, nil, nil, nil, err
		}
		if _, err := enum(source["deliveryStage"], deliveryStages); err != nil {
			return nil, nil, nil, nil, err
		}
		byteCount, hasBytes, err := decimal(source["byteCount"], true)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		contentDigest, hasDigest, err := digest(source["contentSha256"], true)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		_ = byteCount
		sourceCohorts, err := sortedUniqueStrings(source["cohortIds"], cohortIDRE)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		for _, cohort := range sourceCohorts {
			state, exists := cohorts[cohort]
			if !exists || state.adapterID != adapter {
				return nil, nil, nil, nil, reject(rejectIdentity)
			}
			cohortIDs[cohort] = struct{}{}
		}
		observationStart, hasStart, err := timestamp(source["observationStart"], true)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		observationEnd, hasEnd, err := timestamp(source["observationEnd"], true)
		if err != nil || hasStart != hasEnd || hasStart && observationEnd.Before(observationStart) {
			return nil, nil, nil, nil, reject(rejectTimestamp)
		}
		observationTime, hasObservation, err := timestamp(source["observationTime"], true)
		if err != nil || hasObservation && observationTime.After(generatedAt) {
			return nil, nil, nil, nil, reject(rejectTimestamp)
		}
		repositoryReadsDigest, hasRepositoryReads, err := digest(source["repositoryReadsSha256"], true)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		label, err := stringValue(source["displayLabel"])
		if err != nil || !safePublicLabel(label) {
			return nil, nil, nil, nil, reject(rejectPrivacyText)
		}
		profileValue, err := stringValue(source["profile"])
		if err != nil || len(profileValue) == 0 || len(profileValue) > 128 {
			return nil, nil, nil, nil, reject(rejectEnum)
		}
		verifier, err := stringValue(source["verifierId"])
		if err != nil || !safeIdentifier(verifier) {
			return nil, nil, nil, nil, reject(rejectPrivacyText)
		}
		entry := registry[adapter]
		if source["displayLabel"] != adapter+"#"+ordinal || source["deliveryStage"] != entry.stage || verifier != entry.verifier {
			return nil, nil, nil, nil, reject(rejectIdentity)
		}
		if len(entry.profiles) == 0 {
			if profileValue != "unsupported" {
				return nil, nil, nil, nil, reject(rejectEnum)
			}
		} else if _, exists := entry.profiles[profileValue]; !exists {
			return nil, nil, nil, nil, reject(rejectEnum)
		}
		if hasBytes {
			byteValue, ok := new(big.Int).SetString(byteCount, 10)
			if !ok || !byteValue.IsUint64() || byteValue.Uint64() > entry.maxBytes {
				return nil, nil, nil, nil, reject(rejectSize)
			}
		}
		configuredOrdinals[adapter] = append(configuredOrdinals[adapter], ordinal)
		for _, cohort := range sourceCohorts {
			state := cohorts[cohort]
			if state.profile != profileValue || state.producer != verifier {
				return nil, nil, nil, nil, reject(rejectIdentity)
			}
		}
		if err := verifyMembers(source, adapter, validity, contentDigest, hasDigest, byteCount, hasBytes, repositoryReadsDigest, hasRepositoryReads, sourceCohorts, cohorts, repository, hasStart, observationStart, observationEnd); err != nil {
			return nil, nil, nil, nil, err
		}
		identityBasis := map[string]any{"adapterId": adapter, "configuredOrdinal": ordinal, "contentSha256": nil, "profile": profileValue, "repositoryReadsSha256": nil}
		if hasDigest {
			identityBasis["contentSha256"] = contentDigest
		}
		if hasRepositoryReads {
			identityBasis["repositoryReadsSha256"] = repositoryReadsDigest
		}
		basisBytes, err := canonical(identityBasis, false)
		if err != nil || id != "dashboard-source:"+domainDigest("corvint-dashboard-source/0", basisBytes) {
			return nil, nil, nil, nil, reject(rejectIdentity)
		}
		configuredRows = append(configuredRows, map[string]any{"adapterId": adapter, "configuredOrdinal": ordinal, "contentSha256": identityBasis["contentSha256"]})
		exclusions, err := sortedUniqueStrings(source["exclusions"], issueIDRE)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		for _, exclusion := range exclusions {
			issueIDs[exclusion] = struct{}{}
		}
		if validity == "VALID" {
			if !hasBytes || !hasDigest || epistemic == "NOT_OBSERVED" {
				return nil, nil, nil, nil, reject(rejectTruthAxes)
			}
		} else if validity == "NOT_PRESENT" || validity == "INACCESSIBLE" || validity == "UNSUPPORTED" || validity == "DISABLED" || validity == "EXPIRED" {
			if hasBytes || hasDigest || len(sourceCohorts) != 0 || hasObservation || hasStart || epistemic != "NOT_OBSERVED" || completeness != "UNKNOWN" {
				return nil, nil, nil, nil, reject(rejectTruthAxes)
			}
		}
	}
	if err := verifyConfiguredOrdinals(configuredOrdinals); err != nil {
		return nil, nil, nil, nil, err
	}
	for cohort := range cohorts {
		if _, exists := cohortIDs[cohort]; !exists {
			return nil, nil, nil, nil, reject(rejectIdentity)
		}
	}
	return sourceIDs, cohortIDs, issueIDs, configuredRows, nil
}

func verifyConfiguredOrdinals(configuredOrdinals map[string][]string) error {
	for _, ordinals := range configuredOrdinals {
		sort.Slice(ordinals, func(left, right int) bool {
			leftValue, _ := new(big.Int).SetString(ordinals[left], 10)
			rightValue, _ := new(big.Int).SetString(ordinals[right], 10)
			return leftValue.Cmp(rightValue) < 0
		})
		for index, ordinal := range ordinals {
			if ordinal != strconv.Itoa(index) {
				return reject(rejectIdentity)
			}
		}
	}
	return nil
}

func verifiedMetricSources(value any) map[string]metricSourceState {
	sources := value.([]any)
	result := make(map[string]metricSourceState, len(sources))
	for _, sourceValue := range sources {
		source := sourceValue.(map[string]any)
		cohortIDs := make(map[string]struct{})
		for _, cohortID := range source["cohortIds"].([]any) {
			cohortIDs[cohortID.(string)] = struct{}{}
		}
		result[source["id"].(string)] = metricSourceState{adapterID: source["adapterId"].(string), cohortIDs: cohortIDs}
	}
	return result
}

func safePublicLabel(value string) bool {
	if value == "" || len(value) > 96 || strings.ContainsAny(value, "/\\:@\t\r\n") {
		return false
	}
	for _, character := range value {
		if !(character == ' ' || character == '#' || character == '-' || character == '_' || character == '.' || character >= '0' && character <= '9' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z') {
			return false
		}
	}
	return true
}

func verifyMembers(source map[string]any, adapter, validity, contentDigest string, hasDigest bool, byteCount string, hasBytes bool, repositoryReadsDigest string, hasRepositoryReads bool, sourceCohorts []string, cohorts map[string]cohortState, repository repositoryState, hasInterval bool, observationStart, observationEnd time.Time) error {
	if adapter != "local-trace-v1" {
		if source["members"] != nil {
			return reject(rejectFieldSet)
		}
		return nil
	}
	members, err := array(source["members"])
	if err != nil {
		return err
	}
	if len(members) > maxTraceRows {
		return reject(rejectSize)
	}
	var sum big.Int
	previousRevision := ""
	memberRevisions := make(map[string]struct{}, len(members))
	for _, rawMember := range members {
		member, err := object(rawMember)
		if err != nil {
			return err
		}
		if err := exactFields(member, "revision", "contentSha256", "byteCount"); err != nil {
			return err
		}
		revision, err := stringValue(member["revision"])
		if err != nil || previousRevision != "" && previousRevision >= revision {
			return reject(rejectOrdering)
		}
		previousRevision = revision
		memberRevisions[revision] = struct{}{}
		if _, _, err := digest(member["contentSha256"], false); err != nil {
			return err
		}
		memberBytes, _, err := decimal(member["byteCount"], false)
		if err != nil {
			return err
		}
		value, ok := new(big.Int).SetString(memberBytes, 10)
		if !ok {
			return reject(rejectDecimal)
		}
		sum.Add(&sum, value)
	}
	if !hasBytes || byteCount != sum.String() || !hasDigest {
		if validity == "INVALID" && len(members) == 0 && !hasBytes && !hasDigest && !hasRepositoryReads && len(sourceCohorts) == 0 && !hasInterval {
			return nil
		}
		return reject(rejectTruthAxes)
	}
	membersRaw, err := canonical(members, false)
	if err != nil || contentDigest != domainDigest("trace-store/0", membersRaw) {
		return reject(rejectDigest)
	}
	if len(members) == 0 {
		if validity == "INVALID" && !hasRepositoryReads && len(sourceCohorts) == 0 && !hasInterval {
			return nil
		}
		if len(sourceCohorts) != 0 || hasInterval || !hasRepositoryReads || repository.objectFormat == "" || repository.headRevision == "" {
			return reject(rejectTruthAxes)
		}
		witnesses := []any{map[string]any{"kind": "SNAPSHOT_HEAD", "objectFormat": repository.objectFormat, "objectId": repository.headRevision, "objectType": "commit", "revision": nil}}
		witnessBytes, err := canonical(witnesses, false)
		if err != nil || repositoryReadsDigest != domainDigest("corvint-dashboard-repository-reads/0", witnessBytes) {
			return reject(rejectDigest)
		}
		return nil
	}
	if !hasInterval || !hasRepositoryReads || repository.objectFormat == "" || repository.headRevision == "" {
		return reject(rejectTruthAxes)
	}
	matchedRevisions := make(map[string]struct{}, len(sourceCohorts))
	var minimum time.Time
	var maximum time.Time
	for _, cohortID := range sourceCohorts {
		state := cohorts[cohortID]
		if state.observationStart == "" || state.observationEnd == "" || state.sourceRevision == "" || state.sourceTree == "" || state.dirtyDigest == "" || state.objectFormat == "" {
			return reject(rejectTruthAxes)
		}
		start, _, err := timestamp(state.observationStart, false)
		if err != nil {
			return err
		}
		end, _, err := timestamp(state.observationEnd, false)
		if err != nil {
			return err
		}
		if minimum.IsZero() || start.Before(minimum) {
			minimum = start
		}
		if maximum.IsZero() || end.After(maximum) {
			maximum = end
		}
		for revision := range memberRevisions {
			if cohortRevision(cohortID, cohorts) == revision {
				matchedRevisions[revision] = struct{}{}
			}
		}
	}
	if len(sourceCohorts) != len(memberRevisions) || len(matchedRevisions) != len(memberRevisions) || !minimum.Equal(observationStart) || !maximum.Equal(observationEnd) {
		return reject(rejectTruthAxes)
	}
	return nil
}

func cohortRevision(id string, cohorts map[string]cohortState) string {
	return cohorts[id].sourceRevision
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "/\\:@ \t\r\n") {
		return false
	}
	for _, character := range value {
		if !(character == '-' || character == '_' || character == '.' || character >= '0' && character <= '9' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z') {
			return false
		}
	}
	return true
}

func verifyIssues(value any, sourceIDs, referencedIssueIDs map[string]struct{}) (map[string]struct{}, error) {
	issues, err := array(value)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(issues))
	previous := ""
	for _, issueValue := range issues {
		issue, err := object(issueValue)
		if err != nil {
			return nil, err
		}
		if err := exactFields(issue, "id", "code", "severity", "sourceId", "observed", "limit"); err != nil {
			return nil, err
		}
		id, err := stringValue(issue["id"])
		if err != nil || !issueIDRE.MatchString(id) {
			return nil, reject(rejectIdentity)
		}
		if previous >= id && previous != "" {
			return nil, reject(rejectOrdering)
		}
		previous = id
		seen[id] = struct{}{}
		if _, err := enum(issue["code"], issueCodes); err != nil {
			return nil, err
		}
		if _, err := enum(issue["severity"], set("INFO", "WARNING", "ERROR")); err != nil {
			return nil, err
		}
		source, present, err := nullableString(issue["sourceId"])
		if err != nil {
			return nil, err
		}
		if present {
			if _, exists := sourceIDs[source]; !exists {
				return nil, reject(rejectIdentity)
			}
		}
		if _, _, err := decimal(issue["observed"], true); err != nil {
			return nil, err
		}
		if _, _, err := decimal(issue["limit"], true); err != nil {
			return nil, err
		}
		basisBytes, err := canonical(cloneWithout(issue, "id"), false)
		if err != nil || id != "dashboard-issue:"+domainDigest("corvint-dashboard-issue/0", basisBytes) {
			return nil, reject(rejectIdentity)
		}
	}
	for referenced := range referencedIssueIDs {
		if _, exists := seen[referenced]; !exists {
			return nil, reject(rejectIdentity)
		}
	}
	return seen, nil
}

var memberTerminalIssueCodes = set(
	"SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_INVALID_IDENTITY",
	"SOURCE_INVALID_SCHEMA", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_OVERSIZED",
	"SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK", "TRACE_ANCESTRY_BOUND", "VERIFIER_REJECTED",
)

var aggregateOverrideIssueCodes = set("STORE_CHANGED", "TRACE_STORE_BOUND", "REPOSITORY_OBJECT_UNAVAILABLE")

var localTraceIssueCodes = set(
	"OBSERVATION_TIME_UNKNOWN", "REPOSITORY_OBJECT_UNAVAILABLE", "SOURCE_CHANGED_DURING_READ",
	"SOURCE_INACCESSIBLE", "SOURCE_INVALID_IDENTITY", "SOURCE_INVALID_SCHEMA",
	"SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED",
	"SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK", "STORE_CHANGED", "TRACE_ANCESTRY_BOUND",
	"TRACE_STORE_BOUND", "UNSUPPORTED_OBJECT_ALTERNATES", "VERIFIER_REJECTED",
)

func verifyTraceIssueSemantics(sourcesValue, issuesValue any) error {
	sources, err := array(sourcesValue)
	if err != nil {
		return err
	}
	issues, err := array(issuesValue)
	if err != nil {
		return err
	}
	bySource := make(map[string][]map[string]any)
	for _, issueValue := range issues {
		issue, objectErr := object(issueValue)
		if objectErr != nil {
			return objectErr
		}
		sourceID, present, stringErr := nullableString(issue["sourceId"])
		if stringErr != nil {
			return stringErr
		}
		if present {
			bySource[sourceID] = append(bySource[sourceID], issue)
		}
	}
	for _, sourceValue := range sources {
		source, objectErr := object(sourceValue)
		if objectErr != nil {
			return objectErr
		}
		if source["adapterId"] != "local-trace-v1" {
			continue
		}
		sourceID, _ := stringValue(source["id"])
		exclusions, exclusionErr := sortedUniqueStrings(source["exclusions"], issueIDRE)
		if exclusionErr != nil {
			return exclusionErr
		}
		terminalIDs := make([]string, 0)
		observationUnknown := 0
		overrideIDs := make([]string, 0, 1)
		terminalCodes := make(map[string]struct{})
		for _, issue := range bySource[sourceID] {
			code, _ := stringValue(issue["code"])
			id, _ := stringValue(issue["id"])
			if _, allowed := localTraceIssueCodes[code]; !allowed {
				return reject(rejectTruthAxes)
			}
			observed, hasObserved, decimalErr := decimal(issue["observed"], true)
			if decimalErr != nil {
				return decimalErr
			}
			if code == "OBSERVATION_TIME_UNKNOWN" {
				if issue["severity"] != "INFO" || hasObserved || issue["limit"] != nil {
					return reject(rejectTruthAxes)
				}
				observationUnknown++
			}
			if _, exists := aggregateOverrideIssueCodes[code]; exists {
				validLimit := issue["limit"] == nil
				if code == "TRACE_STORE_BOUND" {
					limit, present, limitErr := decimal(issue["limit"], false)
					validLimit = limitErr == nil && present && (limit == "1000" || limit == "262144" || limit == "16777216")
				}
				if issue["severity"] != "ERROR" || hasObserved || !validLimit {
					return reject(rejectTruthAxes)
				}
				overrideIDs = append(overrideIDs, id)
			}
			if _, exists := memberTerminalIssueCodes[code]; exists && hasObserved && observed != "0" {
				if issue["severity"] != "ERROR" || issue["limit"] != nil {
					return reject(rejectTruthAxes)
				}
				if _, duplicate := terminalCodes[code]; duplicate {
					return reject(rejectTruthAxes)
				}
				terminalCodes[code] = struct{}{}
				terminalIDs = append(terminalIDs, id)
			}
		}
		sort.Strings(terminalIDs)
		if len(overrideIDs) != 0 {
			if len(overrideIDs) != 1 || observationUnknown != 0 || len(exclusions) != 1 || exclusions[0] != overrideIDs[0] {
				return reject(rejectTruthAxes)
			}
			continue
		}
		if source["validity"] == "NOT_PRESENT" {
			if len(bySource[sourceID]) != 1 || len(exclusions) != 1 {
				return reject(rejectTruthAxes)
			}
			issue := bySource[sourceID][0]
			if issue["code"] != "SOURCE_NOT_PRESENT" || issue["severity"] != "WARNING" || issue["observed"] != nil || issue["limit"] != nil || issue["id"] != exclusions[0] {
				return reject(rejectTruthAxes)
			}
			continue
		}
		if source["validity"] == "VALID" || source["validity"] == "INVALID" {
			if observationUnknown != 1 || !equalStrings(exclusions, terminalIDs) {
				return reject(rejectTruthAxes)
			}
		}
	}
	return nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func verifyMetricGroup(group string, value any, knownSources map[string]metricSourceState, knownCohorts map[string]cohortState, issueIDs map[string]struct{}, repository repositoryState, seen map[string]int) error {
	metrics, err := array(value)
	if err != nil {
		return err
	}
	previousKey := ""
	for _, metricValue := range metrics {
		metric, err := object(metricValue)
		if err != nil {
			return err
		}
		if err := exactFields(metric, "authorityClass", "cohortIds", "completeness", "currency", "denominator", "deliveryStage", "dimensions", "epistemicClass", "exclusions", "name", "numerator", "scopeClass", "sourceIds", "unit", "validity", "value", "window"); err != nil {
			return err
		}
		name, err := stringValue(metric["name"])
		if err != nil || metricGroup[name] != group {
			return reject(rejectMetric)
		}
		if metric["unit"] != metricUnit[name] {
			return reject(rejectMetric)
		}
		seen[name]++
		validity, err := enum(metric["validity"], validities)
		if err != nil {
			return err
		}
		epistemic, err := enum(metric["epistemicClass"], epistemicClasses)
		if err != nil {
			return err
		}
		authority, err := enum(metric["authorityClass"], authorityClasses)
		if err != nil {
			return err
		}
		completeness, err := enum(metric["completeness"], completenesses)
		if err != nil {
			return err
		}
		currency, err := enum(metric["currency"], currencies)
		if err != nil {
			return err
		}
		if _, err := enum(metric["deliveryStage"], deliveryStages); err != nil {
			return err
		}
		scope, err := enum(metric["scopeClass"], scopeClasses)
		if err != nil {
			return err
		}
		sources, err := sortedUniqueStrings(metric["sourceIds"], sourceIDRE)
		if err != nil {
			return err
		}
		for _, source := range sources {
			if _, exists := knownSources[source]; !exists {
				return reject(rejectIdentity)
			}
		}
		cohorts, err := sortedUniqueStrings(metric["cohortIds"], cohortIDRE)
		if err != nil {
			return err
		}
		for _, cohort := range cohorts {
			if _, exists := knownCohorts[cohort]; !exists {
				return reject(rejectIdentity)
			}
		}
		exclusions, err := sortedUniqueStrings(metric["exclusions"], issueIDRE)
		if err != nil {
			return err
		}
		for _, exclusion := range exclusions {
			if _, exists := issueIDs[exclusion]; !exists {
				return reject(rejectIdentity)
			}
		}
		dimensions, err := verifyDimensions(name, metric["dimensions"], repository, scope)
		if err != nil {
			return err
		}
		dimensionBytes, err := canonical(dimensions, false)
		if err != nil {
			return err
		}
		cohortBytes, err := canonical(metric["cohortIds"], false)
		if err != nil {
			return err
		}
		key := name + "\x00" + string(dimensionBytes) + "\x00" + string(cohortBytes)
		if previousKey != "" && previousKey >= key {
			return reject(rejectOrdering)
		}
		previousKey = key
		if err := verifyMetricValue(name, metric); err != nil {
			return err
		}
		if err := verifyWindow(metric["window"]); err != nil {
			return err
		}
		if _, unavailable := baseUnavailable[name]; unavailable {
			if scope != "UNAVAILABLE" || epistemic != "NOT_OBSERVED" || completeness != "UNKNOWN" || validity == "VALID" || len(dimensions) != 0 || len(sources) != 0 || len(cohorts) != 0 || metric["value"] != nil || metric["numerator"] != nil || metric["denominator"] != nil || metric["window"] != nil {
				return reject(rejectTruthAxes)
			}
		}
		if scope == "UNAVAILABLE" && (authority != "NONE" || epistemic != "NOT_OBSERVED" || completeness != "UNKNOWN" || currency != "UNKNOWN" || validity != "UNSUPPORTED" ||
			len(dimensions) != 0 || len(sources) != 0 || len(cohorts) != 0 || len(exclusions) != 0 || metric["value"] != nil || metric["numerator"] != nil || metric["denominator"] != nil || metric["window"] != nil) {
			return reject(rejectTruthAxes)
		}
		if scope == "SINGLE_COHORT" && len(cohorts) != 1 {
			return reject(rejectTruthAxes)
		}
		if name == "usage.trace.retained" && scope != "UNAVAILABLE" {
			if scope != "SINGLE_COHORT" || len(cohorts) != 1 || metricDimension(metric, "revision") != knownCohorts[cohorts[0]].sourceRevision {
				return reject(rejectIdentity)
			}
			if len(sources) == 0 {
				return reject(rejectIdentity)
			}
			for _, sourceID := range sources {
				source := knownSources[sourceID]
				if source.adapterID != "local-trace-v1" {
					return reject(rejectIdentity)
				}
				if _, contributes := source.cohortIDs[cohorts[0]]; !contributes {
					return reject(rejectIdentity)
				}
			}
		}
		if scope == "MULTI_COHORT_INVENTORY" && name != "data.artifact.count" && name != "data.artifact.bytes" {
			return reject(rejectTruthAxes)
		}
	}
	return nil
}

func verifyDimensions(name string, value any, repository repositoryState, scope string) ([]any, error) {
	dimensions, err := array(value)
	if err != nil {
		return nil, err
	}
	expected := []string{}
	if name == "data.artifact.count" || name == "data.artifact.bytes" {
		expected = []string{"adapterId", "ageBucket", "artifactKind", "authorityClass", "deliveryStage", "revision", "validity"}
	} else if name == "usage.trace.retained" && scope != "UNAVAILABLE" {
		expected = []string{"outcome", "revision"}
	}
	if len(dimensions) != len(expected) {
		return nil, reject(rejectMetric)
	}
	for index, dimensionValue := range dimensions {
		dimension, err := object(dimensionValue)
		if err != nil {
			return nil, err
		}
		if err := exactFields(dimension, "name", "value"); err != nil {
			return nil, err
		}
		if dimension["name"] != expected[index] {
			return nil, reject(rejectOrdering)
		}
		value, present, err := nullableString(dimension["value"])
		if err != nil {
			return nil, err
		}
		switch expected[index] {
		case "adapterId":
			if !present {
				return nil, reject(rejectMetric)
			}
			if _, exists := adapterIDs[value]; !exists {
				return nil, reject(rejectEnum)
			}
		case "ageBucket":
			if _, exists := ageBuckets[value]; !present || !exists {
				return nil, reject(rejectEnum)
			}
		case "artifactKind":
			if value != "CONFIGURED_SOURCE" && value != "RETAINED_MEMBER" {
				return nil, reject(rejectEnum)
			}
		case "authorityClass":
			if _, exists := authorityClasses[value]; !present || !exists {
				return nil, reject(rejectEnum)
			}
		case "deliveryStage":
			if _, exists := deliveryStages[value]; !present || !exists {
				return nil, reject(rejectEnum)
			}
		case "validity":
			if _, exists := validities[value]; !present || !exists {
				return nil, reject(rejectEnum)
			}
		case "outcome":
			if value != "passed" && value != "failed" && value != "blocked" {
				return nil, reject(rejectEnum)
			}
		case "revision":
			if name == "usage.trace.retained" && !present {
				return nil, reject(rejectMetric)
			}
			if present {
				matcher := gitSHA1RE
				if repository.objectFormat == "sha256" {
					matcher = gitSHA256RE
				}
				if repository.objectFormat == "" || !matcher.MatchString(value) {
					return nil, reject(rejectIdentity)
				}
			}
		}
	}
	return dimensions, nil
}

func verifyMetricValue(name string, metric map[string]any) error {
	value, valuePresent, err := decimal(metric["value"], true)
	if err != nil && !(name == "verification.closure" && metric["value"] != nil) {
		return err
	}
	numerator, numeratorPresent, err := decimal(metric["numerator"], true)
	if err != nil {
		return err
	}
	denominator, denominatorPresent, err := decimal(metric["denominator"], true)
	if err != nil {
		return err
	}
	if name == "verification.closure" && metric["value"] != nil {
		fraction, err := stringValue(metric["value"])
		if err != nil {
			return err
		}
		parts := strings.Split(fraction, "/")
		if len(parts) != 2 || !decimalRE.MatchString(parts[0]) || !decimalRE.MatchString(parts[1]) || parts[1] == "0" || !numeratorPresent || !denominatorPresent || parts[0] != numerator || parts[1] != denominator {
			return reject(rejectMetric)
		}
		valuePresent = true
	} else if valuePresent && (!numeratorPresent || value != numerator) {
		return reject(rejectMetric)
	}
	if valuePresent != numeratorPresent || denominatorPresent && !numeratorPresent || metric["completeness"] == "COMPLETE" && numeratorPresent != denominatorPresent {
		return reject(rejectMetric)
	}
	if numeratorPresent {
		numeratorValue, numeratorOK := new(big.Int).SetString(numerator, 10)
		if !numeratorOK {
			return reject(rejectMetric)
		}
		var denominatorValue *big.Int
		if denominatorPresent {
			var denominatorOK bool
			denominatorValue, denominatorOK = new(big.Int).SetString(denominator, 10)
			if !denominatorOK || numeratorValue.Cmp(denominatorValue) > 0 {
				return reject(rejectMetric)
			}
		}
		limit := uint64(0)
		switch name {
		case "usage.trace.retained":
			limit = maxTraceRows
		case "data.artifact.count":
			if metricArtifactKind(metric) == "RETAINED_MEMBER" {
				limit = maxTraceRows
			} else {
				limit = 10_000
			}
		case "data.artifact.bytes":
			if metricArtifactKind(metric) == "RETAINED_MEMBER" {
				limit = maxTraceStoreBytes
			} else {
				limit = 256 << 20
			}
		}
		if limit != 0 && (!numeratorValue.IsUint64() || numeratorValue.Uint64() > limit || denominatorPresent && (!denominatorValue.IsUint64() || denominatorValue.Uint64() > limit)) {
			return reject(rejectSize)
		}
	}
	return nil
}

func metricArtifactKind(metric map[string]any) string {
	dimensions, _ := array(metric["dimensions"])
	for _, dimensionValue := range dimensions {
		dimension, err := object(dimensionValue)
		if err == nil && dimension["name"] == "artifactKind" {
			value, _ := stringValue(dimension["value"])
			return value
		}
	}
	return ""
}

func verifyWindow(value any) error {
	if value == nil {
		return nil
	}
	window, err := object(value)
	if err != nil {
		return err
	}
	if err := exactFields(window, "end", "start"); err != nil {
		return err
	}
	start, _, err := timestamp(window["start"], false)
	if err != nil {
		return err
	}
	end, _, err := timestamp(window["end"], false)
	if err != nil {
		return err
	}
	if end.Before(start) {
		return reject(rejectTimestamp)
	}
	return nil
}

func verifyMetricCompleteness(seen map[string]int) error {
	for name := range baseUnavailable {
		if seen[name] != 1 {
			return reject(rejectMetricSet)
		}
	}
	if seen["data.artifact.count"] == 0 || seen["data.artifact.bytes"] == 0 {
		return reject(rejectMetricSet)
	}
	if seen["usage.trace.retained"] == 0 {
		return reject(rejectMetricSet)
	}
	return nil
}

type inventoryGroup struct {
	metric       map[string]any
	numerator    big.Int
	artifactKind string
}

func verifyTraceInventory(dataValue, sourcesValue, issuesValue any, cohorts map[string]cohortState) error {
	sources, err := array(sourcesValue)
	if err != nil || len(sources) == 0 {
		return err
	}

	if err := verifyRejectedMemberArithmetic(dataValue, sources, issuesValue); err != nil {
		return err
	}

	// Public members are sufficient to recompute inventory only for a stable,
	// complete local-trace universe. Partial/invalid member arithmetic also
	// depends on the owning terminal-issue classification and is checked by the
	// separate trace-semantic verifier.
	traceSources := make([]map[string]any, 0, len(sources))
	var configuredBytes, retainedBytes big.Int
	retainedCount := int64(0)
	for _, sourceValue := range sources {
		source, objectErr := object(sourceValue)
		if objectErr != nil {
			return objectErr
		}
		if source["adapterId"] != "local-trace-v1" || source["validity"] != "VALID" || source["completeness"] != "COMPLETE" {
			return nil
		}
		bytesText, _, decimalErr := decimal(source["byteCount"], false)
		if decimalErr != nil {
			return decimalErr
		}
		byteValue, ok := new(big.Int).SetString(bytesText, 10)
		if !ok {
			return reject(rejectDecimal)
		}
		configuredBytes.Add(&configuredBytes, byteValue)
		members, memberErr := array(source["members"])
		if memberErr != nil {
			return memberErr
		}
		for _, memberValue := range members {
			member, memberErr := object(memberValue)
			if memberErr != nil {
				return memberErr
			}
			memberBytes, _, memberErr := decimal(member["byteCount"], false)
			if memberErr != nil {
				return memberErr
			}
			value, ok := new(big.Int).SetString(memberBytes, 10)
			if !ok {
				return reject(rejectDecimal)
			}
			retainedBytes.Add(&retainedBytes, value)
			retainedCount++
		}
		traceSources = append(traceSources, source)
	}

	dimensions := func(source map[string]any, kind string, revision any, validity string) []any {
		return []any{
			map[string]any{"name": "adapterId", "value": "local-trace-v1"},
			map[string]any{"name": "ageBucket", "value": "UNKNOWN"},
			map[string]any{"name": "artifactKind", "value": kind},
			map[string]any{"name": "authorityClass", "value": source["authorityClass"]},
			map[string]any{"name": "deliveryStage", "value": source["deliveryStage"]},
			map[string]any{"name": "revision", "value": revision},
			map[string]any{"name": "validity", "value": validity},
		}
	}
	groups := make(map[string]*inventoryGroup)
	add := func(source map[string]any, name, kind string, revision any, cohortIDs []any, numerator string) error {
		dims := dimensions(source, kind, revision, "VALID")
		dimensionBytes, canonicalErr := canonical(dims, false)
		if canonicalErr != nil {
			return canonicalErr
		}
		cohortBytes, canonicalErr := canonical(cohortIDs, false)
		if canonicalErr != nil {
			return canonicalErr
		}
		key := name + "\x00" + string(dimensionBytes) + "\x00" + string(cohortBytes)
		group := groups[key]
		if group == nil {
			group = &inventoryGroup{artifactKind: kind, metric: map[string]any{
				"authorityClass": source["authorityClass"], "cohortIds": cohortIDs, "completeness": "COMPLETE",
				"currency": source["currency"], "denominator": nil, "deliveryStage": source["deliveryStage"],
				"dimensions": dims, "epistemicClass": "OBSERVED", "exclusions": []any{},
				"name": name, "numerator": nil, "scopeClass": "MULTI_COHORT_INVENTORY",
				"sourceIds": []any{}, "unit": metricUnit[name], "validity": "VALID", "value": nil, "window": nil,
			}}
			groups[key] = group
		} else if group.metric["authorityClass"] != source["authorityClass"] || group.metric["currency"] != source["currency"] || group.metric["deliveryStage"] != source["deliveryStage"] {
			return reject(rejectMetric)
		}
		exclusions, exclusionErr := array(source["exclusions"])
		if exclusionErr != nil {
			return exclusionErr
		}
		group.metric["exclusions"] = append(group.metric["exclusions"].([]any), exclusions...)
		value, ok := new(big.Int).SetString(numerator, 10)
		if !ok {
			return reject(rejectDecimal)
		}
		group.numerator.Add(&group.numerator, value)
		sourceID, stringErr := stringValue(source["id"])
		if stringErr != nil {
			return stringErr
		}
		ids := group.metric["sourceIds"].([]any)
		if len(ids) == 0 || ids[len(ids)-1] != sourceID {
			group.metric["sourceIds"] = append(ids, sourceID)
		}
		return nil
	}

	for _, source := range traceSources {
		cohortValues, cohortErr := array(source["cohortIds"])
		if cohortErr != nil {
			return cohortErr
		}
		if err := add(source, "data.artifact.count", "CONFIGURED_SOURCE", nil, cohortValues, "1"); err != nil {
			return err
		}
		if err := add(source, "data.artifact.bytes", "CONFIGURED_SOURCE", nil, cohortValues, source["byteCount"].(string)); err != nil {
			return err
		}
		members, _ := array(source["members"])
		if len(members) == 0 {
			if err := add(source, "data.artifact.count", "RETAINED_MEMBER", nil, []any{}, "0"); err != nil {
				return err
			}
			if err := add(source, "data.artifact.bytes", "RETAINED_MEMBER", nil, []any{}, "0"); err != nil {
				return err
			}
			continue
		}
		for _, memberValue := range members {
			member, _ := object(memberValue)
			revision := member["revision"].(string)
			memberCohorts := make([]any, 0, 1)
			for _, cohortValue := range cohortValues {
				cohortID := cohortValue.(string)
				if cohorts[cohortID].sourceRevision == revision {
					memberCohorts = append(memberCohorts, cohortID)
				}
			}
			if len(memberCohorts) != 1 {
				return reject(rejectIdentity)
			}
			if err := add(source, "data.artifact.count", "RETAINED_MEMBER", revision, memberCohorts, "1"); err != nil {
				return err
			}
			if err := add(source, "data.artifact.bytes", "RETAINED_MEMBER", revision, memberCohorts, member["byteCount"].(string)); err != nil {
				return err
			}
		}
	}

	configuredCountText := strconv.Itoa(len(traceSources))
	retainedCountText := strconv.FormatInt(retainedCount, 10)
	expected := make([]any, 0, len(groups))
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		group := groups[key]
		exclusions := group.metric["exclusions"].([]any)
		sort.Slice(exclusions, func(left, right int) bool { return exclusions[left].(string) < exclusions[right].(string) })
		if len(exclusions) > 1 {
			unique := exclusions[:1]
			for _, exclusion := range exclusions[1:] {
				if exclusion != unique[len(unique)-1] {
					unique = append(unique, exclusion)
				}
			}
			exclusions = unique
		}
		group.metric["exclusions"] = exclusions
		value := group.numerator.String()
		group.metric["value"] = value
		group.metric["numerator"] = value
		name := group.metric["name"].(string)
		switch {
		case group.artifactKind == "CONFIGURED_SOURCE" && name == "data.artifact.count":
			group.metric["denominator"] = configuredCountText
		case group.artifactKind == "CONFIGURED_SOURCE":
			group.metric["denominator"] = configuredBytes.String()
		case name == "data.artifact.count":
			group.metric["denominator"] = retainedCountText
		default:
			group.metric["denominator"] = retainedBytes.String()
		}
		expected = append(expected, group.metric)
	}
	actualBytes, err := canonical(dataValue, false)
	if err != nil {
		return err
	}
	expectedBytes, err := canonical(expected, false)
	if err != nil || !bytes.Equal(actualBytes, expectedBytes) {
		return reject(rejectMetric)
	}
	return nil
}

func verifyRejectedMemberArithmetic(dataValue any, sources []any, issuesValue any) error {
	issues, err := array(issuesValue)
	if err != nil {
		return err
	}
	data, err := array(dataValue)
	if err != nil {
		return err
	}
	for _, sourceValue := range sources {
		source, objectErr := object(sourceValue)
		if objectErr != nil {
			return objectErr
		}
		if source["adapterId"] != "local-trace-v1" || source["completeness"] != "PARTIAL" {
			continue
		}
		sourceID, _ := stringValue(source["id"])
		terminalIDs := make([]string, 0)
		var rejected big.Int
		for _, issueValue := range issues {
			issue, _ := object(issueValue)
			if issue["sourceId"] != sourceID {
				continue
			}
			code, _ := stringValue(issue["code"])
			if _, terminal := memberTerminalIssueCodes[code]; !terminal {
				continue
			}
			observed, present, observedErr := decimal(issue["observed"], true)
			if observedErr != nil {
				return observedErr
			}
			if present && observed != "0" {
				value, ok := new(big.Int).SetString(observed, 10)
				if !ok {
					return reject(rejectDecimal)
				}
				rejected.Add(&rejected, value)
				terminalIDs = append(terminalIDs, issue["id"].(string))
			}
		}
		if rejected.Sign() == 0 {
			continue
		}
		sort.Strings(terminalIDs)
		matched := 0
		for _, metricValue := range data {
			metric, _ := object(metricValue)
			if metric["name"] != "data.artifact.count" || metricArtifactKind(metric) != "RETAINED_MEMBER" || metricDimension(metric, "validity") != "INVALID" {
				continue
			}
			sourceIDs, _ := sortedUniqueStrings(metric["sourceIds"], sourceIDRE)
			cohortIDs, _ := sortedUniqueStrings(metric["cohortIds"], cohortIDRE)
			exclusions, _ := sortedUniqueStrings(metric["exclusions"], issueIDRE)
			if len(sourceIDs) == 1 && sourceIDs[0] == sourceID {
				matched++
				if len(cohortIDs) != 0 || !equalStrings(exclusions, terminalIDs) || metric["value"] != rejected.String() || metric["numerator"] != rejected.String() || metric["denominator"] != nil || metric["completeness"] != "PARTIAL" {
					return reject(rejectMetric)
				}
			}
		}
		if matched != 1 {
			return reject(rejectMetric)
		}
	}
	return nil
}

func metricDimension(metric map[string]any, name string) any {
	dimensions, _ := array(metric["dimensions"])
	for _, dimensionValue := range dimensions {
		dimension, _ := object(dimensionValue)
		if dimension["name"] == name {
			return dimension["value"]
		}
	}
	return nil
}

func verifyScanState(observationValue, sourcesValue any) error {
	observation, err := object(observationValue)
	if err != nil {
		return err
	}
	sources, err := array(sourcesValue)
	if err != nil {
		return err
	}
	partial := false
	invalid := false
	for _, sourceValue := range sources {
		source, err := object(sourceValue)
		if err != nil {
			return err
		}
		if source["completeness"] == "PARTIAL" || source["validity"] == "INACCESSIBLE" || source["validity"] == "EXPIRED" {
			partial = true
		}
		if source["validity"] == "INVALID" {
			partial = true
			invalid = true
		}
	}
	switch observation["scanState"] {
	case "COMPLETE":
		if partial {
			return reject(rejectTruthAxes)
		}
	case "PARTIAL":
		if !partial {
			return reject(rejectTruthAxes)
		}
	case "INVALID":
		if !invalid {
			return reject(rejectTruthAxes)
		}
	}
	return nil
}

func verifyPrivacy(value any) error {
	privacy, err := object(value)
	if err != nil {
		return err
	}
	if err := exactFields(privacy, "collection", "outboundNetwork", "pathDisclosure", "rawBodies", "threatBoundary"); err != nil {
		return err
	}
	expected := map[string]string{
		"collection": "DISABLED", "outboundNetwork": "NONE", "pathDisclosure": "NONE",
		"rawBodies": "EXCLUDED", "threatBoundary": "LOCAL_ACCOUNT_NOT_DEFENDED",
	}
	for key, expectedValue := range expected {
		if privacy[key] != expectedValue {
			return reject(rejectPrivacy)
		}
	}
	return nil
}

func verifySnapshotHash(root map[string]any) error {
	claimed, err := stringValue(root["snapshotSha256"])
	if err != nil || !digestRE.MatchString(claimed) {
		return reject(rejectDigest)
	}
	root["snapshotSha256"] = nil
	preimage, err := canonical(root, false)
	root["snapshotSha256"] = claimed
	if err != nil {
		return err
	}
	hash := sha256.New()
	hash.Write([]byte(schema))
	hash.Write([]byte{0})
	hash.Write(preimage)
	want := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	if claimed != want {
		return reject(rejectHash)
	}
	return nil
}

func verifyPath(path string) error {
	raw, err := readBoundedFile(path, maxSnapshotBytes)
	if err != nil {
		return reject(rejectJSON)
	}
	return verifySnapshot(raw)
}

func readBoundedFile(path string, maximum int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil || len(raw) > maximum {
		return nil, reject(rejectSize)
	}
	return raw, nil
}

func runTraceConformance(args []string, stdout, stderr io.Writer) int {
	arguments, err := parseTraceCLIArguments(args)
	if err != nil {
		fmt.Fprintln(stderr, `{"code":"CONFORMANCE_INVALID_ARGUMENT","profile":"corvint-dashboard-trace-adapter-conformance/0"}`)
		return 2
	}
	manifestRaw, err := readBoundedFile(arguments.manifest, maxTraceManifestSize)
	if err != nil {
		fmt.Fprintln(stderr, `{"code":"TRACE_MANIFEST","profile":"corvint-dashboard-trace-adapter-conformance/0"}`)
		return 1
	}
	manifest, err := parseTraceManifest(manifestRaw)
	if err != nil {
		fmt.Fprintf(stderr, "{\"code\":%q,\"profile\":\"corvint-dashboard-trace-adapter-conformance/0\"}\n", rejectionCode(err))
		return 1
	}
	actual, err := readBoundedFile(arguments.snapshot, maxSnapshotBytes)
	if err != nil || verifySnapshot(actual) != nil {
		fmt.Fprintln(stderr, `{"code":"TRACE_SNAPSHOT_INVALID","profile":"corvint-dashboard-trace-adapter-conformance/0"}`)
		return 1
	}
	corpus, err := validateTraceCorpus(arguments.root, manifest)
	if err != nil {
		fmt.Fprintf(stderr, "{\"code\":%q,\"profile\":\"corvint-dashboard-trace-adapter-conformance/0\"}\n", rejectionCode(err))
		return 1
	}
	expected, err := compileExpectedTraceSnapshot(manifest, corpus)
	if err != nil || verifySnapshot(expected) != nil || !bytes.Equal(actual, expected) {
		fmt.Fprintln(stderr, `{"code":"TRACE_SNAPSHOT_MISMATCH","profile":"corvint-dashboard-trace-adapter-conformance/0"}`)
		return 1
	}
	fmt.Fprintln(stdout, `{"profile":"corvint-dashboard-trace-adapter-conformance/0","status":"PASS"}`)
	return 0
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 && args[0] == "verify" {
		return runTraceConformance(args, stdout, stderr)
	}
	if len(args) != 2 || args[0] != "--snapshot" || args[1] == "" {
		fmt.Fprintln(stderr, `{"code":"CONFORMANCE_INVALID_ARGUMENT","profile":"`+profile+`"}`)
		return 2
	}
	err := verifyPath(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "{\"code\":%q,\"profile\":%q}\n", rejectionCode(err), profile)
		return 1
	}
	fmt.Fprintf(stdout, "{\"profile\":%q,\"status\":\"PASS\"}\n", profile)
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
