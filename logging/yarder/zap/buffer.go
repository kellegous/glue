package zap

import "sync"

type circBuffer[T any] struct {
	mu       sync.Mutex
	notEmpty *sync.Cond
	items    []T
	head     int
	count    int
}

func newCircBuffer[T any](size int) *circBuffer[T] {
	if size <= 0 {
		panic("size must be greater than 0")
	}

	buffer := &circBuffer[T]{
		items: make([]T, size),
	}
	buffer.notEmpty = sync.NewCond(&buffer.mu)
	return buffer
}

func (b *circBuffer[T]) Push(item T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == len(b.items) {
		b.items[b.head] = item
		b.head = (b.head + 1) % len(b.items)
		return
	}

	index := (b.head + b.count) % len(b.items)
	b.items[index] = item
	b.count++
	b.notEmpty.Signal()
}

func (b *circBuffer[T]) Pop() T {
	b.mu.Lock()
	defer b.mu.Unlock()

	for b.count == 0 {
		b.notEmpty.Wait()
	}

	item := b.items[b.head]
	var zero T
	b.items[b.head] = zero
	b.head = (b.head + 1) % len(b.items)
	b.count--
	return item
}
