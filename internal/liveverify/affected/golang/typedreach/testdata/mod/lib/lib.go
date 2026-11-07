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

// Values is shared state a test can write through a copy of the slice.
var Values = make([]int, 1)

func pair() (T, error) { return T{}, nil }

// Box converts a T to an interface through a multi-value call.
func Box() (any, error) { return pair() }

// Shared holds a reference inside a struct.
var Shared = struct{ P *int }{new(int)}

// State is written by Guard, which a TestMain condition calls.
var State int

func Guard() bool { State = 1; return true }

// CA and CB are separate constant declarations; CB is built from CA.
const CA = 1

const CB = CA + 1

// Arr's length is CA.
type Arr [CA]int

// Child is what the test binary runs when started as a child.
func Child() int { return 7 }
