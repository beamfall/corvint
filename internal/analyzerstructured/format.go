package analyzerstructured

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"
)

// parseFormat is an exact tuple dispatcher. There is deliberately no extension
// dispatch, content sniffing, fallback parser, or generic permissive profile.
func parseFormat(in decodedInput) (Fact, string) {
	if !utf8.Valid(in.bytes) {
		return Fact{}, "MALFORMED_INPUT"
	}
	var reason string
	switch in.input.Family {
	case JSONProfile:
		reason = parseJSON(in)
	case JSONLProfile:
		reason = parseJSONL(in)
	case YAMLProfile:
		reason = parseYAML(in)
	case TOMLProfile:
		reason = parseTOML(in)
	case XMLProfile:
		reason = parseXML(in, false)
	case PlistProfile:
		reason = parsePlist(in)
	case PropertiesProfile:
		reason = parseProperties(in)
	case HCLProfile:
		reason = parseHCL(in)
	case WebManifestProfile:
		reason = parseWebManifest(in)
	case SVGProfile:
		reason = parseSVG(in)
	default:
		return Fact{}, "UNSUPPORTED_SCHEMA"
	}
	if reason != "" {
		return Fact{}, reason
	}
	return coordinate(in), ""
}

func hasTextControl(raw []byte) bool {
	for _, value := range string(raw) {
		if value < 0x20 && value != '\n' {
			return true
		}
		if value >= 0x7f && value <= 0x9f {
			return true
		}
	}
	return false
}

func coordinate(in decodedInput) Fact {
	witness := sha256.Sum256(append([]byte("corvint-structured-data-witness/v1\x00"), in.bytes...))
	return Fact{Kind: "structured.document.coordinate", InputHandle: in.input.Handle, Format: in.input.Family, Profile: Profile, Path: "$", ByteStart: 0, ByteEnd: len(in.bytes), Line: 1, Column: 1, WitnessSHA256: "sha256:" + hex.EncodeToString(witness[:])}
}
