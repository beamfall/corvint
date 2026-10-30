package analyzerjs

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/maphash"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	Profile           = "corvint-analyzer-candidate/experimental"
	Family            = "javascript-typescript"
	MaxInputs         = 128
	MaxSourceBytes    = 1 << 20
	MaxFacts          = 4096
	MaxOutputBytes    = 1 << 20
	MaxDepth          = 32
	MaxEnvelopeDepth  = 8
	MaxEnvelopeTokens = 4096
	MaxFeatures       = 64
	MaxTextBytes      = 4096
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
	canonical         bool
}
type InputEcho struct {
	Handle string `json:"handle"`
	Family string `json:"family"`
	Path   string `json:"path"`
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
type Candidate struct {
	Profile           string
	Family            string
	RequestID         string `json:"request_id"`
	Status            string
	Reason            string `json:"reason,omitempty"`
	ScopeID           string `json:"scope_id,omitempty"`
	CompilationUnitID string `json:"compilation_unit_id,omitempty"`
	Target            Target
	InputEchoes       []InputEcho `json:"input_echoes,omitempty"`
	Facts             []Fact
	prospective       int
	seal              uint64
	sealed            bool
}
type wire struct {
	Profile           string      `json:"profile"`
	Family            string      `json:"family"`
	RequestID         string      `json:"request_id"`
	Status            string      `json:"status"`
	ScopeID           string      `json:"scope_id"`
	CompilationUnitID string      `json:"compilation_unit_id"`
	Target            Target      `json:"target"`
	InputEchoes       []InputEcho `json:"input_echoes"`
	Facts             *[]Fact     `json:"facts,omitempty"`
	Reason            string      `json:"reason,omitempty"`
}
type cError struct{ reason string }

var candidateSealSeed = maphash.MakeSeed()

func (e cError) Error() string   { return e.reason }
func reject(reason string) error { return cError{reason: reason} }

type sourceFile struct {
	Input
	bytes string
}
type evidenceContext struct {
	prefix []byte
	frame  []byte
}

func Analyze(request Request) (Candidate, error) {
	files, err := validateRequest(request, false)
	if err != nil {
		return Candidate{}, err
	}
	return analyze(request, files)
}
func Rejection(requestID string, reason string) Candidate {
	if requestID != "unknown" || !failureReason(reason) {
		requestID, reason = "unknown", "ANALYZER_FAILURE"
	}
	return finalizedCandidate(Candidate{Profile: Profile, Family: "unknown", RequestID: requestID, Status: "REJECTED", Reason: reason})
}
func RejectionForRequest(request Request, reason string) Candidate {
	if _, err := validateRequest(request, true); err != nil || !request.canonical {
		return Rejection("unknown", reason)
	}
	if !failureReason(reason) {
		reason = "ANALYZER_FAILURE"
	}
	return finalizedCandidate(Candidate{Profile: request.Profile, Family: request.Family, RequestID: request.RequestID, Status: "REJECTED", ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target, InputEchoes: inputEchoes(request.Inputs), Reason: reason})
}
func finalizedCandidate(candidate Candidate) Candidate {
	if encoded, err := marshalCandidate(candidate); err == nil {
		candidate.prospective = len(encoded) + 1
	}
	candidate.sealed = true
	candidate.seal = seal(candidate)
	return candidate
}
func (candidate Candidate) MarshalJSON() ([]byte, error) {
	if !candidate.sealed || candidate.prospective <= 0 || candidate.seal != seal(candidate) {
		return nil, reject("ANALYZER_FAILURE")
	}
	return marshalCandidate(candidate)
}
func marshalCandidate(candidate Candidate) ([]byte, error) {
	if candidate.Status == "REJECTED" {
		if candidate.Family == "unknown" {
			return canonicalJSON(struct {
				Profile   string `json:"profile"`
				Family    string `json:"family"`
				RequestID string `json:"request_id"`
				Status    string `json:"status"`
				Reason    string `json:"reason"`
			}{candidate.Profile, candidate.Family, candidate.RequestID, candidate.Status, candidate.Reason})
		}
	}
	out := wire{Profile: candidate.Profile, Family: candidate.Family, RequestID: candidate.RequestID, Status: candidate.Status, ScopeID: candidate.ScopeID, CompilationUnitID: candidate.CompilationUnitID, Target: candidate.Target, InputEchoes: candidate.InputEchoes, Reason: candidate.Reason}
	if candidate.Status == "CANDIDATE" {
		out.Facts = &candidate.Facts
	}
	return canonicalJSON(out)
}
func validateRequest(request Request, echo bool) ([]sourceFile, error) {
	if request.Profile != Profile && !echo {
		return nil, reject("UNSUPPORTED_SCHEMA")
	}
	if request.Family != Family {
		return nil, reject("UNKNOWN_FAMILY")
	}
	if len(request.Target.Features) > MaxFeatures {
		return nil, reject("LIMIT_EXCEEDED")
	}
	if !identifier(request.RequestID) || !identifier(request.ScopeID) || !identifier(request.CompilationUnitID) || !validTarget(request.Target) {
		return nil, reject("INVALID_IDENTIFIER")
	}
	if len(request.Inputs) == 0 || len(request.Inputs) > MaxInputs {
		return nil, reject("LIMIT_EXCEEDED")
	}
	files := make([]sourceFile, 0, len(request.Inputs))
	total := 0
	for index, input := range request.Inputs {
		if !identifier(input.Handle) || !identifier(input.Family) {
			return nil, reject("INVALID_IDENTIFIER")
		}
		if !logicalPath(input.Path) {
			return nil, reject("INVALID_PATH")
		}
		if !sha256Text(input.SHA256) {
			return nil, reject("DIGEST_MISMATCH")
		}
		if index > 0 && compareInput(request.Inputs[index-1], input) >= 0 {
			return nil, reject("DUPLICATE_VALUE")
		}
		for prior := 0; prior < index; prior++ {
			if request.Inputs[prior].Handle == input.Handle || request.Inputs[prior].Path == input.Path {
				return nil, reject("DUPLICATE_VALUE")
			}
		}
		if echo {
			continue
		}
		if !supportedInput(input.Family, input.Path) {
			if input.Family == "js.bun-lock-v1" || input.Family == "js.tsconfig" {
				return nil, reject("UNSUPPORTED_SCHEMA")
			}
			return nil, reject("UNKNOWN_FAMILY")
		}
		decodedSize, err := boundedBase64Size(input.ContentBase64)
		if err != nil {
			return nil, err
		}
		if decodedSize > MaxSourceBytes-total {
			return nil, reject("LIMIT_EXCEEDED")
		}
		decoded := make([]byte, decodedSize)
		written, err := base64.StdEncoding.Strict().Decode(decoded, []byte(input.ContentBase64))
		if err != nil || written != decodedSize || !utf8.Valid(decoded) {
			return nil, reject("MALFORMED_INPUT")
		}
		total += decodedSize
		digest := sha256.Sum256(decoded)
		if input.SHA256 != "sha256:"+hex.EncodeToString(digest[:]) {
			return nil, reject("DIGEST_MISMATCH")
		}
		files = append(files, sourceFile{Input: input, bytes: string(decoded)})
	}
	return files, nil
}
func boundedBase64Size(value string) (int, error) {
	if len(value) > maxContentBase64Size {
		return 0, reject("LIMIT_EXCEEDED")
	}
	if len(value)%4 != 0 {
		return 0, reject("MALFORMED_INPUT")
	}
	return base64UpperBound(value), nil
}
func supportedInput(family, logicalPath string) bool {
	switch family {
	case "js.package":
		return path.Base(logicalPath) == "package.json"
	case "js.npm-lock-v3":
		return path.Base(logicalPath) == "package-lock.json"
	case "js.source":
		switch path.Ext(logicalPath) {
		case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts":
			return true
		}
	}
	return false
}
func identifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	first := value[0]
	if !((first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') || (first >= '0' && first <= '9')) {
		return false
	}
	for index := range value {
		c := value[index]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || strings.ContainsRune("._:@+~-", rune(c))) {
			return false
		}
	}
	return true
}
func validTarget(target Target) bool {
	if target.Features == nil || !identifier(target.OS) || !identifier(target.Architecture) || !identifier(target.ABI) {
		return false
	}
	for index, feature := range target.Features {
		if !identifier(feature) || (index > 0 && target.Features[index-1] >= feature) {
			return false
		}
	}
	return true
}
func logicalPath(value string) bool {
	if len(value) == 0 || len(value) > MaxTextBytes || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.ContainsAny(value, "\\:%") || path.Clean(value) != value {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." || !identifier("a"+segment[1:]) || !identifier("a"+segment[:1]) {
			return false
		}
	}
	return true
}
func sha256Text(value string) bool {
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
func text(value any) (string, bool) {
	result, ok := value.(string)
	return result, ok && printable(result)
}
func printable(value string) bool {
	if len(value) == 0 || len(value) > MaxTextBytes {
		return false
	}
	for index := range value {
		if value[index] < 0x21 || value[index] > 0x7e || value[index] == '"' || value[index] == '\\' {
			return false
		}
	}
	return true
}
func object(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok
}
func compareInput(left, right Input) int {
	return slices.Compare([]string{left.Handle, left.Family, left.Path, left.SHA256}, []string{right.Handle, right.Family, right.Path, right.SHA256})
}
func compareFact(left, right Fact) int {
	return slices.Compare([]string{left.Kind, left.InputHandle, left.RelatedHandle, left.Subject, left.Predicate, left.Value, left.InstanceID, left.EvidenceSHA256}, []string{right.Kind, right.InputHandle, right.RelatedHandle, right.Subject, right.Predicate, right.Value, right.InstanceID, right.EvidenceSHA256})
}
func sortFacts(facts []Fact) error {
	slices.SortFunc(facts, compareFact)
	for index := 1; index < len(facts); index++ {
		if compareFact(facts[index-1], facts[index]) == 0 {
			return reject("DUPLICATE_VALUE")
		}
	}
	return nil
}
func EncodeCandidate(candidate Candidate) ([]byte, error) {
	if candidate.prospective > MaxOutputBytes {
		return nil, reject("OUTPUT_LIMIT")
	}
	encoded, err := candidate.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if len(encoded)+1 > MaxOutputBytes {
		return nil, reject("OUTPUT_LIMIT")
	}
	if len(encoded)+1 != candidate.prospective {
		return nil, reject("ANALYZER_FAILURE")
	}
	return encoded, nil
}
func seal(candidate Candidate) uint64 {
	var state maphash.Hash
	state.SetSeed(candidateSealSeed)
	write := func(values ...string) {
		for _, value := range values {
			state.WriteString(value)
			state.WriteString("\x00")
		}
	}
	count := func(value int) { state.WriteString(strconv.Itoa(value)); state.WriteString("\x00") }
	write(candidate.Profile, candidate.Family, candidate.RequestID, candidate.Status, candidate.Reason, candidate.ScopeID, candidate.CompilationUnitID)
	write("target:value", candidate.Target.OS, candidate.Target.Architecture, candidate.Target.ABI)
	count(len(candidate.Target.Features))
	write(candidate.Target.Features...)
	count(len(candidate.InputEchoes))
	for _, echo := range candidate.InputEchoes {
		write(echo.Handle, echo.Family, echo.Path, echo.SHA256)
	}
	count(len(candidate.Facts))
	for _, fact := range candidate.Facts {
		write(fact.Kind, fact.InputHandle, fact.RelatedHandle, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID, fact.EvidenceSHA256)
	}
	return state.Sum64()
}
func FailureReason(err error) string {
	if value, ok := err.(cError); ok && failureReason(value.reason) {
		return value.reason
	}
	return "ANALYZER_FAILURE"
}
func failureReason(reason string) bool {
	switch reason {
	case "NONCANONICAL_REQUEST", "INVALID_IDENTIFIER", "INVALID_PATH", "DIGEST_MISMATCH", "UNKNOWN_FAMILY", "UNKNOWN_FIELD", "DUPLICATE_VALUE", "CONFLICTING_VALUE", "MALFORMED_INPUT", "UNSUPPORTED_SCHEMA", "EXACT_BINDING_UNAVAILABLE", "AMBIGUOUS_BINDING", "DYNAMIC_INPUT", "CREDENTIAL_INPUT", "LIMIT_EXCEEDED", "OUTPUT_LIMIT", "ANALYZER_FAILURE":
		return true
	}
	return false
}
func makeFact(evidence *evidenceContext, input sourceFile, related *sourceFile, kind, subject, predicate, value, instance string) (Fact, error) {
	relatedHandle, relatedSHA256 := "-", "-"
	if related != nil {
		relatedHandle, relatedSHA256 = related.Handle, related.SHA256
	}
	fact := Fact{Kind: kind, InputHandle: input.Handle, RelatedHandle: relatedHandle, Subject: subject, Predicate: predicate, Value: value, InstanceID: instance}
	digest, err := evidence.digest(input.SHA256, relatedSHA256, fact)
	if err != nil {
		return Fact{}, err
	}
	fact.EvidenceSHA256 = digest
	return fact, nil
}
func validateFact(fact Fact) bool {
	switch fact.Kind {
	case "js.workspace", "js.package.locked", "js.dependency.locked", "js.source", "js.import.static":
		return identifier(fact.InputHandle) && (fact.RelatedHandle == "-" || identifier(fact.RelatedHandle)) && printable(fact.Subject) && printable(fact.Predicate) && printable(fact.Value) && printable(fact.InstanceID) && sha256Text(fact.EvidenceSHA256)
	}
	return false
}
func newEvidenceContext(request Request) (*evidenceContext, error) {
	context := &evidenceContext{}
	context.prefix = append(context.prefix, "corvint-analyzer-candidate-evidence/experimental"...)
	context.prefix = binary.BigEndian.AppendUint32(context.prefix, 14)
	for _, field := range [...]string{Family, request.RequestID, request.ScopeID, request.CompilationUnitID, targetJSON(request.Target)} {
		if err := context.appendPrefix(field); err != nil {
			return nil, err
		}
	}
	context.frame = make([]byte, len(context.prefix), len(context.prefix)+512)
	copy(context.frame, context.prefix)
	return context, nil
}
func (context *evidenceContext) appendPrefix(field string) error {
	if uint64(len(field)) > uint64(^uint32(0)) || len(context.prefix) > MaxOutputBytes-len(field)-4 {
		return reject("LIMIT_EXCEEDED")
	}
	context.prefix = binary.BigEndian.AppendUint32(context.prefix, uint32(len(field)))
	context.prefix = append(context.prefix, field...)
	return nil
}
func (context *evidenceContext) digest(inputSHA256, relatedSHA256 string, fact Fact) (string, error) {
	frame := context.frame[:len(context.prefix)]
	for _, field := range [...]string{fact.InputHandle, inputSHA256, fact.RelatedHandle, relatedSHA256, fact.Kind, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID} {
		if uint64(len(field)) > uint64(^uint32(0)) {
			return "", reject("LIMIT_EXCEEDED")
		}
		frame = binary.BigEndian.AppendUint32(frame, uint32(len(field)))
		frame = append(frame, field...)
	}
	context.frame = frame
	sum := sha256.Sum256(frame)
	var encoded [len("sha256:") + sha256.Size*2]byte
	copy(encoded[:], "sha256:")
	hex.Encode(encoded[len("sha256:"):], sum[:])
	return string(encoded[:]), nil
}
func targetJSON(target Target) string {
	result := `{"os":"` + target.OS + `","architecture":"` + target.Architecture + `","abi":"` + target.ABI + `","features":[`
	for index, feature := range target.Features {
		if index > 0 {
			result += ","
		}
		result += `"` + feature + `"`
	}
	return result + `]}`
}
func addFact(facts *[]Fact, fact Fact, prospective *int) error {
	if !validateFact(fact) {
		return reject("MALFORMED_INPUT")
	}
	if len(*facts) >= MaxFacts {
		return reject("LIMIT_EXCEEDED")
	}
	delta := factEncodedSize(fact)
	if len(*facts) > 0 {
		delta++
	}
	if delta > MaxOutputBytes-*prospective {
		return reject("OUTPUT_LIMIT")
	}
	*prospective += delta
	*facts = append(*facts, fact)
	return nil
}
func candidateBaseSize(request Request, files []sourceFile) int {
	size := len(`{"profile":"","family":"","request_id":"","status":"CANDIDATE","scope_id":"","compilation_unit_id":"","target":{"os":"","architecture":"","abi":"","features":[]},"input_echoes":[],"facts":[]}`)
	size += len(Profile) + len(Family) + len(request.RequestID) + len(request.ScopeID) + len(request.CompilationUnitID)
	size += len(request.Target.OS) + len(request.Target.Architecture) + len(request.Target.ABI)
	for index, feature := range request.Target.Features {
		size += len(feature) + 2
		if index > 0 {
			size++
		}
	}
	for index, file := range files {
		size += len(`{"handle":"","family":"","path":"","sha256":""}`) + len(file.Handle) + len(file.Family) + len(file.Path) + len(file.SHA256)
		if index > 0 {
			size++
		}
	}
	return size + 1
}
func factEncodedSize(fact Fact) int {
	return len(`{"kind":"","input_handle":"","related_handle":"","subject":"","predicate":"","value":"","instance_id":"","evidence_sha256":""}`) + len(fact.Kind) + len(fact.InputHandle) + len(fact.RelatedHandle) + len(fact.Subject) + len(fact.Predicate) + len(fact.Value) + len(fact.InstanceID) + len(fact.EvidenceSHA256)
}
func inputEchoes(inputs []Input) []InputEcho {
	echoes := make([]InputEcho, len(inputs))
	for index, input := range inputs {
		echoes[index] = InputEcho{Handle: input.Handle, Family: input.Family, Path: input.Path, SHA256: input.SHA256}
	}
	return echoes
}
func requireOnlyKeys(value map[string]any, allowed ...string) error {
	for key := range value {
		known := false
		for _, permitted := range allowed {
			if key == permitted {
				known = true
				break
			}
		}
		if !known {
			return reject("UNKNOWN_FIELD")
		}
	}
	return nil
}
func jsonNumber(value any, expected string) bool {
	number, ok := value.(json.Number)
	return ok && number.String() == expected
}
