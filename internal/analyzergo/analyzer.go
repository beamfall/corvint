// Package analyzergo implements an unregistered experimental Go fact extractor.
package analyzergo

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/scanner"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	Profile         = "corvint-analyzer-candidate/experimental"
	Family          = "go"
	MaxInputBytes   = 1 << 20
	MaxRequestBytes = 1_500_000
	MaxInputs       = 128
	MaxFacts        = 4096
	MaxOutputBytes  = 1 << 20
	maxFeatures     = 64
	maxTokens       = 4096
	maxJSONDepth    = 8
)

var ErrMalformed = errors.New("analyzergo: malformed request")

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
type candidate struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	RequestID         string `json:"request_id"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            Target `json:"target"`
	InputEchoes       []Echo `json:"input_echoes"`
	Facts             []Fact `json:"facts"`
	outputBytes       int
}
type rejection struct {
	Profile   string `json:"profile"`
	Family    string `json:"family"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}

// boundRejection is the frozen post-envelope rejection shape. The complete
// canonical request identity is carried by its scope/unit/target and every
// input echo in request order; partial requests must never be trusted.
type boundRejection struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	RequestID         string `json:"request_id"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            Target `json:"target"`
	InputEchoes       []Echo `json:"input_echoes"`
	Reason            string `json:"reason"`
}

// Process accepts only a byte-identical canonical request and produces one
// canonical response plus LF. It has no filesystem, subprocess, or network path.
func Process(raw []byte) ([]byte, error) {
	return process(raw, analyze)
}

type analyzer func(Request) (candidate, string)

// process owns the public failure boundary. A parser defect must become the
// closed analyzer failure response rather than an unframed process crash.
func process(raw []byte, run analyzer) (out []byte, err error) {
	var r Request
	defer func() {
		if recover() != nil {
			out, err = rejectionFor(r, "ANALYZER_FAILURE"), nil
		}
	}()
	if len(raw) > MaxRequestBytes {
		return reject("unknown", "unknown", "LIMIT_EXCEEDED"), nil
	}
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || bytes.IndexByte(raw[:len(raw)-1], '\n') >= 0 {
		return rejectForRaw(raw, "NONCANONICAL_REQUEST"), nil
	}
	if reason := jsonPreflight(raw[:len(raw)-1]); reason == "UNKNOWN_FIELD" {
		return unknownField(raw[:len(raw)-1]), nil
	} else if reason != "" {
		return rejectForRaw(raw[:len(raw)-1], reason), nil
	}
	if err := json.Unmarshal(raw[:len(raw)-1], &r, json.RejectUnknownMembers(true)); err != nil {
		return rejectForRaw(raw[:len(raw)-1], "NONCANONICAL_REQUEST"), nil
	}
	canon, err := json.Marshal(r)
	if err != nil {
		return rejectForRaw(raw[:len(raw)-1], "NONCANONICAL_REQUEST"), nil
	}
	if string(canon) != string(raw[:len(raw)-1]) {
		return rejectionFor(r, "NONCANONICAL_REQUEST"), nil
	}
	if reason := validateRequest(r); reason != "" {
		return rejectionFor(r, reason), nil
	}
	total := 0
	for i := range r.Inputs {
		if decodedBase64Len(r.Inputs[i].ContentBase64) > MaxInputBytes-total {
			return rejectionFor(r, "LIMIT_EXCEEDED"), nil
		}
		b, reason := decodeInput(r.Inputs[i])
		if reason != "" {
			return rejectionFor(r, reason), nil
		}
		if total > MaxInputBytes-len(b) {
			return rejectionFor(r, "LIMIT_EXCEEDED"), nil
		}
		total += len(b)
		r.Inputs[i].ContentBase64 = string(b)
	}
	response, reason := run(r)
	if reason != "" {
		return rejectionFor(r, reason), nil
	}
	out, err = json.Marshal(response)
	if err != nil || len(out)+1 != response.outputBytes || len(out)+1 > MaxOutputBytes {
		return rejectionFor(r, "OUTPUT_LIMIT"), nil
	}
	return append(out, '\n'), nil
}
func rejectForRaw(_ []byte, reason string) []byte {
	return reject("unknown", "unknown", reason)
}

// unknownField binds UNKNOWN_FIELD only to an otherwise safe canonical
// extension; any other request with unknown members is a listed sentinel.
func unknownField(body []byte) []byte {
	var r Request
	known, ok := withoutExtensions(body)
	if !ok || json.Unmarshal(known, &r, json.RejectUnknownMembers(true)) != nil {
		return reject("unknown", "unknown", "NONCANONICAL_REQUEST")
	}
	if canon, err := json.Marshal(r); err != nil || !bytes.Equal(canon, known) {
		return reject("unknown", "unknown", "NONCANONICAL_REQUEST")
	}
	if reason := echoEnvelope(r); reason != "" {
		return reject("unknown", "unknown", reason)
	}
	return rejectionFor(r, "UNKNOWN_FIELD")
}

// extensionScan re-encodes a request token by token. An extension member is
// canonical only after its object's schema fields, in increasing name order,
// with integer numbers and increasing nested names.
type extensionScan struct {
	d      *jsontext.Decoder
	e      *jsontext.Encoder
	strip  [][2]int64
	encode bytes.Buffer
}

func withoutExtensions(body []byte) ([]byte, bool) {
	s := &extensionScan{d: jsontext.NewDecoder(bytes.NewReader(body))}
	s.e = jsontext.NewEncoder(&s.encode)
	if !s.object(requestRoot) || s.encode.String() != string(body)+"\n" {
		return nil, false
	}
	known := make([]byte, 0, len(body))
	at := int64(0)
	for _, span := range s.strip {
		known, at = append(known, body[at:span[0]]...), span[1]
	}
	return append(known, body[at:]...), true
}

func (s *extensionScan) next() (jsontext.Token, bool) {
	tok, err := s.d.ReadToken()
	return tok, err == nil && s.e.WriteToken(tok) == nil
}

func (s *extensionScan) object(kind requestObject) bool {
	if _, ok := s.next(); !ok {
		return false
	}
	last, extended := "", false
	for s.d.PeekKind() == '"' {
		start := s.d.InputOffset()
		token, _ := s.next()
		name := token.String()
		bit, key := objectField(kind, requestKey([]byte(name)))
		switch {
		case bit != 0 && extended:
			return false
		case bit == 0 && extended && name <= last:
			return false
		case bit == 0:
			extended, last = true, name
			if !s.value() {
				return false
			}
			s.strip = append(s.strip, [2]int64{start, s.d.InputOffset()})
		case key == keyTarget:
			if !s.object(requestTarget) {
				return false
			}
		case key == keyInputs:
			for s.next(); s.d.PeekKind() == '{'; {
				if !s.object(requestInput) {
					return false
				}
			}
			if _, ok := s.next(); !ok {
				return false
			}
		default:
			if !s.value() {
				return false
			}
		}
	}
	_, ok := s.next()
	return ok
}

func (s *extensionScan) value() bool {
	tok, ok := s.next()
	switch tok.Kind() {
	case '0':
		return ok && canonicalInteger(tok.String())
	case '{':
		last := ""
		for first := true; ok && s.d.PeekKind() == '"'; first = false {
			token, _ := s.next()
			name := token.String()
			ok = (first || name > last) && s.value()
			last = name
		}
	case '[':
		for ok && s.d.PeekKind() != ']' {
			ok = s.value()
		}
	default:
		return ok
	}
	_, end := s.next()
	return ok && end
}

func canonicalInteger(v string) bool {
	digits := strings.TrimPrefix(v, "-")
	return v == "0" || digits != "" && digits[0] != '0' && strings.Trim(digits, "0123456789") == ""
}
func rejectionFor(r Request, reason string) []byte {
	if echoEnvelope(r) == "" {
		echoes := make([]Echo, 0, len(r.Inputs))
		for _, in := range r.Inputs {
			echoes = append(echoes, Echo{Handle: in.Handle, SHA256: in.SHA256})
		}
		// A post-envelope response carries the supplied profile verbatim.  That
		// makes two otherwise-identical invalid profile envelopes distinct while
		// the pre-envelope sentinel remains intentionally unbound.
		b, _ := json.Marshal(boundRejection{r.Profile, r.Family, r.RequestID, "REJECTED", r.ScopeID, r.CompilationUnitID, r.Target, echoes, reason})
		return append(b, '\n')
	}
	return reject("unknown", "unknown", reason)
}
func envelopeFamily(s string) bool {
	return s == Family || s == "javascript-typescript" || s == "dotnet" || s == "ruby"
}

// echoEnvelope checks every value a bound rejection echoes. Until it passes,
// a rejection is the fixed sentinel.
func echoEnvelope(r Request) string {
	if !envelopeFamily(r.Family) {
		return "UNKNOWN_FAMILY"
	}
	if !identifier(r.RequestID) || !identifier(r.ScopeID) || !identifier(r.CompilationUnitID) || !identifier(r.Target.OS) || !identifier(r.Target.Architecture) || !identifier(r.Target.ABI) || r.Target.Features == nil {
		return "INVALID_IDENTIFIER"
	}
	features, handles := map[string]bool{}, map[string]bool{}
	for _, f := range r.Target.Features {
		if !identifier(f) {
			return "INVALID_IDENTIFIER"
		}
		if features[f] {
			return "DUPLICATE_VALUE"
		}
		features[f] = true
	}
	for _, in := range r.Inputs {
		if !identifier(in.Handle) {
			return "INVALID_IDENTIFIER"
		}
		if !digest(in.SHA256) {
			return "DIGEST_MISMATCH"
		}
		if handles[in.Handle] {
			return "DUPLICATE_VALUE"
		}
		handles[in.Handle] = true
	}
	return ""
}
func reject(family, id, reason string) []byte {
	b, _ := json.Marshal(rejection{Profile, family, id, "REJECTED", reason})
	return append(b, '\n')
}

// jsonPreflight is one bounded structural walk over the request envelope. It
// decodes member names at their exact schema paths and charges every string
// before json/v2 is allowed to retain a request field.
func jsonPreflight(b []byte) string {
	if !utf8.Valid(b) || len(b) == 0 {
		return "NONCANONICAL_REQUEST"
	}
	s := requestScanner{b: b}
	if reason := s.object(requestRoot); reason != "" || s.i != len(b) {
		if reason != "" {
			return reason
		}
		return "NONCANONICAL_REQUEST"
	}
	if s.noncanonical {
		return "NONCANONICAL_REQUEST"
	}
	if s.unknown {
		return "UNKNOWN_FIELD"
	}
	return ""
}

type requestObject uint8

const (
	requestRoot requestObject = iota
	requestTarget
	requestInput
)

const (
	keyUnknown = iota
	keyProfile
	keyFamily
	keyRequestID
	keyScopeID
	keyUnitID
	keyTarget
	keyInputs
	keyOS
	keyArchitecture
	keyABI
	keyFeatures
	keyHandle
	keyPath
	keySHA256
	keyContentBase64
)

type requestScanner struct {
	b                                    []byte
	i, depth, tokens, content, rejection int
	noncanonical, unknown                bool
}

func (s *requestScanner) object(kind requestObject) string {
	s.space()
	if s.i >= len(s.b) || s.b[s.i] != '{' {
		s.noncanonical = true
		return s.value()
	}
	if reason := s.delimiter('{'); reason != "" {
		return reason
	}
	var seen uint8
	expected := uint8(0)
	switch kind {
	case requestRoot:
		expected = 0x7f
	case requestTarget:
		expected = 0x0f
	case requestInput:
		expected = 0x1f
	}
	for {
		s.space()
		if s.i >= len(s.b) {
			return "NONCANONICAL_REQUEST"
		}
		if s.b[s.i] == '}' {
			s.i++
			s.depth--
			if seen != expected {
				s.noncanonical = true
			}
			return ""
		}
		key, escaped, reason := s.key()
		if reason != "" {
			return reason
		}
		s.space()
		if s.i >= len(s.b) || s.b[s.i] != ':' {
			return "NONCANONICAL_REQUEST"
		}
		s.i++
		bit, value := objectField(kind, key)
		if bit == 0 {
			s.unknown = true
			if reason = s.value(); reason != "" {
				return reason
			}
		} else {
			if escaped || seen&bit != 0 {
				s.noncanonical = true
			}
			seen |= bit
			if reason = s.field(kind, value); reason != "" {
				return reason
			}
		}
		s.space()
		if s.i >= len(s.b) {
			return "NONCANONICAL_REQUEST"
		}
		if s.b[s.i] == '}' {
			continue
		}
		if s.b[s.i] != ',' {
			return "NONCANONICAL_REQUEST"
		}
		s.i++
	}
}

func objectField(object requestObject, key int) (uint8, int) {
	switch object {
	case requestRoot:
		switch key {
		case keyProfile:
			return 1, keyProfile
		case keyFamily:
			return 2, keyFamily
		case keyRequestID:
			return 4, keyRequestID
		case keyScopeID:
			return 8, keyScopeID
		case keyUnitID:
			return 16, keyUnitID
		case keyTarget:
			return 32, keyTarget
		case keyInputs:
			return 64, keyInputs
		}
	case requestTarget:
		switch key {
		case keyOS:
			return 1, keyOS
		case keyArchitecture:
			return 2, keyArchitecture
		case keyABI:
			return 4, keyABI
		case keyFeatures:
			return 8, keyFeatures
		}
	case requestInput:
		switch key {
		case keyHandle:
			return 1, keyHandle
		case keyFamily:
			return 2, keyFamily
		case keyPath:
			return 4, keyPath
		case keySHA256:
			return 8, keySHA256
		case keyContentBase64:
			return 16, keyContentBase64
		}
	}
	return 0, keyUnknown
}

func (s *requestScanner) field(object requestObject, key int) string {
	s.space()
	switch key {
	case keyTarget:
		return s.object(requestTarget)
	case keyInputs:
		return s.inputs()
	case keyFeatures:
		return s.features()
	case keyContentBase64:
		return s.string(true)
	case keyHandle, keySHA256, keyOS, keyArchitecture, keyABI:
		return s.rejectionString()
	case keyFamily, keyRequestID, keyScopeID, keyUnitID:
		if object == requestRoot {
			return s.rejectionString()
		}
		return s.string(false)
	default:
		return s.string(false)
	}
}

func (s *requestScanner) inputs() string {
	s.space()
	if s.i >= len(s.b) || s.b[s.i] != '[' {
		s.noncanonical = true
		return s.value()
	}
	if reason := s.delimiter('['); reason != "" {
		return reason
	}
	count := 0
	for {
		s.space()
		if s.i >= len(s.b) {
			return "NONCANONICAL_REQUEST"
		}
		if s.b[s.i] == ']' {
			s.i++
			s.depth--
			return ""
		}
		count++
		if count > MaxInputs {
			return "LIMIT_EXCEEDED"
		}
		if reason := s.object(requestInput); reason != "" {
			return reason
		}
		s.space()
		if s.i >= len(s.b) {
			return "NONCANONICAL_REQUEST"
		}
		if s.b[s.i] == ']' {
			continue
		}
		if s.b[s.i] != ',' {
			return "NONCANONICAL_REQUEST"
		}
		s.i++
	}
}

func (s *requestScanner) features() string {
	s.space()
	if s.i >= len(s.b) || s.b[s.i] != '[' {
		s.noncanonical = true
		return s.value()
	}
	if reason := s.delimiter('['); reason != "" {
		return reason
	}
	count := 0
	for {
		s.space()
		if s.i >= len(s.b) {
			return "NONCANONICAL_REQUEST"
		}
		if s.b[s.i] == ']' {
			s.i++
			s.depth--
			return ""
		}
		count++
		if count > maxFeatures {
			return "LIMIT_EXCEEDED"
		}
		if reason := s.rejectionString(); reason != "" {
			return reason
		}
		s.space()
		if s.i >= len(s.b) {
			return "NONCANONICAL_REQUEST"
		}
		if s.b[s.i] == ']' {
			continue
		}
		if s.b[s.i] != ',' {
			return "NONCANONICAL_REQUEST"
		}
		s.i++
	}
}

func (s *requestScanner) value() string {
	s.space()
	if s.i >= len(s.b) {
		return "NONCANONICAL_REQUEST"
	}
	switch s.b[s.i] {
	case '{':
		if reason := s.delimiter('{'); reason != "" {
			return reason
		}
		for {
			s.space()
			if s.i >= len(s.b) {
				return "NONCANONICAL_REQUEST"
			}
			if s.b[s.i] == '}' {
				s.i++
				s.depth--
				return ""
			}
			if _, _, reason := s.key(); reason != "" {
				return reason
			}
			s.space()
			if s.i >= len(s.b) || s.b[s.i] != ':' {
				return "NONCANONICAL_REQUEST"
			}
			s.i++
			if reason := s.value(); reason != "" {
				return reason
			}
			s.space()
			if s.i >= len(s.b) || (s.b[s.i] != ',' && s.b[s.i] != '}') {
				return "NONCANONICAL_REQUEST"
			}
			if s.b[s.i] == '}' {
				continue
			}
			s.i++
		}
	case '[':
		if reason := s.delimiter('['); reason != "" {
			return reason
		}
		for {
			s.space()
			if s.i >= len(s.b) {
				return "NONCANONICAL_REQUEST"
			}
			if s.b[s.i] == ']' {
				s.i++
				s.depth--
				return ""
			}
			if reason := s.value(); reason != "" {
				return reason
			}
			s.space()
			if s.i >= len(s.b) || (s.b[s.i] != ',' && s.b[s.i] != ']') {
				return "NONCANONICAL_REQUEST"
			}
			if s.b[s.i] == ']' {
				continue
			}
			s.i++
		}
	case '"':
		return s.string(false)
	default:
		start := s.i
		for s.i < len(s.b) && !jsonDelimiter(s.b[s.i]) {
			s.i++
		}
		if s.i == start {
			return "NONCANONICAL_REQUEST"
		}
		return s.token()
	}
}

func (s *requestScanner) string(content bool) string {
	s.space()
	if s.i >= len(s.b) || s.b[s.i] != '"' {
		s.noncanonical = true
		return s.value()
	}
	start := s.i
	end, decoded, plain, ok := scanJSONString(s.b, start)
	if !ok {
		return "NONCANONICAL_REQUEST"
	}
	if reason := s.token(); reason != "" {
		return reason
	}
	if !content {
		if decoded > 4096 {
			return "LIMIT_EXCEEDED"
		}
		s.i = end + 1
		return ""
	}
	if decoded > 1_398_104 {
		return "LIMIT_EXCEEDED"
	}
	decodedContent := MaxInputBytes + 1
	if plain {
		decodedContent = decodedBase64Bytes(s.b[start+1 : end])
	}
	if decodedContent > MaxInputBytes-s.content {
		return "LIMIT_EXCEEDED"
	}
	s.content += decodedContent
	s.i = end + 1
	return ""
}

// rejectionString reserves the worst-case canonical JSON expansion of each
// field echoed by a bound rejection before json/v2 retains the request. The
// fixed reservation covers frame syntax, 128 echo objects, and field labels.
func (s *requestScanner) rejectionString() string {
	s.space()
	if s.i >= len(s.b) || s.b[s.i] != '"' {
		s.noncanonical = true
		return s.value()
	}
	start := s.i
	end, decoded, _, ok := scanJSONString(s.b, start)
	if !ok {
		return "NONCANONICAL_REQUEST"
	}
	if reason := s.token(); reason != "" {
		return reason
	}
	if decoded > 4096 {
		return "LIMIT_EXCEEDED"
	}
	const staticRejectionReservation = 32 << 10
	const perStringReservation = 64
	if decoded > (MaxOutputBytes-staticRejectionReservation-s.rejection-perStringReservation)/6 {
		return "LIMIT_EXCEEDED"
	}
	s.rejection += decoded*6 + perStringReservation
	s.i = end + 1
	return ""
}

func (s *requestScanner) key() (int, bool, string) {
	s.space()
	if s.i >= len(s.b) || s.b[s.i] != '"' {
		return keyUnknown, false, "NONCANONICAL_REQUEST"
	}
	start := s.i
	var key [32]byte
	n, escaped := 0, false
	for s.i++; s.i < len(s.b); s.i++ {
		c := s.b[s.i]
		if c == '"' {
			s.i++
			if reason := s.token(); reason != "" {
				return keyUnknown, false, reason
			}
			if n > 4096 {
				return keyUnknown, false, "LIMIT_EXCEEDED"
			}
			if n > len(key) {
				return keyUnknown, escaped, ""
			}
			return requestKey(key[:n]), escaped, ""
		}
		if c < 0x20 {
			return keyUnknown, false, "NONCANONICAL_REQUEST"
		}
		var r rune
		if c != '\\' {
			var width int
			r, width = utf8.DecodeRune(s.b[s.i:])
			if r == utf8.RuneError && width == 1 {
				return keyUnknown, false, "NONCANONICAL_REQUEST"
			}
			s.i += width - 1
		} else {
			escaped = true
			s.i++
			if s.i >= len(s.b) {
				return keyUnknown, false, "NONCANONICAL_REQUEST"
			}
			switch s.b[s.i] {
			case '"', '\\', '/':
				r = rune(s.b[s.i])
			case 'b':
				r = '\b'
			case 'f':
				r = '\f'
			case 'n':
				r = '\n'
			case 'r':
				r = '\r'
			case 't':
				r = '\t'
			case 'u':
				if s.i+4 >= len(s.b) {
					return keyUnknown, false, "NONCANONICAL_REQUEST"
				}
				var ok bool
				r, ok = hexRune(s.b[s.i+1 : s.i+5])
				if !ok {
					return keyUnknown, false, "NONCANONICAL_REQUEST"
				}
				s.i += 4
				if 0xD800 <= r && r <= 0xDBFF {
					if s.i+6 >= len(s.b) || s.b[s.i+1] != '\\' || s.b[s.i+2] != 'u' {
						return keyUnknown, false, "NONCANONICAL_REQUEST"
					}
					lo, ok := hexRune(s.b[s.i+3 : s.i+7])
					if !ok || lo < 0xDC00 || lo > 0xDFFF {
						return keyUnknown, false, "NONCANONICAL_REQUEST"
					}
					r = 0x10000 + (r-0xD800)*0x400 + lo - 0xDC00
					s.i += 6
				}
			default:
				return keyUnknown, false, "NONCANONICAL_REQUEST"
			}
		}
		width := utf8.RuneLen(r)
		if width < 0 {
			return keyUnknown, false, "NONCANONICAL_REQUEST"
		}
		if n+width <= len(key) {
			utf8.EncodeRune(key[n:], r)
		}
		n += width
	}
	_ = start
	return keyUnknown, false, "NONCANONICAL_REQUEST"
}

func requestKey(b []byte) int {
	switch {
	case bytes.Equal(b, []byte("profile")):
		return keyProfile
	case bytes.Equal(b, []byte("family")):
		return keyFamily
	case bytes.Equal(b, []byte("request_id")):
		return keyRequestID
	case bytes.Equal(b, []byte("scope_id")):
		return keyScopeID
	case bytes.Equal(b, []byte("compilation_unit_id")):
		return keyUnitID
	case bytes.Equal(b, []byte("target")):
		return keyTarget
	case bytes.Equal(b, []byte("inputs")):
		return keyInputs
	case bytes.Equal(b, []byte("os")):
		return keyOS
	case bytes.Equal(b, []byte("architecture")):
		return keyArchitecture
	case bytes.Equal(b, []byte("abi")):
		return keyABI
	case bytes.Equal(b, []byte("features")):
		return keyFeatures
	case bytes.Equal(b, []byte("handle")):
		return keyHandle
	case bytes.Equal(b, []byte("path")):
		return keyPath
	case bytes.Equal(b, []byte("sha256")):
		return keySHA256
	case bytes.Equal(b, []byte("content_base64")):
		return keyContentBase64
	default:
		return keyUnknown
	}
}

func (s *requestScanner) delimiter(c byte) string {
	if s.i >= len(s.b) || s.b[s.i] != c {
		return "NONCANONICAL_REQUEST"
	}
	s.i++
	s.depth++
	if reason := s.token(); reason != "" {
		return reason
	}
	if s.depth > maxJSONDepth {
		return "LIMIT_EXCEEDED"
	}
	return ""
}

func (s *requestScanner) token() string {
	s.tokens++
	if s.tokens > maxTokens {
		return "LIMIT_EXCEEDED"
	}
	return ""
}

func (s *requestScanner) space() {
	s.i = skipJSONSpace(s.b, s.i)
}

func decodedBase64Bytes(b []byte) int {
	if len(b) == 0 || len(b)%4 != 0 {
		return MaxInputBytes + 1
	}
	n := len(b) / 4 * 3
	if b[len(b)-1] == '=' {
		n--
		if b[len(b)-2] == '=' {
			n--
		}
	}
	return n
}
func skipJSONValue(b []byte, start int) (int, bool) {
	if start >= len(b) {
		return 0, false
	}
	if b[start] == '"' {
		end, _, _, ok := scanJSONString(b, start)
		return end + 1, ok
	}
	if b[start] != '{' && b[start] != '[' {
		i := start
		for i < len(b) && !jsonDelimiter(b[i]) {
			i++
		}
		return i, i > start
	}
	var closes [maxJSONDepth + 1]byte
	depth := 1
	if b[start] == '{' {
		closes[0] = '}'
	} else {
		closes[0] = ']'
	}
	for i := start + 1; i < len(b); i++ {
		if b[i] == '"' {
			end, _, _, ok := scanJSONString(b, i)
			if !ok {
				return 0, false
			}
			i = end
			continue
		}
		if b[i] == '{' || b[i] == '[' {
			if depth == len(closes) {
				return 0, false
			}
			if b[i] == '{' {
				closes[depth] = '}'
			} else {
				closes[depth] = ']'
			}
			depth++
		}
		if b[i] == '}' || b[i] == ']' {
			if b[i] != closes[depth-1] {
				return 0, false
			}
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}
func skipJSONSpace(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\r') {
		i++
	}
	return i
}
func jsonDelimiter(b byte) bool {
	return b == '{' || b == '}' || b == '[' || b == ']' || b == ',' || b == ':' || b == ' ' || b == '\t' || b == '\r'
}
func nextColon(b []byte, i int) bool {
	for i++; i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\r'); i++ {
	}
	return i < len(b) && b[i] == ':'
}
func scanJSONString(b []byte, start int) (int, int, bool, bool) {
	decoded, plain := 0, true
	for i := start + 1; i < len(b); i++ {
		if b[i] == '"' {
			return i, decoded, plain, true
		}
		if b[i] < 0x20 {
			return 0, 0, false, false
		}
		if b[i] != '\\' {
			r, n := utf8.DecodeRune(b[i:])
			if r == utf8.RuneError && n == 1 {
				return 0, 0, false, false
			}
			decoded += n
			i += n - 1
			continue
		}
		plain = false
		i++
		if i >= len(b) {
			return 0, 0, false, false
		}
		switch b[i] {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			decoded++
		case 'u':
			if i+4 >= len(b) {
				return 0, 0, false, false
			}
			r, ok := hexRune(b[i+1 : i+5])
			if !ok {
				return 0, 0, false, false
			}
			i += 4
			if 0xD800 <= r && r <= 0xDBFF {
				if i+6 >= len(b) || b[i+1] != '\\' || b[i+2] != 'u' {
					return 0, 0, false, false
				}
				lo, ok := hexRune(b[i+3 : i+7])
				if !ok || lo < 0xDC00 || lo > 0xDFFF {
					return 0, 0, false, false
				}
				r = 0x10000 + (r-0xD800)*0x400 + lo - 0xDC00
				i += 6
			}
			decoded += utf8.RuneLen(r)
		default:
			return 0, 0, false, false
		}
	}
	return 0, 0, false, false
}
func hexRune(b []byte) (rune, bool) {
	var n rune
	for _, c := range b {
		n <<= 4
		switch {
		case c >= '0' && c <= '9':
			n += rune(c - '0')
		case c >= 'a' && c <= 'f':
			n += rune(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			n += rune(c - 'A' + 10)
		default:
			return 0, false
		}
	}
	return n, true
}
func validateRequest(r Request) string {
	if r.Profile != Profile || r.Family != Family {
		return "UNKNOWN_FAMILY"
	}
	if reason := echoEnvelope(r); reason != "" {
		return reason
	}
	if r.Target.OS != "darwin" || r.Target.Architecture != "arm64" || r.Target.ABI != "none" {
		return "UNSUPPORTED_SCHEMA"
	}
	if len(r.Inputs) == 0 || len(r.Inputs) > MaxInputs {
		return "LIMIT_EXCEEDED"
	}
	if len(r.Target.Features) > maxFeatures {
		return "LIMIT_EXCEEDED"
	}
	if len(r.Target.Features) != 0 {
		return "UNSUPPORTED_SCHEMA"
	}
	paths := map[string]bool{}
	for i, in := range r.Inputs {
		if !logicalPath(in.Path) {
			return "INVALID_PATH"
		}
		if paths[in.Path] {
			return "DUPLICATE_VALUE"
		}
		paths[in.Path] = true
		if i > 0 && compareInput(r.Inputs[i-1], in) >= 0 {
			return "DUPLICATE_VALUE"
		}
		if in.Family != "go.mod" && in.Family != "go.sum" && in.Family != "go.source" && in.Family != "go.work" {
			return "UNSUPPORTED_SCHEMA"
		}
	}
	return ""
}
func decodeInput(in Input) ([]byte, string) {
	if in.ContentBase64 == "" || len(in.ContentBase64) > 1_398_104 {
		return nil, "LIMIT_EXCEEDED"
	}
	b, e := base64.StdEncoding.DecodeString(in.ContentBase64)
	if e != nil || base64.StdEncoding.EncodeToString(b) != in.ContentBase64 {
		return nil, "MALFORMED_INPUT"
	}
	if len(b) > MaxInputBytes {
		return nil, "LIMIT_EXCEEDED"
	}
	s := sha256.Sum256(b)
	if in.SHA256 != "sha256:"+hex.EncodeToString(s[:]) {
		return nil, "DIGEST_MISMATCH"
	}
	return b, ""
}
func decodedBase64Len(s string) int {
	if len(s) == 0 || len(s)%4 != 0 {
		return MaxInputBytes + 1
	}
	n := len(s) / 4 * 3
	if s[len(s)-1] == '=' {
		n--
		if s[len(s)-2] == '=' {
			n--
		}
	}
	return n
}

func analyze(r Request) (candidate, string) {
	var mod, sum, work *Input
	sources := []*Input{}
	for i := range r.Inputs {
		in := &r.Inputs[i]
		switch in.Family {
		case "go.mod":
			if mod != nil {
				return candidate{}, "AMBIGUOUS_BINDING"
			}
			mod = in
		case "go.sum":
			if sum != nil {
				return candidate{}, "AMBIGUOUS_BINDING"
			}
			sum = in
		case "go.work":
			if work != nil {
				return candidate{}, "AMBIGUOUS_BINDING"
			}
			work = in
		default:
			sources = append(sources, in)
		}
	}
	var module modInfo
	var workspace workInfo
	var locked map[requirement]sumWitness
	var reason string
	limits := &resourceLedger{}
	if mod != nil {
		module, reason = parseMod(mod.ContentBase64, limits)
		if reason != "" {
			return candidate{}, reason
		}
	}
	if sum != nil {
		locked, reason = parseSum(sum.ContentBase64)
		if reason != "" {
			return candidate{}, reason
		}
	}
	if work != nil {
		workspace, reason = parseWork(work.ContentBase64, limits)
		if reason != "" {
			return candidate{}, reason
		}
	}
	if mod != nil && work != nil && goLanguageRank(workspace.goVersion) < goLanguageRank(module.goVersion) {
		// A workspace may select a newer language version than one of its
		// members, but an older workspace declaration contradicts that member.
		return candidate{}, "CONFLICTING_VALUE"
	}
	if mod == nil && work == nil && len(sources) == 0 {
		return candidate{}, "UNSUPPORTED_SCHEMA"
	}
	if mod != nil && len(module.requires) > 0 && sum == nil {
		return candidate{}, "EXACT_BINDING_UNAVAILABLE"
	}
	for _, req := range module.requires {
		if w, ok := locked[req]; !ok || !w.archive {
			return candidate{}, "EXACT_BINDING_UNAVAILABLE"
		}
	}
	o := candidate{Profile: Profile, Family: Family, RequestID: r.RequestID, Status: "CANDIDATE", ScopeID: r.ScopeID, CompilationUnitID: r.CompilationUnitID, Target: r.Target, InputEchoes: make([]Echo, 0, len(r.Inputs)), Facts: make([]Fact, 0, 32)}
	evidence := evidenceState{target: `{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`, buf: make([]byte, 0, 512)}
	for _, in := range r.Inputs {
		o.InputEchoes = append(o.InputEchoes, Echo{in.Handle, in.SHA256})
	}
	o.outputBytes = candidateJSONSize(o) + 1
	if mod != nil {
		if reason = addOwned(&o, &evidence, r, mod, nil, "go.module", r.ScopeID, "declares-module", module.path, r.ScopeID); reason != "" {
			return candidate{}, reason
		}
		if reason = addOwned(&o, &evidence, r, mod, nil, "go.language.declaration", "go", "declares-language", module.goVersion, r.ScopeID); reason != "" {
			return candidate{}, reason
		}
		if module.toolchain != "" {
			if reason = addOwned(&o, &evidence, r, mod, nil, "go.toolchain.declaration", "go", "declares-toolchain", module.toolchain, r.ScopeID); reason != "" {
				return candidate{}, reason
			}
		}
		for _, q := range module.requires {
			if reason = addOwned(&o, &evidence, r, mod, sum, "go.dependency.locked", q.path, "locked-at", q.version, q.path+"@"+q.version); reason != "" {
				return candidate{}, reason
			}
		}
	}
	if work != nil {
		if reason = addOwned(&o, &evidence, r, work, nil, "go.language.declaration", "go", "declares-language", workspace.goVersion, r.ScopeID); reason != "" {
			return candidate{}, reason
		}
		if workspace.toolchain != "" {
			if reason = addOwned(&o, &evidence, r, work, nil, "go.toolchain.declaration", "go", "declares-toolchain", workspace.toolchain, r.ScopeID); reason != "" {
				return candidate{}, reason
			}
		}
		for _, use := range workspace.uses {
			if reason = addOwned(&o, &evidence, r, work, nil, "go.workspace.use", r.ScopeID, "uses", use, use); reason != "" {
				return candidate{}, reason
			}
		}
	}
	packageName := ""
	for _, source := range sources {
		pkg, imports, expr, gen, reason := parseSource(source.Path, source.ContentBase64, limits)
		if reason != "" {
			return candidate{}, reason
		}
		if packageName == "" {
			packageName = pkg
		} else if packageName != pkg {
			return candidate{}, "CONFLICTING_VALUE"
		}
		state := "source"
		if strings.HasSuffix(source.Path, "_test.go") {
			state = "test"
		}
		if gen {
			state = "generated"
		}
		if reason = addOwned(&o, &evidence, r, source, nil, "go.package", pkg, "declares-package", pkg, r.CompilationUnitID); reason != "" {
			return candidate{}, reason
		}
		if reason = addOwned(&o, &evidence, r, source, nil, "go.source", source.Path, "classifies", state, source.Path); reason != "" {
			return candidate{}, reason
		}
		if expr != "" {
			if reason = addOwned(&o, &evidence, r, source, nil, "go.build.constraint", source.Path, "selected-for", expr, r.CompilationUnitID); reason != "" {
				return candidate{}, reason
			}
		}
		for _, im := range imports {
			if reason = addOwned(&o, &evidence, r, source, nil, "go.import.static", source.Path, "imports", im, source.Path); reason != "" {
				return candidate{}, reason
			}
		}
	}
	// Typed insertion ordering is allocation-free and the hard fact cap keeps
	// its worst case bounded. Delimiter-concatenated sort keys are forbidden.
	for i := 1; i < len(o.Facts); i++ {
		value := o.Facts[i]
		j := i
		for j > 0 && compareFact(value, o.Facts[j-1]) < 0 {
			o.Facts[j] = o.Facts[j-1]
			j--
		}
		o.Facts[j] = value
	}
	for i := 1; i < len(o.Facts); i++ {
		if compareFact(o.Facts[i-1], o.Facts[i]) == 0 {
			return candidate{}, "DUPLICATE_VALUE"
		}
	}
	return o, ""
}

type requirement struct{ path, version string }
type modInfo struct {
	path, goVersion, toolchain string
	requires                   []requirement
}
type workInfo struct {
	goVersion, toolchain string
	uses                 []string
}
type sumWitness struct{ archive, mod bool }

// resourceLedger makes every retained family object charge one closed global
// budget before it can grow. Its source-byte charge bounds the whole-file AST
// peak; header scanning reserves facts and edges before the full parser runs.
type resourceLedger struct {
	sourceBytes, sources, packages, edges, facts int
}

func (l *resourceLedger) reserveSource(bytes, facts, edges int) string {
	if bytes < 0 || l.sourceBytes > MaxInputBytes-bytes ||
		l.sources >= MaxFacts || l.packages >= MaxFacts ||
		l.edges > MaxFacts-edges || l.facts > MaxFacts-facts {
		return "LIMIT_EXCEEDED"
	}
	l.sourceBytes += bytes
	l.sources++
	l.packages++
	l.edges += edges
	l.facts += facts
	return ""
}

func (l *resourceLedger) reserveFacts(facts, edges int) string {
	if facts < 0 || edges < 0 || l.facts > MaxFacts-facts || l.edges > MaxFacts-edges {
		return "LIMIT_EXCEEDED"
	}
	l.facts += facts
	l.edges += edges
	return ""
}

func parseMod(body string, limits *resourceLedger) (modInfo, string) {
	var out modInfo
	if !lfText(body) {
		return out, "MALFORMED_INPUT"
	}
	in, block, direct := false, false, false
	for rest := strings.TrimSuffix(body, "\n"); ; {
		line, next, more := strings.Cut(rest, "\n")
		if line == "require (" {
			if block || in || direct || out.path == "" || out.goVersion == "" {
				return out, "MALFORMED_INPUT"
			}
			block, in = true, true
		} else if line == ")" {
			if !block || !in {
				return out, "MALFORMED_INPUT"
			}
			in = false
		} else if strings.ContainsAny(line, "\t") || strings.Contains(line, "  ") || strings.Contains(line, "//") {
			return out, "MALFORMED_INPUT"
		} else {
			one, rest, hasSecond := strings.Cut(line, " ")
			two, three, hasThird := strings.Cut(rest, " ")
			if !in && (one == "replace" || one == "exclude" || one == "retract") {
				return out, "UNSUPPORTED_SCHEMA"
			}
			if strings.Contains(three, " ") {
				return out, "MALFORMED_INPUT"
			}
			if in {
				if !hasSecond || hasThird || !goPath(one) || !goVersion(two) {
					return out, "MALFORMED_INPUT"
				}
				if len(out.requires) >= MaxFacts || limits.reserveFacts(1, 1) != "" {
					return out, "LIMIT_EXCEEDED"
				}
				out.requires = append(out.requires, requirement{one, two})
			} else {
				switch one {
				case "module":
					if !hasSecond || hasThird || out.path != "" || !goPath(two) {
						return out, "MALFORMED_INPUT"
					}
					out.path = two
					if limits.reserveFacts(1, 0) != "" {
						return out, "LIMIT_EXCEEDED"
					}
				case "go":
					if !hasSecond || hasThird || out.goVersion != "" || !supportedGoDeclaration(two) {
						return out, "UNSUPPORTED_SCHEMA"
					}
					out.goVersion = two
					if limits.reserveFacts(1, 0) != "" {
						return out, "LIMIT_EXCEEDED"
					}
				case "toolchain":
					if !hasSecond || hasThird || out.toolchain != "" || !strings.HasPrefix(two, "go") || !supportedGoDeclaration(two[2:]) {
						return out, "UNSUPPORTED_SCHEMA"
					}
					out.toolchain = two
					if limits.reserveFacts(1, 0) != "" {
						return out, "LIMIT_EXCEEDED"
					}
				case "require":
					if block || !hasSecond || !hasThird || !goPath(two) || !goVersion(three) {
						return out, "MALFORMED_INPUT"
					}
					if len(out.requires) >= MaxFacts || limits.reserveFacts(1, 1) != "" {
						return out, "LIMIT_EXCEEDED"
					}
					direct = true
					out.requires = append(out.requires, requirement{two, three})
				default:
					return out, "UNSUPPORTED_SCHEMA"
				}
			}
		}
		if !more {
			break
		}
		rest = next
	}
	if in || out.path == "" || out.goVersion == "" {
		return out, "MALFORMED_INPUT"
	}
	if !strictRequirements(out.requires) {
		return out, "DUPLICATE_VALUE"
	}
	return out, ""
}

type sumLine struct {
	path, version string
	mod           bool
	text          string
}

// compareSumLines orders go.sum lines as module.Sort writes them: path, then
// numeric vX.Y.Z version, then the archive line before its /go.mod line.
func compareSumLines(a, b sumLine) int {
	if c := strings.Compare(a.path, b.path); c != 0 {
		return c
	}
	if c := compareCoreVersion(a.version[1:], b.version[1:]); c != 0 {
		return c
	}
	if a.mod == b.mod {
		return strings.Compare(a.text, b.text)
	}
	if b.mod {
		return -1
	}
	return 1
}

// compareCoreVersion compares two canonical X.Y.Z triples numerically; a
// canonical number with more digits is larger.
func compareCoreVersion(a, b string) int {
	for {
		x, restA, _ := strings.Cut(a, ".")
		y, restB, more := strings.Cut(b, ".")
		if c := cmp.Compare(len(x), len(y)); c != 0 {
			return c
		}
		if c := strings.Compare(x, y); c != 0 {
			return c
		}
		if !more {
			return 0
		}
		a, b = restA, restB
	}
}

func parseSum(body string) (map[requirement]sumWitness, string) {
	if !lfText(body) {
		return nil, "MALFORMED_INPUT"
	}
	var last sumLine
	out := make(map[requirement]sumWitness)
	for rest := strings.TrimSuffix(body, "\n"); ; {
		line, next, more := strings.Cut(rest, "\n")
		one, tail, hasSecond := strings.Cut(line, " ")
		two, three, hasThird := strings.Cut(tail, " ")
		if !hasSecond || !hasThird || strings.Contains(three, " ") || !goPath(one) || !h1(three) {
			return nil, "MALFORMED_INPUT"
		}
		v, isMod := strings.TrimSuffix(two, "/go.mod"), strings.HasSuffix(two, "/go.mod")
		if !goVersion(v) {
			return nil, "MALFORMED_INPUT"
		}
		current := sumLine{one, v, isMod, line}
		if last.text == line {
			return nil, "CONFLICTING_VALUE"
		}
		if last.text != "" && compareSumLines(last, current) > 0 {
			return nil, "DUPLICATE_VALUE"
		}
		last = current
		key, w := requirement{one, v}, out[requirement{one, v}]
		if !w.archive && !w.mod && len(out) >= MaxFacts {
			return nil, "LIMIT_EXCEEDED"
		}
		if isMod {
			if w.mod {
				return nil, "AMBIGUOUS_BINDING"
			}
			w.mod = true
		} else {
			if w.archive {
				return nil, "AMBIGUOUS_BINDING"
			}
			w.archive = true
		}
		out[key] = w
		if !more {
			break
		}
		rest = next
	}
	return out, ""
}
func parseWork(body string, limits *resourceLedger) (workInfo, string) {
	var out workInfo
	if !lfText(body) {
		return out, "MALFORMED_INPUT"
	}
	in, block, direct := false, false, false
	for rest := strings.TrimSuffix(body, "\n"); ; {
		line, next, more := strings.Cut(rest, "\n")
		if line == "use (" {
			if in || block || direct || out.goVersion == "" {
				return out, "MALFORMED_INPUT"
			}
			in, block = true, true
			if !more {
				break
			}
			rest = next
			continue
		}
		if line == ")" {
			if !in {
				return out, "MALFORMED_INPUT"
			}
			in = false
			if !more {
				break
			}
			rest = next
			continue
		}
		if strings.ContainsAny(line, "\t") || strings.Contains(line, "  ") || strings.Contains(line, "//") {
			return out, "MALFORMED_INPUT"
		}
		if in {
			if strings.Contains(line, " ") || !logicalPath(line) {
				return out, "MALFORMED_INPUT"
			}
			if len(out.uses) >= MaxFacts || limits.reserveFacts(1, 0) != "" {
				return out, "LIMIT_EXCEEDED"
			}
			out.uses = append(out.uses, line)
		} else {
			key, value, one := strings.Cut(line, " ")
			if !one || strings.Contains(value, " ") {
				return out, "MALFORMED_INPUT"
			}
			switch key {
			case "go":
				if out.goVersion != "" || !supportedGoDeclaration(value) {
					return out, "UNSUPPORTED_SCHEMA"
				}
				out.goVersion = value
				if limits.reserveFacts(1, 0) != "" {
					return out, "LIMIT_EXCEEDED"
				}
			case "toolchain":
				if out.toolchain != "" || !strings.HasPrefix(value, "go") || !supportedGoDeclaration(value[2:]) {
					return out, "UNSUPPORTED_SCHEMA"
				}
				out.toolchain = value
				if limits.reserveFacts(1, 0) != "" {
					return out, "LIMIT_EXCEEDED"
				}
			case "use":
				if block || !logicalPath(value) {
					return out, "MALFORMED_INPUT"
				}
				if len(out.uses) >= MaxFacts || limits.reserveFacts(1, 0) != "" {
					return out, "LIMIT_EXCEEDED"
				}
				direct = true
				out.uses = append(out.uses, value)
			default:
				return out, "UNSUPPORTED_SCHEMA"
			}
		}
		if !more {
			break
		}
		rest = next
	}
	if in || out.goVersion == "" || len(out.uses) == 0 {
		return out, "MALFORMED_INPUT"
	}
	for i := 1; i < len(out.uses); i++ {
		if out.uses[i-1] >= out.uses[i] {
			return out, "DUPLICATE_VALUE"
		}
	}
	return out, ""
}
func strictRequirements(v []requirement) bool {
	for i := 1; i < len(v); i++ {
		if v[i-1].path >= v[i].path || (v[i-1].path == v[i].path && v[i-1].version >= v[i].version) {
			return false
		}
	}
	return true
}

func parseSource(name, body string, limits *resourceLedger) (string, []string, string, bool, string) {
	if !strings.HasSuffix(name, ".go") {
		return "", nil, "", false, "MALFORMED_INPUT"
	}
	if base := path.Base(name); strings.HasPrefix(base, ".") || strings.HasPrefix(base, "_") {
		return "", nil, "", false, "EXACT_BINDING_UNAVAILABLE"
	}
	// MatchFile rejects a mismatched name before opening the file. Preserve
	// that ordering so excluded caller bytes cannot affect the result.
	if !go127FilenameSelected(name, "darwin", "arm64") {
		return "", nil, "", false, "EXACT_BINDING_UNAVAILABLE"
	}
	if !lfText(body) {
		return "", nil, "", false, "MALFORMED_INPUT"
	}
	imports, reason := importHeaderCount(name, body)
	if reason != "" {
		return "", nil, "", false, reason
	}
	expr, reason := buildExprForTarget(body, "darwin", "arm64")
	if reason != "" {
		return "", nil, "", false, reason
	}
	facts := 2 + imports // package and source
	if expr != "" {
		facts++
	}
	// Every source/package/fact/edge and the whole-file AST byte budget is
	// charged before ParseFile can retain the AST.
	if reason = limits.reserveSource(len(body), facts, imports); reason != "" {
		return "", nil, "", false, reason
	}
	fset := token.NewFileSet()
	f, e := parser.ParseFile(fset, name, body, parser.ParseComments|parser.SkipObjectResolution)
	if e != nil {
		return "", nil, "", false, "MALFORMED_INPUT"
	}
	for _, group := range f.Comments {
		for _, comment := range group.List {
			if directiveComment(comment.Text, "//go:embed") || directiveComment(comment.Text, "//go:generate") {
				return "", nil, "", false, "DYNAMIC_INPUT"
			}
		}
	}
	if len(f.Imports) != imports {
		return "", nil, "", false, "MALFORMED_INPUT"
	}
	imps := make([]string, 0, len(f.Imports))
	for _, i := range f.Imports {
		v, qerr := strconv.Unquote(i.Path.Value)
		if qerr != nil {
			return "", nil, "", false, "MALFORMED_INPUT"
		}
		if v == "C" {
			return "", nil, "", false, "EXACT_BINDING_UNAVAILABLE"
		}
		if credentialBearingImport(v) {
			return "", nil, "", false, "CREDENTIAL_INPUT"
		}
		if !goPath(v) {
			return "", nil, "", false, "MALFORMED_INPUT"
		}
		if len(imps) >= MaxFacts {
			return "", nil, "", false, "LIMIT_EXCEEDED"
		}
		imps = append(imps, v)
	}
	sort.Strings(imps)
	for i := 1; i < len(imps); i++ {
		if imps[i-1] == imps[i] {
			return "", nil, "", false, "DUPLICATE_VALUE"
		}
	}
	return f.Name.Name, imps, expr, ast.IsGenerated(f), ""
}

// directiveComment: the comment is exactly the directive or the directive
// followed by whitespace and its arguments — a longer word such as
// //go:embedding is an ordinary comment under the Go 1.27 grammar.
func directiveComment(text, directive string) bool {
	if !strings.HasPrefix(text, directive) {
		return false
	}
	rest := text[len(directive):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

func credentialBearingImport(value string) bool {
	scheme := strings.Index(value, "://")
	if scheme < 1 {
		return false
	}
	authority := value[scheme+3:]
	if slash := strings.IndexByte(authority, '/'); slash >= 0 {
		authority = authority[:slash]
	}
	return strings.Contains(authority, "@")
}

func go127MatchFile(name, body string) (bool, error) {
	return go127MatchFileForTarget(name, body, "darwin", "arm64")
}

func go127MatchFileForTarget(name, body, goos, goarch string) (bool, error) {
	_, reason := buildExprForTarget(body, goos, goarch)
	if reason == "EXACT_BINDING_UNAVAILABLE" {
		return false, nil
	}
	if reason != "" {
		return false, ErrMalformed
	}
	return go127FilenameSelected(name, goos, goarch), nil
}

// importHeaderCount is a non-retaining Go-token pass. It counts only imports
// in the package header before ParseFile may retain a whole-file AST.
func importHeaderCount(name, body string) (int, string) {
	// Most source inputs have no import declaration. The keyword cannot occur
	// in executable Go syntax other than an import declaration, so this avoids
	// scanner allocation on that closed zero-edge path; comments/strings only
	// take the bounded scanner path unnecessarily, never bypass a real import.
	if !strings.Contains(body, "import") {
		return 0, ""
	}
	var first error
	file := token.NewFileSet().AddFile(name, -1, len(body))
	var s scanner.Scanner
	s.Init(file, []byte(body), func(_ token.Position, message string) {
		if first == nil {
			first = errors.New(message)
		}
	}, scanner.ScanComments)
	seenPackage, packageName, inImport, block := false, false, false, false
	count := 0
	for {
		_, tok, _ := s.Scan()
		if tok == token.EOF {
			break
		}
		if first != nil {
			return 0, "MALFORMED_INPUT"
		}
		if tok == token.COMMENT || tok == token.SEMICOLON {
			continue
		}
		if !seenPackage {
			if tok != token.PACKAGE {
				return 0, "MALFORMED_INPUT"
			}
			seenPackage = true
			continue
		}
		if !packageName {
			if tok != token.IDENT {
				return 0, "MALFORMED_INPUT"
			}
			packageName = true
			continue
		}
		if !inImport {
			if tok != token.IMPORT {
				break
			}
			inImport = true
			continue
		}
		if tok == token.LPAREN && !block {
			block = true
			continue
		}
		if tok == token.STRING {
			count++
			if count > MaxFacts {
				return 0, "LIMIT_EXCEEDED"
			}
			if !block {
				inImport = false
			}
			continue
		}
		if tok == token.RPAREN && block {
			inImport, block = false, false
			continue
		}
		if tok == token.IDENT || tok == token.PERIOD {
			continue
		}
		return 0, "MALFORMED_INPUT"
	}
	if first != nil || !seenPackage || !packageName || inImport {
		return 0, "MALFORMED_INPUT"
	}
	return count, ""
}

const maxBuildConstraintBytes = 6305

// buildExprForTarget is a contained Go 1.27 header evaluator.  It reads only
// caller bytes: unlike go/build.Context it cannot initialize from HOME,
// GOPATH, CGO_ENABLED, a directory, or an ambient toolchain configuration.
func buildExprForTarget(body, goos, goarch string) (string, string) {
	// The header mirrors go/build: blank lines, // comments and /* */
	// comments. A //go:build line counts anywhere in it outside a block
	// comment; a // +build line counts only when a blank line follows it
	// before the first non-// line. A legacy spelling go/build honours but
	// this closed parser does not (//+build) rejects instead of vanishing.
	goLine := ""
	plus, pending := []string{}, []string{}
	ended, inSlashStar, pendingNoncanonical := false, false, false
Lines:
	for lineStart := 0; lineStart < len(body); {
		lineEnd := strings.IndexByte(body[lineStart:], '\n')
		next := len(body)
		if lineEnd < 0 {
			lineEnd = len(body)
		} else {
			lineEnd += lineStart
			next = lineEnd + 1
		}
		line := body[lineStart:lineEnd]
		lineStart = next
		content := strings.TrimSpace(line)
		if content == "" && !ended {
			if pendingNoncanonical {
				return "", "MALFORMED_INPUT"
			}
			plus = append(plus, pending...)
			pending = pending[:0]
			if len(plus) > maxBuildVariables+1 {
				return "", "LIMIT_EXCEEDED"
			}
			continue
		}
		if !strings.HasPrefix(content, "//") {
			ended = true
		}
		// Go's header scanner accepts horizontal indentation before a directive;
		// normalize only that header spelling before the closed parser below.
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed != line && (directiveComment(trimmed, "//go:build") || strings.HasPrefix(trimmed, "// +build")) {
			line = trimmed
		}
		if !inSlashStar && directiveComment(line, "//go:build") {
			if goLine != "" || !strings.HasPrefix(line, "//go:build ") {
				return "", "MALFORMED_INPUT"
			}
			goLine = line[11:]
			if len(goLine) > maxBuildConstraintBytes {
				return "", "LIMIT_EXCEEDED"
			}
		}
		if !ended && constraint.IsPlusBuild(content) && !strings.HasPrefix(line, "// +build ") {
			pendingNoncanonical = true
		}
		if !ended && strings.HasPrefix(line, "// +build ") {
			if len(line)-len("// +build ") > maxBuildConstraintBytes {
				return "", "LIMIT_EXCEEDED"
			}
			pending = append(pending, line[10:])
		}
		for content != "" {
			if inSlashStar {
				closeAt := strings.Index(content, "*/")
				if closeAt < 0 {
					continue Lines
				}
				inSlashStar = false
				content = strings.TrimSpace(content[closeAt+2:])
				continue
			}
			if strings.HasPrefix(content, "//") {
				continue Lines
			}
			if strings.HasPrefix(content, "/*") {
				inSlashStar = true
				content = strings.TrimSpace(content[2:])
				continue
			}
			break Lines
		}
	}
	if goLine == "" && len(plus) == 0 {
		return "", ""
	}
	var expr constraint.Expr
	var e error
	if goLine != "" {
		expr, e = constraint.Parse("//go:build " + goLine)
		if e != nil || expr.String() != goLine {
			return "", "MALFORMED_INPUT"
		}
		if len(plus) > 0 {
			legacy, legacyErr := parseLegacyConstraints(plus)
			if legacyErr != nil {
				return "", "MALFORMED_INPUT"
			}
			equivalent, bounded := equivalentBuildConstraints(expr, legacy)
			if !bounded {
				return "", "LIMIT_EXCEEDED"
			}
			if !equivalent {
				return "", "MALFORMED_INPUT"
			}
		}
	} else {
		expr, e = parseLegacyConstraints(plus)
		if e != nil {
			return "", "MALFORMED_INPUT"
		}
	}
	ok := expr.Eval(func(tag string) bool {
		return tag == goos || tag == goarch || (tag == "unix" && go127UnixOS(goos)) || tag == "gc" || contains(releaseTags(), tag)
	})
	if !ok {
		return "", "EXACT_BINDING_UNAVAILABLE"
	}
	return expr.String(), ""
}

func buildExpr(body string) (string, string) {
	return buildExprForTarget(body, "darwin", "arm64")
}
func parseLegacyConstraints(lines []string) (constraint.Expr, error) {
	var expr constraint.Expr
	for _, line := range lines {
		part, err := constraint.Parse("// +build " + line)
		if err != nil {
			return nil, err
		}
		if expr == nil {
			expr = part
		} else {
			expr = &constraint.AndExpr{X: expr, Y: part}
		}
	}
	return expr, nil
}

// go127FilenameSelected applies Go 1.27's closed filename suffix grammar to
// a single logical source path. No directory or ambient state participates.
func go127FilenameSelected(name, goos, goarch string) bool {
	// go/build cuts the name at its first dot and ignores everything before
	// the first underscore, so x_linux.pb.go is a linux file and
	// x.linux_amd64.go carries no suffix.
	base, _, _ := strings.Cut(path.Base(name), ".")
	first := strings.IndexByte(base, '_')
	if first < 0 {
		return true
	}
	parts := strings.Split(base[first:], "_")
	if n := len(parts); n > 0 && parts[n-1] == "test" {
		parts = parts[:n-1]
	}
	n := len(parts)
	if n >= 2 && go127KnownOS(parts[n-2]) && go127KnownArch(parts[n-1]) {
		return parts[n-2] == goos && parts[n-1] == goarch
	}
	if n >= 1 && go127KnownOS(parts[n-1]) {
		return parts[n-1] == goos
	}
	if n >= 1 && go127KnownArch(parts[n-1]) {
		return parts[n-1] == goarch
	}
	return true
}

// go127UnixOS reports whether goos is a member of the "unix" build-tag set
// go/build.Context evaluates (see go/build's unixOS table). It is narrower
// than go127KnownOS: js, nacl, plan9, wasip1, windows and zos are known OS
// names but never satisfy the unix tag.
func go127UnixOS(s string) bool {
	switch s {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd",
		"illumos", "ios", "linux", "netbsd", "openbsd", "solaris":
		return true
	}
	return false
}

func go127KnownOS(s string) bool {
	switch s {
	case "aix", "android", "darwin", "dragonfly", "freebsd", "hurd",
		"illumos", "ios", "js", "linux", "nacl", "netbsd", "openbsd",
		"plan9", "solaris", "wasip1", "windows", "zos":
		return true
	}
	return false
}

func go127KnownArch(s string) bool {
	switch s {
	case "386", "amd64", "amd64p32", "arm", "armbe", "arm64", "arm64be",
		"loong64", "mips", "mipsle", "mips64", "mips64le", "mips64p32",
		"mips64p32le", "ppc", "ppc64", "ppc64le", "riscv", "riscv64",
		"s390", "s390x", "sparc", "sparc64", "wasm":
		return true
	}
	return false
}

const maxBuildVariables = 12

// equivalentBuildConstraints exhaustively compares the two expressions over
// their closed, bounded tag vocabulary. Unlike DNF rewriting, this preserves
// all Boolean identities (tautology, consensus, absorption, and multiline
// legacy conjunction) without depending on presentation.
func equivalentBuildConstraints(a, b constraint.Expr) (bool, bool) {
	var tags [maxBuildVariables]string
	n := 0
	if !collectBuildTags(a, &tags, &n) || !collectBuildTags(b, &tags, &n) {
		return false, false
	}
	for assignment := 0; assignment < 1<<n; assignment++ {
		enabled := func(tag string) bool {
			for i := 0; i < n; i++ {
				if tags[i] == tag {
					return assignment&(1<<i) != 0
				}
			}
			return false
		}
		if a.Eval(enabled) != b.Eval(enabled) {
			return false, true
		}
	}
	return true, true
}

func collectBuildTags(expr constraint.Expr, tags *[maxBuildVariables]string, n *int) bool {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		for i := 0; i < *n; i++ {
			if tags[i] == e.Tag {
				return true
			}
		}
		if *n == len(tags) {
			return false
		}
		tags[*n] = e.Tag
		*n = *n + 1
		return true
	case *constraint.NotExpr:
		return collectBuildTags(e.X, tags, n)
	case *constraint.AndExpr:
		return collectBuildTags(e.X, tags, n) && collectBuildTags(e.Y, tags, n)
	case *constraint.OrExpr:
		return collectBuildTags(e.X, tags, n) && collectBuildTags(e.Y, tags, n)
	default:
		return false
	}
}
func itoa(n int) string {
	if n >= 10 {
		return string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	return string(rune('0' + n))
}
func releaseTags() []string {
	return []string{"go1.1", "go1.2", "go1.3", "go1.4", "go1.5", "go1.6", "go1.7", "go1.8", "go1.9", "go1.10", "go1.11", "go1.12", "go1.13", "go1.14", "go1.15", "go1.16", "go1.17", "go1.18", "go1.19", "go1.20", "go1.21", "go1.22", "go1.23", "go1.24", "go1.25", "go1.26", "go1.27"}
}

type evidenceState struct {
	target string
	buf    []byte
}

func addOwned(c *candidate, state *evidenceState, r Request, in, related *Input, kind, subject, predicate, value, instance string) string {
	relatedHandle, relatedDigest := "-", "-"
	if related != nil {
		relatedHandle, relatedDigest = related.Handle, related.SHA256
	}
	f := Fact{Kind: kind, InputHandle: in.Handle, RelatedHandle: relatedHandle, Subject: subject, Predicate: predicate, Value: value, InstanceID: instance}
	prospective, reason := admitFact(c, f)
	if reason != "" {
		return reason
	}
	digest, reason := evidence(state, r.Family, r, in.Handle, in.SHA256, relatedHandle, relatedDigest, f)
	if reason != "" {
		return reason
	}
	f.EvidenceSHA256 = digest
	c.Facts = append(c.Facts, f)
	c.outputBytes = prospective
	return ""
}
func evidence(state *evidenceState, family string, r Request, inputHandle, inputDigest, relatedHandle, relatedDigest string, f Fact) (string, string) {
	fields := [...]string{family, r.RequestID, r.ScopeID, r.CompilationUnitID, state.target, inputHandle, inputDigest, relatedHandle, relatedDigest, f.Kind, f.Subject, f.Predicate, f.Value, f.InstanceID}
	n := len("corvint-analyzer-candidate-evidence/experimental") + 4
	for _, field := range fields {
		if len(field) > MaxOutputBytes-4-n {
			return "", "LIMIT_EXCEEDED"
		}
		n += 4 + len(field)
	}
	b := state.buf[:0]
	if cap(b) < n {
		state.buf = make([]byte, 0, n)
		b = state.buf
	}
	b = append(b, "corvint-analyzer-candidate-evidence/experimental"...)
	b = appendUint32(b, uint32(len(fields)))
	for _, field := range fields {
		b = appendUint32(b, uint32(len(field)))
		b = append(b, field...)
	}
	s := sha256.Sum256(b)
	var encoded [71]byte
	copy(encoded[:], "sha256:")
	hex.Encode(encoded[7:], s[:])
	return string(encoded[:]), ""
}
func appendUint32(b []byte, n uint32) []byte {
	var frame [4]byte
	binary.BigEndian.PutUint32(frame[:], n)
	return append(b, frame[:]...)
}
func add(c *candidate, f Fact) string {
	prospective, reason := admitFact(c, f)
	if reason != "" {
		return reason
	}
	c.Facts = append(c.Facts, f)
	c.outputBytes = prospective
	return ""
}
func admitFact(c *candidate, f Fact) (int, string) {
	if len(c.Facts) >= MaxFacts {
		return 0, "LIMIT_EXCEEDED"
	}
	if !factField(f.Kind) || !factField(f.InputHandle) || !factField(f.RelatedHandle) || !factField(f.Subject) || !factField(f.Predicate) || !factField(f.Value) || !factField(f.InstanceID) {
		return 0, "MALFORMED_INPUT"
	}
	// The evidence digest has fixed canonical length, so the charge covers the
	// complete encoded fact before its digest frame or fact is retained.
	prospective := c.outputBytes + factJSONSize(Fact{Kind: f.Kind, InputHandle: f.InputHandle, RelatedHandle: f.RelatedHandle, Subject: f.Subject, Predicate: f.Predicate, Value: f.Value, InstanceID: f.InstanceID, EvidenceSHA256: evidencePlaceholder}) + 1
	if len(c.Facts) == 0 {
		prospective--
	} // [] becomes [fact]
	if prospective > MaxOutputBytes {
		return 0, "OUTPUT_LIMIT"
	}
	return prospective, ""
}

const emptyFactJSON = `{"kind":"","input_handle":"","related_handle":"","subject":"","predicate":"","value":"","instance_id":"","evidence_sha256":""}`
const emptyCandidateJSON = `{"profile":"","family":"","request_id":"","status":"","scope_id":"","compilation_unit_id":"","target":{"os":"","architecture":"","abi":"","features":[]},"input_echoes":[],"facts":[]}`
const emptyEchoJSON = `{"handle":"","sha256":""}`
const evidencePlaceholder = "sha256:0000000000000000000000000000000000000000000000000000000000000000"

func factJSONSize(f Fact) int {
	return len(emptyFactJSON) + len(f.Kind) + len(f.InputHandle) + len(f.RelatedHandle) + len(f.Subject) + len(f.Predicate) + len(f.Value) + len(f.InstanceID) + len(f.EvidenceSHA256)
}
func candidateJSONSize(c candidate) int {
	n := len(emptyCandidateJSON) + len(c.Profile) + len(c.Family) + len(c.RequestID) + len(c.Status) + len(c.ScopeID) + len(c.CompilationUnitID) + len(c.Target.OS) + len(c.Target.Architecture) + len(c.Target.ABI)
	if len(c.Target.Features) > 0 {
		n += len(c.Target.Features) - 1
		for _, feature := range c.Target.Features {
			n += len(feature) + 2
		}
	}
	if len(c.InputEchoes) > 0 {
		n += len(c.InputEchoes) - 1
		for _, echo := range c.InputEchoes {
			n += len(emptyEchoJSON) + len(echo.Handle) + len(echo.SHA256)
		}
	}
	return n
}
func compareInput(a, b Input) int {
	return compare([]string{a.Handle, a.Family, a.Path, a.SHA256}, []string{b.Handle, b.Family, b.Path, b.SHA256})
}
func compareFact(a, b Fact) int {
	return compare([]string{a.Kind, a.InputHandle, a.RelatedHandle, a.Subject, a.Predicate, a.Value, a.InstanceID, a.EvidenceSHA256}, []string{b.Kind, b.InputHandle, b.RelatedHandle, b.Subject, b.Predicate, b.Value, b.InstanceID, b.EvidenceSHA256})
}
func compare(a, b []string) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
func identifier(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for i, c := range []byte(s) {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || (i > 0 && strings.ContainsRune("._:@+~-", rune(c)))) {
			return false
		}
	}
	return true
}
func logicalPath(s string) bool {
	if len(s) == 0 || len(s) > 4096 || strings.ContainsAny(s, "\\:%\x00\r\n\t") {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." || len(p) > 128 {
			return false
		}
		for _, c := range []byte(p) {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("._@+~-", rune(c))) {
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
	_, e := hex.DecodeString(s[7:])
	return e == nil && strings.ToLower(s[7:]) == s[7:]
}
func goPath(s string) bool {
	if len(s) == 0 || len(s) > 4096 {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if len(part) == 0 || len(part) > 128 || part[0] == '.' || part[0] == '-' {
			return false
		}
		for _, c := range []byte(part) {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("._~-", rune(c))) {
				return false
			}
		}
	}
	return true
}
func coreVersion(s string) bool {
	p := strings.Split(s, ".")
	return len(p) == 3 && canonicalNumber(p[0]) && canonicalNumber(p[1]) && canonicalNumber(p[2])
}

// supportedGoDeclaration is deliberately a closed profile set, not a
// syntactic version predicate: decision 0007 D7 admits exactly the language
// and toolchain tuples 1.26.Z and 1.27.Z with a canonical patch Z in 0..99.
// A newer minor series must be rejected until its own profile change adds
// evidence.
func supportedGoDeclaration(s string) bool { return goLanguageRank(s) >= 0 }

// goLanguageRank orders admitted declarations by minor then patch, so a
// workspace 1.27.9 is older than a module 1.27.10.
func goLanguageRank(s string) int {
	rest, ok := strings.CutPrefix(s, "1.")
	if !ok {
		return -1
	}
	minor, patch, ok := strings.Cut(rest, ".")
	if !ok || (minor != "26" && minor != "27") || !canonicalNumber(patch) || len(patch) > 2 {
		return -1
	}
	m, _ := strconv.Atoi(minor)
	z, _ := strconv.Atoi(patch)
	return m*100 + z
}
func goVersion(s string) bool       { return strings.HasPrefix(s, "v") && coreVersion(s[1:]) }
func canonicalNumber(s string) bool { return digits(s) && (s == "0" || s[0] != '0') }
func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
func h1(s string) bool {
	if !strings.HasPrefix(s, "h1:") {
		return false
	}
	b, e := base64.StdEncoding.DecodeString(s[3:])
	return e == nil && len(b) == sha256.Size && base64.StdEncoding.EncodeToString(b) == s[3:]
}
func importPath(s string) bool { return logicalPath(s) }
func factField(s string) bool {
	if len(s) == 0 || len(s) > 4096 {
		return false
	}
	for _, c := range []byte(s) {
		if c < 0x20 || c > 0x7e || c == '"' || c == '\\' {
			return false
		}
	}
	return true
}
func lfText(s string) bool {
	return s != "" && utf8.ValidString(s) && strings.HasSuffix(s, "\n") && !strings.Contains(s, "\r")
}
func contains(v []string, s string) bool {
	for _, value := range v {
		if value == s {
			return true
		}
	}
	return false
}
