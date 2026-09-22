// Package analyzerhtmlcss is an unregistered experimental HTML/CSS fact extractor.
// It reads only the caller's framed request; it has no filesystem, process, or network use.
package analyzerhtmlcss

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	Profile             = "corvint-analyzer-candidate/experimental"
	Family              = "html-css"
	HTMLInputProfile    = "html.document/closed-v1"
	CSSInputProfile     = "css.stylesheet/closed-v1"
	SourceCoordinate    = "beamfall-core@da38c59eb30b2121cbac37b912485b30b2e54841"
	WebSourceCoordinate = "beamfall-web@b924fe0ae0d36af08b2a1d50be9383381f809cc0"
	ToolchainCoordinate = "go1.27.0"

	maxRequestBytes       = 1500000
	maxContentBytes       = 1048576
	maxBase64Bytes        = 1398104
	maxJSONDepth          = 8
	maxJSONTokens         = 4096
	maxJSONOrdinaryString = 4096
	maxFeatures           = 64
	maxInputs             = 128
	maxFactsPerInput      = 4096
	maxFacts              = 4096
	maxOutputBytes        = 1048576
)

type target struct {
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	ABI          string   `json:"abi"`
	Features     []string `json:"features"`
}
type input struct {
	Handle        string `json:"handle"`
	Family        string `json:"family"`
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	ContentBase64 string `json:"content_base64"`
}
type request struct {
	Profile           string  `json:"profile"`
	Family            string  `json:"family"`
	RequestID         string  `json:"request_id"`
	ScopeID           string  `json:"scope_id"`
	CompilationUnitID string  `json:"compilation_unit_id"`
	Target            target  `json:"target"`
	Inputs            []input `json:"inputs"`
}
type inputEcho struct {
	Handle string `json:"handle"`
	Family string `json:"family"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type fact struct {
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
	Profile           string      `json:"profile"`
	Family            string      `json:"family"`
	RequestID         string      `json:"request_id"`
	Status            string      `json:"status"`
	ScopeID           string      `json:"scope_id"`
	CompilationUnitID string      `json:"compilation_unit_id"`
	Target            target      `json:"target"`
	InputEchoes       []inputEcho `json:"input_echoes"`
	Facts             []fact      `json:"facts"`
}
type rejected struct {
	Profile           string      `json:"profile"`
	Family            string      `json:"family"`
	RequestID         string      `json:"request_id"`
	Status            string      `json:"status"`
	ScopeID           string      `json:"scope_id"`
	CompilationUnitID string      `json:"compilation_unit_id"`
	Target            target      `json:"target"`
	InputEchoes       []inputEcho `json:"input_echoes"`
	Reason            string      `json:"reason"`
}
type sentinelRejected struct {
	Profile   string `json:"profile"`
	Family    string `json:"family"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}

// Analyze returns exactly one canonical LF-framed success or rejection. It never returns source
// text in a rejection, so callers may safely write it to a public diagnostic stream.
func Analyze(frame []byte) []byte {
	if len(frame) > maxRequestBytes {
		return sentinel("LIMIT_EXCEEDED")
	}
	var raw request
	scan, reason := parseJSON(frame, &raw)
	if reason != "" {
		return sentinel(reason)
	}
	if !canonicalRequest(frame, raw) {
		return sentinel("NONCANONICAL_REQUEST")
	}
	if reason := safeEchoEnvelope(raw); reason != "" {
		return sentinel(reason)
	}
	if raw.Profile != Profile {
		return envelope(raw, "NONCANONICAL_REQUEST")
	}
	if scan.unknownField {
		return envelope(raw, "UNKNOWN_FIELD")
	}
	if raw.Family != Family {
		return envelope(raw, "UNSUPPORTED_SCHEMA")
	}
	decoded, reason := validateAndDecode(raw)
	if reason != "" {
		return envelope(raw, reason)
	}
	collector, reason := newFactCollectorWithDigest(raw, requestDigest(frame))
	if reason != "" {
		return envelope(raw, reason)
	}
	for _, in := range decoded {
		collector.beginInput()
		if in.record.Family == "html.document" {
			reason = parseHTML(in.record, in.content, collector.add)
		} else {
			reason = parseCSS(in.record, in.content, collector.add)
		}
		if reason != "" {
			return envelope(raw, reason)
		}
	}
	sortFacts(collector.facts)
	for i := 1; i < len(collector.facts); i++ {
		if compareFact(collector.facts[i-1], collector.facts[i]) == 0 {
			return envelope(raw, "DUPLICATE_VALUE")
		}
	}
	out, err := json.Marshal(success{Profile, Family, raw.RequestID, "CANDIDATE", raw.ScopeID, raw.CompilationUnitID, raw.Target, echoes(raw), collector.facts})
	if err != nil || len(out)+1 != collector.prospectiveOutput || len(out)+1 > maxOutputBytes {
		return envelope(raw, "OUTPUT_LIMIT")
	}
	return append(out, '\n')
}

