package appflows

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func executionFixture(t *testing.T) (string, NavigationOptions) {
	t.Helper()
	root, m := fixture(t)
	f := navSample("checkout")
	f.Links = nil
	f.Adapter = nil
	for i := range f.Navigation.Steps {
		f.Navigation.Steps[i].Ready = &NavLocator{TestID: "ready"}
		f.Navigation.Steps[i].Expect = []string{"paid"}
		f.Navigation.Steps[i].Effect = EffectRead
	}
	writeIntent(t, root, "flows/checkout.json", f)
	execution := NavigationExecution{Schema: NavigationExecutionSchema, Steps: []ExecutionStep{}}
	for _, id := range []string{"open", "pay"} {
		execution.Steps = append(execution.Steps, ExecutionStep{FlowID: "checkout", StepID: id, Operation: "observe", Observations: []ExecutionObservation{{OutcomeID: "paid", Condition: "visible", Locator: NavLocator{TestID: "paid"}}}})
	}
	raw, _ := json.Marshal(execution)
	writeRaw(t, root, "execution/steps.json", raw)
	origins, _ := json.Marshal(Origins{Schema: OriginsSchema, Disposable: []string{m.Manifest.Origin}})
	writeRaw(t, root, "flows/origins.json", origins)
	commitAll(t, root, "navigation")
	set, err := LoadIntentsAt(context.Background(), root, "flows", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	packet, err := FlowNavigationPacket(context.Background(), root, set, nil, nil, "checkout", EffectRead, "")
	if err != nil {
		t.Fatal(err)
	}
	packetPath := filepath.Join(t.TempDir(), "packet.json")
	if err = os.WriteFile(packetPath, packet, 0600); err != nil {
		t.Fatal(err)
	}
	return root, NavigationOptions{Manifest: "flows.json", Flows: "flows", Packet: packetPath, Execution: "execution/steps.json", MaxEffect: EffectRead}
}
func TestNavigationExecutionAdmission(t *testing.T) {
	t.Run("NEX-V0-001 exact committed packet", func(t *testing.T) {
		root, o := executionFixture(t)
		if _, err := captureNavigation(context.Background(), root, o); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("NEX-V0-001 edited grant cannot bypass", func(t *testing.T) {
		root, o := executionFixture(t)
		raw, _ := os.ReadFile(o.Packet)
		var packet NavigationPacket
		_ = json.Unmarshal(raw, &packet)
		packet.Steps[0].EffectClass = EffectWriteIrreversible
		packet.Steps[0].Grant = GrantGranted
		raw, _ = json.Marshal(packet)
		_ = os.WriteFile(o.Packet, raw, 0600)
		if _, err := captureNavigation(context.Background(), root, o); err == nil {
			t.Fatal("tampered packet admitted")
		}
	})
	t.Run("NEX-V0-005 stale packet", func(t *testing.T) {
		root, o := executionFixture(t)
		writeRaw(t, root, "new.txt", []byte("changed"))
		commitAll(t, root, "drift")
		if _, err := captureNavigation(context.Background(), root, o); err == nil {
			t.Fatal("stale packet admitted")
		}
	})
	t.Run("NEX-V0-005 dirty execution input", func(t *testing.T) {
		root, o := executionFixture(t)
		raw, _ := os.ReadFile(filepath.Join(root, o.Execution))
		writeRaw(t, root, o.Execution, append(raw, ' '))
		if _, err := captureNavigation(context.Background(), root, o); err == nil {
			t.Fatal("dirty input admitted")
		}
	})
	t.Run("NEX-V0-005 dirty origins", func(t *testing.T) {
		root, o := executionFixture(t)
		writeRaw(t, root, "flows/origins.json", []byte(`{"schema":"application-flow-origins/0","disposable":[]}`))
		if _, err := captureNavigation(context.Background(), root, o); err == nil {
			t.Fatal("dirty origins admitted")
		}
	})
	t.Run("NEX-V0-005 dirty source", func(t *testing.T) {
		root, o := executionFixture(t)
		writeRaw(t, root, "app.js", []byte("changed"))
		if _, err := captureNavigation(context.Background(), root, o); err == nil {
			t.Fatal("dirty source admitted")
		}
	})
}
func TestNavigationExecutionClosedMapping(t *testing.T) {
	root, o := executionFixture(t)
	in, err := captureNavigation(context.Background(), root, o)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*navigationInput){
		"NEX-V0-002 prose action":    func(n *navigationInput) { n.Execution.Steps[0].Operation = "execute prose" },
		"NEX-V0-002 extra step":      func(n *navigationInput) { n.Execution.Steps = append(n.Execution.Steps, n.Execution.Steps[0]) },
		"NEX-V0-002 unknown outcome": func(n *navigationInput) { n.Execution.Steps[0].Observations[0].OutcomeID = "unknown" },
		"NEX-V0-002 readiness":       func(n *navigationInput) { n.Packet.Steps[0].Ready = nil },
		"NEX-V0-002 fixture": func(n *navigationInput) {
			n.Execution.Steps[0].Operation = "fill"
			n.Packet.Steps[0].InputFixture = "missing"
		},
		"NEX-V0-002 route": func(n *navigationInput) { n.Packet.Steps[0].State = "ui://outside" },
		"NEX-V0-002 api":   func(n *navigationInput) { n.Packet.Steps[0].State = "api:GET /x" },
		"NEX-V0-003 grant": func(n *navigationInput) { n.Packet.Steps[0].EffectClass = EffectWriteIrreversible },
		"NEX-V0-003 origin": func(n *navigationInput) {
			n.MaxEffect = EffectWriteIrreversible
			n.Packet.Steps[0].EffectClass = EffectWriteIrreversible
			n.Origins.Disposable = nil
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			raw, _ := json.Marshal(in)
			var copy navigationInput
			_ = json.Unmarshal(raw, &copy)
			mutate(&copy)
			if validateExecution(copy) == nil {
				t.Fatal("unsupported execution accepted")
			}
		})
	}
}
func TestNavigationProviderIdentity(t *testing.T) {
	t.Run("NEX-V0-005 execution asset drift", func(t *testing.T) {
		assets := t.TempDir()
		for _, p := range []string{"observe.mjs", "scan.mjs", "lifecycle.mjs", "package-lock.json", "navigation.mjs", "navigation-policy.mjs"} {
			_ = os.WriteFile(filepath.Join(assets, p), []byte("original"), 0600)
		}
		before, err := providerIdentity(assets, "navigation.mjs", "navigation-policy.mjs")
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(assets, "navigation-policy.mjs"), []byte("changed"), 0600)
		after, err := providerIdentity(assets, "navigation.mjs", "navigation-policy.mjs")
		if err != nil || before == after {
			t.Fatal("execution provider drift not detected")
		}
	})
}

