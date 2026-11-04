// Package lib is the changed dependency of the typed-reach fixture.
package lib

import "os/exec"

// Registry holds HandlerA without calling it at initialization.
var Registry = map[string]func() string{"a": HandlerA}

// Eager calls Init at initialization, in every test binary that links lib.
var Eager = Init()

// Counter is state one test writes and another reads.
var Counter int

func A() int { return 1 }

func B() int { return 2 }

func HandlerA() string { return "a" }

func Init() int { return 3 }

func Bump() { Counter++ }

func Read() int { return Counter }

// Run starts a program the model cannot prove runs no module code.
func Run(name string) error { return exec.Command(name).Run() }

// T reaches String only through an interface conversion.
type T struct{}

func (T) String() string { return "t" }
