//go:build aix || darwin || dragonfly || freebsd || netbsd || openbsd || solaris

package main

func establishWrapperAuthority() bool { return true }
