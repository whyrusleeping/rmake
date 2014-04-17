package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"net"

	"github.com/whyrusleeping/rmake/types"
)

type Manager struct {
	getUuid chan int
	putUuid chan int

	list net.Listener
}

func NewManager(listname string) *Manager {

	//Start the server socket
	list, err := net.Listen("tcp", listname)
	if err != nil {
		panic(err)
	}
	m := new(Manager)
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
	//con.RemoteAddr()
}

// goroutine to handle a new connection from a client.
// Determines what resources are avaliable and what
// resources the request requires.
func (m *Manager) HandleConnection(c net.Conn) {
	var gobint interface{}

	unzip, err := gzip.NewReader(c)
	if err != nil {
		return
	}

	dec := gob.NewDecoder(unzip)
	err = dec.Decode(&gobint)
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
