package queue

// Queue is a generic FIFO queue backed by a ring buffer.
type Queue[T any] interface {
	Enqueue(item T)
	Dequeue() (T, bool)
	IsEmpty() bool
}

// New creates an empty Queue with an initial capacity of 8.
func New[T any]() Queue[T] {
	return &queue[T]{items: make([]T, 8)}
}

type queue[T any] struct {
	items []T
	head  int
	tail  int
	count int
}

func (q *queue[T]) Enqueue(item T) {
	if q.count == len(q.items) {
		q.resize(q.count * 2)
	}
	q.items[q.tail] = item
	q.tail = (q.tail + 1) % len(q.items)
	q.count++
}

func (q *queue[T]) Dequeue() (T, bool) {
	if q.count == 0 {
		var zero T
		return zero, false
	}

	item := q.items[q.head]

	var zero T
	q.items[q.head] = zero // prevent memory leaks

	q.head = (q.head + 1) % len(q.items)
	q.count--

	// Shrink when sparse (< 25% usage), min capacity 16.
	if len(q.items) > 16 && q.count > 0 && q.count <= len(q.items)/4 {
		q.resize(len(q.items) / 2)
	}

	return item, true
}

func (q *queue[T]) IsEmpty() bool { return q.count == 0 }

func (q *queue[T]) resize(newCap int) {
	buf := make([]T, newCap)
	if q.count > 0 {
		if q.head < q.tail {
			copy(buf, q.items[q.head:q.tail])
		} else {
			n := copy(buf, q.items[q.head:])
			copy(buf[n:], q.items[:q.tail])
		}
	}
	q.items = buf
	q.head = 0
	q.tail = q.count % newCap
}
