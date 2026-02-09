package jsonpath

import "testing"

func TestNameSelector(t *testing.T) {
	s := NameSelector{Name: "foo"}
	if !s.Match("foo", 0) {
		t.Error("should match exact name")
	}
	if s.Match("bar", 0) {
		t.Error("should not match different name")
	}
	if s.Match(0, 5) {
		t.Error("should not match integer key")
	}
	if s.String() != "foo" {
		t.Errorf("String() = %q, want %q", s.String(), "foo")
	}
}

func TestIndexSelector(t *testing.T) {
	s := IndexSelector{Index: 2}
	if !s.Match(2, 5) {
		t.Error("should match exact index")
	}
	if s.Match(3, 5) {
		t.Error("should not match different index")
	}
	if s.Match("2", 5) {
		t.Error("should not match string key")
	}
}

func TestIndexSelectorNegative(t *testing.T) {
	s := IndexSelector{Index: -1}
	if !s.Match(4, 5) {
		t.Error("-1 should match last element (index 4 of len 5)")
	}
	if s.Match(3, 5) {
		t.Error("-1 should not match index 3 of len 5")
	}

	s2 := IndexSelector{Index: -2}
	if !s2.Match(3, 5) {
		t.Error("-2 should match index 3 of len 5")
	}
}

func TestWildcardSelector(t *testing.T) {
	s := WildcardSelector{}
	if !s.Match("anything", 0) {
		t.Error("wildcard should match string key")
	}
	if !s.Match(42, 100) {
		t.Error("wildcard should match integer key")
	}
	if s.String() != "*" {
		t.Errorf("String() = %q, want %q", s.String(), "*")
	}
}

func TestSliceSelectorBasic(t *testing.T) {
	// [0:3] — matches indices 0, 1, 2
	start, end := 0, 3
	s := SliceSelector{Start: &start, End: &end}

	for i := 0; i < 3; i++ {
		if !s.Match(i, 5) {
			t.Errorf("[0:3] should match index %d", i)
		}
	}
	if s.Match(3, 5) {
		t.Error("[0:3] should not match index 3")
	}
	if s.Match("0", 5) {
		t.Error("slice should not match string key")
	}
}

func TestSliceSelectorStep(t *testing.T) {
	// [0:6:2] — matches 0, 2, 4
	start, end, step := 0, 6, 2
	s := SliceSelector{Start: &start, End: &end, Step: &step}

	if !s.Match(0, 6) {
		t.Error("[0:6:2] should match 0")
	}
	if !s.Match(2, 6) {
		t.Error("[0:6:2] should match 2")
	}
	if !s.Match(4, 6) {
		t.Error("[0:6:2] should match 4")
	}
	if s.Match(1, 6) {
		t.Error("[0:6:2] should not match 1")
	}
	if s.Match(3, 6) {
		t.Error("[0:6:2] should not match 3")
	}
}

func TestSliceSelectorDefaults(t *testing.T) {
	// [:] — matches all (start=0, end=len, step=1)
	s := SliceSelector{}
	for i := 0; i < 5; i++ {
		if !s.Match(i, 5) {
			t.Errorf("[:] should match index %d", i)
		}
	}
}

func TestSliceSelectorNegativeStep(t *testing.T) {
	// [4:0:-1] — matches 4, 3, 2, 1
	start, end, step := 4, 0, -1
	s := SliceSelector{Start: &start, End: &end, Step: &step}

	if !s.Match(4, 5) {
		t.Error("[4:0:-1] should match 4")
	}
	if !s.Match(1, 5) {
		t.Error("[4:0:-1] should match 1")
	}
	if s.Match(0, 5) {
		t.Error("[4:0:-1] should not match 0")
	}
}

func TestSliceSelectorZeroStep(t *testing.T) {
	step := 0
	s := SliceSelector{Step: &step}
	if s.Match(0, 5) {
		t.Error("step=0 should match nothing")
	}
}

func TestSliceSelectorNegativeIndices(t *testing.T) {
	// [-2:] — matches last 2 elements
	start := -2
	s := SliceSelector{Start: &start}
	if !s.Match(3, 5) {
		t.Error("[-2:] should match index 3 of len 5")
	}
	if !s.Match(4, 5) {
		t.Error("[-2:] should match index 4 of len 5")
	}
	if s.Match(2, 5) {
		t.Error("[-2:] should not match index 2 of len 5")
	}
}
