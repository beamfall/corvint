// Package pulse contains the in-memory coordination primitives for Corvint Pulse.
package pulse

import (
	"context"
	"errors"
	"fmt"
	"math"
)

// WorkspaceState is the actor's serialized workspace lifecycle state.
type WorkspaceState uint8

const (
	StateEmpty WorkspaceState = iota
	StateCapturing
	StateCaptured
	StateInvalidated
	StateUnknown
	StateStopping
)

func (s WorkspaceState) String() string {
	switch s {
	case StateEmpty:
		return "EMPTY"
	case StateCapturing:
		return "CAPTURING"
	case StateCaptured:
		return "CAPTURED"
	case StateInvalidated:
		return "INVALIDATED"
	case StateUnknown:
		return "UNKNOWN"
	case StateStopping:
		return "STOPPING"
	default:
		return "UNKNOWN_STATE"
	}
}

// SnapshotState deliberately has no CURRENT or STALE value. A lease proves only
// that its immutable manifest was validated at its workspace generation.
type SnapshotState uint8

const SnapshotValidatedAt SnapshotState = 1

func (s SnapshotState) String() string {
	if s == SnapshotValidatedAt {
		return "VALIDATED_AT"
	}
	return "UNKNOWN_SNAPSHOT_STATE"
}

// ObservationWindow identifies the ordered observations which bounded a
// capture under its declared observer model.
type ObservationWindow struct {
	StartSequence uint64
	EndSequence   uint64
}

// Manifest is an immutable-by-construction capture result: all fields are value
// types. RetainedBytes accounts for immutable data retained behind its digest.
type Manifest struct {
	WSI               string
	Digest            string
	ObservationWindow ObservationWindow
	ObservationModel  string
	RetainedBytes     uint64
}

// CaptureFunc obtains a manifest for exactly the supplied workspace generation.
// It must stop promptly when ctx is cancelled.
type CaptureFunc func(ctx context.Context, generation uint64) (Manifest, error)

// Config defines one memory-only actor instance.
type Config struct {
	SessionID            string
	Capture              CaptureFunc
	MaxRetainedSnapshots int
	MaxRetainedBytes     uint64
}

// Token is an opaque, actor-instance-bound snapshot lease token.
type Token struct {
	owner                    *Actor
	sessionID                string
	snapshotID               uint64
	workspaceGeneration      uint64
	wsi                      string
	manifestDigest           string
	observerModel            string
	observationStartSequence uint64
	observationEndSequence   uint64
}

// Lease is proof that Manifest was validated at WorkspaceGeneration. It makes
// no assertion that the generation is still current when the caller reads it.
type Lease struct {
	Token               Token
	Manifest            Manifest
	SnapshotState       SnapshotState
	WorkspaceGeneration uint64
	ServerSequence      uint64
}

// Update reports a serialized state transition.
type Update struct {
	State               WorkspaceState
	WorkspaceGeneration uint64
	ServerSequence      uint64
}

// Status is a point-in-sequence view of actor state.
type Status struct {
	Update
	SessionID         string
	RetainedSnapshots int
	RetainedBytes     uint64
}

var (
	ErrInvalidConfig       = errors.New("pulse: invalid actor configuration")
	ErrStopped             = errors.New("pulse: actor stopped")
	ErrInvalidated         = errors.New("pulse: capture invalidated")
	ErrGenerationConflict  = errors.New("pulse: workspace generation conflict")
	ErrGenerationExhausted = errors.New("pulse: workspace generation exhausted")
	ErrSequenceExhausted   = errors.New("pulse: server sequence exhausted")
	ErrIdentityExhausted   = errors.New("pulse: internal snapshot identity exhausted")
	ErrCaptureFailed       = errors.New("pulse: capture failed")
	ErrInvalidManifest     = errors.New("pulse: invalid capture manifest")
	ErrSnapshotTooLarge    = errors.New("pulse: snapshot exceeds retention bound")
	ErrForeignToken        = errors.New("pulse: lease token belongs to another actor session")
	ErrLeaseExpired        = errors.New("pulse: snapshot lease expired")
)

