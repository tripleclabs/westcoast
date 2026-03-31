package crdt

import "testing"

func TestVectorTimestamp_Clone(t *testing.T) {
	vt := VectorTimestamp{"a": 1, "b": 2}
	clone := vt.Clone()
	clone["a"] = 99
	if vt["a"] != 1 {
		t.Error("clone should not affect original")
	}
}

func TestVectorTimestamp_Merge(t *testing.T) {
	a := VectorTimestamp{"n1": 3, "n2": 1}
	b := VectorTimestamp{"n1": 1, "n2": 5, "n3": 2}
	m := a.Merge(b)
	if m["n1"] != 3 || m["n2"] != 5 || m["n3"] != 2 {
		t.Errorf("merge: %v", m)
	}
}

func TestVectorTimestamp_HappensBefore(t *testing.T) {
	a := VectorTimestamp{"x": 1, "y": 2}
	b := VectorTimestamp{"x": 1, "y": 3}
	c := VectorTimestamp{"x": 2, "y": 1}

	if !a.HappensBefore(b) {
		t.Error("a should happen before b")
	}
	if b.HappensBefore(a) {
		t.Error("b should not happen before a")
	}
	if a.HappensBefore(c) || c.HappensBefore(a) {
		t.Error("a and c are concurrent")
	}
}

func TestVectorTimestamp_Concurrent(t *testing.T) {
	a := VectorTimestamp{"x": 2, "y": 1}
	b := VectorTimestamp{"x": 1, "y": 2}
	if !a.Concurrent(b) {
		t.Error("should be concurrent")
	}
}

func TestVectorClock_TickAndMerge(t *testing.T) {
	c1 := newVectorClock("n1")
	c2 := newVectorClock("n2")

	ts1 := c1.Tick()
	ts2 := c2.Tick()

	if ts1["n1"] != 1 {
		t.Errorf("expected 1, got %d", ts1["n1"])
	}

	merged := c1.Merge(ts2)
	if merged["n2"] != 1 {
		t.Errorf("expected n2=1, got %d", merged["n2"])
	}

	ts3 := c1.Tick()
	if ts3["n1"] != 2 || ts3["n2"] != 1 {
		t.Errorf("expected n1=2,n2=1, got %v", ts3)
	}
}

func TestVectorClock_CurrentIsCopy(t *testing.T) {
	c := newVectorClock("n1")
	c.Tick()
	c.Tick()
	cur := c.Current()
	cur["n1"] = 99
	if c.Current()["n1"] != 2 {
		t.Error("should return a copy")
	}
}
