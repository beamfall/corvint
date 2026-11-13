package contextindex

import (
	"time"
)

// taskContextCancelStages is every boundary at which compile observes the
// request context (TCP-V0-064), in compile order. "lexical" is inside
// lexicalRows, where V1-0485 observed a cancelled request returning READY.
var taskContextCancelStages = []string{"pair", "mentioned", "subject-slots", "cochange", "lexical-term", "lexical", "ordered", "packet"}

// taskContextReaderHold is how long assertTaskContextJoinHoldsReader keeps
// the held reader from finishing. A join that does not await it returns inside
// the hold; a join that does blocks until the hold ends.
const taskContextReaderHold = 150 * time.Millisecond
