//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	stdjson "encoding/json"
	json "encoding/json/v2"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	maxCandidateFacts       = 4_096
	maxCandidateFeatures    = 64
	maxCandidateStringBytes = 4_096
	maxCandidateJSONTokens  = 4_096
	maxCandidateJSONDepth   = 8
)

type inputEcho struct {
	Handle string `json:"handle"`
	Family string `json:"family,omitempty"`
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256"`
}

type candidateFact struct {
	Kind           string `json:"kind"`
	InputHandle    string `json:"input_handle"`
	RelatedHandle  string `json:"related_handle"`
	Subject        string `json:"subject"`
	Predicate      string `json:"predicate"`
	Value          string `json:"value"`
	InstanceID     string `json:"instance_id"`
	EvidenceSHA256 string `json:"evidence_sha256"`
}

type candidateSuccess struct {
	Profile           string          `json:"profile"`
	Family            string          `json:"family"`
	RequestID         string          `json:"request_id"`
	Status            string          `json:"status"`
	ScopeID           string          `json:"scope_id"`
	CompilationUnitID string          `json:"compilation_unit_id"`
	Target            target          `json:"target"`
	InputEchoes       []inputEcho     `json:"input_echoes"`
	Facts             []candidateFact `json:"facts"`
}

type candidateRejected struct {
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

func decodeCandidateOutput(raw, sourceRequest []byte) (string, bool) {
	if len(raw) < 2 || len(raw) > maxInputBytes || raw[len(raw)-1] != '\n' || bytes.IndexByte(raw[:len(raw)-1], '\n') >= 0 || !utf8.Valid(raw) {
		return "", false
	}
	payload := raw[:len(raw)-1]
	if !boundedJSON(payload) {
		return "", false
	}
	source, ok := candidateSourceRequest(sourceRequest)
	if !ok {
		return "", false
	}
	object, ok := closedJSONObject(payload)
	if !ok {
		return "", false
	}
	status, ok := stringMember(object, "status")
	if !ok {
		return "", false
	}
	switch status {
	case "CANDIDATE":
		success, ok := decodeCandidateSuccess(object, source)
		if !ok || !canonicalCandidateFrame(raw, success) {
			return "", false
		}
		return "CANDIDATE", true
	case "REJECTED":
		rejected, ok := decodeCandidateRejected(object, source)
		if !ok || !canonicalCandidateFrame(raw, rejected) {
			return "", false
		}
		return "REJECTED_" + rejected.Reason, true
	default:
		return "", false
	}
}

func candidateSourceRequest(raw []byte) (request, bool) {
	var source request
	if stdjson.Unmarshal(raw, &source) != nil || source.Profile != profile || !frozenCandidateFamily(source.Family) || !candidateIdentifier(source.RequestID) ||
		!candidateIdentifier(source.ScopeID) || !candidateIdentifier(source.CompilationUnitID) || !validTarget(source.Target) || len(source.Inputs) > maxInputs {
		return request{}, false
	}
	seenHandles := make(map[string]struct{}, len(source.Inputs))
	seenPaths := make(map[string]struct{}, len(source.Inputs))
	for _, in := range source.Inputs {
		if !candidateIdentifier(in.Handle) || !logicalPath(in.Path) || !validDigest(in.SHA256) || in.Family == "" {
			return request{}, false
		}
		if _, duplicate := seenHandles[in.Handle]; duplicate {
			return request{}, false
		}
		if _, duplicate := seenPaths[in.Path]; duplicate {
			return request{}, false
		}
		seenHandles[in.Handle], seenPaths[in.Path] = struct{}{}, struct{}{}
	}
	return source, true
}

func decodeCandidateSuccess(object map[string]stdjson.RawMessage, source request) (candidateSuccess, bool) {
	if !exactFields(object, "profile", "family", "request_id", "status", "scope_id", "compilation_unit_id", "target", "input_echoes", "facts") {
		return candidateSuccess{}, false
	}
	result := candidateSuccess{
		Profile: profile, Family: source.Family, RequestID: source.RequestID, Status: "CANDIDATE",
		ScopeID: source.ScopeID, CompilationUnitID: source.CompilationUnitID, Target: source.Target,
	}
	if !exactString(object, "profile", result.Profile) || !exactString(object, "family", result.Family) ||
		!exactString(object, "request_id", result.RequestID) || !exactString(object, "status", result.Status) ||
		!exactString(object, "scope_id", result.ScopeID) || !exactString(object, "compilation_unit_id", result.CompilationUnitID) {
		return candidateSuccess{}, false
	}
	actualTarget, ok := decodeTarget(object["target"])
	if !ok || !sameTarget(actualTarget, source.Target) {
		return candidateSuccess{}, false
	}
	echoes, ok := decodeInputEchoes(object["input_echoes"], source.Family, source.Inputs)
	if !ok {
		return candidateSuccess{}, false
	}
	facts, ok := decodeCandidateFacts(object["facts"], source)
	if !ok {
		return candidateSuccess{}, false
	}
	result.InputEchoes, result.Facts = echoes, facts
	return result, true
}

func decodeCandidateRejected(object map[string]stdjson.RawMessage, source request) (candidateRejected, bool) {
	if !exactFields(object, "profile", "family", "request_id", "status", "scope_id", "compilation_unit_id", "target", "input_echoes", "reason") {
		return candidateRejected{}, false
	}
	result := candidateRejected{Profile: profile, Family: source.Family, RequestID: source.RequestID, Status: "REJECTED", ScopeID: source.ScopeID, CompilationUnitID: source.CompilationUnitID, Target: source.Target}
	if !exactString(object, "profile", result.Profile) || !exactString(object, "family", result.Family) ||
		!exactString(object, "request_id", result.RequestID) || !exactString(object, "status", result.Status) ||
		!exactString(object, "scope_id", result.ScopeID) || !exactString(object, "compilation_unit_id", result.CompilationUnitID) {
		return candidateRejected{}, false
	}
	actualTarget, ok := decodeTarget(object["target"])
	if !ok || !sameTarget(actualTarget, source.Target) {
		return candidateRejected{}, false
	}
	echoes, ok := decodeInputEchoes(object["input_echoes"], source.Family, source.Inputs)
	if !ok {
		return candidateRejected{}, false
	}
	reason, ok := stringMember(object, "reason")
	if !ok || !candidateRejectedReason(reason) {
		return candidateRejected{}, false
	}
	result.InputEchoes, result.Reason = echoes, reason
	return result, true
}

func decodeTarget(raw stdjson.RawMessage) (target, bool) {
	object, ok := closedJSONObject(raw)
	if !ok || !exactFields(object, "os", "architecture", "abi", "features") {
		return target{}, false
	}
	result := target{}
	var values [3]string
	for i, name := range []string{"os", "architecture", "abi"} {
		value, ok := stringMember(object, name)
		if !ok || !candidateIdentifier(value) {
			return target{}, false
		}
		values[i] = value
	}
	features, ok := stringArray(object["features"])
	if !ok || len(features) > maxCandidateFeatures || !strictIdentifiers(features) {
		return target{}, false
	}
	result.OS, result.Architecture, result.ABI, result.Features = values[0], values[1], values[2], features
	return result, true
}

// descriptorEchoFamily reports the candidate families whose frozen output
// echoes carry the four-field (handle, family, path, sha256) descriptor; every
// other family keeps the two-field (handle, sha256) echo.
func descriptorEchoFamily(family string) bool {
	return family == "javascript-typescript"
}

func decodeInputEchoes(raw stdjson.RawMessage, family string, inputs []input) ([]inputEcho, bool) {
	values, ok := rawArray(raw)
	if !ok || len(values) != len(inputs) {
		return nil, false
	}
	descriptor := descriptorEchoFamily(family)
	echoes := make([]inputEcho, len(values))
	for i, rawEcho := range values {
		object, ok := closedJSONObject(rawEcho)
		if !ok {
			return nil, false
		}
		if descriptor {
			if !exactFields(object, "handle", "family", "path", "sha256") || !exactString(object, "handle", inputs[i].Handle) ||
				!exactString(object, "family", inputs[i].Family) || !exactString(object, "path", inputs[i].Path) ||
				!exactString(object, "sha256", inputs[i].SHA256) {
				return nil, false
			}
			echoes[i] = inputEcho{Handle: inputs[i].Handle, Family: inputs[i].Family, Path: inputs[i].Path, SHA256: inputs[i].SHA256}
			continue
		}
		if !exactFields(object, "handle", "sha256") || !exactString(object, "handle", inputs[i].Handle) || !exactString(object, "sha256", inputs[i].SHA256) {
			return nil, false
		}
		echoes[i] = inputEcho{Handle: inputs[i].Handle, SHA256: inputs[i].SHA256}
	}
	return echoes, true
}

func decodeCandidateFacts(raw stdjson.RawMessage, source request) ([]candidateFact, bool) {
	values, ok := rawArray(raw)
	if !ok || len(values) > maxCandidateFacts {
		return nil, false
	}
	digests := make(map[string]string, len(source.Inputs))
	for _, in := range source.Inputs {
		digests[in.Handle] = in.SHA256
	}
	targetJSON, err := json.Marshal(source.Target)
	if err != nil {
		return nil, false
	}
	facts := make([]candidateFact, len(values))
	for i, rawFact := range values {
		object, ok := closedJSONObject(rawFact)
		if !ok || !exactFields(object, "kind", "input_handle", "related_handle", "subject", "predicate", "value", "instance_id", "evidence_sha256") {
			return nil, false
		}
		fact, ok := candidateFactFromObject(object)
		if !ok || !candidateIdentifier(fact.Kind) || !candidatePrintable(fact.Subject) || !candidatePrintable(fact.Predicate) ||
			!candidatePrintable(fact.Value) || !candidatePrintable(fact.InstanceID) || !validDigest(fact.EvidenceSHA256) {
			return nil, false
		}
		inputDigest, exists := digests[fact.InputHandle]
		if !exists {
			return nil, false
		}
		relatedDigest := "-"
		if fact.RelatedHandle != "-" {
			if fact.RelatedHandle == fact.InputHandle {
				return nil, false
			}
			var relatedExists bool
			relatedDigest, relatedExists = digests[fact.RelatedHandle]
			if !relatedExists {
				return nil, false
			}
		}
		expectedEvidence := candidateEvidenceDigest([]string{
			source.Family, source.RequestID, source.ScopeID, source.CompilationUnitID, string(targetJSON),
			fact.InputHandle, inputDigest, fact.RelatedHandle, relatedDigest, fact.Kind,
			fact.Subject, fact.Predicate, fact.Value, fact.InstanceID,
		})
		if fact.EvidenceSHA256 != expectedEvidence || (i > 0 && compareCandidateFacts(facts[i-1], fact) >= 0) {
			return nil, false
		}
		facts[i] = fact
	}
	return facts, true
}

func candidateFactFromObject(object map[string]stdjson.RawMessage) (candidateFact, bool) {
	var result candidateFact
	for _, field := range []struct {
		name        string
		destination *string
	}{
		{"kind", &result.Kind}, {"input_handle", &result.InputHandle}, {"related_handle", &result.RelatedHandle},
		{"subject", &result.Subject}, {"predicate", &result.Predicate}, {"value", &result.Value},
		{"instance_id", &result.InstanceID}, {"evidence_sha256", &result.EvidenceSHA256},
	} {
		value, ok := stringMember(object, field.name)
		if !ok {
			return candidateFact{}, false
		}
		*field.destination = value
	}
	return result, true
}

func canonicalCandidateFrame(raw []byte, value any) bool {
	encoded, err := json.Marshal(value)
	if err != nil {
		return false
	}
	encoded = append(encoded, '\n')
	return bytes.Equal(raw, encoded)
}

func boundedJSON(raw []byte) bool {
	decoder := stdjson.NewDecoder(bytes.NewReader(raw))
	depth, tokens := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
		tokens++
		if tokens > maxCandidateJSONTokens {
			return false
		}
		if delimiter, ok := token.(stdjson.Delim); ok {
			switch delimiter {
			case '{', '[':
				depth++
				if depth > maxCandidateJSONDepth {
					return false
				}
			case '}', ']':
				depth--
				if depth < 0 {
					return false
				}
			}
		}
	}
	return depth == 0
}

