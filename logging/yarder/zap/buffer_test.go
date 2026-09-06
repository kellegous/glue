package zap

import (
	"testing"
	"time"
)

func assertPop[T comparable](t *testing.T, buffer *circBuffer[T], want T) {
	t.Helper()
	if got := buffer.Pop(); got != want {
		t.Fatalf("Pop() = %v, expected %v", got, want)
	}
}

func TestCircBuffer(t *testing.T) {
	t.Run("waits for an entry", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		popped := make(chan int, 1)
		go func() {
			popped <- buffer.Pop()
		}()

		select {
		case got := <-popped:
			t.Fatalf("Pop() returned before an entry was pushed: %d", got)
		case <-time.After(10 * time.Millisecond):
		}

		buffer.Push(10)
		select {
		case got := <-popped:
			if got != 10 {
				t.Fatalf("Pop() = %d, expected 10", got)
			}
		case <-time.After(time.Second):
			t.Fatal("Pop() did not return after an entry was pushed")
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
