package analyzerstructured

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func matrixFrame(t *testing.T, requestID string, inputs []Input) []byte {
	t.Helper()
	request := Request{Profile: Profile, Family: Family, RequestID: requestID, ScopeID: "scope-1", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(raw, '\n')
}

func matrixInput(handle string, profile FormatProfile, path string, body []byte) Input {
	sum := sha256.Sum256(body)
	return Input{Handle: handle, Family: profile, Path: path, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(body)}
}

func requireBoundReject(t *testing.T, output []byte, requestID, reason string, inputs []Input) {
	t.Helper()
	var result rejected
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("unmarshal rejected output: %v: %s", err, output)
	}
	if result.Profile != Profile || result.Family != Family || result.RequestID != requestID || result.ScopeID != "scope-1" || result.CompilationUnitID != "unit-1" || result.Reason != reason || result.Target.OS != "darwin" || result.Target.Architecture != "arm64" || result.Target.ABI != "none" || result.Target.Features == nil || len(result.Target.Features) != 0 {
		t.Fatalf("unbound result=%s", output)
	}
	if len(result.InputEchoes) != len(inputs) {
		t.Fatalf("echo count=%d want=%d output=%s", len(result.InputEchoes), len(inputs), output)
	}
	for index, input := range inputs {
		if result.InputEchoes[index] != (Echo{Handle: input.Handle, SHA256: input.SHA256}) {
			t.Fatalf("echo[%d]=%+v want=%+v", index, result.InputEchoes[index], Echo{Handle: input.Handle, SHA256: input.SHA256})
		}
	}
}

func TestUnknownProfileRejectionStaysBound(t *testing.T) {
	inputs := []Input{matrixInput("input-1", JSONProfile, "data/input.json", []byte(`{"a":1}`))}
	var results []string
	for _, profile := range []string{"corvint-structured-data/future", "corvint-structured-data/other"} {
		frame := bytes.Replace(matrixFrame(t, "request-1", inputs), []byte(`"profile":"`+Profile+`"`), []byte(`"profile":"`+profile+`"`), 1)
		output := string(Analyze(frame))
		want := `{"profile":"` + profile + `","family":"` + Family + `","request_id":"request-1","status":"REJECTED","scope_id":"scope-1","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"` + inputs[0].SHA256 + `"}],"reason":"NONCANONICAL_REQUEST"}` + "\n"
		if output != want {
			t.Fatalf("profile=%s got=%q want=%q", profile, output, want)
		}
		results = append(results, output)
	}
	if results[0] == results[1] {
		t.Fatal("differently profiled requests collapsed to one rejection")
	}
}

func TestAllProfilesBindExactRejections(t *testing.T) {
	cases := []struct {
		profile FormatProfile
		path    string
		body    string
		reason  string
	}{
		{JSONProfile, "data/a.json", `{"a":1,"a":2}`, "MALFORMED_INPUT"},
		{JSONLProfile, "data/a.jsonl", "1\n", "MALFORMED_INPUT"},
		{YAMLProfile, "data/a.yaml", "a: &x\n", "UNSUPPORTED_SCHEMA"},
		{TOMLProfile, "data/a.toml", "a.b = 1\n", "UNSUPPORTED_SCHEMA"},
		{XMLProfile, "data/a.xml", `<root xmlns="urn:foreign"/>`, "UNSUPPORTED_SCHEMA"},
		{PlistProfile, "data/a.plist", "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<root/>", "UNSUPPORTED_SCHEMA"},
		{PropertiesProfile, "data/a.properties", "a=one\\\n", "MALFORMED_INPUT"},
		{HCLProfile, "data/a.hcl", "a = ${b}\n", "UNSUPPORTED_SCHEMA"},
		{WebManifestProfile, "data/a.webmanifest", `{"unknown":"value"}`, "UNKNOWN_FIELD"},
		{SVGProfile, "data/a.svg", `<svg xmlns="http://www.w3.org/2000/svg"><foreign xmlns="urn:foreign"/></svg>`, "UNSUPPORTED_SCHEMA"},
	}
	for _, tc := range cases {
		t.Run(string(tc.profile), func(t *testing.T) {
			inputs := []Input{
				matrixInput("input-01", tc.profile, tc.path, []byte(tc.body)),
				matrixInput("input-02", tc.profile, "data/second", []byte("unused")),
			}
			output := Analyze(matrixFrame(t, "request-"+string(tc.profile[11]), inputs))
			requireBoundReject(t, output, "request-"+string(tc.profile[11]), tc.reason, inputs)
		})
	}
}

