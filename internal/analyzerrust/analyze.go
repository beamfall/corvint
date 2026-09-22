// Package analyzerrust is an isolated, static-only Rust fact candidate. It
// consumes exactly one canonical JSON/LF request frame of caller-owned bytes
// and emits exactly one canonical JSON/LF frame. It never opens a repository or
// ambient file, reads PATH/HOME/CWD or the environment, invokes cargo, rustc, a
// process, or a shell, uses a network or dynamic loader, or asserts that any
// input compiles.
//
// The family is a separately specified experimental family under
// docs/specs/analyzer-candidate-profiles.md; it reuses the generic transport
// profile, the generic eight-field fact shape, and the frozen fourteen-field
// evidence digest verbatim, so it alters no existing family bytes.
package analyzerrust

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	Profile                 = "corvint-analyzer-candidate/experimental"
	Family                  = "rust"
	CEMOCMState             = "UNSUPPORTED"
	MaxRequestBytes         = 1_500_000
	MaxInputBytes           = 1 << 20
	MaxOutputBytes          = 1 << 20
	MaxInputs               = 128
	MaxFeatures             = 64
	MaxAggregateBase64Bytes = 1_398_104
	MaxJSONDepth            = 8
	MaxJSONTokens           = 4096
	MaxFactFieldBytes       = 4096
	MaxFacts                = 4096
	MaxIdentifierBytes      = 128
	MaxPathBytes            = 4096
	MaxPathSegmentBytes     = 128
	MaxSourceBytes          = 262_144
	MaxManifestBytes        = 65_536
	MaxManifestRows         = 8_192
	MaxNestingDepth         = 64
	MaxRawStringHashes      = 255
	MaxSourceTokens         = 262_144
)

// inputPathSuffix pins each input family to the exact file identity it may
// carry, so a manifest grammar can never be applied to a file that merely
// decodes cleanly. Nothing is inferred from the path: the family selects the
// grammar and the path must then agree with it.
var inputPathSuffix = map[string]string{
	"rust.manifest":  "Cargo.toml",
	"rust.toolchain": "rust-toolchain.toml",
	"rust.lock":      "Cargo.lock",
	"rust.source":    ".rs",
}

// parseInto routes one decoded input to its closed grammar.
func parseInto(in Input, content []byte, _ Request, c *factCollector) string {
	suffix, known := inputPathSuffix[in.Family]
	if !known {
		return "UNSUPPORTED_SCHEMA"
	}
	if !strings.HasSuffix(in.Path, suffix) {
		return "UNSUPPORTED_SCHEMA"
	}
	if suffix != ".rs" && in.Path != suffix && !strings.HasSuffix(in.Path, "/"+suffix) {
		return "UNSUPPORTED_SCHEMA"
	}
	switch in.Family {
	case "rust.manifest":
		return parseManifest(in, content, c)
	case "rust.toolchain":
		return parseToolchain(in, content, c)
	case "rust.lock":
		// Declared but not implemented by this initial candidate, matching the
		// js.bun-lock-v1 and dotnet.packages-lock-v1 precedent. Cargo emits
		// multi-line dependency arrays that no closed grammar here reads, and
		// the behavioural reference never consulted a lockfile at all.
		return "UNSUPPORTED_SCHEMA"
	}
	return parseSource(in, content, c)
}

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

