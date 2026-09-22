// Package analyzerpython implements the unregistered Python 3.12 candidate.
package analyzerpython

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"encoding/json/jsontext"
	json "encoding/json/v2"

	"github.com/Beamfall/corvint/internal/pythongrammar"
)

const (
	Profile = "corvint-analyzer-candidate/experimental"
	Family  = "python"

	MaxRequestBytes = 1_500_000
	MaxInputs       = 128
	MaxInputBytes   = 1_048_576
	MaxFacts        = 4_096
	MaxOutputBytes  = 1_048_576
	maxFeatures     = 64
	maxJSONDepth    = 8
	maxJSONTokens   = 4_096
	maxStringBytes  = 4_096
	maxBase64Bytes  = 1_398_104
	maxPythonTokens = 65_536
	maxPythonDepth  = 256
)

type Target struct {
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	ABI          string   `json:"abi"`
	Features     []string `json:"features"`
}
type Input struct {
	Handle        string `json:"handle"`
	Family        string `json:"family"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	ContentBase64 string `json:"content_base64"`
}
type Request struct {
	Profile           string  `json:"profile"`
	Family            string  `json:"family"`
	RequestID         string  `json:"request_id"`
	ScopeID           string  `json:"scope_id"`
	CompilationUnitID string  `json:"compilation_unit_id"`
	Target            Target  `json:"target"`
	Inputs            []Input `json:"inputs"`
}
type Echo struct {
	Handle string `json:"handle"`
	SHA256 string `json:"sha256"`
}
type Fact struct {
	Kind           string `json:"kind"`
	InputHandle    string `json:"input_handle"`
	RelatedHandle  string `json:"related_handle"`
	Subject        string `json:"subject"`
	Predicate      string `json:"predicate"`
	Value          string `json:"value"`
	InstanceID     string `json:"instance_id"`
	EvidenceSHA256 string `json:"evidence_sha256"`
}
type success struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	RequestID         string `json:"request_id"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            Target `json:"target"`
	InputEchoes       []Echo `json:"input_echoes"`
	Facts             []Fact `json:"facts"`
}
type minimalRejection struct {
	Profile   string `json:"profile"`
	Family    string `json:"family"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}
type boundRejection struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	RequestID         string `json:"request_id"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            Target `json:"target"`
	InputEchoes       []Echo `json:"input_echoes"`
	Reason            string `json:"reason"`
}

type failure string

const (
	failureNoncanonical failure = "NONCANONICAL_REQUEST"
	failureIdentifier   failure = "INVALID_IDENTIFIER"
	failurePath         failure = "INVALID_PATH"
	failureDigest       failure = "DIGEST_MISMATCH"
	failureFamily       failure = "UNKNOWN_FAMILY"
	failureField        failure = "UNKNOWN_FIELD"
	failureDuplicate    failure = "DUPLICATE_VALUE"
	failureMalformed    failure = "MALFORMED_INPUT"
	failureUnsupported  failure = "UNSUPPORTED_SCHEMA"
	failureDynamic      failure = "DYNAMIC_INPUT"
	failureLimit        failure = "LIMIT_EXCEEDED"
	failureOutput       failure = "OUTPUT_LIMIT"
)

// AnalyzeFrame is the public compatibility entry point. It performs no I/O.
func AnalyzeFrame(frame []byte) []byte { return Process(frame) }

