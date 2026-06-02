BINARY  := bin/openerp
VERSION ?= 0.1.0-poc
LDFLAGS := -X github.com/zhoujw/openerp-cli/cmd.Version=$(VERSION)

.PHONY: build vet fmt-check unit-test test clean help

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

vet:
	go vet ./...

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "needs gofmt:"; echo "$$out"; exit 1; fi

unit-test:
	go test ./...

test: vet fmt-check unit-test

clean:
	rm -rf bin

help:
	go run . --help
