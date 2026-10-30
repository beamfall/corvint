package main

import "os/exec"

func findGo() (string, error) { return exec.LookPath("go") }