func TestTextProfilesRejectInvalidUTF8(t *testing.T) {
	for _, profile := range formatProfiles {
		t.Run(string(profile), func(t *testing.T) {
			inputs := []Input{matrixInput("input-1", profile, "data/input", []byte{0xff})}
			output := Analyze(matrixFrame(t, "invalid-utf8", inputs))
			requireBoundReject(t, output, "invalid-utf8", "MALFORMED_INPUT", inputs)
		})
	}
}

func TestClosedLexicalAndXMLBoundaryMatrix(t *testing.T) {
	cases := []struct {
		name    string
		profile FormatProfile
		body    string
		reason  string
	}{
		{"jsonl-crlf", JSONLProfile, "{}\r\n", "MALFORMED_INPUT"},
		{"jsonl-scalar", JSONLProfile, "1\n", "MALFORMED_INPUT"},
		{"yaml-control", YAMLProfile, "a: \"x\x01\"\n", "MALFORMED_INPUT"},
		{"yaml-bad-number", YAMLProfile, "a: 01\n", "MALFORMED_INPUT"},
		{"yaml-infinite-float", YAMLProfile, "a: .inf\n", "MALFORMED_INPUT"},
		{"yaml-mixed-case-bool", YAMLProfile, "a: True\n", "MALFORMED_INPUT"},
		{"yaml-leading-plus-number", YAMLProfile, "a: +7\n", "MALFORMED_INPUT"},
		{"yaml-leading-dot-float", YAMLProfile, "a: .5\n", "MALFORMED_INPUT"},
		{"yaml-negative-zero", YAMLProfile, "a: -0\n", "MALFORMED_INPUT"},
		{"yaml-negative-leading-zero", YAMLProfile, "a: -01\n", "MALFORMED_INPUT"},
		{"yaml-negative-float", YAMLProfile, "a: -1.5\n", "MALFORMED_INPUT"},
		{"yaml-negative-dot-float", YAMLProfile, "a: -.5\n", "MALFORMED_INPUT"},
		{"yaml-typed-key-leading-zero", YAMLProfile, "1: a\n01: b\n", "MALFORMED_INPUT"},
		{"yaml-typed-key-mixed-case-bool", YAMLProfile, "true: a\nTrue: b\n", "MALFORMED_INPUT"},
		{"yaml-typed-key-infinite-float", YAMLProfile, ".inf: a\n", "MALFORMED_INPUT"},
		{"toml-control", TOMLProfile, "a = \"x\x01\"\n", "MALFORMED_INPUT"},
		{"toml-dotted", TOMLProfile, "a.b = 1\n", "UNSUPPORTED_SCHEMA"},
		{"toml-overflow", TOMLProfile, "a = 9223372036854775808\n", "MALFORMED_INPUT"},
		{"hcl-control", HCLProfile, "a = \"x\x01\"\n", "MALFORMED_INPUT"},
		{"hcl-overflow", HCLProfile, "a = 9223372036854775808\n", "MALFORMED_INPUT"},
		{"xml-outside", XMLProfile, "prefix<root/>", "MALFORMED_INPUT"},
		{"xml-declaration-after-text", XMLProfile, " \n<?xml version=\"1.0\"?><root/>", "UNSUPPORTED_SCHEMA"},
		{"xml-namespace", XMLProfile, `<root xmlns="urn:foreign"/>`, "UNSUPPORTED_SCHEMA"},
		{"xml-duplicate-attribute", XMLProfile, `<root a="1" a="2"/>`, "DUPLICATE_VALUE"},
		{"xml-unicode-name", XMLProfile, `<røot/>`, "UNSUPPORTED_SCHEMA"},
		{"plist-declaration", PlistProfile, "<?xml version=\"1.0\"?>\n<plist version=\"1.0\"><dict/></plist>", "UNSUPPORTED_SCHEMA"},
		{"svg-foreign-child", SVGProfile, `<svg xmlns="http://www.w3.org/2000/svg"><g xmlns="urn:foreign"/></svg>`, "UNSUPPORTED_SCHEMA"},
		{"svg-duplicate-attribute", SVGProfile, `<svg xmlns="http://www.w3.org/2000/svg" a="1" a="2"/>`, "DUPLICATE_VALUE"},
		{"svg-script", SVGProfile, `<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`, "UNSUPPORTED_SCHEMA"},
		{"webmanifest-null-icons", WebManifestProfile, `{"name":"a","icons":null}`, "UNSUPPORTED_SCHEMA"},
		{"svg-event", SVGProfile, `<svg xmlns="http://www.w3.org/2000/svg" onload="x"/>`, "UNSUPPORTED_SCHEMA"},
		{"svg-style-element", SVGProfile, `<svg xmlns="http://www.w3.org/2000/svg"><style>@import url(http://x)</style></svg>`, "UNSUPPORTED_SCHEMA"},
		{"svg-fill-url-reference", SVGProfile, `<svg xmlns="http://www.w3.org/2000/svg" fill="url(http://x)"/>`, "UNSUPPORTED_SCHEMA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := []Input{matrixInput("input-1", tc.profile, "data/input", []byte(tc.body))}
			output := Analyze(matrixFrame(t, "closed-"+tc.name, inputs))
			requireBoundReject(t, output, "closed-"+tc.name, tc.reason, inputs)
		})
	}
}

