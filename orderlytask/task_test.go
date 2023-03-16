package orderlytask

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTaskCompleted(t *testing.T) {
	for i := 0; i < 100; i++ {
		func(j int) {
			Start(context.Background(), 1, func(context.Context) (interface{}, error) {
				fmt.Println(j)
				return j, nil
			},
			)
		}(i)
	}

	time.Sleep(10 * time.Second)
}
