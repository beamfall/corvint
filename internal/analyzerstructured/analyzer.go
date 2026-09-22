package analyzerstructured

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"
)

// Analyze accepts precisely one canonical LF-framed request and emits one
// canonical LF-framed candidate or rejection. It never reads ambient state.
func Analyze(frame []byte) []byte {
	request, reason := decodeRequest(frame)
	if reason != "" {
		return sentinel(reason)
	}
	if reason = echoEnvelope(request); reason != "" {
		return sentinel(reason)
	}
	echoes := requestEchoes(request.Inputs)
	if reason = validateEnvelope(request); reason != "" {
		return reject(request, echoes, reason)
	}
	inputs, reason := decodeInputs(request)
	if reason != "" {
		return reject(request, echoes, reason)
	}
	facts := make([]Fact, 0, len(inputs))
	for _, in := range inputs {
		fact, parseReason := parseFormat(in)
		if parseReason != "" {
			return reject(request, echoes, parseReason)
		}
		if len(facts) == maxFacts {
			return reject(request, echoes, "LIMIT_EXCEEDED")
		}
		facts = append(facts, fact)
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].InputHandle < facts[j].InputHandle })
	if prospectiveOutput(request, echoes, facts) > maxOutputBytes {
		return reject(request, echoes, "OUTPUT_LIMIT")
	}
	out, err := json.Marshal(success{Profile, Family, request.RequestID, "CANDIDATE", request.ScopeID, request.CompilationUnitID, request.Target, echoes, facts})
	if err != nil || len(out)+1 > maxOutputBytes {
		return reject(request, echoes, "OUTPUT_LIMIT")
	}
	return append(out, '\n')
}

func decodeRequest(frame []byte) (Request, string) {
	if len(frame) > maxFrameBytes {
		return Request{}, "LIMIT_EXCEEDED"
	}
	if len(frame) < 2 || frame[len(frame)-1] != '\n' || bytes.IndexByte(frame[:len(frame)-1], '\n') >= 0 || !utf8.Valid(frame[:len(frame)-1]) {
		return Request{}, "NONCANONICAL_REQUEST"
	}
	raw := frame[:len(frame)-1]
	if !jsonEnvelope(raw) {
		return Request{}, "NONCANONICAL_REQUEST"
	}
	var request Request
	if json.Unmarshal(raw, &request) != nil {
		return Request{}, "NONCANONICAL_REQUEST"
	}
	encoded, err := json.Marshal(request)
	if err != nil || !bytes.Equal(raw, encoded) {
		return Request{}, "NONCANONICAL_REQUEST"
	}
	return request, ""
}

// echoEnvelope admits a request to a bound rejection only once every echoed
// value (family, identities, target, input handles and digests) has passed its
// closed grammar and duplicate checks; anything else keeps the fixed sentinel.
func echoEnvelope(request Request) string {
	if request.Family != Family {
		return "UNKNOWN_FAMILY"
	}
	if !identifier(request.RequestID) || !identifier(request.ScopeID) || !identifier(request.CompilationUnitID) || !identifier(request.Target.OS) || !identifier(request.Target.Architecture) || !identifier(request.Target.ABI) {
		return "INVALID_IDENTIFIER"
	}
	if request.Target.Features == nil {
		return "NONCANONICAL_REQUEST"
	}
	for i, feature := range request.Target.Features {
		if !identifier(feature) {
			return "INVALID_IDENTIFIER"
		}
		if i > 0 && request.Target.Features[i-1] >= feature {
			return "DUPLICATE_VALUE"
		}
	}
	handles := make(map[string]struct{}, len(request.Inputs))
	paths := make(map[string]struct{}, len(request.Inputs))
	for i, in := range request.Inputs {
		if !identifier(in.Handle) || !logicalPath(in.Path) {
			return "INVALID_IDENTIFIER"
		}
		if !digest(in.SHA256) {
			return "DIGEST_MISMATCH"
		}
		if i > 0 && compareInput(request.Inputs[i-1], in) >= 0 {
			return "DUPLICATE_VALUE"
		}
		if _, duplicate := handles[in.Handle]; duplicate {
			return "DUPLICATE_VALUE"
		}
		if _, duplicate := paths[in.Path]; duplicate {
			return "DUPLICATE_VALUE"
		}
		handles[in.Handle] = struct{}{}
		paths[in.Path] = struct{}{}
	}
	return ""
}

// validateEnvelope runs after echoEnvelope, so each failure it reports binds.
func validateEnvelope(request Request) string {
	if len(request.Target.Features) != 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	if len(request.Inputs) == 0 || len(request.Inputs) > maxInputs {
		return "LIMIT_EXCEEDED"
	}
	for _, in := range request.Inputs {
		if !knownProfile(in.Family) {
			return "UNSUPPORTED_SCHEMA"
		}
		if len(in.ContentBase64) > maxBase64Bytes {
			return "LIMIT_EXCEEDED"
		}
	}
	if request.Profile != Profile {
		return "NONCANONICAL_REQUEST"
	}
	return ""
}

func decodeInputs(request Request) ([]decodedInput, string) {
	total := 0
	decoded := make([]decodedInput, 0, len(request.Inputs))
	for _, in := range request.Inputs {
		body, err := base64.StdEncoding.DecodeString(in.ContentBase64)
		if err != nil || base64.StdEncoding.EncodeToString(body) != in.ContentBase64 {
			return nil, "MALFORMED_INPUT"
		}
		if len(body) > maxInputBytes || total > maxInputBytes-len(body) {
			return nil, "LIMIT_EXCEEDED"
		}
		total += len(body)
		sum := sha256.Sum256(body)
		if in.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			return nil, "DIGEST_MISMATCH"
		}
		decoded = append(decoded, decodedInput{in, body, sum})
	}
	return decoded, ""
}

func requestEchoes(inputs []Input) []Echo {
	out := make([]Echo, len(inputs))
	for i, in := range inputs {
		out[i] = Echo{in.Handle, in.SHA256}
	}
	return out
}
func sentinel(reason string) []byte {
	out, _ := json.Marshal(sentinelRejected{Profile, "unknown", "unknown", "REJECTED", reason})
	return append(out, '\n')
}
func reject(r Request, echoes []Echo, reason string) []byte {
	out, _ := json.Marshal(rejected{r.Profile, Family, r.RequestID, "REJECTED", r.ScopeID, r.CompilationUnitID, r.Target, echoes, reason})
	if len(out)+1 > maxOutputBytes {
		return sentinel("OUTPUT_LIMIT")
	}
	return append(out, '\n')
}

func knownProfile(p FormatProfile) bool {
	for _, profile := range formatProfiles {
		if p == profile {
			return true
		}
	}
	return false
}
func compareInput(left, right Input) int {
	if x := strings.Compare(left.Handle, right.Handle); x != 0 {
		return x
	}
	if x := strings.Compare(string(left.Family), string(right.Family)); x != 0 {
		return x
	}
	if x := strings.Compare(left.Path, right.Path); x != 0 {
		return x
	}
	return strings.Compare(left.SHA256, right.SHA256)
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
	if len(s) == 0 || len(s) > 4096 || strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") {
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

func prospectiveOutput(request Request, echoes []Echo, facts []Fact) int {
	encoded, err := json.Marshal(success{Profile, Family, request.RequestID, "CANDIDATE", request.ScopeID, request.CompilationUnitID, request.Target, echoes, facts})
	if err != nil {
		return maxOutputBytes + 1
	}
	return len(encoded) + 1
}
