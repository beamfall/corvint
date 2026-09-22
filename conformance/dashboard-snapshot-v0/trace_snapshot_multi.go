package main

import (
	"sort"
	"strconv"
)

type compiledTraceSource struct {
	row              map[string]any
	corpus           traceCorpusSource
	cohortByRevision map[string]string
	terminalIssues   []string
	overrideIssue    string
}

func compileExpectedTraceSnapshot(manifest traceManifest, corpus traceCorpusResult) ([]byte, error) {
	localSources := make([]traceConfiguredSource, 0)
	for _, source := range manifest.configuredSources {
		if source.adapterID == "local-trace-v1" {
			localSources = append(localSources, source)
		}
	}
	if len(localSources) == 0 || len(localSources) != len(corpus.sources) || corpus.repository.format == "" {
		return nil, reject(rejectInternal)
	}
	generatedAt := manifest.generatedAt.Format("2006-01-02T15:04:05.000000000Z")
	dirtyValues := make([]any, len(corpus.repository.dirtyPaths))
	for index, value := range corpus.repository.dirtyPaths {
		dirtyValues[index] = value
	}
	dirtyBytes, _ := canonical(dirtyValues, false)
	dirtyDigest := domainDigest("corvint-dashboard-dirty-paths/0", dirtyBytes)

	compiled := make([]compiledTraceSource, 0, len(localSources))
	cohortByID := make(map[string]any)
	issues := make([]any, 0)
	configuredRows := make([]any, 0, len(localSources))
	for index, configured := range localSources {
		if corpus.sources[index].configuredOrdinal != configured.configuredOrdinal {
			return nil, reject(rejectInternal)
		}
		one, sourceCohorts, sourceIssues, err := compileOneTraceSource(configured, corpus.sources[index], corpus.repository, generatedAt, dirtyDigest)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, one)
		for _, cohort := range sourceCohorts {
			cohortByID[cohort.(map[string]any)["cohortId"].(string)] = cohort
		}
		issues = append(issues, sourceIssues...)
		configuredRows = append(configuredRows, map[string]any{
			"adapterId": "local-trace-v1", "configuredOrdinal": configured.configuredOrdinal, "contentSha256": one.row["contentSha256"],
		})
	}

	cohorts := make([]any, 0, len(cohortByID))
	for _, cohort := range cohortByID {
		cohorts = append(cohorts, cohort)
	}
	sortCanonical(cohorts)
	sources := make([]any, 0, len(compiled))
	for _, source := range compiled {
		sources = append(sources, source.row)
	}
	sort.Slice(sources, func(left, right int) bool {
		return sources[left].(map[string]any)["id"].(string) < sources[right].(map[string]any)["id"].(string)
	})
	sort.Slice(issues, func(left, right int) bool {
		return issues[left].(map[string]any)["id"].(string) < issues[right].(map[string]any)["id"].(string)
	})
	sortCanonical(configuredRows)

	data := compileTraceInventory(compiled)
	usage := compileTraceUsage(compiled)
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
	worktreeState := "CLEAN"
	if len(corpus.repository.dirtyPaths) != 0 {
		worktreeState = "MIXED"
	}
	scanState := "COMPLETE"
	for _, source := range compiled {
		if source.row["completeness"] != "COMPLETE" {
			scanState = "PARTIAL"
			break
		}
	}
	root := map[string]any{
		"beamfall": []any{unavailableSnapshotMetric("beamfall.shadow.runs", "UNSUPPORTED")},
		"cohorts":  cohorts, "data": data,
		"frontier":    []any{unavailableSnapshotMetric("frontier.items", "UNSUPPORTED")},
		"generatedAt": generatedAt, "harnesses": []any{}, "issues": issues,
		"observation": map[string]any{
			"adapterRegistrySha256": domainDigest("corvint-dashboard-adapter-registry/0", []byte(canonicalRegistry)),
			"clockSource":           "CALLER", "configuredSourceSetSha256": hashCanonical("corvint-dashboard-configured-sources/0", configuredRows),
			"end": generatedAt, "limitsProfile": "corvint-dashboard-limits/0", "scanState": scanState, "start": generatedAt,
		},
		"privacy": map[string]any{"collection": "DISABLED", "outboundNetwork": "NONE", "pathDisclosure": "NONE", "rawBodies": "EXCLUDED", "threatBoundary": "LOCAL_ACCOUNT_NOT_DEFENDED"},
		"repository": map[string]any{
			"dirtyPathCount": strconv.Itoa(len(corpus.repository.dirtyPaths)), "dirtyPathsSha256": dirtyDigest,
			"headRevision": corpus.repository.head, "objectFormat": corpus.repository.format,
			"treeRevision": corpus.repository.tree, "worktreeState": worktreeState,
		},
		"schema": schema, "snapshotSha256": nil, "sources": sources, "usage": usage, "verification": verification,
	}
	root["snapshotSha256"] = hashCanonical(schema, root)
	return canonical(root, true)
}

