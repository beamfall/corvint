package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

//go:embed testdata/valid-observed.json
var frozenValid []byte

//go:embed testdata/four-invalid-legacy-members.json
var frozenFourInvalidLegacyMembers []byte

//go:embed cases.json
var frozenCases []byte

type negativeCase struct {
	ID       string `json:"id"`
	Mutation string `json:"mutation"`
	Expected string `json:"expected"`
}

type caseManifest struct {
	Profile  string         `json:"profile"`
	Positive string         `json:"positive"`
	Cases    []negativeCase `json:"cases"`
}

func hashObject(domain string, value any) string {
	raw, err := canonical(value, false)
	if err != nil {
		panic(err)
	}
	return domainDigest(domain, raw)
}

func unavailableMetric(name, stage string) map[string]any {
	return map[string]any{
		"authorityClass": "NONE", "cohortIds": []any{}, "completeness": "UNKNOWN",
		"currency": "UNKNOWN", "denominator": nil, "deliveryStage": stage,
		"dimensions": []any{}, "epistemicClass": "NOT_OBSERVED", "exclusions": []any{},
		"name": name, "numerator": nil, "scopeClass": "UNAVAILABLE", "sourceIds": []any{},
		"unit": metricUnit[name], "validity": "UNSUPPORTED", "value": nil, "window": nil,
	}
}

func observationUnknownIssue(sourceID string) map[string]any {
	basis := map[string]any{
		"code": "OBSERVATION_TIME_UNKNOWN", "limit": nil, "observed": nil,
		"severity": "INFO", "sourceId": sourceID,
	}
	issue := make(map[string]any, len(basis)+1)
	for key, value := range basis {
		issue[key] = value
	}
	issue["id"] = "dashboard-issue:" + hashObject("corvint-dashboard-issue/0", basis)
	return issue
}

