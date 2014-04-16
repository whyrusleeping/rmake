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
	mgrEnc  *gob.Encoder
	list    net.Listener

	UpdateFrequency time.Duration
	Running         bool

	mgrReconnect chan struct{}

	UUID string
	//Job Queue TODO: use this?
	JQueue []*rmake.Job
}

func NewBuilder(host string, manager string) *Builder {
	mgr, err := net.Dial("tcp", manager)
	if err != nil {
		panic(err)
	}

	list, err := net.Listen("tcp", host)
	if err != nil {
		mgr.Close()
		panic(err)
	}

	b := new(Builder)
	b.list = list
	b.manager = mgr
	b.mgrEnc = gob.NewEncoder(mgr)
	b.UpdateFrequency = time.Second * 15
	b.mgrReconnect = make(chan struct{})

	return b
}

func (b *Builder) RunJob(req *rmake.BuilderRequest) {
	log.Printf("Starting job for session: '%s'\n", req.Session)
	sdir := path.Join("builds", req.Session)
	for _, f := range req.Input {
		err := f.Save(sdir)
		if err != nil {
			fmt.Println(err)
		}
	}
	//TODO: Make sure all deps are here!
	//Wait in some way if they are not
	for _, dep := range req.BuildJob.Deps {
		depPath := path.Join(sdir, dep)
		_, err := os.Stat(depPath)
		if err != nil {
			fmt.Printf("Missing dependency: '%s'\n", dep)
		}
	}

	resp := new(rmake.BuildFinishedMessage)
	cmd := exec.Command(req.BuildJob.Command, req.BuildJob.Args...)
	cmd.Dir = sdir

	out, err := cmd.CombinedOutput()
	resp.Stdout = string(out)
	if err != nil {
		fmt.Println(err)
		resp.Error = err.Error()
		resp.Success = false
	}
	err = b.SendMsgToManager(resp)
	if err != nil {
		fmt.Println(err)
	}

	if req.ResultAddress == "" {
		fmt.Println("Im the final node! no need to send.")
		return
	}

	var outEnc *gob.Encoder
	if req.ResultAddress == "manager" {
		//Send to manager
		outEnc = b.mgrEnc
	} else {
		//Send to other builder
		send, err := net.Dial("tcp", req.ResultAddress)
		if err != nil {
			fmt.Println(err)
			//TODO: decide what to do if this happens
			fmt.Println("ERROR: this is pretty bad... what do?")
		}
		outEnc = gob.NewEncoder(send)
	}

	fipath := path.Join("builds", req.Session, req.BuildJob.Output)
	fmt.Printf("Loading %s to send on.\n", fipath)
	fi := rmake.LoadFile(fipath)
	if fi == nil {
		fmt.Println("Failed to load output file!")
	}

	results := new(rmake.BuilderResult)
	results.Results = append(results.Results, fi)
	results.Session = req.Session

	i := interface{}(results)
	err = outEnc.Encode(&i)
	if err != nil {
		fmt.Println("Sending of result to target failed.")
	}
	fmt.Println("Job finished!")
	log.Printf("Job for session '%s' finished.\n", req.Session)
}

func (b *Builder) Stop() {
	log.Println("Shutting down builder.")
	b.Running = false
	b.list.Close()
	b.manager.Close()
}

func (b *Builder) Start() {
	log.Println("Starting builder.")
	b.Running = true
	err := b.DoHandshake()
	if err != nil {
		fmt.Println(err)
		return
	}

	go b.StartPublisher()

	for {
		con, err := b.list.Accept()
		if err != nil {
			fmt.Println(err)
			if b.Running {
				//Diagnose?
				fmt.Printf("Listener Error: %s\n", err)
				continue
			} else {
				log.Println("Shutting down server socket...")
				return
			}
		}
		go b.HandleConnection(con)
	}
}

func (b *Builder) SendMsgToManager(i interface{}) error {
	return b.mgrEnc.Encode(&i)
}

func (b *Builder) HandleConnection(con net.Conn) {
	log.Printf("Handling new connection from %s\n", con.RemoteAddr().String())
	dec := gob.NewDecoder(con)

	var i interface{}
	for {
		err := dec.Decode(&i)
		if err != nil {
			fmt.Println(err)
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

	err := b.SendMsgToManager(stat)
	if err != nil {
		fmt.Println(err)
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
	announcement := new(rmake.BuilderAnnouncement)
	host, err := os.Hostname()
	if err != nil {
		return err
	}
	announcement.Hostname = host
	b.SendMsgToManager(announcement)

	con, err := b.list.Accept()
	if err != nil {
		return err
	}

	log.Printf("Handshake from %s\n", con.RemoteAddr().String())
	dec := gob.NewDecoder(con)

	var i interface{}
	err = dec.Decode(&i)
	if err != nil {
		return err
	}
	switch i.(type) {
	case *rmake.ManagerAcknowledge:
		b.UUID = i.(*rmake.ManagerAcknowledge).UUID
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
	// Arguement parsing
	flag.StringVar(&listname, "listname", ":11221",
		"The ip and or port to listen on")
	flag.StringVar(&listname, "l", ":11221",
		"The ip and or port to listen on (shorthand)")
	flag.StringVar(&manager, "m", "", "Address and port of manager node")
	flag.Parse()

	fmt.Println("rmakebuilder\n")

	b := NewBuilder(listname, manager)
	b.Start()
}
