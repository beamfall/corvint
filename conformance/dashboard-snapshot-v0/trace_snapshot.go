package main

import (
	"bytes"
	"sort"
	"strconv"
)

func compileExpectedTraceSnapshotLegacy(manifest traceManifest, corpus traceCorpusResult) ([]byte, error) {
	if len(manifest.configuredSources) != 1 || manifest.configuredSources[0].adapterID != "local-trace-v1" || manifest.configuredSources[0].configuredOrdinal != "0" || corpus.repository.format == "" {
		return nil, reject(rejectInternal)
	}
	generatedAt := manifest.generatedAt.Format("2006-01-02T15:04:05.000000000Z")
	dirtyValues := make([]any, len(corpus.repository.dirtyPaths))
	for index, value := range corpus.repository.dirtyPaths {
		dirtyValues[index] = value
	}
	dirtyBytes, _ := canonical(dirtyValues, false)
	dirtyDigest := domainDigest("corvint-dashboard-dirty-paths/0", dirtyBytes)
	members := make([]any, 0, len(corpus.members))
	cohorts := make([]any, 0, len(corpus.members))
	cohortByRevision := make(map[string]string)
	witnesses := []any{map[string]any{"kind": "SNAPSHOT_HEAD", "objectFormat": corpus.repository.format, "objectId": corpus.repository.head, "objectType": "commit", "revision": nil}}
	totalBytes := 0
	for _, member := range corpus.members {
		members = append(members, map[string]any{"byteCount": strconv.Itoa(member.bytes), "contentSha256": member.digest, "revision": member.revision})
		totalBytes += member.bytes
		tree := corpus.repository.revisionTrees[member.revision]
		cohortBasis := map[string]any{
			"adapterId": "local-trace-v1", "dirtyPathsSha256": dirtyDigest,
			"producerIdentity": "go-local-trace-v1", "profile": "corvint-local-trace/1",
			"repositoryObjectFormat": corpus.repository.format, "sourceObservationEnd": generatedAt,
			"sourceObservationStart": generatedAt, "sourceRevision": member.revision, "sourceTreeRevision": tree,
		}
		cohortID := "dashboard-cohort:" + hashCanonical("corvint-dashboard-cohort/0", cohortBasis)
		cohort := cloneMap(cohortBasis)
		cohort["cohortId"] = cohortID
		cohorts = append(cohorts, cohort)
		cohortByRevision[member.revision] = cohortID
		witnesses = append(witnesses, map[string]any{"kind": "TRACE_REVISION", "objectFormat": corpus.repository.format, "objectId": member.revision, "objectType": "commit", "revision": member.revision})
		for _, objectID := range corpus.repository.pathWitnesses[member.revision] {
			witnesses = append(witnesses, map[string]any{"kind": "TRACE_PATH_OBJECT", "objectFormat": corpus.repository.format, "objectId": objectID, "objectType": "blob", "revision": member.revision})
		}
	}
	sortCanonical(members)
	sortCanonical(cohorts)
	sortCanonical(witnesses)
	witnesses = uniqueCanonical(witnesses)
	membersDigest := hashCanonical("trace-store/0", members)
	repositoryReads := hashCanonical("corvint-dashboard-repository-reads/0", witnesses)
	cohortIDs := make([]any, 0, len(cohorts))
	for _, cohortValue := range cohorts {
		cohortIDs = append(cohortIDs, cohortValue.(map[string]any)["cohortId"])
	}
	sortCanonical(cohortIDs)
	sourceBasis := map[string]any{
		"adapterId": "local-trace-v1", "configuredOrdinal": "0", "contentSha256": membersDigest,
		"profile": "corvint-local-trace/1", "repositoryReadsSha256": repositoryReads,
	}
	sourceID := "dashboard-source:" + hashCanonical("corvint-dashboard-source/0", sourceBasis)
	var observationStart, observationEnd any
	if len(members) != 0 {
		observationStart, observationEnd = generatedAt, generatedAt
	}
	source := map[string]any{
		"adapterId": "local-trace-v1", "authorityClass": "ADVISORY", "byteCount": strconv.Itoa(totalBytes),
		"cohortIds": cohortIDs, "completeness": "COMPLETE", "configuredOrdinal": "0",
		"contentSha256": membersDigest, "currency": "VALIDATED_AT", "deliveryStage": "NOT_STARTED",
		"displayLabel": "local-trace-v1#0", "epistemicClass": "OBSERVED", "exclusions": []any{},
		"id": sourceID, "members": members, "observationEnd": observationEnd, "observationStart": observationStart,
		"observationTime": nil, "profile": "corvint-local-trace/1", "repositoryReadsSha256": repositoryReads,
		"validity": "VALID", "verifierId": "go-local-trace-v1",
	}
	issueBasis := map[string]any{"code": "OBSERVATION_TIME_UNKNOWN", "limit": nil, "observed": nil, "severity": "INFO", "sourceId": sourceID}
	issue := cloneMap(issueBasis)
	issue["id"] = "dashboard-issue:" + hashCanonical("corvint-dashboard-issue/0", issueBasis)
	data := expectedTraceInventoryMetrics(source, corpus.members, cohortByRevision, totalBytes)
	usage := expectedTraceUsageMetrics(source, corpus, cohortByRevision)
	if len(usage) == 0 {
		usage = append(usage, unavailableSnapshotMetric("usage.trace.retained", "NOT_STARTED"))
	}
	for _, name := range []string{"usage.harness.retained", "usage.impact.retained", "usage.latency.p50", "usage.latency.p95", "usage.query.retained", "usage.response.bytes"} {
		usage = append(usage, unavailableSnapshotMetric(name, "UNSUPPORTED"))
	}
	sortMetrics(usage)
	verification := []any{
		unavailableSnapshotMetric("verification.cem.hunks", "NOT_STARTED"),
		unavailableSnapshotMetric("verification.closure", "NOT_STARTED"),
		unavailableSnapshotMetric("verification.live.runs", "UNSUPPORTED"),
		unavailableSnapshotMetric("verification.ocm.obligations", "NOT_STARTED"),
		unavailableSnapshotMetric("verification.pulse.runs", "UNSUPPORTED"),
	}
	configured := []any{map[string]any{"adapterId": "local-trace-v1", "configuredOrdinal": "0", "contentSha256": membersDigest}}
	worktreeState := "CLEAN"
	if len(corpus.repository.dirtyPaths) != 0 {
		worktreeState = "MIXED"
	}
	root := map[string]any{
		"beamfall": []any{unavailableSnapshotMetric("beamfall.shadow.runs", "UNSUPPORTED")},
		"cohorts":  cohorts, "data": data,
		"frontier":    []any{unavailableSnapshotMetric("frontier.items", "UNSUPPORTED")},
		"generatedAt": generatedAt, "harnesses": []any{}, "issues": []any{issue},
		"observation": map[string]any{
			"adapterRegistrySha256": domainDigest("corvint-dashboard-adapter-registry/0", []byte(canonicalRegistry)),
			"clockSource":           "CALLER", "configuredSourceSetSha256": hashCanonical("corvint-dashboard-configured-sources/0", configured),
			"end": generatedAt, "limitsProfile": "corvint-dashboard-limits/0", "scanState": "COMPLETE", "start": generatedAt,
		},
		"privacy": map[string]any{"collection": "DISABLED", "outboundNetwork": "NONE", "pathDisclosure": "NONE", "rawBodies": "EXCLUDED", "threatBoundary": "LOCAL_ACCOUNT_NOT_DEFENDED"},
		"repository": map[string]any{
			"dirtyPathCount": strconv.Itoa(len(corpus.repository.dirtyPaths)), "dirtyPathsSha256": dirtyDigest,
			"headRevision": corpus.repository.head, "objectFormat": corpus.repository.format,
			"treeRevision": corpus.repository.tree, "worktreeState": worktreeState,
		},
		"schema": schema, "snapshotSha256": nil, "sources": []any{source}, "usage": usage, "verification": verification,
	}
	root["snapshotSha256"] = hashCanonical(schema, root)
	return canonical(root, true)
}

