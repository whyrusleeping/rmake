package main

import (
	"flag"
	"encoding/gob"
	"fmt"
	"time"
	"net"
	"github.com/whyrusleeping/rmake/types"
)

type Builder struct {
	manager net.Conn
	mgrEnc *gob.Encoder
	list net.Listener
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

	return b
}

func (b *Builder) Start() {
	go b.StartPublisher()

	for {
		con,err := b.list.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go b.HandleConnection(con)
	}
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
			mes.Payload.Save()
		}
	}
}

func (b *Builder) SendStatusUpdate() {
	stat := new(rmake.BuilderInfoMessage)
	stat.CPULoad = 0.04
	stat.QueuedJobs = 7

	err := b.mgrEnc.Encode(stat)
	if err != nil {
		fmt.Println(err)
		//TODO: think about fixing the connection?
	}
}

func (b *Builder) StartPublisher() {
	tick := time.NewTicker(time.Second * 15)
	for {
		select {
		case <-tick.C:
			b.SendStatusUpdate()
		}
	}
}

// main
func main() {
	//Listens on port 11221 by default
	var listname string
	// Arguement parsing
	flag.StringVar(&listname, "listname", ":11221", "The ip and or port to listen on")
	flag.StringVar(&listname, "l", ":11221", "The ip and or port to listen on (shorthand)")

	flag.Parse()

	fmt.Println("rmakebuilder\n")

}