func TestJSONRejectsLoneSurrogateAfterValidPair(t *testing.T) {
	body := []byte(`{"value":"\uD800\uDC00\uD800"}`)
	inputs := []Input{matrixInput("input-1", JSONProfile, "data/input.json", body)}
	requireBoundReject(t, Analyze(matrixFrame(t, "lone-surrogate-after-pair", inputs)), "lone-surrogate-after-pair", "MALFORMED_INPUT", inputs)
}

func TestEnvelopeDuplicateChecksRemainLinearAtInputBound(t *testing.T) {
	inputs := make([]Input, maxInputs)
	for index := range inputs {
		inputs[index] = matrixInput(fmt.Sprintf("input-%02d", index), JSONProfile, fmt.Sprintf("data/%02d.json", index), []byte("{}"))
	}
	for index := 1; index < len(inputs); index++ {
		if compareInput(inputs[index], inputs[index-1]) < 0 {
			inputs[index], inputs[index-1] = inputs[index-1], inputs[index]
		}
	}
	if output := Analyze(matrixFrame(t, "input-bound-linear", inputs)); !strings.Contains(string(output), `"status":"CANDIDATE"`) {
		t.Fatalf("input bound rejected: %s", output)
	}
}

func TestDecodedAndEnvelopeBoundMatrix(t *testing.T) {
	for _, size := range []int{maxInputBytes - 1, maxInputBytes, maxInputBytes + 1} {
		t.Run(fmt.Sprintf("decoded-%d", size), func(t *testing.T) {
			body := jsonPadding(size)
			inputs := []Input{matrixInput("input-1", JSONProfile, "data/large.json", body)}
			output := Analyze(matrixFrame(t, fmt.Sprintf("decoded-%d", size), inputs))
			if size <= maxInputBytes {
				if !strings.Contains(string(output), `"status":"CANDIDATE"`) {
					t.Fatalf("accepted decoded bound rejected: %s", output)
				}
				return
			}
			requireBoundReject(t, output, fmt.Sprintf("decoded-%d", size), "LIMIT_EXCEEDED", inputs)
		})
	}

	overBase64 := jsonPadding(maxInputBytes + 3)
	overInput := matrixInput("input-1", JSONProfile, "data/base64.json", overBase64)
	if len(overInput.ContentBase64) != maxBase64Bytes+4 {
		t.Fatalf("base64 boundary=%d", len(overInput.ContentBase64))
	}
	overInputs := []Input{overInput}
	requireBoundReject(t, Analyze(matrixFrame(t, "base64-over", overInputs)), "base64-over", "LIMIT_EXCEEDED", overInputs)

	aggregateInputs := []Input{
		matrixInput("input-1", JSONProfile, "data/one.json", jsonPadding(maxInputBytes/2+1)),
		matrixInput("input-2", JSONProfile, "data/two.json", jsonPadding(maxInputBytes/2+1)),
	}
	requireBoundReject(t, Analyze(matrixFrame(t, "aggregate-over", aggregateInputs)), "aggregate-over", "LIMIT_EXCEEDED", aggregateInputs)

	tooMany := make([]Input, maxInputs+1)
	for index := range tooMany {
		tooMany[index] = matrixInput(fmt.Sprintf("input-%02d", index), JSONProfile, fmt.Sprintf("data/%02d.json", index), []byte("[]"))
	}
	requireBoundReject(t, Analyze(matrixFrame(t, "inputs-over", tooMany)), "inputs-over", "LIMIT_EXCEEDED", tooMany)

	request := Request{Profile: Profile, Family: Family, RequestID: "output-over", ScopeID: "scope-1", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}}
	facts := []Fact{{Kind: strings.Repeat("x", maxOutputBytes)}}
	if prospectiveOutput(request, nil, facts) <= maxOutputBytes {
		t.Fatal("output boundary did not exceed declared cap")
	}
}

