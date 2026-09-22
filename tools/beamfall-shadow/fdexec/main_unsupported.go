//go:build !darwin && !linux

package main

import "os"

func main() { os.Exit(127) }
