.PHONY: fmt lint test integration testnode bench-pid bench-local-messaging

fmt:
	gofmt -w ./src ./tests

lint:
	go vet ./...

test:
	go test ./...

testnode:
	GOOS=linux GOARCH=arm64 go build -o testnode-linux ./cmd/testnode/

integration: testnode
	WESTCOAST_INTEGRATION=1 NOVA_BIN=$$(which nova) go test ./tests/integration_cluster/ -v -timeout 10m

bench-pid:
	WC_BENCH_TARGET=200000 go test ./tests/benchmark -run '^$$' -bench BenchmarkPIDResolverLatency -benchmem -benchtime=3s

bench-local-messaging:
	WC_BENCH_TARGET=200000 go test ./tests/benchmark -run '^$$' -bench BenchmarkLocalMessagingPerformance -benchmem -benchtime=3s