func expectedTraceInventoryMetrics(source map[string]any, members []traceCorpusMember, cohortByRevision map[string]string, totalBytes int) []any {
	dimensions := func(kind string, revision any) []any {
		return []any{
			map[string]any{"name": "adapterId", "value": "local-trace-v1"}, map[string]any{"name": "ageBucket", "value": "UNKNOWN"},
			map[string]any{"name": "artifactKind", "value": kind}, map[string]any{"name": "authorityClass", "value": "ADVISORY"},
			map[string]any{"name": "deliveryStage", "value": "NOT_STARTED"}, map[string]any{"name": "revision", "value": revision},
			map[string]any{"name": "validity", "value": "VALID"},
		}
	}
	metric := func(name, kind, value, denominator string, revision any, cohorts []any) any {
		return map[string]any{
			"authorityClass": "ADVISORY", "cohortIds": cohorts, "completeness": "COMPLETE", "currency": "VALIDATED_AT",
			"denominator": denominator, "deliveryStage": "NOT_STARTED", "dimensions": dimensions(kind, revision), "epistemicClass": "OBSERVED",
			"exclusions": []any{}, "name": name, "numerator": value, "scopeClass": "MULTI_COHORT_INVENTORY",
			"sourceIds": []any{source["id"]}, "unit": metricUnit[name], "validity": "VALID", "value": value, "window": nil,
		}
	}
	allCohorts := source["cohortIds"].([]any)
	result := []any{
		metric("data.artifact.bytes", "CONFIGURED_SOURCE", strconv.Itoa(totalBytes), strconv.Itoa(totalBytes), nil, allCohorts),
		metric("data.artifact.count", "CONFIGURED_SOURCE", "1", "1", nil, allCohorts),
	}
	if len(members) == 0 {
		result = append(result,
			metric("data.artifact.bytes", "RETAINED_MEMBER", "0", "0", nil, []any{}),
			metric("data.artifact.count", "RETAINED_MEMBER", "0", "0", nil, []any{}),
		)
	} else {
		for _, member := range members {
			cohorts := []any{cohortByRevision[member.revision]}
			result = append(result,
				metric("data.artifact.bytes", "RETAINED_MEMBER", strconv.Itoa(member.bytes), strconv.Itoa(totalBytes), member.revision, cohorts),
				metric("data.artifact.count", "RETAINED_MEMBER", "1", strconv.Itoa(len(members)), member.revision, cohorts),
			)
		}
	}
	sortMetrics(result)
	return result
}

