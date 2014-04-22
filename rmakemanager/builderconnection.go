package main

import (
	"encoding/gob"
	"net"

	slog "github.com/cihub/seelog"
	"github.com/whyrusleeping/rmake/types"
)

// Builder Connection Type
type BuilderConnection struct {
	// The builder's uuid
	UUID int
	// The builder's hostname
	Hostname string
	// The listening address
	ListenerAddr string
	// The backing network connection
	conn net.Conn
	// The current number of jobs
	NumJobs int
	// The managing manager
	Manager *Manager
	// The gob encoder
	enc *gob.Encoder
	// The gob decoder
	dec *gob.Decoder
	// Index in the priority queue

	//Channel for messages coming from the builder to the manager
	Incoming chan interface{}

	//Channel for messages going to the builder
	Outgoing chan interface{}

	Index int
}

// Sets up a new builder connection
func NewBuilderConnection(c net.Conn, la string, uuid int, hn string, m *Manager) *BuilderConnection {
	// Build bulder connection
	bc := new(BuilderConnection)
	bc.UUID = uuid
	bc.Hostname = hn
	bc.ListenerAddr = la
	bc.conn = c
	bc.NumJobs = 0
	bc.Manager = m
	bc.enc = gob.NewEncoder(c)
	bc.dec = gob.NewDecoder(c)
	bc.Incoming = make(chan interface{})
	bc.Outgoing = make(chan interface{})
	return bc
}

func (b *BuilderConnection) Listen() {
	go func () {

	}()
	var i interface{}
	for {
		err := b.dec.Decode(&i)
		if err != nil {
			slog.Critical(err)
			return
		}

	}
}

//Sorting Heuristic
func (b *BuilderConnection) H() int {
	return b.NumJobs
}

//TODO: this should be more asynchronous.
//Should have a goroutine looping somewhere listening on a channel
//for new messages to send
func (b *BuilderConnection) Send(i interface{}) error {
	err := b.enc.Encode(&i)
	if err != nil {
		return err
	}
	//b.wri.Flush()
	return nil
}

func (b *BuilderConnection) HandleStatusUpdate(bsu *rmake.BuilderStatusUpdate) {
	b.Manager.HandleBuilderStatusUpdate(b, bsu)
}

//
func (b *BuilderConnection) Recieve() (interface{}, error) {
	var i interface{}
	err := b.dec.Decode(&i)
	if err != nil {
		return nil, err
	}
	return i, nil
}

