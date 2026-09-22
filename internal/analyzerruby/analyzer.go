// Package analyzerruby is an isolated, static-only Ruby fact candidate.
//
// It implements the closed Ruby family of
// docs/specs/analyzer-candidate-profiles.md: the `ruby.version`,
// `ruby.gemfile`, `ruby.gemfile-lock`, and `ruby.source` input records and the
// eight `ruby.*` fact kinds bound to them. It reads no path, consults no
// installed Ruby, Bundler, gem index, or network, and executes nothing. The
// request frame handed to AnalyzeCanonical is the sole input.
package analyzerruby

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
	Profile                 = "corvint-analyzer-candidate/experimental"
	Family                  = "ruby"
	MaxRequestBytes         = 1_500_000
	MaxInputBytes           = 1 << 20
	MaxOutputBytes          = 1 << 20
	MaxInputs               = 128
	MaxFeatures             = 64
	MaxAggregateBase64Bytes = 1_398_104
	MaxPathBytes            = 4096
	MaxPathSegmentBytes     = 128
	MaxJSONDepth            = 8
	MaxJSONTokens           = 4096
	MaxFactFieldBytes       = 4096
	MaxFacts                = 4096
	MaxFactsPerInput        = 4096
	MaxSourceBytes          = 65_536
	MaxBytesPerOp           = 35_000
	MaxAllocsPerOp          = 1_000
)

