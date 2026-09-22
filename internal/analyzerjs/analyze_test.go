package analyzerjs

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func input(handle, family, logicalPath, body string) Input {
	sum := sha256.Sum256([]byte(body))
	return Input{Handle: handle, Family: family, Path: logicalPath, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(body))}
}
func candidateRequest(inputs ...Input) Request {
	return Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs}
}
func importsOf(source string) ([]string, error) { return importsOfLimit(source, MaxFacts) }
func canonicalRequest(t *testing.T, request Request) []byte {
	t.Helper()
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}
func fixtureRequest() Request {
	imports := make([]string, 28)
	for index := range imports {
		imports[index] = fmt.Sprintf("import \"pkg%d\";", index)
	}
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	return candidateRequest(
		input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"vitest":"2.0.0"}}`),
		input("input-2", "js.npm-lock-v3", "package-lock.json", fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0","dependencies":{"vitest":"2.0.0"}},"node_modules/vitest":{"name":"vitest","version":"2.0.0","resolved":"https://registry.npmjs.org/vitest/-/vitest-2.0.0.tgz","integrity":"%s"}}}`, integrity)),
		input("input-3", "js.source", "src/test.ts", strings.Join(imports, "")),
		input("input-4", "js.source", "src/a.js", `export {};`),
	)
}
func benchmarkRequest() Request {
	request := fixtureRequest()
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	request.Inputs[0] = input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"v":"2.0.0"}}`)
	request.Inputs[1] = input("input-2", "js.npm-lock-v3", "package-lock.json", fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0","dependencies":{"v":"2.0.0"}},"node_modules/v":{"name":"v","version":"2.0.0","resolved":"https://registry.npmjs.org/v/-/v-2.0.0.tgz","integrity":"%s"}}}`, integrity))
	return request
}
func TestCandidateEnvelope(t *testing.T) {
	candidate, err := Analyze(fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Status != "CANDIDATE" || candidate.Profile != Profile || candidate.Family != Family || len(candidate.InputEchoes) != 4 || len(candidate.Facts) != 32 {
		t.Fatalf("candidate=%#v", candidate)
	}
	found := map[string]bool{}
	for _, fact := range candidate.Facts {
		if !validateFact(fact) {
			t.Fatalf("invalid fact=%#v", fact)
		}
		found[fact.Kind] = true
	}
	for _, kind := range []string{"js.package.locked", "js.dependency.locked", "js.source", "js.import.static"} {
		if !found[kind] {
			t.Fatalf("missing %s: %#v", kind, candidate.Facts)
		}
	}
}
func TestCanonicalRequestAndBunRejection(t *testing.T) {
	request := fixtureRequest()
	raw := canonicalRequest(t, request)
	decoded, err := DecodeRequest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Analyze(decoded); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRequest(raw[:len(raw)-1]); err == nil {
		t.Fatal("missing LF accepted")
	}
	for _, unsupported := range []Input{input("input-1", "js.bun-lock-v1", "bun.lock", `{}`), input("input-1", "js.tsconfig", "tsconfig.json", `{}`)} {
		candidate := candidateRequest(unsupported)
		if _, err := Analyze(candidate); FailureReason(err) != "UNSUPPORTED_SCHEMA" {
			t.Fatalf("family=%s error=%v", unsupported.Family, err)
		}
	}
}

func TestEnvelopeBoundsAndIndependentInputIdentity(t *testing.T) {
	request := fixtureRequest()
	request.Target.Features = make([]string, MaxFeatures+1)
	for index := range request.Target.Features {
		request.Target.Features[index] = fmt.Sprintf("f%d", index)
	}
	if _, err := Analyze(request); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("features error=%v", err)
	}
	request = fixtureRequest()
	request.Inputs[1].Handle = request.Inputs[0].Handle
	if _, err := Analyze(request); FailureReason(err) != "DUPLICATE_VALUE" {
		t.Fatalf("handle error=%v", err)
	}
	request = fixtureRequest()
	request.Inputs[3].Path = request.Inputs[2].Path
	if _, err := Analyze(request); FailureReason(err) != "DUPLICATE_VALUE" {
		t.Fatalf("path error=%v", err)
	}
	request = fixtureRequest()
	request.Inputs[2].ContentBase64 = "AB=="
	if _, err := Analyze(request); FailureReason(err) != "MALFORMED_INPUT" {
		t.Fatalf("base64 error=%v", err)
	}
	deep := strings.Repeat(`{"a":`, MaxEnvelopeDepth+1) + `0` + strings.Repeat(`}`, MaxEnvelopeDepth+1)
	if _, err := decodeObjectBounded(deep, MaxEnvelopeDepth, MaxEnvelopeTokens, nil, false); err == nil {
		t.Fatal("deep envelope accepted")
	}
}

func TestPublicBase64AndDependencyBudgets(t *testing.T) {
	overlong := candidateRequest(Input{Handle: "input-1", Family: "js.source", Path: "src/large.js", SHA256: "sha256:" + strings.Repeat("0", 64), ContentBase64: strings.Repeat("A", maxContentBase64Size+4)})
	if _, err := Analyze(overlong); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("unbounded public base64 error=%v", err)
	}
	group := func(prefix string, count int) string {
		var body strings.Builder
		body.WriteByte('{')
		for index := 0; index < count; index++ {
			if index > 0 {
				body.WriteByte(',')
			}
			fmt.Fprintf(&body, `"%s%04d":"1.0.0"`, prefix, index)
		}
		body.WriteByte('}')
		return body.String()
	}
	count := MaxFacts/4 + 1
	body := fmt.Sprintf(`{"name":"root","version":"1.0.0","dependencies":%s,"devDependencies":%s,"peerDependencies":%s,"optionalDependencies":%s}`, group("a", count), group("b", count), group("c", count), group("d", count))
	budget, prospective := 0, 0
	if _, err := parsePackage(nil, sourceFile{Input: Input{Handle: "input-1"}, bytes: body}, &[]Fact{}, &prospective, &budget, false); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("dependency global budget error=%v", err)
	}
	budget = 0
	if _, err := parsePackage(nil, sourceFile{Input: Input{Handle: "input-1"}, bytes: `{"name":"root","version":"1.0.0","dependencies":{"a":"1.0.0"},"devDependencies":{"a":"1.0.0"}}`}, &[]Fact{}, &prospective, &budget, false); FailureReason(err) != "DUPLICATE_VALUE" {
		t.Fatalf("manifest cross-group duplicate error=%v", err)
	}
	if _, err := importsOfLimit(strings.Repeat(`import "module";`, MaxFacts+1), MaxFacts); err != errImportBudget {
		t.Fatalf("source global budget error=%v", err)
	}
}

func TestPublicBoundaryAtAndOver(t *testing.T) {
	if _, err := Analyze(candidateRequest(input("input-1", "js.source", "src/under.js", strings.Repeat("x", MaxSourceBytes-1)))); err != nil {
		t.Fatalf("decoded under-bound error=%v", err)
	}
	features := make([]string, MaxFeatures)
	for index := range features {
		features[index] = fmt.Sprintf("f%03d", index)
	}
	featureAt := fixtureRequest()
	featureAt.Target.Features = features
	candidate, err := Analyze(featureAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodeCandidate(candidate); err != nil {
		t.Fatalf("feature at-bound output error=%v", err)
	}
	inputAt := make([]Input, MaxInputs)
	for index := range inputAt {
		inputAt[index] = input(fmt.Sprintf("h%03d", index), "js.source", fmt.Sprintf("src/f%03d.js", index), "")
	}
	if _, err := Analyze(candidateRequest(inputAt...)); err != nil {
		t.Fatalf("input at-bound error=%v", err)
	}
	if _, err := Analyze(candidateRequest(inputAt[:MaxInputs-1]...)); err != nil {
		t.Fatalf("input under-bound error=%v", err)
	}
	inputOver := append(inputAt, input("h999", "js.source", "src/f999.js", ""))
	if _, err := Analyze(candidateRequest(inputOver...)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("input over-bound error=%v", err)
	}
	contentAt := candidateRequest(input("input-1", "js.source", "src/full.js", strings.Repeat("x", MaxSourceBytes)))
	if _, err := Analyze(contentAt); err != nil {
		t.Fatalf("decoded at-bound error=%v", err)
	}
	contentOver := candidateRequest(input("input-1", "js.source", "src/over.js", strings.Repeat("x", MaxSourceBytes+1)))
	if _, err := Analyze(contentOver); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("decoded over-bound error=%v", err)
	}
	aggregateAt := candidateRequest(input("input-1", "js.source", "src/a.js", strings.Repeat("x", MaxSourceBytes/2)), input("input-2", "js.source", "src/b.js", strings.Repeat("x", MaxSourceBytes-MaxSourceBytes/2)))
	if _, err := Analyze(aggregateAt); err != nil {
		t.Fatalf("aggregate at-bound error=%v", err)
	}
	aggregateOver := candidateRequest(input("input-1", "js.source", "src/a.js", strings.Repeat("x", MaxSourceBytes/2)), input("input-2", "js.source", "src/b.js", strings.Repeat("x", MaxSourceBytes-MaxSourceBytes/2+1)))
	if _, err := Analyze(aggregateOver); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("aggregate over-bound error=%v", err)
	}
	importsAt, err := importsOfLimit(strings.Repeat(`import "module";`, MaxFacts), MaxFacts)
	if err != nil || len(importsAt) != MaxFacts {
		t.Fatalf("imports at-bound imports=%d err=%v", len(importsAt), err)
	}
	importsOver := candidateRequest(input("input-1", "js.source", "src/imports.js", strings.Repeat(`import "module";`, MaxFacts)))
	if _, err := Analyze(importsOver); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("facts over-bound error=%v", err)
	}
}

