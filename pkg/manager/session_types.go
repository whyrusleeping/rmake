package manager

import (
	"crypto/rand"
	"encoding/hex"
	"github.com/whyrusleeping/rmake/pkg/types"
)

type Session struct {
	// The session ID
	ID string
	// A map of the pending builds related to this session
	Builds map[int]*Build
	//
	getNewBuildID chan int
}

func NewSession() *Session {
	s := new(Session)
	bytes := make([]byte, 16)
	rand.Read(bytes)
	s.ID = hex.EncodeToString(bytes)
	s.Builds = make(map[int]*Build)
	s.getNewBuildID = make(chan int)
	go s.buildIDGenerator()
	return s
}

func (s *Session) buildIDGenerator() {
	nextID := 0
	for {
		select {
		case s.getNewBuildID <- nextID:
			nextID++
		}
	}
}

type Build struct {
	SessionID        string
	TotalJobs        int
	JobsDone         int
	ID               int
	getNewJobID      chan int
	Jobs             map[int]*rmake.Job
	AssignedBuilders map[*rmake.Job]*BuilderConnection
}

func NewBuild(s *Session) *Build {
	b := new(Build)
	b.SessionID = s.ID
	b.TotalJobs = 0
	b.JobsDone = 0
	b.getNewJobID = make(chan int)
	b.ID = <-s.getNewBuildID
	go b.jobIDGenerator()
	return b
}

func (b *Build) jobIDGenerator() {
	nextID := 0
	for {
		select {
		case b.getNewJobID <- nextID:
			nextID++
		}
	}
}

func NewJob(b *Build) *rmake.Job {
	j := new(rmake.Job)
	j.ID = <-b.getNewJobID
	return j
}
