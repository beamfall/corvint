package store

import (
	"context"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"time"
)

func ProgramRecords(ctx context.Context, repo *intent.Repository) ([]snapshot.Program, error) {
	p, e := readLeaseProof(ctx, repo)
	if e != nil {
		return nil, e
	}
	raw := p.Records["programs.json"].Raw
	if len(raw) == 0 {
		return []snapshot.Program{}, nil
	}
	s, e := snapshot.DecodePrograms(raw)
	if e != nil {
		return nil, e
	}
	return s.Entries, nil
}
func RequestProgramControl(ctx context.Context, repo *intent.Repository, actor mutation.Binding, id, control string) error {
	entries, e := ProgramRecords(ctx, repo)
	if e != nil {
		return e
	}
	proof, e := readLeaseProof(ctx, repo)
	if e != nil {
		return e
	}
	q, e := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if e != nil {
		return e
	}
	for _, p := range entries {
		if p.ID != id {
			continue
		}
		p.Control = control
		r, e := ProgramTransition(ctx, repo, actor, q.QueueID.Raw, id+"-control-"+fmt.Sprint(time.Now().UnixNano()), p)
		if e = transitionOK(r, e); e != nil {
			return e
		}
		until := time.NewTimer(15 * time.Second)
		defer until.Stop()
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			current, e := ProgramRecords(ctx, repo)
			if e != nil {
				return e
			}
			found := false
			for _, now := range current {
				if now.ID != id {
					continue
				}
				found = true
				if now.Assignment != p.Assignment || now.CurrentAttempt != p.CurrentAttempt || now.CurrentGeneration != p.CurrentGeneration || now.Epoch != p.Epoch {
					return fmt.Errorf("control target assignment/epoch changed")
				}
				if now.Control != control {
					return fmt.Errorf("control superseded")
				}
				if now.OwnerReleased && now.Quiescence == "PROVED" && now.Phase == "FINISHED" {
					return nil
				}
			}
			if !found {
				return fmt.Errorf("control target disappeared")
			}
			identity, e := supervisor.ProcessIdentity(p.OwnerPID)
			if e != nil {
				return e
			}
			if identity != p.OwnerStarted {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-until.C:
				return fmt.Errorf("control retained; original owner has not retired")
			case <-tick.C:
			}
		}
	}
	return fmt.Errorf("program absent")
}

func ProgramAttempts(ctx context.Context, repo *intent.Repository) (map[string]*snapshot.Attempt, error) {
	proof, e := readLeaseProof(ctx, repo)
	if e != nil {
		return nil, e
	}
	out := map[string]*snapshot.Attempt{}
	raw := proof.Records["programs.json"].Raw
	if len(raw) == 0 {
		return out, nil
	}
	ps, e := snapshot.DecodePrograms(raw)
	if e != nil {
		return nil, e
	}
	for _, p := range ps.Entries {
		if p.CurrentAttempt == "" {
			continue
		}
		r, ok := proof.Records["attempts/"+p.CurrentAttempt+".json"]
		if !ok {
			return nil, fmt.Errorf("current attempt absent")
		}
		a, e := snapshot.DecodeAttempt(r.Raw)
		if e != nil || string(a.Generation) != p.CurrentGeneration {
			return nil, fmt.Errorf("current generation differs")
		}
		out[p.ID] = a
	}
	return out, nil
}
