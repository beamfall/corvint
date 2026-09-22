// Package analyzerkotlinandroid implements an unregistered experimental Kotlin/Android analyzer.
package analyzerkotlinandroid

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/json/jsontext"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	Profile         = "corvint-analyzer-candidate/experimental"
	Family          = "kotlin-android"
	MaxRequestBytes = 1_500_000
	MaxInputBytes   = 1 << 20
	MaxOutputBytes  = 1 << 20
	androidRevision = "52799c501cc291ff003d05f9eda391c2008cd51e"
	uiRevision      = "6e379d7feec88128439d1753325bcfb22194fdfc"
)

// pinnedFixtureSHA256 binds the finite Beamfall dogfood projection.  Paths,
// revisions, and bytes are all caller supplied and verified; the analyzer
// never opens either repository.
var pinnedFixtureSHA256 = map[string]string{
	"android.project.revision\x00project.revision":                                       "sha256:3510c7e754ad72e0f536a8d0f79500a8ea22c1db4f4e06b929a7777abc50dade",
	"android.gradle.build\x00build.gradle.kts":                                           "sha256:9526fde07a6cbf085eaa3692d78681da2f4aadc6ad69f95365c150cd21ee6aef",
	"android.gradle.settings\x00settings.gradle.kts":                                     "sha256:b493750b2fc4f021d3c98ea97f85064983e89d70a91bfa27061b92b0121397b4",
	"android.gradle.wrapper\x00gradle/wrapper/gradle-wrapper.properties":                 "sha256:5eddf98b5bb81bb199cc477cda9de0ab1ad495dbece4cf18743f40c734ec3f14",
	"android.project.version\x00gradle/version.properties":                               "sha256:216f1b3e7709b2701df87b6bf5459715e496aebd39cc19e2e94bdbf3d0f17b88",
	"android.version.catalog\x00gradle/libs.versions.toml":                               "sha256:e8d29992c43017707df4f1ef42a9098a94e3d6b7b1438a0b6e1df705f24efdfa",
	"android.gradle.module\x00app-mobile/build.gradle.kts":                               "sha256:bfe67f835f2140fd20ab969a647af94db204974dbd5895178a8c2931aefc5807",
	"kotlin.source\x00app-mobile/src/main/kotlin/com/beamfall/mobile/BuildIdentity.kt":   "sha256:63fd5a533d603f6e44636ea4428558c039b24b70c783973e14eba59860ce73e5",
	"android-ui.gradle.build\x00ui/build.gradle.kts":                                     "sha256:eb04000cacc559d1fd06f7ee4b1425a62bc6e845bcc28f799ed1ffb3af9bfeab",
	"android-ui.version.catalog\x00ui/gradle/libs.versions.toml":                         "sha256:a6ffb27386a82fae7978f8209d2767dba3d985448feee6c44dee017764855219",
	"android-ui.kotlin.source\x00ui/kit/src/main/kotlin/com/beamfall/kit/BeamfallKit.kt": "sha256:e50f7f133fb86291b285a4be83320892a9e1a49471d06b6668ee013dff5c90fc",
	"android.ui.revision\x00android-ui.revision":                                         "sha256:ef54f0aac2a0f87f4c89de053b5177570ab4ea9c264421141d3e67ef4002fcd1",
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
	StartLine      int    `json:"start_line"`
	StartColumn    int    `json:"start_column"`
	EndLine        int    `json:"end_line"`
	EndColumn      int    `json:"end_column"`
}
type result struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	RequestID         string `json:"request_id"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            Target `json:"target"`
	InputEchoes       []Echo `json:"input_echoes"`
	Facts             []Fact `json:"facts"`
	Reason            string `json:"reason"`
}
type sentinel struct {
	Profile   string `json:"profile"`
	Family    string `json:"family"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}
