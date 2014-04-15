package main

import (
	"flag"
	"encoding/gob"
	"path"
	"fmt"
	"os/exec"
	"time"
	"net"
	"github.com/whyrusleeping/rmake/types"
)

type Builder struct {
	manager net.Conn
	mgrEnc *gob.Encoder
	list net.Listener

	UpdateFrequency time.Duration
	Running bool

	mgrReconnect chan struct{}

	//Job Queue
	JQueue []*rmake.Job
}

func NewBuilder(host string, manager string) *Builder {
	mgr,err := net.Dial("tcp", manager)
	if err != nil {
		panic(err)
	}

	list,err := net.Listen("tcp", host)
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
	sdir := path.Join("builds", req.Session)
	for _,f := range req.Input {
		err := f.Save(sdir)
		if err != nil {
			fmt.Println(err)
		}
	}
	//TODO: Make sure all deps are here!

	resp := new(rmake.BuildFinishedMessage)
	cmd := exec.Command(req.BuildJob.Command, req.BuildJob.Args...)
	cmd.Dir = sdir

	out,err := cmd.CombinedOutput()
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
		//SEND TO MANAGER
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

	var i interface{}
	i = results
	err = outEnc.Encode(&i)
	if err != nil {
		fmt.Println("Sending of result to target failed.")
	}
	fmt.Println("Job finished!")
}

func (b *Builder) Stop() {
	fmt.Println("Shutting down builder.")
	b.Running = false
	b.list.Close()
	b.manager.Close()
}

func (b *Builder) Start() {
	b.Running = true
	go b.StartPublisher()

	for {
		con,err := b.list.Accept()
		if err != nil {
			fmt.Println(err)
			if b.Running {
				//Diagnose?
				fmt.Printf("Listener Error: %s\n", err)
				continue
			} else {
				fmt.Println("Shutting down server socket...")
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
			fmt.Println("Got a builder request!")
			b.RunJob(mes)
		}
	}
}

func (b *Builder) SendStatusUpdate() error {
	//TODO: get system information
	fmt.Println("Sending system load update!");
	stat := new(rmake.BuilderInfoMessage)
	stat.CPULoad = 0.04
	stat.QueuedJobs = 7
	stat.MemUse = 12345

	err := b.SendMsgToManager(stat)
	if err != nil {
		fmt.Println("Failed sending update!")
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
