package analyzerkotlinandroid

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestPinnedBeamfallCorpusProducesCoordinateBoundFacts(t *testing.T) {
	request := pinned(t)
	raw := canonical(t, request)
	first := mustProcess(t, raw)
	for index := 0; index < 1000; index++ {
		request.RequestID = "request-" + string(rune('a'+index%26)) + "-" + strings.Repeat("x", index%31)
		got := mustProcess(t, canonical(t, request))
		if len(got) == 0 || !bytes.HasSuffix(got, []byte("\n")) {
			t.Fatalf("unframed output")
		}
	}
	var response result
	if err := json.Unmarshal(first, &response); err != nil {
		t.Fatal(err)
	}
	if response.Status != "CANDIDATE" || response.Reason != "NONE" || len(response.Facts) < 20 {
		t.Fatalf("response=%+v", response)
	}
	for index, fact := range response.Facts {
		if fact.StartLine < 1 || fact.StartColumn < 1 || fact.EvidenceSHA256 == "" {
			t.Fatalf("fact %d is not coordinate-bound: %+v", index, fact)
		}
		if index > 0 && compareFact(response.Facts[index-1], fact) >= 0 {
			t.Fatalf("facts not typed-sorted")
		}
	}
	for _, want := range []string{"android.project.revision", "android.ui.revision", "android.gradle.plugin", "android.gradle.dependency", "android.version.catalog", "kotlin.package", "kotlin.declaration"} {
		if !hasKind(response.Facts, want) {
			t.Fatalf("missing %s", want)
		}
	}
}

func TestLexerRejectsCommentsStringsAndMalformedSyntax(t *testing.T) {
	input := Input{Handle: "source", Path: "x.kt", SHA256: "sha256:" + strings.Repeat("0", 64)}
	request := Request{RequestID: "request", ScopeID: "scope", CompilationUnitID: "unit", Target: Target{OS: "android", Architecture: "arm64", ABI: "none"}}
	source := []byte("package a.b\n// import fake.value\nval quoted = \"class Fake()\"\nval raw = \"\"\"fun Fake()\"\"\"\n@Composable\ncontext(Foo)\nclass Real<T : Any?>\nfun Real.call(value: String?) = value.length\n")
	facts, reason := kotlinFacts(request, input, source)
	if reason != "" {
		t.Fatal(reason)
	}
	for _, forbidden := range []string{"fake", "Fake"} {
		for _, fact := range facts {
			if fact.Value == forbidden {
				t.Fatalf("extracted %q from comment/string", forbidden)
			}
		}
	}
	if !hasKind(facts, "kotlin.annotation") || !hasKind(facts, "kotlin.context.receiver") || !hasKind(facts, "kotlin.type.generic") || !hasKind(facts, "kotlin.type.nullable") || !hasKind(facts, "kotlin.call.static") {
		t.Fatalf("incomplete lexical facts: %+v", facts)
	}
	if _, reason = kotlinFacts(request, input, []byte("package a.b\n/* nested /* unterminated")); reason != "UNSUPPORTED_SCHEMA" {
		t.Fatalf("reason=%s", reason)
	}
}

func TestRejectionsAreClosed(t *testing.T) {
	if got := reason(t, mustProcess(t, []byte("{\"profile\":\"x\"}\n"))); got != "NONCANONICAL_REQUEST" {
		t.Fatal(got)
	}
	request := pinned(t)
	request.Target.OS = "linux"
	response := mustProcess(t, canonical(t, request))
	if got := reason(t, response); got != "EXACT_BINDING_UNAVAILABLE" {
		t.Fatal(got)
	}
	if bytes.Contains(response, []byte("facts")) {
		t.Fatalf("facts leaked: %s", response)
	}
	request = pinned(t)
	request.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	if got := reason(t, mustProcess(t, canonical(t, request))); got != "DIGEST_MISMATCH" {
		t.Fatal(got)
	}
	request = pinned(t)
	request.Inputs[1].Handle = request.Inputs[0].Handle
	sortInputs(request.Inputs)
	if got := reason(t, mustProcess(t, canonical(t, request))); got != "DUPLICATE_VALUE" {
		t.Fatal(got)
	}
}

