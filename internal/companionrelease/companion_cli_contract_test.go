package companionrelease

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/companioncli"
)

// contractCompanions are the binaries a release ships beside corvint and corvint-tasks
// (CAP-V0-700).
var contractCompanions = []string{
	"corvint-console", "corvint-corpus-mcp", "corvint-dashboard-snapshot", "corvint-docs-mcp",
	"corvint-mcp", "corvint-test-validity-mcp", "corvint-update", "corvint-web-flows",
}

func buildContractCompanions(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	root := filepath.Join("..", "..")
	arguments := []string{"build", "-trimpath", "-o", bin + string(filepath.Separator)}
	for _, name := range contractCompanions {
		arguments = append(arguments, "./cmd/"+name)
	}
	build := exec.Command("go", arguments...)
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
	}
	return bin
}

func runContractCompanion(t *testing.T, binary, dir string, arguments ...string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, arguments...)
	command.Dir = dir
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s %q did not exit within 10s (it started serving)", filepath.Base(binary), arguments)
	}
	code := 0
	if exit, isExit := err.(*exec.ExitError); isExit {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

// CAP-V0-700: every companion answers --help, -h and help with its usage,
// help --json with corvint-companion-help/0, --version with one
// `BINARY VERSION` line and --version --json with corvint-companion-version/0,
// each exit 0 on stdout, from a directory that is not a repository.
func TestCAPV0700CompanionsAnswerTheSharedContract(t *testing.T) {
	bin := buildContractCompanions(t)
	outside := t.TempDir()
	for _, name := range contractCompanions {
		binary := filepath.Join(bin, name)
		var usage string
		for _, form := range [][]string{{"--help"}, {"-h"}, {"help"}} {
			code, stdout, stderr := runContractCompanion(t, binary, outside, form...)
			if code != 0 || stderr != "" || !strings.HasPrefix(stdout, "usage: "+name) || !strings.Contains(stdout, "Example: "+name) {
				t.Fatalf("%s %q: exit %d stdout %q stderr %q", name, form, code, stdout, stderr)
			}
			if usage != "" && stdout != usage {
				t.Fatalf("%s %q prints other usage", name, form)
			}
			usage = stdout
		}
		code, stdout, _ := runContractCompanion(t, binary, outside, "help", "--json")
		var help struct {
			Profile, Binary, Version string
			Usage                    []string
		}
		if code != 0 || json.Unmarshal([]byte(stdout), &help) != nil || help.Profile != companioncli.HelpProfile || help.Binary != name ||
			help.Version != companioncli.ExperimentalVersion || strings.Join(help.Usage, "\n")+"\n" != usage {
			t.Fatalf("%s help --json: exit %d %q", name, code, stdout)
		}
		if code, stdout, _ := runContractCompanion(t, binary, outside, "--version"); code != 0 || stdout != name+" "+companioncli.ExperimentalVersion+"\n" {
			t.Fatalf("%s --version: exit %d %q", name, code, stdout)
		}
		code, stdout, _ = runContractCompanion(t, binary, outside, "--version", "--json")
		var version struct{ Profile, Binary, Version, Commit, CommitState string }
		if code != 0 || json.Unmarshal([]byte(stdout), &version) != nil || version.Profile != companioncli.VersionProfile || version.Binary != name ||
			version.Version != companioncli.ExperimentalVersion || version.Commit == "" || version.CommitState == "" {
			t.Fatalf("%s --version --json: exit %d %q", name, code, stdout)
		}
	}
}

// CAP-V0-701: corvint-console refuses a positional argument before it reads a
// repository or listens, and corvint-update refuses an unknown action with its
// usage and exit 2; its usage names every action, --allow-network and
// --component (V1-3499).
func TestCAPV0701CompanionsRefuseStrayArgumentsFirst(t *testing.T) {
	bin := buildContractCompanions(t)
	outside := t.TempDir()
	code, stdout, stderr := runContractCompanion(t, filepath.Join(bin, "corvint-console"), outside, "-addr", "127.0.0.1:0", "help", "--json", "extra")
	if code != 2 || stdout != "" || !strings.Contains(stderr, `unexpected argument "help"`) || strings.Contains(stderr, "Git repository") {
		t.Fatalf("console stray positional: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	code, stdout, stderr = runContractCompanion(t, filepath.Join(bin, "corvint-update"), outside, "bogus")
	if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "usage: corvint-update") {
		t.Fatalf("update unknown action: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	_, usage, _ := runContractCompanion(t, filepath.Join(bin, "corvint-update"), outside, "--help")
	for _, word := range []string{"check", "apply", "rollback", "--allow-network", "--component core|tasks"} {
		if !strings.Contains(usage, word) {
			t.Fatalf("corvint-update usage lacks %q", word)
		}
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("a refusal wrote %v (%v)", entries, err)
	}
}