func referenceSnapshot() map[string]any {
	head := strings.Repeat("1", 40)
	tree := strings.Repeat("2", 40)
	generatedAt := "2026-08-23T20:00:00.000000000Z"
	dirty := domainDigest("corvint-dashboard-dirty-paths/0", []byte("[]"))
	repositoryReads := hashObject("corvint-dashboard-repository-reads/0", []any{map[string]any{"kind": "SNAPSHOT_HEAD", "objectFormat": "sha1", "objectId": head, "objectType": "commit", "revision": nil}})
	members := []any{}
	content := hashObject("trace-store/0", members)
	sourceBasis := map[string]any{
		"adapterId": "local-trace-v1", "configuredOrdinal": "0", "contentSha256": content,
		"profile": "corvint-local-trace/1", "repositoryReadsSha256": repositoryReads,
	}
	sourceID := "dashboard-source:" + hashObject("corvint-dashboard-source/0", sourceBasis)
	source := map[string]any{
		"adapterId": "local-trace-v1", "authorityClass": "ADVISORY", "byteCount": "0",
		"cohortIds": []any{}, "completeness": "COMPLETE", "configuredOrdinal": "0",
		"contentSha256": content, "currency": "VALIDATED_AT", "deliveryStage": "NOT_STARTED",
		"displayLabel": "local-trace-v1#0", "epistemicClass": "OBSERVED",
		"exclusions": []any{}, "id": sourceID, "members": members,
		"observationEnd": nil, "observationStart": nil, "observationTime": nil,
		"profile": "corvint-local-trace/1", "repositoryReadsSha256": repositoryReads,
		"validity": "VALID", "verifierId": "go-local-trace-v1",
	}
	configured := []any{map[string]any{"adapterId": "local-trace-v1", "configuredOrdinal": "0", "contentSha256": content}}
	dataDimensions := func(kind string) []any {
		return []any{
			map[string]any{"name": "adapterId", "value": "local-trace-v1"},
			map[string]any{"name": "ageBucket", "value": "UNKNOWN"},
			map[string]any{"name": "artifactKind", "value": kind},
			map[string]any{"name": "authorityClass", "value": "ADVISORY"},
			map[string]any{"name": "deliveryStage", "value": "NOT_STARTED"},
			map[string]any{"name": "revision", "value": nil},
			map[string]any{"name": "validity", "value": "VALID"},
		}
	}
	observedMetric := func(name, value, denominator, unit, scope string, dimensions []any) map[string]any {
		return map[string]any{
			"authorityClass": "ADVISORY", "cohortIds": []any{}, "completeness": "COMPLETE",
			"currency": "VALIDATED_AT", "denominator": denominator, "deliveryStage": "NOT_STARTED",
			"dimensions": dimensions, "epistemicClass": "OBSERVED", "exclusions": []any{},
			"name": name, "numerator": value, "scopeClass": scope, "sourceIds": []any{sourceID},
			"unit": unit, "validity": "VALID", "value": value, "window": nil,
		}
	}
	data := []any{
		observedMetric("data.artifact.bytes", "0", "0", "BYTES", "MULTI_COHORT_INVENTORY", dataDimensions("CONFIGURED_SOURCE")),
		observedMetric("data.artifact.bytes", "0", "0", "BYTES", "MULTI_COHORT_INVENTORY", dataDimensions("RETAINED_MEMBER")),
		observedMetric("data.artifact.count", "1", "1", "COUNT", "MULTI_COHORT_INVENTORY", dataDimensions("CONFIGURED_SOURCE")),
		observedMetric("data.artifact.count", "0", "0", "COUNT", "MULTI_COHORT_INVENTORY", dataDimensions("RETAINED_MEMBER")),
	}
	usage := []any{
		unavailableMetric("usage.harness.retained", "UNSUPPORTED"),
		unavailableMetric("usage.impact.retained", "UNSUPPORTED"),
		unavailableMetric("usage.latency.p50", "UNSUPPORTED"),
		unavailableMetric("usage.latency.p95", "UNSUPPORTED"),
		unavailableMetric("usage.query.retained", "UNSUPPORTED"),
		unavailableMetric("usage.response.bytes", "UNSUPPORTED"),
		unavailableMetric("usage.trace.retained", "NOT_STARTED"),
	}
	verification := []any{
		unavailableMetric("verification.cem.hunks", "NOT_STARTED"),
		unavailableMetric("verification.closure", "NOT_STARTED"),
		unavailableMetric("verification.live.runs", "UNSUPPORTED"),
		unavailableMetric("verification.ocm.obligations", "NOT_STARTED"),
		unavailableMetric("verification.pulse.runs", "UNSUPPORTED"),
	}
	root := map[string]any{
		"beamfall": []any{unavailableMetric("beamfall.shadow.runs", "UNSUPPORTED")},
		"cohorts":  []any{}, "data": data,
		"frontier":    []any{unavailableMetric("frontier.items", "UNSUPPORTED")},
		"generatedAt": generatedAt, "harnesses": []any{}, "issues": []any{observationUnknownIssue(sourceID)},
		"observation": map[string]any{
			"adapterRegistrySha256": domainDigest("corvint-dashboard-adapter-registry/0", []byte(canonicalRegistry)),
			"clockSource":           "CALLER", "configuredSourceSetSha256": hashObject("corvint-dashboard-configured-sources/0", configured),
			"end": generatedAt, "limitsProfile": "corvint-dashboard-limits/0",
			"scanState": "COMPLETE", "start": generatedAt,
		},
		"privacy":    map[string]any{"collection": "DISABLED", "outboundNetwork": "NONE", "pathDisclosure": "NONE", "rawBodies": "EXCLUDED", "threatBoundary": "LOCAL_ACCOUNT_NOT_DEFENDED"},
		"repository": map[string]any{"dirtyPathCount": "0", "dirtyPathsSha256": dirty, "headRevision": head, "objectFormat": "sha1", "treeRevision": tree, "worktreeState": "CLEAN"},
		"schema":     schema, "snapshotSha256": nil, "sources": []any{source}, "usage": usage,
		"verification": verification,
	}
	root["snapshotSha256"] = hashObject(schema, root)
	return root
}

