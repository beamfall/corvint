//go:build aix || darwin || dragonfly || freebsd || netbsd || openbsd || solaris

package main

func establishExecutionAuthority() error { return nil }
func processContainmentLabel() string    { return "PROCESS_GROUP_ONLY" }
