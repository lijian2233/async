package fixedroutinetask

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRoutineTask(t *testing.T) {
	rt := NewFixedRoutineTask(10)
	t1 := rt.Start(context.Background(), 1, func(ctx context.Context) (interface{}, error) {
		time.Sleep(10 * time.Second)
		fmt.Println("1111")
		return nil, nil
	})

	t2 := rt.Start(context.Background(), 1, func(ctx context.Context) (interface{}, error) {
		time.Sleep(10 * time.Second)
		fmt.Println("2222")
		return nil, nil
	})

	t2.Cancel()
	t1.Wait(context.Background())

	rt.Stop()
}
