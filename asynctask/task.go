package asynctask

//reference https://github.com/Azure/go-asynctask
import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

var bCheckWait bool

func OpenCheckWait() {
	bCheckWait = true
}

type Action[T any] func(context.Context) (*T, error)

// Task is a handle to the running function.
// which you can use to wait, cancel, get the result.
type Task[T any] struct {
	state      State
	result     *T
	err        error
	cancelFunc context.CancelFunc
	mtx        *sync.Mutex
	action     Action[T]
	done       chan struct{}
	waitState  int32
}

// State return state of the task.
func (t *Task[T]) State() State {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.state
}

// Cancel the task by cancel the context.
// !! this rely on the task function to check context cancellation and proper context handling.
func (t *Task[T]) Cancel() bool {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	if t.state.IsTerminalState() {
		return false
	}

	t.finish(StateCanceled, nil, ErrCanceled)
	return true
}

// Wait block current thread/routine until task isFinished or failed.
// context passed in can terminate the wait, through context cancellation
// but won't terminate the task (unless it's same context)
func (t *Task[T]) Wait(ctx context.Context) error {
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
func (t *Task[T]) WaitWithTimeout(ctx context.Context, timeout time.Duration) (*T, error) {
	// return immediately if task already in terminal state.
	if t.isFinished() {
		return t.result, t.err
	}

	ctx, cancelFunc := context.WithTimeout(ctx, timeout)
	defer cancelFunc()

	return t.Result(ctx)
}

func (t *Task[T]) Result(ctx context.Context) (*T, error) {
	err := t.Wait(ctx)
	if err != nil {
		var result T
		return &result, err
	}

	return t.result, t.err
}

// Start run a async function and returns you a handle which you can Wait or Cancel.
// context passed in may impact task lifetime (from context cancellation)
func Start[T any](ctx context.Context, action Action[T]) *Task[T] {
	ctx, cancel := context.WithCancel(ctx)
	task := &Task[T]{
		state:      StateRunning,
		result:     nil,
		cancelFunc: cancel,
		mtx:        &sync.Mutex{},
		action:     action,
		done:       make(chan struct{}, 1),
	}
	go task.run(ctx)
	return task
}

// NewCompletedTask returns a Completed task, with result=nil, error=nil
func NewCompletedTask[T any](value *T) *Task[T] {
	return &Task[T]{
		state:  StateCompleted,
		result: value,
		err:    nil,
		// nil cancelFunc and wg should be protected with IsTerminalState()
		cancelFunc: nil,
		mtx:        &sync.Mutex{},
	}
}

func (t *Task[T]) run(ctx context.Context) {
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

	result, err := t.action(ctx)
	if err != nil {
		t.finish(StateFailed, result, err)
	} else {
		t.finish(StateCompleted, result, nil)
	}
}

func (t *Task[T]) finish(state State, result *T, err error) {
	// only update state and result if not yet canceled
	if !t.state.IsTerminalState() {
		t.cancelFunc() // cancel the context
		t.state = state
		t.result = result
		t.err = err
		t.done <- struct{}{}
	}
}

func (t *Task[T]) isFinished() bool {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.state.IsTerminalState()
}