// Process accepts one canonical LF-framed Python 3.12 request. It structurally
// bounds JSON before decoding and processes decoded source one input at a time.
func Process(frame []byte) []byte {
	if len(frame) > MaxRequestBytes {
		return minimal(failureLimit)
	}
	body, reason := requestBody(frame)
	if reason != "" {
		return minimal(reason)
	}
	if reason = preflight(body); reason != "" {
		return minimal(reason)
	}
	var request Request
	if err := json.Unmarshal(body, &request, json.RejectUnknownMembers(true), json.MatchCaseInsensitiveNames(false)); err != nil {
		return unknownField(body)
	}
	canonical, err := json.Marshal(request)
	if err != nil || !bytes.Equal(canonical, body) {
		return minimal(failureNoncanonical)
	}
	if reason = validateEnvelope(request); reason != "" {
		return rejection(request, reason)
	}
	// Canonical framing was checked above. Do not keep a second request-sized
	// body while individual base64 values are decoded and discarded.
	frame, body = nil, nil
	collector, reason := newCollector(request)
	if reason != "" {
		return rejection(request, reason)
	}
	remaining := MaxInputBytes
	for index := range request.Inputs {
		item := request.Inputs[index]
		request.Inputs[index].ContentBase64 = ""
		decodedSize, decodeReason := decodedLength(item.ContentBase64)
		if decodeReason != "" {
			return rejection(request, decodeReason)
		}
		if decodedSize > remaining {
			return rejection(request, failureLimit)
		}
		content := make([]byte, decodedSize)
		if _, err = base64.StdEncoding.Strict().Decode(content, []byte(item.ContentBase64)); err != nil {
			return rejection(request, failureMalformed)
		}
		item.ContentBase64 = ""
		remaining -= len(content)
		sum := sha256.Sum256(content)
		if item.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			return rejection(request, failureDigest)
		}
		if reason = analyzeInput(item, content, collector); reason != "" {
			return rejection(request, reason)
		}
	}
	if reason = collector.finish(); reason != "" {
		return rejection(request, reason)
	}
	encoded, err := json.Marshal(collector.result)
	if err != nil || len(encoded)+1 != collector.outputBytes || len(encoded)+1 > MaxOutputBytes {
		return rejection(request, failureOutput)
	}
	return append(encoded, '\n')
}

// unknownField binds UNKNOWN_FIELD only to an otherwise safe canonical
// extension (decision 0222); any other undecodable body is a listed sentinel.
func unknownField(body []byte) []byte {
	var request Request
	known, ok := withoutExtensions(body)
	if !ok || json.Unmarshal(known, &request, json.RejectUnknownMembers(true)) != nil {
		return minimal(failureNoncanonical)
	}
	if canonical, err := json.Marshal(request); err != nil || !bytes.Equal(canonical, known) {
		return minimal(failureNoncanonical)
	}
	if reason := echoEnvelope(request); reason != "" {
		return minimal(reason)
	}
	return rejection(request, failureField)
}

// extensionFields names each schema object's fields; any other member is an extension.
var extensionFields = map[string]string{"": " profile family request_id scope_id compilation_unit_id target inputs ", "target": " os architecture abi features ", "inputs": " handle family path sha256 content_base64 "}

// withoutExtensions strips extension members that follow their object's schema
// fields in increasing name order and re-encode identically with integer-only
// numbers. It reports false for any other body.
func withoutExtensions(body []byte) ([]byte, bool) {
	decoder := jsontext.NewDecoder(bytes.NewReader(body))
	var encoded bytes.Buffer
	encoder := jsontext.NewEncoder(&encoded)
	var spans []int64
	next := func() (jsontext.Token, bool) {
		token, err := decoder.ReadToken()
		return token, err == nil && encoder.WriteToken(token) == nil
	}
	var value func(kind string) bool
	value = func(kind string) bool {
		token, ok := next()
		switch token.Kind() {
		case '0':
			digits := strings.TrimPrefix(token.String(), "-")
			return ok && (token.String() == "0" || digits != "" && digits[0] != '0' && strings.Trim(digits, "0123456789") == "")
		case '{':
			fields, schema := extensionFields[kind]
			last, extended := "", false
			for ok && decoder.PeekKind() == '"' {
				start := decoder.InputOffset()
				token, _ := next()
				name := token.String()
				if schema && !strings.Contains(name, " ") && strings.Contains(fields, " "+name+" ") {
					child := "-"
					if _, nested := extensionFields[name]; nested {
						child = name
					}
					ok = !extended && value(child)
					continue
				}
				ok = (!extended || name > last) && value("-")
				extended, last = true, name
				if schema {
					spans = append(spans, start, decoder.InputOffset())
				}
			}
		case '[':
			for ok && decoder.PeekKind() != ']' {
				ok = value(kind)
			}
		default:
			return ok
		}
		_, end := next()
		return ok && end
	}
	if !value("") || encoded.String() != string(body)+"\n" {
		return nil, false
	}
	known, at := make([]byte, 0, len(body)), int64(0)
	for index := 0; index < len(spans); index += 2 {
		known, at = append(known, body[at:spans[index]]...), spans[index+1]
	}
	return append(known, body[at:]...), true
}

