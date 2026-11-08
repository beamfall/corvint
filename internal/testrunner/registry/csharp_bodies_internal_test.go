// SPDX-License-Identifier: Apache-2.0

package registry

import "testing"

// With no registered reader, the registry claims no C# region and the dotnet
// mutation operator refuses instead of reporting an empty population.
func TestCSharpBodiesFailClosedWithoutReader(t *testing.T) {
	saved := csharpBodiesReader.Swap(nil)
	t.Cleanup(func() { csharpBodiesReader.Store(saved) })
	if bodies, ok := csharpBodies([]byte("class C {\n  void M()\n  {\n    x();\n  }\n}\n")); ok || bodies != nil {
		t.Fatalf("unregistered reader claimed regions: %v %v", bodies, ok)
	}
	if _, err := DotnetBodyMutants([]byte("class C {\n  int M()\n  {\n    return 1;\n  }\n}\n")); err == nil {
		t.Fatal("mutation accepted without a C# reader")
	}
}