// CaptureError preserves both the stable failure class and the injected cause.
type CaptureError struct {
	Generation uint64
	Cause      error
}

func (e *CaptureError) Error() string {
	return fmt.Sprintf("%v at generation %d: %v", ErrCaptureFailed, e.Generation, e.Cause)
}

func (e *CaptureError) Unwrap() []error { return []error{ErrCaptureFailed, e.Cause} }

// ActorError binds an actor-produced failure to the generation and unique
// server sequence at which it was committed.
type ActorError struct {
	Cause               error
	WorkspaceGeneration uint64
	ServerSequence      uint64
}

func (e *ActorError) Error() string { return e.Cause.Error() }
func (e *ActorError) Unwrap() error { return e.Cause }

// Actor serializes all workspace state through one private event loop.
type Actor struct {
	sessionID string
	capture   CaptureFunc
	maxCount  int
	maxBytes  uint64

	commands chan any
	done     chan struct{}
	rootCtx  context.Context
	cancel   context.CancelFunc

	terminal    Update
	terminalErr error
}

type actorState struct {
	state      WorkspaceState
	generation uint64
	sequence   uint64

	nextCaptureID  uint64
	activeCapture  *captureFlight
	flights        map[uint64]*captureFlight
	nextSnapshotID uint64
	retained       []*retainedSnapshot
	retainedBytes  uint64
	waiters        map[chan acquireResult]struct{}
}

type captureFlight struct {
	id         uint64
	generation uint64
	cancel     context.CancelFunc
	finished   chan struct{}
}

type retainedSnapshot struct {
	id         uint64
	generation uint64
	manifest   Manifest
	bytes      uint64
}

type acquireCommand struct{ response chan acquireResult }
type acquireResult struct {
	lease Lease
	err   error
}
type cancelAcquireCommand struct {
	response chan acquireResult
	ack      chan struct{}
}
type captureCompleteCommand struct {
	id         uint64
	generation uint64
	manifest   Manifest
	err        error
}
type invalidateCommand struct {
	expected uint64
	response chan updateResult
}
type resolveCommand struct {
	token    Token
	response chan acquireResult
}
type statusCommand struct{ response chan statusResult }
type statusResult struct {
	status Status
	err    error
}
type shutdownCommand struct{ response chan updateResult }
type updateResult struct {
	update Update
	err    error
}

