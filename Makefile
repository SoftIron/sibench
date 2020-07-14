export GOPATH=$(shell pwd):/root/go
all = sibench comms

all:	$(all)

sibench:
	go install sibench

comms:
	go install comms

test:
	go test -v ./...

clean:
	go clean ./...
	rm -f bin/*
