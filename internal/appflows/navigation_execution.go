package appflows

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

const NavigationExecutionSchema = "application-navigation-execution-input/0"
const NavigationReceiptSchema = "application-navigation-execution-receipt/0"

var navigationNodeVersion = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
var navigationBrowserVersion = regexp.MustCompile(`^chromium-[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$`)

type NavigationExecution struct {
	Schema string          `json:"schema"`
	Steps  []ExecutionStep `json:"steps"`
}
type ExecutionStep struct {
	FlowID       string                 `json:"flow_id"`
	StepID       string                 `json:"step_id"`
	Operation    string                 `json:"operation"`
	Observations []ExecutionObservation `json:"observations"`
}
type ExecutionObservation struct {
	OutcomeID string     `json:"outcome_id"`
	Condition string     `json:"condition"`
	Locator   NavLocator `json:"locator"`
}

type NavigationOptions struct {
	Manifest, Assets, Flows, Packet, Execution, Fixtures, MaxEffect string
	Timeout                                                         time.Duration
}

type navigationInput struct {
	Input
	Packet          NavigationPacket    `json:"packet"`
	Execution       NavigationExecution `json:"execution"`
	Origins         Origins             `json:"origins"`
	Fixtures        map[string]string   `json:"fixtures"`
	PacketDigest    string              `json:"packetDigest"`
	ExecutionDigest string              `json:"executionDigest"`
	OriginsDigest   string              `json:"originsDigest"`
	MaxEffect       string              `json:"maxEffect"`
}

type NavigationStepResult struct {
	FlowID       string `json:"flow_id"`
	StepID       string `json:"step_id"`
	Outcome      string `json:"outcome"`
	Reason       string `json:"reason"`
	Verification string `json:"verification"`
}
type NavigationTraffic struct {
	FlowID     string `json:"flow_id"`
	StepID     string `json:"step_id"`
	Method     string `json:"method"`
	FormSubmit bool   `json:"form_submit"`
	Effect     string `json:"effect"`
	Allowed    bool   `json:"allowed"`
}
type NavigationReceipt struct {
	Schema            string                 `json:"schema"`
	Authority         string                 `json:"authority"`
	Revision          string                 `json:"revision"`
	Binding           Binding                `json:"binding"`
	RunID             string                 `json:"runId"`
	PacketDigest      string                 `json:"packetDigest"`
	ExecutionDigest   string                 `json:"executionDigest"`
	OriginsDigest     string                 `json:"originsDigest"`
	ProviderDigest    string                 `json:"providerDigest"`
	NodeVersion       string                 `json:"nodeVersion"`
	PlaywrightVersion string                 `json:"playwrightVersion"`
	Browser           string                 `json:"browser"`
	Status            string                 `json:"status"`
	Steps             []NavigationStepResult `json:"steps"`
	Traffic           []NavigationTraffic    `json:"traffic"`
	Gaps              []string               `json:"gaps"`
	BrowserClosed     bool                   `json:"browserClosed"`
	ServerExited      bool                   `json:"serverExited"`
	Cleanup           bool                   `json:"cleanup"`
}

