package main

import (
	"compress/gzip"
	"encoding/gob"
	"flag"
	"fmt"
	"net"
)

// The types of packages
const (
	RequestPackage = iota
	BuilderConnection
	BuilderDisconnection
)

// The Package type
// At the base level it only supplies a type.
type Package struct {
	Type int
}

// Takes a package and decodes it so we can get it's subtype.
func ReadPackage(c net.Conn) (*Package, error) {
	pack := new(Package)
	unzip, err := gzip.NewReader(c)
	if err != nil {
		return nil, err
	}
	dec := gob.NewDecoder(unzip)
	err = dec.Decode(pack)
	if err != nil {
		return nil, err
	}
	return pack, nil
}

// goroutine to handle a new connection from a client.
// Determines what resources are avaliable and what
// resources the request requires.
func HandleConnection(c net.Conn) {

	pack, err := ReadPackage(c)
	if err != nil {
		fmt.Println(err)
		c.Close()
		return
	}

	if pack.Type == RequestPackage {
		fmt.Println("Request Package!\n")
	}
	return
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
