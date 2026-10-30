//go:build aix || darwin || dragonfly || freebsd || netbsd || openbsd || solaris

package main

func reapExecutionChildren() error { return nil }
