package main

import (
	"encoding/gob"
	"net"
	"strings"

	slog "github.com/cihub/seelog"
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
	//Channel for messages coming from the builder to the manager
	Incoming chan interface{}
	//Channel for messages going to the builder
	Outgoing chan interface{}
	// Index in the priority queue
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
	addr := c.RemoteAddr().String()
	bc.Hostname = strings.Split(addr, ":")[0]
	bc.enc = gob.NewEncoder(c)
	bc.dec = gob.NewDecoder(c)
	bc.Outgoing = make(chan interface{})
	bc.Incoming = m.Incoming
	return bc
}

func (b *BuilderConnection) Listen() {
	go func() {
		for {
			i := <-b.Outgoing
			err := b.enc.Encode(&i)
			if err != nil {
				panic(err)
			}
		}
	}()
	var i interface{}
	for {
		err := b.dec.Decode(&i)
		if err != nil {
			slog.Critical(err)

			//TEMP! only for debugging
			b.conn.Close()
			return
		}
		slog.Info("Recieved message from builder.")
		b.Incoming <- i
	}
}

//Sorting Heuristic
func (b *BuilderConnection) H() int {
	return b.NumJobs
}
