export GOPATH=$(shell pwd):/root/go
all = sibench comms logger

all:	$(all)

sibench:
	go get $@
	go install $@

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