func TestReviewRound1RegressionWitnesses(t *testing.T) {
	request := pinned(t)
	request.Inputs[0].ContentBase64 = "="
	if got := reason(t, mustProcess(t, canonical(t, request))); got != "MALFORMED_INPUT" {
		t.Fatalf("malformed padding got %q", got)
	}
	request = pinned(t)
	request.Target.Features = []string{"bad feature"}
	if got := reason(t, mustProcess(t, canonical(t, request))); got != "INVALID_IDENTIFIER" {
		t.Fatalf("malformed feature got %q", got)
	}
	deep := strings.Repeat(`{"x":`, 9) + "0" + strings.Repeat("}", 9) + "\n"
	if got := reason(t, mustProcess(t, []byte(deep))); got != "LIMIT_EXCEEDED" {
		t.Fatalf("depth overflow got %q", got)
	}
	beyondValidator := strings.Repeat("[", 10_001) + "0" + strings.Repeat("]", 10_001) + "\n"
	if got := reason(t, mustProcess(t, []byte(beyondValidator))); got != "LIMIT_EXCEEDED" {
		t.Fatalf("beyond-validator depth got %q", got)
	}
	output := mustProcess(t, canonical(t, pinned(t)))
	if bytes.Contains(output, []byte("*.sh")) {
		t.Fatalf("build-file fileTree include leaked as a project fact: %s", output)
	}
}

func TestProspectiveLimitsAndReaderError(t *testing.T) {
	reader := &countingReader{Reader: strings.NewReader(strings.Repeat("x", MaxRequestBytes+1))}
	if got := reason(t, mustReader(t, reader)); got != "LIMIT_EXCEEDED" || reader.n != MaxRequestBytes+1 {
		t.Fatalf("%s %d", got, reader.n)
	}
	if got := reason(t, mustReader(t, errorReader{})); got != "NONCANONICAL_REQUEST" {
		t.Fatal(got)
	}
}

func TestProcessFileKeepsOneDescriptorIdentity(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readEnd.Close()
	raw := canonical(t, pinned(t))
	go func() { _, _ = writeEnd.Write(raw); _ = writeEnd.Close() }()
	output, err := ProcessFile(readEnd)
	if err != nil || !bytes.HasSuffix(output, []byte("\n")) {
		t.Fatalf("output=%q err=%v", output, err)
	}
}

func TestStableDescriptorRejectsSameInodeMutation(t *testing.T) {
	path := t.TempDir() + "/request.json"
	raw := canonical(t, pinned(t))
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, reason := readStableDescriptor(file, func() {
		if _, err := file.WriteAt([]byte{'X'}, 0); err != nil {
			t.Fatal(err)
		}
	}); reason != "NONCANONICAL_REQUEST" {
		t.Fatalf("reason=%q", reason)
	}
}

func TestCanonicalNullFeaturesRejectsWithoutEchoingNull(t *testing.T) {
	request := pinned(t)
	request.Target.Features = nil
	output := mustProcess(t, canonical(t, request))
	if got := reason(t, output); got != "NONCANONICAL_REQUEST" {
		t.Fatalf("reason=%q", got)
	}
	if bytes.Contains(output, []byte(`"features":null`)) {
		t.Fatalf("noncanonical target leaked into rejection: %s", output)
	}
}

func TestInvalidEnvelopeRejectionStaysUnbound(t *testing.T) {
	for want, edit := range map[string]func(*Request){
		"INVALID_IDENTIFIER": func(request *Request) { request.Inputs[0].Handle = "bad handle" },
		"DIGEST_MISMATCH":    func(request *Request) { request.Inputs[0].SHA256 = "invalid" },
		"DUPLICATE_VALUE":    func(request *Request) { request.Inputs[1].Handle = request.Inputs[0].Handle },
	} {
		request := pinned(t)
		edit(&request)
		sentinel, _ := encodeSentinel(want)
		if got := mustProcess(t, canonical(t, request)); !bytes.Equal(got, sentinel) {
			t.Fatalf("got=%s want=%s", got, sentinel)
		}
	}
}

