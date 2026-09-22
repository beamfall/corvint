package gotest

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

type rawMembers map[string]jsontext.Value

type decodedEvent struct {
	event Event
	key   string
	value string
	path  string
}

var runnerFields = map[string]struct{}{
	"Action": {}, "Elapsed": {}, "FailedBuild": {}, "ImportPath": {}, "Key": {},
	"Output": {}, "OutputType": {}, "Package": {}, "Path": {}, "Test": {},
	"Time": {}, "Value": {},
}

func decodeRunnerEvent(data []byte, line int) (decodedEvent, error) {
	if len(data) < 2 || data[0] != '{' || data[len(data)-1] != '}' {
		return decodedEvent{}, fail(FailureMalformedJSON, line, "event must be one object")
	}
	var raw rawMembers
	if err := json.Unmarshal(data, &raw); err != nil {
		if errors.Is(err, jsontext.ErrDuplicateName) {
			return decodedEvent{}, fail(FailureDuplicateKey, line, "duplicate object member")
		}
		return decodedEvent{}, fail(FailureMalformedJSON, line, "invalid event object")
	}
	action, present, err := requiredString(raw, "Action", false)
	if err != nil {
		return decodedEvent{}, fail(FailureInvalidField, line, "Action")
	}
	if !present || action == "" {
		return decodedEvent{}, fail(FailureMissingField, line, "Action")
	}
	if !knownAction(action) {
		return decodedEvent{}, fail(FailureUnknownAction, line, "Action")
	}
	for name := range raw {
		if _, ok := runnerFields[name]; !ok {
			return decodedEvent{}, fail(FailureUnknownField, line, "unknown object member")
		}
	}
	if action == "build-output" || action == "build-fail" {
		return decodeBuildEvent(raw, action, line)
	}
	return decodeTestEvent(raw, action, line)
}

func decodeBuildEvent(raw rawMembers, action string, line int) (decodedEvent, error) {
	allowed := fieldSet("Action", "ImportPath")
	if action == "build-output" {
		allowed["Output"] = struct{}{}
	}
	if !onlyRunnerFields(raw, allowed) {
		return decodedEvent{}, fail(FailureInvalidField, line, "build field matrix")
	}
	importPath, present, err := requiredString(raw, "ImportPath", false)
	if err != nil {
		return decodedEvent{}, fail(FailureInvalidField, line, "ImportPath")
	}
	if !present || importPath == "" {
		return decodedEvent{}, fail(FailureMissingField, line, "ImportPath")
	}
	result := decodedEvent{event: Event{Kind: BuildEvent, Action: action, ImportPath: importPath}}
	if action == "build-output" {
		output, outputPresent, outputErr := requiredString(raw, "Output", true)
		if outputErr != nil {
			return decodedEvent{}, fail(FailureInvalidField, line, "Output")
		}
		if !outputPresent {
			return decodedEvent{}, fail(FailureMissingField, line, "Output")
		}
		setEventOutput(&result.event, output)
	}
	return result, nil
}

