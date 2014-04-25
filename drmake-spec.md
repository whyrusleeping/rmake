#dRmake Specification

##Goals
The primary goal of rmake is to provide a lightning fast distributed build system for groups who work on large projects. We also aim to make using and configuring rmake as simple as possible, other tools that might fill the same niche can be a pain to handle so we are hoping to keep our interface familiar to any prospective users.

##Network Configuration
An rmake build cluster has two components, the manager node, which handles distributing jobs and source code between the builder nodes, and the builder nodes which queue up jobs to be run and execute them, sending the resulting files where they need to be for the next step. The manager and the builder nodes must be on the same network in order to ensure fast connectivity between them. The client, on the other hand, can be situated anywhere, as long as the manager has an IP address that is accessible to it.

##Builds (Quick overview)
Builds are started by a client program who creates a build package (described later) and sends it to the manager node. From there, the manager assigns each of the tasks required to perform the build to builder nodes in the cluster based on their current workload. Builder nodes keep a queue of tasks, referred to as jobs, and execute them and send the output where it is needed.
Currently rmake supports C and C++ style building, where a large number of object files are built asynchronously and sent to a previously selected node for linking the final executable.
Once the final executable is built it is sent from the builder node that linked it, to the manager node, and finally back to the client who initiated the build.

##Jobs
Our current implementation allows the client to specify a series of jobs that need to be run in order to complete the build.
The Job structure looks like:

	type Job struct {
		Command string
		Args []string
		Deps []string
		Output string
	}

And is declared in types/job.go

The command field specifies the base command that will be run (i.e. gcc or clang++). The Args array is an array of the arguments that will be passed to the command. The Deps array is a list of files that are required by this job in order to run. When a builder is starting on this job, it first checks to ensure all of the dependencies are available, if not, it waits. The output field is the name of the resulting file that the command will create, for most commands, this will be an object file, or the final program binary in the case of a linker job. Currently, we only support one output file per command.

##Client (rmake)
The client program parses in an rmake configuration file created either by hand or with the rmake tool itself and uses it to create and send a build package to an rmake manager. 
The build package has the following structure:

	type BuildPackage struct {
		Jobs []*Job
		Arch string
		OS string
		Output string
		Files map[string]*File
	}

Jobs is a list of jobs as defined above. Arch and OS are the CPU architechture and operating system of the build target. Output is the name of the desired binary that will be built, and Files is a map of filenames to File objects. Files in this map are generally source files or other dependencies of the build.

##Manager (rmakemanager)
The manager servers as an intermediary between the client and the builder servers who perform the build itself. The manager is responsible for scheduling which builder should perform which jobs based on their current load. 

###Job Scheduling
The manager maintains a priority queue of all builder nodes in its network. When a job, or several jobs, are recieved from a client, the manager pops the 'least busy' node off of its queue, assigns it a job, and pushes it back into the queue. This way, load it fairly balanced across the cluster.
The manager also recieves periodic updates from the builder using a publish subscribe style system, giving a more accurate depiction of its current load, upon recieving this information, the manager updates the builders position in the queue.

###Job Updates
As different build nodes complete their jobs, notifications of that event are sent to the manager where they are collected and relayed to the client.

###Sessions
When a client connects to the manager, the manager creates a session ID for it. This session ID is used to mark jobs on the builders so that they can talk to each other more easily. Currently, sessions IDs are not reused, but in the future we will use them to speed up builds by only rebuilding files changes since the last build.

###Build Failures
When a build fails, due to poorly written code or other compiler errors, the manager notifies all the builders that they should stop working on that build, the builders the go through their queue of jobs and remove any associated with that build. And then notifies the client that their build has failed, complete with compiler error messages.

##Builder (rmakebuilder)
The builder is responsible for accepting jobs from the manager, performing them, and send the output either back to the manager, or to another builder node as part of a later job.
When starting up a builder, you can specify a maximum number of processes to have running at any given time. For highest efficiency, let this number either be equal to, or one less than the number of logical processors on the build machine. Lower numbers can be used if this machine has other things it needs to be doing at the same time as building.

##Roadmap for the future
- Makefile parsing
- build timing statistics
- Build optimization via analysis
- persistent build sessions (for unchanged file reuse)
- robust easy to use frontend (cli)
- automatic dependency resolution
