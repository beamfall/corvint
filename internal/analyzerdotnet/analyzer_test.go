package analyzerdotnet

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildRequest produces a canonical request envelope by construction: the
// struct field order is the profile's field order and json.Marshal emits the
// shortest escape, so the bytes it returns are canonical without hand-encoding.
func buildRequest(t *testing.T, inputs ...Input) []byte {
	t.Helper()
	return buildRequestWith(t, "request-1", "root", "unit-1", Target{"darwin", "arm64", "none", []string{}}, inputs...)
}

func buildRequestWith(t *testing.T, requestID, scope, unit string, target Target, inputs ...Input) []byte {
	t.Helper()
	request := Request{Profile, Family, requestID, scope, unit, target, inputs}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return append(encoded, '\n')
}

func input(handle, family, logicalPath, body string) Input {
	sum := sha256.Sum256([]byte(body))
	return Input{
		Handle:        handle,
		Family:        family,
		Path:          logicalPath,
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		ContentBase64: base64.StdEncoding.EncodeToString([]byte(body)),
	}
}

func decodeSuccess(t *testing.T, raw []byte) success {
	t.Helper()
	if raw[len(raw)-1] != '\n' {
		t.Fatalf("response is not LF framed: %q", raw)
	}
	var decoded success
	if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
		t.Fatalf("response is not a success envelope: %s (%v)", raw, err)
	}
	if decoded.Status != "CANDIDATE" {
		t.Fatalf("expected CANDIDATE, got %s", raw)
	}
	return decoded
}

func rejectionReason(t *testing.T, raw []byte) string {
	t.Helper()
	var decoded failure
	if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
		t.Fatalf("response is not a failure envelope: %s (%v)", raw, err)
	}
	if decoded.Status != "REJECTED" {
		t.Fatalf("expected REJECTED, got %s", raw)
	}
	return decoded.Reason
}

// factTuples renders each fact as its typed tuple so a test can assert the
// exact matrix row rather than a substring of JSON.
func factTuples(response success) []string {
	tuples := make([]string, 0, len(response.Facts))
	for _, fact := range response.Facts {
		tuples = append(tuples, strings.Join([]string{fact.Kind, fact.InputHandle,
			fact.RelatedHandle, fact.Subject, fact.Predicate, fact.Value, fact.InstanceID}, "|"))
	}
	return tuples
}

func analyzeOne(t *testing.T, family, logicalPath, body string) []byte {
	t.Helper()
	return AnalyzeCanonical(buildRequest(t, input("input-1", family, logicalPath, body)))
}

func wantFacts(t *testing.T, raw []byte, want ...string) {
	t.Helper()
	got := factTuples(decodeSuccess(t, raw))
	if len(got) != len(want) {
		t.Fatalf("fact count=%d want %d\n got=%v\nwant=%v", len(got), len(want), got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("fact[%d]=%q want %q", index, got[index], want[index])
		}
	}
}

func wantReject(t *testing.T, raw []byte, want string) {
	t.Helper()
	if got := rejectionReason(t, raw); got != want {
		t.Errorf("reason=%s want %s", got, want)
	}
}

// ---------------------------------------------------------------- goldens --

// TestFrozenSuccessVector pins the exact response bytes for one canonical
// .NET request, including both 14-field evidence digests. This is the
// family-local equivalent of the contract vector the profile freezes for the
// `go` family: a contract vector for another family cannot qualify this one.
func TestFrozenSuccessVector(t *testing.T) {
	const wantRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"dotnet","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"dotnet.global-json","path":"global.json","sha256":"sha256:9a3f0f1e1d21b1cd1cbc0dd4b0d0e7ac37e0f0b93bc3b0f7fbaa0e6a5e0aa3ec","content_base64":"eyJzZGsiOnsidmVyc2lvbiI6IjguMC40MjMifX0K"}]}`
	body := "{\"sdk\":{\"version\":\"8.0.423\"}}\n"
	request := buildRequest(t, input("input-1", "dotnet.global-json", "global.json", body))

	// The digest inside wantRequest is recomputed rather than trusted, so this
	// golden cannot drift into asserting a hash of the wrong bytes.
	sum := sha256.Sum256([]byte(body))
	pinned := strings.Replace(wantRequest, "sha256:9a3f0f1e1d21b1cd1cbc0dd4b0d0e7ac37e0f0b93bc3b0f7fbaa0e6a5e0aa3ec",
		"sha256:"+hex.EncodeToString(sum[:]), 1)
	if string(request) != pinned+"\n" {
		t.Fatalf("request bytes drifted:\n got=%s\nwant=%s\n", request, pinned)
	}

	response := AnalyzeCanonical(request)
	decoded := decodeSuccess(t, response)
	wantFacts(t, response, "dotnet.sdk.declaration|input-1|-|dotnet-sdk|declares|8.0.423|root")

	// Recompute the evidence digest from the profile's preimage independently
	// of the production helper's call site.
	fact := decoded.Facts[0]
	expected := evidence(Request{Profile: Profile, Family: Family, RequestID: "request-1",
		ScopeID: "root", CompilationUnitID: "unit-1",
		Target: Target{"darwin", "arm64", "none", []string{}}},
		input("input-1", "dotnet.global-json", "global.json", body), fact)
	if fact.EvidenceSHA256 != expected {
		t.Fatalf("evidence digest=%s want %s", fact.EvidenceSHA256, expected)
	}
	if !digestPattern.MatchString(fact.EvidenceSHA256) {
		t.Fatalf("evidence digest is not sha256 text: %s", fact.EvidenceSHA256)
	}
}

// TestEvidenceDigestBindsEveryFrameField proves the digest is a function of
// every one of the fourteen fields: changing any one changes the digest.
func TestEvidenceDigestBindsEveryFrameField(t *testing.T) {
	base := Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root",
		CompilationUnitID: "unit-1", Target: Target{"darwin", "arm64", "none", []string{}}}
	witness := input("input-1", "dotnet.global-json", "global.json", "x")
	fact := Fact{Kind: "dotnet.sdk.declaration", InputHandle: "input-1", RelatedHandle: "-",
		Subject: "dotnet-sdk", Predicate: "declares", Value: "8.0.423", InstanceID: "root"}
	reference := evidence(base, witness, fact)

	mutations := map[string]func(*Request, *Input, *Fact){
		"request_id":          func(r *Request, _ *Input, _ *Fact) { r.RequestID = "request-2" },
		"scope_id":            func(r *Request, _ *Input, _ *Fact) { r.ScopeID = "other" },
		"compilation_unit_id": func(r *Request, _ *Input, _ *Fact) { r.CompilationUnitID = "unit-2" },
		"target":              func(r *Request, _ *Input, _ *Fact) { r.Target.OS = "linux" },
		"input_handle":        func(_ *Request, _ *Input, f *Fact) { f.InputHandle = "input-2" },
		"input_sha256":        func(_ *Request, i *Input, _ *Fact) { i.SHA256 = "sha256:" + strings.Repeat("0", 64) },
		"kind":                func(_ *Request, _ *Input, f *Fact) { f.Kind = "dotnet.source" },
		"subject":             func(_ *Request, _ *Input, f *Fact) { f.Subject = "other" },
		"predicate":           func(_ *Request, _ *Input, f *Fact) { f.Predicate = "classifies" },
		"value":               func(_ *Request, _ *Input, f *Fact) { f.Value = "9.0.316" },
		"instance_id":         func(_ *Request, _ *Input, f *Fact) { f.InstanceID = "unit-1" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			mutatedRequest, mutatedInput, mutatedFact := base, witness, fact
			mutate(&mutatedRequest, &mutatedInput, &mutatedFact)
			if got := evidence(mutatedRequest, mutatedInput, mutatedFact); got == reference {
				t.Fatalf("mutating %s did not change the evidence digest", name)
			}
		})
	}
}

