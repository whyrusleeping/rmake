package main

import (
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"time"

	"reflect"

	"github.com/whyrusleeping/rmake/types"
	slog "github.com/cihub/seelog"
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
	waitfile map[string]chan *rmake.File
	reqfilewait chan *FileWait
	newfiles chan *rmake.RequiredFileMessage

	mgrReconnect chan struct{}

	Procs int
	UUID  int

	Halt   chan struct{}

	//Job Queue TODO: use this?
	JQueue chan *rmake.BuilderRequest
	RunningJobs chan struct{}
}

type FileWait struct {
	File string
	Session string
	Reply chan *rmake.File
}

func NewBuilder(listen string, manager string, nprocs int) *Builder {
	// Setup manager connection
	mgr, err := net.Dial("tcp", manager)
	if err != nil {
		log.Println(err)
		log.Println("Could not connect to manager.")
		return nil
	}

	// Setup socket to listen to
	list, err := net.Listen("tcp", listen)
	if err != nil {
		mgr.Close()
		log.Panic(err)
	}

	//Make sure build directory exists
	os.Mkdir("builds", 0777 | os.ModeDir)

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

	b.JQueue = make(chan *rmake.BuilderRequest)
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
					slog.Error("Recieved nil file!")
					continue
				}
				wpath := path.Join("builds", fi.Session, fi.Payload.Path)
				ch, ok := b.waitfile[wpath]
				if !ok {
					slog.Warnf("Recieved file nobody was asking for, session: '%s', path: '%s'",
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
		select {
			case work := <-b.JQueue:
				b.RunningJobs <- struct{}{}
				b.RunJob(work)
				<-b.RunningJobs
		}
	}
}

//
func (b *Builder) RunJob(req *rmake.BuilderRequest) {
	slog.Infof("Starting job for session: '%s'\n", req.Session)
	sdir := path.Join("builds", req.Session)
	os.Mkdir(sdir, 0777 | os.ModeDir)

	for _, f := range req.Input {
		err := f.Save(sdir)
		if err != nil {
			log.Println(err)
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

	for _,ch := range waitlist {
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
		//TODO: dont hardcode port here!!!
		send, err := net.Dial("tcp", req.ResultAddress + ":11222")
		if err != nil {
			slog.Error(err)
			//TODO: decide what to do if this happens
			slog.Error("ERROR: this is pretty bad... what do?")
		}
		outEnc = gob.NewEncoder(send)
	}

	fipath := path.Join("builds", req.Session)
	slog.Info("Loading %s to send on.\n", req.BuildJob.Output)
	fi,err := rmake.LoadFile(fipath, req.BuildJob.Output)
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
		mes, err := b.RecieveFromManager()
		if err != nil {
			slog.Error(err)
			//TODO: switch on the error type and handle appropriately
			panic(err)
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
		b.HandleMessage(m)
	}
}

//Listen for and handle new connections
func (b *Builder) SocketListener() {
	for {
		con, err := b.list.Accept()
		if err != nil {
			log.Println(err)
			if b.Running {
				//Diagnose?
				log.Printf("Listener Error: %s\n", err)
				continue
			} else {
				log.Println("Shutting down server socket...")
				return
			}
		}
		go b.HandleConnection(con)
	}
}

// Send a message to the manager
func (b *Builder) SendToManager(i interface{}) {
	log.Printf("Send to manager '%s'\n", reflect.TypeOf(i))
	b.outgoing <- i
}

// Read a message from the manager
func (b *Builder) RecieveFromManager() (interface{}, error) {
	var i interface{}
	err := b.dec.Decode(&i)
	fmt.Println("Recieve from manager.")
	if err != nil {
		return nil, err
	}
	return i, nil
}

// Handles all messages except handshake message types
func (b *Builder) HandleMessage(i interface{}) {
	switch message := i.(type) {
	case *rmake.RequiredFileMessage:
		log.Println("Recieved required file.")
		//Get a file from another node
		b.newfiles <- message

	case *rmake.BuilderRequest:
		slog.Info("Recieved builder request.")
		b.JQueue <- message

	case *rmake.BuilderResult:
		slog.Info("Recieved builder result.")
		sdir := path.Join("builds", message.Session)
		for _,f := range message.Results {
			err := f.Save(sdir)
			if err != nil {
				slog.Error("Error saving file!")
				slog.Error(err)
			}
		}

	default:
		slog.Warnf("Recieved invalid message type. '%s'", reflect.TypeOf(message))
	}
}

// Handles a connection from the listener
// This could come from another builder.
// Possibly from the manager too, but most likely another builder.
func (b *Builder) HandleConnection(con net.Conn) {
	log.Printf("Handling new connection from %s\n", con.RemoteAddr().String())
	dec := gob.NewDecoder(con)
	var i interface{}
	err := dec.Decode(&i)
	if err != nil {
		log.Println(err)
		return
	}
	//b.HandleMessage(i)
	// We'll only handle one message ber connection
	b.incoming <- i
	con.Close()
}

func (b *Builder) SendStatusUpdate() {
	//TODO: get actual system information
	log.Println("Sending system load update!")
	stat := new(rmake.BuilderStatusUpdate)
	stat.CPULoad = GetCpuUsage()
	stat.QueuedJobs = len(b.JQueue)
	stat.MemUse = 0
	log.Println(stat)

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

func (b *Builder) DoHandshake() {
	log.Printf("Starting Handshake\n")

	host, err := os.Hostname()
	if err != nil {
		log.Panic(err)
	}

	var i interface{}
	i = rmake.NewBuilderAnnouncement(host, b.ListenerAddr)
	b.enc.Encode(&i)
	log.Printf("Sent Announcement\n")

	inter, err := b.RecieveFromManager()
	if err != nil {
		log.Panic(err)
	}

	var ack *rmake.ManagerAcknowledge
	switch inter := inter.(type) {
	case *rmake.ManagerAcknowledge:
		ack = inter
	default:
		log.Panic(errors.New("Error, recieved unexpected type in handshake.\n"))
	}

	if ack.Success {
		b.UUID = ack.UUID
		log.Printf("Handshake Complete, new UUID: %d\n", b.UUID)
	}
}

func main() {
	var listname string
	var manager string
	var procs int
	// Arguement parsing
	// Listen on ip and port
	flag.StringVar(&listname, "listen", ":11222",
		"The ip and or port to listen on")
	flag.StringVar(&listname, "l", ":11222",
		"The ip and or port to listen on (shorthand)")
	// Manager ip and port
	flag.StringVar(&manager, "manager", ":11221",
		"Address and port of manager node")
	flag.StringVar(&manager, "m", ":11221",
		"Address and port of manager node (shorthand)")
	// Avaliable processors
	flag.IntVar(&procs, "p", 2, "Number of processors to use.")
	flag.Parse()
	flag.PrintDefaults()

	fmt.Println("rmakebuilder")
	if b := NewBuilder(listname, manager, procs); b != nil {
		b.DoHandshake()
		// Start the builder
		b.Run()
	}
}
