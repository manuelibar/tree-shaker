package set

// Set is an insertion-ordered collection of unique elements.
// Add preserves first-seen order; Remove maintains relative order.
type Set[T comparable] interface {
	Add(vals ...T)
	Remove(vals ...T)
	Has(val T) bool
	Values() []T
	Len() int
}

func New[T comparable](vals ...T) Set[T] {
	s := &set[T]{
		index: make(map[T]struct{}, len(vals)),
		items: make([]T, 0, len(vals)),
	}
	s.Add(vals...)
	return s
}

type set[T comparable] struct {
	index map[T]struct{}
	items []T
}

func (s *set[T]) Add(vals ...T) {
	for _, v := range vals {
		if _, ok := s.index[v]; ok {
			continue
		}
		s.index[v] = struct{}{}
		s.items = append(s.items, v)
	}
}

func (s *set[T]) Remove(vals ...T) {
	for _, v := range vals {
		if _, ok := s.index[v]; !ok {
			continue
		}
		delete(s.index, v)
		for i, item := range s.items {
			if item == v {
				s.items = append(s.items[:i], s.items[i+1:]...)
				break
			}
		}
	}
}

func (s *set[T]) Has(val T) bool {
	_, ok := s.index[val]
	return ok
}

func (s *set[T]) Values() []T {
	out := make([]T, len(s.items))
	copy(out, s.items)
	return out
}

func (s *set[T]) Len() int {
	return len(s.items)
}