func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	request := pinned(t)
	body := strings.TrimSuffix(string(canonical(t, request)), "\n")
	base := strings.TrimSuffix(body, "}")
	want, _ := encodeRejected(request, "UNKNOWN_FIELD")
	for _, edited := range []string{
		base + `,"x":[9007199254740993,{"a":"b","c":null}]}`,
		base + `,"target inputs":0}`,
		strings.Replace(body, `"features":[]`, `"features":[],"x":true`, 1),
		strings.Replace(body, `"}]}`, `","x":0,"y":"z"}]}`, 1),
	} {
		if got := mustProcess(t, []byte(edited+"\n")); !bytes.Equal(got, want) {
			t.Fatalf("frame=%s\ngot=%s", edited[len(edited)-80:], got)
		}
	}
	for edited, reason := range map[string]string{
		base + `,"x":1.0}`: "NONCANONICAL_REQUEST", base + `,"y":0,"x":0}`: "NONCANONICAL_REQUEST", base + `,"x":{"b":0,"a":0}}`: "NONCANONICAL_REQUEST",
		strings.Replace(body, `{"profile":`, `{"x":0,"profile":`, 1):  "NONCANONICAL_REQUEST",
		strings.Replace(base, `"sha256:`, `"sha256:X`, 1) + `,"x":0}`: "DIGEST_MISMATCH",
	} {
		if got, sentinel := mustProcess(t, []byte(edited+"\n")), rejectSentinel(reason); !bytes.Equal(got, sentinel) {
			t.Fatalf("frame=%s\ngot=%s", edited[len(edited)-80:], got)
		}
	}
}

func TestBoundedOutputEncoderUnderAtAndOver(t *testing.T) {
	request := pinned(t)
	probe := Fact{Kind: "k", InputHandle: "i", RelatedHandle: "-", Subject: "s", Predicate: "p", Value: "", InstanceID: "u", EvidenceSHA256: "sha256:x", StartLine: 1, StartColumn: 1, EndLine: 1, EndColumn: 1}
	zero, ok := encodeSuccess(request, []Fact{probe})
	if !ok || len(zero) >= MaxOutputBytes {
		t.Fatalf("zero=%d ok=%v", len(zero), ok)
	}
	for _, want := range []struct {
		name      string
		size      int
		ok        bool
		outputLen int
	}{
		{"under", MaxOutputBytes - len(zero) - 1, true, MaxOutputBytes - 1},
		{"at", MaxOutputBytes - len(zero), true, MaxOutputBytes},
		{"over", MaxOutputBytes - len(zero) + 1, false, 0},
	} {
		t.Run(want.name, func(t *testing.T) {
			probe.Value = strings.Repeat("x", want.size)
			output, ok := encodeSuccess(request, []Fact{probe})
			if ok != want.ok || len(output) > MaxOutputBytes {
				t.Fatalf("size=%d ok=%v", len(output), ok)
			}
			if want.ok && len(output) != want.outputLen {
				t.Fatalf("output=%d", len(output))
			}
		})
	}
	request.Inputs = []Input{{Handle: "h", SHA256: "sha256:x"}}
	zero, ok = encodeRejected(request, "INVALID_IDENTIFIER")
	if !ok || len(zero) >= MaxOutputBytes {
		t.Fatalf("rejection zero=%d ok=%v", len(zero), ok)
	}
	for _, want := range []struct {
		name string
		size int
		ok   bool
	}{
		{"under", MaxOutputBytes - len(zero), true},
		{"at", MaxOutputBytes - len(zero) + 1, true},
		{"over", MaxOutputBytes - len(zero) + 2, false},
	} {
		t.Run("rejection-"+want.name, func(t *testing.T) {
			request.Inputs[0].Handle = strings.Repeat("x", want.size)
			output, ok := encodeRejected(request, "INVALID_IDENTIFIER")
			if ok != want.ok || len(output) > MaxOutputBytes {
				t.Fatalf("size=%d ok=%v", len(output), ok)
			}
		})
	}
}

