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

	"github.com/whyrusleeping/rmake/types"
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

	mgrReconnect chan struct{}

	Procs int
	UUID  int
	//Job Queue TODO: use this?
	Halt   chan struct{}
	JQueue chan *rmake.Job
}

func NewBuilder(listen string, manager string, nprocs int) *Builder {
	// Setup manager connection
	mgr, err := net.Dial("tcp", manager)
	if err != nil {
		log.Panic(err)
	}
	// Setup socket to listen to
	list, err := net.Listen("tcp", listen)
	if err != nil {
		mgr.Close()
		log.Panic(err)
	}

	// Build new builder
	b := new(Builder)
	b.list = list
	b.Procs = nprocs
	b.ListenerAddr = listen
	b.ManagerAddr = manager
	b.manager = mgr
	b.enc = gob.NewEncoder(mgr)
	b.dec = gob.NewDecoder(mgr)
	b.UpdateFrequency = time.Second * 15
	b.Halt = make(chan struct{})
	b.mgrReconnect = make(chan struct{})
	return b
}

func (b *Builder) RunJob(req *rmake.BuilderRequest) {
	log.Printf("Starting job for session: '%s'\n", req.Session)
	sdir := path.Join("builds", req.Session)
	for _, f := range req.Input {
		err := f.Save(sdir)
		if err != nil {
			log.Println(err)
		}
	}
	//TODO: Make sure all deps are here!
	//Wait in some way if they are not
	for _, dep := range req.BuildJob.Deps {
		depPath := path.Join(sdir, dep)
		_, err := os.Stat(depPath)
		if err != nil {
			log.Printf("Missing dependency: '%s'\n", dep)
		}
	}

	resp := new(rmake.BuildFinishedMessage)
	cmd := exec.Command(req.BuildJob.Command, req.BuildJob.Args...)
	cmd.Dir = sdir

	out, err := cmd.CombinedOutput()
	resp.Stdout = string(out)
	if err != nil {
		log.Println(err)
		resp.Error = err.Error()
		resp.Success = false
	}
	err = b.SendToManager(resp)
	if err != nil {
		log.Println(err)
	}

	if req.ResultAddress == "" {
		log.Println("Im the final node! no need to send.")
		return
	}

	var outEnc *gob.Encoder
	if req.ResultAddress == "manager" {
		//Send to manager
		outEnc = b.enc
	} else {
		//Send to other builder
		send, err := net.Dial("tcp", req.ResultAddress)
		if err != nil {
			log.Println(err)
			//TODO: decide what to do if this happens
			log.Println("ERROR: this is pretty bad... what do?")
		}
		outEnc = gob.NewEncoder(send)
	}

	fipath := path.Join("builds", req.Session, req.BuildJob.Output)
	log.Printf("Loading %s to send on.\n", fipath)
	fi := rmake.LoadFile(fipath)
	if fi == nil {
		log.Println("Failed to load output file!")
	}

	results := new(rmake.BuilderResult)
	results.Results = append(results.Results, fi)
	results.Session = req.Session

	i := interface{}(results)
	err = outEnc.Encode(&i)
	if err != nil {
		log.Println("Sending of result to target failed.")
	}
	log.Println("Job finished!")
	log.Printf("Job for session '%s' finished.\n", req.Session)
}

func (b *Builder) Run() {
	log.Println("Starting builder.")
	b.Running = true
	// Start Listeners
	go b.ManagerListener()
	go b.SocketListener()
	// Start Heartbeat
	go b.StartPublisher()

	<-b.Halt

	log.Println("Shutting down builder.")
	b.Running = false
	b.list.Close()
	b.manager.Close()
}

func (b *Builder) Stop() {
	b.Halt <- struct{}{}
}

func (b *Builder) ManagerListener() {
	for {
		err, i := b.RecieveFromManager()
		if err != nil {
			log.Println(err)
		} else {
			switch i.(type) {

			}
		}
	}
}

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
func (b *Builder) SendToManager(i interface{}) error {
	err := b.enc.Encode(&i)
	if err != nil {
		return err
	}
	return nil
}

// Read a message from the manager
func (b *Builder) RecieveFromManager() (interface{}, error) {
	var i interface{}
	err := b.dec.Decode(&i)
	if err != nil {
		return nil, err
	}
	return i, nil
}

// Handles all, but handshake message types
func (b *Builder) HandleMessage(i interface{}) {
	switch message := i.(type) {
	// Wat?
	case *rmake.RequiredFileMessage:
		log.Println("Recieved required file.")
		//Get a file from another node
		message.Payload.Save(path.Join("builds", message.Session))

	// Job Request
	case *rmake.BuilderRequest:
		log.Println("Recieved builder request.")
		b.RunJob(message)

	default:
		log.Println("Recieved invalid message type.")
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
	b.HandleMessage(i)
	// We'll only handle one message ber connection
	con.Close()
}

func (b *Builder) SendStatusUpdate() error {
	//TODO: get actual system information
	log.Println("Sending system load update!")
	stat := new(rmake.BuilderStatusUpdate)
	stat.CPULoad = GetCpuUsage()
	stat.QueuedJobs = 0
	stat.MemUse = 0
	log.Println(stat)

	err := b.SendToManager(stat)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (b *Builder) StartPublisher() {
	tick := time.NewTicker(b.UpdateFrequency)
	for {
		select {
		case <-tick.C:
			err := b.SendStatusUpdate()
			if err != nil {
				return
			}
		}
	}
}

func (b *Builder) DoHandshake() {
	log.Printf("Starting Handshake\n")

	host, err := os.Hostname()
	if err != nil {
		log.Panic(err)
	}

	announcement := rmake.NewBuilderAnnouncement(host, b.ListenerAddr)
	b.SendToManager(announcement)
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

	fmt.Println("rmakebuilder")
	b := NewBuilder(listname, manager, procs)
	// Handshake with the manager
	b.DoHandshake()
	// Start the builder
	b.Run()
}
