package tcq

import (
	"github.com/Beamfall/corvint/internal/cem/wire"
)

var observationFields = []string{"commandId", "exitCode", "id", "report", "rows", "spec", "targetRevision", "unkeyedRows"}

// observation is the verified `test-observation/0.1-experimental` projection.
// TCQ-V0-032 makes it a caller-reported projection, never authorization: every
// row it declares is re-derived from the raw report before it is believed.
type observation struct {
	id             string
	commandID      string
	targetRevision string
	exitCode       int64
	reportSHA256   string
	reportBytes    int64
	unkeyedRows    int64
	rows           []wire.Value
}

// parseObservation implements TCQ-V0-031: exact shape, `exitCode` in
// `0..2147483647`, rows sorted by `(executionKeySha256,id)` and strictly unique,
// and a self-ID that reproduces from the document without it.
func parseObservation(raw []byte) (observation, error) {
	value, err := parseCanonical(raw, observationBounds, CodeInvalidObservation, CodeNoncanonicalObservation)
	if err != nil {
		return observation{}, err
	}
	object, err := exactObject(value, observationFields, CodeInvalidObservation)
	if err != nil {
		return observation{}, err
	}
	result, err := readObservationFields(object)
	if err != nil {
		return observation{}, err
	}
	identity, err := stringField(object, "id", CodeInvalidObservation)
	if err != nil {
		return observation{}, err
	}
	if !observationIDMatch.MatchString(identity) {
		return observation{}, fail(CodeInvalidObservation)
	}
	if identity != observationPrefix+domainHash(domainObservation, canonicalValue(withoutMember(value, "id"))) {
		return observation{}, fail(CodeInvalidObservation)
	}
	result.id = identity
	return result, nil
}

func readObservationFields(object *wire.Object) (observation, error) {
	spec, err := stringField(object, "spec", CodeInvalidObservation)
	if err != nil || spec != ObservationSpec {
		return observation{}, fail(CodeInvalidObservation)
	}
	commandID, err := stringField(object, "commandId", CodeInvalidObservation)
	if err != nil || !commandIDPattern.MatchString(commandID) {
		return observation{}, fail(CodeInvalidObservation)
	}
	target, err := stringField(object, "targetRevision", CodeInvalidObservation)
	if err != nil || !oidPattern.MatchString(target) {
		return observation{}, fail(CodeInvalidObservation)
	}
	exitCode, err := intField(object, "exitCode", 0, 2147483647, CodeInvalidObservation)
	if err != nil {
		return observation{}, err
	}
	unkeyed, err := intField(object, "unkeyedRows", 0, maxTestcases, CodeInvalidObservation)
	if err != nil {
		return observation{}, err
	}
	reportBytes, reportSHA, err := readReportSummary(object.Values["report"])
	if err != nil {
		return observation{}, err
	}
	rows, err := readObservationRows(object.Values["rows"])
	if err != nil {
		return observation{}, err
	}
	return observation{
		commandID: commandID, targetRevision: target, exitCode: exitCode,
		reportSHA256: reportSHA, reportBytes: reportBytes, unkeyedRows: unkeyed, rows: rows,
	}, nil
}

func readReportSummary(value wire.Value) (int64, string, error) {
	object, err := exactObject(value, []string{"bytes", "format", "sha256"}, CodeInvalidObservation)
	if err != nil {
		return 0, "", err
	}
	byteCount, err := intField(object, "bytes", 0, maxReportBytes, CodeInvalidObservation)
	if err != nil {
		return 0, "", err
	}
	format, err := stringField(object, "format", CodeInvalidObservation)
	if err != nil || format != ReportFormat {
		return 0, "", fail(CodeInvalidObservation)
	}
	digest, err := stringField(object, "sha256", CodeInvalidObservation)
	if err != nil || !shaPattern.MatchString(digest) {
		return 0, "", fail(CodeInvalidObservation)
	}
	return byteCount, digest, nil
}

var observationRowStatuses = map[string]bool{"PASSED": true, "FAILED": true, "ERROR": true, "SKIPPED": true}

func readObservationRows(value wire.Value) ([]wire.Value, error) {
	if value.Kind != wire.KindArray || len(value.Arr) > maxTestcases {
		return nil, fail(CodeInvalidObservation)
	}
	previousKey, previousID := "", ""
	for index, item := range value.Arr {
		object, err := exactObject(item, []string{"executionKeySha256", "id", "status"}, CodeInvalidObservation)
		if err != nil {
			return nil, err
		}
		key, identity, err := readObservationRow(object)
		if err != nil {
			return nil, err
		}
		if index > 0 && (key < previousKey || (key == previousKey && identity <= previousID)) {
			return nil, fail(CodeInvalidObservation)
		}
		previousKey, previousID = key, identity
	}
	return value.Arr, nil
}

func readObservationRow(object *wire.Object) (string, string, error) {
	key, err := stringField(object, "executionKeySha256", CodeInvalidObservation)
	if err != nil || !shaPattern.MatchString(key) {
		return "", "", fail(CodeInvalidObservation)
	}
	identity, err := stringField(object, "id", CodeInvalidObservation)
	if err != nil || !rowIDPattern.MatchString(identity) {
		return "", "", fail(CodeInvalidObservation)
	}
	status, err := stringField(object, "status", CodeInvalidObservation)
	if err != nil || !observationRowStatuses[status] {
		return "", "", fail(CodeInvalidObservation)
	}
	return key, identity, nil
}

// MakeTestObservation projects one caller-supplied JUnit report under one
// command into a canonical observation. Per TCQ-V0-029 an exit code of zero
// alongside any failed or errored testcase is `report-command-inconsistent`.
func MakeTestObservation(repository Repository, commandRaw, reportRaw []byte, targetRevision string, exitCode int64) ([]byte, error) {
	commandValue, err := parseCommand(commandRaw)
	if err != nil {
		return nil, err
	}
	if len(reportRaw) > maxReportBytes {
		return nil, fail(CodeResourceExhausted)
	}
	report, err := parseJUnit(repository, targetRevision, reportRaw)
	if err != nil {
		return nil, err
	}
	if exitCode == 0 && report.hasNonPassing {
		return nil, fail(CodeReportCommandInconsistent)
	}
	document := jsonObject(
		member{"commandId", jsonString(commandValue.id)},
		member{"exitCode", jsonInt(exitCode)},
		member{"report", jsonObject(
			member{"bytes", jsonInt(int64(report.byteLength))},
			member{"format", jsonString(ReportFormat)},
			member{"sha256", jsonString(report.sha256)},
		)},
		member{"rows", jsonArray(observationRows(report))},
		member{"spec", jsonString(ObservationSpec)},
		member{"targetRevision", jsonString(targetRevision)},
		member{"unkeyedRows", jsonInt(int64(report.unkeyedCount))},
	)
	identity := observationPrefix + domainHash(domainObservation, canonicalValue(document))
	document.Obj.Keys = append(document.Obj.Keys, "id")
	document.Obj.Values["id"] = jsonString(identity)
	raw := canonicalJSON(document)
	if _, err := parseObservation(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
