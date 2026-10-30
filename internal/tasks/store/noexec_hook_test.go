package store

// CALTestNoExecHook installs hook at the named points of a cancelling NO_EXEC
// settlement ("stopped", "cancel" and "release"; see noExecHook)
// so external fixtures can observe or interrupt those windows; the returned
// function restores the product's nil hook.
func CALTestNoExecHook(hook func(point string) error) func() {
	previous := noExecHook
	noExecHook = hook
	return func() { noExecHook = previous }
}