type jsonScanResult struct{ unknownField bool }

// parseJSON first walks the request without retaining arbitrary JSON values. It gives each string,
// token, depth, feature, input, encoded-content, and aggregate decoder allocation a prospective
// bound before encoding/json materializes the already-bounded canonical envelope.
func parseJSON(frame []byte, out *request) (jsonScanResult, string) {
	if len(frame) < 2 || frame[len(frame)-1] != '\n' || frame[len(frame)-2] == '\n' || !utf8.Valid(frame[:len(frame)-1]) {
		return jsonScanResult{}, "NONCANONICAL_REQUEST"
	}
	scanner := jsonScanner{data: frame[:len(frame)-1]}
	result, ok := scanner.scanRequest()
	if scanner.limitExceeded {
		return jsonScanResult{}, "LIMIT_EXCEEDED"
	}
	if !ok || scanner.nonCanonical || scanner.pos != len(scanner.data) {
		return jsonScanResult{}, "NONCANONICAL_REQUEST"
	}
	if err := json.Unmarshal(scanner.data, out); err != nil {
		return jsonScanResult{}, "NONCANONICAL_REQUEST"
	}
	return result, ""
}

type jsonScanner struct {
	data          []byte
	pos           int
	tokens        int
	limitExceeded bool
	nonCanonical  bool
}

func (s *jsonScanner) scanRequest() (jsonScanResult, bool) {
	var result jsonScanResult
	if !s.beginObject(1) {
		return result, false
	}
	seen := [7]bool{}
	nextKnown := 0
	first := true
	for {
		done, ok := s.objectNext(&first)
		if !ok {
			return result, false
		}
		if done {
			return result, nextKnown == len(seen)
		}
		key, ok := s.key()
		if !ok || !s.consume(':') {
			return result, false
		}
		index := -1
		switch key {
		case "profile":
			index, ok = 0, s.stringValue(maxJSONOrdinaryString)
		case "family":
			index, ok = 1, s.stringValue(maxJSONOrdinaryString)
		case "request_id":
			index, ok = 2, s.stringValue(maxJSONOrdinaryString)
		case "scope_id":
			index, ok = 3, s.stringValue(maxJSONOrdinaryString)
		case "compilation_unit_id":
			index, ok = 4, s.stringValue(maxJSONOrdinaryString)
		case "target":
			index, ok = 5, s.scanTarget(&result)
		case "inputs":
			index, ok = 6, s.scanInputs(&result)
		default:
			result.unknownField = true
			ok = s.value(2, maxJSONOrdinaryString)
		}
		if !ok || index >= 0 && (seen[index] || index != nextKnown) {
			return result, false
		}
		if index >= 0 {
			seen[index] = true
			nextKnown++
		} else if nextKnown != len(seen) {
			return result, false
		}
	}
}

func (s *jsonScanner) scanTarget(result *jsonScanResult) bool {
	if !s.beginObject(2) {
		return false
	}
	seen := [4]bool{}
	nextKnown := 0
	first := true
	for {
		done, ok := s.objectNext(&first)
		if !ok {
			return false
		}
		if done {
			return nextKnown == len(seen)
		}
		key, ok := s.key()
		if !ok || !s.consume(':') {
			return false
		}
		index := -1
		switch key {
		case "os":
			index, ok = 0, s.stringValue(maxJSONOrdinaryString)
		case "architecture":
			index, ok = 1, s.stringValue(maxJSONOrdinaryString)
		case "abi":
			index, ok = 2, s.stringValue(maxJSONOrdinaryString)
		case "features":
			index, ok = 3, s.scanStringArray(3, maxFeatures)
		default:
			result.unknownField = true
			ok = s.value(3, maxJSONOrdinaryString)
		}
		if !ok || index >= 0 && (seen[index] || index != nextKnown) {
			return false
		}
		if index >= 0 {
			seen[index] = true
			nextKnown++
		} else if nextKnown != len(seen) {
			return false
		}
	}
}

