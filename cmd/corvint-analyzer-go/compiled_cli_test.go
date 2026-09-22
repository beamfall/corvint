package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	json "encoding/json/v2"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/analyzergo"
)

const pinnedGoTool = "/opt/homebrew/Cellar/go/1.27.1/libexec/bin/go"

// These are independent, LF-framed production-command goldens. They are not
// calculated through Process in this package: the command must reproduce the
// reviewed bytes exactly before the larger fresh-process corpus is considered.
const compiledCLIFrozenSuccessRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"go.mod","path":"go.mod","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI3LjAK"}]}` + "\n"
const compiledCLIFrozenSuccessResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77"}],"facts":[{"kind":"go.language.declaration","input_handle":"input-1","related_handle":"-","subject":"go","predicate":"declares-language","value":"1.27.0","instance_id":"root","evidence_sha256":"sha256:5b07b4320d23c64b15e5bbf8553f5fd91c95b27900b513bf1d10b1051f754ffe"},{"kind":"go.module","input_handle":"input-1","related_handle":"-","subject":"root","predicate":"declares-module","value":"example.com/module","instance_id":"root","evidence_sha256":"sha256:486362f6f76464651f690778f7901c5a7aa1dd42da15b094b014685fc97e0520"}]}` + "\n"
const compiledCLIFrozenFutureRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"go.mod","path":"go.mod","sha256":"sha256:122b77cd2ca528ef857544db73263a320938b18ebc0d559c58cfc34dad79f5aa","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI4LjAK"}]}` + "\n"
const compiledCLIFrozenFutureResponse = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:122b77cd2ca528ef857544db73263a320938b18ebc0d559c58cfc34dad79f5aa"}],"reason":"UNSUPPORTED_SCHEMA"}` + "\n"

// compiledCLIDistinctCorpus is a checked-in literal corpus. Every row binds
// one exact LF-framed request and its exact response digest; the test below
// reconstructs no expected output through analyzergo.Process.
//
//go:embed testdata/compiled-cli-distinct-v1.jsonl
var compiledCLIDistinctCorpus string

// TestCompiledCLICompleteBoundaryMatrix drives the built production command,
// not run() or Process directly, through every externally bounded request
// family. Internal parser tests own exhaustive reason precedence; this command
// test proves bounded stdin/stdout framing under the pinned executable.
func TestCompiledCLICompleteBoundaryMatrix(t *testing.T) {
	binary := buildCompiledCLI(t)
	valid := cliRequest(t, []analyzergo.Input{cliInput("mod", "go.mod", "go.mod", "module example.com/module\ngo 1.27.0\n")})
	cases := []struct {
		name string
		raw  []byte
	}{
		{"framing-under", valid[:len(valid)-1]},
		{"framing-at", valid},
		{"framing-over", append(append([]byte(nil), valid...), '\n')},
		{"request-under", cliSizedFrame(analyzergo.MaxRequestBytes - 1)},
		{"request-at", cliSizedFrame(analyzergo.MaxRequestBytes)},
		{"request-over", cliSizedFrame(analyzergo.MaxRequestBytes + 1)},
		{"inputs-under", cliSources(t, analyzergo.MaxInputs-1, func(int) string { return "package p\n" })},
		{"inputs-at", cliSources(t, analyzergo.MaxInputs, func(int) string { return "package p\n" })},
		{"inputs-over", cliSources(t, analyzergo.MaxInputs+1, func(int) string { return "package p\n" })},
		{"features-under", cliFeatures(t, 63)},
		{"features-at", cliFeatures(t, 64)},
		{"features-over", cliFeatures(t, 65)},
		{"content-under", cliSources(t, 1, func(int) string { return cliSourceBody(analyzergo.MaxInputBytes - 1) })},
		{"content-at", cliSources(t, 1, func(int) string { return cliSourceBody(analyzergo.MaxInputBytes) })},
		{"content-over", cliSources(t, 1, func(int) string { return cliSourceBody(analyzergo.MaxInputBytes + 1) })},
		{"aggregate-under", cliSources(t, 2, func(i int) string {
			if i == 0 {
				return cliSourceBody(analyzergo.MaxInputBytes / 2)
			}
			return cliSourceBody(analyzergo.MaxInputBytes/2 - 1)
		})},
		{"aggregate-at", cliSources(t, 2, func(int) string { return cliSourceBody(analyzergo.MaxInputBytes / 2) })},
		{"aggregate-over", cliSources(t, 2, func(i int) string {
			if i == 0 {
				return cliSourceBody(analyzergo.MaxInputBytes / 2)
			}
			return cliSourceBody(analyzergo.MaxInputBytes/2 + 1)
		})},
		{"nesting-under", []byte(`{"profile":` + strings.Repeat("[", 6) + "0" + strings.Repeat("]", 6) + "}\n")},
		{"nesting-at", []byte(`{"profile":` + strings.Repeat("[", 7) + "0" + strings.Repeat("]", 7) + "}\n")},
		{"nesting-over", []byte(`{"profile":` + strings.Repeat("[", 8) + "0" + strings.Repeat("]", 8) + "}\n")},
		{"tokens-under", cliUnknownFields(2046)},
		{"tokens-at", cliUnknownFields(2047)},
		{"tokens-over", cliUnknownFields(2048)},
		{"source-fact-edge-under", cliImports(t, analyzergo.MaxFacts-3)},
		{"source-fact-edge-at", cliImports(t, analyzergo.MaxFacts-2)},
		{"source-fact-edge-over", cliImports(t, analyzergo.MaxFacts-1)},
	}
	for _, field := range []struct {
		name string
		set  func(*analyzergo.Request, string)
	}{
		{"request-id", func(r *analyzergo.Request, v string) { r.RequestID = v }},
		{"scope-id", func(r *analyzergo.Request, v string) { r.ScopeID = v }},
		{"compilation-unit-id", func(r *analyzergo.Request, v string) { r.CompilationUnitID = v }},
		{"input-handle", func(r *analyzergo.Request, v string) { r.Inputs[0].Handle = v }},
	} {
		for _, n := range []int{127, 128, 129} {
			r := cliDecodedRequest(t, valid)
			field.set(&r, "i"+strings.Repeat("a", n-1))
			cases = append(cases, struct {
				name string
				raw  []byte
			}{fmt.Sprintf("identifier-%s-%d", field.name, n), cliRequest(t, r.Inputs, cliTarget(r.Target), cliEnvelope(r))})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runCompiledCLI(t, binary, tc.raw)
			assertCLIResponse(t, got)
		})
	}
}

