// SPDX-License-Identifier: Apache-2.0

package golang_test

import (
	"context"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// AFP-V0-3661, AFP-V0-3664, V1-3999 fix round 2: a NAME=value entry
// re-exports only the raw environment value. A variable reassigned to a
// transform of its own value, directly or through another variable, carries
// a location value into the child (P1), and a command whose Environ is read
// gives back its Dir as PWD (P3). Each is held by rule (d), so a body-only
// edit of an unrelated package selects the probe's test. Controls: the raw
// value re-exported, directly or through a copy, selects nothing.
func TestLocationReexportHeld_AFPV03661(t *testing.T) {
	const shell = "c := exec.Command(\"sh\", \"-c\", `ls -R \"$ROOT\"`)\nc.Env = []string{\"ROOT=\" + v}\n_ = c.Run()\n"
	for _, test := range []struct {
		name, body string
		held       bool
	}{
		{"self-reassigned value", "v := os.Getenv(\"ROOT\")\nv = filepath.Dir(filepath.Dir(v))\n" + shell, true},
		{"self-extended value", "v := os.Getenv(\"ROOT\")\nv = v + \"/..\"\n" + shell, true},
		{"self-reassigned lookup", "v, _ := os.LookupEnv(\"ROOT\")\nv = filepath.Dir(v)\n" + shell, true},
		{"two-variable cycle", "v := os.Getenv(\"ROOT\")\nw := v\nv = filepath.Dir(w)\n" + shell, true},
		{"command directory read back through Environ", "_, file, _, _ := runtime.Caller(0)\n" +
			"c := exec.Command(\"go\", \"list\", \"-deps\", \"./cmd/tool\")\nc.Dir = filepath.Dir(filepath.Dir(file))\n" +
			"for _, e := range c.Environ() {\nif dir, ok := strings.CutPrefix(e, \"PWD=\"); ok {\n_, _ = os.ReadDir(dir)\n}\n}\n", true},
		{"raw value re-exported", "v := os.Getenv(\"ROOT\")\n" + shell, false},
		{"copied raw value re-exported", "u := os.Getenv(\"ROOT\")\nv := u\n" + shell, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFiles(t, root, map[string]string{
				"go.mod":          "module example.test/m\n",
				"lib/lib.go":      "package lib\n\nfunc F() int { return 1 }\n",
				"lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {}\n",
				"probe/probe_test.go": "package probe\n\nimport (\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"runtime\"\n\t\"strings\"\n\t\"testing\"\n)\n\n" +
					"var _, _, _, _, _ = os.Getpid, exec.Command, filepath.Dir, runtime.GOOS, strings.Cut\n\n" +
					"func TestProbe(t *testing.T) {\n" + test.body + "}\n",
			})
			graph, err := affected.Build(context.Background(), root, golang.New())
			if err != nil {
				t.Fatal(err)
			}
			selected := slices.Contains(affected.Select(graph, []string{"lib/lib.go"}).SelectedTests(), "probe/probe_test.go")
			if selected != test.held {
				t.Fatalf("body-only edit of lib/lib.go selected probe = %v, want %v", selected, test.held)
			}
		})
	}
}
