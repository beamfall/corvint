package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

const (
	profile              = "corvint-pulse-snapshot-conformance/0"
	protocol             = "corvint-pulse-snapshot/0"
	stage                = "P0-A"
	maxCorpusBytes       = 1_048_576
	maxLineBytes         = 1_048_576
	maxCases             = 64
	maxJSONDepth         = 256
	maxSnapshotEntries   = 200_000
	maxSnapshotBytes     = int64(1 << 30)
	maxSnapshotNameBytes = int64(64 << 20)
	commandTimeout       = 5 * time.Second
	maxCommandStdout     = maxLineBytes * 32
	maxCommandStderr     = maxLineBytes
	digestDomain         = profile + "\x00"
	frozenCorpusSHA256   = "2c333d4d8bf6789814bdc436860da46599d7015c751df45a7739117050856354"
	frozenCaseSetSHA256  = "a364aee5d5fc0a0f3868ee9479b62e573bfa79e9e924c6f7870ed4ed482fd406"
	repositoryMutation   = "command:repository-mutated"
	repositoryUnbounded  = "command:repository-snapshot-limit"
	repositoryUnsafe     = "command:repository-unsafe-entry"
)

//go:embed cases.json
var embeddedCorpus []byte

var (
	caseTimeout = commandTimeout
	sessionRE   = regexp.MustCompile(`^[A-Za-z0-9._~-]{16,128}$`)
	caseIDRE    = regexp.MustCompile(`^[a-z][a-z0-9.-]{2,63}$`)
	hexSHA256RE = regexp.MustCompile(`^[0-9a-f]{64}$`)
	integerRE   = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
)

var frozenCaseIDs = []string{
	"duplicate-initialize",
	"duplicate-key",
	"generation-mismatch",
	"initialize-shutdown",
	"overlay-close-unsupported",
	"overlay-replace-unsupported",
	"request-before-initialize",
	"sequence-gap",
	"sequence-regression",
	"snapshot-identity-incomplete",
	"unknown-field",
	"workspace-invalidate-order",
}

var requestTypes = stringSet(
	"initialize",
	"overlay.close",
	"overlay.replace",
	"shutdown",
	"snapshot.acquire",
	"workspace.invalidate",
)

var responseTypes = stringSet(
	"error",
	"initialized",
	"shutdown.complete",
	"workspace.invalidate.ack",
	"workspace.invalidated",
	"workspace.unknown",
)

var capabilities = map[string]any{
	"overlay.close":        false,
	"overlay.replace":      false,
	"shutdown":             true,
	"snapshot.acquire":     false,
	"workspace.invalidate": true,
}

type conformanceError string

func (e conformanceError) Error() string { return string(e) }

func failuref(format string, arguments ...any) error {
	return conformanceError(fmt.Sprintf(format, arguments...))
}

type corpusDocument struct {
	caseSetSHA256 string
	cases         []corpusCase
	raw           map[string]any
}

type corpusCase struct {
	caseSHA256    string
	expectedExit  int
	expectedLines []string
	id            string
	requestLines  []string
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func parseJSON(raw []byte, label string) (any, error) {
	if !utf8.Valid(raw) {
		return nil, failuref("%s:invalid-json", label)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeValue(decoder, label, 0)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, failuref("%s:invalid-json", label)
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder, label string, depth int) (any, error) {
	if depth > maxJSONDepth {
		return nil, failuref("%s:invalid-json", label)
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, failuref("%s:invalid-json", label)
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return token, nil
	}
	switch delimiter {
	case '{':
		result := make(map[string]any)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, failuref("%s:invalid-json", label)
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, failuref("%s:invalid-json", label)
			}
			if _, exists := result[key]; exists {
				return nil, failuref("%s:duplicate-key:%s", label, key)
			}
			value, err := decodeValue(decoder, label, depth+1)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
			return nil, failuref("%s:invalid-json", label)
		}
		return result, nil
	case '[':
		result := make([]any, 0)
		for decoder.More() {
			value, err := decodeValue(decoder, label, depth+1)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
			return nil, failuref("%s:invalid-json", label)
		}
		return result, nil
	default:
		return nil, failuref("%s:invalid-json", label)
	}
}

func canonical(value any, lf bool) ([]byte, error) {
	var output bytes.Buffer
	if err := appendCanonical(&output, value); err != nil {
		return nil, err
	}
	if lf {
		output.WriteByte('\n')
	}
	return output.Bytes(), nil
}