func TestSourceOnlyCandidateAndHalfPackageReject(t *testing.T) {
	sourceOnly := candidateRequest(input("input-1", "js.source", "src/only.ts", `import "node:test";`))
	candidate, err := Analyze(sourceOnly)
	if err != nil || len(candidate.Facts) != 2 {
		t.Fatalf("candidate=%#v err=%v", candidate, err)
	}
	halfPackage := candidateRequest(input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"pkg":"1.0.0"}}`))
	if _, err := Analyze(halfPackage); FailureReason(err) != "EXACT_BINDING_UNAVAILABLE" {
		t.Fatalf("half package error=%v", err)
	}
	workspaceOnly := candidateRequest(input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","workspaces":["packages/a"]}`))
	candidate, err = Analyze(workspaceOnly)
	if err != nil || len(candidate.Facts) != 1 || candidate.Facts[0].Kind != "js.workspace" {
		t.Fatalf("workspace candidate=%#v err=%v", candidate, err)
	}
}

func TestDependencyGroupAndWorkspaceCorrelation(t *testing.T) {
	declared := manifestInfo{}
	declared.dependencyGroups[0] = map[string]string{"a": "1.0.0"}
	declared.dependencyGroups[1] = map[string]string{"b": "1.0.0"}
	transposed := map[string]any{"name": "root", "version": "1.0.0", "dependencies": map[string]any{"b": "1.0.0"}, "devDependencies": map[string]any{"a": "1.0.0"}}
	if FailureReason(validateManifestLockEntry(transposed, manifestInfo{name: "root", version: "1.0.0", dependencyGroups: declared.dependencyGroups})) != "CONFLICTING_VALUE" {
		t.Fatal("transposed root groups accepted")
	}
	if !npmPathFor("packages/a", map[string]struct{}{"packages/a": {}}) {
		t.Fatal("declared workspace instance rejected")
	}
	if !npmPathFor("packages/a/nested/node_modules/b", map[string]struct{}{"packages/a": {}, "packages/a/nested": {}}) {
		t.Fatal("longest declared workspace prefix rejected")
	}
}

func TestLockMetadataAndDirectoryWitnesses(t *testing.T) {
	for _, vector := range []struct {
		name, version, resolved string
		want                    bool
	}{
		{"react", "19.2.7", "https://registry.npmjs.org/react/-/react-19.2.7.tgz", true},
		{"@base-ui/react", "1.6.0", "https://registry.npmjs.org/@base-ui/react/-/react-1.6.0.tgz", true},
		{"react", "19.2.7", "https://registry.npmjs.org/react/-/react.tgz", false},
		{"@base-ui/react", "1.6.0", "https://registry.npmjs.org/@base-ui/react/-/base-ui/react-1.6.0.tgz", false},
		{"react", "19.2.7", "https://user@example.test/react/-/react-19.2.7.tgz", false},
		{"react", "19.2.7", "https://registry.npmjs.org/react/-/react-19.2.7.tgz?x=1", false},
	} {
		if got := npmResolved(vector.resolved, vector.name, vector.version); got != vector.want {
			t.Fatalf("npm resolved=%s got=%t", vector.resolved, got)
		}
	}
	actualIntegrity := "sha512-HNe9WslTbXmFK8o8cmwgAeJFSBvt1bPdHCVKtaaV+WlAN36mpT4hcRpwbf3fY56ar2oIXzsBpOAiIRHAdY0OlQ=="
	if pkg, err := parseLockedPackage("node_modules/react", map[string]any{"version": "19.2.7", "resolved": "https://registry.npmjs.org/react/-/react-19.2.7.tgz", "integrity": actualIntegrity, "license": "MIT", "engines": map[string]any{"node": ">=0.10.0"}}, false); err != nil || pkg.name != "react" {
		t.Fatalf("real npm v3 entry=%#v err=%v", pkg, err)
	}
	declared := manifestInfo{name: "root", version: "1.0.0"}
	if FailureReason(validateManifestLockEntry(map[string]any{"name": "root", "version": "1.0.0", "resolved": false}, declared)) != "MALFORMED_INPUT" {
		t.Fatal("malformed root resolved accepted")
	}
	for _, metadata := range []string{`"resolved":"garbage","integrity":"garbage"`, `"resolved":"https://registry.npmjs.org/root/-/root-1.0.0.tgz"`, `"integrity":"sha512-` + base64.StdEncoding.EncodeToString(make([]byte, 64)) + `"`} {
		request := candidateRequest(input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0"}`), input("input-2", "js.npm-lock-v3", "package-lock.json", `{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0",`+metadata+`}}}`))
		if _, err := Analyze(request); FailureReason(err) != "MALFORMED_INPUT" {
			t.Fatalf("root metadata=%s error=%v", metadata, err)
		}
	}
	if _, err := parseLockedPackage("packages/a", map[string]any{"name": "a", "version": "1.0.0", "integrity": false}, true); FailureReason(err) != "MALFORMED_INPUT" {
		t.Fatalf("malformed workspace integrity error=%v", err)
	}
	if _, err := parseLockedPackage("node_modules/a", map[string]any{"name": "a", "version": "1.0.0", "resolved": "https://registry.npmjs.org/a/-/a-1.0.0.tgz", "integrity": "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64)), "dependencies": map[string]any{"b": "1.0.0"}, "devDependencies": map[string]any{"b": "1.0.0"}}, false); FailureReason(err) != "DUPLICATE_VALUE" {
		t.Fatalf("locked cross-group duplicate error=%v", err)
	}
	misrooted := candidateRequest(
		input("input-1", "js.package", "apps/a/package.json", `{"name":"root","version":"1.0.0"}`),
		input("input-2", "js.npm-lock-v3", "apps/b/package-lock.json", `{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0"}}}`),
	)
	if _, err := Analyze(misrooted); FailureReason(err) != "EXACT_BINDING_UNAVAILABLE" {
		t.Fatalf("misrooted lock error=%v", err)
	}
	missingWitness := candidateRequest(
		input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","workspaces":["packages/a"]}`),
		input("input-2", "js.npm-lock-v3", "package-lock.json", `{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0"},"packages/a":{"name":"a","version":"1.0.0"}}}`),
	)
	if _, err := Analyze(missingWitness); FailureReason(err) != "EXACT_BINDING_UNAVAILABLE" {
		t.Fatalf("missing workspace witness error=%v", err)
	}
}

