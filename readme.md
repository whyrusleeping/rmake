#RMAKE
A very simple build server program

usage:

Run the server program on your build machine, and make sure you can connect to it from the client machine.

On the client machine, go into the directory you want to build in, and `rmake add file` for all the files your build process needs (This only needs to be done once). Then set the servers location with `rmake server myserver.com:1234`. Set your build command with `rmake scr "make -j 500"` and finally, set the name of the binary you want sent back: `rmake bin a.out`. After this, simple run `rmake` to perform a build! Its that easy!

Feedback, bug reports and feature requests are very much appreciated and wanted!