func (s *jsonScanner) scanInputs(result *jsonScanResult) bool {
	if s.consumeLiteral("null") {
		return true
	}
	if !s.beginArray(2) {
		return false
	}
	count, first := 0, true
	for {
		done, ok := s.arrayNext(&first)
		if !ok {
			return false
		}
		if done {
			return true
		}
		if count == maxInputs {
			s.limitExceeded = true
			return false
		}
		if !s.scanInput(result) {
			return false
		}
		count++
	}
}

func (s *jsonScanner) scanInput(result *jsonScanResult) bool {
	if !s.beginObject(3) {
		return false
	}
	seen := [5]bool{}
	nextKnown := 0
	first := true
	for {
		done, ok := s.objectNext(&first)
		if !ok {
			return false
		}
		if done {
			return nextKnown == len(seen)
		}
		key, ok := s.key()
		if !ok || !s.consume(':') {
			return false
		}
		index := -1
		switch key {
		case "handle":
			index, ok = 0, s.stringValue(maxJSONOrdinaryString)
		case "family":
			index, ok = 1, s.stringValue(maxJSONOrdinaryString)
		case "path":
			index, ok = 2, s.stringValue(maxJSONOrdinaryString)
		case "sha256":
			index, ok = 3, s.stringValue(maxJSONOrdinaryString)
		case "content_base64":
			index, ok = 4, s.stringValue(maxBase64Bytes)
		default:
			result.unknownField = true
			ok = s.value(4, maxJSONOrdinaryString)
		}
		if !ok || index >= 0 && (seen[index] || index != nextKnown) {
			return false
		}
		if index >= 0 {
			seen[index] = true
			nextKnown++
		} else if nextKnown != len(seen) {
			return false
		}
	}
}

func (s *jsonScanner) scanStringArray(depth, maximum int) bool {
	if s.consumeLiteral("null") {
		return true
	}
	if !s.beginArray(depth) {
		return false
	}
	count, first := 0, true
	for {
		done, ok := s.arrayNext(&first)
		if !ok {
			return false
		}
		if done {
			return true
		}
		if count == maximum {
			s.limitExceeded = true
			return false
		}
		if !s.stringValue(maxJSONOrdinaryString) {
			return false
		}
		count++
	}
}

func (s *jsonScanner) value(depth, stringLimit int) bool {
	s.skipSpace()
	if s.pos == len(s.data) {
		return false
	}
	switch s.data[s.pos] {
	case '"':
		return s.stringValue(stringLimit)
	case '{':
		if !s.beginObject(depth) {
			return false
		}
		first := true
		for {
			done, ok := s.objectNext(&first)
			if !ok {
				return false
			}
			if done {
				return true
			}
			if _, ok := s.key(); !ok || !s.consume(':') || !s.value(depth+1, stringLimit) {
				return false
			}
		}
	case '[':
		if !s.beginArray(depth) {
			return false
		}
		first := true
		for {
			done, ok := s.arrayNext(&first)
			if !ok {
				return false
			}
			if done {
				return true
			}
			if !s.value(depth+1, stringLimit) {
				return false
			}
		}
	default:
		return s.scalar()
	}
}

