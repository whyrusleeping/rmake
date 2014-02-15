package main

import (
	"os"
	"fmt"
	"net"
	"strings"
	"io/ioutil"
	"encoding/gob"
	"encoding/json"
	"compress/gzip"
)

type Response struct {
	Stdout string
	Binary []byte
}

type File struct {
	Path string
	Contents []byte
}

func LoadFile(path string) *File {
	f := new(File)
	f.Path = path
	cnts,err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	f.Contents = cnts
	return f
}

type Package struct {
	Files []*File
	Command string
	Args []string
	Output string
}

func NewPackage(conf *RMakeConf) *Package {
	p := new(Package)
	p.Output = conf.Output
	p.Command = conf.Command
	p.Args = conf.Args
	for _,v := range conf.Files {
		p.Files = append(p.Files, LoadFile(v))
	}
	return p
}

/* Example rmake.json
{
	"Server" : "jero.my:11221",
	"Files":["makefile","src/main.cpp","src/main.h","src/something.h"],
	"BuildScr" : "make",
	"Output" : "a.out"
}
*/

type RMakeConf struct {
	Server string
	Files []string
	Command string
	Args []string
	Output string
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

	out,err := os.Create(rmc.Output)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer out.Close()

	_,err = out.Write(resp.Binary)
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

func printHelp() {
	fmt.Println("Usage: rmake option params")
}

func main() {
	rmc := LoadRMakeConf("rmake.json")
	if rmc == nil {
		rmc = new(RMakeConf)
	}
	if len(os.Args) == 1 {
		err := rmc.DoBuild()
		if err != nil {
			fmt.Println(err)
		}
		return
	}
	switch os.Args[1] {
		case "add":
			for _,v := range os.Args[2:] {
				rmc.Files = append(rmc.Files, v)
			}
		case "server":
			rmc.Server = os.Args[2]
		case "scr":
			toks := strings.Split(os.Args[2], " ")
			rmc.Command = toks[0]
			rmc.Args = toks[1:]
		case "bin":
			rmc.Output = os.Args[2]
	}
	rmc.Save("rmake.json")
}
