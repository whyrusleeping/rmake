package manager

import "sync"

type BuilderQueue struct {
	// The backing datastructure
	arr []*BuilderConnection
	// Mutex to lock on push and pop
	mut sync.Mutex
	//Sorting Comparison function
	cmp func(*BuilderConnection, *BuilderConnection) bool
}

// Construct a new builder connection queue
func NewBuilderQueue() *BuilderQueue {
	q := new(BuilderQueue)
	q.arr = make([]*BuilderConnection, 1)
	q.cmp = func(a *BuilderConnection, b *BuilderConnection) bool {
		return a.H() > b.H()
	}
	return q
}

// Push a new builder connection on to the queue
// Locks the mutex
func (q *BuilderQueue) Push(bc *BuilderConnection) {
	q.mut.Lock()

	i := len(q.arr)
	bc.Index = i
	q.arr = append(q.arr, bc)
	q.percUpUnsafe(i)

	q.mut.Unlock()
}

// Pop the lowest usage builder connection from the queue
// Locks the mutex
func (q *BuilderQueue) Pop() *BuilderConnection {
	q.mut.Lock()

	ret := q.arr[1]
	q.arr[1] = q.arr[len(q.arr)-1]
	q.arr[1].Index = 1
	q.arr = q.arr[:len(q.arr)-1]
	q.percDownUnsafe(1)

	q.mut.Unlock()
	return ret
}

// Peeks at the top item on the queue
// Locks the mutex
func (q *BuilderQueue) Peek() *BuilderConnection {
	q.mut.Lock()
	p := q.arr[1]
	q.mut.Unlock()
	return p
}

// Get the length of the queue
// Locks the mutex
func (q *BuilderQueue) Len() int {
	q.mut.Lock()
	l := len(q.arr) - 1
	q.mut.Unlock()
	return l
}

// Swap two items
// Locks the mutex
func (q *BuilderQueue) Swap(i, j int) {
	q.mut.Lock()
	q.swapUnsafe(i, j)
	q.mut.Unlock()
}

// Swap to items
// Does not lock the mutex
func (q *BuilderQueue) swapUnsafe(i, j int) {
	q.arr[i], q.arr[j] = q.arr[j], q.arr[i]
	q.arr[i].Index = i
	q.arr[j].Index = j
}

// Percolate Up
// Locks the mut
func (q *BuilderQueue) PercUp(from int) {
	q.mut.Lock()
	q.percUpUnsafe(from)
	q.mut.Unlock()
}

// Percolate Up
// Does not lock the mut
func (q *BuilderQueue) percUpUnsafe(from int) {
	for from > 1 {
		if q.cmp(q.arr[from/2], q.arr[from]) {
			q.swapUnsafe(from/2, from)
			from /= 2
		} else {
			break
		}
	}
}

// Percolate down
// Locks the mut
func (q *BuilderQueue) PercDown(from int) {
	q.mut.Lock()
	q.percDownUnsafe(from)
	q.mut.Unlock()
}

// Percolate down
// Does not lock the mut
func (q *BuilderQueue) percDownUnsafe(from int) {
	for from*2 < len(q.arr) {
		left := from * 2
		right := left + 1
		if from*2+1 < len(q.arr) {
			if q.cmp(q.arr[from], q.arr[left]) {
				if q.cmp(q.arr[left], q.arr[right]) {
					q.swapUnsafe(from, right)
					from = right
				} else {
					q.swapUnsafe(from, left)
					from = left
				}
			} else if q.cmp(q.arr[from], q.arr[right]) {
				q.swapUnsafe(from, right)
				from = right
			} else {
				return
			}
		} else {
			if q.cmp(q.arr[from], q.arr[left]) {
				q.swapUnsafe(from, left)
				from = left
			} else {
				return
			}
		}
	}
}

// Remove the item at i from the queue
// This method locks the mutex
func (q *BuilderQueue) Remove(i int) {
	q.mut.Lock()
	q.removeUnsafe(i)
	q.mut.Unlock()
}

// Remove the item at i from the queue
// This method will not lock the mutex
func (q *BuilderQueue) removeUnsafe(i int) {
	q.arr[i] = q.arr[len(q.arr)-1]
	q.arr = q.arr[:len(q.arr)-1]
	q.percDownUnsafe(i)
}