func TestHostileOversizedRejectionNeverEscapesOutputCap(t *testing.T) {
	request := pinned(t)
	request.Inputs[0].Handle = strings.Repeat("a", 1_100_308)
	output := mustProcess(t, canonical(t, request))
	if len(output) > MaxOutputBytes || reason(t, output) != "INVALID_IDENTIFIER" || bytes.Contains(output, []byte(`"input_echoes"`)) {
		t.Fatalf("size=%d output=%s", len(output), output)
	}
}

func TestRequestByteLimitUnderAtAndOver(t *testing.T) {
	for _, want := range []struct {
		name   string
		size   int
		reason string
	}{
		{"under", MaxRequestBytes - 1, "NONCANONICAL_REQUEST"},
		{"at", MaxRequestBytes, "NONCANONICAL_REQUEST"},
		{"over", MaxRequestBytes + 1, "LIMIT_EXCEEDED"},
	} {
		t.Run(want.name, func(t *testing.T) {
			reader := &countingReader{Reader: strings.NewReader(strings.Repeat("x", want.size))}
			output := mustReader(t, reader)
			if got := reason(t, output); got != want.reason {
				t.Fatalf("reason=%q", got)
			}
			if want.name == "over" && reader.n != MaxRequestBytes+1 {
				t.Fatalf("read=%d", reader.n)
			}
		})
	}
}

// TestGradleStringInterpolationRejectsAsDynamicInputAndLiteralDollarIsKept
// guards the fix for an interpolated string becoming a fact whose value is
// not in the source: a real template ($name or ${...}) must reject the whole
// request rather than emit an invented `<template>` fact, and a literal `$`
// not followed by an identifier or `{` must not be treated as a template.
func TestGradleStringInterpolationRejectsAsDynamicInputAndLiteralDollarIsKept(t *testing.T) {
	request := Request{RequestID: "request", ScopeID: "scope", CompilationUnitID: "unit", Target: Target{OS: "android", Architecture: "arm64", ABI: "none", Features: []string{}}}
	input := Input{Handle: "gradle", Path: "build.gradle.kts", SHA256: "sha256:" + strings.Repeat("0", 64)}
	if _, reason := gradleFacts(request, input, []byte("implementation(\"com.squareup.okhttp3:okhttp:$okhttpVersion\")\n")); reason != "DYNAMIC_INPUT" {
		t.Fatalf("reason=%q, want DYNAMIC_INPUT", reason)
	}
	facts, reason := gradleFacts(request, input, []byte("implementation(\"Save $5 today\")\n"))
	if reason != "" {
		t.Fatal(reason)
	}
	if _, ok := findFact(facts, "android.gradle.dependency", "implementation", "Save $5 today"); !ok {
		t.Fatalf("literal dollar sign was corrupted; facts=%+v", facts)
	}
}

