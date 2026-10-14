.PHONY: build test install clean

build:
	go build -o logirgb ./cmd/logirgb

test:
	go vet ./...
	go test ./...

install:
	go install ./cmd/logirgb

clean:
	rm -f logirgb