func appendCanonical(output *bytes.Buffer, value any) error {
	switch typed := value.(type) {
	case nil:
		output.WriteString("null")
	case bool:
		if typed {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case string:
		appendJSONString(output, typed)
	case json.Number:
		number, err := canonicalNumber(string(typed))
		if err != nil {
			return err
		}
		output.WriteString(number)
	case int:
		output.WriteString(strconv.Itoa(typed))
	case int64:
		output.WriteString(strconv.FormatInt(typed, 10))
	case []string:
		output.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				output.WriteByte(',')
			}
			appendJSONString(output, item)
		}
		output.WriteByte(']')
	case []any:
		output.WriteByte('[')
		for index, item := range typed {
			if index > 0 {
				output.WriteByte(',')
			}
			if err := appendCanonical(output, item); err != nil {
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
			appendJSONString(output, key)
			output.WriteByte(':')
			if err := appendCanonical(output, typed[key]); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON type %T", value)
	}
	return nil
}

func appendJSONString(output *bytes.Buffer, value string) {
	const hexDigits = "0123456789abcdef"
	output.WriteByte('"')
	for _, character := range []byte(value) {
		switch character {
		case '"', '\\':
			output.WriteByte('\\')
			output.WriteByte(character)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if character < 0x20 {
				output.WriteString(`\u00`)
				output.WriteByte(hexDigits[character>>4])
				output.WriteByte(hexDigits[character&0x0f])
			} else {
				output.WriteByte(character)
			}
		}
	}
	output.WriteByte('"')
}

func canonicalNumber(raw string) (string, error) {
	if integerRE.MatchString(raw) {
		integer := new(big.Int)
		if _, ok := integer.SetString(raw, 10); !ok {
			return "", errors.New("invalid JSON number")
		}
		return integer.String(), nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return "", errors.New("invalid JSON number")
	}
	scientific := strconv.FormatFloat(value, 'e', -1, 64)
	exponentIndex := strings.LastIndexByte(scientific, 'e')
	exponent, err := strconv.Atoi(scientific[exponentIndex+1:])
	if err != nil {
		return "", errors.New("invalid JSON number")
	}
	if exponent >= -4 && exponent < 16 {
		fixed := strconv.FormatFloat(value, 'f', -1, 64)
		if !strings.Contains(fixed, ".") {
			fixed += ".0"
		}
		return fixed, nil
	}
	return scientific, nil
}

func digest(kind string, raw []byte) string {
	value := sha256.New()
	_, _ = io.WriteString(value, digestDomain)
	_, _ = io.WriteString(value, kind)
	_, _ = value.Write([]byte{0})
	_, _ = value.Write(raw)
	return hex.EncodeToString(value.Sum(nil))
}

func closed(value any, fields []string, label string) (map[string]any, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return nil, failuref("%s:not-object", label)
	}
	expected := stringSet(fields...)
	missing := make([]string, 0)
	extra := make([]string, 0)
	for field := range expected {
		if _, ok := object[field]; !ok {
			missing = append(missing, field)
		}
	}
	for field := range object {
		if _, ok := expected[field]; !ok {
			extra = append(extra, field)
		}
	}
	if len(missing) != 0 || len(extra) != 0 {
		sort.Strings(missing)
		sort.Strings(extra)
		return nil, failuref("%s:closed-shape:missing=%s:extra=%s", label, strings.Join(missing, ","), strings.Join(extra, ","))
	}
	return object, nil
}

func exactInt(value any) (int64, bool) {
	number, ok := value.(json.Number)
	if !ok || !integerRE.MatchString(string(number)) {
		return 0, false
	}
	result, err := strconv.ParseInt(string(number), 10, 64)
	return result, err == nil
}

func stringArray(value any) ([]string, bool) {
	raw, ok := value.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, len(raw))
	for index, item := range raw {
		text, ok := item.(string)
		if !ok {
			return nil, false
		}
		result[index] = text
	}
	return result, true
}

func validateTemplateLine(rawText any, label string, request bool) ([]byte, error) {
	text, ok := rawText.(string)
	if !ok {
		return nil, failuref("%s:not-string", label)
	}
	raw := []byte(text)
	if len(raw) == 0 || len(raw)+1 > maxLineBytes || strings.ContainsAny(text, "\r\n") {
		return nil, failuref("%s:invalid-line", label)
	}
	if request && strings.Contains(text, "$SESSION") {
		return nil, failuref("%s:request-session-placeholder", label)
	}
	if !request && strings.Count(text, "$SESSION") > 1 {
		return nil, failuref("%s:too-many-session-placeholders", label)
	}
	parsed, err := parseJSON(raw, label)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(text, "$SESSION") {
		encoded, err := canonical(parsed, false)
		if err != nil || !bytes.Equal(raw, encoded) {
			return nil, failuref("%s:not-canonical", label)
		}
	} else {
		substituted := []byte(strings.ReplaceAll(text, "$SESSION", strings.Repeat("S", 16)))
		parsedSubstituted, err := parseJSON(substituted, label)
		if err != nil {
			return nil, err
		}
		encoded, err := canonical(parsedSubstituted, false)
		if err != nil || !bytes.Equal(substituted, encoded) {
			return nil, failuref("%s:template-not-canonical", label)
		}
	}
	return append(raw, '\n'), nil
}

func validateRequestShape(rawText, caseID string, index int) error {
	if caseID == "duplicate-key" && index == 1 {
		return nil
	}
	label := fmt.Sprintf("%s:request[%d]", caseID, index)
	value, err := parseJSON([]byte(rawText), label)
	if err != nil {
		return err
	}
	fields := []string{"body", "clientSeq", "protocol", "requestId", "type"}
	if caseID == "unknown-field" && index == 1 {
		fields = append(fields, "unexpected")
	}
	request, err := closed(value, fields, label)
	if err != nil {
		return err
	}
	sequence, ok := exactInt(request["clientSeq"])
	if !ok || sequence < 1 || sequence >= 1<<53 {
		return failuref("%s:invalid-client-sequence", label)
	}
	requestID, ok := request["requestId"].(string)
	if !ok || len(requestID) < 1 || len(requestID) > 128 {
		return failuref("%s:invalid-request-id", label)
	}
	typeName, typeOK := request["type"].(string)
	_, supportedType := requestTypes[typeName]
	if request["protocol"] != protocol || !typeOK || !supportedType {
		return failuref("%s:invalid-protocol-or-type", label)
	}
	if _, ok := request["body"].(map[string]any); !ok {
		return failuref("%s:body-not-object", label)
	}
	return nil
}

