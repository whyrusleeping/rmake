package main

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"reflect"

	"github.com/dustin/go-humanize"
	"github.com/whyrusleeping/rmake/types"
)

func NewManagerRequest(conf *RMakeConf) *rmake.ManagerRequest {
	p := new(rmake.ManagerRequest)
	p.Jobs = conf.Jobs
	p.Arch = "Arch" //lol
	p.OS = "Arch (the OS)"
	p.Output = conf.Output

	p.Files = make(map[string]*rmake.File)
	for _, v := range conf.Files {
		f,err := rmake.LoadFile("", v.Path)
		if err != nil {
			fmt.Println(err)
			continue
		}
		p.Files[v.Path] = f
	}
	return p
}

//The in memory representation of the configuration file
type RMakeConf struct {
	Server      string
	Files       []*rmake.FileInfo `json:",omitempty"`
	Jobs        []*rmake.Job      `json:",omitempty"`
	Output      string
	Session     string
	Vars        map[string]string
	Verbose		bool
	Compression string
	ignore      []string `json:",omitempty"`
}

//Check to make sure that all the files required by all the jobs are added
func (rmc *RMakeConf) Validate() error {
	for _, j := range rmc.Jobs {
		for _, dep := range j.Deps {
			found := false
			for _, f := range rmc.Files {
				if f.Path == dep {
					found = true
					break
				}
			}
			if !found {
				return errors.New(fmt.Sprintf("'%s' not found!", dep))
			}
		}
	}
	return nil
}

//Create a new empty configuration
func NewRMakeConf() *RMakeConf {
	rmc := new(RMakeConf)
	rmc.Vars = make(map[string]string)
	return rmc
}

//Load list of files to ignore
//Not really used yet
func (rmc *RMakeConf) LoadIgnores(igfile string) {
	fi, err := os.Open(igfile)
	if err != nil {
		return
	}
	scan := bufio.NewScanner(fi)
	for scan.Scan() {
		rmc.ignore = append(rmc.ignore, scan.Text())
	}
}

//Reset all file modtimes to be in the past
//and reset the session ID
func (rmc *RMakeConf) Clean() {
	for _, v := range rmc.Files {
		v.LastTime = time.Now().AddDate(-20, 0, 0)
	}
	rmc.Session = ""
}

//TODO: as part of the 'rmakeignore' file, check whether or not
//a given file is ignored
func (rmc *RMakeConf) IsIgnored(fi string) bool {
	return false
}

//Print a pretty status message
func (rmc *RMakeConf) Status() error {
	fmt.Println("\x1b[0m# Current working tree status\x1b[0m")
	fmt.Println("\x1b[0m#   (use \"rmake remove <file>...\" to no longer track the file)\x1b[0m")
	fmt.Println("#")
	fmt.Println("\x1b[0m# Modified files to be updated\x1b[0m")
	fmt.Println("#")

	for _, v := range rmc.Files {
		inf, err := os.Stat(v.Path)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if inf.ModTime().After(v.LastTime) {
			fmt.Printf("#       \x1b[0;31m%-20s\x1b[1;30m %s\x1b[0m\n", v.Path, humanize.Time(inf.ModTime()))
		}
	}

	fmt.Println("#")
	fmt.Println("\x1b[0m# Non modified files\x1b[0m")
	fmt.Println("#")

	for _, v := range rmc.Files {
		inf, err := os.Stat(v.Path)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		if !inf.ModTime().After(v.LastTime) {
			fmt.Printf("#       \x1b[0;32m%s\x1b[0m\n", v.Path)
		}
	}

	fmt.Println("#")
	fmt.Println("\x1b[0m# Untracked files:\x1b[0m")
	fmt.Println("\x1b[0m#   (use \"rmake add <file>...\" to include in what will be transfered)\x1b[0m")
	fmt.Println("#")
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if path[0] == '.' {
			return nil
		}

		for _, v := range rmc.Files {
			if path == v.Path {
				return nil
			}
		}

		fmt.Printf("#       %s\n", path)
		return nil
	})

	return nil
}

