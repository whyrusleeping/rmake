#dRmake Specification

##Network Configuration
An rmake build cluster has two components, the manager node, which handles distributing jobs and source code between the builder nodes, and the builder nodes which queue up jobs to be run and execute them, sending the resulting files where they need to be for the next step.

##Builds
Builds are started by a client program who creates a build package (described later) and sends it to the manager node. From there, the manager decides on a list of builder nodes that will participate in this build based on their current work loads, cpu usage and memory consumption. After choosing a node set, the manager distributes the jobs to their respective nodes.
Currently rmake supports C and C++ style building, where a large number of object files are built independantly and sent to a final node for linking the final executable.
Once the final executable is built it is sent from the builder node that linked it, to the manager node, and finally back to the client who initiated the build.
