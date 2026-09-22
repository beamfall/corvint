package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	json "encoding/json/v2"

	"github.com/Beamfall/corvint/internal/analyzerpython"
)

const candidateVector = `{"profile":"corvint-analyzer-candidate/experimental","family":"python","request_id":"request-vector","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-001","family":"py.source","path":"tooling/vector.py","sha256":"sha256:ec7673423ada93ef80011715786b9292af76bc065071428e6fbeca675c3b3427","content_base64":"aW1wb3J0IGV4YW1wbGUK"}]}` + "\n"

const candidateVectorOutput = `{"profile":"corvint-analyzer-candidate/experimental","family":"python","request_id":"request-vector","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-001","sha256":"sha256:ec7673423ada93ef80011715786b9292af76bc065071428e6fbeca675c3b3427"}],"facts":[{"kind":"python.import.static","input_handle":"input-001","related_handle":"-","subject":"tooling/vector.py","predicate":"imports","value":"example","instance_id":"tooling/vector.py:1:1","evidence_sha256":"sha256:e9f427f52bd9ce4452a27f44772e5c0016946ce1396379151307a95d4c41ba29"},{"kind":"python.source","input_handle":"input-001","related_handle":"-","subject":"tooling/vector.py","predicate":"parses-as","value":"python-3.12.0","instance_id":"tooling/vector.py:1:1","evidence_sha256":"sha256:4566fac073f4f54a0930a493ba05c48a2186d55a29114657d7619f12ec70fc27"}]}` + "\n"

const rejectionVector = `{"profile":"corvint-analyzer-candidate/experimental","family":"python","request_id":"request-vector","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-001","family":"py.source","path":"tooling/vector.py","sha256":"sha256:0000000000000000000000000000000000000000000000000000000000000000","content_base64":"aW1wb3J0IGV4YW1wbGUK"}]}` + "\n"

const rejectionVectorOutput = `{"profile":"corvint-analyzer-candidate/experimental","family":"python","request_id":"request-vector","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-001","sha256":"sha256:0000000000000000000000000000000000000000000000000000000000000000"}],"reason":"DIGEST_MISMATCH"}` + "\n"

const exceptListVector = `{"profile":"corvint-analyzer-candidate/experimental","family":"python","request_id":"request-future","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-001","family":"py.source","path":"tooling/future.py","sha256":"sha256:9c42e26ebcc579a915943724faa0d2c53a4be12b8c691de250abe8bd73619ae8","content_base64":"dHJ5OgogICAgcGFzcwpleGNlcHQgQSwgQjoKICAgIHBhc3MK"}]}` + "\n"

const exceptListVectorOutput = `{"profile":"corvint-analyzer-candidate/experimental","family":"python","request_id":"request-future","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-001","sha256":"sha256:9c42e26ebcc579a915943724faa0d2c53a4be12b8c691de250abe8bd73619ae8"}],"reason":"UNSUPPORTED_SCHEMA"}` + "\n"

func buildCLI(t testing.TB) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "corvint-analyzer-python")
	command := exec.Command("go", "build", "-trimpath", "-o", binary, ".")
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+filepath.Join(t.TempDir(), "gocache"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	return binary
}

func runCLI(t testing.TB, binary string, frame []byte, env []string, directory string) ([]byte, []byte) {
	t.Helper()
	command := exec.Command(binary)
	command.Stdin = bytes.NewReader(frame)
	command.Env = env
	command.Dir = directory
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("CLI failed: %v stderr=%s", err, stderr.Bytes())
	}
	return stdout.Bytes(), stderr.Bytes()
}

func TestBuiltCLIExactByteVectors(t *testing.T) {
	binary := buildCLI(t)
	for _, vector := range []struct{ request, response string }{
		{candidateVector, candidateVectorOutput},
		{rejectionVector, rejectionVectorOutput},
		{exceptListVector, exceptListVectorOutput},
	} {
		got, stderr := runCLI(t, binary, []byte(vector.request), os.Environ(), "")
		if string(stderr) != "" || string(got) != vector.response {
			t.Fatalf("built CLI byte vector drift\nstdout=%q\nstderr=%q\nwant=%q", got, stderr, vector.response)
		}
	}
}

