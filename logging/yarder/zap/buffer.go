package zap

import (
	"context"
	"sync"
)

type circBuffer[T any] struct {
	mu      sync.Mutex
	changed chan struct{}
	items   []T
	head    int
	count   int
}

func newCircBuffer[T any](size int) *circBuffer[T] {
	if size <= 0 {
		panic("size must be greater than 0")
	}

	return &circBuffer[T]{
		items:   make([]T, size),
		changed: make(chan struct{}),
	}
}

func (b *circBuffer[T]) Push(item T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == len(b.items) {
		b.items[b.head] = item
		b.head = (b.head + 1) % len(b.items)
	} else {
		index := (b.head + b.count) % len(b.items)
		b.items[index] = item
		b.count++
	}

	close(b.changed)
	b.changed = make(chan struct{})
}

func (b *circBuffer[T]) Pop(ctx context.Context) (T, error) {
	var zero T
	for {
		b.mu.Lock()
		if b.count > 0 {
			item := b.items[b.head]
			b.items[b.head] = zero
			b.head = (b.head + 1) % len(b.items)
			b.count--
			b.mu.Unlock()
			return item, nil
		}
		changed := b.changed
		b.mu.Unlock()

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-changed:
		}
	}
}
