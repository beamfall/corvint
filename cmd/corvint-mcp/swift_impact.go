package main

// Path impact reads SwiftPM target membership through the affected Swift
// model, which internal/contextindex cannot import (NGI-V0-988).
import _ "github.com/Beamfall/corvint/internal/swiftimpact"