func TestStructuralBoundMatrix(t *testing.T) {
	if output := Analyze(make([]byte, maxFrameBytes+1)); !strings.Contains(string(output), `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("frame bound=%s", output)
	}
	cases := []struct {
		name    string
		profile FormatProfile
		body    []byte
		reason  string
	}{
		{"json-string", JSONProfile, []byte(`"` + strings.Repeat("a", maxStringBytes+1) + `"`), "MALFORMED_INPUT"},
		{"json-depth", JSONProfile, []byte(strings.Repeat("[", maxDepth+2) + strings.Repeat("]", maxDepth+2)), "MALFORMED_INPUT"},
		{"jsonl-lines", JSONLProfile, []byte(strings.Repeat("{}\n", maxTokenCount+1)), "LIMIT_EXCEEDED"},
		{"xml-text", XMLProfile, []byte("<root>" + strings.Repeat("a", maxStringBytes+1) + "</root>"), "LIMIT_EXCEEDED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inputs := []Input{matrixInput("input-1", tc.profile, "data/input", tc.body)}
			requireBoundReject(t, Analyze(matrixFrame(t, "structural-"+tc.name, inputs)), "structural-"+tc.name, tc.reason, inputs)
		})
	}
}

// TestWebManifestIconCountLimit confirms an icons array over maxFacts is a
// LIMIT_EXCEEDED bound violation, not a schema defect: it is well-formed and
// every icon is individually valid, only the count exceeds the declared cap.
func TestWebManifestIconCountLimit(t *testing.T) {
	icons := make([]map[string]string, maxFacts+1)
	for index := range icons {
		icons[index] = map[string]string{"src": fmt.Sprintf("icon-%02d.png", index)}
	}
	body, err := json.Marshal(map[string]any{"icons": icons})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []Input{matrixInput("input-1", WebManifestProfile, "data/app.webmanifest", body)}
	requireBoundReject(t, Analyze(matrixFrame(t, "icons-over-limit", inputs)), "icons-over-limit", "LIMIT_EXCEEDED", inputs)
}

func jsonPadding(size int) []byte {
	return []byte("[]" + strings.Repeat(" ", size-2))
}

func TestWebManifestMixedInvalidDeterminism(t *testing.T) {
	cases := []struct {
		name, body, reason string
	}{
		{"unknown-beats-invalid-known", `{"aaa":"x","name":1}`, "UNKNOWN_FIELD"},
		{"unknown-beats-invalid-known-reversed", `{"name":1,"zzz":"x"}`, "UNKNOWN_FIELD"},
		{"invalid-known-alone", `{"name":1,"scope":"/"}`, "UNSUPPORTED_SCHEMA"},
		{"icon-unknown-beats-invalid-known", `{"icons":[{"aaa":"x","src":"a.png","type":1}]}`, "UNKNOWN_FIELD"},
		{"icon-unknown-beats-invalid-known-reversed", `{"icons":[{"src":"a.png","type":1,"zzz":"x"}]}`, "UNKNOWN_FIELD"},
		{"icon-invalid-known-alone", `{"icons":[{"src":"a.png","type":1}]}`, "UNSUPPORTED_SCHEMA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for repeat := 0; repeat < 32; repeat++ {
				inputs := []Input{matrixInput("input-1", WebManifestProfile, "data/app.webmanifest", []byte(tc.body))}
				output := Analyze(matrixFrame(t, "request-w", inputs))
				requireBoundReject(t, output, "request-w", tc.reason, inputs)
			}
		})
	}
}

func TestPropertiesTokenBudgetBoundary(t *testing.T) {
	build := func(pairs int) []byte {
		var body []byte
		for index := 0; index < pairs; index++ {
			body = append(body, []byte(fmt.Sprintf("key%04d=v\n", index))...)
		}
		return body
	}
	under := []Input{matrixInput("input-1", PropertiesProfile, "data/a.properties", build(maxTokenCount/2))}
	if output := Analyze(matrixFrame(t, "request-p", under)); !bytes.Contains(output, []byte(`"status":"CANDIDATE"`)) {
		t.Fatalf("at-budget properties rejected: %s", output)
	}
	over := []Input{matrixInput("input-1", PropertiesProfile, "data/a.properties", build(maxTokenCount/2+1))}
	output := Analyze(matrixFrame(t, "request-p", over))
	requireBoundReject(t, output, "request-p", "LIMIT_EXCEEDED", over)
}

func TestInvalidEnvelopeRejectionStaysUnbound(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		mutate       func(*Request)
	}{
		{"family", "UNKNOWN_FAMILY", func(r *Request) { r.Family = "go" }},
		{"scope", "INVALID_IDENTIFIER", func(r *Request) { r.ScopeID = "bad scope" }},
		{"feature", "INVALID_IDENTIFIER", func(r *Request) { r.Target.Features = []string{"bad feature"} }},
		{"null-features", "NONCANONICAL_REQUEST", func(r *Request) { r.Target.Features = nil }},
		{"digest", "DIGEST_MISMATCH", func(r *Request) { r.Inputs[0].SHA256 = "sha256:BAD" }},
		{"duplicate-handle", "DUPLICATE_VALUE", func(r *Request) { r.Inputs[1].Handle = r.Inputs[0].Handle }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "scope-1", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{matrixInput("input-1", JSONProfile, "data/a.json", []byte(`{}`)), matrixInput("input-2", JSONProfile, "data/b.json", []byte(`{}`))}}
			tc.mutate(&request)
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			want := `{"profile":"` + Profile + `","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + tc.reason + `"}` + "\n"
			if got := string(Analyze(append(raw, '\n'))); got != want {
				t.Fatalf("got=%q want=%q", got, want)
			}
		})
	}
	valid := []Input{matrixInput("input-1", JSONProfile, "data/a.json", []byte(`{}`))}
	frame := bytes.Replace(matrixFrame(t, "request-1", valid), []byte(`"features":[]`), []byte(`"features":["neon"]`), 1)
	if got := string(Analyze(frame)); !strings.Contains(got, `"features":["neon"]`) || !strings.Contains(got, `"reason":"UNSUPPORTED_SCHEMA"`) {
		t.Fatalf("valid unsupported feature must stay bound: %s", got)
	}
}
