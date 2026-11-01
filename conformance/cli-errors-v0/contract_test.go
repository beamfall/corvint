package clierrors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
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
// input (AFP-V0-037).
var contracts = map[string]contract{
	"corvint":                    {"./cmd/corvint", 2, "stderr", jsonFields(map[string]any{"code": "invalid-arguments", "ok": false}, "argument command: invalid choice: '"+unknownVerb+"'")},
	"corvint-cem-candidate":      {"./cmd/corvint-cem-candidate", 2, "", nil},
	"corvint-cem-experiments":    {"./cmd/corvint-cem-experiments", 1, "stderr", text("experiment refused: unknown command")},
	"corvint-corpus-republish":   {"./cmd/corvint-corpus-republish", 1, "stderr", text("republish command unavailable")},
	"corvint-dashboard-snapshot": {"./cmd/corvint-dashboard-snapshot", 2, "stderr", jsonFields(map[string]any{"code": "DASHBOARD_INVALID_ARGUMENT", "profile": "corvint-dashboard-error/0"}, "")},
	"corvint-go-test-provider":   {"./cmd/corvint-go-test-provider", 2, "stdout", jsonFields(map[string]any{"code": "IDENTITY_MISMATCH", "profile": "go-live-error/0"}, "")},
	"corvint-intake":             {"./cmd/corvint-intake", 2, "stderr", text("INTAKE_ARGUMENTS")},
	"corvint-js-test-provider":   {"./cmd/corvint-js-test-provider", 2, "stderr", text(`unknown subcommand "` + unknownVerb + `"`)},
	"corvint-postmerge-connect":  {"./cmd/corvint-postmerge-connect", 1, "stderr", text("experimental-and-absolute-root-required")},
	"corvint-postmerge-metrics":  {"./cmd/corvint-postmerge-metrics", 1, "stderr", jsonFields(map[string]any{"error": "invalid-input"}, "")},
	"corvint-postmerge-workflow": {"./cmd/corvint-postmerge-workflow", 2, "stderr", text("usage: corvint-postmerge-workflow")},
	"corvint-pulse":              {"./cmd/corvint-pulse", 2, "stderr", jsonFields(map[string]any{"code": "invalid-arguments", "ok": false}, "")},
	"corvint-ruby-test-provider": {"./cmd/corvint-ruby-test-provider", 2, "stderr", text(`unknown subcommand "` + unknownVerb + `"`)},
	"corvint-tasks":              {"./cmd/corvint-tasks", 1, "stdout", tasksResult("MALFORMED", "unknown verb")},
	"corvint-test":               {"./cmd/corvint-test", 1, "stderr", text("usage: corvint-test")},
}

// TestCLIErrorContracts_ACXV0859 (ACX-V0-859, TCL-V0-859; V1-3736) builds
// every catalog binary and checks its unknown-verb refusal against its row,
// then the near-verb suggestions of corvint and corvint-tasks.
func TestCLIErrorContracts_ACXV0859(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the rows record the POSIX refusals; Windows runs the commands' own tests")
	}
	root := filepath.Clean(filepath.Join("..", ".."))
	trees, err := filepath.Glob(filepath.Join(root, "internal", "capcatalog", "trees", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog, declared []string
	for _, tree := range trees {
		catalog = append(catalog, strings.TrimSuffix(filepath.Base(tree), ".json"))
	}
	for name := range contracts {
		declared = append(declared, name)
	}
	slices.Sort(catalog)
	slices.Sort(declared)
	if !slices.Equal(catalog, declared) {
		t.Fatalf("catalog binaries %v, declared rows %v", catalog, declared)
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
