package fixedroutinetask

import (
	"container/list"
	"sync"
)

type Queue struct {
	C  chan struct{}
	mu sync.Mutex
	q  *list.List
}

func NewQueue() *Queue {
	return &Queue{
		q: list.New(),
		C: make(chan struct{}, 1),
	}
}

func (this *Queue) push(task *Task) {
	this.mu.Lock()
	e := this.q.PushBack(task)
	task.inQ, task.listE = this, e
	this.mu.Unlock()

	select {
	case this.C <- struct{}{}:
	default:

	}
}

func (this *Queue) pop() (t *Task, has bool) {
	this.mu.Lock()
	defer this.mu.Unlock()

	e := this.q.Back()

	if e != nil {
		this.q.Remove(e)
		task, ok := e.Value.(*Task)
		if ok {
			task.listE = nil
			t = task
			has = true
		}
	}

	if this.q.Len() > 0 {
		select {
		case this.C <- struct{}{}:
		default:

		}
	}
	return
}

func (this *Queue) size() int {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.q.Len()
}
