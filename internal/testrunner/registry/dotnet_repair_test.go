// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

func TestRSPV0005DotnetAncestorConfigurationLoaded(t *testing.T) {
	for _, name := range []string{"Directory.Build.props", "Tests/Directory.Build.props", "Directory.Build.targets", "Directory.Packages.props", "global.json", "nuget.config", "NuGet.Config", "Tests/obj/project.assets.json"} {
		t.Run(name, func(t *testing.T) {
			c := dotnetConfig("dotnet-vstest-mstest")
			if _, ok := c.Sources[name]; !ok {
				c.Sources[name] = dotnetSource(name, "before")
			}
			p, _ := newDotnet(t, c)
			before := dotnetNative(t, p, c.Key.Runner)[0].Loaded
			if before.State != testevidence.Present {
				t.Fatal(before)
			}
			oldOID := ""
			for _, blob := range before.Items {
				if blob.Path.Lexical == name {
					oldOID = blob.OID
				}
			}
			if oldOID == "" {
				t.Fatalf("configuration omitted: %s", name)
			}
			data := "after"
			if strings.HasSuffix(name, "project.assets.json") {
				project := c.Projects[c.Project]
				project.Assets = append(project.Assets, '\n')
				c.Projects[c.Project] = project
				data = string(project.Assets)
			}
			c.Sources[name] = dotnetSource(name, data)
			p, _ = newDotnet(t, c)
			after := dotnetNative(t, p, c.Key.Runner)[0].Loaded
			if after.State != testevidence.Present {
				t.Fatal(after)
			}
			found := false
			for _, blob := range after.Items {
				if blob.Path.Lexical == name {
					found = true
					if blob.OID == oldOID {
						t.Fatal("edit did not change loaded")
					}
				}
			}
			if !found {
				t.Fatal("edited configuration omitted")
			}
			source := c.Sources[name]
			source.Blob.OID = strings.Repeat("0", 40)
			c.Sources[name] = source
			p, _ = newDotnet(t, c)
			if loaded := dotnetNative(t, p, c.Key.Runner)[0].Loaded; loaded.State != testevidence.Incomplete || loaded.Reason == "" {
				t.Fatal(loaded)
			}
		})
	}
	c := dotnetConfig("dotnet-vstest-mstest")
	delete(c.Sources, "Tests/obj/project.assets.json")
	p, _ := newDotnet(t, c)
	if loaded := dotnetNative(t, p, c.Key.Runner)[0].Loaded; loaded.State != testevidence.Incomplete || loaded.Reason == "" {
		t.Fatal(loaded)
	}
}

func TestRSPV0005DotnetMissingCollectionRecord(t *testing.T) {
	c := dotnetConfig("dotnet-vstest-mstest")
	c.Collections[0].Tests = []string{"Demo.Probe.A"}
	p, key := newDotnet(t, c)
	in := dotnetInventory(t, p, key, c.Platform)
	records, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: c.Key.Runner, Artifacts: []string{"coverage"}})
	if err != nil || len(records) != len(in.Tests) {
		t.Fatalf("records=%+v err=%v", records, err)
	}
	for _, record := range records {
		var executed testevidence.Component[testevidence.ExecutedFile]
		if err := json.Unmarshal(record.Executed, &executed); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(record.Test, "Demo.Probe.B") {
			if executed.State != testevidence.Incomplete || executed.Reason != "dotnet-coverage-test-not-collected" {
				t.Fatal(executed)
			}
		} else if executed.State != testevidence.Present {
			t.Fatal(executed)
		}
	}
}

func TestRSPV0005DotnetVSTestAmbiguousAttachments(t *testing.T) {
	for _, paths := range [][]string{nil, {"one/coverage.cobertura.xml", "two/coverage.cobertura.xml"}} {
		if got, err := registry.DotnetVSTestArtifacts([]byte(dotnetAttachmentTRX(paths...))); err == nil || err.Error() == "" || got != nil {
			t.Fatal(got, err)
		}
	}
}

// RSP-V0-002/005: accepted inventory must be covered by the immutable binding
// map used to reconcile missing collections, even when MTP supplies a location.
func TestRSPV0005DotnetMTPLocationOnlyMissingCollection(t *testing.T) {
	for _, runner := range []string{"dotnet-mtp-mstest", "dotnet-mtp-nunit", "dotnet-mtp-xunit"} {
		for _, binding := range []string{"", "Tests/unknown.future"} {
			c := dotnetConfig(runner)
			c.Collections[0].Tests = []string{"Demo.Probe.A"}
			delete(c.Bindings, "Demo.Probe.B")
			if binding != "" {
				c.Bindings["Demo.Probe.B"] = binding
				c.Sources[binding] = dotnetSource(binding, "unknown source form")
			}
			p, key := newDotnet(t, c)
			in, err := p.Inventory(context.Background(), registry.InventoryRequest{Key: key, Output: dotnetList(c.Platform, "Demo.Probe.A", "Demo.Probe.B")})
			if err == nil || err.Error() != "dotnet-inventory-source-unresolved:"+binding || in.Complete {
				t.Fatalf("%s/%s: incomplete binding accepted: %+v %v", runner, binding, in, err)
			}
		}
		c := dotnetConfig(runner)
		c.Collections[0].Tests = []string{"Demo.Probe.A"}
		p, key := newDotnet(t, c)
		in := dotnetInventory(t, p, key, c.Platform)
		records, err := p.Evidence(context.Background(), registry.EvidenceRequest{Runner: runner, Artifacts: []string{"coverage"}})
		if err != nil || len(records) != len(in.Tests) {
			t.Fatalf("bound inventory lost a test: %+v %v", records, err)
		}
		found := false
		for _, record := range records {
			if !strings.HasSuffix(record.Test, "Demo.Probe.B") {
				continue
			}
			found = true
			var executed testevidence.Component[testevidence.ExecutedFile]
			if err := json.Unmarshal(record.Executed, &executed); err != nil {
				t.Fatal(err)
			}
			if executed.State != testevidence.Incomplete || executed.Reason != "dotnet-coverage-test-not-collected" {
				t.Fatal(executed)
			}
		}
		if !found {
			t.Fatal("missing collection identity disappeared")
		}
	}
}
