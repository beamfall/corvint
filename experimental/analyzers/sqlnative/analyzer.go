package sqlnative

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
	"unsafe"
)

const (
	Profile            = "corvint-analyzer-candidate/sqlite-3.51.0-source-v1"
	Family             = "sqlite"
	SQLiteDialect      = "sqlite-3.51.0"
	CandidateToolchain = "go1.27.0"
	maxFrameBytes      = 1500000
	maxInputBytes      = 1048576
	maxInputs          = 128
	maxFacts           = 4_096
	maxOutput          = 1_048_576
	maxTokens          = 4_096
	maxStringBytes     = 4_096
	maxJSONDepth       = 8
	maxSeen            = 4_096
	maxIdentifierBytes = 128
)

var preReasons = map[string]struct{}{
	"NONCANONICAL_REQUEST": {}, "INVALID_IDENTIFIER": {}, "UNKNOWN_FAMILY": {},
	"UNKNOWN_FIELD": {}, "MALFORMED_INPUT": {}, "LIMIT_EXCEEDED": {},
}
var (
	errConflict    = errors.New("CONFLICTING_VALUE")
	errDuplicate   = errors.New("DUPLICATE_VALUE")
	errLimit       = errors.New("LIMIT_EXCEEDED")
	errMalformed   = errors.New("MALFORMED_INPUT")
	errOutput      = errors.New("OUTPUT_LIMIT")
	errUnsupported = errors.New("UNSUPPORTED_SCHEMA")
)

type Request struct {
	Profile           string  `json:"profile"`
	Family            string  `json:"family"`
	RequestID         string  `json:"request_id"`
	ScopeID           string  `json:"scope_id"`
	CompilationUnitID string  `json:"compilation_unit_id"`
	Target            Target  `json:"target"`
	Inputs            []Input `json:"inputs"`
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
type response struct {
	Profile           string  `json:"profile"`
	Family            string  `json:"family"`
	RequestID         string  `json:"request_id"`
	Status            string  `json:"status"`
	ScopeID           string  `json:"scope_id,omitempty"`
	CompilationUnitID string  `json:"compilation_unit_id,omitempty"`
	Target            *Target `json:"target,omitempty"`
	InputEchoes       []Echo  `json:"input_echoes,omitempty"`
	Facts             *[]Fact `json:"facts,omitempty"`
	Reason            string  `json:"reason,omitempty"`
}
type Analyzer struct {
	mu          sync.Mutex
	seen        map[string][32]byte
	parse       sqlParser
	outputLimit int
}
type sqlParser func(Request, Input, []byte, func(Fact, *schemaObject) error) error

func New() *Analyzer {
	return &Analyzer{seen: make(map[string][32]byte), parse: parseSQL, outputLimit: maxOutput}
}
func (a *Analyzer) AnalyzeFrame(frame []byte) []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	request, raw, reason := decodeFrame(frame)
	if reason != "" {
		return marshalPre(reason)
	}
	echoes := echoes(request.Inputs)
	if a.parse == nil {
		return marshalRejected(request, echoes, "ANALYZER_FAILURE")
	}
	digest := sha256.Sum256(raw)
	if earlier, exists := a.seen[request.RequestID]; exists && earlier != digest {
		return marshalRejected(request, echoes, "CONFLICTING_VALUE")
	}
	if _, exists := a.seen[request.RequestID]; !exists && len(a.seen) == maxSeen {
		return marshalRejected(request, echoes, "LIMIT_EXCEEDED")
	}
	a.seen[request.RequestID] = digest

	facts := make([]Fact, 0, 16)
	factSeen := make(map[Fact]struct{})
	objects := make(map[schemaObject]string)
	outputBytes := successBaseLen(request, echoes)
	if outputBytes > a.outputLimit {
		return marshalRejected(request, echoes, "OUTPUT_LIMIT")
	}
	emit := func(f Fact, object *schemaObject) error {
		if len(facts) == maxFacts {
			return errLimit
		}
		if _, exists := factSeen[f]; exists {
			return errDuplicate
		}
		if object != nil {
			if prior, exists := objects[*object]; exists {
				if prior == f.Kind {
					return errDuplicate
				}
				return errConflict
			}
			objects[*object] = f.Kind
		}
		addition := factJSONLen(f)
		if len(facts) != 0 {
			addition++
		}
		if addition > a.outputLimit-outputBytes {
			return errOutput
		}
		outputBytes += addition
		factSeen[f] = struct{}{}
		facts = append(facts, f)
		return nil
	}

	for _, input := range request.Inputs {
		content, err := decodeContent(input.ContentBase64)
		if err != nil {
			return marshalRejected(request, echoes, "MALFORMED_INPUT")
		}
		sum := sha256.Sum256(content)
		if input.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			return marshalRejected(request, echoes, "DIGEST_MISMATCH")
		}
		if err := a.parse(request, input, content, emit); err != nil {
			return marshalRejected(request, echoes, errorReason(err))
		}
	}
	sort.Slice(facts, func(i, j int) bool { return compareFact(facts[i], facts[j]) < 0 })
	result := response{Profile: Profile, Family: Family, RequestID: request.RequestID, Status: "CANDIDATE", ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: &request.Target, InputEchoes: echoes, Facts: &facts}
	encoded := marshal(result)
	if len(encoded) > a.outputLimit {
		return marshalRejected(request, echoes, "OUTPUT_LIMIT")
	}
	return encoded
}
func AnalyzeFrame(frame []byte) []byte { return New().AnalyzeFrame(frame) }
func decodeFrame(frame []byte) (Request, []byte, string) {
	if len(frame) > maxFrameBytes {
		return Request{}, nil, "LIMIT_EXCEEDED"
	}
	if len(frame) < 2 || frame[len(frame)-1] != '\n' || bytes.IndexByte(frame[:len(frame)-1], '\n') >= 0 {
		return Request{}, nil, "NONCANONICAL_REQUEST"
	}
	raw := frame[:len(frame)-1]
	if !utf8.Valid(raw) {
		return Request{}, nil, "MALFORMED_INPUT"
	}
	if reason := scanRequest(raw); reason != "" {
		return Request{}, nil, reason
	}
	var request Request
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return Request{}, nil, "UNKNOWN_FIELD"
	}
	if request.Target.Features == nil && bytes.Contains(raw, []byte(`"features":null`)) {
		return Request{}, nil, "MALFORMED_INPUT"
	}
	if !canonicalRequestMatches(raw, request) {
		return Request{}, nil, "NONCANONICAL_REQUEST"
	}
	if request.Profile != Profile {
		return Request{}, nil, "NONCANONICAL_REQUEST"
	}
	if request.Family != Family {
		return Request{}, nil, "UNKNOWN_FAMILY"
	}
	if !identifier(request.RequestID) || !identifier(request.ScopeID) || !identifier(request.CompilationUnitID) {
		return Request{}, nil, "INVALID_IDENTIFIER"
	}
	if err := validateTarget(request.Target); err != nil {
		return Request{}, nil, errorReason(err)
	}
	if len(request.Inputs) == 0 || len(request.Inputs) > maxInputs {
		return Request{}, nil, "LIMIT_EXCEEDED"
	}
	var previous inputIdentity
	havePrevious := false
	seenHandles := make(map[string]struct{}, len(request.Inputs))
	seenPaths := make(map[string]struct{}, len(request.Inputs))
	for _, input := range request.Inputs {
		if !identifier(input.Handle) || !validDigest(input.SHA256) || input.Family != "sqlite.migration" && input.Family != "sqlite.query" || !validPath(input.Path, input.Family) {
			return Request{}, nil, "MALFORMED_INPUT"
		}
		identity := inputIdentity{handle: input.Handle, family: input.Family, path: input.Path, digest: input.SHA256}
		if havePrevious && compareInputIdentity(identity, previous) <= 0 {
			return Request{}, nil, "NONCANONICAL_REQUEST"
		}
		if _, exists := seenHandles[input.Handle]; exists {
			return Request{}, nil, "NONCANONICAL_REQUEST"
		}
		if _, exists := seenPaths[input.Path]; exists {
			return Request{}, nil, "NONCANONICAL_REQUEST"
		}
		previous, havePrevious = identity, true
		seenHandles[input.Handle] = struct{}{}
		seenPaths[input.Path] = struct{}{}
	}
	return request, raw, ""
}

func canonicalRequestMatches(raw []byte, r Request) bool {
	b := marshal(r)
	return len(b) == len(raw)+1 && bytes.Equal(b[:len(raw)], raw)
}

