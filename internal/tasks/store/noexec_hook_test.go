package store

// CALTestNoExecCancelHook installs hook between a NO_EXEC settlement and its
// cancel so external fixtures can observe that window; the returned function
// restores the product's nil hook.
func CALTestNoExecCancelHook(hook func() error) func() {
	previous := noExecCancelHook
	noExecCancelHook = hook
	return func() { noExecCancelHook = previous }
}
