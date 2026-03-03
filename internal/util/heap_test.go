package util

import (
	"testing"
)

// testItem implements HeapItem for testing.
type testItem struct {
	value    string
	priority float64
}

func (t testItem) Priority() float64 { return t.priority }

func TestPriorityQueue_OrdersCorrectly(t *testing.T) {
	pq := NewPriorityQueue[testItem]()

	pq.Push(testItem{"c", 3.0})
	pq.Push(testItem{"a", 1.0})
	pq.Push(testItem{"b", 2.0})
	pq.Push(testItem{"d", 0.5})

	expected := []string{"d", "a", "b", "c"}
	for i, exp := range expected {
		if pq.IsEmpty() {
			t.Fatalf("queue empty at step %d, expected %s", i, exp)
		}
		got := pq.Pop()
		if got.value != exp {
			t.Errorf("step %d: got %s, want %s", i, got.value, exp)
		}
	}

	if !pq.IsEmpty() {
		t.Error("queue should be empty")
	}
}

func TestPriorityQueue_Peek(t *testing.T) {
	pq := NewPriorityQueue[testItem]()
	pq.Push(testItem{"x", 5.0})
	pq.Push(testItem{"y", 2.0})

	got := pq.Peek()
	if got.value != "y" {
		t.Errorf("Peek: got %s, want y", got.value)
	}
	if pq.Len() != 2 {
		t.Errorf("Peek should not remove item, len=%d", pq.Len())
	}
}

func TestPriorityQueue_SingleElement(t *testing.T) {
	pq := NewPriorityQueue[testItem]()
	pq.Push(testItem{"only", 42.0})

	if pq.Len() != 1 {
		t.Fatalf("len: got %d, want 1", pq.Len())
	}
	got := pq.Pop()
	if got.value != "only" {
		t.Errorf("got %s, want only", got.value)
	}
	if !pq.IsEmpty() {
		t.Error("should be empty")
	}
}

func TestPriorityQueue_DuplicatePriorities(t *testing.T) {
	pq := NewPriorityQueue[testItem]()
	pq.Push(testItem{"a", 1.0})
	pq.Push(testItem{"b", 1.0})
	pq.Push(testItem{"c", 1.0})

	// All items should come out; order among ties is implementation-defined.
	seen := make(map[string]bool)
	for i := 0; i < 3; i++ {
		item := pq.Pop()
		seen[item.value] = true
	}
	for _, name := range []string{"a", "b", "c"} {
		if !seen[name] {
			t.Errorf("missing item %s", name)
		}
	}
}
