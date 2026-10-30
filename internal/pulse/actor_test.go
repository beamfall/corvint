package pulse

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"testing"
	"time"
)

func manifest(generation uint64) Manifest {
	return Manifest{
		WSI:               fmt.Sprintf("wsi-%d", generation),
		Digest:            fmt.Sprintf("digest-%d", generation),
		ObservationWindow: ObservationWindow{StartSequence: generation*2 - 1, EndSequence: generation * 2},
		ObservationModel:  "double-observation-v1",
		RetainedBytes:     8,
	}
}

func newTestActor(t *testing.T, session string, capture CaptureFunc) *Actor {
	t.Helper()
	actor, err := NewActor(Config{
		SessionID:            session,
		Capture:              capture,
		MaxRetainedSnapshots: 4,
		MaxRetainedBytes:     4096,
	})
	if err != nil {
		t.Fatalf("NewActor: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_, _ = actor.Shutdown(ctx)
	})
	return actor
}

func TestAcquireThirtyTwoWaySingleflight(t *testing.T) {
	var captures atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	actor := newTestActor(t, "singleflight", func(ctx context.Context, generation uint64) (Manifest, error) {
		if captures.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return manifest(generation), nil
		case <-ctx.Done():
			return Manifest{}, ctx.Err()
		}
	})

	const callers = 32
	responses := make([]chan acquireResult, callers)
	for i := range callers {
		responses[i] = make(chan acquireResult, 1)
		actor.commands <- acquireCommand{response: responses[i]}
	}
	<-started
	close(release)

	sequences := make([]uint64, 0, callers)
	for _, response := range responses {
		result := <-response
		if result.err != nil {
			t.Fatalf("Acquire: %v", result.err)
		}
		lease := result.lease
		if lease.WorkspaceGeneration != 1 || lease.Manifest.Digest != "digest-1" {
			t.Fatalf("unexpected lease: %+v", lease)
		}
		if lease.SnapshotState != SnapshotValidatedAt {
			t.Fatalf("snapshot state = %v", lease.SnapshotState)
		}
		sequences = append(sequences, lease.ServerSequence)
	}
	sort.Slice(sequences, func(i, j int) bool { return sequences[i] < sequences[j] })
	for i := 1; i < len(sequences); i++ {
		if sequences[i] <= sequences[i-1] {
			t.Fatalf("non-monotonic lease sequences: %v", sequences)
		}
	}
	if got := captures.Load(); got != 1 {
		t.Fatalf("captures = %d, want 1", got)
	}
}

func TestAcquireAdmittedAfterCommitRecaptures(t *testing.T) {
	var captures atomic.Int32
	actor := newTestActor(t, "post-commit", func(_ context.Context, generation uint64) (Manifest, error) {
		capture := captures.Add(1)
		result := manifest(generation)
		result.Digest = fmt.Sprintf("digest-%d", capture)
		result.ObservationWindow = ObservationWindow{
			StartSequence: uint64(capture*2 - 1),
			EndSequence:   uint64(capture * 2),
		}
		return result, nil
	})

	first, err := actor.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := actor.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.WorkspaceGeneration != second.WorkspaceGeneration ||
		first.Token.snapshotID == second.Token.snapshotID ||
		first.Manifest.Digest == second.Manifest.Digest ||
		first.Manifest.ObservationWindow == second.Manifest.ObservationWindow {
		t.Fatalf("post-commit request reused validation: first=%+v second=%+v", first, second)
	}
	if got := captures.Load(); got != 2 {
		t.Fatalf("captures = %d, want 2", got)
	}
}

