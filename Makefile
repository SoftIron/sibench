export GOPATH=$(shell pwd):/root/go
VERSION=`git describe`
BUILD_DATE=`date +%FT%T%z`

all = rbd sibench comms logger

all:	$(all)

sibench:
	go env -w GO111MODULE=off
	go get -tags nautilus -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}" $@
	go install -tags nautilus -ldflags "-X main.Version=${VERSION} -X main.BuildDate=${BUILD_DATE}" $@

ifeq ($(shell go env GOOS),linux)
rbd:
	go env -w GO111MODULE=off
	go get -tags nautilus github.com/ceph/go-ceph/rbd
	cp src/extensions/rbd_sibench.go src/github.com/ceph/go-ceph/rbd/rbd_sibench.go
	go install -tags nautilus github.com/ceph/go-ceph/rbd
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
	sed -i 's/TH MANUAL.*/TH "Sibench" "1" ""/' docs/sibench.1
	sed -i 's/Manual \\-/Sibench - Benchmarking Ceph clusters/' docs/sibench.1

.PHONY: rbd comms sibench logger test clean man