func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	frame := buildRequest(t, input("sdk", "dotnet.global-json", "global.json", `{"sdk":{"version":"8.0.423"}}`))
	var request Request
	if err := json.Unmarshal(frame, &request); err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSuffix(string(frame), "}\n")
	for _, edited := range []string{
		base + `,"x":[9007199254740993,{"a":"b","c":null}]}`,
		base + `,"target inputs":0}`,
		strings.Replace(base, `"features":[]`, `"features":[],"x":true`, 1) + "}",
		strings.Replace(base, `"}]`, `","x":0,"y":"z"}]`, 1) + "}",
	} {
		if got, want := string(AnalyzeCanonical([]byte(edited+"\n"))), string(bound(request, "UNKNOWN_FIELD")); got != want {
			t.Fatalf("frame=%s\ngot=%s\nwant=%s", edited, got, want)
		}
	}
	for edited, reason := range map[string]string{
		base + `,"x":1.0}`: "NONCANONICAL_REQUEST", base + `,"y":0,"x":0}`: "NONCANONICAL_REQUEST", base + `,"x":{"b":0,"a":0}}`: "NONCANONICAL_REQUEST",
		strings.Replace(base, `{"profile":`, `{"x":0,"profile":`, 1) + "}": "NONCANONICAL_REQUEST",
		strings.Replace(base, `"unit`, `"!unit`, 1) + `,"x":0}`:            "INVALID_IDENTIFIER",
	} {
		if got := string(AnalyzeCanonical([]byte(edited + "\n"))); got != string(sentinel(reason)) {
			t.Fatalf("frame=%s\ngot=%s", edited, got)
		}
	}
}

func TestResponseIsDeterministic(t *testing.T) {
	request := buildRequest(t, input("input-1", "dotnet.global-json", "global.json",
		"{\"sdk\":{\"version\":\"9.0.316\"}}"))
	first := AnalyzeCanonical(request)
	for attempt := 0; attempt < 8; attempt++ {
		if got := AnalyzeCanonical(request); string(got) != string(first) {
			t.Fatalf("response is not deterministic:\n%s\n%s", first, got)
		}
	}
}

// ------------------------------------------------------------ global.json --

func TestGlobalJSON(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"net8 sdk", `{"sdk":{"version":"8.0.423"}}`, ""},
		{"net9 sdk", `{"sdk":{"version":"9.0.316"}}`, ""},
		{"indented is accepted", "{\n  \"sdk\": {\n    \"version\": \"8.0.423\"\n  }\n}\n", ""},
		{"unpinned version", `{"sdk":{"version":"8.0.100"}}`, "UNSUPPORTED_SCHEMA"},
		{"range", `{"sdk":{"version":"8.0.*"}}`, "UNSUPPORTED_SCHEMA"},
		{"rollforward is unknown", `{"sdk":{"version":"8.0.423","rollForward":"latestMinor"}}`, "UNKNOWN_FIELD"},
		{"extra root field", `{"sdk":{"version":"8.0.423"},"msbuild-sdks":{}}`, "UNKNOWN_FIELD"},
		{"missing sdk", `{}`, "MALFORMED_INPUT"},
		{"missing version", `{"sdk":{}}`, "MALFORMED_INPUT"},
		{"duplicate name", `{"sdk":{"version":"8.0.423"},"sdk":{"version":"9.0.316"}}`, "DUPLICATE_VALUE"},
		{"not an object", `["8.0.423"]`, "MALFORMED_INPUT"},
		{"trailing garbage", `{"sdk":{"version":"8.0.423"}} trailing`, "MALFORMED_INPUT"},
		{"second object", `{"sdk":{"version":"8.0.423"}}{"sdk":{"version":"9.0.316"}}`, "MALFORMED_INPUT"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := analyzeOne(t, "dotnet.global-json", "global.json", testCase.body)
			if testCase.want != "" {
				wantReject(t, response, testCase.want)
				return
			}
			facts := factTuples(decodeSuccess(t, response))
			if len(facts) != 1 || !strings.HasPrefix(facts[0], "dotnet.sdk.declaration|input-1|-|dotnet-sdk|declares|") {
				t.Fatalf("unexpected facts %v", facts)
			}
		})
	}
}

func TestGlobalJSONRequiresItsFilename(t *testing.T) {
	response := analyzeOne(t, "dotnet.global-json", "config/sdk.json", `{"sdk":{"version":"8.0.423"}}`)
	wantReject(t, response, "UNSUPPORTED_SCHEMA")
}

// -------------------------------------------------------------------- slnx --

func TestSolutionXML(t *testing.T) {
	body := `<Solution><Project Path="src/App/App.csproj" /><Project Path="test/AppTests/AppTests.csproj" /></Solution>`
	response := AnalyzeCanonical(buildRequest(t, input("input-1", "dotnet.slnx", "Corvint.slnx", body)))
	wantFacts(t, response,
		"dotnet.solution.project|input-1|-|root|contains-project|src/App/App.csproj|src/App/App.csproj",
		"dotnet.solution.project|input-1|-|root|contains-project|test/AppTests/AppTests.csproj|test/AppTests/AppTests.csproj",
	)
}

func TestSolutionXMLResolvesRelativeToItsOwnDirectory(t *testing.T) {
	body := `<Solution><Project Path="../shared/Lib.csproj" /></Solution>`
	response := AnalyzeCanonical(buildRequest(t, input("input-1", "dotnet.slnx", "build/Corvint.slnx", body)))
	wantFacts(t, response, "dotnet.solution.project|input-1|-|root|contains-project|shared/Lib.csproj|shared/Lib.csproj")
}

