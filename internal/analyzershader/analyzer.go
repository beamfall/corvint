// Package analyzershader is an unregistered, admission-candidate-only shader
// extractor. It accepts immutable caller bytes and deliberately has no ambient
// filesystem, process, network, compiler, or dynamic-loader dependency.
package analyzershader

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/json/jsontext"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	Profile     = "corvint-analyzer-candidate/experimental"
	Family      = "shader"
	CEMOCMState = "UNSUPPORTED"
	maxWire     = 1_500_000
	maxInput    = 1_048_576
	maxOutput   = 1_048_576
	maxFacts    = 4_096
	maxWitness  = 4_096
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
	ShaderProfile     string  `json:"shader_profile"`
	ShaderLanguage    string  `json:"shader_language"`
	ShaderVersion     string  `json:"shader_version"`
	ShaderToolchain   string  `json:"shader_toolchain"`
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
type Position struct {
	Byte   uint32 `json:"byte"`
	Line   uint32 `json:"line"`
	Column uint32 `json:"column"`
}
type Span struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Fact struct {
	Kind           string `json:"kind"`
	InputHandle    string `json:"input_handle"`
	RelatedHandle  string `json:"related_handle"`
	Subject        string `json:"subject"`
	Predicate      string `json:"predicate"`
	Value          string `json:"value"`
	InstanceID     string `json:"instance_id"`
	Span           Span   `json:"span"`
	WitnessBase64  string `json:"witness_base64"`
	WitnessSHA256  string `json:"witness_sha256"`
	EvidenceSHA256 string `json:"evidence_sha256"`
}
type output struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	ShaderProfile     string `json:"shader_profile"`
	ShaderLanguage    string `json:"shader_language"`
	ShaderVersion     string `json:"shader_version"`
	ShaderToolchain   string `json:"shader_toolchain"`
	RequestID         string `json:"request_id"`
	RequestSHA256     string `json:"request_sha256"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            Target `json:"target"`
	InputEchoes       []Echo `json:"input_echoes"`
	Facts             []Fact `json:"facts"`
}
type rejected struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	ShaderProfile     string `json:"shader_profile"`
	ShaderLanguage    string `json:"shader_language"`
	ShaderVersion     string `json:"shader_version"`
	ShaderToolchain   string `json:"shader_toolchain"`
	RequestID         string `json:"request_id"`
	RequestSHA256     string `json:"request_sha256"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            Target `json:"target"`
	InputEchoes       []Echo `json:"input_echoes"`
	Reason            string `json:"reason"`
}

// exactProfile is a closed language/version/toolchain/stage tuple. Toolchain
// names are candidate extractor tuple identities, not compiler observations.
type exactProfile struct {
	language, version, toolchain, stage, inputFamily, sourceVersion string
}

var profiles = map[string]exactProfile{
	"beamfall.glsl-es-3.00.fragment.visual-shaders": {"glsl", "3.00", "glslang-16.4.0.spirv-cross-2026-07-06T12-43-32", "fragment", "shader.glsl", "300 es"},
	"beamfall.glsl-es-3.00.vertex.visual-shaders":   {"glsl", "3.00", "glslang-16.4.0.spirv-cross-2026-07-06T12-43-32", "vertex", "shader.glsl", "300 es"},
	"beamfall.gles-3.00.fragment.android-generated": {"gles", "3.00", "beamfall-android-gles-contract-1.0.0", "fragment", "shader.gles", "300 es"},
	"beamfall.gles-3.00.vertex.android-generated":   {"gles", "3.00", "beamfall-android-gles-contract-1.0.0", "vertex", "shader.gles", "300 es"},
	"beamfall.metal-3.1.vertex.apple-ui":            {"metal", "3.1", "metal-3.1-structural-lexer-v0", "vertex", "shader.metal", ""},
	"beamfall.metal-3.1.fragment.apple-ui":          {"metal", "3.1", "metal-3.1-structural-lexer-v0", "fragment", "shader.metal", ""},
	"beamfall.metal-3.1.compute.apple-ui":           {"metal", "3.1", "metal-3.1-structural-lexer-v0", "compute", "shader.metal", ""},
}

