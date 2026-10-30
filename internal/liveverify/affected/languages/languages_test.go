package languages

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected/shell"
)

// candidateSuffixes is a broad list of source-file endings probed against every
// non-shell plugin, so a suffix a new plugin claims cannot be missed silently.
var candidateSuffixes = []string{
	".go", ".py", ".pyi", ".rb", ".rake", ".rs", ".swift", ".kt", ".kts", ".java", ".gradle",
	".groovy", ".scala", ".cs", ".fs", ".fsx", ".vb", ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx",
	".mts", ".cts", ".vue", ".svelte", ".c", ".h", ".cc", ".cpp", ".cxx", ".c++", ".hh", ".hpp",
	".hxx", ".h++", ".inl", ".ipp", ".tpp", ".m", ".mm", ".php", ".pl", ".lua", ".dart", ".ex",
	".exs", ".erl", ".hs", ".clj", ".r", ".jl", ".zig", ".nim",
}

// TestShellForeignSuffixesMatchTheOtherPlugins_NSAV0012 keeps the shell plugin's
// foreign-language list equal to what the other registered plugins own.
func TestShellForeignSuffixesMatchTheOtherPlugins_NSAV0012(t *testing.T) {
	listed := map[string]bool{}
	for _, suffix := range shell.ForeignSourceSuffixes {
		listed[suffix] = true
	}
	ownedBy := func(name string) string {
		for _, language := range All() {
			if language.Name() != "shell" && language.Owns(name) {
				return language.Name()
			}
		}
		return ""
	}
	for _, suffix := range shell.ForeignSourceSuffixes {
		if ownedBy("pkg/sub/file"+suffix) == "" {
			t.Errorf("shell lists %s but no other plugin owns it", suffix)
		}
	}
	for _, suffix := range candidateSuffixes {
		if owner := ownedBy("pkg/sub/file" + suffix); owner != "" && !listed[suffix] {
			t.Errorf("%s owns %s but shell.ForeignSourceSuffixes omits it", owner, suffix)
		}
	}
	for _, suffix := range shell.Suffixes {
		if owner := ownedBy("pkg/sub/file" + suffix); owner != "" && !strings.HasPrefix(owner, "shell") {
			t.Errorf("%s also owns shell suffix %s", owner, suffix)
		}
	}
}
