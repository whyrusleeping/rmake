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
	open bool
}

func NewRequestQueue() *RequestQueue {
	rq := new(RequestQueue)
	rq.queue = make(chan *rmake.BuilderRequest, 64)
	rq.open = true
	return rq
}

// Push a request to the RequestQueue
func (jq *RequestQueue) Push(br *rmake.BuilderRequest) {
	jq.rwmutex.RLock()
	jq.queue <- br
	jq.rwmutex.RUnlock()
}

// Pop a request from the RequestQueue
func (jq *RequestQueue) Pop() (*rmake.BuilderRequest, bool) {
	jq.rwmutex.RLock()
	if !jq.open {
		return nil,false
	}
	p := <-jq.queue
	jq.rwmutex.RUnlock()
	return p,true
}

// The length of the RequestQueue
func (jq *RequestQueue) Len() int {
	jq.rwmutex.RLock()
	l := jq.lenUnsafe()
	jq.rwmutex.RUnlock()
	return l
}

func (jq *RequestQueue) Close() {
	jq.rwmutex.Lock()
	jq.open = false
	jq.rwmutex.Unlock()
}

// Get the length of the queue in an unsafe manner
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