func TestCompiledCLIFrozenGoldens(t *testing.T) {
	binary := buildCompiledCLI(t)
	for _, golden := range []struct {
		name, request, response string
	}{
		{"candidate", compiledCLIFrozenSuccessRequest, compiledCLIFrozenSuccessResponse},
		{"future-go-version", compiledCLIFrozenFutureRequest, compiledCLIFrozenFutureResponse},
	} {
		t.Run(golden.name, func(t *testing.T) {
			if got := runCompiledCLI(t, binary, []byte(golden.request)); !bytes.Equal(got, []byte(golden.response)) {
				t.Fatalf("compiled CLI literal golden mismatch\nwant=%q\ngot=%q", golden.response, got)
			}
		})
	}
}

func TestCompiledCLI1024DistinctFreshProcessCorpus(t *testing.T) {
	binary := buildCompiledCLI(t)
	lines := strings.Split(strings.TrimSuffix(compiledCLIDistinctCorpus, "\n"), "\n")
	if len(lines) != 1024 {
		t.Fatalf("literal fresh-process corpus records=%d", len(lines))
	}
	seenRequests := make(map[string]struct{}, len(lines))
	rawFrames := make([][]byte, len(lines))
	expectedResponses := make([]string, len(lines))
	for lineNumber, line := range lines {
		var expected struct {
			Ordinal        int    `json:"ordinal"`
			RequestSHA256  string `json:"request_sha256"`
			ResponseSHA256 string `json:"response_sha256"`
		}
		if err := json.Unmarshal([]byte(line), &expected); err != nil {
			t.Fatalf("literal corpus line %d: %v", lineNumber+1, err)
		}
		if expected.Ordinal != lineNumber {
			t.Fatalf("literal corpus ordinal line=%d got=%d", lineNumber+1, expected.Ordinal)
		}
		raw := cliDistinctRequest(t, expected.Ordinal)
		requestSum := sha256.Sum256(raw)
		requestDigest := "sha256:" + hex.EncodeToString(requestSum[:])
		if requestDigest != expected.RequestSHA256 {
			t.Fatalf("literal corpus request drift ordinal=%d want=%s got=%s", expected.Ordinal, expected.RequestSHA256, requestDigest)
		}
		if _, duplicate := seenRequests[requestDigest]; duplicate {
			t.Fatalf("literal corpus repeats request ordinal=%d digest=%s", expected.Ordinal, requestDigest)
		}
		seenRequests[requestDigest] = struct{}{}
		rawFrames[lineNumber] = raw
		expectedResponses[lineNumber] = expected.ResponseSHA256
	}
	responseDigests := make([]string, len(lines))
	t.Run("fresh-process-permutations", func(t *testing.T) {
		for shard := 0; shard < 8; shard++ {
			t.Run(fmt.Sprintf("shard-%d", shard), func(t *testing.T) {
				t.Parallel()
				for index := shard; index < len(lines); index += 8 {
					got := runCompiledCLI(t, binary, rawFrames[index]) // one fresh production process per literal request
					assertCLIResponse(t, got)
					responseSum := sha256.Sum256(got)
					responseDigest := "sha256:" + hex.EncodeToString(responseSum[:])
					if responseDigest != expectedResponses[index] {
						t.Fatalf("literal corpus response drift ordinal=%d want=%s got=%s", index, expectedResponses[index], responseDigest)
					}
					responseDigests[index] = responseDigest
				}
			})
		}
	})
	seenResponses := make(map[string]struct{}, len(lines))
	for ordinal, responseDigest := range responseDigests {
		if _, duplicate := seenResponses[responseDigest]; duplicate {
			t.Fatalf("literal corpus repeats response ordinal=%d digest=%s", ordinal, responseDigest)
		}
		seenResponses[responseDigest] = struct{}{}
	}
	if len(seenRequests) != 1024 || len(seenResponses) != 1024 {
		t.Fatalf("literal fresh corpus distinct requests=%d responses=%d", len(seenRequests), len(seenResponses))
	}
}