func expectedTraceUsageMetrics(source map[string]any, corpus traceCorpusResult, cohortByRevision map[string]string) []any {
	result := make([]any, 0)
	for revision, outcomes := range corpus.outcomes {
		total := 0
		for _, count := range outcomes {
			total += count
		}
		for outcome, count := range outcomes {
			result = append(result, map[string]any{
				"authorityClass": "ADVISORY", "cohortIds": []any{cohortByRevision[revision]}, "completeness": "COMPLETE", "currency": "VALIDATED_AT",
				"denominator": strconv.Itoa(total), "deliveryStage": "NOT_STARTED",
				"dimensions":     []any{map[string]any{"name": "outcome", "value": outcome}, map[string]any{"name": "revision", "value": revision}},
				"epistemicClass": "OBSERVED", "exclusions": []any{}, "name": "usage.trace.retained", "numerator": strconv.Itoa(count),
				"scopeClass": "SINGLE_COHORT", "sourceIds": []any{source["id"]}, "unit": "COUNT", "validity": "VALID", "value": strconv.Itoa(count), "window": nil,
			})
		}
	}
	return result
}

func unavailableSnapshotMetric(name, stage string) any {
	return map[string]any{
		"authorityClass": "NONE", "cohortIds": []any{}, "completeness": "UNKNOWN", "currency": "UNKNOWN",
		"denominator": nil, "deliveryStage": stage, "dimensions": []any{}, "epistemicClass": "NOT_OBSERVED",
		"exclusions": []any{}, "name": name, "numerator": nil, "scopeClass": "UNAVAILABLE", "sourceIds": []any{},
		"unit": metricUnit[name], "validity": "UNSUPPORTED", "value": nil, "window": nil,
	}
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func hashCanonical(domain string, value any) string {
	raw, _ := canonical(value, false)
	return domainDigest(domain, raw)
}

func sortCanonical(values []any) {
	sort.Slice(values, func(left, right int) bool {
		leftBytes, _ := canonical(values[left], false)
		rightBytes, _ := canonical(values[right], false)
		return bytes.Compare(leftBytes, rightBytes) < 0
	})
}

func uniqueCanonical(values []any) []any {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	previous, _ := canonical(values[0], false)
	for _, value := range values[1:] {
		current, _ := canonical(value, false)
		if !bytes.Equal(previous, current) {
			result = append(result, value)
			previous = current
		}
	}
	return result
}

func sortMetrics(values []any) {
	sort.Slice(values, func(left, right int) bool {
		leftMetric := values[left].(map[string]any)
		rightMetric := values[right].(map[string]any)
		leftDimensions, _ := canonical(leftMetric["dimensions"], false)
		rightDimensions, _ := canonical(rightMetric["dimensions"], false)
		leftCohorts, _ := canonical(leftMetric["cohortIds"], false)
		rightCohorts, _ := canonical(rightMetric["cohortIds"], false)
		leftKey := leftMetric["name"].(string) + "\x00" + string(leftDimensions) + "\x00" + string(leftCohorts)
		rightKey := rightMetric["name"].(string) + "\x00" + string(rightDimensions) + "\x00" + string(rightCohorts)
		return leftKey < rightKey
	})
}
