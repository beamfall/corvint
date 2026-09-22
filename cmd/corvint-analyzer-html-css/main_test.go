package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const builtCLISuccessGolden = `{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"html-1","family":"html.document","path":"index.html","sha256":"sha256:becbaa6aa181bf8b498078e42a580ea746e013b2b440a4aa9492a7ba8847e41f"}],"facts":[{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"pages/about.html","instance_id":"index.html","evidence_sha256":"sha256:846696b62552e4f2d0aac7cd484367b093adca675c1691a9f0bc0a617150d6ca"}]}` + "\n"
const builtCLIRejectedGolden = `{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"html-1","family":"html.document","path":"index.html","sha256":"sha256:becbaa6aa181bf8b498078e42a580ea746e013b2b440a4aa9492a7ba8847e41f"}],"reason":"MALFORMED_INPUT"}` + "\n"
const builtCLIVersionGolden = `{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","html_input_profile":"html.document/closed-v1","css_input_profile":"css.stylesheet/closed-v1","source":"beamfall-core@da38c59eb30b2121cbac37b912485b30b2e54841","web_source":"beamfall-web@b924fe0ae0d36af08b2a1d50be9383381f809cc0","toolchain":"go1.27.0"}` + "\n"
const builtCLIPermutationRank0Golden = `{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"html-1","family":"html.document","path":"index.html","sha256":"sha256:3f71dadd8da158d48bd912726d9f7a4cfb55508581a28363ecad9767a18e44c8"}],"facts":[{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"a/1.html","instance_id":"index.html","evidence_sha256":"sha256:18c18eb5995a75a8ce823db636c278ee78d6f0c2dcca6c82167453b4aa79eba8"},{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"a/2.html","instance_id":"index.html","evidence_sha256":"sha256:f03793f243cbb53f6c3bc2dd0c997c43de3423f5dab16d2607bfdcc8b824a95b"},{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"a/3.html","instance_id":"index.html","evidence_sha256":"sha256:860a4c9751a2bd142dd24f2606c5356ad969190b0bcab2f1ee617c2490d25e59"},{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"a/4.html","instance_id":"index.html","evidence_sha256":"sha256:a2b9278eca37822e398e4a132f4dd1599321eb7c3ecf3ccfefb349c409a3b37d"},{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"a/5.html","instance_id":"index.html","evidence_sha256":"sha256:acd738bf86af5baa3edcbe2193b9c33ec7c7e6910b85815b89f55234338c3203"},{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"a/6.html","instance_id":"index.html","evidence_sha256":"sha256:9cb640e68b9656857f6f9c55fca1ce600d046983081ce9d3e845d9e9167499bb"},{"kind":"html.link.static","input_handle":"html-1","related_handle":"-","subject":"index.html","predicate":"links","value":"a/7.html","instance_id":"index.html","evidence_sha256":"sha256:bd20da7139431d7811bdf50b8cbc1964d4e907a5aeed8fbec5f8a194c474dcde"}]}` + "\n"

type independentTuple struct {
	kind, inputHandle, relatedHandle, subject, predicate, value, instanceID string
}

var independentPermutationTuples = []independentTuple{
	{"html.link.static", "html-1", "-", "index.html", "links", "a/1.html", "index.html"},
	{"html.link.static", "html-1", "-", "index.html", "links", "a/2.html", "index.html"},
	{"html.link.static", "html-1", "-", "index.html", "links", "a/3.html", "index.html"},
	{"html.link.static", "html-1", "-", "index.html", "links", "a/4.html", "index.html"},
	{"html.link.static", "html-1", "-", "index.html", "links", "a/5.html", "index.html"},
	{"html.link.static", "html-1", "-", "index.html", "links", "a/6.html", "index.html"},
	{"html.link.static", "html-1", "-", "index.html", "links", "a/7.html", "index.html"},
}

type independentFact struct {
	Kind           string `json:"kind"`
	InputHandle    string `json:"input_handle"`
	RelatedHandle  string `json:"related_handle"`
	Subject        string `json:"subject"`
	Predicate      string `json:"predicate"`
	Value          string `json:"value"`
	InstanceID     string `json:"instance_id"`
	EvidenceSHA256 string `json:"evidence_sha256"`
}

