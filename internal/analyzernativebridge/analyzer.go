package analyzernativebridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"strings"
)

const (
	Profile           = "corvint-analyzer-native-bridge/v0"
	CProfile          = Profile
	ObjectiveCProfile = Profile
	JavaProfile       = Profile
	maxWire           = 1_500_000
	maxInput          = 1_048_576
	maxFacts          = 4_096
	maxOutput         = 1_048_576
	maxJSONDepth      = 8
	maxJSONTokens     = 4_096
	maxJSONString     = 1_398_104
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
type echo struct {
	Handle string `json:"handle"`
	SHA256 string `json:"sha256"`
}
type span struct {
	StartByte   int `json:"start_byte"`
	EndByte     int `json:"end_byte"`
	StartLine   int `json:"start_line"`
	StartColumn int `json:"start_column"`
	EndLine     int `json:"end_line"`
	EndColumn   int `json:"end_column"`
}
type fact struct {
	Kind           string `json:"kind"`
	InputHandle    string `json:"input_handle"`
	RelatedHandle  string `json:"related_handle"`
	Subject        string `json:"subject"`
	Predicate      string `json:"predicate"`
	Value          string `json:"value"`
	InstanceID     string `json:"instance_id"`
	Span           span   `json:"-"`
	Witness        string `json:"-"`
	EvidenceSHA256 string `json:"evidence_sha256"`
}
type output struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	RequestID         string `json:"request_id"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            target `json:"target"`
	InputEchoes       []echo `json:"input_echoes"`
	Facts             []fact `json:"facts"`
}
type rejection struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	RequestID         string `json:"request_id"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            target `json:"target"`
	InputEchoes       []echo `json:"input_echoes"`
	Reason            string `json:"reason"`
}

