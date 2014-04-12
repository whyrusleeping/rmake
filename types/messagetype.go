package rmake

import (
	"time"
	"encoding/gob"
)

func init() {
	gob.Register(&BuilderRequest{})
	gob.Register(&BuilderResult{})
	gob.Register(&ManagerRequest{})
	gob.Register(&FinalBuildResult{})
	gob.Register(&RequiredFileMessage{})
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
//Builder -> Builder
//Builder -> Manager
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
	Jobs []*Job
	//
	Arch string
	//
	OS string

	//Files to be transferred
	Files []*File
}

//The final message sent back from the manager after the build is done
//Manager -> Client
type FinalBuildResult struct {
	Session string
	Results []*File
	BuildTime time.Time
}

//Used for sending files to different builder nodes
//i.e. sending source files from the manager to buidlers
//or sending object files from builders to the linker node
//Manager -> Builder
//Bulider -> Builder
//TODO: name this better
type RequiredFileMessage struct {
	Payload *File
	Session string
}
