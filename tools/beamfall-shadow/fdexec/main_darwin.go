//go:build darwin

package main

import "os"

// Darwin has no fexecve. The harness uses the platform immutable-file fallback.
func main() { os.Exit(127) }