func requestBody(frame []byte) ([]byte, failure) {
	if len(frame) < 2 || frame[len(frame)-1] != '\n' || bytes.IndexByte(frame[:len(frame)-1], '\n') >= 0 || !utf8.Valid(frame) {
		return nil, failureNoncanonical
	}
	return frame[:len(frame)-1], ""
}
func minimal(reason failure) []byte {
	encoded, _ := json.Marshal(minimalRejection{Profile, "unknown", "unknown", "REJECTED", string(reason)})
	return append(encoded, '\n')
}
func rejection(request Request, reason failure) []byte {
	if echoEnvelope(request) != "" {
		return minimal(reason)
	}
	echoes := make([]Echo, len(request.Inputs))
	for index, input := range request.Inputs {
		echoes[index] = Echo{input.Handle, input.SHA256}
	}
	encoded, err := json.Marshal(boundRejection{Profile, request.Family, request.RequestID, "REJECTED", request.ScopeID, request.CompilationUnitID, request.Target, echoes, string(reason)})
	if err != nil || len(encoded)+1 > MaxOutputBytes {
		return minimal(failureOutput)
	}
	return append(encoded, '\n')
}

// echoEnvelope checks every value a bound rejection echoes. Until it passes,
// a rejection is the fixed minimal sentinel.
func echoEnvelope(request Request) failure {
	if request.Family != Family {
		return failureFamily
	}
	if !identifier(request.RequestID) || !identifier(request.ScopeID) || !identifier(request.CompilationUnitID) || !targetIdentity(request.Target) {
		return failureIdentifier
	}
	seen := make(map[string]struct{}, len(request.Inputs))
	for _, item := range request.Inputs {
		if !identifier(item.Handle) {
			return failureIdentifier
		}
		if !digestString(item.SHA256) {
			return failureDigest
		}
		if _, found := seen[item.Handle]; found {
			return failureDuplicate
		}
		seen[item.Handle] = struct{}{}
	}
	return ""
}

func validateEnvelope(request Request) failure {
	if reason := echoEnvelope(request); reason != "" {
		return reason
	}
	if request.Profile != Profile {
		return failureUnsupported
	}
	if len(request.Inputs) == 0 || len(request.Inputs) > MaxInputs {
		return failureLimit
	}
	seenPaths := make(map[string]struct{}, len(request.Inputs))
	for index, item := range request.Inputs {
		if !identifier(item.Family) {
			return failureIdentifier
		}
		if !logicalPath(item.Path) {
			return failurePath
		}
		if len(item.ContentBase64) == 0 || len(item.ContentBase64) > maxBase64Bytes {
			return failureLimit
		}
		if !knownInputFamily(item.Family, item.Path) {
			return failureUnsupported
		}
		if _, found := seenPaths[item.Path]; found {
			return failureDuplicate
		}
		seenPaths[item.Path] = struct{}{}
		if index > 0 && compareInput(request.Inputs[index-1], item) >= 0 {
			return failureNoncanonical
		}
	}
	return ""
}
func targetIdentity(target Target) bool {
	if !identifier(target.OS) || !identifier(target.Architecture) || !identifier(target.ABI) || target.Features == nil || len(target.Features) > maxFeatures {
		return false
	}
	for index, feature := range target.Features {
		if !identifier(feature) || (index > 0 && target.Features[index-1] >= feature) {
			return false
		}
	}
	return true
}
func knownInputFamily(family, path string) bool {
	switch family {
	case "py.project":
		return path == "pyproject.toml"
	case "py.requirements":
		return path == "requirements.txt"
	case "py.toolchain":
		return path == ".python-version"
	case "py.source":
		return strings.HasSuffix(path, ".py")
	default:
		return false
	}
}
func decodedLength(value string) (int, failure) {
	if len(value) == 0 || len(value)%4 != 0 {
		return 0, failureMalformed
	}
	padding := 0
	for index := 0; index < len(value); index++ {
		c := value[index]
		if c == '=' {
			padding++
			if index < len(value)-2 || padding > 2 {
				return 0, failureMalformed
			}
			continue
		}
		if padding != 0 || !base64Byte(c) {
			return 0, failureMalformed
		}
	}
	decoded := len(value)/4*3 - padding
	if padding == 1 && base64Value(value[len(value)-2])&0x03 != 0 {
		return 0, failureMalformed
	}
	if padding == 2 && base64Value(value[len(value)-3])&0x0f != 0 {
		return 0, failureMalformed
	}
	if decoded < 0 || decoded > MaxInputBytes {
		return 0, failureLimit
	}
	return decoded, ""
}
func base64Byte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/'
}
func base64Value(c byte) byte {
	switch {
	case c >= 'A' && c <= 'Z':
		return c - 'A'
	case c >= 'a' && c <= 'z':
		return c - 'a' + 26
	case c >= '0' && c <= '9':
		return c - '0' + 52
	case c == '+':
		return 62
	default:
		return 63
	}
}

