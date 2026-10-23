package store

// CALTestWorktreeRemoveHook installs hook at the named points of a stage
// worktree removal ("recorded" and "removed"; see worktreeRemoveHook) so
// external fixtures can interrupt those windows; the returned function
// restores the product's nil hook.
func CALTestWorktreeRemoveHook(hook func(point string) error) func() {
	previous := worktreeRemoveHook
	worktreeRemoveHook = hook
	return func() { worktreeRemoveHook = previous }
}
