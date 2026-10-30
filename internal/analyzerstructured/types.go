// Package analyzerstructured is an unregistered, admission-candidate-only
// structured-data extractor. It consumes caller-owned immutable bytes only.
package analyzerstructured

import "crypto/sha256"

const (
	Profile = "corvint-structured-data/experimental-v1"
	Family  = "structured-data"

	maxFrameBytes  = 1_500_000
	maxInputBytes  = 65_536
	maxBase64Bytes = 4 * ((maxInputBytes + 2) / 3)
	maxInputs      = 64
	maxFacts       = 64
	maxOutputBytes = 65_536
	maxDepth       = 32
	maxStringBytes = 8_192
	maxTokenCount  = 4_096
)

// FormatProfile is an exact, independently selectable closed parser profile.
type FormatProfile string

const (
	JSONProfile        FormatProfile = "structured.json/rfc8259-v1"
	JSONLProfile       FormatProfile = "structured.jsonl/rfc8259-v1"
	YAMLProfile        FormatProfile = "structured.yaml/1.2-core-v1"
	TOMLProfile        FormatProfile = "structured.toml/1.0.0-v1"
	XMLProfile         FormatProfile = "structured.xml/1.0-v1"
	PlistProfile       FormatProfile = "structured.plist/xml-v1"
	PropertiesProfile  FormatProfile = "structured.properties/java-17-v1"
	HCLProfile         FormatProfile = "structured.hcl/2.0-static-v1"
	WebManifestProfile FormatProfile = "structured.webmanifest/whatwg-v1"
	SVGProfile         FormatProfile = "structured.svg/1.1-static-v1"
)

var formatProfiles = [...]FormatProfile{
	JSONProfile,
	JSONLProfile,
	YAMLProfile,
	TOMLProfile,
	XMLProfile,
	PlistProfile,
	PropertiesProfile,
	HCLProfile,
	WebManifestProfile,
	SVGProfile,
}

type Target struct {
	OS           string   `json:"os"`
	Architecture string   `json:"architecture"`
	ABI          string   `json:"abi"`
	Features     []string `json:"features"`
}

type Input struct {
	Handle        string        `json:"handle"`
	Family        FormatProfile `json:"family"`
	Path          string        `json:"path"`
	SHA256        string        `json:"sha256"`
	ContentBase64 string        `json:"content_base64"`
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

// Fact is a structural coordinate, not a semantic-validity assertion.
type Fact struct {
	Kind          string        `json:"kind"`
	InputHandle   string        `json:"input_handle"`
	Format        FormatProfile `json:"format"`
	Profile       string        `json:"profile"`
	Path          string        `json:"path"`
	ByteStart     int           `json:"byte_start"`
	ByteEnd       int           `json:"byte_end"`
	Line          int           `json:"line"`
	Column        int           `json:"column"`
	WitnessSHA256 string        `json:"witness_sha256"`
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

type sentinelRejected struct {
	Profile   string `json:"profile"`
	Family    string `json:"family"`
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}

type decodedInput struct {
	input  Input
	bytes  []byte
	digest [sha256.Size]byte
}
