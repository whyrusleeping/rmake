all:
	cd rmake && go install .
	cd rmakebuilder && go install .
	cd rmakemanager && go install .