func scanRequest(raw []byte) string {
	depth, tokens, totalDecoded := 0, 0, 0
	expectBase64 := false
	for i := 0; i < len(raw); {
		switch raw[i] {
		case ' ', '\t', '\r', '\n':
			i++
		case '{', '[':
			depth++
			if depth > maxJSONDepth {
				return "LIMIT_EXCEEDED"
			}
			tokens++
			i++
		case '}', ']':
			depth--
			if depth < 0 {
				return "MALFORMED_INPUT"
			}
			tokens++
			i++
		case '"':
			start := i
			end, decoded, escaped, ok := scanJSONString(raw, i)
			if !ok {
				return "MALFORMED_INPUT"
			}
			tokens++
			j := end
			for j < len(raw) && isJSONSpace(raw[j]) {
				j++
			}
			isKey := j < len(raw) && raw[j] == ':'
			if isKey {
				expectBase64 = !escaped && bytes.Equal(raw[start+1:end-1], []byte("content_base64"))
			} else if expectBase64 {
				if escaped || decoded > base64.StdEncoding.EncodedLen(maxInputBytes) || !canonicalBase64(raw[start+1:end-1]) {
					return "MALFORMED_INPUT"
				}
				decodedLen := base64.StdEncoding.DecodedLen(decoded)
				if decoded >= 2 && raw[end-2] == '=' {
					decodedLen--
				}
				if decoded >= 1 && raw[end-3] == '=' {
					decodedLen--
				}
				if decodedLen > maxInputBytes-totalDecoded {
					return "LIMIT_EXCEEDED"
				}
				totalDecoded += decodedLen
				expectBase64 = false
			} else if decoded > maxStringBytes {
				return "LIMIT_EXCEEDED"
			}
			i = end
		default:
			if raw[i] < 0x20 {
				return "MALFORMED_INPUT"
			}
			tokens++
			i++
		}
		if tokens > maxTokens {
			return "LIMIT_EXCEEDED"
		}
	}
	if depth != 0 {
		return "MALFORMED_INPUT"
	}
	return ""
}

func scanJSONString(raw []byte, start int) (end, decoded int, escaped, ok bool) {
	for i := start + 1; i < len(raw); i++ {
		c := raw[i]
		if c == '"' {
			return i + 1, decoded, escaped, true
		}
		if c < 0x20 {
			return 0, 0, false, false
		}
		if c != '\\' {
			decoded++
			continue
		}
		escaped = true
		i++
		if i == len(raw) {
			return 0, 0, false, false
		}
		switch raw[i] {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			decoded++
		case 'u':
			if i+4 >= len(raw) || !hexByte(raw[i+1]) || !hexByte(raw[i+2]) || !hexByte(raw[i+3]) || !hexByte(raw[i+4]) {
				return 0, 0, false, false
			}
			decoded += 3
			i += 4
		default:
			return 0, 0, false, false
		}
	}
	return 0, 0, false, false
}

func isJSONSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }
func hexByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

type byteString interface{ ~string | ~[]byte }

func canonicalBase64[T byteString](value T) bool {
	if len(value) == 0 || len(value)%4 != 0 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '+' || c == '/' {
			continue
		}
		if c == '=' && i >= len(value)-2 {
			continue
		}
		return false
	}
	return canonicalBase64Tail(value)
}

func decodeContent(value string) ([]byte, error) {
	if len(value) == 0 || len(value) > base64.StdEncoding.EncodedLen(maxInputBytes) || !canonicalBase64(value) {
		return nil, errors.New("bad base64")
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) > maxInputBytes {
		return nil, errors.New("bad base64")
	}
	return decoded, nil
}

func canonicalBase64Tail[T byteString](value T) bool {
	if value[len(value)-1] != '=' {
		return true
	}
	if value[len(value)-2] == '=' {
		return b64Index(value[len(value)-3])&15 == 0
	}
	return b64Index(value[len(value)-2])&3 == 0
}
func b64Index(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c - 'A'
	}
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 26
	}
	if c >= '0' && c <= '9' {
		return c - '0' + 52
	}
	if c == '+' {
		return 62
	}
	return 63
}

func validateTarget(t Target) error {
	if (t.OS != "darwin" && t.OS != "linux") || (t.Architecture != "arm64" && t.Architecture != "amd64") || t.ABI != "none" || t.Features == nil || len(t.Features) != 0 {
		return errMalformed
	}
	return nil
}
func identifier(s string) bool {
	if len(s) == 0 || len(s) > maxIdentifierBytes {
		return false
	}
	for i := range s {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || (i > 0 && strings.ContainsRune("._:@+~-", rune(c)))) {
			return false
		}
	}
	return true
}
func validDigest(s string) bool {
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
func validPath(path, family string) bool {
	if len(path) == 0 || len(path) > 4096 || strings.ContainsAny(path, "\\:%") || strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if len(segment) == 0 || len(segment) > maxIdentifierBytes || segment == "." || segment == ".." {
			return false
		}
		for _, c := range []byte(segment) {
			if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || strings.ContainsRune("._@+~-", rune(c))) {
				return false
			}
		}
	}
	if !strings.HasSuffix(path, ".sql") {
		return false
	}
	if family == "sqlite.migration" {
		return strings.HasPrefix(path, "internal/store/migrate/")
	}
	return strings.HasPrefix(path, "internal/store/") || strings.HasPrefix(path, "testdata/antennapod/")
}

type inputIdentity struct{ handle, family, path, digest string }

func compareInputIdentity(a, b inputIdentity) int {
	return comparePairs([][2]string{{a.handle, b.handle}, {a.family, b.family}, {a.path, b.path}, {a.digest, b.digest}})
}
func comparePairs(pairs [][2]string) int {
	for _, pair := range pairs {
		if c := strings.Compare(pair[0], pair[1]); c != 0 {
			return c
		}
	}
	return 0
}
func echoes(inputs []Input) []Echo {
	result := make([]Echo, len(inputs))
	for i, input := range inputs {
		result[i] = Echo{Handle: input.Handle, SHA256: input.SHA256}
	}
	return result
}
func marshal(value any) []byte {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return output.Bytes()
}
func marshalPre(reason string) []byte {
	if _, ok := preReasons[reason]; !ok {
		reason = "NONCANONICAL_REQUEST"
	}
	return marshal(response{Profile: Profile, Family: "unknown", RequestID: "unknown", Status: "REJECTED", Reason: reason})
}
func marshalRejected(r Request, e []Echo, reason string) []byte {
	return marshal(response{Profile: Profile, Family: Family, RequestID: r.RequestID, Status: "REJECTED", ScopeID: r.ScopeID, CompilationUnitID: r.CompilationUnitID, Target: &r.Target, InputEchoes: e, Reason: reason})
}
func errorReason(err error) string {
	if err == nil {
		return ""
	}
	switch err.Error() {
	case "MALFORMED_INPUT", "UNSUPPORTED_SCHEMA", "LIMIT_EXCEEDED", "DIGEST_MISMATCH", "CONFLICTING_VALUE", "DUPLICATE_VALUE", "OUTPUT_LIMIT", "ANALYZER_FAILURE":
		return err.Error()
	}
	return "ANALYZER_FAILURE"
}

// Each JSON string value adds its two quotes; identifiers and digests never need escaping.
func factJSONLen(f Fact) int {
	return len(`{"kind":"","input_handle":"","related_handle":"","subject":"","predicate":"","value":"","instance_id":"","evidence_sha256":""}`) + len(f.Kind) + len(f.InputHandle) + len(f.RelatedHandle) + len(f.Subject) + len(f.Predicate) + len(f.Value) + len(f.InstanceID) + len(f.EvidenceSHA256)
}
func successBaseLen(r Request, echoes []Echo) int {
	n := len(`{"profile":"`+Profile+`","family":"`+Family+`","request_id":"","status":"CANDIDATE","scope_id":"","compilation_unit_id":"","target":{"os":"","architecture":"","abi":"","features":[]},"input_echoes":[`) + len(r.RequestID) + len(r.ScopeID) + len(r.CompilationUnitID) + len(r.Target.OS) + len(r.Target.Architecture) + len(r.Target.ABI)
	for i, e := range echoes {
		if i != 0 {
			n++
		}
		n += len(`{"handle":"","sha256":""}`) + len(e.Handle) + len(e.SHA256)
	}
	return n + len("],\"facts\":[]}\n")
}

