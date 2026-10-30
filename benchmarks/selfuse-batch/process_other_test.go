//go:build !darwin && !linux

package main

func fakeProcessFault(root, mode string) { fail("process fixture unavailable") }
