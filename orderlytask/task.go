package orderlytask

//reference https://github.com/Azure/go-asynctask
import (
	"context"
	"fmt"
	"golang.org/x/exp/slices"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

type SeqMgr struct {
	mux    sync.Mutex
	seqMap map[int64][]*Task
}

var seqMgr = SeqMgr{
	seqMap: map[int64][]*Task{},
}

func (m *SeqMgr) onTaskFinish(t *Task) {
	var next *Task
	m.mux.Lock()

	if s, ok := m.seqMap[t.key]; ok && len(s) > 0 {
		i := 0
		for ; i < len(s); i++ {
			if s[i] == t {
				break
			}
		}

		//not found
		if i == len(s) {
			//should be check reason, why happy
			m.mux.Unlock()
			return
		}

		if i != 0 {
			s = slices.Delete(s, i, i+1)
			m.seqMap[t.key] = s
		} else {
			s = slices.Delete(s, 0, 1)
			if len(s) > 0 {
				next = s[0]
				m.seqMap[t.key] = s
			} else {
				delete(m.seqMap, t.key)
			}
		}
		m.mux.Unlock()

		if next != nil {
			next.run()
		}
	} else {
		m.mux.Unlock()
	}
}

var bCheckWait bool

func OpenCheckWait() {
	bCheckWait = true
}

type Action func(context.Context) (interface{}, error)

// Task is a handle to the running function.
// which you can use to wait, cancel, get the result.
type Task struct {
	ctx        context.Context
	state      State
	result     interface{}
	err        error
	cancelFunc context.CancelFunc
	mtx        *sync.Mutex
	action     Action
	done       chan struct{}
	key        int64
	waitState  int32
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
func Start(ctx context.Context, key int64, action Action) *Task {
	ctx, cancel := context.WithCancel(ctx)
	task := &Task{
		state:      StateRunning,
		result:     nil,
		cancelFunc: cancel,
		mtx:        &sync.Mutex{},
		action:     action,
		done:       make(chan struct{}, 1),
		key:        key,
		ctx:        ctx,
	}
	seqMgr.mux.Lock()
	info, ok := seqMgr.seqMap[key]
	if !ok {
		info = []*Task{task}
		seqMgr.seqMap[key] = info
		seqMgr.mux.Unlock()
		go task.run()
	} else {
		info = append(info, task)
		seqMgr.seqMap[key] = info
		seqMgr.mux.Unlock()
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

func (t *Task) finish(state State, result interface{}, err error) {
	// only update state and result if not yet canceled
	if !t.state.IsTerminalState() {
		t.cancelFunc() // cancel the context
		t.state = state
		t.result = result
		t.err = err
		t.done <- struct{}{}
	}
	seqMgr.onTaskFinish(t)
}

func (t *Task) isFinished() bool {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	return t.state.IsTerminalState()
}
