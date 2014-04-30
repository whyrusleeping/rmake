package main

import (
	"os"
	"encoding/json"
	"strings"
	"fmt"

	"github.com/whyrusleeping/rmake/pkg/client"
	"github.com/whyrusleeping/rmake/pkg/types"
)

func createJobs(rmc *client.RMakeConf, args []string) {
	switch args[2] {
	case "add":
		j := new(rmake.Job)
		spl := strings.Split(args[3], " ")
		j.Command = spl[0]
		j.Args = spl[1:]
		j.Output = args[4]
		j.Deps = args[5:]
		for _,dep := range j.Deps {
			found := false
			for _,fi := range rmc.Files {
				if dep == fi.Path {
					found = true
				}
			}
			if !found {
				rmc.AddFile(dep)
			}
		}
		rmc.Jobs = append(rmc.Jobs, j)
	case "help":
		switch args[3] {
			case "add":
				fmt.Println("usage: rmake job add command output deps...")
				fmt.Println("example: rmake job add \"gcc main.c -c -O3\" main.o main.c mylib.h")
				fmt.Println("The above command adds a job that compiles main.c, whose output")
				fmt.Println("is main.o and that depends on main.c and mylib.h")
		}
	}
}

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
			fmt.Printf("Adding: '%s'\n", v)
			rmc.AddFile(v)
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
	case "job":
		createJobs(rmc, os.Args)
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
