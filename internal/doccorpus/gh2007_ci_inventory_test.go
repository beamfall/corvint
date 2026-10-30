package doccorpus

import (
	"strings"
	"testing"
)

func inventoryIndex(a *Artifact) (map[string]Subject, map[string]Claim) {
	subjects, claims := map[string]Subject{}, map[string]Claim{}
	for _, s := range a.Subjects {
		subjects[s.ID] = s
	}
	for _, c := range a.Claims {
		claims[c.ID] = c
	}
	return subjects, claims
}

// TestDCIV1021PublishJobIsTiedToTheManifestItPublishes: a release or publish job makes a package
// manifest a package only when a literal npm, pnpm or yarn publish step publishes that manifest's
// directory, by step or job working directory, `cd DIR &&` or a folder argument; a job with no
// such step, or whose publish directory is an expression or a workspace selector, is named as a
// candidate on a package_manifest (GitHub #2007).
func TestDCIV1021PublishJobIsTiedToTheManifestItPublishes(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	// The reported shape: a private root, one plugin manifest and a nightly release job that
	// publishes nothing.
	plugins := inventoryRepo(t, parent, "plugins", map[string]string{
		"package.json":                  "{\n  \"name\": \"app\",\n  \"private\": true\n}\n",
		"plugins/p/package.json":        "{\n  \"name\": \"@x/p\"\n}\n",
		".github/workflows/nightly.yml": "on: schedule\njobs:\n  nightly-release:\n    runs-on: ubuntu-latest\n    steps:\n      - run: npm run build\n",
	}, []string{".github", "package.json", "plugins"}, &RepositoryInventory{})
	subjects, claims := inventoryIndex(plugins)
	if s := subjects["native:package:plugins/p/package.json"]; s.Kind != "package_manifest" {
		t.Fatalf("a release job that publishes nothing should leave the manifest a package_manifest: %+v", s)
	}
	if c := claims["native:package:plugins/p/package.json:derivation"]; !strings.Contains(c.Text, "publish evidence NOT_OBSERVED: no release or publish job ties this manifest's directory by a literal npm, pnpm or yarn publish step; candidate CI release or publish job nightly-release at .github/workflows/nightly.yml:3") {
		t.Fatalf("the manifest should name the job as a candidate only: %+v", c)
	}
	if c := claims["native:repository:"+inventoryRootID(t, plugins)+":publish-job:.github/workflows/nightly.yml:nightly-release"]; !strings.Contains(c.Text, "a candidate only") {
		t.Fatalf("the job claim should say it is a candidate only: %+v", c)
	}

	files := map[string]string{
		".github/workflows/release.yml": "name: Release\non: push\njobs:\n" +
			"  publish-a:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - name: Publish\n        run: npm publish # to the registry\n        working-directory: packages/a\n" +
			"  publish-b:\n    defaults:\n      run:\n        working-directory: packages/b\n    steps:\n      - uses: actions/checkout@v4\n      - run: pnpm publish --no-git-checks\n" +
			"  release-c:\n    steps:\n      - uses: actions/checkout@v4\n      - run: cd packages/c && yarn publish\n" +
			"  release-d:\n    steps:\n      - uses: actions/checkout@v4\n      - run: npm publish ./packages/d\n" +
			"  release-e:\n    steps:\n      - run: npm publish\n        working-directory: ${{ matrix.dir }}\n" +
			"  release-f:\n    steps:\n      - run: yarn workspace @x/f npm publish\n      - run: echo done # npm publish later\n",
	}
	for _, dir := range []string{"a", "b", "c", "d", "e"} {
		files["packages/"+dir+"/package.json"] = "{\n  \"name\": \"@x/" + dir + "\"\n}\n"
	}
	tied := inventoryRepo(t, parent, "tied", files, []string{".github", "packages"}, &RepositoryInventory{})
	subjects, claims = inventoryIndex(tied)
	for dir, job := range map[string]string{"a": "publish-a at .github/workflows/release.yml:4 runs a publish step at line 9", "b": "publish-b at .github/workflows/release.yml:11 runs a publish step at line 17", "c": "release-c at .github/workflows/release.yml:18 runs a publish step at line 21", "d": "release-d at .github/workflows/release.yml:22 runs a publish step at line 25"} {
		id := "native:package:packages/" + dir + "/package.json"
		if s := subjects[id]; s.Kind != "package" {
			t.Fatalf("%s should be a package published by %s: %+v", id, job, s)
		}
		if c := claims[id+":derivation"]; !strings.Contains(c.Text, "publish evidence CI job "+job+" in this manifest's directory") || !strings.Contains(c.Text, "package-json/3") {
			t.Fatalf("%s should name the job and step publishing it: %+v", id, c)
		}
	}
	if s := subjects["native:package:packages/e/package.json"]; s.Kind != "package_manifest" {
		t.Fatalf("an expression or workspace-selected publish ties no manifest: %+v", s)
	}
	c := claims["native:package:packages/e/package.json:derivation"]
	if !strings.Contains(c.Text, "no release or publish job ties this manifest's directory by a literal npm, pnpm or yarn publish step; candidate CI release or publish job release-e at .github/workflows/release.yml:26, release-f at .github/workflows/release.yml:30; observed jobs tied to other directories: publish-a at .github/workflows/release.yml:4 (publishing directory packages/a), publish-b at .github/workflows/release.yml:11 (publishing directory packages/b), release-c at .github/workflows/release.yml:18 (publishing directory packages/c), release-d at .github/workflows/release.yml:22 (publishing directory packages/d),") {
		t.Fatalf("the untied manifest should name only the opaque jobs as candidates: %+v", c)
	}
	repo := "native:repository:" + inventoryRootID(t, tied) + ":publish-job:.github/workflows/release.yml:"
	if c := claims[repo+"release-c"]; !strings.Contains(c.Text, "publishing directory packages/c") {
		t.Fatalf("a tied job claim should name the directory it publishes: %+v", c)
	}
}