func validateExpectedShape(rawText, caseID string, index int) error {
	label := fmt.Sprintf("%s:expected[%d]", caseID, index)
	rendered := strings.ReplaceAll(rawText, "$SESSION", strings.Repeat("S", 16))
	parsed, err := parseJSON([]byte(rendered), label)
	if err != nil {
		return err
	}
	value, err := closed(parsed, []string{"body", "protocol", "requestId", "serverSeq", "sessionId", "type"}, label)
	if err != nil {
		return err
	}
	typeName, typeOK := value["type"].(string)
	_, supportedType := responseTypes[typeName]
	if value["protocol"] != protocol || !typeOK || !supportedType {
		return failuref("%s:invalid-protocol-or-type", label)
	}
	serverSequence, ok := exactInt(value["serverSeq"])
	if !ok || serverSequence != int64(index+1) {
		return failuref("%s:nonmonotonic-server-sequence", label)
	}
	session, sessionIsString := value["sessionId"].(string)
	if value["sessionId"] != nil && (!sessionIsString || session != strings.Repeat("S", 16)) {
		return failuref("%s:invalid-session-template", label)
	}
	if strings.Contains(rawText, "$SESSION") && (!sessionIsString || session != strings.Repeat("S", 16)) {
		return failuref("%s:misplaced-session-template", label)
	}
	if value["requestId"] != nil {
		requestID, ok := value["requestId"].(string)
		if !ok || requestID == "" {
			return failuref("%s:invalid-request-id", label)
		}
	}
	body, ok := value["body"].(map[string]any)
	if !ok {
		return failuref("%s:body-not-object", label)
	}
	if typeName == "initialized" {
		expectedRaw, _ := canonical(capabilities, false)
		actual, _ := canonical(body["capabilities"], false)
		generation, generationOK := exactInt(body["workspaceGeneration"])
		if len(body) != 2 || !bytes.Equal(actual, expectedRaw) || !generationOK || generation != 0 {
			return failuref("%s:invalid-p0a-capabilities", label)
		}
	}
	return nil
}

func validateCase(raw any, ordinal int) (corpusCase, error) {
	var result corpusCase
	label := fmt.Sprintf("case[%d]", ordinal)
	value, err := closed(raw, []string{"caseSha256", "expectedExitCode", "expectedLines", "id", "requestLines"}, label)
	if err != nil {
		return result, err
	}
	caseID, ok := value["id"].(string)
	if !ok || !caseIDRE.MatchString(caseID) {
		return result, failuref("%s:invalid-id", label)
	}
	exitCode, ok := exactInt(value["expectedExitCode"])
	if !ok || (exitCode != 0 && exitCode != 2) {
		return result, failuref("%s:invalid-exit-code", caseID)
	}
	requests, ok := stringArray(value["requestLines"])
	if !ok || len(requests) < 1 || len(requests) > 16 {
		return result, failuref("%s:invalid-request-count", caseID)
	}
	expected, ok := stringArray(value["expectedLines"])
	if !ok || len(expected) < 1 || len(expected) > 32 {
		return result, failuref("%s:invalid-expected-count", caseID)
	}
	for index, text := range requests {
		requestLabel := fmt.Sprintf("%s:request[%d]", caseID, index)
		if caseID == "duplicate-key" && index == 1 {
			if strings.Count(text, `"type"`) != 2 {
				return result, failuref("%s:missing-frozen-duplicate", requestLabel)
			}
			_, duplicateErr := parseJSON([]byte(text), requestLabel)
			if duplicateErr == nil {
				return result, failuref("%s:duplicate-not-rejected", requestLabel)
			}
			if !strings.Contains(duplicateErr.Error(), ":duplicate-key:type") {
				return result, duplicateErr
			}
			if strings.ContainsAny(text, "\r\n") || len([]byte(text))+1 > maxLineBytes {
				return result, failuref("%s:invalid-line", requestLabel)
			}
		} else {
			if _, err := validateTemplateLine(text, requestLabel, true); err != nil {
				return result, err
			}
			if err := validateRequestShape(text, caseID, index); err != nil {
				return result, err
			}
		}
	}
	for index, text := range expected {
		if _, err := validateTemplateLine(text, fmt.Sprintf("%s:expected[%d]", caseID, index), false); err != nil {
			return result, err
		}
		if err := validateExpectedShape(text, caseID, index); err != nil {
			return result, err
		}
	}
	basis := make(map[string]any, len(value)-1)
	for key, item := range value {
		if key != "caseSha256" {
			basis[key] = item
		}
	}
	basisRaw, err := canonical(basis, false)
	if err != nil {
		return result, err
	}
	expectedDigest := digest("case", basisRaw)
	actualDigest, ok := value["caseSha256"].(string)
	if !ok || !hexSHA256RE.MatchString(actualDigest) {
		return result, failuref("%s:invalid-case-digest", caseID)
	}
	if actualDigest != expectedDigest {
		return result, failuref("%s:case-digest-mismatch", caseID)
	}
	return corpusCase{
		caseSHA256:    actualDigest,
		expectedExit:  int(exitCode),
		expectedLines: expected,
		id:            caseID,
		requestLines:  requests,
	}, nil
}