// CaptureNavigation rederives all executable inputs at one immutable commit before startup.
func captureNavigation(ctx context.Context, root string, o NavigationOptions) (navigationInput, error) {
	var in navigationInput
	bad := func() (navigationInput, error) { return in, errors.New("navigation-input-refused") }
	rev, err := ResolveRevision(ctx, root, "HEAD")
	if err != nil {
		return in, err
	}
	in.Input, err = Capture(ctx, root, o.Manifest)
	if err != nil {
		return in, err
	}
	raw, err := ReadFile(o.Packet)
	if err != nil {
		return in, err
	}
	if err = Decode(raw, &in.Packet); err != nil {
		return in, err
	}
	if in.Packet.Revision != rev || effectRank[o.MaxEffect] == 0 {
		return bad()
	}
	set, err := LoadIntentsAt(ctx, root, o.Flows, rev)
	if err != nil {
		return in, err
	}
	canonical, err := FlowNavigationPacket(ctx, root, set, nil, nil, in.Packet.Goal, o.MaxEffect, "")
	if err != nil {
		return in, err
	}
	var expected NavigationPacket
	if err = Decode(canonical, &expected); err != nil {
		return in, err
	}
	if !reflect.DeepEqual(in.Packet, expected) {
		return bad()
	}
	in.PacketDigest = Digest(canonical)
	in.MaxEffect = o.MaxEffect
	in.Origins, err = LoadOriginsAt(ctx, root, o.Flows, rev)
	if err != nil {
		return in, err
	}
	originEntries, originErr := flowTree(ctx, root, rev, o.Flows)
	if originErr != nil {
		return in, originErr
	}
	for _, entry := range originEntries {
		if entry.name == OriginsFile {
			committed, readErr := committedBlob(ctx, root, entry)
			materialized, currentErr := readSource(root, path.Join(o.Flows, OriginsFile))
			if readErr != nil || currentErr != nil || !bytes.Equal(committed, materialized) {
				return bad()
			}
		}
	}
	originsRaw, _ := json.Marshal(in.Origins)
	in.OriginsDigest = Digest(originsRaw)
	if !safePath(o.Execution) || o.Execution == "." {
		return bad()
	}
	entries, err := flowTree(ctx, root, rev, path.Dir(o.Execution))
	if err != nil {
		return in, err
	}
	at := slices.IndexFunc(entries, func(e treeEntry) bool { return e.name == path.Base(o.Execution) })
	if at < 0 {
		return bad()
	}
	executionRaw, err := committedBlob(ctx, root, entries[at])
	if err != nil {
		return in, err
	}
	current, err := readSource(root, o.Execution)
	if err != nil || !bytes.Equal(current, executionRaw) {
		return bad()
	}
	in.ExecutionDigest = Digest(executionRaw)
	if err = Decode(executionRaw, &in.Execution); err != nil {
		return in, err
	}
	in.Fixtures = map[string]string{}
	if o.Fixtures != "" {
		fixtureRaw, readErr := ReadFile(o.Fixtures)
		if readErr != nil {
			return in, readErr
		}
		// Fixture values are private data, not source evidence. Never echo or hash them.
		_, parseErr := wire.Parse(fixtureRaw)
		if len(fixtureRaw) > 65536 || parseErr != nil || json.Unmarshal(fixtureRaw, &in.Fixtures) != nil {
			return bad()
		}
	}
	if err = validateExecution(in); err != nil {
		return in, err
	}
	after, err := ResolveRevision(ctx, root, "HEAD")
	if err != nil || after != rev {
		return bad()
	}
	return in, nil
}

func validateExecution(in navigationInput) error {
	bad := errors.New("navigation-execution-refused")
	all := append(slices.Clone(in.Packet.Steps), in.Packet.Recovery...)
	if in.Execution.Schema != NavigationExecutionSchema || len(all) == 0 || len(in.Execution.Steps) != len(all) || len(all) > 256 || len(in.Fixtures) > 256 {
		return bad
	}
	byID := map[string]ExecutionStep{}
	for _, e := range in.Execution.Steps {
		key := e.FlowID + "/" + e.StepID
		if _, ok := byID[key]; ok {
			return bad
		}
		byID[key] = e
	}
	used := map[string]bool{}
	for _, s := range all {
		e, ok := byID[s.FlowID+"/"+s.StepID]
		if !ok || !slices.Contains([]string{"navigate", "fill", "click", "observe"}, e.Operation) {
			return bad
		}
		route := strings.TrimPrefix(s.State, "ui:")
		if !strings.HasPrefix(s.State, "ui:/") || strings.HasPrefix(route, "//") || strings.ContainsAny(route, "{}%?#\\") || !templatePattern.MatchString(route) || s.Locator == nil || !uiLocator(*s.Locator) || s.Ready == nil || !uiLocator(*s.Ready) {
			return bad
		}
		if effectRank[s.EffectClass] == 0 || effectRank[s.EffectClass] > effectRank[in.MaxEffect] || s.Grant != GrantGranted {
			return errors.New("navigation-effect-not-granted")
		}
		if err := AdmitTransition(in.Origins, in.Manifest.Origin, s.EffectClass); err != nil {
			return err
		}
		if (e.Operation == "fill") != (s.InputFixture != "") {
			return bad
		}
		if s.InputFixture != "" {
			v, ok := in.Fixtures[s.InputFixture]
			if !ok || len(v) > 8192 {
				return errors.New("navigation-fixture-unavailable")
			}
			used[s.InputFixture] = true
		}
		if len(s.Expect) == 0 || len(e.Observations) != len(s.Expect) {
			return bad
		}
		seen := map[string]bool{}
		for _, ob := range e.Observations {
			if seen[ob.OutcomeID] || !slices.Contains([]string{"visible", "hidden"}, ob.Condition) || !uiLocator(ob.Locator) || !slices.ContainsFunc(s.Expect, func(out FlowOutcome) bool { return out.OutcomeID == ob.OutcomeID }) {
				return bad
			}
			seen[ob.OutcomeID] = true
		}
	}
	for id := range in.Fixtures {
		if !used[id] {
			return bad
		}
	}
	for _, s := range in.Packet.Recovery {
		if s.Recovery != "" {
			return bad
		}
	}
	return nil
}

