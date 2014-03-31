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

//The data that is sent back to the client after the build
//is completed
type Response struct {
	//Everything printed to stdout
	Stdout string

	//Any error that was encountered
	Error string

	//The built program
	Binary *File

	//Whether or not the build threw a non-zero exit code
	Success bool

	//The session used for this build
	Session string
}

type File struct {
	Path string
	Contents []byte
	Mode os.FileMode
}

func (f *File) Save(builddir string) error {
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
	Vars map[string]string
}

func (p *Package) MakeCmd(dir string) *exec.Cmd {
	proc := exec.Command(p.Command, p.Args...)
	proc.Dir = dir
	for k,v := range p.Vars {
		proc.Env = append(proc.Env, fmt.Sprintf("%s=%s",k,v))
	}
	return proc
}

func ReadPackage(c net.Conn) (*Package, error) {
	pack := new(Package)
	unzip,err := gzip.NewReader(c)
	if err != nil {
		return nil,err
	}
	dec := gob.NewDecoder(unzip)
	err = dec.Decode(pack)
	if err != nil {
		return nil,err
	}
	return pack, nil
}

func HandleConnection(c net.Conn) {
	pack,err := ReadPackage(c)
	if err != nil {
		fmt.Println(err)
		c.Close()
		return
	}
	resp := new(Response)

	//No matter how this function ends, we want to send a response back
	defer func () {
		fmt.Println("Deferred processing now taking place.")
		zip := gzip.NewWriter(c)
		enc := gob.NewEncoder(zip)
		err = enc.Encode(resp)
		if err != nil {
			fmt.Println("Failed to respond!")
			fmt.Println(err)
			return
		}
		fmt.Println("Encoding Finished!")
		zip.Close()
		c.Close()
		fmt.Println("Great Success!")
	}()

	//Set all specified vars
	for key,val := range pack.Vars {
		os.Setenv(key,val)
	}

	//If the user has an existing session, use it.
	dir := pack.Session
	if dir == "" {
		//Otherwise, Get a random dir-name
		dir = RandDir()
	}
	resp.Session = dir
	dir = "build/" + dir
	fmt.Printf("Build dir = '%s'\n", dir)
	os.Mkdir(dir, os.ModeDir | 0777)

	//Write all needed files/updates to disk
	for _,f := range pack.Files {
		f.Save(dir)
	}
	proc := pack.MakeCmd(dir)
	b,err := proc.CombinedOutput()
	resp.Stdout = string(b)
	if err != nil {
		fmt.Println(err)
		resp.Error = err.Error()
		resp.Success = false
		return
	}
	fmt.Println("Loading output.")
	bin := LoadFile(dir + "/" + pack.Output)
	fmt.Printf("Binary size: %d\n", len(bin.Contents))
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
	listname := ":11221"
	if len(os.Args) == 2 {
		listname = os.Args[1]
	}
	os.Mkdir("build", os.ModeDir | 0777)
	list,err := net.Listen("tcp",listname)
	if err != nil {
		panic(err)
	}

	for {
		con,err := list.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go HandleConnection(con)
	}
}