func decodeTestEvent(raw rawMembers, action string, line int) (decodedEvent, error) {
	allowed := fieldSet("Action", "Package", "Time")
	switch action {
	case "run", "pause", "cont", "bench":
		allowed["Test"] = struct{}{}
	case "output":
		allowed["Test"] = struct{}{}
		allowed["Output"] = struct{}{}
		allowed["OutputType"] = struct{}{}
	case "pass", "skip":
		allowed["Test"] = struct{}{}
		allowed["Elapsed"] = struct{}{}
	case "fail":
		allowed["Test"] = struct{}{}
		allowed["Elapsed"] = struct{}{}
		allowed["FailedBuild"] = struct{}{}
	case "attr":
		allowed["Test"] = struct{}{}
		allowed["Key"] = struct{}{}
		allowed["Value"] = struct{}{}
	case "artifacts":
		allowed["Test"] = struct{}{}
		allowed["Path"] = struct{}{}
	}
	if !onlyRunnerFields(raw, allowed) {
		return decodedEvent{}, fail(FailureInvalidField, line, "test field matrix")
	}
	packageName, packagePresent, err := requiredString(raw, "Package", false)
	if err != nil {
		return decodedEvent{}, fail(FailureInvalidField, line, "Package")
	}
	if !packagePresent || packageName == "" {
		return decodedEvent{}, fail(FailureMissingField, line, "Package")
	}
	timeRaw, timePresent, err := requiredString(raw, "Time", false)
	if err != nil {
		return decodedEvent{}, fail(FailureInvalidField, line, "Time")
	}
	if !timePresent || timeRaw == "" {
		return decodedEvent{}, fail(FailureMissingField, line, "Time")
	}
	timeUTC, err := parseStrictTime(timeRaw)
	if err != nil {
		return decodedEvent{}, fail(FailureInvalidField, line, "Time")
	}
	result := decodedEvent{event: Event{
		Kind: TestEvent, Action: action, Package: packageName,
		RawTime: timeRaw, TimeUTC: timeUTC, HasTime: true,
	}}
	if _, ok := raw["Test"]; ok {
		testName, _, testErr := requiredString(raw, "Test", false)
		if testErr != nil || testName == "" {
			return decodedEvent{}, fail(FailureInvalidField, line, "Test")
		}
		result.event.Test = testName
	}
	if requiresTest(action) && result.event.Test == "" {
		return decodedEvent{}, fail(FailureMissingField, line, "Test")
	}
	if elapsedRaw, ok := raw["Elapsed"]; ok {
		ns, elapsedErr := parseStrictElapsed(elapsedRaw)
		if elapsedErr != nil {
			return decodedEvent{}, fail(FailureInvalidField, line, "Elapsed")
		}
		result.event.ElapsedNS, result.event.HasElapsed = ns, true
	}
	if (action == "pass" || action == "fail") && !result.event.HasElapsed {
		return decodedEvent{}, fail(FailureMissingField, line, "Elapsed")
	}

	switch action {
	case "output":
		output, present, outputErr := requiredString(raw, "Output", true)
		if outputErr != nil {
			return decodedEvent{}, fail(FailureInvalidField, line, "Output")
		}
		if !present {
			return decodedEvent{}, fail(FailureMissingField, line, "Output")
		}
		setEventOutput(&result.event, output)
		if outputType, typePresent, typeErr := requiredString(raw, "OutputType", true); typeErr != nil {
			return decodedEvent{}, fail(FailureInvalidField, line, "OutputType")
		} else if typePresent {
			switch outputType {
			case "", "frame", "error", "error-continue":
				result.event.OutputType = outputType
			default:
				return decodedEvent{}, fail(FailureInvalidField, line, "OutputType")
			}
		}
	case "fail":
		if failedBuild, present, failedErr := requiredString(raw, "FailedBuild", false); failedErr != nil {
			return decodedEvent{}, fail(FailureInvalidField, line, "FailedBuild")
		} else if present {
			if result.event.Test != "" || failedBuild == "" {
				return decodedEvent{}, fail(FailureInvalidField, line, "FailedBuild")
			}
			result.event.FailedBuild = failedBuild
		}
	case "attr":
		key, _, keyErr := requiredString(raw, "Key", true)
		value, _, valueErr := requiredString(raw, "Value", true)
		if keyErr != nil || valueErr != nil {
			return decodedEvent{}, fail(FailureInvalidField, line, "attribute")
		}
		result.key, result.value = key, value
		result.event.Attribute = &AttributeDigest{KeySHA256: digestString(key), ValueSHA256: digestString(value)}
	case "artifacts":
		path, present, pathErr := requiredString(raw, "Path", false)
		if pathErr != nil {
			return decodedEvent{}, fail(FailureInvalidField, line, "Path")
		}
		if !present || path == "" {
			return decodedEvent{}, fail(FailureMissingField, line, "Path")
		}
		result.path = path
	}
	return result, nil
}