func compileOneTraceSource(configured traceConfiguredSource, source traceCorpusSource, repository gitSemanticEvidence, generatedAt, dirtyDigest string) (compiledTraceSource, []any, []any, error) {
	members := make([]any, 0, len(source.members))
	cohorts := make([]any, 0, len(source.members))
	cohortByRevision := make(map[string]string)
	totalBytes := 0
	for _, member := range source.members {
		members = append(members, map[string]any{"byteCount": strconv.Itoa(member.bytes), "contentSha256": member.digest, "revision": member.revision})
		totalBytes += member.bytes
		tree := repository.revisionTrees[member.revision]
		if tree == "" {
			return compiledTraceSource{}, nil, nil, reject(rejectInternal)
		}
		basis := map[string]any{
			"adapterId": "local-trace-v1", "dirtyPathsSha256": dirtyDigest,
			"producerIdentity": "go-local-trace-v1", "profile": "corvint-local-trace/1",
			"repositoryObjectFormat": repository.format, "sourceObservationEnd": generatedAt,
			"sourceObservationStart": generatedAt, "sourceRevision": member.revision, "sourceTreeRevision": tree,
		}
		cohortID := "dashboard-cohort:" + hashCanonical("corvint-dashboard-cohort/0", basis)
		cohort := cloneMap(basis)
		cohort["cohortId"] = cohortID
		cohorts = append(cohorts, cohort)
		cohortByRevision[member.revision] = cohortID
	}
	sortCanonical(members)
	sortCanonical(cohorts)
	cohortIDs := make([]any, 0, len(cohorts))
	for _, cohort := range cohorts {
		cohortIDs = append(cohortIDs, cohort.(map[string]any)["cohortId"])
	}
	sortCanonical(cohortIDs)

	validity, completeness, epistemic, currency := "VALID", "COMPLETE", "OBSERVED", "VALIDATED_AT"
	contentDigest := any(hashCanonical("trace-store/0", members))
	byteCount := any(strconv.Itoa(totalBytes))
	var repositoryReads any
	if len(source.members) != 0 || len(source.terminal) == 0 && source.overrideCode == "" {
		witnesses := []any{map[string]any{"kind": "SNAPSHOT_HEAD", "objectFormat": repository.format, "objectId": repository.head, "objectType": "commit", "revision": nil}}
		for _, member := range source.members {
			witnesses = append(witnesses, map[string]any{"kind": "TRACE_REVISION", "objectFormat": repository.format, "objectId": member.revision, "objectType": "commit", "revision": member.revision})
			for _, objectID := range repository.pathWitnesses[member.revision] {
				witnesses = append(witnesses, map[string]any{"kind": "TRACE_PATH_OBJECT", "objectFormat": repository.format, "objectId": objectID, "objectType": "blob", "revision": member.revision})
			}
		}
		sortCanonical(witnesses)
		witnesses = uniqueCanonical(witnesses)
		repositoryReads = hashCanonical("corvint-dashboard-repository-reads/0", witnesses)
	}
	if len(source.terminal) != 0 {
		completeness = "PARTIAL"
		if len(source.members) == 0 {
			validity, epistemic, currency = "INVALID", "NOT_OBSERVED", "UNKNOWN"
		}
	}
	if source.overrideCode != "" {
		validity, completeness, epistemic, currency = "INVALID", "UNKNOWN", "NOT_OBSERVED", "UNKNOWN"
		contentDigest, byteCount, repositoryReads = nil, nil, nil
		members, cohorts, cohortIDs = []any{}, []any{}, []any{}
		cohortByRevision = map[string]string{}
	}
	identityBasis := map[string]any{
		"adapterId": "local-trace-v1", "configuredOrdinal": configured.configuredOrdinal,
		"contentSha256": contentDigest, "profile": "corvint-local-trace/1", "repositoryReadsSha256": repositoryReads,
	}
	sourceID := "dashboard-source:" + hashCanonical("corvint-dashboard-source/0", identityBasis)
	issues := make([]any, 0, len(source.terminal)+2)
	terminalIssueIDs := make([]string, 0, len(source.terminal))
	for code, count := range source.terminal {
		issue := compileTraceIssue(code, "ERROR", sourceID, strconv.Itoa(count), nil)
		issues = append(issues, issue)
		terminalIssueIDs = append(terminalIssueIDs, issue["id"].(string))
	}
	sort.Strings(terminalIssueIDs)
	overrideIssueID := ""
	exclusions := make([]any, 0)
	if source.overrideCode != "" {
		var limit any
		if source.overrideLimit != "" {
			limit = source.overrideLimit
		}
		issue := compileTraceIssue(source.overrideCode, "ERROR", sourceID, nil, limit)
		issues = append(issues, issue)
		overrideIssueID = issue["id"].(string)
		exclusions = append(exclusions, overrideIssueID)
	} else {
		for _, id := range terminalIssueIDs {
			exclusions = append(exclusions, id)
		}
		issues = append(issues, compileTraceIssue("OBSERVATION_TIME_UNKNOWN", "INFO", sourceID, nil, nil))
	}
	var observationStart, observationEnd any
	if len(members) != 0 {
		observationStart, observationEnd = generatedAt, generatedAt
	}
	row := map[string]any{
		"adapterId": "local-trace-v1", "authorityClass": "ADVISORY", "byteCount": byteCount,
		"cohortIds": cohortIDs, "completeness": completeness, "configuredOrdinal": configured.configuredOrdinal,
		"contentSha256": contentDigest, "currency": currency, "deliveryStage": "NOT_STARTED",
		"displayLabel": "local-trace-v1#" + configured.configuredOrdinal, "epistemicClass": epistemic, "exclusions": exclusions,
		"id": sourceID, "members": members, "observationEnd": observationEnd, "observationStart": observationStart,
		"observationTime": nil, "profile": "corvint-local-trace/1", "repositoryReadsSha256": repositoryReads,
		"validity": validity, "verifierId": "go-local-trace-v1",
	}
	return compiledTraceSource{row: row, corpus: source, cohortByRevision: cohortByRevision, terminalIssues: terminalIssueIDs, overrideIssue: overrideIssueID}, cohorts, issues, nil
}

