export GOPATH=$(shell pwd):/root/go
VERSION=`git describe`
BUILD_DATE=`date +%FT%T%z`

all = rbd sibench comms logger

all:	$(all)

sibench:
	go get -tags nautilus -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}" $@
	go install -tags nautilus -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}" $@

rbd:
	go get -tags nautilus github.com/ceph/go-ceph/rbd
	cp src/extensions/rbd_sibench.go src/github.com/ceph/go-ceph/rbd/rbd_sibench.go
	go install -tags nautilus github.com/ceph/go-ceph/rbd

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

.PHONY: rbd comms sibench logger test clean
