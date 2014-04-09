package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

type File struct {
	Path     string
	Contents []byte
	Mode     os.FileMode
}

func (cf *FileInfo) LoadFile() *File {
	inf, err := os.Stat(cf.Path)
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
	cnts, err := ioutil.ReadFile(cf.Path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	f.Contents = cnts
	return f
}

func (f *File) Save() error {
	cur := "."
	spl := strings.Split(f.Path, "/")
	for _, v := range spl[:len(spl)-1] {
		cur += "/" + v
		os.Mkdir(cur, os.ModeDir|0777)
	}
	fi, err := os.OpenFile(f.Path, os.O_CREATE|os.O_WRONLY, f.Mode)
	if err != nil {
		return err
	}
	_, err = fi.Write(f.Contents)
	if err != nil {
		return err
	}
	return nil
}
