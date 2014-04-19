package main

import (
	"testing"
	"math/rand"
	"time"
	"sort"
)

func TestQueue(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	q := NewBuilderQueue()
	arr := make([]int, 10)
	for i,_ := range arr {
		arr[i] = rand.Intn(64)
		q.Push(&BuilderConnection{NumJobs:arr[i]})
	}
	sort.Ints(arr)
	for _,v := range arr {
		if bc := q.Pop(); v != bc.NumJobs {
			t.Fatalf("%d != %d\n", v, bc.NumJobs)
		}
	}
}