type collector struct {
	request     Request
	result      success
	outputBytes int
}

func newCollector(request Request) (*collector, failure) {
	echoes := make([]Echo, len(request.Inputs))
	for index, item := range request.Inputs {
		echoes[index] = Echo{item.Handle, item.SHA256}
	}
	result := success{Profile, Family, request.RequestID, "CANDIDATE", request.ScopeID, request.CompilationUnitID, request.Target, echoes, []Fact{}}
	empty, err := json.Marshal(result)
	if err != nil || len(empty)+1 > MaxOutputBytes {
		return nil, failureOutput
	}
	return &collector{request, result, len(empty) + 1}, ""
}
func (collector *collector) add(item Input, kind, subject, predicate, value string, line, column int) failure {
	if line < 1 || column < 1 {
		return failureDynamic
	}
	coordinate := item.Path + ":" + strconv.Itoa(line) + ":" + strconv.Itoa(column)
	fact := Fact{kind, item.Handle, "-", subject, predicate, value, coordinate, ""}
	if reason := validateFactFields(fact); reason != "" {
		return reason
	}
	fact.EvidenceSHA256 = evidence(collector.request, item, fact)
	if reason := validateFactFields(fact); reason != "" {
		return reason
	}
	encoded, err := json.Marshal(fact)
	if err != nil || len(encoded) > MaxOutputBytes {
		return failureOutput
	}
	if len(collector.result.Facts) == MaxFacts {
		return failureLimit
	}
	additional := len(encoded)
	if len(collector.result.Facts) > 0 {
		additional++
	}
	if additional > MaxOutputBytes-collector.outputBytes {
		return failureOutput
	}
	collector.outputBytes += additional
	collector.result.Facts = append(collector.result.Facts, fact)
	return ""
}

func validateFactFields(fact Fact) failure {
	if len(fact.InstanceID) > maxStringBytes {
		return failureLimit
	}
	for _, field := range []string{fact.Kind, fact.InputHandle, fact.RelatedHandle, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID} {
		if !factAtom(field) {
			return failureDynamic
		}
	}
	if fact.EvidenceSHA256 != "" && !digestString(fact.EvidenceSHA256) {
		return failureMalformed
	}
	return ""
}
func (collector *collector) finish() failure {
	sort.Slice(collector.result.Facts, func(left, right int) bool {
		return compareFact(collector.result.Facts[left], collector.result.Facts[right]) < 0
	})
	for index := 1; index < len(collector.result.Facts); index++ {
		if compareFact(collector.result.Facts[index-1], collector.result.Facts[index]) == 0 {
			return failureDuplicate
		}
	}
	return ""
}

func analyzeInput(item Input, content []byte, collector *collector) failure {
	switch item.Family {
	case "py.project":
		if string(content) != "requires-python = \"==3.12.0\"\n" {
			return failureUnsupported
		}
		return collector.add(item, "python.language.declaration", "python", "declares-language", "3.12.0", 1, 1)
	case "py.toolchain":
		if string(content) != "3.12.0\n" {
			return failureUnsupported
		}
		return collector.add(item, "python.toolchain.declaration", "python", "declares-toolchain", "3.12.0", 1, 1)
	case "py.requirements":
		return analyzeRequirements(item, content, collector)
	case "py.source":
		return analyzeSource(item, content, collector)
	default:
		return failureUnsupported
	}
}
func analyzeRequirements(item Input, content []byte, collector *collector) failure {
	if !utf8.Valid(content) || len(content) == 0 || content[len(content)-1] != '\n' || bytes.IndexByte(content, '\r') >= 0 {
		return failureMalformed
	}
	previous := ""
	for line, start := 1, 0; start < len(content); line++ {
		end := bytes.IndexByte(content[start:], '\n')
		if end < 0 || end > 512 {
			return failureLimit
		}
		end += start
		text := string(content[start:end])
		name, version, found := strings.Cut(text, "==")
		if !found || strings.Contains(version, "==") || !packageName(name) || !coreVersion(version) || previous != "" && previous >= text {
			return failureUnsupported
		}
		if reason := collector.add(item, "python.dependency.pinned", name, "pinned-at", version, line, 1); reason != "" {
			return reason
		}
		previous = text
		start = end + 1
	}
	return ""
}
func analyzeSource(item Input, content []byte, collector *collector) failure {
	if !utf8.Valid(content) || len(content) == 0 || content[len(content)-1] != '\n' || bytes.IndexByte(content, '\r') >= 0 || bytes.IndexByte(content, 0) >= 0 {
		return failureMalformed
	}
	if reason := preflightPythonSource(content); reason != "" {
		return reason
	}
	if pythongrammar.ContainsPython314ExceptList(content) {
		return failureUnsupported
	}
	if reason := collector.add(item, "python.source", item.Path, "parses-as", "python-3.12.0", 1, 1); reason != "" {
		return reason
	}
	return failure(pythongrammar.ParsePython312Subset(item.Path, content, boundSink{item, collector}))
}

