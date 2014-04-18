package main

import "errors"

type BuilderQueue map[int][]*BuilderConnection

func (q BuilderQueue) Push(b *BuilderConnection) {
	slc, exists := q[b.NumJobs]
	if !exists {
		slc = []*BuilderConnection{b}
	} else {
		slc = append(slc, b)
	}
	q[b.NumJobs] = slc
}

func (q BuilderQueue) Pop(b *BuilderConnection) (*BuilderConnection, error) {
	var bc *BuilderConnection
	if len(q) == 0 {
		return bc, errors.New("Error, no items in queue.")
	}
	for k, v := range q {
		bc = v[0]

		bc, q[k] = v[len(v)-1], v[:len(v)-1]
		if len(q[k]) == 0 {
			delete(q, k)
		}
	}
	return bc, nil
}