func compileTraceIssue(code, severity, sourceID string, observed, limit any) map[string]any {
	basis := map[string]any{"code": code, "limit": limit, "observed": observed, "severity": severity, "sourceId": sourceID}
	issue := cloneMap(basis)
	issue["id"] = "dashboard-issue:" + hashCanonical("corvint-dashboard-issue/0", basis)
	return issue
}

type traceMetricAccumulator struct {
	metric    map[string]any
	value     int64
	hasValue  bool
	nullValue bool
	sources   map[string]struct{}
	excludes  map[string]struct{}
}

func compileTraceInventory(sources []compiledTraceSource) []any {
	groups := make(map[string]*traceMetricAccumulator)
	closed := true
	configuredBytes := int64(0)
	retainedCount := int64(0)
	retainedBytes := int64(0)
	for _, source := range sources {
		if source.row["completeness"] != "COMPLETE" {
			closed = false
		}
		if value, ok := source.row["byteCount"].(string); ok {
			parsed, _ := strconv.ParseInt(value, 10, 64)
			configuredBytes += parsed
		}
		retainedCount += int64(len(source.corpus.members))
		for _, member := range source.corpus.members {
			retainedBytes += int64(member.bytes)
		}
	}
	add := func(source compiledTraceSource, name, kind, validity string, revision any, cohortIDs []any, value *int64, exclusions []string, completeness string, epistemic string) {
		dimensions := []any{
			map[string]any{"name": "adapterId", "value": "local-trace-v1"}, map[string]any{"name": "ageBucket", "value": "UNKNOWN"},
			map[string]any{"name": "artifactKind", "value": kind}, map[string]any{"name": "authorityClass", "value": "ADVISORY"},
			map[string]any{"name": "deliveryStage", "value": "NOT_STARTED"}, map[string]any{"name": "revision", "value": revision},
			map[string]any{"name": "validity", "value": validity},
		}
		key := name + "\x00" + string(mustCanonical(dimensions)) + "\x00" + string(mustCanonical(cohortIDs))
		group := groups[key]
		if group == nil {
			group = &traceMetricAccumulator{sources: make(map[string]struct{}), excludes: make(map[string]struct{})}
			group.metric = map[string]any{
				"authorityClass": "ADVISORY", "cohortIds": cohortIDs, "completeness": completeness, "currency": source.row["currency"],
				"denominator": nil, "deliveryStage": "NOT_STARTED", "dimensions": dimensions, "epistemicClass": epistemic,
				"exclusions": []any{}, "name": name, "numerator": nil, "scopeClass": "MULTI_COHORT_INVENTORY",
				"sourceIds": []any{}, "unit": metricUnit[name], "validity": validity, "value": nil, "window": nil,
			}
			groups[key] = group
		}
		group.sources[source.row["id"].(string)] = struct{}{}
		for _, exclusion := range exclusions {
			group.excludes[exclusion] = struct{}{}
		}
		if value == nil {
			group.nullValue = true
		} else {
			group.hasValue = true
			group.value += *value
		}
	}
	for _, source := range sources {
		one := int64(1)
		metricCompleteness := "COMPLETE"
		if source.row["completeness"] != "COMPLETE" {
			metricCompleteness = "PARTIAL"
		}
		cohorts := source.row["cohortIds"].([]any)
		add(source, "data.artifact.count", "CONFIGURED_SOURCE", source.row["validity"].(string), nil, cohorts, &one, anyStrings(source.row["exclusions"].([]any)), metricCompleteness, "OBSERVED")
		if text, ok := source.row["byteCount"].(string); ok {
			value, _ := strconv.ParseInt(text, 10, 64)
			add(source, "data.artifact.bytes", "CONFIGURED_SOURCE", source.row["validity"].(string), nil, cohorts, &value, anyStrings(source.row["exclusions"].([]any)), metricCompleteness, "OBSERVED")
		} else {
			add(source, "data.artifact.bytes", "CONFIGURED_SOURCE", source.row["validity"].(string), nil, cohorts, nil, anyStrings(source.row["exclusions"].([]any)), metricCompleteness, "NOT_OBSERVED")
		}
		if len(source.corpus.members) == 0 && len(source.corpus.terminal) == 0 && source.corpus.overrideCode == "" {
			zero := int64(0)
			add(source, "data.artifact.count", "RETAINED_MEMBER", "VALID", nil, []any{}, &zero, nil, "COMPLETE", "OBSERVED")
			add(source, "data.artifact.bytes", "RETAINED_MEMBER", "VALID", nil, []any{}, &zero, nil, "COMPLETE", "OBSERVED")
		}
		for _, member := range source.corpus.members {
			one := int64(1)
			bytes := int64(member.bytes)
			memberCohorts := []any{source.cohortByRevision[member.revision]}
			add(source, "data.artifact.count", "RETAINED_MEMBER", "VALID", member.revision, memberCohorts, &one, nil, metricCompleteness, "OBSERVED")
			add(source, "data.artifact.bytes", "RETAINED_MEMBER", "VALID", member.revision, memberCohorts, &bytes, nil, metricCompleteness, "OBSERVED")
		}
		if len(source.corpus.terminal) != 0 {
			rejected := int64(0)
			for _, count := range source.corpus.terminal {
				rejected += int64(count)
			}
			add(source, "data.artifact.count", "RETAINED_MEMBER", "INVALID", nil, []any{}, &rejected, source.terminalIssues, "PARTIAL", "OBSERVED")
			add(source, "data.artifact.bytes", "RETAINED_MEMBER", "INVALID", nil, []any{}, nil, source.terminalIssues, "PARTIAL", "NOT_OBSERVED")
		}
		if source.corpus.overrideCode != "" {
			exclusions := []string{source.overrideIssue}
			add(source, "data.artifact.count", "RETAINED_MEMBER", "INVALID", nil, []any{}, nil, exclusions, "PARTIAL", "NOT_OBSERVED")
			add(source, "data.artifact.bytes", "RETAINED_MEMBER", "INVALID", nil, []any{}, nil, exclusions, "PARTIAL", "NOT_OBSERVED")
		}
	}
	result := make([]any, 0, len(groups))
	for _, group := range groups {
		ids := sortedSet(group.sources)
		exclusions := sortedSet(group.excludes)
		group.metric["sourceIds"] = stringsAny(ids)
		group.metric["exclusions"] = stringsAny(exclusions)
		if group.hasValue && !group.nullValue {
			text := strconv.FormatInt(group.value, 10)
			group.metric["value"], group.metric["numerator"] = text, text
		}
		if closed {
			dimensions := group.metric["dimensions"].([]any)
			kind := dimensions[2].(map[string]any)["value"]
			switch {
			case kind == "CONFIGURED_SOURCE" && group.metric["name"] == "data.artifact.count":
				group.metric["denominator"] = strconv.Itoa(len(sources))
			case kind == "CONFIGURED_SOURCE":
				group.metric["denominator"] = strconv.FormatInt(configuredBytes, 10)
			case group.metric["name"] == "data.artifact.count":
				group.metric["denominator"] = strconv.FormatInt(retainedCount, 10)
			default:
				group.metric["denominator"] = strconv.FormatInt(retainedBytes, 10)
			}
		}
		result = append(result, group.metric)
	}
	sortMetrics(result)
	return result
}

