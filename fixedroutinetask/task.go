package fixedroutinetask

import (
	"container/list"
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

var bCheckWait bool

func TraceWait() {
	bCheckWait = true
}

type Action func(context.Context) (interface{}, error)

// Task is a handle to the running function.
// which you can use to wait, cancel, get the result.
type Task struct {
	state      State
	result     interface{}
	err        error
	cancelFunc context.CancelFunc
	mtx        *sync.Mutex
	action     Action
	done       chan struct{}
	waitState  int32
	ctx        context.Context
	listE      *list.Element
	inQ        *Queue
}

// State return state of the task.
func (t *Task) State() State {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.state
}

// Cancel the task by cancel the context.
// !! this rely on the task function to check context cancellation and proper context handling.
func (t *Task) Cancel() bool {
	t.inQ.mu.Lock()
	if t.listE != nil {
		t.inQ.q.Remove(t.listE)
		t.listE = nil
	}
	t.inQ.mu.Unlock()
	t.mtx.Lock()
	defer t.mtx.Unlock()

	return t.finish(StateCanceled, nil, ErrCanceled)
}

// Wait block current thread/routine until task isFinished or failed.
// context passed in can terminate the wait, through context cancellation
// but won't terminate the task (unless it's same context)
func (t *Task) Wait(ctx context.Context) error {
	// return immediately if task already in terminal state.
	if t.isFinished() {
		return t.err
	}

	if bCheckWait {
		if !atomic.CompareAndSwapInt32(&t.waitState, 0, 1) {
			panic(fmt.Sprintf("multi wati %+v", t))
		}

		defer func() {
			t.waitState = 0
		}()
	}

	select {
	case <-t.done:
		return t.err

	case <-ctx.Done():
		return ctx.Err()
	}
}

// WaitWithTimeout block current thread/routine until task isFinished or failed, or exceed the duration specified.
// timeout only stop waiting, taks will remain running.
func (t *Task) WaitWithTimeout(ctx context.Context, timeout time.Duration) (interface{}, error) {
	// return immediately if task already in terminal state.
	if t.isFinished() {
		return t.result, t.err
	}

	ctx, cancelFunc := context.WithTimeout(ctx, timeout)
	defer cancelFunc()

	return t.Result(ctx)
}

func (t *Task) Result(ctx context.Context) (interface{}, error) {
	err := t.Wait(ctx)
	if err != nil {
		return nil, err
	}

	return t.result, t.err
}

// Start run a async function and returns you a handle which you can Wait or Cancel.
// context passed in may impact task lifetime (from context cancellation)
func newTask(ctx context.Context, action Action) *Task {
	ctx, cancel := context.WithCancel(ctx)
	task := &Task{
		state:      StateRunning,
		result:     nil,
		cancelFunc: cancel,
		mtx:        &sync.Mutex{},
		action:     action,
		done:       make(chan struct{}, 1),
		ctx:        ctx,
	}
	return task
}

// NewCompletedTask returns a Completed task, with result=nil, error=nil
func NewCompletedTask(value interface{}) *Task {
	return &Task{
		state:  StateCompleted,
		result: value,
		err:    nil,
		// nil cancelFunc and wg should be protected with IsTerminalState()
		cancelFunc: nil,
		mtx:        &sync.Mutex{},
	}
}

func (t *Task) run() {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	if t.state.IsTerminalState() {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("Panic cought: %v, StackTrace: %s, %w", r, debug.Stack(), ErrPanic)
			t.finish(StateFailed, nil, err)
		}
	}()

	result, err := t.action(t.ctx)
	if err != nil {
		t.finish(StateFailed, result, err)
	} else {
		t.finish(StateCompleted, result, nil)
	}
}

func (t *Task) finish(state State, result interface{}, err error) bool {
	// only update state and result if not yet canceled
	if !t.state.IsTerminalState() {
		t.cancelFunc() // cancel the context
		t.state = state
		t.result = result
		t.err = err
		t.done <- struct{}{}
		return true
	}
	return false
}

func (t *Task) isFinished() bool {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.state.IsTerminalState()
}