var sentinel = []byte(`{"profile":"corvint-analyzer-native-bridge/v0","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n")
var limitSentinel = []byte(`{"profile":"corvint-analyzer-native-bridge/v0","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"LIMIT_EXCEEDED"}` + "\n")

func AnalyzeC(reader io.Reader) []byte { return analyze(reader, "c", parseCBody) }
func AnalyzeObjectiveC(reader io.Reader) []byte {
	return analyze(reader, "objective-c", parseObjectiveCBody)
}
func AnalyzeJava(reader io.Reader) []byte { return analyze(reader, "java", parseJavaBody) }

// NJB-008 argv guard.
func Run(args []string, version string, reader io.Reader, writer io.Writer, analyze func(io.Reader) []byte) error {
	data := sentinel
	if len(args) == 0 {
		data = analyze(reader)
	} else if len(args) == 1 && args[0] == "--version" {
		data = []byte(version + "\n")
	}
	for len(data) != 0 {
		written, err := writer.Write(data)
		if uint(written) > uint(len(data)) {
			return io.ErrShortWrite
		}
		data = data[written:]
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

type parserFunc func(request, input, []byte, *factCollector) string

type factCollector struct {
	facts       []fact
	outputBytes int
}

func newFactCollector(req request, echoes []echo) factCollector {
	return factCollector{facts: make([]fact, 0, 64), outputBytes: emptyOutputBytes(req, echoes)}
}

// add retains each distinct fact once: restating a retained fact is a no-op,
// never an error. The evidence digest binds every newFact identity field.
func (collector *factCollector) add(value fact) string {
	if !validFact(value) {
		return "LIMIT_EXCEEDED"
	}
	for index := range collector.facts {
		if collector.facts[index].EvidenceSHA256 == value.EvidenceSHA256 && collector.facts[index].RelatedHandle == value.RelatedHandle {
			return ""
		}
	}
	if len(collector.facts) == maxFacts {
		return "LIMIT_EXCEEDED"
	}
	prospective, ok := prospectiveFactBytes(collector.outputBytes, len(collector.facts), value)
	if !ok {
		return "OUTPUT_LIMIT"
	}
	collector.outputBytes = prospective
	collector.facts = append(collector.facts, value)
	return ""
}

func analyze(reader io.Reader, requiredFamily string, parser parserFunc) (result []byte) {
	result = sentinel
	defer func() {
		if recover() != nil {
			result = sentinel
		}
	}()
	wire, preflight := readWire(reader)
	if preflight != preflightValid {
		return preflightResponse(preflight)
	}
	if preflight = preflightRequestResult(wire[:len(wire)-1]); preflight != preflightValid {
		return preflightResponse(preflight)
	}
	var req request
	decoder := json.NewDecoder(bytes.NewReader(wire[:len(wire)-1]))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&req) != nil || decoder.More() {
		return sentinel
	}
	canonical, err := json.Marshal(req)
	if err != nil || !bytes.Equal(canonical, wire[:len(wire)-1]) || req.Target.Features == nil {
		return sentinel
	}
	if !bound(req) {
		return sentinel
	}
	if parser == nil {
		return sentinel
	}
	echoes := requestEchoes(req.Inputs)
	if reason := validate(req); reason != "" {
		return rejected(req, echoes, reason)
	}
	if req.Family != requiredFamily {
		return rejected(req, echoes, "EXACT_BINDING_UNAVAILABLE")
	}
	if bytes.Equal(rejected(req, echoes, "OUTPUT_LIMIT"), sentinel) {
		return sentinel
	}
	bodies := make([][]byte, len(req.Inputs))
	total := 0
	for index, in := range req.Inputs {
		if decodedBase64Bytes(in.ContentBase64) > maxInput-total {
			return rejected(req, echoes, "LIMIT_EXCEEDED")
		}
		body, err := base64.StdEncoding.DecodeString(in.ContentBase64)
		if err != nil || base64.StdEncoding.EncodeToString(body) != in.ContentBase64 {
			return rejected(req, echoes, "MALFORMED_INPUT")
		}
		total += len(body)
		if total > maxInput {
			return rejected(req, echoes, "LIMIT_EXCEEDED")
		}
		sum := sha256.Sum256(body)
		if in.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			return rejected(req, echoes, "DIGEST_MISMATCH")
		}
		bodies[index] = body
	}
	collector := newFactCollector(req, echoes)
	for index, in := range req.Inputs {
		if reason := parser(req, in, bodies[index], &collector); reason != "" {
			return rejected(req, echoes, reason)
		}
	}
	facts := collector.facts
	sort.Slice(facts, func(i, j int) bool { return factLess(facts[i], facts[j]) })
	for i := range facts {
		if i > 0 && !factLess(facts[i-1], facts[i]) && !factLess(facts[i], facts[i-1]) {
			return rejected(req, echoes, "DUPLICATE_VALUE")
		}
	}
	encoded, err := json.Marshal(output{req.Profile, req.Family, req.RequestID, "CANDIDATE", req.ScopeID, req.CompilationUnitID, req.Target, echoes, facts})
	if err != nil || len(encoded)+1 > maxOutput {
		return rejected(req, echoes, "OUTPUT_LIMIT")
	}
	return append(encoded, '\n')
}

func prospectiveFactBytes(current, retained int, next fact) (int, bool) {
	encoded, err := json.Marshal(next)
	if err != nil {
		return 0, false
	}
	separator := 0
	if retained != 0 {
		separator = 1
	}
	nextBytes := current + separator + len(encoded)
	return nextBytes, nextBytes <= maxOutput
}
func emptyOutputBytes(req request, echoes []echo) int {
	encoded, err := json.Marshal(output{req.Profile, req.Family, req.RequestID, "CANDIDATE", req.ScopeID, req.CompilationUnitID, req.Target, echoes, []fact{}})
	if err != nil {
		return maxOutput + 1
	}
	return len(encoded) + 1
}

func readWire(reader io.Reader) ([]byte, preflightResult) {
	limited := io.LimitedReader{R: reader, N: maxWire + 1}
	data, err := io.ReadAll(&limited)
	if err == nil && len(data) > maxWire {
		return nil, preflightLimit
	}
	if err != nil || len(data) < 2 || data[len(data)-1] != '\n' || bytes.Count(data, []byte{'\n'}) != 1 {
		return nil, preflightMalformed
	}
	return data, preflightValid
}
func rejected(req request, echoes []echo, reason string) []byte {
	data, _ := json.Marshal(rejection{req.Profile, req.Family, req.RequestID, "REJECTED", req.ScopeID, req.CompilationUnitID, req.Target, echoes, reason})
	if len(data)+1 > maxOutput {
		return sentinel
	}
	return append(data, '\n')
}

// bound reports whether every value a rejection echoes passed its grammar and
// duplicate check; until then a failure is the sentinel.
func bound(req request) bool {
	handles := map[string]bool{}
	for _, in := range req.Inputs {
		if !identifier(in.Handle) || !digest(in.SHA256) || handles[in.Handle] {
			return false
		}
		handles[in.Handle] = true
	}
	return knownFamily(req.Family) && identifier(req.RequestID) && identifier(req.ScopeID) && identifier(req.CompilationUnitID) && identifier(req.Target.OS) && identifier(req.Target.Architecture) && identifier(req.Target.ABI) && validFeatures(req.Target.Features)
}
func requestEchoes(inputs []input) []echo {
	result := make([]echo, len(inputs))
	for i, in := range inputs {
		result[i] = echo{in.Handle, in.SHA256}
	}
	return result
}
func validate(req request) string {
	if req.Profile != Profile {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	if !knownFamily(req.Family) {
		return "UNKNOWN_FAMILY"
	}
	if len(req.Inputs) == 0 || len(req.Inputs) > 128 || !validFeatures(req.Target.Features) {
		return "UNSUPPORTED_SCHEMA"
	}
	if !exactTarget(req) {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	last := ""
	paths := make(map[string]input, len(req.Inputs))
	for _, in := range req.Inputs {
		if !validInput(req.Family, in) {
			return "MALFORMED_INPUT"
		}
		if prior, exists := paths[in.Path]; exists {
			if prior == in {
				return "DUPLICATE_VALUE"
			}
			return "CONFLICTING_VALUE"
		}
		paths[in.Path] = in
		key := in.Handle + "\x00" + in.Family + "\x00" + in.Path + "\x00" + in.SHA256
		if last != "" && key <= last {
			return "NONCANONICAL_REQUEST"
		}
		last = key
	}
	return ""
}

func validInput(family string, in input) bool {
	return logicalPath(in.Path) && len(in.ContentBase64) <= 1_398_104 && validInputFamily(family, in.Family, in.Path)
}

func decodedBase64Bytes(value string) int {
	if len(value)%4 != 0 {
		return maxInput + 1
	}
	padding := 0
	if strings.HasSuffix(value, "=") {
		padding++
	}
	if strings.HasSuffix(value, "==") {
		padding++
	}
	return len(value)/4*3 - padding
}

type preflightResult uint8

const (
	preflightValid preflightResult = iota
	preflightMalformed
	preflightLimit
)

type preflightFrame struct {
	object bool
	owner  string
	key    string
}

func preflightResponse(result preflightResult) []byte {
	if result == preflightLimit {
		return limitSentinel
	}
	return sentinel
}

func preflightRequest(data []byte) bool { return preflightRequestResult(data) == preflightValid }

func preflightRequestResult(data []byte) preflightResult {
	if len(data) == 0 || len(data) >= maxWire {
		return preflightLimit
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var frames [maxJSONDepth]preflightFrame
	depth, inputs, features, tokens, contentBytes, roots := 0, 0, 0, 0, 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			if depth != 0 || roots != 1 || inputs > 128 {
				return preflightMalformed
			}
			return preflightValid
		}
		if err != nil {
			return preflightMalformed
		}
		tokens++
		if tokens > maxJSONTokens {
			return preflightLimit
		}
		if delimiter, ok := token.(json.Delim); ok {
			if delimiter == '{' || delimiter == '[' {
				if depth == 0 {
					roots++
					if roots > 1 {
						return preflightMalformed
					}
				}
				owner, ok := preflightValueOwner(&frames, depth)
				if !ok {
					return preflightMalformed
				}
				depth++
				if depth > maxJSONDepth {
					return preflightLimit
				}
				frames[depth-1] = preflightFrame{object: delimiter == '{', owner: owner}
				if owner == "inputs" && delimiter == '{' {
					inputs++
					if inputs > 128 {
						return preflightLimit
					}
				}
				continue
			}
			if delimiter != '}' && delimiter != ']' || depth == 0 {
				return preflightMalformed
			}
			frame := frames[depth-1]
			if frame.object != (delimiter == '}') {
				return preflightMalformed
			}
			depth--
			continue
		}
		if depth == 0 {
			return preflightMalformed
		}
		frame := &frames[depth-1]
		if frame.object && frame.key == "" {
			key, ok := token.(string)
			if !ok || len(key) > 4096 {
				return preflightMalformed
			}
			frame.key = key
			continue
		}
		key := frame.key
		if frame.object {
			frame.key = ""
		}
		value, stringValue := token.(string)
		if !frame.object && frame.owner == "features" {
			if !stringValue || len(value) > 128 {
				return preflightMalformed
			}
			features++
			if features > 64 {
				return preflightLimit
			}
		}
		if !stringValue {
			continue
		}
		if len(value) > maxJSONString {
			return preflightLimit
		}
		if key != "content_base64" && len(value) > 4096 {
			return preflightLimit
		}
		if key != "content_base64" {
			continue
		}
		contentBytes += decodedBase64Bytes(value)
		if contentBytes > maxInput {
			return preflightLimit
		}
	}
}

func preflightValueOwner(frames *[maxJSONDepth]preflightFrame, depth int) (string, bool) {
	if depth == 0 {
		return "", true
	}
	frame := &frames[depth-1]
	if !frame.object {
		return frame.owner, true
	}
	if frame.key == "" {
		return "", false
	}
	owner := frame.key
	frame.key = ""
	return owner, true
}
func exactTarget(req request) bool {
	if len(req.Target.Features) != 0 {
		return false
	}
	switch req.Family {
	case "c", "java":
		return req.Target.OS == "android" && req.Target.Architecture == "arm64-v8a" && req.Target.ABI == "android-24"
	case "objective-c":
		return req.Target.OS == "darwin" && req.Target.Architecture == "arm64" && req.Target.ABI == "ios-17.0"
	}
	return false
}

func validFeatures(features []string) bool {
	if len(features) > 64 {
		return false
	}
	prior := ""
	for _, feature := range features {
		if !identifier(feature) || prior != "" && feature <= prior {
			return false
		}
		prior = feature
	}
	return true
}
func knownFamily(value string) bool { return value == "c" || value == "objective-c" || value == "java" }
func validInputFamily(family, inputFamily, path string) bool {
	switch family {
	case "c":
		switch inputFamily {
		case "c.source":
			return strings.HasSuffix(path, ".c")
		case "c.header":
			return strings.HasSuffix(path, ".h")
		case "c.cmake":
			return path == "CMakeLists.txt" || strings.HasSuffix(path, "/CMakeLists.txt") || strings.HasSuffix(path, ".cmake")
		}
	case "objective-c":
		switch inputFamily {
		case "objc.source":
			return strings.HasSuffix(path, ".m") || strings.HasSuffix(path, ".mm")
		case "objc.header":
			return strings.HasSuffix(path, ".h")
		}
	case "java":
		return inputFamily == "java.source" && strings.HasSuffix(path, ".java")
	}
	return false
}
func identifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i := range value {
		c := value[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || i > 0 && strings.ContainsRune("._:@+~-", rune(c))) {
			return false
		}
	}
	return true
}
func logicalPath(value string) bool {
	if len(value) == 0 || len(value) > 4096 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.ContainsRune(value, ':') {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." || !identifier(part) {
			return false
		}
	}
	return true
}
func digest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func factLess(left, right fact) bool {
	for _, pair := range [][2]string{{left.Kind, right.Kind}, {left.InputHandle, right.InputHandle}, {left.RelatedHandle, right.RelatedHandle}, {left.Subject, right.Subject}, {left.Predicate, right.Predicate}, {left.Value, right.Value}, {left.InstanceID, right.InstanceID}, {left.EvidenceSHA256, right.EvidenceSHA256}} {
		if pair[0] != pair[1] {
			return pair[0] < pair[1]
		}
	}
	return false
}

func validFact(value fact) bool {
	fields := [...]string{value.Kind, value.InputHandle, value.RelatedHandle, value.Subject, value.Predicate, value.Value, value.InstanceID, value.EvidenceSHA256}
	for _, field := range fields {
		if len(field) == 0 || len(field) > 4096 {
			return false
		}
		for index := range field {
			if field[index] < 0x20 || field[index] > 0x7e {
				return false
			}
		}
	}
	return true
}
func newFact(req request, in input, kind, subject, predicate, value string, at span, witness string) fact {
	target, _ := json.Marshal(req.Target)
	fields := []string{req.Family, req.RequestID, req.ScopeID, req.CompilationUnitID, string(target), in.Handle, in.SHA256, "-", "-", kind, subject, predicate, value, req.CompilationUnitID}
	return fact{kind, in.Handle, "-", subject, predicate, value, req.CompilationUnitID, at, witness, evidence(fields)}
}
func evidence(fields []string) string {
	h := sha256.New()
	_, _ = h.Write([]byte("corvint-analyzer-native-bridge-evidence/v0"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(fields)))
	_, _ = h.Write(length[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		_, _ = h.Write(length[:])
		_, _ = h.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