func (s *jsonScanner) beginObject(depth int) bool {
	if depth > maxJSONDepth {
		s.limitExceeded = true
		return false
	}
	return s.consume('{')
}
func (s *jsonScanner) beginArray(depth int) bool {
	if depth > maxJSONDepth {
		s.limitExceeded = true
		return false
	}
	return s.consume('[')
}
func (s *jsonScanner) objectNext(first *bool) (bool, bool) {
	s.skipSpace()
	if s.consume('}') {
		return true, true
	}
	if !*first && !s.consume(',') {
		return false, false
	}
	*first = false
	return false, true
}
func (s *jsonScanner) arrayNext(first *bool) (bool, bool) {
	s.skipSpace()
	if s.consume(']') {
		return true, true
	}
	if !*first && !s.consume(',') {
		return false, false
	}
	*first = false
	return false, true
}
func (s *jsonScanner) key() (string, bool) {
	s.skipSpace()
	start := s.pos
	if !s.stringValue(maxJSONOrdinaryString) {
		return "", false
	}
	raw := s.data[start+1 : s.pos-1]
	if bytes.IndexByte(raw, '\\') >= 0 {
		return "", false
	}
	return string(raw), true
}
func (s *jsonScanner) stringValue(limit int) bool {
	s.skipSpace()
	if s.pos == len(s.data) || s.data[s.pos] != '"' || !s.token() {
		return false
	}
	s.pos++
	length := 0
	for s.pos < len(s.data) {
		c := s.data[s.pos]
		s.pos++
		if c == '"' {
			return true
		}
		if c < 0x20 || length == limit {
			if length == limit {
				s.limitExceeded = true
			}
			return false
		}
		length++
		if c != '\\' {
			continue
		}
		if s.pos == len(s.data) {
			return false
		}
		escaped := s.data[s.pos]
		s.pos++
		if length == limit {
			s.limitExceeded = true
			return false
		}
		length++
		switch escaped {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		case 'u':
			for range 4 {
				if s.pos == len(s.data) || !hexDigit(s.data[s.pos]) || length == limit {
					if length == limit {
						s.limitExceeded = true
					}
					return false
				}
				s.pos++
				length++
			}
		default:
			return false
		}
	}
	return false
}
func (s *jsonScanner) scalar() bool {
	s.skipSpace()
	if s.consumeLiteral("true") || s.consumeLiteral("false") || s.consumeLiteral("null") {
		return true
	}
	start := s.pos
	if s.pos < len(s.data) && s.data[s.pos] == '-' {
		s.pos++
	}
	if s.pos == len(s.data) || s.data[s.pos] < '0' || s.data[s.pos] > '9' {
		return false
	}
	for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
		s.pos++
	}
	if s.pos < len(s.data) && s.data[s.pos] == '.' {
		s.pos++
		fraction := s.pos
		for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
			s.pos++
		}
		if fraction == s.pos {
			return false
		}
	}
	if s.pos < len(s.data) && (s.data[s.pos] == 'e' || s.data[s.pos] == 'E') {
		s.pos++
		if s.pos < len(s.data) && (s.data[s.pos] == '+' || s.data[s.pos] == '-') {
			s.pos++
		}
		exponent := s.pos
		for s.pos < len(s.data) && s.data[s.pos] >= '0' && s.data[s.pos] <= '9' {
			s.pos++
		}
		if exponent == s.pos {
			return false
		}
	}
	if s.pos-start > maxJSONOrdinaryString {
		s.limitExceeded = true
		return false
	}
	return s.pos > start && s.token()
}
func (s *jsonScanner) consumeLiteral(literal string) bool {
	s.skipSpace()
	if len(s.data)-s.pos < len(literal) || !bytes.Equal(s.data[s.pos:s.pos+len(literal)], []byte(literal)) || !s.token() {
		return false
	}
	s.pos += len(literal)
	return true
}
func (s *jsonScanner) consume(want byte) bool {
	s.skipSpace()
	if s.pos == len(s.data) || s.data[s.pos] != want || !s.token() {
		return false
	}
	s.pos++
	return true
}
func (s *jsonScanner) token() bool {
	if s.tokens >= maxJSONTokens {
		s.limitExceeded = true
		return false
	}
	s.tokens++
	return true
}
func (s *jsonScanner) skipSpace() {
	for s.pos < len(s.data) && (s.data[s.pos] == ' ' || s.data[s.pos] == '\t' || s.data[s.pos] == '\r' || s.data[s.pos] == '\n') {
		s.nonCanonical = true
		s.pos++
	}
}

// canonicalRequest re-encodes the complete envelope, including extensions the typed request does
// not retain. This makes duplicate extension keys and noncanonical extension values fail before
// they can select the safe UNKNOWN_FIELD envelope.
func canonicalRequest(frame []byte, r request) bool {
	raw := frame[:len(frame)-1]
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return false
	}
	targetRaw, ok := root["target"]
	if !ok {
		return false
	}
	targetJSON, ok := canonicalTarget(r.Target, targetRaw)
	if !ok {
		return false
	}
	inputsRaw, ok := root["inputs"]
	if !ok {
		return false
	}
	inputsJSON, ok := canonicalInputs(r.Inputs, inputsRaw)
	if !ok {
		return false
	}
	encoded, ok := canonicalObject(root, []canonicalField{
		{"profile", mustMarshal(r.Profile)},
		{"family", mustMarshal(r.Family)},
		{"request_id", mustMarshal(r.RequestID)},
		{"scope_id", mustMarshal(r.ScopeID)},
		{"compilation_unit_id", mustMarshal(r.CompilationUnitID)},
		{"target", targetJSON},
		{"inputs", inputsJSON},
	})
	return ok && bytes.Equal(encoded, raw)
}

type canonicalField struct {
	name  string
	value []byte
}

func canonicalTarget(value target, raw json.RawMessage) ([]byte, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false
	}
	return canonicalObject(fields, []canonicalField{
		{"os", mustMarshal(value.OS)},
		{"architecture", mustMarshal(value.Architecture)},
		{"abi", mustMarshal(value.ABI)},
		{"features", mustMarshal(value.Features)},
	})
}

