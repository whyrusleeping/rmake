package main

import (
	"testing"
	"fmt"
	"net"
	"time"
	"os"
	"github.com/whyrusleeping/rmake/types"
	"encoding/gob"
)

//Test builder sending load updates
func TestUpdates(t *testing.T) {
	mgr,err := net.Listen("tcp", ":12344")
	if err != nil {
		panic(err)
	}

	var b *Builder
	go func() {
		b = NewBuilder(":12345", "localhost:12344")
		b.UpdateFrequency = time.Millisecond
		b.Start()
	}()

	con,err := mgr.Accept()
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
		case *rmake.BuilderInfoMessage:
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
	mgr,err := net.Listen("tcp", ":12334")
	if err != nil {
		panic(err)
	}

	go func() {
		b := NewBuilder(":12335", "localhost:12334")
		b.Start()
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
		buildCon,err := net.Dial("tcp", "localhost:12335")
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

	con,err := mgr.Accept()
	if err != nil {
		panic(err)
	}

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
		case *rmake.BuilderInfoMessage:
			fmt.Println(i)
		case *rmake.BuildFinishedMessage:
			fmt.Println("Build Finished Message!")
			fmt.Println(i.Stdout)
			recv++
		case *rmake.BuilderResult:
			fmt.Println("Builder Result.")
			recv++
		default:
			fmt.Println("Unrecognized Message.")
		}
	}
	os.RemoveAll("builds")
}