func TestNavigationReceiptCannotInventSuccess(t *testing.T) {
	t.Run("NEX-V0-006 closed receipt and retained failure", func(t *testing.T) {
		root, o := executionFixture(t)
		in, err := captureNavigation(context.Background(), root, o)
		if err != nil {
			t.Fatal(err)
		}
		receipt := NavigationReceipt{Schema: NavigationReceiptSchema, Authority: "CALLER_REPORTED", Revision: in.Packet.Revision, Binding: in.Binding, RunID: "run", PacketDigest: in.PacketDigest, ExecutionDigest: in.ExecutionDigest, OriginsDigest: in.OriginsDigest, PlaywrightVersion: "1.63.0", NodeVersion: "v22.23.3", Browser: "chromium-153.0.8010.12", Status: "passed", Cleanup: true, BrowserClosed: true, ServerExited: true, Steps: []NavigationStepResult{}}
		for _, s := range in.Packet.Steps {
			receipt.Steps = append(receipt.Steps, NavigationStepResult{FlowID: s.FlowID, StepID: s.StepID, Outcome: "passed", Reason: "none", Verification: s.Verification})
		}
		if err = validateNavigationReceipt(in, "run", receipt); err != nil {
			t.Fatal(err)
		}
		receipt.Traffic = []NavigationTraffic{{Method: "POST", Effect: EffectWriteIrreversible, Allowed: false}}
		if validateNavigationReceipt(in, "run", receipt) == nil {
			t.Fatal("blocked traffic passed")
		}
		receipt.Traffic = nil
		receipt.Steps = receipt.Steps[:1]
		if validateNavigationReceipt(in, "run", receipt) == nil {
			t.Fatal("missing step passed")
		}
		receipt.Status = "incomplete"
		receipt.Gaps = []string{"raw DOM prose"}
		if validateNavigationReceipt(in, "run", receipt) == nil {
			t.Fatal("unbounded reason accepted")
		}
	})
}