func TestSolutionXMLRejections(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"folder element", `<Solution><Folder Name="/src/" /></Solution>`, "UNSUPPORTED_SCHEMA"},
		{"root attribute", `<Solution Version="1"><Project Path="a.csproj" /></Solution>`, "UNKNOWN_FIELD"},
		{"project extra attribute", `<Solution><Project Path="a.csproj" Type="cs" /></Solution>`, "UNKNOWN_FIELD"},
		{"project without path", `<Solution><Project Include="a.csproj" /></Solution>`, "UNKNOWN_FIELD"},
		{"namespace", `<Solution xmlns="http://schemas.microsoft.com/developer/msbuild/2003"><Project Path="a.csproj" /></Solution>`, "UNSUPPORTED_SCHEMA"},
		{"wrong root", `<Project><Project Path="a.csproj" /></Project>`, "UNSUPPORTED_SCHEMA"},
		{"duplicate project", `<Solution><Project Path="a.csproj" /><Project Path="./a.csproj" /></Solution>`, "DUPLICATE_VALUE"},
		{"escapes the root", `<Solution><Project Path="../a.csproj" /></Solution>`, "INVALID_PATH"},
		{"backslash separator", `<Solution><Project Path="src\App.csproj" /></Solution>`, "INVALID_PATH"},
		{"glob", `<Solution><Project Path="src/*.csproj" /></Solution>`, "INVALID_PATH"},
		{"text content", `<Solution>text</Solution>`, "MALFORMED_INPUT"},
		{"nested project", `<Solution><Project Path="a.csproj"><Project Path="b.csproj" /></Project></Solution>`, "UNSUPPORTED_SCHEMA"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			wantReject(t, analyzeOne(t, "dotnet.slnx", "Corvint.slnx", testCase.body), testCase.want)
		})
	}
}

// --------------------------------------------------------- XML fail-closed --

// TestXMLSurfaceIsClosed pins that the whole non-element XML surface rejects,
// including the `<?xml ... ?>` prologue, which this subset does not admit.
func TestXMLSurfaceIsClosed(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"xml declaration", "<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<Solution><Project Path=\"a.csproj\" /></Solution>"},
		{"processing instruction", "<?target data?><Solution />"},
		{"comment", "<!-- note --><Solution />"},
		{"doctype", "<!DOCTYPE Solution><Solution />"},
		{"internal entity", "<!DOCTYPE Solution [<!ENTITY x \"y\">]><Solution />"},
		{"character reference", `<Solution><Project Path="a&#x2e;csproj" /></Solution>`},
		{"predefined entity", `<Solution><Project Path="a&amp;b.csproj" /></Solution>`},
		{"prefixed namespace", `<s:Solution xmlns:s="urn:x"><s:Project Path="a.csproj" /></s:Solution>`},
		{"prefixed attribute", `<Solution><Project Path="a.csproj" xml:space="preserve" /></Solution>`},
		{"duplicate attribute", `<Solution><Project Path="a.csproj" Path="b.csproj" /></Solution>`},
		{"two roots", `<Solution /><Solution />`},
		{"trailing text", `<Solution />tail`},
		{"byte order mark", "\xef\xbb\xbf<Solution />"},
		{"carriage return", "<Solution>\r\n</Solution>"},
		{"nul byte", "<Solution>\x00</Solution>"},
		{"unclosed", `<Solution><Project Path="a.csproj" />`},
		{"empty document", ""},
		{"invalid utf8", "<Solution>\xff</Solution>"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := analyzeOne(t, "dotnet.slnx", "Corvint.slnx", testCase.body)
			if reason := rejectionReason(t, response); reason == "" {
				t.Fatalf("expected rejection for %s", testCase.name)
			}
		})
	}
}

func TestXMLIndentationIsInert(t *testing.T) {
	body := "<Solution>\n\t<Project Path=\"a.csproj\" />\n</Solution>\n"
	response := AnalyzeCanonical(buildRequest(t, input("input-1", "dotnet.slnx", "Corvint.slnx", body)))
	wantFacts(t, response, "dotnet.solution.project|input-1|-|root|contains-project|a.csproj|a.csproj")
}

// ----------------------------------------------------------------- project --

const projectPath = "src/App/App.csproj"

func project(t *testing.T, body string) []byte {
	t.Helper()
	return AnalyzeCanonical(buildRequest(t, input("input-1", "dotnet.project", projectPath, body)))
}

func TestProjectCoordinates(t *testing.T) {
	body := `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup>` +
		`<TargetFramework>net8.0</TargetFramework><RuntimeIdentifier>osx-arm64</RuntimeIdentifier>` +
		`<LangVersion>12.0</LangVersion></PropertyGroup></Project>`
	wantFacts(t, project(t, body),
		"dotnet.language.version|input-1|-|src/App/App.csproj|declares-language|12.0|unit-1",
		"dotnet.runtime.identifier|input-1|-|src/App/App.csproj|targets-runtime|osx-arm64|unit-1",
		"dotnet.target.framework|input-1|-|src/App/App.csproj|targets|net8.0|unit-1",
	)
}

func TestProjectPluralListsFanOutOneFactEach(t *testing.T) {
	body := `<Project><PropertyGroup><TargetFrameworks>net8.0;net9.0</TargetFrameworks>` +
		`<RuntimeIdentifiers>linux-x64;osx-arm64;win-x64</RuntimeIdentifiers></PropertyGroup></Project>`
	wantFacts(t, project(t, body),
		"dotnet.runtime.identifier|input-1|-|src/App/App.csproj|targets-runtime|linux-x64|unit-1",
		"dotnet.runtime.identifier|input-1|-|src/App/App.csproj|targets-runtime|osx-arm64|unit-1",
		"dotnet.runtime.identifier|input-1|-|src/App/App.csproj|targets-runtime|win-x64|unit-1",
		"dotnet.target.framework|input-1|-|src/App/App.csproj|targets|net8.0|unit-1",
		"dotnet.target.framework|input-1|-|src/App/App.csproj|targets|net9.0|unit-1",
	)
}

func TestProjectReference(t *testing.T) {
	body := `<Project><ItemGroup><ProjectReference Include="../Lib/Lib.csproj" /></ItemGroup></Project>`
	wantFacts(t, project(t, body),
		"dotnet.project.reference|input-1|-|src/App/App.csproj|references-project|src/Lib/Lib.csproj|src/Lib/Lib.csproj")
}

func TestPackageReferenceIsValidatedButNeverEmitted(t *testing.T) {
	body := `<Project><ItemGroup>` +
		`<PackageReference Include="Serilog" Version="3.1.1" />` +
		`<PackageReference Include="StyleCop.Analyzers" PrivateAssets="all" GeneratePathProperty="true" />` +
		`</ItemGroup></Project>`
	wantFacts(t, project(t, body))
}

