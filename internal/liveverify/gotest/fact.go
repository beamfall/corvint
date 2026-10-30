package gotest

import (
	"encoding/json/jsontext"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"
)

// CanonicalFactBody returns the normalized runner-fact payload. It deliberately
// excludes sequence, WEI, and event identity; those are bound by the coordinator
// only after the workspace execution identity exists.
func (e Event) CanonicalFactBody() ([]byte, error) {
	switch e.Kind {
	case TestEvent:
		return e.canonicalTestFact()
	case BuildEvent:
		return e.canonicalBuildFact()
	default:
		return nil, errors.New("go-test-json:invalid_fact")
	}
}

func (e Event) canonicalTestFact() ([]byte, error) {
	if e.Package == "" || e.Action == "" || !e.HasTime || e.RawTime == "" ||
		!allValidUTF8(e.Package, e.Action, e.RawTime, e.Test, e.OutputType, e.FailedBuild, e.ArtifactPathSHA256) {
		return nil, errors.New("go-test-json:invalid_fact")
	}
	parsedTime, err := parseStrictTime(e.RawTime)
	if err != nil || !parsedTime.Equal(e.TimeUTC) || !validTestFactShape(e) {
		return nil, errors.New("go-test-json:invalid_fact")
	}
	outputDigest := digestString("")
	if e.HasOutput {
		if !validDigest(e.OutputDigest) {
			return nil, errors.New("go-test-json:invalid_fact")
		}
		outputDigest = e.OutputDigest
	}
	if e.Attribute != nil && (!validDigest(e.Attribute.KeySHA256) || !validDigest(e.Attribute.ValueSHA256)) {
		return nil, errors.New("go-test-json:invalid_fact")
	}
	if e.ArtifactPathSHA256 != "" && !validDigest(e.ArtifactPathSHA256) {
		return nil, errors.New("go-test-json:invalid_fact")
	}
	b := make([]byte, 0, 768)
	b = append(b, `{"artifactPathSha256":`...)
	b = appendNullableString(b, e.ArtifactPathSHA256)
	b = append(b, `,"attribute":`...)
	if e.Attribute == nil {
		b = append(b, "null"...)
	} else {
		b = append(b, `{"keySha256":`...)
		b = appendString(b, e.Attribute.KeySHA256)
		b = append(b, `,"valueSha256":`...)
		b = appendString(b, e.Attribute.ValueSHA256)
		b = append(b, '}')
	}
	b = append(b, `,"durationNanoseconds":`...)
	if e.HasElapsed {
		b = appendString(b, strconv.FormatInt(e.ElapsedNS, 10))
	} else {
		b = append(b, "null"...)
	}
	b = append(b, `,"factKind":"TEST","failedBuild":`...)
	b = appendNullableString(b, e.FailedBuild)
	b = append(b, `,"output":{"byteCount":`...)
	b = appendString(b, strconv.FormatUint(e.OutputBytes, 10))
	b = append(b, `,"outputType":`...)
	b = appendNullableString(b, e.OutputType)
	b = append(b, `,"sha256":`...)
	b = appendString(b, outputDigest)
	b = append(b, `,"text":null,"truncated":false},"package":`...)
	b = appendString(b, e.Package)
	b = append(b, `,"parentTestId":null,"rawAction":`...)
	b = appendString(b, e.Action)
	b = append(b, `,"sourceAnchor":null,"test":`...)
	b = appendNullableString(b, e.Test)
	b = append(b, `,"timeRaw":`...)
	b = appendString(b, e.RawTime)
	b = append(b, `,"timeUtc":`...)
	b = appendString(b, e.TimeUTC.UTC().Format(time.RFC3339Nano))
	b = append(b, '}')
	return b, nil
}

func (e Event) canonicalBuildFact() ([]byte, error) {
	if e.ImportPath == "" || (e.Action != "build-output" && e.Action != "build-fail") || !allValidUTF8(e.ImportPath, e.Action) ||
		e.Package != "" || e.Test != "" || e.HasElapsed || e.HasTime || e.OutputType != "" || e.FailedBuild != "" ||
		e.Attribute != nil || e.ArtifactPathSHA256 != "" || (e.Action == "build-output") != e.HasOutput ||
		(!e.HasOutput && (e.OutputBytes != 0 || e.OutputDigest != "")) {
		return nil, errors.New("go-test-json:invalid_fact")
	}
	outputDigest := digestString("")
	if e.HasOutput {
		if !validDigest(e.OutputDigest) {
			return nil, errors.New("go-test-json:invalid_fact")
		}
		outputDigest = e.OutputDigest
	}
	b := make([]byte, 0, 384)
	b = append(b, `{"factKind":"BUILD","importPath":`...)
	b = appendString(b, e.ImportPath)
	b = append(b, `,"output":{"byteCount":`...)
	b = appendString(b, strconv.FormatUint(e.OutputBytes, 10))
	b = append(b, `,"sha256":`...)
	b = appendString(b, outputDigest)
	b = append(b, `,"text":null,"truncated":false},"rawAction":`...)
	b = appendString(b, e.Action)
	b = append(b, '}')
	return b, nil
}

func appendNullableString(dst []byte, value string) []byte {
	if value == "" {
		return append(dst, "null"...)
	}
	return appendString(dst, value)
}

func appendString(dst []byte, value string) []byte {
	result, err := jsontext.AppendQuote(dst, value)
	if err != nil {
		panic("unreachable: Go strings are normalized by the JSON decoder")
	}
	return result
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func validTestFactShape(e Event) bool {
	if e.ImportPath != "" || e.ElapsedNS < 0 {
		return false
	}
	switch e.Action {
	case "start":
		if e.Test != "" {
			return false
		}
	case "run", "pause", "cont", "bench", "attr", "artifacts":
		if e.Test == "" {
			return false
		}
	case "output":
		if e.OutputType != "" && e.OutputType != "frame" && e.OutputType != "error" && e.OutputType != "error-continue" {
			return false
		}
	case "pass", "fail", "skip":
	default:
		return false
	}
	if (e.Action == "output") != e.HasOutput || (!e.HasOutput && (e.OutputBytes != 0 || e.OutputDigest != "")) {
		return false
	}
	if e.OutputType != "" && e.Action != "output" {
		return false
	}
	if (e.Action == "pass" || e.Action == "fail") != e.HasElapsed && e.Action != "skip" && e.Action != "bench" {
		return false
	}
	if e.HasElapsed && e.Action != "pass" && e.Action != "fail" && e.Action != "skip" && e.Action != "bench" {
		return false
	}
	if (e.Attribute != nil) != (e.Action == "attr") || (e.ArtifactPathSHA256 != "") != (e.Action == "artifacts") {
		return false
	}
	if e.FailedBuild != "" && (e.Action != "fail" || e.Test != "") {
		return false
	}
	return true
}

func allValidUTF8(values ...string) bool {
	for _, value := range values {
		if !utf8.ValidString(value) {
			return false
		}
	}
	return true
}