var closedReasons = map[string]struct{}{
	"AMBIGUOUS_BINDING": {}, "ANALYZER_FAILURE": {}, "CREDENTIAL_INPUT": {},
	"CONFLICTING_VALUE": {}, "DIGEST_MISMATCH": {}, "DUPLICATE_VALUE": {},
	"DYNAMIC_INPUT": {}, "EXACT_BINDING_UNAVAILABLE": {}, "INVALID_IDENTIFIER": {},
	"INVALID_PATH": {}, "LIMIT_EXCEEDED": {}, "MALFORMED_INPUT": {},
	"NONCANONICAL_REQUEST": {}, "OUTPUT_LIMIT": {}, "UNKNOWN_FAMILY": {},
	"UNKNOWN_FIELD": {}, "UNSUPPORTED_SCHEMA": {},
}

// sentinelReasons is the analyzer-candidate-profiles sentinel reason list
// (decision 0213).
var sentinelReasons = map[string]struct{}{
	"NONCANONICAL_REQUEST": {}, "INVALID_IDENTIFIER": {}, "INVALID_PATH": {}, "DIGEST_MISMATCH": {},
	"UNKNOWN_FAMILY": {}, "DUPLICATE_VALUE": {}, "LIMIT_EXCEEDED": {},
}

func sentinel(reason string) []byte {
	if _, ok := sentinelReasons[reason]; !ok {
		reason = "ANALYZER_FAILURE"
	}
	return []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + reason + `"}` + "\n")
}

// Analyze accepts exactly one canonical JSON/LF frame and emits exactly one
// canonical JSON/LF frame. It is pure: all data used is reachable from frame.
func Analyze(frame []byte) []byte {
	if len(frame) > maxWire {
		return sentinel("LIMIT_EXCEEDED")
	}
	var request Request
	extended := decode(frame, &request)
	if extended != "" && extended != "UNKNOWN_FIELD" {
		return sentinel(extended)
	}
	if reason := preEnvelope(request); reason != "" {
		return sentinel(reason)
	}
	if reason := boundedEchoedFields(request); reason != "" {
		return sentinel(reason)
	}
	echoes := echoes(request.Inputs)
	requestSHA256 := requestDigest(frame)
	if !rejectionFits(request, echoes, requestSHA256) {
		return sentinel("LIMIT_EXCEEDED")
	}
	if extended != "" {
		return reject(request, echoes, requestSHA256, extended)
	}
	if reason := envelope(request); reason != "" {
		return reject(request, echoes, requestSHA256, reason)
	}
	decoded, reason := contents(request)
	if reason != "" {
		return reject(request, echoes, requestSHA256, reason)
	}
	profile, ok := profiles[request.ShaderProfile]
	if !ok || !matchesExactTuple(request, profile) {
		return reject(request, echoes, requestSHA256, "EXACT_BINDING_UNAVAILABLE")
	}
	if request.Target.Features == nil || len(request.Target.Features) != 0 || !identifier(request.Target.OS) || !identifier(request.Target.Architecture) || !identifier(request.Target.ABI) {
		return reject(request, echoes, requestSHA256, "UNSUPPORTED_SCHEMA")
	}
	facts, reason := newFactAccumulator(request, echoes, requestSHA256)
	if reason != "" {
		return reject(request, echoes, requestSHA256, reason)
	}
	for _, in := range request.Inputs {
		if in.Family != profile.inputFamily {
			return reject(request, echoes, requestSHA256, "EXACT_BINDING_UNAVAILABLE")
		}
		if profile.language == "metal" {
			reason = parseMetal(request, in, decoded[in.Handle], profile, &facts)
		} else {
			reason = parseGLSL(request, in, decoded[in.Handle], profile, &facts)
		}
		if reason != "" {
			return reject(request, echoes, requestSHA256, reason)
		}
	}
	if len(facts.values) == 0 {
		return reject(request, echoes, requestSHA256, "EXACT_BINDING_UNAVAILABLE")
	}
	sort.Slice(facts.values, func(i, j int) bool { return lessFact(facts.values[i], facts.values[j]) })
	for i := 1; i < len(facts.values); i++ {
		if !lessFact(facts.values[i-1], facts.values[i]) && !lessFact(facts.values[i], facts.values[i-1]) {
			return reject(request, echoes, requestSHA256, "DUPLICATE_VALUE")
		}
	}
	result, reason := encodeCandidate(request, echoes, requestSHA256, facts.values)
	if reason != "" {
		return reject(request, echoes, requestSHA256, reason)
	}
	return result
}

