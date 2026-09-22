package doccompiler_test

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/doccompiler"
)

func TestCompilerExportsOnlyPlanning(t *testing.T) {
	compilerType := reflect.TypeOf(doccompiler.New())
	if compilerType.NumMethod() != 1 {
		t.Fatalf("exported Compiler methods = %d, want exactly Plan", compilerType.NumMethod())
	}
	method := compilerType.Method(0)
	if method.Name != "Plan" {
		t.Fatalf("exported Compiler method = %q, want Plan", method.Name)
	}
}
