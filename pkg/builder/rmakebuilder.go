package builder

import (
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"time"

	"reflect"

	slog "github.com/cihub/seelog"
	"github.com/whyrusleeping/rmake/pkg/types"
)

type Builder struct {
	manager net.Conn
	enc     *gob.Encoder
	dec     *gob.Decoder
	list    net.Listener

	ListenerAddr string
	ManagerAddr  string

	UpdateFrequency time.Duration
	Running         bool

	//To synchronize socket reads and writes
	incoming chan interface{}
	outgoing chan interface{}

	//Some data structures to synchronize file transfers
	waitfile    map[string]chan *rmake.File
	reqfilewait chan *FileWait
	newfiles    chan *rmake.RequiredFileMessage

	mgrReconnect chan struct{}

	Procs int
	UUID  int

	Halt chan struct{}

	RequestQueue *RequestQueue
	//Job Queue TODO: use this?
	//JQueue      chan *rmake.BuilderRequest
	RunningJobs chan struct{}
}

//A struct to aid in waiting on dependency files
type FileWait struct {
	File    string
	Session string
	Reply   chan *rmake.File
}

func NewBuilder(listen string, manager string, nprocs int) *Builder {
	// Setup manager connection
	mgr, err := net.Dial("tcp", manager)
	if err != nil {
		slog.Error(err)
		slog.Error("Could not connect to manager.")
		return nil
	}

	// Setup socket to listen to
	list, err := net.Listen("tcp", listen)
	if err != nil {
		mgr.Close()
		slog.Critical(err)
		return nil
	}

	//Make sure build directory exists
	os.Mkdir("builds", 0777|os.ModeDir)

	// Build new builder
	b := new(Builder)
	b.list = list
	b.Procs = nprocs
	b.ListenerAddr = listen
	b.ManagerAddr = manager
	b.manager = mgr
	b.enc = gob.NewEncoder(mgr)
	b.dec = gob.NewDecoder(mgr)

	b.incoming = make(chan interface{})
	b.outgoing = make(chan interface{})

	b.newfiles = make(chan *rmake.RequiredFileMessage)
	b.waitfile = make(map[string]chan *rmake.File)
	b.reqfilewait = make(chan *FileWait)

	b.RequestQueue = NewRequestQueue()
	//b.JQueue = make(chan *rmake.BuilderRequest)
	b.RunningJobs = make(chan struct{}, nprocs)

	b.UpdateFrequency = time.Second * 60
	b.Halt = make(chan struct{})
	b.mgrReconnect = make(chan struct{})

	for i := 0; i < nprocs; i++ {
		go b.BuilderThread()
	}
	return b
}

//Handles incoming files and requests for them
func (b *Builder) FileSyncRoutine() {
	for {
		select {
		case req := <-b.reqfilewait:
			wpath := path.Join("builds", req.Session, req.File)
			slog.Infof("Now waiting on: '%s'", wpath)
			b.waitfile[wpath] = req.Reply
		case fi := <-b.newfiles:
			if fi.Payload == nil {
				slog.Error("Received nil file!")
				continue
			}
			wpath := path.Join("builds", fi.Session, fi.Payload.Path)
			ch, ok := b.waitfile[wpath]
			if !ok {
				slog.Warnf("Received file nobody was asking for, session: '%s', path: '%s'",
					fi.Session, fi.Payload.Path)
			}
			ch <- fi.Payload
			delete(b.waitfile, wpath)
		}
	}
}

//Register a listener for receiving a certain file
func (b *Builder) WaitForFile(session, file string) chan *rmake.File {
	fw := new(FileWait)
	fw.File = file
	fw.Session = session
	fw.Reply = make(chan *rmake.File)

	b.reqfilewait <- fw
	return fw.Reply
}

//A routine that waits for jobs in the job queue
//One of these should be spawned per processor core.
//TODO: Eventually add in shutdown channel to the select statement
//for clean shutdowns
func (b *Builder) BuilderThread() {
	for {
		work := b.RequestQueue.Pop()
		b.RunningJobs <- struct{}{}
		b.RunJob(work)
		<-b.RunningJobs
	}
}

