package main

import (
	"flag"

	log "github.com/cihub/seelog"
	"github.com/whyrusleeping/rmake/pkg/manager"
)

func main() {
	//Listens on port 11221 by default
	var listname string
	// Arguement parsing
	flag.StringVar(&listname,
		"listname", ":11221", "The ip and or port to listen on")
	flag.StringVar(&listname,
		"l", ":11221", "The ip and or port to listen on (shorthand)")

	flag.Parse()

	log.Info("Running as:")
	log.Infof("rmakemanager -l %s", listname)

	manager := manager.NewManager(listname)
	manager.Start()
}
