// Package mcptestvalidityv0 is a black-box consumer of the production
// corvint-test-validity-mcp stdio executable
// (docs/specs/mcp-test-validity-profile-v0.md). It imports no Corvint package
// and speaks only newline-delimited JSON-RPC.
package mcptestvalidityv0

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	protocolVersion = "2026-07-28"
	maxLineBytes    = 1 << 20
)

var serverBinary string

func TestMain(m *testing.M) {
	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	temp, err := os.MkdirTemp("", "corvint-test-validity-mcp-conformance-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	serverBinary = filepath.Join(temp, "corvint-test-validity-mcp")
	if runtime.GOOS == "windows" {
		serverBinary += ".exe"
	}
	command := exec.Command("go", "build", "-o", serverBinary, "./cmd/corvint-test-validity-mcp")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
	exitCode := 1
	if output, buildErr := command.CombinedOutput(); buildErr != nil {
		fmt.Fprintf(os.Stderr, "build corvint-test-validity-mcp: %v\n%s", buildErr, output)
	} else {
		exitCode = m.Run()
	}
	if removeErr := os.RemoveAll(temp); removeErr != nil && exitCode == 0 {
		fmt.Fprintf(os.Stderr, "remove conformance temp: %v\n", removeErr)
		exitCode = 1
	}
	os.Exit(exitCode)
}

type testExpectation struct {
	Name      string `json:"name"`
	Package   string `json:"package"`
	State     string `json:"state"`
	Execution string `json:"execution"`
	Strength  string `json:"strength"`
}

type vector struct {
	ID          string            `json:"id"`
	Requirement string            `json:"requirement"`
	Files       map[string]string `json:"files"`
	Generated   map[string]string `json:"generated"`
	Arguments   map[string]any    `json:"arguments"`
	Expect      struct {
		RPCError     int               `json:"rpcError"`
		IsError      bool              `json:"isError"`
		Code         string            `json:"code"`
		Source       string            `json:"source"`
		Kind         string            `json:"kind"`
		Tier         string            `json:"tier"`
		Promotable   *bool             `json:"promotable"`
		Receipt      any               `json:"receipt"`
		RunAxes      string            `json:"runAxes"`
		RunReason    string            `json:"runReason"`
		RunExecution string            `json:"runExecution"`
		Tests        []testExpectation `json:"tests"`
	} `json:"expect"`
}

type manifest struct {
	Profile         string   `json:"profile"`
	ProtocolVersion string   `json:"protocolVersion"`
	Transport       string   `json:"transport"`
	Server          string   `json:"server"`
	Tools           []string `json:"tools"`
	Vectors         []vector `json:"vectors"`
}

func loadManifest(t *testing.T) manifest {
	t.Helper()
	raw, err := os.ReadFile("cases.json")
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var got manifest
	if err := decoder.Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Profile != "corvint-mcp-test-validity-conformance/0" || got.ProtocolVersion != protocolVersion ||
		got.Transport != "stdio" || got.Server != "corvint-test-validity-mcp" || len(got.Vectors) == 0 {
		t.Fatalf("invalid manifest header: %+v", got)
	}
	return got
}

// MTV-V0-001/MTV-V0-002/MTV-V0-009: the profile advertises exactly one read-only,
// closed-world tool with a closed input schema.
func TestToolCatalogueIsExactlyOneReadOnlyTool(t *testing.T) {
	cases := loadManifest(t)
	client := startServer(t, fixtureRepository(t))
	defer client.close(t)
	result := successResult(t, client.call(t, 1, "tools/list", map[string]any{"_meta": requestMeta()}))
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) != len(cases.Tools) {
		t.Fatalf("tools=%v", result["tools"])
	}
	tool := tools[0].(map[string]any)
	annotations := tool["annotations"].(map[string]any)
	schema := tool["inputSchema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	if tool["name"] != cases.Tools[0] || annotations["readOnlyHint"] != true || annotations["destructiveHint"] != false ||
		annotations["openWorldHint"] != false || schema["additionalProperties"] != false ||
		len(properties) != 2 || properties["receipt"] == nil || properties["discover"] == nil || len(schema["required"].([]any)) != 0 {
		t.Fatalf("tool=%v", tool)
	}
}

// MTV-V0-002..007: every vector's call yields its expected protocol error,
// tool error, or projection, and the whole run leaves the repository
// byte-identical.
func TestVectorsAndReadOnly(t *testing.T) {
	cases := loadManifest(t)
	root := fixtureRepository(t)
	outside := filepath.Join(t.TempDir(), "outside.json")
	writeFile(t, outside, `{"receipt":{"kind":"unit","tests":[]}}`)
	for _, item := range cases.Vectors {
		prepare(t, root, outside, item)
	}
	before := treeDigest(t, root)
	client := startServer(t, root)
	defer client.close(t)
	for index, item := range cases.Vectors {
		t.Run(item.ID, func(t *testing.T) {
			response := client.call(t, 100+index, "tools/call", map[string]any{
				"_meta": requestMeta(), "name": cases.Tools[0], "arguments": item.Arguments,
			})
			checkVector(t, item, response)
		})
	}
	if after := treeDigest(t, root); after != before {
		t.Fatalf("repository changed: before=%s after=%s", before, after)
	}
}

func TestUnknownToolIsInvalidParams(t *testing.T) {
	client := startServer(t, fixtureRepository(t))
	defer client.close(t)
	response := client.call(t, 1, "tools/call", map[string]any{"_meta": requestMeta(), "name": "corvint.query", "arguments": map[string]any{}})
	if code := rpcErrorCode(response); code != -32602 {
		t.Fatalf("response=%v", response)
	}
}

func prepare(t *testing.T, root, outside string, item vector) {
	t.Helper()
	for name, body := range item.Files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), body)
	}
	for name, kind := range item.Generated {
		path := filepath.Join(root, filepath.FromSlash(name))
		switch kind {
		case "oversized-input":
			document := `{"receipt":{"kind":"unit","tests":[]}}`
			writeFile(t, path, document+strings.Repeat(" ", 4<<20+1-len(document)))
		case "many-tests":
			tests := make([]string, 0, 3000)
			for number := range 3000 {
				tests = append(tests, fmt.Sprintf(`{"name":"t%d","state":"passed"}`, number))
			}
			writeFile(t, path, `{"receipt":{"kind":"unit","tests":[`+strings.Join(tests, ",")+`]}}`)
		case "symlink-outside":
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, path); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("%s: unknown generator %q", item.ID, kind)
		}
	}
}

