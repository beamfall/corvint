package doccorpus

import (
	"strings"
	"testing"
)

// The review witnesses of DCI-V1-021 to DCI-V1-023 (GitHub #2007): only an executed publish
// command ties a manifest, an opaque folder argument ties nothing, a step's working directory
// replaces the job's, YAML key order changes no result, an ambiguous name at any level stops the
// inference, and the inferred word is kept as written.

const ciManifest = "{\n  \"name\": \"@x/%s\"\n}\n"

func ciManifests(files map[string]string, dirs ...string) map[string]string {
	for _, dir := range dirs {
		p := dir + "/package.json"
		if dir == "." {
			p = "package.json"
		}
		files[p] = strings.Replace(ciManifest, "%s", strings.ReplaceAll(dir, "/", "-"), 1)
	}
	return files
}

func manifestKind(t *testing.T, a *Artifact, dir string) (string, string) {
	t.Helper()
	id := "native:package:" + dir + "/package.json"
	if dir == "." {
		id = "native:package:package.json"
	}
	subjects, claims := inventoryIndex(a)
	return subjects[id].Kind, claims[id+":derivation"].Text
}

// TestDCIV1021OnlyExecutedPublishCommandsTie: a step name, an environment value or an echo that
// mentions npm publish is no publish command, so it ties no manifest.
func TestDCIV1021OnlyExecutedPublishCommandsTie(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "said", ciManifests(map[string]string{
		".github/workflows/release.yml": "name: Release\non: push\njobs:\n" +
			"  release-named:\n    steps:\n      - uses: actions/checkout@v4\n      - name: npm publish\n        run: npm run build\n" +
			"  release-env:\n    env:\n      NPM_HELP: npm publish\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm test\n" +
			"  release-echo:\n    steps:\n      - uses: actions/checkout@v4\n      - run: echo npm publish\n",
	}, "."), []string{".github", "package.json"}, &RepositoryInventory{})
	if kind, text := manifestKind(t, a, "."); kind != "package_manifest" {
		t.Fatalf("text that only mentions npm publish should tie nothing: %s %s", kind, text)
	}
}

// TestDCIV1021OpaqueFolderArgumentsTieNothing: an absolute, variable or bare folder argument is
// opaque and ties no manifest, not even the working directory's; a quoted literal relative folder
// ties the folder it names.
func TestDCIV1021OpaqueFolderArgumentsTieNothing(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "folders", ciManifests(map[string]string{
		".github/workflows/release.yml": "name: Release\non: push\njobs:\n" +
			"  release-tmp:\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish /tmp/package\n" +
			"  release-var:\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish \"$DIR\"\n" +
			"  release-bare:\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish packages/b\n" +
			"  release-quoted:\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish \"./packages/a\"\n",
	}, ".", "packages/a", "packages/b"), []string{".github", "package.json", "packages"}, &RepositoryInventory{})
	for dir, want := range map[string]string{".": "package_manifest", "packages/b": "package_manifest", "packages/a": "package"} {
		if kind, text := manifestKind(t, a, dir); kind != want {
			t.Fatalf("%s = %s, want %s: %s", dir, kind, want, text)
		}
	}
	if _, text := manifestKind(t, a, "."); !strings.Contains(text, "release-tmp at") || !strings.Contains(text, "release-var at") || !strings.Contains(text, "release-bare at") {
		t.Fatalf("the root manifest should name the opaque jobs as candidates: %s", text)
	}
}

// TestDCIV1021StepWorkingDirectoryReplacesTheJobDefault: a step's working-directory replaces the
// job's defaults.run.working-directory; it is not joined onto it. A manifest no job ties names the
// observed jobs tied to other directories.
func TestDCIV1021StepWorkingDirectoryReplacesTheJobDefault(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "override", ciManifests(map[string]string{
		".github/workflows/release.yml": "name: Release\non: push\njobs:\n" +
			"  publish-b:\n    defaults:\n      run:\n        working-directory: packages/a\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish\n        working-directory: packages/b\n",
	}, "packages/a", "packages/b"), []string{".github", "packages"}, &RepositoryInventory{})
	if kind, text := manifestKind(t, a, "packages/b"); kind != "package" {
		t.Fatalf("the step's working directory should replace the job's: %s %s", kind, text)
	}
	kind, text := manifestKind(t, a, "packages/a")
	if kind != "package_manifest" || strings.Contains(text, "no release or publish job in this repository") || !strings.Contains(text, "no release or publish job ties this manifest's directory") || !strings.Contains(text, "publish-b at .github/workflows/release.yml:4 (publishing directory packages/b)") {
		t.Fatalf("an untied manifest should name the observed job tied elsewhere: %s %s", kind, text)
	}
}