type independentInputEcho struct {
	Handle string `json:"handle"`
	Family string `json:"family"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type independentResponse struct {
	Profile           string `json:"profile"`
	Family            string `json:"family"`
	RequestID         string `json:"request_id"`
	Status            string `json:"status"`
	ScopeID           string `json:"scope_id"`
	CompilationUnitID string `json:"compilation_unit_id"`
	Target            struct {
		OS           string   `json:"os"`
		Architecture string   `json:"architecture"`
		ABI          string   `json:"abi"`
		Features     []string `json:"features"`
	} `json:"target"`
	InputEchoes []independentInputEcho `json:"input_echoes"`
	Facts       []independentFact      `json:"facts"`
}

type shortWriter struct {
	limit int
	calls int
	fail  bool
	bytes []byte
}

func (w *shortWriter) Write(data []byte) (int, error) {
	w.calls++
	if w.fail {
		return 0, errors.New("write failure")
	}
	n := w.limit
	if n > len(data) {
		n = len(data)
	}
	w.bytes = append(w.bytes, data[:n]...)
	return n, nil
}

func TestWriteAllHandlesShortAndErrorWrites(t *testing.T) {
	w := &shortWriter{limit: 1}
	if err := writeAll(w, []byte("exact")); err != nil || string(w.bytes) != "exact" || w.calls != 5 {
		t.Fatalf("short write: err=%v bytes=%q calls=%d", err, w.bytes, w.calls)
	}
	if err := writeAll(&shortWriter{fail: true}, []byte("x")); err == nil {
		t.Fatal("missing writer error")
	}
	if err := writeAll(&shortWriter{limit: 0}, []byte("x")); err == nil {
		t.Fatal("missing zero-write error")
	}
}

// ACP-011: run exits nonzero only when the frame is not written in full.
func TestRunReturnsNonzeroWhenFrameIsNotWritten(t *testing.T) {
	stdin := func() *os.File {
		path := filepath.Join(t.TempDir(), "stdin")
		if err := os.WriteFile(path, cliFixtureFrame(), 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { file.Close() })
		return file
	}
	for _, writer := range []*shortWriter{{fail: true}, {limit: 0}} {
		if got := run(nil, stdin(), writer); got != 1 {
			t.Fatalf("writer=%+v status=%d", writer, got)
		}
	}
	full := &shortWriter{limit: 1}
	if got := run(nil, stdin(), full); got != 0 || string(full.bytes) != builtCLISuccessGolden {
		t.Fatalf("full write status=%d bytes=%q", got, full.bytes)
	}
}

func TestAnalyzeStdinPreEnvelopeFailuresUseClosedSentinel(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "closed-stdin-*")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if got, want := string(analyzeStdin(file)), cliSentinel("NONCANONICAL_REQUEST"); got != want {
		t.Fatalf("closed stdin got %q want %q", got, want)
	}
}

func TestBuiltCLILFOverflowAndStableDescriptor(t *testing.T) {
	binary := buildCandidate(t)
	frame := cliFixtureFrame()
	want := []byte(builtCLISuccessGolden)
	if got := runCandidate(t, binary, frame, nil); !bytes.Equal(got, want) {
		t.Fatalf("frozen success golden drifted:\n got %q\nwant %q", got, want)
	}
	for repeat := 0; repeat < 1024; repeat++ {
		if got := runCandidate(t, binary, frame, nil); !bytes.Equal(got, want) {
			t.Fatalf("fresh identical run %d drifted:\n got %q\nwant %q", repeat, got, want)
		}
	}
	if got := runCandidate(t, binary, cliRejectedFrame(), nil); string(got) != builtCLIRejectedGolden {
		t.Fatalf("frozen rejection golden drifted:\n got %q\nwant %q", got, builtCLIRejectedGolden)
	}
	version := exec.Command(binary, "--version")
	if got, err := version.Output(); err != nil || string(got) != builtCLIVersionGolden {
		t.Fatalf("frozen version golden drifted: err=%v got=%q want=%q", err, got, builtCLIVersionGolden)
	}
	if !bytes.HasSuffix(want, []byte("\n")) || bytes.Count(want, []byte("\n")) != 1 {
		t.Fatalf("success is not exactly LF framed: %q", want)
	}
	if got := runCandidate(t, binary, frame[:len(frame)-1], nil); string(got) != cliSentinel("NONCANONICAL_REQUEST") {
		t.Fatalf("missing LF: %q", got)
	}
	if got := runCandidate(t, binary, append(append([]byte(nil), frame...), '\n'), nil); string(got) != cliSentinel("NONCANONICAL_REQUEST") {
		t.Fatalf("extra LF: %q", got)
	}
	if got := runCandidate(t, binary, bytes.Repeat([]byte("x"), 1500001), nil); string(got) != cliSentinel("LIMIT_EXCEEDED") {
		t.Fatalf("oversize: %q", got)
	}
	input, err := os.CreateTemp(t.TempDir(), "stable-stdin-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := input.Write(frame); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if got := runCandidate(t, binary, nil, input); !bytes.Equal(got, want) {
		t.Fatalf("stable inherited descriptor drifted:\n got %q\nwant %q", got, want)
	}
}

func TestBuiltCLIFreshUniquePermutations(t *testing.T) {
	binary := buildCandidate(t)
	frames := make(map[[32]byte]struct{}, 1024)
	type permutationCase struct {
		frame, want []byte
	}
	cases := make([]permutationCase, 1024)
	for rank := 0; rank < 1024; rank++ {
		content := permutationHTML(rank)
		frame := cliFrame(t, content)
		frameDigest := sha256.Sum256(frame)
		if _, exists := frames[frameDigest]; exists {
			t.Fatalf("rank %d did not produce a unique permutation", rank)
		}
		frames[frameDigest] = struct{}{}
		cases[rank] = permutationCase{frame: frame, want: independentPermutationOutput(t, frame, content)}
	}
	t.Run("fresh-process-permutations", func(t *testing.T) {
		for shard := 0; shard < 8; shard++ {
			t.Run(fmt.Sprintf("shard-%d", shard), func(t *testing.T) {
				t.Parallel()
				for rank := shard; rank < len(cases); rank += 8 {
					out := runCandidate(t, binary, cases[rank].frame, nil)
					if replay := runCandidate(t, binary, cases[rank].frame, nil); !bytes.Equal(out, replay) {
						t.Fatalf("fresh rank %d replay drifted:\nfirst=%q\nsecond=%q", rank, out, replay)
					}
					if !bytes.Equal(out, cases[rank].want) {
						t.Fatalf("rank %d independent tuple/digest oracle drifted:\n got %q\nwant %q", rank, out, cases[rank].want)
					}
					if rank == 0 && string(out) != builtCLIPermutationRank0Golden {
						t.Fatalf("rank-0 digest golden drifted:\n got %q\nwant %q", out, builtCLIPermutationRank0Golden)
					}
				}
			})
		}
	})
	if len(frames) != 1024 {
		t.Fatalf("unique requests=%d", len(frames))
	}
	if builtCLIPermutationRank0Golden == "" {
		t.Fatal("missing independent rank-0 digest golden")
	}
}

func TestBuiltCLIMultiInputCanonicalEchoAndOrder(t *testing.T) {
	binary := buildCandidate(t)
	frame := cliMultiInputFrame(false)
	out := runCandidate(t, binary, frame, nil)
	var response independentResponse
	if err := json.Unmarshal(out, &response); err != nil {
		t.Fatal(err)
	}
	reencoded, err := json.Marshal(response)
	if err != nil || !bytes.Equal(out, append(reencoded, '\n')) {
		t.Fatalf("multi-input output is not one LF-canonical envelope: err=%v output=%q", err, out)
	}
	wantEchoes := []independentInputEcho{
		{"css-1", "css.stylesheet", "assets/site.css", "sha256:134b8c6f92b56609f6d949c823c7b6b34adefb3302bd8c006915ee1bcfa2e71a"},
		{"html-1", "html.document", "index.html", "sha256:8b205c754cae41c96603b8e0ecf37202e939b2a2f37578cc916fd015248d1bd8"},
	}
	if response.Status != "CANDIDATE" || !slices.Equal(response.InputEchoes, wantEchoes) {
		t.Fatalf("multi-input echo=%#v status=%q", response.InputEchoes, response.Status)
	}
	if got, want := string(runCandidate(t, binary, cliMultiInputFrame(true), nil)), cliSentinel("DUPLICATE_VALUE"); got != want {
		t.Fatalf("reversed input order got %q want %q", got, want)
	}
}

func independentPermutationOutput(t *testing.T, frame []byte, content string) []byte {
	t.Helper()
	contentSum := sha256.Sum256([]byte(content))
	contentDigest := "sha256:" + hex.EncodeToString(contentSum[:])
	requestSum := sha256.Sum256(frame)
	requestDigest := "sha256:" + hex.EncodeToString(requestSum[:])
	response := independentResponse{Profile: "corvint-analyzer-candidate/experimental", Family: "html-css", RequestID: "request-1", Status: "CANDIDATE", ScopeID: "root", CompilationUnitID: "unit-1"}
	response.Target.OS, response.Target.Architecture, response.Target.ABI, response.Target.Features = "darwin", "arm64", "none", []string{}
	response.InputEchoes = append(response.InputEchoes, independentInputEcho{"html-1", "html.document", "index.html", contentDigest})
	for _, tuple := range independentPermutationTuples {
		response.Facts = append(response.Facts, independentFact{tuple.kind, tuple.inputHandle, tuple.relatedHandle, tuple.subject, tuple.predicate, tuple.value, tuple.instanceID, independentEvidence(requestDigest, contentDigest, tuple)})
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func independentEvidence(requestDigest, contentDigest string, tuple independentTuple) string {
	fields := []string{"html-css", "request-1", "root", "unit-1", `{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`, requestDigest, "html-1", "html.document", "index.html", contentDigest, "-", "-", "-", "-", tuple.kind, tuple.subject, tuple.predicate, tuple.value, tuple.instanceID}
	hash := sha256.New()
	_, _ = hash.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(fields)))
	_, _ = hash.Write(length[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func buildCandidate(t *testing.T) string {
	t.Helper()
	root := repositoryRoot(t)
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-html-css")
	cache := filepath.Join(t.TempDir(), "gocache")
	command := exec.Command("go", "build", "-o", binary, "./cmd/corvint-analyzer-html-css")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+cache)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build candidate: %v\n%s", err, output)
	}
	return binary
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func runCandidate(t *testing.T, binary string, input []byte, descriptor *os.File) []byte {
	t.Helper()
	command := exec.Command(binary)
	if descriptor != nil {
		command.Stdin = descriptor
	} else {
		command.Stdin = bytes.NewReader(input)
	}
	output, err := command.Output()
	if err != nil {
		t.Fatalf("run candidate: %v", err)
	}
	return output
}

func cliFixtureFrame() []byte {
	return []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"html-1","family":"html.document","path":"index.html","sha256":"sha256:becbaa6aa181bf8b498078e42a580ea746e013b2b440a4aa9492a7ba8847e41f","content_base64":"PGh0bWw+PGhlYWQ+PC9oZWFkPjxib2R5PjxhIGhyZWY9InBhZ2VzL2Fib3V0Lmh0bWwiPjwvYT48L2JvZHk+PC9odG1sPg=="}]}` + "\n")
}