//
func (b *Builder) RunJob(req *rmake.BuilderRequest) {
	slog.Infof("Starting job for session: '%s'\n", req.Session)
	sdir := path.Join("builds", req.Session)
	os.Mkdir(sdir, 0777|os.ModeDir)

	for _, f := range req.Input {
		err := f.Save(sdir)
		if err != nil {
			slog.Error(err)
		}
	}

	var waitlist []chan *rmake.File
	for _, dep := range req.BuildJob.Deps {
		depPath := path.Join(sdir, dep)
		_, err := os.Stat(depPath)
		if err != nil {
			slog.Infof("Missing dependency: '%s'\n", dep)
			fch := b.WaitForFile(req.Session, dep)
			waitlist = append(waitlist, fch)
		}
	}

	for _, ch := range waitlist {
		f := <-ch
		slog.Infof("Got file we were waiting for: '%s'", f.Path)
		f.Save(sdir)
	}

	resp := new(rmake.JobFinishedMessage)
	cmd := exec.Command(req.BuildJob.Command, req.BuildJob.Args...)
	cmd.Dir = sdir

	out, err := cmd.CombinedOutput()
	resp.Stdout = string(out)
	resp.Session = req.Session
	if err != nil {
		slog.Error(err)
		resp.Error = err.Error()
		resp.Success = false
	}
	slog.Info(resp.Stdout)
	b.SendToManager(resp)

	if req.ResultAddress == "" {
		//Actually... shouldnt happen
		slog.Info("Im the final node! no need to send.")
		return
	}

	var outEnc *gob.Encoder
	if req.ResultAddress == "manager" {
		//Send to manager
		outEnc = nil
		fmt.Println("Sending back to manager!")
	} else {
		//Send to other builder
		fmt.Printf("Sending output to: %s\n", req.ResultAddress)
		send, err := net.Dial("tcp", req.ResultAddress)
		if err != nil {
			slog.Error(err)
			slog.Error("Given incorrection information by manager.")
			//TODO: decide what to do if this happens
			slog.Error("ERROR: this is pretty bad... what do?")
		}
		outEnc = gob.NewEncoder(send)
	}

	fipath := path.Join("builds", req.Session)
	slog.Infof("Loading %s to send on.\n", req.BuildJob.Output)
	fi, err := rmake.LoadFile(fipath, req.BuildJob.Output)
	if err != nil {
		slog.Error("Failed to load output file!")
	}

	if outEnc == nil {
		results := new(rmake.BuilderResult)
		if fi != nil {
			results.Results = append(results.Results, fi)
		}
		results.Session = req.Session
		b.SendToManager(results)
	} else {
		rfi := new(rmake.RequiredFileMessage)
		rfi.Session = req.Session
		rfi.Payload = fi
		i := interface{}(rfi)
		err := outEnc.Encode(&i)
		if err != nil {
			slog.Error(err)
		}
	}

	slog.Infof("Job for session '%s' finished.\n", req.Session)
}

func (b *Builder) Run() {
	slog.Info("Starting builder.")
	b.Running = true
	// Start Listeners
	go b.ManagerListener()
	go b.SocketListener()

	// Start message handler
	go b.HandleMessages()
	go b.ManagerSender()
	go b.FileSyncRoutine()

	// Start Heartbeat
	go b.StartPublisher()

	<-b.Halt

	slog.Info("Shutting down builder.")
	b.Running = false
	b.list.Close()
	b.manager.Close()
}

func (b *Builder) Stop() {
	b.Halt <- struct{}{}
}

//poll for messages from manager
func (b *Builder) ManagerListener() {
	for {
		mes, err := b.ReceiveFromManager()
		if err != nil {
			if err.Error() == "EOF" {
				slog.Warn("Connection to manager closed. Attemping reconnect in 5 seconds.")
				time.Sleep(time.Second * 5)
				con, err := net.Dial("tcp", b.ManagerAddr)
				if err != nil {
					slog.Errorf("Reconnect failed: %s", err)
					os.Exit(1)
				}
				b.manager = con
				b.dec = gob.NewDecoder(con)
				b.enc = gob.NewEncoder(con)
				b.DoHandshake()
				continue
			} else {
				panic(err)
			}
		}
		b.incoming <- mes
	}
}

//Synchronize sending messages to manager
func (b *Builder) ManagerSender() {
	for {
		mes := <-b.outgoing
		err := b.enc.Encode(&mes)
		if err != nil {
			slog.Critical(err)
			panic("Error!")
		}
	}
}

