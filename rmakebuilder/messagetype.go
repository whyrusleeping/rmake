package main

type BuilderRequest struct {
	//
	Input []File
	//
	Command string
	//
	ResultAddress string
	//
	Session string
}

//A response that is sent back from the server
//contains the result of a build
type BuilderResult struct {
	//
	Results []File
	//
	Stdout string
	//
	Error string
	//
	Success bool
	//
	Session string
}

//A build package, gets sent to the server to start a build
type ManagerRequest struct {
	//
	Jobs int
	//
	Arch string
	//
	OS string
}

type ManagerResult struct {
	//
	Builders []string
	//
	Session string
}