func encodeCandidate(request Request, inputEchoes []Echo, requestSHA256 string, facts []Fact) ([]byte, string) {
	if facts == nil {
		facts = []Fact{}
	}
	base, err := json.Marshal(output{Profile: Profile, Family: Family, ShaderProfile: request.ShaderProfile, ShaderLanguage: request.ShaderLanguage, ShaderVersion: request.ShaderVersion, ShaderToolchain: request.ShaderToolchain, RequestID: request.RequestID, RequestSHA256: requestSHA256, Status: "CANDIDATE", ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target, InputEchoes: inputEchoes, Facts: []Fact{}})
	if err != nil || len(base)+1 > maxOutput {
		return nil, "OUTPUT_LIMIT"
	}
	prospective := len(base) + 1
	for index := range facts {
		encoded, err := json.Marshal(facts[index])
		if err != nil {
			return nil, "ANALYZER_FAILURE"
		}
		if prospective+len(encoded)+boolByte(index > 0) > maxOutput {
			return nil, "OUTPUT_LIMIT"
		}
		prospective += len(encoded) + boolByte(index > 0)
	}
	result, err := json.Marshal(output{Profile: Profile, Family: Family, ShaderProfile: request.ShaderProfile, ShaderLanguage: request.ShaderLanguage, ShaderVersion: request.ShaderVersion, ShaderToolchain: request.ShaderToolchain, RequestID: request.RequestID, RequestSHA256: requestSHA256, Status: "CANDIDATE", ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target, InputEchoes: inputEchoes, Facts: facts})
	if err != nil || len(result)+1 != prospective || len(result)+1 > maxOutput {
		return nil, "OUTPUT_LIMIT"
	}
	return append(result, '\n'), ""
}

func boolByte(value bool) int {
	if value {
		return 1
	}
	return 0
}
func decode(frame []byte, destination *Request) string {
	if len(frame) < 2 || frame[len(frame)-1] != '\n' || frame[len(frame)-2] == '\n' || !utf8.Valid(frame[:len(frame)-1]) || !json.Valid(frame[:len(frame)-1]) {
		return "NONCANONICAL_REQUEST"
	}
	decoder := json.NewDecoder(bytes.NewReader(frame[:len(frame)-1]))
	tokens := 0
	if !uniqueJSON(decoder, 0, &tokens) {
		return "NONCANONICAL_REQUEST"
	}
	if strictDecode(frame[:len(frame)-1], destination) {
		return ""
	}
	// Only a safe canonical extension binds UNKNOWN_FIELD (decision 0222).
	*destination = Request{}
	if known, ok := withoutExtensions(frame[:len(frame)-1]); ok && len(known) != len(frame)-1 && strictDecode(known, destination) {
		return "UNKNOWN_FIELD"
	}
	return "NONCANONICAL_REQUEST"
}

func strictDecode(body []byte, destination *Request) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	encoded, err := []byte(nil), decoder.Decode(destination)
	if err == nil {
		encoded, err = json.Marshal(*destination)
	}
	return err == nil && bytes.Equal(encoded, body)
}

