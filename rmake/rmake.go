package main

import (
	"os"
	"encoding/json"
	"time"
	"fmt"

	"github.com/whyrusleeping/rmake/pkg/client"
	"github.com/whyrusleeping/rmake/pkg/types"
)

func main() {
	//Try and load default configuration
	rmc, err := client.LoadRMakeConf("rmake.json")
	if err != nil {
		switch err.(type) {
		case *os.PathError:
			rmc = client.NewRMakeConf()
		case *json.SyntaxError:
			fmt.Println("Invalid Syntax:")
			fmt.Println(err)
			return
		}
	}

	//If no args, perform a build
	//eg, user ran "rmake"
	if len(os.Args) == 1 {
		err := rmc.DoBuild()
		if err != nil {
			fmt.Println("Do build errored!")
			fmt.Println(err)
		}
		rmc.Save("rmake.json")
		return
	}

	//Parse command line arguments
	switch os.Args[1] {
	case "add":
		for _, v := range os.Args[2:] {
			fi := new(rmake.FileInfo)
			fi.Path = v
			fi.LastTime = time.Now().AddDate(-20, 0, 0)
			fmt.Printf("Adding: '%s'\n", v)
			rmc.Files = append(rmc.Files, fi)
		}
	case "server":
		rmc.Server = os.Args[2]
	case "bin":
		rmc.Output = os.Args[2]
	case "clean":
		rmc.Clean()
	case "var":
		rmc.Vars[os.Args[2]] = os.Args[3]
	case "check":
		tr,err := rmc.MakeDepTree()
		tr.Print()
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("All is well!")
		}
	case "compress":
		if len(os.Args) == 2 {
			printHelpCompress()
		} else {
			rmc.Compression = os.Args[2]
		}
	case "status":
		rmc.Status()
	case "help":
		if len(os.Args) == 2 {
			printHelp("all")
		} else {
			printHelp(os.Args[2])
		}
	}
	rmc.Save("rmake.json")
}
