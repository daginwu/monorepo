// Based on https://golang.org/pkg/container/heap/
package priorityqueue

import (
	"container/heap"
)

const PRI_INVALID int64 = -1

// An Item is something we manage in a priority queue.
type Item struct {
	value    interface{} // The value of the item; arbitrary.
	priority int64       // The priority of the item in the queue.
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A PriorityQueue implements heap.Interface and holds Items.
type PriorityQueue struct {
	pq *priorityQueue
}

func NewPriorityQueue() *PriorityQueue {
	pq := make(priorityQueue, 0, 100)
	heap.Init(&pq)
	return &PriorityQueue{&pq}
}

func (pq *PriorityQueue) Push(value interface{}, priority int64) {
	// Insert a new item and then modify its priority.
	item := &Item{
		value:    value,
		priority: priority,
	}
	heap.Push(pq.pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	if pq.pq.Len() == 0 {
		return nil
	}
	return heap.Pop(pq.pq).(*Item).value
}

func (pq *PriorityQueue) Peak() interface{} {
	if pq.pq.Len() == 0 {
		return nil
	}
	item := heap.Pop(pq.pq).(*Item)
	value := item.value
	heap.Push(pq.pq, item)
	return value
}

func (pq *PriorityQueue) PeakPriority() int64 {
	if pq.pq.Len() == 0 {
		return PRI_INVALID
	}
	item := heap.Pop(pq.pq).(*Item)
	priority := item.priority
	heap.Push(pq.pq, item)
	return priority
}

// A PriorityQueue implements heap.Interface and holds Items.
type priorityQueue []*Item

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq priorityQueue) Len() int {
	return len(pq)
}

// The highest priority items have the lowest priority value
func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// update modifies the priority and value of an Item in the queue.
func (pq *priorityQueue) update(item *Item, value string, priority int64) {
	item.value = value
	item.priority = priority
	heap.Fix(pq, item.index)
}
