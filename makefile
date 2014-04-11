all:
	cd rmake && go build
	cd rmakebuilder && go build
	cd rmakemanager && go build

