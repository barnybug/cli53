export GO15VENDOREXPERIMENT=1

exe = ./cmd/cli53

.PHONY: all build install test coverage test-deps

all: install

test-deps:
	go get github.com/wadey/gocovmerge
	go get github.com/gucumber/gucumber/cmd/gucumber

build:
	go build -i -v $(exe)

install:
	go install $(exe)

upx:
	upx dist/cli53-linux-386/cli53-linux-386 dist/cli53-linux-arm/cli53-linux-arm dist/cli53-windows-386/cli53-windows-386.exe

test-unit:
	go test

test-integration: build
	gucumber

# run unit and system tests, then recombine coverage output
test-coverage: test-deps
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
