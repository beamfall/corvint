package analyzerswift

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	"errors"
	"strconv"
	"strings"
)

// extension binds UNKNOWN_FIELD only to an otherwise safe canonical extension
// whose remaining bytes pass the closed envelope grammar (SAC-002, decision 0242);
// any other request that fails that grammar is the fixed sentinel.
func extension(wire []byte) []byte {
	if len(wire) < 2 || wire[len(wire)-1] != '\n' {
		return sentinelBytes()
	}
	known, ok := withoutExtensions(wire[:len(wire)-1])
	if !ok || len(known) == len(wire)-1 {
		return sentinelBytes()
	}
	request, _, err := parseCanonicalRequest(append(known, '\n'))
	if err != nil {
		return sentinelBytes()
	}
	return reject(request, echoes(request.Inputs), "UNKNOWN_FIELD")
}

// extensionFields names each schema object's fields; any other member is an extension.
var extensionFields = map[string]string{"": " profile family request_id scope_id compilation_unit_id target inputs ", "target": " os architecture abi features ", "inputs": " handle family path sha256 content_base64 "}

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

// parseCanonicalRequest accepts the one closed Swift/Apple envelope grammar directly.
// It retains no decoded data and no base64 copy: each content slice aliases only the
// bounded wire buffer until decodeInput consumes it.
func parseCanonicalRequest(wire []byte) (Request, [][]byte, error) {
	if len(wire) < 2 || wire[len(wire)-1] != '\n' {
		return Request{}, nil, errors.New("missing LF")
	}
	cursor := canonicalCursor{value: wire[:len(wire)-1]}
	if !cursor.literal(`{"profile":`) {
		return Request{}, nil, errors.New("profile field")
	}
	profile, ok := cursor.string()
	if !ok || string(profile) != Profile || !cursor.literal(`,"family":`) {
		return Request{}, nil, errors.New("profile")
	}
	family, ok := cursor.string()
	if !ok || string(family) != Family || !cursor.literal(`,"request_id":`) {
		return Request{}, nil, errors.New("family")
	}
	requestID, ok := cursor.string()
	if !ok || !identifier(string(requestID)) || !cursor.literal(`,"scope_id":`) {
		return Request{}, nil, errors.New("request ID")
	}
	scopeID, ok := cursor.string()
	if !ok || string(scopeID) != "beamfall-apple" || !cursor.literal(`,"compilation_unit_id":`) {
		return Request{}, nil, errors.New("scope")
	}
	unitID, ok := cursor.string()
	if !ok || string(unitID) != "a11y" || !cursor.literal(`,"target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`) || !cursor.literal(`,"inputs":[`) {
		return Request{}, nil, errors.New("target")
	}
	expected := []Input{
		{Handle: "apple-coordinates", Family: "swift.apple.coordinates", Path: "fixtures/beamfall-apple-8588cec3/coordinates.txt"},
		{Handle: "apple-source", Family: "swift.source", Path: "Sources/BeamfallA11y/A11yID.swift"},
		{Handle: "apple-ui", Family: "swift.apple-ui.package", Path: "Package.swift"},
	}
	request := Request{
		Profile: string(profile), Family: string(family), RequestID: string(requestID),
		ScopeID: string(scopeID), CompilationUnitID: string(unitID),
		Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		Inputs: make([]Input, len(expected)),
	}
	content := make([][]byte, len(expected))
	for index, want := range expected {
		if index > 0 && !cursor.literal(",") {
			return Request{}, nil, errors.New("input delimiter")
		}
		input, encoded, ok := cursor.input(want)
		if !ok {
			return Request{}, nil, errors.New("input")
		}
		request.Inputs[index], content[index] = input, encoded
	}
	if !cursor.literal("]}") || !cursor.done() {
		return Request{}, nil, errors.New("tail")
	}
	return request, content, nil
}

func (cursor *canonicalCursor) input(want Input) (Input, []byte, bool) {
	if !cursor.literal(`{"handle":`) {
		return Input{}, nil, false
	}
	handle, ok := cursor.string()
	if !ok || string(handle) != want.Handle || !cursor.literal(`,"family":`) {
		return Input{}, nil, false
	}
	family, ok := cursor.string()
	if !ok || string(family) != want.Family || !cursor.literal(`,"path":`) {
		return Input{}, nil, false
	}
	path, ok := cursor.string()
	if !ok || string(path) != want.Path || !cursor.literal(`,"sha256":`) {
		return Input{}, nil, false
	}
	digestValue, ok := cursor.string()
	if !ok || !digest(string(digestValue)) || !cursor.literal(`,"content_base64":`) {
		return Input{}, nil, false
	}
	encoded, ok := cursor.string()
	if !ok || len(encoded) == 0 || len(encoded) > 1_398_104 || !cursor.literal("}") {
		return Input{}, nil, false
	}
	return Input{Handle: want.Handle, Family: want.Family, Path: want.Path, SHA256: string(digestValue)}, encoded, true
}