func closedJSONObject(raw []byte) (map[string]stdjson.RawMessage, bool) {
	decoder := stdjson.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != stdjson.Delim('{') {
		return nil, false
	}
	object := make(map[string]stdjson.RawMessage)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, false
		}
		name, ok := key.(string)
		if !ok {
			return nil, false
		}
		if _, exists := object[name]; exists {
			return nil, false
		}
		var value stdjson.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, false
		}
		object[name] = value
	}
	last, err := decoder.Token()
	if err != nil || last != stdjson.Delim('}') || decoder.More() {
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	return object, true
}

func rawArray(raw []byte) ([]stdjson.RawMessage, bool) {
	decoder := stdjson.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != stdjson.Delim('[') {
		return nil, false
	}
	var values []stdjson.RawMessage
	for decoder.More() {
		var value stdjson.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, false
		}
		values = append(values, value)
	}
	last, err := decoder.Token()
	if err != nil || last != stdjson.Delim(']') || decoder.More() {
		return nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	return values, true
}

func stringArray(raw stdjson.RawMessage) ([]string, bool) {
	values, ok := rawArray(raw)
	if !ok {
		return nil, false
	}
	result := make([]string, len(values))
	for i, rawValue := range values {
		if err := stdjson.Unmarshal(rawValue, &result[i]); err != nil {
			return nil, false
		}
	}
	return result, true
}

