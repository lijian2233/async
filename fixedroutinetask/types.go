package fixedroutinetask

import "errors"

type State int

const (
	StatePending   = 0
	StateRunning   = 1
	StateCompleted = 2
	StateFailed    = 3
	StateCanceled  = 4
)

// IsTerminalState tells whether the task isFinished
func (s State) IsTerminalState() bool {
	return s != StateRunning && s != StatePending
}

func (s State) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateRunning:
		return "running"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

// ErrPanic is returned if panic cought in the task
var ErrPanic = errors.New("panic")

// ErrCanceled is returned if a cancel is triggered
var ErrCanceled = errors.New("canceled")
