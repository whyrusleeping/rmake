package rmake

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

type FileInfo struct {
	Path     string
	LastTime time.Time
}

type File struct {
	Path     string
	Contents []byte
	Mode     os.FileMode
}

//Load a file relative to the given directory
func LoadFile(path string) *File {
	inf, err := os.Stat(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	/*
	//TODO: worry about modtimes
	if !inf.ModTime().After(cf.LastTime) {
		return nil
	}
	*/
	fmt.Printf("Sending '%s'\n", path)
	f := new(File)
	f.Path = path
	f.Mode = inf.Mode()
	//LastTime = time.Now()
	cnts, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	f.Contents = cnts
	return f
}

//Make sure all needed directories are created and write file to disk
func (f *File) Save(dir string) error {
	cur := "."
	complete := path.Join(dir, f.Path)
	spl := strings.Split(complete, "/")
	for _, v := range spl[:len(spl)-1] {
		cur = path.Join(cur, v)
		os.Mkdir(cur, os.ModeDir|0777)
	}
	cur = path.Join(cur, spl[len(spl)-1])
	fi, err := os.OpenFile(cur, os.O_CREATE|os.O_WRONLY, f.Mode)
	if err != nil {
		return err
	}
	_, err = fi.Write(f.Contents)
	if err != nil {
		return err
	}
	return nil
}
