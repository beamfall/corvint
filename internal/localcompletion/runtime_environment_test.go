package localcompletion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeEnvironmentStripsExecutableOverrides(t *testing.T) {
	root, key, plan := fixture(t, []string{"/usr/bin/true"})
	repo := beginFixture(t, root, key, plan)
	if err := os.Mkdir(filepath.Join(root, "script"), 0700); err != nil {
		t.Fatal(err)
	}
	body := "[ -z \"${CORVINT_BIN+x}${DOGFOOD_POISON+x}\" ] || exit 91\nprintf clean"
	if err := os.WriteFile(filepath.Join(root, "script", "environment.sh"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORVINT_BIN", "poison")
	t.Setenv("DOGFOOD_POISON", "poison")
	result := repo.runScript(context.Background(), "environment.sh", plan.Base, nil)
	if result.ExitStatus != 0 || string(result.Stdout) != "clean" {
		t.Fatalf("child environment: %#v", result)
	}
}
