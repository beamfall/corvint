package closureguard

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The guards below keep each experimental analyzer candidate, and the local
// admin console, out of the core binary's dependency closure. They moved here
// from the candidates' own packages (V1-5071) unchanged in what they assert.

// TestCoreHasNoDependencyEdge is the containment proof: Core must not reach the
// unregistered Rust candidate, so the family cannot become selectable by
// accident.
func TestCoreHasNoDependencyEdge(t *testing.T) {
	_, edgeFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	edgeRoot := filepath.Dir(filepath.Dir(filepath.Dir(edgeFile)))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "list", "-deps", "./cmd/corvint")
	command.Dir = edgeRoot
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v: %s", err, stderr.String())
	}
	for _, banned := range []string{"internal/analyzerrust", "cmd/corvint-analyzer-rust"} {
		if strings.Contains(string(output), banned) {
			t.Errorf("Core reaches %s; the candidate must stay unreachable from Core", banned)
		}
	}
}

// TestCandidateClosureIsSelfContained pins that the Rust candidate executable
// reaches only its own package, which is what keeps its ACP-009 measurement
// stable.
func TestCandidateClosureIsSelfContained(t *testing.T) {
	_, closureFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	closureRoot := filepath.Dir(filepath.Dir(filepath.Dir(closureFile)))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "list", "-deps", "./cmd/corvint-analyzer-rust")
	command.Dir = closureRoot
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v: %s", err, stderr.String())
	}
	firstParty := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.HasPrefix(line, "github.com/Beamfall/corvint") {
			firstParty = append(firstParty, line)
		}
	}
	want := []string{
		"github.com/Beamfall/corvint/internal/analyzerrust",
		"github.com/Beamfall/corvint/cmd/corvint-analyzer-rust",
	}
	if len(firstParty) != len(want) {
		t.Fatalf("first-party closure=%v want=%v", firstParty, want)
	}
	for _, expected := range want {
		found := false
		for _, actual := range firstParty {
			if actual == expected {
				found = true
			}
		}
		if !found {
			t.Errorf("closure missing %s: %v", expected, firstParty)
		}
	}
}

// TestCandidateIsUnselectedByCore: the shader candidate's invariant is its
// absence from the core dependency closure.
func TestCandidateIsUnselectedByCore(t *testing.T) {
	_, unselectedFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	unselectedRoot := filepath.Dir(filepath.Dir(filepath.Dir(unselectedFile)))
	command := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/corvint")
	command.Dir = unselectedRoot
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	dependencies, err := command.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v: %s", err, stderr.String())
	}
	if strings.Contains(string(dependencies), "internal/analyzershader") || strings.Contains(string(dependencies), "corvint-analyzer-shader") {
		t.Fatalf("Core selected shader candidate: %s", dependencies)
	}
}

// TestCandidateDependencyClosureHasNoAmbientChannels: the structured-data
// candidate's closure links no process, network, plugin or cgo channel.
func TestCandidateDependencyClosureHasNoAmbientChannels(t *testing.T) {
	_, ambientFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	ambientRoot := filepath.Dir(filepath.Dir(filepath.Dir(ambientFile)))
	command := exec.Command("go", "list", "-deps", "./cmd/corvint-analyzer-structured-data")
	command.Dir = ambientRoot
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v: %s", err, stderr.String())
	}
	for _, forbidden := range []string{"os/exec", "net", "net/http", "plugin", "runtime/cgo"} {
		if bytes.Contains(output, []byte(forbidden+"\n")) {
			t.Fatalf("ambient dependency %q in closure", forbidden)
		}
	}
}

// TestCoreParentCandidateByteIdentity: the structured candidate's invariant is
// that the core binary never selects it. In the integration checkpoint the
// core legitimately differs from the frozen base (reviewed runtime work), so
// the former byte-identity comparison against 718dfc7 is asserted as a
// dependency-closure property instead.
func TestCoreParentCandidateByteIdentity(t *testing.T) {
	_, parentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	parentRoot := filepath.Dir(filepath.Dir(filepath.Dir(parentFile)))
	command := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/corvint")
	command.Dir = parentRoot
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOFLAGS=")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	dependencies, err := command.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v: %s", err, stderr.String())
	}
	for _, forbidden := range []string{"internal/analyzerstructured", "corvint-analyzer-structured-data"} {
		if strings.Contains(string(dependencies), forbidden) {
			t.Fatalf("Core selected structured candidate via %s", forbidden)
		}
	}
}

// TestCoreDependencyClosureExcludesPythonCandidate pins PNC-001: Core cannot
// reach the separately buildable Python candidate.
func TestCoreDependencyClosureExcludesPythonCandidate(t *testing.T) {
	requirement := struct{ name string }{name: "PNC-001 Core unreachable separately buildable candidate"}
	if requirement.name == "" {
		t.Fatal("missing requirement claim")
	}
	_, pythonFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	pythonRoot := filepath.Dir(filepath.Dir(filepath.Dir(pythonFile)))
	command := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/corvint")
	command.Dir = pythonRoot
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list Core dependencies: %v\n%s", err, output)
	}
	for _, forbidden := range []string{"github.com/Beamfall/corvint/internal/analyzerpython"} {
		if strings.Contains(string(output), forbidden+"\n") {
			t.Fatalf("Core depends on candidate-only package %s", forbidden)
		}
	}
}

// TestConsoleNotLinkedIntoDefaultBinary pins LAC-V0-001: the default local
// product is one native-Go binary with no UI. A loopback bind and
// statelessness test alone would still pass if cmd/corvint linked the console
// package, so this checks the dependency edge directly.
func TestConsoleNotLinkedIntoDefaultBinary(t *testing.T) {
	_, consoleFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	consoleRoot := filepath.Dir(filepath.Dir(filepath.Dir(consoleFile)))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "list", "-deps", "./cmd/corvint")
	command.Dir = consoleRoot
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v: %s", err, stderr.String())
	}
	if strings.Contains(string(output), "internal/console") {
		t.Error("cmd/corvint reaches internal/console; the default binary must not link the console")
	}
}
