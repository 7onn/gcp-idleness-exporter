test:
	go test -cover ./...

help:
	go run . -h

build:
	go build -o server .