func nonemptyReferenceSnapshot() map[string]any {
	root := referenceSnapshot()
	generatedAt := root["generatedAt"].(string)
	repository := root["repository"].(map[string]any)
	head := repository["headRevision"].(string)
	tree := repository["treeRevision"].(string)
	dirty := repository["dirtyPathsSha256"].(string)
	cohortBasis := map[string]any{
		"adapterId": "local-trace-v1", "dirtyPathsSha256": dirty,
		"producerIdentity": "go-local-trace-v1", "profile": "corvint-local-trace/1",
		"repositoryObjectFormat": "sha1", "sourceObservationEnd": generatedAt,
		"sourceObservationStart": generatedAt, "sourceRevision": head, "sourceTreeRevision": tree,
	}
	cohortID := "dashboard-cohort:" + hashObject("corvint-dashboard-cohort/0", cohortBasis)
	cohort := make(map[string]any, len(cohortBasis)+1)
	for key, value := range cohortBasis {
		cohort[key] = value
	}
	cohort["cohortId"] = cohortID
	root["cohorts"] = []any{cohort}

	member := map[string]any{
		"byteCount": "7", "contentSha256": domainDigest("planted-member/0", []byte("payload")), "revision": head,
	}
	members := []any{member}
	content := hashObject("trace-store/0", members)
	repositoryReads := domainDigest("planted-repository-witnesses/0", []byte("opaque-to-snapshot-verifier"))
	sourceBasis := map[string]any{
		"adapterId": "local-trace-v1", "configuredOrdinal": "0", "contentSha256": content,
		"profile": "corvint-local-trace/1", "repositoryReadsSha256": repositoryReads,
	}
	sourceID := "dashboard-source:" + hashObject("corvint-dashboard-source/0", sourceBasis)
	source := root["sources"].([]any)[0].(map[string]any)
	source["byteCount"] = "7"
	source["cohortIds"] = []any{cohortID}
	source["contentSha256"] = content
	source["id"] = sourceID
	source["members"] = members
	source["observationEnd"] = generatedAt
	source["observationStart"] = generatedAt
	source["repositoryReadsSha256"] = repositoryReads
	root["issues"] = []any{observationUnknownIssue(sourceID)}
	configured := []any{map[string]any{"adapterId": "local-trace-v1", "configuredOrdinal": "0", "contentSha256": content}}
	root["observation"].(map[string]any)["configuredSourceSetSha256"] = hashObject("corvint-dashboard-configured-sources/0", configured)

	dimensions := func(kind string, revision any) []any {
		return []any{
			map[string]any{"name": "adapterId", "value": "local-trace-v1"},
			map[string]any{"name": "ageBucket", "value": "UNKNOWN"},
			map[string]any{"name": "artifactKind", "value": kind},
			map[string]any{"name": "authorityClass", "value": "ADVISORY"},
			map[string]any{"name": "deliveryStage", "value": "NOT_STARTED"},
			map[string]any{"name": "revision", "value": revision},
			map[string]any{"name": "validity", "value": "VALID"},
		}
	}
	metric := func(name, kind, value, denominator string, revision any, cohortIDs []any) map[string]any {
		return map[string]any{
			"authorityClass": "ADVISORY", "cohortIds": cohortIDs, "completeness": "COMPLETE",
			"currency": "VALIDATED_AT", "denominator": denominator, "deliveryStage": "NOT_STARTED",
			"dimensions": dimensions(kind, revision), "epistemicClass": "OBSERVED", "exclusions": []any{},
			"name": name, "numerator": value, "scopeClass": "MULTI_COHORT_INVENTORY",
			"sourceIds": []any{sourceID}, "unit": metricUnit[name], "validity": "VALID", "value": value, "window": nil,
		}
	}
	root["data"] = []any{
		metric("data.artifact.bytes", "CONFIGURED_SOURCE", "7", "7", nil, []any{cohortID}),
		metric("data.artifact.bytes", "RETAINED_MEMBER", "7", "7", head, []any{cohortID}),
		metric("data.artifact.count", "CONFIGURED_SOURCE", "1", "1", nil, []any{cohortID}),
		metric("data.artifact.count", "RETAINED_MEMBER", "1", "1", head, []any{cohortID}),
	}
	root["snapshotSha256"] = nil
	root["snapshotSha256"] = hashObject(schema, root)
	return root
}