type rejected struct {
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

// ProcessReader has no path or descriptor parameter. It consumes one bounded
// stream, hashes each in-memory input before and after parsing, and cannot read
// a repository, execute a process, load a library, or contact a network.
func ProcessReader(reader io.Reader) ([]byte, error) {
	raw, reason := readBounded(reader)
	if reason != "" {
		return rejectSentinel(reason), nil
	}
	return Process(raw)
}

// ProcessFile retains the already-open standard-input descriptor and verifies
// its identity before and after the bounded read. The candidate never accepts
// a path, so there is no path lookup or symlink-following operation to race.
func ProcessFile(file *os.File) ([]byte, error) {
	raw, reason := readStableDescriptor(file, nil)
	if reason != "" {
		return rejectSentinel(reason), nil
	}
	return Process(raw)
}

func readBounded(reader io.Reader) ([]byte, string) {
	raw, err := io.ReadAll(io.LimitReader(reader, MaxRequestBytes+1))
	if err != nil {
		return nil, "NONCANONICAL_REQUEST"
	}
	if len(raw) > MaxRequestBytes {
		return nil, "LIMIT_EXCEEDED"
	}
	return raw, ""
}

// readStableDescriptor never reopens a pathname. Regular-file descriptors are
// read twice through the descriptor, with identity, mode, size, modification
// time, and content equality checked around both reads. The optional hook is
// package-private test instrumentation for the same-inode mutation witness.
func readStableDescriptor(file *os.File, afterFirstRead func()) ([]byte, string) {
	before, err := file.Stat()
	if err != nil {
		return nil, "NONCANONICAL_REQUEST"
	}
	if !before.Mode().IsRegular() {
		raw, reason := readBounded(file)
		if reason != "" {
			return nil, reason
		}
		after, err := file.Stat()
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
			return nil, "NONCANONICAL_REQUEST"
		}
		return raw, ""
	}
	if before.Size() < 0 || before.Size() > MaxRequestBytes {
		return nil, "LIMIT_EXCEEDED"
	}
	readExact := func() ([]byte, string) {
		reader := io.NewSectionReader(file, 0, before.Size())
		raw, err := io.ReadAll(reader)
		if err != nil || int64(len(raw)) != before.Size() {
			return nil, "NONCANONICAL_REQUEST"
		}
		return raw, ""
	}
	first, reason := readExact()
	if reason != "" {
		return nil, reason
	}
	if afterFirstRead != nil {
		afterFirstRead()
	}
	middle, err := file.Stat()
	if err != nil || !sameDescriptorState(before, middle) {
		return nil, "NONCANONICAL_REQUEST"
	}
	second, reason := readExact()
	if reason != "" {
		return nil, reason
	}
	after, err := file.Stat()
	if err != nil || !sameDescriptorState(before, after) {
		return nil, "NONCANONICAL_REQUEST"
	}
	firstDigest, secondDigest := sha256.Sum256(first), sha256.Sum256(second)
	if firstDigest != secondDigest {
		return nil, "NONCANONICAL_REQUEST"
	}
	return first, ""
}