func compileTraceUsage(sources []compiledTraceSource) []any {
	type usageGroup struct {
		metric map[string]any
		count  int
		ids    map[string]struct{}
	}
	groups := make(map[string]*usageGroup)
	totals := make(map[string]int)
	for _, source := range sources {
		for revision, outcomes := range source.corpus.outcomes {
			for _, count := range outcomes {
				totals[revision] += count
			}
		}
	}
	for _, source := range sources {
		for revision, outcomes := range source.corpus.outcomes {
			for outcome, count := range outcomes {
				cohorts := []any{source.cohortByRevision[revision]}
				dimensions := []any{map[string]any{"name": "outcome", "value": outcome}, map[string]any{"name": "revision", "value": revision}}
				key := "usage.trace.retained\x00" + string(mustCanonical(dimensions)) + "\x00" + string(mustCanonical(cohorts))
				group := groups[key]
				if group == nil {
					group = &usageGroup{ids: make(map[string]struct{}), metric: map[string]any{
						"authorityClass": "ADVISORY", "cohortIds": cohorts, "completeness": "COMPLETE", "currency": "VALIDATED_AT",
						"denominator": strconv.Itoa(totals[revision]), "deliveryStage": "NOT_STARTED", "dimensions": dimensions,
						"epistemicClass": "OBSERVED", "exclusions": []any{}, "name": "usage.trace.retained", "numerator": nil,
						"scopeClass": "SINGLE_COHORT", "sourceIds": []any{}, "unit": "COUNT", "validity": "VALID", "value": nil, "window": nil,
					}}
					groups[key] = group
				}
				group.count += count
				group.ids[source.row["id"].(string)] = struct{}{}
			}
		}
	}
	result := make([]any, 0, len(groups))
	for _, group := range groups {
		text := strconv.Itoa(group.count)
		group.metric["value"], group.metric["numerator"] = text, text
		group.metric["sourceIds"] = stringsAny(sortedSet(group.ids))
		result = append(result, group.metric)
	}
	return result
}

func mustCanonical(value any) []byte {
	raw, _ := canonical(value, false)
	return raw
}

func anyStrings(values []any) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.(string)
	}
	return result
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func stringsAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}