type tokenKind uint8

const (
	tokIdent tokenKind = iota
	tokQuoted
	tokString
	tokNumber
	tokBlob
	tokSymbol
)

type token struct {
	kind       tokenKind
	close      int16
	text, norm string
	start, end int
}
type sqlName struct{ display, key string }

func (t token) keyword(word string) bool {
	return t.kind == tokIdent && strings.EqualFold(t.text, word)
}
func (t token) oneOf(words ...string) bool {
	for _, word := range words {
		if t.keyword(word) {
			return true
		}
	}
	return false
}
func (t token) name() (sqlName, bool) {
	if t.kind != tokQuoted && (t.kind != tokIdent || sqliteReserved(t.norm)) {
		return sqlName{}, false
	}
	return sqlName{display: t.text, key: t.norm}, true
}

const kw = ",abort,action,add,after,all,alter,always,analyze,and,as,asc,attach,autoincrement,before,begin,between,by,cascade,case,cast,check,collate,column,commit,conflict,constraint,create,cross,current,current_date,current_time,current_timestamp,database,default,deferrable,deferred,delete,desc,detach,distinct,do,drop,each,else,end,escape,except,exclude,exclusive,exists,explain,fail,filter,first,following,for,foreign,from,full,generated,glob,group,groups,having,if,ignore,immediate,in,index,indexed,initially,inner,insert,instead,intersect,into,is,isnull,join,key,last,left,like,limit,match,materialized,natural,no,not,nothing,notnull,null,nulls,of,offset,on,or,order,others,outer,over,partition,plan,pragma,preceding,primary,query,raise,range,recursive,references,regexp,reindex,release,rename,replace,restrict,returning,right,rollback,row,rows,savepoint,select,set,table,temp,temporary,then,ties,to,transaction,trigger,unbounded,union,unique,update,using,vacuum,values,view,virtual,when,where,window,with,without,"

func sqliteReserved(s string) bool { return strings.Contains(kw, ","+s+",") }

func lexSQL(source []byte) ([]token, error) {
	if len(source) > maxInputBytes {
		return nil, errLimit
	}
	if len(source) == 0 || !utf8.Valid(source) || bytes.IndexByte(source, '\r') >= 0 {
		return nil, errMalformed
	}
	for _, c := range source {
		if c == 0 || c < 0x20 && !isJSONSpace(c) {
			return nil, errMalformed
		}
	}
	result := make([]token, 0, 128)
	appendToken := func(t token) error {
		if len(t.text) > maxStringBytes || len(result) == maxTokens {
			return errLimit
		}
		result = append(result, t)
		return nil
	}
	for i := 0; i < len(source); {
		c := source[i]
		if isJSONSpace(c) {
			i++
			continue
		}
		if i+1 < len(source) && c == '-' && source[i+1] == '-' {
			i += 2
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(source) && c == '/' && source[i+1] == '*' {
			end := bytes.Index(source[i+2:], []byte("*/"))
			if end < 0 {
				return nil, errMalformed
			}
			i += end + 4
			continue
		}
		if c == 0 || c < 0x20 || c >= 0x80 {
			return nil, errMalformed
		}
		start := i
		switch {
		case c == '\'':
			text, end, ok := scanSQLQuoted(source, i, '\'', '\'', true)
			if !ok {
				return nil, errMalformed
			}
			if err := appendToken(token{kind: tokString, text: text, start: start, end: end}); err != nil {
				return nil, err
			}
			i = end
		case c == '"' || c == '`':
			text, end, ok := scanSQLQuoted(source, i, c, c, c == '"')
			if !ok || !validSQLIdentifier(text) {
				return nil, errMalformed
			}
			if err := appendToken(token{kind: tokQuoted, text: text, norm: asciiFold(text), start: start, end: end}); err != nil {
				return nil, err
			}
			i = end
		case c == '[':
			end := bytes.IndexByte(source[i+1:], ']')
			if end < 0 {
				return nil, errMalformed
			}
			end += i + 2
			text := sourceString(source, i+1, end-1)
			if !validSQLIdentifier(text) {
				return nil, errMalformed
			}
			if err := appendToken(token{kind: tokQuoted, text: text, norm: asciiFold(text), start: start, end: end}); err != nil {
				return nil, err
			}
			i = end
		case isIdentStart(c):
			i++
			for i < len(source) && isIdentContinue(source[i]) {
				i++
			}
			word := sourceString(source, start, i)
			if len(word) == 1 && (word == "x" || word == "X") && i < len(source) && source[i] == '\'' {
				text, end, ok := scanSQLQuoted(source, i, '\'', '\'', true)
				if !ok || len(text)%2 != 0 {
					return nil, errMalformed
				}
				for j := range text {
					if !hexByte(text[j]) {
						return nil, errMalformed
					}
				}
				if err := appendToken(token{kind: tokBlob, text: text, start: start, end: end}); err != nil {
					return nil, err
				}
				i = end
				continue
			}
			if forbiddenKeyword(word) {
				return nil, errUnsupported
			}
			if err := appendToken(token{kind: tokIdent, text: word, norm: asciiFold(word), start: start, end: i}); err != nil {
				return nil, err
			}
		case isDigit(c) || (c == '.' && i+1 < len(source) && isDigit(source[i+1])):
			end, ok := scanNumber(source, i)
			if !ok {
				return nil, errMalformed
			}
			if err := appendToken(token{kind: tokNumber, text: sourceString(source, i, end), start: start, end: end}); err != nil {
				return nil, err
			}
			i = end
		default:
			var symbol string
			if i+1 < len(source) {
				two := sourceString(source, i, i+2)
				if twoSymbol(two) {
					symbol = two
					i += 2
				}
			}
			if symbol == "" {
				if !strings.ContainsRune("(),;.=+-*/%<>!|&~", rune(c)) {
					return nil, errUnsupported
				}
				symbol = sourceString(source, i, i+1)
				i++
			}
			if err := appendToken(token{kind: tokSymbol, text: symbol, start: start, end: i}); err != nil {
				return nil, err
			}
		}
	}
	if len(result) == 0 {
		return nil, errMalformed
	}
	linkParens(result)
	return result, nil
}
func twoSymbol(s string) bool {
	switch s {
	case "<=", ">=", "<>", "!=", "==", "||", "<<", ">>", "->":
		return true
	}
	return false
}
func isIdentStart(c byte) bool    { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' }
func isIdentContinue(c byte) bool { return isIdentStart(c) || isDigit(c) || c == '$' }
func isDigit(c byte) bool         { return c >= '0' && c <= '9' }
func validSQLIdentifier(s string) bool {
	if len(s) == 0 || len(s) > maxIdentifierBytes || !isIdentStart(s[0]) {
		return false
	}
	for i := range s {
		if !isIdentContinue(s[i]) {
			return false
		}
	}
	return true
}
func scanSQLQuoted(b []byte, start int, quote, escape byte, doubled bool) (string, int, bool) {
	var out strings.Builder
	for i := start + 1; i < len(b); i++ {
		c := b[i]
		if c < 0x20 || c >= 0x80 {
			return "", 0, false
		}
		if c != quote {
			out.WriteByte(c)
			continue
		}
		if doubled && i+1 < len(b) && b[i+1] == escape {
			out.WriteByte(escape)
			i++
			continue
		}
		return out.String(), i + 1, true
	}
	return "", 0, false
}
func scanNumber(b []byte, i int) (int, bool) {
	start := i
	if i+2 <= len(b) && b[i] == '0' && (b[i+1] == 'x' || b[i+1] == 'X') {
		i += 2
		d := i
		for i < len(b) && hexByte(b[i]) {
			i++
		}
		return i, i > d && i-d <= 16 && (i == len(b) || !isIdentContinue(b[i]))
	}
	if b[i] == '.' {
		i++
		for i < len(b) && isDigit(b[i]) {
			i++
		}
	} else {
		for i < len(b) && isDigit(b[i]) {
			i++
		}
		if i < len(b) && b[i] == '.' {
			i++
			for i < len(b) && isDigit(b[i]) {
				i++
			}
		}
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		d := i
		for i < len(b) && isDigit(b[i]) {
			i++
		}
		if d == i {
			return 0, false
		}
	}
	return i, i > start && (i == len(b) || !isIdentStart(b[i]))
}
func asciiFold(s string) string {
	b := []byte(s)
	changed := false
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
			changed = true
		}
	}
	if !changed {
		return s
	}
	return string(b)
}

