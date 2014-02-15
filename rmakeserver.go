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
	Binary []byte
	Success bool
}

type File struct {
	Path string
	Contents []byte
}

func (f *File) Save(builddir string) error {
	cur := builddir
	spl := strings.Split(f.Path,"/")
	for _,v := range spl[:len(spl)-1] {
		cur += v
		os.Mkdir(cur, os.ModeDir | 0777)
	}
	fi,err := os.Create(builddir + "/" + f.Path)
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
}

func HandleBuild(c net.Conn) {
	dir := RandDir()
	os.Mkdir(dir, os.ModeDir | 0777)
	o := new(bytes.Buffer)
	resp := new(Response)
	pack := new(Package)
	unzip,err := gzip.NewReader(c)
	if err != nil {
		fmt.Fprintln(o, err)
		return
	}
	dec := gob.NewDecoder(unzip)
	err = dec.Decode(pack)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _,f := range pack.Files {
		f.Save(dir)
	}
	proc := exec.Command(pack.Command, pack.Args...)
	proc.Dir = dir
	b,err := proc.Output()
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Stdout = string(b)

	oby,err := ioutil.ReadFile(dir + "/" + pack.Output)
	if err != nil {
		fmt.Println(err)
		return
	}
	resp.Binary = oby

	zip := gzip.NewWriter(c)
	enc := gob.NewEncoder(zip)
	err = enc.Encode(resp)
	if err != nil {
		fmt.Println(err)
		return
	}
	zip.Close()
	c.Close()
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