func TestProjectSchemaRejections(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"condition on property group",
			`<Project><PropertyGroup Condition="'$(OS)'=='Windows_NT'"><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`, "UNKNOWN_FIELD"},
		{"condition on scalar",
			`<Project><PropertyGroup><TargetFramework Condition="true">net8.0</TargetFramework></PropertyGroup></Project>`, "UNKNOWN_FIELD"},
		{"condition on item",
			`<Project><ItemGroup><PackageReference Include="A" Condition="true" /></ItemGroup></Project>`, "UNKNOWN_FIELD"},
		{"import element",
			`<Project><Import Project="../common.props" /></Project>`, "UNSUPPORTED_SCHEMA"},
		{"target element",
			`<Project><Target Name="Build" /></Project>`, "UNSUPPORTED_SCHEMA"},
		{"property expansion",
			`<Project><PropertyGroup><TargetFramework>$(Tfm)</TargetFramework></PropertyGroup></Project>`, "DYNAMIC_INPUT"},
		{"item expansion in include",
			`<Project><ItemGroup><Compile Include="@(Extra)" /></ItemGroup></Project>`, "DYNAMIC_INPUT"},
		{"metadata expansion",
			`<Project><ItemGroup><Compile Include="%(Thing.Identity)" /></ItemGroup></Project>`, "DYNAMIC_INPUT"},
		{"glob include",
			`<Project><ItemGroup><Compile Include="**/*.cs" /></ItemGroup></Project>`, "INVALID_PATH"},
		{"backslash include",
			`<Project><ItemGroup><ProjectReference Include="..\Lib\Lib.csproj" /></ItemGroup></Project>`, "INVALID_PATH"},
		{"unknown scalar",
			`<Project><PropertyGroup><Nullable>enable</Nullable></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"unpinned framework",
			`<Project><PropertyGroup><TargetFramework>net10.0</TargetFramework></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"unpinned rid",
			`<Project><PropertyGroup><RuntimeIdentifier>win-arm64</RuntimeIdentifier></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"unpinned language",
			`<Project><PropertyGroup><LangVersion>latest</LangVersion></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"padded scalar text",
			"<Project><PropertyGroup><TargetFramework> net8.0 </TargetFramework></PropertyGroup></Project>", "UNSUPPORTED_SCHEMA"},
		{"scalar with a child",
			`<Project><PropertyGroup><TargetFramework><X /></TargetFramework></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"package reference without include",
			`<Project><ItemGroup><PackageReference Version="1.0.0" /></ItemGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"package reference version range",
			`<Project><ItemGroup><PackageReference Include="A" Version="[1.0.0,2.0.0)" /></ItemGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"package reference unknown attribute",
			`<Project><ItemGroup><PackageReference Include="A" ExcludeAssets="all" /></ItemGroup></Project>`, "UNKNOWN_FIELD"},
		{"project reference with extra attribute",
			`<Project><ItemGroup><ProjectReference Include="../Lib/Lib.csproj" PrivateAssets="all" /></ItemGroup></Project>`, "UNKNOWN_FIELD"},
		{"compile with two operations",
			`<Project><ItemGroup><Compile Include="A.cs" Remove="B.cs" /></ItemGroup></Project>`, "UNKNOWN_FIELD"},
		{"compile with unknown operation",
			`<Project><ItemGroup><Compile Exclude="A.cs" /></ItemGroup></Project>`, "UNKNOWN_FIELD"},
		{"unknown item",
			`<Project><ItemGroup><EmbeddedResource Include="A.resx" /></ItemGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"root unknown attribute",
			`<Project ToolsVersion="17.0"><PropertyGroup /></Project>`, "UNKNOWN_FIELD"},
		{"item group text",
			`<Project><ItemGroup>text</ItemGroup></Project>`, "MALFORMED_INPUT"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			wantReject(t, project(t, testCase.body), testCase.want)
		})
	}
}

func TestProjectDuplicateAndConflictingDeclarations(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"repeated framework",
			`<Project><PropertyGroup><TargetFramework>net8.0</TargetFramework><TargetFramework>net9.0</TargetFramework></PropertyGroup></Project>`, "DUPLICATE_VALUE"},
		{"repeated framework across groups",
			`<Project><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`, "DUPLICATE_VALUE"},
		{"repeated boolean",
			`<Project><PropertyGroup><IsTestProject>true</IsTestProject><IsTestProject>false</IsTestProject></PropertyGroup></Project>`, "DUPLICATE_VALUE"},
		{"singular beside plural",
			`<Project><PropertyGroup><TargetFramework>net8.0</TargetFramework><TargetFrameworks>net8.0;net9.0</TargetFrameworks></PropertyGroup></Project>`, "CONFLICTING_VALUE"},
		{"repeated language",
			`<Project><PropertyGroup><LangVersion>12.0</LangVersion><LangVersion>12.0</LangVersion></PropertyGroup></Project>`, "DUPLICATE_VALUE"},
		{"unsorted framework list",
			`<Project><PropertyGroup><TargetFrameworks>net9.0;net8.0</TargetFrameworks></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"duplicate framework list entry",
			`<Project><PropertyGroup><TargetFrameworks>net8.0;net8.0</TargetFrameworks></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"empty framework list",
			`<Project><PropertyGroup><TargetFrameworks></TargetFrameworks></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"duplicate project reference",
			`<Project><ItemGroup><ProjectReference Include="../Lib/Lib.csproj" /><ProjectReference Include="./../Lib/Lib.csproj" /></ItemGroup></Project>`, "DUPLICATE_VALUE"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			wantReject(t, project(t, testCase.body), testCase.want)
		})
	}
}

// ---------------------------------------------------- source classification --

func TestSourceClassificationRequiresDefaultItemsOff(t *testing.T) {
	withDefaults := `<Project><ItemGroup><Compile Include="Program.cs" /></ItemGroup></Project>`
	wantFacts(t, project(t, withDefaults))

	explicit := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><Compile Include="Program.cs" /></ItemGroup></Project>`
	wantFacts(t, project(t, explicit),
		"dotnet.source|input-1|-|src/App/Program.cs|classifies|source|src/App/Program.cs")
}