//Right now this function is fairly pointless, but eventually
//we will put some extra logic here to determine what can be handled
//asynchronously
func (b *Builder) HandleMessages() {
	for {
		m := <-b.incoming
		switch message := m.(type) {
		case *rmake.RequiredFileMessage:
			slog.Info("Received required file.")
			//Get a file from another node
			b.newfiles <- message

		case *rmake.BuilderRequest:
			slog.Info("Received builder request.")
			b.RequestQueue.Push(message)
			//b.JQueue <- messag/e

		case *rmake.BuilderResult:
			slog.Info("Received builder result.")
			b.HandleBuilderResult(message)

		default:
			slog.Warnf("Received invalid message type. '%s'", reflect.TypeOf(message))
		}
	}
}

func (b *Builder) HandleBuilderResult(m *rmake.BuilderResult) {
	sdir := path.Join("builds", m.Session)
	for _, f := range m.Results {
		err := f.Save(sdir)
		if err != nil {
			slog.Error("Error saving file!")
			slog.Error(err)
		}
	}
}

//Listen for and handle new connections
func (b *Builder) SocketListener() {
	for {
		con, err := b.list.Accept()
		if err != nil {
			slog.Error(err)
			if b.Running {
				//Diagnose?
				slog.Errorf("Listener Error: %s", err)
				continue
			} else {
				slog.Error("Shutting down server socket...")
				return
			}
		}
		go b.HandleConnection(con)
	}
}

// Send a message to the manager
func (b *Builder) SendToManager(i interface{}) {
	slog.Infof("Send to manager '%s'", reflect.TypeOf(i))
	b.outgoing <- i
}

// Read a message from the manager
func (b *Builder) ReceiveFromManager() (interface{}, error) {
	var i interface{}
	err := b.dec.Decode(&i)
	slog.Info("Received from manager.")
	if err != nil {
		return nil, err
	}
	return i, nil
}

// Handles all messages except handshake message types
func (b *Builder) HandleMessage(i interface{}) {
	switch message := i.(type) {
	case *rmake.RequiredFileMessage:
		slog.Info("Received required file.")
		//Get a file from another node
		b.newfiles <- message

	case *rmake.BuilderRequest:
		slog.Info("Received builder request.")
		b.RequestQueue.Push(message)
		//b.JQueue <- message

	case *rmake.BuilderResult:
		slog.Info("Received builder result.")
		sdir := path.Join("builds", message.Session)
		for _, f := range message.Results {
			err := f.Save(sdir)
			if err != nil {
				slog.Error("Error saving file!")
				slog.Error(err)
			}
		}

	default:
		slog.Warnf("Received invalid message type. '%s'", reflect.TypeOf(message))
	}
}

// Handles a connection from the listener
// This could come from another builder.
// Possibly from the manager too, but most likely another builder.
func (b *Builder) HandleConnection(con net.Conn) {
	slog.Info("Handling new connection from %s", con.RemoteAddr().String())
	dec := gob.NewDecoder(con)
	var i interface{}
	err := dec.Decode(&i)
	if err != nil {
		slog.Error(err)
		return
	}
	//b.HandleMessage(i)
	// We'll only handle one message ber connection
	b.incoming <- i
	con.Close()
}

func (b *Builder) SendStatusUpdate() {
	slog.Info("Sending system load update!")
	stat := new(rmake.BuilderStatusUpdate)
	stat.CPULoad = GetCpuUsage()
	stat.QueuedJobs = b.RequestQueue.Len() //len(b.JQueue)
	stat.RunningJobs = len(b.RunningJobs)

	b.SendToManager(stat)
}

func (b *Builder) StartPublisher() {
	tick := time.NewTicker(b.UpdateFrequency)
	for {
		select {
		case <-tick.C:
			b.SendStatusUpdate()
			//TODO: have a 'shutdown' channel
		}
	}
}

func (b *Builder) DoHandshake() error {
	slog.Info("Starting Handshake")

	host, err := os.Hostname()
	if err != nil {
		slog.Critical(err)
		return err
	}

	var i interface{}
	i = rmake.NewBuilderAnnouncement(host, b.ListenerAddr)
	b.enc.Encode(&i)
	slog.Info("Sent Announcement")

	inter, err := b.ReceiveFromManager()
	if err != nil {
		slog.Critical(err)
		return err
	}

	var ack *rmake.ManagerAcknowledge
	switch inter := inter.(type) {
	case *rmake.ManagerAcknowledge:
		ack = inter
	default:
		err := errors.New("Error, recieved unexpected type in handshake.")
		slog.Error(err)
		return err
	}

	if ack.Success {
		b.UUID = ack.UUID
		slog.Infof("Handshake Complete, new UUID: %d", b.UUID)
	}
	return nil
}
