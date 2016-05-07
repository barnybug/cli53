export GO15VENDOREXPERIMENT=1

version := $(shell git describe --always --dirty)
exe = ./cmd/cli53
buildargs = -ldflags '-w -s -X github.com/barnybug/cli53.version=${version}'

.PHONY: all build install test coverage deps release

all: install

deps:
	go get github.com/pwaller/goupx
	go get github.com/wadey/gocovmerge
	go get github.com/lsegal/gucumber/cmd/gucumber

build:
	go build -i -v $(buildargs) $(exe)

install:
	go install $(buildargs) $(exe)

release:
	GOOS=linux GOARCH=386 go build $(buildargs) -o release/cli53-linux-386 $(exe)
	GOOS=linux GOARCH=amd64 go build $(buildargs) -o release/cli53-linux-amd64 $(exe)
	GOOS=linux GOARCH=arm go build $(buildargs) -o release/cli53-linux-arm $(exe)
	GOOS=darwin GOARCH=amd64 go build $(buildargs) -o release/cli53-mac-amd64 $(exe)
	GOOS=windows GOARCH=386 go build $(buildargs) -o release/cli53-windows-386.exe $(exe)
	GOOS=windows GOARCH=amd64 go build $(buildargs) -o release/cli53-windows-amd64.exe $(exe)
	goupx release/cli53-linux-amd64
	upx release/cli53-linux-386 release/cli53-linux-arm release/cli53-windows-386.exe
	cd release; sha256sum cli53-* > SHA256SUMS

test-unit:
	go test

test-integration: build
	gucumber

# run unit and system tests, then recombine coverage output
test-coverage:
	rm -rf coverage && mkdir coverage
	go test -covermode=count -coverprofile=coverage/unit.txt
	go test -c -covermode=count -coverpkg . -o ./cli53 ./cmd/cli53
	COVERAGE=1 gucumber
	gocovmerge coverage/*.txt > coverage.txt

test: test-unit test-integration

docker-build:
	sudo docker run --rm -v `pwd`:/go/src/github.com/barnybug/cli53 -w /go/src/github.com/barnybug/cli53 golang:1.6-alpine sh -c 'apk add --no-cache make git && make build'
	sudo docker build -t barnybug/cli53 .
	rm -f cli53