// NewActor starts one memory-only workspace actor. SessionID is injected so
// tests and callers can bind events deterministically; tokens are additionally
// bound to the concrete actor instance.
func NewActor(cfg Config) (*Actor, error) {
	if cfg.SessionID == "" || cfg.Capture == nil || cfg.MaxRetainedSnapshots < 1 || cfg.MaxRetainedBytes < 1 {
		return nil, ErrInvalidConfig
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &Actor{
		sessionID: cfg.SessionID,
		capture:   cfg.Capture,
		maxCount:  cfg.MaxRetainedSnapshots,
		maxBytes:  cfg.MaxRetainedBytes,
		commands:  make(chan any),
		done:      make(chan struct{}),
		rootCtx:   ctx,
		cancel:    cancel,
	}
	go a.run()
	return a, nil
}

// Acquire starts or joins the active capture for the current generation. A
// capture committed before admission is historical and never satisfies it.
func (a *Actor) Acquire(ctx context.Context) (Lease, error) {
	response := make(chan acquireResult, 1)
	if err := a.send(ctx, acquireCommand{response: response}); err != nil {
		return Lease{}, err
	}

	select {
	case result := <-response:
		return result.lease, result.err
	case <-a.done:
		select {
		case result := <-response:
			return result.lease, result.err
		default:
			return Lease{}, a.stoppedError()
		}
	case <-ctx.Done():
		select {
		case result := <-response:
			return result.lease, result.err
		default:
		}
		ack := make(chan struct{}, 1)
		select {
		case a.commands <- cancelAcquireCommand{response: response, ack: ack}:
			select {
			case <-ack:
			case <-a.done:
			}
		case <-a.done:
		}
		return Lease{}, ctx.Err()
	}
}

// Invalidate advances the workspace generation iff expectedGeneration is
// current. The accepted command is the cut after which the older capture can
// no longer publish.
func (a *Actor) Invalidate(ctx context.Context, expectedGeneration uint64) (Update, error) {
	response := make(chan updateResult, 1)
	if err := a.send(ctx, invalidateCommand{expected: expectedGeneration, response: response}); err != nil {
		return Update{}, err
	}
	select {
	case result := <-response:
		return result.update, result.err
	case <-a.done:
		select {
		case result := <-response:
			return result.update, result.err
		default:
			return Update{}, a.stoppedError()
		}
	case <-ctx.Done():
		return Update{}, ctx.Err()
	}
}

// Resolve returns a retained validated-at snapshot. It does not assert that
// the snapshot is current relative to the workspace.
func (a *Actor) Resolve(ctx context.Context, token Token) (Lease, error) {
	response := make(chan acquireResult, 1)
	if err := a.send(ctx, resolveCommand{token: token, response: response}); err != nil {
		return Lease{}, err
	}
	select {
	case result := <-response:
		return result.lease, result.err
	case <-a.done:
		select {
		case result := <-response:
			return result.lease, result.err
		default:
			return Lease{}, a.stoppedError()
		}
	case <-ctx.Done():
		return Lease{}, ctx.Err()
	}
}

// Status returns actor state at a unique server sequence.
func (a *Actor) Status(ctx context.Context) (Status, error) {
	response := make(chan statusResult, 1)
	if err := a.send(ctx, statusCommand{response: response}); err != nil {
		return Status{}, err
	}
	select {
	case result := <-response:
		return result.status, result.err
	case <-a.done:
		select {
		case result := <-response:
			return result.status, result.err
		default:
			return Status{}, a.stoppedError()
		}
	case <-ctx.Done():
		return Status{}, ctx.Err()
	}
}

// Shutdown transitions to STOPPING, rejects waiters, cancels the active
// capture, and terminates the actor. It is idempotent after the first stop.
func (a *Actor) Shutdown(ctx context.Context) (Update, error) {
	response := make(chan updateResult, 1)
	select {
	case a.commands <- shutdownCommand{response: response}:
	case <-a.done:
		return a.terminal, a.terminalErr
	case <-ctx.Done():
		return Update{}, ctx.Err()
	}
	select {
	case result := <-response:
		return result.update, result.err
	case <-a.done:
		select {
		case result := <-response:
			return result.update, result.err
		default:
			return a.terminal, a.terminalErr
		}
	case <-ctx.Done():
		return Update{}, ctx.Err()
	}
}

func (a *Actor) send(ctx context.Context, command any) error {
	select {
	case a.commands <- command:
		return nil
	case <-a.done:
		return a.stoppedError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Actor) stoppedError() error {
	if a.terminalErr != nil {
		return a.terminalErr
	}
	return ErrStopped
}

func (a *Actor) run() {
	s := actorState{
		state:      StateEmpty,
		generation: 1,
		waiters:    make(map[chan acquireResult]struct{}),
		flights:    make(map[uint64]*captureFlight),
	}
	defer close(a.done)

	for {
		command := <-a.commands
		stop := false
		switch command := command.(type) {
		case acquireCommand:
			stop = a.handleAcquire(&s, command)
		case cancelAcquireCommand:
			stop = a.handleCancelAcquire(&s, command)
		case captureCompleteCommand:
			stop = a.handleCaptureComplete(&s, command)
		case invalidateCommand:
			stop = a.handleInvalidate(&s, command)
		case resolveCommand:
			stop = a.handleResolve(&s, command)
		case statusCommand:
			sequence, ok := reserveSequences(&s, 1)
			if !ok {
				command.response <- statusResult{err: ErrSequenceExhausted}
				a.stopForExhaustion(&s, ErrSequenceExhausted)
				stop = true
			} else {
				command.response <- statusResult{status: a.status(&s, sequence)}
			}
		case shutdownCommand:
			stop = a.handleShutdown(&s, command)
		}
		if stop {
			return
		}
	}
}

func (a *Actor) handleAcquire(s *actorState, command acquireCommand) bool {
	s.waiters[command.response] = struct{}{}
	if s.activeCapture != nil && s.activeCapture.generation == s.generation {
		return false
	}
	if s.nextCaptureID == math.MaxUint64 {
		a.stopForIdentityExhaustion(s)
		return true
	}
	if _, ok := reserveSequences(s, 1); !ok {
		a.stopForExhaustion(s, ErrSequenceExhausted)
		return true
	}

	s.state = StateCapturing
	s.nextCaptureID++
	captureCtx, cancel := context.WithCancel(a.rootCtx)
	flight := &captureFlight{
		id:         s.nextCaptureID,
		generation: s.generation,
		cancel:     cancel,
		finished:   make(chan struct{}),
	}
	s.activeCapture = flight
	s.flights[flight.id] = flight
	go a.captureGeneration(captureCtx, flight)
	return false
}

func (a *Actor) handleCancelAcquire(s *actorState, command cancelAcquireCommand) bool {
	delete(s.waiters, command.response)
	if len(s.waiters) == 0 && s.activeCapture != nil {
		if _, ok := reserveSequences(s, 1); !ok {
			command.ack <- struct{}{}
			a.stopForExhaustion(s, ErrSequenceExhausted)
			return true
		}
		s.activeCapture.cancel()
		s.activeCapture = nil
		s.state = StateUnknown
	}
	command.ack <- struct{}{}
	return false
}

func (a *Actor) captureGeneration(ctx context.Context, flight *captureFlight) {
	manifest, err := invokeCapture(a.capture, ctx, flight.generation)
	close(flight.finished)
	command := captureCompleteCommand{
		id:         flight.id,
		generation: flight.generation,
		manifest:   manifest,
		err:        err,
	}
	select {
	case a.commands <- command:
	case <-a.done:
	}
}

func invokeCapture(capture CaptureFunc, ctx context.Context, generation uint64) (manifest Manifest, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("capture panic: %v", recovered)
		}
	}()
	return capture(ctx, generation)
}

