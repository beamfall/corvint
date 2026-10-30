package analyzerstructured

import "encoding/json"

// webManifestScalarMembers is the fixed validation order for scalar members,
// so mixed-invalid manifests reject deterministically without map-order races.
var webManifestScalarMembers = [...]string{"name", "short_name", "description", "start_url", "scope", "display", "id", "theme_color", "background_color"}

var webManifestIconMembers = [...]string{"src", "type", "sizes", "purpose"}

// parseWebManifest recognizes a static metadata projection only. It never
// resolves a URL, loads an icon, or attributes application semantics to it.
// Any unknown member rejects before known-member validation, so the emitted
// reason never depends on map iteration order.
func parseWebManifest(in decodedInput) string {
	if !jsonUnique(in.bytes, &tokenBudget{}) {
		return "MALFORMED_INPUT"
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(in.bytes, &object) != nil || len(object) == 0 {
		return "MALFORMED_INPUT"
	}
	for key := range object {
		switch key {
		case "name", "short_name", "description", "start_url", "scope", "display", "id", "theme_color", "background_color", "icons":
		default:
			return "UNKNOWN_FIELD"
		}
	}
	for _, key := range webManifestScalarMembers {
		raw, present := object[key]
		if present && !webScalar(raw) {
			return "UNSUPPORTED_SCHEMA"
		}
	}
	raw, present := object["icons"]
	if !present {
		return ""
	}
	var icons []map[string]json.RawMessage
	if json.Unmarshal(raw, &icons) != nil || icons == nil {
		return "UNSUPPORTED_SCHEMA"
	}
	if len(icons) > maxFacts {
		return "LIMIT_EXCEEDED"
	}
	for _, icon := range icons {
		if _, ok := icon["src"]; !ok {
			return "UNSUPPORTED_SCHEMA"
		}
		for field := range icon {
			if field != "src" && field != "type" && field != "sizes" && field != "purpose" {
				return "UNKNOWN_FIELD"
			}
		}
		for _, field := range webManifestIconMembers {
			value, ok := icon[field]
			if ok && !webScalar(value) {
				return "UNSUPPORTED_SCHEMA"
			}
		}
	}
	return ""
}

func webScalar(raw json.RawMessage) bool {
	var value string
	return json.Unmarshal(raw, &value) == nil && value != "" && len(value) <= maxStringBytes && !hasTextControl([]byte(value))
}
