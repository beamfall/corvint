//go:build darwin

package main

// Darwin has no supported arbitrary-file descriptor execution primitive.
// uchg is mutable by the same UID and therefore cannot provide execution
// authority. invoke refuses to start a candidate on this platform.
func sealImmutable(string, string) error   { return nil }
func unsealImmutable(string, string) error { return nil }

func descriptorExecutorAvailable() bool { return false }