func TestBuiltCLIIdenticalFreshOutputs1001(t *testing.T) {
	binary := buildCLI(t)
	t.Run("fresh-process-permutations", func(t *testing.T) {
		for shard := 0; shard < 8; shard++ {
			t.Run(fmt.Sprintf("shard-%d", shard), func(t *testing.T) {
				t.Parallel()
				for index := shard; index < 1001; index += 8 {
					got, stderr := runCLI(t, binary, []byte(candidateVector), os.Environ(), "")
					if !bytes.Equal(got, []byte(candidateVectorOutput)) || len(stderr) != 0 {
						t.Fatalf("identical fresh output %d stdout=%q stderr=%q", index, got, stderr)
					}
				}
			})
		}
	})
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

type chunkWriter struct{ bytes.Buffer }

func (writer *chunkWriter) Write(value []byte) (int, error) {
	if len(value) > 7 {
		value = value[:7]
	}
	return writer.Buffer.Write(value)
}

func TestRunHandlesShortWrites(t *testing.T) {
	if code := run(nil, strings.NewReader(candidateVector), zeroWriter{}); code != 2 {
		t.Fatalf("zero short write exit=%d", code)
	}
	var writer chunkWriter
	if code := run(nil, strings.NewReader(candidateVector), &writer); code != 0 || writer.String() != candidateVectorOutput {
		t.Fatalf("partial writes exit=%d bytes=%q", code, writer.String())
	}
}

// TestRunFramesReadAndSizeFailures pins ACP-012: a read failure is the
// NONCANONICAL_REQUEST sentinel and an oversize request is LIMIT_EXCEEDED,
// both with exit 0.
func TestRunFramesReadAndSizeFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		input  io.Reader
		reason string
	}{
		{name: "read failure", input: failingReader{}, reason: "NONCANONICAL_REQUEST"},
		{name: "oversize", input: strings.NewReader(strings.Repeat("x", analyzerpython.MaxRequestBytes+1)), reason: "LIMIT_EXCEEDED"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			want := `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"` + test.reason + `"}` + "\n"
			if code := run(nil, test.input, &out); code != 0 || out.String() != want {
				t.Fatalf("code=%d out=%q", code, out.String())
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestRunRejectsUnexpectedArgv(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"--input-file", "request.json"}, strings.NewReader(candidateVector), &out); code != 0 || !strings.Contains(out.String(), `"reason":"NONCANONICAL_REQUEST"`) {
		t.Fatalf("code=%d out=%q", code, out.String())
	}
}

func freshFrame(t testing.TB, index int) []byte {
	t.Helper()
	source := fmt.Sprintf("import module%04d\n", index)
	sum := sha256.Sum256([]byte(source))
	request := analyzerpython.Request{
		Profile: analyzerpython.Profile, Family: analyzerpython.Family, RequestID: "request-permutation",
		ScopeID: "scope-permutation", CompilationUnitID: "unit-permutation",
		Target: analyzerpython.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}},
		Inputs: []analyzerpython.Input{{
			Handle: "input-001", Family: "py.source", Path: "tooling/permutation.py",
			SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(source)),
		}},
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