// extensionFields names each schema object's fields; any other member is an extension.
var extensionFields = map[string]string{"": " profile family shader_profile shader_language shader_version shader_toolchain request_id scope_id compilation_unit_id target inputs ", "target": " os architecture abi features ", "inputs": " handle family path sha256 content_base64 "}

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
func uniqueJSON(decoder *json.Decoder, depth int, tokens *int) bool {
	if depth > 8 || *tokens >= 4096 {
		return false
	}
	value, err := decoder.Token()
	*tokens++
	if err != nil {
		return false
	}
	delimiter, ok := value.(json.Delim)
	if !ok {
		return true
	}
	if delimiter == '{' {
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			*tokens++
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return false
			}
			seen[name] = true
			if !uniqueJSON(decoder, depth+1, tokens) {
				return false
			}
		}
		_, err := decoder.Token()
		*tokens++
		return err == nil
	}
	if delimiter == '[' {
		for decoder.More() {
			if !uniqueJSON(decoder, depth+1, tokens) {
				return false
			}
		}
		_, err := decoder.Token()
		*tokens++
		return err == nil
	}
	return false
}
func preEnvelope(request Request) string {
	if request.Profile != Profile || request.Family != Family {
		return "UNKNOWN_FAMILY"
	}
	if !identifier(request.ShaderProfile) || !identifier(request.ShaderLanguage) || !identifier(request.ShaderVersion) || !identifier(request.ShaderToolchain) || !identifier(request.RequestID) || !identifier(request.ScopeID) || !identifier(request.CompilationUnitID) {
		return "INVALID_IDENTIFIER"
	}
	if len(request.Inputs) == 0 || len(request.Inputs) > 128 {
		return "LIMIT_EXCEEDED"
	}
	seenHandles := make(map[string]struct{}, len(request.Inputs))
	for _, in := range request.Inputs {
		if !identifier(in.Handle) {
			return "INVALID_IDENTIFIER"
		}
		if _, duplicate := seenHandles[in.Handle]; duplicate {
			return "DUPLICATE_VALUE"
		}
		seenHandles[in.Handle] = struct{}{}
	}
	return ""
}

// boundedEchoedFields runs before echo construction. A rejected request may
// retain these fields only after each has a closed grammar and bounded size;
// otherwise it names the listed sentinel reason (decision 0221).
func boundedEchoedFields(request Request) string {
	features := request.Target.Features
	if features == nil {
		return "NONCANONICAL_REQUEST"
	}
	if len(features) > 64 {
		return "LIMIT_EXCEEDED"
	}
	if !identifier(request.Target.OS) || !identifier(request.Target.Architecture) || !identifier(request.Target.ABI) {
		return "INVALID_IDENTIFIER"
	}
	for i, feature := range features {
		if !identifier(feature) {
			return "INVALID_IDENTIFIER"
		}
		if i > 0 && features[i-1] >= feature {
			return "DUPLICATE_VALUE"
		}
	}
	for _, in := range request.Inputs {
		if !digest(in.SHA256) {
			return "DIGEST_MISMATCH"
		}
	}
	return ""
}

// envelope runs after every echoed field is bounded. Its failures can retain
// the complete request binding without allowing an unbounded response.
func envelope(request Request) string {
	previous := Input{}
	seenPaths := make(map[string]struct{}, len(request.Inputs))
	for i, in := range request.Inputs {
		if !logicalPath(in.Path) {
			if credentialLike(in.Path) {
				return "CREDENTIAL_INPUT"
			}
			return "INVALID_PATH"
		}
		if in.Family != "shader.glsl" && in.Family != "shader.gles" && in.Family != "shader.metal" {
			return "UNSUPPORTED_SCHEMA"
		}
		if len(in.ContentBase64) > 1_398_104 {
			return "LIMIT_EXCEEDED"
		}
		if i > 0 && compareInput(previous, in) >= 0 {
			return "DUPLICATE_VALUE"
		}
		if _, duplicate := seenPaths[in.Path]; duplicate {
			return "DUPLICATE_VALUE"
		}
		seenPaths[in.Path] = struct{}{}
		previous = in
	}
	return ""
}
func contents(request Request) (map[string][]byte, string) {
	result := make(map[string][]byte, len(request.Inputs))
	total := 0
	for _, in := range request.Inputs {
		size, ok := canonicalBase64Length(in.ContentBase64)
		if !ok {
			return nil, "MALFORMED_INPUT"
		}
		if size > maxInput || total > maxInput-size {
			return nil, "LIMIT_EXCEEDED"
		}
		total += size
		body, err := base64.StdEncoding.Strict().DecodeString(in.ContentBase64)
		if err != nil || len(body) != size || base64.StdEncoding.EncodeToString(body) != in.ContentBase64 {
			return nil, "MALFORMED_INPUT"
		}
		sum := sha256.Sum256(body)
		if in.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			return nil, "DIGEST_MISMATCH"
		}
		result[in.Handle] = body
	}
	return result, ""
}

