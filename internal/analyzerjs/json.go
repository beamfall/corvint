package analyzerjs

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxRawRequestBytes   = 1_500_000
	maxContentBase64Size = 1_398_104
)

func decodeObject(raw string, dependencies *int, lock bool) (map[string]any, error) {
	return decodeObjectBounded(raw, MaxDepth, MaxFacts*8, dependencies, lock)
}
func decodeObjectBounded(raw string, maxDepth, maxTokens int, dependencies *int, lock bool) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	state := decodeState{limit: maxTokens, dependencies: dependencies, lock: lock}
	v, err := decodeValue(decoder, 0, maxDepth, &state, jsonRoot)
	if err != nil {
		return nil, err
	}
	if _, err = decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing JSON")
	}
	o, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON object required")
	}
	return o, nil
}

type decodeState struct {
	values, limit, packages int
	dependencies            *int
	lock                    bool
}

func (s *decodeState) next() error {
	s.values++
	if s.values > s.limit {
		return fmt.Errorf("JSON value budget")
	}
	return nil
}

type jsonContext byte

const (
	jsonGeneric jsonContext = iota
	jsonRoot
	jsonDependencies
	jsonPackages
	jsonPackage
)

func decodeValue(d *json.Decoder, depth, maxDepth int, state *decodeState, context jsonContext) (any, error) {
	if depth > maxDepth {
		return nil, fmt.Errorf("JSON depth")
	}
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch x := t.(type) {
	case json.Delim:
		switch x {
		case '{':
			out := map[string]any{}
			for d.More() {
				if err := state.next(); err != nil {
					return nil, err
				}
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				key, ok := k.(string)
				if !ok {
					return nil, fmt.Errorf("invalid JSON key")
				}
				if _, exists := out[key]; exists {
					return nil, fmt.Errorf("duplicate or invalid JSON key")
				}
				if context == jsonDependencies {
					if state.dependencies != nil {
						if *state.dependencies == MaxFacts {
							return nil, reject("LIMIT_EXCEEDED")
						}
						*state.dependencies++
					}
				} else if context == jsonPackages {
					state.packages++
					if state.packages > MaxFacts {
						return nil, reject("LIMIT_EXCEEDED")
					}
				}
				next := jsonGeneric
				if (context == jsonRoot || context == jsonPackage) && slices.Contains(dependencyGroups[:], key) {
					next = jsonDependencies
				} else if state.lock && context == jsonRoot && key == "packages" {
					next = jsonPackages
				} else if context == jsonPackages {
					next = jsonPackage
				}
				v, e := decodeValue(d, depth+1, maxDepth, state, next)
				if e != nil {
					return nil, e
				}
				out[key] = v
			}
			if end, e := d.Token(); e != nil || end != json.Delim('}') {
				return nil, fmt.Errorf("object end")
			}
			return out, nil
		case '[':
			out := []any{}
			for d.More() {
				if err := state.next(); err != nil {
					return nil, err
				}
				v, e := decodeValue(d, depth+1, maxDepth, state, jsonGeneric)
				if e != nil {
					return nil, e
				}
				out = append(out, v)
			}
			if end, e := d.Token(); e != nil || end != json.Delim(']') {
				return nil, fmt.Errorf("array end")
			}
			return out, nil
		}
	}
	return t, nil
}
func DecodeRequest(raw []byte) (Request, error) {
	if len(raw) > maxRawRequestBytes || len(raw) < 2 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte("\n")) != 1 || !utf8.Valid(raw) {
		return Request{}, reject("NONCANONICAL_REQUEST")
	}
	payload := raw[:len(raw)-1]
	identity, err := scanEnvelope(payload)
	if err != nil && FailureReason(err) != "UNKNOWN_FIELD" {
		return identity, err
	}
	if _, err = decodeObjectBounded(string(payload), MaxEnvelopeDepth, MaxEnvelopeTokens, nil, false); err != nil {
		return identity, reject("MALFORMED_INPUT")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var request Request
	if err = decoder.Decode(&request); err != nil {
		if json.Unmarshal(payload, &request) != nil {
			return identity, reject("MALFORMED_INPUT")
		}
		// Only a safe canonical extension binds UNKNOWN_FIELD (decision 0222).
		if known, ok := withoutExtensions(payload); ok && len(known) < len(payload) {
			if request, err = DecodeRequest(append(known, '\n')); err == nil {
				if _, err = validateRequest(request, true); err == nil {
					err = reject("UNKNOWN_FIELD")
				}
			}
			return request, err
		}
		return Request{}, reject("NONCANONICAL_REQUEST")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return identity, reject("NONCANONICAL_REQUEST")
	}
	encoded, err := canonicalJSON(request)
	if err != nil {
		return identity, reject("ANALYZER_FAILURE")
	}
	if !bytes.Equal(payload, encoded) {
		return request, reject("NONCANONICAL_REQUEST")
	}
	request.canonical = true
	return request, nil
}

