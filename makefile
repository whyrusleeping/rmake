all:
	cd types && go install
	cd rmake && go install
	cd rmakebuilder && go install -race
	cd rmakemanager && go install -race

test:
	cd rmakebuilder && go test
