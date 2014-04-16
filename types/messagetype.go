package rmake

import (
	"encoding/gob"
	"time"
)

func init() {
	gob.Register(&BuilderRequest{})
	gob.Register(&BuilderResult{})
	gob.Register(&ManagerRequest{})
	gob.Register(&FinalBuildResult{})
	gob.Register(&RequiredFileMessage{})
	gob.Register(&BuilderInfoMessage{})
	gob.Register(&BuildFinishedMessage{})
	gob.Register(&BuildStatus{})
	gob.Register(&Job{})
}

//Manager -> Builder
type BuilderRequest struct {
	BuildJob *Job

	Input []*File
	//The address of the node to send the output to
	//empty string means keep it local
	ResultAddress string
	//
	Session string
}

//A response from a builder who has finished a job
//Builder -> Manager
type BuildFinishedMessage struct {
	//Standard out from running a job
	Stdout  string
	Error   string
	Success bool
}

//A response that is sent back from the server
//contains the result of a build
//Builder -> Builder
//Builder -> Manager
type BuilderResult struct {
	//
	Results []*File
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

// A message to indicate to the client the build status
type BuildStatus struct {
	// The status mesage
	Message string
	// The percent complete
	PercentComplete float32
}

//The final message sent back from the manager after the build is done
//Manager -> Client
type FinalBuildResult struct {
	Session string

	Success   bool
	Error     string
	Stdout    string
	Results   []*File
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

//Sent periodically to the manager to inform it of the builders status
//Builder -> Manager
type BuilderInfoMessage struct {
	QueuedJobs int
	CPULoad    float32
	MemUse     float32
}
