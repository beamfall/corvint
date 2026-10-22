package python

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestAFPV0183FromImportNamesReachSubmodules pins that a name imported with
// `from X import name` is tried as the submodule X.name (AFP-V0-183): a change
// to pkg/sub.py selects the test that imports it that way, whether pkg has an
// __init__.py or is a namespace package, and through a relative import too.
func TestAFPV0183FromImportNamesReachSubmodules(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"namespace package": {"pkg/sub.py": "VALUE = 1\n", "tests/test_x.py": "from pkg import sub\n"},
		"regular package":   {"pkg/__init__.py": "\n", "pkg/sub.py": "VALUE = 1\n", "tests/test_x.py": "from pkg import other, sub as s  # note\n"},
		"parenthesised":     {"pkg/__init__.py": "\n", "pkg/sub.py": "VALUE = 1\n", "tests/test_x.py": "from pkg import (sub)\n"},
		"relative":          {"pkg/__init__.py": "\n", "pkg/sub.py": "VALUE = 1\n", "pkg/user.py": "from . import sub\n", "tests/test_x.py": "import pkg.user\n"},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			for relative, body := range files {
				write(t, root, relative, body)
			}
			graph, err := affected.Build(context.Background(), root, New())
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			plan := affected.Select(graph, []string{"pkg/sub.py"})
			if got := plan.SelectedTests(); !slices.Equal(got, []string{"tests/test_x.py"}) {
				t.Fatalf("selected=%v unknown=%+v, want tests/test_x.py", got, plan.Unknown)
			}
		})
	}
}