func TestSourceClassificationValues(t *testing.T) {
	cases := []struct {
		name    string
		isTest  string
		include string
		want    string
	}{
		{"plain source", "false", "Program.cs", "source"},
		{"test project", "true", "ProgramTests.cs", "test"},
		{"roslyn generated", "false", "Model.g.cs", "generated"},
		{"designer generated", "false", "Form1.Designer.cs", "generated"},
		{"intermediate output", "false", "obj/AssemblyInfo.cs", "generated"},
		{"generated wins over test", "true", "Model.g.cs", "generated"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			body := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems>` +
				`<IsTestProject>` + testCase.isTest + `</IsTestProject></PropertyGroup>` +
				`<ItemGroup><Compile Include="` + testCase.include + `" /></ItemGroup></Project>`
			wantFacts(t, project(t, body),
				"dotnet.source|input-1|-|src/App/"+testCase.include+"|classifies|"+testCase.want+
					"|src/App/"+testCase.include)
		})
	}
}

func TestCompileItemsApplyInDocumentOrder(t *testing.T) {
	removedAfterInclude := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><Compile Include="A.cs" /><Compile Include="B.cs" /><Compile Remove="A.cs" /></ItemGroup></Project>`
	wantFacts(t, project(t, removedAfterInclude),
		"dotnet.source|input-1|-|src/App/B.cs|classifies|source|src/App/B.cs")

	includedAfterRemove := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><Compile Remove="A.cs" /><Compile Include="A.cs" /></ItemGroup></Project>`
	wantFacts(t, project(t, includedAfterRemove),
		"dotnet.source|input-1|-|src/App/A.cs|classifies|source|src/App/A.cs")
}

func TestCompileUpdateNeverAddsAnItem(t *testing.T) {
	body := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><Compile Update="A.cs" /></ItemGroup></Project>`
	wantFacts(t, project(t, body))
}

func TestNoneItemsAreNeverSource(t *testing.T) {
	body := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><None Include="readme.txt" /><Compile Include="A.cs" /></ItemGroup></Project>`
	wantFacts(t, project(t, body),
		"dotnet.source|input-1|-|src/App/A.cs|classifies|source|src/App/A.cs")
}

func TestDuplicateCompileIncludeRejects(t *testing.T) {
	body := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><Compile Include="A.cs" /><Compile Include="./A.cs" /></ItemGroup></Project>`
	wantReject(t, project(t, body), "DUPLICATE_VALUE")
}

// --------------------------------------------------------- Directory.Build --

// TestDirectoryBuildWitnessesNothing pins the matrix boundary: a Directory.Build
// file parses under the same grammar (so a malformed one still rejects) but
// mints no fact at all, including `dotnet.source`. MSBuild evaluates a
// Directory.Build file's items against the importing project's directory, not
// the props file's own directory, and this candidate never learns which
// project imports it -- resolving `Compile Include` against the props file's
// own folder would be an invented certainty, not a fact (decision 0174).
func TestDirectoryBuildWitnessesNothing(t *testing.T) {
	body := `<Project><PropertyGroup><TargetFramework>net8.0</TargetFramework>` +
		`<LangVersion>12.0</LangVersion><RuntimeIdentifier>osx-arm64</RuntimeIdentifier>` +
		`<EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><ProjectReference Include="Lib/Lib.csproj" /><Compile Include="Shared.cs" /></ItemGroup></Project>`
	response := AnalyzeCanonical(buildRequest(t,
		input("input-1", "dotnet.directory-build", "src/Directory.Build.props", body)))
	wantFacts(t, response)
}

func TestDirectoryBuildFilenameIsPinned(t *testing.T) {
	body := `<Project />`
	for _, name := range []string{"src/Directory.Build.props", "src/Directory.Build.targets"} {
		decodeSuccess(t, AnalyzeCanonical(buildRequest(t,
			input("input-1", "dotnet.directory-build", name, body))))
	}
	response := AnalyzeCanonical(buildRequest(t, input("input-1", "dotnet.directory-build", "src/common.props", body)))
	wantReject(t, response, "UNSUPPORTED_SCHEMA")
}

// TestFamilyLaunderingIsRefused proves the path gate is load-bearing: a
// Directory.Build file cannot be submitted under the `dotnet.project` family to
// mint coordinates the matrix reserves for a real project file.
func TestFamilyLaunderingIsRefused(t *testing.T) {
	body := `<Project><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`
	response := AnalyzeCanonical(buildRequest(t,
		input("input-1", "dotnet.project", "src/Directory.Build.props", body)))
	wantReject(t, response, "UNSUPPORTED_SCHEMA")
}

// ------------------------------------------------------- central packages --

func TestCentralPackages(t *testing.T) {
	body := `<Project><PropertyGroup><ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally></PropertyGroup>` +
		`<ItemGroup><PackageVersion Include="Serilog" Version="3.1.1" />` +
		`<PackageVersion Include="Xunit" Version="2.9.0" /></ItemGroup></Project>`
	response := AnalyzeCanonical(buildRequest(t,
		input("input-1", "dotnet.central-packages", "Directory.Packages.props", body)))
	wantFacts(t, response,
		"dotnet.package.central|input-1|-|Serilog|central-version|3.1.1|root",
		"dotnet.package.central|input-1|-|Xunit|central-version|2.9.0|root",
	)
}

// TestCentralPackagesWithheldWhenManagementIsOff is the conservative case: a
// PackageVersion governs nothing unless central management is switched on, so
// asserting `central-version` from such a file would state a resolution the
// SDK would not perform.
func TestCentralPackagesWithheldWhenManagementIsOff(t *testing.T) {
	for _, property := range []string{
		`<PropertyGroup><ManagePackageVersionsCentrally>false</ManagePackageVersionsCentrally></PropertyGroup>`,
		``,
	} {
		body := `<Project>` + property +
			`<ItemGroup><PackageVersion Include="Serilog" Version="3.1.1" /></ItemGroup></Project>`
		response := AnalyzeCanonical(buildRequest(t,
			input("input-1", "dotnet.central-packages", "Directory.Packages.props", body)))
		wantFacts(t, response)
	}
}

func TestCentralPackagesRejections(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"sdk attribute on root",
			`<Project Sdk="Microsoft.NET.Sdk"><ItemGroup /></Project>`, "UNKNOWN_FIELD"},
		{"missing version",
			`<Project><ItemGroup><PackageVersion Include="A" /></ItemGroup></Project>`, "UNKNOWN_FIELD"},
		{"version range",
			`<Project><ItemGroup><PackageVersion Include="A" Version="[1.0.0,)" /></ItemGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"leading zero version",
			`<Project><ItemGroup><PackageVersion Include="A" Version="1.01.0" /></ItemGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"prerelease version",
			`<Project><ItemGroup><PackageVersion Include="A" Version="1.0.0-beta" /></ItemGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"duplicate include",
			`<Project><ItemGroup><PackageVersion Include="A" Version="1.0.0" /><PackageVersion Include="A" Version="2.0.0" /></ItemGroup></Project>`, "DUPLICATE_VALUE"},
		{"switch declared in two property groups",
			`<Project><PropertyGroup><ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally></PropertyGroup>` +
				`<PropertyGroup><ManagePackageVersionsCentrally>false</ManagePackageVersionsCentrally></PropertyGroup>` +
				`<ItemGroup><PackageVersion Include="A" Version="1.0.0" /></ItemGroup></Project>`, "DUPLICATE_VALUE"},
		{"unknown property",
			`<Project><PropertyGroup><CentralPackageTransitivePinningEnabled>true</CentralPackageTransitivePinningEnabled></PropertyGroup></Project>`, "UNSUPPORTED_SCHEMA"},
		{"package reference is not a package version",
			`<Project><ItemGroup><PackageReference Include="A" /></ItemGroup></Project>`, "UNSUPPORTED_SCHEMA"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := AnalyzeCanonical(buildRequest(t,
				input("input-1", "dotnet.central-packages", "Directory.Packages.props", testCase.body)))
			wantReject(t, response, testCase.want)
		})
	}
}

func TestMSBuildNamespaceIsAccepted(t *testing.T) {
	body := `<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003"><PropertyGroup>` +
		`<TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`
	wantFacts(t, project(t, body),
		"dotnet.target.framework|input-1|-|src/App/App.csproj|targets|net8.0|unit-1")
}

func TestForeignNamespaceRejects(t *testing.T) {
	body := `<Project xmlns="urn:other"><PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup></Project>`
	wantReject(t, project(t, body), "UNSUPPORTED_SCHEMA")
}

// ------------------------------------------------- unimplemented families --

func TestUnimplementedFamiliesRejectWholesale(t *testing.T) {
	cases := []struct{ family, logicalPath string }{
		{"dotnet.sln", "Corvint.sln"},
		{"dotnet.packages-lock-v1", "packages.lock.json"},
		{"dotnet.nuget-config", "NuGet.config"},
	}
	for _, testCase := range cases {
		t.Run(testCase.family, func(t *testing.T) {
			wantReject(t, analyzeOne(t, testCase.family, testCase.logicalPath, "anything"), "UNSUPPORTED_SCHEMA")
		})
	}
}

func TestUnknownFamilyRejects(t *testing.T) {
	wantReject(t, analyzeOne(t, "dotnet.editorconfig", "src/.editorconfig", "x"), "UNKNOWN_FAMILY")
}

// ---------------------------------------------------------------- envelope --

func TestEnvelopeRejections(t *testing.T) {
	valid := input("input-1", "dotnet.global-json", "global.json", `{"sdk":{"version":"8.0.423"}}`)
	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{"empty", nil, "NONCANONICAL_REQUEST"},
		{"no trailing lf", []byte(`{"profile":"x"}`), "NONCANONICAL_REQUEST"},
		{"two lines", []byte("{}\n\n"), "NONCANONICAL_REQUEST"},
		{"pretty printed", []byte("{\n \"profile\": \"x\"\n}\n"), "NONCANONICAL_REQUEST"},
		{"not json", []byte("nonsense\n"), "NONCANONICAL_REQUEST"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			wantReject(t, AnalyzeCanonical(testCase.raw), testCase.want)
		})
	}

	t.Run("wrong family", func(t *testing.T) {
		request := Request{Profile, "ruby", "request-1", "root", "unit-1",
			Target{"darwin", "arm64", "none", []string{}}, []Input{valid}}
		encoded, _ := json.Marshal(request)
		wantReject(t, AnalyzeCanonical(append(encoded, '\n')), "UNKNOWN_FAMILY")
	})
	t.Run("wrong profile", func(t *testing.T) {
		request := Request{"corvint-analyzer-candidate/other", Family, "request-1", "root", "unit-1",
			Target{"darwin", "arm64", "none", []string{}}, []Input{valid}}
		encoded, _ := json.Marshal(request)
		wantReject(t, AnalyzeCanonical(append(encoded, '\n')), "MALFORMED_INPUT")
	})
	t.Run("unknown field", func(t *testing.T) {
		raw := []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"dotnet","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[],"extra":1}` + "\n")
		wantReject(t, AnalyzeCanonical(raw), "MALFORMED_INPUT")
	})
	t.Run("duplicate envelope field", func(t *testing.T) {
		raw := []byte(`{"profile":"corvint-analyzer-candidate/experimental","profile":"corvint-analyzer-candidate/experimental","family":"dotnet","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[]}` + "\n")
		wantReject(t, AnalyzeCanonical(raw), "NONCANONICAL_REQUEST")
	})
	t.Run("no inputs", func(t *testing.T) {
		wantReject(t, buildAndAnalyze(t), "MALFORMED_INPUT")
	})
	t.Run("bad request id", func(t *testing.T) {
		wantReject(t, AnalyzeCanonical(buildRequestWith(t, "_bad", "root", "unit-1",
			Target{"darwin", "arm64", "none", []string{}}, valid)), "INVALID_IDENTIFIER")
	})
	t.Run("bad scope id", func(t *testing.T) {
		wantReject(t, AnalyzeCanonical(buildRequestWith(t, "request-1", "_bad", "unit-1",
			Target{"darwin", "arm64", "none", []string{}}, valid)), "INVALID_IDENTIFIER")
	})
	t.Run("features are not defined for this family", func(t *testing.T) {
		wantReject(t, AnalyzeCanonical(buildRequestWith(t, "request-1", "root", "unit-1",
			Target{"darwin", "arm64", "none", []string{"anything"}}, valid)), "UNSUPPORTED_SCHEMA")
	})
	t.Run("digest mismatch", func(t *testing.T) {
		lying := valid
		lying.SHA256 = "sha256:" + strings.Repeat("0", 64)
		wantReject(t, AnalyzeCanonical(buildRequest(t, lying)), "DIGEST_MISMATCH")
	})
	t.Run("absolute path", func(t *testing.T) {
		bad := valid
		bad.Path = "/global.json"
		wantReject(t, AnalyzeCanonical(buildRequest(t, bad)), "INVALID_PATH")
	})
	t.Run("dot dot path", func(t *testing.T) {
		bad := valid
		bad.Path = "../global.json"
		wantReject(t, AnalyzeCanonical(buildRequest(t, bad)), "INVALID_PATH")
	})
}

