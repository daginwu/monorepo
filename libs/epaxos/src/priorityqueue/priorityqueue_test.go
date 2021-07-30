package priorityqueue

import (
	"testing"
)

type testValue struct {
	val int32
}

func TestPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue()
	pushed1 := &testValue{17}
	priority1 := int64(9)
	pushed2 := &testValue{3}
	priority2 := int64(1)
	pq.Push(pushed1, priority1)
	pq.Push(pushed2, priority2)
	ppeak1 := pq.PeakPriority()
	if ppeak1 != priority2 {
		t.Fatalf("Expected %d, got %d", priority2, ppeak1)
	}
	peak1 := pq.Peak()
	if peak1 != pushed2 {
		t.Fatalf("Expected %v, got %v", pushed2, peak1)
	}
	popped1 := pq.Pop()
	if popped1 != pushed2 {
		t.Fatalf("Expected %v, got %v", pushed2, popped1)
	}
	ppeak2 := pq.PeakPriority()
	if ppeak2 != priority1 {
		t.Fatalf("Expected %d, got %d", priority1, ppeak2)
	}
	peak2 := pq.Peak()
	if peak2 != pushed1 {
		t.Fatalf("Expected %v, got %v", pushed1, peak2)
	}
	popped2 := pq.Pop()
	if popped2 != pushed1 {
		t.Fatalf("Expected %v, got %v", pushed1, popped2)
	}
}
