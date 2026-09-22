// Package analyzerdotnet is an isolated, static-only .NET fact candidate.
//
// It reads one canonical request envelope from bytes, validates it against the
// frozen experimental analyzer-candidate profile, and emits closed facts drawn
// only from the request's own declared inputs. It never opens a path, consults
// an installed SDK or NuGet feed, evaluates MSBuild, expands a glob, or
// executes anything.
package analyzerdotnet

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/json/jsontext"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	Profile = "corvint-analyzer-candidate/experimental"
	Family  = "dotnet"

	MaxRequestBytes         = 1_500_000
	MaxInputBytes           = 1 << 20
	MaxOutputBytes          = 1 << 20
	MaxContentBase64Bytes   = 1_398_104
	MaxAggregateBase64Bytes = 1_398_104
	MaxInputs               = 128
	MaxFeatures             = 64
	MaxIdentifierBytes      = 128
	MaxPathBytes            = 4096
	MaxPathSegmentBytes     = 128
	MaxJSONDepth            = 8
	MaxJSONTokens           = 4096
	MaxFactFieldBytes       = 4096

	// ACP: each input carries a prospective fact ceiling and the request
	// carries an independent one. Both are enforced before a fact is retained.
	MaxFactsPerInput = 4096
	MaxFactsPerLoad  = 4096

	// Qualification budget, mirrored by the per-family row in
	// docs/specs/analyzer-candidate-profiles.md.
	MaxBytesPerOp  = 130_000
	MaxAllocsPerOp = 900
	MaxBinaryBytes = 6_291_456
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