func sameDescriptorState(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func Process(raw []byte) ([]byte, error) {
	if len(raw) > MaxRequestBytes {
		return rejectSentinel("LIMIT_EXCEEDED"), nil
	}
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || !utf8.Valid(raw) {
		return rejectSentinel("NONCANONICAL_REQUEST"), nil
	}
	if scanReason := scanCanonicalJSON(raw[:len(raw)-1]); scanReason != "" {
		return rejectSentinel(scanReason), nil
	}
	payload := raw[:len(raw)-1]
	if hasUnknownField(payload) {
		return extension(payload), nil
	}
	var request Request
	if err := json.Unmarshal(payload, &request); err != nil {
		return rejectSentinel("NONCANONICAL_REQUEST"), nil
	}
	canonical, err := json.Marshal(request)
	if err != nil || !bytes.Equal(canonical, payload) {
		return reject(request, "NONCANONICAL_REQUEST"), nil
	}
	if reason := validate(request); reason != "" {
		return reject(request, reason), nil
	}
	contents, reason := decode(request)
	if reason != "" {
		return reject(request, reason), nil
	}
	facts, reason := factsFor(request, contents)
	if reason != "" {
		return reject(request, reason), nil
	}
	if reason := identityRecheck(request, contents); reason != "" {
		return reject(request, reason), nil
	}
	output, ok := encodeSuccess(request, facts)
	if !ok {
		return reject(request, "OUTPUT_LIMIT"), nil
	}
	return output, nil
}

// extension binds UNKNOWN_FIELD only to an otherwise safe canonical extension
// (decision 0222); any other unknown member is the NONCANONICAL_REQUEST sentinel.
func extension(payload []byte) []byte {
	var request Request
	known, ok := withoutExtensions(payload)
	if !ok || json.Unmarshal(known, &request) != nil {
		return rejectSentinel("NONCANONICAL_REQUEST")
	}
	if canonical, err := json.Marshal(request); err != nil || !bytes.Equal(canonical, known) {
		return rejectSentinel("NONCANONICAL_REQUEST")
	}
	if output := reject(request, "UNKNOWN_FIELD"); !bytes.Equal(output, rejectSentinel("UNKNOWN_FIELD")) {
		return output
	}
	return reject(request, validate(request))
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

func hasUnknownField(payload []byte) bool {
	var root map[string]json.RawMessage
	if json.Unmarshal(payload, &root) != nil || unknown(root, "profile", "family", "request_id", "scope_id", "compilation_unit_id", "target", "inputs") {
		return true
	}
	targetRaw, targetPresent := root["target"]
	if !targetPresent {
		return false
	}
	var target map[string]json.RawMessage
	if json.Unmarshal(targetRaw, &target) != nil || unknown(target, "os", "architecture", "abi", "features") {
		return true
	}
	inputsRaw, inputsPresent := root["inputs"]
	if !inputsPresent {
		return false
	}
	var inputs []map[string]json.RawMessage
	if json.Unmarshal(inputsRaw, &inputs) != nil {
		return false
	}
	for _, input := range inputs {
		if unknown(input, "handle", "family", "path", "sha256", "content_base64") {
			return true
		}
	}
	return false
}
func unknown(object map[string]json.RawMessage, allowed ...string) bool {
	for key := range object {
		if !oneOf(key, allowed...) {
			return true
		}
	}
	return false
}
func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func validate(request Request) string {
	if request.Profile != Profile {
		return "NONCANONICAL_REQUEST"
	}
	if request.Family != Family {
		return "UNKNOWN_FAMILY"
	}
	if !id(request.RequestID) || !id(request.ScopeID) || !id(request.CompilationUnitID) || !id(request.Target.OS) || !id(request.Target.Architecture) || !id(request.Target.ABI) {
		return "INVALID_IDENTIFIER"
	}
	if request.Target.Features == nil {
		return "NONCANONICAL_REQUEST"
	}
	if len(request.Target.Features) > 64 {
		return "LIMIT_EXCEEDED"
	}
	if !validIdentifiers(request.Target.Features) {
		return "INVALID_IDENTIFIER"
	}
	if !increasing(request.Target.Features) {
		return "DUPLICATE_VALUE"
	}
	if len(request.Inputs) == 0 {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	if len(request.Inputs) > 128 {
		return "LIMIT_EXCEEDED"
	}
	previous := ""
	handles, paths := map[string]bool{}, map[string]bool{}
	for _, input := range request.Inputs {
		if !id(input.Handle) {
			return "INVALID_IDENTIFIER"
		}
		if !pathOK(input.Path) {
			return "INVALID_PATH"
		}
		if !digestOK(input.SHA256) {
			return "DIGEST_MISMATCH"
		}
		if !oneOf(input.Family, "android.project.revision", "android.gradle.build", "android.gradle.settings", "android.gradle.wrapper", "android.project.version", "android.version.catalog", "android.gradle.module", "kotlin.source", "android-ui.gradle.build", "android-ui.version.catalog", "android-ui.kotlin.source", "android.ui.revision") {
			return "UNSUPPORTED_SCHEMA"
		}
		if handles[input.Handle] || paths[input.Path] {
			return "DUPLICATE_VALUE"
		}
		handles[input.Handle], paths[input.Path] = true, true
		key := input.Handle + "\x00" + input.Family + "\x00" + input.Path + "\x00" + input.SHA256
		if previous != "" && key <= previous {
			return "DUPLICATE_VALUE"
		}
		previous = key
	}
	if request.Target.OS != "android" || request.Target.Architecture != "arm64" || request.Target.ABI != "none" || len(request.Target.Features) != 0 {
		return "EXACT_BINDING_UNAVAILABLE"
	}
	return ""
}
func id(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i, b := range []byte(value) {
		letter := b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
		if !letter && (i == 0 || !strings.ContainsRune("._:@+~-", rune(b))) {
			return false
		}
	}
	return true
}
func validIdentifiers(values []string) bool {
	for _, value := range values {
		if !id(value) {
			return false
		}
	}
	return true
}
func increasing(values []string) bool {
	previous := ""
	for _, value := range values {
		if previous >= value && previous != "" {
			return false
		}
		previous = value
	}
	return true
}
func pathOK(value string) bool {
	if len(value) == 0 || len(value) > 4096 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "." || part == ".." || !id(part) {
			return false
		}
	}
	return true
}
func digestOK(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != 71 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil
}

func decode(request Request) ([][]byte, string) {
	contents, total := make([][]byte, 0, len(request.Inputs)), 0
	var encodedScratch []byte
	for _, input := range request.Inputs {
		if len(input.ContentBase64) > 1_398_104 {
			return nil, "LIMIT_EXCEEDED"
		}
		decodedCapacity := base64.StdEncoding.DecodedLen(len(input.ContentBase64))
		decodedLength := decodedCapacity
		if strings.HasSuffix(input.ContentBase64, "=") {
			decodedLength--
			if strings.HasSuffix(input.ContentBase64, "==") {
				decodedLength--
			}
		}
		if decodedLength < 0 {
			return nil, "MALFORMED_INPUT"
		}
		if decodedLength > MaxInputBytes-total {
			return nil, "LIMIT_EXCEEDED"
		}
		content := make([]byte, decodedCapacity)
		n, err := base64.StdEncoding.Decode(content, []byte(input.ContentBase64))
		content = content[:n]
		if cap(encodedScratch) < len(input.ContentBase64) {
			encodedScratch = make([]byte, len(input.ContentBase64))
		} else {
			encodedScratch = encodedScratch[:len(input.ContentBase64)]
		}
		base64.StdEncoding.Encode(encodedScratch, content)
		if err != nil || !equalStringBytes(input.ContentBase64, encodedScratch) {
			return nil, "MALFORMED_INPUT"
		}
		sum := sha256.Sum256(content)
		if input.SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			return nil, "DIGEST_MISMATCH"
		}
		total += len(content)
		contents = append(contents, content)
	}
	return contents, ""
}

func equalStringBytes(value string, bytes []byte) bool {
	if len(value) != len(bytes) {
		return false
	}
	for index := range bytes {
		if value[index] != bytes[index] {
			return false
		}
	}
	return true
}
func identityRecheck(request Request, contents [][]byte) string {
	for i, content := range contents {
		sum := sha256.Sum256(content)
		if request.Inputs[i].SHA256 != "sha256:"+hex.EncodeToString(sum[:]) {
			return "DIGEST_MISMATCH"
		}
	}
	return ""
}

func factsFor(request Request, contents [][]byte) ([]Fact, string) {
	var output []Fact
	counts := map[string]int{}
	var versionInput, projectInput, uiInput Input
	seen := map[string]bool{}
	for i, input := range request.Inputs {
		content := contents[i]
		counts[input.Family]++
		if seen[input.Family] {
			return nil, "AMBIGUOUS_BINDING"
		}
		seen[input.Family] = true
		expected, ok := pinnedFixtureSHA256[input.Family+"\x00"+input.Path]
		if !ok || input.SHA256 != expected {
			return nil, "EXACT_BINDING_UNAVAILABLE"
		}
		switch input.Family {
		case "android.project.revision":
			if !bytes.Equal(content, []byte(androidRevision+"\n")) {
				return nil, "EXACT_BINDING_UNAVAILABLE"
			}
			projectInput = input
			output = append(output, factAt(request, input, 1, 1, 1, len(androidRevision)+1, "android.project.revision", "beamfall-android", "pins-revision", androidRevision))
		case "android.ui.revision":
			if !bytes.Equal(content, []byte(uiRevision+"\n")) {
				return nil, "EXACT_BINDING_UNAVAILABLE"
			}
			uiInput = input
			output = append(output, factAt(request, input, 1, 1, 1, len(uiRevision)+1, "android.ui.revision", "beamfall-android-ui", "pins-revision", uiRevision))
		case "android.gradle.build":
			facts, reason := gradleFacts(request, input, content)
			if reason != "" {
				return nil, reason
			}
			output = append(output, facts...)
		case "android.gradle.wrapper":
			facts, reason := propertiesFacts(request, input, content)
			if reason != "" {
				return nil, reason
			}
			output = append(output, facts...)
		case "android.project.version":
			versionInput = input
			facts, reason := propertiesFacts(request, input, content)
			if reason != "" {
				return nil, reason
			}
			output = append(output, facts...)
		case "kotlin.source", "android-ui.kotlin.source":
			facts, reason := sourceFacts(request, input, content)
			if reason != "" {
				return nil, reason
			}
			output = append(output, facts...)
		case "android.gradle.settings", "android.gradle.module", "android-ui.gradle.build":
			facts, reason := gradleFacts(request, input, content)
			if reason != "" {
				return nil, reason
			}
			output = append(output, facts...)
		case "android.version.catalog", "android-ui.version.catalog":
			facts, reason := catalogFacts(request, input, content)
			if reason != "" {
				return nil, reason
			}
			output = append(output, facts...)
		}
	}
	for _, family := range []string{"android.project.revision", "android.gradle.build", "android.gradle.settings", "android.gradle.wrapper", "android.project.version", "android.version.catalog", "android.gradle.module", "kotlin.source", "android-ui.gradle.build", "android-ui.version.catalog", "android-ui.kotlin.source", "android.ui.revision"} {
		if counts[family] != 1 {
			if counts[family] > 1 {
				return nil, "AMBIGUOUS_BINDING"
			}
			return nil, "EXACT_BINDING_UNAVAILABLE"
		}
	}
	_ = versionInput
	_ = projectInput
	_ = uiInput
	sort.Slice(output, func(i, j int) bool { return compareFact(output[i], output[j]) < 0 })
	if len(output) > 4096 {
		return nil, "LIMIT_EXCEEDED"
	}
	for i := 1; i < len(output); i++ {
		if compareFact(output[i-1], output[i]) == 0 {
			return nil, "DUPLICATE_VALUE"
		}
	}
	return output, ""
}

func sourceFacts(request Request, input Input, content []byte) ([]Fact, string) {
	return kotlinFacts(request, input, content)
}
func qualified(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !id(part) {
			return false
		}
	}
	return true
}
func fact(request Request, input Input, kind, subject, predicate, value string) Fact {
	return factAt(request, input, 1, 1, 1, 1, kind, subject, predicate, value)
}
func factAt(request Request, input Input, sl, sc, el, ec int, kind, subject, predicate, value string) Fact {
	output := Fact{kind, input.Handle, "-", subject, predicate, value, request.CompilationUnitID, "", sl, sc, el, ec}
	output.EvidenceSHA256 = evidence(request, output, input.SHA256)
	return output
}
func evidence(request Request, fact Fact, digest string) string {
	target, _ := json.Marshal(request.Target)
	parts := []string{Family, request.RequestID, request.ScopeID, request.CompilationUnitID, string(target), fact.InputHandle, digest, fact.RelatedHandle, "-", fact.Kind, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID, strconv.Itoa(fact.StartLine), strconv.Itoa(fact.StartColumn), strconv.Itoa(fact.EndLine), strconv.Itoa(fact.EndColumn)}
	hash := sha256.New()
	hash.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(parts)))
	hash.Write(n[:])
	for _, part := range parts {
		binary.BigEndian.PutUint32(n[:], uint32(len(part)))
		hash.Write(n[:])
		hash.Write([]byte(part))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
func compareFact(left, right Fact) int {
	for _, pair := range [][2]string{{left.Kind, right.Kind}, {left.InputHandle, right.InputHandle}, {left.RelatedHandle, right.RelatedHandle}, {left.Subject, right.Subject}, {left.Predicate, right.Predicate}, {left.Value, right.Value}, {left.InstanceID, right.InstanceID}, {left.EvidenceSHA256, right.EvidenceSHA256}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}
func echoes(request Request) []Echo {
	output := make([]Echo, 0, len(request.Inputs))
	for _, input := range request.Inputs {
		output = append(output, Echo{input.Handle, input.SHA256})
	}
	return output
}
func rejectSentinel(reason string) []byte {
	output, ok := encodeSentinel(reason)
	if ok {
		return output
	}
	return []byte("{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":\"unknown\",\"request_id\":\"unknown\",\"status\":\"REJECTED\",\"reason\":\"ANALYZER_FAILURE\"}\n")
}

// boundInputs reports whether every echoed input handle and digest passed its
// grammar and duplicate check; until then a rejection stays the sentinel.
func boundInputs(inputs []Input) bool {
	handles := map[string]bool{}
	for _, input := range inputs {
		if !id(input.Handle) || !digestOK(input.SHA256) || handles[input.Handle] {
			return false
		}
		handles[input.Handle] = true
	}
	return true
}
func reject(request Request, reason string) []byte {
	if request.Family != Family || !id(request.RequestID) || !id(request.ScopeID) || !id(request.CompilationUnitID) || !id(request.Target.OS) || !id(request.Target.Architecture) || !id(request.Target.ABI) || request.Target.Features == nil || !validIdentifiers(request.Target.Features) || !increasing(request.Target.Features) || !boundInputs(request.Inputs) {
		return rejectSentinel(reason)
	}
	output, ok := encodeRejected(request, reason)
	if !ok {
		return rejectSentinel("OUTPUT_LIMIT")
	}
	return output
}

// outputEncoder emits the canonical envelope directly into a bounded byte
// stream. It never returns a partial frame: callers discard it on overflow.
type outputEncoder struct {
	bytes.Buffer
	overflow bool
}

func newOutputEncoder() *outputEncoder {
	encoder := &outputEncoder{}
	encoder.Grow(4096)
	return encoder
}

func (encoder *outputEncoder) raw(value string) bool {
	if encoder.overflow || len(value) > MaxOutputBytes-encoder.Len() {
		encoder.overflow = true
		return false
	}
	_, _ = encoder.WriteString(value)
	return true
}

func (encoder *outputEncoder) byte(value byte) bool {
	if encoder.overflow || encoder.Len() == MaxOutputBytes {
		encoder.overflow = true
		return false
	}
	_ = encoder.WriteByte(value)
	return true
}

func (encoder *outputEncoder) string(value string) bool {
	if !encoder.byte('"') {
		return false
	}
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '"', '\\':
			if !encoder.byte('\\') || !encoder.byte(value[index]) {
				return false
			}
		case '\b':
			if !encoder.raw("\\b") {
				return false
			}
		case '\f':
			if !encoder.raw("\\f") {
				return false
			}
		case '\n':
			if !encoder.raw("\\n") {
				return false
			}
		case '\r':
			if !encoder.raw("\\r") {
				return false
			}
		case '\t':
			if !encoder.raw("\\t") {
				return false
			}
		case '<':
			if !encoder.raw("\\u003c") {
				return false
			}
		case '>':
			if !encoder.raw("\\u003e") {
				return false
			}
		case '&':
			if !encoder.raw("\\u0026") {
				return false
			}
		default:
			if value[index] < 0x20 {
				const hexadecimal = "0123456789abcdef"
				if !encoder.raw("\\u00") || !encoder.byte(hexadecimal[value[index]>>4]) || !encoder.byte(hexadecimal[value[index]&0x0f]) {
					return false
				}
				continue
			}
			if value[index] == 0xe2 && index+2 < len(value) && value[index+1] == 0x80 && (value[index+2] == 0xa8 || value[index+2] == 0xa9) {
				if value[index+2] == 0xa8 {
					if !encoder.raw("\\u2028") {
						return false
					}
				} else if !encoder.raw("\\u2029") {
					return false
				}
				index += 2
				continue
			}
			if !encoder.byte(value[index]) {
				return false
			}
		}
	}
	return encoder.byte('"')
}

