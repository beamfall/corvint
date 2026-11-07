package clitree

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCAPV07700RootPositionalInput(t *testing.T) {
	tree := Tree{Binary: "receipt", Source: "companion root contract", Nodes: []Node{{Required: []string{"GOROOT"}}, {Path: []string{"help"}}}}
	if _, unknown := UnknownVerb(tree, []string{"relative-goroot"}); unknown {
		t.Fatal("a required positional input is not a command name")
	}
	tree.Nodes[0].Required = []string{"stdin: protocol request JSON"}
	if _, unknown := UnknownVerb(tree, []string{"zz-no-such-command"}); !unknown {
		t.Fatal("protocol roots must still refuse unknown command words")
	}
}

// ACX-V0-904: an operation word that names no node or internal entry point is refused with the
// one shared envelope on stderr and exit 2; a known verb, an internal word or an option is not.
func TestACXV0904UnknownVerbRefusal(t *testing.T) {
	tree := Tree{Binary: "corvint-x", Nodes: []Node{{Path: []string{"run"}}, {Path: []string{"run", "deep"}}}, Internal: []Internal{{Word: "worker"}}}
	for _, args := range [][]string{nil, {"run"}, {"worker"}, {"--root", "x"}, {"Upper"}} {
		if verb, unknown := UnknownVerb(tree, args); unknown {
			t.Fatalf("%q: unknown verb %q", args, verb)
		}
	}
	var stderr bytes.Buffer
	exit, refused := RefuseIfUnknownVerb(tree, []string{"zz-no-such-verb", "--flag"}, &stderr)
	if !refused || exit != 2 {
		t.Fatalf("exit %d refused %v", exit, refused)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil || bytes.Count(stderr.Bytes(), []byte("\n")) != 1 {
		t.Fatalf("not one JSON line: %q (%v)", stderr.String(), err)
	}
	want := map[string]any{"binary": "corvint-x", "code": "unknown-verb", "error": `unknown command "zz-no-such-verb"`, "help": "corvint-x --help", "ok": false}
	for key, value := range want {
		if envelope[key] != value {
			t.Fatalf("%s = %v, want %v", key, envelope[key], value)
		}
	}
	if len(envelope) != len(want) {
		t.Fatalf("envelope %v has extra fields", envelope)
	}
}

// ACX-V0-904: Help.Serve refuses an unknown verb with the shared envelope and leaves a known
// verb to the binary's dispatch.
func TestACXV0904HelpServeRefusesUnknownVerb(t *testing.T) {
	help := Help{Tree: CompanionTree("demo", "demo run --dir DIR\n", map[string]string{"run": "Run once"}, nil, nil, nil)}
	var stdout, stderr bytes.Buffer
	if code, handled := help.Serve([]string{"nope"}, &stdout, &stderr); !handled || code != 2 || stdout.Len() != 0 ||
		stderr.String() != `{"binary":"demo","code":"unknown-verb","error":"unknown command \"nope\"","help":"demo --help","ok":false}`+"\n" {
		t.Fatalf("nope: %v %d %q %q", handled, code, stdout.String(), stderr.String())
	}
	if _, handled := help.Serve([]string{"run", "--dir", "x"}, &stdout, &stderr); handled {
		t.Fatal("run was answered")
	}
}
