package postmergeconnector

import (
	"context"
	"fmt"
	"io"
)

// Execute validates the entire plan before the first effect. Remote providers
// must honor stable-key upsert after partial failure; no transaction is claimed.
func Execute(ctx context.Context, root string, f Fixture, p Policy, plan Plan, tracker, forge Writer) error {
	if err := ValidatePlan(ctx, root, f, p, plan); err != nil {
		return err
	}
	if tracker == nil || forge == nil {
		return fmt.Errorf("writer-missing")
	}
	for _, r := range plan.Requests {
		if err := ctx.Err(); err != nil {
			return err
		}
		writer := tracker
		if r.Operation == "upsert-draft-change" {
			writer = forge
		}
		if err := writer.Upsert(r); err != nil {
			return fmt.Errorf("connector-write-failed: %s", r.Key)
		}
	}
	return nil
}
func JSONL(requests []Request) ([]byte, error) {
	out := []byte{}
	for _, r := range requests {
		b, e := Encode(r)
		if e != nil {
			return nil, e
		}
		out = append(out, b...)
	}
	if len(out) > MaxBytes {
		return nil, fmt.Errorf("request-output-too-large")
	}
	return out, nil
}
func DryRun(ctx context.Context, root string, f Fixture, p Policy, plan Plan, w io.Writer) error {
	if err := ValidatePlan(ctx, root, f, p, plan); err != nil {
		return err
	}
	b, err := JSONL(plan.Requests)
	if err != nil {
		return err
	}
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return err
}

type memoryWriter struct{ values map[string]Request }

func (w memoryWriter) Upsert(r Request) error { w.values[r.Key] = r; return nil }
func NewState() State {
	return State{Profile: Profile, Tracker: map[string]Request{}, Forge: map[string]Request{}}
}
func ApplyLocal(ctx context.Context, root string, f Fixture, p Policy, plan Plan, state State) (State, error) {
	if state.Profile != Profile || state.Tracker == nil || state.Forge == nil {
		return State{}, fmt.Errorf("state-invalid")
	}
	// Work on independent maps so a rejected batch leaves its caller's state intact.
	next := NewState()
	for k, v := range state.Tracker {
		if k != v.Key || v.Profile != Profile || v.Operation == "upsert-draft-change" {
			return State{}, fmt.Errorf("tracker-state-invalid")
		}
		next.Tracker[k] = v
	}
	for k, v := range state.Forge {
		if k != v.Key || v.Profile != Profile || v.Operation != "upsert-draft-change" || !v.Draft {
			return State{}, fmt.Errorf("forge-state-invalid")
		}
		next.Forge[k] = v
	}
	if err := Execute(ctx, root, f, p, plan, memoryWriter{next.Tracker}, memoryWriter{next.Forge}); err != nil {
		return State{}, err
	}
	return next, nil
}
