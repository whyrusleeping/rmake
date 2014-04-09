package main

import (
	"flag"
	"fmt"
)

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
