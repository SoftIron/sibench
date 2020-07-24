export GOPATH=$(shell pwd):/root/go
all = sibench comms

all:	$(all)

sibench:
	go get $@
	go install $@

comms:
	go get $@
	go install $@

test:
	go test -v ./...

.PHONY: clean

clean:
	go clean ./...
	rm -f bin/*