func sourceString(source []byte, start, end int) string {
	return unsafe.String(unsafe.SliceData(source[start:end]), end-start)
}
func forbiddenKeyword(s string) bool {
	switch asciiFold(s) {
	case "attach", "detach", "vacuum", "load_extension", "serial", "engine", "auto_increment", "procedure", "execute", "copy", "merge", "replace", "analyze", "reindex", "createuser", "grant", "revoke", "call", "do", "delimiter", "outfile", "infile", "charset":
		return true
	}
	return false
}

type parser struct {
	ts      []token
	at      int
	request Request
	input   Input
	ordinal int
	emit    func(Fact, *schemaObject) error
}
type schemaObject struct{ name string }

func parseSQL(r Request, in Input, source []byte, emit func(Fact, *schemaObject) error) error {
	ts, err := lexSQL(source)
	if err != nil {
		return err
	}
	p := parser{ts: ts, request: r, input: in, emit: emit}
	for p.at < len(p.ts) {
		if p.ts[p.at].text == ";" {
			return errUnsupported
		}
		p.ordinal++
		if err := p.statement(); err != nil {
			return err
		}
	}
	return nil
}
func (p *parser) statement() error {
	t := p.ts[p.at]
	if t.keyword("CREATE") {
		return p.create()
	}
	if t.keyword("ALTER") {
		return p.alter()
	}
	if t.keyword("DROP") {
		return p.drop()
	}
	if p.input.Family != "sqlite.query" {
		return errUnsupported
	}
	if t.keyword("WITH") {
		return p.withStatement()
	}
	if t.oneOf("SELECT", "INSERT", "UPDATE", "DELETE", "PRAGMA") {
		value := t.norm
		var err error
		switch value {
		case "select":
			err = p.queryStatement()
		case "pragma":
			err = p.pragma()
		default:
			err = p.writeStatement(value)
		}
		if err != nil {
			return err
		}
		kind := "sqlite.query.write"
		if value == "select" || value == "pragma" {
			kind = "sqlite.query.read"
		}
		return p.emit(makeQueryFact(p.request, p.input, p.ordinal, kind, value), nil)
	}
	if t.oneOf("BEGIN", "COMMIT", "END", "ROLLBACK", "SAVEPOINT", "RELEASE") {
		return p.transaction()
	}
	return errUnsupported
}
func (p *parser) withStatement() error {
	end := p.semicolonAt()
	if end < 0 {
		return errMalformed
	}
	tail, ok := withTail(p.ts[p.at:end])
	if !ok {
		return errUnsupported
	}
	kind, fact := "", ""
	switch {
	case validSelect(tail):
		kind, fact = "sqlite.query.read", "select"
	case validInsert(tail):
		kind, fact = "sqlite.query.write", "insert"
	case validUpdate(tail):
		kind, fact = "sqlite.query.write", "update"
	case validDelete(tail):
		kind, fact = "sqlite.query.write", "delete"
	default:
		return errUnsupported
	}
	p.at = end + 1
	return p.emit(makeQueryFact(p.request, p.input, p.ordinal, kind, fact), nil)
}
func (p *parser) create() error {
	p.at++
	temporary := p.takeKeyword("TEMP") || p.takeKeyword("TEMPORARY")
	unique := p.takeKeyword("UNIQUE")
	virtual := p.takeKeyword("VIRTUAL")
	if p.takeKeyword("TABLE") {
		if unique || temporary && virtual {
			return errUnsupported
		}
		return p.createTable(virtual)
	}
	if virtual {
		return errUnsupported
	}
	if p.takeKeyword("INDEX") {
		if temporary {
			return errUnsupported
		}
		return p.createIndex()
	}
	if unique {
		return errUnsupported
	}
	if p.takeKeyword("VIEW") {
		return p.createView()
	}
	if p.takeKeyword("TRIGGER") {
		return p.createTrigger()
	}
	return errUnsupported
}
func (p *parser) createTable(virtual bool) error {
	p.takeIfNotExists()
	name, ok := p.qualifiedName()
	if !ok {
		return errUnsupported
	}
	if virtual {
		if !p.takeKeyword("USING") {
			return errUnsupported
		}
		if _, ok := p.name(); !ok {
			return errUnsupported
		}
		if p.peek("(") {
			args, ok := p.parenthesized()
			if !ok || !validVirtualTableArgs(args) {
				return errUnsupported
			}
		}
		if !p.semi() {
			return errUnsupported
		}
		return p.declare("sqlite.schema.virtual_table", "declares-virtual-table", name)
	}
	elems, ok := p.parenthesized()
	if !ok || !validTableElements(elems) {
		return errUnsupported
	}
	if !p.tableOptions() || !p.semi() {
		return errUnsupported
	}
	return p.declare("sqlite.schema.table", "declares-table", name)
}
func (p *parser) declare(kind, predicate string, name sqlName) error {
	return p.emit(makeFact(p.request, p.input, kind, name.display, predicate, name.display), &schemaObject{name.key})
}
func (p *parser) tableOptions() bool {
	option := func() byte {
		if p.takeKeyword("STRICT") {
			return 's'
		}
		if p.takeKeyword("WITHOUT") && p.takeKeyword("ROWID") {
			return 'w'
		}
		return 0
	}
	if p.peek(";") {
		return true
	}
	first := option()
	if first == 0 {
		return false
	}
	if p.peek(";") {
		return true
	}
	return p.take(",") && (first == 's' && option() == 'w' || first == 'w' && option() == 's') && p.peek(";")
}
func (p *parser) createIndex() error {
	p.takeIfNotExists()
	name, ok := p.qualifiedName()
	if !ok || !p.takeKeyword("ON") {
		return errUnsupported
	}
	if _, ok := p.name(); !ok {
		return errUnsupported
	}
	items, ok := p.parenthesized()
	if !ok || !validIndexItems(items) {
		return errUnsupported
	}
	if p.takeKeyword("WHERE") {
		start := p.at
		p.toSemi()
		if !exprOK(p.ts[start:p.at]) {
			return errUnsupported
		}
	}
	if !p.semi() {
		return errUnsupported
	}
	return p.declare("sqlite.schema.index", "declares-index", name)
}
func (p *parser) createView() error {
	p.takeIfNotExists()
	name, ok := p.qualifiedName()
	if !ok {
		return errUnsupported
	}
	if p.peek("(") {
		items, ok := p.parenthesized()
		if !ok || !validNameList(items) {
			return errUnsupported
		}
	}
	if !p.takeKeyword("AS") {
		return errUnsupported
	}
	end := p.semicolonAt()
	if end < 0 || !validQuery(p.ts[p.at:end]) {
		return errUnsupported
	}
	p.at = end + 1
	return p.declare("sqlite.schema.view", "declares-view", name)
}
func (p *parser) createTrigger() error {
	p.takeIfNotExists()
	name, ok := p.qualifiedName()
	if !ok {
		return errUnsupported
	}
	if !(p.takeKeyword("BEFORE") || p.takeKeyword("AFTER") || p.takeKeyword("INSTEAD")) {
		return errUnsupported
	}
	if p.ts[p.at-1].keyword("INSTEAD") && !p.takeKeyword("OF") {
		return errUnsupported
	}
	if p.takeKeyword("UPDATE") {
		if p.takeKeyword("OF") {
			if !p.nameListTo("ON") {
				return errUnsupported
			}
		}
	} else if !(p.takeKeyword("INSERT") || p.takeKeyword("DELETE")) {
		return errUnsupported
	}
	if !p.takeKeyword("ON") {
		return errUnsupported
	}
	if _, ok := p.qualifiedName(); !ok {
		return errUnsupported
	}
	if p.takeKeyword("FOR") {
		if !p.takeKeyword("EACH") || !p.takeKeyword("ROW") {
			return errUnsupported
		}
	}
	if p.takeKeyword("WHEN") {
		start := p.at
		for p.at < len(p.ts) && !p.ts[p.at].keyword("BEGIN") {
			p.at++
		}
		if !exprOK(p.ts[start:p.at]) {
			return errUnsupported
		}
	}
	if !p.takeKeyword("BEGIN") {
		return errUnsupported
	}
	if p.peekKeyword("END") {
		return errUnsupported
	}
	for {
		if p.at >= len(p.ts) {
			return errMalformed
		}
		if p.takeKeyword("END") {
			if !p.semi() {
				return errUnsupported
			}
			break
		}
		if err := p.triggerBodyStatement(); err != nil {
			return err
		}
	}
	return p.declare("sqlite.schema.trigger", "declares-trigger", name)
}
func (p *parser) triggerBodyStatement() error {
	t := p.ts[p.at]
	if t.oneOf("INSERT", "UPDATE", "DELETE") {
		end := p.semicolonAt()
		if end < 0 {
			return errMalformed
		}
		body, target := p.ts[p.at:end], 2
		if t.keyword("UPDATE") {
			target = 1
		}
		if len(body) > 1 && body[1].keyword("OR") {
			target += 2
		}
		if target+1 < len(body) && body[target+1].text == "." {
			return errUnsupported
		}
		if t.keyword("INSERT") {
			for i := target + 1; i+1 < len(body); i++ {
				if body[i].keyword("DEFAULT") && body[i+1].keyword("VALUES") {
					return errUnsupported
				}
			}
		}
		return p.writeStatement(t.norm)
	}
	if t.keyword("SELECT") {
		return p.queryStatement()
	}
	return errUnsupported
}
func (p *parser) alter() error {
	p.at++
	if !p.takeKeyword("TABLE") {
		return errUnsupported
	}
	table, ok := p.qualifiedName()
	if !ok {
		return errUnsupported
	}
	if p.takeKeyword("ADD") {
		p.takeKeyword("COLUMN")
		start := p.at
		p.toSemi()
		if !validColumn(p.ts[start:p.at]) || !p.semi() {
			return errUnsupported
		}
		column, ok := p.ts[start].name()
		if !ok {
			return errUnsupported
		}
		return p.emit(makeFact(p.request, p.input, "sqlite.schema.column", table.display, "adds-column", column.display), nil)
	}
	if !p.takeKeyword("RENAME") {
		return errUnsupported
	}
	if p.takeKeyword("COLUMN") {
		if _, ok := p.name(); !ok {
			return errUnsupported
		}
	}
	if !p.takeKeyword("TO") {
		return errUnsupported
	}
	if _, ok := p.name(); !ok || !p.semi() {
		return errUnsupported
	}
	return nil
}
func (p *parser) drop() error {
	p.at++
	kind := ""
	for _, k := range []string{"TABLE", "INDEX", "TRIGGER", "VIEW"} {
		if p.takeKeyword(k) {
			kind = asciiFold(k)
			break
		}
	}
	if kind == "" {
		return errUnsupported
	}
	p.takeIfExists()
	name, ok := p.qualifiedName()
	if !ok || !p.semi() {
		return errUnsupported
	}
	return p.emit(makeFact(p.request, p.input, "sqlite.schema.drop", name.display, "drops-"+kind, name.display), nil)
}
func (p *parser) queryStatement() error {
	end := p.semicolonAt()
	if end < 0 || !validQuery(p.ts[p.at:end]) {
		return errUnsupported
	}
	p.at = end + 1
	return nil
}
func (p *parser) writeStatement(kind string) error {
	end := p.semicolonAt()
	if end < 0 {
		return errMalformed
	}
	slice := p.ts[p.at:end]
	ok := false
	switch kind {
	case "insert":
		ok = validInsert(slice)
	case "update":
		ok = validUpdate(slice)
	case "delete":
		ok = validDelete(slice)
	}
	if !ok {
		return errUnsupported
	}
	p.at = end + 1
	return nil
}
func (p *parser) pragma() error {
	p.at++
	if !p.takeKeyword("FOREIGN_KEYS") {
		return errUnsupported
	}
	if p.take("=") {
		start := p.at
		p.toSemi()
		if !pragmaValue(p.ts[start:p.at]) {
			return errUnsupported
		}
	} else if p.peek("(") {
		items, ok := p.parenthesized()
		if !ok || !pragmaValue(items) {
			return errUnsupported
		}
	}
	if !p.semi() {
		return errUnsupported
	}
	return nil
}
func (p *parser) transaction() error {
	first := p.ts[p.at].norm
	p.at++
	switch first {
	case "begin":
		_ = p.takeKeyword("DEFERRED") || p.takeKeyword("IMMEDIATE") || p.takeKeyword("EXCLUSIVE")
		if p.peekKeyword("DEFERRED") || p.peekKeyword("IMMEDIATE") || p.peekKeyword("EXCLUSIVE") {
			return errUnsupported
		}
		p.takeKeyword("TRANSACTION")
	case "commit", "end":
		p.takeKeyword("TRANSACTION")
	case "rollback":
		p.takeKeyword("TRANSACTION")
		if p.takeKeyword("TO") {
			p.takeKeyword("SAVEPOINT")
			if _, ok := p.name(); !ok {
				return errUnsupported
			}
		}
	case "savepoint":
		if _, ok := p.name(); !ok {
			return errUnsupported
		}
	case "release":
		p.takeKeyword("SAVEPOINT")
		if _, ok := p.name(); !ok {
			return errUnsupported
		}
	}
	if !p.semi() {
		return errUnsupported
	}
	return nil
}
func (p *parser) takeKeyword(word string) bool {
	if p.at < len(p.ts) && p.ts[p.at].keyword(word) {
		p.at++
		return true
	}
	return false
}
func (p *parser) take(s string) bool {
	if p.at < len(p.ts) && p.ts[p.at].text == s {
		p.at++
		return true
	}
	return false
}
func (p *parser) peek(s string) bool { return p.at < len(p.ts) && p.ts[p.at].text == s }
func (p *parser) peekKeyword(word string) bool {
	return p.at < len(p.ts) && p.ts[p.at].keyword(word)
}
func (p *parser) semi() bool { return p.take(";") }
func (p *parser) takeIfNotExists() {
	if p.takeKeyword("IF") {
		if !p.takeKeyword("NOT") || !p.takeKeyword("EXISTS") {
			p.at = len(p.ts)
		}
	}
}
func (p *parser) takeIfExists() {
	if p.takeKeyword("IF") {
		if !p.takeKeyword("EXISTS") {
			p.at = len(p.ts)
		}
	}
}
func (p *parser) name() (sqlName, bool) {
	if p.at >= len(p.ts) {
		return sqlName{}, false
	}
	n, ok := p.ts[p.at].name()
	if ok {
		p.at++
	}
	return n, ok
}
func (p *parser) qualifiedName() (sqlName, bool) {
	name, at, ok := qualifiedAt(p.ts, p.at)
	if ok {
		p.at = at
	}
	return name, ok
}
func (p *parser) parenthesized() ([]token, bool) {
	end, ok := parenAt(p.ts, p.at)
	if !ok || end-p.at > maxTokens {
		return nil, false
	}
	items := p.ts[p.at+1 : end-1]
	p.at = end
	return items, true
}
func (p *parser) semicolonAt() int {
	for i := p.at; i < len(p.ts); i++ {
		if p.ts[i].text == ";" {
			return i
		}
	}
	return -1
}
func (p *parser) toSemi() {
	for p.at < len(p.ts) && p.ts[p.at].text != ";" {
		p.at++
	}
}
func (p *parser) nameListTo(stop string) bool {
	start := p.at
	for p.at < len(p.ts) && !p.ts[p.at].keyword(stop) {
		p.at++
	}
	return validNameList(p.ts[start:p.at])
}
func splitTop(ts []token, delimiter string) ([][]token, bool) {
	var out [][]token
	start, depth := 0, 0
	for i, t := range ts {
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
			if depth < 0 {
				return nil, false
			}
		default:
			if t.text == delimiter && depth == 0 {
				if i == start {
					return nil, false
				}
				out = append(out, ts[start:i])
				start = i + 1
			}
		}
	}
	if depth != 0 || start == len(ts) {
		return nil, false
	}
	return append(out, ts[start:]), true
}
func validNameList(ts []token) bool {
	parts, ok := splitTop(ts, ",")
	if !ok {
		return false
	}
	for _, part := range parts {
		if len(part) != 1 {
			return false
		}
		if _, ok := part[0].name(); !ok {
			return false
		}
	}
	return true
}
func validTableElements(ts []token) bool {
	parts, ok := splitTop(ts, ",")
	if !ok {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 {
			return false
		}
		if part[0].oneOf("CONSTRAINT", "PRIMARY", "UNIQUE", "CHECK", "FOREIGN") {
			if !validTableConstraint(part) {
				return false
			}
		} else if !validColumn(part) {
			return false
		}
	}
	return true
}
func validVirtualTableArgs(ts []token) bool {
	parts, ok := splitTop(ts, ",")
	if !ok {
		return false
	}
	for _, part := range parts {
		name, ok := part[0].name()
		if !ok || name.key == "" || len(part) > 3 || len(part) == 2 && !part[1].keyword("UNINDEXED") || len(part) == 3 && (part[1].text != "=" || !validVirtualOptionValue(part[2])) {
			return false
		}
	}
	return true
}
func validVirtualOptionValue(t token) bool {
	return t.kind == tokIdent || t.kind == tokQuoted || t.kind == tokString || t.kind == tokNumber
}
func validColumn(ts []token) bool {
	if len(ts) < 1 {
		return false
	}
	if _, ok := ts[0].name(); !ok {
		return false
	}
	i := 1
	for i < len(ts) && !columnConstraintStart(ts[i]) {
		i++
	}
	columnType := ts[1:i]
	if !validTypeName(columnType) {
		return false
	}
	for i < len(ts) {
		next, ok := consumeColumnConstraint(ts, i, columnType)
		if !ok || next <= i {
			return false
		}
		i = next
	}
	return true
}
func validTypeName(ts []token) bool {
	i := 0
	for i < len(ts) {
		if _, ok := ts[i].name(); !ok {
			break
		}
		i++
	}
	if i == 0 {
		return len(ts) == 0
	}
	if i == len(ts) {
		return true
	}
	if ts[i].text != "(" {
		return false
	}
	i++
	i, ok := signedNumberAt(ts, i)
	if !ok {
		return false
	}
	if i < len(ts) && ts[i].text == "," {
		i, ok = signedNumberAt(ts, i+1)
		if !ok {
			return false
		}
	}
	return i+1 == len(ts) && ts[i].text == ")"
}
func signedNumberAt(ts []token, i int) (int, bool) {
	if i < len(ts) && (ts[i].text == "+" || ts[i].text == "-") {
		i++
	}
	if i >= len(ts) || ts[i].kind != tokNumber {
		return 0, false
	}
	return i + 1, true
}
func columnConstraintStart(t token) bool {
	return t.oneOf("CONSTRAINT", "PRIMARY", "NOT", "UNIQUE", "CHECK", "DEFAULT", "COLLATE", "REFERENCES", "GENERATED", "AS")
}
func consumeColumnConstraint(ts []token, i int, columnType []token) (int, bool) {
	if ts[i].keyword("CONSTRAINT") {
		if i+1 >= len(ts) {
			return 0, false
		}
		if _, ok := ts[i+1].name(); !ok {
			return 0, false
		}
		i += 2
		if i == len(ts) {
			return 0, false
		}
	}
	if ts[i].oneOf("PRIMARY", "UNIQUE") {
		primary := ts[i].keyword("PRIMARY")
		if primary {
			i++
			if i == len(ts) || !ts[i].keyword("KEY") {
				return 0, false
			}
		}
		i++
		if i < len(ts) && ts[i].oneOf("ASC", "DESC") {
			i++
		}
		if i < len(ts) && ts[i].keyword("ON") {
			i++
			if i >= len(ts) || !ts[i].keyword("CONFLICT") {
				return 0, false
			}
			i++
			if i >= len(ts) || !conflictAction(ts[i]) {
				return 0, false
			}
			i++
		}
		if i < len(ts) && ts[i].keyword("AUTOINCREMENT") {
			if !primary || len(columnType) != 1 || !columnType[0].keyword("INTEGER") {
				return 0, false
			}
			i++
		}
		return i, true
	}
	if ts[i].keyword("NOT") {
		if i+1 >= len(ts) || !ts[i+1].keyword("NULL") {
			return 0, false
		}
		i += 2
		return consumeConflict(ts, i)
	}
	if ts[i].oneOf("CHECK", "AS") {
		return parenExpr(ts, i+1)
	}
	if ts[i].keyword("DEFAULT") {
		end := exprOne(ts, i+1)
		return end, end > i+1
	}
	if ts[i].keyword("COLLATE") {
		i++
		if i >= len(ts) {
			return 0, false
		}
		_, ok := ts[i].name()
		return i + 1, ok
	}
	if ts[i].keyword("REFERENCES") {
		return consumeReferenceTail(ts, i+1)
	}
	if ts[i].keyword("GENERATED") {
		i++
		if i < len(ts) && ts[i].keyword("ALWAYS") {
			i++
		}
		if i >= len(ts) || !ts[i].keyword("AS") {
			return 0, false
		}
		end, ok := parenExpr(ts, i+1)
		if !ok {
			return 0, false
		}
		i = end
		if i < len(ts) && ts[i].oneOf("VIRTUAL", "STORED") {
			i++
		}
		return i, true
	}
	return 0, false
}
func consumeConflict(ts []token, i int) (int, bool) {
	if i == len(ts) || !ts[i].keyword("ON") {
		return i, true
	}
	if i+2 >= len(ts) || !ts[i+1].keyword("CONFLICT") || !conflictAction(ts[i+2]) {
		return 0, false
	}
	return i + 3, true
}
func conflictAction(t token) bool {
	return t.oneOf("ROLLBACK", "ABORT", "FAIL", "IGNORE", "REPLACE")
}
func validTableConstraint(ts []token) bool {
	i := 0
	if ts[i].keyword("CONSTRAINT") {
		if len(ts) < 3 {
			return false
		}
		if _, ok := ts[1].name(); !ok {
			return false
		}
		i = 2
	}
	if i >= len(ts) {
		return false
	}
	if ts[i].oneOf("PRIMARY", "UNIQUE") {
		if ts[i].keyword("PRIMARY") {
			i++
			if i >= len(ts) || !ts[i].keyword("KEY") {
				return false
			}
		}
		i++
		end, ok := parenAt(ts, i)
		if !ok || !validIndexItems(ts[i+1:end-1]) {
			return false
		}
		i = end
		i, ok = consumeConflict(ts, i)
		return ok && i == len(ts)
	}
	if ts[i].keyword("CHECK") {
		end, ok := parenExpr(ts, i+1)
		return ok && end == len(ts)
	}
	if ts[i].keyword("FOREIGN") {
		if i+1 >= len(ts) || !ts[i+1].keyword("KEY") {
			return false
		}
		end, ok := parenAt(ts, i+2)
		if !ok || !validNameList(ts[i+3:end-1]) {
			return false
		}
		return validForeignTail(ts[end:])
	}
	return false
}
func consumeReferenceTail(ts []token, i int) (int, bool) {
	_, i, ok := qualifiedAt(ts, i)
	if !ok {
		return 0, false
	}
	if i < len(ts) && ts[i].text == "(" {
		end, ok := parenAt(ts, i)
		if !ok || !validNameList(ts[i+1:end-1]) {
			return 0, false
		}
		i = end
	}
	deferred := false
	for i < len(ts) {
		if ts[i].keyword("ON") {
			if i+2 >= len(ts) || !ts[i+1].oneOf("DELETE", "UPDATE") {
				return 0, false
			}
			i += 2
			if ts[i].keyword("SET") {
				if i+1 >= len(ts) || !ts[i+1].oneOf("NULL", "DEFAULT") {
					return 0, false
				}
				i += 2
			} else if ts[i].oneOf("CASCADE", "RESTRICT") {
				i++
			} else if ts[i].keyword("NO") && i+1 < len(ts) && ts[i+1].keyword("ACTION") {
				i += 2
			} else {
				return 0, false
			}
			continue
		}
		if ts[i].keyword("MATCH") {
			if i+1 >= len(ts) {
				return 0, false
			}
			if _, ok := ts[i+1].name(); !ok {
				return 0, false
			}
			i += 2
			continue
		}
		if !deferred && ts[i].keyword("NOT") {
			if i+1 >= len(ts) || !ts[i+1].keyword("DEFERRABLE") {
				return 0, false
			}
			deferred = true
			i += 2
		} else if !deferred && ts[i].keyword("DEFERRABLE") {
			deferred = true
			i++
		} else {
			break
		}
		if i < len(ts) && ts[i].keyword("INITIALLY") {
			if i+1 >= len(ts) || !ts[i+1].oneOf("DEFERRED", "IMMEDIATE") {
				return 0, false
			}
			i += 2
		}
	}
	return i, true
}
func validForeignTail(ts []token) bool {
	if len(ts) == 0 || !ts[0].keyword("REFERENCES") {
		return false
	}
	i, ok := consumeReferenceTail(ts, 1)
	return ok && i == len(ts)
}
func validIndexItems(ts []token) bool {
	parts, ok := splitTop(ts, ",")
	if !ok {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 {
			return false
		}
		end := len(part)
		if end >= 1 && part[end-1].oneOf("ASC", "DESC") {
			end--
		}
		if end >= 2 && part[end-2].keyword("COLLATE") {
			if _, ok := part[end-1].name(); !ok {
				return false
			}
			end -= 2
		}
		if !exprOK(part[:end]) {
			return false
		}
	}
	return true
}
func linkParens(ts []token) int {
	at := -1
	for i := range ts {
		if ts[i].text == "(" {
			ts[i].close, at = int16(at), i
		}
		if ts[i].text == ")" && at >= 0 {
			n := ts[at].close
			ts[at].close, at = int16(i+1-at), int(n)
		}
	}
	return len(ts)
}
func parenAt(ts []token, at int) (int, bool) {
	if at < len(ts) && ts[at].text == "(" {
		end := at + int(ts[at].close)
		return end, end > at && end <= len(ts)
	}
	return 0, false
}
func parenExpr(ts []token, start int) (int, bool) {
	end, ok := parenAt(ts, start)
	return end, ok && exprOK(ts[start+1:end-1])
}
func qualifiedAt(ts []token, start int) (sqlName, int, bool) {
	if start >= len(ts) {
		return sqlName{}, 0, false
	}
	a, ok := ts[start].name()
	if !ok {
		return sqlName{}, 0, false
	}
	if start+1 < len(ts) && ts[start+1].text == "." {
		if start+2 >= len(ts) {
			return sqlName{}, 0, false
		}
		b, ok := ts[start+2].name()
		if !ok {
			return sqlName{}, 0, false
		}
		return sqlName{display: a.display + "." + b.display, key: a.key + "." + b.key}, start + 3, true
	}
	return a, start + 1, true
}
func validQuery(ts []token) bool {
	tail, ok := withTail(ts)
	return ok && validSelect(tail)
}
func withTail(ts []token) ([]token, bool) {
	if len(ts) == 0 || !ts[0].keyword("WITH") {
		return ts, len(ts) != 0
	}
	i := 1
	if i < len(ts) && ts[i].keyword("RECURSIVE") {
		i++
	}
	for {
		if i >= len(ts) {
			return nil, false
		}
		if _, ok := ts[i].name(); !ok {
			return nil, false
		}
		i++
		if i < len(ts) && ts[i].text == "(" {
			end, ok := parenAt(ts, i)
			if !ok || !validNameList(ts[i+1:end-1]) {
				return nil, false
			}
			i = end
		}
		if i >= len(ts) || !ts[i].keyword("AS") {
			return nil, false
		}
		i++
		if i+1 < len(ts) && ts[i].keyword("NOT") && ts[i+1].keyword("MATERIALIZED") {
			i += 2
		} else if i < len(ts) && ts[i].keyword("MATERIALIZED") {
			i++
		}
		end, ok := parenAt(ts, i)
		if !ok || !validQuery(ts[i+1:end-1]) {
			return nil, false
		}
		i = end
		if i < len(ts) && ts[i].text == "," {
			i++
			continue
		}
		return ts[i:], i < len(ts)
	}
}
func validSelect(ts []token) bool {
	if len(ts) < 2 || !ts[0].keyword("SELECT") {
		return false
	}
	parts, ok := topClauses(ts[1:])
	if !ok {
		return false
	}
	if !results(parts[selectClause]) {
		return false
	}
	if v := parts[fromClause]; v != nil && !fromOK(v) {
		return false
	}
	for _, key := range []clauseKind{whereClause, havingClause} {
		if v := parts[key]; v != nil && !exprOK(v) {
			return false
		}
	}
	if parts[windowClause] != nil {
		return false
	}
	for _, key := range []clauseKind{groupClause, orderClause} {
		if v := parts[key]; v != nil && (len(v) < 2 || !v[0].keyword("BY") || !exprs(v[1:])) {
			return false
		}
	}
	if v := parts[limitClause]; v != nil && !exprOK(v) {
		return false
	}
	if v := parts[compoundClause]; v != nil {
		if len(v) < 2 || !(v[0].keyword("UNION") || v[0].keyword("INTERSECT") || v[0].keyword("EXCEPT")) {
			return false
		}
		j := 1
		if v[0].keyword("UNION") && j < len(v) && v[j].keyword("ALL") {
			j++
		}
		return validSelect(v[j:])
	}
	return true
}

