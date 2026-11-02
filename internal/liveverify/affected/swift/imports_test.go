package swift

import "testing"

// TestNGIV0987ImportedModulesReadsCodeOnlyImports: the module imports a
// Swift source declares, with their lines, skipping comments and strings and
// reading attributed and kind-qualified imports (NGI-V0-987).
func TestNGIV0987ImportedModulesReadsCodeOnlyImports(t *testing.T) {
	text := "// import Hidden\nimport Core\n@testable import App\nimport struct Kit.Thing\nlet s = \"import Quoted\"\n/* import Block */\n"
	got := ImportedModules(text)
	want := []ImportedModule{{"Core", 2}, {"App", 3}, {"Kit", 4}}
	if len(got) != len(want) {
		t.Fatalf("ImportedModules = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ImportedModules = %v, want %v", got, want)
		}
	}
	if module, ok := TargetModule("swift:spm:pkg/Core"); !ok || module != "Core" {
		t.Fatalf("TargetModule = %q, %v", module, ok)
	}
	if _, ok := TargetModule("go:pkg"); ok {
		t.Fatal("TargetModule accepted a non-SwiftPM unit")
	}
}
