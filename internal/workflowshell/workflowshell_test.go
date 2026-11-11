// SPDX-License-Identifier: Apache-2.0
package workflowshell

import (
	"reflect"
	"testing"
)

// TestCINV08426ArgvFollowsTheDeclaredShell_CINV08426: no declaration is GitHub's `bash -e`,
// `shell: bash` adds pipefail as GitHub does, `sh` is `sh -e`, and a step-level shell or another
// interpreter is refused rather than guessed.
func TestCINV08426ArgvFollowsTheDeclaredShell_CINV08426(t *testing.T) {
	for name, tc := range map[string]struct {
		workflow string
		want     []string
		refuse   bool
	}{
		"none":         {"name: CI\non: push\njobs:\n  a:\n    steps:\n      - run: echo\n", []string{"bash", "-e"}, false},
		"bash":         {"name: CI\ndefaults:\n  run:\n    shell: bash\njobs:\n  a:\n    steps:\n      - run: echo\n", []string{"bash", "--noprofile", "--norc", "-eo", "pipefail"}, false},
		"quoted bash":  {"defaults:\n  run:\n    shell: \"bash\" # pipefail\n", []string{"bash", "--noprofile", "--norc", "-eo", "pipefail"}, false},
		"sh":           {"defaults:\n  run:\n    shell: sh\n", []string{"sh", "-e"}, false},
		"other":        {"defaults:\n  run:\n    shell: pwsh\n", nil, true},
		"step shell":   {"defaults:\n  run:\n    shell: bash\njobs:\n  a:\n    steps:\n      - shell: sh\n        run: echo\n", nil, true},
		"job defaults": {"jobs:\n  a:\n    defaults:\n      run:\n        shell: bash\n    steps:\n      - run: echo\n", nil, true},
		"comment only": {"# defaults:\n#   run:\n#     shell: bash\n", []string{"bash", "-e"}, false},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Argv([]byte(tc.workflow))
			if (err != nil) != tc.refuse || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Argv = %q, %v; want %q refuse=%v", got, err, tc.want, tc.refuse)
			}
		})
	}
}
