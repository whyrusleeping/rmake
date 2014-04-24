package main

import (
	"sync"

	"github.com/whyrusleeping/rmake/types"
)

// The RequestQueue
type RequestQueue struct {
	// The backing datastructure
	queue chan *rmake.BuilderRequest
	// The mutex for locking
	rwmutex sync.RWMutex
}

// Push a request to the RequestQueue
func (jq *RequestQueue) Push(br *rmake.BuilderRequest) {
	jq.rwmutex.Lock()
	jq.queue <- br
	jq.rwmutex.Unlock()
}

// Pop a request from the RequestQueue
func (jq *RequestQueue) Pop() *rmake.BuilderRequest {
	jq.rwmutex.Lock()
	p := <-jq.queue
	jq.rwmutex.Unlock()
	return p
}

// The length of the RequestQueue
func (jq *RequestQueue) Len() int {
	jq.rwmutex.RLock()
	l := jq.lenUnsafe()
	jq.rwmutex.RUnlock()
	return l
}

func (jq *RequestQueue) lenUnsafe() int {
	return len(jq.queue)
}

// Abort everything with a specific ID
func (jq *RequestQueue) Remove(id int) []*rmake.BuilderRequest {
	s := make([]*rmake.BuilderRequest, 0)
	// Critical section
	jq.rwmutex.Lock()
	n := jq.lenUnsafe()
	// Iterate over each request in the queue
	for i := 0; i < n; i++ {
		request := <-jq.queue
		if request.BuildJob.ID == id {
			jq.queue <- request
		} else {
			s = append(s, request)
			i++ // Add another
		}
	}
	jq.rwmutex.Unlock()
	return s
}
