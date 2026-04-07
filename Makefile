BINARY_NAME := el

.PHONY: build install clean

build:
	go build -o $(BINARY_NAME) .

install:
	go build -o $(shell go env GOPATH)/bin/el .

clean:
	rm -f $(BINARY_NAME)
