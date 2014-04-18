package main

import "errors"

type BuilderQueue map[int][]*BuilderConnection

// Not necessary unless we expand the type
func NewBuilderQueue() *BuilderQueue {
	bq := new(BuilderQueue)
	return bq
}

// Insert a Builder Connection in the correct location
func (q BuilderQueue) Push(b *BuilderConnection) {
	// Check if this item exists
	slc, exists := q[b.NumJobs]
	if !exists {
		// Not there, make a new slice
		slc = []*BuilderConnection{b}
	} else {
		// Already exists, append
		slc = append(slc, b)
	}
	q[b.NumJobs] = slc
}

// Remove a Builder Connection by least demand
func (q BuilderQueue) Pop(b *BuilderConnection) (*BuilderConnection, error) {
	var bc *BuilderConnection
	if len(q) == 0 {
		return bc, errors.New("Error, no items in queue.")
	}
	for k, v := range q {
		// Grab the bc and update the slice
		bc, q[k] = v[len(v)-1], v[:len(v)-1]
		if len(q[k]) == 0 {
			// The slice is now empty, remove the entry.
			delete(q, k)
		}
	}
	return bc, nil
}
