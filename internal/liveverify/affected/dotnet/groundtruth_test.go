package dotnet_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected/dotnet"
)

// TestRealRepositoryDiscoveryMatchesRunnerAddressableMethods is the opt-in
// ground-truth gate for a real C# test project. The expected count must come
// from runner discovery collapsed to distinct FullyQualifiedName methods.
func TestRealRepositoryDiscoveryMatchesRunnerAddressableMethods(t *testing.T) {
	root := os.Getenv("CORVINT_DOTNET_REAL_REPO")
	if root == "" {
		t.Skip("CORVINT_DOTNET_REAL_REPO is not set")
	}
	want, err := strconv.Atoi(os.Getenv("CORVINT_DOTNET_EXPECT_METHODS"))
	if err != nil || want < 1 {
		t.Fatal("CORVINT_DOTNET_EXPECT_METHODS must be a positive integer")
	}
	result, err := dotnet.New().Units(root)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, unit := range result.Units {
		_, filter, ok := dotnet.Address(unit.ID)
		if !ok {
			continue
		}
		for _, predicate := range strings.Split(filter, "|") {
			method, ok := strings.CutPrefix(predicate, "FullyQualifiedName=")
			if ok {
				found[method] = true
			}
		}
	}
	t.Logf("real repository: discovered %d/%d runner-addressable test methods; frontier=%v", len(found), want, result.Frontier)
	if len(found) != want {
		t.Fatalf("discovered %d/%d runner-addressable test methods", len(found), want)
	}
	assembly := os.Getenv("CORVINT_DOTNET_TEST_ASSEMBLY")
	if assembly == "" {
		return
	}
	diagnostic := filepath.Join(t.TempDir(), "vstest-discovery.log")
	command := exec.Command("dotnet", "vstest", assembly, "--ListTests", "--Diag:"+diagnostic)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("runner discovery: %v\n%s", err, output)
	}
	diagnosticOutput, err := os.ReadFile(diagnostic)
	if err != nil {
		t.Fatalf("read runner diagnostics: %v", err)
	}
	truth, err := parseRunnerMethods(string(diagnosticOutput))
	if err != nil {
		t.Fatal(err)
	}
	if !sameMethods(found, truth) {
		t.Fatalf("static methods differ from runner: static-only=%v runner-only=%v", difference(found, truth), difference(truth, found))
	}
}

var fullyQualifiedNamePattern = regexp.MustCompile(`"FullyQualifiedName":("(?:\\.|[^"\\])*")`)

func TestParseRunnerMethodsCollapsesDataRowsToFQNMethods(t *testing.T) {
	methods, err := parseRunnerMethods(`{"FullyQualifiedName":"Example.Tests.Unit.Works(\"case\")"}`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"Example.Tests.Unit.Works": true}
	if !sameMethods(methods, want) {
		t.Fatalf("methods = %v, want %v", methods, want)
	}
}

func parseRunnerMethods(output string) (map[string]bool, error) {
	methods := map[string]bool{}
	for _, match := range fullyQualifiedNamePattern.FindAllStringSubmatch(output, -1) {
		var method string
		if err := json.Unmarshal([]byte(match[1]), &method); err != nil {
			return nil, err
		}
		if cut := strings.IndexByte(method, '('); cut >= 0 {
			method = method[:cut]
		}
		methods[method] = true
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("runner diagnostics contain no FullyQualifiedName values")
	}
	return methods, nil
}

func sameMethods(left, right map[string]bool) bool {
	return len(left) == len(right) && len(difference(left, right)) == 0
}

func difference(left, right map[string]bool) []string {
	values := make([]string, 0)
	for value := range left {
		if !right[value] {
			values = append(values, value)
		}
	}
	sort.Strings(values)
	return values
}