func cliRejectedFrame() []byte {
	return []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"html-1","family":"html.document","path":"index.html","sha256":"sha256:becbaa6aa181bf8b498078e42a580ea746e013b2b440a4aa9492a7ba8847e41f","content_base64":"!"}]}` + "\n")
}

func cliMultiInputFrame(reversed bool) []byte {
	css := `@import "theme/base.css";:root{--brand:blue;}.hero{background:url("images/hero.png");}`
	html := `<html><head><link rel="stylesheet" href="assets/site.css"></head><body><a href="pages/about.html"></a><script type="module" src="app/main.js"></script><img src="images/logo.png"></body></html>`
	inputs := []string{
		fmt.Sprintf(`{"handle":"css-1","family":"css.stylesheet","path":"assets/site.css","sha256":"sha256:134b8c6f92b56609f6d949c823c7b6b34adefb3302bd8c006915ee1bcfa2e71a","content_base64":"%s"}`, base64.StdEncoding.EncodeToString([]byte(css))),
		fmt.Sprintf(`{"handle":"html-1","family":"html.document","path":"index.html","sha256":"sha256:8b205c754cae41c96603b8e0ecf37202e939b2a2f37578cc916fd015248d1bd8","content_base64":"%s"}`, base64.StdEncoding.EncodeToString([]byte(html))),
	}
	if reversed {
		inputs[0], inputs[1] = inputs[1], inputs[0]
	}
	return []byte(`{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[` + strings.Join(inputs, ",") + `]}` + "\n")
}

func cliFrame(t *testing.T, content string) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	request := fmt.Sprintf(`{"profile":"corvint-analyzer-candidate/experimental","family":"html-css","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"html-1","family":"html.document","path":"index.html","sha256":"sha256:%s","content_base64":"%s"}]}`,
		hex.EncodeToString(sum[:]),
		base64.StdEncoding.EncodeToString([]byte(content)),
	)
	return append([]byte(request), '\n')
}

func permutationHTML(rank int) string {
	parts := []string{
		`<a href="a/1.html"></a>`,
		`<a href="a/2.html"></a>`,
		`<a href="a/3.html"></a>`,
		`<a href="a/4.html"></a>`,
		`<a href="a/5.html"></a>`,
		`<a href="a/6.html"></a>`,
		`<a href="a/7.html"></a>`,
	}
	for i := len(parts) - 1; i > 0; i-- {
		factorial := 1
		for value := 2; value <= i; value++ {
			factorial *= value
		}
		index := rank / factorial
		rank %= factorial
		parts[i], parts[index] = parts[index], parts[i]
	}
	return "<html><head></head><body>" + strings.Join(parts, "") + "</body></html>"
}

func cliSentinel(reason string) string {
	return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + reason + `"}` + "\n"
}