// preflightPythonSource bounds parser work without interpreting Python. It
// counts lexical atoms, strings, and structural nesting while skipping comments
// and string bodies; the native parser remains the sole grammar authority.
func preflightPythonSource(content []byte) failure {
	tokens, depth := 0, 0
	indent := []int{0}
	lineStart, continuation := true, false
	for index := 0; index < len(content); {
		if lineStart {
			column := 0
			for index < len(content) {
				switch content[index] {
				case ' ':
					column++
					index++
				case '\t':
					column += 8 - column%8
					index++
				case '\f':
					column = 0
					index++
				default:
					goto indentationDone
				}
			}
		indentationDone:
			if index == len(content) {
				break
			}
			if content[index] != '\n' && content[index] != '#' && depth == 0 && !continuation {
				if column > indent[len(indent)-1] {
					indent = append(indent, column)
					if len(indent)-1 > maxPythonDepth {
						return failureLimit
					}
				} else {
					for len(indent) > 1 && column < indent[len(indent)-1] {
						indent = indent[:len(indent)-1]
					}
				}
			}
			lineStart, continuation = false, false
		}
		c := content[index]
		if c == ' ' || c == '\t' || c == '\n' || c == '\f' {
			if c == '\n' {
				lineStart = true
				continuation = index > 0 && content[index-1] == '\\'
			}
			index++
			continue
		}
		if c == '#' {
			for index < len(content) && content[index] != '\n' {
				index++
			}
			if index < len(content) {
				index++
				lineStart, continuation = true, false
			}
			continue
		}
		if start, found := pythongrammar.StringStart(content, index); found {
			if reason := chargePythonToken(&tokens); reason != "" {
				return reason
			}
			if pythongrammar.FString(content[index:start]) {
				var reason failure
				index, reason = preflightFString(content, start, &depth, &tokens)
				if reason != "" {
					return reason
				}
			} else {
				index = pythongrammar.SkipString(content, start)
			}
			lineStart = index > 0 && content[index-1] == '\n'
			continuation = false
			continue
		}
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_' || (c >= '0' && c <= '9') {
			for index < len(content) && ((content[index] >= 'A' && content[index] <= 'Z') || (content[index] >= 'a' && content[index] <= 'z') || (content[index] >= '0' && content[index] <= '9') || content[index] == '_') {
				index++
			}
		} else {
			if c == '(' || c == '[' || c == '{' {
				depth++
				if depth > maxPythonDepth {
					return failureLimit
				}
			}
			if (c == ')' || c == ']' || c == '}') && depth > 0 {
				depth--
			}
			index++
		}
		if reason := chargePythonToken(&tokens); reason != "" {
			return reason
		}
	}
	return ""
}

func chargePythonToken(tokens *int) failure {
	*tokens = *tokens + 1
	if *tokens > maxPythonTokens {
		return failureLimit
	}
	return ""
}

// preflightFString treats literal f-string text as opaque while bounding the
// delimiters in interpolation and format-spec expressions before parser entry.
func preflightFString(content []byte, quoteAt int, depth, tokens *int) (int, failure) {
	quote := content[quoteAt]
	triple := quoteAt+2 < len(content) && content[quoteAt+1] == quote && content[quoteAt+2] == quote
	index := quoteAt + 1
	if triple {
		index += 2
	}
	for index < len(content) {
		if triple && index+2 < len(content) && content[index] == quote && content[index+1] == quote && content[index+2] == quote {
			return index + 3, ""
		}
		if !triple && content[index] == quote {
			return index + 1, ""
		}
		if !triple && content[index] == '\n' {
			return index, failureMalformed
		}
		if content[index] == '\\' {
			index++
			continue
		}
		if content[index] == '{' {
			if index+1 < len(content) && content[index+1] == '{' {
				index += 2
				continue
			}
			next, reason := preflightFStringExpression(content, index+1, depth, tokens, triple)
			if reason != "" {
				return next, reason
			}
			index = next
			continue
		}
		if content[index] == '}' {
			if index+1 < len(content) && content[index+1] == '}' {
				index += 2
				continue
			}
			return index, failureMalformed
		}
		index++
	}
	return index, failureMalformed
}

