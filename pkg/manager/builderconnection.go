package manager

import (
	"encoding/gob"
	"net"

	slog "github.com/cihub/seelog"
)

// BuilderConnection handles communications with a certain rmake builder node.
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
	bc.enc = gob.NewEncoder(c)
	bc.dec = gob.NewDecoder(c)
	bc.Outgoing = make(chan interface{})
	bc.Incoming = m.Incoming
	return bc
}

// The listener
func (b *BuilderConnection) Listener() {
	var i interface{}
	for {
		err := b.dec.Decode(&i)
		if err != nil {
			slog.Critical(err)

			//TODO: remove this builder from the managers queue
			b.conn.Close()
			b.Incoming <- b
			return
		}
		slog.Info("Recieved message from builder.")
		b.Incoming <- i
	}
}

// waits for messages from the manager and sends them off to the builder
func (b *BuilderConnection) Sender() {
	for {
		i,ok := <-b.Outgoing
		if !ok {
			return
		}
		err := b.enc.Encode(&i)
		if err != nil {
			panic(err)
		}
	}
}

//Sorting Heuristic
func (b *BuilderConnection) H() int {
	return b.NumJobs
}
