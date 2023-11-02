export GOPATH=$(shell pwd):/root/go
VERSION=`git describe`
BUILD_DATE=`date +%FT%T%z`

all = rbd sibench comms logger

all:	$(all)

sibench:
	go install -tags pacific -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}" $@

ifeq ($(shell go env GOOS),linux)
rbd:
	go get -tags pacific github.com/ceph/go-ceph/rbd
	cp src/extensions/rbd_sibench.go src/github.com/ceph/go-ceph/rbd/rbd_sibench.go
	go install -tags pacific github.com/ceph/go-ceph/rbd
endif

comms:
	go get $@
	go install $@

logger:
	go get $@
	go install $@

test:
	go test -v ./...

clean:
	go clean ./... || true
	rm -f bin/* docs/sibench.1

man:
	rst2man docs/source/manual.rst docs/sibench.1
	sed -i 's/TH MANUAL.*/TH "sibench" "1" ""/' docs/sibench.1
	sed -i 's/Manual \\-/sibench - Benchmarking Ceph clusters/' docs/sibench.1

.PHONY: rbd comms sibench logger test clean man
