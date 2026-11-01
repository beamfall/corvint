package companioncli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// CAP-V0-700: Answer handles exactly the five contract forms; any other
// argument list is left to the binary. The built binaries are checked end to
// end by internal/companionrelease's TestCAPV0700CompanionsAnswerTheSharedContract.
func TestCAPV0700AnswerHandlesOnlyTheContractForms(t *testing.T) {
	spec := Spec{Binary: "corvint-x", Version: ExperimentalVersion, Usage: "usage: corvint-x\nExample: corvint-x run\n"}
	for _, arguments := range [][]string{{"--help"}, {"-h"}, {"help"}, {"help", "--json"}, {"--version"}, {"--version", "--json"}} {
		var stdout bytes.Buffer
		if code, handled := Answer(spec, arguments, &stdout); !handled || code != 0 || stdout.Len() == 0 {
			t.Fatalf("%q: handled=%v code=%d", arguments, handled, code)
		}
	}
	var help bytes.Buffer
	Answer(spec, []string{"help", "--json"}, &help)
	var decoded struct {
		Profile, Binary string
		Usage           []string
	}
	if json.Unmarshal(help.Bytes(), &decoded) != nil || decoded.Profile != HelpProfile || len(decoded.Usage) != 2 {
		t.Fatalf("help --json %s", help.Bytes())
	}
	for _, arguments := range [][]string{nil, {"run"}, {"--help", "extra"}, {"help", "run"}, {"--version", "--yaml"}} {
		var stdout bytes.Buffer
		if _, handled := Answer(spec, arguments, &stdout); handled || stdout.Len() != 0 {
			t.Fatalf("%q handled", arguments)
		}
	}
}
