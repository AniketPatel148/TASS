// Package util provides core data structures and helpers used throughout the simulator.
package util

import "container/heap"

// HeapItem is the interface that items stored in the PriorityQueue must implement.
type HeapItem interface {
	// Priority returns the priority value. Lower values are dequeued first.
	Priority() float64
}

// PriorityQueue is a generic min-heap priority queue.
type PriorityQueue[T HeapItem] struct {
	inner pqInner[T]
}

// NewPriorityQueue creates a new empty PriorityQueue.
func NewPriorityQueue[T HeapItem]() *PriorityQueue[T] {
	pq := &PriorityQueue[T]{}
	heap.Init(&pq.inner)
	return pq
}

// Push adds an item to the queue.
func (pq *PriorityQueue[T]) Push(item T) {
	heap.Push(&pq.inner, item)
}

// Pop removes and returns the item with the lowest priority value.
// Panics if the queue is empty.
func (pq *PriorityQueue[T]) Pop() T {
	return heap.Pop(&pq.inner).(T)
}

// Peek returns the item with the lowest priority value without removing it.
// Panics if the queue is empty.
func (pq *PriorityQueue[T]) Peek() T {
	return pq.inner[0]
}

// Len returns the number of items in the queue.
func (pq *PriorityQueue[T]) Len() int {
	return pq.inner.Len()
}

// IsEmpty returns true if the queue has no items.
func (pq *PriorityQueue[T]) IsEmpty() bool {
	return pq.inner.Len() == 0
}

// --- container/heap interface implementation ---

type pqInner[T HeapItem] []T

func (pq pqInner[T]) Len() int            { return len(pq) }
func (pq pqInner[T]) Less(i, j int) bool   { return pq[i].Priority() < pq[j].Priority() }
func (pq pqInner[T]) Swap(i, j int)        { pq[i], pq[j] = pq[j], pq[i] }

func (pq *pqInner[T]) Push(x interface{}) {
	*pq = append(*pq, x.(T))
}

func (pq *pqInner[T]) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}