func (encoder *outputEncoder) target(target Target) bool {
	if !encoder.raw(`{"os":`) || !encoder.string(target.OS) || !encoder.raw(`,"architecture":`) || !encoder.string(target.Architecture) || !encoder.raw(`,"abi":`) || !encoder.string(target.ABI) || !encoder.raw(`,"features":`) {
		return false
	}
	if target.Features == nil {
		return encoder.raw("null}")
	}
	if !encoder.byte('[') {
		return false
	}
	for index, feature := range target.Features {
		if index > 0 && !encoder.byte(',') {
			return false
		}
		if !encoder.string(feature) {
			return false
		}
	}
	return encoder.raw("]}")
}

func (encoder *outputEncoder) echoes(inputs []Input) bool {
	if !encoder.byte('[') {
		return false
	}
	for index, input := range inputs {
		if index > 0 && !encoder.byte(',') {
			return false
		}
		if !encoder.raw(`{"handle":`) || !encoder.string(input.Handle) || !encoder.raw(`,"sha256":`) || !encoder.string(input.SHA256) || !encoder.byte('}') {
			return false
		}
	}
	return encoder.byte(']')
}

func (encoder *outputEncoder) facts(facts []Fact) bool {
	if !encoder.byte('[') {
		return false
	}
	for index, fact := range facts {
		if index > 0 && !encoder.byte(',') {
			return false
		}
		if !encoder.raw(`{"kind":`) || !encoder.string(fact.Kind) || !encoder.raw(`,"input_handle":`) || !encoder.string(fact.InputHandle) || !encoder.raw(`,"related_handle":`) || !encoder.string(fact.RelatedHandle) || !encoder.raw(`,"subject":`) || !encoder.string(fact.Subject) || !encoder.raw(`,"predicate":`) || !encoder.string(fact.Predicate) || !encoder.raw(`,"value":`) || !encoder.string(fact.Value) || !encoder.raw(`,"instance_id":`) || !encoder.string(fact.InstanceID) || !encoder.raw(`,"evidence_sha256":`) || !encoder.string(fact.EvidenceSHA256) || !encoder.raw(`,"start_line":`) || !encoder.raw(strconv.Itoa(fact.StartLine)) || !encoder.raw(`,"start_column":`) || !encoder.raw(strconv.Itoa(fact.StartColumn)) || !encoder.raw(`,"end_line":`) || !encoder.raw(strconv.Itoa(fact.EndLine)) || !encoder.raw(`,"end_column":`) || !encoder.raw(strconv.Itoa(fact.EndColumn)) || !encoder.byte('}') {
			return false
		}
	}
	return encoder.byte(']')
}

