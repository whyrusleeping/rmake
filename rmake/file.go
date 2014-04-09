package main

import (
	"os"
	"time"
	"strings"
	"fmt"
	"io/ioutil"
)

//A struct holding a files data for sending between machines
type File struct {
	Path string
	Contents []byte
	Mode os.FileMode
}

//Load a file specified by a given FileInfo struct
//returns nil if the file has not been modified since
func (cf *FileInfo) LoadFile() *File {
	inf,err := os.Stat(cf.Path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	if !inf.ModTime().After(cf.LastTime) {
		return nil
	}
	fmt.Printf("Sending '%s'\n", cf.Path)
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
	return f
}

//Write file to disk locally
func (f *File) Save() error {
	cur := "."
	spl := strings.Split(f.Path,"/")
	for _,v := range spl[:len(spl)-1] {
		cur += "/" + v
		os.Mkdir(cur, os.ModeDir | 0777)
	}
	fi,err := os.OpenFile(f.Path, os.O_CREATE | os.O_WRONLY, f.Mode)
	if err != nil {
		return err
	}
	_,err = fi.Write(f.Contents)
	if err != nil {
		return err
	}
	return nil
}
