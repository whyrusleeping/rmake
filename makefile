# Makefile for rmake
# 

# Environment Variables
OS=$(shell go env GOOS)
ARCH=$(shell go env GOARCH)
INSTALL_OPT=

# Figure out the OS and Arch specific options
ifeq ($(OS), $(filter $(OS), linux darwin windows))
	ifeq ($(ARCH),amd64)
	INSTALL_OPT+=-race
	endif
endif

.PHONY: all clean dep test

# Build everything
all:
	cd rmake && go install
	cd rmakebuilder && go install $(INSTALL_OPT)
	cd rmakemanager && go install $(INSTALL_OPT)

# Clean up files
clean:
	go clean
	cd rmake && go clean
	cd rmakebuilder && go clean
	cd rmakemanager && go clean

# Tests for rmake
test:
	cd rmakebuilder && go test

# Dependencies 
dep:
	go get -u github.com/dustin/go-humanize
	go get -u github.com/cihub/seelog
