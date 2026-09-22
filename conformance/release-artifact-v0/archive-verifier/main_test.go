package main

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/conformance/release-artifact-v0/archivewire"
)

func TestVerifierCommandCannotImportAssemblerExecutionOrNetwork(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			name, _ := strconv.Unquote(imported.Path.Value)
			if strings.Contains(name, "archivebuild") || name == "os/exec" || strings.HasPrefix(name, "net") {
				t.Fatalf("%s imports forbidden dependency %s", entry.Name(), name)
			}
		}
	}
}

func TestMaterializeEnforcesRawFileBounds(t *testing.T) {
	directory := t.TempDir()
	paths := make([]string, 5)
	for index := range paths {
		parent := filepath.Join(directory, strconv.Itoa(index))
		if err := os.Mkdir(parent, 0o700); err != nil {
			t.Fatal(err)
		}
		paths[index] = filepath.Join(parent, "candidate")
		if err := os.WriteFile(paths[index], []byte("1234"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	request := archivewire.Request{ArchiveAPath: paths[0], ArchiveBPath: paths[1], BinaryAPath: paths[2], BinaryBPath: paths[3], LooseGateBinaryPath: paths[4], MaximumArchiveBytes: 3, MaximumContentBytes: 4}
	if _, err := materialize(request); err == nil {
		t.Fatal("oversized archive accepted")
	}
	request.MaximumArchiveBytes = 4
	if _, err := materialize(request); err != nil {
		t.Fatalf("bounded files rejected: %v", err)
	}
	request.MaximumArchiveBytes = archivewire.HardMaxArchiveBytes + 1
	if _, err := materialize(request); err == nil || !strings.Contains(err.Error(), "resource-limit") {
		t.Fatalf("caller-controlled archive maximum accepted: %v", err)
	}
	request.MaximumArchiveBytes = 4
	request.MaximumContentBytes = archivewire.HardMaxContentBytes + 1
	if _, err := materialize(request); err == nil || !strings.Contains(err.Error(), "resource-limit") {
		t.Fatalf("caller-controlled content maximum accepted: %v", err)
	}
	request.MaximumContentBytes = 4
	request.ArchiveBPath = request.ArchiveAPath
	if _, err := materialize(request); err == nil || !strings.Contains(err.Error(), "not-distinct") {
		t.Fatalf("identical pair path accepted: %v", err)
	}
}

func TestMaterializeRejectsAliasedOrSymlinkedPairs(t *testing.T) {
	directory := t.TempDir()
	firstDirectory, secondDirectory := filepath.Join(directory, "a"), filepath.Join(directory, "b")
	if err := os.Mkdir(firstDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(secondDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(firstDirectory, "candidate")
	second := filepath.Join(secondDirectory, "candidate")
	if err := os.WriteFile(first, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, second); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readDistinctPair(first, second, 1); err == nil || !strings.Contains(err.Error(), "files-not-distinct") {
		t.Fatalf("hard-linked pair accepted: %v", err)
	}
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(first, second); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readDistinctPair(first, second, 1); err == nil {
		t.Fatal("symlinked pair accepted")
	}
}

func TestRunRejectsMalformedOrNoncanonicalRequests(t *testing.T) {
	request := archivewire.Request{Schema: archivewire.RequestSchema}
	canonical, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	canonical = append(canonical, '\n')
	mutations := map[string][]byte{
		"unknown":   []byte(strings.Replace(string(canonical), `"schema":`, `"unknown":1,"schema":`, 1)),
		"duplicate": []byte(strings.Replace(string(canonical), `"schema":`, `"schema":"`+archivewire.RequestSchema+`","schema":`, 1)),
		"trailing":  append(append([]byte(nil), canonical...), ' '),
		"spacing":   []byte("{ \"schema\":\"" + archivewire.RequestSchema + "\"}\n"),
	}
	for name, raw := range mutations {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			requestPath := filepath.Join(directory, "request.json")
			resultPath := filepath.Join(directory, "result.json")
			if err := os.WriteFile(requestPath, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if got := run([]string{"--request", requestPath, "--result", resultPath}); got != 2 {
				t.Fatalf("malformed request exit=%d", got)
			}
			if _, err := os.Stat(resultPath); !os.IsNotExist(err) {
				t.Fatalf("malformed request produced result: %v", err)
			}
		})
	}
}

func TestReasonCodeCannotLeakPaths(t *testing.T) {
	for _, input := range []string{"/tmp/private", `C:\\private`, "UPPER", strings.Repeat("x", 65)} {
		if got := reasonCode(assertionError(input)); got != "verification-failed" {
			t.Fatalf("reason %q leaked as %q", input, got)
		}
	}
	if got := reasonCode(assertionError("checksum-mismatch")); got != "checksum-mismatch" {
		t.Fatalf("closed reason changed: %q", got)
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