func loadCorpus(raw []byte, requireCanonical bool) (corpusDocument, error) {
	var result corpusDocument
	if len(raw) == 0 || len(raw) > maxCorpusBytes {
		return result, conformanceError("corpus:invalid-size")
	}
	parsed, err := parseJSON(raw, "corpus")
	if err != nil {
		return result, err
	}
	document, err := closed(parsed, []string{"caseSetSha256", "cases", "profile", "stage"}, "corpus")
	if err != nil {
		return result, err
	}
	if requireCanonical {
		canonicalRaw, err := canonical(document, true)
		if err != nil || !bytes.Equal(raw, canonicalRaw) {
			return result, conformanceError("corpus:not-canonical-json-plus-lf")
		}
		corpusDigest := sha256.Sum256(raw)
		if hex.EncodeToString(corpusDigest[:]) != frozenCorpusSHA256 {
			return result, conformanceError("corpus:frozen-digest-mismatch")
		}
	}
	if document["profile"] != profile || document["stage"] != stage {
		return result, conformanceError("corpus:wrong-profile-or-stage")
	}
	rawCases, ok := document["cases"].([]any)
	if !ok || len(rawCases) < 1 || len(rawCases) > maxCases {
		return result, conformanceError("corpus:invalid-case-count")
	}
	cases := make([]corpusCase, len(rawCases))
	caseIDs := make([]string, len(rawCases))
	digests := make([]any, len(rawCases))
	for index, rawCase := range rawCases {
		validated, err := validateCase(rawCase, index)
		if err != nil {
			return result, err
		}
		cases[index] = validated
		caseIDs[index] = validated.id
		digests[index] = validated.caseSHA256
	}
	if len(caseIDs) != len(frozenCaseIDs) {
		return result, conformanceError("corpus:case-ids-not-frozen-unique-sorted")
	}
	for index := range caseIDs {
		if caseIDs[index] != frozenCaseIDs[index] {
			return result, conformanceError("corpus:case-ids-not-frozen-unique-sorted")
		}
	}
	setBasis, _ := canonical(digests, false)
	expectedSetDigest := digest("case-set", setBasis)
	actualSetDigest, ok := document["caseSetSha256"].(string)
	if !ok || actualSetDigest != expectedSetDigest {
		return result, conformanceError("corpus:case-set-digest-mismatch")
	}
	if actualSetDigest != frozenCaseSetSHA256 {
		return result, conformanceError("corpus:frozen-case-set-digest-mismatch")
	}
	return corpusDocument{caseSetSHA256: actualSetDigest, cases: cases, raw: document}, nil
}

func embeddedSelfTests(document corpusDocument) error {
	raw, _ := canonical(document.raw, true)
	type mutation struct {
		raw      []byte
		expected string
	}
	mutations := make([]mutation, 0, 3)

	wrongProfile, _ := parseJSON(raw, "self-test")
	wrongProfile.(map[string]any)["stage"] = "P0-B"
	wrongProfileRaw, _ := canonical(wrongProfile, false)
	mutations = append(mutations, mutation{wrongProfileRaw, "wrong-profile-or-stage"})

	wrongDigest, _ := parseJSON(raw, "self-test")
	wrongDigestCases := wrongDigest.(map[string]any)["cases"].([]any)
	wrongDigestCases[0].(map[string]any)["caseSha256"] = strings.Repeat("0", 64)
	wrongDigestRaw, _ := canonical(wrongDigest, false)
	mutations = append(mutations, mutation{wrongDigestRaw, "case-digest-mismatch"})

	duplicateID, _ := parseJSON(raw, "self-test")
	duplicateDocument := duplicateID.(map[string]any)
	duplicateCases := duplicateDocument["cases"].([]any)
	firstID := duplicateCases[0].(map[string]any)["id"]
	changed := duplicateCases[2].(map[string]any)
	changed["id"] = firstID
	basis := make(map[string]any, len(changed)-1)
	for key, value := range changed {
		if key != "caseSha256" {
			basis[key] = value
		}
	}
	basisRaw, _ := canonical(basis, false)
	changed["caseSha256"] = digest("case", basisRaw)
	digestValues := make([]any, len(duplicateCases))
	for index, rawCase := range duplicateCases {
		digestValues[index] = rawCase.(map[string]any)["caseSha256"]
	}
	setBasis, _ := canonical(digestValues, false)
	duplicateDocument["caseSetSha256"] = digest("case-set", setBasis)
	duplicateRaw, _ := canonical(duplicateDocument, false)
	mutations = append(mutations, mutation{duplicateRaw, "case-ids-not-frozen-unique-sorted"})

	for _, test := range mutations {
		_, err := loadCorpus(test.raw, false)
		if err == nil {
			return failuref("self-test:mutation-not-rejected:%s", test.expected)
		}
		if !strings.Contains(err.Error(), test.expected) {
			return failuref("self-test:expected-%s:got-%s", test.expected, err)
		}
	}
	return nil
}

func minimalEnvironment() []string {
	allowed := []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP"}
	result := make([]string, 0, len(allowed)+2)
	for _, key := range allowed {
		if value, ok := os.LookupEnv(key); ok {
			result = append(result, key+"="+value)
		}
	}
	return append(result, "LANG=C", "LC_ALL=C")
}

func configureOwnedProcess(command *exec.Cmd) {
	attributes := &syscall.SysProcAttr{}
	setProcessGroup := reflect.ValueOf(attributes).Elem().FieldByName("Setpgid")
	if setProcessGroup.IsValid() && setProcessGroup.CanSet() && setProcessGroup.Kind() == reflect.Bool {
		setProcessGroup.SetBool(true)
	}
	command.SysProcAttr = attributes
}

func terminateOwnedProcess(process *os.Process) {
	if process == nil {
		return
	}
	if runtime.GOOS != "windows" {
		if group, err := os.FindProcess(-process.Pid); err == nil {
			_ = group.Signal(os.Kill)
		}
	}
	_ = process.Kill()
}

type limitedCapture struct {
	limit    int
	buffer   bytes.Buffer
	overflow bool
}

