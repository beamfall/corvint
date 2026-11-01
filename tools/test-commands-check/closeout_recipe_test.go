package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// closeoutRecipeCheck is the dogfood-closeout recipe check of
// script/dogfood-cache_test.sh: its awk extraction line and its expected
// recipe.
var closeoutRecipeCheck = regexp.MustCompile(`(?m)^(recipe=\$\(awk '/\^dogfood-closeout:\$?/ .*)\n\[\[ \$recipe == '([^']*)' \]\]`)

// closeoutRecipe runs the shell test's own extraction line against
// root/Makefile and returns the recipe it prints.
func closeoutRecipe(t *testing.T, bash, extraction, root string) string {
	t.Helper()
	command := exec.Command(bash, "-c", extraction+"\nprintf '%s' \"$recipe\"")
	command.Env = append(os.Environ(), "source_root="+root)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("extraction: %v", err)
	}
	return string(output)
}

// TestASDV0946CloseoutRecipeMatchesShellPin (ASD-V0-946, V1-4058): no CI job
// runs make shell-test, so a dogfood-closeout recipe edit that breaks
// script/dogfood-cache_test.sh reached main red (796520b19). This package reads
// the Makefile and script/, so corvint affected and CI's Go shards select it
// for an edit to either side; it runs the shell test's own extraction against
// the Makefile and fails when the recipe drifts from the shell pin.
func TestASDV0946CloseoutRecipeMatchesShellPin(t *testing.T) {
	root := requireMake(t)
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is not on PATH")
	}
	if _, err := exec.LookPath("awk"); err != nil {
		t.Skip("awk is not on PATH")
	}
	source, err := os.ReadFile(filepath.Join(root, "script", "dogfood-cache_test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	match := closeoutRecipeCheck.FindSubmatch(source)
	if match == nil {
		t.Fatal("script/dogfood-cache_test.sh no longer has the dogfood-closeout recipe check this test runs")
	}
	extraction, want := string(match[1]), string(match[2])
	if got := closeoutRecipe(t, bash, extraction, root); got != want {
		t.Fatalf("the Makefile dogfood-closeout recipe reads %q under script/dogfood-cache_test.sh's extraction, which expects %q: update the recipe or the shell pin in the same change", got, want)
	}
	// A recipe step the pin does not name is caught.
	makefile, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	step := "\t@$(MAKE) --no-print-directory dogfood-finish\n"
	if !strings.Contains(string(makefile), step) {
		t.Fatalf("the Makefile no longer has the recipe step %q", step)
	}
	drifted := t.TempDir()
	body := strings.Replace(string(makefile), step, step+"\t@$(MAKE) --no-print-directory batch-drift\n", 1)
	if err := os.WriteFile(filepath.Join(drifted, "Makefile"), []byte(body), 0o666); err != nil {
		t.Fatal(err)
	}
	if got := closeoutRecipe(t, bash, extraction, drifted); got == want {
		t.Fatalf("an added recipe step still reads %q", got)
	}
}