func requiredString(raw rawMembers, name string, emptyAllowed bool) (string, bool, error) {
	value, ok := raw[name]
	if !ok {
		return "", false, nil
	}
	if len(value) < 2 || value[0] != '"' {
		return "", true, errors.New("field is not a string")
	}
	var decoded string
	if err := json.Unmarshal(value, &decoded); err != nil {
		return "", true, err
	}
	if !emptyAllowed && decoded == "" {
		return "", true, errors.New("field is empty")
	}
	return decoded, true, nil
}

func onlyRunnerFields(raw rawMembers, allowed map[string]struct{}) bool {
	for name := range raw {
		if _, ok := allowed[name]; !ok {
			return false
		}
	}
	return true
}

func fieldSet(names ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		result[name] = struct{}{}
	}
	return result
}

func knownAction(action string) bool {
	switch action {
	case "build-output", "build-fail", "start", "run", "pause", "cont", "pass", "bench", "fail", "output", "skip", "attr", "artifacts":
		return true
	default:
		return false
	}
}

func requiresTest(action string) bool {
	switch action {
	case "run", "pause", "cont", "bench", "attr", "artifacts":
		return true
	default:
		return false
	}
}

func setEventOutput(event *Event, output string) {
	event.OutputDigest = digestString(output)
	event.OutputBytes = uint64(len(output))
	event.HasOutput = true
}

func parseStrictElapsed(raw jsontext.Value) (int64, error) {
	value := string(raw)
	if value == "" || value[0] < '0' || value[0] > '9' {
		return 0, errors.New("elapsed is not unsigned decimal")
	}
	whole, fraction := value, ""
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		whole, fraction = value[:dot], value[dot+1:]
		if len(fraction) == 0 || len(fraction) > 9 {
			return 0, errors.New("elapsed precision")
		}
	}
	if whole == "" || (len(whole) > 1 && whole[0] == '0') || !digits(whole) || !digits(fraction) {
		return 0, errors.New("elapsed syntax")
	}
	seconds, err := strconv.ParseUint(whole, 10, 64)
	if err != nil || seconds > uint64(math.MaxInt64)/1_000_000_000 {
		return 0, errors.New("elapsed overflow")
	}
	for len(fraction) < 9 {
		fraction += "0"
	}
	var nanos uint64
	if fraction != "" {
		nanos, err = strconv.ParseUint(fraction, 10, 32)
		if err != nil {
			return 0, errors.New("elapsed fraction")
		}
	}
	total := seconds*1_000_000_000 + nanos
	if total > uint64(math.MaxInt64) {
		return 0, errors.New("elapsed overflow")
	}
	return int64(total), nil
}

func parseStrictTime(value string) (time.Time, error) {
	if !strictTimeShape(value) {
		return time.Time{}, errors.New("timestamp syntax")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func strictTimeShape(value string) bool {
	if len(value) < 20 || value[4] != '-' || value[7] != '-' || value[10] != 'T' || value[13] != ':' || value[16] != ':' {
		return false
	}
	if !digits(value[:4]) || !digits(value[5:7]) || !digits(value[8:10]) || !digits(value[11:13]) || !digits(value[14:16]) || !digits(value[17:19]) {
		return false
	}
	i := 19
	if i < len(value) && value[i] == '.' {
		start := i + 1
		for i = start; i < len(value) && value[i] >= '0' && value[i] <= '9'; i++ {
		}
		if i == start || i-start > 9 {
			return false
		}
	}
	if i == len(value)-1 && value[i] == 'Z' {
		return true
	}
	if len(value)-i != 6 || (value[i] != '+' && value[i] != '-') || value[i+3] != ':' {
		return false
	}
	if !digits(value[i+1:i+3]) || !digits(value[i+4:i+6]) {
		return false
	}
	zoneHour := int(value[i+1]-'0')*10 + int(value[i+2]-'0')
	zoneMinute := int(value[i+4]-'0')*10 + int(value[i+5]-'0')
	return zoneHour <= 23 && zoneMinute <= 59
}

func digits(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}
