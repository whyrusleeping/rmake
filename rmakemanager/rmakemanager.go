package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"net"

	"github.com/whyrusleeping/rmake/types"
)

// Main manager type, basically globals right now
type Manager struct {
	getUuid chan int
	putUuid chan int
	bcMap   map[int]*BuilderConnection
	list    net.Listener
}

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
	// The gzip writing pipe
	//	wri *gzip.Writer
	// The gzip reading pipe
	//	rea *gzip.Reader
	// The gob encoder
	enc *gob.Encoder
	// The gob decoder
	dec *gob.Decoder
}

// Sets up a new builder connection
func NewBuilderConnection(c net.Conn, la string, uuid int, hn string) *BuilderConnection {
	// Build bulder connection
	bc := new(BuilderConnection)
	bc.UUID = uuid
	bc.Hostname = hn
	bc.ListenerAddr = la
	bc.conn = c
	bc.enc = gob.NewEncoder(c)
	bc.dec = gob.NewDecoder(c)
	return bc
}

//
func (b *BuilderConnection) Send(i interface{}) error {
	err := b.enc.Encode(&i)
	if err != nil {
		return err
	}
	//b.wri.Flush()
	return nil
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

// Make a new manager
func NewManager(listname string) *Manager {
	//Start the server socket
	list, err := net.Listen("tcp", listname)
	if err != nil {
		panic(err)
	}
	m := new(Manager)
	m.getUuid = make(chan int)
	m.putUuid = make(chan int)
	m.bcMap = make(map[int]*BuilderConnection)
	m.list = list
	go m.UUIDGenerator()
	return m
}

func (m *Manager) UUIDGenerator() {
	var free []int
	nextUuid := 0
	maxUuid := 0
	for {
		select {
		case m.getUuid <- nextUuid:
			if len(free) > 0 {
				nextUuid = free[0]
				free = free[1:]
			} else {
				maxUuid++
				nextUuid = maxUuid
				fmt.Println("Setting next UUID to: %d\n", nextUuid)
			}
		case id := <-m.putUuid:
			free = append(free, id)
		}
	}
}

// Allocate resources to the request
func (m *Manager) HandleManagerRequest(request *rmake.ManagerRequest) {
	// handle the request

	// So do we want to keep a line open for the FinalBuildResult?
	// Or do we want a handler for that too?
	// Questions for tomorrow.
}

//
func (m *Manager) HandleBuilderAnnouncement(bldr *rmake.BuilderAnnouncement, con net.Conn) {
	fmt.Println("Handling announcement")
	var ack *rmake.ManagerAcknowledge
	// Make the new builder connection
	errored := false
	uuid := <-m.getUuid
	bc := NewBuilderConnection(con, bldr.ListenerAddr, uuid, bldr.Hostname)
	if bldr.ProtocolVersion == rmake.ProtocolVersion {
		// Looks good, add to map and send back success
		ack = rmake.NewManagerAcknowledgeSuccess(uuid)
		m.bcMap[uuid] = bc
	} else {
		// Mismatch send failure
		ack = rmake.NewManagerAcknowledgeFailure("Error, protocol version mismatch")
		errored = true
	}
	// Send off the ack
	err := bc.Send(ack)
	if err != nil {
		fmt.Println(err)
	}
	// If we errored above, free the uuid
	if errored {
		fmt.Println("Errored, returning UUID")
		m.putUuid <- uuid
	}
}

// goroutine to handle a new connection from a client.
// Determines what resources are avaliable and what
// resources the request requires.
func (m *Manager) HandleConnection(c net.Conn) {
	var gobint interface{}

	/*
		unzip, err := gzip.NewReader(c)
		if err != nil {
			return
		}*/

	dec := gob.NewDecoder(c)
	err := dec.Decode(&gobint)
	if err != nil {
		fmt.Println(err)
		return
	}

	switch gobtype := gobint.(type) {
	case *rmake.BuilderResult:
		fmt.Printf("Builder Result: %d\n", gobtype)
	case *rmake.ManagerRequest:
		fmt.Printf("Manager Request: %d\n", gobtype)
	case *rmake.BuilderAnnouncement:
		fmt.Printf("Builder Announcement: %d\n", gobtype)
		m.HandleBuilderAnnouncement(gobint.(*rmake.BuilderAnnouncement), c)
	default:
		fmt.Printf("Unknown Type.\n", gobtype)
	}

	return
}

func (m *Manager) Reply(i interface{}, addr string) error {

	fmt.Println("Replying to %s\n", addr)

	mgr, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	wrt := gzip.NewWriter(mgr)
	enc := gob.NewEncoder(wrt)

	err = enc.Encode(&i)
	if err != nil {
		return err
	}
	wrt.Close()
	mgr.Close()
	return nil
}

func (m *Manager) Start() {
	//Accept and handle new client connections
	for {
		con, err := m.list.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		//Handle clients in separate 'thread'
		go m.HandleConnection(con)
	}
}

// main
func main() {
	//Listens on port 11221 by default
	var listname string
	// Arguement parsing
	flag.StringVar(&listname,
		"listname", ":11221", "The ip and or port to listen on")
	flag.StringVar(&listname,
		"l", ":11221", "The ip and or port to listen on (shorthand)")

	flag.Parse()

	fmt.Println("rmakemanager\n")

	manager := NewManager(listname)
	manager.Start()
}
