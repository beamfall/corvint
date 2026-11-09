package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestCAXV0023VerifyStableNonRegularEnvelope checks CAX-V0-023 on the native
// verify-stable command over the shared cem-non-regular/1 vector: with
// --non-regular unknown the result is wrapped as {nonRegular,
// nonRegularProfile, stableVerification} with the rows EXPECTED.json gives
// and an ACCEPT; without it the closed result is printed bare and REJECTs;
// any other value is refused invalid-arguments with the bare result, as the
// portable verify-stable-canonical does.
func TestCAXV0023VerifyStableNonRegularEnvelope(t *testing.T) {
	dir, err := filepath.Abs("../../../interop/cem01-go/testdata/non-regular")
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		BaseRevision   string           `json:"baseRevision"`
		TargetRevision string           `json:"targetRevision"`
		NonRegular     []map[string]any `json:"nonRegular"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "EXPECTED.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	run := func(repository string, extra ...string) (int, map[string]any) {
		t.Helper()
		argv := append([]string{"cem", "verify-stable", "--repository", repository, "--map", filepath.Join(dir, "stable.json"),
			"--expected-base", want.BaseRevision, "--target", want.TargetRevision, "--artifacts", dir}, extra...)
		var out, errout bytes.Buffer
		exit, handled := RunStableInvocation(context.Background(), argv, &out, &errout)
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || !handled {
			t.Fatalf("handled %v, stdout %q: %v", handled, out.String(), err)
		}
		return exit, result
	}
	if exit, result := run("/nonexistent-repository", "--non-regular", "follow"); exit != 2 || result["code"] != "invalid-arguments" || result["stage"] != "arguments" {
		t.Fatalf("--non-regular follow: exit %d result %v", exit, result)
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	repo, target := externalRecipeRepository(t, filepath.Join(dir, "repository.recipe.json"))
	if target != want.TargetRevision {
		t.Fatalf("recipe target %s, want %s", target, want.TargetRevision)
	}
	exit, envelope := run(repo, "--non-regular", "unknown")
	verification, _ := envelope["stableVerification"].(map[string]any)
	rows, _ := json.Marshal(envelope["nonRegular"])
	wantRows, _ := json.Marshal(want.NonRegular)
	if exit != 0 || envelope["nonRegularProfile"] != "cem-non-regular/1" || verification["outcome"] != "ACCEPT" || !bytes.Equal(rows, wantRows) {
		t.Fatalf("opted-in verify-stable: exit %d envelope %v", exit, envelope)
	}
	if exit, plain := run(repo); exit == 0 || plain["outcome"] != "REJECT" || plain["stableVerification"] != nil {
		t.Fatalf("plain verify-stable: exit %d result %v", exit, plain)
	}
}