func exactFields(object map[string]stdjson.RawMessage, names ...string) bool {
	if len(object) != len(names) {
		return false
	}
	for _, name := range names {
		if _, exists := object[name]; !exists {
			return false
		}
	}
	return true
}

func stringMember(object map[string]stdjson.RawMessage, name string) (string, bool) {
	raw, ok := object[name]
	if !ok {
		return "", false
	}
	var value string
	if stdjson.Unmarshal(raw, &value) != nil || len(value) > maxCandidateStringBytes {
		return "", false
	}
	return value, true
}

func exactString(object map[string]stdjson.RawMessage, name, want string) bool {
	got, ok := stringMember(object, name)
	return ok && got == want
}

func validTarget(value target) bool {
	return candidateIdentifier(value.OS) && candidateIdentifier(value.Architecture) && candidateIdentifier(value.ABI) &&
		len(value.Features) <= maxCandidateFeatures && strictIdentifiers(value.Features)
}

func sameTarget(left, right target) bool {
	if left.OS != right.OS || left.Architecture != right.Architecture || left.ABI != right.ABI || len(left.Features) != len(right.Features) {
		return false
	}
	for i := range left.Features {
		if left.Features[i] != right.Features[i] {
			return false
		}
	}
	return true
}

func strictIdentifiers(values []string) bool {
	for i, value := range values {
		if !candidateIdentifier(value) || (i > 0 && values[i-1] >= value) {
			return false
		}
	}
	return true
}

func candidateIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i := range value {
		c := value[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || (i > 0 && strings.ContainsRune("._:@+~-", rune(c)))) {
			return false
		}
	}
	return true
}

func candidatePrintable(value string) bool {
	if len(value) == 0 || len(value) > maxCandidateStringBytes {
		return false
	}
	for i := range value {
		if value[i] < 0x20 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[len("sha256:"):] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func candidateRejectedReason(value string) bool {
	switch value {
	case "NONCANONICAL_REQUEST", "INVALID_IDENTIFIER", "INVALID_PATH", "DIGEST_MISMATCH", "UNKNOWN_FAMILY", "UNKNOWN_FIELD", "DUPLICATE_VALUE", "CONFLICTING_VALUE", "MALFORMED_INPUT", "UNSUPPORTED_SCHEMA", "EXACT_BINDING_UNAVAILABLE", "AMBIGUOUS_BINDING", "DYNAMIC_INPUT", "CREDENTIAL_INPUT", "LIMIT_EXCEEDED", "OUTPUT_LIMIT", "ANALYZER_FAILURE":
		return true
	default:
		return false
	}
}

func compareCandidateFacts(left, right candidateFact) int {
	for _, values := range [][2]string{{left.Kind, right.Kind}, {left.InputHandle, right.InputHandle}, {left.RelatedHandle, right.RelatedHandle}, {left.Subject, right.Subject}, {left.Predicate, right.Predicate}, {left.Value, right.Value}, {left.InstanceID, right.InstanceID}, {left.EvidenceSHA256, right.EvidenceSHA256}} {
		if values[0] < values[1] {
			return -1
		}
		if values[0] > values[1] {
			return 1
		}
	}
	return 0
}

func candidateEvidenceDigest(fields []string) string {
	if len(fields) != 14 {
		return ""
	}
	hasher := sha256.New()
	_, _ = hasher.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(fields)))
	_, _ = hasher.Write(size[:])
	for _, field := range fields {
		if uint64(len(field)) > uint64(^uint32(0)) {
			return ""
		}
		binary.BigEndian.PutUint32(size[:], uint32(len(field)))
		_, _ = hasher.Write(size[:])
		_, _ = hasher.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