func canonicalInputs(values []input, raw json.RawMessage) ([]byte, bool) {
	if values == nil {
		return []byte("null"), bytes.Equal(raw, []byte("null"))
	}
	var rawValues []json.RawMessage
	if err := json.Unmarshal(raw, &rawValues); err != nil || len(rawValues) != len(values) {
		return nil, false
	}
	out := make([]byte, 0, len(raw))
	out = append(out, '[')
	for index, value := range values {
		encoded, ok := canonicalInput(value, rawValues[index])
		if !ok {
			return nil, false
		}
		if index > 0 {
			out = append(out, ',')
		}
		out = append(out, encoded...)
	}
	return append(out, ']'), true
}

func canonicalInput(value input, raw json.RawMessage) ([]byte, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false
	}
	return canonicalObject(fields, []canonicalField{
		{"handle", mustMarshal(value.Handle)},
		{"family", mustMarshal(value.Family)},
		{"path", mustMarshal(value.Path)},
		{"sha256", mustMarshal(value.SHA256)},
		{"content_base64", mustMarshal(value.ContentBase64)},
	})
}

func canonicalObject(raw map[string]json.RawMessage, fields []canonicalField) ([]byte, bool) {
	extras := make(map[string]json.RawMessage, len(raw))
	for name, value := range raw {
		extras[name] = value
	}
	out := make([]byte, 0)
	out = append(out, '{')
	for index, field := range fields {
		if _, ok := extras[field.name]; !ok {
			return nil, false
		}
		delete(extras, field.name)
		if index > 0 {
			out = append(out, ',')
		}
		out = append(out, '"')
		out = append(out, field.name...)
		out = append(out, '"', ':')
		out = append(out, field.value...)
	}
	names := make([]string, 0, len(extras))
	for name := range extras {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value, ok := canonicalValue(extras[name])
		if !ok {
			return nil, false
		}
		key := mustMarshalCanonicalString(name)
		out = append(out, ',')
		out = append(out, key...)
		out = append(out, ':')
		out = append(out, value...)
	}
	return append(out, '}'), true
}

func canonicalValue(raw json.RawMessage) ([]byte, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil || !canonicalUnknownValue(value) {
		return nil, false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, false
	}
	encoded, err := marshalCanonical(value)
	return encoded, err == nil
}

// mustMarshal encodes known request fields with the same shortest-escape
// encoder as unknown values; encoding/json would HTML-escape <, >, and &.
func mustMarshal(value any) []byte {
	switch typed := value.(type) {
	case string:
		return appendCanonicalJSONString(make([]byte, 0, len(typed)+2), typed)
	case []string:
		if typed == nil {
			return []byte("null")
		}
		out := append(make([]byte, 0, 64), '[')
		for index, item := range typed {
			if index > 0 {
				out = append(out, ',')
			}
			out = appendCanonicalJSONString(out, item)
		}
		return append(out, ']')
	}
	panic("unsupported known-field type")
}

