package shell

import (
	"slices"
	"testing"
)

// AFP-V0-553 (V1-4170): a wrapper the file invokes through a variable is an
// edge when the variable's default names it through the script's own
// directory (`$(dirname "$0")/run.sh`), when the command word is a
// `${NAME:+...}` alternative, or when it runs inside a `sh -c` command string
// held in a variable. Each subtest mirrors a script in this repository
// (script/codex-review.sh, script/analyzer-schema.sh).
func TestInvocationThroughVariables_AFP_V0_553(t *testing.T) {
	for _, tc := range []struct{ name, script string }{
		{name: "dirname default", script: "heavy=${H-$(dirname \"$0\")/run.sh}\n\"$heavy\" x\n"},
		{name: "alternative command word", script: "heavy=${H-use/run.sh}\nexec ${heavy:+\"$heavy\" --admit-only} tool x\n"},
		{name: "codex-review form", script: "heavy=${H-$(dirname \"$0\")/run.sh}\nexec ${heavy:+\"$heavy\" --admit-only} \"$resolved\" exec -m \"$model\" \"$@\"\n"},
		{name: "command string in a variable", script: "f() {\n    heavy=${H-use/run.sh}\n    [ -z \"$heavy\" ] || [ -x \"$heavy\" ] || heavy=\n    command=${C:-\"$heavy env A=1 go test -run '^T\\$' ./x/\"}\n    report=$(sh -c \"$command\" 2>&1) || true\n}\nf\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "use/run.sh", "echo run\n")
			write(t, root, "use/use.sh", tc.script)
			if use := unitOf(t, units(t, root), "use/use.sh"); !slices.Equal(use.Imports, []string{"shell:use/run.sh"}) {
				t.Errorf("use imports %v, want the use/run.sh edge", use.Imports)
			}
		})
	}
	t.Run("an unrelated substitution stays a wildcard", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "use/run.sh", "echo run\n")
		write(t, root, "use/use.sh", "\"$(pick)/x/$(name).sh\" a\n")
		use := unitOf(t, units(t, root), "use/use.sh")
		if len(use.Imports) != 0 || !slices.Equal(use.Frontier, []string{FrontierForeignInvocation}) {
			t.Errorf("use imports %v frontier %v, want no edge and the foreign frontier", use.Imports, use.Frontier)
		}
	})
}
