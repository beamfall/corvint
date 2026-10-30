package jstestprovider

import (
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// externalTableNodes derives, per runner, the Node releases the shared table
// admits for any external profile with the given browser source.
func externalTableNodes(source string) map[string][]string {
	nodes := map[string][]string{}
	for _, row := range admittedRuntimeTuples {
		if isExternalProfile(row.profile) && row.source == source && !slices.Contains(nodes[row.runner], row.node) {
			nodes[row.runner] = append(nodes[row.runner], row.node)
		}
	}
	for runner := range nodes {
		slices.Sort(nodes[runner])
	}
	return nodes
}

// PWP-V0-054: the reporter's external Node pre-filter is exactly the shared
// table's external-profile (runner, Node) rows per browser source, declared
// once; any other Node release literal in the reporter fails the test.
func TestPWPV0054ReporterExternalNodesMatchTable(t *testing.T) {
	reporter := string(qualifiedReporter)
	declarations := map[string]string{"externalBundledNodes": BrowserSourceBundled, "externalSystemNodes": BrowserSourceSystem}
	rest := reporter
	for name, source := range declarations {
		match := regexp.MustCompile(`(?m)^const ` + name + ` = (\{.*\});$`).FindStringSubmatch(reporter)
		if match == nil {
			t.Fatalf("reporter lacks the %s declaration", name)
		}
		var got map[string][]string
		if err := json.Unmarshal([]byte(strings.ReplaceAll(match[1], "'", `"`)), &got); err != nil {
			t.Fatalf("%s is not a runner-to-Node map: %v", name, err)
		}
		for runner := range got {
			slices.Sort(got[runner])
		}
		if want := externalTableNodes(source); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s %v drifted from the shared table %v", name, got, want)
		}
		rest = strings.Replace(rest, match[0], "", 1)
	}
	if stray := regexp.MustCompile(`['"\x60]v\d+\.\d+\.\d+['"\x60]`).FindAllString(rest, -1); len(stray) > 0 {
		t.Fatalf("reporter keeps Node literals outside the table copies: %v", stray)
	}
}