func (a *Actor) handleCaptureComplete(s *actorState, command captureCompleteCommand) bool {
	delete(s.flights, command.id)
	if s.activeCapture == nil || s.activeCapture.id != command.id ||
		s.activeCapture.generation != command.generation || s.generation != command.generation {
		return false
	}
	s.activeCapture.cancel()
	s.activeCapture = nil

	if command.err != nil {
		sequence, ok := reserveSequences(s, uint64(len(s.waiters)))
		if !ok {
			a.stopForExhaustion(s, ErrSequenceExhausted)
			return true
		}
		s.state = StateUnknown
		err := &CaptureError{Generation: command.generation, Cause: command.err}
		a.resolveWaiters(s, nil, err, sequence)
		return false
	}
	bytes, err := validateManifest(command.manifest, a.maxBytes)
	if err != nil {
		sequence, ok := reserveSequences(s, uint64(len(s.waiters)))
		if !ok {
			a.stopForExhaustion(s, ErrSequenceExhausted)
			return true
		}
		s.state = StateUnknown
		a.resolveWaiters(s, nil, &CaptureError{Generation: command.generation, Cause: err}, sequence)
		return false
	}
	if s.nextSnapshotID == math.MaxUint64 {
		a.stopForIdentityExhaustion(s)
		return true
	}
	sequence, ok := reserveSequences(s, uint64(len(s.waiters)))
	if !ok {
		a.stopForExhaustion(s, ErrSequenceExhausted)
		return true
	}

	s.nextSnapshotID++
	snapshot := &retainedSnapshot{
		id:         s.nextSnapshotID,
		generation: command.generation,
		manifest:   command.manifest,
		bytes:      bytes,
	}
	for len(s.retained) >= a.maxCount || bytes > a.maxBytes-s.retainedBytes {
		evicted := s.retained[0]
		s.retained[0] = nil
		s.retained = s.retained[1:]
		s.retainedBytes -= evicted.bytes
	}
	s.retained = append(s.retained, snapshot)
	s.retainedBytes += bytes
	s.state = StateCaptured
	a.resolveWaiters(s, snapshot, nil, sequence)
	return false
}

