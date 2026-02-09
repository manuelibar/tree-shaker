package queue

import "testing"

func TestEnqueueDequeue(t *testing.T) {
	q := New[int]()
	for i := range 5 {
		q.Enqueue(i)
	}
	for i := range 5 {
		v, ok := q.Dequeue()
		if !ok {
			t.Fatalf("dequeue %d: got ok=false", i)
		}
		if v != i {
			t.Fatalf("dequeue %d: got %d", i, v)
		}
	}
	if !q.IsEmpty() {
		t.Fatal("queue should be empty")
	}
}

func TestDequeueEmpty(t *testing.T) {
	q := New[string]()
	v, ok := q.Dequeue()
	if ok {
		t.Fatalf("dequeue on empty: got ok=true, v=%q", v)
	}
}

func TestIsEmpty(t *testing.T) {
	q := New[int]()
	if !q.IsEmpty() {
		t.Fatal("new queue should be empty")
	}
	q.Enqueue(1)
	if q.IsEmpty() {
		t.Fatal("queue with item should not be empty")
	}
	q.Dequeue()
	if !q.IsEmpty() {
		t.Fatal("queue should be empty after dequeue")
	}
}

func TestGrowth(t *testing.T) {
	q := New[int]()
	// Initial cap is 8; push beyond to force resize.
	n := 100
	for i := range n {
		q.Enqueue(i)
	}
	for i := range n {
		v, ok := q.Dequeue()
		if !ok || v != i {
			t.Fatalf("dequeue %d: got (%d, %v)", i, v, ok)
		}
	}
}

func TestWrapAround(t *testing.T) {
	q := New[int]()
	// Fill and drain partially to move head forward, then refill.
	for i := range 6 {
		q.Enqueue(i)
	}
	for range 4 {
		q.Dequeue()
	}
	// head is now at index 4, tail at 6; add more to wrap around.
	for i := 6; i < 12; i++ {
		q.Enqueue(i)
	}
	// Should dequeue in order 4..11.
	for want := 4; want < 12; want++ {
		v, ok := q.Dequeue()
		if !ok || v != want {
			t.Fatalf("got (%d, %v), want (%d, true)", v, ok, want)
		}
	}
}

func TestShrink(t *testing.T) {
	q := New[int]()
	// Push enough to grow beyond min cap of 16, then drain most.
	for i := range 64 {
		q.Enqueue(i)
	}
	// Drain to 4 elements — should trigger shrink.
	for range 60 {
		q.Dequeue()
	}
	// Remaining elements still correct.
	for want := 60; want < 64; want++ {
		v, ok := q.Dequeue()
		if !ok || v != want {
			t.Fatalf("got (%d, %v), want (%d, true)", v, ok, want)
		}
	}
}
