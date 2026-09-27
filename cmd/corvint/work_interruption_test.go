package main

import (
	"context"
	"sync"
)

// workFinalInterruption returns a parent context and the action that ends it at
// the witnessed point: cancellation, or a deadline that expires only when called.
// No wall clock runs before the behaviour under test (decision 0082).
func workFinalInterruption(cause string) (context.Context, func()) {
	if cause == "cancelled" {
		return context.WithCancel(context.Background())
	}
	deadline := &workWitnessedDeadline{Context: context.Background(), done: make(chan struct{})}
	return deadline, deadline.expire
}

type workWitnessedDeadline struct {
	context.Context
	done chan struct{}
	once sync.Once
}

func (ctx *workWitnessedDeadline) Done() <-chan struct{} { return ctx.done }
func (ctx *workWitnessedDeadline) expire()               { ctx.once.Do(func() { close(ctx.done) }) }
func (ctx *workWitnessedDeadline) Err() error {
	select {
	case <-ctx.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}
