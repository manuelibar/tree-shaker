package set

import "testing"

func TestNewEmpty(t *testing.T) {
	s := New[int]()
	if s.Has(1) {
		t.Fatal("new set should not contain 1")
	}
}

func TestNewWithValues(t *testing.T) {
	s := New(1, 2, 3, 2, 1)
	for _, v := range []int{1, 2, 3} {
		if !s.Has(v) {
			t.Errorf("expected set to contain %d", v)
		}
	}
	if s.Has(4) {
		t.Error("set should not contain 4")
	}
}

func TestAddRemove(t *testing.T) {
	s := New[string]()
	s.Add("a", "b", "c")
	for _, v := range []string{"a", "b", "c"} {
		if !s.Has(v) {
			t.Errorf("expected set to contain %q", v)
		}
	}

	s.Remove("b")
	if s.Has("b") {
		t.Fatal("expected b removed")
	}
	if !s.Has("a") || !s.Has("c") {
		t.Fatal("expected a and c to remain")
	}

	// Remove non-existent is no-op.
	s.Remove("z")
	if !s.Has("a") || !s.Has("c") {
		t.Fatal("expected a and c to remain after removing z")
	}
}

func TestAddDuplicate(t *testing.T) {
	s := New[int]()
	s.Add(1, 1, 1)
	s.Remove(1)
	if s.Has(1) {
		t.Fatal("expected 1 removed after single Remove")
	}
}
