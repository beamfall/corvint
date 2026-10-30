// Package analyzershell is an isolated, static-only shell fact candidate.
package analyzershell

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
	Family                  = "shell"
	MaxRequestBytes         = 1_500_000
	MaxInputBytes           = 1 << 20
	MaxOutputBytes          = 1 << 20
	MaxInputs               = 128
	MaxFeatures             = 64
	MaxAggregateBase64Bytes = 1_398_104
	MaxIdentifierBytes      = 128
	MaxPathBytes            = 4096
	MaxPathSegmentBytes     = 128
	MaxJSONDepth            = 8
	MaxJSONTokens           = 4096
	MaxFactFieldBytes       = 4096
	MaxFacts                = 3000
	MaxSourceBytes          = 65_536
	MaxBinaryBytes          = 6_291_456
	MaxBytesPerOp           = 130_000
	MaxAllocsPerOp          = 900
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

// AnalyzeCanonical never reads paths or executes shell; raw is the sole input.
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
	decodedTotal := 0
	collector := newFactCollector(r)
	for _, in := range r.Inputs {
		decodedSize, exact := decodedLength(in.ContentBase64)
		if !exact {
			return bound(r, "MALFORMED_INPUT")
		}
		if decodedSize < 0 || overBound(decodedSize, MaxInputBytes-decodedTotal) {
			return bound(r, "LIMIT_EXCEEDED")
		}
		b := make([]byte, decodedSize)
		n, e := base64.StdEncoding.Decode(b, []byte(in.ContentBase64))
		if e != nil || base64.StdEncoding.EncodeToString(b[:n]) != in.ContentBase64 {
			return bound(r, "MALFORMED_INPUT")
		}
		b = b[:n]
		s := sha256.Sum256(b)
		if in.SHA256 != "sha256:"+hex.EncodeToString(s[:]) {
			return bound(r, "DIGEST_MISMATCH")
		}
		decodedTotal += n
		why := parseInto(in, b, r, collector)
		if why != "" {
			return bound(r, why)
		}
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
		if !id.MatchString(in.Handle) || !digest.MatchString(in.SHA256) || !pathOK(in.Path) || seenHandles[in.Handle] || seenPaths[in.Path] {
			return "MALFORMED_INPUT"
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
func exactTarget(target Target) string {
	if target.ABI != "none" {
		return "UNSUPPORTED_SCHEMA"
	}
	if !((target.OS == "darwin" && target.Architecture == "arm64") || (target.OS == "linux" && target.Architecture == "amd64")) {
		return "UNSUPPORTED_SCHEMA"
	}
	if len(target.Features) == 0 || len(target.Features) == 1 && target.Features[0] == "bash-5.2" {
		return ""
	}
	return "UNSUPPORTED_SCHEMA"
}
func pathOK(p string) bool {
	if p == "" || len(p) > MaxPathBytes || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "*?[]~") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." || len(s) > MaxPathSegmentBytes {
			return false
		}
		for _, b := range []byte(s) {
			if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._@+-", rune(b)) {
				return false
			}
		}
	}
	return true
}
func factKey(f Fact) string {
	return f.Kind + "\x00" + f.InputHandle + "\x00" + f.RelatedHandle + "\x00" + f.Subject + "\x00" + f.Predicate + "\x00" + f.Value + "\x00" + f.InstanceID + "\x00" + f.EvidenceSHA256
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
func evidence(r Request, in Input, f Fact) string {
	target, _ := json.Marshal(r.Target)
	fields := []string{Family, r.RequestID, r.ScopeID, r.CompilationUnitID, string(target), f.InputHandle, in.SHA256, "-", "-", f.Kind, f.Subject, f.Predicate, f.Value, f.InstanceID}
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