// freshExpectedResponse is an independent, literal response oracle. It does
// not call the candidate package or marshal a response object.
func freshExpectedResponse(index int) []byte {
	source := fmt.Sprintf("import module%04d\n", index)
	inputDigest := sha256.Sum256([]byte(source))
	inputSHA := "sha256:" + hex.EncodeToString(inputDigest[:])
	requestID := "request-permutation"
	scopeID := "scope-permutation"
	unitID := "unit-permutation"
	path := "tooling/permutation.py"
	module := fmt.Sprintf("module%04d", index)
	target := `{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`
	importEvidence := freshOracleEvidence("python", requestID, scopeID, unitID, target, "input-001", inputSHA, "-", "-", "python.import.static", path, "imports", module, path+":1:1")
	sourceEvidence := freshOracleEvidence("python", requestID, scopeID, unitID, target, "input-001", inputSHA, "-", "-", "python.source", path, "parses-as", "python-3.12.0", path+":1:1")
	return []byte(fmt.Sprintf(`{"profile":"corvint-analyzer-candidate/experimental","family":"python","request_id":"%s","status":"CANDIDATE","scope_id":"%s","compilation_unit_id":"%s","target":%s,"input_echoes":[{"handle":"input-001","sha256":"%s"}],"facts":[{"kind":"python.import.static","input_handle":"input-001","related_handle":"-","subject":"%s","predicate":"imports","value":"%s","instance_id":"%s:1:1","evidence_sha256":"sha256:%s"},{"kind":"python.source","input_handle":"input-001","related_handle":"-","subject":"%s","predicate":"parses-as","value":"python-3.12.0","instance_id":"%s:1:1","evidence_sha256":"sha256:%s"}]}`+"\n", requestID, scopeID, unitID, target, inputSHA, path, module, path, importEvidence, path, path, sourceEvidence))
}