// Prints a build status
func PrintBuildStatus(status *rmake.BuildStatus) {
	fmt.Printf("%s\n", status.Message)
	fmt.Printf("Percent Complete: %d\n", status.PercentComplete)
}

// Processes feed back as it comes in
// Waits for final build result
func AwaitResult(c net.Conn) (*rmake.FinalBuildResult, error) {
	var gobint interface{}
	var fbr *rmake.FinalBuildResult

	dec := gob.NewDecoder(c)

	// Wait till we have what we want
	for fbr == nil {
		// Decode some data
		err := dec.Decode(&gobint)
		if err != nil {
			fmt.Println(err)
			//return nil, err // I don't think we want to simply die...
			panic(err)
		}

		// Found some data, grab the type...
		switch message := gobint.(type) {
		case *rmake.BuildStatus:
			fmt.Println("Build Status")
			PrintBuildStatus(message) // Doesn't work for some reason
		case *rmake.FinalBuildResult:
			fmt.Println("Final Build Result")
			fbr = message
		case *rmake.BuilderResult:
			fmt.Println("Got builder result.")
			fmt.Printf("Got %d files back.", len(message.Results))
		case *rmake.JobFinishedMessage:
			fmt.Println("I SHOULDNT BE GETTING THIS.")
			fmt.Println(message.Stdout)
		default:
			fmt.Println("Unknown Type.")
			fmt.Println(reflect.TypeOf(message))
		}
	}

	return fbr, nil
}

//Perform a build as specified by the rmake config file
func (rmc *RMakeConf) DoBuild() error {
	start := time.Now()
	//Create a package
	var inter interface{}
	inter = NewManagerRequest(rmc)

	con, err := net.Dial("tcp", rmc.Server)
	if err != nil {
		return err
	}
	defer con.Close()
	enc := gob.NewEncoder(con)
	err = enc.Encode(&inter)
	if err != nil {
		return err
	}

	// Wait for the result
	var fbr *rmake.FinalBuildResult
	fbr, err = AwaitResult(con)
	if err != nil {
		return err
	}

	// What do we want to do with the FinalBuildResult?
	if fbr.Success {
		fmt.Printf("Success!\n")
		for _,f := range fbr.Results {
			err := f.Save("")
			if err != nil {
				fmt.Println(err)
			}
		}
	} else {
		fmt.Printf("Error!\n")
	}

	took := time.Now().Sub(start)
	if rmc.Verbose {
		fmt.Printf("Build took %s\n", took.String())
	}
	return nil
}

/*
//Wrap the given writer in a gzip layer
//level of compression specified in config
func (rmc *RMakeConf) Gzipper(w io.Writer) *gzip.Writer {
	complev := gzip.DefaultCompression
	switch rmc.Compression {
	case "best":
		complev = gzip.BestCompression
	case "none":
		complev = gzip.NoCompression
	case "speed":
		complev = gzip.BestSpeed
	}
	zipper, err := gzip.NewWriterLevel(w, complev)
	if err != nil {
		panic(err)
	}
	return zipper
}
*/

func LoadRMakeConf(file string) (*RMakeConf, error) {
	fi, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	rmc := new(RMakeConf)
	dec := json.NewDecoder(fi)
	err = dec.Decode(rmc)
	if err != nil {
		return nil, err
	}
	rmc.LoadIgnores(".rmakeignore")
	return rmc, nil
}

func (rmc *RMakeConf) Save(file string) error {
	fi, err := os.Create(file)
	if err != nil {
		return err
	}
	defer fi.Close()
	out, _ := json.MarshalIndent(rmc, "", "\t")
	_, err = fi.Write(out)
	return err
}

func main() {
	//Try and load default configuration
	rmc, err := LoadRMakeConf("rmake.json")
	if err != nil {
		switch err.(type) {
			case *os.PathError:
				rmc = NewRMakeConf()
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
		err := rmc.Validate()
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
