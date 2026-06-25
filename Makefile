VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test bench lint install clean demo release

build:
	go build $(LDFLAGS) -o pgloop .

test:
	go test ./... -v

bench:
	go test ./internal/lockmapper -bench=. -benchtime=5s

lint:
	golangci-lint run ./...

install:
	go install $(LDFLAGS) .

demo: build
	@./demo/run.sh

release:
	goreleaser release --clean

clean:
	rm -f pgloop