func checkVector(t *testing.T, item vector, response map[string]any) {
	t.Helper()
	if item.Expect.RPCError != 0 {
		if code := rpcErrorCode(response); code != item.Expect.RPCError {
			t.Fatalf("want rpc error %d: %v", item.Expect.RPCError, response)
		}
		return
	}
	result := successResult(t, response)
	structured := result["structuredContent"].(map[string]any)
	if result["isError"] != item.Expect.IsError {
		t.Fatalf("isError=%v structured=%v", result["isError"], structured)
	}
	if item.Expect.IsError {
		if structured["code"] != item.Expect.Code || structured["mutates"] != false {
			t.Fatalf("tool error=%v", structured)
		}
		return
	}
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"schema":"corvint-mcp-test-validity-result/0"`) {
		t.Fatalf("text block does not carry the result: %.200s", text)
	}
	document := structured["document"].(map[string]any)
	if structured["mutates"] != false || !reflect.DeepEqual(structured["receipt"], item.Expect.Receipt) ||
		document["schema"] != "corvint-test-validity/0" || document["source"] != item.Expect.Source {
		t.Fatalf("structured=%v", structured)
	}
	if item.Expect.Kind != "" && document["kind"] != item.Expect.Kind {
		t.Fatalf("kind=%v", document["kind"])
	}
	if item.Expect.Tier != "" && document["tier"] != item.Expect.Tier {
		t.Fatalf("tier=%v", document["tier"])
	}
	if item.Expect.Promotable != nil && document["promotable"] != *item.Expect.Promotable {
		t.Fatalf("promotable=%v", document["promotable"])
	}
	if item.Expect.RunAxes != "" {
		run := document["run"].(map[string]any)
		for _, axis := range []string{"association", "hygiene", "freshness", "execution", "strength"} {
			value := run[axis].(map[string]any)
			if value["state"] != item.Expect.RunAxes || value["reason"] != item.Expect.RunReason {
				t.Fatalf("run.%s=%v", axis, value)
			}
		}
	}
	if item.Expect.RunExecution != "" {
		run := document["run"].(map[string]any)
		if state := run["execution"].(map[string]any)["state"]; state != item.Expect.RunExecution {
			t.Fatalf("run.execution=%v", state)
		}
	}
	tests := document["tests"].([]any)
	if len(tests) != len(item.Expect.Tests) {
		t.Fatalf("tests=%v", tests)
	}
	for index, want := range item.Expect.Tests {
		got := tests[index].(map[string]any)
		projection := got["projection"].(map[string]any)
		if got["name"] != want.Name || projection["execution"].(map[string]any)["state"] != want.Execution ||
			projection["strength"].(map[string]any)["state"] != want.Strength {
			t.Fatalf("test %d=%v", index, got)
		}
		if want.Package != "" && got["package"] != want.Package {
			t.Fatalf("test %d package=%v", index, got["package"])
		}
		if want.State != "" && got["state"] != want.State {
			t.Fatalf("test %d state=%v", index, got["state"])
		}
	}
}

type stdioClient struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	stderr  bytes.Buffer
}

func startServer(t *testing.T, root string) *stdioClient {
	t.Helper()
	command := exec.Command(serverBinary, "--root", root)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	client := &stdioClient{command: command, stdin: stdin, stdout: bufio.NewReaderSize(stdout, maxLineBytes+2)}
	command.Stderr = &client.stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	return client
}

func (client *stdioClient) call(t *testing.T, id int, method string, params map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.stdin.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
	type readResult struct {
		line []byte
		err  error
	}
	lines := make(chan readResult, 1)
	go func() {
		line, readErr := client.stdout.ReadBytes('\n')
		lines <- readResult{line, readErr}
	}()
	select {
	case got := <-lines:
		if got.err != nil || len(got.line)-1 > maxLineBytes {
			t.Fatalf("read: %v bytes=%d stderr=%q", got.err, len(got.line), client.stderr.String())
		}
		var message map[string]any
		if err := json.Unmarshal(got.line, &message); err != nil {
			t.Fatalf("non-JSON stdout: %v", err)
		}
		if message["id"] != float64(id) {
			t.Fatalf("id=%v want %d", message["id"], id)
		}
		return message
	case <-time.After(30 * time.Second):
		_ = client.command.Process.Kill()
		t.Fatalf("timeout; stderr=%q", client.stderr.String())
	}
	return nil
}

func (client *stdioClient) close(t *testing.T) {
	t.Helper()
	_ = client.stdin.Close()
	done := make(chan error, 1)
	go func() {
		_, _ = io.Copy(io.Discard, client.stdout)
		done <- client.command.Wait()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("server exit: %v; stderr=%q", err, client.stderr.String())
		}
	case <-time.After(10 * time.Second):
		_ = client.command.Process.Kill()
		t.Fatal("server did not exit after stdin EOF")
	}
}

func requestMeta() map[string]any {
	return map[string]any{
		"io.modelcontextprotocol/protocolVersion":    protocolVersion,
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "corvint-test-validity-conformance", "version": "0"},
	}
}

func successResult(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("not a result: %v", response)
	}
	return result
}

func rpcErrorCode(response map[string]any) int {
	body, ok := response["error"].(map[string]any)
	if !ok {
		return 0
	}
	code, _ := body["code"].(float64)
	return int(code)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRepository(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "README.md"), "fixture\n")
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

func treeDigest(t *testing.T, root string) string {
	t.Helper()
	paths := []string{}
	if err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		paths = append(paths, path)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(hash, "%s\x00%s\x00%d\x00", path, info.Mode(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hash.Write(body)
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func findModuleRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("cannot find module root")
		}
		directory = parent
	}
}