func TestGradleAliasesRetainFullCoordinateAndSpan(t *testing.T) {
	request := Request{RequestID: "request", ScopeID: "scope", CompilationUnitID: "unit", Target: Target{OS: "android", Architecture: "arm64", ABI: "none", Features: []string{}}}
	input := Input{Handle: "gradle", Path: "build.gradle.kts", SHA256: "sha256:" + strings.Repeat("0", 64)}
	facts, reason := gradleFacts(request, input, []byte("alias(libs.plugins.roborazzi)\nimplementation(libs.androidx.activity.compose)\nimplementation(platform(libs.compose.bom))\n"))
	if reason != "" {
		t.Fatal(reason)
	}
	want := map[string][2]int{"libs.plugins.roborazzi": {1, 29}, "libs.androidx.activity.compose": {2, 46}, "libs.compose.bom": {3, 41}}
	for _, fact := range facts {
		if end, ok := want[fact.Value]; ok {
			if fact.StartLine != end[0] || fact.EndLine != end[0] || fact.EndColumn != end[1] {
				t.Fatalf("%s coordinates=%d:%d-%d:%d", fact.Value, fact.StartLine, fact.StartColumn, fact.EndLine, fact.EndColumn)
			}
			delete(want, fact.Value)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing aliases: %v", want)
	}
	for _, fact := range facts {
		if fact.Value == "libs" {
			t.Fatalf("collapsed alias: %+v", fact)
		}
	}
}

func TestPinnedGradleConfigurationAliasesAndLiteralSpans(t *testing.T) {
	response := candidateResult(t, canonical(t, pinned(t)))
	wants := []struct {
		kind, predicate, value, evidence           string
		startLine, startColumn, endLine, endColumn int
	}{
		{"android.gradle.plugin", "pins-version", "9.2.1", "", 2, 44, 2, 49},
		{"android.gradle.dependency", "debugImplementation", "libs.compose.ui.tooling", "sha256:d93951d182eca28e5bc698bf82289a6886fd0ede1316e5102ee41d4a62593219", 189, 25, 189, 48},
		{"android.gradle.dependency.alias", "debugImplementation", "libs.compose.ui.tooling", "sha256:846353062a644fb6586d1d58275e409df098f18a24794eebd2597ec8bcc60eb6", 189, 25, 189, 48},
		{"android.gradle.dependency", "debugImplementation", "libs.compose.ui.test.manifest", "sha256:04c23799b1be487113473a47359417bea9ca5825187bf3d778191f1c9fe85e06", 192, 25, 192, 54},
		{"android.gradle.dependency.alias", "debugImplementation", "libs.compose.ui.test.manifest", "sha256:4bd5bb6a597bc09b7590ef627ea33dca46a56dd4b697811995912e21cbfd1ad4", 192, 25, 192, 54},
	}
	for _, want := range wants {
		fact, ok := findFact(response.Facts, want.kind, want.predicate, want.value)
		if !ok {
			t.Fatalf("missing %+v", want)
		}
		if fact.StartLine != want.startLine || fact.StartColumn != want.startColumn || fact.EndLine != want.endLine || fact.EndColumn != want.endColumn {
			t.Fatalf("%s/%s span=%d:%d-%d:%d want=%d:%d-%d:%d", want.kind, want.value, fact.StartLine, fact.StartColumn, fact.EndLine, fact.EndColumn, want.startLine, want.startColumn, want.endLine, want.endColumn)
		}
		if want.evidence != "" && fact.EvidenceSHA256 != want.evidence {
			t.Fatalf("%s/%s evidence=%s", want.kind, want.value, fact.EvidenceSHA256)
		}
	}
	for _, configuration := range []string{"implementation", "testImplementation", "androidTestImplementation"} {
		if _, ok := findFact(response.Facts, "android.gradle.dependency", configuration, "libs.compose.bom"); !ok {
			t.Fatalf("missing pinned %s BOM", configuration)
		}
	}
	input := Input{Handle: "gradle", Path: "build.gradle.kts", SHA256: "sha256:" + strings.Repeat("0", 64)}
	request := Request{RequestID: "request", ScopeID: "scope", CompilationUnitID: "unit", Target: Target{OS: "android", Architecture: "arm64", ABI: "none", Features: []string{}}}
	facts, reason := gradleFacts(request, input, []byte("debugImplementation(libs.debug.fixture)\nreleaseImplementation(libs.release.fixture)\n"))
	if reason != "" {
		t.Fatal(reason)
	}
	if _, ok := findFact(facts, "android.gradle.dependency", "debugImplementation", "libs.debug.fixture"); !ok {
		t.Fatal("debugImplementation alias was not recognized")
	}
	if _, ok := findFact(facts, "android.gradle.dependency", "releaseImplementation", "libs.release.fixture"); ok {
		t.Fatal("un-pinned configuration matched")
	}
}

func TestFactSpansCoverLiteralClaimedBytes(t *testing.T) {
	request := pinned(t)
	response := candidateResult(t, canonical(t, request))
	contents := map[string][]byte{}
	for _, input := range request.Inputs {
		content, err := base64.StdEncoding.DecodeString(input.ContentBase64)
		if err != nil {
			t.Fatal(err)
		}
		contents[input.Handle] = content
	}
	for _, fact := range response.Facts {
		if fact.Value == "present" {
			continue
		}
		if got := literalSpan(contents[fact.InputHandle], fact); got != fact.Value {
			t.Fatalf("%s/%s span value=%q want=%q", fact.Kind, fact.InputHandle, got, fact.Value)
		}
	}
}

func TestFactCountUnderAtAndOver(t *testing.T) {
	request, contents := factCountFixture(t, 0)
	base, reason := factsFor(request, contents)
	if reason != "" {
		t.Fatalf("base facts reason=%q", reason)
	}
	for _, count := range []int{4095, 4096, 4097} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			request, contents := factCountFixture(t, count-len(base))
			facts, reason := factsFor(request, contents)
			if count <= 4096 {
				if reason != "" || len(facts) != count {
					t.Fatalf("facts=%d reason=%q", len(facts), reason)
				}
				return
			}
			if reason != "LIMIT_EXCEEDED" || facts != nil {
				t.Fatalf("facts=%d reason=%q", len(facts), reason)
			}
		})
	}
}