func (capture *limitedCapture) read(reader io.Reader, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	block := make([]byte, 32*1024)
	for {
		count, err := reader.Read(block)
		if count > 0 {
			remaining := capture.limit + 1 - capture.buffer.Len()
			if remaining > 0 {
				if count < remaining {
					remaining = count
				}
				_, _ = capture.buffer.Write(block[:remaining])
			}
			if capture.buffer.Len() > capture.limit {
				capture.overflow = true
			}
		}
		if err != nil {
			return
		}
	}
}

type commandResult struct {
	exitCode int
	stdout   []byte
	stderr   []byte
}

type processWait struct {
	state *os.ProcessState
	err   error
}

func runCommandContext(ctx context.Context, command []string, request []byte, caseID string, timeout time.Duration) (commandResult, error) {
	var result commandResult
	process := exec.Command(command[0], command[1:]...)
	process.Env = minimalEnvironment()
	configureOwnedProcess(process)
	stdin, err := process.StdinPipe()
	if err != nil {
		return result, failuref("%s:cannot-execute:StartError", caseID)
	}
	stdout, err := process.StdoutPipe()
	if err != nil {
		return result, failuref("%s:cannot-execute:StartError", caseID)
	}
	stderr, err := process.StderrPipe()
	if err != nil {
		return result, failuref("%s:cannot-execute:StartError", caseID)
	}
	if err := process.Start(); err != nil {
		return result, failuref("%s:cannot-execute:StartError", caseID)
	}

	stdoutCapture := &limitedCapture{limit: maxCommandStdout}
	stderrCapture := &limitedCapture{limit: maxCommandStderr}
	readDone := make(chan struct{}, 2)
	go stdoutCapture.read(stdout, readDone)
	go stderrCapture.read(stderr, readDone)
	writeDone := make(chan struct{}, 1)
	go func() {
		_, _ = stdin.Write(request)
		_ = stdin.Close()
		writeDone <- struct{}{}
	}()
	waitDone := make(chan processWait, 1)
	go func() {
		state, err := process.Process.Wait()
		waitDone <- processWait{state: state, err: err}
	}()

	timer := time.NewTimer(timeout)
	timedOut := false
	interrupted := false
	var waited processWait
	select {
	case <-timer.C:
		timedOut = true
		terminateOwnedProcess(process.Process)
		waited = <-waitDone
	case <-ctx.Done():
		interrupted = true
		if !timer.Stop() {
			<-timer.C
		}
		terminateOwnedProcess(process.Process)
		waited = <-waitDone
	case waited = <-waitDone:
		if !timer.Stop() {
			<-timer.C
		}
	}
	terminateOwnedProcess(process.Process)
	_ = stdin.Close()
	<-writeDone
	<-readDone
	<-readDone
	_ = stdout.Close()
	_ = stderr.Close()
	if timedOut {
		return result, failuref("%s:cannot-execute:TimeoutExpired", caseID)
	}
	if interrupted {
		return result, failuref("%s:cannot-execute:Interrupted", caseID)
	}
	if waited.err != nil || waited.state == nil {
		return result, failuref("%s:cannot-execute:WaitError", caseID)
	}
	result.exitCode = waited.state.ExitCode()
	result.stdout = append([]byte(nil), stdoutCapture.buffer.Bytes()...)
	result.stderr = append([]byte(nil), stderrCapture.buffer.Bytes()...)
	if stdoutCapture.overflow || stderrCapture.overflow {
		return result, failuref("%s:output-too-large", caseID)
	}
	return result, nil
}

func extractSession(lines [][]byte, caseID string) (string, error) {
	sessions := make(map[string]struct{})
	for index, line := range lines {
		label := fmt.Sprintf("%s:stdout[%d]", caseID, index)
		parsed, err := parseJSON(bytes.TrimSuffix(line, []byte{'\n'}), label)
		if err != nil {
			return "", err
		}
		object, ok := parsed.(map[string]any)
		if !ok {
			return "", failuref("%s:not-object", label)
		}
		if object["type"] == "initialized" {
			session, ok := object["sessionId"].(string)
			if !ok || !sessionRE.MatchString(session) {
				return "", failuref("%s:invalid-session-id", caseID)
			}
			sessions[session] = struct{}{}
		}
	}
	if len(sessions) > 1 {
		return "", failuref("%s:multiple-session-ids", caseID)
	}
	for session := range sessions {
		return session, nil
	}
	return "", nil
}

func expectedStdout(test corpusCase, session string) ([]byte, error) {
	var output bytes.Buffer
	for index, template := range test.expectedLines {
		if strings.Contains(template, "$SESSION") {
			if session == "" {
				return nil, failuref("%s:expected[%d]:missing-session", test.id, index)
			}
			template = strings.ReplaceAll(template, "$SESSION", session)
		}
		output.WriteString(template)
		output.WriteByte('\n')
	}
	return output.Bytes(), nil
}

func executeCase(test corpusCase, command []string) (string, error) {
	return executeCaseContext(context.Background(), test, command)
}

