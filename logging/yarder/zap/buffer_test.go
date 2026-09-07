package zap

import (
	"testing"
)

func assertPop[T comparable](t *testing.T, buffer *circBuffer[T], want T) {
	t.Helper()
	got, ok := buffer.Pop()
	if !ok {
		t.Fatal("Pop() = no item, expected an item")
	}
	if got != want {
		t.Fatalf("Pop() = %v, expected %v", got, want)
	}
}

func TestCircBuffer(t *testing.T) {
	t.Run("is empty when created", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		got, ok := buffer.Pop()
		if got != 0 {
			t.Fatalf("Pop() = %d, expected zero value", got)
		}
		if ok {
			t.Fatal("Pop() returned an item from an empty buffer")
		}
	})

	t.Run("reports its length", func(t *testing.T) {
		buffer := newCircBuffer[int](2)
		if got := buffer.Len(); got != 0 {
			t.Fatalf("Len() = %d, expected 0", got)
		}
		buffer.Push(10)
		if got := buffer.Len(); got != 1 {
			t.Fatalf("Len() = %d, expected 1", got)
		}
		buffer.Push(20)
		buffer.Push(30)
		if got := buffer.Len(); got != 2 {
			t.Fatalf("Len() = %d, expected capacity 2", got)
		}
		assertPop(t, buffer, 20)
		if got := buffer.Len(); got != 1 {
			t.Fatalf("Len() = %d, expected 1", got)
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