var extensionFields = map[string]string{"": "profile family request_id scope_id compilation_unit_id target inputs", "target": "os architecture abi features", "inputs": "handle family path sha256 content_base64"}

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
			if _, nested := extensionFields[name]; slices.Contains(strings.Fields(fields), name) {
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

type envelopeContext uint8

const (
	envelopeGeneric envelopeContext = iota
	envelopeRoot
	envelopeTarget
	envelopeInputs
	envelopeFeatures
	envelopeContent
)

type envelopeFrame struct {
	kind    byte
	context envelopeContext
}
type envelopeScanner struct {
	payload       []byte
	index         int
	tokens        int
	decoded       int
	contentValues int
	frames        [MaxEnvelopeDepth]envelopeFrame
	depth         int
	identity      Request
	rootSeen      uint8
	rootError     string
	stagedKey     string
	stagedDepth   int
}

func scanEnvelope(payload []byte) (Request, error) {
	scanner := envelopeScanner{payload: payload}
	if err := scanner.value(envelopeRoot); err != nil {
		return scanner.identity, err
	}
	scanner.space()
	if scanner.index != len(payload) {
		return scanner.identity, reject("MALFORMED_INPUT")
	}
	if scanner.rootError != "" {
		return scanner.identity, reject(scanner.rootError)
	}
	return scanner.identity, nil
}
func (scanner *envelopeScanner) token() error {
	scanner.tokens++
	if scanner.tokens > MaxEnvelopeTokens {
		return reject("LIMIT_EXCEEDED")
	}
	return nil
}
func (scanner *envelopeScanner) enter(kind byte, context envelopeContext) error {
	if scanner.depth == MaxEnvelopeDepth {
		return reject("LIMIT_EXCEEDED")
	}
	scanner.frames[scanner.depth] = envelopeFrame{kind: kind, context: context}
	scanner.depth++
	return nil
}
func (scanner *envelopeScanner) leave(kind byte) error {
	if scanner.depth == 0 || scanner.frames[scanner.depth-1].kind != kind {
		return reject("MALFORMED_INPUT")
	}
	scanner.depth--
	return nil
}
func (scanner *envelopeScanner) space() {
	for scanner.index < len(scanner.payload) {
		switch scanner.payload[scanner.index] {
		case ' ', '\t', '\r', '\n':
			scanner.index++
		default:
			return
		}
	}
}
func (scanner *envelopeScanner) value(context envelopeContext) error {
	scanner.space()
	if scanner.index == len(scanner.payload) {
		return reject("MALFORMED_INPUT")
	}
	if err := scanner.token(); err != nil {
		return err
	}
	switch scanner.payload[scanner.index] {
	case '{':
		if context == envelopeContent {
			return reject("MALFORMED_INPUT")
		}
		return scanner.object(context)
	case '[':
		return scanner.array(context)
	case '"':
		start, end, escaped, err := scanner.string()
		if err != nil {
			return err
		}
		return scanner.text(context, start, end, escaped)
	default:
		if err := scanner.literal(context); err != nil {
			return err
		}
		if context == envelopeContent {
			return reject("MALFORMED_INPUT")
		}
		return nil
	}
}
func (scanner *envelopeScanner) object(context envelopeContext) error {
	if err := scanner.enter('{', context); err != nil {
		return err
	}
	scanner.index++
	scanner.space()
	if scanner.index < len(scanner.payload) && scanner.payload[scanner.index] == '}' {
		scanner.index++
		return scanner.leave('{')
	}
	for {
		if scanner.index == len(scanner.payload) || scanner.payload[scanner.index] != '"' {
			return reject("MALFORMED_INPUT")
		}
		if err := scanner.token(); err != nil {
			return err
		}
		start, end, _, err := scanner.string()
		if err != nil {
			return err
		}
		if end-start > MaxTextBytes {
			return reject("LIMIT_EXCEEDED")
		}
		scanner.space()
		if scanner.index == len(scanner.payload) || scanner.payload[scanner.index] != ':' {
			return reject("MALFORMED_INPUT")
		}
		scanner.index++
		key, err := envelopeKey(scanner.payload[start:end])
		if err != nil {
			return err
		}
		valueContext := scanner.memberContext(context, key)
		if context == envelopeRoot {
			scanner.rootKey(key)
			scanner.stagedKey, scanner.stagedDepth = key, scanner.depth
		}
		if err := scanner.value(valueContext); err != nil {
			return err
		}
		scanner.stagedKey = ""
		scanner.space()
		if scanner.index == len(scanner.payload) {
			return reject("MALFORMED_INPUT")
		}
		if scanner.payload[scanner.index] == '}' {
			scanner.index++
			return scanner.leave('{')
		}
		if scanner.payload[scanner.index] != ',' {
			return reject("MALFORMED_INPUT")
		}
		scanner.index++
		scanner.space()
	}
}

func envelopeKey(raw []byte) (string, error) {
	var out [len("compilation_unit_id")]byte
	n := 0
	for index := 0; index < len(raw); index++ {
		value := raw[index]
		if value == '\\' {
			index++
			if index == len(raw) {
				return "", reject("MALFORMED_INPUT")
			}
			switch raw[index] {
			case '"', '\\', '/':
				value = raw[index]
			case 'b':
				value = '\b'
			case 'f':
				value = '\f'
			case 'n':
				value = '\n'
			case 'r':
				value = '\r'
			case 't':
				value = '\t'
			case 'u':
				if index+4 >= len(raw) {
					return "", reject("MALFORMED_INPUT")
				}
				value = 0
				for offset := 1; offset <= 4; offset++ {
					hex := raw[index+offset]
					if hex >= '0' && hex <= '9' {
						value = value*16 + hex - '0'
					} else if hex >= 'a' && hex <= 'f' {
						value = value*16 + hex - 'a' + 10
					} else if hex >= 'A' && hex <= 'F' {
						value = value*16 + hex - 'A' + 10
					} else {
						return "", reject("MALFORMED_INPUT")
					}
				}
				index += 4
			default:
				return "", reject("MALFORMED_INPUT")
			}
		}
		if value >= 0x80 || n == len(out) {
			return "\x00", nil
		}
		out[n], n = value, n+1
	}
	return string(out[:n]), nil
}
func (scanner *envelopeScanner) rootKey(key string) {
	var bit uint8
	switch key {
	case "profile":
		bit = 1 << 0
	case "family":
		bit = 1 << 1
	case "request_id":
		bit = 1 << 2
	case "scope_id":
		bit = 1 << 3
	case "compilation_unit_id":
		bit = 1 << 4
	case "target":
		bit = 1 << 5
	case "inputs":
		bit = 1 << 6
	default:
		if scanner.rootError == "" {
			scanner.rootError = "UNKNOWN_FIELD"
		}
		return
	}
	if scanner.rootSeen&bit != 0 && scanner.rootError == "" {
		scanner.rootError = "MALFORMED_INPUT"
	}
	scanner.rootSeen |= bit
}
func (scanner *envelopeScanner) memberContext(parent envelopeContext, key string) envelopeContext {
	switch parent {
	case envelopeRoot:
		switch key {
		case "inputs":
			return envelopeInputs
		case "target":
			return envelopeTarget
		}
	case envelopeTarget:
		if key == "features" {
			return envelopeFeatures
		}
	case envelopeInputs:
		if key == "content_base64" {
			return envelopeContent
		}
	}
	return envelopeGeneric
}
func (scanner *envelopeScanner) array(context envelopeContext) error {
	if err := scanner.enter('[', context); err != nil {
		return err
	}
	scanner.index++
	scanner.space()
	if scanner.index < len(scanner.payload) && scanner.payload[scanner.index] == ']' {
		scanner.index++
		if err := scanner.leave('['); err != nil {
			return err
		}
		if context == envelopeContent {
			return reject("MALFORMED_INPUT")
		}
		return nil
	}
	count := 0
	for {
		count++
		switch context {
		case envelopeInputs:
			if count > MaxInputs {
				return reject("LIMIT_EXCEEDED")
			}
		case envelopeFeatures:
			if count > MaxFeatures {
				return reject("LIMIT_EXCEEDED")
			}
		}
		if err := scanner.value(context); err != nil {
			return err
		}
		scanner.space()
		if scanner.index == len(scanner.payload) {
			return reject("MALFORMED_INPUT")
		}
		if scanner.payload[scanner.index] == ']' {
			scanner.index++
			if err := scanner.leave('['); err != nil {
				return err
			}
			if context == envelopeContent {
				return reject("MALFORMED_INPUT")
			}
			return nil
		}
		if scanner.payload[scanner.index] != ',' {
			return reject("MALFORMED_INPUT")
		}
		scanner.index++
		scanner.space()
	}
}
func (scanner *envelopeScanner) text(context envelopeContext, start, end int, escaped bool) error {
	length := end - start
	if scanner.stagedKey != "" && scanner.stagedDepth == scanner.depth && !escaped {
		value := string(scanner.payload[start:end])
		switch scanner.stagedKey {
		case "profile":
			scanner.identity.Profile = value
		case "family":
			scanner.identity.Family = value
		case "request_id":
			scanner.identity.RequestID = value
		}
	}
	if context != envelopeContent && length > MaxTextBytes {
		return reject("LIMIT_EXCEEDED")
	}
	if context != envelopeContent {
		return nil
	}
	scanner.contentValues++
	if scanner.contentValues > MaxTextBytes || length > maxContentBase64Size {
		return reject("LIMIT_EXCEEDED")
	}
	decoded := base64UpperBound(scanner.payload[start:end])
	if escaped {
		decoded = length
	}
	if decoded > MaxSourceBytes-scanner.decoded {
		return reject("LIMIT_EXCEEDED")
	}
	scanner.decoded += decoded
	return nil
}
func base64UpperBound[T string | []byte](value T) int {
	if len(value) == 0 {
		return 0
	}
	if len(value)%4 != 0 {
		return ((len(value) + 3) / 4) * 3
	}
	decoded := len(value) / 4 * 3
	if value[len(value)-1] == '=' {
		decoded--
	}
	if value[len(value)-2] == '=' {
		decoded--
	}
	return decoded
}
func (scanner *envelopeScanner) literal(context envelopeContext) error {
	start := scanner.index
	for scanner.index < len(scanner.payload) && !strings.ContainsRune(" \t\r\n{}[],:", rune(scanner.payload[scanner.index])) {
		scanner.index++
	}
	if scanner.index == start {
		return reject("MALFORMED_INPUT")
	}
	if context == envelopeContent {
		scanner.contentValues++
		if scanner.contentValues > MaxTextBytes {
			return reject("LIMIT_EXCEEDED")
		}
	}
	return nil
}
func (scanner *envelopeScanner) string() (int, int, bool, error) {
	scanner.index++
	start, escaped := scanner.index, false
	for scanner.index < len(scanner.payload) {
		c := scanner.payload[scanner.index]
		if c < 0x20 {
			return 0, 0, false, reject("MALFORMED_INPUT")
		}
		if c == '"' {
			end := scanner.index
			scanner.index++
			return start, end, escaped, nil
		}
		if c != '\\' {
			scanner.index++
			continue
		}
		escaped = true
		scanner.index++
		if scanner.index == len(scanner.payload) {
			break
		}
		escape := scanner.payload[scanner.index]
		if escape == 'u' {
			if scanner.index+4 >= len(scanner.payload) {
				break
			}
			for offset := 1; offset <= 4; offset++ {
				c := scanner.payload[scanner.index+offset]
				if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
					return 0, 0, false, reject("MALFORMED_INPUT")
				}
			}
			scanner.index += 5
			continue
		}
		if !strings.ContainsRune(`"\\/bfnrt`, rune(escape)) {
			return 0, 0, false, reject("MALFORMED_INPUT")
		}
		scanner.index++
	}
	return 0, 0, false, reject("MALFORMED_INPUT")
}
func canonicalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return buffer.Bytes()[:buffer.Len()-1], nil
}