func executeCaseContext(ctx context.Context, test corpusCase, command []string) (string, error) {
	request := []byte(strings.Join(test.requestLines, "\n") + "\n")
	completed, err := runCommandContext(ctx, command, request, test.id, caseTimeout)
	if err != nil {
		return "", err
	}
	if len(completed.stdout) > maxCommandStdout || len(completed.stderr) > maxCommandStderr {
		return "", failuref("%s:output-too-large", test.id)
	}
	if len(completed.stderr) != 0 {
		return "", failuref("%s:unexpected-stderr", test.id)
	}
	lines := make([][]byte, 0, len(test.expectedLines))
	remainder := completed.stdout
	for len(remainder) > 0 {
		if len(lines) >= 32 {
			return "", failuref("%s:invalid-output-framing", test.id)
		}
		newline := bytes.IndexByte(remainder, '\n')
		if newline < 0 {
			return "", failuref("%s:invalid-output-framing", test.id)
		}
		line := remainder[:newline+1]
		if len(line) > maxLineBytes {
			return "", failuref("%s:invalid-output-framing", test.id)
		}
		lines = append(lines, line)
		remainder = remainder[newline+1:]
	}
	session, err := extractSession(lines, test.id)
	if err != nil {
		return "", err
	}
	expected, err := expectedStdout(test, session)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(completed.stdout, expected) {
		return "", failuref("%s:stdout-byte-mismatch", test.id)
	}
	if completed.exitCode != test.expectedExit {
		return "", failuref("%s:exit-code:%d!=%d", test.id, completed.exitCode, test.expectedExit)
	}
	return session, nil
}

func executeCases(document corpusDocument, command []string) error {
	return executeCasesContext(context.Background(), document, command)
}

func executeCasesContext(ctx context.Context, document corpusDocument, command []string) error {
	if len(command) == 0 || command[0] == "" || strings.HasPrefix(command[0], "-") {
		return conformanceError("command:explicit-executable-required")
	}
	before, root, boundCommand, err := snapshotCommandRoot(command)
	if err != nil {
		return err
	}
	sessions := make(map[string]struct{})
	var executionError error
	for _, test := range document.cases {
		if err := ctx.Err(); err != nil {
			executionError = conformanceError("command:cannot-execute:Interrupted")
			break
		}
		session, err := executeCaseContext(ctx, test, boundCommand)
		if err != nil {
			executionError = err
			break
		}
		if session != "" {
			if _, reused := sessions[session]; reused {
				executionError = conformanceError("command:session-id-reused-across-processes")
				break
			}
			sessions[session] = struct{}{}
		}
	}
	if root != "" {
		after, err := snapshotRepositoryState(root)
		if err != nil {
			return err
		}
		if !sameRepositoryState(before, after) {
			return conformanceError(repositoryMutation)
		}
	}
	return executionError
}

func commandRoot(command []string) string {
	for index := 1; index < len(command); index++ {
		if command[index] == "--root" && index+1 < len(command) {
			return command[index+1]
		}
		if strings.HasPrefix(command[index], "--root=") {
			return strings.TrimPrefix(command[index], "--root=")
		}
	}
	return ""
}

func snapshotCommandRoot(command []string) (repositoryState, string, []string, error) {
	var empty repositoryState
	bound := append([]string(nil), command...)
	root := ""
	rootValueIndex := -1
	rootEqualsIndex := -1
	rootCount := 0
	for index := 1; index < len(command); index++ {
		switch {
		case command[index] == "--root":
			if index+1 >= len(command) || command[index+1] == "" {
				return empty, "", nil, conformanceError(repositoryUnbounded)
			}
			rootCount++
			root = command[index+1]
			rootValueIndex = index + 1
			index++
		case strings.HasPrefix(command[index], "--root="):
			value := strings.TrimPrefix(command[index], "--root=")
			if value == "" {
				return empty, "", nil, conformanceError(repositoryUnbounded)
			}
			rootCount++
			root = value
			rootEqualsIndex = index
		}
	}
	if root == "" {
		return empty, "", bound, nil
	}
	if rootCount != 1 {
		return empty, "", nil, conformanceError(repositoryUnbounded)
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return empty, "", nil, conformanceError(repositoryUnbounded)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return empty, "", nil, conformanceError(repositoryUnbounded)
	}
	if rootValueIndex >= 0 {
		bound[rootValueIndex] = resolved
	} else {
		bound[rootEqualsIndex] = "--root=" + resolved
	}
	digest, err := snapshotRepositoryState(resolved)
	return digest, resolved, bound, err
}

type repositoryState struct {
	digest     string
	identities []os.FileInfo
}

type snapshotRoot struct {
	label string
	root  *os.Root
}

type snapshotBudget struct {
	entries      int
	contentBytes int64
	nameBytes    int64
}

type snapshotHook func(path, phase string)

func sameRepositoryState(left, right repositoryState) bool {
	if left.digest != right.digest || len(left.identities) != len(right.identities) {
		return false
	}
	for index := range left.identities {
		if !os.SameFile(left.identities[index], right.identities[index]) {
			return false
		}
	}
	return true
}

