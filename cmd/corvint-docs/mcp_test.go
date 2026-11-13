// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/docsserver"
)

// MTT-V1-004: `corvint-docs mcp`, with nothing but this executable, lists
// the documentation tools and answers corpus tools on a fixture repository
// with the same receipt `corvint-docs docs corpus` prints (MTT-V1-009). The
// server runs in a child of this test binary because a started server pins
// Git for its whole process.
func TestMTTV1004CorvintDocsMCPAnswersOnAFixture(t *testing.T) {
	root, _ := typedCorpusCLIFixture(t)
	responses := docsMCPChild(t, []string{"--root", root, "--corpus-artifact", "corpus.json"},
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"corvint.docs_get","arguments":{"id":"proof:flow"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"corvint.help","arguments":{}}}`,
	)
	var listed []string
	for _, raw := range responses["1"]["result"].(map[string]any)["tools"].([]any) {
		listed = append(listed, raw.(map[string]any)["name"].(string))
	}
	for _, name := range []string{"corvint.help", "corvint.docs_draft", "corvint.docs_consume", "corvint.docs_get", "corvint.docs_search"} {
		if !slices.Contains(listed, name) {
			t.Fatalf("%s not listed: %q", name, listed)
		}
	}
	result, _ := responses["2"]["result"].(map[string]any)
	if result["isError"] != false {
		t.Fatalf("docs_get: %#v", responses["2"])
	}
	code, out, stderr := corpusCLI(t, root, "docs", "corpus", "get", "--artifact", "corpus.json", "--id", "proof:flow")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	var cli, mcp doccorpus.Receipt
	raw, err := json.Marshal(result["structuredContent"])
	if err != nil || json.Unmarshal(raw, &mcp) != nil || json.Unmarshal([]byte(out), &cli) != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(cli, mcp) || len(cli.Results) == 0 {
		t.Fatalf("corvint-docs mcp docs_get differs from the CLI:\nmcp %+v\ncli %+v", mcp, cli)
	}
	if help, _ := responses["3"]["result"].(map[string]any); help["isError"] != false {
		t.Fatalf("help: %#v", responses["3"])
	}
}

// MTT-V1-004: the verb's help is the server's usage, reachable both ways.
func TestMTTV1004CorvintDocsMCPHelp(t *testing.T) {
	for _, arguments := range [][]string{{"mcp", "--help"}, {"help", "mcp"}} {
		var stdout, stderr bytes.Buffer
		if code := run(arguments, strings.NewReader(""), &stdout, &stderr); code != 0 || stdout.String() != docsserver.Usage {
			t.Fatalf("%q: exit %d stdout %q stderr %q", arguments, code, &stdout, &stderr)
		}
	}
	if !strings.Contains(rootHelp, "corvint-docs mcp --root ABSOLUTE_ROOT") {
		t.Fatal("root help does not name corvint-docs mcp")
	}
}

func docsMCPChild(t *testing.T, extra []string, requests ...string) map[string]map[string]any {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	lines := append([]string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
	}, requests...)
	child := exec.Command(binary, append([]string{"-test.run=^TestMTTV1004DocsMCPHelper$", "--", "--mtt-docs-mcp"}, extra...)...)
	child.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	var stdout, stderr bytes.Buffer
	child.Stdout, child.Stderr = &stdout, &stderr
	if err := child.Run(); err != nil {
		t.Fatalf("corvint-docs mcp child: %v\nstdout %s\nstderr %s", err, &stdout, &stderr)
	}
	responses := map[string]map[string]any{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		var response map[string]any
		if err := json.Unmarshal(line, &response); err != nil {
			t.Fatalf("%v: %s\nstderr %s", err, line, &stderr)
		}
		id, _ := response["id"].(float64)
		responses[strconv.FormatFloat(id, 'f', -1, 64)] = response
	}
	return responses
}

// TestMTTV1004DocsMCPHelper is the child of docsMCPChild, selected by argv:
// it serves `corvint-docs mcp EXTRA... --protocol-version 2025-11-25`.
func TestMTTV1004DocsMCPHelper(t *testing.T) {
	index := slices.Index(os.Args, "--mtt-docs-mcp")
	if index < 0 {
		return
	}
	arguments := append([]string{"mcp", "--protocol-version", "2025-11-25"}, os.Args[index+1:]...)
	os.Exit(run(arguments, os.Stdin, os.Stdout, os.Stderr))
}
