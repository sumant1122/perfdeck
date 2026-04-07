.PHONY: build run test clean

build:
	go build -ldflags "-X main.version=dev" -o perfdeck .

run:
	go run .

test:
	go test ./...

clean:
	rm -f perfdeck