func enterPythonDelimiter(depth *int) failure {
	*depth = *depth + 1
	if *depth > maxPythonDepth {
		return failureLimit
	}
	return ""
}

func preflightFStringExpression(content []byte, index int, depth, tokens *int, triple bool) (int, failure) {
	delimiters := make([]byte, 0, 4)
	for index < len(content) {
		if !triple && content[index] == '\n' {
			return index, failureMalformed
		}
		if strings.ContainsRune(" \t\n\f", rune(content[index])) {
			index++
			continue
		}
		if content[index] == '#' {
			for index < len(content) && content[index] != '\n' {
				index++
			}
			continue
		}
		if quote, found := pythongrammar.StringStart(content, index); found {
			if reason := chargePythonToken(tokens); reason != "" {
				return index, reason
			}
			if strings.Contains(strings.ToLower(string(content[index:quote])), "t") {
				return index, failureUnsupported
			}
			if pythongrammar.FString(content[index:quote]) {
				var reason failure
				index, reason = preflightFString(content, quote, depth, tokens)
				if reason != "" {
					return index, reason
				}
				continue
			}
			index = pythongrammar.SkipString(content, quote)
			continue
		}
		if (content[index] >= 'A' && content[index] <= 'Z') || (content[index] >= 'a' && content[index] <= 'z') || content[index] == '_' || (content[index] >= '0' && content[index] <= '9') {
			for index < len(content) && ((content[index] >= 'A' && content[index] <= 'Z') || (content[index] >= 'a' && content[index] <= 'z') || (content[index] >= '0' && content[index] <= '9') || content[index] == '_') {
				index++
			}
			if reason := chargePythonToken(tokens); reason != "" {
				return index, reason
			}
			continue
		}
		switch content[index] {
		case '(', '[', '{':
			delimiters = append(delimiters, content[index])
			if reason := enterPythonDelimiter(depth); reason != "" {
				return index, reason
			}
		case ')', ']':
			if len(delimiters) == 0 || !pythongrammar.DelimiterPair(delimiters[len(delimiters)-1], content[index]) {
				return index, failureMalformed
			}
			delimiters = delimiters[:len(delimiters)-1]
			*depth = *depth - 1
		case '}':
			if len(delimiters) == 0 {
				return index + 1, ""
			}
			if !pythongrammar.DelimiterPair(delimiters[len(delimiters)-1], content[index]) {
				return index, failureMalformed
			}
			delimiters = delimiters[:len(delimiters)-1]
			*depth = *depth - 1
		case ':':
			if len(delimiters) == 0 {
				return preflightFStringFormat(content, index+1, depth, tokens, triple)
			}
		}
		if reason := chargePythonToken(tokens); reason != "" {
			return index, reason
		}
		index++
	}
	return index, failureMalformed
}

func preflightFStringFormat(content []byte, index int, depth, tokens *int, triple bool) (int, failure) {
	for index < len(content) {
		if !triple && content[index] == '\n' {
			return index, failureMalformed
		}
		if content[index] == '\\' {
			index++
			continue
		}
		if content[index] == '{' {
			if index+1 < len(content) && content[index+1] == '{' {
				index += 2
				continue
			}
			next, reason := preflightFStringExpression(content, index+1, depth, tokens, triple)
			if reason != "" {
				return next, reason
			}
			index = next
			continue
		}
		if content[index] == '}' {
			if index+1 < len(content) && content[index+1] == '}' {
				index += 2
				continue
			}
			return index + 1, ""
		}
		index++
	}
	return index, failureMalformed
}

// containsPython314ExceptList closes PEP 758's unparenthesized exception
// lists. The native parser intentionally tracks newer syntax, while this
// candidate's source profile is exactly Python 3.12. Comments and strings are
// skipped so source text cannot accidentally trigger the closure.

