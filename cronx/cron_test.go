package cronx

import (
	"context"
	"testing"
	"time"
)

func TestWrapCronTaskReturnsWhenHandlerIgnoresTimeout(t *testing.T) {
	finished := make(chan struct{})
	callback := make(chan struct{})
	task := WrapCronTask(func(context.Context) {
		time.Sleep(150 * time.Millisecond)
		close(finished)
	}, func() {
		close(callback)
	}, 20*time.Millisecond)

	started := time.Now()
	task()
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("timeout wrapper blocked for %s", elapsed)
	}

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish")
	}
	select {
	case <-callback:
	case <-time.After(time.Second):
		t.Fatal("completion callback did not run")
	}
}
