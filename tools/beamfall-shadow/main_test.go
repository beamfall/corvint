//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestFrozenRequestIsByteExactAndHasOnlyFourFamilies(t *testing.T) {
	inputs := []input{{Handle: "input-001", Family: "go.source", Path: "a.go", SHA256: "sha256:" + strings.Repeat("a", 64), ContentBase64: "YQ=="}}
	raw, err := rawRequest(target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, strings.Repeat("a", 40), "go", inputs)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"shadow-aaaaaaaaaaaa","scope_id":"target-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","compilation_unit_id":"unit-aaaaaaaaaaaa","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-001","family":"go.source","path":"a.go","sha256":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","content_base64":"YQ=="}]}` + "\n"
	if string(raw) != want {
		t.Fatalf("frozen request drift:\nwant %s\n got %s", want, raw)
	}
	for _, family := range []string{"javascript-typescript", "dotnet", "ruby"} {
		if _, err := rawRequest(target{}, strings.Repeat("a", 40), family, nil); err != nil {
			t.Fatalf("family %s: %v", family, err)
		}
	}
	if _, err := rawRequest(target{}, strings.Repeat("a", 40), "sql", nil); err == nil {
		t.Fatal("fifth request family admitted")
	}
	if bytes.Contains(raw, []byte(`"plugin"`)) {
		t.Fatal("selection leaked into frozen request")
	}
}

func TestCanonicalIdentifierDoesNotAdmitSlash(t *testing.T) {
	if candidateIdentifier("plugin/one") || !candidateIdentifier("plugin.one") {
		t.Fatal("identifier grammar drift")
	}
	if !logicalPath("dir/file.go") {
		t.Fatal("logical paths retain slash grammar")
	}
}

