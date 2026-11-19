// SPDX-License-Identifier: Apache-2.0

package liverepo_test

import (
	"context"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// AFU-V1-1651, AFU-V1-1655 (V1-5195): this repository binds its E2E suites to
// flow intents committed under flows/: every bound consumer is a declared
// binary consumer, every bound flow exists, every intent is valid at HEAD, and
// every declared link carries a review anchor that evaluates `reviewed` at
// HEAD, so the binding has narrowing authority.
func TestRepositoryE2EFlowsAreBoundAndReviewed_AFUV11655(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	declared, err := appflows.ReadE2EFlows(root)
	if err != nil || declared == nil {
		t.Fatalf("declaration: %v (%+v)", err, declared)
	}
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(affected.BinaryConsumersPath)))
	if err != nil {
		t.Fatal(err)
	}
	var consumers struct {
		Consumers map[string]any `json:"consumers"`
	}
	if err := json.Unmarshal(body, &consumers); err != nil {
		t.Fatal(err)
	}
	set, err := appflows.LoadIntentsAt(context.Background(), root, declared.Dir, "HEAD")
	if err != nil {
		t.Fatalf("intents at HEAD: %v", err)
	}
	flows := map[string]bool{}
	for _, intent := range set.Flows {
		flows[intent.FlowID] = true
	}
	bound := map[string]bool{}
	for file, ids := range declared.Consumers {
		if _, ok := consumers.Consumers[file]; !ok {
			t.Errorf("%s binds %s, which %s does not declare", appflows.E2EFlowsPath, file, affected.BinaryConsumersPath)
		}
		for _, id := range ids {
			bound[id] = true
			if !flows[id] {
				t.Errorf("%s binds flow %s, which %s does not hold", appflows.E2EFlowsPath, id, declared.Dir)
			}
		}
	}
	for id := range flows {
		if !bound[id] {
			t.Errorf("flow %s is committed but bound to no consumer", id)
		}
	}
	links, err := appflows.EvaluateLinks(context.Background(), root, set, set.Revision)
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range links {
		if link.Basis == "inferred" {
			t.Errorf("%s: inferred link %s -> %s has no place in a committed binding", link.Flow, link.From, link.Target.Path)
			continue
		}
		if link.Target.Path != "" {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(link.Target.Path))); err != nil {
				t.Errorf("%s: link target %s: %v", link.Flow, link.Target.Path, err)
			}
		}
		if requireReviewed && link.ReviewState != appflows.ReviewReviewed {
			t.Errorf("%s: link %s -> %s is %s, not reviewed; set reviewed_at to the commit that declared it", link.Flow, link.From, link.Target.Path, link.ReviewState)
		}
	}
	for _, intent := range set.Flows {
		if !slices.ContainsFunc(intent.Links, func(l appflows.FlowLink) bool { return l.Target.Type == "test" }) {
			t.Errorf("flow %s links no E2E test", intent.FlowID)
		}
	}
}

// requireReviewed is false only in a commit that adds an intent, whose review
// anchor is that commit itself; the next commit sets the anchors and turns it
// back on.
const requireReviewed = true

// RMV-V1-009: the fixture is copied by both installed drivers. A previously unseen
// filename under the fixture has the same qualification obligations as math.js.
func TestInteractiveFixtureSelectsInstalledQualification(t *testing.T) {
	graph, _ := repositoryOwnershipGraph(t)
	for _, path := range []string{"conformance/interactive-alpha/fixture/math.js", "conformance/interactive-alpha/fixture/app/future.asset"} {
		selected := selectedSet(affected.Select(graph, []string{path}))
		for _, target := range []string{"interactive-alpha-installed-core", "interactive-alpha-installed-editor"} {
			if _, ok := selected["driver:make:Makefile#"+target]; !ok {
				t.Errorf("%s: missing installed qualification target %s", path, target)
			}
		}
	}
}

// RMV-V1-009, AZU-V0-007: plan each provider-only change in isolation. Command
// sources and imported packages must both carry runnable installed consumers.
func TestProviderBuildSelectsInstalledQualification(t *testing.T) {
	graph, _ := repositoryOwnershipGraph(t)
	consumers := map[string]string{
		"conformance/interactive-alpha/core-provider-proof.mjs": "interactive-alpha-installed-core",
		"extensions/vscode/test/installed/run.mjs":              "interactive-alpha-installed-editor",
	}
	for _, path := range []string{
		"cmd/corvint-go-test-provider/main.go", "internal/gotestrun/run.go",
		"cmd/corvint-js-test-provider/main.go", "internal/jstestprovider/digest.go",
	} {
		plan := affected.Select(graph, []string{path})
		selected := selectedSet(plan)
		for file, target := range consumers {
			row := selected["declared-consumer:"+file]
			if !slices.Equal(row.Run, []string{"make", target}) || !slices.Contains(row.Tests, file) || row.Witness.DirtyPath != path {
				t.Errorf("%s: installed consumer %s missing its runnable target: %+v", path, file, row)
			}
		}
		for _, unknown := range plan.Unknown {
			if _, installed := consumers[unknown.Detail]; installed && unknown.Reason == affected.UnknownUnselectedBinaryConsumer {
				t.Errorf("%s: installed consumer still unselected: %+v", path, unknown)
			}
		}
	}
}
