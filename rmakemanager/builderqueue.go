package main

type BuilderQueue struct {
	arr []*BuilderConnection

	//Sorting Comparison function
	cmp func(*BuilderConnection, *BuilderConnection) bool
}

func NewBuilderQueue() *BuilderQueue {
	q := new(BuilderQueue)
	q.arr = make([]*BuilderConnection, 1)
	q.cmp = func (a *BuilderConnection, b *BuilderConnection) bool {
		return a.H() > b.H()
	}
	return q
}

func (q *BuilderQueue) Push(bc *BuilderConnection) {
	i := len(q.arr)
	bc.Index = i
	q.arr = append(q.arr, bc)
	q.percUp(i)
}

func (q *BuilderQueue) Pop() *BuilderConnection {
	ret := q.arr[1]
	q.arr[1] = q.arr[len(q.arr) - 1]
	q.arr[1].Index = 1
	q.arr = q.arr[:len(q.arr) - 1]
	q.percDown(1)
	return ret
}

func (q *BuilderQueue) Peek() *BuilderConnection {
	return q.arr[1]
}

func (q *BuilderQueue) Len() int {
	return len(q.arr) - 1
}

func (q *BuilderQueue) percUp(from int) {
	for from > 1 {
		if q.cmp(q.arr[from/2], q.arr[from]) {
			q.Swap(from/2, from)
			from /= 2
		} else {
			break
		}
	}
}

func (q *BuilderQueue) Swap(i, j int) {
	q.arr[i], q.arr[j] = q.arr[j], q.arr[i]
	q.arr[i].Index = i
	q.arr[j].Index = j
}

func (q *BuilderQueue) percDown(from int) {
	for from*2 < len(q.arr) {
		left := from * 2
		right := left + 1
		if from*2+1 < len(q.arr) {
			if q.cmp(q.arr[from], q.arr[left]) {
				if q.cmp(q.arr[left], q.arr[right]) {
						q.Swap(from, right)
						from = right
				} else {
					q.Swap(from, left)
					from = left
				}
			} else if q.cmp(q.arr[from], q.arr[right]) {
				q.Swap(from, right)
				from = right
			} else {
				return
			}
		} else {
			if q.cmp(q.arr[from], q.arr[left]) {
				q.Swap(from, left)
				from = left
			} else {
				return
			}
		}
	}
}

func (q *BuilderQueue) Remove(i int) {
	q.arr[i] = q.arr[len(q.arr) - 1]
	q.arr = q.arr[:len(q.arr) - 1]
	q.percDown(i)
}