func buildAndAnalyze(t *testing.T, inputs ...Input) []byte {
	t.Helper()
	return AnalyzeCanonical(buildRequest(t, inputs...))
}

func TestInputsMustBeStrictlyOrderedAndUnique(t *testing.T) {
	first := input("input-1", "dotnet.global-json", "global.json", `{"sdk":{"version":"8.0.423"}}`)
	second := input("input-2", "dotnet.slnx", "Corvint.slnx", `<Solution />`)

	wantFacts(t, buildAndAnalyze(t, first, second),
		"dotnet.sdk.declaration|input-1|-|dotnet-sdk|declares|8.0.423|root")
	wantReject(t, buildAndAnalyze(t, second, first), "DUPLICATE_VALUE")
	wantReject(t, buildAndAnalyze(t, first, first), "DUPLICATE_VALUE")

	sameHandle := second
	sameHandle.Handle = "input-1"
	wantReject(t, buildAndAnalyze(t, first, sameHandle), "DUPLICATE_VALUE")
}

// TestInputEchoesPreserveRequestOrder pins the two-field echo schema: the
// four-field descriptor is a JavaScript/TypeScript-only extension and this
// family keeps the frozen common shape.
func TestInputEchoesPreserveRequestOrder(t *testing.T) {
	first := input("input-1", "dotnet.global-json", "global.json", `{"sdk":{"version":"8.0.423"}}`)
	second := input("input-2", "dotnet.slnx", "Corvint.slnx", `<Solution />`)
	response := decodeSuccess(t, buildAndAnalyze(t, first, second))
	if len(response.InputEchoes) != 2 {
		t.Fatalf("echo count=%d", len(response.InputEchoes))
	}
	if response.InputEchoes[0].Handle != "input-1" || response.InputEchoes[1].Handle != "input-2" {
		t.Fatalf("echo order drifted: %+v", response.InputEchoes)
	}
	raw := buildAndAnalyze(t, first, second)
	if strings.Contains(string(raw), `"input_echoes":[{"handle":"input-1","sha256":`) == false {
		t.Fatalf("echo schema is not the frozen two-field record: %s", raw)
	}
}

