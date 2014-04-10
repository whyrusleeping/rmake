package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"net"
)

// goroutine to handle a new connection from a client.
// Determines what resources are avaliable and what
// resources the request requires.
func HandleConnection(c net.Conn) {
	var gobint interface{}

	unzip, err := gzip.NewReader(c)
	if err != nil {
		return
	}

	dec := gob.NewDecoder(unzip)
	err = dec.Decode(&gobint)
	if err != nil {
		fmt.Println(err)
		return
	}

	switch gobtype := gobint.(type) {
	case BuilderResult:
		fmt.Printf("Builder Result: %d\n", gobtype)
	case ManagerRequest:
		fmt.Printf("Manager Request: %d\n", gobtype)
	default:
		fmt.Printf("Unknown Type: %d\n", gobtype)
	}

	return
}

func init() {
	gob.Register(BuilderResult{})
	gob.Register(ManagerRequest{})
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

	fmt.Println("rmakemanager\n")

	//Start the server socket
	list, err := net.Listen("tcp", listname)
	if err != nil {
		panic(err)
	}

	//Accept and handle new client connections
	for {
		con, err := list.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		//Handle clients in separate 'thread'
		go HandleConnection(con)
	}

}
