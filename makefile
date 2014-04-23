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

# Build everything
all:
	cd types && go install
	cd rmake && go install
	cd rmakebuilder && go install $(INSTALL_OPT)
	cd rmakemanager && go install $(INSTALL_OPT)

# Tests for rmake
test:
	cd rmakebuilder && go test
