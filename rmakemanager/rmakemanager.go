package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"compress/gzip"
	"encoding/gob"
	"encoding/base64"
	"crypto/rand"

	"github.com/whyrusleeping/rmake/types"
)

// Main manager type, basically globals right now
type Manager struct {
	getUuid chan int
	putUuid chan int
	bcMap   map[int]*BuilderConnection
	list    net.Listener
	queue	*BuilderQueue
	sessions map[string]bool
}


// Make a new manager
func NewManager(listname string) *Manager {
	//Start the server socket
	log.Printf("Listening on '%s'\n", listname)
	list, err := net.Listen("tcp", listname)
	if err != nil {
		log.Panic(err)
	}
	m := new(Manager)
	m.getUuid = make(chan int)
	m.putUuid = make(chan int)
	m.bcMap = make(map[int]*BuilderConnection)
	m.sessions = make(map[string]bool)
	m.queue = NewBuilderQueue()
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
				fmt.Printf("Setting next UUID to: %d\n", nextUuid)
			}
		case id := <-m.putUuid:
			free = append(free, id)
		}
	}
}

func (m *Manager) GetNewSession() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	session := base64.StdEncoding.EncodeToString(bytes)
	fmt.Printf("Made new session: %s\n", session)
	m.sessions[session] = true
	return session
}

func (m *Manager) ReleaseSession(session string) {

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
	if finaljob == nil {
		fmt.Println("I have no idea what to do.")
		panic("confusion?!")
	}
	br := new(rmake.BuilderRequest)
	br.BuildJob = finaljob
	br.Session = "SESSIONSTANDIN" //TODO: method of creating and tracking sessions?
	br.ResultAddress = "manager" //Key string, recognized by builder

	for _,dep := range finaljob.Deps {
		depfi, ok := request.Files[dep]
		if !ok {
			fmt.Printf("final builder will need to wait on %s\n", dep)
			br.Wait = append(br.Wait, dep)
		} else {
			br.Input = append(br.Input, depfi)
		}
	}

	fmt.Printf("Sending job to '%s'\n", final.Hostname)
	final.Send(br)


	//assign each job to a builder
	for _,j := range request.Jobs {
		if j == finaljob {
			continue
		}
		br := new(rmake.BuilderRequest)
		br.BuildJob = j

		br.Session = "SESSIONSTANDIN"
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
		fmt.Printf("Sending job to '%s'\n", builder.Hostname)
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
		return
	}
	m.queue.Push(bc)
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