func mustMarshalCanonicalString(value string) []byte {
	encoded, err := marshalCanonical(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func marshalCanonical(value any) ([]byte, error) {
	return appendCanonicalJSON(make([]byte, 0, 64), value)
}

func appendCanonicalJSON(out []byte, value any) ([]byte, error) {
	switch typed := value.(type) {
	case nil:
		return append(out, "null"...), nil
	case bool:
		if typed {
			return append(out, "true"...), nil
		}
		return append(out, "false"...), nil
	case string:
		return appendCanonicalJSONString(out, typed), nil
	case json.Number:
		if !canonicalInteger(string(typed)) {
			return nil, errors.New("noncanonical number")
		}
		return append(out, string(typed)...), nil
	case []any:
		out = append(out, '[')
		for index, item := range typed {
			if index > 0 {
				out = append(out, ',')
			}
			var err error
			out, err = appendCanonicalJSON(out, item)
			if err != nil {
				return nil, err
			}
		}
		return append(out, ']'), nil
	case map[string]any:
		names := make([]string, 0, len(typed))
		for name := range typed {
			names = append(names, name)
		}
		sort.Strings(names)
		out = append(out, '{')
		for index, name := range names {
			if index > 0 {
				out = append(out, ',')
			}
			out = appendCanonicalJSONString(out, name)
			out = append(out, ':')
			var err error
			out, err = appendCanonicalJSON(out, typed[name])
			if err != nil {
				return nil, err
			}
		}
		return append(out, '}'), nil
	default:
		return nil, errors.New("unsupported JSON value")
	}
}

func appendCanonicalJSONString(out []byte, value string) []byte {
	out = append(out, '"')
	for _, runeValue := range value {
		switch runeValue {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\b':
			out = append(out, '\\', 'b')
		case '\f':
			out = append(out, '\\', 'f')
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			if runeValue < 0x20 {
				out = append(out, '\\', 'u', '0', '0', hexCharacter(byte(runeValue>>4)), hexCharacter(byte(runeValue)))
				continue
			}
			out = utf8.AppendRune(out, runeValue)
		}
	}
	return append(out, '"')
}

func hexCharacter(value byte) byte {
	value &= 0x0f
	if value < 10 {
		return '0' + value
	}
	return 'a' + value - 10
}

func canonicalUnknownValue(value any) bool {
	switch typed := value.(type) {
	case json.Number:
		return canonicalInteger(string(typed))
	case []any:
		for _, item := range typed {
			if !canonicalUnknownValue(item) {
				return false
			}
		}
	case map[string]any:
		for _, item := range typed {
			if !canonicalUnknownValue(item) {
				return false
			}
		}
	}
	return true
}

func canonicalInteger(value string) bool {
	if value == "0" {
		return true
	}
	if strings.HasPrefix(value, "-") {
		value = value[1:]
	}
	if len(value) == 0 || value[0] < '1' || value[0] > '9' {
		return false
	}
	for i := 1; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func requestDigest(frame []byte) string {
	digest := sha256.Sum256(frame)
	return "sha256:" + hex.EncodeToString(digest[:])
}

type decodedInput struct {
	record  input
	content []byte
}

// safeEchoEnvelope proves every echoed field before it can be reflected in a rejection.
func safeEchoEnvelope(r request) string {
	if !closedFamily(r.Family) {
		return "UNKNOWN_FAMILY"
	}
	if !identifier(r.RequestID) || !identifier(r.ScopeID) || !identifier(r.CompilationUnitID) || !identifier(r.Target.OS) || !identifier(r.Target.Architecture) || !identifier(r.Target.ABI) {
		return "INVALID_IDENTIFIER"
	}
	if r.Target.Features == nil || r.Inputs == nil {
		return "NONCANONICAL_REQUEST"
	}
	if len(r.Target.Features) > maxFeatures || len(r.Inputs) == 0 || len(r.Inputs) > maxInputs {
		return "LIMIT_EXCEEDED"
	}
	for i, feature := range r.Target.Features {
		if !identifier(feature) {
			return "INVALID_IDENTIFIER"
		}
		if i > 0 && r.Target.Features[i-1] >= feature {
			return "DUPLICATE_VALUE"
		}
	}
	previous := input{}
	paths := make(map[string]struct{}, len(r.Inputs))
	for index, in := range r.Inputs {
		if !identifier(in.Handle) {
			return "INVALID_IDENTIFIER"
		}
		if !identifier(in.Family) {
			return "INVALID_IDENTIFIER"
		}
		if !logicalPath(in.Path) {
			return "INVALID_PATH"
		}
		if !digest(in.SHA256) {
			return "DIGEST_MISMATCH"
		}
		if _, seen := paths[in.Path]; seen {
			return "DUPLICATE_VALUE"
		}
		paths[in.Path] = struct{}{}
		if index > 0 && (in.Handle == previous.Handle || compareInput(previous, in) >= 0) {
			return "DUPLICATE_VALUE"
		}
		previous = in
	}
	return ""
}
func closedFamily(value string) bool {
	switch value {
	case "go", "python", "javascript-typescript", "dotnet", "ruby", "sqlite", Family, "shell", "swift-apple":
		return true
	}
	return false
}

func validateAndDecode(r request) ([]decodedInput, string) {
	total := 0
	decoded := make([]decodedInput, 0, len(r.Inputs))
	for _, in := range r.Inputs {
		if _, ok := inputProfile(in.Family); !ok {
			return nil, "UNSUPPORTED_SCHEMA"
		}
		if len(in.ContentBase64) > maxBase64Bytes || !canonicalBase64(in.ContentBase64) {
			return nil, "MALFORMED_INPUT"
		}
		decodedLength := decodedBase64Length(in.ContentBase64)
		if decodedLength > maxContentBytes || total > maxContentBytes-decodedLength {
			return nil, "LIMIT_EXCEEDED"
		}
		body, err := base64.StdEncoding.DecodeString(in.ContentBase64)
		if err != nil || len(body) != decodedLength {
			return nil, "MALFORMED_INPUT"
		}
		total += len(body)
		got := sha256.Sum256(body)
		if in.SHA256 != "sha256:"+hex.EncodeToString(got[:]) {
			return nil, "DIGEST_MISMATCH"
		}
		decoded = append(decoded, decodedInput{in, body})
	}
	return decoded, ""
}

func inputProfile(value string) (string, bool) {
	switch value {
	case "html.document":
		return HTMLInputProfile, true
	case "css.stylesheet":
		return CSSInputProfile, true
	default:
		return "", false
	}
}
func canonicalBase64(value string) bool {
	if len(value)%4 != 0 {
		return false
	}
	padding := 0
	if len(value) > 0 && value[len(value)-1] == '=' {
		padding++
		if len(value) > 1 && value[len(value)-2] == '=' {
			padding++
		}
	}
	for i := range len(value) - padding {
		if base64Value(value[i]) < 0 {
			return false
		}
	}
	for i := len(value) - padding; i < len(value); i++ {
		if value[i] != '=' {
			return false
		}
	}
	if padding == 1 && len(value) >= 2 && base64Value(value[len(value)-2])&3 != 0 {
		return false
	}
	if padding == 2 && len(value) >= 3 && base64Value(value[len(value)-3])&15 != 0 {
		return false
	}
	return true
}
func base64Value(value byte) int {
	switch {
	case value >= 'A' && value <= 'Z':
		return int(value - 'A')
	case value >= 'a' && value <= 'z':
		return int(value-'a') + 26
	case value >= '0' && value <= '9':
		return int(value-'0') + 52
	case value == '+':
		return 62
	case value == '/':
		return 63
	default:
		return -1
	}
}
func decodedBase64Length(value string) int {
	padding := 0
	if len(value) > 0 && value[len(value)-1] == '=' {
		padding++
		if value[len(value)-2] == '=' {
			padding++
		}
	}
	return len(value)/4*3 - padding
}

type factCollector struct {
	request           request
	requestDigest     string
	inputs            map[string]input
	targetJSON        string
	facts             []fact
	inputFactCount    int
	prospectiveOutput int
}

func newFactCollector(r request) (*factCollector, string) {
	frame, err := json.Marshal(r)
	if err != nil {
		return nil, "OUTPUT_LIMIT"
	}
	return newFactCollectorWithDigest(r, requestDigest(append(frame, '\n')))
}

func newFactCollectorWithDigest(r request, completeRequestDigest string) (*factCollector, string) {
	targetBytes, err := json.Marshal(r.Target)
	if err != nil || len(targetBytes) > math.MaxUint32 {
		return nil, "LIMIT_EXCEEDED"
	}
	collector := &factCollector{
		request:       r,
		requestDigest: completeRequestDigest,
		inputs:        make(map[string]input, len(r.Inputs)),
		targetJSON:    string(targetBytes),
		facts:         make([]fact, 0, 32),
	}
	for _, in := range r.Inputs {
		collector.inputs[in.Handle] = in
	}
	empty, err := json.Marshal(success{Profile, Family, r.RequestID, "CANDIDATE", r.ScopeID, r.CompilationUnitID, r.Target, echoes(r), collector.facts})
	if err != nil || len(empty)+1 > maxOutputBytes {
		return nil, "OUTPUT_LIMIT"
	}
	collector.prospectiveOutput = len(empty) + 1
	return collector, ""
}
func (c *factCollector) beginInput() { c.inputFactCount = 0 }
func (c *factCollector) add(next fact) string {
	if c.inputFactCount == maxFactsPerInput || len(c.facts) == maxFacts {
		return "LIMIT_EXCEEDED"
	}
	evidence, ok := c.evidence(next)
	if !ok {
		return "LIMIT_EXCEEDED"
	}
	next.EvidenceSHA256 = evidence
	size := factJSONSize(next)
	comma := 0
	if len(c.facts) > 0 {
		comma = 1
	}
	if !withinOutputLimit(c.prospectiveOutput, size, comma) {
		return "OUTPUT_LIMIT"
	}
	c.prospectiveOutput += size + comma
	c.inputFactCount++
	c.facts = append(c.facts, next)
	return ""
}

func withinOutputLimit(total, next, comma int) bool {
	return total <= maxOutputBytes-next-comma
}

const factJSONOverhead = len(`{"kind":"","input_handle":"","related_handle":"","subject":"","predicate":"","value":"","instance_id":"","evidence_sha256":""}`)

func factJSONSize(value fact) int {
	return factJSONOverhead + len(value.Kind) + len(value.InputHandle) + len(value.RelatedHandle) + len(value.Subject) + len(value.Predicate) + len(value.Value) + len(value.InstanceID) + len(value.EvidenceSHA256)
}
func compareInput(left, right input) int {
	if value := strings.Compare(left.Handle, right.Handle); value != 0 {
		return value
	}
	if value := strings.Compare(left.Family, right.Family); value != 0 {
		return value
	}
	if value := strings.Compare(left.Path, right.Path); value != 0 {
		return value
	}
	return strings.Compare(left.SHA256, right.SHA256)
}

// sortFacts returns the exact production comparator work for its in-place canonical ordering.
// The caller ignores the metric; the deterministic ratchet consumes it without using timing.
func sortFacts(facts []fact) int {
	comparisons := 0
	slices.SortFunc(facts, func(left, right fact) int {
		comparisons++
		return compareFact(left, right)
	})
	return comparisons
}
func compareFact(left, right fact) int {
	if value := strings.Compare(left.Kind, right.Kind); value != 0 {
		return value
	}
	if value := strings.Compare(left.InputHandle, right.InputHandle); value != 0 {
		return value
	}
	if value := strings.Compare(left.RelatedHandle, right.RelatedHandle); value != 0 {
		return value
	}
	if value := strings.Compare(left.Subject, right.Subject); value != 0 {
		return value
	}
	if value := strings.Compare(left.Predicate, right.Predicate); value != 0 {
		return value
	}
	if value := strings.Compare(left.Value, right.Value); value != 0 {
		return value
	}
	if value := strings.Compare(left.InstanceID, right.InstanceID); value != 0 {
		return value
	}
	return strings.Compare(left.EvidenceSHA256, right.EvidenceSHA256)
}
func identifier(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for i := range s {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || (i > 0 && strings.ContainsRune("._:@+~-", rune(c)))) {
			return false
		}
	}
	return true
}
func logicalPath(s string) bool {
	if len(s) == 0 || len(s) > maxJSONOrdinaryString || strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." || len(p) > 128 {
			return false
		}
		for _, c := range []byte(p) {
			if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune("._@+~-", rune(c))) {
				return false
			}
		}
	}
	return true
}
func digest(s string) bool {
	if len(s) != 71 || !strings.HasPrefix(s, "sha256:") {
		return false
	}
	for _, c := range s[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func echoes(r request) []inputEcho {
	out := make([]inputEcho, len(r.Inputs))
	for i, in := range r.Inputs {
		out[i] = inputEcho{in.Handle, in.Family, in.Path, in.SHA256}
	}
	return out
}
func sentinel(reason string) []byte {
	if !sentinelReason(reason) {
		reason = "NONCANONICAL_REQUEST"
	}
	out, _ := json.Marshal(sentinelRejected{Profile, "unknown", "unknown", "REJECTED", reason})
	return append(out, '\n')
}
func envelope(r request, reason string) []byte {
	out, _ := json.Marshal(rejected{r.Profile, r.Family, r.RequestID, "REJECTED", r.ScopeID, r.CompilationUnitID, r.Target, echoes(r), reason})
	return append(out, '\n')
}
func (c *factCollector) evidence(f fact) (string, bool) {
	related := input{Handle: "-", Family: "-", Path: "-", SHA256: "-"}
	if f.RelatedHandle != "-" {
		var ok bool
		related, ok = c.inputs[f.RelatedHandle]
		if !ok {
			return "", false
		}
	}
	own, ok := c.inputs[f.InputHandle]
	if !ok {
		return "", false
	}
	fields := []string{c.request.Family, c.request.RequestID, c.request.ScopeID, c.request.CompilationUnitID, c.targetJSON, c.requestDigest, own.Handle, own.Family, own.Path, own.SHA256, related.Handle, related.Family, related.Path, related.SHA256, f.Kind, f.Subject, f.Predicate, f.Value, f.InstanceID}
	if len(fields) > math.MaxUint32 {
		return "", false
	}
	h := sha256.New()
	_, _ = h.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(fields)))
	_, _ = h.Write(n[:])
	for _, value := range fields {
		if len(value) > math.MaxUint32 {
			return "", false
		}
		binary.BigEndian.PutUint32(n[:], uint32(len(value)))
		_, _ = h.Write(n[:])
		_, _ = h.Write([]byte(value))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), true
}

func sentinelReason(value string) bool {
	switch value {
	case "NONCANONICAL_REQUEST", "INVALID_IDENTIFIER", "INVALID_PATH", "DIGEST_MISMATCH", "UNKNOWN_FAMILY", "DUPLICATE_VALUE", "LIMIT_EXCEEDED":
		return true
	default:
		return false
	}
}
func hexDigit(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}
