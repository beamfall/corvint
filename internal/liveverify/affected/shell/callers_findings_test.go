package shell

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestForeignCallerFindingsMatchThePlugin_TSP_V0_920 pins one caller rule
// for the shell plugin and the repository declaration test (V1-3899): a shell
// file that builds a name the script may be counts, a .txt file does not, and
// ForeignCallerFindings is empty exactly when the plugin accepts the
// declaration. A Go comment naming the script (V1-3913) is reported as the
// offending file:line rather than only FrontierForeignCallersInvalid.
func TestForeignCallerFindingsMatchThePlugin_TSP_V0_920(t *testing.T) {
	base := map[string]string{
		"run/run.sh":    "#!/bin/sh\n. \"$ENVFILE\"\n",
		"use/use.sh":    "\"$here/$name.sh\" x\n",
		"notes/run.txt": "run run.sh by hand\n",
		"Makefile":      "check:\n\tsh run/run.sh\n",
	}
	declare := func(callers string) string {
		return `{"profile":"corvint-shell-foreign-callers/0","scripts":{"run/run.sh":{"callers":[` + callers + `],"reason":"computed paths"}}}`
	}
	for _, tc := range []struct {
		name        string
		declaration string
		extra       map[string]string
		want        []string
	}{
		{name: "built name and txt", declaration: declare(`"Makefile","use/use.sh"`)},
		{name: "built name missing", declaration: declare(`"Makefile"`), want: []string{"use/use.sh: builds a name run/run.sh may be"}},
		{name: "txt listed is stale", declaration: declare(`"Makefile","notes/run.txt","use/use.sh"`), want: []string{"notes/run.txt: listed for run/run.sh"}},
		{
			name: "go comment", declaration: declare(`"Makefile","use/use.sh"`),
			extra: map[string]string{"go.mod": "module example.test/m\n", "lane/lane.go": "package lane\n\n// Tests run through run.sh on the lane host.\nfunc Lane() {}\n"},
			want:  []string{"lane/lane.go:3: names run/run.sh but is not a declared caller or mention"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for relative, content := range base {
				write(t, root, relative, content)
			}
			for relative, content := range tc.extra {
				write(t, root, relative, content)
			}
			write(t, root, affected.ShellForeignCallersPath, tc.declaration)
			source := affected.DiskSourceContext(context.Background(), root)
			findings, err := ForeignCallerFindings(source)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != len(tc.want) {
				t.Fatalf("findings %v, want %d naming %v", findings, len(tc.want), tc.want)
			}
			for index, finding := range findings {
				if !strings.HasPrefix(finding.String(), tc.want[index]) {
					t.Errorf("finding %q, want prefix %q", finding, tc.want[index])
				}
			}
			invalid := slices.Contains(units(t, root).Frontier, FrontierForeignCallersInvalid)
			if invalid != (len(findings) > 0) {
				t.Errorf("plugin invalid %v but %d findings: the rules disagree", invalid, len(findings))
			}
		})
	}
}
