package main

import (
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"flag"
	"net"

	"reflect"

	log "github.com/cihub/seelog"
	"github.com/whyrusleeping/rmake/types"
)

// Main manager type, basically globals right now
type Manager struct {
	getUuid  chan int
	putUuid  chan int
	bcMap    map[int]*BuilderConnection
	list     net.Listener
	queue    *BuilderQueue
	sessions map[string]chan interface{}

	//Messages coming in to the manager
	Incoming chan interface{}
}

// Make a new manager
func NewManager(listname string) *Manager {
	//Start the server socket
	log.Infof("Listening on '%s'\n", listname)
	list, err := net.Listen("tcp", listname)
	if err != nil {
		log.Error(err)
		panic(err)
	}
	m := new(Manager)
	m.getUuid = make(chan int)
	m.putUuid = make(chan int)
	m.bcMap = make(map[int]*BuilderConnection)
	m.sessions = make(map[string]chan interface{})
	m.queue = NewBuilderQueue()
	m.list = list
	m.Incoming = make(chan interface{})
	go m.UUIDGenerator()
	go m.MessageListener()
	return m
}

func (m *Manager) SendToClient(session string, mes interface{}) {
	ch, ok := m.sessions[session]
	if ok {
		ch <- mes
	} else {
		log.Critical("Tried to send message to nonexistant client session!")
	}
}

//All incoming messages are synchronized here
func (m *Manager) MessageListener() {
	for {
		mes := <-m.Incoming
		switch mes := mes.(type) {
		case *rmake.BuildStatus:
			log.Info("Build Status Update.")
			log.Infof("Session: %d Completion: %f", mes.Session, mes.PercentComplete)
		case *rmake.BuilderResult:
			fbr := new(rmake.FinalBuildResult)
			fbr.Results = mes.Results
			fbr.Session = mes.Session
			fbr.Success = true
			m.SendToClient(mes.Session, fbr)

		case *rmake.JobFinishedMessage:
			log.Infof("Job finished for session: %s", mes.Session)
			//TODO, update build info

			m.SendToClient(mes.Session, mes)

		case *rmake.BuilderStatusUpdate:
			log.Info("Builder updated load")
		default:
			log.Warn("Unrecognized message type")
			log.Warn(reflect.TypeOf(mes))
		}
	}
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
				log.Infof("Setting next UUID to: %d\n", nextUuid)
			}
		case id := <-m.putUuid:
			free = append(free, id)
		}
	}
}

//Generates a random session string and registers it in the session map
func (m *Manager) GetNewSession() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	session := hex.EncodeToString(bytes)
	log.Infof("Made new session: %s\n", session)
	m.sessions[session] = make(chan interface{})
	return session
}

//When a session isstring complete, remove it from the session map
func (m *Manager) ReleaseSession(session string) {

}

// Allocate resources to the request
//TODO: time this and other handlers for performance analytics
/*Pseudocode for future implementation

...
//Start with
assignJobs(finaljob)
...

func assignJobs(j job) {
	j.sendTo(builders.GetNext())
	j.assigned = true
	for dep in j.deps {
		if dep in filesfromclient {
			continue
		}
		sub = jobsByOutput[dep]
		if sub == nil {
			panic("no such thing!")
		}
		if !sub.assigned {
			assignJobs(sub)
		}
	}
}

This will ensure that the build dependency heirarchy is satisfied
*/
func (m *Manager) HandleManagerRequest(request *rmake.ManagerRequest, c net.Conn) {
	// handle the request
	session := m.GetNewSession()

	//Take the freest node as the final node
	final := m.queue.Pop()
	final.NumJobs++
	m.queue.Push(final)

	//Find the 'final' job in our list
	var finaljob *rmake.Job
	for _, j := range request.Jobs {
		if request.Output == j.Output {
			finaljob = j
		}
	}
	if finaljob == nil {
		log.Error("I have no idea what to do.")
		//TODO: Determine how to handle this case
		panic("confusion?!")
	}
	br := new(rmake.BuilderRequest)
	br.BuildJob = finaljob
	br.Session = session         //TODO: method of creating and tracking sessions?
	br.ResultAddress = "manager" //Key string, recognized by builder

	for _, dep := range finaljob.Deps {
		depfi, ok := request.Files[dep]
		if !ok {
			log.Infof("final builder will need to wait on %s\n", dep)
			br.Wait = append(br.Wait, dep)
		} else {
			br.Input = append(br.Input, depfi)
		}
	}

	log.Infof("Sending job to '%s'\n", final.ListenerAddr)
	final.Outgoing <- br

	//assign each job to a builder
	for _, j := range request.Jobs {
		if j == finaljob {
			continue
		}
		br := new(rmake.BuilderRequest)
		br.BuildJob = j

		br.Session = session
		br.ResultAddress = final.ListenerAddr

		for _, dep := range j.Deps {
			depfi, ok := request.Files[dep]
			if !ok {
				log.Infof("Builder will need to wait on %s\n", dep)
				br.Wait = append(br.Wait, dep)
			} else {
				br.Input = append(br.Input, depfi)
			}
		}

		builder := m.queue.Pop()
		log.Infof("Sending job to '%s'\n", builder.Hostname)
		builder.Outgoing <- br
		builder.NumJobs++
		m.queue.Push(builder)
	}

	// Reply to client
	enc := gob.NewEncoder(c)
	for {
		mes := <-m.sessions[session]
		err := enc.Encode(&mes)
		if err != nil {
			log.Warn(err)
			return
		}
	}
}

// Handles a builder announcement
func (m *Manager) HandleBuilderAnnouncement(bldr *rmake.BuilderAnnouncement, con net.Conn) {
	log.Info("Handling announcement")
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
	i := interface{}(ack)
	err := bc.enc.Encode(&i)
	if err != nil {
		log.Error(err)
	}

	// If we errored above, free the uuid
	if errored {
		log.Error("Errored, returning UUID")
		m.putUuid <- uuid
		return
	}
	go bc.Sender()
	go bc.Listener()

	//TODO: potential race condition here
	m.queue.Push(bc)
}

func (m *Manager) HandleBuilderStatusUpdate(b *BuilderConnection, bsu *rmake.BuilderStatusUpdate) {
	//Get current heuristic
	cur := b.H()

	//Update values
	b.NumJobs = bsu.QueuedJobs
	//...

	if b.H() > cur {
		m.queue.PercDown(b.Index)
	} else if b.H() < cur {
		m.queue.PercUp(b.Index)
	}
}

// goroutine to handle a new connection from a client.
// Determines what resources are avaliable and what
// resources the request requires.
func (m *Manager) HandleConnection(c net.Conn) {
	// Decode the message
	var gobint interface{}

	dec := gob.NewDecoder(c)
	err := dec.Decode(&gobint)
	if err != nil {
		log.Error(err)
		panic(err)
	}

	switch message := gobint.(type) {
	case *rmake.ManagerRequest:
		log.Info("Manager Request")
		m.HandleManagerRequest(message, c)
	case *rmake.BuilderAnnouncement:
		m.HandleBuilderAnnouncement(message, c)
		//return
	default:
		log.Info(reflect.TypeOf(message))
		log.Info("Unknown Type.")
	}

	return
}

func (m *Manager) Start() {
	//Accept and handle new client connections
	for {
		con, err := m.list.Accept()
		if err != nil {
			log.Error(err)
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

	log.Info("rmakemanager")

	manager := NewManager(listname)
	manager.Start()
}