func cliDistinctRequest(t testing.TB, ordinal int) []byte {
	t.Helper()
	n := fmt.Sprintf("%04d", ordinal)
	body := "module example.com/corvint-corpus-" + n + "\ngo 1.27.0\n"
	r := analyzergo.Request{
		Profile:           analyzergo.Profile,
		Family:            analyzergo.Family,
		RequestID:         "request-" + n,
		ScopeID:           "scope-" + n,
		CompilationUnitID: "unit-" + n,
		Target:            analyzergo.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
	}
	return cliRequest(t, []analyzergo.Input{cliInput("mod-"+n, "go.mod", "go.mod", body)}, cliEnvelope(r))
}

func assertCLIResponse(t testing.TB, raw []byte) {
	t.Helper()
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || bytes.Contains(raw[:len(raw)-1], []byte{'\n'}) {
		t.Fatalf("CLI response is not one LF-framed JSON object: %q", raw)
	}
	var response struct {
		Profile string `json:"profile"`
		Family  string `json:"family"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(raw[:len(raw)-1], &response); err != nil {
		t.Fatalf("CLI response JSON: %v\n%s", err, raw)
	}
	if response.Profile != analyzergo.Profile || (response.Status != "CANDIDATE" && response.Status != "REJECTED") {
		t.Fatalf("CLI response envelope=%+v", response)
	}
	if response.Status == "REJECTED" && response.Reason == "" {
		t.Fatalf("CLI rejection omits reason: %s", raw)
	}
}

type cliEnvelope analyzergo.Request
type cliTarget analyzergo.Target

func cliRequest(t testing.TB, inputs []analyzergo.Input, options ...any) []byte {
	t.Helper()
	r := analyzergo.Request{
		Profile:           analyzergo.Profile,
		Family:            analyzergo.Family,
		RequestID:         "request-1",
		ScopeID:           "root",
		CompilationUnitID: "unit-1",
		Target:            analyzergo.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		Inputs:            inputs,
	}
	for _, option := range options {
		switch v := option.(type) {
		case cliTarget:
			r.Target = analyzergo.Target(v)
		case cliEnvelope:
			r.Profile, r.Family, r.RequestID, r.ScopeID, r.CompilationUnitID = v.Profile, v.Family, v.RequestID, v.ScopeID, v.CompilationUnitID
		default:
			t.Fatalf("unsupported CLI request option %T", option)
		}
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func cliDecodedRequest(t testing.TB, raw []byte) analyzergo.Request {
	t.Helper()
	var r analyzergo.Request
	if err := json.Unmarshal(raw[:len(raw)-1], &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func cliInput(handle, family, path, body string) analyzergo.Input {
	sum := sha256.Sum256([]byte(body))
	return analyzergo.Input{
		Handle:        handle,
		Family:        family,
		Path:          path,
		SHA256:        "sha256:" + hex.EncodeToString(sum[:]),
		ContentBase64: base64.StdEncoding.EncodeToString([]byte(body)),
	}
}

func cliSources(t testing.TB, count int, body func(int) string) []byte {
	t.Helper()
	inputs := make([]analyzergo.Input, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("%04d", i)
		inputs = append(inputs, cliInput("source-"+id, "go.source", "src/"+id+".go", body(i)))
	}
	return cliRequest(t, inputs)
}

func cliFeatures(t testing.TB, count int) []byte {
	t.Helper()
	r := cliDecodedRequest(t, cliRequest(t, []analyzergo.Input{cliInput("mod", "go.mod", "go.mod", "module example.com/module\ngo 1.27.0\n")}))
	r.Target.Features = make([]string, count)
	for i := range r.Target.Features {
		r.Target.Features[i] = fmt.Sprintf("feature-%02d", i)
	}
	return cliRequest(t, r.Inputs, cliTarget(r.Target), cliEnvelope(r))
}

func cliSourceBody(size int) string {
	const prefix = "package p\n//"
	if size < len(prefix)+1 {
		panic("source body too short")
	}
	return prefix + strings.Repeat("x", size-len(prefix)-1) + "\n"
}

func cliImports(t testing.TB, count int) []byte {
	t.Helper()
	var body strings.Builder
	body.WriteString("package p\nimport (\n")
	for i := 0; i < count; i++ {
		body.WriteString("\"example.com/p")
		body.WriteString(fmt.Sprintf("%04d", i))
		body.WriteString("\"\n")
	}
	body.WriteString(")\n")
	return cliSources(t, 1, func(int) string { return body.String() })
}

func cliSizedFrame(size int) []byte {
	raw := make([]byte, size)
	if size >= 2 {
		raw[0], raw[size-2], raw[size-1] = '{', '}', '\n'
	}
	return raw
}

func cliUnknownFields(count int) []byte {
	var raw strings.Builder
	raw.WriteByte('{')
	for i := 0; i < count; i++ {
		if i > 0 {
			raw.WriteByte(',')
		}
		raw.WriteString("\"x")
		raw.WriteString(fmt.Sprintf("%04d", i))
		raw.WriteString("\":0")
	}
	raw.WriteString("}\n")
	return []byte(raw.String())
}

func buildCompiledCLI(t testing.TB) string {
	return buildCompiledCLIWithOptions(t)
}

func buildCompiledCLIWithOptions(t testing.TB, options ...string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "corvint-analyzer-go")
	tool, env := pinnedGoBuildEnvironment(t, root)
	args := []string{"build", "-trimpath", "-buildvcs=false", "-o", path}
	args = append(args, options...)
	args = append(args, ".")
	cmd := exec.Command(tool, args...)
	cmd.Env = env
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return path
}

func pinnedGoBuildEnvironment(t testing.TB, root string) (string, []string) {
	t.Helper()
	tool := pinnedGoTool
	env := []string{
		"HOME=" + filepath.Join(root, "home"),
		"PATH=/usr/bin:/bin",
		"GOROOT=/opt/homebrew/Cellar/go/1.27.1/libexec",
		"GOPATH=" + filepath.Join(root, "gopath"),
		"GOMODCACHE=" + filepath.Join(root, "modcache"),
		"GOENV=off", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "CGO_ENABLED=0",
		"GOCACHE=" + filepath.Join(root, "cache"),
	}
	if runtime.GOOS != "darwin" {
		// The exact Homebrew receipt belongs to the Darwin performance harness.
		// Command conformance itself must also run in Linux CI, where its own
		// provisioned Go executable is the correct host toolchain.
		var err error
		tool, err = exec.LookPath("go")
		if err != nil {
			t.Fatal(err)
		}
		env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off", "CGO_ENABLED=0")
	}
	return tool, env
}

func runCompiledCLI(t testing.TB, binary string, raw []byte) []byte {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Stdin = bytes.NewReader(raw)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run CLI: %v\n%s", err, output)
	}
	return output
}
