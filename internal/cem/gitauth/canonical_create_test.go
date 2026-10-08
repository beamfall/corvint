package gitauth

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func canonicalCreateGitInput(t *testing.T, root, stdin string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	command.Dir = root
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+t.TempDir(), "XDG_CONFIG_HOME="+t.TempDir())
	command.Stdin = strings.NewReader(stdin)
	out, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func canonicalCreateTree(t *testing.T, root string, entries ...string) string {
	t.Helper()
	return canonicalCreateGitInput(t, root, strings.Join(entries, ""), "mktree")
}

func canonicalCreateRepo(t *testing.T) (root, base, target string) {
	t.Helper()
	root = t.TempDir()
	gitCmd(t, root, "init", "-q", "--object-format=sha1")
	canonicalCreateGitInput(t, root, "", "config", "user.name", "Fixture")
	canonicalCreateGitInput(t, root, "", "config", "user.email", "fixture@invalid")
	blob := canonicalCreateGitInput(t, root, "created\n", "hash-object", "-w", "--stdin")
	keep := canonicalCreateGitInput(t, root, "keep\n", "hash-object", "-w", "--stdin")
	inner := canonicalCreateGitInput(t, root, "inner\n", "hash-object", "-w", "--stdin")
	emptyTree := canonicalCreateTree(t, root)
	corruptLoose(t, root, emptyTree, "tree", nil)
	seedTree := canonicalCreateTree(t, root, "100644 blob "+keep+"\tseed\n")
	seed := gitCmd(t, root, "commit-tree", seedTree, "-m", "seed")
	nonempty := canonicalCreateTree(t, root, "100644 blob "+inner+"\tchild.txt\n")
	sidecarBase := canonicalCreateTree(t, root, "100644 blob "+keep+"\tkeep.txt\n")
	baseTree := canonicalCreateTree(t, root,
		"040000 tree "+emptyTree+"\tempty\n",
		"040000 tree "+nonempty+"\tdir\n",
		"040000 tree "+sidecarBase+"\t.corvint\n",
		"160000 commit "+seed+"\tsub\n",
	)
	base = gitCmd(t, root, "commit-tree", baseTree, "-m", "base")
	sidecarTarget := canonicalCreateTree(t, root,
		"100644 blob "+keep+"\tkeep.txt\n",
		"100644 blob "+blob+"\tnew.txt\n",
	)
	nested := canonicalCreateTree(t, root, "100644 blob "+blob+"\tnew.txt\n")
	targetTree := canonicalCreateTree(t, root,
		"100644 blob "+blob+"\tempty\n",
		"100644 blob "+blob+"\tdir\n",
		"040000 tree "+sidecarTarget+"\t.corvint\n",
		"040000 tree "+nested+"\tnested\n",
		"100644 blob "+blob+"\tordinary.txt\n",
		"100644 blob "+blob+"\tsub\n",
	)
	target = gitCmd(t, root, "commit-tree", targetTree, "-p", base, "-m", "target")
	gitCmd(t, root, "update-ref", "refs/heads/main", target)
	return root, base, target
}

func TestCanonicalDiffWithCreateDestinationsProvesBaseEntries(t *testing.T) {
	root, base, target := canonicalCreateRepo(t)
	repo := open(t, root)
	ctx := context.Background()
	legacy, err := repo.CanonicalDiff(ctx, base, target)
	if err != nil {
		t.Fatal(err)
	}
	got, proofs, err := repo.CanonicalDiffWithCreateDestinations(ctx, base, target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, legacy) {
		t.Fatalf("proof diff changed legacy bytes\n--- proof ---\n%s\n--- legacy ---\n%s", got, legacy)
	}
	for _, path := range []string{"ordinary.txt", "nested/new.txt", ".corvint/new.txt"} {
		if entry, ok := proofs[path]; !ok || entry.OID != "" {
			t.Fatalf("%s proof = %+v ok=%v, want authenticated absence", path, entry, ok)
		}
	}
	for path, typ := range map[string]string{"empty": "tree", "dir": "tree", "sub": "commit"} {
		if entry, ok := proofs[path]; !ok || entry.Type != typ || entry.OID == "" {
			t.Fatalf("%s proof = %+v ok=%v, want base %s entry", path, entry, ok, typ)
		}
	}
	if _, ok := proofs["dir/child.txt"]; ok {
		t.Fatal("deleted path must not appear as a create destination proof")
	}
	if _, ok := proofs[".corvint/change.cem.json"]; ok {
		t.Fatalf("exact excluded sidecar unexpectedly proved: %+v", proofs[".corvint/change.cem.json"])
	}
}

func TestCanonicalDiffWithCreateDestinationsWithholdsProofOnProvenanceFailure(t *testing.T) {
	root, base, target := canonicalCreateRepo(t)
	raw := gitCmd(t, root, "cat-file", "commit", target)
	corruptLoose(t, root, target, "commit", append([]byte(raw), "forged\n"...))
	if patch, proofs, err := open(t, root).CanonicalDiffWithCreateDestinations(context.Background(), base, target); err == nil || patch != nil || proofs != nil {
		t.Fatalf("CanonicalDiffWithCreateDestinations patch=%q proofs=%v err=%v, want no proof on failure", patch, proofs, err)
	}
}
