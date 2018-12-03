PROJECT_NAME := alertmanager-sns-forwarder
GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')
 

all: clean dep test bin

bin: bin/linux/${PROJECT_NAME}

bin/%: LDFLAGS=-X github.com/DataReply/${PROJECT_NAME}/${PROJECT_NAME}.Version=${APP_VERSION}
bin/%: $(GOFILES)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o "bin/darwin/${PROJECT_NAME}" github.com/DataReply/${PROJECT_NAME}/
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o "bin/linux/${PROJECT_NAME}" github.com/DataReply/${PROJECT_NAME}/
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o "bin/windows/${PROJECT_NAME}" github.com/DataReply/${PROJECT_NAME}/

test:
	CGO_ENABLED=0 go test github.com/DataReply/${PROJECT_NAME}/...

dep:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure -vendor-only

clean:
	rm -rf bin

.PHONY: all
