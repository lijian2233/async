package asynctask

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestTaskCompleted(t *testing.T) {
	/*task := Start[int](context.Background(), func(ctx context.Context) (*int, error) {
		return nil, nil
	})

	//task.Cancel()

	task.Wait(context.Background())

	fmt.Println(task.State())*/
}

func TestTaskCanceled(t *testing.T) {
	task := Start[int](context.Background(), func(ctx context.Context) (*int, error) {
		return nil, errors.New("fail")
	})

	task.Cancel()
	task.Wait(context.Background())
	fmt.Println(task.State())
}

func TestTaskFailed(t *testing.T) {
	task := Start[int](context.Background(), func(ctx context.Context) (*int, error) {
		return nil, errors.New("fail")
	})

	task.Cancel()

	task.Wait(context.Background())

	fmt.Println(task.State())
}

func TestTaskMultiWait(t *testing.T) {
	OpenCheckWait()
	task := Start[int](context.Background(), func(ctx context.Context) (*int, error) {
		time.Sleep(4 * time.Second)
		return nil, errors.New("fail")
	})

	go func() {
		task.Wait(context.Background())
	}()
	task.Wait(context.Background())

	fmt.Println(task.State())
}
