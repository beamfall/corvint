package main

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/Beamfall/corvint/internal/pulse"
)

type pulseWorkspaceActor struct {
	actor *pulse.Actor
	root  string
}

func newWorkspaceActor(root, sessionID string) (workspaceActor, error) {
	actor, err := pulse.NewActor(pulse.Config{
		SessionID: sessionID,
		Capture: func(context.Context, uint64) (pulse.Manifest, error) {
			return pulse.Manifest{}, errors.New("P0-A snapshot capture is unavailable")
		},
		MaxRetainedSnapshots: 1,
		MaxRetainedBytes:     1,
	})
	if err != nil {
		return nil, fmt.Errorf("create Pulse actor: %w", err)
	}
	return &pulseWorkspaceActor{actor: actor, root: root}, nil
}

func (a *pulseWorkspaceActor) Invalidate(ctx context.Context, expectedWireGeneration uint64) (uint64, bool, error) {
	if expectedWireGeneration == math.MaxUint64 {
		return 0, false, errors.New("wire generation cannot be translated")
	}
	update, err := a.actor.Invalidate(ctx, expectedWireGeneration+1)
	wireGeneration, translatedErr := translateActorResult(update, err)
	if translatedErr != nil && !errors.Is(translatedErr, pulse.ErrGenerationConflict) {
		return wireGeneration, false, translatedErr
	}
	if errors.Is(translatedErr, pulse.ErrGenerationConflict) {
		return wireGeneration, false, nil
	}
	return wireGeneration, true, nil
}

func translateActorResult(update pulse.Update, err error) (uint64, error) {
	var actorErr *pulse.ActorError
	if err != nil && errors.As(err, &actorErr) {
		if actorErr.WorkspaceGeneration == 0 {
			return 0, errors.New("Pulse actor returned invalid error coordinates")
		}
		wireGeneration := actorErr.WorkspaceGeneration - 1
		if update.WorkspaceGeneration != 0 &&
			(actorErr.WorkspaceGeneration != update.WorkspaceGeneration ||
				actorErr.ServerSequence != update.ServerSequence) {
			return wireGeneration, errors.New("Pulse actor returned inconsistent error coordinates")
		}
		return wireGeneration, &pulse.ActorError{
			Cause:               actorErr.Cause,
			WorkspaceGeneration: wireGeneration,
			ServerSequence:      actorErr.ServerSequence,
		}
	}
	if update.WorkspaceGeneration == 0 {
		if err != nil {
			return 0, err
		}
		return 0, errors.New("Pulse actor returned an invalid generation")
	}
	wireGeneration := update.WorkspaceGeneration - 1
	if err != nil {
		return wireGeneration, err
	}
	return wireGeneration, nil
}

func (a *pulseWorkspaceActor) Shutdown(ctx context.Context) error {
	_, err := a.actor.Shutdown(ctx)
	return err
}
