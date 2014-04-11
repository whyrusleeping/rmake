package rmake

import (
	"encoding/gob"
)

func init() {
	gob.Register(&BuilderResult{})
	gob.Register(&ManagerRequest{})
}

//A response that is sent back from the server
//contains the result of a build
type BuilderResult struct {
	Stdout  string
	Error   string
	Binary  *File
	Success bool
	Session string
}

//A build package, gets sent to the server to start a build
type ManagerRequest struct {
	Type    int
	Files   []*File
	Command string
	Args    []string
	Output  string
	Session string
	Vars    map[string]string
}
