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

Next set the servers location with 

    rmake server myserver.com:1234
	
Set your build command with 

    rmake scr "make -j 500"
	
And finally, set the name of the binary you want sent back: 

    rmake bin a.out
	
As of now, rmake only supports the return of a single file, if it is needed, support for returning multiple files can be added.

After all that, simple run `rmake` to perform a build! Its that easy!

Feedback, bug reports and feature requests are very much appreciated and wanted!
