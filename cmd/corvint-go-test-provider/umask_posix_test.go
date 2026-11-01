//go:build unix

package main

import "syscall"

// init makes the fixtures this package creates, and the helper processes it
// re-executes, independent of the caller's umask (V1-3251): a group-writable
// umask such as 0002 would otherwise leave fixture authority paths
// group-writable, which the provider refuses.
func init() { syscall.Umask(0o022) }