func TestClosedPackageLockAndExactReachableBindings(t *testing.T) {
	for field, value := range map[string]string{
		"private":        "true",
		"type":           `"module"`,
		"description":    `"open metadata"`,
		"scripts":        `{}`,
		"engines":        `{}`,
		"packageManager": `"npm@10.9.8"`,
		"license":        `"AGPL-3.0-only"`,
	} {
		request := candidateRequest(input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","`+field+`":`+value+`}`))
		if _, err := Analyze(request); FailureReason(err) != "UNKNOWN_FIELD" {
			t.Fatalf("package field=%s err=%v", field, err)
		}
	}
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	for _, field := range []string{"hasInstallScript", "bin", "bundleDependencies", "cpu", "deprecated", "engines", "funding", "libc", "license", "os", "peerDependenciesMeta", "workspaces"} {
		lock := fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0"},"node_modules/v":{"name":"v","version":"1.0.0","resolved":"https://registry.npmjs.org/v/-/v-1.0.0.tgz","integrity":"%s","%s":true}}}`, integrity, field)
		request := candidateRequest(input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0"}`), input("input-2", "js.npm-lock-v3", "package-lock.json", lock))
		if _, err := Analyze(request); FailureReason(err) != "UNKNOWN_FIELD" {
			t.Fatalf("lock field=%s err=%v", field, err)
		}
	}
	for _, request := range []Request{
		candidateRequest(input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"v":"^1.0.0"}}`)),
		candidateRequest(
			input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"v":"1.0.0"}}`),
			input("input-2", "js.npm-lock-v3", "package-lock.json", fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0","dependencies":{"v":"1.0.0"}},"node_modules/v":{"name":"v","version":"1.0.0","resolved":"https://registry.npmjs.org/v/-/v-1.0.0.tgz","integrity":"%s","dependencies":{"child":"~1.0.0"}}}}`, integrity))),
	} {
		if _, err := Analyze(request); FailureReason(err) != "DYNAMIC_INPUT" {
			t.Fatalf("range err=%v", err)
		}
	}
	nearest := candidateRequest(
		input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","workspaces":["packages/a"]}`),
		input("input-2", "js.package", "packages/a/package.json", `{"name":"a","version":"1.0.0","dependencies":{"b":"1.0.0"}}`),
		input("input-3", "js.npm-lock-v3", "package-lock.json", fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0"},"packages/a":{"name":"a","version":"1.0.0","dependencies":{"b":"1.0.0"}},"node_modules/b":{"name":"b","version":"1.0.0","resolved":"https://registry.npmjs.org/b/-/b-1.0.0.tgz","integrity":"%s"},"packages/a/node_modules/b":{"name":"b","version":"1.0.0","resolved":"https://registry.npmjs.org/b/-/b-1.0.0.tgz","integrity":"%s"}}}`, integrity, integrity)),
	)
	if _, err := Analyze(nearest); err != nil {
		t.Fatalf("nearest binding err=%v", err)
	}
	nearMismatch := candidateRequest(
		input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","workspaces":["packages/a"]}`),
		input("input-2", "js.package", "packages/a/package.json", `{"name":"a","version":"1.0.0","dependencies":{"b":"1.0.0"}}`),
		input("input-3", "js.npm-lock-v3", "package-lock.json", fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0"},"packages/a":{"name":"a","version":"1.0.0","dependencies":{"b":"1.0.0"}},"node_modules/b":{"name":"b","version":"1.0.0","resolved":"https://registry.npmjs.org/b/-/b-1.0.0.tgz","integrity":"%s"},"packages/a/node_modules/b":{"name":"b","version":"2.0.0","resolved":"https://registry.npmjs.org/b/-/b-2.0.0.tgz","integrity":"%s"}}}`, integrity, integrity)),
	)
	if _, err := Analyze(nearMismatch); FailureReason(err) != "EXACT_BINDING_UNAVAILABLE" {
		t.Fatalf("near mismatch err=%v", err)
	}
	if _, err := Analyze(candidateRequest(input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","workspaces":["root"]}`))); FailureReason(err) != "INVALID_PATH" {
		t.Fatalf("root workspace collision err=%v", err)
	}
	if _, err := Analyze(candidateRequest(input("input-1", "js.source", "src/main.js", `import "same"; import "same";`))); FailureReason(err) != "DUPLICATE_VALUE" {
		t.Fatalf("duplicate fact err=%v", err)
	}
}

func TestCanonicalErrorGolden(t *testing.T) {
	encoded, err := EncodeCandidate(Rejection("unknown", "NONCANONICAL_REQUEST"))
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}`
	if string(encoded) != want {
		t.Fatalf("got=%s want=%s", encoded, want)
	}
}

func TestPostEnvelopeRejectionBindsCanonicalRequest(t *testing.T) {
	var results []string
	for _, body := range []string{"{}", "{\"lock\":\"changed\"}"} {
		input := input("input-1", "js.bun-lock-v1", "bun.lock", body)
		request, err := DecodeRequest(canonicalRequest(t, candidateRequest(input)))
		if err != nil {
			t.Fatal(err)
		}
		candidate, err := EncodeCandidate(RejectionForRequest(request, "UNSUPPORTED_SCHEMA"))
		want := `{"profile":"` + Profile + `","family":"` + Family + `","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"` + input.Handle + `","family":"` + input.Family + `","path":"` + input.Path + `","sha256":"` + input.SHA256 + `"}],"reason":"UNSUPPORTED_SCHEMA"}`
		if err != nil || string(candidate) != want || strings.Contains(string(candidate), "request_sha256") {
			t.Fatalf("post-envelope=%q want=%q error=%v", candidate, want, err)
		}
		results = append(results, string(candidate))
	}
	if results[0] == results[1] {
		t.Fatal("same request ID with changed input replayed")
	}
	first := input("input-1", "js.bun-lock-v1", "bun.lock", "{}")
	second := input("input-1", "js.tsconfig", "tsconfig.json", "{}")
	firstRaw := canonicalRequest(t, candidateRequest(first))
	secondRaw := canonicalRequest(t, candidateRequest(second))
	one, err := DecodeRequest(firstRaw)
	if err != nil {
		t.Fatal(err)
	}
	two, err := DecodeRequest(secondRaw)
	if err != nil {
		t.Fatal(err)
	}
	firstWire, err := EncodeCandidate(RejectionForRequest(one, "UNSUPPORTED_SCHEMA"))
	if err != nil {
		t.Fatal(err)
	}
	secondWire, err := EncodeCandidate(RejectionForRequest(two, "UNSUPPORTED_SCHEMA"))
	firstBinding, secondBinding := sha256.Sum256(firstRaw), sha256.Sum256(secondRaw)
	if err != nil || firstBinding == secondBinding || bytes.Equal(firstWire, secondWire) || !bytes.Contains(firstWire, []byte(`"family":"js.bun-lock-v1","path":"bun.lock"`)) || !bytes.Contains(secondWire, []byte(`"family":"js.tsconfig","path":"tsconfig.json"`)) {
		t.Fatalf("descriptor replay first=%s second=%s err=%v", firstWire, secondWire, err)
	}
	partial := RejectionForRequest(Request{Profile: Profile, Family: Family, RequestID: "request-1"}, "UNSUPPORTED_SCHEMA")
	encoded, err := EncodeCandidate(partial)
	const sentinel = `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"UNSUPPORTED_SCHEMA"}`
	if err != nil || string(encoded) != sentinel {
		t.Fatalf("partial=%q want=%q error=%v", encoded, sentinel, err)
	}
}

func TestUnknownProfileRejectionStaysBound(t *testing.T) {
	input := input("input-1", "js.bun-lock-v1", "bun.lock", "{}")
	var results []string
	for _, profile := range []string{"corvint-analyzer-candidate/future", "corvint-analyzer-candidate/other"} {
		supplied := candidateRequest(input)
		supplied.Profile = profile
		request, err := DecodeRequest(canonicalRequest(t, supplied))
		if err != nil {
			t.Fatal(err)
		}
		_, err = Analyze(request)
		candidate, encodeErr := EncodeCandidate(RejectionForRequest(request, FailureReason(err)))
		want := `{"profile":"` + profile + `","family":"` + Family + `","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"` + input.Handle + `","family":"` + input.Family + `","path":"` + input.Path + `","sha256":"` + input.SHA256 + `"}],"reason":"UNSUPPORTED_SCHEMA"}`
		if encodeErr != nil || string(candidate) != want {
			t.Fatalf("profile=%s got=%q want=%q error=%v", profile, candidate, want, encodeErr)
		}
		results = append(results, string(candidate))
	}
	if results[0] == results[1] {
		t.Fatal("differently profiled requests collapsed to one rejection")
	}
}

func TestInvalidEnvelopeRejectionStaysUnbound(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		mutate       func(*Request)
	}{
		{"scope", "INVALID_IDENTIFIER", func(r *Request) { r.ScopeID = "bad scope" }},
		{"feature", "INVALID_IDENTIFIER", func(r *Request) { r.Target.Features = []string{"bad feature"} }},
		{"input-family", "INVALID_IDENTIFIER", func(r *Request) { r.Inputs[0].Family = "bad family" }},
		{"digest", "DIGEST_MISMATCH", func(r *Request) { r.Inputs[0].SHA256 = "sha256:BAD" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			supplied := candidateRequest(input("input-1", "js.bun-lock-v1", "bun.lock", "{}"))
			tc.mutate(&supplied)
			request, err := DecodeRequest(canonicalRequest(t, supplied))
			if err != nil {
				t.Fatal(err)
			}
			_, err = Analyze(request)
			candidate, encodeErr := EncodeCandidate(RejectionForRequest(request, FailureReason(err)))
			want := `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + tc.reason + `"}`
			if encodeErr != nil || string(candidate) != want {
				t.Fatalf("got=%q want=%q error=%v", candidate, want, encodeErr)
			}
		})
	}
}

func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	raw := canonicalRequest(t, candidateRequest(input("input-1", "js.bun-lock-v1", "bun.lock", "{}")))
	outcome := func(frame string) string {
		request, err := DecodeRequest([]byte(frame + "\n"))
		encoded, _ := EncodeCandidate(RejectionForRequest(request, FailureReason(err)))
		return string(encoded)
	}
	body := strings.TrimSuffix(string(raw), "\n")
	base := strings.TrimSuffix(body, "}")
	request, _ := DecodeRequest(raw)
	want, _ := EncodeCandidate(RejectionForRequest(request, "UNKNOWN_FIELD"))
	for _, edited := range []string{
		base + `,"x":[9007199254740993,{"a":"b","c":null}]}`,
		base + `,"target inputs":0}`,
		strings.Replace(body, `"features":[]`, `"features":[],"x":true`, 1),
		strings.Replace(body, `"}]}`, `","x":0,"y":"z"}]}`, 1),
	} {
		if got := outcome(edited); got != string(want) || !strings.Contains(got, `"scope_id"`) {
			t.Fatalf("frame=%s\ngot=%s", edited, got)
		}
	}
	for edited, reason := range map[string]string{
		base + `,"x":1.0}`: "NONCANONICAL_REQUEST", base + `,"y":0,"x":0}`: "NONCANONICAL_REQUEST", base + `,"x":{"b":0,"a":0}}`: "NONCANONICAL_REQUEST",
		strings.Replace(body, `{"profile":`, `{"x":0,"profile":`, 1):  "NONCANONICAL_REQUEST",
		strings.Replace(base, `"sha256:`, `"sha256:X`, 1) + `,"x":0}`: "DIGEST_MISMATCH",
	} {
		if got := outcome(edited); got != `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"`+reason+`"}` {
			t.Fatalf("frame=%s\ngot=%s", edited, got)
		}
	}
}

func TestClosedRejectionReasons(t *testing.T) {
	for _, reason := range []string{"NONCANONICAL_REQUEST", "INVALID_IDENTIFIER", "INVALID_PATH", "DIGEST_MISMATCH", "UNKNOWN_FAMILY", "UNKNOWN_FIELD", "DUPLICATE_VALUE", "CONFLICTING_VALUE", "MALFORMED_INPUT", "UNSUPPORTED_SCHEMA", "EXACT_BINDING_UNAVAILABLE", "AMBIGUOUS_BINDING", "DYNAMIC_INPUT", "CREDENTIAL_INPUT", "LIMIT_EXCEEDED", "OUTPUT_LIMIT", "ANALYZER_FAILURE"} {
		candidate := Rejection("unknown", reason)
		encoded, err := EncodeCandidate(candidate)
		if err != nil || !strings.Contains(string(encoded), `"reason":"`+reason+`"`) {
			t.Fatalf("reason=%s encoded=%q error=%v", reason, encoded, err)
		}
	}
}

func TestEncodeCandidateClosedBeforeOutput(t *testing.T) {
	if _, err := EncodeCandidate(Candidate{Profile: Profile, Family: Family, Status: "CANDIDATE"}); FailureReason(err) != "ANALYZER_FAILURE" {
		t.Fatalf("unsealed candidate error=%v", err)
	}
	sealed, err := Analyze(fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	sealed.Facts[0].Kind = "not-a-fact"
	if _, err := EncodeCandidate(sealed); FailureReason(err) != "ANALYZER_FAILURE" {
		t.Fatalf("mutated candidate error=%v", err)
	}
	if _, err := (Candidate{Profile: Profile, Family: Family, RequestID: "request-1", Status: "CANDIDATE"}).MarshalJSON(); FailureReason(err) != "ANALYZER_FAILURE" {
		t.Fatalf("public forged marshal error=%v", err)
	}
	if _, err := json.Marshal(Candidate{Profile: Profile, Family: Family, RequestID: "request-1", Status: "CANDIDATE"}); err == nil {
		t.Fatal("json marshal accepted public forgery")
	}
	badReason, err := EncodeCandidate(Rejection("request-1", "NOT_A_REASON"))
	if err != nil || !strings.Contains(string(badReason), `"reason":"ANALYZER_FAILURE"`) {
		t.Fatalf("closed rejection=%q error=%v", badReason, err)
	}
	fact := Fact{Kind: "js.import.static", InputHandle: "input-1", RelatedHandle: "-", Subject: "src/main.js", Predicate: "imports", Value: "module", InstanceID: "src/main.js", EvidenceSHA256: "sha256:" + strings.Repeat("0", 64)}
	prospective := MaxOutputBytes - factEncodedSize(fact)
	if err := addFact(&[]Fact{}, fact, &prospective); err != nil || prospective != MaxOutputBytes {
		t.Fatalf("output cap at error=%v prospective=%d", err, prospective)
	}
	prospective = MaxOutputBytes - factEncodedSize(fact) + 1
	if err := addFact(&[]Fact{}, fact, &prospective); FailureReason(err) != "OUTPUT_LIMIT" {
		t.Fatalf("output cap over error=%v", err)
	}
}

func TestRereviewEnvelopeHostiles(t *testing.T) {
	request := fixtureRequest()
	raw := canonicalRequest(t, request)
	unknown := append(append([]byte{}, raw[:len(raw)-2]...), []byte(",\"unknown\":true}\n")...)
	decoded, err := DecodeRequest(unknown)
	if FailureReason(err) != "UNKNOWN_FIELD" || decoded.Family != Family || decoded.RequestID != request.RequestID {
		t.Fatalf("unknown=%#v err=%v", decoded, err)
	}
	tooMany := `{"profile":"` + Profile + `","family":"` + Family + `","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[],"unknown":{` + strings.Repeat(`"x":0,`, 2200) + `"z":0}}` + "\n"
	if _, err := DecodeRequest([]byte(tooMany)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("tokens error=%v", err)
	}
	request.ScopeID = strings.Repeat("a", MaxTextBytes+1)
	if _, err := DecodeRequest(canonicalRequest(t, request)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("scope error=%v", err)
	}
	large := candidateRequest(input("input-1", "js.source", "src/large.js", strings.Repeat("x", 131073)))
	if _, err := Analyze(large); err != nil {
		t.Fatalf("large source error=%v", err)
	}
	contentArray := `{"profile":"` + Profile + `","family":"` + Family + `","request_id":"request-1","inputs":[{"content_base64":[` + strings.TrimSuffix(strings.Repeat(`"x",`, MaxTextBytes+1), ",") + `]}]}` + "\n"
	if _, err := DecodeRequest([]byte(contentArray)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("content array retained before limit: %v", err)
	}
	featureOverflow := `{"profile":"` + Profile + `","family":"` + Family + `","request_id":"request-1","target":{"features":[` + strings.TrimSuffix(strings.Repeat(`"f",`, MaxFeatures+1), ",") + `]}}` + "\n"
	if _, err := DecodeRequest([]byte(featureOverflow)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("feature overflow retained before limit: %v", err)
	}
	inputOverflow := `{"profile":"` + Profile + `","family":"` + Family + `","request_id":"request-1","inputs":[` + strings.TrimSuffix(strings.Repeat(`{},`, MaxInputs+1), ",") + `]}` + "\n"
	if _, err := DecodeRequest([]byte(inputOverflow)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("input overflow retained before limit: %v", err)
	}
	escapedInputs := `{"profile":"` + Profile + `","family":"` + Family + `","request_id":"request-1","inpu\u0074s":[` + strings.TrimSuffix(strings.Repeat(`{},`, MaxInputs+1), ",") + `]}`
	if _, err := scanEnvelope([]byte(escapedInputs)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("escaped inputs preflight=%v", err)
	}
	escapedFeatures := `{"profile":"` + Profile + `","target":{"fea\u0074ures":[` + strings.TrimSuffix(strings.Repeat(`"f",`, MaxFeatures+1), ",") + `]}}`
	if _, err := scanEnvelope([]byte(escapedFeatures)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("escaped features preflight=%v", err)
	}
	escapedContent := `{"profile":"` + Profile + `","inpu\u0074s":[{"content_base6\u0034":"` + strings.Repeat("A", maxContentBase64Size) + `"},{"content_base6\u0034":"AAAA"}]}`
	if _, err := scanEnvelope([]byte(escapedContent)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("escaped decoded preflight=%v", err)
	}
	if _, err := scanEnvelope([]byte(`{"inputs":[],"inpu\u0074s":[]}`)); FailureReason(err) != "MALFORMED_INPUT" {
		t.Fatalf("escaped duplicate=%v", err)
	}
	if _, err := scanEnvelope([]byte(`{"\u0075nknown":true}`)); FailureReason(err) != "UNKNOWN_FIELD" {
		t.Fatalf("escaped unknown=%v", err)
	}
	decodedOverflow := `{"profile":"` + Profile + `","family":"` + Family + `","request_id":"request-1","inputs":[{"content_base64":"` + strings.Repeat("A", maxContentBase64Size) + `"},{"content_base64":"AAAA"}]}` + "\n"
	if _, err := DecodeRequest([]byte(decodedOverflow)); FailureReason(err) != "LIMIT_EXCEEDED" {
		t.Fatalf("decoded aggregate retained before limit: %v", err)
	}
}
func TestInertAndAmbiguousInputRejects(t *testing.T) {
	request := fixtureRequest()
	request.Inputs[0] = input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"vitest":"github:user/repo"}}`)
	if _, err := Analyze(request); FailureReason(err) != "DYNAMIC_INPUT" {
		t.Fatalf("range error=%v", err)
	}
	request = fixtureRequest()
	request.Inputs[1] = input("input-2", "js.npm-lock-v3", "package-lock.json", `{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0","dependencies":{"vitest":"2.0.0"}},"node_modules/vitest":{"name":"vitest","version":"2.0.0","resolved":"https://registry.npmjs.org/vitest/-/vitest-2.0.0.tgz","integrity":"sha512-exact"}}}`)
	if _, err := Analyze(request); FailureReason(err) != "MALFORMED_INPUT" {
		t.Fatalf("integrity error=%v", err)
	}
}
func TestLexerExcludesInertPropertyAndDynamicForms(t *testing.T) {
	for _, source := range []string{
		`const a = "import 'forged'"; obj./*gap*/require("forged");`,
		`const require = value => value; require("forged");`,
		`import("forged");`,
		"`import \"forged\" ${import(\"dynamic\")}`",
	} {
		imports, err := importsOf(source)
		if err != nil || len(imports) != 0 {
			t.Fatalf("source=%q imports=%v err=%v", source, imports, err)
		}
	}
	imports, err := importsOf(`const view = <div>import "forged"</div>; import "later"; require("bare");`)
	if err != nil || fmt.Sprint(imports) != "[bare later]" {
		t.Fatalf("imports=%v err=%v", imports, err)
	}
	if _, err := importsOf(`<div>inert</div>; é`); err != errLexicalInput {
		t.Fatalf("post-JSX validation error=%v", err)
	}
	imports, err = importsOf("// Unicode inert — permitted\nimport /* gap */ type { T } from \"react\";\nimport Q = require(\"node:test\");\nexport { T } from \"vitest\";")
	if err != nil || fmt.Sprint(imports) != "[node:test react vitest]" {
		t.Fatalf("imports=%v err=%v", imports, err)
	}
}

func TestLexerDivisionRegexAndCredentialSpecifiers(t *testing.T) {
	imports, err := importsOf(`import "real"; class C extends /import "forged-class"/.constructor {}; try {} finally {} /import "forged-finally"/.test(x); if (x) {} else {} /import "forged-else"/.test(x); type T = {}; /import "forged-type"/.test(x); export /[/] from "forged-class"/; const view = <div>import "forged-jsx"</div>;`)
	if err != nil || fmt.Sprint(imports) != "[real]" {
		t.Fatalf("imports=%v err=%v", imports, err)
	}
	if _, err := importsOf(`/* unterminated`); err != errLexicalInput {
		t.Fatalf("unterminated=%v", err)
	}
	if _, err := importsOf(`import "https://user@example.test/pkg";`); err == nil {
		t.Fatal("credential URL accepted")
	}
	if _, err := importsOf(`import "data:text/plain,x";`); err == nil {
		t.Fatal("raw URL specifier accepted")
	}
}

func TestLexerModuleScannerSkipsOnlyInertTokenForms(t *testing.T) {
	for _, source := range []string{
		"export `from \"forged-template\"`; import \"real\";",
		`export /from "forged-regex"/; import "real";`,
		`export <div>from "forged-jsx"</div>; import "real";`,
		`if (ready) /[/] from "forged-control"/; import "real";`,
		`const quotient = value / import "forged-division"; import "real";`,
	} {
		imports, err := importsOf(source)
		if err != nil || fmt.Sprint(imports) != "[real]" {
			t.Fatalf("source=%q imports=%v err=%v", source, imports, err)
		}
	}
}

func TestLexerSeparatesDefaultExportExpressionsFromFromClauses(t *testing.T) {
	t.Run("division-asi", func(t *testing.T) {
		request := candidateRequest(input("input-1", "js.source", "src/default.js", "export default 1 / from\n\"standalone\";"))
		candidate, err := Analyze(request)
		if err != nil || candidate.Status != "CANDIDATE" || len(candidate.Facts) != 1 || candidate.Facts[0].Kind != "js.source" {
			t.Fatalf("facts=%v err=%v", candidate.Facts, err)
		}
	})
	t.Run("identifier-asi", func(t *testing.T) {
		imports, err := importsOf("export default from\n\"standalone\";")
		if err != nil || len(imports) != 0 {
			t.Fatalf("imports=%v err=%v", imports, err)
		}
	})
	t.Run("re-export", func(t *testing.T) {
		imports, err := importsOf(`export { value } from "real";`)
		if err != nil || fmt.Sprint(imports) != "[real]" {
			t.Fatalf("imports=%v err=%v", imports, err)
		}
	})
	t.Run("default-regex", func(t *testing.T) {
		request := candidateRequest(input("input-1", "js.source", "src/default.js", `export default /require("standalone")/;`))
		candidate, err := Analyze(request)
		if err != nil || candidate.Status != "CANDIDATE" || len(candidate.Facts) != 1 || candidate.Facts[0].Kind != "js.source" {
			t.Fatalf("facts=%v err=%v", candidate.Facts, err)
		}
	})
}

func TestLexerStatementKeywordsOpenRegexAcrossASI(t *testing.T) {
	for name, source := range map[string]string{
		"break":    "for (;;) {\n  break\n  /require(\"standalone\")/;\n}",
		"continue": "for (;;) {\n  continue\n  /require(\"standalone\")/;\n}",
		"debugger": "debugger\n/require(\"standalone\")/;",
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || len(imports) != 0 {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
	t.Run("switch-default-label", func(t *testing.T) {
		imports, err := importsOf("switch (x) {\ncase 1:\n  break;\ndefault:\n  f(require(\"real\"));\n}")
		if err != nil || fmt.Sprint(imports) != "[real]" {
			t.Fatalf("imports=%v err=%v", imports, err)
		}
	})
	for name, source := range map[string]string{
		"member-default": `const x = obj.default / require("real") / 2;`,
		"member-in":      `const x = obj.in / require("real") / 2;`,
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || fmt.Sprint(imports) != "[real]" {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
}

func TestLexerBlockCommentLineBreakOpensStatement(t *testing.T) {
	for name, tc := range map[string]struct{ source, want string }{
		"newline":     {"const a = 1 /*\n*/ import x from \"./block\"", "[./block]"},
		"crlf":        {"const a = 1 /*\r\n*/ import x from \"./block\"", "[./block]"},
		"single-line": {"const a = 1 /* same line */ import x from \"./block\"", "[]"},
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(tc.source)
			if err != nil || fmt.Sprint(imports) != tc.want {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
	t.Run("unterminated-at-eof", func(t *testing.T) {
		imports, err := importsOf("const a = 1 /*\nimport x from \"./block\"")
		if err != errLexicalInput || imports != nil {
			t.Fatalf("imports=%v err=%v", imports, err)
		}
	})
}

func TestLexerLineCommentEndsAtEveryLineTerminator(t *testing.T) {
	// U+2028 and U+2029 end the comment and then fail closed as non-ASCII code.
	for terminator, want := range map[string]error{"\n": nil, "\r": nil, "\u2028": errLexicalInput, "\u2029": errLexicalInput} {
		imports, err := importsOf("// c é" + terminator + "require(\"a\");")
		if err != want || want == nil && fmt.Sprint(imports) != "[a]" {
			t.Errorf("%q: imports=%v err=%v", terminator, imports, err)
		}
	}
}

func TestLexerControlHeaderRegexCommentsStayInert(t *testing.T) {
	for name, source := range map[string]string{
		"control-spaced":        `if (ready) /require("forged")/;`,
		"control-block-comment": `if /* before */ (ready) /* after */ /require("forged")/;`,
		"control-line-comment":  "if (ready) // after\n/require(\"forged\")/;",
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || len(imports) != 0 {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
}

func TestLexerJSXExpressionTemplateStaysInert(t *testing.T) {
	source := "export default <div>{`</div> from \"forged\"`}</div>;"
	imports, err := importsOf(source)
	if err != nil || len(imports) != 0 {
		t.Fatalf("imports=%v err=%v", imports, err)
	}
}

func TestLexerRequireShadowingUsesTokensAndStaysBounded(t *testing.T) {
	for _, source := range []string{
		"// const require\nrequire(\"real-comment\");",
		`const requires = value; require("real-identifier");`,
	} {
		imports, err := importsOf(source)
		if err != nil || len(imports) != 1 || !strings.HasPrefix(imports[0], "real-") {
			t.Fatalf("source=%q imports=%v err=%v", source, imports, err)
		}
	}
	large := "const require = noop;" + strings.Repeat(`require("inert");`, 50000)
	imports, err := importsOfLimit(large, 0)
	if err != nil || len(imports) != 0 {
		t.Fatalf("suppressed bounded imports=%v err=%v", imports, err)
	}
	request := candidateRequest(input("input-1", "js.source", "src/arrow.js", `(require) => require("forged");`))
	if _, err := Analyze(request); FailureReason(err) != "UNSUPPORTED_SCHEMA" {
		t.Fatalf("unmodeled shadowing err=%v", err)
	}
}

func TestLexerRejectsArrowRequireBindingForms(t *testing.T) {
	for name, source := range map[string]string{
		"arrow-bare-require":          `const f = require => require("forged");`,
		"arrow-multi-require":         `(a, require) => require("forged");`,
		"arrow-default-require":       `(a = value, require = fallback) => require("forged");`,
		"arrow-bare-escaped":          `const f = \u0072equire => require("forged");`,
		"arrow-parenthesized-escaped": `(\u0072equire) => require("forged");`,
	} {
		t.Run(name, func(t *testing.T) {
			request := candidateRequest(input("input-1", "js.source", "src/"+name+".js", source))
			if _, err := Analyze(request); FailureReason(err) != "UNSUPPORTED_SCHEMA" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestLexerModuleAndCatchBindingsOfRequire(t *testing.T) {
	for name, source := range map[string]string{
		"import-default":    "import require from \"y\";\nrequire(\"forged\");",
		"import-named":      "import { require } from \"y\";\nrequire(\"forged\");",
		"import-renamed":    "import { a as require } from \"y\";\nrequire(\"forged\");",
		"import-namespace":  "import x, * as require from \"y\";\nrequire(\"forged\");",
		"import-type":       "import type { require } from \"y\";\nrequire(\"forged\");",
		"import-hoisted":    "require(\"forged\");\nimport require from \"y\";",
		"import-equals":     "import require = require(\"y\");\nrequire(\"forged\");",
		"later-declarator":  "let a = 1, require;\nrequire(\"forged\");",
		"later-initialized": "var a = g(), require = h;\nrequire(\"forged\");",
		"later-commented":   "const a = 1, // c\n  { b: require } = h;\nrequire(\"forged\");",
		"method-parameter":  "({ m(require) { require(\"forged\"); } });",
		"method-annotated":  "class C { m(a, require: R): void { require(\"forged\"); } }",
		"constructor":       "class C { constructor(require) { require(\"forged\"); } }",
		"setter-parameter":  "({ set x(require) { require(\"forged\"); } });",
		"hoisted-var":       "require(\"forged\");\nvar require = f;",
		"function-var":      "function g() { require(\"forged\"); { var require = f; } }",
		"hoisted-function":  "require(\"forged\");\nfunction require() {}",
		"generator":         "function* require() {}\nrequire(\"forged\");",
		"lexical-closure":   "function g() { return require(\"forged\"); }\nconst require = f;",
		"ts-enum":           "enum require { A }\nrequire(\"forged\");",
		"ts-namespace":      "namespace require {}\nrequire(\"forged\");",
	} {
		t.Run(name, func(t *testing.T) {
			request := candidateRequest(input("input-1", "js.source", "src/"+name+".ts", source))
			if _, err := Analyze(request); FailureReason(err) != "UNSUPPORTED_SCHEMA" {
				t.Fatalf("err=%v", err)
			}
		})
	}
	for name, source := range map[string]string{
		"export-const":       "export const require = f;\nrequire(\"forged\");",
		"export-class":       "export class require {}\nrequire(\"forged\");",
		"export-function":    "export function g() { a(); b(); }\nrequire(\"accepted\");",
		"export-parameter":   "export async function f(require) { a(); require(\"forged\"); }",
		"export-initializer": "export const a = 1\nexport const b = require(\"accepted\")",
		"catch-parameter":    "try { a(); } catch (require) { require(\"forged\"); }\ntry {} catch { require(\"accepted\"); }",
		"property-key":       "module.exports = { a, require: \"ts-node/register\" };\nrequire(\"accepted\");",
		"typeof-require":     "if (typeof require === \"function\") require(\"accepted\");",
		"asi-declarations":   "const a = 1\nconst b = [a, 2]\nrequire(\"accepted\")",
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || slices.Contains(imports, "forged") {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
			if strings.Contains(source, `require("accepted")`) && !slices.Contains(imports, "accepted") {
				t.Fatalf("missing accepted import: %v", imports)
			}
		})
	}
}

func TestLexerNonLiteralRequireYieldsNoFact(t *testing.T) {
	for name, source := range map[string]string{
		"identifier":     "require(name);\nrequire(\"accepted\");",
		"template":       "require(`tmpl`);\nrequire(\"accepted\");",
		"concatenation":  "require(a + b);\nrequire(\"accepted\");",
		"literal-prefix": "require(\"forged\" + b);\nrequire(\"accepted\");",
		"empty-call":     "require();\nrequire(\"accepted\");",
		"class-method":   "class A { require(id) { return 1 } }\nrequire(\"accepted\");",
		"object-method":  "const o = { require() {} };\nrequire(\"accepted\");",
		"property-call":  "obj.require(x);\nrequire(\"accepted\");",
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || fmt.Sprint(imports) != "[accepted]" {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
}

func TestLexerArrowParameterPatterns(t *testing.T) {
	for name, source := range map[string]string{
		"zero-parameter":       `const f = () => require("accepted");`,
		"object-pattern":       `test("x", async ({ page }) => { page(require("accepted")); });`,
		"array-pattern":        `const f = ([a, b = 2]) => require("accepted");`,
		"nested-pattern":       `const f = ({ a: { b }, c = [1] }, d) => require("accepted");`,
		"annotated-parameters": `const f = (foreground: string, background: string) => require("accepted");`,
		"annotated-pattern":    `const f = ({ page }: Fixtures = base) => require("accepted");`,
		"rest-parameter":       `const f = (...rest) => require("accepted");`,
		"multiline-pattern":    "test(\"x\", async ({\n  browser,\n}) => require(\"accepted\"));",
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || len(imports) != 1 || imports[0] != "accepted" {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
	for name, source := range map[string]string{
		"pattern-binds-require":   `({ require }) => require("forged");`,
		"pattern-renames-require": `({ r: require }) => require("forged");`,
		"pattern-key-require":     `({ require: r }) => require("forged");`,
		"array-pattern-require":   `([require]) => require("forged");`,
		"nested-pattern-require":  `({ a: [{ b: require }] }) => require("forged");`,
		"rest-binds-require":      `(...require) => require("forged");`,
		"pattern-escaped":         `({ \u0072equire }) => require("forged");`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := importsOf(source); err != errUnsupportedLexical {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestLexerDeclarationPatternsAndTypeArguments(t *testing.T) {
	for name, source := range map[string]string{
		"cjs-destructured-require": `const { readFile } = require("fs");`,
		"object-declaration":       `const { baseURL } = harnessState(); require("fs");`,
		"array-declaration":        `let [first, second = 2] = pair(); require("fs");`,
		"call-site-type-argument":  `const el = document.querySelector<HTMLElement>(sel)!; require("fs");`,
		"type-object-annotation":   "function f(): Promise<{ close: () => Promise<void> }> { return require(\"fs\"); }",
		"comparison-chain":         `if (a < b && c > d) require("fs");`,
	} {
		t.Run(name, func(t *testing.T) {
			imports, err := importsOf(source)
			if err != nil || len(imports) != 1 || imports[0] != "fs" {
				t.Fatalf("imports=%v err=%v", imports, err)
			}
		})
	}
	for name, source := range map[string]string{
		"object-declares-require":        `const { require } = m; require("forged");`,
		"array-declares-require":         `let [require] = m; require("forged");`,
		"renamed-declared-require":       `const { r: require } = m; require("forged");`,
		"later-pattern-require":          `const { x } = obj, { r: require } = z; require("forged");`,
		"later-declarator-require":       `const { x } = obj, require = stub; require("forged");`,
		"inner-block-var-require":        `function f() { { var { x } = obj, require = stub; } require("forged"); }`,
		"unbounded-declaration-tail":     "const { x } = f()\n, require = stub; require(\"forged\");",
		"malformed-pattern-operator":     `({+}) => require("forged");`,
		"rest-not-last":                  `(...rest, next) => require("forged");`,
		"empty-annotation":               `(x:) => require("forged");`,
		"rest-with-default":              `(...rest = fallback) => require("forged");`,
		"mismatched-pattern-closers":     `({a]) => require("forged");`,
		"jsx-swallows-parameter":         `({x = <Foo>y, require = z </Foo>/}) => require("forged");`,
		"jsx-swallows-declarator":        `const { a } = <Foo>y, require = z </Foo>/; require("forged");`,
		"jsx-comment-newline-parameter":  `({a = <Foo>x/*\n*/var require/*\n*/y </Foo>/}) => require("forged");`,
		"jsx-comment-newline-declarator": `const { a } = <Foo>x/*\n*/var require/*\n*/y </Foo>/; require("forged");`,
		"jsx-comment-u2028-parameter":    "({a = <Foo/*\u2028*/var require/*\u2028*//>}) => require(\"forged\");",
		"jsx-comment-u2028-declarator":   "const { a } = <Foo/*\u2028*/var require/*\u2028*//>; require(\"forged\");",
		"jsx-comment-u2029-parameter":    "({a = <Foo/*\u2029*/var require/*\u2029*//>}) => require(\"forged\");",
		"jsx-comment-u2029-declarator":   "const { a } = <Foo/*\u2029*/var require/*\u2029*//>; require(\"forged\");",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := importsOf(source); err != errUnsupportedLexical {
				t.Fatalf("err=%v", err)
			}
		})
	}
	forAwait := `for await (const x of xs) <Foo>require("ignored")</Foo>; const y = /a/; require("real");`
	if imports, err := importsOf(forAwait); err != nil || len(imports) != 1 || imports[0] != "real" {
		t.Fatalf("for-await jsx imports=%v err=%v", imports, err)
	}
	spreadJSX := `const xs = [...<X>require("ignored")</X>]; const r = /a/; require("real");`
	if imports, err := importsOf(spreadJSX); err != nil || len(imports) != 1 || imports[0] != "real" {
		t.Fatalf("spread jsx imports=%v err=%v", imports, err)
	}
	tailJSX := `const { a } = f(<X/>); require("real");`
	if imports, err := importsOf(tailJSX); err != nil || len(imports) != 1 || imports[0] != "real" {
		t.Fatalf("tail jsx imports=%v err=%v", imports, err)
	}
	defaultJSX := `const g = ({ x = <X/> }) => require("real");`
	if imports, err := importsOf(defaultJSX); err != nil || len(imports) != 1 || imports[0] != "real" {
		t.Fatalf("default jsx imports=%v err=%v", imports, err)
	}
}

func TestLexerTemplateNestingBudget(t *testing.T) {
	nested := func(count int) string {
		return "`" + strings.Repeat("${`", count) + "0" + strings.Repeat("`}", count) + "`"
	}
	if imports, err := importsOf(nested(MaxDepth/2 - 1)); err != nil || len(imports) != 0 {
		t.Fatalf("at-limit imports=%v err=%v", imports, err)
	}
	if _, err := importsOf(nested(MaxDepth / 2)); err != errLexicalInput {
		t.Fatalf("over-limit err=%v", err)
	}
}

func TestLexerFailsClosedOnForgedLexicalContexts(t *testing.T) {
	for _, source := range []string{
		"const x = `unterminated\\`",
		`const x = <a href="import \"forged\"">; import "later";`,
		"export const x = value\nimport \"forged\";",
		`function f(require) { require("forged"); } import "real";`,
	} {
		imports, err := importsOf(source)
		if strings.Contains(source, "unterminated") || strings.Contains(source, "<a ") {
			if err == nil || len(imports) != 0 {
				t.Fatalf("source=%q imports=%v err=%v", source, imports, err)
			}
			continue
		}
		want := "[real]"
		if strings.Contains(source, "export const") {
			want = "[forged]"
		}
		if fmt.Sprint(imports) != want || err != nil {
			t.Fatalf("source=%q imports=%v err=%v", source, imports, err)
		}
	}
}

func TestPinnedBeamfallCoreAndWebSourceVectors(t *testing.T) {
	const core = `import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router";
import "./styles/global.css";
import { I18nProvider } from "@/lib/i18n";
import { registerServiceWorker } from "@/lib/pwa";
import { RegisterProvider } from "@/lib/register";
import { SessionProvider } from "@/lib/session";
import { App } from "./App";

const container = document.getElementById("root");
if (!container) throw new Error("missing #root");

// BASE_URL is "/web/" (the cutover mount; vite base) — the router basename
// tracks it so client routing works under whichever mount point it is built for.
const basename = import.meta.env.BASE_URL.replace(/\/$/, "");

createRoot(container).render(
  <StrictMode>
    <BrowserRouter basename={basename}>
      <I18nProvider>
        <RegisterProvider>
          <SessionProvider>
            <App />
          </SessionProvider>
        </RegisterProvider>
      </I18nProvider>
    </BrowserRouter>
  </StrictMode>,
);

// Install the shell-only service worker (ADR-0007 §1). No-op in dev.
registerServiceWorker();
`
	const web = `import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
)
`
	for _, vector := range []struct {
		name, path, body, digest string
		modules                  []string
	}{
		{"core-da38c59", "internal/web/app/src/main.tsx", core, "5fafbe6106348cf503b00f09b553be7b1cdddd2656752df63ec5bafeeccfee3f", []string{"./App", "./styles/global.css", "@/lib/i18n", "@/lib/pwa", "@/lib/register", "@/lib/session", "react", "react-dom/client", "react-router"}},
		{"web-b924fe0", "src/app/main.tsx", web, "ebc6b666f7ed4f05a0d4c6650784d984f5d3938180bb61246b6d9cac8970d77d", []string{"./App.tsx", "react", "react-dom/client", "react-router"}},
	} {
		sum := sha256.Sum256([]byte(vector.body))
		if hex.EncodeToString(sum[:]) != vector.digest {
			t.Fatalf("%s fixture digest changed", vector.name)
		}
		candidate, err := Analyze(candidateRequest(input("source-1", "js.source", vector.path, vector.body)))
		if err != nil {
			t.Fatalf("%s analyze=%v", vector.name, err)
		}
		got := make([]string, 0, len(vector.modules))
		for _, fact := range candidate.Facts {
			if fact.Kind == "js.import.static" {
				got = append(got, fact.Value)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(vector.modules) || len(candidate.Facts) != len(vector.modules)+1 {
			t.Fatalf("%s facts=%#v", vector.name, candidate.Facts)
		}
	}
}

func TestWorkspaceNestedDependencyFacts(t *testing.T) {
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	request := candidateRequest(
		input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","workspaces":["packages/a"]}`),
		input("input-2", "js.package", "packages/a/package.json", `{"name":"@scope/a","version":"1.0.0","dependencies":{"b":"1.0.0"}}`),
		input("input-3", "js.npm-lock-v3", "package-lock.json", fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0"},"packages/a":{"name":"@scope/a","version":"1.0.0","dependencies":{"b":"1.0.0"}},"packages/a/node_modules/b":{"name":"b","version":"1.0.0","resolved":"https://registry.npmjs.org/b/-/b-1.0.0.tgz","integrity":"%s"}}}`, integrity)),
	)
	files, err := validateRequest(request, false)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := newEvidenceContext(request)
	if err != nil {
		t.Fatal(err)
	}
	budget, prospective := 0, candidateBaseSize(request, files)
	facts := make([]Fact, 0)
	info, err := parsePackage(evidence, files[0], &facts, &prospective, &budget, true)
	if err != nil {
		t.Fatal(err)
	}
	witnesses, err := workspaceWitnesses(&files[0], []*sourceFile{&files[1]}, &files[0], info, evidence, &facts, &prospective, &budget)
	if err != nil {
		t.Fatal(err)
	}
	if err := parseNPMLock(evidence, files[2], files[0], info, witnesses, &facts, &prospective, &budget); err != nil {
		t.Fatal(err)
	}
	candidate, err := Analyze(request)
	if err != nil {
		t.Fatal(err)
	}
	var packageFact, edgeFact bool
	for _, fact := range candidate.Facts {
		if fact.Kind == "js.package.locked" && fact.Subject == "b" && fact.InstanceID == "packages/a/node_modules/b" {
			packageFact = true
		}
		if fact.Kind == "js.dependency.locked" && fact.RelatedHandle == "input-2" && fact.Subject == "packages/a" && fact.Value == "packages/a/node_modules/b" && fact.InstanceID == "packages/a" {
			edgeFact = true
		}
	}
	if !packageFact || !edgeFact {
		t.Fatalf("missing workspace facts: %#v", candidate.Facts)
	}
}

// TestDeclaredWorkspaceWithoutLockEntryRejects pins that a lock must correlate
// every declared workspace, whether or not its manifest witness is supplied.
func TestDeclaredWorkspaceWithoutLockEntryRejects(t *testing.T) {
	root := input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","workspaces":["packages/a"]}`)
	lock := input("input-3", "js.npm-lock-v3", "package-lock.json", `{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0"}}}`)
	witness := input("input-2", "js.package", "packages/a/package.json", `{"name":"@scope/a","version":"1.0.0","dependencies":{"b":"1.0.0"}}`)
	for name, request := range map[string]Request{"with-witness": candidateRequest(root, witness, lock), "without-witness": candidateRequest(root, lock)} {
		if _, err := Analyze(request); FailureReason(err) != "EXACT_BINDING_UNAVAILABLE" {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
}

func TestLockOnlyParentCannotBorrowRootWitness(t *testing.T) {
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	request := candidateRequest(
		input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"a":"1.0.0"}}`),
		input("input-2", "js.npm-lock-v3", "package-lock.json", fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0","dependencies":{"a":"1.0.0"}},"node_modules/a":{"name":"a","version":"1.0.0","resolved":"https://registry.npmjs.org/a/-/a-1.0.0.tgz","integrity":"%s","dependencies":{"b":"1.0.0"}},"node_modules/a/node_modules/b":{"name":"b","version":"1.0.0","resolved":"https://registry.npmjs.org/b/-/b-1.0.0.tgz","integrity":"%s"}}}`, integrity, integrity)),
	)
	candidate, err := Analyze(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range candidate.Facts {
		if fact.Kind == "js.dependency.locked" && fact.Subject == "node_modules/a" {
			t.Fatalf("forged external edge=%#v", fact)
		}
	}
}

func TestLockTransitiveBindingMustResolveWithoutManifestWitness(t *testing.T) {
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))
	request := candidateRequest(
		input("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"a":"1.0.0"}}`),
		input("input-2", "js.npm-lock-v3", "package-lock.json", fmt.Sprintf(`{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0","dependencies":{"a":"1.0.0"}},"node_modules/a":{"name":"a","version":"1.0.0","resolved":"https://registry.npmjs.org/a/-/a-1.0.0.tgz","integrity":"%s","dependencies":{"b":"1.0.0"}}}}`, integrity)),
	)
	if _, err := Analyze(request); FailureReason(err) != "EXACT_BINDING_UNAVAILABLE" {
		t.Fatalf("missing locked transitive binding err=%v", err)
	}
}
func BenchmarkAnalyzeCandidate(b *testing.B) {
	request := benchmarkRequest()
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		candidate, err := Analyze(request)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(candidate)
	}
}
func TestPublicOutputBoundary(t *testing.T) {
	for _, size := range []int{MaxOutputBytes - 1, MaxOutputBytes, MaxOutputBytes + 1} {
		candidate := outputCandidateAt(t, size)
		encoded, err := EncodeCandidate(candidate)
		if size <= MaxOutputBytes && (err != nil || len(encoded)+1 != size) {
			t.Fatalf("output size=%d bytes=%d err=%v", size, len(encoded)+1, err)
		}
		if size > MaxOutputBytes && FailureReason(err) != "OUTPUT_LIMIT" {
			t.Fatalf("output size=%d err=%v", size, err)
		}
	}
}

func outputCandidateAt(t *testing.T, size int) Candidate {
	t.Helper()
	template := Fact{Kind: "js.source", InputHandle: "input-1", RelatedHandle: "-", Subject: "src/main.js", Predicate: "classifies", InstanceID: "src/main.js", EvidenceSHA256: "sha256:" + strings.Repeat("0", 64)}
	candidate := Candidate{Profile: Profile, Family: Family, RequestID: "request-1", Status: "CANDIDATE", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, InputEchoes: []InputEcho{{Handle: "input-1", Family: "js.source", Path: "src/main.js", SHA256: "sha256:" + strings.Repeat("0", 64)}}}
	for {
		candidate.Facts = append(candidate.Facts, template)
		encoded, err := marshalCandidate(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if len(encoded)+1+MaxTextBytes*len(candidate.Facts) >= size {
			remaining := size - len(encoded) - 1
			for index := range candidate.Facts {
				width := min(remaining, MaxTextBytes)
				candidate.Facts[index].Value = strings.Repeat("x", width)
				remaining -= width
			}
			if remaining != 0 {
				t.Fatalf("output size=%d remaining=%d", size, remaining)
			}
			return finalizedCandidate(candidate)
		}
	}
}

func TestProductionSourceCeiling(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source path unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(here), "../.."))
	total := 0
	for _, directory := range []string{"internal/analyzerjs", "cmd/corvint-analyzer-js"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			total += len(contents)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if total > 92_045 {
		t.Fatalf("candidate source=%d want <=92045", total)
	}
}

func TestProspectiveOutputAccounting(t *testing.T) {
	request := fixtureRequest()
	files, err := validateRequest(request, false)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := Analyze(request)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	want := candidateBaseSize(request, files)
	for index, fact := range candidate.Facts {
		want += factEncodedSize(fact)
		if index > 0 {
			want++
		}
	}
	if len(encoded)+1 != want {
		t.Fatalf("encoded=%d prospective=%d", len(encoded), want)
	}
	requestWithFeature := fixtureRequest()
	requestWithFeature.Target.Features = []string{"feature-a"}
	filesWithFeature, err := validateRequest(requestWithFeature, false)
	if err != nil {
		t.Fatal(err)
	}
	candidateWithFeature, err := Analyze(requestWithFeature)
	if err != nil {
		t.Fatal(err)
	}
	encodedWithFeature, err := EncodeCandidate(candidateWithFeature)
	if err != nil || len(encodedWithFeature)+1 != candidateBaseSize(requestWithFeature, filesWithFeature)+factListSize(candidateWithFeature.Facts) {
		t.Fatalf("feature prospective output=%d err=%v", len(encodedWithFeature), err)
	}
	candidate.ScopeID = "rootx"
	if _, err := EncodeCandidate(candidate); FailureReason(err) != "ANALYZER_FAILURE" {
		t.Fatalf("mutated prospective error=%v", err)
	}
	candidate, err = Analyze(request)
	if err != nil {
		t.Fatal(err)
	}
	candidate.ScopeID = "toot" // Same encoded length must not evade the seal.
	if _, err := EncodeCandidate(candidate); FailureReason(err) != "ANALYZER_FAILURE" {
		t.Fatalf("same-size candidate mutation error=%v", err)
	}
}

func factListSize(facts []Fact) int {
	size := 0
	for index, fact := range facts {
		size += factEncodedSize(fact)
		if index > 0 {
			size++
		}
	}
	return size
}

func TestEvidenceFrameAndFactMatrix(t *testing.T) {
	request := fixtureRequest()
	candidate, err := Analyze(request)
	if err != nil {
		t.Fatal(err)
	}
	var found Fact
	for _, fact := range candidate.Facts {
		if fact.Kind == "js.dependency.locked" {
			found = fact
			break
		}
	}
	if found.Subject != "root" || found.Predicate != "depends-on" || found.Value != "node_modules/vitest" || found.InstanceID != "root" || found.RelatedHandle != "input-1" {
		t.Fatalf("dependency tuple=%#v", found)
	}
	fields := [...]string{Family, request.RequestID, request.ScopeID, request.CompilationUnitID, targetJSON(request.Target), found.InputHandle, request.Inputs[1].SHA256, found.RelatedHandle, request.Inputs[0].SHA256, found.Kind, found.Subject, found.Predicate, found.Value, found.InstanceID}
	frame := append([]byte("corvint-analyzer-candidate-evidence/experimental"), 0, 0, 0, 14)
	for _, field := range fields {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		frame = append(frame, length[:]...)
		frame = append(frame, field...)
	}
	sum := sha256.Sum256(frame)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if found.EvidenceSHA256 != want {
		t.Fatalf("evidence=%s want=%s", found.EvidenceSHA256, want)
	}
	for index := range fields {
		mutated := fields
		mutated[index] += "x"
		mutationFrame := append([]byte("corvint-analyzer-candidate-evidence/experimental"), 0, 0, 0, 14)
		for _, field := range mutated {
			var length [4]byte
			binary.BigEndian.PutUint32(length[:], uint32(len(field)))
			mutationFrame = append(mutationFrame, length[:]...)
			mutationFrame = append(mutationFrame, field...)
		}
		mutatedSum := sha256.Sum256(mutationFrame)
		if "sha256:"+hex.EncodeToString(mutatedSum[:]) == found.EvidenceSHA256 {
			t.Fatalf("evidence field %d did not bind", index)
		}
	}
}

func TestFactsDeterministic(t *testing.T) {
	first, err := Analyze(fixtureRequest())
	if err != nil {
		t.Fatal(err)
	}
	one, err := EncodeCandidate(first)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 1000; index++ {
		candidate, err := Analyze(fixtureRequest())
		if err != nil {
			t.Fatal(err)
		}
		two, err := EncodeCandidate(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if string(one) != string(two) {
			t.Fatalf("iteration=%d", index)
		}
	}
	_ = fmt.Sprint
}

func TestInputOrderPermutationsRejectDeterministically(t *testing.T) {
	request := fixtureRequest()
	for index := 0; index < len(request.Inputs)-1; index++ {
		permuted := fixtureRequest()
		permuted.Inputs[index], permuted.Inputs[index+1] = permuted.Inputs[index+1], permuted.Inputs[index]
		if _, err := Analyze(permuted); FailureReason(err) != "DUPLICATE_VALUE" {
			t.Fatalf("swap=%d error=%v", index, err)
		}
	}
	if _, err := Analyze(request); err != nil {
		t.Fatal(err)
	}
}
