package fixedroutinetask

import (
	"context"
	"sync"
	"sync/atomic"
)

type FixedRoutineTask struct {
	maxRoutines int
	queues      []*Queue
	wg          sync.WaitGroup
	die         chan struct{}
	stopState   int32
}

func NewFixedRoutineTask(maxRoutines int) *FixedRoutineTask {
	if maxRoutines < 1 {
		maxRoutines = 1
	}

	ht := &FixedRoutineTask{
		maxRoutines: maxRoutines,
		queues:      make([]*Queue, maxRoutines),
		die:         make(chan struct{}, 1),
	}

	for i := 0; i < maxRoutines; i++ {
		ht.queues[i] = NewQueue()
	}
	ht.run()
	return ht
}

func (ht *FixedRoutineTask) Start(ctx context.Context, key int, action Action) *Task {
	hashId := key % ht.maxRoutines
	task := newTask(ctx, action)
	q := ht.queues[hashId]
	q.push(task)
	return task
}

func (ht *FixedRoutineTask) run() {
	ht.wg.Add(ht.maxRoutines)
	for i := 0; i < ht.maxRoutines; i++ {
		go func(q *Queue) {
			defer ht.wg.Done()

			for {
				select {
				case <-ht.die:
					return
				case _, ok := <-q.C:
					if !ok {
						return
					}

					if task, ok := q.pop(); ok {
						task.run()
					}
				}
			}

		}(ht.queues[i])
	}
}

func (ht *FixedRoutineTask) Stop() {
	if atomic.CompareAndSwapInt32(&ht.stopState, 0, 1) {
		close(ht.die)
	}
}