var id = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@+~-]{0,127}$`)
var digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// inputFamilies is the closed set of accepted input record families. An input
// whose family is outside this set rejects the whole request; nothing is
// inferred from the path suffix.
var inputFamilies = map[string]bool{
	"rust.manifest":  true,
	"rust.toolchain": true,
	"rust.lock":      true,
	"rust.source":    true,
}

// sentinelReasons is the closed set a pre-envelope sentinel may carry. Any
// other reason reduces to MALFORMED_INPUT rather than becoming a new string.
var sentinelReasons = map[string]bool{
	"NONCANONICAL_REQUEST": true, "MALFORMED_INPUT": true, "UNKNOWN_FIELD": true,
	"UNKNOWN_FAMILY": true, "INVALID_IDENTIFIER": true, "INVALID_PATH": true,
	"DUPLICATE_VALUE": true, "LIMIT_EXCEEDED": true,
}

// boundReasons is the closed set a request-bound rejection may carry.
var boundReasons = map[string]bool{
	"AMBIGUOUS_BINDING": true, "ANALYZER_FAILURE": true, "CONFLICTING_VALUE": true,
	"CREDENTIAL_INPUT": true, "DIGEST_MISMATCH": true, "DUPLICATE_VALUE": true,
	"DYNAMIC_INPUT": true, "EXACT_BINDING_UNAVAILABLE": true, "INVALID_IDENTIFIER": true,
	"INVALID_PATH": true, "LIMIT_EXCEEDED": true, "MALFORMED_INPUT": true,
	"OUTPUT_LIMIT": true, "UNSUPPORTED_SCHEMA": true,
}

// AnalyzeCanonical is pure: every byte it uses is reachable from raw.
func AnalyzeCanonical(raw []byte) []byte {
	if len(raw) == 0 || len(raw) > MaxRequestBytes || raw[len(raw)-1] != '\n' ||
		bytes.Count(raw, []byte("\n")) != 1 || !utf8.Valid(raw) {
		return sentinel("NONCANONICAL_REQUEST")
	}
	body := raw[:len(raw)-1]
	if duplicateJSONName(body) {
		return sentinel("NONCANONICAL_REQUEST")
	}
	var r Request
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&r); err != nil {
		if strings.HasPrefix(err.Error(), "json: unknown field ") {
			return sentinel("UNKNOWN_FIELD")
		}
		return sentinel("MALFORMED_INPUT")
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(canonical, body) {
		return sentinel("NONCANONICAL_REQUEST")
	}
	if r.Profile != Profile {
		return sentinel("MALFORMED_INPUT")
	}
	if r.Family != Family {
		return sentinel("UNKNOWN_FAMILY")
	}
	if !id.MatchString(r.RequestID) {
		return sentinel("INVALID_IDENTIFIER")
	}
	if why := validPreEnvelope(&r); why != "" {
		return sentinel(why)
	}
	if why := exactTarget(r.Target); why != "" {
		return bound(r, why)
	}
	return analyzeBound(r)
}

// analyzeBound runs only after every echoed field has passed its closed grammar
// and bounded size, so each rejection below may retain the full request binding.
func analyzeBound(r Request) []byte {
	collector, why := newFactCollector(r)
	if why != "" {
		return bound(r, why)
	}
	decodedTotal := 0
	for _, in := range r.Inputs {
		content, why := decodeInput(in, MaxInputBytes-decodedTotal)
		if why != "" {
			return bound(r, why)
		}
		decodedTotal += len(content)
		if why := parseInto(in, content, r, collector); why != "" {
			return bound(r, why)
		}
	}
	if why := collector.crossCheck(r); why != "" {
		return bound(r, why)
	}
	// An input that yields nothing — an empty file, a comments-only file — must
	// encode as the empty array the charge was computed from, not as null.
	facts := collector.facts
	if facts == nil {
		facts = []Fact{}
	}
	sort.Slice(facts, func(i, j int) bool { return factLess(facts[i], facts[j]) })
	for i := 1; i < len(facts); i++ {
		if !factLess(facts[i-1], facts[i]) {
			return bound(r, "DUPLICATE_VALUE")
		}
	}
	out, err := json.Marshal(success{Profile, Family, r.RequestID, "CANDIDATE",
		r.ScopeID, r.CompilationUnitID, r.Target, echoes(r), facts})
	if err != nil || len(out)+1 != collector.outputSize || len(out)+1 > MaxOutputBytes {
		return bound(r, "OUTPUT_LIMIT")
	}
	return append(out, '\n')
}

// decodeInput enforces strict base64, exact re-encoding, and the supplied
// digest before any byte of content is handed to a parser.
func decodeInput(in Input, remaining int) ([]byte, string) {
	size, exact := decodedLength(in.ContentBase64)
	if !exact {
		return nil, "MALFORMED_INPUT"
	}
	if size > remaining {
		return nil, "LIMIT_EXCEEDED"
	}
	content := make([]byte, size)
	n, err := base64.StdEncoding.Strict().Decode(content, []byte(in.ContentBase64))
	if err != nil || n != size || base64.StdEncoding.EncodeToString(content[:n]) != in.ContentBase64 {
		return nil, "MALFORMED_INPUT"
	}
	content = content[:n]
	sum := sha256.Sum256(content)
	if in.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
		return nil, "DIGEST_MISMATCH"
	}
	return content, ""
}

// decodedLength admits the empty string: a zero-byte source file is valid Rust
// and analyzes to zero facts rather than rejecting.
func decodedLength(encoded string) (int, bool) {
	if len(encoded)%4 != 0 {
		return 0, false
	}
	if encoded == "" {
		return 0, true
	}
	size := base64.StdEncoding.DecodedLen(len(encoded))
	if encoded[len(encoded)-1] == '=' {
		size--
	}
	if len(encoded) > 1 && encoded[len(encoded)-2] == '=' {
		size--
	}
	return size, size >= 0
}

func duplicateJSONName(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	tokens := 0
	var walk func(int) bool
	walk = func(depth int) bool {
		tokens++
		if depth > MaxJSONDepth || tokens > MaxJSONTokens {
			return true
		}
		token, err := d.Token()
		if err != nil {
			return true
		}
		delim, object := token.(json.Delim)
		if !object {
			return false
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return true
				}
				value, ok := key.(string)
				if !ok || seen[value] {
					return true
				}
				seen[value] = true
				if walk(depth + 1) {
					return true
				}
			}
			_, err := d.Token()
			return err != nil
		case '[':
			for d.More() {
				if walk(depth + 1) {
					return true
				}
			}
			_, err := d.Token()
			return err != nil
		}
		return true
	}
	if walk(0) {
		return true
	}
	_, err := d.Token()
	return err == nil
}

func sentinel(reason string) []byte {
	if !sentinelReasons[reason] {
		reason = "MALFORMED_INPUT"
	}
	b, _ := json.Marshal(failure{Profile: Profile, Family: "unknown",
		RequestID: "unknown", Status: "REJECTED", Reason: reason})
	return append(b, '\n')
}

func bound(r Request, reason string) []byte {
	if !boundReasons[reason] {
		reason = "ANALYZER_FAILURE"
	}
	t := r.Target
	b, err := json.Marshal(failure{Profile, Family, r.RequestID, "REJECTED",
		r.ScopeID, r.CompilationUnitID, &t, echoes(r), reason})
	if err != nil || len(b)+1 > MaxOutputBytes {
		return sentinel("LIMIT_EXCEEDED")
	}
	return append(b, '\n')
}

func echoes(r Request) []Echo {
	x := make([]Echo, len(r.Inputs))
	for i := range r.Inputs {
		x[i] = Echo{r.Inputs[i].Handle, r.Inputs[i].SHA256}
	}
	return x
}

func validPreEnvelope(r *Request) string {
	if r.Target.Features == nil || r.Inputs == nil {
		return "MALFORMED_INPUT"
	}
	if !id.MatchString(r.ScopeID) || !id.MatchString(r.CompilationUnitID) ||
		!id.MatchString(r.Target.OS) || !id.MatchString(r.Target.Architecture) ||
		!id.MatchString(r.Target.ABI) {
		return "INVALID_IDENTIFIER"
	}
	if len(r.Target.Features) > MaxFeatures {
		return "LIMIT_EXCEEDED"
	}
	if len(r.Inputs) == 0 || len(r.Inputs) > MaxInputs {
		return "LIMIT_EXCEEDED"
	}
	for i, feature := range r.Target.Features {
		if !id.MatchString(feature) {
			return "INVALID_IDENTIFIER"
		}
		if i > 0 && r.Target.Features[i-1] >= feature {
			return "DUPLICATE_VALUE"
		}
	}
	return validInputs(r.Inputs)
}

func validInputs(inputs []Input) string {
	total := 0
	seenHandles := map[string]bool{}
	seenPaths := map[string]bool{}
	for i, in := range inputs {
		if !id.MatchString(in.Handle) {
			return "INVALID_IDENTIFIER"
		}
		if !digest.MatchString(in.SHA256) {
			return "MALFORMED_INPUT"
		}
		if !pathOK(in.Path) {
			if credentialLike(in.Path) {
				return "INVALID_PATH"
			}
			return "INVALID_PATH"
		}
		if !inputFamilies[in.Family] {
			return "MALFORMED_INPUT"
		}
		if seenHandles[in.Handle] || seenPaths[in.Path] {
			return "DUPLICATE_VALUE"
		}
		if i > 0 && !inputLess(inputs[i-1], in) {
			return "DUPLICATE_VALUE"
		}
		seenHandles[in.Handle] = true
		seenPaths[in.Path] = true
		total += len(in.ContentBase64)
		if total > MaxAggregateBase64Bytes {
			return "LIMIT_EXCEEDED"
		}
	}
	return ""
}

func inputLess(left, right Input) bool {
	for _, pair := range [][2]string{{left.Handle, right.Handle}, {left.Family, right.Family},
		{left.Path, right.Path}, {left.SHA256, right.SHA256}} {
		if pair[0] == pair[1] {
			continue
		}
		return pair[0] < pair[1]
	}
	return false
}

// exactTarget accepts only the closed coordinate set. The extractor is purely
// lexical and resolves no cfg predicate, so the target binds the receipt rather
// than selecting source.
func exactTarget(target Target) string {
	if target.ABI != "none" || len(target.Features) != 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	switch {
	case target.OS == "darwin" && target.Architecture == "arm64":
		return ""
	case target.OS == "linux" && target.Architecture == "amd64":
		return ""
	}
	return "UNSUPPORTED_SCHEMA"
}

func pathOK(p string) bool {
	if p == "" || len(p) > MaxPathBytes || strings.HasPrefix(p, "/") ||
		strings.HasSuffix(p, "/") || strings.ContainsAny(p, ":\\%") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." || len(s) > MaxPathSegmentBytes {
			return false
		}
		for i := 0; i < len(s); i++ {
			c := s[i]
			if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
				strings.IndexByte("._@+~-", c) >= 0) {
				return false
			}
		}
	}
	return true
}

func credentialLike(value string) bool {
	return strings.Contains(value, "://") && strings.Contains(value, "@")
}

func factLess(left, right Fact) bool {
	for _, pair := range [][2]string{{left.Kind, right.Kind}, {left.InputHandle, right.InputHandle},
		{left.RelatedHandle, right.RelatedHandle}, {left.Subject, right.Subject},
		{left.Predicate, right.Predicate}, {left.Value, right.Value},
		{left.InstanceID, right.InstanceID}, {left.EvidenceSHA256, right.EvidenceSHA256}} {
		if pair[0] == pair[1] {
			continue
		}
		return pair[0] < pair[1]
	}
	return false
}

// evidence is the frozen fourteen-field generic candidate digest. Its field
// order, domain separator, and length framing are identical to the vector
// pinned by internal/analyzercap/candidate_profile_vectors_test.go.
func evidence(r Request, inputDigest string, f Fact) string {
	target, _ := json.Marshal(r.Target)
	fields := []string{Family, r.RequestID, r.ScopeID, r.CompilationUnitID, string(target),
		f.InputHandle, inputDigest, f.RelatedHandle, "-", f.Kind, f.Subject, f.Predicate,
		f.Value, f.InstanceID}
	h := sha256.New()
	h.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(len(fields)))
	h.Write(b[:])
	for _, v := range fields {
		binary.BigEndian.PutUint32(b[:], uint32(len(v)))
		h.Write(b[:])
		h.Write([]byte(v))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
