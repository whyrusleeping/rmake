package main

import (
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
	"github.com/whyrusleeping/rmake/pkg/types"
)

//Test builder sending load updates
func TestUpdates(t *testing.T) {
	mgr, err := net.Listen("tcp", ":12344")
	if err != nil {
		panic(err)
	}

	var b *Builder
	go func() {
		b = NewBuilder(":12345", "localhost:12344", 2)
		b.UpdateFrequency = time.Millisecond
		b.Run()
	}()

	con, err := mgr.Accept()
	if err != nil {
		panic(err)
	}

	var i interface{}
	dec := gob.NewDecoder(con)
	recv := 0
	for {
		err := dec.Decode(&i)
		if err != nil {
			panic(err)
		}
		switch i := i.(type) {
		case *rmake.BuilderStatusUpdate:
			/*
				fmt.Println("Got update message!")
				fmt.Printf("Queued Jobs: %d\n", i.QueuedJobs)
				fmt.Printf("CPU load: %f\n", i.CPULoad)
				fmt.Printf("Mem Usage: %f\n", i.MemUse)
			*/
			fmt.Println(i)
			recv++
		default:
			fmt.Println("Unrecognized Message.")
		}
		if recv > 4 {
			mgr.Close()
			con.Close()
			b.Stop()
			return
		}
	}
}

func TestBuild(t *testing.T) {
	fmt.Println("\nStarting build test.\n")
	mgr, err := net.Listen("tcp", ":12334")
	if err != nil {
		panic(err)
	}

	go func() {
		//Setup builder
		b := NewBuilder(":12335", "localhost:12334", 2)
		b.Run()
	}()

	go func() {
		j := new(rmake.BuilderRequest)
		infi := new(rmake.File)
		infi.Contents = []byte("#include <stdio.h>\nint main() {printf(\"Hello\");}")
		infi.Mode = 0666
		infi.Path = "main.c"
		j.Input = append(j.Input, infi)
		j.ResultAddress = "manager"
		j.Session = "TESTSESSION"
		j.BuildJob = new(rmake.Job)
		j.BuildJob.Command = "gcc"
		j.BuildJob.Args = []string{"main.c", "-Wall"}
		j.BuildJob.Output = "a.out"
		j.BuildJob.Deps = []string{"main.c"}
		time.Sleep(time.Second)
		buildCon, err := net.Dial("tcp", "localhost:12335")
		if err != nil {
			panic(err)
		}
		enc := gob.NewEncoder(buildCon)
		var i interface{}
		i = j
		err = enc.Encode(&i)
		if err != nil {
			panic(err)
		}
	}()

	con, err := mgr.Accept()
	if err != nil {
		panic(err)
	}

	fmt.Println("Got builder connection!")

	var i interface{}
	dec := gob.NewDecoder(con)
	recv := 0
	for {
		if recv >= 2 {
			break
		}
		err := dec.Decode(&i)
		if err != nil {
			panic(err)
		}
		switch i := i.(type) {
		case *rmake.BuilderStatusUpdate:
			fmt.Println(i)
		//case *rmake.BuildFinishedMessage:
		//fmt.Println("Build Finished Message!")
		//fmt.Println(i.Stdout)
		//recv++
		case *rmake.BuilderResult:
			fmt.Println("Builder Result.")
			recv++
		default:
			fmt.Println("Unrecognized Message.")
		}
	}
	os.RemoveAll("builds")
}

func TestHandshake(t *testing.T) {
	fmt.Println("\nStarting build test.\n")
	mgr, err := net.Listen("tcp", ":12340")
	if err != nil {
		panic(err)
	}

	success := make(chan struct{})
	go func() {
		b := NewBuilder(":12341", "localhost:12340", 2)
		b.DoHandshake()
		if err != nil {
			panic(err)
		}
		success <- struct{}{}
		b.Stop()
	}()

	con, err := mgr.Accept()
	if err != nil {
		panic(err)
	}

	enc := gob.NewEncoder(con)

	var i interface{}
	dec := gob.NewDecoder(con)
	recv := 0
	for {
		if recv >= 2 {
			break
		}
		err := dec.Decode(&i)
		if err != nil {
			panic(err)
		}
		switch i := i.(type) {
		case *rmake.BuilderStatusUpdate:
			fmt.Println(i)
		//case *rmake.BuildFinishedMessage:
		//	fmt.Println("Build Finished Message!")
		//	fmt.Println(i.Stdout)
		//	recv++
		case *rmake.BuilderResult:
			fmt.Println("Builder Result.")
			recv++
		case *rmake.BuilderAnnouncement:
			fmt.Printf("Recieved Announcement from %s\n", i.Hostname)
			ack := new(rmake.ManagerAcknowledge)
			ack.UUID = 6
			ack.Success = true
			var inter interface{}
			inter = ack
			err := enc.Encode(&inter)
			if err != nil {
				panic(err)
			}
			<-success
			os.RemoveAll("builds")
			return
		default:
			fmt.Println("Unrecognized Message.")
		}
	}
}