func decodeInput(input Input, encoded []byte, remaining int) ([]byte, string) {
	decodedLength := decodedLength(encoded)
	if decodedLength < 0 || decodedLength > remaining {
		return nil, "LIMIT_EXCEEDED"
	}
	body := make([]byte, decodedLength)
	n, err := base64.StdEncoding.Strict().Decode(body, encoded)
	if err != nil {
		return nil, "MALFORMED_INPUT"
	}
	body = body[:n]
	sum := sha256.Sum256(body)
	if !digestMatches(sum, input.SHA256) {
		return nil, "DIGEST_MISMATCH"
	}
	return body, ""
}

func validateEncodedInput(input Input, encoded []byte, remaining int, blob string) string {
	length := decodedLength(encoded)
	if length < 0 || length > remaining {
		return "LIMIT_EXCEEDED"
	}
	sha := sha256.New()
	git := sha1.New()
	_, _ = git.Write([]byte("blob " + strconv.Itoa(length) + "\x00"))
	var decoded [768]byte
	for len(encoded) > 0 {
		chunkSize := minInt(len(encoded), 1024)
		chunkSize -= chunkSize % 4
		if chunkSize == 0 {
			return "MALFORMED_INPUT"
		}
		count, err := base64.StdEncoding.Strict().Decode(decoded[:], encoded[:chunkSize])
		if err != nil {
			return "MALFORMED_INPUT"
		}
		_, _ = sha.Write(decoded[:count])
		_, _ = git.Write(decoded[:count])
		encoded = encoded[chunkSize:]
	}
	if !digestBytesMatch(sha.Sum(nil), input.SHA256) {
		return "DIGEST_MISMATCH"
	}
	if !gitDigestMatches(git.Sum(nil), blob) {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	return ""
}

func decodedLength(encoded []byte) int {
	length := base64.StdEncoding.DecodedLen(len(encoded))
	if len(encoded) >= 1 && encoded[len(encoded)-1] == '=' {
		length--
	}
	if len(encoded) >= 2 && encoded[len(encoded)-2] == '=' {
		length--
	}
	return length
}

func digestMatches(sum [sha256.Size]byte, value string) bool {
	return digestBytesMatch(sum[:], value)
}

func digestBytesMatch(sum []byte, value string) bool {
	if len(value) != 71 || value[:7] != "sha256:" {
		return false
	}
	for index, byteValue := range sum {
		if value[7+index*2] != hexDigit(byteValue>>4) || value[8+index*2] != hexDigit(byteValue&0x0f) {
			return false
		}
	}
	return true
}

func gitDigestMatches(sum []byte, value string) bool {
	if len(value) != len(sum)*2 {
		return false
	}
	for index, byteValue := range sum {
		if value[index*2] != hexDigit(byteValue>>4) || value[index*2+1] != hexDigit(byteValue&0x0f) {
			return false
		}
	}
	return true
}

func hexDigit(value byte) byte {
	if value < 10 {
		return '0' + value
	}
	return 'a' + value - 10
}

type canonicalCursor struct {
	value []byte
	index int
}

func (cursor *canonicalCursor) literal(value string) bool {
	if len(cursor.value)-cursor.index < len(value) || !bytes.Equal(cursor.value[cursor.index:cursor.index+len(value)], []byte(value)) {
		return false
	}
	cursor.index += len(value)
	return true
}

func (cursor *canonicalCursor) string() ([]byte, bool) {
	if cursor.index >= len(cursor.value) || cursor.value[cursor.index] != '"' {
		return nil, false
	}
	start := cursor.index + 1
	for cursor.index = start; cursor.index < len(cursor.value); cursor.index++ {
		value := cursor.value[cursor.index]
		if value == '"' {
			result := cursor.value[start:cursor.index]
			cursor.index++
			return result, true
		}
		if value < 32 || value == '\\' {
			return nil, false
		}
	}
	return nil, false
}

func (cursor canonicalCursor) done() bool { return cursor.index == len(cursor.value) }