func validateManifest(manifest Manifest, maxBytes uint64) (uint64, error) {
	if manifest.WSI == "" || manifest.Digest == "" || manifest.ObservationWindow.StartSequence == 0 ||
		manifest.ObservationWindow.EndSequence < manifest.ObservationWindow.StartSequence ||
		manifest.ObservationModel == "" {
		return 0, ErrInvalidManifest
	}
	bytes := manifest.RetainedBytes
	for _, value := range []string{
		manifest.WSI,
		manifest.Digest,
		manifest.ObservationModel,
	} {
		if uint64(len(value)) > math.MaxUint64-bytes {
			return 0, ErrSnapshotTooLarge
		}
		bytes += uint64(len(value))
	}
	if bytes > maxBytes {
		return 0, ErrSnapshotTooLarge
	}
	return bytes, nil
}

func (a *Actor) resolveWaiters(
	s *actorState,
	snapshot *retainedSnapshot,
	err error,
	sequence uint64,
) {
	remaining := len(s.waiters)
	for waiter := range s.waiters {
		if err != nil {
			waiter <- acquireResult{err: actorError(err, s.generation, sequence)}
		} else {
			waiter <- acquireResult{lease: a.lease(snapshot, sequence)}
		}
		delete(s.waiters, waiter)
		remaining--
		if remaining > 0 {
			sequence++
		}
	}
}

func (a *Actor) handleInvalidate(s *actorState, command invalidateCommand) bool {
	if command.expected != s.generation {
		sequence, ok := reserveSequences(s, 1)
		if !ok {
			command.response <- updateResult{err: ErrSequenceExhausted}
			a.stopForExhaustion(s, ErrSequenceExhausted)
			return true
		}
		command.response <- updateResult{
			update: a.update(s, sequence),
			err:    actorError(ErrGenerationConflict, s.generation, sequence),
		}
		return false
	}
	if s.generation == math.MaxUint64 {
		sequence, ok := reserveSequences(s, 1)
		if !ok {
			command.response <- updateResult{err: ErrSequenceExhausted}
			a.stopForExhaustion(s, ErrSequenceExhausted)
			return true
		}
		command.response <- updateResult{
			update: a.update(s, sequence),
			err:    actorError(ErrGenerationExhausted, s.generation, sequence),
		}
		return false
	}
	firstSequence, ok := reserveSequences(s, uint64(len(s.waiters))+1)
	if !ok {
		command.response <- updateResult{err: ErrSequenceExhausted}
		a.stopForExhaustion(s, ErrSequenceExhausted)
		return true
	}

	s.generation++
	s.state = StateInvalidated
	if s.activeCapture != nil {
		s.activeCapture.cancel()
		s.activeCapture = nil
	}
	a.resolveWaiters(s, nil, ErrInvalidated, firstSequence)
	command.response <- updateResult{update: a.update(s, s.sequence)}
	return false
}

func (a *Actor) handleResolve(s *actorState, command resolveCommand) bool {
	sequence, ok := reserveSequences(s, 1)
	if !ok {
		command.response <- acquireResult{err: ErrSequenceExhausted}
		a.stopForExhaustion(s, ErrSequenceExhausted)
		return true
	}
	if command.token.owner != a || command.token.sessionID != a.sessionID {
		command.response <- acquireResult{err: actorError(ErrForeignToken, s.generation, sequence)}
		return false
	}
	for _, snapshot := range s.retained {
		if snapshot.id == command.token.snapshotID &&
			snapshot.generation == command.token.workspaceGeneration &&
			snapshot.manifest.WSI == command.token.wsi &&
			snapshot.manifest.Digest == command.token.manifestDigest &&
			snapshot.manifest.ObservationModel == command.token.observerModel &&
			snapshot.manifest.ObservationWindow.StartSequence == command.token.observationStartSequence &&
			snapshot.manifest.ObservationWindow.EndSequence == command.token.observationEndSequence {
			command.response <- acquireResult{lease: a.lease(snapshot, sequence)}
			return false
		}
	}
	command.response <- acquireResult{err: actorError(ErrLeaseExpired, s.generation, sequence)}
	return false
}

