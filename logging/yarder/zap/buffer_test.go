package zap

import (
	"context"
	"errors"
	"testing"
	"time"
)

func assertPop[T comparable](t *testing.T, buffer *circBuffer[T], want T) {
	t.Helper()
	got, err := buffer.Pop(context.Background())
	if err != nil {
		t.Fatalf("Pop() error = %v", err)
	}
	if got != want {
		t.Fatalf("Pop() = %v, expected %v", got, want)
	}
}

func TestCircBuffer(t *testing.T) {
	t.Run("waits for an entry", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		popped := make(chan int, 1)
		go func() {
			item, err := buffer.Pop(context.Background())
			if err == nil {
				popped <- item
			}
		}()

		select {
		case got := <-popped:
			t.Fatalf("Pop() returned before an entry was pushed: %d", got)
		case <-time.After(10 * time.Millisecond):
		}

		pushed := make(chan struct{})
		go func() {
			buffer.Push(10)
			close(pushed)
		}()
		select {
		case got := <-popped:
			if got != 10 {
				t.Fatalf("Pop() = %d, expected 10", got)
			}
		case <-time.After(time.Second):
			t.Fatal("Pop() did not return after an entry was pushed")
		}

		select {
		case <-pushed:
		case <-time.After(time.Second):
			t.Fatal("Push() did not return")
		}
	})

	t.Run("returns the context error when empty", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		got, err := buffer.Pop(ctx)
		if got != 0 {
			t.Fatalf("Pop() = %d, expected zero value", got)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Pop() error = %v, expected context cancellation", err)
		}
	})

	t.Run("wakes all waiting consumers", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		popped := make(chan int, 2)
		for range 2 {
			go func() {
				item, err := buffer.Pop(context.Background())
				if err == nil {
					popped <- item
				}
			}()
		}

		time.Sleep(10 * time.Millisecond)
		buffer.Push(10)
		buffer.Push(20)

		got := map[int]bool{}
		for range 2 {
			select {
			case item := <-popped:
				got[item] = true
			case <-time.After(time.Second):
				t.Fatal("not all waiting consumers returned")
			}
		}
		if !got[10] || !got[20] {
			t.Fatalf("Pop() results = %v, expected 10 and 20", got)
		}
	})

	t.Run("pops in FIFO order", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		buffer.Push(10)
		buffer.Push(20)
		assertPop(t, buffer, 10)
		assertPop(t, buffer, 20)
	})

	t.Run("drops the oldest item when full", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		buffer.Push(1)
		buffer.Push(2)
		buffer.Push(3)
		assertPop(t, buffer, 2)
		assertPop(t, buffer, 3)
	})

	t.Run("wraparound", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		buffer.Push(1)
		buffer.Push(2)
		assertPop(t, buffer, 1)
		buffer.Push(3)
		assertPop(t, buffer, 2)
		assertPop(t, buffer, 3)
		buffer.Push(4)
		assertPop(t, buffer, 4)
	})
}