func snapshotRepositoryState(root string) (repositoryState, error) {
	var empty repositoryState
	worktree, err := openSnapshotRoot("worktree", root)
	if err != nil {
		return empty, err
	}
	roots := []snapshotRoot{worktree}
	defer func() {
		for _, item := range roots {
			_ = item.root.Close()
		}
	}()

	gitInfo, err := worktree.root.Lstat(".git")
	if err != nil && !os.IsNotExist(err) {
		return empty, conformanceError(repositoryMutation)
	}
	if err == nil && !gitInfo.IsDir() {
		if !gitInfo.Mode().IsRegular() {
			return empty, conformanceError(repositoryUnsafe)
		}
		gitDirectory, err := readGitPath(worktree.root, ".git", "gitdir: ", root)
		if err != nil {
			return empty, err
		}
		gitRoot, err := openSnapshotRoot("gitdir", gitDirectory)
		if err != nil {
			return empty, err
		}
		roots = append(roots, gitRoot)
		commonInfo, err := gitRoot.root.Lstat("commondir")
		if err == nil {
			if !commonInfo.Mode().IsRegular() {
				return empty, conformanceError(repositoryUnsafe)
			}
			commonDirectory, err := readGitPath(gitRoot.root, "commondir", "", gitDirectory)
			if err != nil {
				return empty, err
			}
			if commonDirectory != gitDirectory {
				commonRoot, err := openSnapshotRoot("commondir", commonDirectory)
				if err != nil {
					return empty, err
				}
				roots = append(roots, commonRoot)
			}
		} else if !os.IsNotExist(err) {
			return empty, conformanceError(repositoryMutation)
		}
	}

	combined := sha256.New()
	identities := make([]os.FileInfo, 0, len(roots))
	for _, item := range roots {
		digest, identity, err := snapshotOpenRoot(item.root, nil)
		if err != nil {
			return empty, err
		}
		writeDigestField(combined, []byte(item.label))
		writeDigestField(combined, []byte(digest))
		identities = append(identities, identity)
	}
	return repositoryState{digest: hex.EncodeToString(combined.Sum(nil)), identities: identities}, nil
}

func openSnapshotRoot(label, path string) (snapshotRoot, error) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		return snapshotRoot{}, conformanceError(repositoryUnbounded)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return snapshotRoot{}, conformanceError(repositoryMutation)
	}
	return snapshotRoot{label: label, root: root}, nil
}

func readGitPath(root *os.Root, marker, prefix, relativeTo string) (string, error) {
	raw, err := readBoundedRegular(root, marker, maxLineBytes, nil)
	if err != nil || len(raw) == 0 || !utf8.Valid(raw) {
		return "", conformanceError(repositoryUnbounded)
	}
	text := strings.TrimSuffix(string(raw), "\n")
	text = strings.TrimSuffix(text, "\r")
	if strings.ContainsAny(text, "\r\n\x00") || !strings.HasPrefix(text, prefix) {
		return "", conformanceError(repositoryUnbounded)
	}
	path := strings.TrimPrefix(text, prefix)
	if path == "" {
		return "", conformanceError(repositoryUnbounded)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(relativeTo, path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", conformanceError(repositoryUnbounded)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", conformanceError(repositoryUnbounded)
	}
	return resolved, nil
}

func snapshotRepository(root string) (string, error) {
	return snapshotRepositoryWithHook(root, nil)
}

func snapshotRepositoryWithHook(path string, hook snapshotHook) (string, error) {
	root, err := openSnapshotRoot("repository", path)
	if err != nil {
		return "", err
	}
	defer root.root.Close()
	digest, _, err := snapshotOpenRoot(root.root, hook)
	return digest, err
}

func snapshotOpenRoot(root *os.Root, hook snapshotHook) (string, os.FileInfo, error) {
	identity, err := root.Lstat(".")
	if err != nil || !identity.IsDir() {
		return "", nil, conformanceError(repositoryMutation)
	}
	value := sha256.New()
	writeSnapshotMetadata(value, ".", identity)
	budget := &snapshotBudget{entries: 1, nameBytes: 1}
	if err := snapshotDirectory(root, "", value, budget, hook); err != nil {
		return "", nil, err
	}
	finalIdentity, err := root.Lstat(".")
	if err != nil || !sameSnapshotInfo(identity, finalIdentity, false) {
		return "", nil, conformanceError(repositoryMutation)
	}
	return hex.EncodeToString(value.Sum(nil)), identity, nil
}

func snapshotDirectory(root *os.Root, prefix string, value hash.Hash, budget *snapshotBudget, hook snapshotHook) error {
	directory, err := root.Open(".")
	if err != nil {
		return conformanceError(repositoryMutation)
	}
	entries := make([]string, 0)
	for {
		batch, readErr := directory.ReadDir(256)
		for _, entry := range batch {
			name := entry.Name()
			budget.entries++
			budget.nameBytes += int64(len(name))
			if budget.entries > maxSnapshotEntries || budget.nameBytes > maxSnapshotNameBytes {
				_ = directory.Close()
				return conformanceError(repositoryUnbounded)
			}
			entries = append(entries, name)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			_ = directory.Close()
			return conformanceError(repositoryMutation)
		}
	}
	if err := directory.Close(); err != nil {
		return conformanceError(repositoryMutation)
	}
	sort.Strings(entries)
	for _, name := range entries {
		relative := name
		if prefix != "" {
			relative = prefix + "/" + name
		}
		if len(relative)+1 > maxLineBytes {
			return conformanceError(repositoryUnbounded)
		}
		info, err := root.Lstat(name)
		if err != nil {
			return conformanceError(repositoryMutation)
		}
		if hook != nil {
			hook(relative, "after-lstat")
		}
		writeSnapshotMetadata(value, relative, info)
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := root.Readlink(name)
			if err != nil || len(target) > maxLineBytes {
				return conformanceError(repositoryMutation)
			}
			budget.nameBytes += int64(len(target))
			if budget.nameBytes > maxSnapshotNameBytes {
				return conformanceError(repositoryUnbounded)
			}
			writeDigestField(value, []byte(target))
			post, err := root.Lstat(name)
			if err != nil || !sameSnapshotInfo(info, post, false) {
				return conformanceError(repositoryMutation)
			}
		case info.IsDir():
			subroot, err := root.OpenRoot(name)
			if err != nil {
				return conformanceError(repositoryMutation)
			}
			opened, err := subroot.Lstat(".")
			if err != nil || !sameSnapshotInfo(info, opened, false) {
				_ = subroot.Close()
				return conformanceError(repositoryMutation)
			}
			if err := snapshotDirectory(subroot, relative, value, budget, hook); err != nil {
				_ = subroot.Close()
				return err
			}
			if err := subroot.Close(); err != nil {
				return conformanceError(repositoryMutation)
			}
			post, err := root.Lstat(name)
			if err != nil || !sameSnapshotInfo(info, post, false) {
				return conformanceError(repositoryMutation)
			}
		case info.Mode().IsRegular():
			if info.Size() < 0 || info.Size() > maxSnapshotBytes-budget.contentBytes {
				return conformanceError(repositoryUnbounded)
			}
			if err := hashBoundedRegular(root, name, info, value); err != nil {
				return err
			}
			budget.contentBytes += info.Size()
			if budget.contentBytes > maxSnapshotBytes {
				return conformanceError(repositoryUnbounded)
			}
		default:
			return conformanceError(repositoryUnsafe)
		}
	}
	return nil
}

