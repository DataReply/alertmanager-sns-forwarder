PROJECT_NAME := alertmanager-sns-forwarder
GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')
 

all: clean dep test bin

bin: bin/linux/${PROJECT_NAME}

bin/%: LDFLAGS=-X github.com/DataReply/${PROJECT_NAME}/${PROJECT_NAME}.Version=${APP_VERSION}
bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	CGO_ENABLED=0 GOOS=$(word 1, $(subst /, ,$*)) GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o "$@" github.com/DataReply/${PROJECT_NAME}/

test:
	go test github.com/DataReply/${PROJECT_NAME}/...

dep:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure -vendor-only

clean:
	rm -rf bin

.PHONY: all