func TestInvalidationDuringCaptureCutsOffOlderPublicationAndRecaptures(t *testing.T) {
	var captures atomic.Int32
	firstStarted := make(chan struct{})
	firstExited := make(chan struct{})
	actor := newTestActor(t, "invalidate", func(ctx context.Context, generation uint64) (Manifest, error) {
		switch captures.Add(1) {
		case 1:
			close(firstStarted)
			<-ctx.Done()
			close(firstExited)
			return manifest(generation), nil // hostile late success after cancellation
		default:
			return manifest(generation), nil
		}
	})

	firstResult := make(chan error, 1)
	go func() {
		_, err := actor.Acquire(context.Background())
		firstResult <- err
	}()
	<-firstStarted

	cut, err := actor.Invalidate(context.Background(), 1)
	if err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	if cut.State != StateInvalidated || cut.WorkspaceGeneration != 2 {
		t.Fatalf("unexpected cut: %+v", cut)
	}
	if err := <-firstResult; !errors.Is(err, ErrInvalidated) {
		t.Fatalf("pre-cut Acquire error = %v, want ErrInvalidated", err)
	}
	<-firstExited

	lease, err := actor.Acquire(context.Background())
	if err != nil {
		t.Fatalf("post-cut Acquire: %v", err)
	}
	if lease.WorkspaceGeneration != 2 || lease.Manifest.WSI != "wsi-2" {
		t.Fatalf("older capture published after cut: %+v", lease)
	}
	if got := captures.Load(); got != 2 {
		t.Fatalf("captures = %d, want 2", got)
	}
	status, err := actor.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != StateCaptured || status.WorkspaceGeneration != 2 || status.RetainedSnapshots != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestCaptureFailureTransitionsUnknown(t *testing.T) {
	cause := errors.New("observer disagreement")
	actor := newTestActor(t, "failure", func(context.Context, uint64) (Manifest, error) {
		return Manifest{}, cause
	})

	_, err := actor.Acquire(context.Background())
	if !errors.Is(err, ErrCaptureFailed) || !errors.Is(err, cause) {
		t.Fatalf("Acquire error = %v", err)
	}
	var actorErr *ActorError
	if !errors.As(err, &actorErr) || actorErr.WorkspaceGeneration != 1 || actorErr.ServerSequence == 0 {
		t.Fatalf("unsequenced capture error: %#v", err)
	}
	status, err := actor.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != StateUnknown || status.WorkspaceGeneration != 1 || status.RetainedSnapshots != 0 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.ServerSequence <= actorErr.ServerSequence {
		t.Fatalf("status sequence %d <= error sequence %d", status.ServerSequence, actorErr.ServerSequence)
	}
}

func TestCaptureFailureGivesEveryWaiterUniqueSequence(t *testing.T) {
	cause := errors.New("capture failed")
	started := make(chan struct{})
	release := make(chan struct{})
	actor := newTestActor(t, "error-sequences", func(context.Context, uint64) (Manifest, error) {
		close(started)
		<-release
		return Manifest{}, cause
	})

	const callers = 32
	responses := make([]chan acquireResult, callers)
	for i := range callers {
		responses[i] = make(chan acquireResult, 1)
		actor.commands <- acquireCommand{response: responses[i]}
	}
	<-started
	close(release)

	seen := make(map[uint64]struct{}, callers)
	for _, response := range responses {
		result := <-response
		var actorErr *ActorError
		if !errors.As(result.err, &actorErr) || actorErr.WorkspaceGeneration != 1 || !errors.Is(result.err, cause) {
			t.Fatalf("unsequenced waiter error: %#v", result.err)
		}
		if _, duplicate := seen[actorErr.ServerSequence]; duplicate {
			t.Fatalf("duplicate error sequence %d", actorErr.ServerSequence)
		}
		seen[actorErr.ServerSequence] = struct{}{}
	}
}

func TestServerSequenceAndGenerationAreMonotonic(t *testing.T) {
	actor := newTestActor(t, "sequence", func(_ context.Context, generation uint64) (Manifest, error) {
		return manifest(generation), nil
	})

	status1, err := actor.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	lease1, err := actor.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := actor.Resolve(context.Background(), lease1.Token)
	if err != nil {
		t.Fatal(err)
	}
	cut, err := actor.Invalidate(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	lease2, err := actor.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	status2, err := actor.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	sequences := []uint64{
		status1.ServerSequence,
		lease1.ServerSequence,
		resolved.ServerSequence,
		cut.ServerSequence,
		lease2.ServerSequence,
		status2.ServerSequence,
	}
	for i := 1; i < len(sequences); i++ {
		if sequences[i] <= sequences[i-1] {
			t.Fatalf("sequences not strictly monotonic: %v", sequences)
		}
	}
	if lease1.WorkspaceGeneration != 1 || cut.WorkspaceGeneration != 2 ||
		lease2.WorkspaceGeneration != 2 || status2.WorkspaceGeneration != 2 {
		t.Fatalf("generation regression: lease1=%d cut=%d lease2=%d status=%d",
			lease1.WorkspaceGeneration, cut.WorkspaceGeneration,
			lease2.WorkspaceGeneration, status2.WorkspaceGeneration)
	}

	conflict, err := actor.Invalidate(context.Background(), 1)
	if !errors.Is(err, ErrGenerationConflict) || conflict.WorkspaceGeneration != 2 {
		t.Fatalf("stale CAS = (%+v, %v)", conflict, err)
	}
	var actorErr *ActorError
	if !errors.As(err, &actorErr) || actorErr.ServerSequence != conflict.ServerSequence ||
		actorErr.WorkspaceGeneration != conflict.WorkspaceGeneration {
		t.Fatalf("unsequenced CAS error: update=%+v error=%#v", conflict, err)
	}
}

func TestNewActorRejectsOldTokenEvenWithReusedSessionID(t *testing.T) {
	capture := func(_ context.Context, generation uint64) (Manifest, error) {
		return manifest(generation), nil
	}
	first := newTestActor(t, "reused-session", capture)
	lease, err := first.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	second := newTestActor(t, "reused-session", capture)
	if _, err := second.Resolve(context.Background(), lease.Token); !errors.Is(err, ErrForeignToken) {
		t.Fatalf("Resolve old token error = %v, want ErrForeignToken", err)
	} else {
		var actorErr *ActorError
		if !errors.As(err, &actorErr) || actorErr.WorkspaceGeneration != 1 || actorErr.ServerSequence == 0 {
			t.Fatalf("unsequenced foreign-token error: %#v", err)
		}
	}
}

func TestShutdownCancelsCaptureAndResolvesWaiter(t *testing.T) {
	started := make(chan struct{})
	exited := make(chan struct{})
	actor := newTestActor(t, "shutdown", func(ctx context.Context, generation uint64) (Manifest, error) {
		close(started)
		<-ctx.Done()
		close(exited)
		return Manifest{}, ctx.Err()
	})

	result := make(chan error, 1)
	go func() {
		_, err := actor.Acquire(context.Background())
		result <- err
	}()
	<-started
	update, err := actor.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if update.State != StateStopping || update.WorkspaceGeneration != 1 || update.ServerSequence == 0 {
		t.Fatalf("unexpected shutdown update: %+v", update)
	}
	select {
	case <-exited:
	default:
		t.Fatal("Shutdown returned before capture cancellation completed")
	}
	if err := <-result; !errors.Is(err, ErrStopped) {
		t.Fatalf("Acquire error = %v, want ErrStopped", err)
	} else {
		var actorErr *ActorError
		if !errors.As(err, &actorErr) || actorErr.ServerSequence >= update.ServerSequence {
			t.Fatalf("unsequenced shutdown waiter: %#v; update=%+v", err, update)
		}
	}
	if _, err := actor.Acquire(context.Background()); !errors.Is(err, ErrStopped) {
		t.Fatalf("Acquire after shutdown = %v", err)
	}
	repeated, err := actor.Shutdown(context.Background())
	if err != nil || repeated != update {
		t.Fatalf("repeated Shutdown = (%+v, %v), want (%+v, nil)", repeated, err, update)
	}
}

func TestShutdownReapsCaptureInvalidatedBeforeShutdown(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	actor := newTestActor(t, "reap-invalidated", func(context.Context, uint64) (Manifest, error) {
		close(started)
		<-release // deliberately ignores cancellation until the test permits exit
		return manifest(1), nil
	})

	waiter := make(chan error, 1)
	go func() {
		_, err := actor.Acquire(context.Background())
		waiter <- err
	}()
	<-started
	if _, err := actor.Invalidate(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := <-waiter; !errors.Is(err, ErrInvalidated) {
		t.Fatalf("Acquire = %v, want ErrInvalidated", err)
	}

	shutdown := make(chan updateResult, 1)
	go func() {
		update, err := actor.Shutdown(context.Background())
		shutdown <- updateResult{update: update, err: err}
	}()
	select {
	case result := <-shutdown:
		t.Fatalf("Shutdown returned before invalidated flight exited: %+v", result)
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	result := <-shutdown
	if result.err != nil || result.update.State != StateStopping {
		t.Fatalf("Shutdown = (%+v, %v)", result.update, result.err)
	}
}

func TestCancelledAcquireDoesNotPoisonSharedCapture(t *testing.T) {
	var captures atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	actor := newTestActor(t, "cancel", func(ctx context.Context, generation uint64) (Manifest, error) {
		captures.Add(1)
		close(started)
		select {
		case <-release:
			return manifest(generation), nil
		case <-ctx.Done():
			return Manifest{}, ctx.Err()
		}
	})

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancelledResult := make(chan error, 1)
	go func() {
		_, err := actor.Acquire(cancelledCtx)
		cancelledResult <- err
	}()
	<-started
	otherResult := make(chan acquireResult, 1)
	actor.commands <- acquireCommand{response: otherResult}
	cancel()
	if err := <-cancelledResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Acquire = %v", err)
	}
	close(release)
	result := <-otherResult
	if result.err != nil || result.lease.Manifest.WSI != "wsi-1" {
		t.Fatalf("shared Acquire = (%+v, %v)", result.lease, result.err)
	}
	if got := captures.Load(); got != 1 {
		t.Fatalf("captures = %d, want 1", got)
	}
}

func TestSoleCancelledAcquireCancelsUnneededCapture(t *testing.T) {
	started := make(chan struct{})
	exited := make(chan struct{})
	actor := newTestActor(t, "sole-cancel", func(ctx context.Context, generation uint64) (Manifest, error) {
		close(started)
		<-ctx.Done()
		close(exited)
		return Manifest{}, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := actor.Acquire(ctx)
		result <- err
	}()
	<-started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire = %v, want context.Canceled", err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("unneeded capture did not stop")
	}
	status, err := actor.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateUnknown || status.RetainedSnapshots != 0 {
		t.Fatalf("cancelled capture published: %+v", status)
	}
}

func TestRetentionBoundsExpireOldestLease(t *testing.T) {
	actor, err := NewActor(Config{
		SessionID: "retention",
		Capture: func(_ context.Context, generation uint64) (Manifest, error) {
			return manifest(generation), nil
		},
		MaxRetainedSnapshots: 2,
		MaxRetainedBytes:     4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = actor.Shutdown(context.Background()) })

	leases := make([]Lease, 0, 3)
	for generation := uint64(1); generation <= 3; generation++ {
		lease, err := actor.Acquire(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		leases = append(leases, lease)
		if generation < 3 {
			if _, err := actor.Invalidate(context.Background(), generation); err != nil {
				t.Fatal(err)
			}
		}
	}

	if _, err := actor.Resolve(context.Background(), leases[0].Token); !errors.Is(err, ErrLeaseExpired) {
		t.Fatalf("oldest Resolve = %v, want ErrLeaseExpired", err)
	}
	for _, lease := range leases[1:] {
		if _, err := actor.Resolve(context.Background(), lease.Token); err != nil {
			t.Fatalf("retained Resolve: %v", err)
		}
	}
	status, err := actor.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.RetainedSnapshots != 2 || status.RetainedBytes > 4096 {
		t.Fatalf("retention bounds violated: %+v", status)
	}
}

func TestOversizedSnapshotIsUnknownAndNeverRetained(t *testing.T) {
	actor, err := NewActor(Config{
		SessionID: "oversized",
		Capture: func(_ context.Context, generation uint64) (Manifest, error) {
			result := manifest(generation)
			result.RetainedBytes = 100
			return result, nil
		},
		MaxRetainedSnapshots: 1,
		MaxRetainedBytes:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = actor.Shutdown(context.Background()) })

	_, err = actor.Acquire(context.Background())
	if !errors.Is(err, ErrCaptureFailed) || !errors.Is(err, ErrSnapshotTooLarge) {
		t.Fatalf("Acquire error = %v", err)
	}
	status, err := actor.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.State != StateUnknown || status.RetainedSnapshots != 0 || status.RetainedBytes != 0 {
		t.Fatalf("oversized snapshot retained: %+v", status)
	}
}

func TestCountersFailClosedBeforeWrap(t *testing.T) {
	s := actorState{sequence: math.MaxUint64 - 1}
	sequence, ok := reserveSequences(&s, 1)
	if !ok || sequence != math.MaxUint64 || s.sequence != math.MaxUint64 {
		t.Fatalf("last sequence = (%d, %t), state=%d", sequence, ok, s.sequence)
	}
	if _, ok := reserveSequences(&s, 1); ok || s.sequence != math.MaxUint64 {
		t.Fatalf("sequence wrapped: %d", s.sequence)
	}

	ctx, cancel := context.WithCancel(context.Background())
	actor := &Actor{rootCtx: ctx, cancel: cancel}
	response := make(chan acquireResult, 1)
	s = actorState{
		state:         StateEmpty,
		generation:    1,
		nextCaptureID: math.MaxUint64,
		waiters:       make(map[chan acquireResult]struct{}),
		flights:       make(map[uint64]*captureFlight),
	}
	if stopped := actor.handleAcquire(&s, acquireCommand{response: response}); !stopped {
		t.Fatal("capture identity exhaustion did not stop actor")
	}
	if result := <-response; !errors.Is(result.err, ErrIdentityExhausted) {
		t.Fatalf("capture identity exhaustion = %v", result.err)
	} else {
		var actorErr *ActorError
		if !errors.As(result.err, &actorErr) || actorErr.ServerSequence == 0 {
			t.Fatalf("unsequenced capture identity exhaustion: %#v", result.err)
		}
	}
	if s.nextCaptureID != math.MaxUint64 || actor.terminal.State != StateStopping {
		t.Fatalf("capture identity wrapped or actor remained live: id=%d terminal=%+v", s.nextCaptureID, actor.terminal)
	}

	ctx, cancel = context.WithCancel(context.Background())
	actor = &Actor{rootCtx: ctx, cancel: cancel, maxCount: 1, maxBytes: 4096}
	response = make(chan acquireResult, 1)
	finished := make(chan struct{})
	close(finished)
	flight := &captureFlight{id: 1, generation: 1, cancel: func() {}, finished: finished}
	s = actorState{
		state:          StateCapturing,
		generation:     1,
		activeCapture:  flight,
		flights:        map[uint64]*captureFlight{flight.id: flight},
		nextSnapshotID: math.MaxUint64,
		waiters:        map[chan acquireResult]struct{}{response: {}},
	}
	if stopped := actor.handleCaptureComplete(&s, captureCompleteCommand{
		id: 1, generation: 1, manifest: manifest(1),
	}); !stopped {
		t.Fatal("snapshot identity exhaustion did not stop actor")
	}
	if result := <-response; !errors.Is(result.err, ErrIdentityExhausted) {
		t.Fatalf("snapshot identity exhaustion = %v", result.err)
	} else {
		var actorErr *ActorError
		if !errors.As(result.err, &actorErr) || actorErr.ServerSequence == 0 {
			t.Fatalf("unsequenced snapshot identity exhaustion: %#v", result.err)
		}
	}
	if s.nextSnapshotID != math.MaxUint64 || actor.terminal.State != StateStopping {
		t.Fatalf("snapshot identity wrapped or actor remained live: id=%d terminal=%+v", s.nextSnapshotID, actor.terminal)
	}

	ctx, cancel = context.WithCancel(context.Background())
	actor = &Actor{rootCtx: ctx, cancel: cancel}
	response = make(chan acquireResult, 1)
	s = actorState{
		state:      StateCaptured,
		generation: 7,
		sequence:   math.MaxUint64,
		flights:    make(map[uint64]*captureFlight),
		waiters:    make(map[chan acquireResult]struct{}),
	}
	if stopped := actor.handleResolve(&s, resolveCommand{response: response}); !stopped {
		t.Fatal("sequence exhaustion did not stop actor")
	}
	if result := <-response; !errors.Is(result.err, ErrSequenceExhausted) {
		t.Fatalf("sequence exhaustion = %v", result.err)
	}
	if s.sequence != math.MaxUint64 || actor.terminal.ServerSequence != math.MaxUint64 ||
		actor.terminal.WorkspaceGeneration != 7 {
		t.Fatalf("sequence wrapped or terminal metadata regressed: state=%d terminal=%+v", s.sequence, actor.terminal)
	}
}
