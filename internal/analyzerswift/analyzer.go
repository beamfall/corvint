// Package analyzerswift is an unregistered experimental Swift/Apple fact extractor.
package analyzerswift

import (
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
)

const (
	Profile = "corvint-analyzer-candidate/experimental"
	Family  = "swift-apple"

	maxWire       = 1_500_000
	maxData       = 1_048_576
	maxInputs     = 3
	maxFacts      = 16
	maxSourceSize = 65_536
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

type output struct {
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

type rejection struct {
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

var sentinel = []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n")

// Analyze consumes exactly one bounded canonical envelope. It performs no filesystem,
// process, toolchain, environment, dynamic-loader, or network I/O.
func Analyze(reader io.Reader) []byte {
	wire, err := readBounded(reader)
	if err != nil {
		return sentinelBytes()
	}
	return analyzeWire(wire)
}

// analyzeWire consumes one immutable, bounded canonical envelope. Descriptor
// acquisition owns this byte slice, so the production command avoids a second
// whole-request copy before decoding its selected inputs.
func analyzeWire(wire []byte) []byte {
	request, encoded, err := parseCanonicalRequest(wire)
	if err != nil {
		return extension(wire)
	}
	inputEchoes := echoes(request.Inputs)
	coordinate, reason := decodeInput(request.Inputs[0], encoded[0], maxData)
	if reason != "" {
		return reject(request, inputEchoes, reason)
	}
	source, reason := decodeInput(request.Inputs[1], encoded[1], maxData-len(coordinate))
	if reason != "" {
		return reject(request, inputEchoes, reason)
	}
	reason = validateEncodedInput(request.Inputs[2], encoded[2], maxData-len(coordinate)-len(source), uiPackageBlob)
	if reason != "" {
		return reject(request, inputEchoes, reason)
	}
	coordinateFacts, reason := parseCoordinates(request, request.Inputs, coordinate, gitBlobMatches(source, a11yBlob), true)
	if reason != "" {
		return reject(request, inputEchoes, reason)
	}
	sourceFacts, reason := parseSource(request, request.Inputs[1], source)
	if reason != "" {
		return reject(request, inputEchoes, reason)
	}
	budget, ok := newOutputBudget(request, inputEchoes)
	if !ok {
		return reject(request, inputEchoes, "OUTPUT_LIMIT")
	}
	facts := make([]Fact, 0, len(coordinateFacts)+len(sourceFacts))
	for _, group := range [][]Fact{coordinateFacts, sourceFacts} {
		for _, fact := range group {
			if len(facts) == maxFacts {
				return reject(request, inputEchoes, "LIMIT_EXCEEDED")
			}
			if !budget.add(fact) {
				return reject(request, inputEchoes, "OUTPUT_LIMIT")
			}
			facts = append(facts, fact)
		}
	}
	sort.Slice(facts, func(i, j int) bool { return compareFact(facts[i], facts[j]) < 0 })
	for i := 1; i < len(facts); i++ {
		if compareFact(facts[i-1], facts[i]) == 0 {
			return reject(request, inputEchoes, "DUPLICATE_VALUE")
		}
	}
	if !factsFitOutput(request, inputEchoes, facts) {
		return reject(request, inputEchoes, "OUTPUT_LIMIT")
	}
	result, err := json.Marshal(output{
		Profile: Profile, Family: Family, RequestID: request.RequestID, Status: "CANDIDATE",
		ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target,
		InputEchoes: inputEchoes, Facts: facts,
	})
	if err != nil || len(result)+1 > maxData {
		return reject(request, inputEchoes, "OUTPUT_LIMIT")
	}
	return append(result, '\n')
}

func sentinelBytes() []byte { return append([]byte(nil), sentinel...) }

func readBounded(reader io.Reader) ([]byte, error) {
	if sized, ok := reader.(interface{ Len() int }); ok {
		length := sized.Len()
		if length < 2 || length > maxWire {
			return nil, errors.New("invalid wire")
		}
		value := make([]byte, length)
		if _, err := io.ReadFull(reader, value); err != nil {
			return nil, errors.New("invalid wire")
		}
		var trailing [1]byte
		if count, err := reader.Read(trailing[:]); count != 0 || err != io.EOF {
			return nil, errors.New("invalid wire")
		}
		if value[len(value)-1] != '\n' {
			return nil, errors.New("invalid wire")
		}
		return value, nil
	}
	value := make([]byte, 0, 4096)
	var chunk [4096]byte
	for {
		count, err := reader.Read(chunk[:])
		if count > 0 {
			if len(value)+count > maxWire {
				return nil, errors.New("invalid wire")
			}
			value = append(value, chunk[:count]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil || count == 0 {
			return nil, errors.New("invalid wire")
		}
	}
	if len(value) < 2 || value[len(value)-1] != '\n' {
		return nil, errors.New("invalid wire")
	}
	return value, nil
}

func reject(request Request, inputEchoes []Echo, reason string) []byte {
	value, err := json.Marshal(rejection{
		Profile: Profile, Family: Family, RequestID: request.RequestID, Status: "REJECTED",
		ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target,
		InputEchoes: inputEchoes, Reason: reason,
	})
	if err != nil || len(value)+1 > maxData {
		return sentinelBytes()
	}
	return append(value, '\n')
}

func echoes(inputs []Input) []Echo {
	result := make([]Echo, len(inputs))
	for i, input := range inputs {
		result[i] = Echo{Handle: input.Handle, SHA256: input.SHA256}
	}
	return result
}

func identifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i := range value {
		c := value[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || i > 0 && strings.ContainsRune("._:@+~-", rune(c))) {
			return false
		}
	}
	return true
}

func digest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[7:] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func compareFact(left, right Fact) int {
	leftFields := []string{left.Kind, left.InputHandle, left.RelatedHandle, left.Subject, left.Predicate, left.Value, left.InstanceID, left.EvidenceSHA256}
	rightFields := []string{right.Kind, right.InputHandle, right.RelatedHandle, right.Subject, right.Predicate, right.Value, right.InstanceID, right.EvidenceSHA256}
	for i := range leftFields {
		if leftFields[i] < rightFields[i] {
			return -1
		}
		if leftFields[i] > rightFields[i] {
			return 1
		}
	}
	return 0
}

func factsFitOutput(request Request, inputEchoes []Echo, facts []Fact) bool {
	budget, ok := newOutputBudget(request, inputEchoes)
	if !ok || len(facts) > maxFacts {
		return false
	}
	for _, fact := range facts {
		if !budget.add(fact) {
			return false
		}
	}
	return true
}

type outputBudget struct {
	used  int
	facts int
}

func newOutputBudget(request Request, inputEchoes []Echo) (outputBudget, bool) {
	if request.Target.Features == nil || len(inputEchoes) != maxInputs {
		return outputBudget{}, false
	}
	empty, err := json.Marshal(output{Profile: Profile, Family: Family, RequestID: request.RequestID, Status: "CANDIDATE", ScopeID: request.ScopeID, CompilationUnitID: request.CompilationUnitID, Target: request.Target, InputEchoes: inputEchoes, Facts: []Fact{}})
	if err != nil {
		return outputBudget{}, false
	}
	return outputBudget{used: len(empty) + 1}, len(empty)+1 <= maxData
}

func (budget *outputBudget) add(fact Fact) bool {
	if budget.facts >= maxFacts {
		return false
	}
	for _, value := range []string{fact.Kind, fact.InputHandle, fact.RelatedHandle, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID, fact.EvidenceSHA256} {
		if len(value) == 0 || len(value) > 4096 {
			return false
		}
	}
	size, ok := factJSONSize(fact)
	if !ok || budget.used+size+boolSize(budget.facts > 0) > maxData {
		return false
	}
	budget.used += size + boolSize(budget.facts > 0)
	budget.facts++
	return true
}

func factJSONSize(fact Fact) (int, bool) {
	values := []string{fact.Kind, fact.InputHandle, fact.RelatedHandle, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID, fact.EvidenceSHA256}
	keys := []string{`{"kind":`, `,"input_handle":`, `,"related_handle":`, `,"subject":`, `,"predicate":`, `,"value":`, `,"instance_id":`, `,"evidence_sha256":`}
	size := 1
	for index, value := range values {
		encoded, ok := jsonStringSize(value)
		if !ok {
			return 0, false
		}
		size += len(keys[index]) + encoded
	}
	return size, true
}

func jsonStringSize(value string) (int, bool) {
	size := 2
	for index := 0; index < len(value); index++ {
		if value[index] < 32 || value[index] == 127 || value[index] == '\\' || value[index] == '"' {
			return 0, false
		}
		size++
	}
	return size, true
}

func boolSize(value bool) int {
	if value {
		return 1
	}
	return 0
}
