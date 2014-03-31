package main

import (
	"os"
	"fmt"
	"path/filepath"
	"net"
	"time"
	"strings"
	"io"
	"bufio"
	"encoding/gob"
	"github.com/dustin/go-humanize"
	"encoding/json"
	"compress/gzip"
)

type Response struct {
	Stdout string
	Binary *File
	Success bool
	Session string
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
	p.Session = conf.Session
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
	Compression string
	ignore []string
}

func NewRMakeConf() *RMakeConf {
	rmc := new(RMakeConf)
	rmc.Vars = make(map[string]string)
	return rmc
}

func (rmc *RMakeConf) LoadIgnores(igfile string) {
	fi,err := os.Open(igfile)
	if err != nil {
		return
	}
	scan := bufio.NewScanner(fi)
	for scan.Scan() {
		rmc.ignore = append(rmc.ignore, scan.Text())
	}
}

func (rmc *RMakeConf) Clean() {
	for _,v := range rmc.Files {
		v.LastTime = time.Now().AddDate(-20,0,0)
	}
	rmc.Session = ""
}

func (rmc *RMakeConf) IsIgnored(fi string) bool {
	return false
}

func (rmc *RMakeConf) Status() error {
	fmt.Println("\x1b[0m# Current working tree status\x1b[0m")
	fmt.Println("\x1b[0m#   (use \"rmake remove <file>...\" to no longer track the file)\x1b[0m")
	fmt.Println("#")
	fmt.Println("\x1b[0m# Modified files to be updated\x1b[0m")
	fmt.Println("#")

	for _,v := range rmc.Files {
		inf,err := os.Stat(v.Path)
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

	for _,v := range rmc.Files {
		inf,err := os.Stat(v.Path)
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

		for _,v := range rmc.Files {
			if path == v.Path {
				return nil
			}
		}

		fmt.Printf("#       %s\n", path);
		return nil
	})

	return nil
}

func (rmc *RMakeConf) DoBuild() error {
	pack := NewPackage(rmc)
	con,err := net.Dial("tcp", rmc.Server)
	if err != nil {
		return err
	}
	defer con.Close()
	zipp := rmc.Gzipper(con)
	enc := gob.NewEncoder(zipp)
	err = enc.Encode(pack)
	if err != nil {
		return err
	}
	//Make sure all data gets flushed through
	zipp.Close()

	resp := new(Response)
	unzip,err := gzip.NewReader(con)
	if err != nil {
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
		rmc.Clean()
		return nil
	}
	fmt.Printf("Build finished, output size: %d\n", len(resp.Binary.Contents))
	rmc.Session = resp.Session
	err = resp.Binary.Save()
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println(resp.Stdout)
	return nil
}

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
	zipper,err := gzip.NewWriterLevel(w, complev)
	if err != nil {
		panic(err)
	}
	return zipper
}

func LoadRMakeConf(file string) (*RMakeConf,error) {
	fi,err := os.Open(file)
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
	return rmc,nil
}

func (rmc *RMakeConf) Save(file string) error {
	fi,err := os.Create(file)
	if err != nil {
		return err
	}
	defer fi.Close()
	out,_ := json.MarshalIndent(rmc,"","\t")
	_,err = fi.Write(out)
	return err
}

func main() {
	rmc,err := LoadRMakeConf("rmake.json")
	if err != nil {
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
			fi.LastTime = time.Now().AddDate(-20,0,0)
			fmt.Printf("Adding: '%s'\n", v)
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
		rmc.Clean()
	case "var":
		rmc.Vars[os.Args[2]] = os.Args[3]
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