func evidence(request Request, item Input, fact Fact) string {
	targetJSON, _ := json.Marshal(request.Target)
	fields := []string{Family, request.RequestID, request.ScopeID, request.CompilationUnitID, string(targetJSON), item.Handle, item.SHA256, fact.RelatedHandle, "-", fact.Kind, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID}
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(fields)))
	_, _ = hasher.Write(length[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		_, _ = hasher.Write(length[:])
		_, _ = hasher.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
func compareInput(left, right Input) int {
	return compareFields(left.Handle, right.Handle, left.Family, right.Family, left.Path, right.Path, left.SHA256, right.SHA256)
}
func compareFact(left, right Fact) int {
	return compareFields(left.Kind, right.Kind, left.InputHandle, right.InputHandle, left.RelatedHandle, right.RelatedHandle, left.Subject, right.Subject, left.Predicate, right.Predicate, left.Value, right.Value, left.InstanceID, right.InstanceID, left.EvidenceSHA256, right.EvidenceSHA256)
}
func compareFields(values ...string) int {
	for index := 0; index < len(values); index += 2 {
		if values[index] < values[index+1] {
			return -1
		}
		if values[index] > values[index+1] {
			return 1
		}
	}
	return 0
}
func identifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index := range value {
		c := value[index]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || index > 0 && strings.ContainsRune("._:@+~-", rune(c))) {
			return false
		}
	}
	return true
}
func logicalPath(value string) bool {
	if len(value) == 0 || len(value) > 4096 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if len(part) == 0 || len(part) > 128 || part == "." || part == ".." {
			return false
		}
		for _, c := range part {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || strings.ContainsRune("._@+~-", c)) {
				return false
			}
		}
	}
	return true
}
func digestString(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[len("sha256:"):] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
func factAtom(value string) bool {
	if len(value) == 0 || len(value) > maxStringBytes {
		return false
	}
	for _, c := range value {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}

func packageName(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index := range value {
		c := value[index]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || index > 0 && strings.ContainsRune("._-", rune(c))) {
			return false
		}
	}
	return true
}
func coreVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// jsonScanner charges depth, tokens, and string bytes before json/v2 retains fields.
type jsonScanner struct {
	body                 []byte
	index, depth, tokens int
}

func preflight(body []byte) failure {
	if len(body) == 0 || !utf8.Valid(body) {
		return failureNoncanonical
	}
	scanner := jsonScanner{body: body}
	if reason := scanner.value(maxStringBytes, ""); reason != "" || scanner.index != len(body) {
		if reason != "" {
			return reason
		}
		return failureNoncanonical
	}
	return ""
}
func (scanner *jsonScanner) value(stringLimit int, field string) failure {
	if scanner.tokens == maxJSONTokens {
		return failureLimit
	}
	scanner.tokens++
	if scanner.index >= len(scanner.body) {
		return failureNoncanonical
	}
	switch scanner.body[scanner.index] {
	case '{':
		return scanner.compound('{', '}', field)
	case '[':
		return scanner.compound('[', ']', field)
	case '"':
		_, reason := scanner.string(stringLimit)
		return reason
	case 't':
		return scanner.literal("true")
	case 'f':
		return scanner.literal("false")
	case 'n':
		return scanner.literal("null")
	default:
		return scanner.number()
	}
}
func (scanner *jsonScanner) compound(open, close byte, field string) failure {
	scanner.index++
	scanner.depth++
	if scanner.depth > maxJSONDepth {
		return failureLimit
	}
	if scanner.index >= len(scanner.body) {
		return failureNoncanonical
	}
	if scanner.body[scanner.index] == close {
		scanner.index++
		scanner.depth--
		return ""
	}
	elements := 0
	for {
		if open == '[' {
			elements++
			if field == "inputs" && elements > MaxInputs || field == "features" && elements > maxFeatures {
				return failureLimit
			}
		}
		stringLimit := maxStringBytes
		member := ""
		if open == '{' {
			if scanner.tokens == maxJSONTokens {
				return failureLimit
			}
			scanner.tokens++
			key, reason := scanner.string(maxStringBytes)
			if reason != "" {
				return reason
			}
			if scanner.index >= len(scanner.body) || scanner.body[scanner.index] != ':' {
				return failureNoncanonical
			}
			scanner.index++
			switch canonicalJSONKey(key) {
			case jsonKeyContentBase64:
				stringLimit = maxBase64Bytes
				member = "content_base64"
			case jsonKeyInputs:
				member = "inputs"
			case jsonKeyFeatures:
				member = "features"
			}
		}
		if reason := scanner.value(stringLimit, member); reason != "" {
			return reason
		}
		if scanner.index >= len(scanner.body) {
			return failureNoncanonical
		}
		if scanner.body[scanner.index] == close {
			scanner.index++
			scanner.depth--
			return ""
		}
		if scanner.body[scanner.index] != ',' {
			return failureNoncanonical
		}
		scanner.index++
	}
}

type jsonKey uint8

const (
	jsonKeyUnknown jsonKey = iota
	jsonKeyContentBase64
	jsonKeyInputs
	jsonKeyFeatures
)

// canonicalJSONKey decodes the bounded JSON key before applying prospective
// member bounds, without retaining an attacker-controlled decoded string.
func canonicalJSONKey(value []byte) jsonKey {
	if canonicalJSONKeyEquals(value, "content_base64") {
		return jsonKeyContentBase64
	}
	if canonicalJSONKeyEquals(value, "inputs") {
		return jsonKeyInputs
	}
	if canonicalJSONKeyEquals(value, "features") {
		return jsonKeyFeatures
	}
	return jsonKeyUnknown
}

func canonicalJSONKeyEquals(value []byte, want string) bool {
	matched := 0
	for index := 0; index < len(value); {
		decoded := rune(value[index])
		index++
		if decoded == '\\' {
			if index >= len(value) {
				return false
			}
			escape := value[index]
			index++
			switch escape {
			case '"', '\\', '/':
				decoded = rune(escape)
			case 'b':
				decoded = '\b'
			case 'f':
				decoded = '\f'
			case 'n':
				decoded = '\n'
			case 'r':
				decoded = '\r'
			case 't':
				decoded = '\t'
			case 'u':
				if index+4 > len(value) {
					return false
				}
				decoded = jsonHexRune(value[index : index+4])
				index += 4
			default:
				return false
			}
		}
		if decoded > 0x7f || matched >= len(want) || byte(decoded) != want[matched] {
			return false
		}
		matched++
	}
	return matched == len(want)
}

func jsonHexRune(value []byte) rune {
	decoded := rune(0)
	for _, digit := range value {
		decoded <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			decoded += rune(digit - '0')
		case digit >= 'a' && digit <= 'f':
			decoded += rune(digit - 'a' + 10)
		default:
			decoded += rune(digit - 'A' + 10)
		}
	}
	return decoded
}
func (scanner *jsonScanner) string(limit int) ([]byte, failure) {
	if scanner.index >= len(scanner.body) || scanner.body[scanner.index] != '"' {
		return nil, failureNoncanonical
	}
	start := scanner.index
	scanner.index++
	for scanner.index < len(scanner.body) {
		c := scanner.body[scanner.index]
		scanner.index++
		if c == '"' {
			if scanner.index-start-2 > limit {
				return nil, failureLimit
			}
			return scanner.body[start+1 : scanner.index-1], ""
		}
		if c < 0x20 {
			return nil, failureNoncanonical
		}
		if c != '\\' {
			continue
		}
		if scanner.index >= len(scanner.body) {
			return nil, failureNoncanonical
		}
		escape := scanner.body[scanner.index]
		scanner.index++
		if escape == 'u' {
			if scanner.index+4 > len(scanner.body) {
				return nil, failureNoncanonical
			}
			for _, hex := range scanner.body[scanner.index : scanner.index+4] {
				if !((hex >= '0' && hex <= '9') || (hex >= 'a' && hex <= 'f') || (hex >= 'A' && hex <= 'F')) {
					return nil, failureNoncanonical
				}
			}
			scanner.index += 4
			continue
		}
		if !strings.ContainsRune("\\\"/bfnrt", rune(escape)) {
			return nil, failureNoncanonical
		}
	}
	return nil, failureNoncanonical
}
func (scanner *jsonScanner) literal(value string) failure {
	if !bytes.HasPrefix(scanner.body[scanner.index:], []byte(value)) {
		return failureNoncanonical
	}
	scanner.index += len(value)
	return ""
}
func (scanner *jsonScanner) number() failure {
	start := scanner.index
	for scanner.index < len(scanner.body) && strings.ContainsRune("-+0123456789.eE", rune(scanner.body[scanner.index])) {
		scanner.index++
	}
	if start == scanner.index {
		return failureNoncanonical
	}
	return ""
}

// boundSink adapts the neutral grammar sink to the analyzer's collector, which
// needs the input's identity for evidence. The grammar names only the subject,
// so the input travels here rather than through the parser.
type boundSink struct {
	item      Input
	collector factCollector
}

func (sink boundSink) Add(kind, subject, predicate, value string, line, column int) pythongrammar.Failure {
	return pythongrammar.Failure(sink.collector.add(sink.item, kind, subject, predicate, value, line, column))
}

type factCollector interface {
	add(Input, string, string, string, string, int, int) failure
}
