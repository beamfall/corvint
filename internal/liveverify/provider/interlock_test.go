package provider

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCorvintGoDependencyClosureExcludesUncontainedLiveTestExecution(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate module root")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{.ImportPath}}", "./cmd/corvint")
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list corvint dependencies: %v\n%s", err, output)
	}
	forbidden := map[string]struct{}{
		"github.com/Beamfall/corvint/internal/liveverify/gorunner": {},
		"github.com/Beamfall/corvint/internal/liveverify/provider": {},
	}
	for _, dependency := range strings.Fields(string(output)) {
		if _, found := forbidden[dependency]; found {
			t.Errorf("cmd/corvint reaches uncontained live-test execution package %s", dependency)
		}
	}
}