func factCountFixture(t *testing.T, declarations int) (Request, [][]byte) {
	t.Helper()
	if declarations < 0 {
		t.Fatal("negative declaration count")
	}
	request := pinned(t)
	contents := make([][]byte, len(request.Inputs))
	for index, item := range request.Inputs {
		content, err := base64.StdEncoding.DecodeString(item.ContentBase64)
		if err != nil {
			t.Fatal(err)
		}
		contents[index] = content
		if item.Family != "kotlin.source" {
			continue
		}
		content = []byte("package bounds.facts\n")
		for declaration := 0; declaration < declarations; declaration++ {
			content = append(content, "class Bound"+strconv.Itoa(declaration)+"\n"...)
		}
		request.Inputs[index] = input(item.Handle, item.Family, item.Path, content)
		contents[index] = content
		key := item.Family + "\x00" + item.Path
		original := pinnedFixtureSHA256[key]
		pinnedFixtureSHA256[key] = request.Inputs[index].SHA256
		t.Cleanup(func() { pinnedFixtureSHA256[key] = original })
	}
	return request, contents
}

func TestLexerRetainsQuotedAndMultilineTokenEnds(t *testing.T) {
	tokens, reason := kotlinTokens([]byte("val x = \"hello\"\nval y = \"\"\"a\nb\"\"\"\n"))
	if reason != "" {
		t.Fatal(reason)
	}
	var quoted []token
	for _, token := range tokens {
		if token.quoted {
			quoted = append(quoted, token)
		}
	}
	if len(quoted) != 2 || quoted[0].line != 1 || quoted[0].column != 9 || quoted[0].endLine != 1 || quoted[0].endColumn != 16 || quoted[1].line != 2 || quoted[1].column != 9 || quoted[1].endLine != 3 || quoted[1].endColumn != 5 {
		t.Fatalf("quoted=%+v", quoted)
	}
}