// TestDCIV1021KeyOrderChangesNoTie: a step's working directory before or after its run line, a
// workflow default after jobs:, and a CircleCI run mapping in either key order tie the same
// directories.
func TestDCIV1021KeyOrderChangesNoTie(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "order", ciManifests(map[string]string{
		".github/workflows/release.yml": "name: Release\non: push\njobs:\n" +
			"  publish-a:\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish\n        working-directory: packages/a\n" +
			"  publish-c:\n    steps:\n      - uses: actions/checkout@v4\n      - working-directory: packages/c\n        run: npm publish\n" +
			"  publish-d:\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish\n" +
			"defaults:\n  run:\n    working-directory: packages/d\n",
		".circleci/config.yml": "version: 2.1\njobs:\n" +
			"  publish-e:\n    steps:\n      - checkout\n      - run:\n          command: npm publish\n          working_directory: packages/e\n" +
			"  publish-f:\n    steps:\n      - checkout\n      - run:\n          working_directory: packages/f\n          command: npm publish\n",
	}, ".", "packages/a", "packages/c", "packages/d", "packages/e", "packages/f"), []string{".circleci", ".github", "package.json", "packages"}, &RepositoryInventory{})
	for _, dir := range []string{"packages/a", "packages/c", "packages/d", "packages/e", "packages/f"} {
		if kind, text := manifestKind(t, a, dir); kind != "package" {
			t.Fatalf("%s should be tied whatever the key order: %s %s", dir, kind, text)
		}
	}
	if kind, text := manifestKind(t, a, "."); kind != "package_manifest" {
		t.Fatalf("a workflow default after jobs: still moves the publish off the root: %s %s", kind, text)
	}
}

// TestDCIV1022AmbiguityStopsAndTheWordIsKept: a name with two environment words at any level stops
// the inference, even when a later level names one; the inferred word keeps its spelling.
func TestDCIV1022AmbiguityStopsAndTheWordIsKept(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "names", map[string]string{
		".github/workflows/deploy-prod.yml":    "name: Deploy dev and prod\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/deploy-uat.yml":     "name: Deploy\non: push\njobs:\n  deploy-dev-prod:\n    runs-on: ubuntu-latest\n",
		".github/workflows/shout.yml":          "name: Deploy to PROD\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/hyphen.yml":         "name: Deploy pre-prod\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/name-after.yml":     "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\nname: Staging deploy\n",
		".github/workflows/deploy-Stg.yml":     "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/alias-name.yml":     "name: *prod\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/deploy-qa-2.yml":    "name: >\n  Deploy\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/deploy-sandbox.yml": "name: 'Deploy to \"uat\"'\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
	}, []string{".github"}, &RepositoryInventory{})
	subjects, claims := inventoryIndex(a)
	for file, want := range map[string]string{
		"deploy-prod.yml":                "deploy",
		"deploy-uat.yml:deploy-dev-prod": "deploy-dev-prod",
		"shout.yml":                      "deploy (PROD)",
		"hyphen.yml":                     "deploy (pre-prod)",
		"name-after.yml":                 "deploy (Staging)",
		"deploy-Stg.yml":                 "deploy (Stg)",
		"alias-name.yml":                 "deploy",
		"deploy-qa-2.yml":                "deploy",
		"deploy-sandbox.yml":             "deploy (uat)",
	} {
		id := "native:deployable:.github/workflows/" + file
		if !strings.Contains(file, ":") {
			id += ":deploy"
		}
		s := subjects[id]
		if s.Name != want {
			t.Fatalf("%s = %q, want %q: %s", id, s.Name, want, claims[id+":derivation"].Text)
		}
		if want == "deploy" || want == "deploy-dev-prod" {
			if !strings.Contains(claims[id+":derivation"].Text, "environment is NOT_OBSERVED") {
				t.Fatalf("%s should leave its environment NOT_OBSERVED: %s", id, claims[id+":derivation"].Text)
			}
		}
	}
}

// TestDCIV1023CircleCIDirectoriesAndCheckout: a CircleCI step working_directory under ~ or
// absolute is opaque; the job's working_directory is where checkout places the repository, so a
// publish after a default checkout ties the root; a publish with no prior checkout, or after a
// checkout to another path, ties nothing.
func TestDCIV1023CircleCIDirectoriesAndCheckout(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "circledirs", ciManifests(map[string]string{
		".circleci/config.yml": "version: 2.1\njobs:\n" +
			"  publish-home:\n    steps:\n      - checkout\n      - run:\n          command: npm publish\n          working_directory: ~/project/packages/a\n" +
			"  publish-abs:\n    steps:\n      - checkout\n      - run:\n          command: npm publish\n          working_directory: /srv/packages/b\n" +
			"  publish-attached:\n    steps:\n      - attach_workspace:\n          at: packages/b\n      - run: npm publish\n" +
			"  publish-moved:\n    steps:\n      - checkout:\n          path: packages/b\n      - run: npm publish\n" +
			"  publish-root:\n    working_directory: ~/repo\n    steps:\n      - checkout\n      - run: npm publish\n",
	}, ".", "packages/a", "packages/b"), []string{".circleci", "package.json", "packages"}, &RepositoryInventory{})
	for dir, want := range map[string]string{".": "package", "packages/a": "package_manifest", "packages/b": "package_manifest"} {
		if kind, text := manifestKind(t, a, dir); kind != want {
			t.Fatalf("%s = %s, want %s: %s", dir, kind, want, text)
		}
	}
	_, text := manifestKind(t, a, "packages/a")
	for _, job := range []string{"publish-home", "publish-abs", "publish-attached", "publish-moved"} {
		if !strings.Contains(text, job+" at .circleci/config.yml:") {
			t.Fatalf("the untied manifest should name %s as a candidate: %s", job, text)
		}
	}
}