// TestBoundRejectionEchoesTheWholeEnvelope pins that a post-structural failure
// binds family, request, scope, unit, target, and every input digest, so one
// rejection cannot replay across different inputs.
func TestBoundRejectionEchoesTheWholeEnvelope(t *testing.T) {
	lying := input("input-1", "dotnet.global-json", "global.json", `{"sdk":{"version":"8.0.423"}}`)
	lying.SHA256 = "sha256:" + strings.Repeat("0", 64)
	raw := buildAndAnalyze(t, lying)
	var decoded failure
	if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Family != Family || decoded.RequestID != "request-1" || decoded.ScopeID != "root" ||
		decoded.CompilationUnitID != "unit-1" || decoded.Target == nil || len(decoded.InputEchoes) != 1 {
		t.Fatalf("rejection is not fully bound: %s", raw)
	}
	if decoded.InputEchoes[0].SHA256 != lying.SHA256 {
		t.Fatalf("rejection does not echo the request digest: %s", raw)
	}
}

// TestSentinelCarriesNothingDerived pins that a pre-envelope failure leaks no
// scope, unit, target, echo, or path.
func TestSentinelCarriesNothingDerived(t *testing.T) {
	raw := AnalyzeCanonical([]byte("nonsense\n"))
	const want = `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
	if string(raw) != want {
		t.Fatalf("sentinel bytes drifted:\n got=%s\nwant=%s", raw, want)
	}
}

func TestOversizeRequestIsRejectedByLength(t *testing.T) {
	raw := append([]byte(strings.Repeat("x", MaxRequestBytes+1)), '\n')
	wantReject(t, AnalyzeCanonical(raw), "NONCANONICAL_REQUEST")
}

// TestOutputAccountingMatchesTheEncoder proves the prospective size function
// and the encoder agree byte for byte on a response with many facts; a
// disagreement is what the OUTPUT_LIMIT guard in AnalyzeCanonical would catch.
func TestOutputAccountingMatchesTheEncoder(t *testing.T) {
	var items strings.Builder
	for index := 0; index < 200; index++ {
		items.WriteString(`<Compile Include="File`)
		items.WriteString(string(rune('A' + index%26)))
		items.WriteString(string(rune('a' + index/26)))
		items.WriteString(`.cs" />`)
	}
	body := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup><ItemGroup>` +
		items.String() + `</ItemGroup></Project>`
	response := project(t, body)
	decoded := decodeSuccess(t, response)
	if len(decoded.Facts) != 200 {
		t.Fatalf("fact count=%d want 200", len(decoded.Facts))
	}
}

func TestFactsAreStrictlyIncreasing(t *testing.T) {
	body := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems>` +
		`<TargetFrameworks>net8.0;net9.0</TargetFrameworks><LangVersion>13.0</LangVersion></PropertyGroup>` +
		`<ItemGroup><Compile Include="B.cs" /><Compile Include="A.cs" />` +
		`<ProjectReference Include="../Lib/Lib.csproj" /></ItemGroup></Project>`
	decoded := decodeSuccess(t, project(t, body))
	for index := 1; index < len(decoded.Facts); index++ {
		if !factLess(decoded.Facts[index-1], decoded.Facts[index]) {
			t.Fatalf("facts are not strictly increasing at %d: %+v", index, decoded.Facts)
		}
	}
}

func TestMultipleInputsShareOneRequestBudget(t *testing.T) {
	solution := input("input-1", "dotnet.slnx", "Corvint.slnx",
		`<Solution><Project Path="src/App/App.csproj" /></Solution>`)
	sdk := input("input-2", "dotnet.global-json", "global.json", `{"sdk":{"version":"9.0.316"}}`)
	app := input("input-3", "dotnet.project", "src/App/App.csproj",
		`<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net9.0</TargetFramework></PropertyGroup></Project>`)
	wantFacts(t, buildAndAnalyze(t, solution, sdk, app),
		"dotnet.sdk.declaration|input-2|-|dotnet-sdk|declares|9.0.316|root",
		"dotnet.solution.project|input-1|-|root|contains-project|src/App/App.csproj|src/App/App.csproj",
		"dotnet.target.framework|input-3|-|src/App/App.csproj|targets|net9.0|unit-1",
	)
}