func pinned(t *testing.T) Request {
	t.Helper()
	fixtures := []struct{ handle, family, filePath, fixture string }{
		{"a-project", "android.project.revision", "project.revision", ""}, {"b-build", "android.gradle.build", "build.gradle.kts", "android-build.gradle.kts"}, {"c-settings", "android.gradle.settings", "settings.gradle.kts", "android-settings.gradle.kts"}, {"d-wrapper", "android.gradle.wrapper", "gradle/wrapper/gradle-wrapper.properties", "android-wrapper.properties"}, {"e-version", "android.project.version", "gradle/version.properties", "android-version.properties"}, {"f-catalog", "android.version.catalog", "gradle/libs.versions.toml", "android-libs.versions.toml"}, {"g-module", "android.gradle.module", "app-mobile/build.gradle.kts", "android-app-mobile.gradle.kts"}, {"h-source", "kotlin.source", "app-mobile/src/main/kotlin/com/beamfall/mobile/BuildIdentity.kt", "android-build-identity.kt"}, {"i-ui-build", "android-ui.gradle.build", "ui/build.gradle.kts", "ui-build.gradle.kts"}, {"j-ui-catalog", "android-ui.version.catalog", "ui/gradle/libs.versions.toml", "ui-libs.versions.toml"}, {"k-ui-source", "android-ui.kotlin.source", "ui/kit/src/main/kotlin/com/beamfall/kit/BeamfallKit.kt", "ui-beamfall-kit.kt"}, {"l-ui-revision", "android.ui.revision", "android-ui.revision", ""},
	}
	inputs := make([]Input, 0, len(fixtures))
	for _, fixture := range fixtures {
		content := []byte(androidRevision + "\n")
		if fixture.family == "android.ui.revision" {
			content = []byte(uiRevision + "\n")
		} else if fixture.fixture != "" {
			var err error
			content, err = os.ReadFile("testdata/" + fixture.fixture)
			if err != nil {
				t.Fatal(err)
			}
		}
		inputs = append(inputs, input(fixture.handle, fixture.family, fixture.filePath, content))
	}
	return Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "beamfall-android@" + androidRevision, CompilationUnitID: "mobile-arm64", Target: Target{OS: "android", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs}
}
func input(handle, family, filePath string, content []byte) Input {
	sum := sha256.Sum256(content)
	return Input{Handle: handle, Family: family, Path: filePath, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString(content)}
}
func canonical(t *testing.T, request Request) []byte {
	t.Helper()
	output, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(output, '\n')
}
func mustProcess(t *testing.T, raw []byte) []byte {
	t.Helper()
	output, err := Process(raw)
	if err != nil {
		t.Fatal(err)
	}
	return output
}
func mustReader(t *testing.T, reader io.Reader) []byte {
	t.Helper()
	output, err := ProcessReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	return output
}
func reason(t *testing.T, output []byte) string {
	t.Helper()
	var value struct{ Reason string }
	if err := json.Unmarshal(output, &value); err != nil {
		t.Fatal(err)
	}
	return value.Reason
}
func hasKind(facts []Fact, want string) bool {
	for _, fact := range facts {
		if fact.Kind == want {
			return true
		}
	}
	return false
}

func candidateResult(t *testing.T, raw []byte) result {
	t.Helper()
	var response result
	if err := json.Unmarshal(mustProcess(t, raw), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func findFact(facts []Fact, kind, predicate, value string) (Fact, bool) {
	for _, fact := range facts {
		if fact.Kind == kind && fact.Predicate == predicate && fact.Value == value {
			return fact, true
		}
	}
	return Fact{}, false
}

func literalSpan(source []byte, fact Fact) string {
	lines := bytes.SplitAfter(source, []byte("\n"))
	if fact.StartLine < 1 || fact.EndLine != fact.StartLine || fact.EndLine > len(lines) || fact.StartColumn < 1 || fact.EndColumn < fact.StartColumn || fact.EndColumn > len(lines[fact.StartLine-1])+1 {
		return ""
	}
	line := lines[fact.StartLine-1]
	return string(line[fact.StartColumn-1 : fact.EndColumn-1])
}
func sortInputs(inputs []Input) {
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Handle < inputs[j].Handle })
}

type countingReader struct {
	io.Reader
	n int
}

func (r *countingReader) Read(buffer []byte) (int, error) {
	n, err := r.Reader.Read(buffer)
	r.n += n
	return n, err
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func BenchmarkAnalyzeCandidate(b *testing.B) {
	request := pinned(&testing.T{})
	raw, _ := json.Marshal(request)
	raw = append(raw, '\n')
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	for index := 0; index < b.N; index++ {
		if _, err := Process(raw); err != nil {
			b.Fatal(err)
		}
	}
}
func TestFixtureBytesArePinned(t *testing.T) {
	request := pinned(t)
	for _, item := range request.Inputs {
		if expected, ok := pinnedFixtureSHA256[item.Family+"\x00"+item.Path]; ok && item.SHA256 != expected {
			t.Fatalf("fixture drift %s", item.Path)
		}
	}
}
func TestDeterministicDigest(t *testing.T) {
	raw := canonical(t, pinned(t))
	want := sha256.Sum256(mustProcess(t, raw))
	for index := 0; index < 1000; index++ {
		if got := sha256.Sum256(mustProcess(t, raw)); got != want {
			t.Fatalf("%x != %x", got, want)
		}
	}
}
