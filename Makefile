.PHONY: fmt lint test bench-pid bench-local-messaging

fmt:
	gofmt -w ./src ./tests

lint:
	go vet ./...

test:
	go test ./...

bench-pid:
	WC_BENCH_TARGET=200000 go test ./tests/benchmark -run '^$$' -bench BenchmarkPIDResolverLatency -benchmem -benchtime=3s

bench-local-messaging:
	WC_BENCH_TARGET=200000 go test ./tests/benchmark -run '^$$' -bench BenchmarkLocalMessagingPerformance -benchmem -benchtime=3s