func encodeSuccess(request Request, facts []Fact) ([]byte, bool) {
	encoder := newOutputEncoder()
	ok := encoder.raw(`{"profile":`) && encoder.string(Profile) && encoder.raw(`,"family":`) && encoder.string(Family) && encoder.raw(`,"request_id":`) && encoder.string(request.RequestID) && encoder.raw(`,"status":"CANDIDATE","scope_id":`) && encoder.string(request.ScopeID) && encoder.raw(`,"compilation_unit_id":`) && encoder.string(request.CompilationUnitID) && encoder.raw(`,"target":`) && encoder.target(request.Target) && encoder.raw(`,"input_echoes":`) && encoder.echoes(request.Inputs) && encoder.raw(`,"facts":`) && encoder.facts(facts) && encoder.raw(`,"reason":"NONE"}`) && encoder.byte('\n')
	return encoder.Bytes(), ok && !encoder.overflow
}

func encodeRejected(request Request, reason string) ([]byte, bool) {
	encoder := newOutputEncoder()
	ok := encoder.raw(`{"profile":`) && encoder.string(Profile) && encoder.raw(`,"family":`) && encoder.string(request.Family) && encoder.raw(`,"request_id":`) && encoder.string(request.RequestID) && encoder.raw(`,"status":"REJECTED","scope_id":`) && encoder.string(request.ScopeID) && encoder.raw(`,"compilation_unit_id":`) && encoder.string(request.CompilationUnitID) && encoder.raw(`,"target":`) && encoder.target(request.Target) && encoder.raw(`,"input_echoes":`) && encoder.echoes(request.Inputs) && encoder.raw(`,"reason":`) && encoder.string(reason) && encoder.byte('}') && encoder.byte('\n')
	return encoder.Bytes(), ok && !encoder.overflow
}

func encodeSentinel(reason string) ([]byte, bool) {
	encoder := newOutputEncoder()
	ok := encoder.raw(`{"profile":`) && encoder.string(Profile) && encoder.raw(`,"family":"unknown","request_id":"unknown","status":"REJECTED","reason":`) && encoder.string(reason) && encoder.byte('}') && encoder.byte('\n')
	return encoder.Bytes(), ok && !encoder.overflow
}
