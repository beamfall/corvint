// SPDX-License-Identifier: Apache-2.0
package clierrors_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const unknownVerb = "zz-no-such-verb"

// contract is one binary's refusal of an unknown verb: the exit code, the
// stream that carries the envelope (the other stays empty; "" means both are
// empty) and the envelope check.
type contract struct {
	pkg    string
	exit   int
	stream string
	check  func(out string) error
}

// contracts is keyed by binary name; the package path literal is the build
// input (AFP-V0-037). Every companion refuses with the one shared envelope
// (ACX-V0-904, V1-3878); corvint keeps its argparse-parity invalid-arguments
// envelope (ACX-V0-859), corvint-tasks its Tasks command result on stdout
// and corvint-work-queue the exact text WQO-V0-007 and WQO-V0-030 pin.
var contracts = map[string]contract{
	"corvint":                    {"./cmd/corvint", 2, "stderr", jsonFields(map[string]any{"code": "invalid-arguments", "ok": false}, "argument command: invalid choice: '"+unknownVerb+"'")},
	"corvint-cem-candidate":      companion("./cmd/corvint-cem-candidate"),
	"corvint-cem-experiments":    companion("./cmd/corvint-cem-experiments"),
	"corvint-corpus-republish":   companion("./cmd/corvint-corpus-republish"),
	"corvint-dashboard-snapshot": companion("./cmd/corvint-dashboard-snapshot"),
	"corvint-gateway":            companion("./cmd/corvint-gateway"),
	"corvint-go-test-provider":   companion("./cmd/corvint-go-test-provider"),
	"corvint-intake":             companion("./cmd/corvint-intake"),
	"corvint-js-test-provider":   companion("./cmd/corvint-js-test-provider"),
	"corvint-postmerge-connect":  companion("./cmd/corvint-postmerge-connect"),
	"corvint-postmerge-metrics":  companion("./cmd/corvint-postmerge-metrics"),
	"corvint-postmerge-workflow": companion("./cmd/corvint-postmerge-workflow"),
	"corvint-pulse":              companion("./cmd/corvint-pulse"),
	"corvint-ruby-test-provider": companion("./cmd/corvint-ruby-test-provider"),
	"corvint-tasks":              {"./cmd/corvint-tasks", 1, "stdout", tasksResult("MALFORMED", "unknown verb")},
	"corvint-test":               companion("./cmd/corvint-test"),
	"corvint-update":             companion("./cmd/corvint-update"),
	"corvint-work-queue":         {"./cmd/corvint-work-queue", 2, "stderr", text("unsupported operation")},
}

// companion is the shared unknown-verb refusal row (ACX-V0-904): exit 2 and
// one JSON line on stderr naming the binary, the unknown-verb code and the word.
func companion(pkg string) contract {
	binary := pkg[len("./cmd/"):]
	return contract{pkg, 2, "stderr", envelope(map[string]any{
		"binary": binary, "code": "unknown-verb", "error": `unknown command "` + unknownVerb + `"`,
		"help": binary + " --help", "ok": false,
	})}
}

