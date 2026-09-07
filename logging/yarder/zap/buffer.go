package zap

type circBuffer[T any] struct {
	items []T
	head  int
	count int
}

func newCircBuffer[T any](size int) *circBuffer[T] {
	if size <= 0 {
		panic("size must be greater than 0")
	}

	return &circBuffer[T]{
		items: make([]T, size),
	}
}

func (b *circBuffer[T]) Len() int {
	return b.count
}

func (b *circBuffer[T]) Push(item T) {
	if b.count == len(b.items) {
		b.items[b.head] = item
		b.head = (b.head + 1) % len(b.items)
	} else {
		index := (b.head + b.count) % len(b.items)
		b.items[index] = item
		b.count++
	}
}

func (b *circBuffer[T]) Pop() (T, bool) {
	var zero T
	if b.count == 0 {
		return zero, false
	}

	item := b.items[b.head]
	b.items[b.head] = zero
	b.head = (b.head + 1) % len(b.items)
	b.count--
	return item, true
}
