package source

import (
	"errors"
	"time"
)

// Clock is a closed acquisition-time source. Its zero value is invalid; use
// ProcessClock or FixedClock so production and conformance cannot select an
// ambient or caller-defined clock implementation.
type Clock struct {
	mode  uint8
	fixed time.Time
}

const (
	processClockMode uint8 = 1
	fixedClockMode   uint8 = 2
)

func ProcessClock() Clock {
	return Clock{mode: processClockMode}
}

func FixedClock(value time.Time) (Clock, error) {
	if value.Location() != time.UTC || value != value.Round(0) || value.Year() < 0 || value.Year() > 9999 {
		return Clock{}, errors.New("invalid fixed dashboard clock")
	}
	formatted := value.Format("2006-01-02T15:04:05.000000000Z")
	parsed, err := time.Parse("2006-01-02T15:04:05.000000000Z", formatted)
	if err != nil || parsed != value {
		return Clock{}, errors.New("invalid fixed dashboard clock")
	}
	return Clock{mode: fixedClockMode, fixed: value}, nil
}

func (clock Clock) valid() bool {
	return clock.mode == processClockMode || (clock.mode == fixedClockMode && clock.fixed.Location() == time.UTC && clock.fixed == clock.fixed.Round(0))
}

func (clock Clock) sample() time.Time {
	if clock.mode == fixedClockMode {
		return clock.fixed
	}
	return time.Now().UTC().Round(0)
}
