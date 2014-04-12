package rmake

import (
	"encoding/gob"
)

func init() {
	gob.Register(&BuilderResult{})
	gob.Register(&ManagerRequest{})
	gob.Register(&ManagerResult{})
	gob.Register(&BuilderRequest{})
}

//Manager -> Builder
type BuilderRequest struct {
	//
	Input []*File
	//
	Command string
	//
	ResultAddress string
	//
	Session string
}

//A response that is sent back from the server
//contains the result of a build
//Builder -> ????
type BuilderResult struct {
	//
	Results []*File
	//
	Stdout string
	//
	Error string
	//
	Success bool
	//
	Session string
}

//A build package, gets sent to the manager to start a build
//Client -> Manager
type ManagerRequest struct {
	//
	Jobs int
	Jobs []*Job
	//
	Arch string
	//
	OS string
}

//Manager -> Client
type ManagerResult struct {
	//
	Builders []string
	//
	Session string
}

//Used for sending files to different builder nodes
//i.e. sending source files from the manager to buidlers
//or sending object files from builders to the linker node
//Manager -> Builder
//Bulider -> Builder
type RequiredFileMessage struct {
	Payload *File
	Session string
}
