package main

import (
	"flag"
	"fmt"

	"github.com/whyrusleeping/rmake/pkg/builder"
)

func main() {
	var listname string
	var manager string
	var procs int
	var showhelp bool
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

	flag.BoolVar(&showhelp, "h", false, "Show help")
	flag.Parse()

	if showhelp {
		fmt.Println("Usage: %s -[l|listen] \"address to listen on\" -[m|manager] \"address of manager\"")
		return
	}

	fmt.Println("rmakebuilder")
	if b := builder.NewBuilder(listname, manager, procs); b != nil {
		b.DoHandshake()
		// Start the builder
		b.Run()
	}
}