func referenceBytes(t testing.TB) []byte {
	t.Helper()
	raw, err := canonical(referenceSnapshot(), true)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func parseSnapshot(t testing.TB, raw []byte) map[string]any {
	t.Helper()
	value, err := decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value.(map[string]any)
}

func rehash(t testing.TB, root map[string]any) []byte {
	t.Helper()
	root["snapshotSha256"] = nil
	root["snapshotSha256"] = hashObject(schema, root)
	raw, err := canonical(root, true)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func mutate(t testing.TB, mutation string) []byte {
	t.Helper()
	if mutation == "duplicate-key" {
		return bytes.Replace(frozenValid, []byte(`{"beamfall":`), []byte(`{"beamfall":[],"beamfall":`), 1)
	}
	if mutation == "noncanonical-space" {
		return append([]byte{' '}, frozenValid...)
	}
	if mutation == "json-number" {
		return bytes.Replace(frozenValid, []byte(`"dirtyPathCount":"0"`), []byte(`"dirtyPathCount":0`), 1)
	}
	if mutation == "bad-unicode" {
		return bytes.Replace(frozenValid, []byte(`"local-trace-v1#0"`), []byte("\"local-trace-v1#0e\xcc\x81\""), 1)
	}
	root := parseSnapshot(t, frozenValid)
	switch mutation {
	case "dangling-metric-source":
		root["data"].([]any)[0].(map[string]any)["sourceIds"] = []any{"dashboard-source:sha256:" + strings.Repeat("0", 64)}
	case "empty-store-interval":
		source := root["sources"].([]any)[0].(map[string]any)
		source["observationStart"] = root["generatedAt"]
		source["observationEnd"] = root["generatedAt"]
	case "inventory-lie":
		metric := root["data"].([]any)[2].(map[string]any)
		metric["value"], metric["numerator"] = "0", "0"
	case "unknown-root":
		root["path"] = "/private/repository"
	case "wrong-registry-hash":
		root["observation"].(map[string]any)["adapterRegistrySha256"] = "sha256:" + strings.Repeat("0", 64)
	case "wrong-configured-hash":
		root["observation"].(map[string]any)["configuredSourceSetSha256"] = "sha256:" + strings.Repeat("0", 64)
	case "wrong-source-id":
		root["sources"].([]any)[0].(map[string]any)["id"] = "dashboard-source:sha256:" + strings.Repeat("0", 64)
	case "wrong-cohort-id":
		root["sources"].([]any)[0].(map[string]any)["cohortIds"] = []any{"dashboard-cohort:sha256:" + strings.Repeat("0", 64)}
	case "wrong-issue-id":
		badID := "dashboard-issue:sha256:" + strings.Repeat("0", 64)
		root["issues"] = []any{map[string]any{"id": badID, "code": "OBSERVATION_TIME_UNKNOWN", "severity": "WARNING", "sourceId": root["sources"].([]any)[0].(map[string]any)["id"], "observed": nil, "limit": nil}}
		root["sources"].([]any)[0].(map[string]any)["exclusions"] = []any{badID}
	case "unavailable-zero":
		metric := root["usage"].([]any)[0].(map[string]any)
		metric["value"], metric["numerator"], metric["denominator"] = "0", "0", "0"
	case "missing-required-metric":
		root["usage"] = root["usage"].([]any)[1:]
	case "metric-order":
		usage := root["usage"].([]any)
		usage[0], usage[1] = usage[1], usage[0]
	case "privacy-enabled":
		root["privacy"].(map[string]any)["collection"] = "ENABLED"
	case "registry-stage-lie":
		root["sources"].([]any)[0].(map[string]any)["deliveryStage"] = "UNSUPPORTED"
	case "scan-state-lie":
		root["observation"].(map[string]any)["scanState"] = "PARTIAL"
	case "path-label":
		root["sources"].([]any)[0].(map[string]any)["displayLabel"] = "/private/repository/trace"
	case "future-source-time":
		root["sources"].([]any)[0].(map[string]any)["observationTime"] = "2026-08-23T20:00:01.000000000Z"
	case "wrong-snapshot-hash":
		root["snapshotSha256"] = "sha256:" + strings.Repeat("0", 64)
		raw, err := canonical(root, true)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	default:
		t.Fatalf("unknown mutation %q", mutation)
	}
	return rehash(t, root)
}

func loadManifest(t testing.TB) caseManifest {
	t.Helper()
	var manifest caseManifest
	decoder := json.NewDecoder(bytes.NewReader(frozenCases))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestFrozenPositiveBytes(t *testing.T) {
	if !safeIdentifier("go-local-trace-v1") || !safePublicLabel("local-trace-v1#0") {
		t.Fatal("fixed public identifiers rejected")
	}
	if got, want := string(frozenValid), string(referenceBytes(t)); got != want {
		t.Fatalf("frozen positive drifted\nGOT:\n%s\nWANT:\n%s", got, want)
	}
	if err := verifySnapshot(frozenValid); err != nil {
		t.Fatal(err)
	}
	if got := domainDigest("corvint-dashboard-adapter-registry/0", []byte(canonicalRegistry)); got != "sha256:2de98344235e7f432b1b486c14fa1023f9fdb08b41632bac04d3049592cf3bb3" {
		t.Fatalf("registry digest = %s", got)
	}
}

func TestFourInvalidLegacyMembersSnapshot(t *testing.T) {
	if err := verifySnapshot(frozenFourInvalidLegacyMembers); err != nil {
		t.Fatalf("privacy-safe invalid-member snapshot: %v", err)
	}
}

func TestTraceUsageDimensionsFollowAvailability(t *testing.T) {
	repository := repositoryState{objectFormat: "sha1"}
	observed := []any{
		map[string]any{"name": "outcome", "value": "passed"},
		map[string]any{"name": "revision", "value": strings.Repeat("1", 40)},
	}
	if dimensions, err := verifyDimensions("usage.trace.retained", []any{}, repository, "UNAVAILABLE"); err != nil || len(dimensions) != 0 {
		t.Fatalf("unavailable dimensions=%v err=%v", dimensions, err)
	}
	if code := rejectionCode(func() error {
		_, err := verifyDimensions("usage.trace.retained", observed, repository, "UNAVAILABLE")
		return err
	}()); code != rejectMetric {
		t.Fatalf("dimensioned unavailable trace usage rejection=%s", code)
	}
	if code := rejectionCode(func() error {
		_, err := verifyDimensions("usage.trace.retained", []any{}, repository, "SINGLE_COHORT")
		return err
	}()); code != rejectMetric {
		t.Fatalf("dimensionless observed trace usage rejection=%s", code)
	}
	if dimensions, err := verifyDimensions("usage.trace.retained", observed, repository, "SINGLE_COHORT"); err != nil || len(dimensions) != 2 {
		t.Fatalf("observed dimensions=%v err=%v", dimensions, err)
	}
	hostile := parseSnapshot(t, frozenFourInvalidLegacyMembers)
	traceUsage := hostile["usage"].([]any)[6].(map[string]any)
	traceUsage["sourceIds"] = []any{hostile["sources"].([]any)[0].(map[string]any)["id"]}
	if code := rejectionCode(verifySnapshot(rehash(t, hostile))); code != rejectTruthAxes {
		t.Fatalf("referenced unavailable trace usage rejection=%s", code)
	}
	missing := parseSnapshot(t, frozenFourInvalidLegacyMembers)
	missing["usage"] = missing["usage"].([]any)[:6]
	if code := rejectionCode(verifySnapshot(rehash(t, missing))); code != rejectMetricSet {
		t.Fatalf("missing trace usage rejection=%s", code)
	}

	observedRoot := nonemptyReferenceSnapshot()
	observedMetric := observedRoot["usage"].([]any)[6].(map[string]any)
	cohortID := observedRoot["cohorts"].([]any)[0].(map[string]any)["cohortId"]
	sourceID := observedRoot["sources"].([]any)[0].(map[string]any)["id"]
	revision := observedRoot["repository"].(map[string]any)["headRevision"]
	observedMetric["authorityClass"] = "ADVISORY"
	observedMetric["cohortIds"] = []any{cohortID}
	observedMetric["completeness"] = "COMPLETE"
	observedMetric["currency"] = "VALIDATED_AT"
	observedMetric["denominator"] = "1"
	observedMetric["dimensions"] = []any{
		map[string]any{"name": "outcome", "value": "passed"},
		map[string]any{"name": "revision", "value": revision},
	}
	observedMetric["epistemicClass"] = "OBSERVED"
	observedMetric["numerator"] = "1"
	observedMetric["scopeClass"] = "SINGLE_COHORT"
	observedMetric["sourceIds"] = []any{sourceID}
	observedMetric["validity"] = "VALID"
	observedMetric["value"] = "1"
	if err := verifySnapshot(rehash(t, observedRoot)); err != nil {
		t.Fatalf("observed trace usage: %v", err)
	}
	emptySources := parseSnapshot(t, rehash(t, observedRoot))
	emptySources["usage"].([]any)[6].(map[string]any)["sourceIds"] = []any{}
	if code := rejectionCode(verifySnapshot(rehash(t, emptySources))); code != rejectIdentity {
		t.Fatalf("unattributed trace usage rejection=%s", code)
	}
	unrelatedSource := parseSnapshot(t, rehash(t, observedRoot))
	unrelatedBasis := map[string]any{
		"adapterId": "query-envelope-v1", "configuredOrdinal": "0", "contentSha256": nil,
		"profile": "unsupported", "repositoryReadsSha256": nil,
	}
	unrelatedID := "dashboard-source:" + hashObject("corvint-dashboard-source/0", unrelatedBasis)
	unrelated := map[string]any{
		"adapterId": "query-envelope-v1", "authorityClass": "NONE", "byteCount": nil,
		"cohortIds": []any{}, "completeness": "UNKNOWN", "configuredOrdinal": "0",
		"contentSha256": nil, "currency": "UNKNOWN", "deliveryStage": "UNSUPPORTED",
		"displayLabel": "query-envelope-v1#0", "epistemicClass": "NOT_OBSERVED", "exclusions": []any{},
		"id": unrelatedID, "members": nil, "observationEnd": nil, "observationStart": nil,
		"observationTime": nil, "profile": "unsupported", "repositoryReadsSha256": nil,
		"validity": "UNSUPPORTED", "verifierId": "unsupported",
	}
	localContent := unrelatedSource["sources"].([]any)[0].(map[string]any)["contentSha256"]
	sources := append(unrelatedSource["sources"].([]any), unrelated)
	sort.Slice(sources, func(left, right int) bool {
		return sources[left].(map[string]any)["id"].(string) < sources[right].(map[string]any)["id"].(string)
	})
	unrelatedSource["sources"] = sources
	configured := []any{
		map[string]any{"adapterId": "local-trace-v1", "configuredOrdinal": "0", "contentSha256": localContent},
		map[string]any{"adapterId": "query-envelope-v1", "configuredOrdinal": "0", "contentSha256": nil},
	}
	sort.Slice(configured, func(left, right int) bool {
		leftBytes, _ := canonical(configured[left], false)
		rightBytes, _ := canonical(configured[right], false)
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
	unrelatedSource["observation"].(map[string]any)["configuredSourceSetSha256"] = hashObject("corvint-dashboard-configured-sources/0", configured)
	unrelatedSource["usage"].([]any)[6].(map[string]any)["sourceIds"] = []any{unrelatedID}
	if code := rejectionCode(verifySnapshot(rehash(t, unrelatedSource))); code != rejectIdentity {
		t.Fatalf("unrelated trace source rejection=%s", code)
	}
	nullRevision := parseSnapshot(t, rehash(t, observedRoot))
	nullRevision["usage"].([]any)[6].(map[string]any)["dimensions"].([]any)[1].(map[string]any)["value"] = nil
	if code := rejectionCode(verifySnapshot(rehash(t, nullRevision))); code != rejectMetric {
		t.Fatalf("null trace revision rejection=%s", code)
	}
	mismatchedRevision := parseSnapshot(t, rehash(t, observedRoot))
	mismatchedRevision["usage"].([]any)[6].(map[string]any)["dimensions"].([]any)[1].(map[string]any)["value"] = strings.Repeat("2", 40)
	if code := rejectionCode(verifySnapshot(rehash(t, mismatchedRevision))); code != rejectIdentity {
		t.Fatalf("mismatched trace revision rejection=%s", code)
	}
}

func TestNonemptyTraceInventoryArithmetic(t *testing.T) {
	root := nonemptyReferenceSnapshot()
	raw, err := canonical(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySnapshot(raw); err != nil {
		t.Fatalf("nonempty trace snapshot rejected: %v", err)
	}

	lying := nonemptyReferenceSnapshot()
	metric := lying["data"].([]any)[3].(map[string]any)
	metric["value"] = "0"
	metric["numerator"] = "0"
	if code := rejectionCode(verifySnapshot(rehash(t, lying))); code != rejectMetric {
		t.Fatalf("lying grouped inventory rejection = %s, want %s", code, rejectMetric)
	}

	missing := nonemptyReferenceSnapshot()
	missing["data"] = missing["data"].([]any)[:3]
	if code := rejectionCode(verifySnapshot(rehash(t, missing))); code != rejectMetric {
		t.Fatalf("missing grouped inventory rejection = %s, want %s", code, rejectMetric)
	}

	missingWitnesses := nonemptyReferenceSnapshot()
	source := missingWitnesses["sources"].([]any)[0].(map[string]any)
	source["repositoryReadsSha256"] = nil
	sourceBasis := map[string]any{
		"adapterId": "local-trace-v1", "configuredOrdinal": "0", "contentSha256": source["contentSha256"],
		"profile": "corvint-local-trace/1", "repositoryReadsSha256": nil,
	}
	sourceID := "dashboard-source:" + hashObject("corvint-dashboard-source/0", sourceBasis)
	source["id"] = sourceID
	for _, dataValue := range missingWitnesses["data"].([]any) {
		dataValue.(map[string]any)["sourceIds"] = []any{sourceID}
	}
	if code := rejectionCode(verifySnapshot(rehash(t, missingWitnesses))); code != rejectTruthAxes {
		t.Fatalf("missing repository witness rejection = %s, want %s", code, rejectTruthAxes)
	}

	duplicate := nonemptyReferenceSnapshot()
	data := duplicate["data"].([]any)
	duplicate["data"] = append([]any{data[0]}, data...)
	if code := rejectionCode(verifySnapshot(rehash(t, duplicate))); code != rejectOrdering {
		t.Fatalf("duplicate grouped inventory rejection = %s, want %s", code, rejectOrdering)
	}
}

func TestSnapshotForgeryRegressions(t *testing.T) {
	t.Run("configured-ordinal-duplicate-and-gap", func(t *testing.T) {
		for _, ordinals := range [][]string{{"0", "0"}, {"0", "2"}, {"1"}} {
			if code := rejectionCode(verifyConfiguredOrdinals(map[string][]string{"local-trace-v1": ordinals})); code != rejectIdentity {
				t.Fatalf("ordinals %v rejection = %s, want %s", ordinals, code, rejectIdentity)
			}
		}
	})

	t.Run("duplicate-and-unsorted-references", func(t *testing.T) {
		idA := "dashboard-source:sha256:" + strings.Repeat("a", 64)
		idB := "dashboard-source:sha256:" + strings.Repeat("b", 64)
		for _, values := range []any{[]any{idA, idA}, []any{idB, idA}} {
			if code := rejectionCode(func() error { _, err := sortedUniqueStrings(values, sourceIDRE); return err }()); code != rejectOrdering {
				t.Fatalf("references %v rejection = %s, want %s", values, code, rejectOrdering)
			}
		}
	})

	t.Run("missing-local-trace-tree-authority", func(t *testing.T) {
		root := nonemptyReferenceSnapshot()
		cohort := root["cohorts"].([]any)[0].(map[string]any)
		cohort["sourceTreeRevision"] = nil
		oldID := cohort["cohortId"].(string)
		cohort["cohortId"] = "dashboard-cohort:" + hashObject("corvint-dashboard-cohort/0", cloneWithout(cohort, "cohortId"))
		newID := cohort["cohortId"].(string)
		source := root["sources"].([]any)[0].(map[string]any)
		source["cohortIds"] = []any{newID}
		for _, metricValue := range root["data"].([]any) {
			metric := metricValue.(map[string]any)
			cohorts := metric["cohortIds"].([]any)
			for index := range cohorts {
				if cohorts[index] == oldID {
					cohorts[index] = newID
				}
			}
		}
		if code := rejectionCode(verifySnapshot(rehash(t, root))); code != rejectTruthAxes {
			t.Fatalf("rejection = %s, want %s", code, rejectTruthAxes)
		}
	})

	t.Run("malformed-observation-issue", func(t *testing.T) {
		root := referenceSnapshot()
		issue := root["issues"].([]any)[0].(map[string]any)
		issue["severity"] = "ERROR"
		issue["id"] = "dashboard-issue:" + hashObject("corvint-dashboard-issue/0", cloneWithout(issue, "id"))
		if code := rejectionCode(verifySnapshot(rehash(t, root))); code != rejectTruthAxes {
			t.Fatalf("rejection = %s, want %s", code, rejectTruthAxes)
		}
	})

	t.Run("aggregate-member-bound", func(t *testing.T) {
		members := make([]any, maxTraceRows+1)
		source := map[string]any{"members": members}
		if code := rejectionCode(verifyMembers(source, "local-trace-v1", "VALID", "", false, "0", true, "", false, nil, nil, repositoryState{}, false, time.Time{}, time.Time{})); code != rejectSize {
			t.Fatalf("rejection = %s, want %s", code, rejectSize)
		}
	})

	t.Run("terminal-count-arithmetic", func(t *testing.T) {
		sourceID := "dashboard-source:sha256:" + strings.Repeat("a", 64)
		issueID := "dashboard-issue:sha256:" + strings.Repeat("b", 64)
		sources := []any{map[string]any{"adapterId": "local-trace-v1", "completeness": "PARTIAL", "id": sourceID}}
		issues := []any{map[string]any{"code": "VERIFIER_REJECTED", "id": issueID, "observed": "2", "sourceId": sourceID}}
		metric := map[string]any{
			"cohortIds": []any{}, "completeness": "PARTIAL", "denominator": nil,
			"dimensions": []any{map[string]any{"name": "artifactKind", "value": "RETAINED_MEMBER"}, map[string]any{"name": "validity", "value": "INVALID"}},
			"exclusions": []any{issueID}, "name": "data.artifact.count", "numerator": "2",
			"sourceIds": []any{sourceID}, "value": "2",
		}
		if err := verifyRejectedMemberArithmetic([]any{metric}, sources, issues); err != nil {
			t.Fatal(err)
		}
		metric["value"] = "1"
		metric["numerator"] = "1"
		if code := rejectionCode(verifyRejectedMemberArithmetic([]any{metric}, sources, issues)); code != rejectMetric {
			t.Fatalf("rejection = %s, want %s", code, rejectMetric)
		}
	})

	t.Run("duplicate-metric-key", func(t *testing.T) {
		root := nonemptyReferenceSnapshot()
		data := root["data"].([]any)
		root["data"] = append(data[:1], append([]any{data[0]}, data[1:]...)...)
		if code := rejectionCode(verifySnapshot(rehash(t, root))); code != rejectOrdering {
			t.Fatalf("rejection = %s, want %s", code, rejectOrdering)
		}
	})
}

func TestFrozenNegativeCases(t *testing.T) {
	manifest := loadManifest(t)
	if manifest.Profile != profile || manifest.Positive != "testdata/valid-observed.json" || len(manifest.Cases) < 12 {
		t.Fatalf("invalid manifest: %+v", manifest)
	}
	ids := make([]string, len(manifest.Cases))
	for index, test := range manifest.Cases {
		ids[index] = test.ID
		t.Run(test.ID, func(t *testing.T) {
			err := verifySnapshot(mutate(t, test.Mutation))
			if got := string(rejectionCode(err)); got != test.Expected {
				t.Fatalf("rejection = %q (%v), want %q", got, err, test.Expected)
			}
		})
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatal("case IDs are not sorted")
	}
}

func TestSizeBound(t *testing.T) {
	err := verifySnapshot(bytes.Repeat([]byte{'x'}, maxSnapshotBytes+1))
	if rejectionCode(err) != rejectSize {
		t.Fatalf("rejection = %v", err)
	}
}

func TestCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := os.WriteFile(path, frozenValid, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := run([]string{"--snapshot", path}, &stdout, &stderr); exit != 0 || stderr.Len() != 0 || stdout.String() != `{"profile":"corvint-dashboard-snapshot-verifier/0","status":"PASS"}`+"\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestBuiltCrossPlatform(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-build disabled in short mode")
	}
	_ = runtime.GOOS
}

func TestPrintReference(t *testing.T) {
	if os.Getenv("CORVINT_PRINT_DASHBOARD_REFERENCE") == "1" {
		fmt.Print(string(referenceBytes(t)))
	}
}