// TestBuildDirectoryIsNotGenerated pins the conservative-planning repair the
// reference implementation had to make: `build/` is the conventional home for
// hand-written MSBuild extensions, so classifying it as tool output silently
// drops real source.
func TestBuildDirectoryIsNotGenerated(t *testing.T) {
	body := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><Compile Include="build/Tasks.cs" /><Compile Include="obj/Gen.cs" />` +
		`<Compile Include="bin/Stale.cs" /><Compile Include="generated/Api.cs" /></ItemGroup></Project>`
	wantFacts(t, project(t, body),
		"dotnet.source|input-1|-|src/App/bin/Stale.cs|classifies|generated|src/App/bin/Stale.cs",
		"dotnet.source|input-1|-|src/App/build/Tasks.cs|classifies|source|src/App/build/Tasks.cs",
		"dotnet.source|input-1|-|src/App/generated/Api.cs|classifies|generated|src/App/generated/Api.cs",
		"dotnet.source|input-1|-|src/App/obj/Gen.cs|classifies|generated|src/App/obj/Gen.cs",
	)
}

// TestEveryExpansionSigilIsCaught pins that the dynamism guard covers item
// lists and metadata, not only property expansion.
func TestEveryExpansionSigilIsCaught(t *testing.T) {
	values := []string{"$(Prop)", "@(Item)", "%(Meta)", "$([MSBuild]::Add(1,2))"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			body := `<Project><PropertyGroup><TargetFramework>` + value + `</TargetFramework></PropertyGroup></Project>`
			wantReject(t, project(t, body), "DYNAMIC_INPUT")
		})
	}
	for _, value := range values {
		t.Run("slnx "+value, func(t *testing.T) {
			body := `<Solution><Project Path="` + value + `/App.csproj" /></Solution>`
			wantReject(t, analyzeOne(t, "dotnet.slnx", "Corvint.slnx", body), "DYNAMIC_INPUT")
		})
	}
}

// TestTestPackagesDoNotImplyTestIdentity pins the deliberate divergence: the
// reference infers is_test from an xunit/nunit package reference; a dependency
// name is not a declaration.
func TestTestPackagesDoNotImplyTestIdentity(t *testing.T) {
	body := `<Project><PropertyGroup><EnableDefaultItems>false</EnableDefaultItems></PropertyGroup>` +
		`<ItemGroup><PackageReference Include="xunit" Version="2.9.0" />` +
		`<Compile Include="Cases.cs" /></ItemGroup></Project>`
	wantFacts(t, project(t, body),
		"dotnet.source|input-1|-|src/App/Cases.cs|classifies|source|src/App/Cases.cs")
}

// benchmarkFixture is one representative multi-input request: a solution, an
// SDK declaration, a central package manifest, and two projects.
func benchmarkFixture(t testing.TB) []byte {
	project := func(name string) string {
		return `<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup>` +
			`<TargetFrameworks>net8.0;net9.0</TargetFrameworks><LangVersion>13.0</LangVersion>` +
			`<RuntimeIdentifiers>linux-x64;osx-arm64</RuntimeIdentifiers>` +
			`<EnableDefaultItems>false</EnableDefaultItems></PropertyGroup><ItemGroup>` +
			`<PackageReference Include="Serilog" Version="3.1.1" />` +
			`<Compile Include="` + name + `.cs" /><Compile Include="` + name + `.g.cs" />` +
			`</ItemGroup></Project>`
	}
	inputs := []Input{
		input("input-1", "dotnet.central-packages", "Directory.Packages.props",
			`<Project><PropertyGroup><ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally></PropertyGroup>`+
				`<ItemGroup><PackageVersion Include="Serilog" Version="3.1.1" /></ItemGroup></Project>`),
		input("input-2", "dotnet.global-json", "global.json", `{"sdk":{"version":"9.0.316"}}`),
		input("input-3", "dotnet.project", "src/App/App.csproj", project("Program")),
		input("input-4", "dotnet.project", "src/Lib/Lib.csproj", project("Library")),
		input("input-5", "dotnet.slnx", "Corvint.slnx",
			`<Solution><Project Path="src/App/App.csproj" /><Project Path="src/Lib/Lib.csproj" /></Solution>`),
	}
	request := Request{Profile, Family, "request-1", "root", "unit-1",
		Target{"darwin", "arm64", "none", []string{}}, inputs}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return append(encoded, '\n')
}

func BenchmarkAnalyzeCandidate(b *testing.B) {
	raw := benchmarkFixture(b)
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		_ = AnalyzeCanonical(raw)
	}
}

// TestFixtureAnalyzesEndToEnd proves the benchmark fixture is a real success
// path rather than a rejection the benchmark would measure instead.
func TestFixtureAnalyzesEndToEnd(t *testing.T) {
	decoded := decodeSuccess(t, AnalyzeCanonical(benchmarkFixture(t)))
	if len(decoded.Facts) != 18 {
		t.Fatalf("fact count=%d want 18:\n%v", len(decoded.Facts), factTuples(decoded))
	}
}

// TestAllocationAndOutputRatchets pins the qualification budget this candidate
// is measured against. Race instrumentation changes both, so it is excluded.
func TestAllocationAndOutputRatchets(t *testing.T) {
	if raceBuild {
		t.Skip("race instrumentation is outside the absolute allocation profile")
	}
	raw := benchmarkFixture(t)
	if got := len(AnalyzeCanonical(raw)); got > MaxOutputBytes {
		t.Fatalf("output bytes=%d max=%d", got, MaxOutputBytes)
	}
	measured := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; index < b.N; index++ {
			_ = AnalyzeCanonical(raw)
		}
	})
	if got := measured.AllocedBytesPerOp(); got > MaxBytesPerOp {
		t.Fatalf("bytes/op=%d max=%d", got, MaxBytesPerOp)
	}
	if got := measured.AllocsPerOp(); got > MaxAllocsPerOp {
		t.Fatalf("allocs/op=%d max=%d", got, MaxAllocsPerOp)
	}
}

// TestSchemaRejectionsAreBound proves an unsupported or unknown input family
// produces a fully bound rejection, not the unbound sentinel. An unbound
// rejection would replay across different inputs at the same commit.
func TestSchemaRejectionsAreBound(t *testing.T) {
	cases := []struct{ name, family, logicalPath, want string }{
		{"unimplemented family", "dotnet.sln", "Corvint.sln", "UNSUPPORTED_SCHEMA"},
		{"wrong filename", "dotnet.global-json", "config/sdk.json", "UNSUPPORTED_SCHEMA"},
		{"unknown family", "dotnet.editorconfig", "src/.editorconfig", "UNKNOWN_FAMILY"},
		{"feature supplied", "", "", "UNSUPPORTED_SCHEMA"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			raw := analyzeOne(t, testCase.family, testCase.logicalPath, "x")
			if testCase.family == "" {
				raw = AnalyzeCanonical(buildRequestWith(t, "request-1", "root", "unit-1",
					Target{"darwin", "arm64", "none", []string{"bash-5.2"}},
					input("input-1", "dotnet.global-json", "global.json", "x")))
			}
			var decoded failure
			if err := json.Unmarshal(raw[:len(raw)-1], &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Reason != testCase.want {
				t.Fatalf("reason=%s want %s", decoded.Reason, testCase.want)
			}
			if decoded.Family != Family || decoded.ScopeID != "root" ||
				decoded.CompilationUnitID != "unit-1" || decoded.Target == nil ||
				len(decoded.InputEchoes) != 1 {
				t.Fatalf("rejection is not bound: %s", raw)
			}
		})
	}
}

// TestStandaloneBinaryCeiling enforces the profile's 6,291,456-byte standalone
// ceiling on the candidate CLI it is declared for, so MaxBinaryBytes is a
// checked bound rather than a documented intention.
func TestStandaloneBinaryCeiling(t *testing.T) {
	if raceBuild {
		t.Skip("race instrumentation is outside the absolute binary profile")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-dotnet")
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build",
		"-trimpath", "-buildvcs=false", "-ldflags=-s -w -buildid=", "-o", binary,
		"./cmd/corvint-analyzer-dotnet")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("binary build: %v\n%s", err, output)
	}
	info, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > MaxBinaryBytes {
		t.Fatalf("binary bytes=%d max=%d", info.Size(), MaxBinaryBytes)
	}
	t.Logf("standalone binary=%d bytes, ceiling=%d, headroom=%d",
		info.Size(), MaxBinaryBytes, MaxBinaryBytes-info.Size())
}