// TestCLIErrorContracts_ACXV0859 (ACX-V0-859, TCL-V0-859; V1-3736) builds
// every catalog binary and checks its unknown-verb refusal against its row,
// then the near-verb suggestions of corvint and corvint-tasks.
func TestCLIErrorContracts_ACXV0859(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the rows record the POSIX refusals; Windows runs the commands' own tests")
	}
	root := filepath.Clean(filepath.Join("..", ".."))
	var declared []string
	for name := range contracts {
		declared = append(declared, name)
	}
	// ACX-V0-664: every unknown-verb refusal carries a code, so no row may be
	// silent or unchecked.
	for _, name := range declared {
		if row := contracts[name]; row.stream == "" || row.check == nil {
			t.Errorf("%s: unknown-verb refusal has no checked, coded envelope", name)
		}
	}
	bin := t.TempDir()
	build := exec.Command("go", "build", "-trimpath", "-o", bin+string(filepath.Separator))
	for _, name := range declared {
		build.Args = append(build.Args, contracts[name].pkg)
	}
	build.Dir = root
	build.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	work := t.TempDir()
	for _, name := range declared {
		row := contracts[name]
		code, stdout, stderr := launch(t, filepath.Join(bin, name), work, unknownVerb)
		if code != row.exit {
			t.Errorf("%s: exit %d, want %d (stdout %q stderr %q)", name, code, row.exit, stdout, stderr)
			continue
		}
		var out, other string
		switch row.stream {
		case "stdout":
			out, other = stdout, stderr
		case "stderr":
			out, other = stderr, stdout
		default:
			other = stdout + stderr
		}
		if other != "" || (row.stream != "" && out == "") {
			t.Errorf("%s: want the envelope on %q only; stdout %q stderr %q", name, row.stream, stdout, stderr)
			continue
		}
		if row.check != nil {
			if err := row.check(out); err != nil {
				t.Errorf("%s: %v: %q", name, err, out)
			}
		}
	}
	if _, _, stderr := launch(t, filepath.Join(bin, "corvint"), work, "contxt"); !strings.Contains(stderr, "; did you mean 'context'?") {
		t.Errorf("corvint contxt: no suggestion: %q", stderr)
	}
	if _, stdout, _ := launch(t, filepath.Join(bin, "corvint-tasks"), work, "tikcet", "show"); !strings.Contains(stdout, "did you mean `corvint-tasks ticket --help`") {
		t.Errorf("corvint-tasks tikcet: no suggestion: %q", stdout)
	}
}

func launch(t *testing.T, binary, dir string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	command := exec.Command(binary, args...)
	command.Dir = dir
	command.Stdin = strings.NewReader("")
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), stdout.String(), stderr.String()
	} else if err != nil {
		t.Fatalf("%s: %v", binary, err)
	}
	return 0, stdout.String(), stderr.String()
}

// text checks that a plain-text refusal contains want.
func text(want string) func(string) error {
	return func(out string) error {
		if !strings.Contains(out, want) {
			return fmt.Errorf("text does not contain %q", want)
		}
		return nil
	}
}

// jsonFields checks one JSON object whose fields include want and, when
// errorPrefix is set, whose "error" string starts with it.
func jsonFields(want map[string]any, errorPrefix string) func(string) error {
	return func(out string) error {
		var got map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			return fmt.Errorf("not one JSON object: %v", err)
		}
		for key, value := range want {
			if got[key] != value {
				return fmt.Errorf("field %s = %v, want %v", key, got[key], value)
			}
		}
		if message, _ := got["error"].(string); errorPrefix != "" && !strings.HasPrefix(message, errorPrefix) {
			return fmt.Errorf("error %q does not start with %q", message, errorPrefix)
		}
		return nil
	}
}

// envelope checks one JSON line whose fields are exactly want.
func envelope(want map[string]any) func(string) error {
	return func(out string) error {
		if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
			return fmt.Errorf("want one line")
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			return fmt.Errorf("not one JSON object: %v", err)
		}
		if len(got) != len(want) {
			return fmt.Errorf("fields %v, want %v", got, want)
		}
		for key, value := range want {
			if got[key] != value {
				return fmt.Errorf("field %s = %v, want %v", key, got[key], value)
			}
		}
		return nil
	}
}

// tasksResult checks a taskman-command-result/0 ERROR envelope that carries
// code and a warning containing warning.
func tasksResult(code, warning string) func(string) error {
	return func(out string) error {
		var got struct {
			Profile  string   `json:"profile"`
			Outcome  string   `json:"outcome"`
			Codes    []string `json:"codes"`
			Warnings []string `json:"warnings"`
		}
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			return fmt.Errorf("not one JSON object: %v", err)
		}
		if got.Profile != "taskman-command-result/0" || got.Outcome != "ERROR" || !slices.Contains(got.Codes, code) || !strings.Contains(strings.Join(got.Warnings, ";"), warning) {
			return fmt.Errorf("want an ERROR result with code %s and a %q warning", code, warning)
		}
		return nil
	}
}
