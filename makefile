all:
	cd types && go install
	cd rmake && go install
	cd rmakebuilder && go install
	cd rmakemanager && go install

test:
	cd rmakebuilder && go test
