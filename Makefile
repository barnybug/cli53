export GO15VENDOREXPERIMENT=1

exe = ./cmd/cli53

.PHONY: all build install test coverage test-deps

all: install

test-deps:
	go get github.com/wadey/gocovmerge
	go install github.com/wadey/gocovmerge
	go get github.com/gucumber/gucumber/cmd/gucumber
	go install github.com/gucumber/gucumber/cmd/gucumber

build:
	go build -v $(exe)

install:
	go install $(exe)

test-unit:
	go test

test-integration: build
	@tmp_gopath=$$(mktemp -d /tmp/cli53-gucumber.XXXXXX); \
	GOBIN=$$tmp_gopath/bin go install github.com/gucumber/gucumber/cmd/gucumber@v0.0.0-20180127021336-7d5c79e832a2; \
	mkdir -p $$tmp_gopath/src/github.com/barnybug; \
	ln -s $$(pwd) $$tmp_gopath/src/github.com/barnybug/cli53; \
	cd $$tmp_gopath/src/github.com/barnybug/cli53 && \
	GOPATH=$$tmp_gopath $$tmp_gopath/bin/gucumber; \
	status=$$?; \
	chmod -R +w $$tmp_gopath; \
	rm -rf $$tmp_gopath; \
	exit $$status

# run unit and system tests, then recombine coverage output
test-coverage: test-deps
	rm -rf coverage && mkdir coverage
	go test -covermode=count -coverprofile=coverage/unit.txt
	go test -c -covermode=count -coverpkg . -o ./cli53 ./cmd/cli53
	@tmp_gopath=$$(mktemp -d /tmp/cli53-gucumber.XXXXXX); \
	GOBIN=$$tmp_gopath/bin go install github.com/gucumber/gucumber/cmd/gucumber@v0.0.0-20180127021336-7d5c79e832a2; \
	mkdir -p $$tmp_gopath/src/github.com/barnybug; \
	ln -s $$(pwd) $$tmp_gopath/src/github.com/barnybug/cli53; \
	cd $$tmp_gopath/src/github.com/barnybug/cli53 && \
	COVERAGE=1 GOPATH=$$tmp_gopath $$tmp_gopath/bin/gucumber; \
	status=$$?; \
	chmod -R +w $$tmp_gopath; \
	rm -rf $$tmp_gopath; \
	exit $$status
	gocovmerge coverage/*.txt > coverage.txt

test: test-unit test-integration

docker-build:
	sudo docker run --rm -v `pwd`:/go/src/github.com/barnybug/cli53 -w /go/src/github.com/barnybug/cli53 golang:1.6-alpine sh -c 'apk add --no-cache make git && make build'
	sudo docker build -t barnybug/cli53 .
	rm -f cli53
