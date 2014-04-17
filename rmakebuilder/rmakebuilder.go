package main

import (
	"flag"
	"encoding/gob"
	"path"
	"errors"
	"fmt"
	"os"
	"log"
	"os/exec"
	"time"
	"net"
	"github.com/whyrusleeping/rmake/types"
)

type Builder struct {
	manager net.Conn
	enc     *gob.Encoder
	dec     *gob.Decoder
	list    net.Listener

	UpdateFrequency time.Duration
	Running         bool

	mgrReconnect chan struct{}

	UUID int
	//Job Queue TODO: use this?
	JQueue chan *rmake.Job
}

func NewBuilder(listen string, manager string) *Builder {
	//fmt.Printf("Listen on: %s\nConnect to: %s\n", host, manager)
	// Setup manager connection
	mgr, err := net.Dial("tcp", manager)
	if err != nil {
		panic(err)
	}
	// Setup socket to listen to
	list, err := net.Listen("tcp", listen)
	if err != nil {
		mgr.Close()
		panic(err)
	}


	// Build new builder
	b := new(Builder)
	b.list = list
	b.manager = mgr
	b.enc = gob.NewEncoder(mgr)
	b.dec = gob.NewDecoder(mgr)
	b.UpdateFrequency = time.Second * 15
	b.mgrReconnect = make(chan struct{})
	fmt.Println("Final test")
	return b
}

func (b *Builder) RunJob(req *rmake.BuilderRequest) {
	log.Printf("Starting job for session: '%s'\n", req.Session)
	sdir := path.Join("builds", req.Session)
	for _,f := range req.Input {
		err := f.Save(sdir)
		if err != nil {
			log.Println(err)
		}
	}

	//TODO: Make sure all deps are here!
	//Wait in some way if they are not
	for _,dep := range req.BuildJob.Deps {
		depPath := path.Join(sdir, dep)
		_,err := os.Stat(depPath)
		if err != nil {
			log.Printf("Missing dependency: '%s'\n", dep)
		}
	}

	resp := new(rmake.BuildFinishedMessage)
	cmd := exec.Command(req.BuildJob.Command, req.BuildJob.Args...)
	cmd.Dir = sdir

	out,err := cmd.CombinedOutput()
	resp.Stdout = string(out)
	if err != nil {
		log.Println(err)
		resp.Error = err.Error()
		resp.Success = false
	}
	err = b.Send(resp)
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

func (b *Builder) Stop() {
	log.Println("Shutting down builder.")
	b.Running = false
	b.list.Close()
	b.manager.Close()
}

func (b *Builder) Start(nproc int) {
	log.Println("Starting builder.")
	b.Running = true

	go b.StartPublisher()

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
func (b *Builder) Send(i interface{}) error {
	err := b.enc.Encode(&i)
	if err != nil {
		return err
	}
	return nil
}

// Read a message from the manager
func (b *Builder) Recieve() (interface{}, error) {
	var i interface{}
	err := b.dec.Decode(&i)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (b *Builder) HandleConnection(con net.Conn) {
	log.Printf("Handling new connection from %s\n", con.RemoteAddr().String())
	dec := gob.NewDecoder(con)

	var i interface{}
	for {
		err := dec.Decode(&i)
		if err != nil {
			log.Println(err)
			return
		}
		switch mes := i.(type) {
		case *rmake.RequiredFileMessage:
			//Get a file from another node
			mes.Payload.Save(path.Join("builds", mes.Session))
		case *rmake.BuilderRequest:
			log.Println("Got a builder request!")
			b.RunJob(mes)
		}
	}
}

func (b *Builder) SendStatusUpdate() error {
	//TODO: get actual system information
	log.Println("Sending system load update!")
	stat := new(rmake.BuilderInfoMessage)
	stat.CPULoad = GetCpuUsage()
	stat.QueuedJobs = 0
	stat.MemUse = 0
	log.Println(stat)

	err := b.Send(stat)
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

func (b *Builder) DoHandshake() error {
	fmt.Printf("Starting Handshake\n")
	announcement := new(rmake.BuilderAnnouncement)
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	announcement.Hostname = host
	b.Send(announcement)
	fmt.Printf("Sent message\n")

	inter, err := b.Recieve()
	if err != nil {
		return err
	}
	switch inter.(type) {
	case *rmake.ManagerAcknowledge:
		b.UUID = inter.(*rmake.ManagerAcknowledge).UUID
		break
	default:
		log.Println("Recieved unknown type.")
		return errors.New("Error, recieved unexpected type in handshake.\n")
	}

	log.Println("Handshake Complete, new UUID: %s", b.UUID)
	return nil
}

func main() {
	//Listens on port 11221 by default
	var listname string
	var manager string
	var procs int
	// Arguement parsing
	flag.StringVar(&listname, "listen", ":11222",
		"The ip and or port to listen on")
	flag.StringVar(&listname, "l", ":11222",
		"The ip and or port to listen on (shorthand)")
	flag.StringVar(&manager, "manager", ":11221",
		"Address and port of manager node")
	flag.StringVar(&manager, "m", ":11221",
		"Address and port of manager node (shorthand)")
flag.IntVar(&procs, "p", 2, "Number of processors to use.")
	flag.Parse()

	fmt.Println("rmakebuilder")

	b := NewBuilder(listname, manager)
	err := b.DoHandshake()
	if err != nil {
		panic(err)
	}
	b.Start(procs)
}
