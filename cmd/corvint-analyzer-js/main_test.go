package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/analyzerjs"
)

type deniedReader struct{}

func (deniedReader) Read([]byte) (int, error) { return 0, fmt.Errorf("stdin denied") }

func TestRunClassifiesStdinFailureAsNoncanonical(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, deniedReader{}, &stdout, &stderr); code != 0 || stderr.Len() != 0 || !bytes.Contains(stdout.Bytes(), []byte(`"reason":"NONCANONICAL_REQUEST"`)) {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

func TestRunRejectsUnexpectedArgv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--input-file", "request.json"}, bytes.NewReader(commandRequest(t)), &stdout, &stderr); code != 0 || stderr.Len() != 0 || !bytes.Contains(stdout.Bytes(), []byte(`"reason":"NONCANONICAL_REQUEST"`)) {
		t.Fatalf("code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

// Decision 0228: a nonzero exit means only that the frame was not written in full.
func TestRunReturnsNonzeroWhenFrameIsNotWritten(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(nil, bytes.NewReader(commandRequest(t)), zeroWriter{}, &stderr); code != 2 {
		t.Fatalf("code=%d", code)
	}
}

func commandInput(handle, family, path, body string) analyzerjs.Input {
	sum := sha256.Sum256([]byte(body))
	return analyzerjs.Input{Handle: handle, Family: family, Path: path, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(body))}
}
func commandRequest(t *testing.T) []byte {
	request := analyzerjs.Request{Profile: analyzerjs.Profile, Family: analyzerjs.Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: analyzerjs.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []analyzerjs.Input{commandInput("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0"}`), commandInput("input-2", "js.npm-lock-v3", "package-lock.json", `{"name":"root","version":"1.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"1.0.0"}}}`)}}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}
func commandSourceRequest(t *testing.T, imports int, width int) []byte {
	var source strings.Builder
	for index := 0; index < imports; index++ {
		source.WriteString(`import "` + strings.Repeat("a", width) + fmt.Sprintf("%04d", index) + `";`)
	}
	request := analyzerjs.Request{Profile: analyzerjs.Profile, Family: analyzerjs.Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: analyzerjs.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []analyzerjs.Input{commandInput("input-1", "js.source", "src/main.js", source.String())}}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}
func commandSingleSourceRequest(t *testing.T, source string) []byte {
	request := analyzerjs.Request{Profile: analyzerjs.Profile, Family: analyzerjs.Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: analyzerjs.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []analyzerjs.Input{commandInput("input-1", "js.source", "src/main.js", source)}}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}
func TestRunEmitsCanonicalCandidate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, bytes.NewReader(commandRequest(t)), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var candidate analyzerjs.Candidate
	if err := json.Unmarshal(bytes.TrimSuffix(stdout.Bytes(), []byte("\n")), &candidate); err != nil || candidate.Status != "CANDIDATE" {
		t.Fatalf("candidate=%q error=%v", stdout.String(), err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"facts":[]`)) {
		t.Fatalf("missing empty facts array: %q", stdout.String())
	}
}
func TestRunEmitsCanonicalRejection(t *testing.T) {
	request := commandRequest(t)
	request = bytes.Replace(request, []byte(`"js.npm-lock-v3"`), []byte(`"js.bun-lock-v1"`), 1)
	var stdout, stderr bytes.Buffer
	if code := run(nil, bytes.NewReader(request), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var candidate analyzerjs.Candidate
	if err := json.Unmarshal(bytes.TrimSuffix(stdout.Bytes(), []byte("\n")), &candidate); err != nil || candidate.Status != "REJECTED" || candidate.Reason != "UNSUPPORTED_SCHEMA" || candidate.ScopeID != "root" || len(candidate.InputEchoes) != 2 {
		t.Fatalf("candidate=%q error=%v", stdout.String(), err)
	}
}

func TestRunPreservesStagedIdentityOnEnvelopeRejection(t *testing.T) {
	request := commandRequest(t)
	request = append(append([]byte{}, request[:len(request)-2]...), []byte(",\"unknown\":1.5}\n")...)
	var stdout, stderr bytes.Buffer
	if code := run(nil, bytes.NewReader(request), &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var candidate analyzerjs.Candidate
	if err := json.Unmarshal(bytes.TrimSuffix(stdout.Bytes(), []byte("\n")), &candidate); err != nil {
		t.Fatal(err)
	}
	if candidate.Profile != analyzerjs.Profile || candidate.Family != "unknown" || candidate.RequestID != "unknown" || candidate.Status != "REJECTED" || candidate.Reason != "NONCANONICAL_REQUEST" || candidate.Target.OS != "" || candidate.Target.Architecture != "" || candidate.Target.ABI != "" || candidate.Target.Features != nil || candidate.InputEchoes != nil {
		t.Fatalf("candidate=%#v", candidate)
	}
}

func TestRunHasNoAmbientProcessOrNetworkDependency(t *testing.T) {
	var baselineOut, baselineErr bytes.Buffer
	if code := run(nil, bytes.NewReader(commandRequest(t)), &baselineOut, &baselineErr); code != 0 || baselineErr.Len() != 0 {
		t.Fatalf("baseline code=%d stderr=%q", code, baselineErr.String())
	}
	temporary := t.TempDir()
	marker := filepath.Join(temporary, "process-spy")
	if err := os.WriteFile(filepath.Join(temporary, "git"), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	connected := make(chan struct{}, 1)
	listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr == nil {
		defer listener.Close()
		go func() {
			connection, acceptErr := listener.Accept()
			if acceptErr == nil {
				connection.Close()
				connected <- struct{}{}
			}
		}()
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(temporary); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	t.Setenv("PATH", temporary)
	t.Setenv("HOME", temporary)
	if listenErr == nil {
		t.Setenv("HTTP_PROXY", "http://"+listener.Addr().String())
		t.Setenv("HTTPS_PROXY", "http://"+listener.Addr().String())
	}
	var stdout, stderr bytes.Buffer
	if code := run(nil, bytes.NewReader(commandRequest(t)), &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.String() != baselineOut.String() {
		t.Fatalf("ambient code=%d stderr=%q output=%q", code, stderr.String(), stdout.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ambient process marker err=%v", err)
	}
	if listenErr == nil {
		select {
		case <-connected:
			t.Fatal("ambient network connection observed")
		case <-time.After(25 * time.Millisecond):
		}
	} else {
		t.Logf("network listener unavailable for local spy: %v", listenErr)
	}
}

func TestRunRawRequestBoundary(t *testing.T) {
	for _, size := range []int{1499999, 1500000, 1500001} {
		var stdout, stderr bytes.Buffer
		code := run(nil, bytes.NewReader(bytes.Repeat([]byte("x"), size)), &stdout, &stderr)
		var candidate analyzerjs.Candidate
		if err := json.Unmarshal(bytes.TrimSuffix(stdout.Bytes(), []byte("\n")), &candidate); code != 0 || err != nil {
			t.Fatalf("size=%d code=%d output=%q err=%v", size, code, stdout.String(), err)
		}
		want := "NONCANONICAL_REQUEST"
		if size > 1500000 {
			want = "LIMIT_EXCEEDED"
		}
		if candidate.Status != "REJECTED" || candidate.Reason != want {
			t.Fatalf("size=%d candidate=%#v", size, candidate)
		}
	}
}

func builtCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "corvint-analyzer-js")
	command := exec.Command("go", "build", "-o", path, ".")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v: %s", err, output)
	}
	return path
}

func runCLI(t *testing.T, path string, raw []byte, directory string, environment []string) ([]byte, []byte, int) {
	t.Helper()
	command := exec.Command(path)
	command.Stdin = bytes.NewReader(raw)
	command.Dir, command.Env = directory, environment
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if err == nil {
		return stdout.Bytes(), stderr.Bytes(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return stdout.Bytes(), stderr.Bytes(), exit.ExitCode()
	}
	t.Fatalf("run CLI: %v", err)
	return nil, nil, -1
}

func TestBuiltCLIExactBoundaryAndPermutationBytes(t *testing.T) {
	path, directory := builtCLI(t), t.TempDir()
	baseline, _, _ := runCLI(t, path, commandRequest(t), directory, os.Environ())
	var bun analyzerjs.Request
	if err := json.Unmarshal(bytes.TrimSuffix(commandRequest(t), []byte("\n")), &bun); err != nil {
		t.Fatal(err)
	}
	bun.Inputs[1].Family = "js.bun-lock-v1"
	bunRaw, err := json.Marshal(bun)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLI(t, path, append(bunRaw, '\n'), directory, os.Environ())
	want := `{"profile":"` + analyzerjs.Profile + `","family":"` + analyzerjs.Family + `","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"` + bun.Inputs[0].Handle + `","family":"` + bun.Inputs[0].Family + `","path":"` + bun.Inputs[0].Path + `","sha256":"` + bun.Inputs[0].SHA256 + `"},{"handle":"` + bun.Inputs[1].Handle + `","family":"` + bun.Inputs[1].Family + `","path":"` + bun.Inputs[1].Path + `","sha256":"` + bun.Inputs[1].SHA256 + `"}],"reason":"UNSUPPORTED_SCHEMA"}` + "\n"
	if code != 0 || len(stderr) != 0 || string(stdout) != want || bytes.Contains(stdout, []byte("request_sha256")) {
		t.Fatalf("full echo code=%d stderr=%q output=%q want=%q", code, stderr, stdout, want)
	}
	for _, raw := range [][]byte{commandRequest(t), bytes.Repeat([]byte("x"), 1500000), bytes.Repeat([]byte("x"), 1500001)} {
		stdout, stderr, code = runCLI(t, path, raw, directory, os.Environ())
		if code != 0 || len(stderr) != 0 || len(stdout) == 0 {
			t.Fatalf("raw=%d code=%d stderr=%q output=%q", len(raw), code, stderr, stdout)
		}
	}
	for _, vector := range []struct {
		raw    []byte
		reason string
	}{{commandSourceRequest(t, analyzerjs.MaxFacts, 1), "LIMIT_EXCEEDED"}, {commandSourceRequest(t, 3_800, 200), "OUTPUT_LIMIT"}} {
		stdout, stderr, code := runCLI(t, path, vector.raw, directory, os.Environ())
		if code != 0 || len(stderr) != 0 || !bytes.Contains(stdout, []byte(`"reason":"`+vector.reason+`"`)) {
			t.Fatalf("limit=%s code=%d stderr=%q output=%q", vector.reason, code, stderr, stdout)
		}
	}
	var request analyzerjs.Request
	if err := json.Unmarshal(bytes.TrimSuffix(commandRequest(t), []byte("\n")), &request); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < len(request.Inputs)-1; index++ {
		permuted := request
		permuted.Inputs = append([]analyzerjs.Input(nil), request.Inputs...)
		permuted.Inputs[index], permuted.Inputs[index+1] = permuted.Inputs[index+1], permuted.Inputs[index]
		raw, err := json.Marshal(permuted)
		if err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLI(t, path, append(raw, '\n'), directory, os.Environ())
		if code != 0 || len(stderr) != 0 || !bytes.Contains(stdout, []byte(`"reason":"DUPLICATE_VALUE"`)) || bytes.Equal(stdout, baseline) {
			t.Fatalf("permutation=%d code=%d stderr=%q output=%q", index, code, stderr, stdout)
		}
	}
	nonASCII := analyzerjs.Request{Profile: analyzerjs.Profile, Family: analyzerjs.Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: analyzerjs.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []analyzerjs.Input{commandInput("input-1", "js.source", "src/main.js", `const x = érequire("forged");`)}}
	raw, err := json.Marshal(nonASCII)
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = runCLI(t, path, append(raw, '\n'), directory, os.Environ())
	if code != 0 || len(stderr) != 0 || !bytes.Contains(stdout, []byte(`"reason":"MALFORMED_INPUT"`)) {
		t.Fatalf("non-ASCII code=%d stderr=%q output=%q", code, stderr, stdout)
	}
	for _, source := range []string{
		`class C {} /import "forged-class"/.test(x); import "real";`,
		`try {} finally {} /import "forged-finally"/.test(x); import "real";`,
		`if (x) {} else {} /import "forged-else"/.test(x); import "real";`,
	} {
		stdout, stderr, code = runCLI(t, path, commandSingleSourceRequest(t, source), directory, os.Environ())
		if code != 0 || len(stderr) != 0 || bytes.Contains(stdout, []byte("forged-")) {
			t.Fatalf("statement source=%q code=%d stderr=%q output=%q", source, code, stderr, stdout)
		}
	}
}

func TestBuiltCLIUniqueFreshPermutations(t *testing.T) {
	path, directory := builtCLI(t), t.TempDir()
	modules := []string{"./App", "./styles/global.css", "@/lib/i18n", "@/lib/pwa", "@/lib/register", "@/lib/session", "react", "react-dom/client", "react-router"}
	outputs, permutations := map[[sha256.Size]byte]struct{}{}, map[string]struct{}{}
	for index := 0; index < 1024; index++ {
		left, order := index, append([]string(nil), modules...)
		for slot := len(order) - 1; slot > 0; slot-- {
			choice := left % (slot + 1)
			left /= slot + 1
			order[slot], order[choice] = order[choice], order[slot]
		}
		permutation := strings.Join(order, ",")
		if _, seen := permutations[permutation]; seen {
			t.Fatalf("duplicate permutation %d", index)
		}
		permutations[permutation] = struct{}{}
		var source strings.Builder
		for _, module := range order {
			source.WriteString(`import "` + module + `";`)
		}
		request := analyzerjs.Request{Profile: analyzerjs.Profile, Family: analyzerjs.Family, RequestID: "permutation", ScopeID: "root", CompilationUnitID: "unit-1", Target: analyzerjs.Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []analyzerjs.Input{commandInput("input-1", "js.source", "src/main.tsx", source.String())}}
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLI(t, path, append(raw, '\n'), directory, os.Environ())
		if code != 0 || len(stderr) != 0 || bytes.Count(stdout, []byte("\n")) != 1 {
			t.Fatalf("permutation=%d code=%d stderr=%q output=%q", index, code, stderr, stdout)
		}
		var candidate struct {
			Status string            `json:"status"`
			Facts  []analyzerjs.Fact `json:"facts"`
		}
		if err := json.Unmarshal(bytes.TrimSuffix(stdout, []byte("\n")), &candidate); err != nil || candidate.Status != "CANDIDATE" || len(candidate.Facts) != len(modules)+1 {
			t.Fatalf("permutation=%d candidate=%#v err=%v", index, candidate, err)
		}
		sum := sha256.Sum256(stdout)
		if _, seen := outputs[sum]; seen {
			t.Fatalf("duplicate output %d", index)
		}
		outputs[sum] = struct{}{}
	}
	if len(permutations) != 1024 || len(outputs) != 1024 {
		t.Fatalf("permutations=%d outputs=%d", len(permutations), len(outputs))
	}
}

func TestBuiltCLIFreshProcessExactOutputs(t *testing.T) {
	path, directory := builtCLI(t), t.TempDir()
	var baseline []byte
	for index := 0; index < 1001; index++ {
		stdout, stderr, code := runCLI(t, path, commandRequest(t), directory, os.Environ())
		if code != 0 || len(stderr) != 0 || bytes.Count(stdout, []byte("\n")) != 1 {
			t.Fatalf("fresh=%d code=%d stderr=%q output=%q", index, code, stderr, stdout)
		}
		if index == 0 {
			baseline = stdout
		} else if !bytes.Equal(baseline, stdout) {
			t.Fatalf("fresh=%d output differs", index)
		}
	}
}

func TestBuiltCLIReachableReasonBytes(t *testing.T) {
	path, directory := builtCLI(t), t.TempDir()
	var base analyzerjs.Request
	if err := json.Unmarshal(bytes.TrimSuffix(commandRequest(t), []byte("\n")), &base); err != nil {
		t.Fatal(err)
	}
	encode := func(request analyzerjs.Request) []byte {
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		return append(raw, '\n')
	}
	clone := func() analyzerjs.Request {
		request := base
		request.Inputs = append([]analyzerjs.Input(nil), base.Inputs...)
		return request
	}
	badID, badPath, badDigest, duplicate, conflict, unsupported, exact := clone(), clone(), clone(), clone(), clone(), clone(), clone()
	badID.RequestID = "-bad"
	badPath.Inputs[0].Path = "../bad.js"
	badDigest.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	duplicate.Inputs = append(duplicate.Inputs, duplicate.Inputs[0])
	conflict.Inputs[1] = commandInput("input-2", "js.npm-lock-v3", "package-lock.json", `{"name":"root","version":"2.0.0","lockfileVersion":3,"requires":true,"packages":{"":{"name":"root","version":"2.0.0"}}}`)
	unsupported.Inputs[1].Family = "js.bun-lock-v1"
	exact.Inputs = []analyzerjs.Input{commandInput("input-1", "js.package", "package.json", `{"name":"root","version":"1.0.0","dependencies":{"v":"1.0.0"}}`)}
	malformed := commandSingleSourceRequest(t, `const x = érequire("forged");`)
	dynamic := commandSingleSourceRequest(t, `import "data:text/plain,x";`)
	credential := commandSingleSourceRequest(t, `import "https://user@example.test/pkg";`)
	unknownFamily := clone()
	unknownFamily.Family = "other"
	vectors := []struct {
		raw    []byte
		reason string
	}{
		{[]byte("{}\n"), "NONCANONICAL_REQUEST"}, {encode(badID), "INVALID_IDENTIFIER"}, {encode(badPath), "INVALID_PATH"}, {encode(badDigest), "DIGEST_MISMATCH"}, {encode(unknownFamily), "UNKNOWN_FAMILY"},
		{append(append([]byte{}, commandRequest(t)[:len(commandRequest(t))-2]...), []byte(",\"unknown\":true}\n")...), "UNKNOWN_FIELD"}, {encode(duplicate), "DUPLICATE_VALUE"}, {encode(conflict), "CONFLICTING_VALUE"}, {malformed, "MALFORMED_INPUT"}, {encode(unsupported), "UNSUPPORTED_SCHEMA"}, {encode(exact), "EXACT_BINDING_UNAVAILABLE"}, {dynamic, "DYNAMIC_INPUT"}, {credential, "CREDENTIAL_INPUT"}, {commandSourceRequest(t, analyzerjs.MaxFacts, 1), "LIMIT_EXCEEDED"}, {commandSourceRequest(t, 3_800, 200), "OUTPUT_LIMIT"},
	}
	for _, vector := range vectors {
		stdout, stderr, code := runCLI(t, path, vector.raw, directory, os.Environ())
		if code != 0 || len(stderr) != 0 || bytes.Count(stdout, []byte("\n")) != 1 || bytes.Contains(stdout, []byte("request_sha256")) {
			t.Fatalf("reason=%s code=%d stderr=%q output=%q", vector.reason, code, stderr, stdout)
		}
		var candidate struct {
			Profile   string `json:"profile"`
			Family    string `json:"family"`
			RequestID string `json:"request_id"`
			Status    string `json:"status"`
			Reason    string `json:"reason"`
		}
		if err := json.Unmarshal(bytes.TrimSuffix(stdout, []byte("\n")), &candidate); err != nil || candidate.Profile != analyzerjs.Profile || candidate.Status != "REJECTED" || candidate.Reason != vector.reason {
			t.Fatalf("reason=%s candidate=%#v err=%v", vector.reason, candidate, err)
		}
		if !bytes.HasPrefix(stdout, []byte(`{"profile":"`+analyzerjs.Profile+`"`)) {
			t.Fatalf("reason=%s noncanonical=%q", vector.reason, stdout)
		}
	}
}

func TestBuiltCLIAmbientSpy(t *testing.T) {
	path, directory := builtCLI(t), t.TempDir()
	baseline, baselineErr, baselineCode := runCLI(t, path, commandRequest(t), directory, os.Environ())
	if baselineCode != 0 || len(baselineErr) != 0 {
		t.Fatalf("baseline code=%d stderr=%q", baselineCode, baselineErr)
	}
	marker := filepath.Join(directory, "process-marker")
	probe := filepath.Join(directory, "ambient-probe")
	if err := os.WriteFile(probe, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(probe).CombinedOutput(); err != nil {
		t.Fatalf("process positive control: %v: %s", err, output)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("local TCP spy unavailable: %v", err)
	}
	defer listener.Close()
	connected := make(chan struct{}, 1)
	go func() {
		if connection, accept := listener.Accept(); accept == nil {
			connection.Close()
			connected <- struct{}{}
		}
	}()
	environment := []string{"PATH=" + directory, "HOME=" + directory, "TMPDIR=" + directory, "HTTP_PROXY=http://" + listener.Addr().String(), "HTTPS_PROXY=http://" + listener.Addr().String(), "ALL_PROXY=http://" + listener.Addr().String(), "NO_PROXY="}
	stdout, stderr, code := runCLI(t, path, commandRequest(t), directory, environment)
	if code != 0 || len(stderr) != 0 || !bytes.Equal(stdout, baseline) {
		t.Fatalf("ambient code=%d stderr=%q output=%q", code, stderr, stdout)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ambient process marker=%v", err)
	}
	select {
	case <-connected:
		t.Fatal("ambient network observed")
	case <-time.After(25 * time.Millisecond):
	}
	if runtime.GOOS != "darwin" {
		return
	}
	sandboxPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	profile := func(extra string) string {
		return `(version 1) (allow default) (deny process-exec) (allow process-exec (literal "` + sandboxPath + `")) ` + extra
	}
	stdout, stderr, code = runSandboxCLI(t, profile(""), path, commandRequest(t), directory, environment)
	if strings.Contains(string(stderr), "sandbox_apply: Operation not permitted") {
		t.Fatalf("host sandbox unavailable: %s", stderr)
	}
	if code != 0 || len(stderr) != 0 || !bytes.Equal(stdout, baseline) {
		t.Fatalf("process policy code=%d stderr=%q output=%q", code, stderr, stdout)
	}
	if _, err := exec.Command("/usr/bin/sandbox-exec", "-p", profile(""), "/bin/sh", "-c", "true").CombinedOutput(); err == nil {
		t.Fatal("absolute subprocess negative control passed")
	}
	network := profile(`(deny network*) (allow process-exec (literal "/usr/bin/nc"))`)
	stdout, stderr, code = runSandboxCLI(t, network, path, commandRequest(t), directory, environment)
	if code != 0 || len(stderr) != 0 || !bytes.Equal(stdout, baseline) {
		t.Fatalf("network policy code=%d stderr=%q output=%q", code, stderr, stdout)
	}
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command("/usr/bin/sandbox-exec", "-p", network, "/usr/bin/nc", "-z", host, port).CombinedOutput(); err == nil {
		t.Fatal("direct network negative control passed")
	}
	repository := directory
	sandboxRepository, err := filepath.EvalSymlinks(repository)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "ambient-probe"), []byte("probe"), 0o600); err != nil {
		t.Fatal(err)
	}
	file := profile(`(deny file-read* (subpath "` + sandboxRepository + `")) (allow process-exec (literal "/bin/cat"))`)
	stdout, stderr, code = runSandboxCLI(t, file, path, commandRequest(t), directory, environment)
	if code != 0 || len(stderr) != 0 || !bytes.Equal(stdout, baseline) {
		t.Fatalf("repository policy code=%d stderr=%q output=%q", code, stderr, stdout)
	}
	if _, err := exec.Command("/usr/bin/sandbox-exec", "-p", file, "/bin/cat", filepath.Join(repository, "ambient-probe")).CombinedOutput(); err == nil {
		t.Fatal("repository-read negative control passed")
	}
	write := profile(`(deny file-write* (subpath "` + sandboxRepository + `")) (allow process-exec (literal "/usr/bin/touch"))`)
	stdout, stderr, code = runSandboxCLI(t, write, path, commandRequest(t), directory, environment)
	if code != 0 || len(stderr) != 0 || !bytes.Equal(stdout, baseline) {
		t.Fatalf("write policy code=%d stderr=%q output=%q", code, stderr, stdout)
	}
	if _, err := exec.Command("/usr/bin/sandbox-exec", "-p", write, "/usr/bin/touch", filepath.Join(repository, "ambient-write")).CombinedOutput(); err == nil {
		t.Fatal("repository-write negative control passed")
	}
}

func runSandboxCLI(t *testing.T, profile, path string, raw []byte, directory string, environment []string) ([]byte, []byte, int) {
	t.Helper()
	command := exec.Command("/usr/bin/sandbox-exec", "-p", profile, path)
	command.Stdin, command.Dir, command.Env = bytes.NewReader(raw), directory, environment
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if err == nil {
		return stdout.Bytes(), stderr.Bytes(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return stdout.Bytes(), stderr.Bytes(), exit.ExitCode()
	}
	t.Fatalf("sandbox CLI: %v", err)
	return nil, nil, -1
}