func freshOracleEvidence(fields ...string) string {
	if len(fields) != 14 {
		panic("fresh oracle field count")
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("corvint-analyzer-candidate-evidence/experimental"))
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], uint32(len(fields)))
	_, _ = hash.Write(encoded[:])
	for _, field := range fields {
		binary.BigEndian.PutUint32(encoded[:], uint32(len(field)))
		_, _ = hash.Write(encoded[:])
		_, _ = hash.Write([]byte(field))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func TestBuiltCLIFreshPermutationReplay1024(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-008 independent fresh process full response oracle"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	binary := buildCLI(t)
	requests := make(map[[32]byte]struct{}, 1024)
	responses := make(map[[32]byte]struct{}, 1024)
	requestFrames := make([][]byte, 1024)
	for index := 0; index < 1024; index++ {
		request := freshFrame(t, index)
		requestDigest := sha256.Sum256(request)
		if _, exists := requests[requestDigest]; exists {
			t.Fatalf("duplicate fresh request index=%d digest=%x", index, requestDigest)
		}
		requests[requestDigest] = struct{}{}
		requestFrames[index] = request
		responseDigest := sha256.Sum256(freshExpectedResponse(index))
		if _, exists := responses[responseDigest]; exists {
			t.Fatalf("duplicate semantic response index=%d digest=%x", index, responseDigest)
		}
		responses[responseDigest] = struct{}{}
	}
	t.Run("fresh-process-permutations", func(t *testing.T) {
		for shard := 0; shard < 8; shard++ {
			t.Run(fmt.Sprintf("shard-%d", shard), func(t *testing.T) {
				t.Parallel()
				for index := shard; index < len(requestFrames); index += 8 {
					response, stderr := runCLI(t, binary, requestFrames[index], os.Environ(), "")
					if len(stderr) != 0 || !bytes.Equal(response, freshExpectedResponse(index)) {
						t.Fatalf("permutation %d full-response oracle mismatch stdout=%q", index, response)
					}
					if index%127 == 0 {
						replayed, stderr := runCLI(t, binary, requestFrames[index], os.Environ(), "")
						if len(stderr) != 0 || !bytes.Equal(response, replayed) {
							t.Fatalf("fresh replay %d drifted: %q != %q", index, response, replayed)
						}
					}
				}
			})
		}
	})
	if len(requests) != 1024 || len(responses) != 1024 {
		t.Fatalf("semantic uniqueness requests=%d responses=%d", len(requests), len(responses))
	}
}

func TestIsolationHelper(t *testing.T) {
	if os.Getenv("CORVINT_ANALYZER_NEGATIVE_CONTROL") != "1" {
		return
	}
	// The trap intentionally exits non-zero after recording process use.
	_ = exec.Command("python3").Run()
	for _, root := range []string{mustEnv(t, "CORVINT_SPY_CWD"), mustEnv(t, "HOME"), mustEnv(t, "TMPDIR")} {
		if err := os.WriteFile(filepath.Join(root, "negative-control"), []byte("written"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	connection, err := net.Dial("tcp", mustEnv(t, "CORVINT_SPY_NETWORK"))
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
}

func mustEnv(t testing.TB, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("missing %s", key)
	}
	return value
}

func TestBuiltCLIIsolationAllChannelsAndControls(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-009 no ambient process filesystem network channels"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	if runtime.GOOS == "windows" {
		t.Skip("POSIX process trap is separately covered by cross-build evidence")
	}
	binary := buildCLI(t)
	root := t.TempDir()
	cwd, home, temporary, traps := filepath.Join(root, "cwd"), filepath.Join(root, "home"), filepath.Join(root, "tmp"), filepath.Join(root, "traps")
	for _, directory := range []string{cwd, home, temporary, traps} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	processMarker := filepath.Join(root, "process-marker")
	trap := "#!/bin/sh\nprintf process > \"$CORVINT_SPY_PROCESS\"\nexit 43\n"
	for _, name := range []string{"python", "python3", "git", "sh", "bash"} {
		path := filepath.Join(traps, name)
		if err := os.WriteFile(path, []byte(trap), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	hit := make(chan struct{}, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			_ = connection.Close()
			select {
			case hit <- struct{}{}:
			default:
			}
		}
	}()
	before := snapshots(t, cwd, home, temporary)
	env := append([]string{}, os.Environ()...)
	env = append(env,
		"PATH="+traps, "HOME="+home, "TMPDIR="+temporary, "TMP="+temporary, "TEMP="+temporary,
		"XDG_CACHE_HOME="+home, "XDG_CONFIG_HOME="+home, "XDG_DATA_HOME="+home,
		"CORVINT_SPY_PROCESS="+processMarker, "CORVINT_SPY_NETWORK="+listener.Addr().String(), "CORVINT_SPY_CWD="+cwd,
	)
	stdout, stderr := runCLI(t, binary, []byte(candidateVector), env, cwd)
	if string(stdout) != candidateVectorOutput || len(stderr) != 0 {
		t.Fatalf("isolated CLI bytes stdout=%q stderr=%q", stdout, stderr)
	}
	assertSpyClear(t, processMarker, before, cwd, home, temporary, hit)

	control := exec.Command(os.Args[0], "-test.run=^TestIsolationHelper$")
	control.Dir, control.Env = cwd, append(env, "CORVINT_ANALYZER_NEGATIVE_CONTROL=1")
	if output, err := control.CombinedOutput(); err != nil {
		t.Fatalf("negative control did not complete: %v %s", err, output)
	}
	assertSpyDetected(t, processMarker, before, cwd, home, temporary, hit)
}

func snapshots(t testing.TB, roots ...string) string {
	t.Helper()
	values := make([]string, 0)
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if path == root || info.IsDir() {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(content)
			values = append(values, path+":"+hex.EncodeToString(sum[:]))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(values)
	return strings.Join(values, "\n")
}

func assertSpyClear(t testing.TB, processMarker, before string, roots ...interface{}) {
	t.Helper()
	if _, err := os.Stat(processMarker); !os.IsNotExist(err) {
		t.Fatalf("process spy marker: %v", err)
	}
	cwd, home, temporary, hit := roots[0].(string), roots[1].(string), roots[2].(string), roots[3].(chan struct{})
	if after := snapshots(t, cwd, home, temporary); after != before {
		t.Fatalf("ambient write: %q != %q", after, before)
	}
	select {
	case <-hit:
		t.Fatal("network spy observed candidate")
	case <-time.After(50 * time.Millisecond):
	}
}

func assertSpyDetected(t testing.TB, processMarker, before string, roots ...interface{}) {
	t.Helper()
	if _, err := os.Stat(processMarker); err != nil {
		t.Fatalf("process negative control escaped: %v", err)
	}
	cwd, home, temporary, hit := roots[0].(string), roots[1].(string), roots[2].(string), roots[3].(chan struct{})
	if after := snapshots(t, cwd, home, temporary); after == before {
		t.Fatal("write negative control escaped")
	}
	select {
	case <-hit:
	case <-time.After(time.Second):
		t.Fatal("network negative control escaped")
	}
}

var _ io.Writer = zeroWriter{}