// ObserveNavigation is separate from packet generation and emits no test-run evidence.
func ObserveNavigation(ctx context.Context, root string, o NavigationOptions) (NavigationReceipt, error) {
	var receipt NavigationReceipt
	if o.Timeout <= 0 || o.Timeout > 5*time.Minute {
		return receipt, errors.New("navigation-timeout-invalid")
	}
	in, err := captureNavigation(ctx, root, o)
	if err != nil {
		return receipt, err
	}
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return receipt, err
	}
	in.RunID = hex.EncodeToString(nonce[:])
	in.Observe = true
	raw, err := json.Marshal(in)
	if err != nil {
		return receipt, err
	}
	before, err := providerIdentity(o.Assets, "navigation.mjs", "navigation-policy.mjs")
	if err != nil {
		return receipt, err
	}
	output, err := runObserver(ctx, root, o.Assets, "navigation.mjs", raw, o.Timeout)
	if err != nil {
		return receipt, err
	}
	if err = Decode(output, &receipt); err != nil {
		return receipt, err
	}
	after, err := captureNavigation(ctx, root, o)
	if err != nil {
		return NavigationReceipt{}, errors.New("navigation-source-drift")
	}
	in.Input.RunID = ""
	in.Input.Observe = false
	if !reflect.DeepEqual(in, after) {
		return NavigationReceipt{}, errors.New("navigation-input-drift")
	}
	provider, err := providerIdentity(o.Assets, "navigation.mjs", "navigation-policy.mjs")
	if err != nil || before != provider {
		return NavigationReceipt{}, errors.New("navigation-provider-drift")
	}
	receipt.ProviderDigest = provider
	receipt.Cleanup = true
	if err = validateNavigationReceipt(in, hex.EncodeToString(nonce[:]), receipt); err != nil {
		return NavigationReceipt{}, err
	}
	return receipt, nil
}

func validateNavigationReceipt(in navigationInput, runID string, r NavigationReceipt) error {
	bad := errors.New("navigation-receipt-invalid")
	if r.Schema != NavigationReceiptSchema || r.Authority != "CALLER_REPORTED" || r.Revision != in.Packet.Revision || !reflect.DeepEqual(r.Binding, in.Binding) || r.RunID != runID || r.PacketDigest != in.PacketDigest || r.ExecutionDigest != in.ExecutionDigest || r.OriginsDigest != in.OriginsDigest || r.PlaywrightVersion != "1.63.0" || len(r.NodeVersion) > 64 || !navigationNodeVersion.MatchString(r.NodeVersion) || !navigationBrowserVersion.MatchString(r.Browser) || len(r.Browser) > 64 || len(r.Traffic) > 8192 || len(r.Gaps) > 32 || !r.Cleanup || !r.BrowserClosed || !r.ServerExited {
		return bad
	}
	if !slices.Contains([]string{"passed", "incomplete"}, r.Status) {
		return bad
	}
	all := append(slices.Clone(in.Packet.Steps), in.Packet.Recovery...)
	seen := map[string]bool{}
	for _, s := range r.Steps {
		at := slices.IndexFunc(all, func(t Transition) bool { return t.FlowID == s.FlowID && t.StepID == s.StepID })
		key := s.FlowID + "/" + s.StepID
		if at < 0 || seen[key] || s.Verification != all[at].Verification || !slices.Contains([]string{"passed", "failed", "recovered"}, s.Outcome) || !slices.Contains([]string{"none", "step-failed", "request-blocked"}, s.Reason) {
			return bad
		}
		seen[key] = true
	}
	for _, g := range r.Gaps {
		if !slices.Contains([]string{"request-blocked", "traffic-budget", "form-blocked", "identity-drift", "execution-failed", "browser-close-unobserved", "server-exit-unobserved", "websocket-blocked"}, g) {
			return bad
		}
	}
	for _, tr := range r.Traffic {
		if !methodPattern.MatchString(tr.Method) || effectRank[tr.Effect] == 0 {
			return bad
		}
		if tr.FlowID != "" || tr.StepID != "" {
			if !slices.ContainsFunc(all, func(t Transition) bool { return t.FlowID == tr.FlowID && t.StepID == tr.StepID }) {
				return bad
			}
		}
		if tr.Allowed && (effectRank[tr.Effect] > effectRank[in.MaxEffect] || AdmitTransition(in.Origins, in.Manifest.Origin, tr.Effect) != nil) {
			return bad
		}
	}
	if r.Status == "passed" {
		if len(r.Gaps) != 0 || len(r.Steps) != len(in.Packet.Steps) {
			return bad
		}
		for i, s := range r.Steps {
			expected := in.Packet.Steps[i]
			if s.FlowID != expected.FlowID || s.StepID != expected.StepID || s.Outcome != "passed" || s.Reason != "none" {
				return bad
			}
		}
		if slices.ContainsFunc(r.Traffic, func(t NavigationTraffic) bool { return !t.Allowed }) {
			return bad
		}
	}
	return nil
}