func TestManifestRegistryRejectsCrossProductsAndNofollow(t *testing.T) {
	artifact := fixtureArtifact(t)
	for _, tuple := range supportedDogfoodTuples {
		name := fixtureManifest(t, tuple, artifact)
		if _, _, err := readDogfoodManifest(context.Background(), name); err != nil {
			t.Fatalf("valid tuple %s: %v", tuple.Family, err)
		}
	}
	bad := supportedDogfoodTuples[0]
	bad.Language = supportedDogfoodTuples[1].Language
	if _, _, err := readDogfoodManifest(context.Background(), fixtureManifest(t, bad, artifact)); err == nil {
		t.Fatal("cross-product tuple admitted")
	}
	link := filepath.Join(t.TempDir(), "manifest-link")
	if err := os.Symlink(fixtureManifest(t, supportedDogfoodTuples[0], artifact), link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readDogfoodManifest(context.Background(), link); err == nil {
		t.Fatal("manifest symlink admitted")
	}
}

func TestMatrixFixturesAreLiteralDistinctExternalArtifacts(t *testing.T) {
	root := projectRoot(t)
	executor := ""
	if descriptorExecutorAvailable() {
		executor = testFDExecutor(t, t.TempDir())
	}
	for _, tuple := range supportedDogfoodTuples {
		t.Run(tuple.Family, func(t *testing.T) {
			dir := filepath.Join(root, "tools", "beamfall-shadow", "fixtures", "matrix", tuple.Family)
			artifactPath := filepath.Join(dir, "artifact.sh")
			artifact, err := os.ReadFile(artifactPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(artifact, []byte("#!")) || !bytes.Contains(artifact, []byte(tuple.Family)) {
				t.Fatal("missing family-specific executable artifact")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) < 2 {
				t.Fatal("missing literal family fixture")
			}
			var inputPath string
			for _, entry := range entries {
				if !entry.IsDir() && entry.Name() != "artifact.sh" {
					inputPath = filepath.Join(dir, entry.Name())
					break
				}
			}
			literal, err := os.ReadFile(inputPath)
			if err != nil || len(literal) == 0 {
				t.Fatalf("literal input %q: %v", inputPath, err)
			}
			manifestPath := fixtureManifest(t, tuple, artifactPath)
			manifest, rawManifest, err := readDogfoodManifest(context.Background(), manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			guard, err := stageExternalArtifact(context.Background(), manifest, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := guard.Close(); err != nil {
					t.Error(err)
				}
			}()
			if tuple.CandidateFamily == "" {
				if manifest.identity(rawManifest, "sha256:"+strings.Repeat("0", 64)).CandidateFamily != "" {
					t.Fatal("unmapped fixture acquired a frozen-envelope mapping")
				}
				return
			}
			if executor == "" {
				return
			}
			raw, err := rawRequest(target{}, strings.Repeat("a", 40), tuple.CandidateFamily, []input{{
				Handle: "matrix-input", Family: tuple.InputFamily, Path: filepath.Base(inputPath), SHA256: digest(literal), ContentBase64: base64.StdEncoding.EncodeToString(literal),
			}})
			if err != nil {
				t.Fatal(err)
			}
			result := invoke(context.Background(), executor, t.TempDir(), raw, time.Second, "matrix-oracle", guard, manifest.identity(rawManifest, "sha256:"+strings.Repeat("0", 64)))
			if result.Status != "UNEXPECTED_OUTPUT" || result.Reason != "UNEXPECTED_OUTPUT" || result.Cleanup.ExecutionDirectory != "REMOVED" {
				t.Fatalf("literal artifact oracle=%#v", result)
			}
		})
	}
}

func TestBoundReceiptContainsEveryIdentityAndFailure(t *testing.T) {
	targetRepo := fixtureTargetRepository(t)
	manifest := fixtureManifest(t, supportedDogfoodTuples[0], fixtureArtifact(t))
	receiptPath := filepath.Join(t.TempDir(), "private", "receipt.json")
	err := run([]string{"--plugin-manifest", manifest, "--target-repo", targetRepo, "--target-rev", "HEAD", "--input", "a.go", "--input", "b.go", "--receipt", receiptPath}, io.Discard)
	if !descriptorExecutorAvailable() {
		if err == nil {
			t.Fatal("unsupported authority unexpectedly passed")
		}
	} else if err != nil {
		t.Fatal(err)
	}
	got := readReceipt(t, receiptPath)
	if got.Identity.PluginID != "fixture.plugin" || got.Identity.ReleaseID != "fixture.release" || got.Identity.Version != "fixture-v1" || !validDigest(got.Identity.ManifestSHA256) || !validDigest(got.Identity.ArtifactSHA256) || !validDigest(got.Identity.HarnessDigest) || !validDigest(got.RequestSHA256) || !validDigest(got.ReceiptBindingSHA256) {
		t.Fatalf("incomplete receipt binding: %#v", got)
	}
	if descriptorExecutorAvailable() {
		if len(got.Runs) != 7 {
			t.Fatalf("runs=%d", len(got.Runs))
		}
		for _, run := range got.Runs {
			if run.Identity != got.Identity || !validDigest(run.ResponseBindingSHA256) || run.Cleanup.ExecutionDirectory == "" || run.WallNS < 0 {
				t.Fatalf("unbound run: %#v", run)
			}
		}
	}
	badReceipt := filepath.Join(t.TempDir(), "private", "failure.json")
	err = run([]string{"--plugin-manifest", filepath.Join(t.TempDir(), "absent"), "--target-repo", targetRepo, "--target-rev", "HEAD", "--input", "a.go", "--input", "b.go", "--receipt", badReceipt}, io.Discard)
	if err == nil {
		t.Fatal("bad manifest succeeded")
	}
	failed := readReceipt(t, badReceipt)
	if failed.Outcome != "FAILED" || failed.Reason == "" || failed.Cleanup.RunDirectory != "NOT_RUN" || failed.MaxRSSState != "NOT_OBSERVED" || failed.WallNS < 0 {
		t.Fatalf("failure receipt=%#v", failed)
	}
}

func TestDeterministicReplayKeepsBoundIdentities(t *testing.T) {
	targetRepo := fixtureTargetRepository(t)
	manifest := fixtureManifest(t, supportedDogfoodTuples[0], fixtureArtifact(t))
	first := filepath.Join(t.TempDir(), "private", "first.json")
	second := filepath.Join(t.TempDir(), "private", "second.json")
	arguments := []string{"--plugin-manifest", manifest, "--target-repo", targetRepo, "--target-rev", "HEAD", "--input", "a.go", "--input", "b.go"}
	if err := run(append(append([]string{}, arguments...), "--receipt", first), io.Discard); descriptorExecutorAvailable() && err != nil {
		t.Fatal(err)
	}
	if err := run(append(append([]string{}, arguments...), "--receipt", second), io.Discard); descriptorExecutorAvailable() && err != nil {
		t.Fatal(err)
	}
	left, right := readReceipt(t, first), readReceipt(t, second)
	if left.Identity != right.Identity || left.RequestSHA256 != right.RequestSHA256 || left.ReceiptBindingSHA256 != right.ReceiptBindingSHA256 {
		t.Fatalf("replay drift: left=%#v right=%#v", left, right)
	}
}

func TestUnmappedTupleProducesBoundedNotRunReceipt(t *testing.T) {
	targetRepo := fixtureTargetRepository(t)
	tuple := supportedDogfoodTuples[5]
	receiptPath := filepath.Join(t.TempDir(), "private", "receipt.json")
	err := run([]string{"--plugin-manifest", fixtureManifest(t, tuple, fixtureArtifact(t)), "--target-repo", targetRepo, "--target-rev", "HEAD", "--input", "a.go", "--input", "b.go", "--receipt", receiptPath}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "UNMAPPED_FROZEN_ENVELOPE") {
		t.Fatalf("err=%v", err)
	}
	got := readReceipt(t, receiptPath)
	if got.Reason != "UNMAPPED_FROZEN_ENVELOPE" || got.Identity.Family != "sql" || len(got.Runs) != 0 {
		t.Fatalf("receipt=%#v", got)
	}
}

func TestArtifactPassCausalIORatchet(t *testing.T) {
	data := bytes.Repeat([]byte("a"), 2*ioChunkBytes+17)
	current := &countingReader{Reader: bytes.NewReader(data)}
	if _, n, err := copyHashContext(context.Background(), io.Discard, current, int64(len(data))); err != nil || n != int64(len(data)) {
		t.Fatalf("one pass: n=%d err=%v", n, err)
	}
	if current.BytesRead != int64(len(data)) {
		t.Fatalf("artifact pass read %d want %d", current.BytesRead, len(data))
	}
	restoredSource := bytes.NewReader(data)
	restored := &countingReader{Reader: restoredSource}
	if _, _, err := copyHashContext(context.Background(), io.Discard, restored, int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if _, err := restoredSource.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, _, err := copyHashContext(context.Background(), io.Discard, restored, int64(len(data))); err != nil {
		t.Fatal(err)
	}
	if restored.BytesRead != 2*int64(len(data)) {
		t.Fatalf("restored second pass bytes=%d", restored.BytesRead)
	}
	if artifactAllocationRatchetEnabled() {
		allocs := testing.AllocsPerRun(50, func() {
			_, _, _ = copyHashContext(context.Background(), io.Discard, bytes.NewReader(data), int64(len(data)))
		})
		if allocs > 6 {
			t.Fatalf("one-pass artifact allocations=%g want <=6", allocs)
		}
	}
	t.Logf("causal I/O bytes: one-pass=%d restored-second-pass=%d; wall time is diagnostic only", current.BytesRead, restored.BytesRead)
}

func TestArtifactCopyCancellationClosesBlockedDescriptor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	blocked := &blockedReadCloser{closed: make(chan struct{}), started: make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, _, err := copyHashContext(ctx, io.Discard, blocked, 1); done <- err }()
	select {
	case <-blocked.started:
	case <-time.After(time.Second):
		t.Fatal("blocked reader never started")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !blocked.wasClosed() {
			t.Fatalf("err=%v closed=%t", err, blocked.wasClosed())
		}
	case <-time.After(time.Second):
		t.Fatal("canceled staged copy hung")
	}
}

func TestTimeoutReapsDescendantAndReceiptsCleanup(t *testing.T) {
	if !descriptorExecutorAvailable() {
		t.Skip("no descriptor execution")
	}
	stub := fixtureScript(t, "#!/bin/sh\nsleep 30 &\nwait\n")
	m := fixtureManifest(t, supportedDogfoodTuples[0], stub)
	manifest, raw, err := readDogfoodManifest(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := stageExternalArtifact(context.Background(), manifest, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := guard.Close(); err != nil {
			t.Error(err)
		}
	}()
	dir := t.TempDir()
	result := invoke(context.Background(), testFDExecutor(t, dir), dir, []byte("{}\n"), 20*time.Millisecond, "timeout", guard, manifest.identity(raw, "sha256:"+strings.Repeat("0", 64)))
	if result.Status != "TIMEOUT" || result.Cleanup.ProcessGroup != "GROUP_EMPTY_OBSERVED" || result.Cleanup.ExecutionDirectory != "REMOVED" {
		t.Fatalf("timeout=%#v", result)
	}
}

type countingReader struct {
	io.Reader
	BytesRead int64
}

type blockedReadCloser struct {
	closed    chan struct{}
	started   chan struct{}
	once      sync.Once
	startOnce sync.Once
}

func (r *blockedReadCloser) Read([]byte) (int, error) {
	r.startOnce.Do(func() { close(r.started) })
	<-r.closed
	return 0, io.EOF
}
func (r *blockedReadCloser) Close() error { r.once.Do(func() { close(r.closed) }); return nil }
func (r *blockedReadCloser) wasClosed() bool {
	select {
	case <-r.closed:
		return true
	default:
		return false
	}
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, e := r.Reader.Read(p)
	r.BytesRead += int64(n)
	return n, e
}
func readReceipt(t *testing.T, name string) receipt {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var result receipt
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if mode(t, name) != 0o600 {
		t.Fatalf("mode=%#o", mode(t, name))
	}
	return result
}
func fixtureManifest(t *testing.T, tuple manifestTuple, artifact string) string {
	t.Helper()
	raw, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	m := dogfoodManifest{Schema: "beamfall-shadow-dogfood/2", PluginID: "fixture.plugin", ReleaseID: "fixture.release", Version: "fixture-v1", Tuple: tuple, Artifact: artifactSelection{artifact, digest(raw)}}
	encoded, err := jsonv2.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(name, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}
func fixtureArtifact(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	source, binary := filepath.Join(directory, "main.go"), filepath.Join(directory, "analyzer")
	write(t, source, fixtureGoAnalyzer)
	build := exec.Command("go", "build", "-o", binary, source)
	build.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v: %s", err, output)
	}
	return binary
}
func fixtureScript(t *testing.T, body string) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "artifact")
	if err := os.WriteFile(name, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	return name
}
func fixtureTargetRepository(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q")
	write(t, filepath.Join(repo, "a.go"), "package fixture\n")
	write(t, filepath.Join(repo, "b.go"), "package fixture\n")
	gitRun(t, repo, "add", ".")
	gitRun(t, repo, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	return repo
}
func testFDExecutor(t *testing.T, dir string) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	out, err := buildFDExecutor(context.Background(), goTool, dir)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func projectRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if raw, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, raw)
	}
}
func write(t *testing.T, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
func mode(t *testing.T, name string) os.FileMode {
	t.Helper()
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

const fixtureGoAnalyzer = `package main
import ("crypto/sha256"; "encoding/base64"; "encoding/json"; "fmt"; "os"; "strings")
func main() {
 var r map[string]json.RawMessage
 if err:=json.NewDecoder(os.Stdin).Decode(&r);err!=nil{os.Exit(2)}
 var inputs []map[string]json.RawMessage
 if err:=json.Unmarshal(r["inputs"],&inputs);err!=nil{os.Exit(2)}
 echoes:=[]string{}; reason:=""; prior:=""
 for i,x:=range inputs{var handle,sum,encoded string;json.Unmarshal(x["handle"],&handle);json.Unmarshal(x["sha256"],&sum);json.Unmarshal(x["content_base64"],&encoded)
  echoes=append(echoes,fmt.Sprintf("{\"handle\":%s,\"sha256\":%s}",x["handle"],x["sha256"]));if i>0&&prior>=handle{reason="DUPLICATE_VALUE"};prior=handle
  content,err:=base64.StdEncoding.Strict().DecodeString(encoded);actual:="invalid";if err==nil{actual=fmt.Sprintf("sha256:%x",sha256.Sum256(content))};if actual!=sum{reason="DIGEST_MISMATCH"}
 }
 status,tail:="CANDIDATE","\"facts\":[]"
 if reason!=""{status,tail="REJECTED",fmt.Sprintf("\"reason\":%q",reason)}
 // The frame is byte-compared against the harness's struct-ordered canonical encoding, so the
 // request's raw members are echoed in declaration order rather than through a sorted map.
 fmt.Fprintf(os.Stdout,"{\"profile\":\"corvint-analyzer-candidate/experimental\",\"family\":%s,\"request_id\":%s,\"status\":%q,\"scope_id\":%s,\"compilation_unit_id\":%s,\"target\":%s,\"input_echoes\":[%s],%s}\n",r["family"],r["request_id"],status,r["scope_id"],r["compilation_unit_id"],r["target"],strings.Join(echoes,","),tail)
}
`

var _ = syscall.SIGTERM

func TestDescriptorEchoFamilyValidation(t *testing.T) {
	sum := "sha256:" + strings.Repeat("a", 64)
	makeRequest := func(family, inputFamily string) []byte {
		raw, err := json.Marshal(request{Profile: profile, Family: family, RequestID: "req-1", ScopeID: "root", CompilationUnitID: "unit-1",
			Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
			Inputs: []input{{Handle: "input-001", Family: inputFamily, Path: "src/a.ts", SHA256: sum, ContentBase64: ""}}})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	response := func(family string, echoes []inputEcho) []byte {
		raw, err := json.Marshal(candidateSuccess{Profile: profile, Family: family, RequestID: "req-1", Status: "CANDIDATE", ScopeID: "root", CompilationUnitID: "unit-1",
			Target: target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, InputEchoes: echoes, Facts: []candidateFact{}})
		if err != nil {
			t.Fatal(err)
		}
		return append(raw, '\n')
	}
	four := []inputEcho{{Handle: "input-001", Family: "js.source", Path: "src/a.ts", SHA256: sum}}
	two := []inputEcho{{Handle: "input-001", SHA256: sum}}
	if status, ok := decodeCandidateOutput(response("javascript-typescript", four), makeRequest("javascript-typescript", "js.source")); !ok || status != "CANDIDATE" {
		t.Fatalf("javascript four-field descriptor echo rejected: ok=%v status=%q", ok, status)
	}
	if _, ok := decodeCandidateOutput(response("javascript-typescript", two), makeRequest("javascript-typescript", "js.source")); ok {
		t.Fatal("javascript two-field echo accepted")
	}
	if status, ok := decodeCandidateOutput(response("go", two), makeRequest("go", "go.source")); !ok || status != "CANDIDATE" {
		t.Fatalf("frozen two-field echo rejected for go: ok=%v status=%q", ok, status)
	}
	if _, ok := decodeCandidateOutput(response("go", four), makeRequest("go", "go.source")); ok {
		t.Fatal("go four-field echo accepted")
	}
}