type clauseKind uint8

const (
	selectClause clauseKind = iota
	fromClause
	whereClause
	groupClause
	havingClause
	windowClause
	orderClause
	limitClause
	compoundClause
)

func topClauses(ts []token) ([9][]token, bool) {
	var out [9][]token
	start, depth := 0, 0
	current := selectClause
	lastOrder := 0
	seen := uint16(1 << selectClause)
	for i, t := range ts {
		if t.text == "(" {
			depth++
			continue
		}
		if t.text == ")" {
			depth--
			if depth < 0 {
				return out, false
			}
			continue
		}
		if depth != 0 {
			continue
		}
		key, order := clause(t)
		if order != 0 {
			if order <= lastOrder {
				return out, false
			}
			if seen&(1<<key) != 0 || i == start {
				return out, false
			}
			out[current] = ts[start:i]
			current = key
			start = i + 1
			lastOrder = order
			seen |= 1 << key
		}
	}
	if depth != 0 || start == len(ts) {
		return out, false
	}
	out[current] = ts[start:]
	return out, true
}
func clause(t token) (clauseKind, int) {
	if t.kind != tokIdent {
		return 0, 0
	}
	switch t.norm {
	case "from":
		return fromClause, 1
	case "where":
		return whereClause, 2
	case "group":
		return groupClause, 3
	case "having":
		return havingClause, 4
	case "window":
		return windowClause, 5
	case "order":
		return orderClause, 6
	case "limit":
		return limitClause, 7
	case "union", "intersect", "except":
		return compoundClause, 8
	}
	return 0, 0
}
func results(ts []token) bool {
	if len(ts) > 0 && (ts[0].keyword("DISTINCT") || ts[0].keyword("ALL")) {
		ts = ts[1:]
	}
	return result(ts) || partsOK(ts, result)
}
func result(ts []token) bool {
	if isStar(ts) || exprOK(ts) {
		return true
	}
	i := len(ts) - 1
	if i < 1 {
		return false
	}
	if _, ok := ts[i].name(); !ok {
		return false
	}
	if ts[i-1].keyword("AS") {
		i--
	}
	return exprOK(ts[:i])
}
func fromOK(ts []token) bool {
	return partsOK(ts, source)
}
func source(ts []token) bool {
	_, i, ok := qualifiedAt(ts, 0)
	if !ok {
		return false
	}
	if i == len(ts) {
		return true
	}
	if ts[i].keyword("AS") {
		i++
	}
	if i+1 != len(ts) {
		return false
	}
	_, ok = ts[i].name()
	return ok
}
func exprOK(ts []token) bool {
	depth, want, esc := 0, true, false
	for i := 0; i < len(ts); i++ {
		t := ts[i]
		if want {
			if t.text == "+" || t.text == "-" || t.text == "~" || t.keyword("NOT") {
				continue
			}
			if t.text == "(" {
				depth++
				continue
			}
			_, name := t.name()
			if !name && !literal(t) {
				return false
			}
			want = false
			if name && i+1 < len(ts) && ts[i+1].text == "(" {
				if forbiddenKeyword(t.norm) {
					return false
				}
				if i+2 < len(ts) && ts[i+2].text == ")" {
					i += 2
				} else if i+3 < len(ts) && ts[i+2].text == "*" && ts[i+3].text == ")" {
					i += 3
				} else {
					i++
					depth++
					want = true
				}
			}
			continue
		}
		if t.text == ")" {
			if depth == 0 {
				return false
			}
			depth--
			continue
		}
		if t.text == "," {
			if depth == 0 {
				return false
			}
			want, esc = true, false
			continue
		}
		if t.text == "." {
			if i+1 >= len(ts) {
				return false
			}
			if _, ok := ts[i-1].name(); !ok {
				return false
			}
			if _, ok := ts[i+1].name(); !ok {
				return false
			}
			i++
			continue
		}
		if t.keyword("ESCAPE") {
			if !esc {
				return false
			}
			want, esc = true, false
			continue
		}
		if t.keyword("NOT") && i+1 < len(ts) && ts[i+1].oneOf("LIKE", "GLOB", "MATCH", "REGEXP") {
			i++
			want, esc = true, ts[i].keyword("LIKE")
			continue
		}
		if binOp(t.text) || t.oneOf("AND", "OR", "IS", "LIKE", "GLOB", "MATCH", "REGEXP") {
			want, esc = true, t.keyword("LIKE")
			continue
		}
		return false
	}
	return depth == 0 && !want
}
func binOp(s string) bool {
	switch s {
	case "=", "==", "!=", "<>", "<", ">", "<=", ">=", "+", "-", "*", "/", "%", "||", "|", "&", "<<", ">>", "->":
		return true
	}
	return false
}
func exprs(ts []token) bool {
	return partsOK(ts, exprOK)
}
func partsOK(ts []token, f func([]token) bool) bool {
	parts, ok := splitTop(ts, ",")
	if !ok {
		return false
	}
	for _, part := range parts {
		if !f(part) {
			return false
		}
	}
	return true
}
func isStar(ts []token) bool {
	if len(ts) == 1 {
		return ts[0].text == "*"
	}
	_, ok := ts[0].name()
	return ok && len(ts) == 3 && ts[1].text == "." && ts[2].text == "*"
}
func validInsert(ts []token) bool {
	if len(ts) < 4 || !ts[0].keyword("INSERT") {
		return false
	}
	i := 1
	if i+1 < len(ts) && ts[i].keyword("OR") && conflictAction(ts[i+1]) {
		i += 2
	}
	if i >= len(ts) || !ts[i].keyword("INTO") {
		return false
	}
	_, i, ok := qualifiedAt(ts, i+1)
	if !ok {
		return false
	}
	if i < len(ts) && ts[i].text == "(" {
		end, ok := parenAt(ts, i)
		if !ok || !validNameList(ts[i+1:end-1]) {
			return false
		}
		i = end
	}
	if i >= len(ts) {
		return false
	}
	if !ts[i].keyword("VALUES") {
		return ts[i].keyword("DEFAULT") && i+2 == len(ts) && ts[i+1].keyword("VALUES") || validQuery(ts[i:])
	}
	for i++; ; {
		end, ok := parenAt(ts, i)
		if !ok || !exprs(ts[i+1:end-1]) {
			return false
		}
		i = end
		if i == len(ts) {
			return true
		}
		if ts[i].text != "," {
			return validReturning(ts[i:])
		}
		i++
	}
}
func validUpdate(ts []token) bool {
	if len(ts) < 5 || !ts[0].keyword("UPDATE") {
		return false
	}
	i := 1
	if i+1 < len(ts) && ts[i].keyword("OR") && conflictAction(ts[i+1]) {
		i += 2
	}
	_, i, ok := qualifiedAt(ts, i)
	if !ok || i >= len(ts) || !ts[i].keyword("SET") {
		return false
	}
	i++
	parts := topWriteClauses(ts[i:], []string{"from", "where", "returning"})
	if parts == nil || !validAssignments(parts["body"]) {
		return false
	}
	if v := parts["from"]; v != nil && !fromOK(v) {
		return false
	}
	if v := parts["where"]; v != nil && !exprOK(v) {
		return false
	}
	return validReturning(parts["returning"])
}
func validDelete(ts []token) bool {
	if len(ts) < 3 || !ts[0].keyword("DELETE") || !ts[1].keyword("FROM") {
		return false
	}
	_, i, ok := qualifiedAt(ts, 2)
	if !ok {
		return false
	}
	parts := topWriteClauses(ts[i:], []string{"where", "returning"})
	if parts == nil || len(parts["body"]) != 0 {
		return false
	}
	if v := parts["where"]; v != nil && !exprOK(v) {
		return false
	}
	return validReturning(parts["returning"])
}