func (a *Actor) handleShutdown(s *actorState, command shutdownCommand) bool {
	firstSequence, ok := reserveSequences(s, uint64(len(s.waiters))+1)
	if !ok {
		command.response <- updateResult{err: ErrSequenceExhausted}
		a.stopForExhaustion(s, ErrSequenceExhausted)
		return true
	}
	s.state = StateStopping
	a.cancel()
	for _, flight := range s.flights {
		flight.cancel()
	}
	a.resolveWaiters(s, nil, ErrStopped, firstSequence)
	a.reapFlights(s)
	a.clearRetained(s)
	a.terminal = a.update(s, s.sequence)
	command.response <- updateResult{update: a.terminal}
	return true
}

func (a *Actor) stopForExhaustion(s *actorState, cause error) {
	s.state = StateStopping
	a.cancel()
	for _, flight := range s.flights {
		flight.cancel()
	}
	for waiter := range s.waiters {
		waiter <- acquireResult{err: cause}
		delete(s.waiters, waiter)
	}
	a.reapFlights(s)
	a.clearRetained(s)
	a.terminal = Update{
		State:               StateStopping,
		WorkspaceGeneration: s.generation,
		ServerSequence:      s.sequence,
	}
	a.terminalErr = cause
}

func (a *Actor) stopForIdentityExhaustion(s *actorState) {
	sequence, ok := reserveSequences(s, uint64(len(s.waiters)))
	if !ok {
		a.stopForExhaustion(s, ErrSequenceExhausted)
		return
	}
	a.resolveWaiters(s, nil, ErrIdentityExhausted, sequence)
	a.stopForExhaustion(s, ErrIdentityExhausted)
}

func (a *Actor) reapFlights(s *actorState) {
	for id, flight := range s.flights {
		<-flight.finished
		delete(s.flights, id)
	}
	s.activeCapture = nil
}

func (a *Actor) clearRetained(s *actorState) {
	for index := range s.retained {
		s.retained[index] = nil
	}
	s.retained = nil
	s.retainedBytes = 0
}

func reserveSequences(s *actorState, count uint64) (uint64, bool) {
	if count == 0 {
		return s.sequence, true
	}
	if count > math.MaxUint64-s.sequence {
		return 0, false
	}
	first := s.sequence + 1
	s.sequence += count
	return first, true
}

func actorError(cause error, generation, sequence uint64) error {
	return &ActorError{
		Cause:               cause,
		WorkspaceGeneration: generation,
		ServerSequence:      sequence,
	}
}

func (a *Actor) lease(snapshot *retainedSnapshot, sequence uint64) Lease {
	return Lease{
		Token: Token{
			owner:                    a,
			sessionID:                a.sessionID,
			snapshotID:               snapshot.id,
			workspaceGeneration:      snapshot.generation,
			wsi:                      snapshot.manifest.WSI,
			manifestDigest:           snapshot.manifest.Digest,
			observerModel:            snapshot.manifest.ObservationModel,
			observationStartSequence: snapshot.manifest.ObservationWindow.StartSequence,
			observationEndSequence:   snapshot.manifest.ObservationWindow.EndSequence,
		},
		Manifest:            snapshot.manifest,
		SnapshotState:       SnapshotValidatedAt,
		WorkspaceGeneration: snapshot.generation,
		ServerSequence:      sequence,
	}
}

func (a *Actor) update(s *actorState, sequence uint64) Update {
	return Update{
		State:               s.state,
		WorkspaceGeneration: s.generation,
		ServerSequence:      sequence,
	}
}

func (a *Actor) status(s *actorState, sequence uint64) Status {
	return Status{
		Update:            a.update(s, sequence),
		SessionID:         a.sessionID,
		RetainedSnapshots: len(s.retained),
		RetainedBytes:     s.retainedBytes,
	}
}