// inventoryRootID is the root-commit repository ID of a single-repository inventory artifact.
func inventoryRootID(t *testing.T, a *Artifact) string {
	t.Helper()
	for _, s := range a.Subjects {
		if s.Kind == "repository" {
			return strings.TrimPrefix(s.ID, "native:repository:")
		}
	}
	t.Fatal("no repository subject")
	return ""
}

// TestDCIV1022DeployEnvironmentFromItsName: a deploy job with no environment key takes the one
// environment word its job key, workflow name or file name names, labelled as name-derived,
// weaker evidence; an environment key, even an expression, and an ambiguous name infer nothing
// (GitHub #2007).
func TestDCIV1022DeployEnvironmentFromItsName(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "deploys", map[string]string{
		".github/workflows/deploy-preprd.yml": "name: Deploy to preprd\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/deploy-prod.yml":   "name: \"Deploy to prod\"\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/apps.yml":          "name: Apps\non: push\njobs:\n  deploy-staging:\n    runs-on: ubuntu-latest\n",
		".github/workflows/ship-uat.yml":      "name: Ship\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
		".github/workflows/both.yml":          "name: Deploy dev and prod\non: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n",
	}, []string{".github"}, &RepositoryInventory{})
	subjects, claims := inventoryIndex(a)
	for file, want := range map[string][4]string{
		"deploy-preprd.yml": {"deploy", "deploy (preprd)", "the workflow name at line 1", "1"},
		"deploy-prod.yml":   {"deploy", "deploy (prod)", "the workflow name at line 1", "1"},
		"apps.yml":          {"deploy-staging", "deploy-staging (staging)", "the job key", ""},
		"ship-uat.yml":      {"deploy", "deploy (uat)", "the workflow file name", ""},
	} {
		id := "native:deployable:.github/workflows/" + file + ":" + want[0]
		s := subjects[id]
		if s.Name != want[1] || !strings.Contains(strings.Join(s.Evidence.Limitations, "; "), "inferred from "+want[2]+", not declared by an environment key") {
			t.Fatalf("%s = %+v, want %q inferred from %s", id, s, want[1], want[2])
		}
		if c := claims[id+":derivation"]; !strings.Contains(c.Text, "inferred from "+want[2]+" (no environment key; name-derived, weaker evidence)") || !strings.Contains(c.Text, "github-workflow-job/3") {
			t.Fatalf("%s claim should label the inference: %+v", id, c)
		}
		if anchored := len(s.Evidence.Anchors) == 2 && s.Evidence.Anchors[1].Start == 1; anchored != (want[3] == "1") {
			t.Fatalf("%s anchors %+v", id, s.Evidence.Anchors)
		}
	}
	if s := subjects["native:deployable:.github/workflows/both.yml:deploy"]; s.Name != "deploy" || !strings.Contains(claims[s.ID+":derivation"].Text, "environment is NOT_OBSERVED (no environment key") {
		t.Fatalf("an ambiguous name should infer no environment: %+v", s)
	}
}

// TestDCIV1023CircleCIJobs: a CircleCI configuration is read by circleci-job/1: a deploy job is a
// deployable unit (its environment key holds variables, so it names no environment), a publish
// job ties its manifest, and the repository has no unsupported-CI or deployable-unknown gap
// (GitHub #2007).
func TestDCIV1023CircleCIJobs(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "circle", map[string]string{
		"package.json": "{\n  \"name\": \"@x/svc\"\n}\n",
		".circleci/config.yml": "version: 2.1\njobs:\n  build:\n    docker:\n      - image: cimg/node:20.0\n    steps:\n      - checkout\n" +
			"  deploy-prod:\n    environment:\n      NODE_ENV: staging\n    steps:\n      - run: ./deploy.sh\n" +
			"  publish:\n    steps:\n      - checkout\n      - run: npm publish\n" +
			"workflows:\n  main:\n    jobs:\n      - build\n      - deploy-prod\n",
	}, []string{".circleci", "package.json"}, &RepositoryInventory{})
	subjects, claims := inventoryIndex(a)
	id := "native:deployable:.circleci/config.yml:deploy-prod"
	if s := subjects[id]; s.Kind != "deployable_unit" || s.Name != "deploy-prod (prod)" || !strings.Contains(claims[id+":derivation"].Text, "circleci-job/1") {
		t.Fatalf("the CircleCI deploy job should be a deployable unit named from its key: %+v %+v", s, claims[id+":derivation"])
	}
	if s := subjects["native:package:package.json"]; s.Kind != "package" {
		t.Fatalf("the CircleCI publish job should tie the root manifest: %+v", s)
	}
	for _, kind := range []string{"inventory-ci-unsupported", "deployable-evidence-unknown"} {
		if g := gapReasons(t, a, kind); len(g) != 0 {
			t.Fatalf("a read CircleCI configuration should leave no %s gap: %v", kind, g)
		}
	}
}
