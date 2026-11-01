package affected_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// TestRepositoryShellReadScopes_TSP_V0_951 checks this repository's
// .corvint/shell-read-scopes.json against the tree: the shell plugin accepts
// it and read-scopes the ratchet test, and every non-standard package the
// test's go build compiles lies in a declared read, so a new dependency of the
// receipt tool fails here instead of going unselected.
func TestRepositoryShellReadScopes_TSP_V0_951(t *testing.T) {
	const ratchet = "script/check-sql-native-ratchets_test.sh"
	root := repositoryRoot(t)
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(affected.ShellReadScopesPath)))
	if err != nil {
		t.Fatal(err)
	}
	var declared struct {
		Tests map[string]struct {
			Reads []string `json:"reads"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(body, &declared); err != nil {
		t.Fatal(err)
	}
	reads := declared.Tests[ratchet].Reads
	if len(reads) == 0 {
		t.Fatalf("%s is not declared: %s", ratchet, body)
	}

	result, err := shell.New().Units(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(result.Frontier, shell.FrontierShellReadScopesInvalid) {
		t.Fatalf("the plugin refused %s: frontier %v", affected.ShellReadScopesPath, result.Frontier)
	}
	for test := range declared.Tests {
		index := slices.IndexFunc(result.Units, func(unit affected.Unit) bool { return unit.ID == "shell:"+test })
		if index < 0 || !result.Units[index].ReadScoped || result.Units[index].UnboundedReads != "" {
			t.Fatalf("%s is not read-scoped: %+v", test, result.Units)
		}
	}

	for _, file := range []string{"Makefile", "go.mod", "go.sum"} {
		if !slices.Contains(reads, file) {
			t.Errorf("%s reads %s but does not declare it", ratchet, file)
		}
	}
	list := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.Dir}}{{end}}", "./cmd/corvint-go-toolchain-receipt")
	list.Dir = root
	list.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	output, err := list.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, directory := range listedDirectories(string(output)) {
		relative, err := filepath.Rel(root, directory)
		if err != nil || strings.HasPrefix(relative, "..") {
			t.Errorf("the receipt tool compiles %s, outside the repository", directory)
			continue
		}
		relative = filepath.ToSlash(relative) + "/"
		if !slices.ContainsFunc(reads, func(entry string) bool { return strings.HasSuffix(entry, "/") && strings.HasPrefix(relative, entry) }) {
			t.Errorf("the receipt tool compiles %s, which %s does not declare for %s", relative, affected.ShellReadScopesPath, ratchet)
		}
	}
}

// listedDirectories splits `go list` output into one directory a line: a
// directory may hold spaces, so only the line ending (LF or CRLF) splits.
func listedDirectories(output string) []string {
	var directories []string
	for _, line := range strings.Split(output, "\n") {
		if line = strings.TrimSuffix(line, "\r"); line != "" {
			directories = append(directories, line)
		}
	}
	return directories
}

// TestListedDirectoriesKeepSpaces_TSP_V0_951 pins that a package directory
// holding a space stays one path, under LF and CRLF line endings.
func TestListedDirectoriesKeepSpaces_TSP_V0_951(t *testing.T) {
	got := listedDirectories("/repo/my dir/pkg\r\n/repo/internal/x\n\n")
	if want := []string{"/repo/my dir/pkg", "/repo/internal/x"}; !slices.Equal(got, want) {
		t.Fatalf("listed %q, want %q", got, want)
	}
}
