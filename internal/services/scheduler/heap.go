package scheduler

import "container/heap"

type StateHeap []*State

func (sh StateHeap) Len() int {
	return len(sh)
}

func (sh StateHeap) Less(i, j int) bool {
	return sh[i].nextDue.Before(sh[j].nextDue)
}

func (sh *StateHeap) Pop() any {
	old := *sh
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*sh = old[0 : n-1]
	return item
}

func (sh *StateHeap) Push(x any) {
	n := len(*sh)
	item := x.(*State)
	item.index = n
	*sh = append(*sh, item)
}

func (sh StateHeap) Swap(i, j int) {
	sh[i], sh[j] = sh[j], sh[i]
	sh[i].index = i
	sh[j].index = j
}

var _ heap.Interface = &StateHeap{}