// The single accepted gem source. The Gemfile spells it without a trailing
// slash and the lock's `remote:` row spells it with one; `ruby.source.identity`
// digests the trailing-slash spelling whichever record witnessed it.
const (
	gemfileRemote = "https://rubygems.org"
	lockRemote    = "https://rubygems.org/"
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

var id = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@+~-]{0,127}$`)
var digest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// decoded pairs one validated request input with its decoded bytes.
type decoded struct {
	in   Input
	body []byte
}

// AnalyzeCanonical never reads paths, executes Ruby, or resolves gems; raw is
// the sole input.
func AnalyzeCanonical(raw []byte) []byte {
	if len(raw) == 0 || overBound(len(raw), MaxRequestBytes) || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte("\n")) != 1 || !utf8.Valid(raw) {
		return sentinel("NONCANONICAL_REQUEST")
	}
	r, why := envelope(raw[:len(raw)-1])
	if why == "UNKNOWN_FIELD" {
		return bound(r, why)
	}
	if why != "" {
		return sentinel(why)
	}
	if why := exactTarget(r.Target); why != "" {
		return bound(r, why)
	}
	inputs, why := decodeInputs(r)
	if why != "" {
		return bound(r, why)
	}
	collector := newFactCollector(r)
	if why := analyze(r, inputs, collector); why != "" {
		return bound(r, why)
	}
	facts := collector.facts
	sort.Slice(facts, func(i, j int) bool { return factLess(facts[i], facts[j]) })
	e := echoes(r)
	out, _ := json.Marshal(success{Profile, Family, r.RequestID, "CANDIDATE", r.ScopeID, r.CompilationUnitID, r.Target, e, facts})
	if len(out)+1 != collector.outputSize || overBound(len(out)+1, MaxOutputBytes) {
		return bound(r, "OUTPUT_LIMIT")
	}
	return append(out, '\n')
}

// decodeInputs decodes and digest-checks every input before any grammar runs,
// so a request whose bytes do not match their declared digest never reaches a
// parser.
func decodeInputs(r Request) ([]decoded, string) {
	inputs := make([]decoded, 0, len(r.Inputs))
	total := 0
	for _, in := range r.Inputs {
		size, exact := decodedLength(in.ContentBase64)
		if !exact {
			return nil, "MALFORMED_INPUT"
		}
		if size < 0 || overBound(size, MaxInputBytes-total) {
			return nil, "LIMIT_EXCEEDED"
		}
		b := make([]byte, size)
		n, e := base64.StdEncoding.Decode(b, []byte(in.ContentBase64))
		if e != nil || base64.StdEncoding.EncodeToString(b[:n]) != in.ContentBase64 {
			return nil, "MALFORMED_INPUT"
		}
		b = b[:n]
		if !digestMatches(b, in.SHA256) {
			return nil, "DIGEST_MISMATCH"
		}
		total += n
		inputs = append(inputs, decoded{in: in, body: b})
	}
	return inputs, ""
}

// digestMatches compares against a stack-allocated rendering so the per-input
// digest check costs no heap allocation.
func digestMatches(body []byte, declared string) bool {
	sum := sha256.Sum256(body)
	var encoded [71]byte
	copy(encoded[:], "sha256:")
	hex.Encode(encoded[7:], sum[:])
	return declared == string(encoded[:])
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

func overBound(value, bound int) bool { return value > bound }

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

// envelope returns a sentinel reason, or UNKNOWN_FIELD when the only defect is
// a safe canonical extension (decision 0222).
func envelope(body []byte) (r Request, why string) {
	if duplicateJSONName(body) {
		return r, "NONCANONICAL_REQUEST"
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		if json.Unmarshal(body, &Request{}) != nil {
			return r, "MALFORMED_INPUT"
		}
		if known, ok := withoutExtensions(body); ok && len(known) < len(body) {
			if r, why = envelope(known); why == "" {
				why = "UNKNOWN_FIELD"
			}
			return r, why
		}
		return r, "NONCANONICAL_REQUEST"
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(canonical, body) {
		return r, "NONCANONICAL_REQUEST"
	}
	if r.Profile != Profile {
		return r, "MALFORMED_INPUT"
	}
	if r.Family != Family {
		return r, "UNKNOWN_FAMILY"
	}
	if !id.MatchString(r.RequestID) {
		return r, "INVALID_IDENTIFIER"
	}
	return r, validPreEnvelope(&r)
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

func sentinel(reason string) []byte {
	b, _ := json.Marshal(failure{Profile: Profile, Family: "unknown", RequestID: "unknown", Status: "REJECTED", Reason: reason})
	return append(b, '\n')
}

func bound(r Request, reason string) []byte {
	t := r.Target
	b, _ := json.Marshal(failure{Profile, Family, r.RequestID, "REJECTED", r.ScopeID, r.CompilationUnitID, &t, echoes(r), reason})
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
	if !id.MatchString(r.ScopeID) || !id.MatchString(r.CompilationUnitID) || !id.MatchString(r.Target.OS) || !id.MatchString(r.Target.Architecture) || !id.MatchString(r.Target.ABI) {
		return "INVALID_IDENTIFIER"
	}
	if overBound(len(r.Target.Features), MaxFeatures) || overBound(len(r.Inputs), MaxInputs) {
		return "LIMIT_EXCEEDED"
	}
	for i, feature := range r.Target.Features {
		if !id.MatchString(feature) || i > 0 && r.Target.Features[i-1] >= feature {
			return "MALFORMED_INPUT"
		}
	}
	total := 0
	seenHandles := map[string]bool{}
	seenPaths := map[string]bool{}
	for i, in := range r.Inputs {
		if !id.MatchString(in.Handle) || !digest.MatchString(in.SHA256) || seenHandles[in.Handle] || seenPaths[in.Path] {
			return "MALFORMED_INPUT"
		}
		if !pathOK(in.Path) {
			return "INVALID_PATH"
		}
		if i > 0 && !inputLess(r.Inputs[i-1], in) {
			return "MALFORMED_INPUT"
		}
		seenHandles[in.Handle] = true
		seenPaths[in.Path] = true
		total += len(in.ContentBase64)
		if overBound(total, MaxAggregateBase64Bytes) {
			return "LIMIT_EXCEEDED"
		}
	}
	return ""
}

func inputLess(left, right Input) bool {
	for _, pair := range [][2]string{{left.Handle, right.Handle}, {left.Family, right.Family}, {left.Path, right.Path}, {left.SHA256, right.SHA256}} {
		if pair[0] == pair[1] {
			continue
		}
		return pair[0] < pair[1]
	}
	return false
}

// exactTarget accepts any well-formed coordinate. Ruby manifest and source
// facts are target-independent: the lock's PLATFORMS rows are a fact value,
// not a request coordinate, so narrowing OS/architecture here would reject
// requests the closed matrix does not narrow. `target.features` must be empty
// because the Ruby family declares no dialect feature; any feature names a
// schema this candidate does not implement.
func exactTarget(target Target) string {
	if len(target.Features) != 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	return ""
}

func pathOK(p string) bool {
	if p == "" || len(p) > MaxPathBytes || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." || len(s) > MaxPathSegmentBytes {
			return false
		}
		for _, b := range []byte(s) {
			if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._@+~-", rune(b)) {
				return false
			}
		}
	}
	return true
}

func factLess(left, right Fact) bool {
	for _, pair := range [][2]string{{left.Kind, right.Kind}, {left.InputHandle, right.InputHandle}, {left.RelatedHandle, right.RelatedHandle}, {left.Subject, right.Subject}, {left.Predicate, right.Predicate}, {left.Value, right.Value}, {left.InstanceID, right.InstanceID}, {left.EvidenceSHA256, right.EvidenceSHA256}} {
		if pair[0] == pair[1] {
			continue
		}
		return pair[0] < pair[1]
	}
	return false
}

func factJSONSize(f Fact) int {
	return len(`{"kind":"","input_handle":"","related_handle":"","subject":"","predicate":"","value":"","instance_id":"","evidence_sha256":""}`) + len(f.Kind) + len(f.InputHandle) + len(f.RelatedHandle) + len(f.Subject) + len(f.Predicate) + len(f.Value) + len(f.InstanceID) + len(f.EvidenceSHA256)
}

func emptySuccessSize(r Request) int {
	return 10 + len("profile") + 3 + len(Profile) + 2 + len("family") + 3 + len(Family) + 2 + len("request_id") + 3 + len(r.RequestID) + 2 + len("status") + 3 + len("CANDIDATE") + 2 + len("scope_id") + 3 + len(r.ScopeID) + 2 + len("compilation_unit_id") + 3 + len(r.CompilationUnitID) + 2 + len("target") + 3 + targetJSONSize(r.Target) + len("input_echoes") + 3 + echoJSONSize(r) + len("facts") + 5
}

func targetJSONSize(t Target) int {
	features := 2
	for i, f := range t.Features {
		if i > 0 {
			features++
		}
		features += len(f) + 2
	}
	return 5 + len("os") + 3 + len(t.OS) + 2 + len("architecture") + 3 + len(t.Architecture) + 2 + len("abi") + 3 + len(t.ABI) + 2 + len("features") + 3 + features
}

func echoJSONSize(r Request) int {
	size := 2
	for i, e := range echoes(r) {
		if i > 0 {
			size++
		}
		size += len(`{"handle":"","sha256":""}`) + len(e.Handle) + len(e.SHA256)
	}
	return size
}

const evidenceTag = "corvint-analyzer-candidate-evidence/experimental"

// evidenceState caches the per-request half of the evidence preimage. The
// canonical target JSON is identical for every fact in one request, so
// marshaling it per fact would dominate the ACP-009 allocation budget; the
// scratch buffer is reused across facts for the same reason.
type evidenceState struct {
	target string
	buf    []byte
}

func newEvidenceState(r Request) *evidenceState {
	target, _ := json.Marshal(r.Target)
	return &evidenceState{target: string(target)}
}

// evidence binds a fact to the exact request bytes that witnessed it. The
// preimage is the spec's 14-field frame; relatedDigest is "-" when the fact has
// a single input witness.
func (s *evidenceState) evidence(r Request, inputDigest, relatedHandle, relatedDigest string, f Fact) string {
	fields := [14]string{Family, r.RequestID, r.ScopeID, r.CompilationUnitID, s.target, f.InputHandle, inputDigest, relatedHandle, relatedDigest, f.Kind, f.Subject, f.Predicate, f.Value, f.InstanceID}
	size := len(evidenceTag) + 4
	for _, field := range fields {
		size += 4 + len(field)
	}
	if cap(s.buf) < size {
		s.buf = make([]byte, 0, size)
	}
	b := append(s.buf[:0], evidenceTag...)
	b = appendUint32(b, uint32(len(fields)))
	for _, field := range fields {
		b = appendUint32(b, uint32(len(field)))
		b = append(b, field...)
	}
	s.buf = b
	sum := sha256.Sum256(b)
	var encoded [71]byte
	copy(encoded[:], "sha256:")
	hex.Encode(encoded[7:], sum[:])
	return string(encoded[:])
}

func appendUint32(b []byte, n uint32) []byte {
	var frame [4]byte
	binary.BigEndian.PutUint32(frame[:], n)
	return append(b, frame[:]...)
}

// factCollector is shared by every input in one request. It charges each fact
// against the prospective output counter before retaining it, so the candidate
// never builds an output it must then discard.
type factCollector struct {
	r          Request
	facts      []Fact
	seen       map[Fact]struct{}
	perInput   map[string]int
	evidence   *evidenceState
	digest     string
	outputSize int
}

func newFactCollector(r Request) *factCollector {
	return &factCollector{r: r, facts: make([]Fact, 0, 16), seen: make(map[Fact]struct{}, 16), perInput: make(map[string]int, len(r.Inputs)), evidence: newEvidenceState(r), digest: sourceDigest(), outputSize: emptySuccessSize(r) + 1}
}

// add retains one fact bound to in, optionally correlated with related.
func (c *factCollector) add(in Input, related *Input, kind, subject, predicate, value, instance string) string {
	f := Fact{Kind: kind, InputHandle: in.Handle, RelatedHandle: "-", Subject: subject, Predicate: predicate, Value: value, InstanceID: instance}
	relatedHandle, relatedDigest := "-", "-"
	if related != nil {
		f.RelatedHandle = related.Handle
		relatedHandle, relatedDigest = related.Handle, related.SHA256
	}
	if !factFieldsOK(f, false) {
		return "LIMIT_EXCEEDED"
	}
	f.EvidenceSHA256 = c.evidence.evidence(c.r, in.SHA256, relatedHandle, relatedDigest, f)
	if !factFieldsOK(f, true) {
		return "LIMIT_EXCEEDED"
	}
	if _, ok := c.seen[f]; ok {
		return "DUPLICATE_VALUE"
	}
	if overBound(len(c.facts)+1, MaxFacts) || overBound(c.perInput[in.Handle]+1, MaxFactsPerInput) {
		return "LIMIT_EXCEEDED"
	}
	next := c.outputSize + factJSONSize(f)
	if len(c.facts) != 0 {
		next++
	}
	if overBound(next, MaxOutputBytes) {
		return "OUTPUT_LIMIT"
	}
	c.facts = append(c.facts, f)
	c.seen[f] = struct{}{}
	c.perInput[in.Handle]++
	c.outputSize = next
	return ""
}

func factFieldsOK(f Fact, includeEvidence bool) bool {
	if !factFieldOK(f.Kind) || !factFieldOK(f.InputHandle) || !factFieldOK(f.RelatedHandle) || !factFieldOK(f.Subject) || !factFieldOK(f.Predicate) || !factFieldOK(f.Value) || !factFieldOK(f.InstanceID) {
		return false
	}
	return !includeEvidence || factFieldOK(f.EvidenceSHA256)
}

// factFieldOK enforces the envelope's printable-ASCII fact-field rule.
func factFieldOK(field string) bool {
	if len(field) == 0 || overBound(len(field), MaxFactFieldBytes) {
		return false
	}
	for i := 0; i < len(field); i++ {
		if field[i] < 0x20 || field[i] > 0x7e {
			return false
		}
	}
	return true
}
