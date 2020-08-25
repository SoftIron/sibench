export GOPATH=$(shell pwd):/root/go
VERSION=`git describe`
BUILD_DATE=`date +%FT%T%z`

all = sibench comms logger

all:	$(all)

sibench:
	go get -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}" $@
	go install -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}" $@

comms:
	go get $@
	go install $@

logger:
	go get $@
	go install $@

test:
	go test -v ./...

clean:
	go clean ./...
	rm -f bin/*

.PHONY: comms sibench logger test clean