func canonicalBase64Length(value string) (int, bool) {
	if len(value) == 0 || len(value)%4 != 0 {
		return 0, false
	}
	padding := 0
	if value[len(value)-1] == '=' {
		padding++
		if value[len(value)-2] == '=' {
			padding++
		}
	}
	for index := 0; index < len(value)-padding; index++ {
		c := value[index]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/') {
			return 0, false
		}
	}
	for index := len(value) - padding; index < len(value); index++ {
		if value[index] != '=' {
			return 0, false
		}
	}
	return len(value)/4*3 - padding, true
}
func reject(request Request, inputEchoes []Echo, requestSHA256, reason string) []byte {
	value, ok := rejectedFrame(request, inputEchoes, requestSHA256, reason, maxOutput)
	if !ok {
		return sentinel("LIMIT_EXCEEDED")
	}
	return value
}

func rejectionFits(request Request, inputEchoes []Echo, requestSHA256 string) bool {
	_, ok := rejectedFrame(request, inputEchoes, requestSHA256, "EXACT_BINDING_UNAVAILABLE", maxOutput)
	return ok
}

func rejectedFrame(request Request, inputEchoes []Echo, requestSHA256, reason string, maximum int) ([]byte, bool) {
	if !closedReason(reason) {
		reason = "ANALYZER_FAILURE"
	}
	value, err := json.Marshal(rejected{Profile: Profile, Family: Family, ShaderProfile: request.ShaderProfile, ShaderLanguage: request.ShaderLanguage, ShaderVersion: request.ShaderVersion, ShaderToolchain: request.ShaderToolchain, RequestID: request.RequestID, RequestSHA256: requestSHA256, Status: "REJECTED", ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target, InputEchoes: inputEchoes, Reason: reason})
	if err != nil || len(value)+1 > maximum {
		return nil, false
	}
	return append(value, '\n'), true
}

func closedReason(reason string) bool {
	_, ok := closedReasons[reason]
	return ok
}

func matchesExactTuple(request Request, profile exactProfile) bool {
	return request.ShaderLanguage == profile.language && request.ShaderVersion == profile.version && request.ShaderToolchain == profile.toolchain
}

func requestDigest(frame []byte) string {
	sum := sha256.Sum256(frame[:len(frame)-1])
	return "sha256:" + hex.EncodeToString(sum[:])
}

type factAccumulator struct {
	values      []Fact
	prospective int
}

func newFactAccumulator(request Request, inputEchoes []Echo, requestSHA256 string) (factAccumulator, string) {
	base, err := json.Marshal(output{Profile: Profile, Family: Family, ShaderProfile: request.ShaderProfile, ShaderLanguage: request.ShaderLanguage, ShaderVersion: request.ShaderVersion, ShaderToolchain: request.ShaderToolchain, RequestID: request.RequestID, RequestSHA256: requestSHA256, Status: "CANDIDATE", ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target, InputEchoes: inputEchoes, Facts: []Fact{}})
	if err != nil || len(base)+1 > maxOutput {
		return factAccumulator{}, "OUTPUT_LIMIT"
	}
	return factAccumulator{prospective: len(base) + 1}, ""
}

