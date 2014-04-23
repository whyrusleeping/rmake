package rmake

import (
	"encoding/gob"
	"time"
)

const ProtocolVersion = 1

func init() {
	gob.Register(&BuilderRequest{})
	gob.Register(&BuilderResult{})
	gob.Register(&ManagerRequest{})
	gob.Register(&FinalBuildResult{})
	gob.Register(&RequiredFileMessage{})
	gob.Register(&BuilderStatusUpdate{})
	gob.Register(&JobFinishedMessage{})
	gob.Register(&BuildStatus{})
	gob.Register(&BuilderAnnouncement{})
	gob.Register(&ManagerAcknowledge{})
	gob.Register(&Job{})
}

// Announce a builder
type BuilderAnnouncement struct {
	// The builder's hostname
	Hostname string
	// The Builder's listening address
	ListenerAddr string
	// The version of the protocol we are using
	ProtocolVersion int
}

// Create a new builder announcement
func NewBuilderAnnouncement(hn string, la string) *BuilderAnnouncement {
	ba := new(BuilderAnnouncement)
	ba.Hostname = hn
	ba.ListenerAddr = la
	ba.ProtocolVersion = ProtocolVersion
	return ba
}

// Acknowledge a builder announcement
// Provide a uuid
type ManagerAcknowledge struct {
	// The UUID of the builder
	UUID int
	// The version of the protocol we are using
	ProtocolVersion int
	// Successful or not
	Success bool
	// Message
	Message string
}

// Create a new manager ack
func NewManagerAcknowledge(uuid int, s bool, m string) *ManagerAcknowledge {
	ma := new(ManagerAcknowledge)
	ma.ProtocolVersion = ProtocolVersion
	ma.Success = s
	ma.Message = m
	ma.UUID = uuid
	return ma
}

// Create a new successful manager ack
func NewManagerAcknowledgeSuccess(uuid int) *ManagerAcknowledge {
	return NewManagerAcknowledge(uuid, true, "")
}

// Create a new failed manager ack
func NewManagerAcknowledgeFailure(message string) *ManagerAcknowledge {
	return NewManagerAcknowledge(-1, false, message)
}

//Manager -> Builder
type BuilderRequest struct {
	BuildJob *Job

	Input []*File
	Wait []string
	//The address of the node to send the output to
	//empty string means keep it local
	ResultAddress string
	//
	Session string
}

func (br *BuilderRequest) GetFile(fi string) *File {
	for _,f := range br.Input {
		if f.Path == fi {
			return f
		}
	}
	return nil
}

//A response from a builder who has finished a job
//Builder -> Manager
type JobFinishedMessage struct {
	//Standard out from running a job
	Stdout  string
	Error   string
	Success bool
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

	//The file that we are expecting to be built
	Output string

	//Files to be transferred
	Files map[string]*File
}

//A message to indicate to the client the build status
//Manager -> Client
type BuildStatus struct {
	// The status mesage
	Message string
	// The percent complete
	PercentComplete float32

	Session string
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
type BuilderStatusUpdate struct {
	QueuedJobs int
	CPULoad    float32
	MemUse     float32
}
