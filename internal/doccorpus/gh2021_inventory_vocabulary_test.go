package doccorpus

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// circleJobs is a CircleCI configuration with one checkout-only job per key.
func circleJobs(keys ...string) string {
	out := "version: 2.1\njobs:\n"
	for _, k := range keys {
		out += "  " + k + ":\n    docker:\n      - image: x\n    steps:\n      - checkout\n"
	}
	return out + "workflows:\n  w:\n    jobs:\n      - " + strings.Join(keys, "\n      - ") + "\n"
}

// TestDCIV1027DeployWordsAndShortEnvironments: prd, sbx and eph name environments, and a job is a
// deploy job only when one of its words is deploy, so contract-broker checks such as
// pact_can_deploy and pact_record_deployment_* are no deployable units (GitHub #2021).
func TestDCIV1027DeployWordsAndShortEnvironments(t *testing.T) {
	t.Parallel()
	a := inventoryRepo(t, t.TempDir(), "short", map[string]string{
		".circleci/config.yml": circleJobs("deploy-stg", "deploy-prd", "deploy_sbx", "deploy-eph", "deploy", "build_and_deploy",
			"pact_can_deploy", "pact_can_deploy_sbx", "pact_record_deployment_sbx", "pact_record_deployment_staging", "can-i-deploy", "redeploy"),
	}, []string{".circleci"}, &RepositoryInventory{})
	subjects, claims := inventoryIndex(a)
	for job, name := range map[string]string{
		"deploy-stg": "deploy-stg (stg)", "deploy-prd": "deploy-prd (prd)", "deploy_sbx": "deploy_sbx (sbx)",
		"deploy-eph": "deploy-eph (eph)", "deploy": "deploy", "build_and_deploy": "build_and_deploy",
	} {
		id := "native:deployable:.circleci/config.yml:" + job
		if s := subjects[id]; s.Kind != "deployable_unit" || s.Name != name {
			t.Fatalf("%s = %+v, want deployable unit %q", job, s, name)
		}
	}
	if c := claims["native:deployable:.circleci/config.yml:deploy:derivation"]; !strings.Contains(c.Text, "NOT_OBSERVED") {
		t.Fatalf("a plain deploy job keeps its environment NOT_OBSERVED: %+v", c)
	}
	for _, job := range []string{"pact_can_deploy", "pact_can_deploy_sbx", "pact_record_deployment_sbx", "pact_record_deployment_staging", "can-i-deploy", "redeploy"} {
		if s, ok := subjects["native:deployable:.circleci/config.yml:"+job]; ok {
			t.Fatalf("%s should be no deployable unit: %+v", job, s)
		}
	}
	for job, want := range map[string]bool{
		"deploy": true, "Deploy": true, "deploy-prd": true, "deploy_prd": true, "prd-deploy": true, "ship_deploy": true,
		"deployProd": true, "build-and-deploy-prod": true, "pact_can_deploy": false, "can-i-deploy": false,
		"pact_can_i_deploy_sbx": false, "record_deployment": false, "deployment": false, "redeploy": false, "build": false,
	} {
		if got := deployJobName(job); got != want {
			t.Fatalf("deployJobName(%q) = %v, want %v", job, got, want)
		}
	}
}

// TestDCIV1028DeclaredEnvironmentWords: repository_inventory.environment_words extends the
// vocabulary a deploy job's name is read with, still as weaker, name-derived evidence, and a
// malformed word list is refused.
func TestDCIV1028DeclaredEnvironmentWords(t *testing.T) {
	t.Parallel()
	files := map[string]string{".github/workflows/ship.yml": "name: Ship\non: push\njobs:\n  deploy-live:\n    runs-on: ubuntu-latest\n"}
	id := "native:deployable:.github/workflows/ship.yml:deploy-live"
	without, _ := inventoryIndex(inventoryRepo(t, t.TempDir(), "plain", files, []string{".github"}, &RepositoryInventory{}))
	if s := without[id]; s.Name != "deploy-live" {
		t.Fatalf("live is no built-in environment word: %+v", s)
	}
	subjects, claims := inventoryIndex(inventoryRepo(t, t.TempDir(), "words", files, []string{".github"}, &RepositoryInventory{EnvironmentWords: []string{"live", "perf"}}))
	if s := subjects[id]; s.Name != "deploy-live (live)" || !strings.Contains(claims[id+":derivation"].Text, "inferred from the job key (no environment key; name-derived, weaker evidence)") {
		t.Fatalf("a declared word should name the environment as inferred evidence: %+v %+v", s, claims[id+":derivation"])
	}
	m := Manifest{}
	m.Providers = []Provider{{ID: "native", Kind: "native"}}
	for _, words := range [][]string{{}, {"b", "a"}, {"a", "a"}, {"Live"}, {"pre-prod"}, {strings.Repeat("a", 33)}} {
		m.RepositoryInventory = &RepositoryInventory{EnvironmentWords: words}
		if err := validateRepositoryInventory(m); err == nil || !strings.Contains(err.Error(), "environment") {
			t.Fatalf("environment_words %v should be refused: %v", words, err)
		}
	}
}

