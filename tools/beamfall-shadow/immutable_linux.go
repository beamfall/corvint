//go:build linux

package main

func sealImmutable(string, string) error   { return nil }
func unsealImmutable(string, string) error { return nil }
func descriptorExecutorAvailable() bool    { return true }
