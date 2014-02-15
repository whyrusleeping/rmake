package main

import (
	"os"
	"fmt"
	"net"
	"time"
	"strings"
	"io/ioutil"
	"encoding/gob"
	"encoding/json"
	"compress/gzip"
)

type Response struct {
	Stdout string
	Binary *File
	Success bool
}

type File struct {
	Path string
	Contents []byte
	Mode os.FileMode
}

func (cf *FileInfo) LoadFile() *File {
	inf,err := os.Stat(cf.Path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	if !inf.ModTime().After(cf.LastTime) {
		fmt.Printf("Skipping: '%s'\n", cf.Path)
		return nil
	}
	f := new(File)
	f.Path = cf.Path
	f.Mode = inf.Mode()
	cf.LastTime = time.Now()
	cnts,err := ioutil.ReadFile(cf.Path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	f.Contents = cnts
	fmt.Printf("file was %d bytes.\n", len(cnts))
	return f
}

func (f *File) Save() error {
	cur := "."
	spl := strings.Split(f.Path,"/")
	for _,v := range spl[:len(spl)-1] {
		cur += "/" + v
		os.Mkdir(cur, os.ModeDir | 0777)
	}
	fi,err := os.OpenFile(f.Path, os.O_CREATE, f.Mode)
	if err != nil {
		return err
	}
	fi.Write(f.Contents)
	return nil
}

type Package struct {
	Files []*File
	Command string
	Args []string
	Output string
	Session string
	Vars map[string]string
}

func NewPackage(conf *RMakeConf) *Package {
	p := new(Package)
	p.Output = conf.Output
	p.Command = conf.Command
	p.Args = conf.Args
	for _,v := range conf.Files {
		f := v.LoadFile()
		if f != nil {
			p.Files = append(p.Files, f)
		}
	}
	return p
}

type FileInfo struct {
	Path string
	LastTime time.Time
}

type RMakeConf struct {
	Server string
	Files []*FileInfo
	Command string
	Args []string
	Output string
	Session string
	Vars map[string]string
}

func NewRMakeConf() *RMakeConf {
	rmc := new(RMakeConf)
	rmc.Vars = make(map[string]string)
	return rmc
}

func (rmc *RMakeConf) DoBuild() error {
	pack := NewPackage(rmc)
	con,err := net.Dial("tcp", rmc.Server)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer con.Close()
	zipp := gzip.NewWriter(con)
	enc := gob.NewEncoder(zipp)
	err = enc.Encode(pack)
	//Make sure all data gets flushed through
	zipp.Close()
	if err != nil {
		fmt.Println(err)
		return err
	}

	resp := new(Response)
	unzip,err := gzip.NewReader(con)
	if err != nil {
		fmt.Println(err)
		return err
	}
	dec := gob.NewDecoder(unzip)
	err = dec.Decode(resp)
	if err != nil {
		fmt.Println(err)
		return err
	}

	if !resp.Success {
		fmt.Println("Build failed.")
		fmt.Println(resp.Stdout)
		return nil
	}
	err = resp.Binary.Save()
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println(resp.Stdout)
	return nil
}

func LoadRMakeConf(file string) *RMakeConf {
	fi,err := os.Open(file)
	if err != nil {
		return nil
	}
	rmc := new(RMakeConf)
	dec := json.NewDecoder(fi)
	err = dec.Decode(rmc)
	if err != nil {
		return nil
	}
	return rmc
}

func (rmc *RMakeConf) Save(file string) error {
	fi,err := os.Create(file)
	if err != nil {
		return err
	}
	defer fi.Close()
	enc := json.NewEncoder(fi)
	return enc.Encode(rmc)
}

func printHelpAdd() {
	fmt.Println("rmake add: Adds files to be used in the build process.")
}

func printHelpBin() {
	fmt.Println("rmake bin: set the name of the output binary to return.")
}

func printHelpScr() {
	fmt.Println("rmake scr: set the build command to run on the server.")
}

func printHelpServer() {
	fmt.Println("rmake server: set the url and port of the build server.")
}

func printHelpClean() {
	fmt.Println("rmake clean: resets mod times on your files and starts a new session with the build server.")
}

func printHelp(which string) {
	switch which {
	case "add":
		printHelpAdd()
	case "bin":
		printHelpBin()
	case "server":
		printHelpServer()
	case "scr":
		printHelpScr()
	case "clean":
		printHelpClean()
	default:
		fmt.Println("Usage: rmake [command] [args...]")
		printHelpAdd()
		printHelpBin()
		printHelpScr()
		printHelpServer()
		printHelpClean()
	}
}

func main() {
	rmc := LoadRMakeConf("rmake.json")
	if rmc == nil {
		rmc = NewRMakeConf()
	}
	if len(os.Args) == 1 {
		err := rmc.DoBuild()
		if err != nil {
			fmt.Println(err)
		}
		rmc.Save("rmake.json")
		return
	}
	switch os.Args[1] {
		case "add":
			for _,v := range os.Args[2:] {
				fi := new(FileInfo)
				fi.Path = v
				fi.LastTime = time.Now().AddDate(-10,0,0)
				rmc.Files = append(rmc.Files, fi)
			}
		case "server":
			rmc.Server = os.Args[2]
		case "scr":
			toks := strings.Split(os.Args[2], " ")
			rmc.Command = toks[0]
			rmc.Args = toks[1:]
		case "bin":
			rmc.Output = os.Args[2]
		case "clean":
			for _,v := range rmc.Files {
				v.LastTime = time.Now().AddDate(-20,0,0)
			}
			rmc.Session = ""
		case "var":
			rmc.Vars[os.Args[2]] = os.Args[3]
		case "help":
			if len(os.Args) == 2 {
				printHelp("all")
			} else {
				printHelp(os.Args[2])
			}
	}
	rmc.Save("rmake.json")
}
