// Package dogfoodoperation serializes local-completion and public dogfood
// writers through the same private administrative lock.
package dogfoodoperation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type capabilityKey struct{}
type capability struct {
	mu       sync.Mutex
	gitDir   string
	active   bool
	held     bool
	retained any
}

var ErrCleanupHold = errors.New("aggregate-cleanup-hold")

// heldCapabilities deliberately retain uncertain ownership until this process
// exits. HOLD has no automatic recovery, expiry, or numeric-PID retry path.
var heldCapabilities sync.Map

// Check refuses all further work through a capability once cleanup is uncertain.
// A read-only caller has no capability and acquires no lock by calling Check.
func Check(ctx context.Context) error {
	if held, ok := ctx.Value(capabilityKey{}).(*capability); ok {
		held.mu.Lock()
		defer held.mu.Unlock()
		if held.held {
			return ErrCleanupHold
		}
	}
	return nil
}

type holdError struct {
	retained    any
	persistence error
}

func (e *holdError) Error() string { return ErrCleanupHold.Error() }
func (e *holdError) Unwrap() error { return ErrCleanupHold }

// Hold makes writer ownership sticky before attempting bounded diagnostics.
// Failure to persist a diagnostic never releases the lock. Read-only callers
// retain the handle in the returned error without writing administrative state.
func Hold(ctx context.Context, observation []byte, retained any) error {
	result := &holdError{retained: retained}
	held, ok := ctx.Value(capabilityKey{}).(*capability)
	if !ok {
		return result
	}
	held.mu.Lock()
	defer held.mu.Unlock()
	if held.held {
		return result
	}
	held.held, held.retained = true, retained
	heldCapabilities.Store(held.gitDir, held)
	if len(observation) == 0 || len(observation) > 64<<10 || !json.Valid(observation) {
		result.persistence = errors.New("invalid HOLD observation")
		return result
	}
	name := filepath.Join(held.gitDir, "corvint", "local-completion", "operation.lock", "hold.json")
	if err := CheckParents(name); err != nil {
		result.persistence = err
		return result
	}
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		result.persistence = err
		return result
	}
	_, writeErr := file.Write(observation)
	closeErr := file.Close()
	result.persistence = errors.Join(writeErr, closeErr)
	return result
}

// Acquire lends only an in-process, live context capability for this canonical
// GitDir. Environment strings and another worktree's context confer no access.
func Acquire(ctx context.Context, gitDir string) (context.Context, func(), error) {
	if err := CheckParents(gitDir); err != nil {
		return ctx, nil, err
	}
	canonical, err := filepath.EvalSymlinks(gitDir)
	if err != nil {
		return ctx, nil, err
	}
	canonical, err = filepath.Abs(canonical)
	if err != nil {
		return ctx, nil, err
	}
	if held, ok := ctx.Value(capabilityKey{}).(*capability); ok {
		held.mu.Lock()
		if held.held {
			held.mu.Unlock()
			return ctx, nil, ErrCleanupHold
		}
		valid := held.active && held.gitDir == canonical
		held.mu.Unlock()
		if valid {
			return ctx, func() {}, nil
		}
	}
	release, err := LockDirectory(filepath.Join(canonical, "corvint", "local-completion"))
	if err != nil {
		return ctx, nil, err
	}
	held := &capability{gitDir: canonical, active: true}
	var once sync.Once
	return context.WithValue(ctx, capabilityKey{}, held), func() {
		once.Do(func() {
			held.mu.Lock()
			held.active = false
			if !held.held {
				release()
			}
			held.mu.Unlock()
		})
	}, nil
}

// LockDirectory preserves the established local-completion lock and errors.
// It does not create a capability, so legacy callers cannot lend a forged one.
func LockDirectory(directory string) (func(), error) {
	if err := CheckParents(directory); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, Failure(err)
	}
	name := filepath.Join(directory, "operation.lock")
	if err := os.Mkdir(name, 0700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, errors.New("operation-in-progress")
		}
		return nil, Failure(err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = os.Remove(name) }) }, nil
}

// CheckParents rejects static symlinks. It does not claim confinement against
// hostile same-UID replacement, and it never steals or removes a stale lock.
func CheckParents(name string) error {
	for current := filepath.Clean(name); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("local-state-symlink")
		}
		if current == filepath.Dir(current) {
			return nil
		}
	}
}

type lockError struct {
	cause error
	code  string
}

func (e *lockError) Error() string { return e.code }
func (e *lockError) Unwrap() error { return e.cause }
func Failure(err error) error {
	code := "operation-lock-create-failed"
	if errors.Is(err, os.ErrPermission) {
		code = "operation-lock-permission-denied"
	}
	return &lockError{err, code}
}