type failure struct {
	Profile           string  `json:"profile"`
	Family            string  `json:"family"`
	RequestID         string  `json:"request_id"`
	Status            string  `json:"status"`
	ScopeID           string  `json:"scope_id,omitempty"`
	CompilationUnitID string  `json:"compilation_unit_id,omitempty"`
	Target            *Target `json:"target,omitempty"`
	InputEchoes       []Echo  `json:"input_echoes,omitempty"`
	Reason            string  `json:"reason"`
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@+~-]{0,127}$`)
var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// AnalyzeCanonical is the whole candidate. raw is the sole input; no path is
// ever opened and no subprocess is ever started.
func AnalyzeCanonical(raw []byte) []byte {
	request, why := decodeRequest(raw)
	if why == "UNKNOWN_FIELD" {
		return bound(request, why)
	}
	if why != "" {
		return sentinel(why)
	}
	// The whole echoed envelope -- family, request, scope, unit, target, and
	// every input handle and digest -- has now passed its grammar and duplicate
	// checks, so every failure past this point binds it. A schema failure must
	// bind: two requests with the same false digest but different inputs would
	// otherwise collapse to one replayable response.
	if why := exactTarget(request.Target); why != "" {
		return bound(request, why)
	}
	for _, input := range request.Inputs {
		if why := admissibleInput(input); why != "" {
			return bound(request, why)
		}
	}
	collector := newFactCollector(request)
	decodedTotal := 0
	for _, input := range request.Inputs {
		body, why := decodeContent(input, MaxInputBytes-decodedTotal)
		if why != "" {
			return bound(request, why)
		}
		decodedTotal += len(body)
		collector.beginInput()
		if why := parseInto(input, body, collector); why != "" {
			return bound(request, why)
		}
	}
	facts := collector.facts
	sort.Slice(facts, func(i, j int) bool { return factLess(facts[i], facts[j]) })
	encoded, err := json.Marshal(success{Profile, Family, request.RequestID, "CANDIDATE",
		request.ScopeID, request.CompilationUnitID, request.Target, echoes(request), facts})
	if err != nil {
		return bound(request, "ANALYZER_FAILURE")
	}
	if len(encoded)+1 != collector.outputSize || over(len(encoded)+1, MaxOutputBytes) {
		return bound(request, "OUTPUT_LIMIT")
	}
	return append(encoded, '\n')
}

func decodeRequest(raw []byte) (Request, string) {
	if len(raw) == 0 || over(len(raw), MaxRequestBytes) || raw[len(raw)-1] != '\n' ||
		bytes.Count(raw, []byte("\n")) != 1 || !utf8.Valid(raw) {
		return Request{}, "NONCANONICAL_REQUEST"
	}
	body := raw[:len(raw)-1]
	if !boundedDistinctNames(body) {
		return Request{}, "NONCANONICAL_REQUEST"
	}
	var request Request
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		if json.Unmarshal(body, &Request{}) != nil {
			return Request{}, "MALFORMED_INPUT"
		}
		// Only a safe canonical extension binds UNKNOWN_FIELD (decision 0222).
		if known, ok := withoutExtensions(body); ok && len(known) < len(body) {
			request, why := decodeRequest(append(known, '\n'))
			if why == "" {
				why = "UNKNOWN_FIELD"
			}
			return request, why
		}
		return Request{}, "NONCANONICAL_REQUEST"
	}
	canonical, err := json.Marshal(request)
	if err != nil || !bytes.Equal(canonical, body) {
		return Request{}, "NONCANONICAL_REQUEST"
	}
	if request.Profile != Profile {
		return Request{}, "MALFORMED_INPUT"
	}
	if request.Family != Family {
		return Request{}, "UNKNOWN_FAMILY"
	}
	if !identifierPattern.MatchString(request.RequestID) {
		return Request{}, "INVALID_IDENTIFIER"
	}
	return request, validPreEnvelope(&request)
}

var extensionFields = map[string]string{"": " profile family request_id scope_id compilation_unit_id target inputs ", "target": " os architecture abi features ", "inputs": " handle family path sha256 content_base64 "}

func withoutExtensions(body []byte) ([]byte, bool) {
	decoder, encoded, known, at := jsontext.NewDecoder(bytes.NewReader(body)), &bytes.Buffer{}, []byte{}, int64(0)
	encoder := jsontext.NewEncoder(encoded)
	var value func(kind string) bool
	value = func(kind string) bool {
		token, err := decoder.ReadToken()
		delimiter, text := token.Kind(), token.String()
		ok, fields, extended, last := err == nil && encoder.WriteToken(token) == nil, extensionFields[kind], false, ""
		for ok && delimiter == '[' && decoder.PeekKind() != ']' {
			ok = value(kind)
		}
		for ok && delimiter == '{' && decoder.PeekKind() == '"' {
			start := decoder.InputOffset()
			token, _ = decoder.ReadToken()
			name := token.String()
			ok = encoder.WriteToken(token) == nil
			if _, nested := extensionFields[name]; !strings.Contains(name, " ") && strings.Contains(fields, " "+name+" ") {
				if !nested {
					name = "-"
				}
				ok = ok && !extended && value(name)
				continue
			}
			ok = ok && (!extended || name > last) && value("-")
			extended, last = true, name
			if fields != "" {
				known, at = append(known, body[at:start]...), decoder.InputOffset()
			}
		}
		if delimiter == '[' || delimiter == '{' {
			token, err = decoder.ReadToken()
			ok = ok && err == nil && encoder.WriteToken(token) == nil
		}
		return ok && (delimiter != '0' || text != "-0" && !strings.ContainsAny(text, ".eE"))
	}
	ok := value("") && encoded.String() == string(body)+"\n"
	return append(known, body[at:]...), ok
}

// decodeContent decodes exactly one input's content and proves its digest.
// remaining is the aggregate decoded budget still available to this request.
func decodeContent(input Input, remaining int) ([]byte, string) {
	size, exact := decodedLength(input.ContentBase64)
	if !exact {
		return nil, "MALFORMED_INPUT"
	}
	if size < 0 || over(size, remaining) {
		return nil, "LIMIT_EXCEEDED"
	}
	decoded := make([]byte, size)
	written, err := base64.StdEncoding.Decode(decoded, []byte(input.ContentBase64))
	if err != nil || base64.StdEncoding.EncodeToString(decoded[:written]) != input.ContentBase64 {
		return nil, "MALFORMED_INPUT"
	}
	decoded = decoded[:written]
	sum := sha256.Sum256(decoded)
	if input.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
		return nil, "DIGEST_MISMATCH"
	}
	return decoded, ""
}

func decodedLength(encoded string) (int, bool) {
	if len(encoded)%4 != 0 {
		return 0, false
	}
	size := base64.StdEncoding.DecodedLen(len(encoded))
	if len(encoded) > 0 && encoded[len(encoded)-1] == '=' {
		size--
	}
	if len(encoded) > 1 && encoded[len(encoded)-2] == '=' {
		size--
	}
	return size, size >= 0
}

func over(value, bound int) bool { return value > bound }

// boundedDistinctNames walks the request within the profile's depth and token
// bounds and rejects any duplicate object name, which json.Decode would
// otherwise silently last-wins.
func boundedDistinctNames(raw []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	tokens := 0
	var walk func(int) bool
	walk = func(depth int) bool {
		tokens++
		if depth > MaxJSONDepth || tokens > MaxJSONTokens {
			return false
		}
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		delim, isDelim := token.(json.Delim)
		if !isDelim {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				name, err := decoder.Token()
				if err != nil {
					return false
				}
				text, ok := name.(string)
				if !ok || seen[text] {
					return false
				}
				seen[text] = true
				if !walk(depth + 1) {
					return false
				}
			}
			_, err := decoder.Token()
			return err == nil
		case '[':
			for decoder.More() {
				if !walk(depth + 1) {
					return false
				}
			}
			_, err := decoder.Token()
			return err == nil
		}
		return false
	}
	if !walk(0) {
		return false
	}
	_, err := decoder.Token()
	return err != nil
}

func sentinel(reason string) []byte {
	encoded, _ := json.Marshal(failure{Profile: Profile, Family: "unknown",
		RequestID: "unknown", Status: "REJECTED", Reason: reason})
	return append(encoded, '\n')
}

func bound(request Request, reason string) []byte {
	target := request.Target
	encoded, _ := json.Marshal(failure{Profile, Family, request.RequestID, "REJECTED",
		request.ScopeID, request.CompilationUnitID, &target, echoes(request), reason})
	return append(encoded, '\n')
}

func echoes(request Request) []Echo {
	records := make([]Echo, len(request.Inputs))
	for index := range request.Inputs {
		records[index] = Echo{request.Inputs[index].Handle, request.Inputs[index].SHA256}
	}
	return records
}

func validPreEnvelope(request *Request) string {
	if request.Target.Features == nil || request.Inputs == nil {
		return "MALFORMED_INPUT"
	}
	if !identifierPattern.MatchString(request.ScopeID) ||
		!identifierPattern.MatchString(request.CompilationUnitID) ||
		!identifierPattern.MatchString(request.Target.OS) ||
		!identifierPattern.MatchString(request.Target.Architecture) ||
		!identifierPattern.MatchString(request.Target.ABI) {
		return "INVALID_IDENTIFIER"
	}
	if over(len(request.Target.Features), MaxFeatures) || over(len(request.Inputs), MaxInputs) {
		return "LIMIT_EXCEEDED"
	}
	if len(request.Inputs) == 0 {
		return "MALFORMED_INPUT"
	}
	for index, feature := range request.Target.Features {
		if !identifierPattern.MatchString(feature) || index > 0 && request.Target.Features[index-1] >= feature {
			return "MALFORMED_INPUT"
		}
	}
	aggregate := 0
	seenHandles := map[string]bool{}
	seenPaths := map[string]bool{}
	for index, input := range request.Inputs {
		if !identifierPattern.MatchString(input.Handle) {
			return "INVALID_IDENTIFIER"
		}
		if !pathOK(input.Path) {
			return "INVALID_PATH"
		}
		if !digestPattern.MatchString(input.SHA256) {
			return "DIGEST_MISMATCH"
		}
		if seenHandles[input.Handle] || seenPaths[input.Path] {
			return "DUPLICATE_VALUE"
		}
		if index > 0 && !inputLess(request.Inputs[index-1], input) {
			return "DUPLICATE_VALUE"
		}
		if over(len(input.ContentBase64), MaxContentBase64Bytes) {
			return "LIMIT_EXCEEDED"
		}
		seenHandles[input.Handle] = true
		seenPaths[input.Path] = true
		aggregate += len(input.ContentBase64)
		if over(aggregate, MaxAggregateBase64Bytes) {
			return "LIMIT_EXCEEDED"
		}
	}
	return ""
}

func inputLess(left, right Input) bool {
	for _, pair := range [][2]string{{left.Handle, right.Handle}, {left.Family, right.Family},
		{left.Path, right.Path}, {left.SHA256, right.SHA256}} {
		if pair[0] != pair[1] {
			return pair[0] < pair[1]
		}
	}
	return false
}

// exactTarget accepts any grammatical target coordinate. No .NET fact in the
// closed matrix is target-conditioned -- DOTNET_RID is read from the project
// file, never from the request -- so pinning a coordinate here would invent a
// rule the profile does not state. Features are a different case: no feature
// is defined for this family, so a supplied one must reject rather than be
// accepted and ignored.
func exactTarget(target Target) string {
	if len(target.Features) != 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}

func pathOK(value string) bool {
	if value == "" || over(len(value), MaxPathBytes) ||
		strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if !pathSegmentOK(segment) {
			return false
		}
	}
	return true
}

func pathSegmentOK(segment string) bool {
	if segment == "" || segment == "." || segment == ".." || over(len(segment), MaxPathSegmentBytes) {
		return false
	}
	for index := 0; index < len(segment); index++ {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._@+~-", rune(segment[index])) {
			return false
		}
	}
	return true
}

// factCollector is shared by every input in one request. Both the per-input and
// the per-request ceilings are charged before a fact is retained, so a hostile
// input can never grow an unbounded intermediate slice.
type factCollector struct {
	request    Request
	facts      []Fact
	seen       map[Fact]struct{}
	outputSize int
	perInput   int
}

func newFactCollector(request Request) *factCollector {
	return &factCollector{
		request:    request,
		facts:      make([]Fact, 0, 16),
		seen:       make(map[Fact]struct{}, 16),
		outputSize: emptySuccessSize(request) + 1,
	}
}

func (collector *factCollector) beginInput() { collector.perInput = 0 }

func (collector *factCollector) add(input Input, kind, subject, predicate, value, instance string) string {
	fact := Fact{Kind: kind, InputHandle: input.Handle, RelatedHandle: "-",
		Subject: subject, Predicate: predicate, Value: value, InstanceID: instance}
	if !factFieldsOK(fact, false) {
		return "LIMIT_EXCEEDED"
	}
	fact.EvidenceSHA256 = evidence(collector.request, input, fact)
	if !factFieldsOK(fact, true) {
		return "LIMIT_EXCEEDED"
	}
	if _, duplicate := collector.seen[fact]; duplicate {
		return "DUPLICATE_VALUE"
	}
	if over(collector.perInput+1, MaxFactsPerInput) || over(len(collector.facts)+1, MaxFactsPerLoad) {
		return "LIMIT_EXCEEDED"
	}
	next := collector.outputSize + factJSONSize(fact)
	if len(collector.facts) != 0 {
		next++
	}
	if over(next, MaxOutputBytes) {
		return "OUTPUT_LIMIT"
	}
	collector.facts = append(collector.facts, fact)
	collector.seen[fact] = struct{}{}
	collector.outputSize = next
	collector.perInput++
	return ""
}

func factFieldsOK(fact Fact, includeEvidence bool) bool {
	for _, field := range [...]string{fact.Kind, fact.InputHandle, fact.RelatedHandle,
		fact.Subject, fact.Predicate, fact.Value, fact.InstanceID} {
		if !factFieldOK(field) {
			return false
		}
	}
	return !includeEvidence || factFieldOK(fact.EvidenceSHA256)
}

// factFieldOK enforces the profile's printable-ASCII fact-field grammar. A
// quote or backslash would need a JSON escape and is excluded outright, so the
// prospective size function and the encoder can never disagree.
func factFieldOK(field string) bool {
	if field == "" || over(len(field), MaxFactFieldBytes) {
		return false
	}
	for index := 0; index < len(field); index++ {
		character := field[index]
		if character < 0x21 || character > 0x7e || character == '"' || character == '\\' {
			return false
		}
	}
	return true
}

func factLess(left, right Fact) bool {
	for _, pair := range [][2]string{{left.Kind, right.Kind}, {left.InputHandle, right.InputHandle},
		{left.RelatedHandle, right.RelatedHandle}, {left.Subject, right.Subject},
		{left.Predicate, right.Predicate}, {left.Value, right.Value},
		{left.InstanceID, right.InstanceID}, {left.EvidenceSHA256, right.EvidenceSHA256}} {
		if pair[0] != pair[1] {
			return pair[0] < pair[1]
		}
	}
	return false
}

func factJSONSize(fact Fact) int {
	return len(`{"kind":"","input_handle":"","related_handle":"","subject":"","predicate":"","value":"","instance_id":"","evidence_sha256":""}`) +
		len(fact.Kind) + len(fact.InputHandle) + len(fact.RelatedHandle) + len(fact.Subject) +
		len(fact.Predicate) + len(fact.Value) + len(fact.InstanceID) + len(fact.EvidenceSHA256)
}

func emptySuccessSize(request Request) int {
	return 10 + len("profile") + 3 + len(Profile) + 2 + len("family") + 3 + len(Family) +
		2 + len("request_id") + 3 + len(request.RequestID) +
		2 + len("status") + 3 + len("CANDIDATE") +
		2 + len("scope_id") + 3 + len(request.ScopeID) +
		2 + len("compilation_unit_id") + 3 + len(request.CompilationUnitID) +
		2 + len("target") + 3 + targetJSONSize(request.Target) +
		len("input_echoes") + 3 + echoJSONSize(request) + len("facts") + 5
}

func targetJSONSize(target Target) int {
	features := 2
	for index, feature := range target.Features {
		if index > 0 {
			features++
		}
		features += len(feature) + 2
	}
	return 5 + len("os") + 3 + len(target.OS) + 2 + len("architecture") + 3 + len(target.Architecture) +
		2 + len("abi") + 3 + len(target.ABI) + 2 + len("features") + 3 + features
}

func echoJSONSize(request Request) int {
	size := 2
	for index, echo := range echoes(request) {
		if index > 0 {
			size++
		}
		size += len(`{"handle":"","sha256":""}`) + len(echo.Handle) + len(echo.SHA256)
	}
	return size
}

// evidence recomputes the profile's 14-field preimage from this candidate's own
// owned request bytes and the fact. Every .NET fact has exactly one input
// witness, so the related handle and related digest are always the literal "-".
func evidence(request Request, input Input, fact Fact) string {
	target, _ := json.Marshal(request.Target)
	fields := []string{Family, request.RequestID, request.ScopeID, request.CompilationUnitID,
		string(target), fact.InputHandle, input.SHA256, "-", "-",
		fact.Kind, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID}
	hash := sha256.New()
	hash.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(fields)))
	hash.Write(length[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		hash.Write(length[:])
		hash.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

// Sentinel is the fixed pre-envelope failure response, used before any part of
// the request has passed its grammar.
func Sentinel(reason string) []byte { return sentinel(reason) }