func (facts *factAccumulator) retain(fact Fact) string {
	if len(facts.values) == maxFacts {
		return "LIMIT_EXCEEDED"
	}
	encoded, err := json.Marshal(fact)
	if err != nil {
		return "ANALYZER_FAILURE"
	}
	additional := len(encoded)
	if len(facts.values) > 0 {
		additional++
	}
	if facts.prospective > maxOutput-additional {
		return "OUTPUT_LIMIT"
	}
	facts.prospective += additional
	facts.values = append(facts.values, fact)
	return ""
}
func echoes(inputs []Input) []Echo {
	values := make([]Echo, len(inputs))
	for i := range inputs {
		values[i] = Echo{inputs[i].Handle, inputs[i].SHA256}
	}
	return values
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
	if len(value) == 0 || len(value) > 4096 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.ContainsRune(value, ':') || strings.ContainsRune(value, '\\') || strings.ContainsRune(value, '%') {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." || !pathSegment(part) {
			return false
		}
	}
	return true
}
func pathSegment(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for index := range value {
		c := value[index]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune("._@+~-", rune(c))) {
			return false
		}
	}
	return true
}
func credentialLike(value string) bool {
	return strings.Contains(value, "://") && strings.Contains(value, "@")
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
func compareInput(left, right Input) int {
	for _, pair := range [][2]string{{left.Handle, right.Handle}, {left.Family, right.Family}, {left.Path, right.Path}, {left.SHA256, right.SHA256}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}
func lessFact(left, right Fact) bool {
	leftFields := [...]string{left.Kind, left.InputHandle, left.RelatedHandle, left.Subject, left.Predicate, left.Value, left.InstanceID, left.WitnessSHA256, left.EvidenceSHA256}
	rightFields := [...]string{right.Kind, right.InputHandle, right.RelatedHandle, right.Subject, right.Predicate, right.Value, right.InstanceID, right.WitnessSHA256, right.EvidenceSHA256}
	for index := range leftFields {
		if leftFields[index] != rightFields[index] {
			return leftFields[index] < rightFields[index]
		}
	}
	return false
}

type evidenceBinding struct {
	family, shaderProfile, shaderLanguage, shaderVersion, shaderToolchain string
	requestID, scopeID, compilationUnitID, requestSHA256, target          string
	inputDigests                                                          map[string]string
}

func newEvidenceBinding(request Request) (evidenceBinding, bool) {
	target, err := json.Marshal(request.Target)
	if err != nil {
		return evidenceBinding{}, false
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return evidenceBinding{}, false
	}
	requestSum := sha256.Sum256(requestBytes)
	digests := make(map[string]string, len(request.Inputs))
	for _, input := range request.Inputs {
		digests[input.Handle] = input.SHA256
	}
	return evidenceBinding{family: request.Family, shaderProfile: request.ShaderProfile, shaderLanguage: request.ShaderLanguage, shaderVersion: request.ShaderVersion, shaderToolchain: request.ShaderToolchain, requestID: request.RequestID, scopeID: request.ScopeID, compilationUnitID: request.CompilationUnitID, requestSHA256: "sha256:" + hex.EncodeToString(requestSum[:]), target: string(target), inputDigests: digests}, true
}

func evidence(request Request, fact Fact) string {
	binding, ok := newEvidenceBinding(request)
	if !ok {
		return ""
	}
	return binding.evidence(fact)
}

func (binding evidenceBinding) evidence(fact Fact) string {
	fields := []string{binding.family, binding.shaderProfile, binding.shaderLanguage, binding.shaderVersion, binding.shaderToolchain, binding.requestID, binding.requestSHA256, binding.scopeID, binding.compilationUnitID, binding.target, fact.InputHandle, binding.inputDigest(fact.InputHandle), fact.RelatedHandle, binding.inputDigest(fact.RelatedHandle), fact.Kind, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID, fact.WitnessSHA256, uint32String(fact.Span.Start.Byte), uint32String(fact.Span.Start.Line), uint32String(fact.Span.Start.Column), uint32String(fact.Span.End.Byte), uint32String(fact.Span.End.Line), uint32String(fact.Span.End.Column)}
	h := sha256.New()
	_, _ = h.Write([]byte("corvint-analyzer-shader-evidence/experimental"))
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(fields)))
	_, _ = h.Write(count[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(count[:], uint32(len(field)))
		_, _ = h.Write(count[:])
		_, _ = h.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func uint32String(value uint32) string {
	if value == 0 {
		return "0"
	}
	var digits [10]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
func (binding evidenceBinding) inputDigest(handle string) string {
	if handle == "-" {
		return "-"
	}
	return binding.inputDigests[handle]
}
