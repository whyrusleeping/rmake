package main

import (
	"os"
	"os/exec"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"io"
	"io/ioutil"
	"bytes"
	"encoding/gob"
	"compress/gzip"
	"crypto/rand"
)

type Response struct {
	Stdout string
	Binary *File
	Success bool
	Session string
}

type File struct {
	Path string
	Contents []byte
	Mode os.FileMode
}

func (f *File) Save(builddir string) error {
	fmt.Printf("Saving file: '%s'\n", f.Path)
	cur := builddir
	spl := strings.Split(f.Path,"/")
	for _,v := range spl[:len(spl)-1] {
		cur += "/" + v
		os.Mkdir(cur, os.ModeDir | 0777)
	}
	fi,err := os.OpenFile(builddir + "/" + f.Path, os.O_CREATE | os.O_WRONLY, f.Mode)
	if err != nil {
		fmt.Println("File creation failed.")
		return err
	}
	fi.Write(f.Contents)
	fmt.Printf("Wrote %d bytes.\n", len(f.Contents))
	fi.Close()
	return nil
}

func LoadFile(path string) *File {
	inf,err := os.Stat(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	f := new(File)
	f.Path = path
	f.Mode = inf.Mode()
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
	Session string
	Output string
}

func HandleBuild(c net.Conn) {
	defer c.Close()
	resp := new(Response)
	pack := new(Package)
	unzip,err := gzip.NewReader(c)
	if err != nil {
		fmt.Println(err)
		return
	}
	dec := gob.NewDecoder(unzip)
	err = dec.Decode(pack)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func () {
		zip := gzip.NewWriter(c)
		enc := gob.NewEncoder(zip)
		err = enc.Encode(resp)
		if err != nil {
			fmt.Println("Failed to respond!")
			fmt.Println(err)
			return
		}
		zip.Close()
	}()
	dir := pack.Session
	if dir == "" {
		dir = RandDir()
	}
	fmt.Printf("Build dir = '%s'\n", dir)
	os.Mkdir(dir, os.ModeDir | 0777)
	for _,f := range pack.Files {
		f.Save(dir)
	}
	proc := exec.Command(pack.Command, pack.Args...)
	proc.Dir = dir
	fmt.Println(proc)
	b,err := proc.CombinedOutput()
	if err != nil {
		fmt.Println(err)
		fmt.Println(string(b))
		/*
		b,_ = ioutil.ReadAll(proc.Stdout)
		fmt.Println(string(b))
		*/
		resp.Success = false
		return
	}
	resp.Stdout = string(b)
	bin := LoadFile(dir + "/" + pack.Output)
	bin.Path = pack.Output
	resp.Binary = bin
	resp.Success = true
	return
}

func RandDir() string {
	buf := new(bytes.Buffer)
	io.CopyN(buf, rand.Reader, 16)
	return hex.EncodeToString(buf.Bytes())
}

func main() {
	os.Mkdir("build", os.ModeDir | 0777)
	list,err := net.Listen("tcp",":11221")
	if err != nil {
		panic(err)
	}
	for {
		con,err := list.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go HandleBuild(con)
	}
}
