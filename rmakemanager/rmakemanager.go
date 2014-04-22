package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/whyrusleeping/rmake/types"
)

// Main manager type, basically globals right now
type Manager struct {
	getUuid chan int
	putUuid chan int
	bcMap   map[int]*BuilderConnection
	list    net.Listener
	queue	*BuilderQueue
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
	// The current number of jobs
	NumJobs int
	// The managing manager
	Manager *Manager
	// The gob encoder
	enc *gob.Encoder
	// The gob decoder
	dec *gob.Decoder
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
	return bc
}

func (b *BuilderConnection) H() int {
	return b.NumJobs
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

// Make a new manager
func NewManager(listname string) *Manager {
	//Start the server socket
	list, err := net.Listen("tcp", listname)
	if err != nil {
		log.Panic(err)
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
//TODO: time this and other handlers for performance analytics
func (m *Manager) HandleManagerRequest(request *rmake.ManagerRequest) {
	// handle the request

	//Take the freest node as the final node
	final := m.queue.Pop()
	final.NumJobs++
	m.queue.Push(final)

	//Find the 'final' job in our list
	var finaljob *rmake.Job
	for _,j := range request.Jobs {
		if request.Output == j.Output {
			finaljob = j
		}
	}
	br := new(rmake.BuilderRequest)
	br.BuildJob = finaljob
	br.Session = "GET A SESSION!" //TODO: method of creating and tracking sessions?
	br.ResultAddress = "manager" //Key string, recognized by builder
	for _,dep := range j.Deps {
		depfi, ok := request.Files[dep]
		if !ok {
			fmt.Printf("final builder will need to wait on %s\n", dep)
			br.Wait = append(br.Wait, dep)
		} else {
			br.Input = append(br.Input, depfi)
		}
	}

	fmt.Println("Sending job to '%s'\n", builder.Hostname)
	final.Send(br)


	//assign each job to a builder
	for _,j := range request.Jobs {
		if j == finaljob {
			continue
		}
		br := new(rmake.BuilderRequest)
		br.BuildJob = j

		br.Session = "GET A SESSION!"

		br.ResultAddress = final.Hostname

		for _,dep := range j.Deps {
			depfi, ok := request.Files[dep]
			if !ok {
				fmt.Printf("Builder will need to wait on %s\n", dep)
				br.Wait = append(br.Wait, dep)
			} else {
				br.Input = append(br.Input, depfi)
			}
		}

		builder := m.queue.Pop()
		fmt.Println("Sending job to '%s'\n", builder.Hostname)
		builder.Send(br)
		builder.NumJobs++
		m.queue.Push(builder)
	}
}

//
func (m *Manager) HandleBuilderAnnouncement(bldr *rmake.BuilderAnnouncement, con net.Conn) {
	fmt.Println("Handling announcement")
	var ack *rmake.ManagerAcknowledge
	// Make the new builder connection
	errored := false
	uuid := <-m.getUuid
	bc := NewBuilderConnection(con, bldr.ListenerAddr, uuid, bldr.Hostname, m)
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

func (m *Manager) HandleBuilderStatusUpdate(b *BuilderConnection, bsu *rmake.BuilderStatusUpdate) {
	//Get current heuristic
	cur := b.H()

	//Update values
	b.NumJobs = bsu.QueuedJobs
	//...

	if b.H() > cur {
		m.queue.percDown(b.Index)
	} else if b.H() < cur {
		m.queue.percUp(b.Index)
	}
}

// goroutine to handle a new connection from a client.
// Determines what resources are avaliable and what
// resources the request requires.
func (m *Manager) HandleConnection(c net.Conn) {
	var gobint interface{}


	dec := gob.NewDecoder(c)
	err := dec.Decode(&gobint)
	if err != nil {
		fmt.Println(err)
		panic(err)
		return
	}

	switch message := gobint.(type) {
	case *rmake.BuilderResult:
		fmt.Println("Builder Result")
	case *rmake.ManagerRequest:
		fmt.Println("Manager Request")
		m.HandleManagerRequest(message)
	case *rmake.BuilderAnnouncement:
		fmt.Println("Builder Announcement")
		m.HandleBuilderAnnouncement(message, c)
	default:
		fmt.Println("Unknown Type.\n")
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
