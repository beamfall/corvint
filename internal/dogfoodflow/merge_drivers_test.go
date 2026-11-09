// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/mergedrivers"
)

func TestMergeDriverHelper(t *testing.T) {
	if os.Getenv("CORVINT_BATCH_DRIVER_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg != "--" {
			continue
		}
		args := os.Args[i+1:]
		if len(args) != 4 {
			os.Exit(2)
		}
		if args[0] == "keep-ours" {
			os.Exit(0)
		}
		var data [3][]byte
		for i, path := range args[1:] {
			var err error
			data[i], err = os.ReadFile(path)
			if err != nil {
				os.Exit(2)
			}
		}
		var result []byte
		var err error
		clean := true
		if args[0] == "append-union" {
			result, clean, err = mergedrivers.Append(context.Background(), "", data[0], data[1], data[2], 7)
		} else {
			result, err = mergedrivers.JSON(data[0], data[1], data[2])
		}
		if err != nil || !clean {
			os.Exit(1)
		}
		if os.WriteFile(args[2], result, 0o600) != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(2)
}

func installBatchDrivers(t *testing.T, repo *batchRepo) {
	t.Helper()
	t.Setenv("CORVINT_BATCH_DRIVER_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	repo.write(".gitattributes", "docs/specs/*.md merge=specappend\ndocs/specs/README.md merge=specgenerated\ndocs/specs/INDEX.json merge=specgenerated\ndocs/specs/REQUIREMENTS.tsv merge=specgenerated\n.corvint/test-read-scopes.json merge=registryunion\n")
	for name, mode := range map[string]string{"specappend": "append-union", "registryunion": "json-union", "specgenerated": "keep-ours"} {
		driver := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "' -test.run=^TestMergeDriverHelper$ -- " + mode + ` "%O" "%A" "%B"`
		testGit(t, repo.root, "config", "merge."+name+".driver", driver)
	}
}

func TestBatchCheckDWCV07704Drivers(t *testing.T) {
	for _, mode := range []string{"append-union", "json-union", "json-union-add-add", "regenerated-specs", "append-union+regenerated-specs", "append-union+json-union+regenerated-specs"} {
		t.Run(mode, func(t *testing.T) {
			repo := specFixtureRepo(t)
			installBatchDrivers(t, repo)
			if mode != "json-union-add-add" {
				repo.write(".corvint/test-read-scopes.json", "{\"packages\":{}}\n")
			}
			repo.base = repo.commit("driver fixture")
			lane := func(name string) string {
				testGit(t, repo.root, "checkout", "-q", "-b", name, repo.base)
				if strings.Contains(mode, "append-union") {
					suffix := "\n## " + name + "\nA section.\n"
					if strings.Contains(mode, "regenerated-specs") {
						suffix = "- `FXA-V0-" + map[string]string{"ours": "002", "theirs": "003"}[name] + "`: Extra.\n"
					}
					repo.write("docs/specs/fixture-a-v0.md", specFixture("Fixture A", "- `FXA-V0-001`: The first requirement MUST hold.\n", suffix))
				}
				if strings.Contains(mode, "json-union") {
					repo.write(".corvint/test-read-scopes.json", "{\"packages\":{\""+name+"\":[]}}\n")
				}
				if mode == "regenerated-specs" {
					// All three projections conflict. Preserved prose/fields come from
					// ours, while staged source requirements must both survive replay.
					repo.write("docs/specs/README.md", "# Specs\n\n"+name+"\n")
					repo.write("docs/specs/INDEX.json", "{\"lane\":\""+name+"\"}\n")
					if name == "ours" {
						repo.specFixtureSpecs("- `FXA-V0-002`: Extra A.\n", "")
					} else {
						repo.specFixtureSpecs("", "- `FXC-V0-000`: Extra C.\n")
					}
				}
				repo.regenerateSpecs()
				repo.commit("work " + name)
				repo.write(".corvint/change.cem.json", `{"spec":"cem/1.0","baseRevision":"`+repo.base+`","hunks":[]}`+"\n")
				bind := repo.commit("chore: bind change evidence")
				if err := os.MkdirAll(filepath.Join(repo.root, ".corvint/changes"), 0o700); err != nil {
					t.Fatal(err)
				}
				testGit(t, repo.root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+bind+".cem.json")
				return repo.commit("chore: seal change evidence")
			}
			ours, theirs := lane("ours"), lane("theirs")
			repo.batch(ours)
			testGit(t, repo.root, "merge", "--no-ff", "--no-commit", theirs)
			if conflicts := testGit(t, repo.root, "diff", "--name-only", "--diff-filter=U"); conflicts != "" {
				t.Fatalf("driver conflicts: %s", conflicts)
			}
			repo.regenerateSpecs()
			merge := repo.commit("Merge driver results")
			// Config must never supply the replay rule. A malicious local driver
			// would write a sentinel if batch-check consulted the caller's config.
			for _, name := range []string{"specappend", "registryunion", "specgenerated"} {
				testGit(t, repo.root, "config", "merge."+name+".driver", "touch driver-executed; exit 1")
			}
			before := testGit(t, repo.root, "status", "--porcelain")
			code, stdout, stderr := repo.check()
			derivation := strings.TrimSuffix(mode, "-add-add")
			if code != 0 || !strings.Contains(stdout, "MERGE "+merge+" derivation="+derivation+"\n") {
				t.Fatalf("driver batch exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if _, err := os.Stat(filepath.Join(repo.root, "driver-executed")); !os.IsNotExist(err) || testGit(t, repo.root, "status", "--porcelain") != before {
				t.Fatalf("batch-check mutated the repository: %v", err)
			}
			// One extra hand-written file must not validate itself through replay.
			repo.write("unbound.txt", "manual resolution\n")
			testGit(t, repo.root, "add", "unbound.txt")
			testGit(t, repo.root, "commit", "-q", "--amend", "--no-edit")
			code, _, stderr = repo.check()
			if code != 1 || !strings.Contains(stderr, "FAIL merge-resolution-unbound") {
				t.Fatalf("hand edit accepted: exit=%d %s", code, stderr)
			}
		})
	}
}

func TestBatchCheckDWCV07704DivergentRegistryStaysUnbound(t *testing.T) {
	repo := newBatchRepo(t)
	installBatchDrivers(t, repo)
	path := ".corvint/test-read-scopes.json"
	repo.write(path, `{"packages":{"key":[]}}`+"\n")
	repo.base = repo.commit("registry fixture")
	_, ours := repo.lane("ours", path, `{"packages":{"key":["ours"]}}`+"\n", "")
	_, theirs := repo.lane("theirs", path, `{"packages":{"key":["theirs"]}}`+"\n", "")
	repo.batch(ours)
	if err := exec.Command("git", "-C", repo.root, "merge", "--no-commit", theirs).Run(); err == nil {
		t.Fatal("same-key divergence did not conflict")
	}
	repo.write(path, `{"packages":{"key":["hand"]}}`+"\n")
	repo.commit("Merge hand resolution")
	code, _, stderr := repo.check()
	if code != 1 || !strings.Contains(stderr, "FAIL merge-resolution-unbound") {
		t.Fatalf("divergent key accepted: exit=%d %s", code, stderr)
	}
}

func TestBatchCheckDWCV07704CleanJSON(t *testing.T) {
	repo := newBatchRepo(t)
	installBatchDrivers(t, repo)
	path := ".corvint/test-read-scopes.json"
	base := "{\n  \"packages\": {\n    \"a\": [],\n    \"b\": [],\n    \"c\": [],\n    \"d\": [],\n    \"e\": []\n  }\n}\n"
	repo.write(path, base)
	repo.base = repo.commit("clean JSON fixture")
	_, ours := repo.lane("ours", path, strings.Replace(base, `"a": []`, `"a": ["ours"]`, 1), "")
	_, theirs := repo.lane("theirs", path, strings.Replace(base, `"e": []`, `"e": ["theirs"]`, 1), "")
	if _, status := mergeTreeStatus(t, repo.root, ours, theirs); status != 0 {
		t.Fatal("fixture must have a clean text merge")
	}
	repo.batch(ours)
	testGit(t, repo.root, "merge", "--no-ff", "--no-edit", theirs)
	code, stdout, stderr := repo.check()
	if code != 0 || !strings.Contains(stdout, "derivation=json-union\n") {
		t.Fatalf("clean custom JSON merge: %d %s %s", code, stdout, stderr)
	}
	// An additional key was not supplied by either parent and stays unbound.
	merged := testGit(t, repo.root, "show", "HEAD:"+path)
	repo.write(path, strings.Replace(merged, `"packages": {`, `"packages": {"hand": [],`, 1)+"\n")
	testGit(t, repo.root, "add", path)
	testGit(t, repo.root, "commit", "-q", "--amend", "--no-edit")
	code, _, stderr = repo.check()
	if code != 1 || !strings.Contains(stderr, "FAIL merge-resolution-unbound") {
		t.Fatalf("hand-added key accepted: %d %s", code, stderr)
	}
}