func topWriteClauses(ts []token, a []string) map[string][]token {
	out := map[string][]token{}
	s, d, last := 0, 0, 0
	cur := "body"
	for i, t := range ts {
		if t.text == "(" {
			d++
			continue
		}
		if t.text == ")" {
			d--
			if d < 0 {
				return nil
			}
			continue
		}
		if d != 0 {
			continue
		}
		k, order := "", 0
		for n, name := range a {
			if t.keyword(name) {
				k, order = name, n+1
			}
		}
		if k != "" {
			if order <= last || (i == s && cur != "body") {
				return nil
			}
			out[cur] = ts[s:i]
			if k == "returning" {
				out[k] = ts[i:]
				return out
			}
			cur, last = k, order
			s = i + 1
		}
	}
	if d != 0 {
		return nil
	}
	if s < len(ts) {
		out[cur] = ts[s:]
	} else if cur != "body" {
		return nil
	}
	return out
}
func validAssignments(ts []token) bool {
	parts, ok := splitTop(ts, ",")
	if !ok {
		return false
	}
	for _, part := range parts {
		if len(part) < 3 || part[1].text != "=" {
			return false
		}
		if _, ok := part[0].name(); !ok || !exprOK(part[2:]) {
			return false
		}
	}
	return true
}
func returningClauseFor(ts []token) ([]token, bool) {
	ok := len(ts) == 0 || len(ts) > 1 && ts[0].keyword("RETURNING") && ts[0].norm == asciiFold(ts[0].text) && exprs(ts[1:])
	return ts, ok
}
func validReturning(ts []token) bool { _, ok := returningClauseFor(ts); return ok }
func pragmaValue(ts []token) bool {
	return len(ts) == 1 && strings.Contains(",0,1,on,off,true,false,yes,no,", ","+asciiFold(ts[0].text)+",")
}
func exprOne(ts []token, start int) int {
	if start >= len(ts) {
		return start
	}
	if ts[start].text == "(" {
		end, ok := parenAt(ts, start)
		if ok && exprOK(ts[start+1:end-1]) {
			return end
		}
		return start
	}
	if ts[start].text == "+" || ts[start].text == "-" {
		if start+1 < len(ts) && ts[start+1].kind == tokNumber {
			return start + 2
		}
		return start
	}
	if literal(ts[start]) {
		return start + 1
	}
	return start
}
func literal(t token) bool {
	return t.kind == tokString || t.kind == tokNumber || t.kind == tokBlob || t.oneOf("NULL", "CURRENT_TIME", "CURRENT_DATE", "CURRENT_TIMESTAMP")
}
func makeFact(r Request, in Input, kind, subject, predicate, value string) Fact {
	f := Fact{Kind: kind, InputHandle: in.Handle, RelatedHandle: "-", Subject: subject, Predicate: predicate, Value: value, InstanceID: r.ScopeID}
	f.EvidenceSHA256 = evidence(r, in, f)
	return f
}
func makeQueryFact(r Request, in Input, ordinal int, kind, value string) Fact {
	f := Fact{Kind: kind, InputHandle: in.Handle, RelatedHandle: "-", Subject: "query", Predicate: "uses-statement", Value: value, InstanceID: fmt.Sprintf("%s:%d", r.ScopeID, ordinal)}
	f.EvidenceSHA256 = evidence(r, in, f)
	return f
}
func compareFact(a, b Fact) int {
	return comparePairs([][2]string{{a.Kind, b.Kind}, {a.InputHandle, b.InputHandle}, {a.RelatedHandle, b.RelatedHandle}, {a.Subject, b.Subject}, {a.Predicate, b.Predicate}, {a.Value, b.Value}, {a.InstanceID, b.InstanceID}, {a.EvidenceSHA256, b.EvidenceSHA256}})
}
func evidence(r Request, in Input, f Fact) string {
	target := `{"os":"` + r.Target.OS + `","architecture":"` + r.Target.Architecture + `","abi":"` + r.Target.ABI + `","features":[]}`
	fields := [...]string{r.Family, r.RequestID, r.ScopeID, r.CompilationUnitID, target, f.InputHandle, in.SHA256, f.RelatedHandle, "-", f.Kind, f.Subject, f.Predicate, f.Value, f.InstanceID}
	h := sha256.New()
	_, _ = h.Write([]byte("corvint-analyzer-candidate-evidence/sqlite-3.51.0-source-v1"))
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(len(fields)))
	_, _ = h.Write(b[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(b[:], uint32(len(field)))
		_, _ = h.Write(b[:])
		_, _ = h.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
func EncodeRequest(r Request) []byte { return marshal(r) }
func RequestForInput(handle, family, path string, content []byte) Input {
	sum := sha256.Sum256(content)
	return Input{Handle: handle, Family: family, Path: path, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(content)}
}
func (r Request) String() string { return fmt.Sprintf("%s/%s/%s", r.Profile, r.Family, r.RequestID) }
