package rmake

import (
	"crypto/rand"
	"encoding/hex"
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
	Jobs             map[int]*Job
	AssignedBuilders map[*Job]*BuilderConnection
}

func NewBuild(s *Session) *Build {
	b := new(Build)
	b.SessionID = s.ID
	b.TotalJobs = 0
	b.JobsDone = 0
	b.ID = 0
	return b
}

type Job struct {
	Command string
	Args    []string
	Deps    []string
	Output  string
	ID      int
}