// TestDCIV1029DeclaredDeployJob: repository_inventory.deploy_jobs names the environment of a job
// its name does not reveal, makes a declared job deployable whatever its name, says so in the
// claim, names a differing environment key, and records a declaration no input holds as a gap.
func TestDCIV1029DeclaredDeployJob(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	a := inventoryRepo(t, parent, "declared", map[string]string{
		".circleci/config.yml":       circleJobs("deploy", "ship", "deploy-stg"),
		".github/workflows/prod.yml": "on: push\njobs:\n  deploy:\n    runs-on: ubuntu-latest\n    environment: staging\n",
	}, []string{".circleci", ".github"}, &RepositoryInventory{})
	repo := a.Manifest.Repository.ID
	m := a.Manifest
	m.RepositoryInventory = &RepositoryInventory{DeployJobs: []DeployJobDeclaration{
		{Repository: repo, Path: ".circleci/config.yml", Job: "deploy", Environment: "production"},
		{Repository: repo, Path: ".circleci/config.yml", Job: "gone", Environment: "production"},
		{Repository: repo, Path: ".circleci/config.yml", Job: "ship", Environment: "prd"},
		{Repository: repo, Path: ".github/workflows/prod.yml", Job: "deploy", Environment: "production"},
	}}
	built, err := Build(context.Background(), filepath.Join(parent, "declared"), m)
	if err != nil {
		t.Fatal(err)
	}
	subjects, claims := inventoryIndex(built)
	for id, name := range map[string]string{
		"native:deployable:.circleci/config.yml:deploy":       "deploy (production)",
		"native:deployable:.circleci/config.yml:ship":         "ship (prd)",
		"native:deployable:.github/workflows/prod.yml:deploy": "deploy (production)",
		"native:deployable:.circleci/config.yml:deploy-stg":   "deploy-stg (stg)",
	} {
		if s := subjects[id]; s.Kind != "deployable_unit" || s.Name != name {
			t.Fatalf("%s = %+v, want %q", id, s, name)
		}
	}
	if c := claims["native:deployable:.circleci/config.yml:deploy:derivation"]; !strings.Contains(c.Text, "declared by repository_inventory deploy_jobs") {
		t.Fatalf("a declared job's claim should say it is declared: %+v", c)
	}
	if c := claims["native:deployable:.github/workflows/prod.yml:deploy:derivation"]; !strings.Contains(c.Text, "its environment key names staging") {
		t.Fatalf("a differing environment key should be named: %+v", c)
	}
	if g := gapReasons(t, built, "deploy-declaration-unmatched"); len(g) != 1 || !strings.Contains(g[0], "job gone of .circleci/config.yml") {
		t.Fatalf("an unmatched declaration should be one gap: %v", g)
	}

	bad := m
	for _, jobs := range [][]DeployJobDeclaration{
		{},
		{{Repository: strings.Repeat("0", 40), Path: ".circleci/config.yml", Job: "deploy", Environment: "production"}},
		{{Repository: repo, Path: "ci/config.yml", Job: "deploy", Environment: "production"}},
		{{Repository: repo, Path: ".circleci/config.yml", Job: "de ploy", Environment: "production"}},
		{{Repository: repo, Path: ".circleci/config.yml", Job: "deploy", Environment: ""}},
		{{Repository: repo, Path: ".circleci/config.yml", Job: "ship", Environment: "prd"}, {Repository: repo, Path: ".circleci/config.yml", Job: "deploy", Environment: "prd"}},
		{{Repository: repo, Path: ".circleci/config.yml", Job: "deploy", Environment: "prd"}, {Repository: repo, Path: ".circleci/config.yml", Job: "deploy", Environment: "prd"}},
	} {
		bad.RepositoryInventory = &RepositoryInventory{DeployJobs: jobs}
		if err := validateRepositoryInventory(bad); err == nil || !strings.Contains(err.Error(), "deploy") {
			t.Fatalf("deploy_jobs %+v should be refused: %v", jobs, err)
		}
	}
}
