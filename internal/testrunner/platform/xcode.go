package platform

import (
	"encoding/json"
	"fmt"
	"strings"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

type xcodeNode struct {
	Name     string      `json:"name"`
	Kind     string      `json:"nodeType"`
	ID       string      `json:"nodeIdentifierURL"`
	Result   string      `json:"result"`
	Children []xcodeNode `json:"children"`
	Source   struct {
		File string `json:"filePath"`
	} `json:"sourceLocation"`
}

func parseXcode(in tr.Input, o *tr.Observation) error {
	if len(in.Reports) != 2 {
		return fmt.Errorf("Xcode requires both native tests and summary reports")
	}
	b, ok := in.Reports["xcode-tests.json"]
	if !ok {
		return fmt.Errorf("Xcode tests report missing")
	}
	if err := checkJSON(b); err != nil {
		return err
	}
	var tree struct {
		Nodes   []xcodeNode `json:"testNodes"`
		Devices []struct {
			ID       string `json:"deviceId"`
			Platform string `json:"platform"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(b, &tree); err != nil {
		return err
	}
	if len(tree.Devices) != 1 || tree.Devices[0].ID == "" || tree.Devices[0].Platform != "macOS" {
		return fmt.Errorf("Xcode profile requires one observed macOS destination")
	}
	b, ok = in.Reports["xcode-summary.json"]
	if !ok {
		return fmt.Errorf("Xcode summary report missing")
	}
	if err := checkJSON(b); err != nil {
		return err
	}
	var summary struct {
		Total    *int     `json:"totalTestCount"`
		Passed   *int     `json:"passedTests"`
		Failed   *int     `json:"failedTests"`
		Skipped  *int     `json:"skippedTests"`
		Expected *int     `json:"expectedFailures"`
		Start    *float64 `json:"startTime"`
		Finish   *float64 `json:"finishTime"`
		Result   string   `json:"result"`
		Failures []struct {
			ID   string `json:"testIdentifierURL"`
			Text string `json:"failureText"`
		} `json:"testFailures"`
		Destinations []struct {
			Device struct {
				ID string `json:"deviceId"`
			} `json:"device"`
		} `json:"devicesAndConfigurations"`
	}
	if err := json.Unmarshal(b, &summary); err != nil {
		return err
	}
	if summary.Total == nil || summary.Passed == nil || summary.Failed == nil || summary.Skipped == nil || summary.Expected == nil || summary.Start == nil || summary.Finish == nil || *summary.Finish < *summary.Start || *summary.Total < 0 || *summary.Total > tr.MaxTests {
		return fmt.Errorf("Xcode summary lifecycle/counts incomplete")
	}
	if len(summary.Destinations) != 1 || summary.Destinations[0].Device.ID != tree.Devices[0].ID {
		return fmt.Errorf("Xcode destination reports disagree")
	}
	if *summary.Expected != 0 {
		problem(o, "EXPECTED_FAILURES", "Xcode expected failures do not establish passing criteria")
	}
	failIDs := map[string]bool{}
	for _, f := range summary.Failures {
		if f.ID == "" || failIDs[f.ID] {
			return fmt.Errorf("Xcode duplicate/absent failure identity")
		}
		failIDs[f.ID] = true
	}
	counts := map[string]int{}
	var walk func([]xcodeNode, int) error
	walk = func(nodes []xcodeNode, depth int) error {
		if depth > 16 {
			return fmt.Errorf("Xcode tree depth bound")
		}
		for _, n := range nodes {
			switch n.Kind {
			case "Test Plan", "Unit test bundle", "Test Suite":
				if n.Result != "Passed" && n.Result != "Failed" && n.Result != "Skipped" {
					return fmt.Errorf("unknown Xcode container result")
				}
				if len(n.Children) == 0 {
					return fmt.Errorf("Xcode empty container")
				}
				first := len(o.Tests)
				if err := walk(n.Children, depth+1); err != nil {
					return err
				}
				failed, allSkipped := false, true
				for _, child := range o.Tests[first:] {
					failed = failed || child.State == tr.Failed
					allSkipped = allSkipped && child.State == tr.Skipped
				}
				if (n.Result == "Failed") != failed || (n.Result == "Skipped" && !allSkipped) {
					return fmt.Errorf("Xcode container result contradicts descendants")
				}
			case "Test Case":
				if !strings.HasPrefix(n.ID, "test://com.apple.xcode/") || n.Name == "" {
					return fmt.Errorf("Xcode test identity missing")
				}
				state, kind, message, file := tr.Unknown, "", "", ""
				switch n.Result {
				case "Passed":
					state = tr.Passed
				case "Failed":
					state = tr.Failed
					kind = tr.Unknown
				case "Skipped":
					state = tr.Skipped
				default:
					return fmt.Errorf("unknown/flaky/repeated Xcode case requires qualified run identities")
				}
				counts[state]++
				for _, c := range n.Children {
					if c.Kind == "Failure Message" && state == tr.Failed {
						if strings.HasPrefix(c.Name, "XCTAssert") || strings.HasPrefix(c.Name, "XCTFail") {
							kind = tr.Assertion
						}
						message += c.Name + "\n"
						file = c.Source.File
					} else if c.Kind == "Skip Message" && state == tr.Skipped {
						message += c.Name + "\n"
					} else {
						return fmt.Errorf("unhandled/contradictory Xcode case child")
					}
				}
				if (state == tr.Failed) != failIDs[n.ID] {
					return fmt.Errorf("Xcode failed-test identity mismatch")
				}
				if state == tr.Failed && message == "" {
					return fmt.Errorf("Xcode failed test has no failure observation")
				}
				o.Tests = append(o.Tests, test(n.ID, n.Name, file, "", state, kind, strings.TrimSpace(message)))
			default:
				return fmt.Errorf("unsupported Xcode node %q", n.Kind)
			}
		}
		return nil
	}
	if err := walk(tree.Nodes, 0); err != nil {
		return err
	}
	if len(o.Tests) != *summary.Total || counts[tr.Passed] != *summary.Passed || counts[tr.Failed] != *summary.Failed || counts[tr.Skipped] != *summary.Skipped || len(failIDs) != *summary.Failed {
		return fmt.Errorf("Xcode native result counts disagree")
	}
	failed := counts[tr.Failed] > 0
	if (failed && (summary.Result != "Failed" || in.ExitCode != 65)) || (!failed && (summary.Result != "Passed" || in.ExitCode != 0)) {
		problem(o, "EXIT_CONTRADICTION", "Xcode report and test process exit disagree")
	}
	o.Complete = len(o.Problems) == 0
	return nil
}
