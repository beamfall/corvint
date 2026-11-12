package closureguard

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestLTAV0009LiveContextPathCannotReachTheLedgers (moved from
// internal/contextindex, V1-5071): the live context index never links the
// bounded learning ledgers or the slot learner.
func TestLTAV0009LiveContextPathCannotReachTheLedgers(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	command := exec.Command("go", "list", "-deps", "./internal/contextindex")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, output)
	}
	for _, forbidden := range []string{"/internal/observations", "/internal/slotlearn"} {
		if strings.Contains(string(output), forbidden+"\n") {
			t.Fatalf("contextindex depends on %s", forbidden)
		}
	}
}

// TestNGIV0988ImpactBinariesLinkReader (moved from internal/swiftimpact,
// V1-5071): every binary that links the Swift model and can reach path impact
// links this reading, so their impact answers agree. corvint-docs-mcp and
// corvint-web-flows never reach path impact, so their unreachable registrations
// were removed (docs/build-log/2026-11-09-layer-tests-targets.md). The
// corvint-tasks closure half left with Corvint Tasks (decision 0594).
func TestNGIV0988ImpactBinariesLinkReader(t *testing.T) {
	const module = "github.com/Beamfall/corvint/"
	linked := []string{"corvint", "corvint-mcp", "corvint-test-validity-mcp"}
	args := []string{"list", "-f", "{{.ImportPath}} {{join .Deps \" \"}}"}
	for _, name := range linked {
		args = append(args, module+"cmd/"+name)
	}
	cmd := exec.Command("go", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, stderr.String())
	}
	deps := map[string]map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		set := map[string]bool{}
		for _, dep := range fields[1:] {
			set[dep] = true
		}
		deps[strings.TrimPrefix(fields[0], module+"cmd/")] = set
	}
	for _, name := range linked {
		if !deps[name][module+"internal/swiftimpact"] {
			t.Errorf("%s does not link internal/swiftimpact", name)
		}
	}
}
