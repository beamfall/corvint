//go:build windows

package releasegate

func killProcess(int)      {}
func processGone(int) bool { return true }
