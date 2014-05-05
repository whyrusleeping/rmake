#RMAKE
A very simple build server program

usage:

Run the server program on your build machine, and make sure you can connect to it from the client machine.

On the client machine, cd into the directory you want to build in, and run 

    rmake add file
	rmake add src/*.cpp
	rmake add src/*.h
	rmake add builddeps/*
	
for all the files your build process needs, This only needs to be done once per project. rmake will also keep track of the mod times on files and only update the files on the server if youve made a change.

With the our latest version of rmake, you will also have to specify the jobs that have to be run in order to allow the system to distribute the builds across multiple servers.

Next set the servers location with 

    rmake server myserver.com:1234
	
Now youll need to create jobs that will be run.

    rmake job add "gcc -c main.c" main.o main.c header.h

This command would add a job that depends on main.c and header.h, runs "gcc -c main.c" and outputs main.o. This syntax is not finalized and suggestions for a better method are very welcome!

And finally, set the name of the output you want sent back: 

    rmake out a.out
	
As of now, rmake only supports the return of a single file, if it is needed, support for returning multiple files can be added.

After all that, simple run `rmake` to perform a build! Its that easy!

##Extra Options

rmake provides the ability to specify environment variables for the servers build environment.

	rmake var CFLAGS "-O2 -g -Wall"
	rmake var CXX clang++

##Installation
Installation is pretty simple with go get. First, make sure you have go installed via your package manager, or built from source, and your gopath configured.

To configure your gopath, add the following to your .bashrc:

	export GOPATH=$HOME/gopkg

or your fish config:

	set -x GOPATH $HOME/gopkg

Then, simply:

	go get github.com/whyrusleeping/rmake/rmake
	go get github.com/whyrusleeping/rmake/rmakemanager
	go get github.com/whyrusleeping/rmake/rmakebuilder
	
Feedback, bug reports and feature requests are very much appreciated and wanted!
