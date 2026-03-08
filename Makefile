.PHONY: fmt lint test

fmt:
	gofmt -w ./src ./tests

lint:
	go vet ./...

test:
	go test ./...