func hashBoundedRegular(root *os.Root, name string, expected os.FileInfo, value hash.Hash) error {
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return conformanceError(repositoryMutation)
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !sameSnapshotInfo(expected, opened, true) {
		_ = file.Close()
		return conformanceError(repositoryMutation)
	}
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(opened.Size()))
	writeDigestField(value, size[:])
	written, copyErr := io.CopyN(value, file, opened.Size())
	var extra [1]byte
	extraCount, extraErr := file.Read(extra[:])
	postOpen, statErr := file.Stat()
	closeErr := file.Close()
	postPath, pathErr := root.Lstat(name)
	if copyErr != nil || written != opened.Size() || extraCount != 0 || !errors.Is(extraErr, io.EOF) {
		return conformanceError(repositoryMutation)
	}
	if statErr != nil || closeErr != nil || pathErr != nil {
		return conformanceError(repositoryMutation)
	}
	if !sameSnapshotInfo(opened, postOpen, true) || !sameSnapshotInfo(expected, postPath, true) {
		return conformanceError(repositoryMutation)
	}
	return nil
}

func readBoundedRegular(root *os.Root, name string, limit int, expected os.FileInfo) ([]byte, error) {
	if limit < 0 {
		return nil, conformanceError(repositoryUnbounded)
	}
	if expected == nil {
		var err error
		expected, err = root.Lstat(name)
		if err != nil || !expected.Mode().IsRegular() {
			return nil, conformanceError(repositoryUnsafe)
		}
	}
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, conformanceError(repositoryMutation)
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !sameSnapshotInfo(expected, opened, true) {
		_ = file.Close()
		return nil, conformanceError(repositoryMutation)
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	postOpen, statErr := file.Stat()
	closeErr := file.Close()
	postPath, pathErr := root.Lstat(name)
	if readErr != nil || statErr != nil || closeErr != nil || pathErr != nil {
		return nil, conformanceError(repositoryMutation)
	}
	if len(raw) > limit {
		return nil, conformanceError(repositoryUnbounded)
	}
	if int64(len(raw)) != opened.Size() {
		return nil, conformanceError(repositoryMutation)
	}
	if !sameSnapshotInfo(opened, postOpen, true) || !sameSnapshotInfo(expected, postPath, true) {
		return nil, conformanceError(repositoryMutation)
	}
	return raw, nil
}

func sameSnapshotInfo(left, right os.FileInfo, compareSize bool) bool {
	if left == nil || right == nil || left.Mode() != right.Mode() || !os.SameFile(left, right) {
		return false
	}
	return !compareSize || left.Size() == right.Size()
}

func writeSnapshotMetadata(value hash.Hash, relative string, info os.FileInfo) {
	writeDigestField(value, []byte(filepath.ToSlash(relative)))
	var mode [4]byte
	binary.BigEndian.PutUint32(mode[:], uint32(info.Mode()))
	writeDigestField(value, mode[:])
}

func writeDigestField(value hash.Hash, field []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(field)))
	_, _ = value.Write(length[:])
	_, _ = value.Write(field)
}

func parseArguments(arguments []string) ([]string, error) {
	if len(arguments) == 0 {
		return nil, nil
	}
	if arguments[0] != "--command" {
		return nil, conformanceError("command:explicit-executable-required")
	}
	return arguments[1:], nil
}

func emitEnvelope(output io.Writer, value map[string]any) {
	raw, err := canonical(value, true)
	if err == nil {
		_, _ = output.Write(raw)
	}
}

func runCLI(arguments []string, stdout, stderr io.Writer) int {
	return runCLIContext(context.Background(), arguments, stdout, stderr)
}

func runCLIContext(ctx context.Context, arguments []string, stdout, stderr io.Writer) int {
	command, argumentError := parseArguments(arguments)
	var document corpusDocument
	var err error
	if argumentError != nil {
		err = argumentError
	} else {
		document, err = loadCorpus(embeddedCorpus, true)
		if err == nil {
			err = embeddedSelfTests(document)
		}
		if err == nil && command != nil {
			err = executeCasesContext(ctx, document, command)
		}
	}
	if err != nil {
		emitEnvelope(stderr, map[string]any{
			"error":   err.Error(),
			"profile": profile,
			"stage":   stage,
			"status":  "FAIL",
		})
		return 1
	}
	mode := "self-check"
	if command != nil {
		mode = "executable"
	}
	emitEnvelope(stdout, map[string]any{
		"cases":   len(document.cases),
		"mode":    mode,
		"profile": profile,
		"stage":   stage,
		"status":  "PASS",
	})
	return 0
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.Signal(15))
	defer stop()
	os.Exit(runCLIContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
